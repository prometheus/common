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

import "testing"

func testMetric(t testing.TB) {
	var scenarios = []struct {
		input           LabelSet
		fingerprint     Fingerprint
		fastFingerprint Fingerprint
	}{
		{
			input:           LabelSet{},
			fingerprint:     14695981039346656037,
			fastFingerprint: 14695981039346656037,
		},
		{
			input: LabelSet{
				"first_name":   "electro",
				"occupation":   "robot",
				"manufacturer": "westinghouse",
			},
			fingerprint:     5911716720268894962,
			fastFingerprint: 11310079640881077873,
		},
		{
			input: LabelSet{
				"x": "y",
			},
			fingerprint:     8241431561484471700,
			fastFingerprint: 13948396922932177635,
		},
		{
			input: LabelSet{
				"a": "bb",
				"b": "c",
			},
			fingerprint:     3016285359649981711,
			fastFingerprint: 3198632812309449502,
		},
		{
			input: LabelSet{
				"a":  "b",
				"bb": "c",
			},
			fingerprint:     7122421792099404749,
			fastFingerprint: 5774953389407657638,
		},
	}

	for i, scenario := range scenarios {
		input := Metric(scenario.input)

		if scenario.fingerprint != input.Fingerprint() {
			t.Errorf("%d. expected %d, got %d", i, scenario.fingerprint, input.Fingerprint())
		}
		if scenario.fastFingerprint != input.FastFingerprint() {
			t.Errorf("%d. expected %d, got %d", i, scenario.fastFingerprint, input.FastFingerprint())
		}
	}
}

func TestMetric(t *testing.T) {
	testMetric(t)
}

func BenchmarkMetric(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testMetric(b)
	}
}

func TestMetricNameIsLegacyValid(t *testing.T) {
	var scenarios = []struct {
		mn          LabelValue
		legacyValid bool
		utf8Valid   bool
	}{
		{
			mn:          "Avalid_23name",
			legacyValid: true,
			utf8Valid:   true,
		},
		{
			mn:          "_Avalid_23name",
			legacyValid: true,
			utf8Valid:   true,
		},
		{
			mn:          "1valid_23name",
			legacyValid: false,
			utf8Valid:   true,
		},
		{
			mn:          "avalid_23name",
			legacyValid: true,
			utf8Valid:   true,
		},
		{
			mn:          "Ava:lid_23name",
			legacyValid: true,
			utf8Valid:   true,
		},
		{
			mn:          "a lid_23name",
			legacyValid: false,
			utf8Valid:   true,
		},
		{
			mn:          ":leading_colon",
			legacyValid: true,
			utf8Valid:   true,
		},
		{
			mn:          "colon:in:the:middle",
			legacyValid: true,
			utf8Valid:   true,
		},
		{
			mn:          "",
			legacyValid: false,
			utf8Valid:   false,
		},
		{
			mn:          "a\xc5z",
			legacyValid: false,
			utf8Valid:   false,
		},
	}

	for _, s := range scenarios {
		NameValidationScheme = LegacyValidation
		if IsValidMetricName(s.mn) != s.legacyValid {
			t.Errorf("Expected %v for %q using legacy IsValidMetricName method", s.legacyValid, s.mn)
		}
		if MetricNameRE.MatchString(string(s.mn)) != s.legacyValid {
			t.Errorf("Expected %v for %q using regexp matching", s.legacyValid, s.mn)
		}
		NameValidationScheme = UTF8Validation
		if IsValidMetricName(s.mn) != s.utf8Valid {
			t.Errorf("Expected %v for %q using utf8 IsValidMetricName method", s.legacyValid, s.mn)
		}
	}
}

func TestMetricClone(t *testing.T) {
	m := Metric{
		"first_name":   "electro",
		"occupation":   "robot",
		"manufacturer": "westinghouse",
	}

	m2 := m.Clone()

	if len(m) != len(m2) {
		t.Errorf("expected the length of the cloned metric to be equal to the input metric")
	}

	for ln, lv := range m2 {
		expected := m[ln]
		if expected != lv {
			t.Errorf("expected label value %s but got %s for label name %s", expected, lv, ln)
		}
	}
}

func TestMetricToString(t *testing.T) {
	scenarios := []struct {
		name     string
		input    Metric
		expected string
	}{
		{
			name: "valid metric without __name__ label",
			input: Metric{
				"first_name":   "electro",
				"occupation":   "robot",
				"manufacturer": "westinghouse",
			},
			expected: `{first_name="electro", manufacturer="westinghouse", occupation="robot"}`,
		},
		{
			name: "valid metric with __name__ label",
			input: Metric{
				"__name__":     "electro",
				"occupation":   "robot",
				"manufacturer": "westinghouse",
			},
			expected: `electro{manufacturer="westinghouse", occupation="robot"}`,
		},
		{
			name: "empty metric with __name__ label",
			input: Metric{
				"__name__": "fooname",
			},
			expected: "fooname",
		},
		{
			name:     "empty metric",
			input:    Metric{},
			expected: "{}",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			actual := scenario.input.String()
			if actual != scenario.expected {
				t.Errorf("expected string output %s but got %s", actual, scenario.expected)
			}
		})
	}
}
