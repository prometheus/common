// Copyright 2018 The Prometheus Authors
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
	"bytes"
	"net/http"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/prometheus/common/model"

	dto "github.com/prometheus/client_model/go"
)

func TestNegotiate(t *testing.T) {
	acceptValuePrefix := "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily"
	tests := []struct {
		name              string
		acceptHeaderValue string
		expectedFmt       string
	}{
		{
			name:              "delimited format",
			acceptHeaderValue: acceptValuePrefix + ";encoding=delimited",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited; escaping=underscores",
		},
		{
			name:              "text format",
			acceptHeaderValue: acceptValuePrefix + ";encoding=text",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=text; escaping=underscores",
		},
		{
			name:              "compact text format",
			acceptHeaderValue: acceptValuePrefix + ";encoding=compact-text",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=compact-text; escaping=underscores",
		},
		{
			name:              "plain text format",
			acceptHeaderValue: "text/plain;version=0.0.4",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=underscores",
		},
		{
			name:              "delimited format utf-8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=delimited; escaping=allow-utf-8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited; escaping=allow-utf-8",
		},
		{
			name:              "text format utf-8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=text; escaping=allow-utf-8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=text; escaping=allow-utf-8",
		},
		{
			name:              "compact text format utf-8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=compact-text; escaping=allow-utf-8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=compact-text; escaping=allow-utf-8",
		},
		{
			name:              "plain text format 0.0.4 with utf-8 not valid, falls back",
			acceptHeaderValue: "text/plain;version=0.0.4;",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=underscores",
		},
		{
			name:              "plain text format 0.0.4 with utf-8 not valid, falls back",
			acceptHeaderValue: "text/plain;version=0.0.4; escaping=values;",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=values",
		},
	}

	oldDefault := model.NameEscapingScheme
	model.NameEscapingScheme = model.UnderscoreEscaping
	defer func() {
		model.NameEscapingScheme = oldDefault
	}()

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := http.Header{}
			h.Add(hdrAccept, test.acceptHeaderValue)
			actualFmt := string(Negotiate(h))
			if actualFmt != test.expectedFmt {
				t.Errorf("case %d: expected Negotiate to return format %s, but got %s instead", i, test.expectedFmt, actualFmt)
			}
		})
	}
}

func TestNegotiateOpenMetrics(t *testing.T) {
	acceptValuePrefix := "application/vnd.google.protobuf;proto=io.prometheus.client.MetricFamily"
	tests := []struct {
		name              string
		acceptHeaderValue string
		expectedFmt       string
	}{
		{
			name:              "OM format, no version",
			acceptHeaderValue: "application/openmetrics-text",
			expectedFmt:       "application/openmetrics-text; version=0.0.1; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, 0.0.1 version",
			acceptHeaderValue: "application/openmetrics-text;version=0.0.1; escaping=underscores",
			expectedFmt:       "application/openmetrics-text; version=0.0.1; charset=utf-8; escaping=underscores",
		},
		{
			name:              "OM format, 1.0.0 version",
			acceptHeaderValue: "application/openmetrics-text;version=1.0.0",
			expectedFmt:       "application/openmetrics-text; version=1.0.0; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, 0.0.1 version with utf-8 is not valid, falls back",
			acceptHeaderValue: "application/openmetrics-text;version=0.0.1",
			expectedFmt:       "application/openmetrics-text; version=0.0.1; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, 1.0.0 version with utf-8 is not valid, falls back",
			acceptHeaderValue: "application/openmetrics-text;version=1.0.0; escaping=values;",
			expectedFmt:       "application/openmetrics-text; version=1.0.0; charset=utf-8; escaping=values",
		},
		{
			name:              "OM format, invalid version",
			acceptHeaderValue: "application/openmetrics-text;version=0.0.4",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=values",
		},
		{
			name:              "compact text format",
			acceptHeaderValue: acceptValuePrefix + ";encoding=compact-text; escaping=underscores",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=compact-text; escaping=underscores",
		},
		{
			name:              "plain text format",
			acceptHeaderValue: "text/plain;version=0.0.4",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=values",
		},
		{
			name:              "plain text format 0.0.4",
			acceptHeaderValue: "text/plain;version=0.0.4; escaping=allow-utf-8",
			expectedFmt:       "text/plain; version=0.0.4; charset=utf-8; escaping=allow-utf-8",
		},
		{
			name:              "delimited format utf-8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=delimited; escaping=allow-utf-8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited; escaping=allow-utf-8",
		},
		{
			name:              "text format utf-8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=text; escaping=allow-utf-8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=text; escaping=allow-utf-8",
		},
		{
			name:              "compact text format utf-8",
			acceptHeaderValue: acceptValuePrefix + ";encoding=compact-text; escaping=allow-utf-8;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=compact-text; escaping=allow-utf-8",
		},
		{
			name:              "delimited format escaped",
			acceptHeaderValue: acceptValuePrefix + ";encoding=delimited; escaping=underscores;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=delimited; escaping=underscores",
		},
		{
			name:              "text format escaped",
			acceptHeaderValue: acceptValuePrefix + ";encoding=text; escaping=underscores;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=text; escaping=underscores",
		},
		{
			name:              "compact text format escaped",
			acceptHeaderValue: acceptValuePrefix + ";encoding=compact-text; escaping=underscores;",
			expectedFmt:       "application/vnd.google.protobuf; proto=io.prometheus.client.MetricFamily; encoding=compact-text; escaping=underscores",
		},
	}

	oldDefault := model.NameEscapingScheme
	model.NameEscapingScheme = model.ValueEncodingEscaping
	defer func() {
		model.NameEscapingScheme = oldDefault
	}()

	for i, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := http.Header{}
			h.Add(hdrAccept, test.acceptHeaderValue)
			actualFmt := string(NegotiateIncludingOpenMetrics(h))
			if actualFmt != test.expectedFmt {
				t.Errorf("case %d: expected Negotiate to return format %s, but got %s instead", i, test.expectedFmt, actualFmt)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	metric1 := &dto.MetricFamily{
		Name: proto.String("foo_metric"),
		Type: dto.MetricType_UNTYPED.Enum(),
		Unit: proto.String("seconds"),
		Metric: []*dto.Metric{
			{
				Untyped: &dto.Untyped{
					Value: proto.Float64(1.234),
				},
			},
		},
	}

	scenarios := []struct {
		metric  *dto.MetricFamily
		format  Format
		options []EncoderOption
		expOut  string
	}{
		// 1: Untyped ProtoDelim
		{
			metric: metric1,
			format: FmtProtoDelim,
		},
		// 2: Untyped FmtProtoCompact
		{
			metric: metric1,
			format: FmtProtoCompact,
		},
		// 3: Untyped FmtProtoText
		{
			metric: metric1,
			format: FmtProtoText,
		},
		// 4: Untyped FmtText
		{
			metric: metric1,
			format: FmtText,
			expOut: `# TYPE foo_metric untyped
foo_metric 1.234
`,
		},
		// 5: Untyped FmtOpenMetrics_0_0_1
		{
			metric: metric1,
			format: FmtOpenMetrics_0_0_1,
			expOut: `# TYPE foo_metric unknown
foo_metric 1.234
`,
		},
		// 6: Untyped FmtOpenMetrics_1_0_0
		{
			metric: metric1,
			format: FmtOpenMetrics_1_0_0,
			expOut: `# TYPE foo_metric unknown
foo_metric 1.234
`,
		},
		// 7: Simple Counter FmtOpenMetrics_0_0_1 unit opted in
		{
			metric:  metric1,
			format:  FmtOpenMetrics_0_0_1,
			options: []EncoderOption{WithUnit()},
			expOut: `# TYPE foo_metric_seconds unknown
# UNIT foo_metric_seconds seconds
foo_metric_seconds 1.234
`,
		},
		// 8: Simple Counter FmtOpenMetrics_1_0_0 unit opted out
		{
			metric: metric1,
			format: FmtOpenMetrics_1_0_0,
			expOut: `# TYPE foo_metric unknown
foo_metric 1.234
`,
		},
	}
	for i, scenario := range scenarios {
		out := bytes.NewBuffer(make([]byte, 0, len(scenario.expOut)))
		enc := NewEncoder(out, scenario.format, scenario.options...)
		err := enc.Encode(scenario.metric)
		if err != nil {
			t.Errorf("%d. error: %s", i, err)
			continue
		}

		if expected, got := len(scenario.expOut), len(out.Bytes()); expected != 0 && expected != got {
			t.Errorf(
				"%d. expected %d bytes written, got %d",
				i, expected, got,
			)
		}
		if expected, got := scenario.expOut, out.String(); expected != "" && expected != got {
			t.Errorf(
				"%d. expected out=%q, got %q",
				i, expected, got,
			)
		}

		if len(out.Bytes()) == 0 {
			t.Errorf(
				"%d. expected output not to be empty",
				i,
			)
		}
	}
}

func TestEscapedEncode(t *testing.T) {
	var buff bytes.Buffer
	delimEncoder := NewEncoder(&buff, FmtProtoDelim+"; escaping=underscores")
	metric := &dto.MetricFamily{
		Name: proto.String("foo.metric"),
		Type: dto.MetricType_UNTYPED.Enum(),
		Metric: []*dto.Metric{
			{
				Untyped: &dto.Untyped{
					Value: proto.Float64(1.234),
				},
			},
			{
				Label: []*dto.LabelPair{
					{
						Name:  proto.String("dotted.label.name"),
						Value: proto.String("my.label.value"),
					},
				},
				Untyped: &dto.Untyped{
					Value: proto.Float64(8),
				},
			},
		},
	}

	err := delimEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out := buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}

	dec := NewDecoder(bytes.NewReader(out), FmtProtoDelim)
	var gotFamily dto.MetricFamily
	err = dec.Decode(&gotFamily)
	if err != nil {
		t.Errorf("unexpected error during proto decode: %s", err.Error())
	}
	expectEscapedName := "foo_metric"
	if gotFamily.GetName() != expectEscapedName {
		t.Errorf("incorrect encoded metric name, want %v, got %v", expectEscapedName, gotFamily.GetName())
	}

	buff.Reset()
	compactEncoder := NewEncoder(&buff, FmtProtoCompact)
	err = compactEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out = buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}
	dec = NewDecoder(bytes.NewReader(out), FmtProtoCompact)
	err = dec.Decode(&gotFamily)
	if err != nil {
		t.Errorf("unexpected error during proto decode: %s", err.Error())
	}
	if gotFamily.GetName() != "foo_metric" {
		t.Errorf("incorrect encoded metricname , want foo_metric, got %v", gotFamily.GetName())
	}

	buff.Reset()
	protoTextEncoder := NewEncoder(&buff, FmtProtoText)
	err = protoTextEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out = buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}
	dec = NewDecoder(bytes.NewReader(out), FmtProtoText)
	err = dec.Decode(&gotFamily)
	if err != nil {
		t.Errorf("unexpected error during proto decode: %s", err.Error())
	}
	if gotFamily.GetName() != "foo_metric" {
		t.Errorf("incorrect encoded metric name, want foo_metic, got %v", gotFamily.GetName())
	}

	buff.Reset()
	textEncoder := NewEncoder(&buff, FmtText)
	err = textEncoder.Encode(metric)
	if err != nil {
		t.Errorf("unexpected error during encode: %s", err.Error())
	}

	out = buff.Bytes()
	if len(out) == 0 {
		t.Errorf("expected the output bytes buffer to be non-empty")
	}

	expected := `# TYPE foo_metric untyped
foo_metric 1.234
foo_metric{dotted_label_name="my.label.value"} 8
`

	if string(out) != expected {
		t.Errorf("expected TextEncoder to return %s, but got %s instead", expected, string(out))
	}
}

func TestDottedEncode(t *testing.T) {
	//nolint:staticcheck
	model.NameValidationScheme = model.UTF8Validation
	metric := &dto.MetricFamily{
		Name: proto.String("foo.metric"),
		Type: dto.MetricType_COUNTER.Enum(),
		Metric: []*dto.Metric{
			{
				Counter: &dto.Counter{
					Value: proto.Float64(1.234),
				},
			},
			{
				Label: []*dto.LabelPair{
					{
						Name:  proto.String("dotted.label.name"),
						Value: proto.String("my.label.value"),
					},
				},
				Counter: &dto.Counter{
					Value: proto.Float64(8),
				},
			},
		},
	}

	scenarios := []struct {
		format           Format
		expectMetricName string
		expectLabelName  string
	}{
		{
			format:           FmtProtoDelim,
			expectMetricName: "foo_metric",
			expectLabelName:  "dotted_label_name",
		},
		{
			format:           FmtProtoDelim.WithEscapingScheme(model.NoEscaping),
			expectMetricName: "foo.metric",
			expectLabelName:  "dotted.label.name",
		},
		{
			format:           FmtProtoDelim.WithEscapingScheme(model.DotsEscaping),
			expectMetricName: "foo_dot_metric",
			expectLabelName:  "dotted_dot_label_dot_name",
		},
		{
			format:           FmtProtoCompact,
			expectMetricName: "foo_metric",
			expectLabelName:  "dotted_label_name",
		},
		{
			format:           FmtProtoCompact.WithEscapingScheme(model.NoEscaping),
			expectMetricName: "foo.metric",
			expectLabelName:  "dotted.label.name",
		},
		{
			format:           FmtProtoCompact.WithEscapingScheme(model.ValueEncodingEscaping),
			expectMetricName: "U__foo_2e_metric",
			expectLabelName:  "U__dotted_2e_label_2e_name",
		},
		{
			format:           FmtProtoText,
			expectMetricName: "foo_metric",
			expectLabelName:  "dotted_label_name",
		},
		{
			format:           FmtProtoText.WithEscapingScheme(model.NoEscaping),
			expectMetricName: "foo.metric",
			expectLabelName:  "dotted.label.name",
		},
		{
			format:           FmtText,
			expectMetricName: "foo_metric",
			expectLabelName:  "dotted_label_name",
		},
		{
			format:           FmtText.WithEscapingScheme(model.NoEscaping),
			expectMetricName: "foo.metric",
			expectLabelName:  "dotted.label.name",
		},
		// common library does not support open metrics parsing so we do not test
		// that here.
	}

	for i, scenario := range scenarios {
		out := bytes.NewBuffer(make([]byte, 0))
		enc := NewEncoder(out, scenario.format)
		err := enc.Encode(metric)
		if err != nil {
			t.Errorf("%d. error: %s", i, err)
			continue
		}

		dec := NewDecoder(bytes.NewReader(out.Bytes()), scenario.format)
		var gotFamily dto.MetricFamily
		err = dec.Decode(&gotFamily)
		if err != nil {
			t.Errorf("%v: unexpected error during decode: %s", scenario.format, err.Error())
		}
		if gotFamily.GetName() != scenario.expectMetricName {
			t.Errorf("%v: incorrect encoded metric name, want %v, got %v", scenario.format, scenario.expectMetricName, gotFamily.GetName())
		}
		lName := gotFamily.GetMetric()[1].Label[0].GetName()
		if lName != scenario.expectLabelName {
			t.Errorf("%v: incorrect encoded label name, want %v, got %v", scenario.format, scenario.expectLabelName, lName)
		}
	}
}
