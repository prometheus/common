// Copyright The Prometheus Authors
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

package config

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"
)

const debugBodyPreviewLimit = 512

var redactedDebugRequestHeaders = map[string]struct{}{
	"authorization":           {},
	"cf-access-client-id":     {},
	"cf-access-client-secret": {},
	"cf-access-token":         {},
	"proxy-authorization":     {},
}

// NewDebugRoundTripper returns a RoundTripper that writes outgoing HTTP
// requests and their responses to out. Credential-bearing request headers are
// redacted, and response body previews are limited to 512 bytes.
func NewDebugRoundTripper(out io.Writer, next http.RoundTripper) http.RoundTripper {
	return &debugRoundTripper{out: out, next: next}
}

type debugRoundTripper struct {
	out  io.Writer
	next http.RoundTripper
}

// RoundTrip implements http.RoundTripper.
func (rt *debugRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	fmt.Fprintf(rt.out, "--> %s %s\n", req.Method, req.URL)

	trace := &httptrace.ClientTrace{
		WroteHeaderField: func(key string, values []string) {
			fmt.Fprintf(rt.out, "    %s: %s\n", key, redactDebugHeader(key, values))
		},
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

	start := time.Now()
	resp, err := rt.next.RoundTrip(req)
	elapsed := time.Since(start)
	if err != nil {
		fmt.Fprintf(rt.out, "<-- error after %s: %v\n", elapsed, err)
		return resp, err
	}

	fmt.Fprintf(rt.out, "<-- %s in %s (content-type: %s)\n", resp.Status, elapsed, resp.Header.Get("Content-Type"))

	if resp.Body != nil {
		preview := make([]byte, debugBodyPreviewLimit)
		n, _ := io.ReadFull(resp.Body, preview)
		resp.Body = &previewedBody{
			preview: preview[:n],
			rest:    resp.Body,
		}
		if n > 0 {
			fmt.Fprintf(rt.out, "    body preview: %q\n", preview[:n])
		}
	}

	return resp, nil
}

func (rt *debugRoundTripper) CloseIdleConnections() {
	if ci, ok := rt.next.(closeIdler); ok {
		ci.CloseIdleConnections()
	}
}

func redactDebugHeader(name string, values []string) string {
	if _, ok := redactedDebugRequestHeaders[strings.ToLower(name)]; ok {
		return "<redacted>"
	}
	return strings.Join(values, ", ")
}

type previewedBody struct {
	preview []byte
	off     int
	rest    io.ReadCloser
}

func (b *previewedBody) Read(p []byte) (int, error) {
	if b.off < len(b.preview) {
		n := copy(p, b.preview[b.off:])
		b.off += n
		return n, nil
	}
	return b.rest.Read(p)
}

func (b *previewedBody) Close() error {
	return b.rest.Close()
}
