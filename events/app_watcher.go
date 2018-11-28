package events

import (
	"fmt"
	"bytes"
	"encoding/json"
	"time"

	sonde_events "github.com/cloudfoundry/sonde-go/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/cloudfoundry-community/go-cfclient"
)

type AppWatcher struct {
	appGuid               string
	MetricsForInstance    []InstanceMetrics
	numberOfInstancesChan chan int
	registerer            prometheus.Registerer
	streamProvider        AppStreamProvider
}

type InstanceMetrics struct {
	Registerer        prometheus.Registerer
	Cpu               prometheus.Gauge
	Crashes           prometheus.Counter
	DiskBytes         prometheus.Gauge
	DiskUtilization   prometheus.Gauge
	MemoryBytes       prometheus.Gauge
	MemoryUtilization prometheus.Gauge
	Requests          *prometheus.CounterVec
	ResponseTime      *prometheus.HistogramVec
}

func NewInstanceMetrics(instanceIndex int, registerer prometheus.Registerer) InstanceMetrics {
	constLabels := prometheus.Labels{
		"instance": fmt.Sprintf("%d", instanceIndex),
	}

	im := InstanceMetrics{
		Registerer: registerer,
		Cpu: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "cpu",
				Help: "CPU utilisation in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
		Crashes: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "crashes",
				Help: "Number of app instance crashes",
				ConstLabels: constLabels,
			},
		),
		DiskBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "disk_bytes",
				Help: "Disk usage in bytes",
				ConstLabels: constLabels,
			},
		),
		DiskUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "disk_utilization",
				Help: "Disk space currently in use in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
		MemoryBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "memory_bytes",
				Help: "Memory usage in bytes",
				ConstLabels: constLabels,
			},
		),
		MemoryUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "memory_utilization",
				Help: "Memory currently in use in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
		Requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "requests",
				Help: "Counter of http requests for a given app instance",
				ConstLabels: constLabels,
			},
			[]string{"status_range"},
		),
		ResponseTime: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "response_time",
				Help: "Histogram of http request time for a given app instance",
				ConstLabels: constLabels,
			},
			[]string{"status_range"},
		),
	}

	// This initalises with a zero value for requests and responseTime for each
	// statusRange
	for _, statusRange := range []string{"2xx", "3xx", "4xx", "5xx"} {
		im.Requests.GetMetricWithLabelValues(statusRange)
		im.ResponseTime.GetMetricWithLabelValues(statusRange)
	}

	im.registerInstanceMetrics()

	return im
}

func (im *InstanceMetrics) registerInstanceMetrics() {
	im.Registerer.MustRegister(im.Cpu)
	im.Registerer.MustRegister(im.Crashes)
	im.Registerer.MustRegister(im.DiskBytes)
	im.Registerer.MustRegister(im.DiskUtilization)
	im.Registerer.MustRegister(im.MemoryBytes)
	im.Registerer.MustRegister(im.MemoryUtilization)
	im.Registerer.MustRegister(im.Requests)
	im.Registerer.MustRegister(im.ResponseTime)
}

func (im *InstanceMetrics) unregisterInstanceMetrics() {
	im.Registerer.Unregister(im.Cpu)
	im.Registerer.Unregister(im.Crashes)
	im.Registerer.Unregister(im.DiskBytes)
	im.Registerer.Unregister(im.DiskUtilization)
	im.Registerer.Unregister(im.MemoryBytes)
	im.Registerer.Unregister(im.MemoryUtilization)
	im.Registerer.Unregister(im.Requests)
	im.Registerer.Unregister(im.ResponseTime)
}

func NewAppWatcher(
	app cfclient.App,
	registerer prometheus.Registerer,
	streamProvider AppStreamProvider,
) *AppWatcher {
	appWatcher := &AppWatcher{
		appGuid:               app.Guid,
		MetricsForInstance:    make([]InstanceMetrics, 0),
		numberOfInstancesChan: make(chan int, 5),
		registerer:            registerer,
		streamProvider:        streamProvider,
	}
	appWatcher.scaleTo(app.Instances)

	// TODO: what if the appWatcher errors? we currently ignore it
	go appWatcher.Run()
	return appWatcher
}

func (m *AppWatcher) Run() error {
	msgs, errs := m.streamProvider.OpenStreamFor(m.appGuid)
	defer m.streamProvider.Close()

	err := m.mainLoop(msgs, errs)
	return err
}

func (m *AppWatcher) mainLoop(msgs <-chan *sonde_events.Envelope, errs <-chan error) error {
	for {
		select {
		case message, ok := <-msgs:
			if !ok {
				// delete all instances
				m.Close()
				msgs = nil
				continue
			}
			switch message.GetEventType() {
			case sonde_events.Envelope_LogMessage:
				err := m.processLogMessage(message.GetLogMessage())
				if err != nil {
					return err
				}
			case sonde_events.Envelope_ContainerMetric:
				m.processContainerMetric(message.GetContainerMetric())
			case sonde_events.Envelope_HttpStartStop:
				m.processHttpStartStopMetric(message.GetHttpStartStop())
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if err == nil {
				continue
			}
			return err
		case newNumberOfInstances, ok := <-m.numberOfInstancesChan:
			if !ok {
				m.scaleTo(0)
				return nil
			}

			m.scaleTo(newNumberOfInstances)
		}
	}
}

func (m *AppWatcher) processContainerMetric(metric *sonde_events.ContainerMetric) {
	index := metric.GetInstanceIndex()
	if int(index) < len(m.MetricsForInstance) {
		instance := m.MetricsForInstance[index]

		diskUtilizationPercentage := float64(metric.GetDiskBytes()) / float64(metric.GetDiskBytesQuota()) * 100
		memoryUtilizationPercentage := float64(metric.GetMemoryBytes()) / float64(metric.GetMemoryBytesQuota()) * 100

		instance.Cpu.Set(metric.GetCpuPercentage())
		instance.DiskBytes.Set(float64(metric.GetDiskBytes()))
		instance.DiskUtilization.Set(diskUtilizationPercentage)
		instance.MemoryBytes.Set(float64(metric.GetMemoryBytes()))
		instance.MemoryUtilization.Set(memoryUtilizationPercentage)
	}
}

func (m *AppWatcher) processLogMessage(logMessage *sonde_events.LogMessage) error {
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
	if index < len(m.MetricsForInstance) {
		m.MetricsForInstance[index].Crashes.Inc()
	}
	return nil
}

func (m *AppWatcher) processHttpStartStopMetric(httpStartStop *sonde_events.HttpStartStop) {
	responseDuration := float64(time.Duration(httpStartStop.GetStopTimestamp() - httpStartStop.GetStartTimestamp())) / float64(time.Second)
	index := int(httpStartStop.GetInstanceIndex())
	if index < len(m.MetricsForInstance) {
		statusRange := fmt.Sprintf("%dxx", *httpStartStop.StatusCode / 100)
		m.MetricsForInstance[index].Requests.WithLabelValues(statusRange).Inc()
		m.MetricsForInstance[index].ResponseTime.WithLabelValues(statusRange).Observe(responseDuration)
	}
}

func (m *AppWatcher) UpdateAppInstances(newNumberOfInstances int) {
	m.numberOfInstancesChan <- newNumberOfInstances
}

func (m *AppWatcher) Close() {
	close(m.numberOfInstancesChan)
}

func (m *AppWatcher) scaleTo(newInstanceCount int) {
	currentInstanceCount := len(m.MetricsForInstance)

	if currentInstanceCount < newInstanceCount {
		for i := currentInstanceCount; i < newInstanceCount; i++ {
			m.MetricsForInstance = append(m.MetricsForInstance, NewInstanceMetrics(i, m.registerer))
		}
	} else {
		for i := currentInstanceCount; i > newInstanceCount; i-- {
			m.MetricsForInstance[i - 1].unregisterInstanceMetrics()
		}
		m.MetricsForInstance = m.MetricsForInstance[0:newInstanceCount]
	}
}
