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

//go:build go1.8
// +build go1.8

package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
)

func TestJSONMarshalSecret(t *testing.T) {
	type tmp struct {
		S Secret
	}
	for _, tc := range []struct {
		desc     string
		data     tmp
		expected string
	}{
		{
			desc: "inhabited",
			// u003c -> "<"
			// u003e -> ">"
			data:     tmp{"test"},
			expected: "{\"S\":\"\\u003csecret\\u003e\"}",
		},
		{
			desc:     "empty",
			data:     tmp{},
			expected: "{\"S\":\"\"}",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			c, err := json.Marshal(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if tc.expected != string(c) {
				t.Fatalf("Secret not marshaled correctly, got '%s'", string(c))
			}
		})
	}
}

func TestSecretLoadFromFile(t *testing.T) {
	dn, err := ioutil.TempDir("", "test-secret-loadfromfile.")
	if err != nil {
		t.Fatalf("cannot create temporary directory: %s", err)
	}
	defer os.RemoveAll(dn)

	fh, err := ioutil.TempFile(dn, "")
	if err != nil {
		t.Fatalf("cannot create temporary file: %s", err)
	}

	fn := fh.Name()

	secretData := "test"

	n, err := fh.WriteString(secretData)
	if err != nil {
		t.Fatalf("cannot write to temporary file %s: %s", fn, err)
	}

	if n != len(secretData) {
		t.Fatalf("short write writing to temporary file %s, expecting %d, got %d", fn, len(secretData), n)
	}

	err = fh.Close()
	if err != nil {
		t.Fatalf("error closing temporary file %s after write: %s", fn, err)
	}

	var s Secret
	err = s.LoadFromFile(fn)
	if err != nil {
		t.Fatalf("cannot read secret from temporary file %s: %s", fn, err)
	}

	if string(s) != secretData {
		t.Fatalf("unexpected secret data, expected %q, actual %q", secretData, string(s))
	}

	err = os.Remove(fn)
	if err != nil {
		t.Fatalf("cannot remove temporary file %s: %s", fn, err)
	}

	// this should report an error now
	err = s.LoadFromFile(fn)
	if err == nil {
		t.Fatalf("expecting error reading non-existent temporary file %s, got nil", fn)
	}
}
