// Copyright 2025 The Prometheus Authors
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

package secrets

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v2"
)

func TestField_ParseRawConfig(t *testing.T) {
	tests := []struct {
		name                 string
		yaml                 string
		expectProviderName   string
		expectProviderConfig ProviderConfig
		expectSettings       FieldSettings
		expectedID           string
		expectErr            string
	}{
		{
			name:               "Unmarshal plain string into InlineProvider",
			yaml:               `my_secret_value`,
			expectProviderName: "inline",
			expectProviderConfig: &InlineProviderConfig{
				secret: "my_secret_value",
			},
			expectedID: "inline>",
		},
		{
			name:               "Unmarshal InlineProvider shorthand",
			yaml:               `inline: my_secret_value`,
			expectProviderName: "inline",
			expectProviderConfig: &InlineProviderConfig{
				secret: "my_secret_value",
			},
			expectedID: "inline>",
		},
		{
			name:      "Unmarshal nil shorthand",
			yaml:      `nil: my_secret_value`,
			expectErr: "not support instantiation from string",
		},
		{
			name: "Unmarshal file provider",
			yaml: `
file:
  path: /path/to/secret
`,
			expectProviderName: "file",
			expectProviderConfig: &FileProviderConfig{
				Path: "/path/to/secret",
			},
			expectedID: "file>/path/to/secret",
		},
		{
			name: "Unmarshal file provider shorthand",
			yaml: `
file: /path/to/secret
`,
			expectProviderName: "file",
			expectProviderConfig: &FileProviderConfig{
				Path: "/path/to/secret",
			},
			expectedID: "file>/path/to/secret",
		},
		{
			name: "Unmarshal file provider with settings",
			yaml: `
file:
  path: /path/to/secret
options:
  refresh_interval: 5m
`,
			expectProviderName: "file",
			expectProviderConfig: &FileProviderConfig{
				Path: "/path/to/secret",
			},
			expectSettings: FieldSettings{
				RefreshInterval: 5 * time.Minute,
			},
			expectedID: "file>/path/to/secret",
		},
		{
			name: "Error on multiple providers",
			yaml: `
file:
  path: /path/to/secret
inline: another_secret
`,
			expectErr: "secret must contain exactly one provider type, but multiple were found: ",
		},
		{
			name: "Error on no provider",
			yaml: `
options:
  refresh_interval: 5m
`,
			expectErr: `no valid secret provider found in configuration:`,
		},
		{
			name: "Error on unknown provider",
			yaml: `
moon_secret_manager:
  moon_phase: full
`,
			expectErr: `unknown provider: moon_secret_manager`,
		},
		{
			name: "Error on invalid provider config",
			yaml: `
file:
  path: [ "this", "should", "be", "a", "string" ]
`,
			expectErr: "failed to unmarshal into file provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sf Field
			err := yaml.Unmarshal([]byte(tt.yaml), &sf)
			require.NoError(t, err)

			state, err := sf.parseRawConfig(Providers, "test_path")

			if tt.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectProviderName, state.providerName)
				assert.Regexp(t, tt.expectedID, state.getDedupID())
				assert.Equal(t, tt.expectProviderConfig, state.config)
				assert.Equal(t, tt.expectSettings, state.settings)
			}
		})
	}
}

func TestField_MarshalYAML(t *testing.T) {
	tests := []struct {
		name         string
		field        Field
		policy       PolicyFunc
		expectedYAML string
	}{
		{
			name: "Should marshal raw config when policy is true",
			field: Field{
				rawConfig: map[string]interface{}{
					"file": map[string]interface{}{
						"path": "/path/to/token",
					},
				},
			},
			policy:       func() bool { return true },
			expectedYAML: "file:\n  path: /path/to/token\n",
		},
		{
			name: "Should marshal as <secret> when policy is false",
			field: Field{
				rawConfig: "supersecret",
			},
			policy:       func() bool { return false },
			expectedYAML: "<secret>\n",
		},
		{
			name: "Should marshal inline secret when policy is true",
			field: Field{
				rawConfig: "inline_secret",
			},
			policy:       func() bool { return true },
			expectedYAML: "inline_secret\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentPolicy = tt.policy
			defer func() {
				currentPolicy = func() bool { return true }
				policyInitialized.Store(false)
			}()

			b, err := yaml.Marshal(tt.field)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedYAML, string(b))
		})
	}
}

func TestSecretField_MarshalJSON(t *testing.T) {
	// JSON marshaling is just a wrapper around YAML marshaling, so a simple test is sufficient.
	sf := Field{
		rawConfig: map[string]interface{}{
			"file": map[string]interface{}{
				"path": "/path/to/token",
			},
		},
	}
	b, err := json.Marshal(sf)
	require.NoError(t, err)
	expected := `{"file":{"path":"/path/to/token"}}`
	assert.JSONEq(t, expected, string(b))
}
