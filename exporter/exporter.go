package exporter

import (
	"log"
	"net/url"
	"time"

	"github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/client_golang/prometheus"
)

//go:generate counterfeiter -o mocks/cfclient.go . CFClient
type CFClient interface {
	ListAppsByQuery(url.Values) ([]cfclient.App, error)
}

type PaasExporter struct {
	cf             CFClient
	watcherManager WatcherManager
	appNameByGuid  map[string]string
}

func New(cf CFClient, wm WatcherManager) *PaasExporter {
	return &PaasExporter{
		cf:             cf,
		watcherManager: wm,
		appNameByGuid:  make(map[string]string),
	}
}

func (e *PaasExporter) createNewWatcher(app cfclient.App) {
	e.appNameByGuid[app.Guid] = app.Name
	e.watcherManager.AddWatcher(app, prometheus.WrapRegistererWith(
		prometheus.Labels{"guid": app.Guid, "app": app.Name},
		prometheus.DefaultRegisterer,
	))
}

func (e *PaasExporter) checkForNewApps() error {
	apps, err := e.cf.ListAppsByQuery(url.Values{})
	if err != nil {
		return err
	}

	running := map[string]bool{}

	for _, app := range apps {
		if app.State == "STARTED" {
			running[app.Guid] = true

			if _, ok := e.appNameByGuid[app.Guid]; ok {
				if e.appNameByGuid[app.Guid] != app.Name {
					// Name changed, stop and restart
					e.watcherManager.DeleteWatcher(app.Guid)
					e.createNewWatcher(app)
				} else {
					// notify watcher that instances may have changed
					e.watcherManager.UpdateAppInstances(app.Guid, app.Instances)
				}
			} else {
				// new app
				e.createNewWatcher(app)
			}
		}
	}

	for appGuid, _ := range e.appNameByGuid {
		if ok := running[appGuid]; !ok {
			e.watcherManager.DeleteWatcher(appGuid)
			delete(e.appNameByGuid, appGuid)
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
