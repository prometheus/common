// Copyright 2015 The Prometheus Authors
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
	"errors"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/prometheus/common/model"
)

var (
	ts = model.Now()
	in = `
# Only a quite simple scenario with two metric families.
# More complicated tests of the parser itself can be found in the text package.
# TYPE mf2 counter
mf2 3
mf1{label="value1"} -3.14 123456
mf1{label="value2"} 42
mf2 4
`
	out = model.Samples{
		&model.Sample{
			Metric:    model.Metric{model.MetricNameLabel: "mf1", "label": "value1"},
			Value:     -3.14,
			Timestamp: 123456,
		},
		&model.Sample{
			Metric:    model.Metric{model.MetricNameLabel: "mf1", "label": "value2"},
			Value:     42,
			Timestamp: ts,
		},
		&model.Sample{
			Metric:    model.Metric{model.MetricNameLabel: "mf2"},
			Value:     3,
			Timestamp: ts,
		},
		&model.Sample{
			Metric:    model.Metric{model.MetricNameLabel: "mf2"},
			Value:     4,
			Timestamp: ts,
		},
	}
)

func TestTextDecoder(t *testing.T) {
	dec := &SampleDecoder{
		Dec: &textDecoder{r: strings.NewReader(in)},
		Opts: &DecodeOptions{
			Timestamp: ts,
		},
	}
	var all model.Samples
	for {
		var smpls model.Samples
		err := dec.Decode(&smpls)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		all = append(all, smpls...)
	}
	sort.Sort(all)
	sort.Sort(out)
	if !reflect.DeepEqual(all, out) {
		t.Fatalf("output does not match")
	}
}

func testDiscriminatorHTTPHeader(t testing.TB) {
	var scenarios = []struct {
		input  map[string]string
		output Decoder
		err    error
	}{
		{
			input:  map[string]string{"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="delimited"`},
			output: &protoDecoder{},
			err:    nil,
		},
		{
			input:  map[string]string{"Content-Type": `application/vnd.google.protobuf; proto="illegal"; encoding="delimited"`},
			output: nil,
			err:    errors.New("unrecognized protocol message illegal"),
		},
		{
			input:  map[string]string{"Content-Type": `application/vnd.google.protobuf; proto="io.prometheus.client.MetricFamily"; encoding="illegal"`},
			output: nil,
			err:    errors.New("unsupported encoding illegal"),
		},
		{
			input:  map[string]string{"Content-Type": `text/plain; version=0.0.4`},
			output: &textDecoder{},
			err:    nil,
		},
		{
			input:  map[string]string{"Content-Type": `text/plain`},
			output: &textDecoder{},
			err:    nil,
		},
		{
			input:  map[string]string{"Content-Type": `text/plain; version=0.0.3`},
			output: nil,
			err:    errors.New("unrecognized protocol version 0.0.3"),
		},
	}

	for i, scenario := range scenarios {
		var header http.Header

		if len(scenario.input) > 0 {
			header = http.Header{}
		}

		for key, value := range scenario.input {
			header.Add(key, value)
		}

		actual, err := NewDecoder(nil, header)

		if scenario.err != err {
			if scenario.err != nil && err != nil {
				if scenario.err.Error() != err.Error() {
					t.Errorf("%d. expected %s, got %s", i, scenario.err, err)
				}
			} else if scenario.err != nil || err != nil {
				t.Errorf("%d. expected %s, got %s", i, scenario.err, err)
			}
		}

		if !reflect.DeepEqual(scenario.output, actual) {
			t.Errorf("%d. expected %s, got %s", i, scenario.output, actual)
		}
	}
}

func TestDiscriminatorHTTPHeader(t *testing.T) {
	testDiscriminatorHTTPHeader(t)
}

func BenchmarkDiscriminatorHTTPHeader(b *testing.B) {
	for i := 0; i < b.N; i++ {
		testDiscriminatorHTTPHeader(b)
	}
}
