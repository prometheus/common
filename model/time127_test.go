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
	"math"
	"strconv"
	"testing"
)

func TestTimeJSONV2(t *testing.T) {
	cases := []int64{
		math.MinInt64,
		-9999999999995500,
		-9999999999994500,
		-9007199254740992, // smallest integer with float precision
		-8123456789012345,
		math.MinInt32,
		-10000,
		-9000,
		-8000,
		-7000,
		-6000,
		-5000,
		-4000,
		-3000,
		-2000,
		-1000,
		-900,
		-800,
		-700,
		-600,
		-500,
		-400,
		-300,
		-200,
		-100,
		-10,
		-1,
		0,
		1,
		10,
		100,
		200,
		300,
		400,
		500,
		600,
		700,
		800,
		900,
		1000,
		2000,
		3000,
		4000,
		5000,
		6000,
		7000,
		8000,
		9000,
		10000,
		math.MaxInt32,
		8123456789012345,
		9007199254740992, // largest integer with float precision
		math.MaxInt64,
	}

	for _, i := range cases {
		t.Run(strconv.FormatInt(i, 10), func(t *testing.T) {
			if i != math.MinInt64 {
				testV1V2Marshal(t, "-1", Time(i-1))
			}
			testV1V2Marshal(t, "=", Time(i))
			if i != math.MaxInt64 {
				testV1V2Marshal(t, "+1", Time(i+1))
			}
		})
	}
	testV1V2Marshal(t, "-1", Time(-1))
	testV1V2Marshal(t, "0", Time(0))
	testV1V2Marshal(t, "1", Time(1))
}
