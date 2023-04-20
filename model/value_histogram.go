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
	"strings"
)

type FloatString float64

func (v FloatString) String() string {
	return strconv.FormatFloat(float64(v), 'f', -1, 64)
}

func (v FloatString) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

func (v *FloatString) UnmarshalJSON(b []byte) error {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return fmt.Errorf("float value must be a quoted string")
	}
	f, err := strconv.ParseFloat(string(b[1:len(b)-1]), 64)
	if err != nil {
		return err
	}
	*v = FloatString(f)
	return nil
}

type HistogramBucket struct {
	Boundaries int32
	Lower      FloatString
	Upper      FloatString
	Count      FloatString
}

func (hb HistogramBucket) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(hb.Boundaries)
	if err != nil {
		return nil, err
	}
	l, err := json.Marshal(hb.Lower)
	if err != nil {
		return nil, err
	}
	u, err := json.Marshal(hb.Upper)
	if err != nil {
		return nil, err
	}
	c, err := json.Marshal(hb.Count)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("[%s,%s,%s,%s]", b, l, u, c)), nil
}

func (hb *HistogramBucket) UnmarshalJSON(buf []byte) error {
	tmp := []interface{}{&hb.Boundaries, &hb.Lower, &hb.Upper, &hb.Count}
	wantLen := len(tmp)
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return err
	}
	if gotLen := len(tmp); gotLen != wantLen {
		return fmt.Errorf("wrong number of fields: %d != %d", gotLen, wantLen)
	}
	return nil
}

func (hb *HistogramBucket) Equal(o *HistogramBucket) bool {
	return hb == o || (hb.Boundaries == o.Boundaries && hb.Lower == o.Lower && hb.Upper == o.Upper && hb.Count == o.Count)
}

func (hb HistogramBucket) String() string {
	var sb strings.Builder
	lowerInclusive := hb.Boundaries == 1 || hb.Boundaries == 3
	upperInclusive := hb.Boundaries == 0 || hb.Boundaries == 3
	if lowerInclusive {
		sb.WriteRune('[')
	} else {
		sb.WriteRune('(')
	}
	fmt.Fprintf(&sb, "%g,%g", hb.Lower, hb.Upper)
	if upperInclusive {
		sb.WriteRune(']')
	} else {
		sb.WriteRune(')')
	}
	fmt.Fprintf(&sb, ":%v", hb.Count)
	return sb.String()
}

type HistogramBuckets []*HistogramBucket

func (s HistogramBuckets) Equal(o HistogramBuckets) bool {
	if len(s) != len(o) {
		return false
	}

	for i, bucket := range s {
		if !bucket.Equal(o[i]) {
			return false
		}
	}
	return true
}

type SampleHistogram struct {
	Count   FloatString      `json:"count"`
	Sum     FloatString      `json:"sum"`
	Buckets HistogramBuckets `json:"buckets"`
}

func (s SampleHistogram) String() string {
	return fmt.Sprintf("Count: %f, Sum: %f, Buckets: %v", s.Count, s.Sum, s.Buckets)
}

func (s *SampleHistogram) Equal(o *SampleHistogram) bool {
	return s == o || (s.Count == o.Count && s.Sum == o.Sum && s.Buckets.Equal(o.Buckets))
}

type SampleHistogramPair struct {
	Timestamp Time
	// Histogram should never be nil, it's only stored as pointer for efficiency.
	Histogram *SampleHistogram
}

func (s SampleHistogramPair) MarshalJSON() ([]byte, error) {
	if s.Histogram == nil {
		return nil, fmt.Errorf("histogram is nil")
	}
	t, err := json.Marshal(s.Timestamp)
	if err != nil {
		return nil, err
	}
	v, err := json.Marshal(s.Histogram)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf("[%s,%s]", t, v)), nil
}

func (s *SampleHistogramPair) UnmarshalJSON(buf []byte) error {
	tmp := []interface{}{&s.Timestamp, &s.Histogram}
	wantLen := len(tmp)
	if err := json.Unmarshal(buf, &tmp); err != nil {
		return err
	}
	if gotLen := len(tmp); gotLen != wantLen {
		return fmt.Errorf("wrong number of fields: %d != %d", gotLen, wantLen)
	}
	if s.Histogram == nil {
		return fmt.Errorf("histogram is null")
	}
	return nil
}

func (s SampleHistogramPair) String() string {
	return fmt.Sprintf("%s @[%s]", s.Histogram, s.Timestamp)
}

func (s *SampleHistogramPair) Equal(o *SampleHistogramPair) bool {
	return s == o || (s.Histogram.Equal(o.Histogram) && s.Timestamp.Equal(o.Timestamp))
}
