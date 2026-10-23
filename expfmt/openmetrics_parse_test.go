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
	"errors"
	"io"
	"math"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/prometheus/common/model"
)

func decodeAllOpenMetrics(t *testing.T, input string, format Format) map[string]*dto.MetricFamily {
	t.Helper()
	dec := NewDecoder(strings.NewReader(input), format)
	out := make(map[string]*dto.MetricFamily)
	for {
		var mf dto.MetricFamily
		err := dec.Decode(&mf)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		cp := proto.Clone(&mf).(*dto.MetricFamily)
		out[cp.GetName()] = cp
	}
	return out
}

func decodeOpenMetricsErr(input string, format Format) error {
	dec := NewDecoder(strings.NewReader(input), format)
	for {
		var mf dto.MetricFamily
		if err := dec.Decode(&mf); err != nil {
			return err
		}
	}
}

func assertProtoEqual(t *testing.T, want, got proto.Message, msgAndArgs ...any) {
	t.Helper()
	require.Truef(t, proto.Equal(want, got), "proto mismatch:\nwant: %v\ngot:  %v\n%v", want, got, msgAndArgs)
}

func TestOpenMetricsParse(t *testing.T) {
	input := `# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
# UNIT go_gc_duration_seconds seconds
go_gc_duration_seconds{quantile="0"} 4.9351e-05
go_gc_duration_seconds{quantile="0.25"} 7.424100000000001e-05
go_gc_duration_seconds{quantile="0.5",a="b"} 8.3835e-05
# HELP nohelp1 
# HELP help2 escape \ \n \\ \" \x chars
# UNIT nounit 
go_gc_duration_seconds{quantile="1.0",a="b"} 8.3835e-05
go_gc_duration_seconds_count 99
some:aggregate:rate5m{a_b="c"} 1
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 33 123.123
# TYPE hh histogram
hh_bucket{le="+Inf"} 1
# TYPE gh gaugehistogram
gh_bucket{le="+Inf"} 1
# TYPE hhh histogram
hhh_bucket{le="+Inf"} 1 # {id="histogram-bucket-test"} 4
hhh_count 1
# TYPE ggh gaugehistogram
ggh_bucket{le="+Inf"} 1 # {id="gaugehistogram-bucket-test",xx="yy"} 4 123.123
ggh_gcount 1
# TYPE smr_seconds summary
smr_seconds_count 2
smr_seconds_sum 42.0
# TYPE un unknown
_metric_starting_with_underscore 1
testmetric{_label_starting_with_underscore="foo"} 1
testmetric{label="\"bar\""} 1
# HELP foo Counter with and without labels to certify ST is parsed for both cases
# TYPE foo counter
foo_total 17.0 1520879607.789 # {id="counter-test"} 5
foo_created 1520872607.123
foo_total{a="b"} 17.0 1520879607.789 # {id="counter-test"} 5
foo_created{a="b"} 1520872607.123
foo_total{le="c"} 21.0
foo_created{le="c"} 1520872621.123
foo_total{le="1"} 10.0
# HELP bar Summary with ST at the end, making sure we find ST even if it's multiple lines a far
# TYPE bar summary
bar_count 17
bar_sum 324789.3
bar{quantile="0.95"} 123.7
bar{quantile="0.99"} 150.0
bar_created 1520872608.124
# HELP baz Histogram with the same objective as above's summary
# TYPE baz histogram
baz_bucket{le="0.0"} 0
baz_bucket{le="+Inf"} 17
baz_count 17
baz_sum 324789.3
baz_created 1520872609.125
# HELP fizz_created Gauge which shouldn't be parsed as ST
# TYPE fizz_created gauge
fizz_created 17.0
# HELP something Histogram with _created between buckets and summary
# TYPE something histogram
something_count 18
something_sum 324789.4
something_created 1520430001
something_bucket{le="0.0"} 1
something_bucket{le="1"} 2
something_bucket{le="+Inf"} 18
# HELP yum Summary with _created between sum and quantiles
# TYPE yum summary
yum_count 20
yum_sum 324789.5
yum_created 1520430003
yum{quantile="0.95"} 123.7
yum{quantile="0.99"} 150.0
# HELP foobar Summary with _created as the first line
# TYPE foobar summary
foobar_count 21
foobar_created 1520430004
foobar_sum 324789.6
foobar{quantile="0.95"} 123.8
foobar{quantile="0.99"} 150.1
# HELP nansum Summary with NaN value
# TYPE nansum summary
nansum_count 0
nansum_sum 0.0
nansum{quantile="0.95"} nan
nansum{quantile="0.99"} NaN`
	input += "\n# HELP metric foo\x00bar"
	input += "\nnull_byte_metric{a=\"abc\x00\"} 1"
	input += "\n# EOF\n"

	fams := decodeAllOpenMetrics(t, input, FmtOpenMetrics_1_0_0)

	// Empty descriptor-only families (nohelp1, help2, nounit, metric) are not emitted.
	require.NotContains(t, fams, "nohelp1")
	require.NotContains(t, fams, "help2")
	require.NotContains(t, fams, "nounit")
	require.NotContains(t, fams, "metric")

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("go_gc_duration_seconds"),
		Help: proto.String("A summary of the GC invocation durations."),
		Type: dto.MetricType_SUMMARY.Enum(),
		Unit: proto.String("seconds"),
		Metric: []*dto.Metric{
			{
				Summary: &dto.Summary{
					SampleCount: proto.Uint64(99),
					Quantile: []*dto.Quantile{
						{Quantile: proto.Float64(0), Value: proto.Float64(4.9351e-05)},
						{Quantile: proto.Float64(0.25), Value: proto.Float64(7.424100000000001e-05)},
					},
				},
			},
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("a"), Value: proto.String("b")},
				},
				Summary: &dto.Summary{
					Quantile: []*dto.Quantile{
						{Quantile: proto.Float64(0.5), Value: proto.Float64(8.3835e-05)},
						{Quantile: proto.Float64(1.0), Value: proto.Float64(8.3835e-05)},
					},
				},
			},
		},
	}, fams["go_gc_duration_seconds"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("go_goroutines"),
		Help: proto.String("Number of goroutines that currently exist."),
		Type: dto.MetricType_GAUGE.Enum(),
		Metric: []*dto.Metric{
			{
				Gauge:       &dto.Gauge{Value: proto.Float64(33)},
				TimestampMs: proto.Int64(123123),
			},
		},
	}, fams["go_goroutines"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("hhh"),
		Type: dto.MetricType_HISTOGRAM.Enum(),
		Metric: []*dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: proto.Uint64(1),
					Bucket: []*dto.Bucket{
						{
							CumulativeCount: proto.Uint64(1),
							UpperBound:      proto.Float64(math.Inf(1)),
							Exemplar: &dto.Exemplar{
								Label: []*dto.LabelPair{
									{Name: proto.String("id"), Value: proto.String("histogram-bucket-test")},
								},
								Value: proto.Float64(4),
							},
						},
					},
				},
			},
		},
	}, fams["hhh"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("ggh"),
		Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
		Metric: []*dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount: proto.Uint64(1),
					Bucket: []*dto.Bucket{
						{
							CumulativeCount: proto.Uint64(1),
							UpperBound:      proto.Float64(math.Inf(1)),
							Exemplar: &dto.Exemplar{
								Label: []*dto.LabelPair{
									{Name: proto.String("id"), Value: proto.String("gaugehistogram-bucket-test")},
									{Name: proto.String("xx"), Value: proto.String("yy")},
								},
								Value:     proto.Float64(4),
								Timestamp: timestamppb.New(time.Unix(123, 123000000)),
							},
						},
					},
				},
			},
		},
	}, fams["ggh"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("foo_total"),
		Help: proto.String("Counter with and without labels to certify ST is parsed for both cases"),
		Type: dto.MetricType_COUNTER.Enum(),
		Metric: []*dto.Metric{
			{
				Counter: &dto.Counter{
					Value:            proto.Float64(17.0),
					CreatedTimestamp: timestamppb.New(time.Unix(1520872607, 123000000)),
					Exemplar: &dto.Exemplar{
						Label: []*dto.LabelPair{
							{Name: proto.String("id"), Value: proto.String("counter-test")},
						},
						Value: proto.Float64(5),
					},
				},
				TimestampMs: proto.Int64(1520879607789),
			},
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("a"), Value: proto.String("b")},
				},
				Counter: &dto.Counter{
					Value:            proto.Float64(17.0),
					CreatedTimestamp: timestamppb.New(time.Unix(1520872607, 123000000)),
					Exemplar: &dto.Exemplar{
						Label: []*dto.LabelPair{
							{Name: proto.String("id"), Value: proto.String("counter-test")},
						},
						Value: proto.Float64(5),
					},
				},
				TimestampMs: proto.Int64(1520879607789),
			},
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("le"), Value: proto.String("c")},
				},
				Counter: &dto.Counter{
					Value:            proto.Float64(21.0),
					CreatedTimestamp: timestamppb.New(time.Unix(1520872621, 123000000)),
				},
			},
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("le"), Value: proto.String("1")},
				},
				Counter: &dto.Counter{
					Value: proto.Float64(10.0),
				},
			},
		},
	}, fams["foo_total"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("something"),
		Help: proto.String("Histogram with _created between buckets and summary"),
		Type: dto.MetricType_HISTOGRAM.Enum(),
		Metric: []*dto.Metric{
			{
				Histogram: &dto.Histogram{
					SampleCount:      proto.Uint64(18),
					SampleSum:        proto.Float64(324789.4),
					CreatedTimestamp: timestamppb.New(time.Unix(1520430001, 0)),
					Bucket: []*dto.Bucket{
						{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.0)},
						{CumulativeCount: proto.Uint64(2), UpperBound: proto.Float64(1.0)},
						{CumulativeCount: proto.Uint64(18), UpperBound: proto.Float64(math.Inf(1))},
					},
				},
			},
		},
	}, fams["something"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("foobar"),
		Help: proto.String("Summary with _created as the first line"),
		Type: dto.MetricType_SUMMARY.Enum(),
		Metric: []*dto.Metric{
			{
				Summary: &dto.Summary{
					SampleCount:      proto.Uint64(21),
					SampleSum:        proto.Float64(324789.6),
					CreatedTimestamp: timestamppb.New(time.Unix(1520430004, 0)),
					Quantile: []*dto.Quantile{
						{Quantile: proto.Float64(0.95), Value: proto.Float64(123.8)},
						{Quantile: proto.Float64(0.99), Value: proto.Float64(150.1)},
					},
				},
			},
		},
	}, fams["foobar"])

	nansum := fams["nansum"]
	require.NotNil(t, nansum)
	require.Len(t, nansum.GetMetric(), 1)
	require.Len(t, nansum.GetMetric()[0].GetSummary().GetQuantile(), 2)
	require.True(t, math.IsNaN(nansum.GetMetric()[0].GetSummary().GetQuantile()[0].GetValue()))
	require.True(t, math.IsNaN(nansum.GetMetric()[0].GetSummary().GetQuantile()[1].GetValue()))

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("null_byte_metric"),
		Type: dto.MetricType_UNTYPED.Enum(),
		Metric: []*dto.Metric{
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("a"), Value: proto.String("abc\x00")},
				},
				Untyped: &dto.Untyped{Value: proto.Float64(1)},
			},
		},
	}, fams["null_byte_metric"])
}

func TestOpenMetricsParse_UTF8(t *testing.T) {
	input := `# HELP "go.gc_duration_seconds" A summary of the GC invocation durations.
# TYPE "go.gc_duration_seconds" summary
# UNIT "go.gc_duration_seconds" seconds
{"go.gc_duration_seconds",quantile="0"} 4.9351e-05
{"go.gc_duration_seconds",quantile="0.25"} 7.424100000000001e-05
{"go.gc_duration_seconds_created"} 1520872607.123
{"go.gc_duration_seconds",quantile="0.5",a="b"} 8.3835e-05
{"http.status",q="0.9",a="b"} 8.3835e-05
{"http.status",q="0.9",a="b"} 8.3835e-05
{q="0.9","http.status",a="b"} 8.3835e-05
{"go.gc_duration_seconds_sum"} 0.004304266
{"Heizölrückstoßabdämpfung 10€ metric with \"interesting\" {character\nchoices}","strange©™\n'quoted' \"name\""="6"} 10.0
# TYPE quotedexemplar counter
quotedexemplar_total 1 # {"id.thing"="histogram-count-test"} 4
# TYPE quotedexemplar2 counter
quotedexemplar2_total 1 # {"id.thing"="histogram-count-test",other="hello"} 4
# EOF
`

	// With default LegacyValidation (FmtOpenMetrics_1_0_0), UTF-8 names must fail.
	err := decodeOpenMetricsErr(input, FmtOpenMetrics_1_0_0)
	require.ErrorContains(t, err, `invalid metric name "go.gc_duration_seconds"`)

	// With allow-utf-8 (NoEscaping), UTF-8 metric and label names succeed.
	utf8Format := FmtOpenMetrics_1_0_0.WithEscapingScheme(model.NoEscaping)
	fams := decodeAllOpenMetrics(t, input, utf8Format)

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("go.gc_duration_seconds"),
		Help: proto.String("A summary of the GC invocation durations."),
		Type: dto.MetricType_SUMMARY.Enum(),
		Unit: proto.String("seconds"),
		Metric: []*dto.Metric{
			{
				Summary: &dto.Summary{
					CreatedTimestamp: timestamppb.New(time.Unix(1520872607, 123000000)),
					Quantile: []*dto.Quantile{
						{Quantile: proto.Float64(0), Value: proto.Float64(4.9351e-05)},
						{Quantile: proto.Float64(0.25), Value: proto.Float64(7.424100000000001e-05)},
					},
				},
			},
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("a"), Value: proto.String("b")},
				},
				Summary: &dto.Summary{
					Quantile: []*dto.Quantile{
						{Quantile: proto.Float64(0.5), Value: proto.Float64(8.3835e-05)},
					},
				},
			},
		},
	}, fams["go.gc_duration_seconds"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("go.gc_duration_seconds_sum"),
		Type: dto.MetricType_UNTYPED.Enum(),
		Metric: []*dto.Metric{
			{
				Untyped: &dto.Untyped{Value: proto.Float64(0.004304266)},
			},
		},
	}, fams["go.gc_duration_seconds_sum"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("http.status"),
		Type: dto.MetricType_UNTYPED.Enum(),
		Metric: []*dto.Metric{
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("q"), Value: proto.String("0.9")},
					{Name: proto.String("a"), Value: proto.String("b")},
				},
				Untyped: &dto.Untyped{Value: proto.Float64(8.3835e-05)},
			},
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("q"), Value: proto.String("0.9")},
					{Name: proto.String("a"), Value: proto.String("b")},
				},
				Untyped: &dto.Untyped{Value: proto.Float64(8.3835e-05)},
			},
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("q"), Value: proto.String("0.9")},
					{Name: proto.String("a"), Value: proto.String("b")},
				},
				Untyped: &dto.Untyped{Value: proto.Float64(8.3835e-05)},
			},
		},
	}, fams["http.status"])

	specialName := "Heizölrückstoßabdämpfung 10€ metric with \"interesting\" {character\nchoices}"
	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String(specialName),
		Type: dto.MetricType_UNTYPED.Enum(),
		Metric: []*dto.Metric{
			{
				Label: []*dto.LabelPair{
					{Name: proto.String("strange©™\n'quoted' \"name\""), Value: proto.String("6")},
				},
				Untyped: &dto.Untyped{Value: proto.Float64(10.0)},
			},
		},
	}, fams[specialName])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("quotedexemplar_total"),
		Type: dto.MetricType_COUNTER.Enum(),
		Metric: []*dto.Metric{
			{
				Counter: &dto.Counter{
					Value: proto.Float64(1),
					Exemplar: &dto.Exemplar{
						Label: []*dto.LabelPair{
							{Name: proto.String("id.thing"), Value: proto.String("histogram-count-test")},
						},
						Value: proto.Float64(4),
					},
				},
			},
		},
	}, fams["quotedexemplar_total"])

	assertProtoEqual(t, &dto.MetricFamily{
		Name: proto.String("quotedexemplar2_total"),
		Type: dto.MetricType_COUNTER.Enum(),
		Metric: []*dto.Metric{
			{
				Counter: &dto.Counter{
					Value: proto.Float64(1),
					Exemplar: &dto.Exemplar{
						Label: []*dto.LabelPair{
							{Name: proto.String("id.thing"), Value: proto.String("histogram-count-test")},
							{Name: proto.String("other"), Value: proto.String("hello")},
						},
						Value: proto.Float64(4),
					},
				},
			},
		},
	}, fams["quotedexemplar2_total"])
}

func TestOpenMetricsParseErrors(t *testing.T) {
	utf8Format := FmtOpenMetrics_1_0_0.WithEscapingScheme(model.NoEscaping)

	cases := []struct {
		name    string
		input   string
		wantEOF bool
		wantErr string
	}{
		// Happy cases. EOF is returned by the decoder at the end of valid data.
		{
			name:    "bare EOF",
			input:   "# EOF",
			wantEOF: true,
		},
		{
			name:    "EOF with newline",
			input:   "# EOF\n",
			wantEOF: true,
		},
		// Unhappy cases ported from model/textparse/openmetricsparse_test.go.
		{
			name:    "empty input",
			input:   "",
			wantErr: "data does not end with # EOF",
		},
		{
			name:    "single newline",
			input:   "\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "metric without value",
			input:   "metric",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "metric with value but no newline or EOF",
			input:   "metric 1",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "metric with value and newline but no EOF",
			input:   "metric 1\n",
			wantErr: "text format parsing error in line 2",
		},
		{
			name:    "metric with exemplar but no EOF",
			input:   "metric_total 1 # {aa=\"bb\"} 4",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "metric without value followed by #EOF",
			input:   "a\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "blank lines before #EOF",
			input:   "\n\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "leading space before metric",
			input:   " a 1\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "digit as metric name",
			input:   "9\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "untyped instead of unknown",
			input:   "# TYPE u untyped\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "trailing space after counter type",
			input:   "# TYPE c counter \n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "double space after hash in TYPE",
			input:   "#  TYPE c counter\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing metric name and type in TYPE",
			input:   "# TYPE \n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing type in TYPE",
			input:   "# TYPE m\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "unit not a suffix of metric",
			input:   "# UNIT metric suffix\n# EOF\n",
			wantErr: `unit "suffix" not a suffix of metric "metric"`,
		},
		{
			name:    "unit without underscore separator",
			input:   "# UNIT metricsuffix suffix\n# EOF\n",
			wantErr: `unit "suffix" not a suffix of metric "metricsuffix"`,
		},
		{
			name:    "unit longer than metric name",
			input:   "# UNIT m suffix\n# EOF\n",
			wantErr: `unit "suffix" not a suffix of metric "m"`,
		},
		{
			name:    "missing metric name in UNIT",
			input:   "# UNIT \n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing unit in UNIT",
			input:   "# UNIT m\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing metric name in HELP",
			input:   "# HELP \n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing help text in HELP",
			input:   "# HELP m\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "tab between metric and value",
			input:   "a\t1\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "tab between value and timestamp",
			input:   "a 1\t2\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "trailing space after timestamp",
			input:   "a 1 2 \n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "incomplete exemplar after timestamp",
			input:   "a 1 2 #\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "invalid timestamp suffix",
			input:   "a 1 1z\n#EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "leading space before # EOF",
			input:   " # EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "data after # EOF",
			input:   "# EOF\na 1",
			wantErr: "text format parsing error in line",
		},
		{
			name:    "extra newline after # EOF",
			input:   "# EOF\n\n",
			wantErr: "text format parsing error in line",
		},
		{
			name:    "data attached to # EOF",
			input:   "# EOFa 1",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "tab after hash in TYPE",
			input:   "#\tTYPE c counter\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "double space before counter in TYPE",
			input:   "# TYPE c  counter\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "three numbers on sample line",
			input:   "a 1 1 1\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "single quotes around label value",
			input:   "a{b='c'} 1\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "leading comma in label set",
			input:   "a{,b=\"c\"} 1\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing comma between labels",
			input:   "a{b=\"c\"d=\"e\"} 1\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "double comma between labels",
			input:   "a{b=\"c\",,d=\"e\"} 1\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "unclosed label set",
			input:   "a{b=\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "invalid UTF-8 in unquoted label name",
			input:   "a{\xff=\"foo\"} 1\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "invalid UTF-8 in label value",
			input:   "a{b=\"\xff\"} 1\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "malformed quoted label in braces",
			input:   "{\"a\",\"b = \"c\"}\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "escape sequence in unquoted label name",
			input:   "{\"a\",b\\nc=\"d\"} 1\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "boolean sample value",
			input:   "a true\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "unclosed quote in label value",
			input:   "something_weird{problem=\"\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "empty label name",
			input:   "empty_label_name{=\"\"} 0\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "bare identifier in braces without space",
			input:   "{A}0\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "underscore in float value",
			input:   "foo 1_2\n\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "hex float lower p",
			input:   "foo 0x1p-3\n\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "hex float upper P",
			input:   "foo 0x1P-3\n\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "underscore in timestamp",
			input:   "foo 0 1_2\n\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "unquoted exemplar label value",
			input:   "custom_metric_total 1 # {aa=bb}\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing value after exemplar labels",
			input:   "custom_metric_total 1 # {aa=\"bb\"}\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing value and EOF after exemplar labels",
			input:   `custom_metric_total 1 # {aa="bb"}`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "bare identifier in exemplar labels",
			input:   `custom_metric_total 1 # {bb}`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "bare identifier with other label in exemplar",
			input:   `custom_metric_total 1 # {bb, a="dd"}`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "double comma in exemplar labels",
			input:   `custom_metric_total 1 # {aa="bb",,cc="dd"} 1`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "underscore in exemplar float",
			input:   `custom_metric_total 1 # {aa="bb"} 1_2`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "hex float in exemplar",
			input:   `custom_metric_total 1 # {aa="bb"} 0x1p-3`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "boolean in exemplar value",
			input:   `custom_metric_total 1 # {aa="bb"} true`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "empty label value in exemplar",
			input:   `custom_metric_total 1 # {aa="bb",cc=}`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "invalid UTF-8 in exemplar label value",
			input:   "custom_metric_total 1 # {aa=\"\\xff\"} 9.0\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "missing metric name in braces with trailing comma",
			input:   `{b="c",} 1`,
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "NaN timestamp",
			input:   "a 1 NaN\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "-Inf timestamp",
			input:   "a 1 -Inf\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "Inf timestamp",
			input:   "a 1 Inf\n# EOF\n",
			wantErr: "text format parsing error in line 1",
		},
		{
			name:    "NaN exemplar timestamp",
			input:   "# TYPE hhh histogram\nhhh_bucket{le=\"+Inf\"} 1 # {aa=\"bb\"} 4 NaN\n# EOF\n",
			wantErr: "text format parsing error in line 2",
		},
		{
			name:    "-Inf exemplar timestamp",
			input:   "# TYPE hhh histogram\nhhh_bucket{le=\"+Inf\"} 1 # {aa=\"bb\"} 4 -Inf\n# EOF\n",
			wantErr: "text format parsing error in line 2",
		},
		// Unsupported info and stateset types in dto.MetricType.
		{
			name:    "unsupported info metric type",
			input:   "# TYPE ii info\nii_info{foo=\"bar\"} 1\n# EOF\n",
			wantErr: `unsupported metric type "info"`,
		},
		{
			name:    "unsupported stateset metric type",
			input:   "# TYPE ss stateset\nss{ss=\"foo\"} 1\n# EOF\n",
			wantErr: `unsupported metric type "stateset"`,
		},
		// Exemplar rune limit in OpenMetrics 1.0 (128 runes).
		{
			name:    "exemplar exceeds 128 rune limit",
			input:   "# TYPE c counter\nc_total 1 # {id=\"" + strings.Repeat("a", 127) + "\"} 1\n# EOF\n",
			wantErr: "exemplar label set exceeds 128 runes",
		},
		// Exemplar on disallowed metric types.
		{
			name:    "exemplar on gauge",
			input:   "# TYPE g gauge\ng 1 # {id=\"a\"} 1\n# EOF\n",
			wantErr: `exemplar not allowed on gauge metric "g"`,
		},
		{
			name:    "exemplar on summary",
			input:   "# TYPE s summary\ns_count 1 # {id=\"a\"} 1\n# EOF\n",
			wantErr: `exemplar not allowed on summary metric "s"`,
		},
		{
			name:    "exemplar on histogram count",
			input:   "# TYPE h histogram\nh_count 1 # {id=\"a\"} 1\n# EOF\n",
			wantErr: `exemplar not allowed on count sample for histogram "h"`,
		},
		// Duplicate labels.
		{
			name:    "duplicate label names on sample",
			input:   "a{b=\"1\",b=\"2\"} 1\n# EOF\n",
			wantErr: `duplicate label "b" for metric "a"`,
		},
		{
			name:    "duplicate label names on exemplar",
			input:   "# TYPE c counter\nc_total 1 # {b=\"1\",b=\"2\"} 1\n# EOF\n",
			wantErr: `duplicate exemplar label name "b"`,
		},
		{
			name:    "counter with _created only and no _total",
			input:   "# TYPE c counter\nc_created 1520872607.123\n# EOF\n",
			wantErr: `counter "c" missing _total value`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := decodeOpenMetricsErr(tc.input, utf8Format)
			if tc.wantEOF {
				require.ErrorIs(t, err, io.EOF)
				return
			}
			require.Error(t, err)
			require.NotErrorIs(t, err, io.EOF)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestOpenMetricsParseBareIdentifierInBraces(t *testing.T) {
	require.NotPanics(t, func() {
		err := decodeOpenMetricsErr("{A} 0\n# EOF\n", FmtOpenMetrics_1_0_0.WithEscapingScheme(model.NoEscaping))
		require.Error(t, err)
		require.NotErrorIs(t, err, io.EOF)
	})
}

func TestOMNullByteHandling(t *testing.T) {
	cases := []struct {
		input   string
		wantErr bool
	}{
		{
			input:   "null_byte_metric{a=\"abc\x00\"} 1\n# EOF\n",
			wantErr: false,
		},
		{
			input:   "a{b=\"\x00ss\"} 1\n# EOF\n",
			wantErr: false,
		},
		{
			input:   "a{b=\"\x00\"} 1\n# EOF\n",
			wantErr: false,
		},
		{
			input:   "a{b=\"\x00\"} 1\n# EOF",
			wantErr: false,
		},
		{
			input:   "a{b=\x00\"ssss\"} 1\n# EOF\n",
			wantErr: true,
		},
		{
			input:   "a{b=\"\x00",
			wantErr: true,
		},
		{
			input:   "a{b\x00=\"hiih\"}\t1",
			wantErr: true,
		},
		{
			input:   "a\x00{b=\"ddd\"} 1",
			wantErr: true,
		},
		{
			input:   "#",
			wantErr: true,
		},
		{
			input:   "# H",
			wantErr: true,
		},
		{
			input:   "custom_metric_total 1 # {b=\x00\"ssss\"} 1\n",
			wantErr: true,
		},
		{
			input:   "custom_metric_total 1 # {b=\"\x00ss\"} 1\n",
			wantErr: true,
		},
	}

	for i, c := range cases {
		err := decodeOpenMetricsErr(c.input, FmtOpenMetrics_1_0_0)
		if !c.wantErr {
			require.ErrorIsf(t, err, io.EOF, "test %d", i)
			continue
		}
		require.Errorf(t, err, "test %d", i)
		require.NotErrorIsf(t, err, io.EOF, "test %d", i)
	}
}

func TestOpenMetricsParseCreatedOrdering(t *testing.T) {
	t.Run("_created line first in histogram and counter", func(t *testing.T) {
		input := `# HELP thing histogram with _created as first line
# TYPE thing histogram
thing_created 1520872607.123
thing_count 17
thing_sum 324789.3
thing_bucket{le="0.0"} 0
thing_bucket{le="+Inf"} 17
# HELP thing_c counter with _created as first line
# TYPE thing_c counter
thing_c_created 1520872607.123
thing_c_total 14123.232
# EOF
`
		fams := decodeAllOpenMetrics(t, input, FmtOpenMetrics_1_0_0)
		wantTS := timestamppb.New(time.Unix(1520872607, 123000000))

		assertProtoEqual(t, wantTS, fams["thing"].GetMetric()[0].GetHistogram().GetCreatedTimestamp())
		assertProtoEqual(t, wantTS, fams["thing_c_total"].GetMetric()[0].GetCounter().GetCreatedTimestamp())
		require.Equal(t, 14123.232, fams["thing_c_total"].GetMetric()[0].GetCounter().GetValue())
	})

	t.Run("counter-like names without TYPE metadata are untyped", func(t *testing.T) {
		input := `foo_total 17.0
foo_created 1520872607.123
foo_total{a="b"} 17.0
foo_created{a="b"} 1520872608.123
# EOF
`
		fams := decodeAllOpenMetrics(t, input, FmtOpenMetrics_1_0_0)
		require.Contains(t, fams, "foo_total")
		require.Contains(t, fams, "foo_created")
		require.Equal(t, dto.MetricType_UNTYPED, fams["foo_total"].GetType())
		require.Equal(t, dto.MetricType_UNTYPED, fams["foo_created"].GetType())
	})
}

func TestOpenMetricsRoundTrip(t *testing.T) {
	inFams := []*dto.MetricFamily{
		{
			Name: proto.String("http_requests_total"),
			Help: proto.String("Total number of HTTP requests."),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("method"), Value: proto.String("GET")},
					},
					Counter: &dto.Counter{
						Value:            proto.Float64(1027),
						CreatedTimestamp: timestamppb.New(time.Unix(12345, 500000000)),
						Exemplar: &dto.Exemplar{
							Label: []*dto.LabelPair{
								{Name: proto.String("trace_id"), Value: proto.String("abc")},
							},
							Value:     proto.Float64(1),
							Timestamp: timestamppb.New(time.Unix(12345, 600000000)),
						},
					},
					TimestampMs: proto.Int64(12345600),
				},
			},
		},
		{
			Name: proto.String("request_duration_seconds"),
			Help: proto.String("Request latency histogram."),
			Unit: proto.String("seconds"),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount:      proto.Uint64(10),
						SampleSum:        proto.Float64(4.5),
						CreatedTimestamp: timestamppb.New(time.Unix(1700000000, 0)),
						Bucket: []*dto.Bucket{
							{CumulativeCount: proto.Uint64(4), UpperBound: proto.Float64(0.1)},
							{CumulativeCount: proto.Uint64(10), UpperBound: proto.Float64(math.Inf(1))},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	for _, mf := range inFams {
		_, err := MetricFamilyToOpenMetrics(&buf, mf, WithCreatedLines())
		require.NoError(t, err)
	}
	_, err := FinalizeOpenMetrics(&buf)
	require.NoError(t, err)

	gotFams := decodeAllOpenMetrics(t, buf.String(), FmtOpenMetrics_1_0_0)
	require.Len(t, gotFams, len(inFams))

	assertProtoEqual(t, inFams[0], gotFams["http_requests_total"])
	assertProtoEqual(t, inFams[1], gotFams["request_duration_seconds"])
}

func TestOpenMetricsStreaming(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pr.Close() })

	dec := NewDecoder(pr, FmtOpenMetrics_1_0_0)

	errCh := make(chan error, 1)
	stepCh := make(chan struct{})
	go func() {
		_, err := io.WriteString(pw, "# TYPE first counter\nfirst_total 1\n# TYPE second gauge\nsecond 2\n")
		if err != nil {
			errCh <- err
			return
		}
		<-stepCh
		_, err = io.WriteString(pw, "# EOF\n")
		_ = pw.CloseWithError(err)
		errCh <- nil
	}()

	var mf1 dto.MetricFamily
	require.NoError(t, dec.Decode(&mf1))
	require.Equal(t, "first_total", mf1.GetName())
	require.Equal(t, dto.MetricType_COUNTER, mf1.GetType())
	require.Len(t, mf1.GetMetric(), 1)
	require.Equal(t, 1.0, mf1.GetMetric()[0].GetCounter().GetValue())

	close(stepCh)

	var mf2 dto.MetricFamily
	require.NoError(t, dec.Decode(&mf2))
	require.Equal(t, "second", mf2.GetName())
	require.Equal(t, dto.MetricType_GAUGE, mf2.GetType())
	require.Len(t, mf2.GetMetric(), 1)
	require.Equal(t, 2.0, mf2.GetMetric()[0].GetGauge().GetValue())

	var mf3 dto.MetricFamily
	require.ErrorIs(t, dec.Decode(&mf3), io.EOF)
	require.NoError(t, <-errCh)
}
