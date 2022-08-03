// Copyright 2021 The Prometheus Authors
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

package sigv4

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func loadSigv4Config(filename string) (*SigV4Config, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	cfg := SigV4Config{}
	if err = yaml.UnmarshalStrict(content, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func testGoodConfig(t *testing.T, filename string) {
	_, err := loadSigv4Config(filename)
	if err != nil {
		t.Fatalf("Unexpected error parsing %s: %s", filename, err)
	}
}

func TestGoodSigV4Configs(t *testing.T) {
	filesToTest := []string{"testdata/sigv4_good.yaml", "testdata/sigv4_good.yaml"}
	for _, filename := range filesToTest {
		testGoodConfig(t, filename)
	}
}

func TestBadSigV4Config(t *testing.T) {
	filename := "testdata/sigv4_bad.yaml"
	_, err := loadSigv4Config(filename)
	if err == nil {
		t.Fatalf("Did not receive expected error unmarshaling bad sigv4 config")
	}
	if !strings.Contains(err.Error(), "must provide a AWS SigV4 Access key and Secret Key") {
		t.Errorf("Received unexpected error from unmarshal of %s: %s", filename, err.Error())
	}
}
