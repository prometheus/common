// Copyright The Prometheus Authors
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

package expfmt

import (
	"bytes"
	"io"
	"math"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestCreateOpenMetrics20(t *testing.T) {
	scenarios := []struct {
		name string
		in   *dto.MetricFamily
		out  string
	}{
		{
			name: "Counter",
			in: &dto.MetricFamily{
				Name: proto.String("http_requests_total"),
				Help: proto.String("Total number of HTTP requests."),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("method"), Value: proto.String("GET")},
							{Name: proto.String("code"), Value: proto.String("200")},
						},
						Counter: &dto.Counter{
							Value:            proto.Float64(1027),
							CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890},
						},
					},
				},
			},
			out: `# HELP http_requests_total Total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="GET",code="200"} 1027 st@1234567890
`,
		},
		{
			name: "Gauge",
			in: &dto.MetricFamily{
				Name: proto.String("node_memory_active_bytes"),
				Help: proto.String("Active memory in bytes."),
				Type: dto.MetricType_GAUGE.Enum(),
				Metric: []*dto.Metric{
					{
						Gauge: &dto.Gauge{
							Value: proto.Float64(1.2345e+09),
						},
					},
				},
			},
			out: `# HELP node_memory_active_bytes Active memory in bytes.
# TYPE node_memory_active_bytes gauge
node_memory_active_bytes 1.2345e+09
`,
		},
		{
			name: "GaugeWithUnit",
			in: &dto.MetricFamily{
				Name: proto.String("node_memory_active_bytes"),
				Help: proto.String("Active memory in bytes."),
				Type: dto.MetricType_GAUGE.Enum(),
				Unit: proto.String("bytes"),
				Metric: []*dto.Metric{
					{
						Gauge: &dto.Gauge{
							Value: proto.Float64(1.2345e+09),
						},
					},
				},
			},
			out: `# HELP node_memory_active_bytes Active memory in bytes.
# TYPE node_memory_active_bytes gauge
# UNIT node_memory_active_bytes bytes
node_memory_active_bytes 1.2345e+09
`,
		},
		{
			name: "GaugeWithTimestamp",
			in: &dto.MetricFamily{
				Name: proto.String("node_memory_active_bytes"),
				Type: dto.MetricType_GAUGE.Enum(),
				Metric: []*dto.Metric{
					{
						Gauge: &dto.Gauge{
							Value: proto.Float64(1.2345e+09),
						},
						TimestampMs: proto.Int64(1234567890000),
					},
				},
			},
			out: `# TYPE node_memory_active_bytes gauge
node_memory_active_bytes 1.2345e+09 1234567890
`,
		},
		{
			name: "CounterWithExemplar",
			in: &dto.MetricFamily{
				Name: proto.String("http_requests_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Counter: &dto.Counter{
							Value: proto.Float64(1027),
							Exemplar: &dto.Exemplar{
								Label: []*dto.LabelPair{
									{Name: proto.String("trace_id"), Value: proto.String("1234")},
								},
								Value: proto.Float64(1),
							},
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027 # {trace_id="1234"} 1.0
`,
		},
		{
			name: "Untyped",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_UNTYPED.Enum(),
				Metric: []*dto.Metric{
					{
						Untyped: &dto.Untyped{
							Value: proto.Float64(1.23),
						},
					},
				},
			},
			out: `# TYPE test_metric unknown
test_metric 1.23
`,
		},
		{
			name: "CounterWithoutCreatedTimestamp",
			in: &dto.MetricFamily{
				Name: proto.String("http_requests_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Counter: &dto.Counter{
							Value: proto.Float64(1027),
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027
`,
		},
		{
			name: "UTF8Support",
			in: &dto.MetricFamily{
				Name: proto.String("你好_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("🌎"), Value: proto.String("🌍")},
						},
						Counter: &dto.Counter{
							Value: proto.Float64(1027),
						},
					},
				},
			},
			out: `# TYPE "你好_total" counter
{"你好_total","🌎"="🌍"} 1027
`,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := MetricFamilyToOpenMetrics20(&buf, scenario.in)
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != scenario.out {
				t.Errorf("expected out:\n%s\ngot:\n%s", scenario.out, buf.String())
			}
		})
	}
}

func TestWriteOpenMetrics20Timestamp_SpecialValues(t *testing.T) {
	tests := []struct {
		name string
		val  float64
		out  string
	}{
		{"NaN", math.NaN(), "NaN"},
		{"+Inf", math.Inf(+1), "+Inf"},
		{"-Inf", math.Inf(-1), "-Inf"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := enhancedWriter(&buf)
			_, err := writeOpenMetrics20Timestamp(w, tc.val)
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != tc.out {
				t.Errorf("expected %q, got %q", tc.out, buf.String())
			}
		})
	}
}

func TestCreateOpenMetrics20_Errors(t *testing.T) {
	tests := []struct {
		name string
		in   *dto.MetricFamily
	}{
		{
			name: "NoName",
			in: &dto.MetricFamily{
				Type: dto.MetricType_COUNTER.Enum(),
			},
		},
		{
			name: "UnknownType",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType(100).Enum(),
			},
		},
		{
			name: "MissingCounter",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{},
				},
			},
		},
		{
			name: "MissingGauge",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_GAUGE.Enum(),
				Metric: []*dto.Metric{
					{},
				},
			},
		},
		{
			name: "MissingUntyped",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_UNTYPED.Enum(),
				Metric: []*dto.Metric{
					{},
				},
			},
		},
		{
			name: "MissingSummary",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_SUMMARY.Enum(),
				Metric: []*dto.Metric{
					{},
				},
			},
		},
		{
			name: "MissingHistogram",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{},
				},
			},
		},
		{
			name: "SummaryNotImplemented",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_SUMMARY.Enum(),
				Metric: []*dto.Metric{
					{Summary: &dto.Summary{}},
				},
			},
		},
		{
			name: "HistogramNotImplemented",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{Histogram: &dto.Histogram{}},
				},
			},
		},
		{
			name: "GaugeHistogramNotImplemented",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{Histogram: &dto.Histogram{}},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := MetricFamilyToOpenMetrics20(&buf, tc.in)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestWriteOpenMetrics20Sample_UseIntValue(t *testing.T) {
	var buf bytes.Buffer
	w := enhancedWriter(&buf)
	metric := &dto.Metric{}
	_, err := writeOpenMetrics20Sample(w, "test_metric", metric, 0, 123, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := "test_metric 123\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestCreateOpenMetrics20_SimpleWriter(t *testing.T) {
	in := &dto.MetricFamily{
		Name: proto.String("http_requests_total"),
		Type: dto.MetricType_COUNTER.Enum(),
		Metric: []*dto.Metric{
			{
				Counter: &dto.Counter{
					Value: proto.Float64(1027),
				},
			},
		},
	}

	var buf bytes.Buffer
	// Wrap bytes.Buffer in a struct that only implements io.Writer
	sw := struct {
		io.Writer
	}{&buf}

	_, err := MetricFamilyToOpenMetrics20(sw, in)
	if err != nil {
		t.Fatal(err)
	}

	expected := `# TYPE http_requests_total counter
http_requests_total 1027
`
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}
