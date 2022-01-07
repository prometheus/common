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

package config

import (
	"encoding/json"
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
