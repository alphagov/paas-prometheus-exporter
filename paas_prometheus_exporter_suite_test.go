package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestPaasPrometheusExporter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PaasPrometheusExporter Suite")
}
