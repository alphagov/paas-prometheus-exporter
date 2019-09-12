package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/alphagov/paas-prometheus-exporter/cf"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	sonde_events "github.com/cloudfoundry/sonde-go/events"
	"github.com/prometheus/client_golang/prometheus"
)

type Watcher struct {
	app                   cfclient.App
	MetricsForInstance    []InstanceMetrics
	numberOfInstancesChan chan int
	registerer            prometheus.Registerer
	streamProvider        cf.AppStreamProvider
	cancel                context.CancelFunc
}

type InstanceMetrics struct {
	Registerer        prometheus.Registerer
	Cpu               prometheus.Gauge
	Crash             prometheus.Counter
	DiskBytes         prometheus.Gauge
	DiskUtilization   prometheus.Gauge
	MemoryBytes       prometheus.Gauge
	MemoryUtilization prometheus.Gauge
	Requests          *prometheus.CounterVec
	ResponseTime      *prometheus.HistogramVec
}

func NewInstanceMetrics(instanceIndex int, registerer prometheus.Registerer) (InstanceMetrics, error) {
	constLabels := prometheus.Labels{
		"instance": fmt.Sprintf("%d", instanceIndex),
	}

	im := InstanceMetrics{
		Registerer: registerer,
		Cpu: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "cpu",
				Help:        "CPU utilisation in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
		Crash: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name:        "crash",
				Help:        "Number of app instance crashes",
				ConstLabels: constLabels,
			},
		),
		DiskBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "disk_bytes",
				Help:        "Disk usage in bytes",
				ConstLabels: constLabels,
			},
		),
		DiskUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "disk_utilization",
				Help:        "Disk space currently in use in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
		MemoryBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "memory_bytes",
				Help:        "Memory usage in bytes",
				ConstLabels: constLabels,
			},
		),
		MemoryUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name:        "memory_utilization",
				Help:        "Memory currently in use in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
		Requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:        "requests",
				Help:        "Counter of http requests for a given app instance",
				ConstLabels: constLabels,
			},
			[]string{"status_range"},
		),
		ResponseTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:        "response_time",
				Help:        "Histogram of http request time for a given app instance",
				ConstLabels: constLabels,
			},
			[]string{"status_range"},
		),
	}

	// This initalises with a zero value for requests and responseTime for each
	// statusRange
	for _, statusRange := range []string{"2xx", "3xx", "4xx", "5xx"} {
		_, err := im.Requests.GetMetricWithLabelValues(statusRange)
		if err != nil {
			return im, err
		}

		_, err = im.ResponseTime.GetMetricWithLabelValues(statusRange)
		if err != nil {
			return im, err
		}
	}

	im.registerInstanceMetrics()

	return im, nil
}

func (im *InstanceMetrics) registerInstanceMetrics() {
	for _, collector := range []prometheus.Collector{
		im.Cpu,
		im.Crash,
		im.DiskBytes,
		im.DiskUtilization,
		im.MemoryBytes,
		im.MemoryUtilization,
		im.Requests,
		im.ResponseTime,
	} {
		if err := im.Registerer.Register(collector); err != nil {
			panic(err)
		}
	}

}

func (im *InstanceMetrics) unregisterInstanceMetrics() {
	im.Registerer.Unregister(im.Cpu)
	im.Registerer.Unregister(im.Crash)
	im.Registerer.Unregister(im.DiskBytes)
	im.Registerer.Unregister(im.DiskUtilization)
	im.Registerer.Unregister(im.MemoryBytes)
	im.Registerer.Unregister(im.MemoryUtilization)
	im.Registerer.Unregister(im.Requests)
	im.Registerer.Unregister(im.ResponseTime)
}

func NewWatcher(
	app cfclient.App,
	registerer prometheus.Registerer,
	streamProvider cf.AppStreamProvider,
) (*Watcher, error) {
	appRegisterer := prometheus.WrapRegistererWith(
		prometheus.Labels{
			"guid":         app.Guid,
			"app":          app.Name,
			"space":        app.SpaceData.Entity.Name,
			"organisation": app.SpaceData.Entity.OrgData.Entity.Name,
		},
		registerer,
	)

	watcher := &Watcher{
		app:                   app,
		MetricsForInstance:    make([]InstanceMetrics, 0),
		numberOfInstancesChan: make(chan int, 5),
		registerer:            appRegisterer,
		streamProvider:        streamProvider,
	}

	return watcher, nil
}

func (w *Watcher) Run(ctx context.Context) error {
	ctx, w.cancel = context.WithCancel(ctx)

	msgs, errs := w.streamProvider.Start()
	defer w.streamProvider.Close()

	w.scaleTo(w.app.Instances)
	defer w.scaleTo(0)
	return w.mainLoop(ctx, msgs, errs)
}

func (w *Watcher) mainLoop(ctx context.Context, msgs <-chan *sonde_events.Envelope, errs <-chan error) error {
	for {
		select {
		case message, ok := <-msgs:
			if !ok {
				return fmt.Errorf("AppWatcher messages channel was closed, for guid %s", w.app.Guid)
			}
			switch message.GetEventType() {
			case sonde_events.Envelope_LogMessage:
				err := w.processLogMessage(message.GetLogMessage())
				if err != nil {
					return err
				}
			case sonde_events.Envelope_ContainerMetric:
				w.processContainerMetric(message.GetContainerMetric())
			case sonde_events.Envelope_HttpStartStop:
				w.processHttpStartStopMetric(message.GetHttpStartStop())
			}
		case err, ok := <-errs:
			if !ok {
				return fmt.Errorf("AppWatcher errors channel was closed, for guid %s", w.app.Guid)
			}
			if err == nil {
				continue
			}
			return err
		case newNumberOfInstances := <-w.numberOfInstancesChan:
			err := w.scaleTo(newNumberOfInstances)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}

	}
}

func (w *Watcher) processContainerMetric(metric *sonde_events.ContainerMetric) {
	index := metric.GetInstanceIndex()
	if int(index) < len(w.MetricsForInstance) {
		instance := w.MetricsForInstance[index]

		diskUtilizationPercentage := float64(metric.GetDiskBytes()) / float64(metric.GetDiskBytesQuota()) * 100
		memoryUtilizationPercentage := float64(metric.GetMemoryBytes()) / float64(metric.GetMemoryBytesQuota()) * 100

		instance.Cpu.Set(metric.GetCpuPercentage())
		instance.DiskBytes.Set(float64(metric.GetDiskBytes()))
		instance.DiskUtilization.Set(diskUtilizationPercentage)
		instance.MemoryBytes.Set(float64(metric.GetMemoryBytes()))
		instance.MemoryUtilization.Set(memoryUtilizationPercentage)
	}
}

func (w *Watcher) processLogMessage(logMessage *sonde_events.LogMessage) error {
	if logMessage.GetSourceType() != "API" || logMessage.GetMessageType() != sonde_events.LogMessage_OUT {
		return nil
	}
	if !bytes.HasPrefix(logMessage.Message, []byte("App instance exited with guid ")) {
		return nil
	}

	payloadStartMarker := []byte(" payload: {")
	payloadStartMarkerPosition := bytes.Index(logMessage.Message, payloadStartMarker)
	if payloadStartMarkerPosition < 0 {
		return fmt.Errorf("unable to find start of payload in app instance exit log: %s", logMessage.Message)
	}
	payloadStartPosition := payloadStartMarkerPosition + len(payloadStartMarker) - 1

	payload := logMessage.Message[payloadStartPosition:]
	payloadAsJson := bytes.Replace(payload, []byte("=>"), []byte(":"), -1)

	var logMessagePayload struct {
		Index  int    `json:"index"`
		Reason string `json:"reason"`
	}
	err := json.Unmarshal(payloadAsJson, &logMessagePayload)
	if err != nil {
		return fmt.Errorf("unable to parse payload in app instance exit log: %s", err)
	}

	if logMessagePayload.Reason != "CRASHED" {
		return nil
	}

	index := logMessagePayload.Index
	if index < len(w.MetricsForInstance) {
		w.MetricsForInstance[index].Crash.Inc()
	}
	return nil
}

func (w *Watcher) processHttpStartStopMetric(httpStartStop *sonde_events.HttpStartStop) {
	if httpStartStop.PeerType != nil && *httpStartStop.PeerType == sonde_events.PeerType_Server {
		return
	}

	responseDuration := time.Duration(httpStartStop.GetStopTimestamp() - httpStartStop.GetStartTimestamp()).Seconds()
	index := int(httpStartStop.GetInstanceIndex())
	if index < len(w.MetricsForInstance) {
		statusRange := fmt.Sprintf("%dxx", *httpStartStop.StatusCode/100)
		w.MetricsForInstance[index].Requests.WithLabelValues(statusRange).Inc()
		w.MetricsForInstance[index].ResponseTime.WithLabelValues(statusRange).Observe(responseDuration)
	}
}

func (w *Watcher) UpdateAppInstances(newNumberOfInstances int) {
	w.app.Instances = newNumberOfInstances
	w.numberOfInstancesChan <- newNumberOfInstances
}

func (w *Watcher) Close() {
	if w.cancel == nil {
		log.Fatal("Watcher.Close() called without Start()")
	}
	w.cancel()
}

func (w *Watcher) scaleTo(newInstanceCount int) error {
	currentInstanceCount := len(w.MetricsForInstance)

	if currentInstanceCount < newInstanceCount {
		for i := currentInstanceCount; i < newInstanceCount; i++ {
			im, err := NewInstanceMetrics(i, w.registerer)
			if err != nil {
				return err
			}
			w.MetricsForInstance = append(w.MetricsForInstance, im)
		}
	} else {
		for i := currentInstanceCount; i > newInstanceCount; i-- {
			w.MetricsForInstance[i-1].unregisterInstanceMetrics()
		}
		w.MetricsForInstance = w.MetricsForInstance[0:newInstanceCount]
	}
	return nil
}
