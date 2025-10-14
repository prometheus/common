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
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func newMockProvider(secret string) *mockProvider {
	return &mockProvider{secret: secret}
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

func (*mockProvider) Name() string { return "mock" }

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

// mockValidator allows controlling validation logic for tests.
type mockValidator struct {
	mtx            sync.RWMutex
	secrets        map[string]bool
	settings       ValidationSettings
	verifiedLatest string
}

func newMockValidator() *mockValidator {
	return &mockValidator{
		secrets:  make(map[string]bool),
		settings: DefaultValidationSettings(),
	}
}

func (mv *mockValidator) Validate(_ context.Context, secret string) bool {
	mv.mtx.Lock()
	defer mv.mtx.Unlock()
	m, e := mv.secrets[secret]
	mv.verifiedLatest = secret
	return m && e
}

func (mv *mockValidator) Settings() ValidationSettings {
	return mv.settings
}

func (mv *mockValidator) setValid(secret string, isValid bool) {
	mv.mtx.Lock()
	defer mv.mtx.Unlock()
	mv.secrets[secret] = isValid
}

// testConfig is a struct used for discovering SecretFields in tests.
type testConfig struct {
	APIKeys []SecretField `yaml:"api_keys"`
}

func setupManagerTest(t *testing.T, cfg *testConfig) (*Manager, *prometheus.Registry) {
	// Register the mock provider for tests.
	originalProviders := Providers
	Providers = &ProviderRegistry{}
	Providers.Register(func() Provider { return &InlineProvider{} })
	Providers.Register(func() Provider { return &FileProvider{} })
	Providers.Register(func() Provider { return &mockProvider{} })

	t.Cleanup(func() {
		Providers = originalProviders
	})

	reg := prometheus.NewRegistry()
	m, err := NewManager(reg, cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	m.Start(ctx)
	t.Cleanup(cancel)
	t.Cleanup(m.Stop)

	return m, reg
}

func TestNewManager(t *testing.T) {
	provider1 := newMockProvider("secret1")
	provider2 := newMockProvider("secret2")

	cfg := &testConfig{
		APIKeys: []SecretField{
			{provider: provider1},
			{provider: provider2},
			{provider: &InlineProvider{secret: "inline_secret"}},
		},
	}

	reg := prometheus.NewRegistry()
	m, err := NewManager(reg, cfg)
	require.NoError(t, err)

	require.Lenf(t, m.secrets, 3, "Manager should discover 3 secrets")
	assert.NotNil(t, m.secrets[&cfg.APIKeys[0]])
	assert.NotNil(t, m.secrets[&cfg.APIKeys[1]])
	assert.NotNil(t, m.secrets[&cfg.APIKeys[2]])
}

func TestManager_SecretLifecycle(t *testing.T) {
	provider := newMockProvider("initial_secret")
	cfg := &testConfig{
		APIKeys: []SecretField{
			{
				provider: provider,
				settings: SecretFieldSettings{RefreshInterval: 50 * time.Millisecond},
			},
		},
	}

	m, _ := setupManagerTest(t, cfg)

	// 1. Initial fetch
	require.Eventuallyf(t, provider.hasFetchedLatest, time.Second, 10*time.Millisecond, "Initial fetch should occur")
	assert.Equal(t, "initial_secret", cfg.APIKeys[0].Get())

	ready, err := m.SecretsReady(cfg)
	require.NoError(t, err)
	assert.Truef(t, ready, "Secrets should be ready after initial fetch")

	// 2. Scheduled refresh
	provider.setSecret("refreshed_secret")
	require.Eventuallyf(t, provider.hasFetchedLatest, time.Second, 10*time.Millisecond, "Scheduled refresh should occur")
	assert.Equal(t, "refreshed_secret", cfg.APIKeys[0].Get())

	// 3. Triggered refresh
	provider.setSecret("triggered_secret")
	cfg.APIKeys[0].TriggerRefresh()
	require.Eventuallyf(t, provider.hasFetchedLatest, time.Second, 10*time.Millisecond, "Triggered refresh should occur")
	assert.Equal(t, "triggered_secret", cfg.APIKeys[0].Get())
}

func TestManager_FetchErrorAndRecovery(t *testing.T) {
	provider := newMockProvider("")
	provider.setFetchError(errors.New("fetch failed"))
	cfg := &testConfig{
		APIKeys: []SecretField{
			{
				provider: provider,
			},
		},
	}
	m, _ := setupManagerTest(t, cfg)

	// Initial fetch fails.
	require.Eventuallyf(t, provider.hasFetchedLatest, time.Second, 10*time.Millisecond, "A fetch should be attempted")
	assert.Emptyf(t, cfg.APIKeys[0].Get(), "Secret should be empty after failed fetch")

	ready, err := m.SecretsReady(cfg)
	require.NoError(t, err)
	assert.Falsef(t, ready, "Secrets should not be ready after failed fetch")

	// Recovery.
	provider.setFetchError(nil)
	provider.setSecret("recovered_secret")
	require.Eventuallyf(t, func() bool { return cfg.APIKeys[0].Get() == "recovered_secret" }, 2*time.Second, 50*time.Millisecond, "Manager should recover after error")

	ready, err = m.SecretsReady(cfg)
	require.NoError(t, err)
	assert.Truef(t, ready, "Secrets should be ready after recovery")
}

func TestManager_Validation(t *testing.T) {
	provider := newMockProvider("initial_valid")
	cfg := &testConfig{
		APIKeys: []SecretField{
			{
				provider: provider,
				settings: SecretFieldSettings{RefreshInterval: 10 * time.Millisecond},
			},
		},
	}
	m, _ := setupManagerTest(t, cfg)
	validator := newMockValidator()
	validator.setValid("initial_valid", true)
	validator.setValid("finally_valid", true)
	// Make validation super fast for the test.
	validator.settings.InitialBackoff = 5 * time.Millisecond

	cfg.APIKeys[0].SetSecretValidation(validator)

	// 1. Initial fetch with successful validation.
	require.Eventuallyf(t, provider.hasFetchedLatest, time.Second, 10*time.Millisecond, "A fetch should be attempted")
	assert.Equal(t, "initial_valid", cfg.APIKeys[0].Get())
	require.Eventuallyf(t, func() bool {
		m.secrets[&cfg.APIKeys[0]].mtx.RLock()
		defer m.secrets[&cfg.APIKeys[0]].mtx.RUnlock()
		return m.secrets[&cfg.APIKeys[0]].verified
	},
		time.Second, 10*time.Millisecond, "should be eventually valid")

	// 2. Refresh with an invalid secret.
	provider.setSecret("next_invalid")
	require.Eventuallyf(t, provider.hasFetchedLatest, time.Second, 10*time.Millisecond, "A refresh should be attempted")

	// Wait a bit to ensure validation is attempted and fails.
	time.Sleep(100 * time.Millisecond)
	assert.Equalf(t, "initial_valid", cfg.APIKeys[0].Get(), "Old secret should be kept after validation failure")

	// 3. Refresh again with a now-valid secret.
	provider.setSecret("finally_valid")
	require.Eventuallyf(t, provider.hasFetchedLatest, time.Second, 10*time.Millisecond, "Another refresh should be attempted")
	require.Eventuallyf(t, func() bool { return cfg.APIKeys[0].Get() == "finally_valid" }, time.Second, 10*time.Millisecond, "New secret should be adopted after validation succeeds")
}

func TestManager_InlineSecret(t *testing.T) {
	inlineSecret := "this-is-inline"
	cfg := &testConfig{
		APIKeys: []SecretField{
			{provider: &InlineProvider{secret: inlineSecret}},
		},
	}
	setupManagerTest(t, cfg)

	assert.Equal(t, inlineSecret, cfg.APIKeys[0].Get())
}
