// Copyright 2015 The Prometheus Authors
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
	input := []float64{0, 1, 60, 3600, 86400, 86400 + 3600, -(86400*2 + 3600*3 + 60*4 + 5), 899.99}
	expected := []string{"0s", "1s", "1m 0s", "1h 0m 0s", "1d 0h 0m 0s", "1d 1h 0m 0s", "-2d 3h 4m 5s", "14m 59s"}

	for index, value := range input {
		result, err := HumanizeDuration(value)
		require.NoError(t, err)
		require.Equal(t, expected[index], result)
	}
}

func TestHumanizeDurationSubsecondAndFractionalSecondsFloat64(t *testing.T) {
	input := []float64{.1, .0001, .12345, 60.1, 60.5, 1.2345, 12.345}
	expected := []string{"100ms", "100us", "123.5ms", "1m 0s", "1m 0s", "1.234s", "12.35s"}

	for index, value := range input {
		result, err := HumanizeDuration(value)
		require.NoError(t, err)
		require.Equal(t, expected[index], result)
	}
}

func TestHumanizeDurationErrorString(t *testing.T) {
	_, err := HumanizeDuration("one")
	require.Error(t, err)
}

func TestHumanizeDurationSecondsString(t *testing.T) {
	input := []string{"0", "1", "60", "3600", "86400"}
	expected := []string{"0s", "1s", "1m 0s", "1h 0m 0s", "1d 0h 0m 0s"}

	for index, value := range input {
		result, err := HumanizeDuration(value)
		require.NoError(t, err)
		require.Equal(t, expected[index], result)
	}
}

func TestHumanizeDurationSubsecondAndFractionalSecondsString(t *testing.T) {
	input := []string{".1", ".0001", ".12345", "60.1", "60.5", "1.2345", "12.345"}
	expected := []string{"100ms", "100us", "123.5ms", "1m 0s", "1m 0s", "1.234s", "12.35s"}

	for index, value := range input {
		result, err := HumanizeDuration(value)
		require.NoError(t, err)
		require.Equal(t, expected[index], result)
	}
}

func TestHumanizeDurationSecondsInt(t *testing.T) {
	input := []int{0, -1, 1, 1234567}
	expected := []string{"0s", "-1s", "1s", "14d 6h 56m 7s"}

	for index, value := range input {
		result, err := HumanizeDuration(value)
		require.NoError(t, err)
		require.Equal(t, expected[index], result)
	}
}
