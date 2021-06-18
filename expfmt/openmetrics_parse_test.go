package expfmt

import (
	"math"
	"strings"
	"testing"

	"github.com/golang/protobuf/proto"

	dto "github.com/prometheus/client_model/go"
)

var openMetricParser OpenMetricsParser

func testOpenMetricsParse(t testing.TB) {
	var scenarios = []struct {
		in  string
		out []*dto.MetricFamily
	}{
		// 0: Counter, timestamp given, no _total suffix.
		{
			in: `# HELP name two-line\n doc  str\\ing
# TYPE name counter
name_total{labelname="val1",basename="basevalue"} 42.0
name_total{labelname="val2",basename="basevalue"} 0.23 1.23456789e+06
# EOF`,
			out: []*dto.MetricFamily{&dto.MetricFamily{
				Name: proto.String("name_total"),
				Help: proto.String("two-line\n doc  str\\ing"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					&dto.Metric{
						Label: []*dto.LabelPair{
							&dto.LabelPair{
								Name:  proto.String("basename"),
								Value: proto.String("basevalue"),
							},
							&dto.LabelPair{
								Name:  proto.String("labelname"),
								Value: proto.String("val1"),
							},
						},
						Counter: &dto.Counter{
							Value: proto.Float64(42),
						},
					},
					&dto.Metric{
						Label: []*dto.LabelPair{
							&dto.LabelPair{
								Name:  proto.String("basename"),
								Value: proto.String("basevalue"),
							},
							&dto.LabelPair{
								Name:  proto.String("labelname"),
								Value: proto.String("val2"),
							},
						},
						Counter: &dto.Counter{
							Value: proto.Float64(.23),
						},
						TimestampMs: proto.Int64(1234567890),
					},
				},
			},
			},
		},
		// 1: Gauge, some escaping required, +Inf as value, multi-byte characters in label values.
		{
			in: `# HELP gauge_name gauge\ndoc\nstr\"ing
# TYPE gauge_name gauge
gauge_name{name_1="val with\nnew line",name_2="val with \\backslash and \"quotes\""} +Inf
gauge_name{name_1="Björn",name_2="佖佥"} 3.14e+42
# EOF`,
			out: []*dto.MetricFamily{
				// 1: Gauge, some escaping required, +Inf as value, multi-byte characters in label values.
				&dto.MetricFamily{
					Name: proto.String("gauge_name"),
					Help: proto.String("gauge\ndoc\nstr\"ing"),
					Type: dto.MetricType_GAUGE.Enum(),
					Metric: []*dto.Metric{
						&dto.Metric{
							Label: []*dto.LabelPair{
								&dto.LabelPair{
									Name:  proto.String("name_1"),
									Value: proto.String("val with\nnew line"),
								},
								&dto.LabelPair{
									Name:  proto.String("name_2"),
									Value: proto.String("val with \\backslash and \"quotes\""),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(math.Inf(+1)),
							},
						},
						&dto.Metric{
							Label: []*dto.LabelPair{
								&dto.LabelPair{
									Name:  proto.String("name_1"),
									Value: proto.String("Björn"),
								},
								&dto.LabelPair{
									Name:  proto.String("name_2"),
									Value: proto.String("佖佥"),
								},
							},
							Gauge: &dto.Gauge{
								Value: proto.Float64(3.14e42),
							},
						},
					},
				},
			},
		},
		// 2: Unknown, no help, one sample with no labels and -Inf as value, another sample with one label.
		{
			in: `# TYPE unknown_name unknown
unknown_name -Inf
unknown_name{name_1="value 1"} -1.23e-45
# EOF`,
			out: []*dto.MetricFamily{
				&dto.MetricFamily{
					Name: proto.String("unknown_name"),
					Type: dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{
						&dto.Metric{
							Untyped: &dto.Untyped{
								Value: proto.Float64(math.Inf(-1)),
							},
						},
						&dto.Metric{
							Label: []*dto.LabelPair{
								&dto.LabelPair{
									Name:  proto.String("name_1"),
									Value: proto.String("value 1"),
								},
							},
							Untyped: &dto.Untyped{
								Value: proto.Float64(-1.23e-45),
							},
						},
					},
				},
			},
		},
		// 3: Summary.
		{
			in: `# HELP summary_name summary docstring
# TYPE summary_name summary
summary_name{quantile="0.5"} -1.23
summary_name{quantile="0.9"} 0.2342354
summary_name{quantile="0.99"} 0.0
summary_name_sum -3.4567
summary_name_count 42
summary_name{name_1="value 1",name_2="value 2",quantile="0.5"} 1.0
summary_name{name_1="value 1",name_2="value 2",quantile="0.9"} 2.0
summary_name{name_1="value 1",name_2="value 2",quantile="0.99"} 3.0
summary_name_sum{name_1="value 1",name_2="value 2"} 2010.1971
summary_name_count{name_1="value 1",name_2="value 2"} 4711
# EOF`,

			out: []*dto.MetricFamily{
				&dto.MetricFamily{
					Name: proto.String("summary_name"),
					Help: proto.String("summary docstring"),
					Type: dto.MetricType_SUMMARY.Enum(),
					Metric: []*dto.Metric{
						&dto.Metric{
							Summary: &dto.Summary{
								SampleCount: proto.Uint64(42),
								SampleSum:   proto.Float64(-3.4567),
								Quantile: []*dto.Quantile{
									&dto.Quantile{
										Quantile: proto.Float64(0.5),
										Value:    proto.Float64(-1.23),
									},
									&dto.Quantile{
										Quantile: proto.Float64(0.9),
										Value:    proto.Float64(.2342354),
									},
									&dto.Quantile{
										Quantile: proto.Float64(0.99),
										Value:    proto.Float64(0),
									},
								},
							},
						},
						&dto.Metric{
							Label: []*dto.LabelPair{
								&dto.LabelPair{
									Name:  proto.String("name_1"),
									Value: proto.String("value 1"),
								},
								&dto.LabelPair{
									Name:  proto.String("name_2"),
									Value: proto.String("value 2"),
								},
							},
							Summary: &dto.Summary{
								SampleCount: proto.Uint64(4711),
								SampleSum:   proto.Float64(2010.1971),
								Quantile: []*dto.Quantile{
									&dto.Quantile{
										Quantile: proto.Float64(0.5),
										Value:    proto.Float64(1),
									},
									&dto.Quantile{
										Quantile: proto.Float64(0.9),
										Value:    proto.Float64(2),
									},
									&dto.Quantile{
										Quantile: proto.Float64(0.99),
										Value:    proto.Float64(3),
									},
								},
							},
						},
					},
				},
			},
		},
		// 4: histogram.
		{
			in: `# HELP request_duration_microseconds The response latency.
# TYPE request_duration_microseconds histogram
request_duration_microseconds_bucket{le="100.0"} 123
request_duration_microseconds_bucket{le="120.0"} 412
request_duration_microseconds_bucket{le="144.0"} 592
request_duration_microseconds_bucket{le="172.8"} 1524
request_duration_microseconds_bucket{le="+Inf"} 2693
request_duration_microseconds_sum 1.7560473e+06
request_duration_microseconds_count 2693
# EOF`,
			out: []*dto.MetricFamily{
				&dto.MetricFamily{
					Name: proto.String("request_duration_microseconds"),
					Help: proto.String("The response latency."),
					Type: dto.MetricType_HISTOGRAM.Enum(),
					Metric: []*dto.Metric{
						&dto.Metric{
							Histogram: &dto.Histogram{
								SampleCount: proto.Uint64(2693),
								SampleSum:   proto.Float64(1756047.3),
								Bucket: []*dto.Bucket{
									&dto.Bucket{
										UpperBound:      proto.Float64(100),
										CumulativeCount: proto.Uint64(123),
									},
									&dto.Bucket{
										UpperBound:      proto.Float64(120),
										CumulativeCount: proto.Uint64(412),
									},
									&dto.Bucket{
										UpperBound:      proto.Float64(144),
										CumulativeCount: proto.Uint64(592),
									},
									&dto.Bucket{
										UpperBound:      proto.Float64(172.8),
										CumulativeCount: proto.Uint64(1524),
									},
									&dto.Bucket{
										UpperBound:      proto.Float64(math.Inf(+1)),
										CumulativeCount: proto.Uint64(2693),
									},
								},
							},
						},
					},
				},
			},
		},
		// 5: Simple Counter.
		{
			in: `# HELP foos Number of foos.
# TYPE foos counter
foos_total 42.0
# EOF`,
			out: []*dto.MetricFamily{
				&dto.MetricFamily{
					Name: proto.String("foos_total"),
					Help: proto.String("Number of foos."),
					Type: dto.MetricType_COUNTER.Enum(),
					Metric: []*dto.Metric{
						&dto.Metric{
							Counter: &dto.Counter{
								Value: proto.Float64(42),
							},
						},
					},
				},
			},
		},
	}

	for i, scenario := range scenarios {

		out, err := openMetricParser.OpenMetricsToMetricFamilies(strings.NewReader(scenario.in))
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
			name := expected.GetName()
			if *(expected.Type) == dto.MetricType_COUNTER {
				name = openMetricsCounterName(expected.GetName())
			}
			got, ok := out[name]
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

func TestOpenMetricsParse(t *testing.T) {
	testOpenMetricsParse(t)
}

func BenchmarkOpenMetricsParse(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testOpenMetricsParse(b)
	}
}
