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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/common/promslog"
)

// testConfig is a struct used for discovering SecretFields in tests.
type testConfig struct {
	APIKeys []Field `yaml:"api_keys"`
}

func fetchMockProvider(t *testing.T, field Field) *MockProvider {
	config, ok := field.state.config.(*MockProvider)
	require.Truef(t, ok, "fetching non-mock provider")
	return config
}

func TestNewManager(t *testing.T) {
	content := `
api_keys:
  - mock: secret1
  - mock: secret2
  - "inline_secret"
`

	m, cfg, _ := SetupManagerForTest[testConfig](t, content, &MockProvider{})

	require.Lenf(t, m.secrets, 3, "Manager should discover 3 secrets")

	assert.Equal(t, "mock>secret1", cfg.APIKeys[0].state.getDedupID())
	assert.Equal(t, "mock>secret2", cfg.APIKeys[1].state.getDedupID())
	assert.Regexp(t, "inline>", cfg.APIKeys[2].state.getDedupID())
	assert.NotNil(t, m.secrets[cfg.APIKeys[0].state.getDedupID()])
	assert.NotNil(t, m.secrets[cfg.APIKeys[1].state.getDedupID()])
	assert.NotNil(t, m.secrets[cfg.APIKeys[2].state.getDedupID()])
}

func TestNewManagerConfigMultiError(t *testing.T) {
	content := `
api_keys:
  - mock:
      apple: 1
  - mock:
      bee: 2
  - nil: inline_secret
`

	cfg := ParseConfig[testConfig](t, content)
	reg := prometheus.NewRegistry()
	_, err := NewManager(promslog.NewNopLogger(), reg, Providers, cfg)
	require.ErrorContains(t, err, "testConfig.APIKeys[0]")
	require.ErrorContains(t, err, "testConfig.APIKeys[1]")
	require.ErrorContains(t, err, "testConfig.APIKeys[2]")
}

func TestManager_SecretLifecycle(t *testing.T) {
	content := `
api_keys:
  - mock: initial_secret
    options:
      refresh_interval: 50ms
`
	_, cfg, populate := SetupManagerForTest[testConfig](t, content, &MockProvider{ProviderID: "stable"})
	mock := fetchMockProvider(t, cfg.APIKeys[0])

	// 1. Initial fetch
	require.Eventuallyf(t, mock.HasFetchedLatest, time.Second, 10*time.Millisecond, "Initial fetch should occur")
	cfg = populate()
	assert.Equal(t, "initial_secret", cfg.APIKeys[0].Value())

	// 2. Scheduled refresh
	mock.SetSecret("refreshed_secret")
	require.Eventuallyf(t, mock.HasFetchedLatest, time.Second, 10*time.Millisecond, "Scheduled refresh should occur")

	cfg = populate()
	assert.Equal(t, "refreshed_secret", cfg.APIKeys[0].Value())

	// 3. Triggered refresh
	mock.SetSecret("triggered_secret")
	cfg.APIKeys[0].TriggerRefresh()
	require.Eventuallyf(t, mock.HasFetchedLatest, time.Second, 10*time.Millisecond, "Triggered refresh should occur")
	cfg = populate()
	assert.Equal(t, "triggered_secret", cfg.APIKeys[0].Value())
}

func TestManager_FetchErrorAndRecovery(t *testing.T) {
	content := `
api_keys:
  - mock: ""
`
	_, cfg, populate := SetupManagerForTest[testConfig](t, content, &MockProvider{
		fetchErr:   errors.New("fetch failed"),
		ProviderID: "stable",
	})
	mock := fetchMockProvider(t, cfg.APIKeys[0])

	// Initial fetch fails.
	assert.Truef(t, mock.HasFetchedLatest(), "A fetch should have been attempted")
	assert.Emptyf(t, cfg.APIKeys[0].Value(), "Secret should be empty after failed fetch")

	// Recovery.
	mock.SetFetchError(nil)
	mock.SetSecret("recovered_secret")
	require.Eventuallyf(t, func() bool {
		return populate().APIKeys[0].Value() == "recovered_secret"
	}, 2*time.Second, 50*time.Millisecond, "Manager should eventually get the correct secret")
}

func TestManager_InlineSecret(t *testing.T) {
	inlineSecret := "this-is-inline"
	content := fmt.Sprintf(`
api_keys:
  - "%s"
`, inlineSecret)
	_, cfg, _ := SetupManagerForTest[testConfig](t, content, &MockProvider{})
	assert.Equal(t, inlineSecret, cfg.APIKeys[0].Value())
}
