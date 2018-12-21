package test

import "github.com/prometheus/client_golang/prometheus"

//go:generate counterfeiter -o mocks/registerer.go . Registerer
type Registerer interface {
	prometheus.Registerer
}
