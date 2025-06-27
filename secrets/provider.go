package secrets

import (
	"context"
)

type Provider interface {
	// FetchSecret retrieves the secret value.
	FetchSecret(ctx context.Context) (string, error)

	// Name returns the provider's name (e.g., "inline").
	Name() string

	// Key returns a stable identifier for the secret being fetched.
	// It is combined with Name() to deduplicate fetch calls.
	Key() string
}

// SecretValidator allows for validating a new secret before it is
// rotated into active use. If invalid, the old secret will be used
// while it is still considered valid and has not expired.
// This interface is optional, to prevent monitoring gaps if the
// system being scraped hasn't had its secret refreshed yet.
type SecretValidator interface {
	Validate(ctx context.Context, secret string) bool
}
