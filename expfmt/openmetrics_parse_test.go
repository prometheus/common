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
	"strings"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

func testOpenMetricsParse(t testing.TB) {
	var omParser OpenMetricsParser
	scenarios := []struct {
		in  string
		out []*dto.MetricFamily
	}{
		// 0: EOF as input
		{
			in: `# EOF
`,

			out: []*dto.MetricFamily{},
		},

		// 1: only has type as input
		{
			in: `# TYPE foo counter
# EOF
`,

			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo"),
					Type: dto.MetricType_COUNTER.Enum(),
				},
			},
		},

		// 2: has type and unit as input
		{
			in: `# TYPE foo_seconds counter
# UNIT foo_seconds seconds
# EOF
`,

			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo_seconds"),
					Type: dto.MetricType_COUNTER.Enum(),
					Unit: proto.String("seconds"),
				},
			},
		},

		// 3: has type, unit, help as input
		{
			in: `# HELP foo_seconds abc
# TYPE foo_seconds counter
# UNIT foo_seconds seconds
# EOF
`,

			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo_seconds"),
					Type: dto.MetricType_COUNTER.Enum(),
					Unit: proto.String("seconds"),
					Help: proto.String("abc"),
				},
			},
		},

		// 4: type gauge
		{
			in: `# HELP foo_seconds abc
# TYPE foo_seconds gauge
# UNIT foo_seconds seconds
# EOF
`,

			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo_seconds"),
					Type: dto.MetricType_GAUGE.Enum(),
					Unit: proto.String("seconds"),
					Help: proto.String("abc"),
				},
			},
		},

		// 5: type histogram
		{
			in: `# HELP foo_seconds abc
# TYPE foo_seconds histogram
# UNIT foo_seconds seconds
# EOF
`,

			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo_seconds"),
					Type: dto.MetricType_HISTOGRAM.Enum(),
					Unit: proto.String("seconds"),
					Help: proto.String("abc"),
				},
			},
		},

		// 6: type summary
		{
			in: `# HELP foo_seconds abc
# TYPE foo_seconds summary
# UNIT foo_seconds seconds
# EOF
`,

			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo_seconds"),
					Type: dto.MetricType_SUMMARY.Enum(),
					Unit: proto.String("seconds"),
					Help: proto.String("abc"),
				},
			},
		},

		// 7: type summary
		{
			in: `# HELP foo_seconds abc
# TYPE foo_seconds gaugehistogram
# UNIT foo_seconds seconds
# EOF
`,

			out: []*dto.MetricFamily{
				{
					Name: proto.String("foo_seconds"),
					Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
					Unit: proto.String("seconds"),
					Help: proto.String("abc"),
				},
			},
		},

		// 8: a normal comment
		{
			in: `#
# TYPE name_seconds counter
# UNIT name_seconds seconds
# HELP name_seconds two-line\n doc  str\\ing
# HELP  name2 doc str"ing 2
#    TYPE    name2 gauge
# EOF
`,

			out: []*dto.MetricFamily{
				{
					Name: proto.String("name_seconds"),
					Type: dto.MetricType_COUNTER.Enum(),
					Unit: proto.String("seconds"),
					Help: proto.String("two-line\n doc  str\\ing"),
				},
				{
					Name: proto.String("name2"),
					Type: dto.MetricType_GAUGE.Enum(),
					Help: proto.String("doc str\"ing 2"),
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
		// 0:
		{
			in: `# TYPE metric counter
# TYPE metric untyped
`,
			err: `openmetrics format parsing error in line 2: second TYPE line for metric name "metric", or TYPE reported after samples`,
		},
		// 1:
		{
			in: `# TYPE metric bla
`,
			err: "openmetrics format parsing error in line 1: unknown metric type",
		},
		// 2:
		{
			in: `# TYPE met-ric
`,
			err: "openmetrics format parsing error in line 1: invalid metric name in comment",
		},
		// 3: metrics ends without unit
		{
			in: `# TYPE metric counter
# UNIT metric seconds
`,
			err: `openmetrics format parsing error in line 2: expected unit as metric name suffix, found metric "metric"`,
		},

		// 4: metrics ends without EOF
		{
			in: `# TYPE metric_seconds counter
# UNIT metric_seconds seconds
`,
			err: `openmetrics format parsing error in line 3: expected EOF keyword at the end`,
		},

		// 5: line after EOF
		{
			in: `# EOF
# TYPE metric counter
`,
			err: `openmetrics format parsing error in line 2: unexpected line after EOF, got '#'`,
		},

		// 6: invalid start token
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
