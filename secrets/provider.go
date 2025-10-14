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
	"time"
)

type Provider interface {
	// FetchSecret retrieves the secret value.
	FetchSecret(ctx context.Context) (string, error)

	// Name returns the provider's name (e.g., "inline").
	Name() string
}

// SecretValidator allows for validating a new secret before it is
// rotated into active use. If invalid, the old secret will be used
// while it is still considered valid and has not expired.
// This interface is optional, to prevent monitoring gaps if the
// system being scraped hasn't had its secret refreshed yet.
type SecretValidator interface {
	Validate(ctx context.Context, secret string) bool
	Settings() ValidationSettings
}

// ValidationSettings holds configurable parameters for secret validation.
type ValidationSettings struct {
	// Timeout governs the maximum time a single validation attempt can take.
	Timeout time.Duration
	// InitialBackoff is the initial backoff duration for re-validating a secret after a failure.
	InitialBackoff time.Duration
	// MaxBackoff is the maximum backoff duration for retrying a failed validation.
	MaxBackoff time.Duration
	// MaxRetries is the maximum number of retries for a failed validation.
	MaxRetries int
}

type DefaultValidator struct{}

func (DefaultValidator) Validate(_ context.Context, _ string) bool {
	return true
}

func (DefaultValidator) Settings() ValidationSettings {
	return DefaultValidationSettings()
}

// DefaultValidationSettings returns a ValidationSettings struct with default values.
func DefaultValidationSettings() ValidationSettings {
	return ValidationSettings{
		Timeout:        30 * time.Second,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		MaxRetries:     10,
	}
}
