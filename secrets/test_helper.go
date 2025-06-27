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
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert/yaml"
	"github.com/stretchr/testify/require"

	"github.com/prometheus/common/promslog"
)

func MockInline(inline string) Field {
	if !testing.Testing() {
		panic("MockInline can only be used for testing")
	}
	return Field{
		rawConfig: inline,
		state: &fieldState{
			path:         "mocked_path",
			providerName: InlineProviderName,
			config: &InlineProviderConfig{
				secret: inline,
			},
			value: inline,
		},
	}
}

func MockField(mock *MockProvider) Field {
	if !testing.Testing() {
		panic("MockField can only be used for testing")
	}
	return Field{
		rawConfig: map[string]any{
			"mock": mock,
		},
		state: &fieldState{
			path:         "mocked_path",
			providerName: InlineProviderName,
			config:       mock,
			value:        mock.Secret,
		},
	}
}

func ParseConfig[T any](t *testing.T, content string) *T {
	if !testing.Testing() {
		panic("ParseConfig can only be used for testing")
	}
	var cfg T
	require.NoError(t, yaml.Unmarshal([]byte(content), &cfg))
	return &cfg
}

func SetupManagerForTest[T any](t *testing.T, content string, mockPrototype *MockProvider) (*Manager, *T, func() *T) {
	if !testing.Testing() {
		panic("SetupManagerForTest can only be used for testing")
	}
	reg := prometheus.NewRegistry()
	t.Cleanup(func() {
		currentPolicy = func() bool { return true }
		policyInitialized = atomic.Bool{}
	})

	providerReg := &ProviderRegistry{}
	providerReg.Register("inline", &InlineProviderConfig{})
	providerReg.Register("file", &FileProviderConfig{})
	providerReg.Register("nil", &NilProviderConfig{})
	providerReg.Register("mock", mockPrototype)

	cfg := ParseConfig[T](t, content)

	m, err := NewManager(promslog.NewNopLogger(), reg, providerReg, cfg)
	require.NoError(t, err)

	go m.Run(t.Context())
	return m, cfg, func() *T {
		// Populate
		cfg := ParseConfig[T](t, content)
		require.NoError(t, m.PopulateConfig(cfg))
		return cfg
	}
}
