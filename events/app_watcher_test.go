package events_test

import (
	"errors"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry-community/go-cfclient"
	sonde_events "github.com/cloudfoundry/sonde-go/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/alphagov/paas-prometheus-exporter/events"
	"github.com/alphagov/paas-prometheus-exporter/events/mocks"
)

type FakeRegistry struct {
	mustRegisterCount int
	unregisterCount int
	sync.Mutex
}

func (m *FakeRegistry) MustRegister(...prometheus.Collector) {
	m.Lock()
	defer m.Unlock()
	m.mustRegisterCount++
}

func (m *FakeRegistry) Register(prometheus.Collector) error {
	return errors.New("Not implemented")
}

func (m *FakeRegistry) Unregister(prometheus.Collector) bool {
	m.Lock()
	defer m.Unlock()
	m.unregisterCount++
	return true
}

func (m *FakeRegistry) MustRegisterCallCount() int {
	m.Lock()
	defer m.Unlock()
	return m.mustRegisterCount
}

func (m *FakeRegistry) UnregisterCallCount() int {
	m.Lock()
	defer m.Unlock()
	return m.unregisterCount
}

var _ = Describe("AppWatcher", func() {
	var (
		appWatcher *events.AppWatcher
		registerer *FakeRegistry
		streamProvider *mocks.FakeAppStreamProvider
	)

	BeforeEach(func() {
		apps := []cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123"},
		}

		registerer = &FakeRegistry{}
		streamProvider = &mocks.FakeAppStreamProvider{}
		appWatcher = events.NewAppWatcher(apps[0], registerer, streamProvider)
	})
	AfterEach(func() {})

	Describe("AppName", func() {
		It("knows the name of its application", func() {
			Expect(appWatcher.AppName()).To(Equal("foo"))
		})
	})

	Describe("Run", func() {
		It("Registers metrics on startup", func() {
			// go appWatcher.Run()
			// defer appWatcher.Close()

			Eventually(registerer.MustRegisterCallCount).Should(Equal(1))
		})

		It("Unregisters metrics on close", func() {
			go appWatcher.Run()

			appWatcher.Close()

			Eventually(registerer.UnregisterCallCount).Should(Equal(1))
		})

		It("Registers more metrics when new instances are created", func() {
			go appWatcher.Run()
			defer appWatcher.Close()

			Eventually(registerer.MustRegisterCallCount).Should(Equal(1))

			appWatcher.UpdateApp(cfclient.App{
				Guid: "33333333-3333-3333-3333-333333333333",
				Instances: 2,
				Name: "foo",
				SpaceURL: "/v2/spaces/123",
			})

			Eventually(registerer.MustRegisterCallCount).Should(Equal(2))
		})

		It("Unregisters some metrics when old instances are deleted", func() {
			go appWatcher.Run()
			defer appWatcher.Close()

			appWatcher.UpdateApp(cfclient.App{
				Guid: "33333333-3333-3333-3333-333333333333",
				Instances: 2,
				Name: "foo",
				SpaceURL: "/v2/spaces/123",
			})

			Eventually(registerer.MustRegisterCallCount).Should(Equal(2))

			appWatcher.UpdateApp(cfclient.App{
				Guid: "33333333-3333-3333-3333-333333333333",
				Instances: 1,
				Name: "foo",
				SpaceURL: "/v2/spaces/123",
			})

			Eventually(registerer.UnregisterCallCount).Should(Equal(1))
		})

		It("sets a CPU metric on an instance", func() {
			cpuPercentage := 10.0
			var instanceIndex int32 = 0
			containerMetric := sonde_events.ContainerMetric{
				CpuPercentage: &cpuPercentage,
				InstanceIndex: &instanceIndex,
			}
			messages := make(chan *sonde_events.Envelope,1)
			metricType := sonde_events.Envelope_ContainerMetric
			messages <- &sonde_events.Envelope{ContainerMetric: &containerMetric, EventType: &metricType}
			streamProvider.OpenStreamForReturns(messages, nil)

			go appWatcher.Run()
			defer appWatcher.Close()
			
			cpuGauge := appWatcher.MetricsForInstance[instanceIndex].Cpu

			Eventually(func() float64 {return testutil.ToFloat64(cpuGauge)}).Should(Equal(cpuPercentage))
		})
	})
})
