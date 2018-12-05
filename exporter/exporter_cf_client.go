package exporter

import (
	"net/url"

	"github.com/cloudfoundry-community/go-cfclient"
)

type ExporterCFClient struct {
	cf	*cfclient.Client
}

func NewCFClient(cf *cfclient.Client) CFClient {
	return &ExporterCFClient {
		cf: cf,
	}
}

func (e *ExporterCFClient) ListAppsWithSpaceAndOrg() ([]cfclient.App, error) {
	apps, err := e.cf.ListAppsByQuery(url.Values{})
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
