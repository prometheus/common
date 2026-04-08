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

// This package no longer handles safe yaml parsing. In order to
// ensure correct yaml unmarshalling, use "yaml.UnmarshalStrict()".

package config

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReservedHeaders(t *testing.T) {
	for k := range ReservedHeaders {
		l := http.CanonicalHeaderKey(k)
		if k != l {
			t.Errorf("ReservedHeaders keys should be lowercase: got %q, expected %q", k, http.CanonicalHeaderKey(k))
		}
	}
}

func TestHeadersRoundTripperSameHost(t *testing.T) {
	// All headers, including sensitive ones, must be forwarded on same-host requests.
	for _, header := range []string{"Cookie", "X-Custom-Header"} {
		t.Run(header, func(t *testing.T) {
			received := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received = r.Header.Get(header)
				fmt.Fprint(w, "ok")
			}))
			t.Cleanup(server.Close)

			headers := &Headers{
				Headers: map[string]Header{
					header: {Values: []string{"testvalue"}},
				},
			}
			rt := NewHeadersRoundTripper(headers, http.DefaultTransport)

			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			require.NoError(t, err)

			resp, err := rt.RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.Equal(t, "ok", strings.TrimSpace(string(body)))
			require.Equalf(t, "testvalue", received, "header %q must be forwarded on same-host request", header)
		})
	}
}

func TestHeadersRoundTripperCrossHostRedirect(t *testing.T) {
	// Cookie must be set on the initial request but stripped on cross-host redirects.
	cookieOnRedirect := ""
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieOnRedirect = r.Header.Get("Cookie")
		fmt.Fprint(w, "ok")
	}))
	t.Cleanup(target.Close)

	// Use "localhost" as the redirect target hostname so that it differs from
	// "127.0.0.1" used by the origin server, making it a cross-host redirect.
	targetPort := target.Listener.Addr().(*net.TCPAddr).Port
	targetURL := fmt.Sprintf("http://localhost:%d", targetPort)

	cookieOnOrigin := ""
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookieOnOrigin = r.Header.Get("Cookie")
		http.Redirect(w, r, targetURL, http.StatusFound)
	}))
	t.Cleanup(origin.Close)

	cfg := HTTPClientConfig{
		FollowRedirects: true,
		HTTPHeaders: &Headers{
			Headers: map[string]Header{
				"Cookie": {Values: []string{"session=abc"}},
			},
		},
	}
	client, err := NewClientFromConfig(cfg, "test")
	require.NoError(t, err)

	resp, err := client.Get(origin.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equalf(t, "session=abc", cookieOnOrigin, "Cookie must be set on the initial request.")
	require.Empty(t, cookieOnRedirect, "Cookie must not be forwarded on a cross-host redirect.")
}

func TestHeadersRoundTripperSameHostRedirect(t *testing.T) {
	// Cookie must be forwarded on same-host redirects.
	mux := http.NewServeMux()
	cookieOnRedirect := ""
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/end", http.StatusFound)
	})
	mux.HandleFunc("/end", func(w http.ResponseWriter, r *http.Request) {
		cookieOnRedirect = r.Header.Get("Cookie")
		fmt.Fprint(w, "ok")
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	cfg := HTTPClientConfig{
		FollowRedirects: true,
		HTTPHeaders: &Headers{
			Headers: map[string]Header{
				"Cookie": {Values: []string{"session=abc"}},
			},
		},
	}
	client, err := NewClientFromConfig(cfg, "test")
	require.NoError(t, err)

	resp, err := client.Get(server.URL + "/start")
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	require.Equalf(t, "session=abc", cookieOnRedirect, "Cookie must be forwarded on a same-host redirect.")
}
