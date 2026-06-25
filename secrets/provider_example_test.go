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

package secrets_test

import (
	"context"
	"fmt"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"go.yaml.in/yaml/v2"

	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/secrets"
)

// The following example demonstrates how to create and register a custom environment
// variable secret provider.

// 1. Define a configuration struct.
type EnvProviderConfig struct {
	Var string `yaml:"var"`
}

// 1.1. Implement the secrets.ProviderConfig interface.
func (c *EnvProviderConfig) NewProvider() (secrets.Provider, error) {
	return &envProvider{varName: c.Var}, nil
}

func (c *EnvProviderConfig) Clone() secrets.ProviderConfig {
	return &EnvProviderConfig{Var: c.Var}
}

// 3. Define the provider.
type envProvider struct {
	varName string
}

// 3.1 Implement the provider interface.
func (p *envProvider) FetchSecret(_ context.Context) (string, error) {
	return os.Getenv(p.varName), nil
}

type MyConfig struct {
	DBPassword secrets.Field `yaml:"db_password"`
}

func ExampleProvider() {
	// 4. Register the custom provider.
	// This should be done in an init in a real application.
	secrets.Providers.Register("env", &EnvProviderConfig{})

	// Set the environment variable with the secret.
	if err := os.Setenv("MY_SECRET_VAR", "secret_from_env"); err != nil {
		panic(fmt.Errorf("Error setting environment variable: %w", err))
	}
	defer os.Unsetenv("MY_SECRET_VAR")

	// User can then use your secret provider!
	configData := []byte(`
db_password:
  env:
    var: MY_SECRET_VAR
`)

	var cfg MyConfig
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		panic(fmt.Errorf("Error unmarshaling config: %w", err))
	}

	// Create a secret manager to manage the secret.
	manager, err := secrets.NewManager(promslog.NewNopLogger(), prometheus.NewRegistry(), secrets.Providers, &cfg)
	if err != nil {
		panic(fmt.Errorf("Error creating secret manager: %w", err))
	}
	go manager.Run(context.Background())

	// Access the secret.
	fmt.Printf("DB Password: %s\n", cfg.DBPassword.Value())

	// Output:
	// DB Password: secret_from_env
}
