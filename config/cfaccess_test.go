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
	"errors"
	"net/http"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudflare/cloudflared/token"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

// signedTestToken returns a JWT with the given expiry encoded in its "exp"
// claim. cfAccessTokenExpiry does not verify the signature, so the signing
// key is arbitrary.
func signedTestToken(t *testing.T, expiry time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": jwt.NewNumericDate(expiry),
	})
	signed, err := tok.SignedString([]byte("test-signing-key"))
	require.NoError(t, err)
	return signed
}

func TestCFAccessTokenExpiry(t *testing.T) {
	t.Run("valid token", func(t *testing.T) {
		expiry := time.Now().Add(time.Hour).Truncate(time.Second)
		got := cfAccessTokenExpiry(signedTestToken(t, expiry))
		require.WithinDuration(t, expiry, got, time.Second)
	})

	t.Run("malformed token", func(t *testing.T) {
		require.True(t, cfAccessTokenExpiry("not-a-jwt").IsZero())
	})

	t.Run("token without exp claim", func(t *testing.T) {
		tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{})
		signed, err := tok.SignedString([]byte("test-signing-key"))
		require.NoError(t, err)
		require.True(t, cfAccessTokenExpiry(signed).IsZero())
	})
}

func TestIsCFAccessAuthType(t *testing.T) {
	for _, tc := range []struct {
		authType string
		want     bool
	}{
		{"cf-access", true},
		{"CF-Access", true},
		{"  cf-access  ", true},
		{"Bearer", false},
		{"", false},
	} {
		require.Equalf(t, tc.want, isCFAccessAuthType(tc.authType), "authType=%q", tc.authType)
	}
}

// withFakeCFAccess overrides cfAccessGetAppInfo and cfAccessFetchToken for
// the duration of the test, restoring the real cloudflared-backed
// implementations afterwards.
func withFakeCFAccess(
	t *testing.T,
	getAppInfo func(reqURL *url.URL) (*token.AppInfo, error),
	fetchToken func(appURL *url.URL, appInfo *token.AppInfo) (string, error),
) {
	t.Helper()

	origGetAppInfo := cfAccessGetAppInfo
	origFetchToken := cfAccessFetchToken
	t.Cleanup(func() {
		cfAccessGetAppInfo = origGetAppInfo
		cfAccessFetchToken = origFetchToken
	})

	cfAccessGetAppInfo = getAppInfo
	cfAccessFetchToken = func(appURL *url.URL, appInfo *token.AppInfo, _, _ bool, _ *zerolog.Logger) (string, error) {
		return fetchToken(appURL, appInfo)
	}
}

func TestCFAccessRoundTripper(t *testing.T) {
	fakeNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	origNow := cfAccessNow
	cfAccessNow = func() time.Time { return fakeNow }
	t.Cleanup(func() { cfAccessNow = origNow })

	origMargin := cfAccessTokenExpiryMargin
	cfAccessTokenExpiryMargin = 30 * time.Second
	t.Cleanup(func() { cfAccessTokenExpiryMargin = origMargin })

	var (
		getAppInfoCalls atomic.Int32
		fetchTokenCalls atomic.Int32
	)

	shortLivedToken := signedTestToken(t, fakeNow.Add(time.Minute))
	longLivedToken := signedTestToken(t, fakeNow.Add(time.Hour))

	withFakeCFAccess(t,
		func(reqURL *url.URL) (*token.AppInfo, error) {
			getAppInfoCalls.Add(1)
			return &token.AppInfo{AuthDomain: "auth." + reqURL.Host, AppAUD: "aud", AppDomain: reqURL.Host}, nil
		},
		func(*url.URL, *token.AppInfo) (string, error) {
			n := fetchTokenCalls.Add(1)
			if n == 1 {
				return shortLivedToken, nil
			}
			return longLivedToken, nil
		},
	)

	var gotHeader string
	next := NewRoundTripCheckRequest(func(req *http.Request) {
		gotHeader = req.Header.Get(cfAccessTokenHeader)
	}, &http.Response{StatusCode: http.StatusOK}, nil)

	rt := newCFAccessRoundTripper(next, "test")

	req1, err := http.NewRequest(http.MethodGet, "https://app.example.com/query", http.NoBody)
	require.NoError(t, err)
	_, err = rt.RoundTrip(req1)
	require.NoError(t, err)
	require.Equal(t, shortLivedToken, gotHeader)
	require.EqualValues(t, 1, getAppInfoCalls.Load())
	require.EqualValues(t, 1, fetchTokenCalls.Load())

	// A second request to the same host, while the cached token is still
	// valid, must not re-fetch anything.
	req2, err := http.NewRequest(http.MethodGet, "https://app.example.com/query", http.NoBody)
	require.NoError(t, err)
	_, err = rt.RoundTrip(req2)
	require.NoError(t, err)
	require.Equal(t, shortLivedToken, gotHeader)
	require.EqualValues(t, 1, getAppInfoCalls.Load())
	require.EqualValues(t, 1, fetchTokenCalls.Load())

	// A request to a different host must fetch a fresh token, independent
	// of the first host's cached state.
	req3, err := http.NewRequest(http.MethodGet, "https://other.example.com/query", http.NoBody)
	require.NoError(t, err)
	_, err = rt.RoundTrip(req3)
	require.NoError(t, err)
	require.Equal(t, longLivedToken, gotHeader)
	require.EqualValues(t, 2, getAppInfoCalls.Load())
	require.EqualValues(t, 2, fetchTokenCalls.Load())

	// Advancing the clock past the short-lived token's expiry margin must
	// trigger a refetch for the original host, reusing the already-known
	// AppInfo.
	fakeNow = fakeNow.Add(time.Minute)
	req4, err := http.NewRequest(http.MethodGet, "https://app.example.com/query", http.NoBody)
	require.NoError(t, err)
	_, err = rt.RoundTrip(req4)
	require.NoError(t, err)
	require.Equal(t, longLivedToken, gotHeader)
	require.EqualValuesf(t, 2, getAppInfoCalls.Load(), "AppInfo should be cached across token refreshes")
	require.EqualValues(t, 3, fetchTokenCalls.Load())
}

var errFakeGetAppInfo = errors.New("fake GetAppInfo failure")

func TestCFAccessRoundTripperGetAppInfoError(t *testing.T) {
	withFakeCFAccess(t,
		func(*url.URL) (*token.AppInfo, error) {
			return nil, errFakeGetAppInfo
		},
		func(*url.URL, *token.AppInfo) (string, error) {
			t.Fatal("FetchToken must not be called when GetAppInfo fails")
			return "", nil
		},
	)

	next := NewRoundTripCheckRequest(func(*http.Request) {
		t.Fatal("next RoundTripper must not be called when authentication fails")
	}, nil, nil)

	rt := newCFAccessRoundTripper(next, "test")
	req, err := http.NewRequest(http.MethodGet, "https://app.example.com/query", http.NoBody)
	require.NoError(t, err)

	_, err = rt.RoundTrip(req)
	require.Error(t, err)
	require.ErrorIs(t, err, errFakeGetAppInfo)
}
