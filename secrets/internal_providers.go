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

type fileProvider struct {
	path string
}

func (fp *fileProvider) FetchSecret(_ context.Context) (string, error) {
	content, err := os.ReadFile(fp.path)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// FileProviderConfig is the configuration for the `file` provider.
//
// The `file` provider reads the secret from a file.
// To use the `file` provider, configure it in your YAML file as follows:
//
//	password:
//	  file:
//	    path: /path/to/password.txt
type FileProviderConfig struct {
	Path string `yaml:"path" json:"path"`
}

func (fpc *FileProviderConfig) NewProvider() (Provider, error) {
	return &fileProvider{path: fpc.Path}, nil
}

func (fpc *FileProviderConfig) Clone() ProviderConfig {
	return &FileProviderConfig{Path: fpc.Path}
}

func (fpc *FileProviderConfig) ID() string {
	return fpc.Path
}

type inlineProvider struct {
	secret string
}

func (ip *inlineProvider) FetchSecret(_ context.Context) (string, error) {
	return ip.secret, nil
}

// InlineProviderConfig is the configuration for the `inline` provider.
//
// The `inline` provider uses a secret that is specified directly in the
// configuration file. This is the default provider if a plain string is
// provided for a secret field.
//
// To use the `inline` provider, configure it in your YAML file as follows:
//
//	api_key: "my_super_secret_api_key"
type InlineProviderConfig struct {
	secret string
}

func (ipc *InlineProviderConfig) NewProvider() (Provider, error) {
	return &inlineProvider{secret: ipc.secret}, nil
}

func (ipc *InlineProviderConfig) Clone() ProviderConfig {
	return &InlineProviderConfig{secret: ipc.secret}
}

func init() {
	Providers.Register("inline", &InlineProviderConfig{})
	Providers.Register("file", &FileProviderConfig{})
}
