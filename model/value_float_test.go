// Copyright 2013 The Prometheus Authors
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

package model

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	samplePairMatrixPlain = `[{"metric":{"__name__":"test_metric"},"values":[[1234.567,"123.1"],[12345.678,"123.12"]]},{"metric":{"foo":"bar"},"values":[[2234.567,"223.1"],[22345.678,"223.12"]]}]`
	samplePairMatrixValue = Matrix{
		&SampleStream{
			Metric: Metric{
				MetricNameLabel: "test_metric",
			},
			Values: []SamplePair{
				{
					Value:     123.1,
					Timestamp: 1234567,
				},
				{
					Value:     123.12,
					Timestamp: 12345678,
				},
			},
		},
		&SampleStream{
			Metric: Metric{
				"foo": "bar",
			},
			Values: []SamplePair{
				{
					Value:     223.1,
					Timestamp: 2234567,
				},
				{
					Value:     223.12,
					Timestamp: 22345678,
				},
			},
		},
	}
)

func TestEqualValues(t *testing.T) {
	tests := map[string]struct {
		in1, in2 SampleValue
		want     bool
	}{
		"equal floats": {
			in1:  3.14,
			in2:  3.14,
			want: true,
		},
		"unequal floats": {
			in1:  3.14,
			in2:  3.1415,
			want: false,
		},
		"positive infinities": {
			in1:  SampleValue(math.Inf(+1)),
			in2:  SampleValue(math.Inf(+1)),
			want: true,
		},
		"negative infinities": {
			in1:  SampleValue(math.Inf(-1)),
			in2:  SampleValue(math.Inf(-1)),
			want: true,
		},
		"different infinities": {
			in1:  SampleValue(math.Inf(+1)),
			in2:  SampleValue(math.Inf(-1)),
			want: false,
		},
		"number and infinity": {
			in1:  42,
			in2:  SampleValue(math.Inf(+1)),
			want: false,
		},
		"number and NaN": {
			in1:  42,
			in2:  SampleValue(math.NaN()),
			want: false,
		},
		"NaNs": {
			in1:  SampleValue(math.NaN()),
			in2:  SampleValue(math.NaN()),
			want: true, // !!!
		},
	}

	for name, test := range tests {
		got := test.in1.Equal(test.in2)
		if got != test.want {
			t.Errorf("Comparing %s, %f and %f: got %t, want %t", name, test.in1, test.in2, got, test.want)
		}
	}
}

func TestSamplePairJSON(t *testing.T) {
	input := []struct {
		plain string
		value SamplePair
	}{
		{
			plain: `[1234.567,"123.1"]`,
			value: SamplePair{
				Value:     123.1,
				Timestamp: 1234567,
			},
		},
	}

	for _, test := range input {
		b, err := json.Marshal(test.value)
		if err != nil {
			t.Error(err)
			continue
		}

		if string(b) != test.plain {
			t.Errorf("encoding error: expected %q, got %q", test.plain, b)
			continue
		}

		var sp SamplePair
		err = json.Unmarshal(b, &sp)
		if err != nil {
			t.Error(err)
			continue
		}

		if sp != test.value {
			t.Errorf("decoding error: expected %v, got %v", test.value, sp)
		}
	}
}

func TestSampleJSON(t *testing.T) {
	input := []struct {
		plain string
		value Sample
	}{
		{
			plain: `{"metric":{"__name__":"test_metric"},"value":[1234.567,"123.1"]}`,
			value: Sample{
				Metric: Metric{
					MetricNameLabel: "test_metric",
				},
				Value:     123.1,
				Timestamp: 1234567,
			},
		},
	}

	for _, test := range input {
		b, err := json.Marshal(test.value)
		if err != nil {
			t.Error(err)
			continue
		}

		if string(b) != test.plain {
			t.Errorf("encoding error: expected %q, got %q", test.plain, b)
			continue
		}

		var sv Sample
		err = json.Unmarshal(b, &sv)
		if err != nil {
			t.Error(err)
			continue
		}

		if !reflect.DeepEqual(sv, test.value) {
			t.Errorf("decoding error: expected %v, got %v", test.value, sv)
		}
	}
}

func TestVectorJSON(t *testing.T) {
	input := []struct {
		plain string
		value Vector
	}{
		{
			plain: `[]`,
			value: Vector{},
		},
		{
			plain: `[{"metric":{"__name__":"test_metric"},"value":[1234.567,"123.1"]}]`,
			value: Vector{&Sample{
				Metric: Metric{
					MetricNameLabel: "test_metric",
				},
				Value:     123.1,
				Timestamp: 1234567,
			}},
		},
		{
			plain: `[{"metric":{"__name__":"test_metric"},"value":[1234.567,"123.1"]},{"metric":{"foo":"bar"},"value":[1.234,"+Inf"]}]`,
			value: Vector{
				&Sample{
					Metric: Metric{
						MetricNameLabel: "test_metric",
					},
					Value:     123.1,
					Timestamp: 1234567,
				},
				&Sample{
					Metric: Metric{
						"foo": "bar",
					},
					Value:     SampleValue(math.Inf(1)),
					Timestamp: 1234,
				},
			},
		},
	}

	for _, test := range input {
		b, err := json.Marshal(test.value)
		if err != nil {
			t.Error(err)
			continue
		}

		if string(b) != test.plain {
			t.Errorf("encoding error: expected %q, got %q", test.plain, b)
			continue
		}

		var vec Vector
		err = json.Unmarshal(b, &vec)
		if err != nil {
			t.Error(err)
			continue
		}

		if !reflect.DeepEqual(vec, test.value) {
			t.Errorf("decoding error: expected %v, got %v", test.value, vec)
		}
	}
}

func TestMatrixJSON(t *testing.T) {
	input := []struct {
		plain string
		value Matrix
	}{
		{
			plain: `[]`,
			value: Matrix{},
		},
		{
			plain: samplePairMatrixPlain,
			value: samplePairMatrixValue,
		},
	}

	for _, test := range input {
		b, err := json.Marshal(test.value)
		if err != nil {
			t.Error(err)
			continue
		}

		if string(b) != test.plain {
			t.Errorf("encoding error: expected %q, got %q", test.plain, b)
			continue
		}

		var mat Matrix
		err = json.Unmarshal(b, &mat)
		if err != nil {
			t.Error(err)
			continue
		}

		if !reflect.DeepEqual(mat, test.value) {
			t.Errorf("decoding error: expected %v, got %v", test.value, mat)
		}
	}
}

func BenchmarkJSONMarshallingSamplePairMatrix(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(samplePairMatrixValue)
		require.NoErrorf(b, err, "error marshalling")
	}
}

func TestQuotedFloatJSONEscapes(t *testing.T) {
	t.Run("sample", func(t *testing.T) {
		testQuotedFloatJSONEscapes[SampleValue](t, json.Unmarshal)
	})
	t.Run("histogram", func(t *testing.T) {
		testQuotedFloatJSONEscapes[FloatString](t, json.Unmarshal)
	})
}

func testQuotedFloatJSONEscapes[T ~float64](t *testing.T, unmarshal func([]byte, any) error) {
	t.Helper()
	for _, tc := range []struct {
		name, input string
		want        float64
		wantErr     bool
	}{
		{"one", `"\u0031"`, 1, false},
		{"zero", `"\u0030"`, 0, false},
		{"negative_zero", `"\u002d0"`, math.Copysign(0, -1), false},
		{"max_float", `"\u0031.7976931348623157e308"`, math.MaxFloat64, false},
		{"subnormal", `"\u0035e-324"`, math.SmallestNonzeroFloat64, false},
		{"positive_inf", `"\u002bInf"`, math.Inf(1), false},
		{"negative_inf", `"\u002dInf"`, math.Inf(-1), false},
		{"nan", `"\u004eaN"`, math.NaN(), false},
		{"overflow", `"\u0031e309"`, 0, true},
		{"invalid_number", `"\u0031x"`, 0, true},
		{"unquoted", `1`, 0, true},
		{"null", `null`, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got T
			err := unmarshal([]byte(tc.input), &got)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if math.IsNaN(tc.want) {
				require.True(t, math.IsNaN(float64(got)))
			} else {
				require.Equal(t, tc.want, float64(got))
				require.Equal(t, math.Signbit(tc.want), math.Signbit(float64(got)))
			}
		})
	}
}

func BenchmarkSampleValueUnmarshalJSON(b *testing.B) {
	for name, input := range map[string]string{
		"plain":   `"123.456"`,
		"escaped": `"\u003123.456"`,
	} {
		b.Run(name, func(b *testing.B) {
			data := []byte(input)
			var value SampleValue
			b.ReportAllocs()
			for b.Loop() {
				if err := value.UnmarshalJSON(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
