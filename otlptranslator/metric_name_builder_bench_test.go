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
	"fmt"
	"testing"
)

var benchmarkInputs = []struct {
	name       string
	metricName string
	unit       string
	metricType MetricType
}{
	{
		name:       "simple_metric",
		metricName: "http_requests",
		unit:       "",
		metricType: MetricTypeGauge,
	},
	{
		name:       "compound_unit",
		metricName: "request_throughput",
		unit:       "By/s",
		metricType: MetricTypeMonotonicCounter,
	},
	{
		name:       "complex_unit",
		metricName: "disk_usage",
		unit:       "KiBy/m",
		metricType: MetricTypeGauge,
	},
	{
		name:       "ratio_metric",
		metricName: "cpu_utilization",
		unit:       "1",
		metricType: MetricTypeGauge,
	},
	{
		name:       "metric_with_dots",
		metricName: "system.cpu.usage.idle",
		unit:       "%",
		metricType: MetricTypeGauge,
	},
	{
		name:       "metric_with_unicode",
		metricName: "メモリ使用率",
		unit:       "By",
		metricType: MetricTypeGauge,
	},
	{
		name:       "metric_with_special_chars",
		metricName: "error-rate@host{instance}/service#component",
		unit:       "ms",
		metricType: MetricTypeMonotonicCounter,
	},
	{
		name:       "metric_with_multiple_slashes",
		metricName: "network/throughput/total",
		unit:       "By/s/min",
		metricType: MetricTypeGauge,
	},
	{
		name:       "metric_with_spaces",
		metricName: "api   response   time   total",
		unit:       "ms",
		metricType: MetricTypeMonotonicCounter,
	},
	{
		name:       "metric_with_curly_braces",
		metricName: "custom_{tag}_metric",
		unit:       "{custom}/s",
		metricType: MetricTypeGauge,
	},
	{
		name:       "metric_starting_with_digit",
		metricName: "5xx_error_count",
		unit:       "1",
		metricType: MetricTypeMonotonicCounter,
	},
	{
		name:       "empty_metric",
		metricName: "",
		unit:       "",
		metricType: MetricTypeGauge,
	},
	{
		name:       "metric_with_SI_units",
		metricName: "power_consumption",
		unit:       "W",
		metricType: MetricTypeGauge,
	},
	{
		name:       "metric_with_temperature",
		metricName: "server_temperature",
		unit:       "Cel",
		metricType: MetricTypeGauge,
	},
}

func BenchmarkBuildMetricName(b *testing.B) {
	for _, addSuffixes := range []bool{true, false} {
		b.Run(fmt.Sprintf("with_metric_suffixes=%t", addSuffixes), func(b *testing.B) {
			for _, input := range benchmarkInputs {
				for i := 0; i < b.N; i++ {
					BuildMetricName(input.metricName, input.unit, input.metricType, addSuffixes)
				}
			}
		})
	}
}

func BenchmarkBuildCompliantMetricName(b *testing.B) {
	for _, addSuffixes := range []bool{true, false} {
		b.Run(fmt.Sprintf("with_metric_suffixes=%t", addSuffixes), func(b *testing.B) {
			for _, input := range benchmarkInputs {
				for i := 0; i < b.N; i++ {
					BuildCompliantMetricName(input.metricName, input.unit, input.metricType, addSuffixes)
				}
			}
		})
	}
}
