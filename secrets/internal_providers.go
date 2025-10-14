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
)

// FileProvider fetches secrets from a file.
type FileProvider struct {
	Path string `yaml:"path" json:"path"`
}

func (fp *FileProvider) FetchSecret(_ context.Context) (string, error) {
	content, err := os.ReadFile(fp.Path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

func (*FileProvider) Name() string {
	return "file"
}

func (fp *FileProvider) Key() string {
	return fp.Path
}

func (fp *FileProvider) MarshalYAML() (interface{}, error) {
	return map[string]interface{}{
		"path": fp.Path,
	}, nil
}

// InlineProvider reads an config secret.
type InlineProvider struct {
	secret string
}

func (ip *InlineProvider) FetchSecret(_ context.Context) (string, error) {
	return ip.secret, nil
}

func (*InlineProvider) Name() string {
	return "inline"
}

func (ip *InlineProvider) Key() string {
	return ip.secret
}

func (*InlineProvider) MarshalYAML() (interface{}, error) {
	return "<secret>", nil
}

func init() {
	Providers.Register(func() Provider { return &InlineProvider{} })
	Providers.Register(func() Provider { return &FileProvider{} })
}
