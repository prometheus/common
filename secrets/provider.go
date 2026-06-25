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
)

// ProviderConfig is the configuration for a secret provider.
//
// To create a custom secret provider, you must implement this interface and
// register a new instance of your configuration struct with the
// ProviderRegistry.
//
// The NewProvider method should return a new Provider instance based on the
// configuration.
//
// The Clone method should return a deep copy of the configuration.
type ProviderConfig interface {
	// NewProvider creates a new provider from the config.
	NewProvider() (Provider, error)
	// Clone clones the config.
	Clone() ProviderConfig
}

// ProviderConfigFromString is a config you can set from one value.
type ProviderConfigFromString interface {
	ProviderConfig
	// Initialize the config from a string.
	FromString(string)
}

// ProviderConfigID is an interface for uniquely identifying a ProviderConfig
// instance.
//
// The ID method should return a string that is unique for each provider
// configuration. This is used by the Manager to identify and manage secrets.
type ProviderConfigID interface {
	ProviderConfig
	// ID returns a unique identifier string. Two provider configs should return the same ID if and only if they would fetch identical secret values.
	ID() string
}

// Provider is the interface for a secret provider.
//
// A Provider is responsible for fetching a secret value from a source.
type Provider interface {
	// FetchSecret retrieves the secret value.
	// The context can be used to pass cancellation signals to the provider.
	FetchSecret(ctx context.Context) (string, error)
}
