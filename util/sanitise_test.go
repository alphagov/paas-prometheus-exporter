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
		Entry("invalid chars", "invalid chars?-", "invalid_chars"),
		Entry("uppercase chars", "Uppercase_Label", "uppercase_label"),
		Entry("starts with number", "2label", "_2label"),
		Entry("multiple underscores", "multiple___underscores", "multiple_underscores"),
	)
})

var _ = Describe("SanitisePrometheusLabels", func() {

	It("should sanitise all labels", func() {
		labels := map[string]string{
			"valid_label_123":        "1",
			"invalid chars?-":        "2",
			"Uppercase_Label":        "3",
			"2label":                 "4",
			"multiple___underscores": "5",
		}

		Expect(util.SanitisePrometheusLabels(labels, nil, nil)).To(Equal(map[string]string{
			"valid_label_123":      "1",
			"invalid_chars":        "2",
			"uppercase_label":      "3",
			"_2label":              "4",
			"multiple_underscores": "5",
		}))
	})

	It("should prefix reserved labels with underscore", func() {
		labels := map[string]string{
			"valid_label_123": "1",
			"reserved_label":  "2",
		}

		reservedLabels := []string{"reserved_label"}

		Expect(util.SanitisePrometheusLabels(labels, reservedLabels, nil)).To(Equal(map[string]string{
			"valid_label_123": "1",
			"_reserved_label": "2",
		}))
	})

	It("should not include the excluded labels", func() {
		labels := map[string]string{
			"valid_label_123": "1",
			"excluded_label":  "2",
		}

		excludedLabels := []string{"excluded_label"}

		Expect(util.SanitisePrometheusLabels(labels, nil, excludedLabels)).To(Equal(map[string]string{
			"valid_label_123": "1",
		}))
	})

})
