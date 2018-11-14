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
	ListAppsByQuery() ([]cfclient.App, error)
}

var appWatchers = make(map[string]*events.AppWatcher)

func createNewWatcher(config *cfclient.Config, app cfclient.App) {
	appWatcher := events.NewAppWatcher(config, app, prometheus.WrapRegistererWith(
		prometheus.Labels{"guid": app.Guid, "app": app.Name},
		prometheus.DefaultRegisterer,
	))
	appWatchers[app.Guid] = appWatcher
	go appWatcher.Run()
}

func Test(client CFClient) ([]cfclient.App, error) {
	return client.ListAppsByQuery()
}

func checkForNewApps(cf *cfclient.Client, config *cfclient.Config) error {
	apps, err := cf.ListAppsByQuery(url.Values{})
	if err != nil {
		return err
	}

	return CheckForNewAppsNew(apps, config)
}

func CheckForNewAppsNew(apps []cfclient.App, config *cfclient.Config) error {
	running := map[string]bool{}

	for _, app := range apps {
		// Do we need to check they're running or does the API return all of them?
		running[app.Guid] = true

		appWatcher, present := appWatchers[app.Guid]
		if present {
			if appWatcher.AppName() != app.Name {
				// Name changed, stop and restart
				appWatcher.Close()
				createNewWatcher(config, app)
			} else {
				// notify watcher that instances may have changed
				appWatcher.UpdateApp(app)
			}
		} else {
			// new app
			createNewWatcher(config, app)
		}
	}

	for appGuid, appWatcher := range appWatchers {
		if ok := running[appGuid]; !ok {
			appWatcher.Close()
			delete(appWatchers, appGuid)
		}
	}
	return nil
}

func StartApp(apiEndpoint string, skipSSLValidation bool, username string, password string, clientID string, clientSecret string, updateFrequency time.Duration, prometheusBindPort int) {
	config := &cfclient.Config{
		ApiAddress:        apiEndpoint,
		SkipSslValidation: skipSSLValidation,
		Username:          username,
		Password:          password,
		ClientID:          clientID,
		ClientSecret:      clientSecret,
	}

	cf, err := cfclient.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			log.Println("checking for new apps")
			err := checkForNewApps(cf, config)
			if err != nil {
				log.Fatal(err)
			}

			time.Sleep(updateFrequency)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", prometheusBindPort), nil))
}
