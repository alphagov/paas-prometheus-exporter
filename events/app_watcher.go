package events

import (
	"fmt"

	"github.com/cloudfoundry-community/go-cfclient"
	sonde_events "github.com/cloudfoundry/sonde-go/events"
	"github.com/prometheus/client_golang/prometheus"
)

type AppWatcher struct {
	MetricsForInstance    []InstanceMetrics
	appName               string
	appGuid               string
	numberOfInstancesChan chan int
	registerer            prometheus.Registerer
	streamProvider        AppStreamProvider
}

type InstanceMetrics struct {
	Cpu prometheus.Gauge
}

func NewInstanceMetrics(instanceIndex int, registerer prometheus.Registerer) InstanceMetrics {
	im := InstanceMetrics{
		Cpu: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "Cpu",
				Help: " ",
				ConstLabels: prometheus.Labels{
					"instance": fmt.Sprintf("%d", instanceIndex),
				},
			},
		),
	}
	registerer.MustRegister(im.Cpu)
	return im
}

func NewAppWatcher(
	app cfclient.App,
	registerer prometheus.Registerer,
	streamProvider AppStreamProvider,
) *AppWatcher {
	appWatcher := &AppWatcher{
		MetricsForInstance:    make([]InstanceMetrics, 0),
		appName:               app.Name,
		appGuid:               app.Guid,
		registerer:            registerer,
		numberOfInstancesChan: make(chan int, 5),
		streamProvider:        streamProvider,
	}
	appWatcher.scaleTo(app.Instances)

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
		instance.Cpu.Set(metric.GetCpuPercentage())
	}
}

func (m *AppWatcher) AppName() string {
	return m.appName
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
}
