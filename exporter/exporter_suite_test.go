package exporter_test

import (
	"testing"
	"log"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestExporter(t *testing.T) {
	log.SetOutput(GinkgoWriter)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Exporter Suite")
}
