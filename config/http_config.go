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

// +build go1.8

package config

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mwitkow/go-conntrack"
	"golang.org/x/net/http2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	"gopkg.in/yaml.v2"
)

// DefaultHTTPClientConfig is the default HTTP client configuration.
var DefaultHTTPClientConfig = HTTPClientConfig{
	FollowRedirects: true,
}

// defaultHTTPClientOptions holds the default HTTP client options.
var defaultHTTPClientOptions = httpClientOptions{
	keepAlivesEnabled: true,
	http2Enabled:      true,
	// 5 minutes is typically above the maximum sane scrape interval. So we can
	// use keepalive for all configurations.
	idleConnTimeout: 5 * time.Minute,
}

type closeIdler interface {
	CloseIdleConnections()
}

// BasicAuth contains basic HTTP authentication credentials.
type BasicAuth struct {
	Username     string `yaml:"username" json:"username"`
	Password     Secret `yaml:"password,omitempty" json:"password,omitempty"`
	PasswordFile string `yaml:"password_file,omitempty" json:"password_file,omitempty"`
}

// SetDirectory joins any relative file paths with dir.
func (a *BasicAuth) SetDirectory(dir string) {
	if a == nil {
		return
	}
	a.PasswordFile = JoinDir(dir, a.PasswordFile)
}

// Authorization contains HTTP authorization credentials.
type Authorization struct {
	Type            string `yaml:"type,omitempty" json:"type,omitempty"`
	Credentials     Secret `yaml:"credentials,omitempty" json:"credentials,omitempty"`
	CredentialsFile string `yaml:"credentials_file,omitempty" json:"credentials_file,omitempty"`
}

// SetDirectory joins any relative file paths with dir.
func (a *Authorization) SetDirectory(dir string) {
	if a == nil {
		return
	}
	a.CredentialsFile = JoinDir(dir, a.CredentialsFile)
}

// URL is a custom URL type that allows validation at configuration load time.
type URL struct {
	*url.URL
}

// UnmarshalYAML implements the yaml.Unmarshaler interface for URLs.
func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}

	urlp, err := url.Parse(s)
	if err != nil {
		return err
	}
	u.URL = urlp
	return nil
}

// MarshalYAML implements the yaml.Marshaler interface for URLs.
func (u URL) MarshalYAML() (interface{}, error) {
	if u.URL != nil {
		return u.String(), nil
	}
	return nil, nil
}

// UnmarshalJSON implements the json.Marshaler interface for URL.
func (u *URL) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	urlp, err := url.Parse(s)
	if err != nil {
		return err
	}
	u.URL = urlp
	return nil
}

// MarshalJSON implements the json.Marshaler interface for URL.
func (u URL) MarshalJSON() ([]byte, error) {
	if u.URL != nil {
		return json.Marshal(u.URL.String())
	}
	return []byte("null"), nil
}

// OAuth2 is the oauth2 client configuration.
type OAuth2 struct {
	ClientID         string            `yaml:"client_id" json:"client_id"`
	ClientSecret     Secret            `yaml:"client_secret" json:"client_secret"`
	ClientSecretFile string            `yaml:"client_secret_file" json:"client_secret_file"`
	Scopes           []string          `yaml:"scopes,omitempty" json:"scopes,omitempty"`
	TokenURL         string            `yaml:"token_url" json:"token_url"`
	EndpointParams   map[string]string `yaml:"endpoint_params,omitempty" json:"endpoint_params,omitempty"`
}

// SetDirectory joins any relative file paths with dir.
func (a *OAuth2) SetDirectory(dir string) {
	if a == nil {
		return
	}
	a.ClientSecretFile = JoinDir(dir, a.ClientSecretFile)
}

// HTTPClientConfig configures an HTTP client.
type HTTPClientConfig struct {
	// The HTTP basic authentication credentials for the targets.
	BasicAuth *BasicAuth `yaml:"basic_auth,omitempty" json:"basic_auth,omitempty"`
	// The HTTP authorization credentials for the targets.
	Authorization *Authorization `yaml:"authorization,omitempty" json:"authorization,omitempty"`
	// The OAuth2 client credentials used to fetch a token for the targets.
	OAuth2 *OAuth2 `yaml:"oauth2,omitempty" json:"oauth2,omitempty"`
	// The bearer token for the targets. Deprecated in favour of
	// Authorization.Credentials.
	BearerToken Secret `yaml:"bearer_token,omitempty" json:"bearer_token,omitempty"`
	// The bearer token file for the targets. Deprecated in favour of
	// Authorization.CredentialsFile.
	BearerTokenFile string `yaml:"bearer_token_file,omitempty" json:"bearer_token_file,omitempty"`
	// HTTP proxy server to use to connect to the targets.
	ProxyURL URL `yaml:"proxy_url,omitempty" json:"proxy_url,omitempty"`
	// TLSConfig to use to connect to the targets.
	TLSConfig TLSConfig `yaml:"tls_config,omitempty" json:"tls_config,omitempty"`
	// FollowRedirects specifies whether the client should follow HTTP 3xx redirects.
	// The omitempty flag is not set, because it would be hidden from the
	// marshalled configuration when set to false.
	FollowRedirects bool `yaml:"follow_redirects" json:"follow_redirects"`
}

// SetDirectory joins any relative file paths with dir.
func (c *HTTPClientConfig) SetDirectory(dir string) {
	if c == nil {
		return
	}
	c.TLSConfig.SetDirectory(dir)
	c.BasicAuth.SetDirectory(dir)
	c.Authorization.SetDirectory(dir)
	c.OAuth2.SetDirectory(dir)
	c.BearerTokenFile = JoinDir(dir, c.BearerTokenFile)
}

// Validate validates the HTTPClientConfig to check only one of BearerToken,
// BasicAuth and BearerTokenFile is configured.
func (c *HTTPClientConfig) Validate() error {
	// Backwards compatibility with the bearer_token field.
	if len(c.BearerToken) > 0 && len(c.BearerTokenFile) > 0 {
		return fmt.Errorf("at most one of bearer_token & bearer_token_file must be configured")
	}
	if (c.BasicAuth != nil || c.OAuth2 != nil) && (len(c.BearerToken) > 0 || len(c.BearerTokenFile) > 0) {
		return fmt.Errorf("at most one of basic_auth, oauth2, bearer_token & bearer_token_file must be configured")
	}
	if c.BasicAuth != nil && (string(c.BasicAuth.Password) != "" && c.BasicAuth.PasswordFile != "") {
		return fmt.Errorf("at most one of basic_auth password & password_file must be configured")
	}
	if c.Authorization != nil {
		if len(c.BearerToken) > 0 || len(c.BearerTokenFile) > 0 {
			return fmt.Errorf("authorization is not compatible with bearer_token & bearer_token_file")
		}
		if string(c.Authorization.Credentials) != "" && c.Authorization.CredentialsFile != "" {
			return fmt.Errorf("at most one of authorization credentials & credentials_file must be configured")
		}
		c.Authorization.Type = strings.TrimSpace(c.Authorization.Type)
		if len(c.Authorization.Type) == 0 {
			c.Authorization.Type = "Bearer"
		}
		if strings.ToLower(c.Authorization.Type) == "basic" {
			return fmt.Errorf(`authorization type cannot be set to "basic", use "basic_auth" instead`)
		}
		if c.BasicAuth != nil || c.OAuth2 != nil {
			return fmt.Errorf("at most one of basic_auth, oauth2 & authorization must be configured")
		}
	} else {
		if len(c.BearerToken) > 0 {
			c.Authorization = &Authorization{Credentials: c.BearerToken}
			c.Authorization.Type = "Bearer"
			c.BearerToken = ""
		}
		if len(c.BearerTokenFile) > 0 {
			c.Authorization = &Authorization{CredentialsFile: c.BearerTokenFile}
			c.Authorization.Type = "Bearer"
			c.BearerTokenFile = ""
		}
	}
	if c.OAuth2 != nil {
		if c.BasicAuth != nil {
			return fmt.Errorf("at most one of basic_auth, oauth2 & authorization must be configured")
		}
		if len(c.OAuth2.ClientID) == 0 {
			return fmt.Errorf("oauth2 client_id must be configured")
		}
		if len(c.OAuth2.ClientSecret) == 0 && len(c.OAuth2.ClientSecretFile) == 0 {
			return fmt.Errorf("either oauth2 client_secret or client_secret_file must be configured")
		}
		if len(c.OAuth2.TokenURL) == 0 {
			return fmt.Errorf("oauth2 token_url must be configured")
		}
		if len(c.OAuth2.ClientSecret) > 0 && len(c.OAuth2.ClientSecretFile) > 0 {
			return fmt.Errorf("at most one of oauth2 client_secret & client_secret_file must be configured")
		}
	}
	return nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface
func (c *HTTPClientConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain HTTPClientConfig
	*c = DefaultHTTPClientConfig
	if err := unmarshal((*plain)(c)); err != nil {
		return err
	}
	return c.Validate()
}

// UnmarshalJSON implements the json.Marshaler interface for URL.
func (c *HTTPClientConfig) UnmarshalJSON(data []byte) error {
	type plain HTTPClientConfig
	*c = DefaultHTTPClientConfig
	if err := json.Unmarshal(data, (*plain)(c)); err != nil {
		return err
	}
	return c.Validate()
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (a *BasicAuth) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain BasicAuth
	return unmarshal((*plain)(a))
}

// DialContextFunc defines the signature of the DialContext() function implemented
// by net.Dialer.
type DialContextFunc func(context.Context, string, string) (net.Conn, error)

type httpClientOptions struct {
	dialContextFunc   DialContextFunc
	keepAlivesEnabled bool
	http2Enabled      bool
	idleConnTimeout   time.Duration
}

// HTTPClientOption defines an option that can be applied to the HTTP client.
type HTTPClientOption func(options *httpClientOptions)

// WithDialContextFunc allows you to override func gets used for the actual dialing. The default is `net.Dialer.DialContext`.
func WithDialContextFunc(fn DialContextFunc) HTTPClientOption {
	return func(opts *httpClientOptions) {
		opts.dialContextFunc = fn
	}
}

// WithKeepAlivesDisabled allows to disable HTTP keepalive.
func WithKeepAlivesDisabled() HTTPClientOption {
	return func(opts *httpClientOptions) {
		opts.keepAlivesEnabled = false
	}
}

// WithHTTP2Disabled allows to disable HTTP2.
func WithHTTP2Disabled() HTTPClientOption {
	return func(opts *httpClientOptions) {
		opts.http2Enabled = false
	}
}

// WithIdleConnTimeout allows setting the idle connection timeout.
func WithIdleConnTimeout(timeout time.Duration) HTTPClientOption {
	return func(opts *httpClientOptions) {
		opts.idleConnTimeout = timeout
	}
}

// NewClient returns a http.Client using the specified http.RoundTripper.
func newClient(rt http.RoundTripper) *http.Client {
	return &http.Client{Transport: rt}
}

// NewClientFromConfig returns a new HTTP client configured for the
// given config.HTTPClientConfig and config.HTTPClientOption.
// The name is used as go-conntrack metric label.
func NewClientFromConfig(cfg HTTPClientConfig, name string, optFuncs ...HTTPClientOption) (*http.Client, error) {
	rt, err := NewRoundTripperFromConfig(cfg, name, optFuncs...)
	if err != nil {
		return nil, err
	}
	client := newClient(rt)
	if !cfg.FollowRedirects {
		client.CheckRedirect = func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return client, nil
}

// NewRoundTripperFromConfig returns a new HTTP RoundTripper configured for the
// given config.HTTPClientConfig and config.HTTPClientOption.
// The name is used as go-conntrack metric label.
func NewRoundTripperFromConfig(cfg HTTPClientConfig, name string, optFuncs ...HTTPClientOption) (http.RoundTripper, error) {
	opts := defaultHTTPClientOptions
	for _, f := range optFuncs {
		f(&opts)
	}

	var dialContext func(ctx context.Context, network, addr string) (net.Conn, error)

	if opts.dialContextFunc != nil {
		dialContext = conntrack.NewDialContextFunc(
			conntrack.DialWithDialContextFunc((func(context.Context, string, string) (net.Conn, error))(opts.dialContextFunc)),
			conntrack.DialWithTracing(),
			conntrack.DialWithName(name))
	} else {
		dialContext = conntrack.NewDialContextFunc(
			conntrack.DialWithTracing(),
			conntrack.DialWithName(name))
	}

	newRT := func(tlsConfig *tls.Config) (http.RoundTripper, error) {
		// The only timeout we care about is the configured scrape timeout.
		// It is applied on request. So we leave out any timings here.
		var rt http.RoundTripper = &http.Transport{
			Proxy:                 http.ProxyURL(cfg.ProxyURL.URL),
			MaxIdleConns:          20000,
			MaxIdleConnsPerHost:   1000, // see https://github.com/golang/go/issues/13801
			DisableKeepAlives:     !opts.keepAlivesEnabled,
			TLSClientConfig:       tlsConfig,
			DisableCompression:    true,
			IdleConnTimeout:       opts.idleConnTimeout,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DialContext:           dialContext,
		}
		if opts.http2Enabled {
			// HTTP/2 support is golang has many problematic cornercases where
			// dead connections would be kept and used in connection pools.
			// https://github.com/golang/go/issues/32388
			// https://github.com/golang/go/issues/39337
			// https://github.com/golang/go/issues/39750
			// TODO: Re-Enable HTTP/2 once upstream issue is fixed.
			// TODO: use ForceAttemptHTTP2 when we move to Go 1.13+.
			err := http2.ConfigureTransport(rt.(*http.Transport))
			if err != nil {
				return nil, err
			}
		}

		// If a authorization_credentials is provided, create a round tripper that will set the
		// Authorization header correctly on each request.
		if cfg.Authorization != nil && len(cfg.Authorization.Credentials) > 0 {
			rt = NewAuthorizationCredentialsRoundTripper(cfg.Authorization.Type, cfg.Authorization.Credentials, rt)
		} else if cfg.Authorization != nil && len(cfg.Authorization.CredentialsFile) > 0 {
			rt = NewAuthorizationCredentialsFileRoundTripper(cfg.Authorization.Type, cfg.Authorization.CredentialsFile, rt)
		}
		// Backwards compatibility, be nice with importers who would not have
		// called Validate().
		if len(cfg.BearerToken) > 0 {
			rt = NewAuthorizationCredentialsRoundTripper("Bearer", cfg.BearerToken, rt)
		} else if len(cfg.BearerTokenFile) > 0 {
			rt = NewAuthorizationCredentialsFileRoundTripper("Bearer", cfg.BearerTokenFile, rt)
		}

		if cfg.BasicAuth != nil {
			rt = NewBasicAuthRoundTripper(cfg.BasicAuth.Username, cfg.BasicAuth.Password, cfg.BasicAuth.PasswordFile, rt)
		}

		if cfg.OAuth2 != nil {
			rt = NewOAuth2RoundTripper(cfg.OAuth2, rt)
		}
		// Return a new configured RoundTripper.
		return rt, nil
	}

	tlsConfig, err := NewTLSConfig(&cfg.TLSConfig)
	if err != nil {
		return nil, err
	}

	if len(cfg.TLSConfig.getCAName()) == 0 {
		// No need for a RoundTripper that reloads the CA file automatically.
		return newRT(tlsConfig)
	}

	certStore, _ := newCertStore(string(cfg.TLSConfig.CA), cfg.TLSConfig.CAFile)

	return NewTLSRoundTripper(tlsConfig, certStore, newRT)
}

type authorizationCredentialsRoundTripper struct {
	authType        string
	authCredentials Secret
	rt              http.RoundTripper
}

// NewAuthorizationCredentialsRoundTripper adds the provided credentials to a
// request unless the authorization header has already been set.
func NewAuthorizationCredentialsRoundTripper(authType string, authCredentials Secret, rt http.RoundTripper) http.RoundTripper {
	return &authorizationCredentialsRoundTripper{authType, authCredentials, rt}
}

func (rt *authorizationCredentialsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(req.Header.Get("Authorization")) == 0 {
		req = cloneRequest(req)
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", rt.authType, string(rt.authCredentials)))
	}
	return rt.rt.RoundTrip(req)
}

func (rt *authorizationCredentialsRoundTripper) CloseIdleConnections() {
	if ci, ok := rt.rt.(closeIdler); ok {
		ci.CloseIdleConnections()
	}
}

type authorizationCredentialsFileRoundTripper struct {
	authType            string
	authCredentialsFile string
	rt                  http.RoundTripper
}

// NewAuthorizationCredentialsFileRoundTripper adds the authorization
// credentials read from the provided file to a request unless the authorization
// header has already been set. This file is read for every request.
func NewAuthorizationCredentialsFileRoundTripper(authType, authCredentialsFile string, rt http.RoundTripper) http.RoundTripper {
	return &authorizationCredentialsFileRoundTripper{authType, authCredentialsFile, rt}
}

func (rt *authorizationCredentialsFileRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(req.Header.Get("Authorization")) == 0 {
		b, err := ioutil.ReadFile(rt.authCredentialsFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read authorization credentials file %s: %s", rt.authCredentialsFile, err)
		}
		authCredentials := strings.TrimSpace(string(b))

		req = cloneRequest(req)
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", rt.authType, authCredentials))
	}

	return rt.rt.RoundTrip(req)
}

func (rt *authorizationCredentialsFileRoundTripper) CloseIdleConnections() {
	if ci, ok := rt.rt.(closeIdler); ok {
		ci.CloseIdleConnections()
	}
}

type basicAuthRoundTripper struct {
	username     string
	password     Secret
	passwordFile string
	rt           http.RoundTripper
}

// NewBasicAuthRoundTripper will apply a BASIC auth authorization header to a request unless it has
// already been set.
func NewBasicAuthRoundTripper(username string, password Secret, passwordFile string, rt http.RoundTripper) http.RoundTripper {
	return &basicAuthRoundTripper{username, password, passwordFile, rt}
}

func (rt *basicAuthRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(req.Header.Get("Authorization")) != 0 {
		return rt.rt.RoundTrip(req)
	}
	req = cloneRequest(req)
	if rt.passwordFile != "" {
		bs, err := ioutil.ReadFile(rt.passwordFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read basic auth password file %s: %s", rt.passwordFile, err)
		}
		req.SetBasicAuth(rt.username, strings.TrimSpace(string(bs)))
	} else {
		req.SetBasicAuth(rt.username, strings.TrimSpace(string(rt.password)))
	}
	return rt.rt.RoundTrip(req)
}

func (rt *basicAuthRoundTripper) CloseIdleConnections() {
	if ci, ok := rt.rt.(closeIdler); ok {
		ci.CloseIdleConnections()
	}
}

type oauth2RoundTripper struct {
	config *OAuth2
	rt     http.RoundTripper
	next   http.RoundTripper
	secret string
	mtx    sync.RWMutex
}

func NewOAuth2RoundTripper(config *OAuth2, next http.RoundTripper) http.RoundTripper {
	return &oauth2RoundTripper{
		config: config,
		next:   next,
	}
}

func (rt *oauth2RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		secret  string
		changed bool
	)

	if rt.config.ClientSecretFile != "" {
		data, err := ioutil.ReadFile(rt.config.ClientSecretFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read oauth2 client secret file %s: %s", rt.config.ClientSecretFile, err)
		}
		secret = strings.TrimSpace(string(data))
		rt.mtx.RLock()
		changed = secret != rt.secret
		rt.mtx.RUnlock()
	}

	if changed || rt.rt == nil {
		if rt.config.ClientSecret != "" {
			secret = string(rt.config.ClientSecret)
		}

		config := &clientcredentials.Config{
			ClientID:       rt.config.ClientID,
			ClientSecret:   secret,
			Scopes:         rt.config.Scopes,
			TokenURL:       rt.config.TokenURL,
			EndpointParams: mapToValues(rt.config.EndpointParams),
		}

		tokenSource := config.TokenSource(context.Background())

		rt.mtx.Lock()
		rt.secret = secret
		rt.rt = &oauth2.Transport{
			Base:   rt.next,
			Source: tokenSource,
		}
		rt.mtx.Unlock()
	}

	rt.mtx.RLock()
	currentRT := rt.rt
	rt.mtx.RUnlock()
	return currentRT.RoundTrip(req)
}

func (rt *oauth2RoundTripper) CloseIdleConnections() {
	// OAuth2 RT does not support CloseIdleConnections() but the next RT might.
	if ci, ok := rt.next.(closeIdler); ok {
		ci.CloseIdleConnections()
	}
}

func mapToValues(m map[string]string) url.Values {
	v := url.Values{}
	for name, value := range m {
		v.Set(name, value)
	}

	return v
}

// cloneRequest returns a clone of the provided *http.Request.
// The clone is a shallow copy of the struct and its Header map.
func cloneRequest(r *http.Request) *http.Request {
	// Shallow copy of the struct.
	r2 := new(http.Request)
	*r2 = *r
	// Deep copy of the Header.
	r2.Header = make(http.Header)
	for k, s := range r.Header {
		r2.Header[k] = s
	}
	return r2
}

// NewTLSConfig creates a new tls.Config from the given TLSConfig.
func NewTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify}

	// If a CA cert is provided then let's read it in so we can validate the
	// scrape target's certificate properly.
	if ca, err := cfg.getCA(); err != nil {
		return nil, fmt.Errorf("unable to get specified CA %s: %w", cfg.getCAName(), err)
	} else if ca != nil {
		if !updateRootCA(tlsConfig, ca) {
			return nil, fmt.Errorf("unable to use specified CA %s", cfg.getCAName())
		}
	}

	if len(cfg.ServerName) > 0 {
		tlsConfig.ServerName = cfg.ServerName
	}
	// If a client cert & key is provided then configure TLS config accordingly.
	if certName, keyName := cfg.getCertName(), cfg.getKeyName(); len(certName) > 0 && len(keyName) == 0 {
		return nil, fmt.Errorf("client cert file %q specified without client key file", certName)
	} else if len(keyName) > 0 && len(certName) == 0 {
		return nil, fmt.Errorf("client key file %q specified without client cert file", keyName)
	} else if len(certName) > 0 && len(keyName) > 0 {
		// Verify that client cert and key are valid.
		if _, err := cfg.getClientCertificate(nil); err != nil {
			return nil, err
		}
		tlsConfig.GetClientCertificate = cfg.getClientCertificate
	}

	return tlsConfig, nil
}

// TLSConfig configures the options for TLS connections.
type TLSConfig struct {
	// The CA cert to use for the targets.
	CA Secret `yaml:"ca,omitempty" json:"ca,omitempty"`
	// The CA cert file to use for the targets.
	CAFile string `yaml:"ca_file,omitempty" json:"ca_file,omitempty"`
	// The client cert for the targets.
	Cert Secret `yaml:"cert,omitempty" json:"cert,omitempty"`
	// The client cert file for the targets.
	CertFile string `yaml:"cert_file,omitempty" json:"cert_file,omitempty"`
	// The client key for the targets.
	Key Secret `yaml:"key,omitempty" json:"key,omitempty"`
	// The client key file for the targets.
	KeyFile string `yaml:"key_file,omitempty" json:"key_file,omitempty"`
	// Used to verify the hostname for the targets.
	ServerName string `yaml:"server_name,omitempty" json:"server_name,omitempty"`
	// Disable target certificate validation.
	InsecureSkipVerify bool `yaml:"insecure_skip_verify" json:"insecure_skip_verify"`
}

// SetDirectory joins any relative file paths with dir.
func (c *TLSConfig) SetDirectory(dir string) {
	if c == nil {
		return
	}
	if len(c.CAFile) > 0 {
		c.CAFile = JoinDir(dir, c.CAFile)
	}
	if len(c.CertFile) > 0 {
		c.CertFile = JoinDir(dir, c.CertFile)
	}
	if len(c.KeyFile) > 0 {
		c.KeyFile = JoinDir(dir, c.KeyFile)
	}
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (c *TLSConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain TLSConfig
	return unmarshal((*plain)(c))
}

func getCertificate(inline Secret, filename string) ([]byte, error) {
	if len(inline) != 0 {
		return []byte(inline), nil
	}

	if len(filename) != 0 {
		return readCertificateFile(filename)
	}

	return nil, nil
}

func getCertificateName(inline Secret, filename string) string {
	if len(inline) > 0 {
		return "<inline>"
	} else if len(filename) > 0 {
		return filename
	}

	return ""
}

func (c *TLSConfig) getCA() ([]byte, error) {
	return getCertificate(c.CA, c.CAFile)
}

func (c *TLSConfig) getCAName() string {
	return getCertificateName(c.CA, c.CAFile)
}

func (c *TLSConfig) getCert() ([]byte, error) {
	return getCertificate(c.Cert, c.CertFile)
}

func (c *TLSConfig) getCertName() string {
	return getCertificateName(c.Cert, c.CertFile)
}

func (c *TLSConfig) getKey() ([]byte, error) {
	return getCertificate(c.Key, c.KeyFile)
}

func (c *TLSConfig) getKeyName() string {
	return getCertificateName(c.Key, c.KeyFile)
}

// getClientCertificate reads the pair of client cert and key from disk and returns a tls.Certificate.
func (c *TLSConfig) getClientCertificate(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
	var (
		certBlob, keyBlob []byte
		err               error
	)

	certBlob, err = c.getCert()
	if err != nil {
		return nil, fmt.Errorf("unable to use specified client cert (%s) & key (%s): %s", c.getCertName(), c.getKeyName(), err)
	}

	keyBlob, err = c.getKey()
	if err != nil {
		return nil, fmt.Errorf("unable to use specified client cert (%s) & key (%s): %s", c.getCertName(), c.getKeyName(), err)
	}

	cert, err := tls.X509KeyPair(certBlob, keyBlob)
	if err != nil {
		return nil, fmt.Errorf("unable to use specified client cert (%s) & key (%s): %s", c.getCertName(), c.getKeyName(), err)
	}
	return &cert, nil
}

// readCertificateFile reads the CA cert file from disk.
func readCertificateFile(f string) ([]byte, error) {
	data, err := ioutil.ReadFile(f)
	if err != nil {
		return nil, fmt.Errorf("unable to load specified certificate file %s: %s", f, err)
	}
	return data, nil
}

// updateRootCA parses the given byte slice as a series of PEM encoded certificates and updates tls.Config.RootCAs.
func updateRootCA(cfg *tls.Config, b []byte) bool {
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(b) {
		return false
	}
	cfg.RootCAs = caCertPool
	return true
}

// CertGetter defines how to access certificates.
type CertGetter interface {
	// GetCert returns the corresponding certificate, whether or not
	// it's been updated, or an error if it's not possible to
	// retrieve the certificate.
	GetCert() (cert []byte, updated bool, err error)
}

// newCertStore creates a CertGetter that provides access to the
// certificate stored in the specified filename or the inline
// certificate cert.
func newCertStore(cert, filename string) (CertGetter, error) {
	if len(filename) > 0 {
		return &fileCertStore{filename: filename}, nil
	} else if len(cert) > 0 {
		return inlineCertStore{cert: []byte(cert)}, nil
	} else {
		return nil, errors.New("invalid certificate inputs")
	}
}

// inlineCertStore implements a CertGetter that never changes.
type inlineCertStore struct {
	cert []byte
}

func (s inlineCertStore) GetCert() ([]byte, bool, error) {
	return s.cert, false, nil
}

// fileCertStore loads a certificate from a filename each time the
// certificate is requested.
type fileCertStore struct {
	filename string
	mtx      sync.Mutex // mtx protects accesses to cert
	cert     []byte
}

func (s *fileCertStore) GetCert() ([]byte, bool, error) {
	updated := false
	newCert, err := readCertificateFile(s.filename)
	if err != nil {
		return nil, false, err
	}

	s.mtx.Lock()
	if !bytes.Equal(s.cert, newCert) {
		s.cert = newCert
		updated = true
	}
	s.mtx.Unlock()

	return newCert, updated, nil
}

// tlsRoundTripper is a RoundTripper that updates automatically its TLS
// configuration whenever the content of the CA file changes.
type tlsRoundTripper struct {
	certStore CertGetter
	// newRT returns a new RoundTripper.
	newRT func(*tls.Config) (http.RoundTripper, error)

	mtx       sync.RWMutex
	rt        http.RoundTripper
	tlsConfig *tls.Config
}

func NewTLSRoundTripper(
	cfg *tls.Config,
	certStore CertGetter,
	newRT func(*tls.Config) (http.RoundTripper, error),
) (http.RoundTripper, error) {
	t := &tlsRoundTripper{
		certStore: certStore,
		newRT:     newRT,
		tlsConfig: cfg,
	}

	rt, err := t.newRT(t.tlsConfig)
	if err != nil {
		return nil, err
	}
	t.rt = rt

	_, _, err = t.certStore.GetCert()
	if err != nil {
		return nil, err
	}

	return t, nil
}

// RoundTrip implements the http.RoundTrip interface.
func (t *tlsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	b, updated, err := t.certStore.GetCert()
	if err != nil {
		return nil, err
	}

	t.mtx.RLock()
	rt := t.rt
	t.mtx.RUnlock()
	if !updated {
		// The CA cert hasn't changed, use the existing RoundTripper.
		return rt.RoundTrip(req)
	}

	// Create a new RoundTripper.
	tlsConfig := t.tlsConfig.Clone()
	if !updateRootCA(tlsConfig, b) {
		return nil, errors.New("unable to use specified CA cert")
	}
	rt, err = t.newRT(tlsConfig)
	if err != nil {
		return nil, err
	}
	t.CloseIdleConnections()

	t.mtx.Lock()
	t.rt = rt
	t.mtx.Unlock()

	return rt.RoundTrip(req)
}

func (t *tlsRoundTripper) CloseIdleConnections() {
	t.mtx.RLock()
	defer t.mtx.RUnlock()
	if ci, ok := t.rt.(closeIdler); ok {
		ci.CloseIdleConnections()
	}
}

func (c HTTPClientConfig) String() string {
	b, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Sprintf("<error creating http client config string: %s>", err)
	}
	return string(b)
}
