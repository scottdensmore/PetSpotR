package webfrontend

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/store"
)

// EmbeddedFiles exports embeddedFiles for external test packages.
var EmbeddedFiles = embeddedFiles

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
