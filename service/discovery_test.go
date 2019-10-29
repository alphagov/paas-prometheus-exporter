package service_test

import (
	"context"
	"errors"
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/alphagov/paas-prometheus-exporter/cf"
	"github.com/alphagov/paas-prometheus-exporter/test"

	cfmocks "github.com/alphagov/paas-prometheus-exporter/cf/mocks"
	"github.com/alphagov/paas-prometheus-exporter/service"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const guid = "33333333-3333-3333-3333-333333333333"

var serviceFixture = cf.ServiceInstance{
	ServiceInstance: cfclient.ServiceInstance{
		Guid: "33333333-3333-3333-3333-333333333333",
		Name: "test-service",
	},
	SpaceData: cfclient.SpaceResource{
		Entity: cfclient.Space{
			Name: "test-space",
			OrgData: cfclient.OrgResource{
				Entity: cfclient.Org{
					Name: "test-org",
				},
			},
		},
	},
}

var _ = Describe("Service discovery", func() {

	var discovery *service.Discovery
	var fakeClient *cfmocks.FakeClient
	var ctx context.Context
	var cancel context.CancelFunc
	var registry *prometheus.Registry
	var errChan chan error
	var fakeLogCacheClient *cfmocks.FakeLogCacheClient

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())

		fakeClient = &cfmocks.FakeClient{}
		fakeLogCacheClient = &cfmocks.FakeLogCacheClient{}
		fakeClient.NewLogCacheClientReturns(fakeLogCacheClient)
		registry = prometheus.NewRegistry()
		discovery = service.NewDiscovery(fakeClient, registry, 100*time.Millisecond, 100*time.Millisecond)
		errChan = make(chan error, 1)
	})

	AfterEach(func() {
		cancel()
	})

	It("checks for new services regularly", func() {
		discovery.Start(ctx, errChan)

		Eventually(fakeClient.ListServicesWithSpaceAndOrgCallCount).Should(Equal(2))
	})

	It("returns an error if it fails to list the services", func() {
		err := errors.New("some error")
		fakeClient.ListServicesWithSpaceAndOrgReturns(nil, err)

		discovery.Start(ctx, errChan)

		Eventually(errChan).Should(Receive(MatchError("failed to list the services: some error")))

		Consistently(fakeClient.ListServicesWithSpaceAndOrgCallCount, 200*time.Millisecond).Should(Equal(1))
	})

	It("creates a new service", func() {
		fakeClient.ListServicesWithSpaceAndOrgReturns([]cf.ServiceInstance{serviceFixture}, nil)
		fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

		discovery.Start(ctx, errChan)

		Eventually(fakeClient.NewLogCacheClientCallCount).Should(Equal(1))

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid": guid,
			})
		}).ShouldNot(BeNil())
	})

	It("deletes a service watcher when a service is deleted", func() {
		fakeClient.ListServicesWithSpaceAndOrgReturnsOnCall(0, []cf.ServiceInstance{serviceFixture}, nil)
		fakeClient.ListServicesWithSpaceAndOrgReturns(nil, nil)
		fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

		discovery.Start(ctx, errChan)

		Eventually(fakeClient.NewLogCacheClientCallCount).Should(Equal(1))
		Eventually(func() []*dto.Metric { return test.GetMetrics(registry) }).ShouldNot(BeEmpty())

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid": guid,
			})
		}).Should(BeNil())
	})

	It("deletes and recreates a service watcher when a service is renamed", func() {
		service1 := serviceFixture
		service1.ServiceInstance.Name = "foo"
		fakeClient.ListServicesWithSpaceAndOrgReturnsOnCall(0, []cf.ServiceInstance{service1}, nil)

		service2 := serviceFixture
		service2.ServiceInstance.Name = "bar"
		fakeClient.ListServicesWithSpaceAndOrgReturns([]cf.ServiceInstance{service2}, nil)

		fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

		discovery.Start(ctx, errChan)

		Eventually(fakeClient.NewLogCacheClientCallCount).Should(Equal(2))

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid":    guid,
				"service": "bar",
			})
		}).ShouldNot(BeNil())

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid":    guid,
				"service": "foo",
			})
		}).Should(BeNil())
	})

	It("deletes and recreates a service watcher when a space is renamed", func() {
		service1 := serviceFixture
		service1.SpaceData.Entity.Name = "spacename"
		fakeClient.ListServicesWithSpaceAndOrgReturnsOnCall(0, []cf.ServiceInstance{service1}, nil)

		service2 := serviceFixture
		service2.SpaceData.Entity.Name = "spacenamenew"
		fakeClient.ListServicesWithSpaceAndOrgReturns([]cf.ServiceInstance{service2}, nil)

		fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

		discovery.Start(ctx, errChan)

		Eventually(fakeClient.NewLogCacheClientCallCount).Should(Equal(2))

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid":  guid,
				"space": "spacenamenew",
			})
		}).ShouldNot(BeNil())

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid":  guid,
				"space": "spacename",
			})
		}).Should(BeNil())
	})

	It("deletes and recreates a service watcher when an org is renamed", func() {
		service1 := serviceFixture
		service1.SpaceData.Entity.OrgData.Entity.Name = "orgname"
		fakeClient.ListServicesWithSpaceAndOrgReturnsOnCall(0, []cf.ServiceInstance{service1}, nil)

		service2 := serviceFixture
		service2.SpaceData.Entity.OrgData.Entity.Name = "orgnamenew"
		fakeClient.ListServicesWithSpaceAndOrgReturns([]cf.ServiceInstance{service2}, nil)

		fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

		discovery.Start(ctx, errChan)

		Eventually(fakeClient.NewLogCacheClientCallCount).Should(Equal(2))

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid":         guid,
				"organisation": "orgnamenew",
			})
		}).ShouldNot(BeNil())

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid":         guid,
				"organisation": "orgname",
			})
		}).Should(BeNil())
	})

	It("recreates a service watcher when it has an error", func() {
		fakeClient.ListServicesWithSpaceAndOrgReturns([]cf.ServiceInstance{serviceFixture}, nil)
		fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

		// Reading from log-cache has retry logic, with max retries set to 3.
		// Only when all 3 retries have failed will it enter the error state.
		fakeLogCacheClient.ReadReturnsOnCall(1, nil, errors.New("some error"))
		fakeLogCacheClient.ReadReturnsOnCall(2, nil, errors.New("some error"))
		fakeLogCacheClient.ReadReturnsOnCall(3, nil, errors.New("some error"))

		discovery.Start(ctx, errChan)

		Eventually(fakeClient.NewLogCacheClientCallCount, 10 * time.Second).Should(Equal(2))

		Eventually(func() *dto.Metric {
			return test.FindMetric(registry, map[string]string{
				"guid": guid,
			})
		}).ShouldNot(BeNil())
	})

})
