// Copyright 2020 The Prometheus Authors
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
	"math"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func testOpenMetricsParse(t testing.TB) {
	var omParser OpenMetricsParser
	metricTimestamp := timestamppb.New(time.Unix(123456, 600000000))
	scenarios := []struct {
		in  string
		out []*dto.MetricFamily
	}{
		// 1: EOF as input
		{
			in: `# EOF
`,

			out: []*dto.MetricFamily{},
		},

		// 2: Counter with int64 value
		{
			in: `# TYPE foo counter
foo_total 12345678901234567890
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						{
							Counter: &dto.Counter{
								Value: proto.Float64(12345678901234567890),
							},
						},
					},
				},
			},
		},

		// 3: Counter without unit.
		{
			in: `# HELP foos Number of foos.
# TYPE foos counter
foos_total 42.0
foos_created 123456.7
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos"),
					Help: proto.String("Number of foos."),
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						{
							Counter: &dto.Counter{
								Value:            proto.Float64(42),
								CreatedTimestamp: metricTimestamp,
							},
						},
					},
				},
			},
		},

		// 4: Counter with unit
		{
			in: `# TYPE foos_seconds counter
# HELP foos_seconds help
# UNIT foos_seconds seconds
foos_seconds_total 1
foos_seconds_created 123456.7
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos_seconds"),
					Help: proto.String("help"),
					Type: dto.MetricType_COUNTER.Enum(),
					Unit: proto.String("seconds"),
					Metric: []*dto.Metric{
						{
							Counter: &dto.Counter{
								Value:            proto.Float64(1),
								CreatedTimestamp: metricTimestamp,
							},
						},
					},
				},
			},
		},

		// 5: Counter with labels
		{
			in: `# TYPE foos_seconds counter
# HELP foos_seconds help
# UNIT foos_seconds seconds
foos_seconds_total{a="1", b="2"} 1
foos_seconds_created{a="1", b="2"} 12345.6
foos_seconds_total{a="2", b="3"} 2
foos_seconds_created{a="2", b="3"} 123456.6
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos_seconds"),
					Help: proto.String("help"),
					Type: dto.MetricType_COUNTER.Enum(),
					Unit: proto.String("seconds"),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("a"),
									Value: proto.String("1"),
								},
								{
									Name:  proto.String("b"),
									Value: proto.String("2"),
								},
							},
							Counter: &dto.Counter{
								Value:            proto.Float64(1),
								CreatedTimestamp: timestamppb.New(time.Unix(12345, 600000000)),
							},
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("a"),
									Value: proto.String("2"),
								},
								{
									Name:  proto.String("b"),
									Value: proto.String("3"),
								},
							},
							Counter: &dto.Counter{
								Value:            proto.Float64(2),
								CreatedTimestamp: timestamppb.New(time.Unix(123456, 600000000)),
							},
						},
					},
				},
			},
		},

		// 6: Counter without timestamp and created
		{
			in: `# TYPE foo counter
foo_total 17.0
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						{
							Counter: &dto.Counter{
								Value: proto.Float64(17),
							},
						},
					},
				},
			},
		},

		// 7: Counter with timestamp
		{
			in: `# TYPE foo counter
foo_total 17.0 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						{
							Counter: &dto.Counter{
								Value: proto.Float64(17),
							},
							TimestampMs: proto.Int64(123456),
						},
					},
				},
			},
		},

		// 8: Counter with exemplar
		{
			in: `# TYPE foo counter
# HELP foo help
foo_total{b="c"} 0 123456 # {a="b"} 0.5 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Help: proto.String("help"),
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("b"),
									Value: proto.String("c"),
								},
							},
							Counter: &dto.Counter{
								Value: proto.Float64(0),
								Exemplar: &dto.Exemplar{
									Label: []*dto.LabelPair{
										{
											Name:  proto.String("a"),
											Value: proto.String("b"),
										},
									},
									Value:     proto.Float64(0.5),
									Timestamp: metricTimestamp,
								},
							},
							TimestampMs: proto.Int64(123456),
						},
					},
				},
			},
		},

		// 9: Counter empty labelset
		{
			in: `# TYPE foo counter
# HELP foo help
foo_total{} 0 123456 # {a="b"} 0.5
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Help: proto.String("help"),
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						{
							Counter: &dto.Counter{
								Value: proto.Float64(0),
								Exemplar: &dto.Exemplar{
									Label: []*dto.LabelPair{
										{
											Name:  proto.String("a"),
											Value: proto.String("b"),
										},
									},
									Value: proto.Float64(0.5),
								},
							},
							TimestampMs: proto.Int64(123456),
						},
					},
				},
			},
		},

		// 10: Gauge with unit
		{
			in: `# TYPE foos_seconds gauge
# HELP foos_seconds help
# UNIT foos_seconds seconds
foos_seconds{b="c"} 0
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos_seconds"),
					Help: proto.String("help"),
					Unit: proto.String("seconds"),
					Type: dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("b"),
									Value: proto.String("c"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(0),
							},
						},
					},
				},
			},
		},

		// 11: Gauge with unit and timestamp
		{
			in: `# TYPE foos_seconds gauge
# HELP foos_seconds help
# UNIT foos_seconds seconds
foos_seconds{b="c"} 0 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos_seconds"),
					Help: proto.String("help"),
					Unit: proto.String("seconds"),
					Type: dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("b"),
									Value: proto.String("c"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(0),
							},
							TimestampMs: proto.Int64(123456),
						},
					},
				},
			},
		},

		// 12: Gauge with float value
		{
			in: `# TYPE foos_seconds gauge
# HELP foos_seconds help
# UNIT foos_seconds seconds
foos_seconds{b="c"} 0.12345678
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos_seconds"),
					Help: proto.String("help"),
					Unit: proto.String("seconds"),
					Type: dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("b"),
									Value: proto.String("c"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(0.12345678),
							},
						},
					},
				},
			},
		},

		// 13: Gauge empty labelset
		{
			in: `# TYPE foos_seconds gauge
# HELP foos_seconds help
# UNIT foos_seconds seconds
foos_seconds{} 0.12345678
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos_seconds"),
					Help: proto.String("help"),
					Type: dto.MetricType_GAUGE.Enum(),
					Unit: proto.String("seconds"),
					Metric: []*dto.Metric{
						{
							Gauge: &dto.Gauge{
								Value: proto.Float64(0.12345678),
							},
						},
					},
				},
			},
		},

		// 14: Untyped metric
		{
			in: `# TYPE foos_seconds untyped
# HELP foos_seconds help
# UNIT foos_seconds seconds
foos_seconds{a="v"} 0.12345678
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos_seconds"),
					Help: proto.String("help"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Unit: proto.String("seconds"),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("a"),
									Value: proto.String("v"),
								},
							},
							Untyped: &dto.Untyped{
								Value: proto.Float64(0.12345678),
							},
						},
					},
				},
			},
		},

		// 15: Unsupported metric type(info, stateset)
		{
			in: `# TYPE foos_info info
# HELP foos_info help
foos_info{a="v"} 1
# TYPE foos stateset
# HELP foos help
foos{foos="a"} 1
foos{foos="b"} 0
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foos_info"),
					Help: proto.String("help"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("a"),
									Value: proto.String("v"),
								},
							},
							Untyped: &dto.Untyped{
								Value: proto.Float64(1),
							},
						},
					},
				},
				{
					Name: proto.String("foos"),
					Help: proto.String("help"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("foos"),
									Value: proto.String("a"),
								},
							},
							Untyped: &dto.Untyped{
								Value: proto.Float64(1),
							},
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("foos"),
									Value: proto.String("b"),
								},
							},
							Untyped: &dto.Untyped{
								Value: proto.Float64(0),
							},
						},
					},
				},
			},
		},

		// 16: Simple summary with quantile
		{
			in: `# TYPE a summary
# HELP a help
a_count 1
a_sum 2
a{quantile="0.5"} 0.7
a{quantile="1"} 0.8
a_created 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("a"),
					Help: proto.String("help"),
					Type: dto.MetricType_SUMMARY.Enum(),
					Metric: []*dto.Metric{
						{
							Summary: &dto.Summary{
								SampleCount: proto.Uint64(1),
								SampleSum:   proto.Float64(2),
								Quantile: []*dto.Quantile{
									{
										Quantile: proto.Float64(0.5),
										Value:    proto.Float64(0.7),
									},
									{
										Quantile: proto.Float64(1),
										Value:    proto.Float64(0.8),
									},
								},
								CreatedTimestamp: metricTimestamp,
							},
						},
					},
				},
			},
		},

		// 17: Simple summary with labels
		{
			in: `# TYPE a summary
# HELP a help
a_count{b="c1"} 1
a_sum{b="c1"} 2
a{b="c1", quantile="0.5"} 0.7
a{b="c1", quantile="1"} 0.8
a_created{b="c1"} 123456
a_count{b="c2"} 2
a_sum{b="c2"} 3
a{b="c2", quantile="0.5"} 0.1
a{b="c2", quantile="1"} 0.2
a_created{b="c2"} 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("a"),
					Help: proto.String("help"),
					Type: dto.MetricType_SUMMARY.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("b"),
									Value: proto.String("c1"),
								},
							},
							Summary: &dto.Summary{
								SampleCount: proto.Uint64(1),
								SampleSum:   proto.Float64(2),
								Quantile: []*dto.Quantile{
									{
										Quantile: proto.Float64(0.5),
										Value:    proto.Float64(0.7),
									},
									{
										Quantile: proto.Float64(1),
										Value:    proto.Float64(0.8),
									},
								},
								CreatedTimestamp: metricTimestamp,
							},
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("b"),
									Value: proto.String("c2"),
								},
							},
							Summary: &dto.Summary{
								SampleCount: proto.Uint64(2),
								SampleSum:   proto.Float64(3),
								Quantile: []*dto.Quantile{
									{
										Quantile: proto.Float64(0.5),
										Value:    proto.Float64(0.1),
									},
									{
										Quantile: proto.Float64(1),
										Value:    proto.Float64(0.2),
									},
								},
								CreatedTimestamp: metricTimestamp,
							},
						},
					},
				},
			},
		},

		// 18: Simple histogram with labels
		{
			in: `# TYPE foo histogram
# HELP foo help
foo_bucket{a="b", le="0.0"} 0
foo_bucket{a="b", le="1e-05"} 0
foo_bucket{a="b", le="0.0001"} 5
foo_bucket{a="b", le="0.1"} 8
foo_bucket{a="b", le="1.0"} 10
foo_bucket{a="b", le="10.0"} 11
foo_bucket{a="b", le="100000.0"} 11
foo_bucket{a="b", le="1e+06"} 15
foo_bucket{a="b", le="1e+23"} 16
foo_bucket{a="b", le="1.1e+23"} 17
foo_bucket{a="b", le="+Inf"} 17
foo_count{a="b"} 17
foo_sum{a="b"} 324789.3
foo_created{a="b"} 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Help: proto.String("help"),
					Type: dto.MetricType_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("a"),
									Value: proto.String("b"),
								},
							},
							Histogram: &dto.Histogram{
								SampleCount: proto.Uint64(17),
								SampleSum:   proto.Float64(324789.3),
								Bucket: []*dto.Bucket{
									{
										UpperBound:      proto.Float64(0.0),
										CumulativeCount: proto.Uint64(0),
									},
									{
										UpperBound:      proto.Float64(1e-05),
										CumulativeCount: proto.Uint64(0),
									},
									{
										UpperBound:      proto.Float64(0.0001),
										CumulativeCount: proto.Uint64(5),
									},
									{
										UpperBound:      proto.Float64(0.1),
										CumulativeCount: proto.Uint64(8),
									},
									{
										UpperBound:      proto.Float64(1),
										CumulativeCount: proto.Uint64(10),
									},
									{
										UpperBound:      proto.Float64(10.0),
										CumulativeCount: proto.Uint64(11),
									},
									{
										UpperBound:      proto.Float64(100000),
										CumulativeCount: proto.Uint64(11),
									},
									{
										UpperBound:      proto.Float64(1e+06),
										CumulativeCount: proto.Uint64(15),
									},
									{
										UpperBound:      proto.Float64(1e+23),
										CumulativeCount: proto.Uint64(16),
									},
									{
										UpperBound:      proto.Float64(1.1e+23),
										CumulativeCount: proto.Uint64(17),
									},
									{
										UpperBound:      proto.Float64(math.Inf(+1)),
										CumulativeCount: proto.Uint64(17),
									},
								},
								CreatedTimestamp: metricTimestamp,
							},
						},
					},
				},
			},
		},

		// 19: Simple histogram with exemplars
		{
			in: `# TYPE foo histogram
# HELP foo help
foo_bucket{a="b", le="0.0"} 0 # {l="1"} 0.5
foo_bucket{a="b", le="1e-05"} 0
foo_bucket{a="b", le="0.0001"} 5
foo_bucket{a="b", le="0.1"} 8
foo_bucket{a="b", le="1.0"} 10
foo_bucket{a="b", le="10.0"} 11
foo_bucket{a="b", le="100000.0"} 11
foo_bucket{a="b", le="1e+06"} 15 # {l="2"} 1
foo_bucket{a="b", le="1e+23"} 16
foo_bucket{a="b", le="1.1e+23"} 17
foo_bucket{a="b", le="+Inf"} 17
foo_count{a="b"} 17
foo_sum{a="b"} 324789.3
foo_created{a="b"} 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Help: proto.String("help"),
					Type: dto.MetricType_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("a"),
									Value: proto.String("b"),
								},
							},
							Histogram: &dto.Histogram{
								SampleCount: proto.Uint64(17),
								SampleSum:   proto.Float64(324789.3),
								Bucket: []*dto.Bucket{
									{
										UpperBound:      proto.Float64(0.0),
										CumulativeCount: proto.Uint64(0),
										Exemplar: &dto.Exemplar{
											Label: []*dto.LabelPair{
												{
													Name:  proto.String("l"),
													Value: proto.String("1"),
												},
											},
											Value: proto.Float64(0.5),
										},
									},
									{
										UpperBound:      proto.Float64(1e-05),
										CumulativeCount: proto.Uint64(0),
									},
									{
										UpperBound:      proto.Float64(0.0001),
										CumulativeCount: proto.Uint64(5),
									},
									{
										UpperBound:      proto.Float64(0.1),
										CumulativeCount: proto.Uint64(8),
									},
									{
										UpperBound:      proto.Float64(1),
										CumulativeCount: proto.Uint64(10),
									},
									{
										UpperBound:      proto.Float64(10.0),
										CumulativeCount: proto.Uint64(11),
									},
									{
										UpperBound:      proto.Float64(100000),
										CumulativeCount: proto.Uint64(11),
									},
									{
										UpperBound:      proto.Float64(1e+06),
										CumulativeCount: proto.Uint64(15),
										Exemplar: &dto.Exemplar{
											Label: []*dto.LabelPair{
												{
													Name:  proto.String("l"),
													Value: proto.String("2"),
												},
											},
											Value: proto.Float64(1),
										},
									},
									{
										UpperBound:      proto.Float64(1e+23),
										CumulativeCount: proto.Uint64(16),
									},
									{
										UpperBound:      proto.Float64(1.1e+23),
										CumulativeCount: proto.Uint64(17),
									},
									{
										UpperBound:      proto.Float64(math.Inf(+1)),
										CumulativeCount: proto.Uint64(17),
									},
								},
								CreatedTimestamp: metricTimestamp,
							},
						},
					},
				},
			},
		},

		// 20: Simple gaugehistogram
		{
			in: `# TYPE foo gaugehistogram
foo_bucket{le="0.01"} 20.0
foo_bucket{le="0.1"} 25.0
foo_bucket{le="1"} 34.0
foo_bucket{le="10"} 34.0
foo_bucket{le="+Inf"} 42.0
foo_gcount 42.0
foo_gsum 3289.3
foo_created 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						{
							Histogram: &dto.Histogram{
								SampleCount: proto.Uint64(42),
								SampleSum:   proto.Float64(3289.3),
								Bucket: []*dto.Bucket{
									{
										UpperBound:      proto.Float64(0.01),
										CumulativeCount: proto.Uint64(20),
									},
									{
										UpperBound:      proto.Float64(0.1),
										CumulativeCount: proto.Uint64(25),
									},
									{
										UpperBound:      proto.Float64(1),
										CumulativeCount: proto.Uint64(34),
									},
									{
										UpperBound:      proto.Float64(10),
										CumulativeCount: proto.Uint64(34),
									},
									{
										UpperBound:      proto.Float64(math.Inf(+1)),
										CumulativeCount: proto.Uint64(42),
									},
								},
								CreatedTimestamp: metricTimestamp,
							},
						},
					},
				},
			},
		},

		// 21: Simple gaugehistogram with labels and exemplars
		{
			in: `# TYPE foo gaugehistogram
foo_bucket{l="label", le="0.01"} 20.0 # {trace_id="a"} 0.5 123456
foo_bucket{l="label", le="0.1"} 25.0 # {trace_id="b"} 0.6
foo_bucket{l="label", le="1"} 34.0
foo_bucket{l="label", le="10"} 34.0
foo_bucket{l="label", le="+Inf"} 42.0
foo_gcount{l="label"} 42.0
foo_gsum{l="label"} 3289.3
foo_created{l="label"} 123456
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("l"),
									Value: proto.String("label"),
								},
							},
							Histogram: &dto.Histogram{
								SampleCount: proto.Uint64(42),
								SampleSum:   proto.Float64(3289.3),
								Bucket: []*dto.Bucket{
									{
										UpperBound:      proto.Float64(0.01),
										CumulativeCount: proto.Uint64(20),
										Exemplar: &dto.Exemplar{
											Label: []*dto.LabelPair{
												{
													Name:  proto.String("trace_id"),
													Value: proto.String("a"),
												},
											},
											Value:     proto.Float64(0.5),
											Timestamp: metricTimestamp,
										},
									},
									{
										UpperBound:      proto.Float64(0.1),
										CumulativeCount: proto.Uint64(25),
										Exemplar: &dto.Exemplar{
											Label: []*dto.LabelPair{
												{
													Name:  proto.String("trace_id"),
													Value: proto.String("b"),
												},
											},
											Value: proto.Float64(0.6),
										},
									},
									{
										UpperBound:      proto.Float64(1),
										CumulativeCount: proto.Uint64(34),
									},
									{
										UpperBound:      proto.Float64(10),
										CumulativeCount: proto.Uint64(34),
									},
									{
										UpperBound:      proto.Float64(math.Inf(+1)),
										CumulativeCount: proto.Uint64(42),
									},
								},
								CreatedTimestamp: metricTimestamp,
							},
						},
					},
				},
			},
		},

		// 22: Minimal case
		{
			in: `minimal_metric 1.234
another_metric -3e3 103948
# Even that:
no_labels{} 3
# HELP line for non-existing metric will be ignored.
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("minimal_metric"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{
							Untyped: &dto.Untyped{
								Value: proto.Float64(1.234),
							},
						},
					},
				},
				{
					Name: proto.String("another_metric"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{
							Untyped: &dto.Untyped{
								Value: proto.Float64(-3e3),
							},
							TimestampMs: proto.Int64(103948),
						},
					},
				},
				{
					Name: proto.String("no_labels"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{
							Untyped: &dto.Untyped{
								Value: proto.Float64(3),
							},
						},
					},
				},
			},
		},

		// 23: Counters with exemplars and created timestamp & gauges,
		//     docstrings, various whitespace, escape sequences.
		{
			in: `# A normal comment.
#
# TYPE name_seconds counter
# UNIT name_seconds seconds
name_seconds_total{labelname="val1",basename="basevalue"} NaN # {a="b"} 0.5
name_seconds_created{labelname="val1",basename="basevalue"} 123456789
name_seconds_total{labelname="val2",basename="base\"v\\al\nue"} 0.23 1234567890 # {a="c"} 1
# HELP name_seconds two-line\n doc  str\\ing

# HELP  name2  	doc str"ing 2
#    TYPE    name2 gauge
name2{labelname="val2"	,basename   =   "basevalue2"		} +Inf 54321
name2{ labelname = "val1" , }-Inf
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("name_seconds"),
					Unit: proto.String("seconds"),
					Help: proto.String("two-line\n doc  str\\ing"),
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("labelname"),
									Value: proto.String("val1"),
								},
								{
									Name:  proto.String("basename"),
									Value: proto.String("basevalue"),
								},
							},
							Counter: &dto.Counter{
								Value: proto.Float64(math.NaN()),
								Exemplar: &dto.Exemplar{
									Label: []*dto.LabelPair{
										{
											Name:  proto.String("a"),
											Value: proto.String("b"),
										},
									},
									Value: proto.Float64(0.5),
								},
								CreatedTimestamp: timestamppb.New(time.Unix(123456789, 600000000)),
							},
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("labelname"),
									Value: proto.String("val2"),
								},
								{
									Name:  proto.String("basename"),
									Value: proto.String("base\"v\\al\nue"),
								},
							},
							Counter: &dto.Counter{
								Value: proto.Float64(.23),
								Exemplar: &dto.Exemplar{
									Label: []*dto.LabelPair{
										{
											Name:  proto.String("a"),
											Value: proto.String("c"),
										},
									},
									Value: proto.Float64(1),
								},
							},
							TimestampMs: proto.Int64(1234567890),
						},
					},
				},
				{
					Name: proto.String("name2"),
					Help: proto.String("doc str\"ing 2"),
					Type: dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("labelname"),
									Value: proto.String("val2"),
								},
								{
									Name:  proto.String("basename"),
									Value: proto.String("basevalue2"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(math.Inf(+1)),
							},
							TimestampMs: proto.Int64(54321),
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("labelname"),
									Value: proto.String("val1"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(math.Inf(-1)),
							},
						},
					},
				},
			},
		},

		// 24: The evil summary with created timestamp,
		//    mixed with other types and funny comments.
		{
			in: `# TYPE my_summary summary
my_summary{n1="val1",quantile="0.5"} 110
decoy -1 -2
my_summary{n1="val1",quantile="0.9"} 140 1
my_summary_count{n1="val1"} 42
# Latest timestamp wins in case of a summary.
my_summary_sum{n1="val1"} 4711 2
my_summary_created{n1="val1"} 123456789
fake_sum{n1="val1"} 2001
# TYPE another_summary summary
another_summary_count{n2="val2",n1="val1"} 20
my_summary_count{n2="val2",n1="val1"} 5 5
another_summary{n1="val1",n2="val2",quantile=".3"} -1.2
my_summary_sum{n1="val2"} 08 15
my_summary{n1="val3", quantile="0.2"} 4711
my_summary{n1="val1",n2="val2",quantile="-12.34",} NaN
# some
# funny comments
# HELP
# HELP
# HELP my_summary
# HELP my_summary
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("fake_sum"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("n1"),
									Value: proto.String("val1"),
								},
							},
							Untyped: &dto.Untyped{
								Value: proto.Float64(2001),
							},
						},
					},
				},
				{
					Name: proto.String("decoy"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{
							Untyped: &dto.Untyped{
								Value: proto.Float64(-1),
							},
							TimestampMs: proto.Int64(-2),
						},
					},
				},
				{
					Name: proto.String("my_summary"),
					Type: dto.MetricType_SUMMARY.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("n1"),
									Value: proto.String("val1"),
								},
							},
							Summary: &dto.Summary{
								SampleCount: proto.Uint64(42),
								SampleSum:   proto.Float64(4711),
								Quantile: []*dto.Quantile{
									{
										Quantile: proto.Float64(0.5),
										Value:    proto.Float64(110),
									},
									{
										Quantile: proto.Float64(0.9),
										Value:    proto.Float64(140),
									},
								},
								CreatedTimestamp: timestamppb.New(time.Unix(123456789, 600000000)),
							},
							TimestampMs: proto.Int64(2),
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("n2"),
									Value: proto.String("val2"),
								},
								{
									Name:  proto.String("n1"),
									Value: proto.String("val1"),
								},
							},
							Summary: &dto.Summary{
								SampleCount: proto.Uint64(5),
								Quantile: []*dto.Quantile{
									{
										Quantile: proto.Float64(-12.34),
										Value:    proto.Float64(math.NaN()),
									},
								},
							},
							TimestampMs: proto.Int64(5),
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("n1"),
									Value: proto.String("val2"),
								},
							},
							Summary: &dto.Summary{
								SampleSum: proto.Float64(8),
							},
							TimestampMs: proto.Int64(15),
						},
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("n1"),
									Value: proto.String("val3"),
								},
							},
							Summary: &dto.Summary{
								Quantile: []*dto.Quantile{
									{
										Quantile: proto.Float64(0.2),
										Value:    proto.Float64(4711),
									},
								},
							},
						},
					},
				},
				{
					Name: proto.String("another_summary"),
					Type: dto.MetricType_SUMMARY.Enum(),
					Metric: []*dto.Metric{
						{
							Label: []*dto.LabelPair{
								{
									Name:  proto.String("n2"),
									Value: proto.String("val2"),
								},
								{
									Name:  proto.String("n1"),
									Value: proto.String("val1"),
								},
							},
							Summary: &dto.Summary{
								SampleCount: proto.Uint64(20),
								Quantile: []*dto.Quantile{
									{
										Quantile: proto.Float64(0.3),
										Value:    proto.Float64(-1.2),
									},
								},
							},
						},
					},
				},
			},
		},

		// 25: The histogram with created timestamp and exemplars.
		{
			in: `# HELP request_duration_microseconds The response latency.
# TYPE request_duration_microseconds histogram
request_duration_microseconds_bucket{le="100"} 123 # {trace_id="a"} 0.67
request_duration_microseconds_bucket{le="120"} 412 # {trace_id="b"} 1 123456
request_duration_microseconds_bucket{le="144"} 592
request_duration_microseconds_bucket{le="172.8"} 1524
request_duration_microseconds_bucket{le="+Inf"} 2693 # {} 2
request_duration_microseconds_sum 1.7560473e+06
request_duration_microseconds_count 2693
request_duration_microseconds_created 123456789.123
# EOF
`,
			out: []*dto.MetricFamily{
				{
					Name: proto.String("request_duration_microseconds"),
					Help: proto.String("The response latency."),
					Type: dto.MetricType_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						{
							Histogram: &dto.Histogram{
								SampleCount:      proto.Uint64(2693),
								SampleSum:        proto.Float64(1756047.3),
								CreatedTimestamp: timestamppb.New(time.Unix(123456789, 600000000)),
								Bucket: []*dto.Bucket{
									{
										UpperBound:      proto.Float64(100),
										CumulativeCount: proto.Uint64(123),
										Exemplar: &dto.Exemplar{
											Label: []*dto.LabelPair{
												{
													Name:  proto.String("trace_id"),
													Value: proto.String("a"),
												},
											},
											Value: proto.Float64(0.67),
										},
									},
									{
										UpperBound:      proto.Float64(120),
										CumulativeCount: proto.Uint64(412),
										Exemplar: &dto.Exemplar{
											Label: []*dto.LabelPair{
												{
													Name:  proto.String("trace_id"),
													Value: proto.String("b"),
												},
											},
											Value:     proto.Float64(1),
											Timestamp: metricTimestamp,
										},
									},
									{
										UpperBound:      proto.Float64(144),
										CumulativeCount: proto.Uint64(592),
									},
									{
										UpperBound:      proto.Float64(172.8),
										CumulativeCount: proto.Uint64(1524),
									},
									{
										UpperBound:      proto.Float64(math.Inf(+1)),
										CumulativeCount: proto.Uint64(2693),
										Exemplar: &dto.Exemplar{
											Label: []*dto.LabelPair{},
											Value: proto.Float64(2),
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for i, scenario := range scenarios {
		out, err := omParser.OpenMetricsToMetricFamilies(strings.NewReader(scenario.in))
		if err != nil {
			t.Errorf("%d. error: %s", i, err)
			continue
		}
		if expected, got := len(scenario.out), len(out); expected != got {
			t.Errorf(
				"%d. expected %d MetricFamilies, got %d",
				i, expected, got,
			)
		}
		for _, expected := range scenario.out {
			got, ok := out[expected.GetName()]
			if !ok {
				t.Errorf(
					"%d. expected MetricFamily %q, found none",
					i, expected.GetName(),
				)
				continue
			}
			if expected.String() != got.String() {
				t.Errorf(
					"%d. expected MetricFamily %s, got %s",
					i, expected, got,
				)
			}
		}
	}
}

func testOpenMetricParseError(t testing.TB) {
	scenarios := []struct {
		in  string
		err string
	}{
		// 0: No new-line at end of input.
		{
			in: `bla 3.14
blubber 42`,
			err: "openmetrics format parsing error in line 2: unexpected end of input stream",
		},
		// 1: Invalid escape sequence in label value.
		{
			in:  `metric{label="\t"} 3.14`,
			err: "openmetrics format parsing error in line 1: invalid escape sequence",
		},
		// 2: Newline in label value.
		{
			in: `metric{label="new
line"} 3.14
`,
			err: `openmetrics format parsing error in line 1: label value "new" contains unescaped new-line`,
		},
		// 3:
		{
			in:  `metric{@="bla"} 3.14`,
			err: "openmetrics format parsing error in line 1: invalid label name for metric",
		},
		// 4:
		{
			in:  `metric{__name__="bla"} 3.14`,
			err: `openmetrics format parsing error in line 1: label name "__name__" is reserved`,
		},
		// 5:
		{
			in:  `metric{label+="bla"} 3.14`,
			err: "openmetrics format parsing error in line 1: expected '=' after label name",
		},
		// 6:
		{
			in:  `metric{label=bla} 3.14`,
			err: "openmetrics format parsing error in line 1: expected '\"' at start of label value",
		},
		// 7:
		{
			in: `# TYPE metric summary
metric{quantile="bla"} 3.14
`,
			err: "openmetrics format parsing error in line 2: expected float as value for 'quantile' label",
		},
		// 8:
		{
			in:  `metric{label="bla"+} 3.14`,
			err: "openmetrics format parsing error in line 1: unexpected end of label value",
		},
		// 9:
		{
			in: `metric{label="bla"} 3.14 2.72
			`,
			err: "openmetrics format parsing error in line 1: expected integer as timestamp",
		},
		// 10:
		{
			in: `metric{label="bla"} 3.14 2 3
			`,
			err: "openmetrics format parsing error in line 1: spurious string after timestamp",
		},
		// 11:
		{
			in: `metric{label="bla"} blubb
			`,
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 12:
		{
			in: `# HELP metric one
# HELP metric two
`,
			err: "openmetrics format parsing error in line 2: second HELP line for metric name",
		},
		// 13:
		{
			in: `# TYPE metric counter
# TYPE metric untyped
`,
			err: `openmetrics format parsing error in line 2: second TYPE line for metric name "metric", or TYPE reported after samples`,
		},
		// 14:
		{
			in: `metric 4.12
# TYPE metric counter
`,
			err: `openmetrics format parsing error in line 2: second TYPE line for metric name "metric", or TYPE reported after samples`,
		},
		// 15:
		{
			in: `# TYPE metric bla
`,
			err: "openmetrics format parsing error in line 1: unknown metric type",
		},
		// 16:
		{
			in: `# TYPE met-ric
`,
			err: "openmetrics format parsing error in line 1: invalid metric name in comment",
		},
		// 17:
		{
			in:  `@invalidmetric{label="bla"} 3.14 2`,
			err: "openmetrics format parsing error in line 1: '@' is not a valid start token",
		},
		// 18:
		{
			in:  `{label="bla"} 3.14 2`,
			err: "openmetrics format parsing error in line 1: '{' is not a valid start token",
		},
		// 19:
		{
			in: `# TYPE metric histogram
metric_bucket{le="bla"} 3.14
			`,
			err: "openmetrics format parsing error in line 2: expected float as value for 'le' label",
		},
		// 20: Invalid UTF-8 in label value.
		{
			in:  "metric{l=\"\xbd\"} 3.14\n",
			err: "openmetrics format parsing error in line 1: invalid label value \"\\xbd\"",
		},
		// 21: Go 1.13 sometimes allows underscores in numbers.
		{
			in:  "foo 1_2\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 22: Go 1.13 supports hex floating point.
		{
			in:  "foo 0x1p-3\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 23: Check for various other literals variants, just in case.
		{
			in:  "foo 0x1P-3\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 24:
		{
			in:  "foo 0B1\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 25:
		{
			in:  "foo 0O1\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 26:
		{
			in:  "foo 0X1\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 27:
		{
			in:  "foo 0x1\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 28:
		{
			in:  "foo 0b1\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 29:
		{
			in:  "foo 0o1\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 30:
		{
			in:  "foo 0x1\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 31:
		{
			in:  "foo 0x1\n",
			err: "openmetrics format parsing error in line 1: expected float as value",
		},
		// 32: Check histogram label.
		{
			in: `# TYPE metric histogram
metric_bucket{le="0x1p-3"} 3.14
`,
			err: "openmetrics format parsing error in line 2: expected float as value for 'le' label",
		},
		// 33: Check quantile label.
		{
			in: `# TYPE metric summary
metric{quantile="0x1p-3"} 3.14
`,
			err: "openmetrics format parsing error in line 2: expected float as value for 'quantile' label",
		},
		// 34: Check duplicate label.
		{
			in:  `metric{label="bla",label="bla"} 3.14`,
			err: "openmetrics format parsing error in line 1: duplicate label names for metric",
		},
		// 35: Exemplars in gauge metric.
		{
			in: `# TYPE metric gauge
metric{le="0x1p-3"} 3.14 # {} 1
`,
			err: `openmetrics format parsing error in line 2: unexpected exemplar for metric name "metric" type gauge`,
		},
		// 36: Exemplars in summary metric.
		{
			in: `# TYPE metric summary
metric{quantile="0.1"} 3.14 # {} 1
`,
			err: `openmetrics format parsing error in line 2: unexpected exemplar for metric name "metric" type summary`,
		},
		// 37: Counter ends without '_total'
		{
			in: `# TYPE metric counter
metric{t="1"} 3.14
`,
			err: `openmetrics format parsing error in line 2: expected '_total' or '_created' as counter metric name suffix, got metric name "metric"`,
		},
		// 38: metrics ends without unit
		{
			in: `# TYPE metric counter
# UNIT metric seconds
`,
			err: `openmetrics format parsing error in line 2: expected unit as metric name suffix, found metric "metric"`,
		},

		// 39: metrics ends without EOF
		{
			in: `# TYPE metric_seconds counter
# UNIT metric_seconds seconds
`,
			err: `openmetrics format parsing error in line 3: expected EOF keyword at the end`,
		},

		// 40: line after EOF
		{
			in: `# EOF
# TYPE metric counter
`,
			err: `openmetrics format parsing error in line 2: unexpected line after EOF, got '#'`,
		},

		// 41: invalid start token
		{
			in: `# TYPE metric_seconds counter
				# UNIT metric_seconds seconds
`,
			err: `openmetrics format parsing error in line 2: '\t' is not a valid start token`,
		},
	}
	var omParser OpenMetricsParser

	for i, scenario := range scenarios {
		_, err := omParser.OpenMetricsToMetricFamilies(strings.NewReader(scenario.in))
		if err == nil {
			t.Errorf("%d. expected error, got nil", i)
			continue
		}
		if expected, got := scenario.err, err.Error(); strings.Index(got, expected) != 0 {
			t.Errorf(
				"%d. expected error starting with %q, got %q",
				i, expected, got,
			)
		}
	}
}

func TestOpenMetricsParse(t *testing.T) {
	testOpenMetricsParse(t)
}

func BenchmarkOpenMetricParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testOpenMetricsParse(b)
	}
}

func TestOpenMetricParseError(t *testing.T) {
	testOpenMetricParseError(t)
}

func BenchmarkOpenMetricParseError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testOpenMetricParseError(b)
	}
}
