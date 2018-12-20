package service_test

import (
	"time"

	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
)

var metricTime = time.Now().Add(-5 * time.Minute)

var gaugeFixture = loggregator_v2.Envelope{
	SourceId: guid,
	Tags: map[string]string{
		"labela": "valuea",
		"labelb": "valueb",
	},
	Message: &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				"test_metric": &loggregator_v2.GaugeValue{
					Unit:  "seconds",
					Value: 1.1,
				},
				"test_metric_2": &loggregator_v2.GaugeValue{
					Unit:  "bytes",
					Value: 2.1,
				},
			},
		},
	},
	Timestamp: metricTime.UnixNano(),
}

var invalidNameFixture = loggregator_v2.Envelope{
	SourceId: guid,
	Message: &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				"invalid-name": &loggregator_v2.GaugeValue{
					Unit:  "seconds",
					Value: 1.1,
				},
			},
		},
	},
	Timestamp: metricTime.UnixNano(),
}

var invalidLabelsFixture = loggregator_v2.Envelope{
	SourceId: guid,
	Tags: map[string]string{
		"label-a": "valuea",
		"label-b": "valueb",
	},
	Message: &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				"test_metric": &loggregator_v2.GaugeValue{
					Unit:  "seconds",
					Value: 1.1,
				},
			},
		},
	},
	Timestamp: metricTime.UnixNano(),
}

var duplicatedLabelsFixture = loggregator_v2.Envelope{
	SourceId: guid,
	Tags: map[string]string{
		"guid": "other-guid",
	},
	Message: &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				"test_metric": &loggregator_v2.GaugeValue{
					Unit:  "seconds",
					Value: 1.1,
				},
			},
		},
	},
	Timestamp: metricTime.UnixNano(),
}

var excludedLabelsFixture = loggregator_v2.Envelope{
	SourceId: guid,
	Tags: map[string]string{
		"deployment": "test-deployment",
		"index":      "01234567-0123-0123-0123-01234567890a",
		"ip":         "1.2.3.4",
		"job":        "test_job",
		"origin":     "test-origin",
		"source":     "test-source",
	},
	Message: &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				"test_metric": &loggregator_v2.GaugeValue{
					Unit:  "seconds",
					Value: 1.1,
				},
			},
		},
	},
	Timestamp: metricTime.UnixNano(),
}

var invalidUnitFixture = loggregator_v2.Envelope{
	SourceId: guid,
	Message: &loggregator_v2.Envelope_Gauge{
		Gauge: &loggregator_v2.Gauge{
			Metrics: map[string]*loggregator_v2.GaugeValue{
				"test_metric": &loggregator_v2.GaugeValue{
					Unit:  "unknown unit",
					Value: 1.1,
				},
			},
		},
	},
	Timestamp: metricTime.UnixNano(),
}
