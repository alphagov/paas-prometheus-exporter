package cf

import (
	"fmt"
	"net/url"
	"strings"

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
}

type OptionFunc func(c *client) error

type client struct {
	config           *cfclient.Config
	cfClient         *cfclient.Client
	spaces           []string
	logCacheEndpoint string
}

func WithSpaces(spaces string) OptionFunc {
	list := strings.Split(spaces, ",")
	if len(list) == 1 && list[0] == "" {
		list = []string{}
	}

	return func(c *client) error {
		c.spaces = list
		return nil
	}
}

func NewClient(config *cfclient.Config, logCacheEndpoint string, opts ...OptionFunc) (Client, error) {
	cfClient, err := cfclient.NewClient(config)
	if err != nil {
		return nil, err
	}

	client := &client{
		config:           config,
		cfClient:         cfClient,
		logCacheEndpoint: logCacheEndpoint,
	}
	for _, o := range opts {
		if err := o(client); err != nil {
			return nil, err
		}
	}
	return client, nil
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

	queryParams := url.Values{}

	if len(c.spaces) > 0 {
		queryParams.Add("q", fmt.Sprintf("space_guid IN %s", strings.Join(c.spaces, ",")))
	}

	apps, err := c.cfClient.ListAppsByQuery(queryParams)
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

	queryParams := url.Values{}

	if len(c.spaces) > 0 {
		queryParams.Add("q", fmt.Sprintf("space_guid IN %s", strings.Join(c.spaces, ",")))
	}

	services, err := c.cfClient.ListServiceInstancesByQuery(queryParams)
	if err != nil {
		return nil, err
	}
	var resultServices []ServiceInstance
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
