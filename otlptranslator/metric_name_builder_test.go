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

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestBuildMetricName(t *testing.T) {
	tests := []struct {
		name              string
		metric            pmetric.Metric
		addMetricSuffixes bool
		expected          string
	}{
		{
			name:              "simple metric without suffixes",
			metric:            createGauge("http_requests", ""),
			addMetricSuffixes: false,
			expected:          "http_requests",
		},
		{
			name:              "counter with total suffix",
			metric:            createCounter("http_requests", ""),
			addMetricSuffixes: true,
			expected:          "http_requests_total",
		},
		{
			name:              "gauge with time unit",
			metric:            createGauge("request_duration", "s"),
			addMetricSuffixes: true,
			expected:          "request_duration_seconds",
		},
		{
			name:              "counter with time unit",
			metric:            createCounter("request_duration", "ms"),
			addMetricSuffixes: true,
			expected:          "request_duration_milliseconds_total",
		},
		{
			name:              "gauge with compound unit",
			metric:            createGauge("throughput", "By/s"),
			addMetricSuffixes: true,
			expected:          "throughput_bytes_per_second",
		},
		{
			name:              "ratio metric",
			metric:            createGauge("cpu_utilization", "1"),
			addMetricSuffixes: true,
			expected:          "cpu_utilization_ratio",
		},
		{
			name:              "counter with unit 1 (no ratio suffix)",
			metric:            createCounter("error_count", "1"),
			addMetricSuffixes: true,
			expected:          "error_count_total",
		},
		{
			name:              "metric with byte units",
			metric:            createGauge("memory_usage", "MiBy"),
			addMetricSuffixes: true,
			expected:          "memory_usage_mebibytes",
		},
		{
			name:              "metric with SI units",
			metric:            createGauge("temperature", "Cel"),
			addMetricSuffixes: true,
			expected:          "temperature_celsius",
		},
		{
			name:              "metric with dots",
			metric:            createGauge("system.cpu.usage", "1"),
			addMetricSuffixes: true,
			expected:          "system.cpu.usage_ratio",
		},
		{
			name: "metric with japanese characters (memory usage rate)",
			// memori shiyouritsu (memory usage rate) xD
			metric:            createGauge("メモリ使用率", "By"),
			addMetricSuffixes: true,
			expected:          "メモリ使用率_bytes",
		},
		{
			name: "metric with mixed special characters (system.memory.usage.rate)",
			// system.memory.usage.rate
			metric:            createGauge("system.メモリ.usage.率", "By/s"),
			addMetricSuffixes: true,
			expected:          "system.メモリ.usage.率_bytes_per_second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildMetricName(tt.metric, "", tt.addMetricSuffixes)
			require.Equal(t, tt.expected, result)
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
			require.Equal(t, tt.expectedMain, mainUnit)
			require.Equal(t, tt.expectedPerUnit, perUnit)
		})
	}
}

func TestBuildCompliantMetricName(t *testing.T) {
	tests := []struct {
		name              string
		metric            pmetric.Metric
		addMetricSuffixes bool
		expected          string
	}{
		{
			name:              "simple valid metric name",
			metric:            createGauge("http_requests", ""),
			addMetricSuffixes: false,
			expected:          "http_requests",
		},
		{
			name:              "metric name with invalid characters",
			metric:            createCounter("http-requests@in_flight", ""),
			addMetricSuffixes: false,
			expected:          "http_requests_in_flight",
		},
		{
			name:              "metric name starting with digit",
			metric:            createGauge("5xx_errors", ""),
			addMetricSuffixes: false,
			expected:          "_5xx_errors",
		},
		{
			name:              "metric name starting with digit, with suffixes",
			metric:            createCounter("5xx_errors", ""),
			addMetricSuffixes: true,
			expected:          "_5xx_errors_total",
		},
		{
			name:              "metric name with multiple consecutive invalid chars",
			metric:            createGauge("api..//request--time", ""),
			addMetricSuffixes: false,
			expected:          "api_request_time",
		},
		{
			name:              "full normalization with units and type",
			metric:            createCounter("system.cpu-utilization", "ms/s"),
			addMetricSuffixes: true,
			expected:          "system_cpu_utilization_milliseconds_per_second_total",
		},
		{
			name:              "metric with special characters and ratio",
			metric:            createGauge("memory.usage%rate", "1"),
			addMetricSuffixes: true,
			expected:          "memory_usage_rate_ratio",
		},
		{
			name:              "metric with unicode characters",
			metric:            createGauge("error_rate_£_€_¥", ""),
			addMetricSuffixes: false,
			expected:          "error_rate_____",
		},
		{
			name:              "metric with multiple spaces",
			metric:            createGauge("api   response   time", "ms"),
			addMetricSuffixes: true,
			expected:          "api_response_time_milliseconds",
		},
		{
			name:              "metric with colons (valid prometheus chars)",
			metric:            createGauge("app:request:latency", "s"),
			addMetricSuffixes: true,
			expected:          "app:request:latency_seconds",
		},
		{
			name:              "empty metric name",
			metric:            createGauge("", ""),
			addMetricSuffixes: false,
			expected:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildCompliantMetricName(tt.metric, "", tt.addMetricSuffixes)
			require.Equal(t, tt.expected, result)
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
			require.Equal(t, tt.expected, result)
		})
	}
}
