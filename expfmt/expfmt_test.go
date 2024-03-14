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

package expfmt

import (
	"testing"

	"github.com/prometheus/common/model"
)

// Test Format to Escapting Scheme conversion
// Path: expfmt/expfmt_test.go
// Compare this snippet from expfmt/expfmt.go:
func TestToFormatType(t *testing.T) {
	tests := []struct {
		format   Format
		expected FormatType
	}{
		{
			format:   fmtProtoCompact,
			expected: TypeProtoCompact,
		},
		{
			format:   fmtProtoDelim,
			expected: TypeProtoDelim,
		},
		{
			format:   fmtProtoText,
			expected: TypeProtoText,
		},
		{
			format:   fmtOpenMetrics_1_0_0,
			expected: TypeOpenMetrics,
		},
		{
			format:   fmtText,
			expected: TypeTextPlain,
		},
		{
			format:   fmtOpenMetrics_0_0_1,
			expected: TypeOpenMetrics,
		},
		{
			format:   "application/vnd.google.protobuf; proto=BadProtocol; encoding=text",
			expected: TypeUnknown,
		},
		{
			format:   "application/vnd.google.protobuf",
			expected: TypeUnknown,
		},
		{
			format:   "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily=bad",
			expected: TypeUnknown,
		},
		// encoding missing
		{
			format:   "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily",
			expected: TypeUnknown,
		},
		// invalid encoding
		{
			format:   "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=textual",
			expected: TypeUnknown,
		},
		// bad charset, must be utf-8
		{
			format:   "application/openmetrics-text; version=1.0.0; charset=ascii",
			expected: TypeUnknown,
		},
		{
			format:   "text/plain",
			expected: TypeTextPlain,
		},
		{
			format:   "text/plain; version=invalid",
			expected: TypeUnknown,
		},
		{
			format:   "gobbledygook",
			expected: TypeUnknown,
		},
	}
	for _, test := range tests {
		if test.format.FormatType() != test.expected {
			t.Errorf("expected %v got %v", test.expected, test.format.FormatType())
		}
	}
}

func TestToEscapingScheme(t *testing.T) {
	tests := []struct {
		format   Format
		expected model.EscapingScheme
	}{
		{
			format:   fmtProtoCompact,
			expected: model.ValueEncodingEscaping,
		},
		{
			format:   "application/openmetrics-text; version=1.0.0; charset=utf-8; escaping=underscores",
			expected: model.UnderscoreEscaping,
		},
		{
			format:   "application/openmetrics-text; version=1.0.0; charset=utf-8; escaping=allow-utf-8",
			expected: model.NoEscaping,
		},
		// error returns default
		{
			format:   "application/openmetrics-text; version=1.0.0; charset=utf-8; escaping=invalid",
			expected: model.NameEscapingScheme,
		},
	}
	for _, test := range tests {
		if test.format.ToEscapingScheme() != test.expected {
			t.Errorf("expected %v got %v", test.expected, test.format.ToEscapingScheme())
		}
	}
}
