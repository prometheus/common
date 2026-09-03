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
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDebugRoundTripper(t *testing.T) {
	const responseBody = "complete response body"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, err := io.WriteString(w, responseBody)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	var output bytes.Buffer
	client := &http.Client{Transport: NewDebugRoundTripper(&output, http.DefaultTransport)}
	req, err := http.NewRequest(http.MethodGet, server.URL+"/alerts", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer authorization-secret")
	req.Header.Set("Cf-Access-Token", "cf-access-secret")
	req.Header.Set("X-Debug-Test", "visible")

	resp, err := client.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, resp.Body.Close()) })

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, responseBody, string(body))

	log := output.String()
	require.Contains(t, log, "--> GET "+server.URL+"/alerts")
	require.Contains(t, log, "Authorization: <redacted>")
	require.Contains(t, log, "Cf-Access-Token: <redacted>")
	require.Contains(t, log, "X-Debug-Test: visible")
	require.Contains(t, log, "<-- 200 OK")
	require.Contains(t, log, `body preview: "complete response body"`)
	require.NotContains(t, log, "authorization-secret")
	require.NotContains(t, log, "cf-access-secret")
}

func TestDebugRoundTripperError(t *testing.T) {
	expectedErr := errors.New("request failed")
	next := NewRoundTripCheckRequest(func(*http.Request) {}, nil, expectedErr)
	var output bytes.Buffer
	rt := NewDebugRoundTripper(&output, next)
	req, err := http.NewRequest(http.MethodGet, "https://example.com/alerts", http.NoBody)
	require.NoError(t, err)

	_, err = rt.RoundTrip(req)
	require.ErrorIs(t, err, expectedErr)
	require.True(t, strings.HasPrefix(output.String(), "--> GET https://example.com/alerts\n"))
	require.Contains(t, output.String(), "<-- error after ")
	require.Contains(t, output.String(), expectedErr.Error())
}
