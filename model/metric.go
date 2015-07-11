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
	"fmt"
	"sort"
	"strings"
)

// A Metric is fixed set of labels that describes a stream of samples,
// i.e. a single time series. It is safe for modification when passed
// into another function after calling Clone().
// The underlying LabelSet is accesible but must not be directly modified.
type Metric struct {
	LabelSet

	Copied bool
}

// NewMetric returns a new metric based on the given label set.
// The underlying label set
func NewMetric(ls LabelSet) Metric {
	return Metric{
		LabelSet: ls,
		Copied:   true,
	}
}

func (m *Metric) Name() string {
	return string(m.LabelSet[MetricNameLabel])
}

func (m *Metric) Len() int {
	return len(m.LabelSet)
}

func (m *Metric) Get(ln LabelName) LabelValue {
	return m.LabelSet[ln]
}

func (m *Metric) Has(ln LabelName) (LabelValue, bool) {
	v, ok := m.LabelSet[ln]
	return v, ok
}

func (m *Metric) Set(ln LabelName, lv LabelValue) {
	if !m.Copied {
		m.Copy()
	}
	m.LabelSet[ln] = lv
}

func (m *Metric) Del(ln LabelName) {
	if !m.Copied {
		m.Copy()
	}
	delete(m.LabelSet, ln)
}

func (m *Metric) Copy() {
	m.LabelSet = m.LabelSet.Clone()
	m.Copied = true
}

// Equal compares the metrics.
func (m *Metric) Equal(o Metric) bool {
	return m.LabelSet.Equal(o.LabelSet)
}

// Before compares the metrics' underlying label sets.
func (m *Metric) Before(o Metric) bool {
	return m.LabelSet.Before(o.LabelSet)
}

// String implements Stringer.
func (m *Metric) String() string {
	metricName, hasName := m.LabelSet[MetricNameLabel]
	numLabels := len(m.LabelSet) - 1
	if !hasName {
		numLabels++
	}
	labelStrings := make([]string, 0, numLabels)
	for label, value := range m.LabelSet {
		if label != MetricNameLabel {
			labelStrings = append(labelStrings, fmt.Sprintf("%s=%q", label, value))
		}
	}

	switch numLabels {
	case 0:
		if hasName {
			return string(metricName)
		}
		return "{}"
	default:
		sort.Strings(labelStrings)
		return fmt.Sprintf("%s{%s}", metricName, strings.Join(labelStrings, ", "))
	}
}

// MarshalJSON implements json.Marshaler.
func (m Metric) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.LabelSet)
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Metric) UnmarshalJSON(b []byte) error {
	if m.LabelSet != nil {
		m.Copy()
	}
	m.Copied = true
	return json.Unmarshal(b, &m.LabelSet)
}

// Fingerprint returns a Metric's Fingerprint.
func (m Metric) Fingerprint() Fingerprint {
	return m.LabelSet.Fingerprint()
}

// FastFingerprint returns a Metric's Fingerprint calculated by a faster hashing
// algorithm, which is, however, more susceptible to hash collisions.
func (m Metric) FastFingerprint() Fingerprint {
	return m.LabelSet.FastFingerprint()
}
