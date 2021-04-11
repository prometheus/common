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
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Config is the client configuration.
type Config struct {
	ClientID     string
	ClientSecret string
	Scopes       []string
	TokenURL     string
}

// Token is an OAuth2 access token and associated data.
type Token struct {
	AccessToken string
	TokenType   string
	ExpiresIn   int64
	ExpiresAt   int64

	// So we can refresh.
	config Config
	lock   sync.Mutex
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	ErrorURI         string `json:"error_uri"`
}

// GetAccessToken gets an access token from the token URL using the client credentials.
func (config Config) GetAccessToken() (*Token, error) {
	values := url.Values{
		"grant_type": {"client_credentials"},
	}
	if len(config.Scopes) > 0 {
		values.Set("scope", strings.Join(config.Scopes, " "))
	}

	req, err := http.NewRequest("POST", config.TokenURL, strings.NewReader(values.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(url.QueryEscape(config.ClientID), url.QueryEscape(config.ClientSecret))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		var data tokenErrorResponse
		err = json.Unmarshal(body, &data)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("Error obtaining access token: %s %s %s", data.Error, data.ErrorDescription, data.ErrorURI)
	}

	var data tokenResponse
	err = json.Unmarshal(body, &data)
	if err != nil {
		return nil, err
	}

	if data.AccessToken == "" {
		return nil, fmt.Errorf("Missing fields in access token response: AccessToken: '%v'", data.AccessToken)
	}

	if data.TokenType == "" {
		data.TokenType = "Bearer"
	}

	var expiresAt int64
	if data.ExpiresIn != 0 {
		expiresAt = time.Now().Unix() + data.ExpiresIn
	}

	return &Token{AccessToken: data.AccessToken, TokenType: data.TokenType, ExpiresIn: data.ExpiresIn, ExpiresAt: expiresAt}, nil
}

// Get checks if the access token is still valid, and if so re-fetches it before returning along with the token type.
func (token *Token) Get() (string, string, error) {
	if token.Valid() {
		return token.AccessToken, token.TokenType, nil
	}

	token.lock.Lock()
	defer token.lock.Unlock()

	t, err := token.config.GetAccessToken()
	if err != nil {
		return "", "", err
	}

	token = t
	return token.AccessToken, token.TokenType, nil
}

// Valid checks if the token has expired.
func (token *Token) Valid() bool {
	token.lock.Lock()
	defer token.lock.Unlock()

	return token.ExpiresAt == 0 || token.ExpiresAt > time.Now().Unix()
}
