package webfrontend

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/store"
)

// EmbeddedFiles exports embeddedFiles for external test packages.
var EmbeddedFiles = embeddedFiles

// NewTestServer constructs a test Server with rate limiting disabled.
func NewTestServer(t testing.TB, optStore ...store.StateStore) *Server {
	if t != nil {
		t.Helper()
	}
	var st store.StateStore
	if len(optStore) > 0 && optStore[0] != nil {
		st = optStore[0]
	} else {
		st = store.NewMemoryStore()
	}
	return NewServerWithOptions(st, ServerOptions{
		AllowPrivilegedMutations: true,
		DisableRateLimiting:      true,
	})
}

// StateStore returns the internal state store for tests.
func (s *Server) StateStore() store.StateStore {
	return s.stateStore
}
