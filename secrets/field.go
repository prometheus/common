package secrets

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v2"
)

// SecretField is a field containing a secret.
type SecretField struct {
	provider Provider
	manager  *Manager
	// TODO: Add global secret options here
}

func (s SecretField) String() string {
	return fmt.Sprintf("SecretField{Provider: %s}", s.provider.Name())
}

// MarshalYAML implements the yaml.Marshaler interface for SecretField.
func (s SecretField) MarshalYAML() (interface{}, error) {
	if s.provider.Name() == "inline" && s.manager != nil && (*s.manager).MarshalInlineSecrets {
		return s.Get(), nil
	}
	out := make(map[string]interface{})
	out[s.provider.Name()] = s.provider
	return out, nil
}

// MarshalJSON implements the json.Marshaler interface for SecretField.
func (s SecretField) MarshalJSON() ([]byte, error) {
	data, err := s.MarshalYAML()
	if err != nil {
		return nil, err
	}
	return json.Marshal(data)
}

// providerBase is used to extract the type of the provider.
type providerBase = map[string]map[string]interface{}

func (s *SecretField) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var plainSecret string
	if err := unmarshal(&plainSecret); err == nil {
		s.provider = &InlineProvider{
			secret: plainSecret,
		}
	}

	var base providerBase
	if err := unmarshal(&base); err != nil {
		return err
	}

	if len(base) != 1 {
		return fmt.Errorf("secret must contain exactly one provider type, but found %d.", len(base))
	}

	var name string
	var providerConfig map[string]interface{}
	for providerType, data := range base {
		name = providerType
		providerConfig = data
		break
	}

	concreteProvider, err := Providers.Get(name)
	if err != nil {
		return err
	}

	configBytes, err := yaml.Marshal(providerConfig)
	if err != nil {
		return fmt.Errorf("failed to re-marshal config for %s provider: %w", name, err)
	}

	if err := yaml.Unmarshal(configBytes, &concreteProvider); err != nil {
		return fmt.Errorf("failed to unmarshal into %s provider: %w", name, err)
	}

	s.provider = concreteProvider
	return nil
}

// SetSecretValidation registers an optional validation function for the secret.
//
// When the secret manager fetches a new version of the secret, it will not
// be used immediately if there is a validator. Instead, the manager will
// hold the new secret in a pending state and call the provided Validate
// with it until it returns true, there is an explicit refresh request,
// there is a time out, or the old secret was never valid.
func (s *SecretField) SetSecretValidation(validator SecretValidator) {
	if s.manager == nil {
		panic("secret field has not been discovered by a manager; was NewManager(&cfg) called?")
	}
	(*s.manager).setSecretValidation(s, validator)
}

func (s *SecretField) Get() string {
	if s.manager == nil {
		panic("secret field has not been discovered by a manager; was NewManager(&cfg) called?")
	}
	return s.manager.get(s)
}

func (s *SecretField) TriggerRefresh() {
	if s.manager == nil {
		panic("secret field has not been discovered by a manager; was NewManager(&cfg) called?")
	}
	s.manager.triggerRefresh(s)
}
