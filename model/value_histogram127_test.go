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
	"testing"
)

func TestSampleHistogramPairJSONV2(t *testing.T) {
	for _, input := range []string{`[1.123,{}]`, `[null,{}]`, `[1.123,{"count":"1"}]`, `[1.123,{"count":"1","COUNT":"2","unknown":"value"}]`} {
		testDecodeSuccessParity(t, input, func() *SampleHistogramPair { return &SampleHistogramPair{Timestamp: Time(9)} })
	}
	for _, input := range []string{`null`, `[]`, `[`, `[1.123]`, `[1.123,null]`, `[1.123,{} true`, `[1.123,{},"bogus","trailing","data"]`, `{}`} {
		testDecodeErrorParity(t, input, func() *SampleHistogramPair { return &SampleHistogramPair{Timestamp: Time(9)} })
	}
}

func TestHistogramBucketJSONV2(t *testing.T) {
	for _, input := range []string{`[2,"2.123","3.123","4.123"]`, `[null,"2.123","3.123","4.123"]`, `[2,null,"3.123","4.123"]`, `[2,"2.123",null,"4.123"]`, `[2,"2.123","3.123",null]`} {
		testDecodeSuccessParity(t, input, func() *HistogramBucket {
			return &HistogramBucket{Boundaries: 1, Lower: FloatString(1.123), Upper: FloatString(1.123), Count: FloatString(1.123)}
		})
	}
	for _, input := range []string{`null`, `[]`, `[2]`, `[2,"2.123"]`, `[2,"2.123","3.123"]`, `[2,"2.123","3.123","4.123","random","trailing","data",true]`, `[`, `[1.123,null]`, `[2 true`, `{}`} {
		testDecodeErrorParity(t, input, func() *HistogramBucket {
			return &HistogramBucket{Boundaries: 1, Lower: FloatString(1.123), Upper: FloatString(1.123), Count: FloatString(1.123)}
		})
	}
}
