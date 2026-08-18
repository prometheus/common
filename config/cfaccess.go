// Copyright The Prometheus Authors
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
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudflare/cloudflared/token"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
)

// cfAccessAuthType is the value of Authorization.Type that selects
// Cloudflare Access authentication instead of a literal HTTP Authorization
// scheme. See https://developers.cloudflare.com/cloudflare-one/policies/access/.
const cfAccessAuthType = "cf-access"

// cfAccessTokenHeader is the header Cloudflare Access checks for a JWT
// obtained through its browser-based login flow.
//
// See https://developers.cloudflare.com/cloudflare-one/tutorials/cli/#curl.
const cfAccessTokenHeader = "Cf-Access-Token"

// cfAccessTokenExpiryMargin is how long before a cached Cloudflare Access
// token's expiry cfAccessRoundTripper proactively fetches a new one.
// Overridable in tests.
var cfAccessTokenExpiryMargin = 30 * time.Second

// cfAccessNow stands in for time.Now, overridable in tests so that token
// expiry can be exercised deterministically instead of via real sleeps.
var cfAccessNow = time.Now

// isCFAccessAuthType reports whether authType selects Cloudflare Access
// authentication.
func isCFAccessAuthType(authType string) bool {
	return strings.EqualFold(strings.TrimSpace(authType), cfAccessAuthType)
}

// cfAccessGetAppInfo and cfAccessFetchToken are indirections over the
// github.com/cloudflare/cloudflared/token package, overridable in tests.
var (
	cfAccessGetAppInfo = token.GetAppInfo
	cfAccessFetchToken = token.FetchToken
)

// cfAccessLogger is shared by all cfAccessRoundTrippers to report the
// progress of interactive Cloudflare Access logins.
var cfAccessLogger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"}).With().Timestamp().Logger()

// cfAccessApp caches the Cloudflare Access application info and the most
// recently obtained token for a single scheme://host.
type cfAccessApp struct {
	mtx     sync.Mutex
	info    *token.AppInfo
	token   string
	expires time.Time
}

// cfAccessRoundTripper authenticates requests against applications protected
// by Cloudflare Access, by attaching a Cf-Access-Token header obtained
// through cloudflared's login flow.
//
// Unlike a static Authorization header, the target application (and
// therefore its audience) is only known once an actual request is made, so
// the login for each scheme://host seen by this RoundTripper is performed
// lazily, on the first request to it, and cached both in memory and (via
// cloudflared) on disk, until the token is close to expiring.
type cfAccessRoundTripper struct {
	next http.RoundTripper

	mtx  sync.Mutex
	apps map[string]*cfAccessApp
}

// newCFAccessRoundTripper returns a RoundTripper that authenticates requests
// against Cloudflare Access before forwarding them to next. name identifies
// the calling application in the User-Agent header used while
// authenticating, and in Cloudflare Access's own logs.
func newCFAccessRoundTripper(next http.RoundTripper, name string) http.RoundTripper {
	token.Init(name)
	return &cfAccessRoundTripper{
		next: next,
		apps: make(map[string]*cfAccessApp),
	}
}

// appFor returns the cfAccessApp tracking state for the host targeted by
// req, creating one if this is the first time it has been seen.
func (rt *cfAccessRoundTripper) appFor(key string) *cfAccessApp {
	rt.mtx.Lock()
	defer rt.mtx.Unlock()

	app, ok := rt.apps[key]
	if !ok {
		app = &cfAccessApp{}
		rt.apps[key] = app
	}
	return app
}

// RoundTrip implements http.RoundTripper.
func (rt *cfAccessRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.URL.Scheme + "://" + req.URL.Host
	app := rt.appFor(key)

	tok, err := app.fetch(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare access: %w", err)
	}

	req.Header.Set(cfAccessTokenHeader, tok)
	return rt.next.RoundTrip(req)
}

// fetch returns a valid Cloudflare Access token for the application behind
// req.URL, fetching or refreshing it as needed. It may block on an
// interactive browser login if no valid cached token is available, either
// in memory or in cloudflared's own on-disk token cache.
func (a *cfAccessApp) fetch(req *http.Request) (string, error) {
	a.mtx.Lock()
	defer a.mtx.Unlock()

	if a.token != "" && cfAccessNow().Add(cfAccessTokenExpiryMargin).Before(a.expires) {
		return a.token, nil
	}

	if a.info == nil {
		info, err := cfAccessGetAppInfo(req.URL)
		if err != nil {
			return "", fmt.Errorf("failed to detect Cloudflare Access application for %s://%s: %w", req.URL.Scheme, req.URL.Host, err)
		}
		a.info = info
	}

	tok, err := cfAccessFetchToken(req.URL, a.info, false, false, &cfAccessLogger)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Cloudflare Access token: %w", err)
	}

	a.token = tok
	a.expires = cfAccessTokenExpiry(tok)
	return tok, nil
}

// cfAccessTokenExpiry returns the expiry time encoded in the "exp" claim of
// tok, or the zero time if it cannot be determined. tok is not signature
// verified: it was just obtained directly from cloudflared over an
// authenticated channel, so verification against Cloudflare's public keys
// would add complexity without a meaningful security benefit here. A zero
// return value simply means the token will be treated as already expired,
// and a new one fetched (which, thanks to cloudflared's own on-disk cache,
// is cheap and does not by itself trigger a new interactive login) on the
// next request.
func cfAccessTokenExpiry(tok string) time.Time {
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(tok, claims); err != nil {
		return time.Time{}
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return time.Time{}
	}
	return exp.Time
}
