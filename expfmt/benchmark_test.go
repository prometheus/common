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
	"math"
	"testing"

	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func benchmarkMetricFamilies() []*dto.MetricFamily {
	return []*dto.MetricFamily{
		{
			Name: proto.String("network_transmit_bytes_total"),
			Help: proto.String("Total number of bytes transmitted over the network."),
			Type: dto.MetricType_COUNTER.Enum(),
			Unit: proto.String("bytes"),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("device"), Value: proto.String("eth0")},
					},
					Counter: &dto.Counter{
						Value:            proto.Float64(1024),
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890},
						Exemplar: &dto.Exemplar{
							Label: []*dto.LabelPair{
								{Name: proto.String("trace_id"), Value: proto.String("1234567890abcdef")},
							},
							Value:     proto.Float64(512),
							Timestamp: &timestamppb.Timestamp{Seconds: 1234567890, Nanos: 500000000},
						},
					},
					TimestampMs: proto.Int64(1234567891000),
				},
			},
		},
		{
			Name: proto.String("process_cpu_seconds_total"),
			Help: proto.String("Total user and system CPU time spent in seconds."),
			Type: dto.MetricType_GAUGE.Enum(),
			Unit: proto.String("seconds"),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("mode"), Value: proto.String("user")},
					},
					Gauge:       &dto.Gauge{Value: proto.Float64(123.45)},
					TimestampMs: proto.Int64(1234567891000),
				},
			},
		},
		{
			Name: proto.String("memory_free_bytes"),
			Help: proto.String("Free memory in bytes."),
			Type: dto.MetricType_UNTYPED.Enum(),
			Unit: proto.String("bytes"),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("numa_node"), Value: proto.String("0")},
					},
					Untyped:     &dto.Untyped{Value: proto.Float64(4294967296.0)},
					TimestampMs: proto.Int64(1234567891000),
				},
			},
		},
		{
			Name: proto.String("rpc_duration_seconds"),
			Help: proto.String("RPC latency summary in seconds."),
			Type: dto.MetricType_SUMMARY.Enum(),
			Unit: proto.String("seconds"),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("service"), Value: proto.String("auth")},
					},
					Summary: &dto.Summary{
						SampleCount:      proto.Uint64(100),
						SampleSum:        proto.Float64(25.5),
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890},
						Quantile: []*dto.Quantile{
							{Quantile: proto.Float64(0.5), Value: proto.Float64(0.12)},
							{Quantile: proto.Float64(0.9), Value: proto.Float64(0.45)},
							{Quantile: proto.Float64(0.99), Value: proto.Float64(0.89)},
						},
					},
					TimestampMs: proto.Int64(1234567891000),
				},
			},
		},
		{
			Name: proto.String("http_request_duration_seconds"),
			Help: proto.String("HTTP request latency in seconds."),
			Type: dto.MetricType_HISTOGRAM.Enum(),
			Unit: proto.String("seconds"),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("handler"), Value: proto.String("query")},
						{Name: proto.String("user_agent"), Value: proto.String("Mozilla/5.0 (with \\backslash, \"quotes\", and\nnewline)")},
						{Name: proto.String("region"), Value: proto.String("Björn-佖佥")},
					},
					Histogram: &dto.Histogram{
						SampleCount:      proto.Uint64(2693),
						SampleSum:        proto.Float64(350.25),
						CreatedTimestamp: &timestamppb.Timestamp{Seconds: 1234567890},
						Schema:           proto.Int32(1),
						ZeroThreshold:    proto.Float64(0.001),
						ZeroCount:        proto.Uint64(2),
						PositiveSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(2)},
						},
						PositiveDelta: []int64{1, 2},
						NegativeSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(1)},
						},
						NegativeDelta: []int64{1},
						Bucket: []*dto.Bucket{
							{UpperBound: proto.Float64(0.1), CumulativeCount: proto.Uint64(123)},
							{
								UpperBound:      proto.Float64(0.2),
								CumulativeCount: proto.Uint64(412),
								Exemplar: &dto.Exemplar{
									Label: []*dto.LabelPair{
										{Name: proto.String("trace_id"), Value: proto.String("abcdef123456")},
									},
									Value:     proto.Float64(0.1152),
									Timestamp: &timestamppb.Timestamp{Seconds: 1234567890},
								},
							},
							{UpperBound: proto.Float64(0.5), CumulativeCount: proto.Uint64(592)},
							{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(2693)},
						},
					},
					TimestampMs: proto.Int64(1234567891000),
				},
			},
		},
		{
			Name: proto.String("queue_latency_seconds"),
			Help: proto.String("Gauge histogram of current queue wait time in seconds."),
			Type: dto.MetricType_GAUGE_HISTOGRAM.Enum(),
			Unit: proto.String("seconds"),
			Metric: []*dto.Metric{
				{
					Label: []*dto.LabelPair{
						{Name: proto.String("queue"), Value: proto.String("priority")},
					},
					Histogram: &dto.Histogram{
						SampleCount:   proto.Uint64(500),
						SampleSum:     proto.Float64(750.5),
						Schema:        proto.Int32(1),
						ZeroThreshold: proto.Float64(0.01),
						ZeroCount:     proto.Uint64(5),
						PositiveSpan: []*dto.BucketSpan{
							{Offset: proto.Int32(0), Length: proto.Uint32(2)},
						},
						PositiveDelta: []int64{10, 15},
						Bucket: []*dto.Bucket{
							{UpperBound: proto.Float64(1.0), CumulativeCount: proto.Uint64(100)},
							{UpperBound: proto.Float64(2.0), CumulativeCount: proto.Uint64(350)},
							{UpperBound: proto.Float64(math.Inf(+1)), CumulativeCount: proto.Uint64(500)},
						},
					},
					TimestampMs: proto.Int64(1234567891000),
				},
			},
		},
	}
}

func BenchmarkConvertMetricFamily(b *testing.B) {
	mfs := benchmarkMetricFamilies()

	for _, mf := range mfs {
		b.Run("TEXT/"+mf.GetType().String(), func(b *testing.B) {
			out := bytes.NewBuffer(make([]byte, 0, 1024))
			for b.Loop() {
				_, err := MetricFamilyToText(out, mf)
				if err != nil {
					b.Fatal(err)
				}
				out.Reset()
			}
		})
		b.Run("OM1.0/"+mf.GetType().String(), func(b *testing.B) {
			out := bytes.NewBuffer(make([]byte, 0, 1024))
			for b.Loop() {
				_, err := MetricFamilyToOpenMetrics(out, mf)
				if err != nil {
					b.Fatal(err)
				}
				out.Reset()
			}
		})
		b.Run("OM2.0/"+mf.GetType().String(), func(b *testing.B) {
			out := bytes.NewBuffer(make([]byte, 0, 1024))
			if _, err := MetricFamilyToOpenMetrics20(out, mf); err != nil {
				b.Skipf("skipping unsupported type: %v", err)
			}
			out.Reset()
			for b.Loop() {
				_, err := MetricFamilyToOpenMetrics20(out, mf)
				if err != nil {
					b.Fatal(err)
				}
				out.Reset()
			}
		})
	}
}
