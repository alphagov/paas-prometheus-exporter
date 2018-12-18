package util_test

import (
	"github.com/alphagov/paas-prometheus-exporter/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("SanitisePrometheusName", func() {
	DescribeTable("SanitisePrometheusName",
		func(name string, expected string) {
			Expect(util.SanitisePrometheusName(name)).To(Equal(expected))
		},
		Entry("valid label", "valid_label_123", "valid_label_123"),
		Entry("invalid chars", "invalid label?-", "invalid_label"),
		Entry("uppercase chars", "Uppercase_Label", "uppercase_label"),
		Entry("starts with number", "2label", "_2label"),
		Entry("multiple underscores", "invalid___label", "invalid_label"),
	)
})
