// Copyright 2021 The Prometheus Authors
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
	"encoding/json"
	"math"
	"strings"
	"testing"
)

var openMetricParser = OpenMetricsParser{}

func testOpenMetricsTextParse(t testing.TB) {
	var scenarios = []struct {
		in  string
		out []*OpenMetricFamily
	}{
		// 0: Simple Counter.
		{
			in: `# TYPE a counter
# HELP a help
a_total 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "a_total", Value: 1}},
				},
			},
		},
		// 1: uint64 Counter.
		{
			in: `# TYPE a counter
# HELP a help
a_total 9223372036854775808
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "a_total", Value: 9223372036854775808}},
				},
			},
		},
		// 2: Simple Gauge with unit.
		{
			in: `# TYPE a_seconds gauge
# UNIT seconds
# HELP a_seconds help
a_seconds 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a_seconds"),
					Help:       String("help"),
					Unit:       String("seconds"),
					MetricType: String(OpenMetricTypeGauge),
					Samples:    []*Sample{&Sample{Name: "a_seconds", Value: 1}},
				},
			},
		},
		// 3: Float Gauge.
		{
			in: `# TYPE a gauge
# HELP a help
a 1.2
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGauge),
					Samples:    []*Sample{&Sample{Name: "a", Value: 1.2}},
				},
			},
		},
		// 4: Leading zeros simple gauge.
		{
			in: `# TYPE a gauge
# HELP a help
a 0000000000000000000000000001
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGauge),
					Samples:    []*Sample{&Sample{Name: "a", Value: 1}},
				},
			},
		},
		// 5: Leading zeros float gauge.
		{
			in: `# TYPE a gauge
# HELP a help
a 0000000000000000000000000000000000000000001.2e-1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGauge),
					Samples:    []*Sample{&Sample{Name: "a", Value: .12}},
				},
			},
		},
		// 6: NaN gauge.
		{
			in: `# TYPE a gauge
# HELP a help
a NaN
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGauge),
					Samples:    []*Sample{&Sample{Name: "a", Value: Value(math.NaN())}},
				},
			},
		},
		// 7: Simple gauge.
		{
			in: `# TYPE a gauge
# HELP a help
a NaN
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGauge),
					Samples:    []*Sample{&Sample{Name: "a", Value: Value(math.NaN())}},
				},
			},
		},
		// 8: Simple summary.
		{
			in: `# TYPE a summary
# HELP a help
a_count 1
a_sum 2
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeSummary),
					Samples: []*Sample{&Sample{Name: "a_count", Value: 1},
						&Sample{Name: "a_sum", Value: 2}},
				},
			},
		},
		// 9: Summary with quantile.
		{
			in: `# TYPE a summary
# HELP a help
a_count 1
a_sum 2
a{quantile="0.5"} 0.7
a{quantile="1"} 0.8
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeSummary),
					Samples: []*Sample{&Sample{Name: "a_count", Value: 1},
						&Sample{Name: "a_sum", Value: 2},
						&Sample{Name: "a", Value: 0.7, Labels: map[string]string{"quantile": "0.5"}},
						&Sample{Name: "a", Value: 0.8, Labels: map[string]string{"quantile": "1"}},
					},
				},
			},
		},
		// 10: Simple Histogram.
		{
			in: `# TYPE a histogram
# HELP a help
a_bucket{le="1.0"} 0
a_bucket{le="+Inf"} 3
a_count 3
a_sum 2
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeHistogram),
					Samples: []*Sample{&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1.0"}},
						&Sample{Name: "a_bucket", Value: 3, Labels: map[string]string{"le": "+Inf"}},
						&Sample{Name: "a_count", Value: 3},
						&Sample{Name: "a_sum", Value: 2},
					},
				},
			},
		},

		// 11: Non-canonical histogram.
		{
			in: `# TYPE a histogram
# HELP a help
a_bucket{le="0"} 0
a_bucket{le="0.00000000001"} 0
a_bucket{le="0.0000000001"} 0
a_bucket{le="1e-04"} 0
a_bucket{le="1.1e-4"} 0
a_bucket{le="1.1e-3"} 0
a_bucket{le="1.1e-2"} 0
a_bucket{le="1"} 0
a_bucket{le="1e+05"} 0
a_bucket{le="10000000000"} 0
a_bucket{le="100000000000.0"} 0
a_bucket{le="+Inf"} 3
a_count 3
a_sum 2
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeHistogram),
					Samples: []*Sample{&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "0"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "0.00000000001"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "0.0000000001"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1e-04"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1.1e-4"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1.1e-3"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1.1e-2"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1e+05"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "10000000000"}},
						&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "100000000000.0"}},
						&Sample{Name: "a_bucket", Value: 3, Labels: map[string]string{"le": "+Inf"}},
						&Sample{Name: "a_count", Value: 3},
						&Sample{Name: "a_sum", Value: 2},
					},
				},
			},
		},
		// 12: Negative histogram.
		{
			in: `# TYPE a histogram
# HELP a help
a_bucket{le="-1.0"} 0
a_bucket{le="1.0"} 1
a_bucket{le="+Inf"} 3
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeHistogram),
					Samples: []*Sample{&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "-1.0"}},
						&Sample{Name: "a_bucket", Value: 1, Labels: map[string]string{"le": "1.0"}},
						&Sample{Name: "a_bucket", Value: 3, Labels: map[string]string{"le": "+Inf"}},
					},
				},
			},
		},
		// 13: Histogram has exemplars.
		{
			in: `# TYPE a histogram
# HELP a help
a_bucket{le="1.0"} 0 # {a="b"} 0.5
a_bucket{le="2.0"} 2 # {a="c"} 0.5
a_bucket{le="+Inf"} 3 # {a="2345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678"} 4 123
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeHistogram),
					Samples: []*Sample{&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1.0"}, Exemplar: &Exemplar{Value: 0.5, Labels: map[string]string{"a": "b"}}},
						&Sample{Name: "a_bucket", Value: 2, Labels: map[string]string{"le": "2.0"}, Exemplar: &Exemplar{Value: 0.5, Labels: map[string]string{"a": "c"}}},
						&Sample{Name: "a_bucket", Value: 3, Labels: map[string]string{"le": "+Inf"}, Exemplar: &Exemplar{Value: 4, Labels: map[string]string{"a": "2345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678"}, Timestamp: &Timestamp{Sec: 123}}},
					},
				},
			},
		},
		// 14: Simple gaugehistogram.
		{
			in: `# TYPE a gaugehistogram
# HELP a help
a_bucket{le="1.0"} 0
a_bucket{le="+Inf"} 3
a_gcount 3
a_gsum 2
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGaugeHistogram),
					Samples: []*Sample{&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1.0"}},
						&Sample{Name: "a_bucket", Value: 3, Labels: map[string]string{"le": "+Inf"}},
						&Sample{Name: "a_gcount", Value: 3},
						&Sample{Name: "a_gsum", Value: 2},
					},
				},
			},
		},
		// 15: Negative gaugehistogram.
		{
			in: `# TYPE a gaugehistogram
# HELP a help
a_bucket{le="-1.0"} 1
a_bucket{le="1.0"} 2
a_bucket{le="+Inf"} 3
a_gcount 3
a_gsum -5
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGaugeHistogram),
					Samples: []*Sample{&Sample{Name: "a_bucket", Value: 1, Labels: map[string]string{"le": "-1.0"}},
						&Sample{Name: "a_bucket", Value: 2, Labels: map[string]string{"le": "1.0"}},
						&Sample{Name: "a_bucket", Value: 3, Labels: map[string]string{"le": "+Inf"}},
						&Sample{Name: "a_gcount", Value: 3},
						&Sample{Name: "a_gsum", Value: -5},
					},
				},
			},
		},
		// 16: Gaugehistogram has exemplars.
		{
			in: `# TYPE a gaugehistogram
# HELP a help
a_bucket{le="1.0"} 0 123 # {a="b"} 0.5
a_bucket{le="2.0"} 2 123 # {a="c"} 0.5
a_bucket{le="+Inf"} 3 123 # {a="d"} 4 123
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGaugeHistogram),
					Samples: []*Sample{&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1.0"}, Timestamp: &Timestamp{Sec: 123}, Exemplar: &Exemplar{Value: 0.5, Labels: map[string]string{"a": "b"}}},
						&Sample{Name: "a_bucket", Value: 2, Labels: map[string]string{"le": "2.0"}, Timestamp: &Timestamp{Sec: 123}, Exemplar: &Exemplar{Value: 0.5, Labels: map[string]string{"a": "c"}}},
						&Sample{Name: "a_bucket", Value: 3, Labels: map[string]string{"le": "+Inf"}, Timestamp: &Timestamp{Sec: 123}, Exemplar: &Exemplar{Value: 4, Labels: map[string]string{"a": "d"}, Timestamp: &Timestamp{Sec: 123}}},
					},
				},
			},
		},
		// 17: Counter has exemplars.
		{
			in: `# TYPE a counter
# HELP a help
a_total 0 123 # {a="b"} 0.5
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "a_total", Value: 0, Timestamp: &Timestamp{Sec: 123}, Exemplar: &Exemplar{Value: 0.5, Labels: map[string]string{"a": "b"}}}},
				},
			},
		},

		// 18: Counter empty bracket.
		{
			in: `# TYPE a counter
# HELP a help
a_total{} 0 123 # {a="b"} 0.5
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "a_total", Value: 0, Timestamp: &Timestamp{Sec: 123}, Exemplar: &Exemplar{Value: 0.5, Labels: map[string]string{"a": "b"}}}},
				},
			},
		},
		// 19: Simple Info
		{
			in: `# TYPE a info
# HELP a help
a_info{foo="bar"} 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeInfo),
					Samples:    []*Sample{&Sample{Name: "a_info", Value: 1, Labels: map[string]string{"foo": "bar"}}},
				},
			},
		},
		// 20: Info has timestamps.
		{
			in: `# TYPE a info
# HELP a help
a_info{a="1",foo="bar"} 1 1
a_info{a="2",foo="bar"} 1 0
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeInfo),
					Samples: []*Sample{&Sample{Name: "a_info", Value: 1, Timestamp: &Timestamp{Sec: 1}, Labels: map[string]string{"foo": "bar", "a": "1"}},
						&Sample{Name: "a_info", Value: 1, Timestamp: &Timestamp{Sec: 0}, Labels: map[string]string{"foo": "bar", "a": "2"}},
					},
				},
			},
		},

		// 21: Simple stateset.
		{
			in: `# TYPE a stateset
# HELP a help
a{a="bar"} 0
a{a="foo"} 1.0
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeStateset),
					Samples: []*Sample{&Sample{Name: "a", Value: 0, Labels: map[string]string{"a": "bar"}},
						&Sample{Name: "a", Value: 1.0, Labels: map[string]string{"a": "foo"}},
					},
				},
			},
		},
		// 22: Timestamp <1ns.
		{
			in: `# TYPE a gauge
# HELP a help
a{a="1",foo="bar"} 3 0.0000000010
a{a="1",foo="bar"} 2 0.0000000001
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGauge),
					Samples: []*Sample{&Sample{Name: "a", Value: 3, Labels: map[string]string{"a": "1", "foo": "bar"}, Timestamp: &Timestamp{NSec: 1}},
						&Sample{Name: "a", Value: 2, Labels: map[string]string{"a": "1", "foo": "bar"}, Timestamp: &Timestamp{}},
					},
				},
			},
		},
		// 23: No metadata.
		{
			in: `a 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					MetricType: String(OpenMetricTypeUnknown),
					Samples:    []*Sample{&Sample{Name: "a", Value: 1}},
				},
			},
		},
		// 24: Untyped.
		{
			in: `# HELP redis_connected_clients Redis connected clients
# TYPE redis_connected_clients unknown
redis_connected_clients{instance="rough-snowflake-web",port="6380"} 10.0
redis_connected_clients{instance="rough-snowflake-web",port="6381"} 12.0
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("redis_connected_clients"),
					Help:       String("Redis connected clients"),
					MetricType: String(OpenMetricTypeUnknown),
					Samples: []*Sample{&Sample{Name: "redis_connected_clients", Value: 10.0, Labels: map[string]string{"port": "6380", "instance": "rough-snowflake-web"}},
						&Sample{Name: "redis_connected_clients", Value: 12.0, Labels: map[string]string{"port": "6381", "instance": "rough-snowflake-web"}},
					},
				},
			},
		},

		// 25: Type help switched.
		{
			in: `# HELP a help
# TYPE a counter
a_total 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "a_total", Value: 1}},
				},
			},
		},

		// 26: Labels with curly braces.
		{
			in: `# TYPE a counter
# HELP a help
a_total{foo="bar",bar="b{a}z"} 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "a_total", Value: 1, Labels: map[string]string{"foo": "bar", "bar": "b{a}z"}}},
				},
			},
		},
		// 27: Empty help.
		{
			in: `# TYPE a counter
# HELP a 
a_total 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String(""),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "a_total", Value: 1}},
				},
			},
		},
		// 28: Labels and infinite.
		{
			in: `# TYPE a gauge
# HELP a help
a{foo="bar"} +Inf
a{foo="baz"} -Inf
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeGauge),
					Samples: []*Sample{&Sample{Name: "a", Value: Value(math.Inf(1)), Labels: map[string]string{"foo": "bar"}},
						&Sample{Name: "a", Value: Value(math.Inf(-1)), Labels: map[string]string{"foo": "baz"}},
					},
				},
			},
		},
		// 29: Labels and infinite.
		{
			in: `# TYPE a counter
# HELP a help
a_total{} 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "a_total", Value: Value(1)}},
				},
			},
		},
		// 30: NaN.
		{
			in: `a NaN
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					MetricType: String(OpenMetricTypeUnknown),
					Samples:    []*Sample{&Sample{Name: "a", Value: Value(math.NaN())}},
				},
			},
		},
		// 31: Empty label values.
		{
			in: `# TYPE a counter
# HELP a help
a_total{foo="bar"} 1
a_total{foo=""} 2
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples: []*Sample{&Sample{Name: "a_total", Value: 1, Labels: map[string]string{"foo": "bar"}},
						&Sample{Name: "a_total", Value: 2, Labels: map[string]string{"foo": ""}},
					},
				},
			},
		},
		// 32: Counters & gauges, docstrings, various whitespace, escape sequences.
		{
			in: `
# A normal comment.
#
# TYPE name counter
name_total{labelname="val1",basename="basevalue"} NaN
name_total {labelname="val2",basename="base\"v\\al\nue"} 0.23 1234567890
# HELP name two-line\n doc  str\\ing

 # HELP  name2  	doc str"ing 2
  #    TYPE    name2 gauge
name2{labelname="val2"	,basename   =   "basevalue2"		} +Inf 54321
name2{ labelname = "val1" , }-Inf
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("name"),
					Help:       String("two-line\n doc  str\\ing"),
					MetricType: String(OpenMetricTypeCounter),
					Samples: []*Sample{&Sample{Name: "name_total", Value: Value(math.NaN()), Labels: map[string]string{"labelname": "val1", "basename": "basevalue"}},
						&Sample{Name: "name_total", Value: 0.23, Labels: map[string]string{"labelname": "val2", "basename": "base\"v\\al\nue"}, Timestamp: &Timestamp{Sec: 1234567890}},
					},
				},
				&OpenMetricFamily{
					Name:       String("name2"),
					Help:       String("doc str\"ing 2"),
					MetricType: String(OpenMetricTypeGauge),
					Samples: []*Sample{&Sample{Name: "name2", Value: Value(math.Inf(1)), Labels: map[string]string{"labelname": "val2", "basename": "basevalue2"}, Timestamp: &Timestamp{Sec: 54321}},
						&Sample{Name: "name2", Value: Value(math.Inf(-1)), Labels: map[string]string{"labelname": "val1"}},
					},
				},
			},
		},
		// 33: Timestamps.
		{
			in: `# TYPE a counter
# HELP a help
a_total{foo="1"} 1 000
a_total{foo="2"} 1 0.0
a_total{foo="3"} 1 1.1
a_total{foo="4"} 1 12345678901234.1234567890
# TYPE b counter
# HELP b help
b_total 2 1234567890
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples: []*Sample{&Sample{Name: "a_total", Value: 1, Labels: map[string]string{"foo": "1"}, Timestamp: &Timestamp{}},
						&Sample{Name: "a_total", Value: 1, Labels: map[string]string{"foo": "2"}, Timestamp: &Timestamp{}},
						&Sample{Name: "a_total", Value: 1, Labels: map[string]string{"foo": "3"}, Timestamp: &Timestamp{Sec: 1, NSec: 100000000}},
						&Sample{Name: "a_total", Value: 1, Labels: map[string]string{"foo": "4"}, Timestamp: &Timestamp{Sec: 12345678901234, NSec: 123456789}},
					},
				},
				&OpenMetricFamily{
					Name:       String("b"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "b_total", Value: 2, Timestamp: &Timestamp{Sec: 1234567890}}},
				},
			},
		},
		// 34: Hash in label values.
		{
			in: `# TYPE a counter
# HELP a help
a_total{foo="foo # bar"} 1
a_total{foo="} foo # bar # "} 1
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeCounter),
					Samples: []*Sample{&Sample{Name: "a_total", Value: 1, Labels: map[string]string{"foo": "foo # bar"}},
						&Sample{Name: "a_total", Value: 1, Labels: map[string]string{"foo": "} foo # bar # "}},
					},
				},
			},
		},
		// 35: Exemplars with hash in label values.
		{
			in: `# TYPE a histogram
# HELP a help
a_bucket{le="1.0",foo="bar # "} 0 # {a="b",foo="bar # bar"} 0.5
a_bucket{le="2.0",foo="bar # "} 2 # {a="c",foo="bar # bar"} 0.5
a_bucket{le="+Inf",foo="bar # "} 3 # {a="d",foo="bar # bar"} 4
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("a"),
					Help:       String("help"),
					MetricType: String(OpenMetricTypeHistogram),
					Samples: []*Sample{&Sample{Name: "a_bucket", Value: 0, Labels: map[string]string{"le": "1.0", "foo": "bar # "}, Exemplar: &Exemplar{Value: 0.5, Labels: map[string]string{"a": "b", "foo": "bar # bar"}}},
						&Sample{Name: "a_bucket", Value: 2, Labels: map[string]string{"le": "2.0", "foo": "bar # "}, Exemplar: &Exemplar{Value: 0.5, Labels: map[string]string{"a": "c", "foo": "bar # bar"}}},
						&Sample{Name: "a_bucket", Value: 3, Labels: map[string]string{"le": "+Inf", "foo": "bar # "}, Exemplar: &Exemplar{Value: 4, Labels: map[string]string{"a": "d", "foo": "bar # bar"}}},
					},
				},
			},
		},
		// 36: The evil summary, mixed with other types and funny comments.
		{
			in: `
# TYPE my_summary summary
my_summary{n1="val1",quantile="0.5"} 110
decoy -1 -2
my_summary{n1="val1",quantile="0.9"} 140 1
my_summary_count{n1="val1"} 42
# Latest timestamp wins in case of a summary.
my_summary_sum{n1="val1"} 4711 2
fake_sum{n1="val1"} 2001
# TYPE another_summary summary
another_summary_count{n2="val2",n1="val1"} 20
my_summary_count{n2="val2",n1="val1"} 5 5
another_summary{n1="val1",n2="val2",quantile=".3"} -1.2
my_summary_sum{n1="val2"} 08 15
my_summary{n1="val3", quantile="0.2"} 4711
  my_summary{n1="val1",n2="val2",quantile="0.99",} NaN
# some
# funny comments
# HELP 
# HELP
# HELP my_summary
# HELP my_summary 
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("my_summary"),
					Help:       String(""),
					MetricType: String(OpenMetricTypeSummary),
					Samples: []*Sample{&Sample{Name: "my_summary", Value: 110, Labels: map[string]string{"n1": "val1", "quantile": "0.5"}},
						&Sample{Name: "my_summary", Value: 140, Labels: map[string]string{"n1": "val1", "quantile": "0.9"}, Timestamp: &Timestamp{Sec: 1}},
						&Sample{Name: "my_summary_count", Value: 42, Labels: map[string]string{"n1": "val1"}},
						&Sample{Name: "my_summary_sum", Value: 4711, Labels: map[string]string{"n1": "val1"}, Timestamp: &Timestamp{Sec: 2}},
						&Sample{Name: "my_summary_count", Value: 5, Labels: map[string]string{"n1": "val1", "n2": "val2"}, Timestamp: &Timestamp{Sec: 5}},
						&Sample{Name: "my_summary_sum", Value: 8, Labels: map[string]string{"n1": "val2"}, Timestamp: &Timestamp{Sec: 15}},
						&Sample{Name: "my_summary", Value: 4711, Labels: map[string]string{"n1": "val3", "quantile": "0.2"}},
						&Sample{Name: "my_summary", Value: Value(math.NaN()), Labels: map[string]string{"n1": "val1", "n2": "val2", "quantile": "0.99"}},
					},
				},
				&OpenMetricFamily{
					Name:       String("decoy"),
					Help:       String(""),
					MetricType: String(OpenMetricTypeUnknown),
					Samples:    []*Sample{&Sample{Name: "decoy", Value: -1, Timestamp: &Timestamp{Sec: -2}}},
				},
				&OpenMetricFamily{
					Name:       String("fake_sum"),
					Help:       String(""),
					MetricType: String(OpenMetricTypeUnknown),
					Samples:    []*Sample{&Sample{Name: "fake_sum", Value: 2001, Labels: map[string]string{"n1": "val1"}}},
				},
				&OpenMetricFamily{
					Name:       String("another_summary"),
					Help:       String(""),
					MetricType: String(OpenMetricTypeSummary),
					Samples: []*Sample{&Sample{Name: "another_summary_count", Value: 20, Labels: map[string]string{"n2": "val2", "n1": "val1"}},
						&Sample{Name: "another_summary", Value: -1.2, Labels: map[string]string{"n1": "val1", "n2": "val2", "quantile": ".3"}},
					},
				},
			},
		},
		// 37: The RoundTrip.
		{
			in: `# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0.0"} 0.013300656000000001
go_gc_duration_seconds{quantile="0.25"} 0.013638736
go_gc_duration_seconds{quantile="0.5"} 0.013759906
go_gc_duration_seconds{quantile="0.75"} 0.013962066
go_gc_duration_seconds{quantile="1.0"} 0.021383540000000003
go_gc_duration_seconds_sum 56.12904785
go_gc_duration_seconds_count 7476.0
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 166.0
# HELP prometheus_local_storage_indexing_batch_duration_milliseconds Quantiles for batch indexing duration in milliseconds.
# TYPE prometheus_local_storage_indexing_batch_duration_milliseconds summary
prometheus_local_storage_indexing_batch_duration_milliseconds{quantile="0.5"} NaN
prometheus_local_storage_indexing_batch_duration_milliseconds{quantile="0.9"} NaN
prometheus_local_storage_indexing_batch_duration_milliseconds{quantile="0.99"} NaN
prometheus_local_storage_indexing_batch_duration_milliseconds_sum 871.5665949999999
prometheus_local_storage_indexing_batch_duration_milliseconds_count 229.0
# HELP process_cpu_seconds Total user and system CPU time spent in seconds.
# TYPE process_cpu_seconds counter
process_cpu_seconds_total 29323.4
# HELP process_virtual_memory_bytes Virtual memory size in bytes.
# TYPE process_virtual_memory_bytes gauge
process_virtual_memory_bytes 2.478268416e+09
# HELP prometheus_build_info A metric with a constant '1' value labeled by version, revision, and branch from which Prometheus was built.
# TYPE prometheus_build_info info
prometheus_build_info{branch="HEAD",revision="ef176e5",version="0.16.0rc1"} 1.0
# HELP prometheus_local_storage_chunk_ops The total number of chunk operations by their type.
# TYPE prometheus_local_storage_chunk_ops counter
prometheus_local_storage_chunk_ops_total{type="clone"} 28.0
prometheus_local_storage_chunk_ops_total{type="create"} 997844.0
prometheus_local_storage_chunk_ops_total{type="drop"} 1.345758e+06
prometheus_local_storage_chunk_ops_total{type="load"} 1641.0
prometheus_local_storage_chunk_ops_total{type="persist"} 981408.0
prometheus_local_storage_chunk_ops_total{type="pin"} 32662.0
prometheus_local_storage_chunk_ops_total{type="transcode"} 980180.0
prometheus_local_storage_chunk_ops_total{type="unpin"} 32662.0
# HELP foo histogram Testing histogram buckets.
# TYPE foo histogram
foo_bucket{le="0.0"} 0.0
foo_bucket{le="1e-05"} 0.0
foo_bucket{le="0.0001"} 0.0
foo_bucket{le="0.1"} 8.0
foo_bucket{le="1.0"} 10.0
foo_bucket{le="10.0"} 17.0
foo_bucket{le="100000.0"} 17.0
foo_bucket{le="1e+06"} 17.0
foo_bucket{le="1.55555555555552e+06"} 17.0
foo_bucket{le="1e+23"} 17.0
foo_bucket{le="+Inf"} 17.0
foo_count 17.0
foo_sum 324789.3
foo_created 1.520430000123e+09
# HELP bar histogram Testing with labels.
# TYPE bar histogram
bar_bucket{a="b",le="+Inf"} 0.0
bar_bucket{a="c",le="+Inf"} 0.0
# EOF
`,
			out: []*OpenMetricFamily{
				&OpenMetricFamily{
					Name:       String("go_gc_duration_seconds"),
					Help:       String("A summary of the GC invocation durations."),
					MetricType: String(OpenMetricTypeSummary),
					Samples: []*Sample{&Sample{Name: "go_gc_duration_seconds", Value: 0.013300656000000001, Labels: map[string]string{"quantile": "0.0"}},
						&Sample{Name: "go_gc_duration_seconds", Value: 0.013638736, Labels: map[string]string{"quantile": "0.25"}},
						&Sample{Name: "go_gc_duration_seconds", Value: 0.013759906, Labels: map[string]string{"quantile": "0.5"}},
						&Sample{Name: "go_gc_duration_seconds", Value: 0.013962066, Labels: map[string]string{"quantile": "0.75"}},
						&Sample{Name: "go_gc_duration_seconds", Value: 0.021383540000000003, Labels: map[string]string{"quantile": "1.0"}},
						&Sample{Name: "go_gc_duration_seconds_sum", Value: 56.12904785},
						&Sample{Name: "go_gc_duration_seconds_count", Value: 7476.0},
					},
				},
				&OpenMetricFamily{
					Name:       String("go_goroutines"),
					Help:       String("Number of goroutines that currently exist."),
					MetricType: String(OpenMetricTypeGauge),
					Samples:    []*Sample{&Sample{Name: "go_goroutines", Value: 166.0}},
				},
				&OpenMetricFamily{
					Name:       String("prometheus_local_storage_indexing_batch_duration_milliseconds"),
					Help:       String("Quantiles for batch indexing duration in milliseconds."),
					MetricType: String(OpenMetricTypeSummary),
					Samples: []*Sample{&Sample{Name: "prometheus_local_storage_indexing_batch_duration_milliseconds", Value: Value(math.NaN()), Labels: map[string]string{"quantile": "0.5"}},
						&Sample{Name: "prometheus_local_storage_indexing_batch_duration_milliseconds", Value: Value(math.NaN()), Labels: map[string]string{"quantile": "0.9"}},
						&Sample{Name: "prometheus_local_storage_indexing_batch_duration_milliseconds", Value: Value(math.NaN()), Labels: map[string]string{"quantile": "0.99"}},
						&Sample{Name: "prometheus_local_storage_indexing_batch_duration_milliseconds_sum", Value: 871.5665949999999},
						&Sample{Name: "prometheus_local_storage_indexing_batch_duration_milliseconds_count", Value: 229.0},
					},
				},
				&OpenMetricFamily{
					Name:       String("process_cpu_seconds"),
					Help:       String("Total user and system CPU time spent in seconds."),
					MetricType: String(OpenMetricTypeCounter),
					Samples:    []*Sample{&Sample{Name: "process_cpu_seconds_total", Value: 29323.4}},
				},
				&OpenMetricFamily{
					Name:       String("process_virtual_memory_bytes"),
					Help:       String("Virtual memory size in bytes."),
					MetricType: String(OpenMetricTypeGauge),
					Samples:    []*Sample{&Sample{Name: "process_virtual_memory_bytes", Value: 2.478268416e+09}},
				},
				&OpenMetricFamily{
					Name:       String("prometheus_build_info"),
					Help:       String("A metric with a constant '1' value labeled by version, revision, and branch from which Prometheus was built."),
					MetricType: String(OpenMetricTypeInfo),
					Samples:    []*Sample{&Sample{Name: "prometheus_build_info", Value: 1.0, Labels: map[string]string{"branch": "HEAD", "revision": "ef176e5", "version": "0.16.0rc1"}}},
				},
				&OpenMetricFamily{
					Name:       String("prometheus_local_storage_chunk_ops"),
					Help:       String("The total number of chunk operations by their type."),
					MetricType: String(OpenMetricTypeCounter),
					Samples: []*Sample{&Sample{Name: "prometheus_local_storage_chunk_ops_total", Value: 28.0, Labels: map[string]string{"type": "clone"}},
						&Sample{Name: "prometheus_local_storage_chunk_ops_total", Value: 997844.0, Labels: map[string]string{"type": "create"}},
						&Sample{Name: "prometheus_local_storage_chunk_ops_total", Value: 1.345758e+06, Labels: map[string]string{"type": "drop"}},
						&Sample{Name: "prometheus_local_storage_chunk_ops_total", Value: 1641.0, Labels: map[string]string{"type": "load"}},
						&Sample{Name: "prometheus_local_storage_chunk_ops_total", Value: 981408.0, Labels: map[string]string{"type": "persist"}},
						&Sample{Name: "prometheus_local_storage_chunk_ops_total", Value: 32662.0, Labels: map[string]string{"type": "pin"}},
						&Sample{Name: "prometheus_local_storage_chunk_ops_total", Value: 980180.0, Labels: map[string]string{"type": "transcode"}},
						&Sample{Name: "prometheus_local_storage_chunk_ops_total", Value: 32662.0, Labels: map[string]string{"type": "unpin"}},
					},
				},
				&OpenMetricFamily{
					Name:       String("foo"),
					Help:       String("histogram Testing histogram buckets."),
					MetricType: String(OpenMetricTypeHistogram),
					Samples: []*Sample{&Sample{Name: "foo_bucket", Value: 0.0, Labels: map[string]string{"le": "0.0"}},
						&Sample{Name: "foo_bucket", Value: 0.0, Labels: map[string]string{"le": "1e-05"}},
						&Sample{Name: "foo_bucket", Value: 0.0, Labels: map[string]string{"le": "0.0001"}},
						&Sample{Name: "foo_bucket", Value: 8.0, Labels: map[string]string{"le": "0.1"}},
						&Sample{Name: "foo_bucket", Value: 10.0, Labels: map[string]string{"le": "1.0"}},
						&Sample{Name: "foo_bucket", Value: 17.0, Labels: map[string]string{"le": "10.0"}},
						&Sample{Name: "foo_bucket", Value: 17.0, Labels: map[string]string{"le": "100000.0"}},
						&Sample{Name: "foo_bucket", Value: 17.0, Labels: map[string]string{"le": "1e+06"}},
						&Sample{Name: "foo_bucket", Value: 17.0, Labels: map[string]string{"le": "1.55555555555552e+06"}},
						&Sample{Name: "foo_bucket", Value: 17.0, Labels: map[string]string{"le": "1e+23"}},
						&Sample{Name: "foo_bucket", Value: 17.0, Labels: map[string]string{"le": "+Inf"}},
						&Sample{Name: "foo_count", Value: 17.0},
						&Sample{Name: "foo_sum", Value: 324789.3},
						&Sample{Name: "foo_created", Value: 1.520430000123e+09},
					},
				},
				&OpenMetricFamily{
					Name:       String("bar"),
					Help:       String("histogram Testing with labels."),
					MetricType: String(OpenMetricTypeHistogram),
					Samples: []*Sample{&Sample{Name: "bar_bucket", Value: 0.0, Labels: map[string]string{"a": "b", "le": "+Inf"}},
						&Sample{Name: "bar_bucket", Value: 0.0, Labels: map[string]string{"a": "c", "le": "+Inf"}},
					},
				},
			},
		},
	}
	for i, scenario := range scenarios[36:] {
		out, err := openMetricParser.TextToOpenMetricFamilies(strings.NewReader(scenario.in))
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
			if !compareOpenMetricFamily(expected, got) {
				t.Errorf(
					"%d. expected MetricFamily %v, got %v",
					i, mustMarshal(expected), mustMarshal(got),
				)
			}
		}
	}
}

func TestOpenMetricsTextParse(t *testing.T) {
	testOpenMetricsTextParse(t)
}

func BenchmarkOpenMetricsTextParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testOpenMetricsTextParse(b)
	}
}

func testOpenMetricsParseError(t testing.TB) {
	var scenarios = []struct {
		in  string
		err string
	}{
		// 0: No new-line at end of input.
		{
			in: `
bla 3.14
blubber 42`,
			err: "text format parsing error in line 3: unexpected end of input stream",
		},
		// 1: Invalid escape sequence in label value.
		{
			in:  `metric{label="\t"} 3.14`,
			err: "text format parsing error in line 1: invalid escape sequence",
		},
		// 2: Newline in label value.
		{
			in: `
metric{label="new
line"} 3.14
`,
			err: `text format parsing error in line 2: label value "new" contains unescaped new-line`,
		},
		// 3:
		{
			in:  `metric{@="bla"} 3.14`,
			err: "text format parsing error in line 1: invalid label name for metric",
		},
		// 4:
		{
			in:  `metric{__name__="bla"} 3.14`,
			err: `text format parsing error in line 1: label name "__name__" is reserved`,
		},
		// 5:
		{
			in:  `metric{label+="bla"} 3.14`,
			err: "text format parsing error in line 1: expected '=' after label name",
		},
		// 6:
		{
			in:  `metric{label=bla} 3.14`,
			err: "text format parsing error in line 1: expected '\"' at start of label value",
		},
		// 7:
		{
			in: `
# TYPE metric summary
metric{quantile="bla"} 3.14
`,
			err: "text format parsing error in line 3: expected float as value for 'quantile' label",
		},
		// 8:
		{
			in:  `metric{label="bla"+} 3.14`,
			err: "text format parsing error in line 1: unexpected end of label value",
		},
		// 9:
		{
			in: `metric{label="bla"} 3.14 2 3
`,
			err: "text format parsing error in line 1: unexpected byte '3' after timestamp",
		},
		// 10:
		{
			in: `metric{label="bla"} blubb
`,
			err: "text format parsing error in line 1: expected float as value",
		},
		// 11:
		{
			in: `
# HELP metric one
# HELP metric two
`,
			err: "text format parsing error in line 3: second HELP line for metric name",
		},
		// 12:
		{
			in: `
# TYPE metric counter
# TYPE metric untyped
`,
			err: `text format parsing error in line 3: second TYPE line for metric name "metric", or TYPE reported after samples`,
		},
		// 13:
		{
			in: `
metric 4.12
# TYPE metric counter
`,
			err: `text format parsing error in line 3: second TYPE line for metric name "metric", or TYPE reported after samples`,
		},
		// 14:
		{
			in: `
# TYPE metric bla
`,
			err: "text format parsing error in line 2: unknown metric type",
		},
		// 15:
		{
			in: `
# TYPE met-ric
`,
			err: "text format parsing error in line 2: invalid metric name in comment",
		},
		// 16:
		{
			in:  `@invalidmetric{label="bla"} 3.14 2`,
			err: "text format parsing error in line 1: invalid metric name",
		},
		// 17:
		{
			in:  `{label="bla"} 3.14 2`,
			err: "text format parsing error in line 1: invalid metric name",
		},
		// 18:
		{
			in: `
# TYPE metric histogram
metric_bucket{le="bla"} 3.14
`,
			err: "text format parsing error in line 3: expected float as value for 'le' label",
		},
		// 19: Invalid UTF-8 in label value.
		{
			in:  "metric{l=\"\xbd\"} 3.14\n",
			err: "text format parsing error in line 1: invalid label value \"\\xbd\"",
		},
		// 20: Go 1.13 sometimes allows underscores in numbers.
		{
			in:  "foo 1_2\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 21: Go 1.13 supports hex floating point.
		{
			in:  "foo 0x1p-3\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 22: Check for various other literals variants, just in case.
		{
			in:  "foo 0x1P-3\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 23:
		{
			in:  "foo 0B1\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 24:
		{
			in:  "foo 0O1\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 25:
		{
			in:  "foo 0X1\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 26:
		{
			in:  "foo 0x1\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 27:
		{
			in:  "foo 0b1\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 28:
		{
			in:  "foo 0o1\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 29:
		{
			in:  "foo 0x1\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 30:
		{
			in:  "foo 0x1\n",
			err: "text format parsing error in line 1: expected float as value",
		},
		// 31: Check histogram label.
		{
			in: `
# TYPE metric histogram
metric_bucket{le="0x1p-3"} 3.14
`,
			err: "text format parsing error in line 3: expected float as value for 'le' label",
		},
		// 32: Check quantile label.
		{
			in: `
# TYPE metric summary
metric{quantile="0x1p-3"} 3.14
`,
			err: "text format parsing error in line 3: expected float as value for 'quantile' label",
		},
		// 33: Check duplicate label.
		{
			in:  `metric{label="bla",label="bla"} 3.14`,
			err: "text format parsing error in line 1: duplicate label names for metric",
		},
		// 34: Check missing '# EOF'.
		{
			in: `metric{label="bla"} 3.14
`,
			err: "text format parsing error in line 2: expected '# EOF' at end",
		},
		// 35: Check line after '# EOF'.
		{
			in: `metric{label="bla"} 3.14
# EOF
# TYPE metric counter
`,
			err: "text format parsing error in line 3: a line after '# EOF'",
		},
		// 36: Check invalid eof line.
		{
			in: `metric{label="bla"} 3.14
# EOF a
`,
			err: "text format parsing error in line 2: invalid eof line",
		},
		// 37: Check exemplar length out of 128.
		{
			in: `metric{label="bla"} 3.14 # {a="23456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789"} 1
# EOF
`,
			err: "text format parsing error in line 1: out of exemplar max length 128",
		},
		// 38: Check invalid unit.
		{
			in: `# TYPE foo counter
# UNIT foo seconds
`,
			err: "text format parsing error in line 2: expected unit as metric name suffix, found \"foo\"",
		},
	}

	for i, scenario := range scenarios {
		_, err := openMetricParser.TextToOpenMetricFamilies(strings.NewReader(scenario.in))
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

func TestOpenMetricsParseError(t *testing.T) {
	testOpenMetricsParseError(t)
}

func BenchmarkOpenMetricsParseError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testOpenMetricsParseError(b)
	}
}

func mustMarshal(o *OpenMetricFamily) string {
	data, err := json.Marshal(o)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func compareOpenMetricFamily(omf1, omf2 *OpenMetricFamily) bool {
	if omf1 == omf2 {
		return true
	}
	if omf1.GetName() != omf2.GetName() {
		return false
	}
	if omf1.GetHelp() != omf2.GetHelp() {
		return false
	}
	if omf1.GetUnit() != omf2.GetUnit() {
		return false
	}
	if omf1.GetType() != omf2.GetType() {
		return false
	}
	if omf1.GetSample() != omf2.GetSample() {
		return false
	}
	for i := range omf1.Samples {
		sample := omf1.Samples[i]
		comparingSample := omf2.Samples[i]
		if sample.Name != comparingSample.Name {
			return false
		}
		if len(sample.Labels) != len(comparingSample.Labels) {
			return false
		}
		for lbn, lbv := range sample.Labels {
			if lbv != comparingSample.Labels[lbn] {
				return false
			}
		}
		if sample.Value != comparingSample.Value &&
			!(math.IsNaN(float64(sample.Value)) && math.IsNaN(float64(comparingSample.Value))) {
			return false
		}
		if sample.Timestamp != nil && comparingSample.Timestamp != nil {
			if sample.Timestamp.Sec != comparingSample.Timestamp.Sec ||
				sample.Timestamp.NSec != comparingSample.Timestamp.NSec {
				return false
			}
		} else if sample.Timestamp == nil && comparingSample.Timestamp == nil {
		} else {
			return false
		}
		if sample.Exemplar != nil && comparingSample.Exemplar != nil {
			if sample.Exemplar.Value != comparingSample.Exemplar.Value &&
				!(math.IsNaN(float64(sample.Exemplar.Value)) && math.IsNaN(float64(comparingSample.Exemplar.Value))) {
				return false
			}
			if len(sample.Exemplar.Labels) != len(comparingSample.Exemplar.Labels) {
				return false
			}
			for lbn, lbv := range sample.Exemplar.Labels {
				if lbv != comparingSample.Exemplar.Labels[lbn] {
					return false
				}
			}
			if sample.Exemplar.Timestamp != nil && comparingSample.Exemplar.Timestamp != nil {
				if sample.Exemplar.Timestamp.Sec != comparingSample.Exemplar.Timestamp.Sec ||
					sample.Exemplar.Timestamp.NSec != comparingSample.Exemplar.Timestamp.NSec {
					return false
				}
			} else if sample.Exemplar.Timestamp == nil && comparingSample.Exemplar.Timestamp == nil {
			} else {
				return false
			}
		} else if sample.Exemplar == nil && comparingSample.Exemplar == nil {
		} else {
			return false
		}
	}
	return true
}
