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
	"context"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Config is the client configuration.
type Config struct {
	ClientID       string            `yaml:"client_id"`
	ClientSecret   string            `yaml:"client_secret"`
	Scopes         []string          `yaml:"scopes,omitempty"`
	TokenURL       string            `yaml:"token_url"`
	EndpointParams map[string]string `yaml:"endpoint_params,omitempty"`
}

// NewOAuth2RoundTripper returns a new http.RoundTripper that authenticates the request
// with a token fetched using the provided configuration.
func (c *Config) NewOAuth2RoundTripper(ctx context.Context, next http.RoundTripper) (http.RoundTripper, error) {
	config := &clientcredentials.Config{
		ClientID:       c.ClientID,
		ClientSecret:   c.ClientSecret,
		Scopes:         c.Scopes,
		TokenURL:       c.TokenURL,
		EndpointParams: mapToValues(c.EndpointParams),
	}

	tokenSource := config.TokenSource(ctx)

	return &oauth2.Transport{
		Base:   next,
		Source: tokenSource,
	}, nil
}

func mapToValues(m map[string]string) url.Values {
	v := url.Values{}
	for name, value := range m {
		v.Set(name, value)
	}

	return v
}
