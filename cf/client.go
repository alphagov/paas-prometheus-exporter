package cf

import (
	"fmt"
	"net/http"
	"net/url"

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
	ListProcessWithAppsSpaceAndOrg() ([]AppWithProcesses, error)
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

type AppWithProcesses struct {
	AppGUID   string
	Processes []cfclient.Process
	App       cfclient.App
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

func (c *client) ListProcessWithAppsSpaceAndOrg() ([]AppWithProcesses, error) {
	var appswithprocesses []AppWithProcesses

	apps, err := c.cfClient.ListAppsByQuery(url.Values{})
	if err != nil {
		return appswithprocesses, err
	}
	for idx, app := range apps {
		space, err := app.Space()
		if err != nil {
			return appswithprocesses, err
		}
		org, err := space.Org()
		if err != nil {
			return appswithprocesses, err
		}
		space.OrgData.Entity = org
		app.SpaceData.Entity = space
		apps[idx] = app
		process := AppWithProcesses{
			AppGUID:   app.Guid,
			Processes: []cfclient.Process{},
			App:       app,
		}
		appswithprocesses = append(appswithprocesses, process)
	}
	return appswithprocesses, nil
}

func (c *client) ListServicesWithSpaceAndOrg() ([]ServiceInstance, error) {
	services, err := c.cfClient.ListServiceInstances()
	if err != nil {
		return nil, err
	}
	resultServices := []ServiceInstance{}
	for _, service := range services {
		space, err := c.cfClient.GetSpaceByGuid(service.SpaceGuid)
		if err != nil {
			return nil, err
		}
		org, err := space.Org()
		if err != nil {
			return nil, err
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
	token, err := l.tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get token: %s", err)
	}

	authHeader := fmt.Sprintf("bearer %s", token.AccessToken)
	req.Header.Set("Authorization", authHeader)

	return l.client.Do(req)
}
