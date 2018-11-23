package exporter

import (
	"log"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/client_golang/prometheus"
)

//go:generate counterfeiter -o mocks/cfclient.go . CFClient
type CFClient interface {
	ListAppsWithSpaceAndOrg() ([]cfclient.App, error)
}

// Struct to store all names related to an app (app name, space name, org name) so we can track if these have changed
// for a given app and if so delete and recreate its app watcher
type cfNames struct {
	appName   string
	spaceName string
	orgName   string
}

func newCfNames(appName string, spaceName string, orgName string) cfNames {
	return cfNames{
		appName:   appName,
		spaceName: spaceName,
		orgName:   orgName,
	}
}

type PaasExporter struct {
	cf             CFClient
	watcherManager WatcherManager
	cfNamesByGuid  map[string]cfNames
}

func New(cf CFClient, wm WatcherManager) *PaasExporter {
	return &PaasExporter{
		cf:             cf,
		watcherManager: wm,
		cfNamesByGuid:  make(map[string]cfNames),
	}
}

func (e *PaasExporter) createNewWatcher(app cfclient.App) {
	e.cfNamesByGuid[app.Guid] = newCfNames(
		app.Name,
		app.SpaceData.Entity.Name,
		app.SpaceData.Entity.OrgData.Entity.Name,
	)
	e.watcherManager.AddWatcher(app, prometheus.WrapRegistererWith(
		prometheus.Labels{
			"guid": app.Guid,
			"app": app.Name,
			"space": app.SpaceData.Entity.Name,
			"org": app.SpaceData.Entity.OrgData.Entity.Name,
		},
		prometheus.DefaultRegisterer,
	))
}

func (e *PaasExporter) deleteWatcher(appGuid string) {
	e.watcherManager.DeleteWatcher(appGuid)
	delete(e.cfNamesByGuid, appGuid)
}

func (e *PaasExporter) checkForNewApps() error {
	apps, err := e.cf.ListAppsWithSpaceAndOrg()
	if err != nil {
		return err
	}

	running := map[string]bool{}

	for _, app := range apps {
		if app.State == "STARTED" {
			running[app.Guid] = true

			if cfNamesForGuid, ok := e.cfNamesByGuid[app.Guid]; ok {
				latestCFNames := newCfNames(
					app.Name,
					app.SpaceData.Entity.Name,
					app.SpaceData.Entity.OrgData.Entity.Name,
				)
				if cfNamesForGuid != latestCFNames {
					// Either the name of the app, the name of it's space or the name of it's org has changed
					e.deleteWatcher(app.Guid)
					e.createNewWatcher(app)
				} else {
					// notify watcher that instances may have changed
					e.watcherManager.UpdateAppInstances(app)
				}
			} else {
				// new app
				e.createNewWatcher(app)
			}
		}
	}

	for appGuid, _ := range e.cfNamesByGuid {
		if ok := running[appGuid]; !ok {
			e.deleteWatcher(appGuid)
		}
	}
	return nil
}

func (e *PaasExporter) Start(updateFrequency time.Duration) {
	for {
		log.Println("checking for new apps")
		err := e.checkForNewApps()
		if err != nil {
			log.Fatal(err)
		}

		time.Sleep(updateFrequency)
	}
}
