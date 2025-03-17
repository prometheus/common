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

	"go.opentelemetry.io/collector/pdata/pmetric"
)

var benchmarkInputs = []struct {
	name   string
	metric pmetric.Metric
}{
	{
		name:   "simple_metric",
		metric: createGauge("http_requests", ""),
	},
	{
		name:   "compound_unit",
		metric: createCounter("request_throughput", "By/s"),
	},
	{
		name:   "complex_unit",
		metric: createGauge("disk_usage", "KiBy/m"),
	},
	{
		name:   "ratio_metric",
		metric: createGauge("cpu_utilization", "1"),
	},
	{
		name:   "metric_with_dots",
		metric: createGauge("system.cpu.usage.idle", "%"),
	},
	{
		name:   "metric_with_unicode",
		metric: createGauge("メモリ使用率", "By"),
	},
	{
		name:   "metric_with_special_chars",
		metric: createGauge("error-rate@host{instance}/service#component", "ms"),
	},
	{
		name:   "metric_with_multiple_slashes",
		metric: createGauge("network/throughput/total", "By/s/min"),
	},
	{
		name:   "metric_with_spaces",
		metric: createCounter("api   response   time   total", "ms"),
	},
	{
		name:   "metric_with_curly_braces",
		metric: createGauge("custom_{tag}_metric", "{custom}/s"),
	},
	{
		name:   "metric_starting_with_digit",
		metric: createCounter("5xx_error_count", "1"),
	},
	{
		name:   "empty_metric",
		metric: createGauge("", ""),
	},
	{
		name:   "metric_with_SI_units",
		metric: createGauge("power_consumption", "W"),
	},
	{
		name:   "metric_with_temperature",
		metric: createGauge("server_temperature", "Cel"),
	},
}

func BenchmarkBuildMetricName(b *testing.B) {
	for _, addSuffixes := range []bool{true, false} {
		b.Run(fmt.Sprintf("with_metric_suffixes=%t", addSuffixes), func(b *testing.B) {
			for _, input := range benchmarkInputs {
				for i := 0; i < b.N; i++ {
					BuildMetricName(input.metric, "", addSuffixes)
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
					BuildCompliantMetricName(input.metric, "", addSuffixes)
				}
			}
		})
	}
}
