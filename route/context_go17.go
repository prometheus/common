// +build go1.7

package route

import (
	"context"
	"net/http"
)

// Context returns the context for the request.
func Context(r *http.Request) context.Context {
	return r.Context()
}

func newContext(r *http.Request) context.Context {
	return r.Context()
}

func setContext(ctx context.Context, r *http.Request) *http.Request {
	return r.WithContext(ctx)
}

func deleteContext(r *http.Request) {
}
