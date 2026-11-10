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
	"math"
	"testing"
)

func TestSampleValueJSONV2(t *testing.T) {
	cases := []float64{
		math.Inf(1),
		math.MaxFloat64,
		math.MaxFloat32,
		math.SmallestNonzeroFloat64,
		math.SmallestNonzeroFloat32,
		1e21,
		1.0,
		1e-6,
		0,
		-1e-6,
		-1.0,
		-1e21,
		-math.MaxFloat64,
		-math.MaxFloat32,
		-math.SmallestNonzeroFloat64,
		-math.SmallestNonzeroFloat32,
		math.Inf(-1),
		math.NaN(),
	}

	for _, i := range cases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			if i != -math.MaxFloat64 {
				testV1V2Marshal(t, "-", SampleValue(math.Nextafter(i, math.Inf(-1))))
			}
			testV1V2Marshal(t, "=", SampleValue(i))
			if i != math.MaxFloat64 {
				testV1V2Marshal(t, "+", SampleValue(math.Nextafter(i, math.Inf(1))))
			}
		})
	}
	testRoundTrip(t, "-1", SampleValue(-1))
	testRoundTrip(t, "-1.1", SampleValue(-1.1))
	testRoundTrip(t, "0", SampleValue(0))
	testRoundTrip(t, "1", SampleValue(1))
	testRoundTrip(t, "1.1", SampleValue(1.1))
}

func TestSamplePairJSONV2(t *testing.T) {
	cases := []SamplePair{
		{},
		{Timestamp: Time(1)},
		{Value: SampleValue(1)},
		{Timestamp: Time(1), Value: SampleValue(1)},
	}

	for _, v := range cases {
		testV1V2Marshal(t, fmt.Sprintf("%#v", v), v)
		testRoundTrip(t, fmt.Sprintf("%#v", v), v)
	}

	for _, input := range []string{`null`, `[]`, `[1.123]`, `[1.123,"2"]`, `[null,"2"]`, `[1.123,null]`, `[1.123,"2","bogus","trailing","data"]`} {
		testDecodeSuccessParity(t, input, func() *SamplePair { return &SamplePair{Timestamp: Time(9), Value: SampleValue(9)} })
	}
	for _, input := range []string{`[`, `[1.123,"2" true`, `{}`} {
		testDecodeErrorParity(t, input, func() *SamplePair { return &SamplePair{Timestamp: Time(9), Value: SampleValue(9)} })
	}
}
