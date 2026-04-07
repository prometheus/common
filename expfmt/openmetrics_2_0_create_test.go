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
				Name: proto.String("node_memory_Active_bytes"),
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
			out: `# HELP node_memory_Active_bytes Active memory in bytes.
# TYPE node_memory_Active_bytes gauge
node_memory_Active_bytes 1.2345e+09
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
