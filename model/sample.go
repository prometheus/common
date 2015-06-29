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
	"strconv"
)

// A SampleValue is a representation of a value for a given sample at a given
// time.
type SampleValue float64

// Equal does a straight v==o.
func (v SampleValue) Equal(o SampleValue) bool {
	return v == o
}

// MarshalJSON implements json.Marshaler.
func (v SampleValue) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%s"`, v)), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (v *SampleValue) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("sample value must be a quoted string")
	}
	f, err := strconv.ParseFloat(string(b[1:len(b)-1]), 64)
	if err != nil {
		return err
	}
	*v = SampleValue(f)
	return nil
}

func (v SampleValue) String() string {
	return strconv.FormatFloat(float64(v), 'f', -1, 64)
}

// Sample is a sample value with a timestamp and a metric.
type Sample struct {
	Metric    Metric
	Value     SampleValue
	Timestamp Time
}

// Equal compares first the metrics, then the timestamp, then the value.
func (s *Sample) Equal(o *Sample) bool {
	if s == o {
		return true
	}

	if !s.Metric.Equal(o.Metric) {
		return false
	}
	if !s.Timestamp.Equal(o.Timestamp) {
		return false
	}
	if !s.Value.Equal(o.Value) {
		return false
	}

	return true
}

// Samples is a sortable Sample slice. It implements sort.Interface.
type Samples []*Sample

func (s Samples) Len() int {
	return len(s)
}

// Less compares first the metrics, then the timestamp.
func (s Samples) Less(i, j int) bool {
	switch {
	case s[i].Metric.Before(s[j].Metric):
		return true
	case s[j].Metric.Before(s[i].Metric):
		return false
	case s[i].Timestamp.Before(s[j].Timestamp):
		return true
	default:
		return false
	}
}

func (s Samples) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Equal compares two sets of samples and returns true if they are equal.
func (s Samples) Equal(o Samples) bool {
	if len(s) != len(o) {
		return false
	}

	for i, sample := range s {
		if !sample.Equal(o[i]) {
			return false
		}
	}
	return true
}

// SamplePair pairs a SampleValue with a Timestamp.
type SamplePair struct {
	Timestamp Time
	Value     SampleValue
}

// MarshalJSON implements json.Marshaler.
func (s SamplePair) MarshalJSON() ([]byte, error) {
	t, err := json.Marshal(s.Timestamp)
	if err != nil {
		return nil, err
	}
	v, err := json.Marshal(s.Value)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("[%s, %s]", t, v)), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (s *SamplePair) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("sample pair must be array")
	}

	b = b[1 : len(b)-1]

	return json.Unmarshal(b, []json.Unmarshaler{&s.Timestamp, &s.Value})
}

// Equal returns true if this SamplePair and o have equal Values and equal
// Timestamps.
func (s *SamplePair) Equal(o *SamplePair) bool {
	if s == o {
		return true
	}

	return s.Value.Equal(o.Value) && s.Timestamp.Equal(o.Timestamp)
}

func (s *SamplePair) String() string {
	return fmt.Sprintf("SamplePair at %s of %s", s.Timestamp, s.Value)
}
