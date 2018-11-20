package events_test

import (
	"errors"
	"sync"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/alphagov/paas-prometheus-exporter/events"
	"github.com/alphagov/paas-prometheus-exporter/events/mocks"
	"github.com/cloudfoundry-community/go-cfclient"
	sonde_events "github.com/cloudfoundry/sonde-go/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

type FakeRegistry struct {
	mustRegisterCount int
	unregisterCount   int
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
	const METRICS_PER_INSTANCE = 4

	var (
		appWatcher     *events.AppWatcher
		registerer     *FakeRegistry
		streamProvider *mocks.FakeAppStreamProvider
	)

	BeforeEach(func() {
		apps := []cfclient.App{
			{Guid: "33333333-3333-3333-3333-333333333333", Instances: 1, Name: "foo", SpaceURL: "/v2/spaces/123"},
		}

		registerer = &FakeRegistry{}
		streamProvider = &mocks.FakeAppStreamProvider{}
		appWatcher = events.NewAppWatcher(apps[0].Guid, apps[0].Instances, registerer, streamProvider)
	})
	AfterEach(func() {})

	Describe("Run", func() {
		It("Registers metrics on startup", func() {
			defer appWatcher.Close()

			Eventually(registerer.MustRegisterCallCount).Should(Equal(1 * METRICS_PER_INSTANCE))
		})

		It("Unregisters metrics on close", func() {
			appWatcher.Close()

			Eventually(registerer.UnregisterCallCount).Should(Equal(1 * METRICS_PER_INSTANCE))
		})

		It("Registers more metrics when new instances are created", func() {
			defer appWatcher.Close()

			Eventually(registerer.MustRegisterCallCount).Should(Equal(1 * METRICS_PER_INSTANCE))

			appWatcher.UpdateAppInstances(2)

			Eventually(registerer.MustRegisterCallCount).Should(Equal(2 * METRICS_PER_INSTANCE))
		})

		It("Unregisters some metrics when old instances are deleted", func() {
			defer appWatcher.Close()

			appWatcher.UpdateAppInstances(2)

			Eventually(registerer.MustRegisterCallCount).Should(Equal(2 * METRICS_PER_INSTANCE))

			appWatcher.UpdateAppInstances(1)

			Eventually(registerer.UnregisterCallCount).Should(Equal(1 * METRICS_PER_INSTANCE))
		})

		It("sets a CPU metric on an instance", func() {
			cpuPercentage := 10.0
			var instanceIndex int32 = 0
			containerMetric := sonde_events.ContainerMetric{
				CpuPercentage: &cpuPercentage,
				InstanceIndex: &instanceIndex,
			}
			messages := make(chan *sonde_events.Envelope, 1)
			metricType := sonde_events.Envelope_ContainerMetric
			messages <- &sonde_events.Envelope{ContainerMetric: &containerMetric, EventType: &metricType}
			streamProvider.OpenStreamForReturns(messages, nil)

			defer appWatcher.Close()

			cpuGauge := appWatcher.MetricsForInstance[instanceIndex].Cpu

			Eventually(func() float64 { return testutil.ToFloat64(cpuGauge) }).Should(Equal(cpuPercentage))
		})

		It("sets a diskBytes metric on an instance", func() {
			var diskBytes uint64 = 2300
			var instanceIndex int32 = 0
			containerMetric := sonde_events.ContainerMetric{
				DiskBytes:     &diskBytes,
				InstanceIndex: &instanceIndex,
			}
			messages := make(chan *sonde_events.Envelope, 1)
			metricType := sonde_events.Envelope_ContainerMetric
			messages <- &sonde_events.Envelope{ContainerMetric: &containerMetric, EventType: &metricType}
			streamProvider.OpenStreamForReturns(messages, nil)

			defer appWatcher.Close()

			diskBytesGauge := appWatcher.MetricsForInstance[instanceIndex].DiskBytes

			Eventually(func() float64 { return testutil.ToFloat64(diskBytesGauge) }).Should(Equal(float64(diskBytes)))
		})

		It("sets a memoryBytes metric on an instance", func() {
			var memoryBytes uint64 = 2301
			var instanceIndex int32 = 0
			containerMetric := sonde_events.ContainerMetric{
				DiskBytes:     &memoryBytes,
				InstanceIndex: &instanceIndex,
			}
			messages := make(chan *sonde_events.Envelope, 1)
			metricType := sonde_events.Envelope_ContainerMetric
			messages <- &sonde_events.Envelope{ContainerMetric: &containerMetric, EventType: &metricType}
			streamProvider.OpenStreamForReturns(messages, nil)

			defer appWatcher.Close()

			memoryBytesGauge := appWatcher.MetricsForInstance[instanceIndex].DiskBytes

			Eventually(func() float64 { return testutil.ToFloat64(memoryBytesGauge) }).Should(Equal(float64(memoryBytes)))
		})

		It("sets a diskUtilization metric on an instance", func() {
			var diskBytesQuota uint64 = 1024
			var diskBytes uint64 = 512
			var instanceIndex int32 = 0
			containerMetric := sonde_events.ContainerMetric{
				DiskBytes:      &diskBytes,
				DiskBytesQuota: &diskBytesQuota,
				InstanceIndex:  &instanceIndex,
			}
			messages := make(chan *sonde_events.Envelope, 1)
			metricType := sonde_events.Envelope_ContainerMetric
			messages <- &sonde_events.Envelope{ContainerMetric: &containerMetric, EventType: &metricType}
			streamProvider.OpenStreamForReturns(messages, nil)

			defer appWatcher.Close()

			diskUtilizationGauge := appWatcher.MetricsForInstance[instanceIndex].DiskUtilization

			Eventually(func() float64 { return testutil.ToFloat64(diskUtilizationGauge) }).Should(Equal(float64(50)))
		})
	})
})
