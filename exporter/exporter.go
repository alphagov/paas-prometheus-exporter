package exporter

import (
	"fmt"
	"log"
	"net/http"
	"net/url"

	//"strings"
	"time"

	"github.com/alphagov/paas-prometheus-exporter/events"
	"github.com/cloudfoundry-community/go-cfclient"

	// sonde_events "github.com/cloudfoundry/sonde-go/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:generate counterfeiter -o mocks/cfclient.go . CFClient
type CFClient interface {
	ListAppsByQuery(url.Values) ([]cfclient.App, error)
}

type PaasExporter struct {
	cf       CFClient
	config   *cfclient.Config
	watchers map[string]*events.AppWatcher
}

func New(cf CFClient, config *cfclient.Config) *PaasExporter {
	return &PaasExporter{
		cf: cf,
		config: config,
		watchers: make(map[string]*events.AppWatcher),
	}
}

func (e *PaasExporter) createNewWatcher(app cfclient.App) {
	var provider events.AppStreamProvider = &events.DopplerAppStreamProvider{
		Config: e.config,
	}
	appWatcher := events.NewAppWatcher(app, prometheus.WrapRegistererWith(
		prometheus.Labels{"guid": app.Guid, "app": app.Name},
		prometheus.DefaultRegisterer,
	), provider)
	e.watchers[app.Guid] = appWatcher
	go appWatcher.Run()
}

func (e *PaasExporter) CheckForNewApps() error {
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

func (e *PaasExporter) Start(updateFrequency time.Duration, prometheusBindPort int) {
	go func() {
		for {
			log.Println("checking for new apps")
			err := e.CheckForNewApps()
			if err != nil {
				log.Fatal(err)
			}

			time.Sleep(updateFrequency)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", prometheusBindPort), nil))
}
