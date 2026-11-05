// Copyright 2016 The Prometheus Authors
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
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v2"
)

// LoadTLSConfig parses the given file into a tls.Config.
func LoadTLSConfig(filename string) (*tls.Config, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	cfg := TLSConfig{}
	switch filepath.Ext(filename) {
	case ".yml":
		if err = yaml.UnmarshalStrict(content, &cfg); err != nil {
			return nil, err
		}
	case ".json":
		decoder := json.NewDecoder(bytes.NewReader(content))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&cfg); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unknown extension: %s", filepath.Ext(filename))
	}
	return NewTLSConfig(&cfg)
}

var expectedTLSConfigs = []struct {
	filename string
	config   *tls.Config
}{
	{
		filename: "tls_config.empty.good.json",
		config:   &tls.Config{},
	},
	{
		filename: "tls_config.insecure.good.json",
		config:   &tls.Config{InsecureSkipVerify: true},
	},
	{
		filename: "tls_config.tlsversion.good.json",
		config:   &tls.Config{MinVersion: tls.VersionTLS11},
	},
	{
		filename: "tls_config.max_version.good.json",
		config:   &tls.Config{MaxVersion: tls.VersionTLS12},
	},
	{
		filename: "tls_config.empty.good.yml",
		config:   &tls.Config{},
	},
	{
		filename: "tls_config.insecure.good.yml",
		config:   &tls.Config{InsecureSkipVerify: true},
	},
	{
		filename: "tls_config.tlsversion.good.yml",
		config:   &tls.Config{MinVersion: tls.VersionTLS11},
	},
	{
		filename: "tls_config.max_version.good.yml",
		config:   &tls.Config{MaxVersion: tls.VersionTLS12},
	},
	{
		filename: "tls_config.max_and_min_version.good.yml",
		config:   &tls.Config{MaxVersion: tls.VersionTLS12, MinVersion: tls.VersionTLS11},
	},
	{
		filename: "tls_config.max_and_min_version_same.good.yml",
		config:   &tls.Config{MaxVersion: tls.VersionTLS12, MinVersion: tls.VersionTLS12},
	},
	{
		filename: "tls_config.cipher_suites.good.yml",
		config: &tls.Config{CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		}},
	},
	{
		filename: "tls_config.cipher_suites.good.json",
		config: &tls.Config{CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		}},
	},
}

func TestValidTLSConfig(t *testing.T) {
	for _, cfg := range expectedTLSConfigs {
		got, err := LoadTLSConfig("testdata/" + cfg.filename)
		require.NoErrorf(t, err, "Error parsing %s: %s", cfg.filename, err)
		// non-nil functions are never equal.
		got.GetClientCertificate = nil
		require.Truef(t, reflect.DeepEqual(got, cfg.config), "%v: unexpected config result: \n\n%v\n expected\n\n%v", cfg.filename, got, cfg.config)
	}
}

var invalidTLSConfigs = []struct {
	filename string
	errMsg   string
}{
	{
		filename: "tls_config.max_and_min_version.bad.yml",
		errMsg:   "tls_config.max_version must be greater than or equal to tls_config.min_version if both are specified",
	},
	{
		filename: "tls_config.cipher_suites.bad.yml",
		errMsg:   "unknown TLS cipher suite: TLS_NOT_A_CIPHER_SUITE",
	},
}

func TestInvalidTLSConfig(t *testing.T) {
	for _, ee := range invalidTLSConfigs {
		_, err := LoadTLSConfig("testdata/" + ee.filename)
		if err == nil {
			t.Error("Expected error with config but got none")
			continue
		}
		if !strings.Contains(err.Error(), ee.errMsg) {
			t.Errorf("Expected error for invalid HTTP client configuration to contain %q but got: %s", ee.errMsg, err)
		}
	}
}

func TestTLSVersionStringer(t *testing.T) {
	s := TLSVersion(tls.VersionTLS13)
	require.Equalf(t, "TLS13", s.String(), "tls.VersionTLS13 string should be TLS13, got %s", s.String())
}

func TestTLSVersionStringerUnknown(t *testing.T) {
	// An unknown TLS version (not present in TLSVersions) must fall back to its
	// numeric value, not to the pointer address.
	s := TLSVersion(0x9999) // 39321
	require.Equal(t, "39321", s.String())
}

func TestTLSVersionMarshalYAML(t *testing.T) {
	tests := []struct {
		input    TLSVersion
		expected string
		err      error
	}{
		{
			input:    TLSVersions["TLS13"],
			expected: "TLS13\n",
			err:      nil,
		},
		{
			input:    TLSVersions["TLS10"],
			expected: "TLS10\n",
			err:      nil,
		},
		{
			input:    TLSVersion(999),
			expected: "",
			err:      errors.New("unknown TLS version: 999"),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("MarshalYAML(%d)", test.input), func(t *testing.T) {
			actualBytes, err := yaml.Marshal(&test.input)
			if err != nil {
				if test.err == nil || err.Error() != test.err.Error() {
					t.Fatalf("error %v, expected %v", err, test.err)
				}
				return
			}
			actual := string(actualBytes)
			require.Equalf(t, test.expected, actual, "returned %s, expected %s", actual, test.expected)
		})
	}
}

func TestTLSVersionMarshalJSON(t *testing.T) {
	tests := []struct {
		input    TLSVersion
		expected string
		err      error
	}{
		{
			input:    TLSVersions["TLS13"],
			expected: `"TLS13"`,
			err:      nil,
		},
		{
			input:    TLSVersions["TLS10"],
			expected: `"TLS10"`,
			err:      nil,
		},
		{
			input:    TLSVersion(999),
			expected: "",
			err:      errors.New("unknown TLS version: 999"),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("MarshalJSON(%d)", test.input), func(t *testing.T) {
			actualBytes, err := json.Marshal(&test.input)
			if err != nil {
				if test.err == nil || !strings.HasSuffix(err.Error(), test.err.Error()) {
					t.Fatalf("error %v, expected %v", err, test.err)
				}
				return
			}
			actual := string(actualBytes)
			require.Equalf(t, test.expected, actual, "returned %s, expected %s", actual, test.expected)
		})
	}
}

func TestTLSCipherSuiteMarshal(t *testing.T) {
	cs := TLSCipherSuite(tls.TLS_RSA_WITH_AES_128_GCM_SHA256)

	y, err := yaml.Marshal(cs)
	require.NoError(t, err)
	require.Equal(t, "TLS_RSA_WITH_AES_128_GCM_SHA256\n", string(y))

	j, err := json.Marshal(cs)
	require.NoError(t, err)
	require.JSONEq(t, `"TLS_RSA_WITH_AES_128_GCM_SHA256"`, string(j))

	require.Equal(t, "TLS_RSA_WITH_AES_128_GCM_SHA256", cs.String())
}

func TestTLSCipherSuitesReachRSAKeyExchangeServer(t *testing.T) {
	// A server that only supports RSA key exchange, like many older appliances. Go removed
	// these suites from its default client list, so they must be configured explicitly.
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	server.TLS = &tls.Config{
		MaxVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{tls.TLS_RSA_WITH_AES_128_GCM_SHA256},
	}
	server.StartTLS()
	defer server.Close()

	get := func(cfg TLSConfig) error {
		cfg.InsecureSkipVerify = true
		tlsConfig, err := NewTLSConfig(&cfg)
		require.NoError(t, err)
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}
		resp, err := client.Get(server.URL)
		if err != nil {
			return err
		}
		return resp.Body.Close()
	}

	require.Error(t, get(TLSConfig{}), "the default cipher suites should not include RSA key exchange")
	require.NoError(t, get(TLSConfig{CipherSuites: []TLSCipherSuite{TLSCipherSuite(tls.TLS_RSA_WITH_AES_128_GCM_SHA256)}}))
}
