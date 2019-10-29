package service

import (
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/alphagov/paas-prometheus-exporter/util"

	"github.com/alphagov/paas-prometheus-exporter/cf"
	"github.com/prometheus/client_golang/prometheus"
)

var reservedLabels = []string{"guid", "service", "space", "organisation"}

var excludedLabels = []string{"deployment", "index", "ip", "job", "origin"}

var validUnits = []string{"percent", "byte", "bytes", "s", "second", "seconds", "ms"}

type timestampedCollector struct {
	prometheus.Collector
	t time.Time
}

func (m *timestampedCollector) Collect(out chan<- prometheus.Metric) {
	metricsChan := make(chan prometheus.Metric, 1)

	go func() {
		m.Collector.Collect(metricsChan)
		close(metricsChan)
	}()

	for metric := range metricsChan {
		out <- prometheus.NewMetricWithTimestamp(m.t, metric)
	}
}

type Watcher struct {
	logcacheClient     cf.LogCacheClient
	service            cf.ServiceInstance
	registerer         prometheus.Registerer
	cancel             context.CancelFunc
	metricsForInstance map[string]*timestampedCollector
	checkInterval      time.Duration
}

func NewWatcher(
	service cf.ServiceInstance,
	registerer prometheus.Registerer,
	logcacheClient cf.LogCacheClient,
	checkInterval time.Duration,
) *Watcher {
	serviceRegisterer := prometheus.WrapRegistererWith(
		prometheus.Labels{
			"guid":         service.Guid,
			"service":      service.Name,
			"space":        service.SpaceData.Entity.Name,
			"organisation": service.SpaceData.Entity.OrgData.Entity.Name,
		},
		registerer,
	)

	watcher := &Watcher{
		logcacheClient:     logcacheClient,
		service:            service,
		registerer:         serviceRegisterer,
		metricsForInstance: map[string]*timestampedCollector{},
		checkInterval:      checkInterval,
	}

	return watcher
}

func (w *Watcher) Run(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)
	return w.mainLoop(ctx)
}

func (w *Watcher) mainLoop(ctx context.Context) error {
	defer w.cancel()

	defer func() {
		for _, metric := range w.metricsForInstance {
			w.registerer.Unregister(metric)
		}
	}()

	timer := time.NewTimer(0)
	defer timer.Stop()

	for {
		select {
		case <-timer.C:
			if err := w.processLogCacheEvents(ctx); err != nil {
				return err
			}
			timer.Reset(w.checkInterval)
		case <-ctx.Done():
			return nil
		}
	}
}

func (w *Watcher) processLogCacheEvents(ctx context.Context) error {
	envelopes, err := readLogsWithRetry(w.logcacheClient, ctx, w.service.Guid, time.Now().Add(-15*time.Minute), 3, 1 * time.Second)
	if err != nil {
		return fmt.Errorf("failed to read the log-cache logs for service %s after all retries: %s", w.service.Guid, err)
	}
	for _, e := range envelopes {
		if g := e.GetGauge(); g != nil {
			for name, gaugeMetric := range g.GetMetrics() {
				gaugeName := name

				if gaugeMetric.GetUnit() != "" {
					for _, validUnit := range validUnits {
						if validUnit == gaugeMetric.GetUnit() {
							gaugeName = gaugeName + "_" + gaugeMetric.GetUnit()
							break
						}
					}
				}

				m, ok := w.metricsForInstance[gaugeName]
				if !ok {
					m = &timestampedCollector{
						Collector: prometheus.NewGauge(
							prometheus.GaugeOpts{
								Name:        util.SanitisePrometheusName(gaugeName),
								ConstLabels: util.SanitisePrometheusLabels(e.Tags, reservedLabels, excludedLabels),
							},
						),
					}
					w.metricsForInstance[gaugeName] = m
					if err := w.registerer.Register(m); err != nil {
						return err
					}
				}
				metricTime := time.Unix(0, e.GetTimestamp())
				if metricTime.After(m.t) {
					m.Collector.(prometheus.Gauge).Set(gaugeMetric.GetValue())
					m.t = metricTime
				}
			}
		}
	}

	return nil
}

func (w *Watcher) Close() {
	if w.cancel == nil {
		log.Fatal("Watcher.Close() called without Start()")
	}
	w.cancel()
}

func readLogsWithRetry(
	client cf.LogCacheClient,
	ctx context.Context,
	serviceGuid string,
	start time.Time,
	maxRetries int,
	fallOffSeconds time.Duration,
) ([]*loggregator_v2.Envelope, error) {
	var (
		i         int
		envelopes []*loggregator_v2.Envelope
		err       error
	)

	for i = 0; i < maxRetries; i++ {
		envelopes, err = client.Read(ctx, serviceGuid, start)

		if err != nil {
			log.Printf("reading log-cache lines for service %s failed (attempt %d of %d). Error: %s", serviceGuid, i+1, maxRetries, err.Error())

			sleep := time.Duration(float64(i+1) * fallOffSeconds.Seconds())
			time.Sleep(sleep)
			continue
		}
		return envelopes, nil
	}

	return envelopes, err
}
