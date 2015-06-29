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
	"math"
	"strconv"
	"time"
)

// Time is the number of milliseconds since the epoch
// (1970-01-01 00:00 UTC) excluding leap seconds.
type Time int64

const (
	// MinimumTick is the minimum supported time resolution. This has to be
	// at least time.Second in order for the code below to work.
	MinimumTick = time.Millisecond
	// second is the Time duration equivalent to one second.
	second = int64(time.Second / MinimumTick)
	// The number of nanoseconds per minimum tick.
	nanosPerTick = int64(MinimumTick / time.Nanosecond)

	// Earliest is the earliest Time representable. Handy for
	// initializing a high watermark.
	Earliest = Time(math.MinInt64)
	// Latest is the latest Time representable. Handy for initializing
	// a low watermark.
	Latest = Time(math.MaxInt64)
)

// Equal reports whether two Times represent the same instant.
func (t Time) Equal(o Time) bool {
	return t == o
}

// Before reports whether the Time t is before o.
func (t Time) Before(o Time) bool {
	return t < o
}

// After reports whether the Time t is after o.
func (t Time) After(o Time) bool {
	return t > o
}

// Add returns the Time t + d.
func (t Time) Add(d time.Duration) Time {
	return t + Time(d/MinimumTick)
}

// Sub returns the Duration t - o.
func (t Time) Sub(o Time) time.Duration {
	return time.Duration(t-o) * MinimumTick
}

// Time returns the time.Time representation of t.
func (t Time) Time() time.Time {
	return time.Unix(int64(t)/second, (int64(t)%second)*nanosPerTick)
}

// Unix returns t as a Unix time, the number of seconds elapsed
// since January 1, 1970 UTC.
func (t Time) Unix() int64 {
	return int64(t) / second
}

// UnixNano returns t as a Unix time, the number of nanoseconds elapsed
// since January 1, 1970 UTC.
func (t Time) UnixNano() int64 {
	return int64(t) * nanosPerTick
}

// String returns a string representation of the Time.
func (t Time) String() string {
	return strconv.FormatFloat(float64(t)/float64(second), 'f', -1, 64)
}

// MarshalJSON implements the json.Marshaler interface.
func (t Time) MarshalJSON() ([]byte, error) {
	return []byte(t.String()), nil
}

func (t *Time) UnmarshalJSON(b []byte) error {
	ts, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	*t = TimeFromUnixNano(int64(ts * float64(time.Second)))
	return nil
}

// Now returns the current time as a Time.
func Now() Time {
	return TimeFromTime(time.Now())
}

// TimeFromTime returns the Time equivalent to the time.Time t.
func TimeFromTime(t time.Time) Time {
	return TimeFromUnixNano(t.UnixNano())
}

// TimeFromUnix returns the Time equivalent to the Unix Time t
// provided in seconds.
func TimeFromUnix(t int64) Time {
	return Time(t * second)
}

// TimeFromUnixNano returns the Time equivalent to the Unix Time
// t provided in nanoseconds.
func TimeFromUnixNano(t int64) Time {
	return Time(t / nanosPerTick)
}
