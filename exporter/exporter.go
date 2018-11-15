package exporter

import (
	"log"
	"net/url"

	"time"

	"github.com/alphagov/paas-prometheus-exporter/events"
	"github.com/cloudfoundry-community/go-cfclient"

	"github.com/prometheus/client_golang/prometheus"
)

//go:generate counterfeiter -o mocks/cfclient.go . CFClient
type CFClient interface {
	ListAppsByQuery(url.Values) ([]cfclient.App, error)
}

//go:generate counterfeiter -o mocks/watcher_manager.go . WatcherManager
type WatcherManager interface {
	CreateWatcher(cfclient.App, prometheus.Registerer) *events.AppWatcher
	DeleteWatcher(appGuid string)
}

type ConcreteWatcherManager struct {
	Config *cfclient.Config
}

func (wm *ConcreteWatcherManager) CreateWatcher(app cfclient.App, registry prometheus.Registerer) *events.AppWatcher {
	var provider events.AppStreamProvider = &events.DopplerAppStreamProvider{
		Config: wm.Config,
	}
	return events.NewAppWatcher(app, registry, provider)
}

func (wm *ConcreteWatcherManager) DeleteWatcher(appGuid string) {

}

type PaasExporter struct {
	cf             CFClient
	watchers       map[string]*events.AppWatcher
	watcherManager WatcherManager
}

func New(cf CFClient, wc WatcherManager) *PaasExporter {
	return &PaasExporter{
		cf:             cf,
		watchers:       make(map[string]*events.AppWatcher),
		watcherManager: wc,
	}
}

func (e *PaasExporter) createNewWatcher(app cfclient.App) {
	appWatcher := e.watcherManager.CreateWatcher(app, prometheus.WrapRegistererWith(
		prometheus.Labels{"guid": app.Guid, "app": app.Name},
		prometheus.DefaultRegisterer,
	))

	e.watchers[app.Guid] = appWatcher
}

func (e *PaasExporter) checkForNewApps() error {
	apps, err := e.cf.ListAppsByQuery(url.Values{})
	if err != nil {
		return err
	}

	running := map[string]bool{}

	for _, app := range apps {
		// Do we need to check they're running or does the API return all of them?
		// need to check app.State is "STARTED"
		running[app.Guid] = true

		appWatcher, present := e.watchers[app.Guid]
		if present {
			if appWatcher.AppName() != app.Name {
				// Name changed, stop and restart
				appWatcher.Close()
				e.createNewWatcher(app)
			} else {
				// notify watcher that instances may have changed
				appWatcher.UpdateApp(app)
			}
		} else {
			// new app
			e.createNewWatcher(app)
		}
	}

	for appGuid, appWatcher := range e.watchers {
		if ok := running[appGuid]; !ok {
			appWatcher.Close()
			delete(e.watchers, appGuid)
		}
	}
	return nil
}

func (e *PaasExporter) WatcherCount() int {
	return len(e.watchers)
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
