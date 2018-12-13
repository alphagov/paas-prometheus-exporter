package cf

import (
	"net/url"

	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/cloudfoundry/noaa/consumer"
)

//go:generate counterfeiter -o mocks/client.go . Client
type Client interface {
	ListAppsWithSpaceAndOrg() ([]cfclient.App, error)
	NewAppStreamProvider(appGUID string) AppStreamProvider
	GetToken() (token string, authError error)
	consumer.TokenRefresher
	DopplerEndpoint() string
}

type client struct {
	config   *cfclient.Config
	cfClient *cfclient.Client
}

func NewClient(config *cfclient.Config) (Client, error) {
	cfClient, err := cfclient.NewClient(config)
	if err != nil {
		return nil, err
	}

	return &client{
		config:   config,
		cfClient: cfClient,
	}, nil
}

func (c *client) ListAppsWithSpaceAndOrg() ([]cfclient.App, error) {
	apps, err := c.cfClient.ListAppsByQuery(url.Values{})
	if err != nil {
		return apps, err
	}
	for idx, app := range apps {
		space, err := app.Space()
		if err != nil {
			return apps, err
		}
		org, err := space.Org()
		if err != nil {
			return apps, err
		}
		space.OrgData.Entity = org
		app.SpaceData.Entity = space
		apps[idx] = app
	}
	return apps, nil
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
