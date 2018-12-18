package util

import (
	"regexp"
	"strings"
)

var beginsWithNumberRegex = regexp.MustCompile("^[0-9]+")
var invalidCharRegex = regexp.MustCompile("[^a-zA-Z0-9_:]")
var multipleUnderscoresRegexp = regexp.MustCompile("__+")

func SanitisePrometheusName(name string) string {
	name = invalidCharRegex.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	name = multipleUnderscoresRegexp.ReplaceAllString(name, "_")
	name = strings.ToLower(name)

	if beginsWithNumberRegex.MatchString(name) {
		return "_" + name
	}

	return name
}

func SanitisePrometheusLabels(labels map[string]string, reservedLabels []string) map[string]string {
	ret := make(map[string]string, len(labels))
	for name, value := range labels {
		name = SanitisePrometheusName(name)
		for _, reservedLabel := range reservedLabels {
			if reservedLabel == name {
				name = "_" + name
			}
		}

		ret[name] = value
	}
	return ret
}
