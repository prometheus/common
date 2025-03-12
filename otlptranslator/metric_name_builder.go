// Copyright 2025 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlptranslator

import (
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// The map to translate OTLP units to Prometheus units
// OTLP metrics use the c/s notation as specified at https://ucum.org/ucum.html
// (See also https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/metrics/semantic_conventions/README.md#instrument-units)
// Prometheus best practices for units: https://prometheus.io/docs/practices/naming/#base-units
// OpenMetrics specification for units: https://github.com/prometheus/OpenMetrics/blob/v1.0.0/specification/OpenMetrics.md#units-and-base-units
var unitMap = map[string]string{
	// Time
	"d":   "days",
	"h":   "hours",
	"min": "minutes",
	"s":   "seconds",
	"ms":  "milliseconds",
	"us":  "microseconds",
	"ns":  "nanoseconds",

	// Bytes
	"By":   "bytes",
	"KiBy": "kibibytes",
	"MiBy": "mebibytes",
	"GiBy": "gibibytes",
	"TiBy": "tibibytes",
	"KBy":  "kilobytes",
	"MBy":  "megabytes",
	"GBy":  "gigabytes",
	"TBy":  "terabytes",

	// SI
	"m": "meters",
	"V": "volts",
	"A": "amperes",
	"J": "joules",
	"W": "watts",
	"g": "grams",

	// Misc
	"Cel": "celsius",
	"Hz":  "hertz",
	"1":   "",
	"%":   "percent",
}

// The map that translates the "per" unit
// Example: s => per second (singular)
var perUnitMap = map[string]string{
	"s":  "second",
	"m":  "minute",
	"h":  "hour",
	"d":  "day",
	"w":  "week",
	"mo": "month",
	"y":  "year",
}

var (
	nonMetricNameCharRE = regexp.MustCompile(`[^a-zA-Z0-9:]`)
	// Regexp for metric name characters that should be replaced with _.
	invalidMetricCharRE   = regexp.MustCompile(`[^a-zA-Z0-9:_]`)
	multipleUnderscoresRE = regexp.MustCompile(`__+`)
)

// BuildMetricName builds a valid metric name but without following Prometheus naming conventions.
// It doesn't do any character transformation, it only prefixes the metric name with the namespace, if any,
// and adds metric type suffixes, e.g. "_total" for counters and unit suffixes.
//
// Differently from BuildCompliantMetricName, it doesn't check for the presence of unit and type suffixes.
// If "addMetricSuffixes" is true, it will add them anyway.
//
// Please use BuildCompliantMetricName for a metric name that follows Prometheus naming conventions.
func BuildMetricName(name, unit string, metricType MetricType, addMetricSuffixes bool) string {
	if addMetricSuffixes {
		mainUnitSuffix, perUnitSuffix := buildUnitSuffixes(unit)
		if mainUnitSuffix != "" {
			name = name + "_" + mainUnitSuffix
		}
		if perUnitSuffix != "" {
			name = name + "_" + perUnitSuffix
		}

		// Append _total for Counters
		if metricType == MetricTypeMonotonicCounter {
			name = name + "_total"
		}

		// Append _ratio for metrics with unit "1"
		// Some OTel receivers improperly use unit "1" for counters of objects
		// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aissue+some+metric+units+don%27t+follow+otel+semantic+conventions
		// Until these issues have been fixed, we're appending `_ratio` for gauges ONLY
		// Theoretically, counters could be ratios as well, but it's absurd (for mathematical reasons)
		if unit == "1" && metricType == MetricTypeGauge {
			name = name + "_ratio"
		}
	}
	return name
}

// BuildCompliantMetricName builds a Prometheus-compliant metric name for the specified metric.
//
// Metric name is prefixed with specified namespace and underscore (if any).
// Namespace is not cleaned up. Make sure specified namespace follows Prometheus
// naming convention.
//
// See rules at https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels,
// https://prometheus.io/docs/practices/naming/#metric-and-label-naming
// and https://github.com/open-telemetry/opentelemetry-specification/blob/v1.38.0/specification/compatibility/prometheus_and_openmetrics.md#otlp-metric-points-to-prometheus.
func BuildCompliantMetricName(name, unit string, metricType MetricType, addMetricSuffixes bool) string {
	// Full normalization following standard Prometheus naming conventions
	if addMetricSuffixes {
		return normalizeName(name, unit, metricType)
	}

	// Simple case (no full normalization, no units, etc.).
	metricName := strings.Join(strings.FieldsFunc(name, func(r rune) bool {
		return invalidMetricCharRE.MatchString(string(r))
	}), "_")

	// Metric name starts with a digit? Prefix it with an underscore.
	if metricName != "" && unicode.IsDigit(rune(metricName[0])) {
		metricName = "_" + metricName
	}

	return metricName
}

// Build a normalized name for the specified metric.
func normalizeName(metric, unit string, metricType MetricType) string {
	// Split metric name into "tokens" (of supported metric name runes).
	// Note that this has the side effect of replacing multiple consecutive underscores with a single underscore.
	// This is part of the OTel to Prometheus specification: https://github.com/open-telemetry/opentelemetry-specification/blob/v1.38.0/specification/compatibility/prometheus_and_openmetrics.md#otlp-metric-points-to-prometheus.
	nameTokens := strings.FieldsFunc(
		metric,
		func(r rune) bool { return nonMetricNameCharRE.MatchString(string(r)) },
	)

	mainUnitSuffix, perUnitSuffix := buildUnitSuffixes(unit)
	nameTokens = addUnitTokens(nameTokens, CleanUpString(mainUnitSuffix), CleanUpString(perUnitSuffix))

	// Append _total for Counters
	if metricType == MetricTypeMonotonicCounter {
		nameTokens = append(removeItem(nameTokens, "total"), "total")
	}

	// Append _ratio for metrics with unit "1"
	// Some OTel receivers improperly use unit "1" for counters of objects
	// See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues?q=is%3Aissue+some+metric+units+don%27t+follow+otel+semantic+conventions
	// Until these issues have been fixed, we're appending `_ratio` for gauges ONLY
	// Theoretically, counters could be ratios as well, but it's absurd (for mathematical reasons)
	if unit == "1" && metricType == MetricTypeGauge {
		nameTokens = append(removeItem(nameTokens, "ratio"), "ratio")
	}

	// Build the string from the tokens, separated with underscores
	normalizedName := strings.Join(nameTokens, "_")

	// Metric name cannot start with a digit, so prefix it with "_" in this case
	if normalizedName != "" && unicode.IsDigit(rune(normalizedName[0])) {
		normalizedName = "_" + normalizedName
	}

	return normalizedName
}

// buildUnitSuffixes builds the main and per unit suffixes for the specified unit
// but doesn't do any special character transformation to accommodate Prometheus naming conventions.
// Removing trailing underscores or appending suffixes is done in the caller.
func buildUnitSuffixes(unit string) (mainUnitSuffix, perUnitSuffix string) {
	// Split unit at the '/' if any
	unitTokens := strings.SplitN(unit, "/", 2)

	if len(unitTokens) > 0 {
		// Main unit
		// Update if not blank and doesn't contain '{}'
		mainUnitOTel := strings.TrimSpace(unitTokens[0])
		if mainUnitOTel != "" && !strings.ContainsAny(mainUnitOTel, "{}") {
			mainUnitSuffix = unitMapGetOrDefault(mainUnitOTel)
		}

		// Per unit
		// Update if not blank and doesn't contain '{}'
		if len(unitTokens) > 1 && unitTokens[1] != "" {
			perUnitOTel := strings.TrimSpace(unitTokens[1])
			if perUnitOTel != "" && !strings.ContainsAny(perUnitOTel, "{}") {
				perUnitSuffix = perUnitMapGetOrDefault(perUnitOTel)
			}
			if perUnitSuffix != "" {
				perUnitSuffix = "per_" + perUnitSuffix
			}
		}
	}

	return mainUnitSuffix, perUnitSuffix
}

// Retrieve the Prometheus "basic" unit corresponding to the specified "basic" unit
// Returns the specified unit if not found in unitMap
func unitMapGetOrDefault(unit string) string {
	if promUnit, ok := unitMap[unit]; ok {
		return promUnit
	}
	return unit
}

// Retrieve the Prometheus "per" unit corresponding to the specified "per" unit
// Returns the specified unit if not found in perUnitMap
func perUnitMapGetOrDefault(perUnit string) string {
	if promPerUnit, ok := perUnitMap[perUnit]; ok {
		return promPerUnit
	}
	return perUnit
}

// addUnitTokens will add the suffixes to the nameTokens if they are not already present.
// It will also remove trailing underscores from the main suffix to avoid double underscores
// when joining the tokens.
//
// If the 'per' unit ends with underscore, the underscore will be removed. If the per unit is just
// 'per_', it will be entirely removed.
func addUnitTokens(nameTokens []string, mainUnitSuffix, perUnitSuffix string) []string {
	if slices.Contains(nameTokens, mainUnitSuffix) {
		mainUnitSuffix = ""
	}

	if perUnitSuffix == "per_" {
		perUnitSuffix = ""
	} else {
		perUnitSuffix = strings.TrimSuffix(perUnitSuffix, "_")
		if slices.Contains(nameTokens, perUnitSuffix) {
			perUnitSuffix = ""
		}
	}

	if perUnitSuffix != "" {
		mainUnitSuffix = strings.TrimSuffix(mainUnitSuffix, "_")
	}

	if mainUnitSuffix != "" {
		nameTokens = append(nameTokens, mainUnitSuffix)
	}
	if perUnitSuffix != "" {
		nameTokens = append(nameTokens, perUnitSuffix)
	}
	return nameTokens
}

// CleanUpString cleans up a string so it matches model.LabelNameRE.
// CleanUpString is usually used to clean up unit strings, but can be used for any string, e.g. namespaces.
func CleanUpString(s string) string {
	// Multiple consecutive underscores are replaced with a single underscore.
	// This is part of the OTel to Prometheus specification: https://github.com/open-telemetry/opentelemetry-specification/blob/v1.38.0/specification/compatibility/prometheus_and_openmetrics.md#otlp-metric-points-to-prometheus.
	return strings.TrimPrefix(multipleUnderscoresRE.ReplaceAllString(
		nonMetricNameCharRE.ReplaceAllString(s, "_"),
		"_",
	), "_")
}

// Remove the specified value from the slice
func removeItem(slice []string, value string) []string {
	newSlice := make([]string, 0, len(slice))
	for _, sliceEntry := range slice {
		if sliceEntry != value {
			newSlice = append(newSlice, sliceEntry)
		}
	}
	return newSlice
}
