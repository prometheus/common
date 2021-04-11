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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

var expectedConfig = Config{
	ClientID:     "1234",
	ClientSecret: "12345",
}
var expectedToken = Token{
	AccessToken: "123456",
	TokenType:   "Bearer",
	ExpiresIn:   1234567,
}

func TestOAuth2(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if grant := r.Form.Get("grant_type"); grant != "client_credentials" {
			res, _ := json.Marshal(tokenErrorResponse{
				Error: "unsupported_grant_type",
			})
			http.Error(w, string(res), 400)
			return
		}

		if scope := r.Form.Get("scope"); scope != "" && scope != "A B" {
			res, _ := json.Marshal(tokenErrorResponse{
				Error: "invalid_scope",
			})
			http.Error(w, string(res), 400)
			return
		}

		clientID, clientSecret, ok := r.BasicAuth()
		if !ok {
			res, _ := json.Marshal(tokenErrorResponse{
				Error: "invalid_request",
			})
			http.Error(w, string(res), 400)
			return
		}

		if clientID != expectedConfig.ClientID || clientSecret != expectedConfig.ClientSecret {
			res, _ := json.Marshal(tokenErrorResponse{
				Error:            "invalid_client",
				ErrorDescription: "bad credentials",
			})
			http.Error(w, string(res), 401)
			return
		}

		res, _ := json.Marshal(tokenResponse{
			AccessToken: expectedToken.AccessToken,
			TokenType:   expectedToken.TokenType,
			ExpiresIn:   expectedToken.ExpiresIn,
		})
		w.Write(res)
	}))
	defer ts.Close()

	cases := []struct {
		name                 string
		config               Config
		expectedToken        *Token
		expectError          bool
		expectedErrorMessage string
	}{
		{
			name: "valid request",
			config: Config{
				ClientID:     expectedConfig.ClientID,
				ClientSecret: expectedConfig.ClientSecret,
				TokenURL:     ts.URL,
			},
			expectedToken: &expectedToken,
		},
		{
			name: "valid request with scopes",
			config: Config{
				ClientID:     expectedConfig.ClientID,
				ClientSecret: expectedConfig.ClientSecret,
				TokenURL:     ts.URL,
				Scopes:       []string{"A", "B"},
			},
			expectedToken: &expectedToken,
		},
		{
			name: "invalid request with bad credentials",
			config: Config{
				TokenURL: ts.URL,
			},
			expectError:          true,
			expectedErrorMessage: "Error obtaining access token: invalid_client bad credentials ",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			token, err := c.config.GetAccessToken()
			if c.expectError {
				if err == nil {
					t.Fatalf("Expected error '%s', recieved none", c.expectedErrorMessage)
				}
				if err.Error() != c.expectedErrorMessage {
					t.Fatalf("Expected error '%s', received error '%v'", c.expectedErrorMessage, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Expected no error, recieved error '%v'", err)
			}

			if c.expectedToken.ExpiresIn != 0 {
				c.expectedToken.ExpiresAt = time.Now().Unix() + token.ExpiresIn
			}

			same := token.AccessToken == c.expectedToken.AccessToken || token.TokenType == c.expectedToken.TokenType || token.ExpiresIn == c.expectedToken.ExpiresIn || token.ExpiresAt <= c.expectedToken.ExpiresAt

			if !same {
				t.Fatalf("Expected token '%v', recieved token '%v'", c.expectedToken, token)
			}

			if accessToken, _ := token.Get(); accessToken != token.AccessToken {
				t.Fatalf("Expected access token '%s' from token.Get(), recieved access token '%s'", token.AccessToken, accessToken)
			}

			if valid := token.Valid(); !valid {
				t.Fatal("Expected token to be valid")
			}
		})
	}

	token := &Token{
		AccessToken: "654321",
		ExpiresAt:   time.Now().Unix() - 1000,
		config: Config{
			ClientID:     expectedConfig.ClientID,
			ClientSecret: expectedConfig.ClientSecret,
			TokenURL:     ts.URL,
		},
	}

	if valid := token.Valid(); valid {
		t.Fatal("Expected token to be invalid")
	}

	accessToken, err := token.Get()
	if err != nil {
		t.Fatalf("Expected no error, got error '%v'", err)
	}

	if accessToken == "654321" {
		t.Fatal("Expected token to have been refetched, instead it stayed the same")
	}
}
