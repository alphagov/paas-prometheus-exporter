package cf

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"

	logcache "code.cloudfoundry.org/log-cache/pkg/client"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry/noaa/consumer"
)

type ServiceInstance struct {
	cfclient.ServiceInstance
	SpaceData cfclient.SpaceResource
}

//go:generate counterfeiter -o mocks/client.go . Client
type Client interface {
	ListAppsWithSpaceAndOrg() ([]cfclient.App, error)
	ListServicesWithSpaceAndOrg() ([]ServiceInstance, error)

	NewAppStreamProvider(appGUID string) AppStreamProvider
	GetToken() (token string, authError error)
	consumer.TokenRefresher
	DopplerEndpoint() string
	NewLogCacheClient() LogCacheClient
}

type client struct {
	config           *cfclient.Config
	cfClient         *cfclient.Client
	logCacheEndpoint string
}

func NewClient(config *cfclient.Config, logCacheEndpoint string) (Client, error) {
	cfClient, err := cfclient.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &client{
		config:           config,
		cfClient:         cfClient,
		logCacheEndpoint: logCacheEndpoint,
	}, nil
}

func (c *client) getOrgsAndSpacesByGuid() (map[string]cfclient.Org, map[string]cfclient.Space, error) {
	var envOrgs = os.Getenv("ENV_ORGS")
	var envSpaces = os.Getenv("ENV_SPACES")

	org := cfclient.Org{}
	err := errors.New("")

	spacesByGuid := map[string]cfclient.Space{}
	orgsByGuid := map[string]cfclient.Org{}

	if envOrgs != "" {
		orgNames := strings.Split(envOrgs, ",")
		for _, orgName := range orgNames {
			org, err = c.cfClient.GetOrgByName(orgName)
			if err != nil {
				log.Printf(err.Error())
				return nil, nil, err
			}
			orgsByGuid[org.Guid] = org
		}
	} else {
		orgs, err := c.cfClient.ListOrgs()
		if err != nil {
			return nil, nil, err
		}
		for _, org := range orgs {
			orgsByGuid[org.Guid] = org
		}
	}
	if envSpaces != "" {
		spaceNames := strings.Split(envSpaces, ",")
		for _, spaceName := range spaceNames {
			for _, orgByGuid := range orgsByGuid {
				space, err := c.cfClient.GetSpaceByName(spaceName, orgByGuid.Guid)
				if err != nil {
					log.Printf(err.Error())
				} else {
					spacesByGuid[space.Guid] = space
				}
			}
		}
	} else {
		spaces, err := c.cfClient.ListSpaces()
		if err != nil {
			return orgsByGuid, nil, err
		}
		for _, space := range spaces {
			spacesByGuid[space.Guid] = space
		}
	}
	return orgsByGuid, spacesByGuid, nil
}

func (c *client) ListAppsWithSpaceAndOrg() ([]cfclient.App, error) {
	orgsByGuid, spacesByGuid, err := c.getOrgsAndSpacesByGuid()
	if err != nil {
		return nil, err
	}

	apps, err := c.cfClient.ListAppsByQuery(url.Values{})
	resultApps := []cfclient.App{}
	if err != nil {
		return apps, err
	}
	for idx, app := range apps {
		space, ok := spacesByGuid[app.SpaceGuid]
		if !ok {
			continue
		}
		org, ok := orgsByGuid[space.OrganizationGuid]
		if !ok {
			continue
		}
		space.OrgData.Entity = org
		app.SpaceData.Entity = space
		apps[idx] = app
		resultApps = append(resultApps, apps[idx])
	}
	return resultApps, nil
}

func (c *client) ListServicesWithSpaceAndOrg() ([]ServiceInstance, error) {
	orgsByGuid, spacesByGuid, err := c.getOrgsAndSpacesByGuid()
	if err != nil {
		return nil, err
	}
	services, err := c.cfClient.ListServiceInstances()
	if err != nil {
		return nil, err
	}
	resultServices := []ServiceInstance{}
	for _, service := range services {
		space, ok := spacesByGuid[service.SpaceGuid]
		if !ok {
			continue
		}
		org, ok := orgsByGuid[space.OrganizationGuid]
		if !ok {
			continue
		}
		space.OrgData.Entity = org

		resultServices = append(resultServices, ServiceInstance{
			ServiceInstance: service,
			SpaceData:       cfclient.SpaceResource{Entity: space},
		})
	}
	return resultServices, nil
}

func (c *client) NewAppStreamProvider(appGUID string) AppStreamProvider {
	return NewDopplerAppStreamProvider(c, appGUID)
}

// RefreshAuthToken satisfies the `consumer.TokenRefresher` interface.
func (c *client) RefreshAuthToken() (token string, authError error) {
	return c.GetToken()
}

func (c *client) GetToken() (token string, authError error) {
	token, err := c.cfClient.GetToken()
	if err != nil {
		cfClient, err := cfclient.NewClient(c.config)
		if err != nil {
			return "", err
		}

		c.cfClient = cfClient

		return c.cfClient.GetToken()
	}

	return token, nil
}

func (c *client) DopplerEndpoint() string {
	return c.cfClient.Endpoint.DopplerEndpoint
}

func (c *client) NewLogCacheClient() LogCacheClient {
	return logcache.NewClient(c.logCacheEndpoint,
		logcache.WithHTTPClient(&logCacheHTTPClient{
			tokenSource: c.cfClient.Config.TokenSource,
			client:      http.DefaultClient,
		}),
	)
}

type logCacheHTTPClient struct {
	tokenSource oauth2.TokenSource
	client      *http.Client
}

func (l *logCacheHTTPClient) Do(req *http.Request) (*http.Response, error) {
	token, err := getTokenWithRetry(l.tokenSource, 3, 1*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %s", err)
	}

	authHeader := fmt.Sprintf("bearer %s", token.AccessToken)
	req.Header.Set("Authorization", authHeader)

	return l.client.Do(req)
}

func getTokenWithRetry(tokenSource oauth2.TokenSource, maxRetries int, fallOffSeconds time.Duration) (*oauth2.Token, error) {
	var (
		i     int
		token *oauth2.Token
		err   error
	)

	for i = 0; i < maxRetries; i++ {
		token, err = tokenSource.Token()

		if err != nil {
			log.Printf("getting token failed (attempt %d of %d). Retrying. Error: %s", i+1, maxRetries, err.Error())

			sleep := time.Duration(fallOffSeconds.Seconds() * float64(i+1))
			time.Sleep(sleep)
			continue
		}
		return token, nil
	}

	return token, err
}
