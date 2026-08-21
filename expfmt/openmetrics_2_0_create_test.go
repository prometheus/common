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
	"strings"
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
http_requests_total{method="GET",code="200"} 1027.0 st@1234567890
`,
		},
		{
			name: "CounterWithSubsecondCreatedTimestamp",
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
							CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890, Nanos: 987654321},
						},
					},
				},
			},
			out: `# HELP http_requests_total Total number of HTTP requests.
# TYPE http_requests_total counter
http_requests_total{method="GET",code="200"} 1027.0 st@1234567890.987654321
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
							Value:            proto.Float64(1027),
							CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890},
							Exemplar: &dto.Exemplar{
								Label: []*dto.LabelPair{
									{Name: proto.String("trace_id"), Value: proto.String("1234")},
								},
								Value:     proto.Float64(1),
								Timestamp: &timestamppb.Timestamp{Seconds: 1234567890, Nanos: 500000000},
							},
						},
						TimestampMs: proto.Int64(1234567891000),
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027.0 1234567891 st@1234567890 # {trace_id="1234"} 1.0 1234567890.5
`,
		},
		{
			name: "CounterWithExemplarWithoutLabels",
			in: &dto.MetricFamily{
				Name: proto.String("http_requests_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Counter: &dto.Counter{
							Value: proto.Float64(1027),
							Exemplar: &dto.Exemplar{
								Value:     proto.Float64(1),
								Timestamp: &timestamppb.Timestamp{Seconds: 1234567890, Nanos: 500000000},
							},
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027.0 # {} 1.0 1234567890.5
`,
		},
		{
			name: "CounterWithExemplarWithoutLabels",
			in: &dto.MetricFamily{
				Name: proto.String("http_requests_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Counter: &dto.Counter{
							Value: proto.Float64(1027),
							Exemplar: &dto.Exemplar{
								Value:     proto.Float64(1),
								Timestamp: &timestamppb.Timestamp{Seconds: 1234567890, Nanos: 500000000},
							},
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027 # {} 1 1234567890.5
`,
		},
		{
			name: "CounterWithExemplarWithoutTimestamp",
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
http_requests_total 1027.0
`,
		},
		{
			name: "CounterWithInvalidExemplarTimestamp",
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
								Timestamp: &timestamppb.Timestamp{
									Nanos: -1,
								},
							},
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027.0
`,
		},
		{
			name: "CounterWithInvalidExemplarLabel",
			in: &dto.MetricFamily{
				Name: proto.String("http_requests_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Counter: &dto.Counter{
							Value: proto.Float64(1027),
							Exemplar: &dto.Exemplar{
								Label: []*dto.LabelPair{
									{Name: proto.String(""), Value: proto.String("1234")},
								},
								Value:     proto.Float64(1),
								Timestamp: &timestamppb.Timestamp{Seconds: 1234567890},
							},
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027.0
`,
		},
		{
			name: "CounterWithNewlineInExemplarLabelName",
			in: &dto.MetricFamily{
				Name: proto.String("http_requests_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Counter: &dto.Counter{
							Value: proto.Float64(1027),
							Exemplar: &dto.Exemplar{
								Label: []*dto.LabelPair{
									{Name: proto.String("trace\nid"), Value: proto.String("1234")},
								},
								Value:     proto.Float64(1),
								Timestamp: &timestamppb.Timestamp{Seconds: 1234567890},
							},
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027.0
`,
		},
		{
			name: "CounterWithNilExemplarLabelPair",
			in: &dto.MetricFamily{
				Name: proto.String("http_requests_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Counter: &dto.Counter{
							Value: proto.Float64(1027),
							Exemplar: &dto.Exemplar{
								Label:     []*dto.LabelPair{nil},
								Value:     proto.Float64(1),
								Timestamp: &timestamppb.Timestamp{Seconds: 1234567890},
							},
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027.0
`,
		},
		{
			name: "CounterWithNaNExemplar",
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
								Value:     proto.Float64(math.NaN()),
								Timestamp: &timestamppb.Timestamp{Seconds: 1234567890},
							},
						},
					},
				},
			},
			out: `# TYPE http_requests_total counter
http_requests_total 1027.0 # {trace_id="1234"} NaN 1234567890
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
http_requests_total 1027.0
`,
		},
		{
			name: "GaugeWithAccidentalCounterCreatedTimestamp",
			in: &dto.MetricFamily{
				Name: proto.String("node_memory_active_bytes"),
				Help: proto.String("Active memory in bytes."),
				Type: dto.MetricType_GAUGE.Enum(),
				Metric: []*dto.Metric{
					{
						Gauge: &dto.Gauge{
							Value: proto.Float64(1.2345e+09),
						},
						Counter: &dto.Counter{
							CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890},
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
			name: "UntypedWithAccidentalCounterCreatedTimestamp",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType_UNTYPED.Enum(),
				Metric: []*dto.Metric{
					{
						Untyped: &dto.Untyped{
							Value: proto.Float64(1.23),
						},
						Counter: &dto.Counter{
							CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890},
						},
					},
				},
			},
			out: `# TYPE test_metric unknown
test_metric 1.23
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
{"你好_total","🌎"="🌍"} 1027.0
`,
		},
		{
			name: "ClassicHistogram",
			in: &dto.MetricFamily{
				Name: proto.String("request_duration_seconds"),
				Help: proto.String("Request duration histogram."),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Unit: proto.String("seconds"),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("handler"), Value: proto.String("query")},
						},
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(3),
							SampleSum:   proto.Float64(6.0),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCount: proto.Uint64(1)},
								{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(2)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(3)},
							},
						},
					},
				},
			},
			out: `# HELP request_duration_seconds Request duration histogram.
# TYPE request_duration_seconds histogram
# UNIT request_duration_seconds seconds
request_duration_seconds{handler="query"} {count:3,sum:6,bucket:[0.1:1,1:2,+Inf:3]}
`,
		},
		{
			name: "ClassicHistogram_ImplicitPosInf",
			in: &dto.MetricFamily{
				Name: proto.String("request_duration_seconds"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(2),
							SampleSum:   proto.Float64(1.5),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCount: proto.Uint64(1)},
								{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(2)},
							},
						},
					},
				},
			},
			out: `# TYPE request_duration_seconds histogram
request_duration_seconds {count:2,sum:1.5,bucket:[0.1:1,1:2,+Inf:2]}
`,
		},
		{
			name: "ClassicHistogram_LargeCount_ImplicitPosInf",
			in: &dto.MetricFamily{
				Name: proto.String("request_duration_seconds"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(9007199254740993),
							SampleSum:   proto.Float64(1.5),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCount: proto.Uint64(1)},
							},
						},
					},
				},
			},
			out: `# TYPE request_duration_seconds histogram
request_duration_seconds {count:9007199254740993,sum:1.5,bucket:[0.1:1,+Inf:9007199254740993]}
`,
		},
		{
			name: "ClassicHistogram_NegativeThresholds",
			in: &dto.MetricFamily{
				Name: proto.String("temperature_deviation"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(10),
							SampleSum:   proto.Float64(15.0),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(math.Inf(-1)), CumulativeCount: proto.Uint64(0)},
								{UpperBound: proto.Float64(-1.0), CumulativeCount: proto.Uint64(2)},
								{UpperBound: proto.Float64(0.5), CumulativeCount: proto.Uint64(5)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(10)},
							},
						},
					},
				},
			},
			out: `# TYPE temperature_deviation histogram
temperature_deviation {count:10,sum:15,bucket:[-Inf:0,-1:2,0.5:5,+Inf:10]}
`,
		},
		{
			name: "ClassicHistogram_FloatCounts",
			in: &dto.MetricFamily{
				Name: proto.String("request_duration_seconds"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(5.5),
							SampleSum:        proto.Float64(12.1),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCountFloat: proto.Float64(2.5)},
								{UpperBound: proto.Float64(1.0), CumulativeCountFloat: proto.Float64(5.5)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCountFloat: proto.Float64(5.5)},
							},
						},
					},
				},
			},
			out: `# TYPE request_duration_seconds histogram
request_duration_seconds {count:5.5,sum:12.1,bucket:[0.1:2.5,1:5.5,+Inf:5.5]}
`,
		},
		{
			name: "ClassicHistogram_WithTimestampsAndExemplars",
			in: &dto.MetricFamily{
				Name: proto.String("request_duration_seconds"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:      proto.Uint64(17),
							SampleSum:        proto.Float64(324789.3),
							CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1520879607, Nanos: 789000000},
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.01), CumulativeCount: proto.Uint64(0)},
								{
									UpperBound:      proto.Float64(0.1),
									CumulativeCount: proto.Uint64(8),
									Exemplar: &dto.Exemplar{
										Value:     proto.Float64(0.054),
										Timestamp: &timestamppb.Timestamp{Seconds: 1520879607, Nanos: 700000000},
									},
								},
								{
									UpperBound:      proto.Float64(1.0),
									CumulativeCount: proto.Uint64(11),
									Exemplar: &dto.Exemplar{
										Label: []*dto.LabelPair{
											{Name: proto.String("trace_id"), Value: proto.String("KOO5S4vxi0o")},
										},
										Value:     proto.Float64(1.67),
										Timestamp: &timestamppb.Timestamp{Seconds: 1520879602, Nanos: 890000000},
									},
								},
								{
									UpperBound:      proto.Float64(10.0),
									CumulativeCount: proto.Uint64(17),
									Exemplar: &dto.Exemplar{
										Label: []*dto.LabelPair{
											{Name: proto.String("trace_id"), Value: proto.String("oHg5SJYRHA0")},
										},
										Value:     proto.Float64(9.8),
										Timestamp: &timestamppb.Timestamp{Seconds: 1520879607, Nanos: 789000000},
									},
								},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(17)},
							},
						},
						TimestampMs: proto.Int64(1520879610000),
					},
				},
			},
			out: `# TYPE request_duration_seconds histogram
request_duration_seconds {count:17,sum:324789.3,bucket:[0.01:0,0.1:8,1:11,10:17,+Inf:17]} 1520879610 st@1520879607.789 # {} 0.054 1520879607.7 # {trace_id="KOO5S4vxi0o"} 1.67 1520879602.89 # {trace_id="oHg5SJYRHA0"} 9.8 1520879607.789
`,
		},
		{
			name: "NativeHistogram_PositiveSpans",
			in: &dto.MetricFamily{
				Name: proto.String("latency_seconds"),
				Help: proto.String("Service latency (native histogram)."),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Unit: proto.String("seconds"),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(5),
							SampleSum:     proto.Float64(12.1),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(0.001),
							ZeroCount:     proto.Uint64(2),
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(0), Length: proto.Uint32(3)},
							},
							PositiveDelta: []int64{1, 0, 0},
						},
					},
				},
			},
			out: `# HELP latency_seconds Service latency (native histogram).
# TYPE latency_seconds histogram
# UNIT latency_seconds seconds
latency_seconds {count:5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:3],positive_buckets:[1,1,1]}
`,
		},
		{
			name: "NativeHistogram_NegativeAndPositiveSpans_WithExemplars",
			in: &dto.MetricFamily{
				Name: proto.String("acme_http_request_seconds"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("path"), Value: proto.String("/api/v1")},
							{Name: proto.String("method"), Value: proto.String("GET")},
						},
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(59),
							SampleSum:     proto.Float64(120.0),
							Schema:        proto.Int32(7),
							ZeroThreshold: proto.Float64(1e-4),
							ZeroCount:     proto.Uint64(0),
							NegativeSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(1), Length: proto.Uint32(2)},
							},
							NegativeDelta: []int64{5, 2},
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(-1), Length: proto.Uint32(2)},
								{Offset: proto.Int32(3), Length: proto.Uint32(4)},
							},
							PositiveDelta: []int64{5, 2, 3, -1, -1, 0},
							Exemplars: []*dto.Exemplar{
								{
									Label: []*dto.LabelPair{
										{Name: proto.String("trace_id"), Value: proto.String("shaZ8oxi")},
									},
									Value:     proto.Float64(0.67),
									Timestamp: &timestamppb.Timestamp{Seconds: 1520879607, Nanos: 789000000},
								},
							},
						},
					},
				},
			},
			out: `# TYPE acme_http_request_seconds histogram
acme_http_request_seconds{path="/api/v1",method="GET"} {count:59,sum:120,schema:7,zero_threshold:0.0001,zero_count:0,negative_spans:[1:2],negative_buckets:[5,7],positive_spans:[-1:2,3:4],positive_buckets:[5,7,10,9,8,8]} # {trace_id="shaZ8oxi"} 0.67 1520879607.789
`,
		},
		{
			name: "NativeHistogram_ZeroObservations",
			in: &dto.MetricFamily{
				Name: proto.String("acme_http_request_seconds"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("path"), Value: proto.String("/api/v1")},
							{Name: proto.String("method"), Value: proto.String("GET")},
						},
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(0),
							SampleSum:     proto.Float64(0),
							Schema:        proto.Int32(3),
							ZeroThreshold: proto.Float64(1e-4),
							ZeroCount:     proto.Uint64(0),
						},
					},
				},
			},
			out: `# TYPE acme_http_request_seconds histogram
acme_http_request_seconds{path="/api/v1",method="GET"} {count:0,sum:0,schema:3,zero_threshold:0.0001,zero_count:0}
`,
		},
		{
			name: "NativeFloatHistogram",
			in: &dto.MetricFamily{
				Name: proto.String("payload_size"),
				Help: proto.String("Payload size (float native histogram)."),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(5.5),
							SampleSum:        proto.Float64(12.1),
							Schema:           proto.Int32(0),
							ZeroThreshold:    proto.Float64(0.001),
							ZeroCountFloat:   proto.Float64(2.5),
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(0), Length: proto.Uint32(2)},
							},
							PositiveCount: []float64{2.0, 1.0},
						},
					},
				},
			},
			out: `# HELP payload_size Payload size (float native histogram).
# TYPE payload_size histogram
payload_size {count:5.5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2.5,positive_spans:[0:2],positive_buckets:[2,1]}
`,
		},
		{
			name: "DualHistogram_NativeAndClassic",
			in: &dto.MetricFamily{
				Name: proto.String("acme_http_request_seconds"),
				Help: proto.String("Latency histogram of all of ACME's HTTP requests."),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Unit: proto.String("seconds"),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("path"), Value: proto.String("/api/v1")},
							{Name: proto.String("method"), Value: proto.String("GET")},
						},
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(2),
							SampleSum:     proto.Float64(120.0),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(1e-4),
							ZeroCount:     proto.Uint64(0),
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(1), Length: proto.Uint32(2)},
							},
							PositiveDelta: []int64{1, 0},
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.5), CumulativeCount: proto.Uint64(1)},
								{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(2)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(2)},
							},
						},
					},
				},
			},
			out: `# HELP acme_http_request_seconds Latency histogram of all of ACME's HTTP requests.
# TYPE acme_http_request_seconds histogram
# UNIT acme_http_request_seconds seconds
acme_http_request_seconds{path="/api/v1",method="GET"} {count:2,sum:120,schema:0,zero_threshold:0.0001,zero_count:0,positive_spans:[1:2],positive_buckets:[1,1],bucket:[0.5:1,1:2,+Inf:2]}
`,
		},
		{
			name: "GaugeHistogram_Classic",
			in: &dto.MetricFamily{
				Name: proto.String("foo"),
				Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(42),
							SampleSum:   proto.Float64(3289.3),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.01), CumulativeCount: proto.Uint64(20)},
								{UpperBound: proto.Float64(0.1), CumulativeCount: proto.Uint64(25)},
								{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(34)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(42)},
							},
						},
					},
				},
			},
			out: `# TYPE foo gaugehistogram
foo {gcount:42,gsum:3289.3,bucket:[0.01:20,0.1:25,1:34,+Inf:42]}
`,
		},
		{
			name: "GaugeHistogram_NativeFloat",
			in: &dto.MetricFamily{
				Name: proto.String("acme_http_request_seconds:rate5m"),
				Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("path"), Value: proto.String("/api/v1")},
							{Name: proto.String("method"), Value: proto.String("GET")},
						},
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(0.01),
							SampleSum:        proto.Float64(2.0),
							Schema:           proto.Int32(0),
							ZeroThreshold:    proto.Float64(1e-4),
							ZeroCountFloat:   proto.Float64(0.0),
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(1), Length: proto.Uint32(2)},
							},
							PositiveCount: []float64{0.005, 0.005},
						},
					},
				},
			},
			out: `# TYPE acme_http_request_seconds:rate5m gaugehistogram
acme_http_request_seconds:rate5m{path="/api/v1",method="GET"} {gcount:0.01,gsum:2,schema:0,zero_threshold:0.0001,zero_count:0,positive_spans:[1:2],positive_buckets:[0.005,0.005]}
`,
		},
		{
			name: "GaugeHistogram_NegativeValues",
			in: &dto.MetricFamily{
				Name: proto.String("net_flow_rate"),
				Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(-1.5),
							SampleSum:        proto.Float64(-3.2),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(-0.5), CumulativeCountFloat: proto.Float64(-2.0)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCountFloat: proto.Float64(-1.5)},
							},
						},
					},
				},
			},
			out: `# TYPE net_flow_rate gaugehistogram
net_flow_rate {gcount:-1.5,gsum:-3.2,bucket:[-0.5:-2,+Inf:-1.5]}
`,
		},
		{
			name: "ClassicHistogram_EmptyBuckets",
			in: &dto.MetricFamily{
				Name: proto.String("empty_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(0),
							SampleSum:   proto.Float64(0),
						},
					},
				},
			},
			out: `# TYPE empty_histogram histogram
empty_histogram {count:0,sum:0,bucket:[+Inf:0]}
`,
		},
		{
			name: "Histogram_UTF8",
			in: &dto.MetricFamily{
				Name: proto.String("http.latency"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("service.name"), Value: proto.String("my-service")},
						},
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(1),
							SampleSum:   proto.Float64(0.05),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCount: proto.Uint64(1)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(1)},
							},
						},
					},
				},
			},
			out: `# TYPE "http.latency" histogram
{"http.latency","service.name"="my-service"} {count:1,sum:0.05,bucket:[0.1:1,+Inf:1]}
`,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var buf bytes.Buffer
			n, err := MetricFamilyToOpenMetrics20(&buf, scenario.in)
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != scenario.out {
				t.Errorf("expected out:\n%s\ngot:\n%s", scenario.out, buf.String())
			}
			if n != len(scenario.out) {
				t.Errorf("expected %d bytes written, got %d", len(scenario.out), n)
			}
		})
	}
}

func TestWriteOpenMetrics20Timestamp(t *testing.T) {
	tests := []struct {
		name string
		val  float64
		out  string
	}{
		{"Integer", 1234567890, "1234567890"},
		{"Subsecond", 1234567890.123, "1234567890.123"},
		{"Zero", 0, "0"},
		{"NaN", math.NaN(), "NaN"},
		{"+Inf", math.Inf(+1), "+Inf"},
		{"-Inf", math.Inf(-1), "-Inf"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := enhancedWriter(&buf)
			n, err := writeOpenMetrics20Timestamp(w, tc.val)
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != tc.out {
				t.Errorf("expected %q, got %q", tc.out, buf.String())
			}
			if n != len(tc.out) {
				t.Errorf("expected %d bytes written, got %d", len(tc.out), n)
			}
		})
	}
}

func TestCreateOpenMetrics20_Errors(t *testing.T) {
	tests := []struct {
		name        string
		in          *dto.MetricFamily
		expectedErr string
	}{
		{
			name: "NoName",
			in: &dto.MetricFamily{
				Type: dto.MetricType_COUNTER.Enum(),
			},
			expectedErr: "MetricFamily has no name",
		},
		{
			name: "UnknownType",
			in: &dto.MetricFamily{
				Name: proto.String("test_metric"),
				Type: dto.MetricType(100).Enum(),
			},
			expectedErr: "unknown metric type",
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
			expectedErr: "expected counter in metric",
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
			expectedErr: "expected gauge in metric",
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
			expectedErr: "expected untyped in metric",
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
			expectedErr: "expected summary in metric",
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
			expectedErr: "expected histogram in metric",
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
			expectedErr: "summary not implemented yet",
		},
		{
			name: "HistogramCountNegative",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(-1.0),
							SampleSum:        proto.Float64(0.0),
						},
					},
				},
			},
			expectedErr: "histogram count cannot be negative",
		},
		{
			name: "HistogramCountNaN",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(math.NaN()),
							SampleSum:        proto.Float64(0.0),
						},
					},
				},
			},
			expectedErr: "histogram count cannot be NaN",
		},
		{
			name: "GaugeHistogramCountNaN",
			in: &dto.MetricFamily{
				Name: proto.String("test_gauge_histogram"),
				Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(math.NaN()),
							SampleSum:        proto.Float64(0.0),
						},
					},
				},
			},
			expectedErr: "gaugehistogram count cannot be NaN",
		},
		{
			name: "MetricHasLeLabel_ClassicHistogram",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("le"), Value: proto.String("0.1")},
						},
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(1),
							SampleSum:   proto.Float64(0.1),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCount: proto.Uint64(1)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(1)},
							},
						},
					},
				},
			},
			expectedErr: "has classic buckets but label set contains \"le\" label",
		},
		{
			name: "MetricHasLeLabel_ClassicGaugeHistogram",
			in: &dto.MetricFamily{
				Name: proto.String("test_gauge_histogram"),
				Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("le"), Value: proto.String("0.1")},
						},
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(1),
							SampleSum:   proto.Float64(0.1),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCount: proto.Uint64(1)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(1)},
							},
						},
					},
				},
			},
			expectedErr: "has classic buckets but label set contains \"le\" label",
		},
		{
			name: "ClassicBucketThresholdNaN",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(1),
							SampleSum:   proto.Float64(0.1),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(math.NaN()), CumulativeCount: proto.Uint64(1)},
							},
						},
					},
				},
			},
			expectedErr: "classic bucket upper bound cannot be NaN",
		},
		{
			name: "ClassicBucketThresholdsUnsorted",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(2),
							SampleSum:   proto.Float64(0.3),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(1)},
								{UpperBound: proto.Float64(0.5), CumulativeCount: proto.Uint64(2)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(2)},
							},
						},
					},
				},
			},
			expectedErr: "classic bucket upper bounds must be strictly increasing",
		},
		{
			name: "ClassicBucketThresholdsDuplicate",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(2),
							SampleSum:   proto.Float64(0.3),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(1)},
								{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(2)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(2)},
							},
						},
					},
				},
			},
			expectedErr: "classic bucket upper bounds must be strictly increasing",
		},
		{
			name: "ClassicBucketPosInfMismatch",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(5),
							SampleSum:   proto.Float64(10.0),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(2)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(4)},
							},
						},
					},
				},
			},
			expectedErr: "classic bucket +Inf count (4) does not match sample count (5)",
		},
		{
			name: "ClassicBucketCountNaN",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(1.0),
							SampleSum:        proto.Float64(0.1),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCountFloat: proto.Float64(math.NaN())},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCountFloat: proto.Float64(1.0)},
							},
						},
					},
				},
			},
			expectedErr: "classic bucket count cannot be NaN",
		},
		{
			name: "ClassicBucketCountNegative",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(1.0),
							SampleSum:        proto.Float64(0.1),
							Bucket: []*dto.Bucket{
								{UpperBound: proto.Float64(0.1), CumulativeCountFloat: proto.Float64(-1.0)},
								{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCountFloat: proto.Float64(1.0)},
							},
						},
					},
				},
			},
			expectedErr: "classic bucket count cannot be negative",
		},
		{
			name: "NativeHistogramSchemaOutOfRange_Low",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(0),
							SampleSum:     proto.Float64(0),
							Schema:        proto.Int32(-5),
							ZeroThreshold: proto.Float64(0.001),
							ZeroCount:     proto.Uint64(0),
						},
					},
				},
			},
			expectedErr: "native histogram schema -5 is out of range [-4, 8]",
		},
		{
			name: "NativeHistogramSchemaOutOfRange_High",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(0),
							SampleSum:     proto.Float64(0),
							Schema:        proto.Int32(9),
							ZeroThreshold: proto.Float64(0.001),
							ZeroCount:     proto.Uint64(0),
						},
					},
				},
			},
			expectedErr: "native histogram schema 9 is out of range [-4, 8]",
		},
		{
			name: "NativeHistogramZeroThresholdNegative",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(0),
							SampleSum:     proto.Float64(0),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(-0.001),
							ZeroCount:     proto.Uint64(0),
						},
					},
				},
			},
			expectedErr: "native histogram zero_threshold -0.001 must be a non-negative, finite number",
		},
		{
			name: "NativeHistogramZeroThresholdNaN",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(0),
							SampleSum:     proto.Float64(0),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(math.NaN()),
							ZeroCount:     proto.Uint64(0),
						},
					},
				},
			},
			expectedErr: "native histogram zero_threshold NaN must be a non-negative, finite number",
		},
		{
			name: "NativeHistogramZeroThresholdInf",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(0),
							SampleSum:     proto.Float64(0),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(math.Inf(+1)),
							ZeroCount:     proto.Uint64(0),
						},
					},
				},
			},
			expectedErr: "native histogram zero_threshold +Inf must be a non-negative, finite number",
		},
		{
			name: "NativeHistogramZeroCountNaN",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(0),
							SampleSum:        proto.Float64(0),
							Schema:           proto.Int32(0),
							ZeroThreshold:    proto.Float64(0.001),
							ZeroCountFloat:   proto.Float64(math.NaN()),
						},
					},
				},
			},
			expectedErr: "native histogram zero_count cannot be NaN",
		},
		{
			name: "NativeHistogramZeroCountNegative",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(0),
							SampleSum:        proto.Float64(0),
							Schema:           proto.Int32(0),
							ZeroThreshold:    proto.Float64(0.001),
							ZeroCountFloat:   proto.Float64(-1.0),
						},
					},
				},
			},
			expectedErr: "native histogram zero_count cannot be negative",
		},
		{
			name: "NativeHistogramSubsequentSpanNegativeOffset",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(2),
							SampleSum:     proto.Float64(1.0),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(0.001),
							ZeroCount:     proto.Uint64(0),
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(0), Length: proto.Uint32(1)},
								{Offset: proto.Int32(-1), Length: proto.Uint32(1)},
							},
							PositiveDelta: []int64{1, 0},
						},
					},
				},
			},
			expectedErr: "subsequent positive span offset cannot be negative: -1",
		},
		{
			name: "NativeHistogramSpanLengthMismatch",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(2),
							SampleSum:     proto.Float64(1.0),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(0.001),
							ZeroCount:     proto.Uint64(0),
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(0), Length: proto.Uint32(3)},
							},
							PositiveDelta: []int64{1, 0},
						},
					},
				},
			},
			expectedErr: "sum of positive span lengths (3) does not match bucket count (2)",
		},
		{
			name: "NativeHistogramBucketCountNaN",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCountFloat: proto.Float64(1.0),
							SampleSum:        proto.Float64(1.0),
							Schema:           proto.Int32(0),
							ZeroThreshold:    proto.Float64(0.001),
							ZeroCountFloat:   proto.Float64(0),
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(0), Length: proto.Uint32(1)},
							},
							PositiveCount: []float64{math.NaN()},
						},
					},
				},
			},
			expectedErr: "positive bucket count cannot be NaN",
		},
		{
			name: "NativeHistogramBucketCountNegative",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(1),
							SampleSum:     proto.Float64(1.0),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(0.001),
							ZeroCount:     proto.Uint64(0),
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(0), Length: proto.Uint32(1)},
							},
							PositiveDelta: []int64{-1},
						},
					},
				},
			},
			expectedErr: "positive bucket count cannot be negative (-1)",
		},
		{
			name: "HistogramInvalidCreatedTimestamp",
			in: &dto.MetricFamily{
				Name: proto.String("test_histogram"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(0),
							SampleSum:   proto.Float64(0),
							CreatedTimestamp: &timestamppb.Timestamp{
								Nanos: -1,
							},
						},
					},
				},
			},
			expectedErr: "invalid created timestamp in metric test_histogram",
		},
		{
			name: "CounterValueNaN",
			in: &dto.MetricFamily{
				Name: proto.String("test_counter_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{Counter: &dto.Counter{Value: proto.Float64(math.NaN())}},
				},
			},
			expectedErr: "counter value cannot be NaN",
		},
		{
			name: "CounterValueNegative",
			in: &dto.MetricFamily{
				Name: proto.String("test_counter_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{Counter: &dto.Counter{Value: proto.Float64(-5.0)}},
				},
			},
			expectedErr: "counter value cannot be negative",
		},
		{
			name: "EmptyLabelName",
			in: &dto.MetricFamily{
				Name: proto.String("test_counter_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String(""), Value: proto.String("bar")},
						},
						Counter: &dto.Counter{Value: proto.Float64(1.0)},
					},
				},
			},
			expectedErr: "label name cannot be empty",
		},
		{
			name: "NewlineInMetricName",
			in: &dto.MetricFamily{
				Name: proto.String("test_counter\ntotal"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{Counter: &dto.Counter{Value: proto.Float64(1.0)}},
				},
			},
			expectedErr: "contains raw newlines",
		},
		{
			name: "NewlineInUnit",
			in: &dto.MetricFamily{
				Name: proto.String("test_counter_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Unit: proto.String("seconds\n"),
				Metric: []*dto.Metric{
					{Counter: &dto.Counter{Value: proto.Float64(1.0)}},
				},
			},
			expectedErr: "contains raw newlines",
		},
		{
			name: "CarriageReturnInUnit",
			in: &dto.MetricFamily{
				Name: proto.String("test_counter_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Unit: proto.String("seconds\r"),
				Metric: []*dto.Metric{
					{Counter: &dto.Counter{Value: proto.Float64(1.0)}},
				},
			},
			expectedErr: "contains raw newlines",
		},
		{
			name: "NilMetric",
			in: &dto.MetricFamily{
				Name:   proto.String("test_counter_total"),
				Type:   dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{nil},
			},
			expectedErr: "expected non-nil metric",
		},
		{
			name: "NilLabelPair",
			in: &dto.MetricFamily{
				Name: proto.String("test_counter_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Label:   []*dto.LabelPair{nil},
						Counter: &dto.Counter{Value: proto.Float64(1.0)},
					},
				},
			},
			expectedErr: "expected non-nil label pair",
		},
		{
			name: "CounterInvalidCreatedTimestamp",
			in: &dto.MetricFamily{
				Name: proto.String("test_counter_total"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					{
						Counter: &dto.Counter{
							Value: proto.Float64(1.0),
							CreatedTimestamp: &timestamppb.Timestamp{
								Nanos: -1,
							},
						},
					},
				},
			},
			expectedErr: "invalid created timestamp in metric test_counter_total",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			_, err := MetricFamilyToOpenMetrics20(&buf, tc.in)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Fatalf("expected error containing %q, got %q", tc.expectedErr, err.Error())
			}
		})
	}
}

func TestWriteOpenMetrics20Sample_UseIntValue(t *testing.T) {
	var buf bytes.Buffer
	w := enhancedWriter(&buf)
	metric := &dto.Metric{}
	n, err := writeOpenMetrics20Sample(w, "test_metric", metric, 0, 123, true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	expected := "test_metric 123\n"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
	if n != len(expected) {
		t.Errorf("expected %d bytes written, got %d", len(expected), n)
	}
}

func TestWriteProtoTimestamp(t *testing.T) {
	tests := []struct {
		name string
		ts   *timestamppb.Timestamp
		out  string
	}{
		{
			name: "WholeSecondsPositive",
			ts:   &timestamppb.Timestamp{Seconds: 1234567890},
			out:  "1234567890",
		},
		{
			name: "SubsecondPositive",
			ts:   &timestamppb.Timestamp{Seconds: 1234567890, Nanos: 500000000},
			out:  "1234567890.5",
		},
		{
			name: "SubsecondPositiveFullPrecision",
			ts:   &timestamppb.Timestamp{Seconds: 1234567890, Nanos: 987654321},
			out:  "1234567890.987654321",
		},
		{
			name: "ZeroSecondsSubsecond",
			ts:   &timestamppb.Timestamp{Seconds: 0, Nanos: 500000000},
			out:  "0.5",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := enhancedWriter(&buf)
			n, err := writeProtoTimestamp(w, tc.ts)
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != tc.out {
				t.Errorf("expected %q, got %q", tc.out, buf.String())
			}
			if n != len(tc.out) {
				t.Errorf("expected %d bytes written, got %d", len(tc.out), n)
			}
		})
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

	n, err := MetricFamilyToOpenMetrics20(sw, in)
	if err != nil {
		t.Fatal(err)
	}

	expected := `# TYPE http_requests_total counter
http_requests_total 1027.0
`
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
	if n != len(expected) {
		t.Errorf("expected %d bytes written, got %d", len(expected), n)
	}
}
