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

	jsoniter "github.com/json-iterator/go"
)

// from https://github.com/prometheus/prometheus/blob/main/util/jsonutil/marshal.go
// MarshalTimestamp marshals a point timestamp using the passed jsoniter stream.
func MarshalTimestamp(t int64, stream *jsoniter.Stream) {
	// Write out the timestamp as a float divided by 1000.
	// This is ~3x faster than converting to a float.
	if t < 0 {
		stream.WriteRaw(`-`)
		t = -t
	}
	stream.WriteInt64(t / 1000)
	fraction := t % 1000
	if fraction != 0 {
		stream.WriteRaw(`.`)
		if fraction < 100 {
			stream.WriteRaw(`0`)
		}
		if fraction < 10 {
			stream.WriteRaw(`0`)
		}
		stream.WriteInt64(fraction)
	}
}

// adapted from https://github.com/prometheus/prometheus/blob/main/util/jsonutil/marshal.go
// MarshalValue marshals a point value using the passed jsoniter stream.
func MarshalValue(f FloatString, stream *jsoniter.Stream) {
	v := float64(f)
	stream.WriteRaw(`"`)
	// Taken from https://github.com/json-iterator/go/blob/master/stream_float.go#L71 as a workaround
	// to https://github.com/json-iterator/go/issues/365 (jsoniter, to follow json standard, doesn't allow inf/nan).
	buf := stream.Buffer()
	abs := math.Abs(v)
	fmt := byte('f')
	// Note: Must use float32 comparisons for underlying float32 value to get precise cutoffs right.
	if abs != 0 {
		if abs < 1e-6 || abs >= 1e21 {
			fmt = 'e'
		}
	}
	buf = strconv.AppendFloat(buf, v, fmt, -1, 64)
	stream.SetBuffer(buf)
	stream.WriteRaw(`"`)
}

// adapted from https://github.com/prometheus/prometheus/blob/main/web/api/v1/api.go
func MarshalHistogramBucket(b HistogramBucket, stream *jsoniter.Stream) {
	stream.WriteArrayStart()
	stream.WriteInt32(b.Boundaries)
	stream.WriteMore()
	MarshalValue(b.Lower, stream)
	stream.WriteMore()
	MarshalValue(b.Upper, stream)
	stream.WriteMore()
	MarshalValue(b.Count, stream)
	stream.WriteArrayEnd()
}

// adapted from https://github.com/prometheus/prometheus/blob/main/web/api/v1/api.go
func MarshalHistogram(h SampleHistogram, stream *jsoniter.Stream) {
	stream.WriteObjectStart()
	stream.WriteObjectField(`count`)
	MarshalValue(h.Count, stream)
	stream.WriteMore()
	stream.WriteObjectField(`sum`)
	MarshalValue(h.Sum, stream)

	bucketFound := false
	for _, bucket := range h.Buckets {
		if bucket.Count == 0 {
			continue // No need to expose empty buckets in JSON.
		}
		stream.WriteMore()
		if !bucketFound {
			stream.WriteObjectField(`buckets`)
			stream.WriteArrayStart()
		}
		bucketFound = true
		MarshalHistogramBucket(*bucket, stream)
	}
	if bucketFound {
		stream.WriteArrayEnd()
	}
	stream.WriteObjectEnd()
}
