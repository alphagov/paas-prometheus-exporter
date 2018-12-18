package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/alphagov/paas-prometheus-exporter/cf"
	"github.com/prometheus/client_golang/prometheus"
)

type serviceMetadata struct {
	serviceName string
	spaceName   string
	orgName     string
}

func newServiceMetadata(service cf.ServiceInstance) serviceMetadata {
	return serviceMetadata{
		serviceName: service.Name,
		spaceName:   service.SpaceData.Entity.Name,
		orgName:     service.SpaceData.Entity.OrgData.Entity.Name,
	}
}

type Discovery struct {
	lock                  sync.Mutex
	client                cf.Client
	prometheusRegisterer  prometheus.Registerer
	checkInterval         time.Duration
	watcherInterval       time.Duration
	serviceMetadataByGUID map[string]serviceMetadata
	watchers              map[string]*Watcher
}

func NewDiscovery(
	client cf.Client,
	prometheusRegisterer prometheus.Registerer,
	checkInterval time.Duration,
	watcherInterval time.Duration,
) *Discovery {
	return &Discovery{
		client:                client,
		prometheusRegisterer:  prometheusRegisterer,
		checkInterval:         checkInterval,
		watcherInterval:       watcherInterval,
		serviceMetadataByGUID: make(map[string]serviceMetadata),
		watchers:              make(map[string]*Watcher),
	}
}

func (s *Discovery) Start(ctx context.Context, errChan chan error) {
	go func() {
		if err := s.run(ctx); err != nil {
			errChan <- err
		}
	}()
}

func (s *Discovery) run(ctx context.Context) error {
	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			if err := s.checkForNewServices(ctx); err != nil {
				return err
			}
			timer.Reset(s.checkInterval)
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *Discovery) checkForNewServices(ctx context.Context) error {
	log.Println("checking for new services")

	services, err := s.client.ListServicesWithSpaceAndOrg()
	if err != nil {
		return fmt.Errorf("failed to list the services: %s", err)
	}

	running := map[string]bool{}

	for _, service := range services {
		running[service.Guid] = true

		if serviceMetadata, ok := s.serviceMetadataByGUID[service.Guid]; ok {
			if serviceMetadata != newServiceMetadata(service) {
				s.deleteWatcher(service.Guid)
				s.createNewWatcher(ctx, service)
			}
		} else {
			s.createNewWatcher(ctx, service)
		}
	}

	for serviceGUID, _ := range s.serviceMetadataByGUID {
		if ok := running[serviceGUID]; !ok {
			s.deleteWatcher(serviceGUID)
		}
	}
	return nil
}

func (s *Discovery) createNewWatcher(ctx context.Context, service cf.ServiceInstance) {
	s.lock.Lock()
	defer s.lock.Unlock()

	logCacheClient := s.client.NewLogCacheClient()

	watcher := NewWatcher(service, s.prometheusRegisterer, logCacheClient, s.watcherInterval)

	s.watchers[service.Guid] = watcher
	s.serviceMetadataByGUID[service.Guid] = newServiceMetadata(service)

	go func() {
		err := watcher.Run(ctx)
		if err != nil {
			log.Println(err)
			s.deleteWatcher(service.Guid)
		}
	}()
}

func (s *Discovery) deleteWatcher(serviceGUID string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.watchers[serviceGUID].Close()
	delete(s.watchers, serviceGUID)
	delete(s.serviceMetadataByGUID, serviceGUID)
}
