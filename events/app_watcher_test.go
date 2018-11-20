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
	const METRICS_PER_INSTANCE = 5

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

			Eventually(registerer.MustRegisterCallCount).Should(Equal(METRICS_PER_INSTANCE))
		})

		It("Unregisters metrics on close", func() {
			appWatcher.Close()

			Eventually(registerer.UnregisterCallCount).Should(Equal(METRICS_PER_INSTANCE))
		})

		It("Registers more metrics when new instances are created", func() {
			defer appWatcher.Close()

			Eventually(registerer.MustRegisterCallCount).Should(Equal(METRICS_PER_INSTANCE))

			appWatcher.UpdateAppInstances(2)

			Eventually(registerer.MustRegisterCallCount).Should(Equal(2 * METRICS_PER_INSTANCE))
		})

		It("Unregisters some metrics when old instances are deleted", func() {
			defer appWatcher.Close()

			appWatcher.UpdateAppInstances(2)

			Eventually(registerer.MustRegisterCallCount).Should(Equal(2 * METRICS_PER_INSTANCE))

			appWatcher.UpdateAppInstances(1)

			Eventually(registerer.UnregisterCallCount).Should(Equal(METRICS_PER_INSTANCE))
		})

		It("sets container metrics for an instance", func() {
			defer appWatcher.Close()

			var instanceIndex int32 = 0

			cpuPercentage := 10.0
			var diskBytes uint64 = 512
			var diskBytesQuota uint64 = 1024
			var memoryBytes uint64 = 1024
			var memoryBytesQuota uint64 = 4096

			containerMetric := sonde_events.ContainerMetric{
				CpuPercentage:    &cpuPercentage,
				DiskBytes:        &diskBytes,
				DiskBytesQuota:   &diskBytesQuota,
				InstanceIndex:    &instanceIndex,
				MemoryBytes:      &memoryBytes,
				MemoryBytesQuota: &memoryBytesQuota,
			}

			messages := make(chan *sonde_events.Envelope, 1)
			metricType := sonde_events.Envelope_ContainerMetric
			messages <- &sonde_events.Envelope{ContainerMetric: &containerMetric, EventType: &metricType}
			streamProvider.OpenStreamForReturns(messages, nil)

			cpuGauge := appWatcher.MetricsForInstance[instanceIndex].Cpu
			diskBytesGauge := appWatcher.MetricsForInstance[instanceIndex].DiskBytes
			diskUtilizationGauge := appWatcher.MetricsForInstance[instanceIndex].DiskUtilization
			memoryBytesGauge := appWatcher.MetricsForInstance[instanceIndex].MemoryBytes
			memoryUtilizationGauge := appWatcher.MetricsForInstance[instanceIndex].MemoryUtilization

			Eventually(func() float64 { return testutil.ToFloat64(cpuGauge) }).Should(Equal(cpuPercentage))
			Eventually(func() float64 { return testutil.ToFloat64(diskBytesGauge) }).Should(Equal(float64(diskBytes)))
			Eventually(func() float64 { return testutil.ToFloat64(memoryBytesGauge) }).Should(Equal(float64(memoryBytes)))

			// diskUtilization is a rounded percentage based on diskBytes/diskBytesQuota*100 (512/1024*100 = 50)
			Eventually(func() float64 { return testutil.ToFloat64(diskUtilizationGauge) }).Should(Equal(float64(50)))

			// diskUtilization is a rounded percentage based on memoryBytes/memoryBytesQuota*100 (1024/4096*100 = 25)
			Eventually(func() float64 { return testutil.ToFloat64(memoryUtilizationGauge) }).Should(Equal(float64(25)))
		})
	})
})
