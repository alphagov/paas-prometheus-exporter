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

func newCfNames(app cfclient.App) cfNames {
	return cfNames{
		appName:   app.Name,
		spaceName: app.SpaceData.Entity.Name,
		orgName:   app.SpaceData.Entity.OrgData.Entity.Name,
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

func (e *PaasExporter) createNewWatcher(app cfclient.App) error {
	e.cfNamesByGuid[app.Guid] = newCfNames(app)
	err := e.watcherManager.AddWatcher(app, prometheus.WrapRegistererWith(
		prometheus.Labels{
			"guid": app.Guid,
			"app": app.Name,
			"space": app.SpaceData.Entity.Name,
			"organisation": app.SpaceData.Entity.OrgData.Entity.Name,
		},
		prometheus.DefaultRegisterer,
	))
	if err != nil {
		return err
	}
	return nil
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
				if cfNamesForGuid != newCfNames(app) {
					// Either the name of the app, the name of it's space or the name of it's org has changed
					e.deleteWatcher(app.Guid)
					err := e.createNewWatcher(app)
					if err != nil {
						return err
					}
				} else {
					// notify watcher that instances may have changed
					e.watcherManager.UpdateAppInstances(app)
				}
			} else {
				// new app
				err := e.createNewWatcher(app)
				if err != nil {
					return err
				}
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
