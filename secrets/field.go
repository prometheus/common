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
	"errors"
	"fmt"
	"reflect"
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
			"\tExpected Owner: Only 'prometheus/common/config' should initialize this policy.\n")
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
	// rawConfig is a blob of yaml that is parsed from one of two formats:
	// 1. A string literal
	// 2. A map[string]any, which is used for secret provider configs.
	//
	// It's stored in this raw format because we need to be able to marshal
	// it back to YAML for display purposes. The actual parsing and resolution
	// of the secret is handled by the Manager.
	rawConfig any
	state     *fieldState
}

type fieldState struct {
	path         string
	manager      *Manager
	settings     FieldSettings
	providerName string
	config       ProviderConfig
	fetched      time.Time
	value        string
}

// FieldSettings contains any settings that are generic to a secret field,
// and are not specific to a particular secret provider.
//
// Provider developers should be aware that these settings are parsed from
// the configuration map after the provider-specific configuration has been
// extracted. This means that any fields in the secret's YAML configuration
// that are *not* a registered provider name will be considered a FieldSetting.
// This is useful for adding generic, cross-provider functionality without
// requiring each provider to implement it. For example, a `default_value` can
// be provided, which will be used if the secret provider returns an error.
type FieldSettings struct {
	RefreshInterval time.Duration `yaml:"refresh_interval,omitempty"`
	DefaultValue    string        `yaml:"default_value,omitempty"`
}

func (fs *fieldState) getDedupID() string {
	var id string
	if provider, ok := fs.config.(ProviderConfigID); ok {
		id = provider.ID()
	} else {
		id = fmt.Sprintf("%p", fs)
	}
	return fmt.Sprintf("%s>%s", fs.providerName, id)
}

func (s Field) Equal(other Field) bool {
	return reflect.DeepEqual(s.rawConfig, other.rawConfig)
}

func (s Field) IsZero() bool {
	return s.rawConfig == nil
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
	if printRaw {
		return s.rawConfig, nil
	}
	if s.rawConfig == nil {
		return nil, nil
	}
	return "<secret>", nil
}

// MarshalJSON implements the json.Marshaler interface for Field.
func (s Field) MarshalJSON() ([]byte, error) {
	data, err := s.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return json.Marshal(data)
}

// parseSecretField separates provider-specific configuration from the generic SecretField settings
// and parses the provider.
func parseSecretField(provReg *ProviderRegistry, baseMap map[string]any) (name string, cfg ProviderConfig, settings FieldSettings, err error) {
	var settingsRaw any

	for k, v := range baseMap {
		// Check if the key corresponds to a registered provider.
		if targetCfg, err := provReg.Get(k); err == nil {
			if name != "" {
				// A provider has already been found, which is an error.
				return "", nil, settings, fmt.Errorf("secret must contain exactly one provider type, but multiple were found: %s, %s", name, k)
			}
			name = k

			if str, isStr := v.(string); isStr {
				strCfg, isFromStr := targetCfg.(ProviderConfigFromString)
				if !isFromStr {
					return "", nil, settings, fmt.Errorf("secret provider %q does not support instantiation from string", name)
				}
				strCfg.FromString(str)
			} else if err := unmarshalTo(v, targetCfg); err != nil {
				return "", nil, settings, fmt.Errorf("failed to unmarshal into %s provider: %w", name, err)
			}
			cfg = targetCfg
		} else if k == OptionsKey {
			// Registry reserves OptionsKey for us. We will detect missing provider later.
			settingsRaw = v
		} else {
			return "", nil, settings, fmt.Errorf("unknown provider: %s", k)
		}
	}

	if name == "" {
		// Marshal the map back to YAML for a readable error message.
		yamlBytes, err := yaml.Marshal(baseMap)
		if err != nil {
			// Fallback to the original format if marshalling fails for some reason.
			return "", nil, settings, fmt.Errorf("no valid secret provider found in configuration: %v", baseMap)
		}
		return "", nil, settings, fmt.Errorf("no valid secret provider found in configuration:\n%s", string(yamlBytes))
	}

	if err := unmarshalTo(settingsRaw, &settings); err != nil {
		return "", nil, settings, fmt.Errorf("failed to unmarshal secret field settings: %w", err)
	}

	return name, cfg, settings, nil
}

// unmarshalTo takes a yaml-parsed any and unmarshals it into a typed struct.
// It achieves this by first marshalling the input to YAML and then unmarshalling
// it into the target struct.
func unmarshalTo(source, target any) error {
	bytes, err := yaml.Marshal(source)
	if err != nil {
		return fmt.Errorf("failed to re-marshal config: %w", err)
	}
	if err := yaml.UnmarshalStrict(bytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for Field.
//
// It attempts to unmarshal the YAML into a string first, and if that fails,
// it tries to unmarshal it into a map[string]any. This is because secrets
// can be represented as either a raw string or a configuration block for a
// secret provider. The raw, unparsed data is stored in s.rawConfig.
func (s *Field) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var inline string
	if err := unmarshal(&inline); err == nil {
		s.rawConfig = inline
		return nil
	}
	var provider map[string]any
	err := unmarshal(&provider)
	s.rawConfig = provider
	return err
}

func (s *Field) parseRawConfig(reg *ProviderRegistry, path string) (*fieldState, error) {
	// Based on the type of rawConfig, we can determine how to parse the
	// secret. If it's a string, it's an inline secret. If it's a map,
	// we need to parse it to determine the provider and its configuration.
	if s.rawConfig == nil {
		return &fieldState{
			path:         path,
			providerName: NilProviderName,
			config:       &NilProviderConfig{},
			value:        "",
		}, nil
	}

	var baseMap map[string]any

	switch rawConfig := s.rawConfig.(type) {
	case string:
		return &fieldState{
			path:         path,
			providerName: InlineProviderName,
			config:       &InlineProviderConfig{secret: rawConfig},
			value:        rawConfig,
		}, nil

	case map[string]any:
		baseMap = rawConfig

	default:
		return nil, fmt.Errorf("secret field must be a string or a map[string]any, got %v", s.rawConfig)
	}

	providerName, providerConfig, settings, err := parseSecretField(reg, baseMap)
	if err != nil {
		return nil, err
	}

	return &fieldState{
		path:         path,
		providerName: providerName,
		config:       providerConfig,
		settings:     settings,
		value:        settings.DefaultValue,
	}, nil
}

func (s *Field) IsNil() bool {
	return s.rawConfig == nil
}

func (s *Field) Set(providerName string, config ProviderConfig) error {
	if s.state != nil {
		return errors.New("secrets.Field.Set() can only be called before secrets have been populated")
	}
	s.rawConfig = map[string]any{
		providerName: config,
	}

	return nil
}

func (s *Field) OrFileProvider(path, fieldName, fileFieldName string) error {
	c := count(s.IsNil(), path == "")
	if c != 1 {
		return fmt.Errorf("exactly one of %s and %s can be specified, got %d", fieldName, fileFieldName, c)
	}

	return s.Set("file", &FileProviderConfig{
		Path: path,
	})
}

// Value returns the secret value.
//
// This method will panic if the Field has not been populated by a Manager.
// To avoid this, ensure that NewManager is called with a pointer to the
// configuration struct containing the Field.
func (s *Field) Value() string {
	if s.state == nil {
		panic("secret field has not been populated by a manager; was NewManager(&cfg) called?")
	}
	return s.state.value
}

func (s *Field) WasFetched() bool {
	if s.state == nil {
		panic("secret field has not been populated by a manager; was NewManager(&cfg) called?")
	}
	return !s.state.fetched.IsZero()
}

// TriggerRefresh signals the Manager to refresh the secret.
//
// This method will panic if the Field has not been populated by a Manager.
// To avoid this, ensure that NewManager is called with a pointer to the
// configuration struct containing the Field.
func (s *Field) TriggerRefresh() {
	if s.state == nil {
		panic("secret field has not been populated by a manager; was NewManager(&cfg) called?")
	}
	s.state.manager.triggerRefresh(s)
}
