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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v2"
)

func TestFileProvider(t *testing.T) {
	ctx := context.Background()
	secretContent := "my-super-secret-password"
	tempDir := t.TempDir()
	secretFile := filepath.Join(tempDir, "secret.txt")

	err := os.WriteFile(secretFile, []byte(secretContent), 0o600)
	require.NoError(t, err)

	fp := &FileProvider{Path: secretFile}

	t.Run("FetchSecret_Success", func(t *testing.T) {
		content, err := fp.FetchSecret(ctx)
		require.NoError(t, err)
		assert.Equal(t, secretContent, content)
	})

	t.Run("FetchSecret_NotFound", func(t *testing.T) {
		badFP := &FileProvider{Path: filepath.Join(tempDir, "non-existant.txt")}
		_, err := badFP.FetchSecret(ctx)
		require.Error(t, err)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, "file", fp.Name())
	})

	t.Run("Key", func(t *testing.T) {
		assert.Equal(t, secretFile, fp.Key())
	})

	t.Run("MarshalYAML", func(t *testing.T) {
		data, err := fp.MarshalYAML()
		require.NoError(t, err)
		expected := map[string]interface{}{"path": secretFile}
		assert.Equal(t, expected, data)
	})
}

func TestInlineProvider(t *testing.T) {
	ctx := context.Background()
	secretContent := "my-inline-secret"
	ip := &InlineProvider{secret: secretContent}

	t.Run("FetchSecret", func(t *testing.T) {
		content, err := ip.FetchSecret(ctx)
		require.NoError(t, err)
		assert.Equal(t, secretContent, content)
	})

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, "inline", ip.Name())
	})

	t.Run("Key", func(t *testing.T) {
		assert.Equal(t, secretContent, ip.Key())
	})

	t.Run("MarshalYAML", func(t *testing.T) {
		data, err := ip.MarshalYAML()
		require.NoError(t, err)
		assert.Equal(t, "<secret>", data)

		// Also check the output of a full yaml marshal
		out, err := yaml.Marshal(ip)
		require.NoError(t, err)
		assert.Equal(t, "<secret>\n", string(out))
	})
}
