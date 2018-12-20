package service_test

import (
	"context"
	"errors"
	"time"

	"github.com/alphagov/paas-prometheus-exporter/test"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/alphagov/paas-prometheus-exporter/cf"
	cfmocks "github.com/alphagov/paas-prometheus-exporter/cf/mocks"
	"github.com/alphagov/paas-prometheus-exporter/service"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("ServiceWatcher", func() {

	var (
		serviceWatcher     *service.Watcher
		registry           *prometheus.Registry
		fakeLogCacheClient *cfmocks.FakeLogCacheClient
		ctx                context.Context
		cancel             context.CancelFunc
		stopped            chan struct{}
	)

	BeforeEach(func() {
		serviceInstance := cf.ServiceInstance{
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

		registry = prometheus.NewRegistry()
		fakeLogCacheClient = &cfmocks.FakeLogCacheClient{}
		serviceWatcher = service.NewWatcher(serviceInstance, registry, fakeLogCacheClient, 100*time.Millisecond)

		ctx, cancel = context.WithCancel(context.Background())
		stopped = make(chan struct{}, 1)
	})

	AfterEach(func() {
		cancel()
		<-stopped
	})

	var start = func() {
		go func() {
			defer GinkgoRecover()
			defer func() {
				stopped <- struct{}{}
			}()

			err := serviceWatcher.Run(ctx)
			Expect(err).ToNot(HaveOccurred())
		}()
	}

	Describe("Run", func() {
		It("registers/unregister metrics on startup and close", func() {
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(2))

			serviceWatcher.Close()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(0))
		})

		It("converts the metrics correctly", func() {
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(2))

			metricFamilies := test.GetMetricFamilies(registry)

			expectedLabels := []*dto.LabelPair{
				label("guid", "33333333-3333-3333-3333-333333333333"),
				label("organisation", "test-org"),
				label("service", "test-service"),
				label("space", "test-space"),
				label("labela", "valuea"),
				label("labelb", "valueb"),
			}

			Expect(metricFamilies).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": PointTo(Equal("test_metric_seconds")),
					"Help": PointTo(Equal("")),
					"Type": PointTo(Equal(dto.MetricType_GAUGE)),
					"Metric": ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Label": ConsistOf(expectedLabels),
							"Gauge": PointTo(MatchFields(IgnoreExtras, Fields{
								"Value": PointTo(Equal(1.1)),
							})),
							"TimestampMs": PointTo(Equal(metricTime.UnixNano() / 1000000)),
						})),
					),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Name": PointTo(Equal("test_metric_2_bytes")),
					"Help": PointTo(Equal("")),
					"Type": PointTo(Equal(dto.MetricType_GAUGE)),
					"Metric": ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Label": ConsistOf(expectedLabels),
							"Gauge": PointTo(MatchFields(IgnoreExtras, Fields{
								"Value": PointTo(Equal(2.1)),
							})),
							"TimestampMs": PointTo(Equal(metricTime.UnixNano() / 1000000)),
						})),
					),
				})),
			))
		})

		It("sanitises the metric name", func() {
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&invalidNameFixture}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(1))

			metricFamilies := test.GetMetricFamilies(registry)

			Expect(metricFamilies[0].Name).To(PointTo(Equal("invalid_name_seconds")))
		})

		It("sanitises the metric labels", func() {
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&invalidLabelsFixture}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(1))

			metricFamilies := test.GetMetricFamilies(registry)

			Expect(metricFamilies[0].Metric[0].Label).To(ContainElement(label("label_a", "valuea")))
			Expect(metricFamilies[0].Metric[0].Label).To(ContainElement(label("label_b", "valueb")))
		})

		It("should add underscore to duplicated label names", func() {
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&duplicatedLabelsFixture}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(1))

			metricFamilies := test.GetMetricFamilies(registry)

			Expect(metricFamilies[0].Metric[0].Label).To(ContainElement(label("guid", guid)))
			Expect(metricFamilies[0].Metric[0].Label).To(ContainElement(label("_guid", "other-guid")))
		})

		It("should not include any excluded labels", func() {
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&excludedLabelsFixture}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(1))

			metricFamilies := test.GetMetricFamilies(registry)

			keys := []string{}
			for _, labelPair := range metricFamilies[0].Metric[0].Label {
				keys = append(keys, *labelPair.Name)
			}

			Expect(keys).ToNot(ContainElement("deployment"))
			Expect(keys).ToNot(ContainElement("index"))
			Expect(keys).ToNot(ContainElement("ip"))
			Expect(keys).ToNot(ContainElement("job"))
			Expect(keys).ToNot(ContainElement("origin"))
		})

		It("should only add valid units as a suffic", func() {
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&invalidUnitFixture}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(1))

			metricFamilies := test.GetMetricFamilies(registry)

			Expect(*metricFamilies[0].Name).To(Equal("test_metric"))
		})

		It("registers a metric with the correct timestamp", func() {
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&gaugeFixture}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(2))

			Expect(test.GetMetrics(registry)[0].GetTimestampMs()).To(Equal(metricTime.UnixNano() / 1000000))
			Expect(test.GetMetrics(registry)[1].GetTimestampMs()).To(Equal(metricTime.UnixNano() / 1000000))
		})

		It("only records the latest value for the same metric", func() {
			fakeLogCacheClient.ReadReturnsOnCall(0, []*loggregator_v2.Envelope{&gaugeFixture}, nil)

			olderMetric := gaugeFixture
			olderMetric.Timestamp = metricTime.Add(-5 * time.Minute).UnixNano()
			fakeLogCacheClient.ReadReturns([]*loggregator_v2.Envelope{&olderMetric}, nil)

			start()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(2))

			Consistently(
				func() []int64 {
					return []int64{
						test.GetMetrics(registry)[0].GetTimestampMs(),
						test.GetMetrics(registry)[1].GetTimestampMs(),
					}
				},
			).Should(Equal([]int64{
				metricTime.UnixNano() / 1000000,
				metricTime.UnixNano() / 1000000,
			}))
		})

		It("handles errors from log-cache and unregisters all metrics", func() {
			fakeLogCacheClient.ReadReturnsOnCall(0, []*loggregator_v2.Envelope{&gaugeFixture}, nil)

			err := errors.New("some error")
			fakeLogCacheClient.ReadReturns(nil, err)

			go func() {
				defer GinkgoRecover()
				defer func() {
					stopped <- struct{}{}
				}()

				err := serviceWatcher.Run(ctx)
				Expect(err).To(MatchError(err))
			}()

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(2))

			Eventually(func() int { return len(test.GetMetrics(registry)) }).Should(Equal(0))
		})

	})

})

func label(name string, value string) *dto.LabelPair {
	return &dto.LabelPair{
		Name:  &name,
		Value: &value,
	}
}
