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
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.yaml.in/yaml/v2"
)

// PolicyFunc returns true if secrets should be printed (insecure),
// and false if they should be scrubbed (secure).
type PolicyFunc func() bool

var (
	currentPolicy PolicyFunc = func() bool { return true }

	// policyInitialized tracks if the policy has been explicitly set.
	policyInitialized atomic.Bool
)

// SetVisibilityPolicy sets the global function that determines if secrets
// should be printed or scrubbed.
//
// This is designed to be called exactly ONCE, typically by the
// prometheus/common/config package during initialization.
func SetVisibilityPolicy(p PolicyFunc) {
	if !policyInitialized.CompareAndSwap(false, true) {
		panic("prometheus/common/secrets: duplicate initialization of secret visibility policy.\n" +
			"\tReason: The secret scrubbing policy is a global singleton that must be consistent across the application.\n" +
			"\tExpected Owner: Only 'prometheus/common/config' should initialize this policy.\n",
		)
	}

	currentPolicy = p
}

// Field is a field containing a secret.
//
// In a configuration struct, use secrets.Field for any fields that should
// contain secrets.
//
// secrets.Field handles YAML unmarshaling and can be either a plain string
// (for inline secrets) or a structure for a specific secret provider.
//
// For example:
//
//	type MyConfig struct {
//	    APIKey    secrets.Field `yaml:"api_key"`
//	    Password  secrets.Field `yaml:"password"`
//	}
//
// In the YAML file, the secrets can be configured as follows:
//
//	api_key: "my_super_secret_api_key"
//	password:
//	  file:
//	    path: /path/to/password.txt
type Field struct {
	rawConfig any
	state     *fieldState
}

type fieldState struct {
	mutex          sync.Mutex
	path           string
	settings       FieldSettings
	providerName   string
	config         ProviderConfig
	requestRefresh bool
	value          string
}

type FieldSettings struct {
	RefreshInterval time.Duration `yaml:"refreshInterval,omitempty"`
}

func (fs *fieldState) id() string {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	providerID := fs.path
	if provider, ok := fs.config.(ProviderConfigID); ok {
		providerID = provider.ID()
	}
	return fmt.Sprintf("%s>%s", fs.providerName, providerID)
}

func (s Field) String() string {
	if s.state == nil {
		return "Field{UNPARSED}"
	}
	return fmt.Sprintf("Field[%s]{Provider: %s}", s.state.path, s.state.providerName)
}

// MarshalYAML implements the yaml.Marshaler interface for Field.
func (s Field) MarshalYAML() (interface{}, error) {
	printRaw := currentPolicy()
	if !printRaw {
		return "<secret>", nil
	}
	return s.rawConfig, nil
}

// MarshalJSON implements the json.Marshaler interface for Field.
func (s Field) MarshalJSON() ([]byte, error) {
	data, err := s.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return json.Marshal(data)
}

type mapType = map[string]any

// splitProviderAndSettings separates provider-specific configuration from the generic SecretField settings.
func splitProviderAndSettings(provReg *ProviderRegistry, baseMap mapType) (providerName string, providerData interface{}, settingsMap mapType, err error) {
	settingsMap = make(mapType)

	for k, v := range baseMap {
		// Check if the key corresponds to a registered provider.
		if _, err := provReg.Get(k); err == nil {
			if providerName != "" {
				// A provider has already been found, which is an error.
				return "", nil, nil, fmt.Errorf("secret must contain exactly one provider type, but multiple were found: %s, %s", providerName, k)
			}
			providerName = k
			providerData = v
		} else {
			// If it's not a provider key, treat it as a setting.
			settingsMap[k] = v
		}
	}

	if providerName == "" {
		// Marshal the map back to YAML for a readable error message.
		yamlBytes, err := yaml.Marshal(baseMap)
		if err != nil {
			// Fallback to the original format if marshalling fails for some reason.
			return "", nil, nil, fmt.Errorf("no valid secret provider found in configuration: %v", baseMap)
		}
		return "", nil, nil, fmt.Errorf("no valid secret provider found in configuration:\n%s", string(yamlBytes))
	}

	return providerName, providerData, settingsMap, nil
}

// convertConfig takes a yaml-parsed any and unmarshals it into a typed struct.
// It achieves this by first marshalling the input to YAML and then unmarshalling
// it into the target struct.
func convertConfig[T any](source any, target T) error {
	bytes, err := yaml.Marshal(source)
	if err != nil {
		return fmt.Errorf("failed to re-marshal config: %w", err)
	}
	if err := yaml.Unmarshal(bytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for Field.
func (s *Field) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshal(&s.rawConfig)
}

func (s *Field) parseRawConfig(reg *ProviderRegistry, path string) (*fieldState, error) {
	var plainSecret string
	if err := convertConfig(s.rawConfig, &plainSecret); err == nil {
		return &fieldState{
			path:         path,
			providerName: "inline",
			config: &InlineProviderConfig{
				secret: plainSecret,
			},
			value: plainSecret,
		}, nil
	}

	var baseMap mapType
	if err := convertConfig(s.rawConfig, &baseMap); err != nil {
		return nil, err
	}

	providerName, providerConfigData, settingsMap, err := splitProviderAndSettings(reg, baseMap)
	if err != nil {
		return nil, err
	}

	providerConfig, err := reg.Get(providerName)
	if err != nil {
		return nil, err
	}

	if err := convertConfig(providerConfigData, providerConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal into %s provider: %w", providerName, err)
	}
	var settings FieldSettings
	if err := convertConfig(settingsMap, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal secret field settings: %w", err)
	}

	return &fieldState{
		path:         path,
		providerName: providerName,
		config:       providerConfig,
		settings:     settings,
	}, nil
}

// Get returns the secret value.
//
// This method will panic if the Field has not been discovered by a Manager.
// To avoid this, ensure that NewManager is called with a pointer to the
// configuration struct containing the Field.
func (s *Field) Get() string {
	if s.state == nil {
		panic("secret field has not been discovered by a manager; was NewManager(&cfg) called?")
	}
	s.state.mutex.Lock()
	defer s.state.mutex.Unlock()
	return s.state.value
}

// TriggerRefresh signals the Manager to refresh the secret.
//
// This method will panic if the Field has not been discovered by a Manager.
// To avoid this, ensure that NewManager is called with a pointer to the
// configuration struct containing the Field.
func (s *Field) TriggerRefresh() {
	if s.state == nil {
		panic("secret field has not been discovered by a manager; was NewManager(&cfg) called?")
	}
	s.state.mutex.Lock()
	defer s.state.mutex.Unlock()
	s.state.requestRefresh = true
}
