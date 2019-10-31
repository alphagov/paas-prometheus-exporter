package cf

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
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
	orgs, err := c.cfClient.ListOrgs()
	if err != nil {
		return nil, nil, err
	}
	orgsByGuid := map[string]cfclient.Org{}
	for _, org := range orgs {
		orgsByGuid[org.Guid] = org
	}
	spaces, err := c.cfClient.ListSpaces()
	if err != nil {
		return orgsByGuid, nil, err
	}
	spacesByGuid := map[string]cfclient.Space{}
	for _, space := range spaces {
		spacesByGuid[space.Guid] = space
	}
	return orgsByGuid, spacesByGuid, nil
}

func (c *client) ListAppsWithSpaceAndOrg() ([]cfclient.App, error) {
	orgsByGuid, spacesByGuid, err := c.getOrgsAndSpacesByGuid()
	if err != nil {
		return nil, err
	}

	apps, err := c.cfClient.ListAppsByQuery(url.Values{})
	if err != nil {
		return apps, err
	}
	for idx, app := range apps {
		space, ok := spacesByGuid[app.SpaceGuid]
		if !ok {
			return apps, fmt.Errorf(
				"could not find a space for app %s, space guid %s",
				app.Guid,
				app.SpaceGuid,
			)
		}
		org, ok := orgsByGuid[space.OrganizationGuid]
		if !ok {
			return apps, fmt.Errorf(
				"could not find an org for app %s in space %s, org guid %s",
				app.Guid,
				space.Guid,
				space.OrganizationGuid,
			)
		}
		space.OrgData.Entity = org
		app.SpaceData.Entity = space
		apps[idx] = app
	}
	return apps, nil
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
			return nil, fmt.Errorf(
				"could not find a space for service %s, space guid %s",
				service.Guid,
				service.SpaceGuid,
			)
		}
		org, ok := orgsByGuid[space.OrganizationGuid]
		if !ok {
			return nil, fmt.Errorf(
				"could not find a org for service %s in space %s, org guid %s",
				service.Guid,
				service.SpaceGuid,
				space.OrganizationGuid,
			)
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

			sleep := time.Duration(fallOffSeconds.Seconds() * float64(i + 1))
			time.Sleep(sleep)
			continue
		}
		return token, nil
	}

	return token, err
}
