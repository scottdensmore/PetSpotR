package webfrontend

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/store"
)

// NewTestServer constructs a test Server with rate limiting disabled.
func NewTestServer(t testing.TB, st store.StateStore) *Server {
	if t != nil {
		t.Helper()
	}
	if st == nil {
		st = store.NewMemoryStore()
	}
	return NewServerWithOptions(st, ServerOptions{
		AllowPrivilegedMutations: true,
		DisableRateLimiting:      true,
	})
}
