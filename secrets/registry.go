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

import "fmt"

const OptionsKey = "options"

// ProviderRegistry is a registry for secret provider configurations.
//
// It is used to register and retrieve secret provider configurations by name.
type ProviderRegistry struct {
	providerConfigs map[string]ProviderConfig
}

// Get retrieves a secret provider configuration by name.
// It returns a clone of the registered configuration.
func (r *ProviderRegistry) Get(name string) (ProviderConfig, error) {
	if config, ok := r.providerConfigs[name]; ok {
		return config.Clone(), nil
	}
	return nil, fmt.Errorf("unknown provider type: %q", name)
}

// Register registers a new secret provider configuration.
//
// This method should be called in an init() function to register a custom
// secret provider. The name must be unique, or the method will panic.
func (r *ProviderRegistry) Register(name string, config ProviderConfig) {
	if name == OptionsKey {
		panic(fmt.Sprintf("name %q is reserved for global options", name))
	}

	if _, ok := r.providerConfigs[name]; ok {
		panic(fmt.Sprintf("attempt to register duplicate type: %q", name))
	}
	if r.providerConfigs == nil {
		r.providerConfigs = make(map[string]ProviderConfig)
	}
	r.providerConfigs[name] = config
}

// Providers is the global provider registry.
// Custom secret providers should be registered with this registry.
var Providers = &ProviderRegistry{}
