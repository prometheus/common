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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type PreparerConfig struct {
	SecretField     Field  `yaml:"secret_field"`
	SecretFieldFile string `yaml:"secret_field_file"`
}

func (p *PreparerConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type Alias PreparerConfig
	aux := (*Alias)(p)

	if err := unmarshal(aux); err != nil {
		return err
	}
	return aux.SecretField.OrFileProvider(aux.SecretFieldFile, "secret_field", "secret_field_file")
}

func TestManager_PrepareSecrets(t *testing.T) {
	secretFile := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(secretFile, []byte("file_secret"), 0o600))

	contents := fmt.Sprintf(`
secret_field_file: %q
`, secretFile)

	_, cfg, _ := SetupManagerForTest[PreparerConfig](t, contents, &MockProvider{})

	assert.Equalf(t, "file_secret", cfg.SecretField.Value(), "expected to get secret field from file.")
}
