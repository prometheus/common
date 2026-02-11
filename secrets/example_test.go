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

func Example() {
	// A Prometheus registry is needed to register the secret manager's metrics.
	promRegisterer := prometheus.NewRegistry()

	// Create a temporary file to simulate a file-based secret (e.g., Kubernetes mount).
	passwordFile, err := os.CreateTemp("", "password_secret")
	if err != nil {
		panic(err)
	}
	defer os.Remove(passwordFile.Name())

	if _, err := passwordFile.WriteString("my_super_secret_password"); err != nil {
		passwordFile.Close()
		panic(err)
	}
	passwordFile.Close()

	// In your configuration struct, use the `secrets.Field` type for any fields
	// that should contain secrets.
	type MyConfig struct {
		APIKey   secrets.Field `yaml:"api_key"`
		Password secrets.Field `yaml:"password"`
	}

	// Users can then provide secrets in their YAML configuration file.
	// We inject the temporary file path created above.
	configData := []byte(fmt.Sprintf(`
api_key: "my_super_secret_api_key"
password:
  file:
    path: %s
`, passwordFile.Name()))

	var cfg MyConfig
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		panic(fmt.Errorf("error unmarshaling config: %w", err))
	}

	// Create a secret manager. This discovers and manages all Fields in cfg.
	// The manager will handle refreshing secrets in the background.
	manager, err := secrets.NewManager(promslog.NewNopLogger(), promRegisterer, secrets.Providers, &cfg)
	if err != nil {
		panic(fmt.Errorf("error creating secret manager: %w", err))
	}

	// Start the manager's background refresh loop.
	go manager.Run(context.Background())

	// Access the secret values.
	apiKey := cfg.APIKey.Value()
	password := cfg.Password.Value()

	fmt.Printf("API Key: %s\n", apiKey)
	fmt.Printf("Password: %s\n", password)

	// Output:
	// API Key: my_super_secret_api_key
	// Password: my_super_secret_password
}
