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

// This package no longer handles safe yaml parsing. In order to
// ensure correct yaml unmarshalling, use "yaml.UnmarshalStrict()".

package config

import (
	"fmt"
	"net/http"
	"os"
	"strings"
)

// reservedHeaders that change the connection, are set by Prometheus, or car be set
// otherwise can't be changed.
var reservedHeaders = map[string]struct{}{
	"Authorization":                       {},
	"Host":                                {},
	"Content-Encoding":                    {},
	"Content-Length":                      {},
	"Content-Type":                        {},
	"User-Agent":                          {},
	"Connection":                          {},
	"Keep-Alive":                          {},
	"Proxy-Authenticate":                  {},
	"Proxy-Authorization":                 {},
	"Www-Authenticate":                    {},
	"Accept-Encoding":                     {},
	"X-Prometheus-Remote-Write-Version":   {},
	"X-Prometheus-Remote-Read-Version":    {},
	"X-Prometheus-Scrape-Timeout-Seconds": {},

	// Added by SigV4.
	"X-Amz-Date":           {},
	"X-Amz-Security-Token": {},
	"X-Amz-Content-Sha256": {},
}

// Headers represents the configuration for HTTP headers.
type Headers struct {
	Headers       map[string][]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	SecretHeaders map[string][]Secret `yaml:"secret_headers,omitempty" json:"secret_headers,omitempty"`
	Files         map[string][]string `yaml:"files,omitempty" json:"files,omitempty"`
	dir           string
}

// SetDirectory records the directory to make headers file relative to the
// configuration file.
func (h *Headers) SetDirectory(dir string) {
	if h == nil {
		return
	}
	h.dir = dir
}

// Validate validates the Headers config.
func (h *Headers) Validate() error {
	uniqueHeaders := make(map[string]struct{}, len(h.Headers))
	for k := range h.Headers {
		uniqueHeaders[http.CanonicalHeaderKey(k)] = struct{}{}
	}
	for k := range h.SecretHeaders {
		if _, ok := uniqueHeaders[http.CanonicalHeaderKey(k)]; ok {
			return fmt.Errorf("header %q is defined in multiple sections", http.CanonicalHeaderKey(k))
		}
		uniqueHeaders[http.CanonicalHeaderKey(k)] = struct{}{}
	}
	for k, v := range h.Files {
		if _, ok := uniqueHeaders[http.CanonicalHeaderKey(k)]; ok {
			return fmt.Errorf("header %q is defined in multiple sections", http.CanonicalHeaderKey(k))
		}
		uniqueHeaders[http.CanonicalHeaderKey(k)] = struct{}{}
		for _, file := range v {
			f := JoinDir(h.dir, file)
			_, err := os.ReadFile(f)
			if err != nil {
				return fmt.Errorf("unable to read header %q from file %s: %w", http.CanonicalHeaderKey(k), f, err)
			}
		}
	}
	for k := range uniqueHeaders {
		if _, ok := reservedHeaders[http.CanonicalHeaderKey(k)]; ok {
			return fmt.Errorf("setting header %q is not allowed", http.CanonicalHeaderKey(k))
		}
	}
	return nil
}

// NewHeadersRoundTripper returns a RoundTripper that sets HTTP headers on
// requests as configured.
func NewHeadersRoundTripper(config *Headers, next http.RoundTripper) http.RoundTripper {
	if len(config.Headers)+len(config.SecretHeaders)+len(config.Files) == 0 {
		return next
	}
	return &headersRoundTripper{
		config: config,
		next:   next,
	}
}

type headersRoundTripper struct {
	next   http.RoundTripper
	config *Headers
}

// RoundTrip implements http.RoundTripper.
func (rt *headersRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = cloneRequest(req)
	for k, v := range rt.config.Headers {
		for i, h := range v {
			if i == 0 {
				req.Header.Set(k, h)
				continue
			}
			req.Header.Add(k, h)
		}
	}
	for k, v := range rt.config.SecretHeaders {
		for i, h := range v {
			if i == 0 {
				req.Header.Set(k, string(h))
				continue
			}
			req.Header.Add(k, string(h))
		}
	}
	for k, v := range rt.config.Files {
		for i, h := range v {
			f := JoinDir(rt.config.dir, h)
			b, err := os.ReadFile(f)
			if err != nil {
				return nil, fmt.Errorf("unable to read headers file %s: %w", f, err)
			}
			if i == 0 {
				req.Header.Set(k, strings.TrimSpace(string(b)))
				continue
			}
			req.Header.Add(k, strings.TrimSpace(string(b)))
		}
	}
	return rt.next.RoundTrip(req)
}

// CloseIdleConnections implements closeIdler.
func (rt *headersRoundTripper) CloseIdleConnections() {
	if ci, ok := rt.next.(closeIdler); ok {
		ci.CloseIdleConnections()
	}
}
