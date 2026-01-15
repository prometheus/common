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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testProvider struct{}

func (*testProvider) FetchSecret(_ context.Context) (string, error) {
	return "test_secret", nil
}

type testProviderConfig struct{}

func (*testProviderConfig) NewProvider() (Provider, error) {
	return &testProvider{}, nil
}

func (*testProviderConfig) Clone() ProviderConfig {
	return &testProviderConfig{}
}

func TestProviderRegistry(t *testing.T) {
	t.Run("GetInitialProviders", func(t *testing.T) {
		// Test that providers from init() are registered in the global registry.
		p, err := Providers.Get("inline")
		require.NoError(t, err)
		assert.IsType(t, &InlineProviderConfig{}, p)

		p, err = Providers.Get("file")
		require.NoError(t, err)
		assert.IsType(t, &FileProviderConfig{}, p)
	})

	t.Run("GetUnknownProvider", func(t *testing.T) {
		_, err := Providers.Get("unknown-provider")
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown provider type: "unknown-provider"`)
	})

	t.Run("RegisterAndGet", func(t *testing.T) {
		reg := &ProviderRegistry{}
		config := &testProviderConfig{}

		reg.Register("test", config)
		p, err := reg.Get("test")
		require.NoError(t, err)
		assert.IsType(t, &testProviderConfig{}, p)
	})

	t.Run("RegisterDuplicate", func(t *testing.T) {
		reg := &ProviderRegistry{}
		config1 := &testProviderConfig{}
		config2 := &testProviderConfig{}

		reg.Register("duplicate", config1)
		assert.PanicsWithValue(t, `attempt to register duplicate type: "duplicate"`, func() {
			reg.Register("duplicate", config2)
		})
	})
}
