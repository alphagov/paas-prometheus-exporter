package test

import (
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

func GetMetrics(registry *prometheus.Registry) []*dto.Metric {
	metrics := make([]*dto.Metric, 0)
	metricsFamilies, _ := registry.Gather()
	for _, metricsFamily := range metricsFamilies {
		metrics = append(metrics, metricsFamily.Metric...)
	}
	return metrics
}

func GetMetricFamilies(registry *prometheus.Registry) []*dto.MetricFamily {
	metricsFamilies, _ := registry.Gather()
	return metricsFamilies
}

func MetricHasLabels(metric *dto.Metric, labels map[string]string) bool {
	actualLabels := make(map[string]string)
	for _, pair := range metric.Label {
		actualLabels[*pair.Name] = *pair.Value
	}

	for k, v := range labels {
		if actualValue, ok := actualLabels[k]; !ok || actualValue != v {
			return false
		}
	}

	return true
}

func FindMetric(registry *prometheus.Registry, labels map[string]string) *dto.Metric {
	for _, metric := range GetMetrics(registry) {
		if MetricHasLabels(metric, labels) {
			return metric
		}
	}

	return nil
}
