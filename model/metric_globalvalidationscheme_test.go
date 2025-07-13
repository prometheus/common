// Copyright 2025 The Prometheus Authors
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

//go:build !localvalidationscheme

package model

import "testing"

func testIsValidMetricName(t *testing.T, metricName LabelValue, legacyValid, utf8Valid bool) {
	t.Helper()
	origScheme := NameValidationScheme
	t.Cleanup(func() {
		NameValidationScheme = origScheme
	})

	NameValidationScheme = LegacyValidation
	if IsValidMetricName(metricName) != legacyValid {
		t.Errorf("Expected %v for %q using legacy IsValidMetricName method", legacyValid, metricName)
	}
	if MetricNameRE.MatchString(string(metricName)) != legacyValid {
		t.Errorf("Expected %v for %q using regexp matching", legacyValid, metricName)
	}
	NameValidationScheme = UTF8Validation
	if IsValidMetricName(metricName) != utf8Valid {
		t.Errorf("Expected %v for %q using UTF-8 IsValidMetricName method", utf8Valid, metricName)
	}
}
