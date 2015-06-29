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
		input := NewMetric(scenario.input)

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

func TestMetricModify(t *testing.T) {
	testMetric := LabelSet{
		"to_delete": "test1",
		"to_change": "test2",
	}

	scenarios := []struct {
		fn  func(*Metric)
		out LabelSet
	}{
		{
			fn: func(cm *Metric) {
				cm.Del("to_delete")
			},
			out: LabelSet{
				"to_change": "test2",
			},
		},
		{
			fn: func(cm *Metric) {
				cm.Set("to_change", "changed")
			},
			out: LabelSet{
				"to_delete": "test1",
				"to_change": "changed",
			},
		},
	}

	for i, s := range scenarios {
		orig := testMetric.Clone()
		cm := NewMetric(orig)

		s.fn(&cm)

		// Test that the original metric was not modified.
		if !orig.Equal(testMetric) {
			t.Fatalf("%d. original metric changed; expected %v, got %v", i, testMetric, orig)
		}

		// Test that the new metric has the right changes.
		if !cm.LabelSet.Equal(s.out) {
			t.Fatalf("%d. copied metric doesn't contain expected changes; expected %v, got %v", i, s.out, cm.LabelSet)
		}
	}
}
