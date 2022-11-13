// Copyright 2022 The Prometheus Authors
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

// PROOF OF CONCEPT ONLY
//
// This is an experiment in providing a nicer experience for the "look at the
// metrics page in a browser" experience. It provides a path forward should we
// ever want to make a binary format the default for actual scraping. When
// negotiating a format with the client, we can take into account that the
// client may be a browser, and is asking for text/html.

package expfmt

import (
	"bytes"
	"html/template"
	"io"

	dto "github.com/prometheus/client_model/go"
)

// TODO: make this prettier
var preamble = []byte("<html><head><title>Metrics</title></head><body><h1>Metrics</h1>")
var metricFamilyTemplate = template.Must(template.New("metrics-page").Parse(`<pre>{{.}}</pre>`))
var postamble = []byte("</body></html>")

// MetricFamilyToHTML writes the HTML for a single MetricFamily.
func MetricFamilyToHTML(out io.Writer, in *dto.MetricFamily) error {
	buf := &bytes.Buffer{}

	_, err := MetricFamilyToText(buf, in)
	if err != nil {
		return nil
	}

	return metricFamilyTemplate.Execute(out, buf)
}

// HTMLPreamble writes the header and general front matter for the HTML
// representation.
func HTMLPreamble(out io.Writer) error {
	_, err := out.Write(preamble)
	return err
}

// HTMLPostamble writes the footer and closing tags for the HTML representation.
// It closes all tags opened in the preamble.
func HTMLPostamble(out io.Writer) error {
	_, err := out.Write(postamble)
	return err
}

func NewHTMLEncoder(w io.Writer) encoderCloser {
	// TODO: don't swallow errors. The current interfaces don't allow for a global preamble, so we're writing it right here 🤞
	_ = HTMLPreamble(w)
	return encoderCloser{
		encode: func(v *dto.MetricFamily) error {
			return MetricFamilyToHTML(w, v)
		},
		close: func() error { return HTMLPostamble(w) },
	}
}
