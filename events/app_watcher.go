package events

import (
	"fmt"

	sonde_events "github.com/cloudfoundry/sonde-go/events"
	"github.com/prometheus/client_golang/prometheus"
)

type AppWatcher struct {
	appGuid               string
	MetricsForInstance    []InstanceMetrics
	numberOfInstancesChan chan int
	registerer            prometheus.Registerer
	streamProvider        AppStreamProvider
}

type InstanceMetrics struct {
	Cpu               prometheus.Gauge
	DiskBytes         prometheus.Gauge
	DiskUtilization   prometheus.Gauge
	MemoryBytes       prometheus.Gauge
	MemoryUtilization prometheus.Gauge
}

func NewInstanceMetrics(instanceIndex int, registerer prometheus.Registerer) InstanceMetrics {
	constLabels := prometheus.Labels{
		"instance": fmt.Sprintf("%d", instanceIndex),
	}

	im := InstanceMetrics{
		Cpu: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "cpu",
				Help: "CPU utilisation in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
		DiskBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "diskBytes",
				Help: "Disk usage in bytes",
				ConstLabels: constLabels,
			},
		),
		DiskUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "diskUtilization",
				Help: "Disk utilisation in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
		MemoryBytes: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "memoryBytes",
				Help: "Memory usage in bytes",
				ConstLabels: constLabels,
			},
		),
		MemoryUtilization: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "memoryUtilization",
				Help: "Memory utilisation in percent (0-100)",
				ConstLabels: constLabels,
			},
		),
	}

	registerer.MustRegister(im.Cpu)
	registerer.MustRegister(im.DiskBytes)
	registerer.MustRegister(im.DiskUtilization)
	registerer.MustRegister(im.MemoryBytes)
	registerer.MustRegister(im.MemoryUtilization)
	return im
}

func NewAppWatcher(
	appGuid string,
	appInstances int,
	registerer prometheus.Registerer,
	streamProvider AppStreamProvider,
) *AppWatcher {
	appWatcher := &AppWatcher{
		appGuid:               appGuid,
		MetricsForInstance:    make([]InstanceMetrics, 0),
		numberOfInstancesChan: make(chan int, 5),
		registerer:            registerer,
		streamProvider:        streamProvider,
	}
	appWatcher.scaleTo(appInstances)

	// FIXME: what if the appWatcher errors? we currently ignore it
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
			case sonde_events.Envelope_ContainerMetric:
				m.processContainerMetric(message.GetContainerMetric())
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
			m.unregisterInstanceMetrics(i - 1)
		}
		m.MetricsForInstance = m.MetricsForInstance[0:newInstanceCount]
	}
}

func (m *AppWatcher) unregisterInstanceMetrics(instanceIndex int) {
	m.registerer.Unregister(m.MetricsForInstance[instanceIndex].Cpu)
	m.registerer.Unregister(m.MetricsForInstance[instanceIndex].DiskBytes)
	m.registerer.Unregister(m.MetricsForInstance[instanceIndex].DiskUtilization)
	m.registerer.Unregister(m.MetricsForInstance[instanceIndex].MemoryBytes)
	m.registerer.Unregister(m.MetricsForInstance[instanceIndex].MemoryUtilization)
}
