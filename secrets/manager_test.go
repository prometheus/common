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
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v2"
)

// mockProvider allows controlling the secret value and simulating errors for tests.
type mockProvider struct {
	mtx           sync.RWMutex
	secret        string
	fetchErr      error
	fetchedLatest bool
	blockChan     chan struct{}
	releaseChan   chan struct{}
}

func (mp *mockProvider) FetchSecret(ctx context.Context) (string, error) {
	// Block if the test requires it, to simulate fetch latency.
	if mp.blockChan != nil {
		select {
		case <-mp.blockChan:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	// Release if the test requires it, to signal fetch has started.
	if mp.releaseChan != nil {
		close(mp.releaseChan)
	}

	mp.mtx.RLock()
	defer mp.mtx.RUnlock()
	mp.fetchedLatest = true
	return mp.secret, mp.fetchErr
}

func (mp *mockProvider) setSecret(s string) {
	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	mp.fetchedLatest = false
	mp.secret = s
}

func (mp *mockProvider) setFetchError(err error) {
	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	mp.fetchedLatest = false
	mp.fetchErr = err
}

func (mp *mockProvider) hasFetchedLatest() bool {
	mp.mtx.Lock()
	defer mp.mtx.Unlock()
	return mp.fetchedLatest
}

type mockProviderConfig struct {
	Secret   string `yaml:"secret"`
	provider *mockProvider
}

func (mpc *mockProviderConfig) ID() string {
	return mpc.Secret
}

func (mpc *mockProviderConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err == nil {
		mpc.Secret = s
		return nil
	}
	type plain mockProviderConfig
	return unmarshal((*plain)(mpc))
}

func (mpc *mockProviderConfig) NewProvider() (Provider, error) {
	if mpc.provider == nil {
		mpc.provider = &mockProvider{secret: mpc.Secret}
	}
	return mpc.provider, nil
}

func (mpc *mockProviderConfig) Clone() ProviderConfig {
	return &mockProviderConfig{
		provider: mpc.provider,
	}
}

// testConfig is a struct used for discovering SecretFields in tests.
type testConfig struct {
	APIKeys []Field `yaml:"api_keys"`
}

func setupManagerTest(t *testing.T, content string) (*Manager, *testConfig) {
	reg := prometheus.NewRegistry()
	t.Cleanup(func() {
		currentPolicy = func() bool { return true }
		policyInitialized = atomic.Bool{}
	})

	providerReg := &ProviderRegistry{}
	providerReg.Register("inline", &InlineProviderConfig{})
	providerReg.Register("file", &FileProviderConfig{})
	providerReg.Register("mock", &mockProviderConfig{})

	cfg := &testConfig{}
	require.NoError(t, yaml.Unmarshal([]byte(content), cfg))

	m, err := NewManager(reg, providerReg, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	t.Cleanup(cancel)
	t.Cleanup(m.Stop)
	return m, cfg
}

func fetchMockProvider(t *testing.T, field Field) *mockProvider {
	config, ok := field.state.config.(*mockProviderConfig)
	require.Truef(t, ok, "fetching non-mock provider")
	return config.provider
}

func TestNewManager(t *testing.T) {
	content := `
api_keys:
  - mock: secret1
  - mock: secret2
  - "inline_secret"
`

	m, cfg := setupManagerTest(t, content)

	require.Lenf(t, m.secrets, 3, "Manager should discover 3 secrets")

	assert.Equal(t, "mock>secret1", cfg.APIKeys[0].state.id())
	assert.Equal(t, "mock>secret2", cfg.APIKeys[1].state.id())
	assert.Equal(t, "inline>APIKeys.[2]", cfg.APIKeys[2].state.id())
	assert.NotNil(t, m.secrets[cfg.APIKeys[0].state.id()])
	assert.NotNil(t, m.secrets[cfg.APIKeys[1].state.id()])
	assert.NotNil(t, m.secrets[cfg.APIKeys[2].state.id()])
}

func TestManager_SecretLifecycle(t *testing.T) {
	content := `
api_keys:
  - mock: initial_secret
    refreshInterval: 50ms
`
	m, cfg := setupManagerTest(t, content)
	mock := fetchMockProvider(t, cfg.APIKeys[0])

	// 1. Initial fetch
	require.Eventuallyf(t, mock.hasFetchedLatest, time.Second, 10*time.Millisecond, "Initial fetch should occur")
	ready, err := m.SecretsReady(cfg)
	require.NoError(t, err)
	assert.Truef(t, ready, "Secrets should be ready after initial fetch")
	assert.Equal(t, "initial_secret", cfg.APIKeys[0].Get())

	// 2. Scheduled refresh
	mock.setSecret("refreshed_secret")
	require.Eventuallyf(t, mock.hasFetchedLatest, time.Second, 10*time.Millisecond, "Scheduled refresh should occur")
	_, err = m.SecretsReady(cfg)
	require.NoError(t, err)
	assert.Equal(t, "refreshed_secret", cfg.APIKeys[0].Get())

	// 3. Triggered refresh
	mock.setSecret("triggered_secret")
	cfg.APIKeys[0].TriggerRefresh()
	require.Eventuallyf(t, mock.hasFetchedLatest, time.Second, 10*time.Millisecond, "Triggered refresh should occur")
	_, err = m.SecretsReady(cfg)
	require.NoError(t, err)
	assert.Equal(t, "triggered_secret", cfg.APIKeys[0].Get())
}

func TestManager_FetchErrorAndRecovery(t *testing.T) {
	content := `
api_keys:
  - mock: ""
`
	m, cfg := setupManagerTest(t, content)
	mock := fetchMockProvider(t, cfg.APIKeys[0])
	mock.setFetchError(errors.New("fetch failed"))

	// Initial fetch fails.
	require.Eventuallyf(t, mock.hasFetchedLatest, time.Second, 10*time.Millisecond, "A fetch should be attempted")
	assert.Emptyf(t, cfg.APIKeys[0].Get(), "Secret should be empty after failed fetch")

	ready, err := m.SecretsReady(cfg)
	require.NoError(t, err)
	assert.Falsef(t, ready, "Secrets should not be ready after failed fetch")

	// Recovery.
	mock.setFetchError(nil)
	mock.setSecret("recovered_secret")
	require.Eventuallyf(t, func() bool {
		ready, err := m.SecretsReady(cfg)
		require.NoError(t, err)
		return ready
	}, 2*time.Second, 50*time.Millisecond, "Manager should recover after error")

	assert.Equal(t, "recovered_secret", cfg.APIKeys[0].Get())
	ready, err = m.SecretsReady(cfg)
	require.NoError(t, err)
	assert.Truef(t, ready, "Secrets should be ready after recovery")
}

func TestManager_InlineSecret(t *testing.T) {
	inlineSecret := "this-is-inline"
	content := fmt.Sprintf(`
api_keys:
  - "%s"
`, inlineSecret)
	_, cfg := setupManagerTest(t, content)
	assert.Equal(t, inlineSecret, cfg.APIKeys[0].Get())
}
