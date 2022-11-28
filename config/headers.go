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
	"net/textproto"
	"os"
	"strings"
)

// reservedHeaders that change the connection, are set by Prometheus, or car be set
// otherwise can't be changed.
var reservedHeaders = map[string]struct{}{
	"authorization":                       {},
	"host":                                {},
	"content-encoding":                    {},
	"content-length":                      {},
	"content-type":                        {},
	"user-agent":                          {},
	"connection":                          {},
	"keep-alive":                          {},
	"proxy-authenticate":                  {},
	"proxy-authorization":                 {},
	"www-authenticate":                    {},
	"accept-encoding":                     {},
	"x-prometheus-remote-write-version":   {},
	"x-prometheus-remote-read-version":    {},
	"x-prometheus-scrape-timeout-seconds": {},

	// Added by SigV4.
	"x-amz-date":           {},
	"x-amz-security-token": {},
	"x-amz-content-sha256": {},
}

// Headers represents the configuration for HTTP headers.
type Headers struct {
	Headers       map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	SecretHeaders map[string]Secret `yaml:"secret_headers,omitempty" json:"secret_headers,omitempty"`
	Files         map[string]string `yaml:"files,omitempty" json:"files,omitempty"`
	dir           string
}

// SetDirectory records the directory to make headers file relative to the
// configuration file.
func (h *Headers) SetDirectory(dir string) {
	h.dir = dir
}

// Validate validates the Headers config.
func (h *Headers) Validate() error {
	uniqueHeaders := make(map[string]struct{}, len(h.Headers))
	for k := range h.Headers {
		uniqueHeaders[strings.ToLower(k)] = struct{}{}
	}
	for k := range h.SecretHeaders {
		if _, ok := uniqueHeaders[strings.ToLower(k)]; ok {
			return fmt.Errorf("header %q is defined in multiple sections", textproto.CanonicalMIMEHeaderKey(k))
		}
		uniqueHeaders[strings.ToLower(k)] = struct{}{}
	}
	for k, v := range h.Files {
		if _, ok := uniqueHeaders[strings.ToLower(k)]; ok {
			return fmt.Errorf("header %q is defined in multiple sections", textproto.CanonicalMIMEHeaderKey(k))
		}
		uniqueHeaders[strings.ToLower(k)] = struct{}{}
		f := JoinDir(h.dir, v)
		_, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("unable to read header %q from file %s: %w", textproto.CanonicalMIMEHeaderKey(k), f, err)
		}
	}
	for k := range uniqueHeaders {
		if _, ok := reservedHeaders[strings.ToLower(k)]; ok {
			return fmt.Errorf("setting header %q is not allowed", textproto.CanonicalMIMEHeaderKey(k))
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
		req.Header.Set(textproto.CanonicalMIMEHeaderKey(k), v)
	}
	for k, v := range rt.config.SecretHeaders {
		req.Header.Set(textproto.CanonicalMIMEHeaderKey(k), string(v))
	}
	for k, v := range rt.config.Files {
		f := JoinDir(rt.config.dir, v)
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("unable to read headers file %s: %w", f, err)
		}
		req.Header.Set(textproto.CanonicalMIMEHeaderKey(k), strings.TrimSpace(string(b)))
	}
	return rt.next.RoundTrip(req)
}

// CloseIdleConnections implements closeIdler.
func (rt *headersRoundTripper) CloseIdleConnections() {
	if ci, ok := rt.next.(closeIdler); ok {
		ci.CloseIdleConnections()
	}
}
