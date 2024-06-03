// Copyright 2024 The Prometheus Authors
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

package templates

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHumanizeDurationSecondsFloat64(t *testing.T) {
	tc := []struct {
		name     string
		input    float64
		expected string
	}{
		{name: "zero", input: 0, expected: "0s"},
		{name: "one second", input: 1, expected: "1s"},
		{name: "one minute", input: 60, expected: "1m 0s"},
		{name: "one hour", input: 3600, expected: "1h 0m 0s"},
		{name: "one day", input: 86400, expected: "1d 0h 0m 0s"},
		{name: "one day and one hour", input: 86400 + 3600, expected: "1d 1h 0m 0s"},
		{name: "negative duration", input: -(86400*2 + 3600*3 + 60*4 + 5), expected: "-2d 3h 4m 5s"},
		{name: "using a float", input: 899.99, expected: "14m 59s"},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HumanizeDuration(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestHumanizeDurationSubsecondAndFractionalSecondsFloat64(t *testing.T) {
	tc := []struct {
		name     string
		input    float64
		expected string
	}{
		{name: "millseconds", input: .1, expected: "100ms"},
		{name: "nanoseconds", input: .0001, expected: "100us"},
		{name: "milliseconds + nanoseconds", input: .12345, expected: "123.5ms"},
		{name: "minute + millisecond", input: 60.1, expected: "1m 0s"},
		{name: "minute + milliseconds", input: 60.5, expected: "1m 0s"},
		{name: "second + milliseconds", input: 1.2345, expected: "1.234s"},
		{name: "second + milliseconds rounded", input: 12.345, expected: "12.35s"},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HumanizeDuration(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestHumanizeDurationErrorString(t *testing.T) {
	_, err := HumanizeDuration("one")
	require.Error(t, err)
}

func TestHumanizeDurationSecondsString(t *testing.T) {
	tc := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "zero", input: "0", expected: "0s"},
		{name: "second", input: "1", expected: "1s"},
		{name: "minute", input: "60", expected: "1m 0s"},
		{name: "hour", input: "3600", expected: "1h 0m 0s"},
		{name: "day", input: "86400", expected: "1d 0h 0m 0s"},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HumanizeDuration(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestHumanizeDurationSubsecondAndFractionalSecondsString(t *testing.T) {
	tc := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "millseconds", input: ".1", expected: "100ms"},
		{name: "nanoseconds", input: ".0001", expected: "100us"},
		{name: "milliseconds + nanoseconds", input: ".12345", expected: "123.5ms"},
		{name: "minute + millisecond", input: "60.1", expected: "1m 0s"},
		{name: "minute + milliseconds", input: "60.5", expected: "1m 0s"},
		{name: "second + milliseconds", input: "1.2345", expected: "1.234s"},
		{name: "second + milliseconds rounded", input: "12.345", expected: "12.35s"},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HumanizeDuration(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestHumanizeDurationSecondsInt(t *testing.T) {
	tc := []struct {
		name     string
		input    int
		expected string
	}{
		{name: "zero", input: 0, expected: "0s"},
		{name: "negative", input: -1, expected: "-1s"},
		{name: "second", input: 1, expected: "1s"},
		{name: "days", input: 1234567, expected: "14d 6h 56m 7s"},
	}

	for _, tt := range tc {
		t.Run(tt.name, func(t *testing.T) {
			result, err := HumanizeDuration(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result)
		})
	}
}
