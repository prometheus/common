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
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"

	"github.com/prometheus/common/model"
)

func TestDecoderDuplicateBucketsAndQuantiles(t *testing.T) {
	for _, tc := range []struct {
		name       string
		metricType dto.MetricType
		values     []float64
		wantErr    string
	}{
		{
			name:       "histogram duplicate finite bound",
			metricType: dto.MetricType_HISTOGRAM,
			values:     []float64{1, 2, 1},
			wantErr:    "duplicate bucket upper bound 1",
		},
		{
			name:       "histogram duplicate infinity bound",
			metricType: dto.MetricType_HISTOGRAM,
			values:     []float64{1, math.Inf(1), math.Inf(1)},
			wantErr:    "duplicate bucket upper bound +Inf",
		},
		{
			name:       "histogram signed zero bounds",
			metricType: dto.MetricType_HISTOGRAM,
			values:     []float64{0, math.Copysign(0, -1)},
			wantErr:    "duplicate bucket upper bound",
		},
		{
			name:       "gauge histogram duplicate bound",
			metricType: dto.MetricType_GAUGE_HISTOGRAM,
			values:     []float64{1, 1},
			wantErr:    "duplicate bucket upper bound 1",
		},
		{
			name:       "histogram distinct unsorted bounds",
			metricType: dto.MetricType_HISTOGRAM,
			values:     []float64{2, 1, math.Inf(1)},
		},
		{
			name:       "summary duplicate quantile",
			metricType: dto.MetricType_SUMMARY,
			values:     []float64{0.5, 0.9, 0.5},
			wantErr:    "duplicate quantile 0.5",
		},
		{
			name:       "summary distinct unsorted quantiles",
			metricType: dto.MetricType_SUMMARY,
			values:     []float64{0.9, 0.5},
		},
		{
			name:       "summary no quantiles",
			metricType: dto.MetricType_SUMMARY,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			family := duplicateTestFamily(tc.metricType, tc.values)
			var text bytes.Buffer
			_, err := MetricFamilyToText(&text, family)
			require.NoError(t, err)

			var binary bytes.Buffer
			_, err = protodelim.MarshalTo(&binary, family)
			require.NoError(t, err)

			for _, input := range []struct {
				name   string
				format Format
				data   []byte
			}{
				{"text", FmtText, text.Bytes()},
				{"protobuf", FmtProtoDelim, binary.Bytes()},
			} {
				t.Run(input.name, func(t *testing.T) {
					dec := NewDecoder(bytes.NewReader(input.data), input.format)
					var got dto.MetricFamily
					err := dec.Decode(&got)
					if tc.wantErr != "" {
						require.ErrorContains(t, err, tc.wantErr)
						require.ErrorContains(t, err, `metric family "request_duration_seconds"`)
					} else {
						require.NoError(t, err)
						require.Len(t, got.GetMetric(), 1)
						if input.format == FmtProtoDelim {
							require.True(t, proto.Equal(family, &got))
						}
					}
					require.ErrorIs(t, dec.Decode(&got), io.EOF)
				})
			}
		})
	}
}

func TestDecoderBoundsAreScopedToEachMetric(t *testing.T) {
	for _, metricType := range []dto.MetricType{dto.MetricType_HISTOGRAM, dto.MetricType_SUMMARY} {
		t.Run(metricType.String(), func(t *testing.T) {
			family := duplicateTestFamily(metricType, []float64{0.5, 1})
			first := family.Metric[0]
			first.Label = []*dto.LabelPair{{Name: proto.String("method"), Value: proto.String("GET")}}
			second := proto.Clone(first).(*dto.Metric)
			second.Label[0].Value = proto.String("POST")
			family.Metric = append(family.Metric, second)

			for _, format := range []Format{FmtText, FmtProtoDelim} {
				t.Run(string(format), func(t *testing.T) {
					var data bytes.Buffer
					if format == FmtText {
						_, err := MetricFamilyToText(&data, family)
						require.NoError(t, err)
					} else {
						_, err := protodelim.MarshalTo(&data, family)
						require.NoError(t, err)
					}
					dec := NewDecoder(&data, format)
					var got dto.MetricFamily
					require.NoError(t, dec.Decode(&got))
					require.Len(t, got.Metric, 2)
				})
			}
		})
	}
}

func TestSampleDecoderRejectsDuplicateBuckets(t *testing.T) {
	dec := SampleDecoder{
		Dec: NewDecoder(strings.NewReader(`# TYPE request_duration_seconds histogram
request_duration_seconds_bucket{le="1"} 2
request_duration_seconds_bucket{le="1.0"} 3
request_duration_seconds_sum 4
request_duration_seconds_count 3
`), FmtText),
		Opts: &DecodeOptions{},
	}
	var samples model.Vector
	require.ErrorContains(t, dec.Decode(&samples), "duplicate bucket upper bound 1")
	require.Empty(t, samples)
}

func TestExtractSamplesSkipsFamilyWithDuplicateBounds(t *testing.T) {
	for _, metricType := range []dto.MetricType{dto.MetricType_HISTOGRAM, dto.MetricType_SUMMARY} {
		t.Run(metricType.String(), func(t *testing.T) {
			invalid := duplicateTestFamily(metricType, []float64{0.5, 0.5})
			valid := &dto.MetricFamily{
				Name: proto.String("requests_total"), Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{{Counter: &dto.Counter{Value: proto.Float64(7)}}},
			}
			samples, err := ExtractSamples(&DecodeOptions{}, invalid, valid)
			require.ErrorContains(t, err, "duplicate")
			require.Len(t, samples, 1)
			require.Equal(t, model.SampleValue(7), samples[0].Value)
			require.Equal(t, model.LabelValue("requests_total"), samples[0].Metric[model.MetricNameLabel])
		})
	}
}

func duplicateTestFamily(metricType dto.MetricType, values []float64) *dto.MetricFamily {
	metric := &dto.Metric{}
	if metricType == dto.MetricType_SUMMARY {
		metric.Summary = &dto.Summary{SampleCount: proto.Uint64(3), SampleSum: proto.Float64(4)}
		for i, value := range values {
			metric.Summary.Quantile = append(metric.Summary.Quantile, &dto.Quantile{
				Quantile: proto.Float64(value), Value: proto.Float64(float64(i + 1)),
			})
		}
	} else {
		metric.Histogram = &dto.Histogram{SampleCount: proto.Uint64(3), SampleSum: proto.Float64(4)}
		for i, value := range values {
			metric.Histogram.Bucket = append(metric.Histogram.Bucket, &dto.Bucket{
				UpperBound: proto.Float64(value), CumulativeCount: proto.Uint64(uint64(i + 1)),
			})
		}
	}
	return &dto.MetricFamily{
		Name: proto.String("request_duration_seconds"), Type: metricType.Enum(), Metric: []*dto.Metric{metric},
	}
}
