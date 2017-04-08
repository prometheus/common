// +build !go1.7

package route

import (
	"net/http"
	"sync"

	"golang.org/x/net/context"
)

var (
	mtx   = sync.RWMutex{}
	ctxts = map[*http.Request]context.Context{}
)

// Context returns the context for the request.
func Context(r *http.Request) context.Context {
	mtx.RLock()
	defer mtx.RUnlock()
	return ctxts[r]
}

func newContext(r *http.Request) context.Context {
	return context.Background()
}

func setContext(ctx context.Context, r *http.Request) *http.Request {
	mtx.Lock()
	defer mtx.Unlock()
	ctxts[r] = ctx
	return r
}

func deleteContext(r *http.Request) {
	mtx.Lock()
	defer mtx.Unlock()
	delete(ctxts, r)
}
