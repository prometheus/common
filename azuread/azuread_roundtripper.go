package azuread

import (
	"context"
	"net/http"
)

// Round tripper adding Azure AD authorization to calls
type azureAdRoundTripper struct {
	next          http.RoundTripper
	tokenProvider TokenProvider
}

// Creates round tripper adding Azure AD authorization to calls
func NewAzureAdRoundTripper(cfg *AzureAdConfig, next http.RoundTripper) (http.RoundTripper, error) {
	if next == nil {
		next = http.DefaultTransport
	}

	cred, err := NewTokenCredential(cfg)
	if err != nil {
		return nil, err
	}

	tokenProvider, err := NewTokenProvider(context.Background(), cfg, cred)
	if err != nil {
		return nil, err
	}

	rt := &azureAdRoundTripper{
		next:          next,
		tokenProvider: tokenProvider,
	}
	return rt, nil
}

// Sets Auhtorization header for requests
func (rt *azureAdRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	bearerAccessToken := "Bearer " + rt.tokenProvider.GetAccessToken()
	req.Header.Set("Authorization", bearerAccessToken)

	return rt.next.RoundTrip(req)
}
