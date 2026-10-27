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

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/prometheus/common/model"
)

func decodeOpenMetrics20ToMap(r io.Reader, format Format) (map[string]*dto.MetricFamily, error) {
	dec := NewDecoder(r, format)
	fams := make(map[string]*dto.MetricFamily)
	for {
		mf := &dto.MetricFamily{}
		if err := dec.Decode(mf); err != nil {
			if errors.Is(err, io.EOF) {
				return fams, nil
			}
			return nil, err
		}
		if _, exists := fams[mf.GetName()]; exists {
			return nil, errors.New("duplicate MetricFamily " + mf.GetName())
		}
		fams[mf.GetName()] = mf
	}
}

func requireEqualFamilies(t *testing.T, expected, actual map[string]*dto.MetricFamily) {
	t.Helper()
	require.Len(t, actual, len(expected))
	for name, want := range expected {
		got, ok := actual[name]
		require.Truef(t, ok, "missing metric family %q", name)
		assertProtoEqual(t, want, got, "metric family %q mismatch", name)
	}
}

func TestOpenMetrics2Parse(t *testing.T) {
	input := `# HELP go_gc_duration_seconds A summary of GC invocation durations.
# TYPE go_gc_duration_seconds summary
# UNIT go_gc_duration_seconds seconds
go_gc_duration_seconds {count:99,sum:0.004,quantile:[0:4.9351e-05,0.5:8.3835e-05]}
# TYPE go_goroutines gauge
go_goroutines 33 123456.789
# TYPE hh histogram
hh {count:1,sum:1.0,bucket:[+Inf:1]}
# TYPE un unknown
un 1
# TYPE foo_total counter
foo_total 17.0 1520879607.789 # {id="counter-test"} 5 1520879600.789
# EOF
`

	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"go_gc_duration_seconds": {
			Name: proto.String("go_gc_duration_seconds"),
			Help: proto.String("A summary of GC invocation durations."),
			Type: dto.MetricType_SUMMARY.Enum(),
			Unit: proto.String("seconds"),
			Metric: []*dto.Metric{
				{
					Summary: &dto.Summary{
						SampleCount: proto.Uint64(99),
						SampleSum:   proto.Float64(0.004),
						Quantile: []*dto.Quantile{
							{Quantile: proto.Float64(0.0), Value: proto.Float64(4.9351e-05)},
							{Quantile: proto.Float64(0.5), Value: proto.Float64(8.3835e-05)},
						},
					},
				},
			},
		},
		"go_goroutines": {
			Name: proto.String("go_goroutines"),
			Type: dto.MetricType_GAUGE.Enum(),
			Metric: []*dto.Metric{
				{
					Gauge:       &dto.Gauge{Value: proto.Float64(33)},
					TimestampMs: proto.Int64(123456789),
				},
			},
		},
		"hh": {
			Name: proto.String("hh"),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount: proto.Uint64(1),
						SampleSum:   proto.Float64(1.0),
						Bucket: []*dto.Bucket{
							{
								CumulativeCount: proto.Uint64(1),
								UpperBound:      proto.Float64(math.Inf(1)),
							},
						},
					},
				},
			},
		},
		"un": {
			Name: proto.String("un"),
			Type: dto.MetricType_UNTYPED.Enum(),
			Metric: []*dto.Metric{
				{
					Untyped: &dto.Untyped{Value: proto.Float64(1)},
				},
			},
		},
		"foo_total": {
			Name: proto.String("foo_total"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Counter: &dto.Counter{
						Value: proto.Float64(17.0),
						Exemplar: &dto.Exemplar{
							Label: []*dto.LabelPair{
								{Name: proto.String("id"), Value: proto.String("counter-test")},
							},
							Value:     proto.Float64(5),
							Timestamp: &timestamppb.Timestamp{Seconds: 1520879600, Nanos: 789000000},
						},
					},
					TimestampMs: proto.Int64(1520879607789),
				},
			},
		},
	}

	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseStartTimestamp(t *testing.T) {
	input := `# TYPE requests_total counter
requests_total 42.0 1000000.0 st@500000.0
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"requests_total": {
			Name: proto.String("requests_total"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Counter: &dto.Counter{
						Value:            proto.Float64(42.0),
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 500000, Nanos: 0},
					},
					TimestampMs: proto.Int64(1000000000),
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseStartTimestampNoMainTimestamp(t *testing.T) {
	input := `# TYPE requests_total counter
requests_total 42.0 st@500000.0
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"requests_total": {
			Name: proto.String("requests_total"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Counter: &dto.Counter{
						Value:            proto.Float64(42.0),
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 500000, Nanos: 0},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseMultipleExemplars(t *testing.T) {
	input := `# TYPE foo_total counter
foo_total 17.0 # {id="a"} 1.0 1234566.0 # {id="b"} 2.0 1234567.0
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	// dto.Counter has a single Exemplar field, so it retains the first exemplar;
	// dto.Histogram has an Exemplars slice that holds all exemplars.
	expected := map[string]*dto.MetricFamily{
		"foo_total": {
			Name: proto.String("foo_total"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Counter: &dto.Counter{
						Value: proto.Float64(17.0),
						Exemplar: &dto.Exemplar{
							Label:     []*dto.LabelPair{{Name: proto.String("id"), Value: proto.String("a")}},
							Value:     proto.Float64(1.0),
							Timestamp: &timestamppb.Timestamp{Seconds: 1234566, Nanos: 0},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseExemplarMultipleLabels(t *testing.T) {
	input := `# TYPE http_requests_total counter
http_requests_total 1027 1710000000.0 # {trace_id="abc123",span_id="def456"} 1.0 1709999999.0
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"http_requests_total": {
			Name: proto.String("http_requests_total"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Counter: &dto.Counter{
						Value: proto.Float64(1027),
						Exemplar: &dto.Exemplar{
							Label: []*dto.LabelPair{
								{Name: proto.String("trace_id"), Value: proto.String("abc123")},
								{Name: proto.String("span_id"), Value: proto.String("def456")},
							},
							Value:     proto.Float64(1.0),
							Timestamp: &timestamppb.Timestamp{Seconds: 1709999999, Nanos: 0},
						},
					},
					TimestampMs: proto.Int64(1710000000000),
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseExemplarExceedsOM1Limit(t *testing.T) {
	longValue := strings.Repeat("x", 200)
	input := `# TYPE foo_total counter
foo_total 1.0 # {data="` + longValue + `"} 1.0 1234567.0
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"foo_total": {
			Name: proto.String("foo_total"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Counter: &dto.Counter{
						Value: proto.Float64(1.0),
						Exemplar: &dto.Exemplar{
							Label: []*dto.LabelPair{
								{Name: proto.String("data"), Value: proto.String(longValue)},
							},
							Value:     proto.Float64(1.0),
							Timestamp: &timestamppb.Timestamp{Seconds: 1234567, Nanos: 0},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseExemplarQuotedUTF8LabelKey(t *testing.T) {
	input := `# TYPE foo_total counter
foo_total 17.0 # {"my.id"="abc"} 5 1520879600.789
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"foo_total": {
			Name: proto.String("foo_total"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Counter: &dto.Counter{
						Value: proto.Float64(17.0),
						Exemplar: &dto.Exemplar{
							Label: []*dto.LabelPair{
								{Name: proto.String("my.id"), Value: proto.String("abc")},
							},
							Value:     proto.Float64(5),
							Timestamp: &timestamppb.Timestamp{Seconds: 1520879600, Nanos: 789000000},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseCompositeClassicGaugeHistogram(t *testing.T) {
	input := `# TYPE foo gaugehistogram
foo {gcount:42,gsum:3289.3,bucket:[0.01:20,0.1:25,1:34,+Inf:42]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"foo": {
			Name: proto.String("foo"),
			Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount: proto.Uint64(42),
						SampleSum:   proto.Float64(3289.3),
						Bucket: []*dto.Bucket{
							{CumulativeCount: proto.Uint64(20), UpperBound: proto.Float64(0.01)},
							{CumulativeCount: proto.Uint64(25), UpperBound: proto.Float64(0.1)},
							{CumulativeCount: proto.Uint64(34), UpperBound: proto.Float64(1)},
							{CumulativeCount: proto.Uint64(42), UpperBound: proto.Float64(math.Inf(1))},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseCompositeSummary(t *testing.T) {
	input := `# HELP rpc_duration_seconds RPC duration.
# TYPE rpc_duration_seconds summary
rpc_duration_seconds {count:100,sum:30000.0,quantile:[0.5:100.0,0.9:200.0,0.99:300.0]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"rpc_duration_seconds": {
			Name: proto.String("rpc_duration_seconds"),
			Help: proto.String("RPC duration."),
			Type: dto.MetricType_SUMMARY.Enum(),
			Metric: []*dto.Metric{
				{
					Summary: &dto.Summary{
						SampleCount: proto.Uint64(100),
						SampleSum:   proto.Float64(30000.0),
						Quantile: []*dto.Quantile{
							{Quantile: proto.Float64(0.5), Value: proto.Float64(100.0)},
							{Quantile: proto.Float64(0.9), Value: proto.Float64(200.0)},
							{Quantile: proto.Float64(0.99), Value: proto.Float64(300.0)},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseCompositeClassicHistogram(t *testing.T) {
	input := `# HELP req_duration Request duration histogram.
# TYPE req_duration histogram
req_duration {count:3,sum:6.0,bucket:[0.1:1,1.0:2,+Inf:3]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"req_duration": {
			Name: proto.String("req_duration"),
			Help: proto.String("Request duration histogram."),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount: proto.Uint64(3),
						SampleSum:   proto.Float64(6.0),
						Bucket: []*dto.Bucket{
							{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.1)},
							{CumulativeCount: proto.Uint64(2), UpperBound: proto.Float64(1.0)},
							{CumulativeCount: proto.Uint64(3), UpperBound: proto.Float64(math.Inf(1))},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseClassicHistogramNegInfBucket(t *testing.T) {
	input := `# TYPE req_duration histogram
req_duration {count:3,sum:6.0,bucket:[-Inf:1,1.0:2,+Inf:3]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"req_duration": {
			Name: proto.String("req_duration"),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount: proto.Uint64(3),
						SampleSum:   proto.Float64(6.0),
						Bucket: []*dto.Bucket{
							{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(math.Inf(-1))},
							{CumulativeCount: proto.Uint64(2), UpperBound: proto.Float64(1.0)},
							{CumulativeCount: proto.Uint64(3), UpperBound: proto.Float64(math.Inf(1))},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseCompositeNativeAndClassicHistogram(t *testing.T) {
	input := `# HELP req_duration Request duration histogram with both representations.
# TYPE req_duration histogram
req_duration {count:3,sum:6.0,schema:0,zero_threshold:0.001,zero_count:0,positive_spans:[0:2],positive_buckets:[1,2],bucket:[0.1:1,1.0:2,+Inf:3]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"req_duration": {
			Name: proto.String("req_duration"),
			Help: proto.String("Request duration histogram with both representations."),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount:   proto.Uint64(3),
						SampleSum:     proto.Float64(6.0),
						Schema:        proto.Int32(0),
						ZeroThreshold: proto.Float64(0.001),
						ZeroCount:     proto.Uint64(0),
						PositiveSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(2)},
						},
						PositiveDelta: []int64{1, 1},
						Bucket: []*dto.Bucket{
							{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.1)},
							{CumulativeCount: proto.Uint64(2), UpperBound: proto.Float64(1.0)},
							{CumulativeCount: proto.Uint64(3), UpperBound: proto.Float64(math.Inf(1))},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseNativeHistogram(t *testing.T) {
	input := `# HELP test_histogram Native histogram.
# TYPE test_histogram histogram
test_histogram {count:6,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,negative_spans:[],negative_buckets:[],positive_spans:[0:3],positive_buckets:[1,2,1]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"test_histogram": {
			Name: proto.String("test_histogram"),
			Help: proto.String("Native histogram."),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount:   proto.Uint64(6),
						SampleSum:     proto.Float64(12.1),
						Schema:        proto.Int32(0),
						ZeroThreshold: proto.Float64(0.001),
						ZeroCount:     proto.Uint64(2),
						PositiveSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(3)},
						},
						PositiveDelta: []int64{1, 1, -1},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)

	t.Run("multiple positive and negative spans", func(t *testing.T) {
		multiSpanInput := `# TYPE multi_span histogram
multi_span {count:8,sum:15.0,schema:0,zero_threshold:0.001,zero_count:1,negative_spans:[-2:1,1:1],negative_buckets:[1,2],positive_spans:[0:2,1:2],positive_buckets:[1,1,1,1]}
# EOF
`
		got, err := decodeOpenMetrics20ToMap(strings.NewReader(multiSpanInput), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)
		requireEqualFamilies(t, map[string]*dto.MetricFamily{
			"multi_span": {
				Name: proto.String("multi_span"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Histogram: &dto.Histogram{
							SampleCount:   proto.Uint64(8),
							SampleSum:     proto.Float64(15.0),
							Schema:        proto.Int32(0),
							ZeroThreshold: proto.Float64(0.001),
							ZeroCount:     proto.Uint64(1),
							NegativeSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(-2), Length: proto.Uint32(1)},
								{Offset: proto.Int32(1), Length: proto.Uint32(1)},
							},
							NegativeDelta: []int64{1, 1},
							PositiveSpan: []*dto.BucketSpan{
								{Offset: proto.Int32(0), Length: proto.Uint32(2)},
								{Offset: proto.Int32(1), Length: proto.Uint32(2)},
							},
							PositiveDelta: []int64{1, 0, 0, 0},
						},
					},
				},
			},
		}, got)
	})
}

func TestOpenMetrics2ParseNativeHistogramOmittedEmptyFields(t *testing.T) {
	input := `# TYPE test_histogram histogram
test_histogram {count:6,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:3],positive_buckets:[1,2,1]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"test_histogram": {
			Name: proto.String("test_histogram"),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount:   proto.Uint64(6),
						SampleSum:     proto.Float64(12.1),
						Schema:        proto.Int32(0),
						ZeroThreshold: proto.Float64(0.001),
						ZeroCount:     proto.Uint64(2),
						PositiveSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(3)},
						},
						PositiveDelta: []int64{1, 1, -1},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseNativeHistogramOmittedPositiveFields(t *testing.T) {
	input := `# TYPE test_histogram histogram
test_histogram {count:6,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,negative_spans:[0:3],negative_buckets:[1,2,1]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"test_histogram": {
			Name: proto.String("test_histogram"),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount:   proto.Uint64(6),
						SampleSum:     proto.Float64(12.1),
						Schema:        proto.Int32(0),
						ZeroThreshold: proto.Float64(0.001),
						ZeroCount:     proto.Uint64(2),
						NegativeSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(3)},
						},
						NegativeDelta: []int64{1, 1, -1},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseUTF8MetricName(t *testing.T) {
	input := `# TYPE "metric.name" gauge
{"metric.name",label="value"} 1.0
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"metric.name": {
			Name: proto.String("metric.name"),
			Type: dto.MetricType_GAUGE.Enum(),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("label"), Value: proto.String("value")},
					},
					Gauge: &dto.Gauge{Value: proto.Float64(1.0)},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseUnknown(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		exp   map[string]*dto.MetricFamily
	}{
		{
			name: "explicit help and type",
			input: `# HELP un Some unknown metric.
# TYPE un unknown
un 1
# EOF
`,
			exp: map[string]*dto.MetricFamily{
				"un": {
					Name: proto.String("un"),
					Help: proto.String("Some unknown metric."),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{Untyped: &dto.Untyped{Value: proto.Float64(1)}},
					},
				},
			},
		},
		{
			name: "no type declaration",
			input: `bare_metric 42
# EOF
`,
			exp: map[string]*dto.MetricFamily{
				"bare_metric": {
					Name: proto.String("bare_metric"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						{Untyped: &dto.Untyped{Value: proto.Float64(42)}},
					},
				},
			},
		},
		{
			name: "explicit type and unit",
			input: `# TYPE un_seconds unknown
# UNIT un_seconds seconds
un_seconds 1.5
# EOF
`,
			exp: map[string]*dto.MetricFamily{
				"un_seconds": {
					Name: proto.String("un_seconds"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Unit: proto.String("seconds"),
					Metric: []*dto.Metric{
						{Untyped: &dto.Untyped{Value: proto.Float64(1.5)}},
					},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fams, err := decodeOpenMetrics20ToMap(strings.NewReader(tc.input), FmtOpenMetrics_2_0_0)
			require.NoError(t, err)
			requireEqualFamilies(t, tc.exp, fams)
		})
	}
}

func TestOpenMetrics2ParseCompositeExtraLabels(t *testing.T) {
	t.Run("UTF-8 metric name", func(t *testing.T) {
		input := `# TYPE "req.duration" histogram
{"req.duration",job="api"} {count:3,sum:6.0,bucket:[0.1:1,+Inf:3]}
# EOF
`
		fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)

		expected := map[string]*dto.MetricFamily{
			"req.duration": {
				Name: proto.String("req.duration"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("job"), Value: proto.String("api")},
						},
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(3),
							SampleSum:   proto.Float64(6.0),
							Bucket: []*dto.Bucket{
								{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.1)},
								{CumulativeCount: proto.Uint64(3), UpperBound: proto.Float64(math.Inf(1))},
							},
						},
					},
				},
			},
		}
		requireEqualFamilies(t, expected, fams)
	})

	t.Run("multiple labels", func(t *testing.T) {
		input := `# TYPE req_duration histogram
req_duration{job="api",env="prod"} {count:3,sum:6.0,bucket:[0.1:1,1.0:2,+Inf:3]}
# EOF
`
		fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)

		expected := map[string]*dto.MetricFamily{
			"req_duration": {
				Name: proto.String("req_duration"),
				Type: dto.MetricType_HISTOGRAM.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("job"), Value: proto.String("api")},
							{Name: proto.String("env"), Value: proto.String("prod")},
						},
						Histogram: &dto.Histogram{
							SampleCount: proto.Uint64(3),
							SampleSum:   proto.Float64(6.0),
							Bucket: []*dto.Bucket{
								{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(0.1)},
								{CumulativeCount: proto.Uint64(2), UpperBound: proto.Float64(1.0)},
								{CumulativeCount: proto.Uint64(3), UpperBound: proto.Float64(math.Inf(1))},
							},
						},
					},
				},
			},
		}
		requireEqualFamilies(t, expected, fams)
	})

	t.Run("quoted UTF-8 label with equals", func(t *testing.T) {
		input := `# TYPE foo summary
foo{"a=b"="v",job="api"} {count:1,sum:2.0,quantile:[0.5:1.0]}
# EOF
`
		fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)

		expected := map[string]*dto.MetricFamily{
			"foo": {
				Name: proto.String("foo"),
				Type: dto.MetricType_SUMMARY.Enum(),
				Metric: []*dto.Metric{
					{
						Label: []*dto.LabelPair{
							{Name: proto.String("a=b"), Value: proto.String("v")},
							{Name: proto.String("job"), Value: proto.String("api")},
						},
						Summary: &dto.Summary{
							SampleCount: proto.Uint64(1),
							SampleSum:   proto.Float64(2.0),
							Quantile: []*dto.Quantile{
								{Quantile: proto.Float64(0.5), Value: proto.Float64(1.0)},
							},
						},
					},
				},
			},
		}
		requireEqualFamilies(t, expected, fams)
	})
}

func TestOpenMetrics2ParseCompositeWithTimestamp(t *testing.T) {
	input := `# TYPE req_duration histogram
req_duration {count:2,sum:4.0,bucket:[+Inf:2]} 1234567.0 st@1000.0
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"req_duration": {
			Name: proto.String("req_duration"),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount:      proto.Uint64(2),
						SampleSum:        proto.Float64(4.0),
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1000, Nanos: 0},
						Bucket: []*dto.Bucket{
							{CumulativeCount: proto.Uint64(2), UpperBound: proto.Float64(math.Inf(1))},
						},
					},
					TimestampMs: proto.Int64(1234567000),
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseFloatHistogram(t *testing.T) {
	input := `# TYPE test_histogram histogram
test_histogram {count:5.5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2.5,negative_spans:[],negative_buckets:[],positive_spans:[0:2],positive_buckets:[2.0,1.0]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"test_histogram": {
			Name: proto.String("test_histogram"),
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
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseNativeHistogramIntVsFloatDiscriminator(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		hist  *dto.Histogram
	}{
		{
			name:  "all integer fields -> integer Histogram",
			input: `h {count:5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:2],positive_buckets:[2,1]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCount: proto.Uint64(2), SampleCount: proto.Uint64(5), SampleSum: proto.Float64(12.1),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveDelta: []int64{2, -1},
			},
		},
		{
			name:  "fractional count -> FloatHistogram",
			input: `h {count:5.5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:2],positive_buckets:[2,1]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCountFloat: proto.Float64(2), SampleCountFloat: proto.Float64(5.5), SampleSum: proto.Float64(12.1),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveCount: []float64{2, 1},
			},
		},
		{
			name:  "fractional zero_count -> FloatHistogram",
			input: `h {count:5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2.5,positive_spans:[0:2],positive_buckets:[2,1]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCountFloat: proto.Float64(2.5), SampleCountFloat: proto.Float64(5), SampleSum: proto.Float64(12.1),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveCount: []float64{2, 1},
			},
		},
		{
			name:  "fractional positive bucket -> FloatHistogram",
			input: `h {count:5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:2],positive_buckets:[2.0,1.0]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCountFloat: proto.Float64(2), SampleCountFloat: proto.Float64(5), SampleSum: proto.Float64(12.1),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveCount: []float64{2.0, 1.0},
			},
		},
		{
			name:  "fractional negative bucket -> FloatHistogram",
			input: `h {count:5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,negative_spans:[0:2],negative_buckets:[2.0,1.0]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCountFloat: proto.Float64(2), SampleCountFloat: proto.Float64(5), SampleSum: proto.Float64(12.1),
				NegativeSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				NegativeCount: []float64{2.0, 1.0},
			},
		},
		{
			name:  "exponent notation in bucket -> FloatHistogram",
			input: `h {count:5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:2],positive_buckets:[1e2,1]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCountFloat: proto.Float64(2), SampleCountFloat: proto.Float64(5), SampleSum: proto.Float64(12.1),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveCount: []float64{1e2, 1},
			},
		},
		{
			name:  "count with no uint64 representation -> FloatHistogram",
			input: `h {count:1e20,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:2],positive_buckets:[2,1]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCountFloat: proto.Float64(2), SampleCountFloat: proto.Float64(1e20), SampleSum: proto.Float64(12.1),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveCount: []float64{2, 1},
			},
		},
		{
			name:  "infinite count -> FloatHistogram",
			input: `h {count:+Inf,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:2],positive_buckets:[2,1]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCountFloat: proto.Float64(2), SampleCountFloat: proto.Float64(math.Inf(1)), SampleSum: proto.Float64(12.1),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveCount: []float64{2, 1},
			},
		},
		{
			name:  "zero_count with no uint64 representation -> FloatHistogram",
			input: `h {count:5,sum:12.1,schema:0,zero_threshold:0.001,zero_count:1e20,positive_spans:[0:2],positive_buckets:[2,1]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCountFloat: proto.Float64(1e20), SampleCountFloat: proto.Float64(5), SampleSum: proto.Float64(12.1),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveCount: []float64{2, 1},
			},
		},
		{
			name:  "fractional sum only -> integer Histogram (sum does not discriminate)",
			input: `h {count:5,sum:12.5,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:2],positive_buckets:[2,1]}`,
			hist: &dto.Histogram{
				Schema: proto.Int32(0), ZeroThreshold: proto.Float64(0.001),
				ZeroCount: proto.Uint64(2), SampleCount: proto.Uint64(5), SampleSum: proto.Float64(12.5),
				PositiveSpan:  []*dto.BucketSpan{{Offset: proto.Int32(0), Length: proto.Uint32(2)}},
				PositiveDelta: []int64{2, -1},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := "# TYPE h histogram\n" + tc.input + "\n# EOF\n"
			fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
			require.NoError(t, err)
			expected := map[string]*dto.MetricFamily{
				"h": {
					Name:   proto.String("h"),
					Type:   dto.MetricType_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{{Histogram: tc.hist}},
				},
			}
			requireEqualFamilies(t, expected, fams)
		})
	}
}

func TestOpenMetrics2ParseCompositeGaugeHistogram(t *testing.T) {
	input := `# TYPE req_size gaugehistogram
req_size {gcount:6,gsum:100.0,schema:0,zero_threshold:0.001,zero_count:1,negative_spans:[],negative_buckets:[],positive_spans:[0:2],positive_buckets:[3,2]}
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"req_size": {
			Name: proto.String("req_size"),
			Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount:   proto.Uint64(6),
						SampleSum:     proto.Float64(100.0),
						Schema:        proto.Int32(0),
						ZeroThreshold: proto.Float64(0.001),
						ZeroCount:     proto.Uint64(1),
						PositiveSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(2)},
						},
						PositiveDelta: []int64{3, -1},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseExemplarOnCompositeLine(t *testing.T) {
	input := `# TYPE req_duration histogram
req_duration {count:2,sum:4.0,bucket:[1.0:1,+Inf:2]} # {id="req-1"} 3.8 9999999.0
# EOF
`
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"req_duration": {
			Name: proto.String("req_duration"),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount: proto.Uint64(2),
						SampleSum:   proto.Float64(4.0),
						Bucket: []*dto.Bucket{
							{CumulativeCount: proto.Uint64(1), UpperBound: proto.Float64(1.0)},
							{CumulativeCount: proto.Uint64(2), UpperBound: proto.Float64(math.Inf(1))},
						},
						Exemplars: []*dto.Exemplar{
							{
								Label: []*dto.LabelPair{
									{Name: proto.String("id"), Value: proto.String("req-1")},
								},
								Value:     proto.Float64(3.8),
								Timestamp: &timestamppb.Timestamp{Seconds: 9999999, Nanos: 0},
							},
						},
					},
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseDescriptorOrdering(t *testing.T) {
	t.Run("TYPE before HELP and UNIT", func(t *testing.T) {
		input := `# TYPE open_fds gauge
# HELP open_fds Number of open file descriptors.
# UNIT open_fds fds
open_fds 8.0
# EOF
`
		fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)

		expected := map[string]*dto.MetricFamily{
			"open_fds": {
				Name:   proto.String("open_fds"),
				Help:   proto.String("Number of open file descriptors."),
				Type:   dto.MetricType_GAUGE.Enum(),
				Unit:   proto.String("fds"),
				Metric: []*dto.Metric{{Gauge: &dto.Gauge{Value: proto.Float64(8.0)}}},
			},
		}
		requireEqualFamilies(t, expected, fams)
	})

	t.Run("UNIT before TYPE", func(t *testing.T) {
		input := `# UNIT http_requests requests
# TYPE http_requests counter
http_requests 1.0
# EOF
`
		fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)

		expected := map[string]*dto.MetricFamily{
			"http_requests": {
				Name:   proto.String("http_requests"),
				Type:   dto.MetricType_COUNTER.Enum(),
				Unit:   proto.String("requests"),
				Metric: []*dto.Metric{{Counter: &dto.Counter{Value: proto.Float64(1.0)}}},
			},
		}
		requireEqualFamilies(t, expected, fams)
	})
}

func TestOpenMetrics2ParseFamilyReset(t *testing.T) {
	t.Run("undeclared metric after declared metric", func(t *testing.T) {
		input := `# TYPE http_requests counter
# UNIT http_requests requests
http_requests 1.0
bare_metric 2.0
# EOF
`
		fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)

		expected := map[string]*dto.MetricFamily{
			"http_requests": {
				Name:   proto.String("http_requests"),
				Type:   dto.MetricType_COUNTER.Enum(),
				Unit:   proto.String("requests"),
				Metric: []*dto.Metric{{Counter: &dto.Counter{Value: proto.Float64(1.0)}}},
			},
			"bare_metric": {
				Name:   proto.String("bare_metric"),
				Type:   dto.MetricType_UNTYPED.Enum(),
				Metric: []*dto.Metric{{Untyped: &dto.Untyped{Value: proto.Float64(2.0)}}},
			},
		}
		requireEqualFamilies(t, expected, fams)
	})

	t.Run("suffix mismatch reverts to untyped", func(t *testing.T) {
		input := `# TYPE foo counter
foo_total 1.0
# EOF
`
		fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)

		// "foo" has no samples so it is omitted; "foo_total" is parsed as its own untyped family.
		expected := map[string]*dto.MetricFamily{
			"foo_total": {
				Name:   proto.String("foo_total"),
				Type:   dto.MetricType_UNTYPED.Enum(),
				Metric: []*dto.Metric{{Untyped: &dto.Untyped{Value: proto.Float64(1.0)}}},
			},
		}
		requireEqualFamilies(t, expected, fams)
	})
}

func TestOpenMetrics2ParseExemplarEscapedLabelNamesAndNullBytes(t *testing.T) {
	input := "# TYPE foo_total counter\nfoo_total 1.0 1700000000.0 # {\"trace\\\\id\\n\\\"1\\\"\"=\"a\x00b\\nc\"} 1.0 1700000000.5\n# EOF\n"
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"foo_total": {
			Name: proto.String("foo_total"),
			Type: dto.MetricType_COUNTER.Enum(),
			Metric: []*dto.Metric{
				{
					Counter: &dto.Counter{
						Value: proto.Float64(1.0),
						Exemplar: &dto.Exemplar{
							Label: []*dto.LabelPair{
								{Name: proto.String("trace\\id\n\"1\""), Value: proto.String("a\x00b\nc")},
							},
							Value:     proto.Float64(1.0),
							Timestamp: &timestamppb.Timestamp{Seconds: 1700000000, Nanos: 500000000},
						},
					},
					TimestampMs: proto.Int64(1700000000000),
				},
			},
		},
	}
	requireEqualFamilies(t, expected, fams)

	t.Run("NaN and Inf exemplar values", func(t *testing.T) {
		nanInfInput := "# TYPE c_total counter\nc_total{k=\"nan\"} 1.0 # {id=\"1\"} NaN 1700000000.0\nc_total{k=\"inf\"} 2.0 # {id=\"2\"} +Inf 1700000000.0\n# EOF\n"
		got, err := decodeOpenMetrics20ToMap(strings.NewReader(nanInfInput), FmtOpenMetrics_2_0_0)
		require.NoError(t, err)
		require.Len(t, got["c_total"].GetMetric(), 2)
		require.True(t, math.IsNaN(got["c_total"].GetMetric()[0].GetCounter().GetExemplar().GetValue()))
		require.True(t, math.IsInf(got["c_total"].GetMetric()[1].GetCounter().GetExemplar().GetValue(), 1))
	})
}

func TestOpenMetrics2ParseCarriageReturnInHelp(t *testing.T) {
	input := "# HELP foo first\rsecond\nfoo 1\n# EOF\n"
	fams, err := decodeOpenMetrics20ToMap(strings.NewReader(input), FmtOpenMetrics_2_0_0)
	require.NoError(t, err)

	expected := map[string]*dto.MetricFamily{
		"foo": {
			Name:   proto.String("foo"),
			Help:   proto.String("first\rsecond"),
			Type:   dto.MetricType_UNTYPED.Enum(),
			Metric: []*dto.Metric{{Untyped: &dto.Untyped{Value: proto.Float64(1)}}},
		},
	}
	requireEqualFamilies(t, expected, fams)
}

func TestOpenMetrics2ParseUnsupportedTypes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		err   string
	}{
		{
			name: "info metric type",
			input: `# TYPE ii_info info
ii_info{foo="bar"} 1
# EOF
`,
			err: `unsupported metric type "info"`,
		},
		{
			name: "stateset metric type",
			input: `# TYPE ss stateset
ss{ss="foo"} 1
# EOF
`,
			err: `unsupported metric type "stateset"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeOpenMetrics20ToMap(strings.NewReader(tc.input), FmtOpenMetrics_2_0_0)
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestOpenMetrics2ParseErrors(t *testing.T) {
	cases := []struct {
		name  string
		input string
		err   string
	}{
		{
			name:  "missing EOF",
			input: "metric 1.0\n",
			err:   "data does not end with # EOF",
		},
		{
			name:  "extra data after EOF",
			input: "# EOF\nextra\n",
			err:   "unexpected data after # EOF",
		},
		{
			name: "composite value with unknown type",
			input: `foo {count:1,sum:1.0,bucket:[+Inf:1]}
# EOF
`,
			err: "composite value not supported",
		},
		{
			name: "exemplar missing timestamp",
			input: `# TYPE foo_total counter
foo_total 1.0 # {id="x"} 1.0
# EOF
`,
			err: "parse error",
		},
		{
			name:  "histogram missing count",
			input: "# TYPE foo histogram\nfoo {sum:1.0,schema:0,zero_threshold:0,zero_count:0}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "histogram missing sum",
			input: "# TYPE foo histogram\nfoo {count:1,schema:0,zero_threshold:0,zero_count:0}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "histogram missing zero_threshold",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:0,zero_count:0}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "histogram missing zero_count",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:0,zero_threshold:0}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "duplicate timestamp",
			input: "# TYPE requests_total counter\nrequests_total 42.0 1000000.0 2000000.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "duplicate start timestamp",
			input: "# TYPE requests_total counter\nrequests_total 42.0 1000000.0 st@500000.0 st@600000.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "start timestamp before timestamp",
			input: "# TYPE requests_total counter\nrequests_total 42.0 st@500000.0 1000000.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "unsorted quantiles",
			input: "# TYPE foo summary\nfoo {count:3,sum:6.0,quantile:[0.99:1.0,0.5:2.0]}\n# EOF\n",
			err:   "quantiles must be sorted in increasing order",
		},
		{
			name:  "invalid UTF-8 metric name in sample",
			input: "{\"a\xffb\"} 1.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "invalid UTF-8 metric name in TYPE",
			input: "# TYPE \"a\xffb\" counter\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "invalid UTF-8 label name",
			input: "# TYPE foo counter\nfoo{\"a\xffb\"=\"v\"} 1.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "empty quoted metric name in sample",
			input: "{\"\",label=\"v\"} 1.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "empty quoted metric name in TYPE",
			input: "# TYPE \"\" counter\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "metric name not first in braces",
			input: "{label=\"v\",\"m.n\"} 1.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "unsorted classic histogram buckets",
			input: "# TYPE foo histogram\nfoo {count:3,sum:6.0,bucket:[+Inf:3,0.1:1,1.0:2]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "unsorted finite classic histogram buckets",
			input: "# TYPE foo histogram\nfoo {count:3,sum:6.0,bucket:[1.0:2,0.1:1,+Inf:3]}\n# EOF\n",
			err:   "classic histogram buckets must be sorted in increasing order",
		},
		{
			name:  "duplicate composite field",
			input: "# TYPE foo histogram\nfoo {count:1,count:99,sum:2.0,bucket:[+Inf:1]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "unknown composite field on histogram",
			input: "# TYPE foo histogram\nfoo {count:5,sum:2.0,buckets:[1:3,+Inf:5]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "unknown composite field on summary",
			input: "# TYPE foo summary\nfoo {count:1,sum:2.0,quantiles:[0.5:1.0]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "schema > 8",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:9,zero_threshold:0,zero_count:0}\n# EOF\n",
			err:   "schema must be between -4 and 8",
		},
		{
			name:  "schema < -4",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:-5,zero_threshold:0,zero_count:0}\n# EOF\n",
			err:   "schema must be between -4 and 8",
		},
		{
			name:  "schema overflow",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:4294967296,zero_threshold:0,zero_count:0}\n# EOF\n",
			err:   "schema must be between -4 and 8",
		},
		{
			name:  "negative zero_threshold",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:0,zero_threshold:-1,zero_count:0}\n# EOF\n",
			err:   "zero_threshold must be a non-negative, finite number",
		},
		{
			name:  "NaN zero_threshold",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:0,zero_threshold:NaN,zero_count:0}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "+Inf zero_threshold",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:0,zero_threshold:+Inf,zero_count:0}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "scalar value for histogram",
			input: "# TYPE foo histogram\nfoo 5\n# EOF\n",
			err:   "composite value required for metric type",
		},
		{
			name:  "scalar value for gaugehistogram",
			input: "# TYPE foo gaugehistogram\nfoo 5\n# EOF\n",
			err:   "composite value required for metric type",
		},
		{
			name:  "scalar value for summary",
			input: "# TYPE foo summary\nfoo 5\n# EOF\n",
			err:   "composite value required for metric type",
		},
		{
			name:  "classic histogram missing +Inf bucket",
			input: "# TYPE foo histogram\nfoo {count:1,sum:2.0,bucket:[0.1:1]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "counter NaN value",
			input: "# TYPE foo_total counter\nfoo_total NaN\n# EOF\n",
			err:   "counter sample value must not be NaN",
		},
		{
			name:  "negative histogram count",
			input: "# TYPE foo histogram\nfoo {count:-1,sum:NaN,schema:0,zero_threshold:0,zero_count:0,positive_spans:[0:2],positive_buckets:[2,1]}\n# EOF\n",
			err:   "count must not be negative",
		},
		{
			name:  "negative gaugehistogram gcount",
			input: "# TYPE foo gaugehistogram\nfoo {gcount:-1,gsum:NaN,schema:0,zero_threshold:0,zero_count:0,positive_spans:[0:2],positive_buckets:[2,1]}\n# EOF\n",
			err:   "gcount must not be negative",
		},
		{
			name:  "negative zero_count",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1.0,schema:0,zero_threshold:0,zero_count:-1}\n# EOF\n",
			err:   "zero_count must not be negative",
		},
		{
			name:  "negative classic bucket count",
			input: "# TYPE foo histogram\nfoo {count:3,sum:6.0,bucket:[1.0:-1,+Inf:3]}\n# EOF\n",
			err:   "value must not be negative",
		},
		{
			name:  "invalid classic bucket threshold",
			input: "# TYPE foo histogram\nfoo {count:3,sum:6.0,bucket:[abc:1,+Inf:3]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "NaN classic bucket threshold",
			input: "# TYPE foo histogram\nfoo {count:3,sum:6.0,bucket:[NaN:1,+Inf:3]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "-Inf not first bucket",
			input: "# TYPE foo histogram\nfoo {count:3,sum:6.0,bucket:[1.0:1,-Inf:2,+Inf:3]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "invalid summary quantile",
			input: "# TYPE foo summary\nfoo {count:1,sum:2.0,quantile:[abc:1.0]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "summary quantile > 1.0",
			input: "# TYPE foo summary\nfoo {count:1,sum:2.0,quantile:[1.5:1.0]}\n# EOF\n",
			err:   "quantile must be between 0.0 and 1.0",
		},
		{
			name:  "NaN summary quantile",
			input: "# TYPE foo summary\nfoo {count:1,sum:2.0,quantile:[NaN:1.0]}\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "empty quoted label name in sample",
			input: "foo{\"\"=\"val\"} 1.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "empty quoted label name in exemplar",
			input: "# TYPE foo counter\nfoo 1.0 # {\"\"=\"val\"} 1.0 1700000000.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "carriage return in HELP line",
			input: "# HELP foo help text\r\nfoo 1.0\n# EOF\n",
			err:   "unexpected carriage return",
		},
		{
			name:  "carriage return in TYPE line",
			input: "# TYPE foo gauge\r\nfoo 1.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "carriage return in UNIT line",
			input: "# UNIT foo seconds\r\nfoo 1.0\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "carriage return in scalar sample line",
			input: "foo 1.0\r\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "carriage return in composite sample line",
			input: "# TYPE foo histogram\nfoo {count:1,sum:1,bucket:[+Inf:1]}\r\n# EOF\n",
			err:   "parse error",
		},
		{
			name:  "carriage return in EOF line",
			input: "foo 1.0\n# EOF\r\n",
			err:   "parse error",
		},
		{
			name:  "UTF-8 metric name rejected under explicit legacy escaping",
			input: "{\"metric.name\"} 1.0\n# EOF\n",
			err:   `invalid metric name "metric.name"`,
		},
		{
			name:  "interleaved metric families rejected",
			input: "# TYPE a gauge\na{k=\"1\"} 1\n# TYPE b gauge\nb 2\na{k=\"2\"} 3\n# EOF\n",
			err:   `interleaved metric family "a"`,
		},
		{
			name:  "summary float count overflowing uint64 rejected",
			input: "# TYPE s summary\ns {count:18446744073709551616.0,sum:1.0,quantile:[]}\n# EOF\n",
			err:   `invalid summary count "18446744073709551616.0"`,
		},
		{
			name:  "decreasing classic histogram bucket counts rejected",
			input: "# TYPE foo histogram\nfoo {count:3,sum:6.0,bucket:[0.1:2,1.0:1,+Inf:3]}\n# EOF\n",
			err:   "classic bucket counts must be monotonically increasing: 1 < 2",
		},
		{
			name:  "classic histogram +Inf bucket count mismatch rejected",
			input: "# TYPE foo histogram\nfoo {count:5,sum:6.0,bucket:[0.1:1,1.0:2,+Inf:3]}\n# EOF\n",
			err:   "classic bucket +Inf count (3) does not match sample count (5)",
		},
		{
			name:  "native histogram bucket observations mismatch rejected",
			input: "# TYPE foo histogram\nfoo {count:10,sum:12.1,schema:0,zero_threshold:0.001,zero_count:2,positive_spans:[0:2],positive_buckets:[2,1]}\n# EOF\n",
			err:   "native histogram bucket observations (5) do not match sample count (10)",
		},
		{
			name:  "negative native bucket count in gaugehistogram rejected",
			input: "# TYPE foo gaugehistogram\nfoo {gcount:1,gsum:1.0,schema:0,zero_threshold:0,zero_count:0,positive_spans:[0:1],positive_buckets:[-1]}\n# EOF\n",
			err:   `invalid positive_buckets value "-1"`,
		},
		{
			name:  "sample timestamp overflowing int64 ms rejected",
			input: "# TYPE foo gauge\nfoo 1.0 1e20\n# EOF\n",
			err:   `invalid timestamp "1e20"`,
		},
		{
			name:  "start timestamp out of protobuf range rejected",
			input: "# TYPE foo_total counter\nfoo_total 1.0 st@1e20\n# EOF\n",
			err:   `invalid start timestamp "1e20"`,
		},
		{
			name:  "negative_spans and negative_buckets length mismatch",
			input: "# TYPE foo histogram\nfoo {count:2,sum:1.0,schema:0,zero_threshold:0,zero_count:1,negative_spans:[0:2],negative_buckets:[1]}\n# EOF\n",
			err:   "negative_spans length (2) does not match negative_buckets count (1)",
		},
		{
			name:  "positive_spans and positive_buckets length mismatch",
			input: "# TYPE foo histogram\nfoo {count:2,sum:1.0,schema:0,zero_threshold:0,zero_count:1,positive_spans:[0:2],positive_buckets:[1]}\n# EOF\n",
			err:   "positive_spans length (2) does not match positive_buckets count (1)",
		},
		{
			name:  "duplicate TYPE metadata",
			input: "# TYPE foo gauge\n# TYPE foo gauge\nfoo 1\n# EOF\n",
			err:   `second TYPE line for metric name "foo"`,
		},
		{
			name:  "duplicate HELP metadata",
			input: "# HELP foo first\n# HELP foo second\nfoo 1\n# EOF\n",
			err:   `second HELP line for metric name "foo"`,
		},
		{
			name:  "duplicate UNIT metadata",
			input: "# UNIT foo_seconds seconds\n# UNIT foo_seconds seconds\nfoo_seconds 1\n# EOF\n",
			err:   `second UNIT line for metric name "foo_seconds"`,
		},
		{
			name:  "metadata after samples",
			input: "foo 1\n# HELP foo help\n# EOF\n",
			err:   `metadata for metric "foo" after samples`,
		},
		{
			name:  "reserved __name__ label",
			input: "foo{__name__=\"bar\"} 1\n# EOF\n",
			err:   `label name "__name__" is reserved`,
		},
		{
			name:  "duplicate label name",
			input: "foo{k=\"1\",k=\"2\"} 1\n# EOF\n",
			err:   `duplicate label "k" for metric "foo"`,
		},
		{
			name:  "negative counter value",
			input: "# TYPE foo_total counter\nfoo_total -1\n# EOF\n",
			err:   "counter sample value must not be negative",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			format := FmtOpenMetrics_2_0_0
			if strings.Contains(tc.name, "explicit legacy escaping") {
				format = FmtOpenMetrics_2_0_0.WithEscapingScheme(model.ValueEncodingEscaping)
			}
			_, err := decodeOpenMetrics20ToMap(strings.NewReader(tc.input), format)
			require.Error(t, err)
			require.ErrorContains(t, err, tc.err)
		})
	}
}

func TestOpenMetrics2RoundTrip(t *testing.T) {
	families := []*dto.MetricFamily{
		{
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
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890, Nanos: 500000000},
						Exemplar: &dto.Exemplar{
							Label: []*dto.LabelPair{
								{Name: proto.String("trace_id"), Value: proto.String("abc")},
							},
							Value:     proto.Float64(1),
							Timestamp: &timestamppb.Timestamp{Seconds: 1234567891, Nanos: 0},
						},
					},
					TimestampMs: proto.Int64(1234567892000),
				},
			},
		},
		{
			Name: proto.String("node.memory.active_bytes"),
			Help: proto.String("Active memory in bytes."),
			Type: dto.MetricType_GAUGE.Enum(),
			Unit: proto.String("bytes"),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("host.name"), Value: proto.String("node-1")},
					},
					Gauge: &dto.Gauge{Value: proto.Float64(1.2345e+09)},
				},
			},
		},
		{
			Name: proto.String("rpc_duration_seconds"),
			Help: proto.String("RPC duration."),
			Type: dto.MetricType_SUMMARY.Enum(),
			Unit: proto.String("seconds"),
			Metric: []*dto.Metric{
				{
					Summary: &dto.Summary{
						SampleCount:      proto.Uint64(100),
						SampleSum:        proto.Float64(300.5),
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1000, Nanos: 0},
						Quantile: []*dto.Quantile{
							{Quantile: proto.Float64(0.5), Value: proto.Float64(1.2)},
							{Quantile: proto.Float64(0.99), Value: proto.Float64(5.4)},
						},
					},
				},
			},
		},
		{
			Name: proto.String("req_duration_seconds"),
			Help: proto.String("Native and classic histogram."),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Unit: proto.String("seconds"),
			Metric: []*dto.Metric{
				{
					Histogram: &dto.Histogram{
						SampleCount:      proto.Uint64(6),
						SampleSum:        proto.Float64(12.1),
						Schema:           proto.Int32(0),
						ZeroThreshold:    proto.Float64(0.001),
						ZeroCount:        proto.Uint64(2),
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 2000, Nanos: 0},
						PositiveSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(3)},
						},
						PositiveDelta: []int64{1, 1, -1},
						Bucket: []*dto.Bucket{
							{CumulativeCount: proto.Uint64(2), UpperBound: proto.Float64(0.1)},
							{CumulativeCount: proto.Uint64(6), UpperBound: proto.Float64(math.Inf(1))},
						},
						Exemplars: []*dto.Exemplar{
							{
								Label:     []*dto.LabelPair{{Name: proto.String("id"), Value: proto.String("1")}},
								Value:     proto.Float64(0.5),
								Timestamp: &timestamppb.Timestamp{Seconds: 2001, Nanos: 0},
							},
							{
								Label:     []*dto.LabelPair{{Name: proto.String("id"), Value: proto.String("2")}},
								Value:     proto.Float64(1.5),
								Timestamp: &timestamppb.Timestamp{Seconds: 2002, Nanos: 0},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	format := FmtOpenMetrics_2_0_0.WithEscapingScheme(model.NoEscaping)
	enc := NewEncoder(&buf, format)
	for _, mf := range families {
		require.NoError(t, enc.Encode(mf))
	}
	closer, ok := enc.(Closer)
	require.True(t, ok)
	require.NoError(t, closer.Close())

	got, err := decodeOpenMetrics20ToMap(&buf, format)
	require.NoError(t, err)

	expected := make(map[string]*dto.MetricFamily, len(families))
	for _, mf := range families {
		expected[mf.GetName()] = mf
	}
	requireEqualFamilies(t, expected, got)
}

func TestOpenMetrics2Streaming(t *testing.T) {
	pr, pw := io.Pipe()
	t.Cleanup(func() { _ = pr.Close() })

	dec := NewDecoder(pr, FmtOpenMetrics_2_0_0)

	errCh := make(chan error, 1)
	stepCh := make(chan struct{})
	go func() {
		_, err := io.WriteString(pw, "# TYPE first_total counter\nfirst_total 1\n# TYPE second gauge\nsecond 2\n")
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
