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

package oauth2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
)

type testServerResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

func TestOAuth2(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res, _ := json.Marshal(testServerResponse{
			AccessToken: "12345",
			TokenType:   "Bearer",
		})
		w.Header().Add("Content-Type", "application/json")
		_, _ = w.Write(res)
	}))
	defer ts.Close()

	var yamlConfig = fmt.Sprintf(`
client_id: 1
client_secret: 2
scopes:
 - A
 - B
token_url: %s
endpoint_params:
 hi: hello
`, ts.URL)
	expectedConfig := Config{
		ClientID:       "1",
		ClientSecret:   "2",
		Scopes:         []string{"A", "B"},
		EndpointParams: map[string]string{"hi": "hello"},
		TokenURL:       ts.URL,
	}

	var unmarshalledConfig Config
	err := yaml.Unmarshal([]byte(yamlConfig), &unmarshalledConfig)
	if err != nil {
		t.Fatalf("Expected no error unmarshalling yaml, got %v", err)
	}
	if !reflect.DeepEqual(unmarshalledConfig, expectedConfig) {
		t.Fatalf("Got unmarshalled config %q, expected %q", unmarshalledConfig, expectedConfig)
	}

	rt, err := expectedConfig.NewOAuth2RoundTripper(http.DefaultTransport)
	if err != nil {
		t.Fatalf("Expected no error creating round tripper, got %v", err)
	}

	client := http.Client{
		Transport: rt,
	}
	resp, _ := client.Get(ts.URL)

	authorization := resp.Request.Header.Get("Authorization")
	if authorization != "Bearer 12345" {
		t.Fatalf("Expected authorization header to be 'Bearer 12345', got '%s'", authorization)
	}
}
