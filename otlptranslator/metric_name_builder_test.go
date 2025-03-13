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
	"reflect"
	"testing"
)

func TestBuildMetricName(t *testing.T) {
	tests := []struct {
		name              string
		metricName        string
		unit              string
		metricType        MetricType
		addMetricSuffixes bool
		expected          string
	}{
		{
			name:              "simple metric without suffixes",
			metricName:        "http_requests",
			unit:              "",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: false,
			expected:          "http_requests",
		},
		{
			name:              "counter with total suffix",
			metricName:        "http_requests",
			unit:              "",
			metricType:        MetricTypeMonotonicCounter,
			addMetricSuffixes: true,
			expected:          "http_requests_total",
		},
		{
			name:              "gauge with time unit",
			metricName:        "request_duration",
			unit:              "s",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "request_duration_seconds",
		},
		{
			name:              "counter with time unit",
			metricName:        "request_duration",
			unit:              "ms",
			metricType:        MetricTypeMonotonicCounter,
			addMetricSuffixes: true,
			expected:          "request_duration_milliseconds_total",
		},
		{
			name:              "gauge with compound unit",
			metricName:        "throughput",
			unit:              "By/s",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "throughput_bytes_per_second",
		},
		{
			name:              "ratio metric",
			metricName:        "cpu_utilization",
			unit:              "1",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "cpu_utilization_ratio",
		},
		{
			name:              "counter with unit 1 (no ratio suffix)",
			metricName:        "error_count",
			unit:              "1",
			metricType:        MetricTypeMonotonicCounter,
			addMetricSuffixes: true,
			expected:          "error_count_total",
		},
		{
			name:              "metric with byte units",
			metricName:        "memory_usage",
			unit:              "MiBy",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "memory_usage_mebibytes",
		},
		{
			name:              "metric with SI units",
			metricName:        "temperature",
			unit:              "Cel",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "temperature_celsius",
		},
		{
			name:              "metric with dots",
			metricName:        "system.cpu.usage",
			unit:              "1",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "system.cpu.usage_ratio",
		},
		{
			name:              "metric with japanese characters (memory usage rate)",
			metricName:        "メモリ使用率", // memori shiyouritsu (memory usage rate) xD
			unit:              "By",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "メモリ使用率_bytes",
		},
		{
			name:              "metric with mixed special characters (system.memory.usage.rate)",
			metricName:        "system.メモリ.usage.率", // system.memory.usage.rate
			unit:              "By/s",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "system.メモリ.usage.率_bytes_per_second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildMetricName(tt.metricName, tt.unit, tt.metricType, tt.addMetricSuffixes)
			if tt.expected != result {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestBuildUnitSuffixes(t *testing.T) {
	tests := []struct {
		name            string
		unit            string
		expectedMain    string
		expectedPerUnit string
	}{
		{
			name:            "empty unit",
			unit:            "",
			expectedMain:    "",
			expectedPerUnit: "",
		},
		{
			name:            "simple time unit",
			unit:            "s",
			expectedMain:    "seconds",
			expectedPerUnit: "",
		},
		{
			name:            "compound unit",
			unit:            "By/s",
			expectedMain:    "bytes",
			expectedPerUnit: "per_second",
		},
		{
			name:            "complex compound unit",
			unit:            "KiBy/m",
			expectedMain:    "kibibytes",
			expectedPerUnit: "per_minute",
		},
		{
			name:            "unit with spaces",
			unit:            " ms / s ",
			expectedMain:    "milliseconds",
			expectedPerUnit: "per_second",
		},
		{
			name:            "invalid unit",
			unit:            "invalid",
			expectedMain:    "invalid",
			expectedPerUnit: "",
		},
		{
			name:            "unit with curly braces",
			unit:            "{custom}/s",
			expectedMain:    "",
			expectedPerUnit: "per_second",
		},
		{
			name:            "multiple slashes",
			unit:            "By/s/h",
			expectedMain:    "bytes",
			expectedPerUnit: "per_s/h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mainUnit, perUnit := buildUnitSuffixes(tt.unit)
			if tt.expectedMain != mainUnit {
				t.Errorf("expected main unit %s, got %s", tt.expectedMain, mainUnit)
			}
			if tt.expectedPerUnit != perUnit {
				t.Errorf("expected per unit %s, got %s", tt.expectedPerUnit, perUnit)
			}
		})
	}
}

func TestBuildCompliantMetricName(t *testing.T) {
	tests := []struct {
		name              string
		metricName        string
		unit              string
		metricType        MetricType
		addMetricSuffixes bool
		expected          string
	}{
		{
			name:              "simple valid metric name",
			metricName:        "http_requests",
			unit:              "",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: false,
			expected:          "http_requests",
		},
		{
			name:              "metric name with invalid characters",
			metricName:        "http-requests@in_flight",
			unit:              "",
			metricType:        MetricTypeNonMonotonicCounter,
			addMetricSuffixes: false,
			expected:          "http_requests_in_flight",
		},
		{
			name:              "metric name starting with digit",
			metricName:        "5xx_errors",
			unit:              "",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: false,
			expected:          "_5xx_errors",
		},
		{
			name:              "metric name starting with digit, with suffixes",
			metricName:        "5xx_errors",
			unit:              "",
			metricType:        MetricTypeMonotonicCounter,
			addMetricSuffixes: true,
			expected:          "_5xx_errors_total",
		},
		{
			name:              "metric name with multiple consecutive invalid chars",
			metricName:        "api..//request--time",
			unit:              "",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: false,
			expected:          "api_request_time",
		},
		{
			name:              "full normalization with units and type",
			metricName:        "system.cpu-utilization",
			unit:              "ms/s",
			metricType:        MetricTypeMonotonicCounter,
			addMetricSuffixes: true,
			expected:          "system_cpu_utilization_milliseconds_per_second_total",
		},
		{
			name:              "metric with special characters and ratio",
			metricName:        "memory.usage%rate",
			unit:              "1",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "memory_usage_rate_ratio",
		},
		{
			name:              "metric with unicode characters",
			metricName:        "error_rate_£_€_¥",
			unit:              "",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: false,
			expected:          "error_rate_____",
		},
		{
			name:              "metric with multiple spaces",
			metricName:        "api   response   time",
			unit:              "ms",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "api_response_time_milliseconds",
		},
		{
			name:              "metric with colons (valid prometheus chars)",
			metricName:        "app:request:latency",
			unit:              "s",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: true,
			expected:          "app:request:latency_seconds",
		},
		{
			name:              "empty metric name",
			metricName:        "",
			unit:              "",
			metricType:        MetricTypeGauge,
			addMetricSuffixes: false,
			expected:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildCompliantMetricName(tt.metricName, tt.unit, tt.metricType, tt.addMetricSuffixes)
			if tt.expected != result {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestAddUnitTokens(t *testing.T) {
	tests := []struct {
		nameTokens     []string
		mainUnitSuffix string
		perUnitSuffix  string
		expected       []string
	}{
		{[]string{}, "", "", []string{}},
		{[]string{"token1"}, "main", "", []string{"token1", "main"}},
		{[]string{"token1"}, "", "per", []string{"token1", "per"}},
		{[]string{"token1"}, "main", "per", []string{"token1", "main", "per"}},
		{[]string{"token1", "per"}, "main", "per", []string{"token1", "per", "main"}},
		{[]string{"token1", "main"}, "main", "per", []string{"token1", "main", "per"}},
		{[]string{"token1"}, "main_", "per", []string{"token1", "main", "per"}},
		{[]string{"token1"}, "main_unit", "per_seconds_", []string{"token1", "main_unit", "per_seconds"}}, // trailing underscores are removed
		{[]string{"token1"}, "main_unit", "per_", []string{"token1", "main_unit"}},                        // 'per_' is removed entirely
	}

	for _, test := range tests {
		result := addUnitTokens(test.nameTokens, test.mainUnitSuffix, test.perUnitSuffix)
		if !reflect.DeepEqual(test.expected, result) {
			t.Errorf("expected %v, got %v", test.expected, result)
		}
	}
}

func TestRemoveItem(t *testing.T) {
	if !reflect.DeepEqual([]string{}, removeItem([]string{}, "test")) {
		t.Errorf("expected %v, got %v", []string{}, removeItem([]string{}, "test"))
	}
	if !reflect.DeepEqual([]string{}, removeItem([]string{}, "")) {
		t.Errorf("expected %v, got %v", []string{}, removeItem([]string{}, ""))
	}
	if !reflect.DeepEqual([]string{"a", "b", "c"}, removeItem([]string{"a", "b", "c"}, "d")) {
		t.Errorf("expected %v, got %v", []string{"a", "b", "c"}, removeItem([]string{"a", "b", "c"}, "d"))
	}
	if !reflect.DeepEqual([]string{"a", "b", "c"}, removeItem([]string{"a", "b", "c"}, "")) {
		t.Errorf("expected %v, got %v", []string{"a", "b", "c"}, removeItem([]string{"a", "b", "c"}, ""))
	}
	if !reflect.DeepEqual([]string{"a", "b"}, removeItem([]string{"a", "b", "c"}, "c")) {
		t.Errorf("expected %v, got %v", []string{"a", "b"}, removeItem([]string{"a", "b", "c"}, "c"))
	}
	if !reflect.DeepEqual([]string{"a", "c"}, removeItem([]string{"a", "b", "c"}, "b")) {
		t.Errorf("expected %v, got %v", []string{"a", "c"}, removeItem([]string{"a", "b", "c"}, "b"))
	}
	if !reflect.DeepEqual([]string{"b", "c"}, removeItem([]string{"a", "b", "c"}, "a")) {
		t.Errorf("expected %v, got %v", []string{"b", "c"}, removeItem([]string{"a", "b", "c"}, "a"))
	}
}

func TestCleanUpStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already valid string",
			input:    "valid_metric_name",
			expected: "valid_metric_name",
		},
		{
			name:     "invalid characters",
			input:    "metric-name@with#special$chars",
			expected: "metric_name_with_special_chars",
		},
		{
			name:     "multiple consecutive invalid chars",
			input:    "metric---name###special",
			expected: "metric_name_special",
		},
		{
			name:     "leading invalid chars",
			input:    "@#$metric_name",
			expected: "metric_name",
		},
		{
			name:     "trailing invalid chars",
			input:    "metric_name@#$",
			expected: "metric_name_",
		},
		{
			name:     "multiple consecutive underscores",
			input:    "metric___name____test",
			expected: "metric_name_test",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only invalid chars",
			input:    "@#$%^&",
			expected: "",
		},
		{
			name:     "colons are valid",
			input:    "system.cpu:usage.rate",
			expected: "system_cpu:usage_rate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanUpString(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
