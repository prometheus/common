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

func TestSecretField_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name           string
		yaml           string
		expectProvider Provider
		expectSettings SecretFieldSettings
		expectErr      string
	}{
		{
			name: "Unmarshal plain string into InlineProvider",
			yaml: `my_secret_value`,
			expectProvider: &InlineProvider{
				secret: "my_secret_value",
			},
		},
		{
			name: "Unmarshal file provider",
			yaml: `
file:
  path: /path/to/secret
`,
			expectProvider: &FileProvider{
				Path: "/path/to/secret",
			},
		},
		{
			name: "Unmarshal file provider with settings",
			yaml: `
file:
  path: /path/to/secret
refreshInterval: 5m
`,
			expectProvider: &FileProvider{
				Path: "/path/to/secret",
			},
			expectSettings: SecretFieldSettings{
				RefreshInterval: 5 * time.Minute,
			},
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
			name: "Error on unknown provider",
			yaml: `
moon_secret_manager:
  moon_phase: full
`,
			expectErr: `no valid secret provider found in configuration:`,
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
			var sf SecretField
			err := yaml.Unmarshal([]byte(tt.yaml), &sf)

			if tt.expectErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectProvider.Name(), sf.provider.Name())
				assert.Equal(t, tt.expectProvider, sf.provider)
				assert.Equal(t, tt.expectSettings, sf.settings)
			}
		})
	}
}

func TestSecretField_MarshalYAML(t *testing.T) {
	t.Run("Marshal non-inline provider", func(t *testing.T) {
		sf := SecretField{
			provider: &FileProvider{Path: "/path/to/token"},
		}
		b, err := yaml.Marshal(sf)
		require.NoError(t, err)
		expected := "file:\n  path: /path/to/token\n"
		assert.Equal(t, expected, string(b))
	})

	t.Run("Marshal non-inline provider with settings", func(t *testing.T) {
		sf := SecretField{
			provider: &FileProvider{Path: "/path/to/token"},
			settings: SecretFieldSettings{
				RefreshInterval: 10 * time.Minute,
			},
		}
		b, err := yaml.Marshal(sf)
		require.NoError(t, err)
		expected := "file:\n  path: /path/to/token\nrefreshInterval: 10m0s\n"
		assert.Equal(t, expected, string(b))
	})

	t.Run("Marshal inline provider without manager", func(t *testing.T) {
		sf := SecretField{
			provider: &InlineProvider{secret: "my-password"},
		}
		b, err := yaml.Marshal(sf)
		require.NoError(t, err)
		expected := "inline: <secret>\n"
		assert.Equal(t, expected, string(b))
	})

	t.Run("Marshal inline provider with manager and MarshalInlineSecrets=false", func(t *testing.T) {
		m := &Manager{MarshalInlineSecrets: false}
		sf := SecretField{
			manager:  m,
			provider: &InlineProvider{secret: "my-password"},
		}
		b, err := yaml.Marshal(sf)
		require.NoError(t, err)
		expected := "inline: <secret>\n"
		assert.Equal(t, expected, string(b))
	})

	t.Run("Marshal inline provider with manager and MarshalInlineSecrets=true", func(t *testing.T) {
		m := &Manager{MarshalInlineSecrets: true}
		sf := SecretField{
			manager:  m,
			provider: &InlineProvider{secret: "my-password"},
		}
		b, err := yaml.Marshal(sf)
		require.NoError(t, err)
		expected := "my-password\n" // Marshals as a plain string
		assert.Equal(t, expected, string(b))
	})
}

func TestSecretField_MarshalJSON(t *testing.T) {
	// JSON marshaling is just a wrapper around YAML marshaling, so a simple test is sufficient.
	sf := SecretField{
		provider: &FileProvider{Path: "/path/to/token"},
	}
	b, err := json.Marshal(sf)
	require.NoError(t, err)
	expected := `{"file":{"path":"/path/to/token"}}`
	assert.JSONEq(t, expected, string(b))
}

func TestSecretField_ManagerPanics(t *testing.T) {
	sf := SecretField{} // No manager attached

	assert.PanicsWithValuef(t, "secret field has not been discovered by a manager; was NewManager(&cfg) called?", func() { sf.Get() }, "Get should panic without a manager")
	assert.PanicsWithValuef(t, "secret field has not been discovered by a manager; was NewManager(&cfg) called?", func() { sf.TriggerRefresh() }, "TriggerRefresh should panic without a manager")
}
