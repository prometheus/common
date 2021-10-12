// Copyright 2016 The Prometheus Authors
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

// This package no longer handles safe yaml parsing. In order to
// ensure correct yaml unmarshalling, use "yaml.UnmarshalStrict()".

package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"
)

const secretToken = "<secret>"

type SecretLoader struct {
	secret     string
	secretFile string
}

// SetFile sets the filename from which the SecretLoader will load
// the secret.
func (s *SecretLoader) SetFile(fn string) {
	s.secretFile = fn
}

// IsSet reports whether one of the inline secret or the secret filename
// is set. In the later case, it _does not_ validate that the file
// exists or that it can be read.
func (s *SecretLoader) IsSet() bool {
	return len(s.secret) > 0 || len(s.secretFile) > 0
}

// String returns the stored secret, without reloading it from the file
// if one has been specified. If you need to make sure that the file is
// (re-)read, use Get instead.
func (s *SecretLoader) String() string {
	return s.secret
}

// Get returns the secret, loading it from the file if necesary. It
// reports whether the secret changed from the last time this method was
// called.
func (s *SecretLoader) Get() (string, bool, error) {
	changed := false

	if len(s.secretFile) > 0 {
		newSecret, err := ioutil.ReadFile(s.secretFile)
		if err != nil {
			return "", false, err
		}

		if newStr := strings.TrimSpace(string(newSecret)); newStr != s.secret {
			s.secret = newStr
			changed = true
		}
	}

	return s.secret, changed, nil
}

func (s *SecretLoader) MarshalYAML() (interface{}, error) {
	if s.secret != "" {
		return secretToken, nil
	}

	return nil, nil
}

func (s *SecretLoader) UnmarshalYAML(unmarshal func(interface{}) error) error {
	return unmarshal(&s.secret)
}

func (s *SecretLoader) MarshalJSON() ([]byte, error) {
	if len(s.secret) == 0 {
		return json.Marshal("")
	}

	return json.Marshal(secretToken)
}

func validateSecret(secretField string, secret SecretLoader, secretFilenameField string, secretFilename string) error {
	if len(secretFilename) != 0 && len(secret.secretFile) == 0 && len(secret.secret) != 0 {
		return fmt.Errorf("at most one of %s & %s must be configured", secretField, secretFilenameField)
	}

	return nil
}

// Secret special type for storing secrets.
type Secret string

// MarshalYAML implements the yaml.Marshaler interface for Secrets.
func (s Secret) MarshalYAML() (interface{}, error) {
	if s != "" {
		return secretToken, nil
	}
	return nil, nil
}

//UnmarshalYAML implements the yaml.Unmarshaler interface for Secrets.
func (s *Secret) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain Secret
	return unmarshal((*plain)(s))
}

// MarshalJSON implements the json.Marshaler interface for Secret.
func (s Secret) MarshalJSON() ([]byte, error) {
	if len(s) == 0 {
		return json.Marshal("")
	}
	return json.Marshal(secretToken)
}

// DirectorySetter is a config type that contains file paths that may
// be relative to the file containing the config.
type DirectorySetter interface {
	// SetDirectory joins any relative file paths with dir.
	// Any paths that are empty or absolute remain unchanged.
	SetDirectory(dir string)
}

// JoinDir joins dir and path if path is relative.
// If path is empty or absolute, it is returned unchanged.
func JoinDir(dir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}
