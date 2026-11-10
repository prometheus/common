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

//go:build go1.27

package model

import (
	"fmt"
	"testing"
)

func TestSampleStreamJSONV2(t *testing.T) {
	cases := []struct {
		name   string
		object SampleStream
	}{
		{name: "empty", object: SampleStream{}},
		{name: "metric only", object: SampleStream{Metric: Metric{}}},
		{name: "metric ordering", object: SampleStream{Metric: Metric{"a": "", "b": "", "c": "", "d": "", "e": "", "f": "", "g": ""}}},
		{name: "zero length values", object: SampleStream{Values: []SamplePair{}}},
		{name: "empty value", object: SampleStream{Values: []SamplePair{{}}}},
		{name: "populated value", object: SampleStream{Values: []SamplePair{{Timestamp: Time(1), Value: SampleValue(1)}}}},
		{name: "histogram only", object: SampleStream{Histograms: []SampleHistogramPair{{Timestamp: Time(1), Histogram: &SampleHistogram{}}}}},
		{name: "metric and values", object: SampleStream{Metric: Metric{}, Values: []SamplePair{}}},
		{name: "all fields", object: SampleStream{Metric: Metric{}, Values: []SamplePair{{}}, Histograms: []SampleHistogramPair{{Timestamp: Time(1), Histogram: &SampleHistogram{}}}}},
	}

	for i, v := range cases {
		testV1V2Marshal(t, fmt.Sprintf("%d_%s", i, v.name), v.object)
		testRoundTrip(t, fmt.Sprintf("%d_%s", i, v.name), v.object)
	}
}
