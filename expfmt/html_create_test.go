// Copyright 2022 The Prometheus Authors
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

	"github.com/golang/protobuf/proto" //nolint:staticcheck // Ignore SA1019. Need to keep deprecated package for compatibility.
	dto "github.com/prometheus/client_model/go"
)

func TestCreateHTML(t *testing.T) {
	var scenarios = []struct {
		in  *dto.MetricFamily
		out string
	}{
		// 0: Counter, NaN as value, timestamp given.
		{
			in: &dto.MetricFamily{
				Name: proto.String("name"),
				Help: proto.String("two-line\n doc  str\\ing"),
				Type: dto.MetricType_COUNTER.Enum(),
				Metric: []*dto.Metric{
					&dto.Metric{
						Label: []*dto.LabelPair{
							&dto.LabelPair{
								Name:  proto.String("labelname"),
								Value: proto.String("val1"),
							},
							&dto.LabelPair{
								Name:  proto.String("basename"),
								Value: proto.String("basevalue"),
							},
						},
						Counter: &dto.Counter{
							Value: proto.Float64(math.NaN()),
						},
					},
					&dto.Metric{
						Label: []*dto.LabelPair{
							&dto.LabelPair{
								Name:  proto.String("labelname"),
								Value: proto.String("val2"),
							},
							&dto.LabelPair{
								Name:  proto.String("basename"),
								Value: proto.String("basevalue"),
							},
						},
						Counter: &dto.Counter{
							Value: proto.Float64(.23),
						},
						TimestampMs: proto.Int64(1234567890),
					},
				},
			},
			out: `<pre># HELP name two-line\n doc  str\\ing
# TYPE name counter
name{labelname=&#34;val1&#34;,basename=&#34;basevalue&#34;} NaN
name{labelname=&#34;val2&#34;,basename=&#34;basevalue&#34;} 0.23 1234567890
</pre>`,
		},
	}

	for i, scenario := range scenarios {
		out := bytes.NewBuffer(make([]byte, 0, len(scenario.out)))
		err := MetricFamilyToHTML(out, scenario.in)
		if err != nil {
			t.Errorf("%d. error: %s", i, err)
			continue
		}
		if expected, got := scenario.out, out.String(); expected != got {
			t.Errorf(
				"%d. expected\nout: %q\ngot: %q",
				i, expected, got,
			)
		}
	}

}
