package exporter

import (
  "github.com/alphagov/paas-prometheus-exporter/events"
  "github.com/cloudfoundry-community/go-cfclient"

  "github.com/prometheus/client_golang/prometheus"
)

//go:generate counterfeiter -o mocks/watcher_manager.go . WatcherManager
type WatcherManager interface {
	AddWatcher(cfclient.App, prometheus.Registerer)
	DeleteWatcher(appGuid string)
	UpdateAppInstances(appGuid string, instances int)
}

type ConcreteWatcherManager struct {
	config         *cfclient.Config
	watchers       map[string]*events.AppWatcher
}

func NewWatcherManager(config *cfclient.Config) WatcherManager {
	return &ConcreteWatcherManager{
		config:   config,
		watchers: make(map[string]*events.AppWatcher),
	}
}

func (wm *ConcreteWatcherManager) AddWatcher(app cfclient.App, registry prometheus.Registerer) {
	var provider events.AppStreamProvider = &events.DopplerAppStreamProvider{
		Config: wm.config,
	}
	wm.watchers[app.Guid] =	events.NewAppWatcher(app, registry, provider)
}

func (wm *ConcreteWatcherManager) DeleteWatcher(appGuid string) {
	wm.watchers[appGuid].Close()
	delete(wm.watchers, appGuid)
}

func (wm *ConcreteWatcherManager) UpdateAppInstances(appGuid string, instances int) {
	wm.watchers[appGuid].UpdateAppInstances(instances)
}
