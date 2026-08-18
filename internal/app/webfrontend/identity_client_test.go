package webfrontend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestIdentityClientConfigReflectsOnlyEnabledPublicSettings(t *testing.T) {
	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		t.Parallel()
		srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{})
		response := requestIdentityClientConfig(t, srv)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("disabled config = %d Cache-Control %q", response.Code, response.Header().Get("Cache-Control"))
		}
		assertIdentityClientConfig(t, response, identity.WebClientConfig{})
	})

	t.Run("Google provider", func(t *testing.T) {
		t.Parallel()
		config := identity.WebClientConfig{
			Enabled:         true,
			Provider:        identity.ProviderGoogle,
			APIKey:          "public-browser-key",
			AuthDomain:      "demo-petspotr-auth.firebaseapp.com",
			ProjectID:       "demo-petspotr-auth",
			AuthEmulatorURL: "http://127.0.0.1:9099",
		}
		srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
			IdentitySessions:     &stubSessionManager{},
			IdentityClientConfig: config,
		})
		response := requestIdentityClientConfig(t, srv)
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("enabled config = %d Cache-Control %q", response.Code, response.Header().Get("Cache-Control"))
		}
		assertIdentityClientConfig(t, response, config)
		if body := response.Body.String(); containsAny(body, "session", "cookie", "secret") {
			t.Fatalf("client config exposed private settings: %s", body)
		}
		policy := response.Header().Get("Content-Security-Policy")
		for _, source := range []string{
			"https://www.gstatic.com", "https://identitytoolkit.googleapis.com",
			"https://securetoken.googleapis.com", "https://demo-petspotr-auth.firebaseapp.com",
			"http://127.0.0.1:9099",
		} {
			if !strings.Contains(policy, source) {
				t.Errorf("enabled Content-Security-Policy omitted %q: %s", source, policy)
			}
		}
	})
}

func TestIdentityClientConfigAllowsOnlyGet(t *testing.T) {
	t.Parallel()
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/session/client-config", nil)
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, request)
	if response.Code != http.StatusMethodNotAllowed || response.Header().Get("Allow") != http.MethodGet {
		t.Fatalf("POST config = %d Allow %q", response.Code, response.Header().Get("Allow"))
	}
}

func TestIdentityClientConfigFailsClosedBeforeBuildingSecurityPolicy(t *testing.T) {
	t.Parallel()
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions: &stubSessionManager{},
		IdentityClientConfig: identity.WebClientConfig{
			Enabled: true, Provider: identity.ProviderGoogle, APIKey: "public-browser-key",
			AuthDomain: "auth.example; script-src *", ProjectID: "petspotr-test",
		},
	})
	response := requestIdentityClientConfig(t, srv)
	assertIdentityClientConfig(t, response, identity.WebClientConfig{})
	if got := response.Header().Get("Content-Security-Policy"); got != contentSecurityPolicy {
		t.Fatalf("invalid config security policy = %q, want secure disabled policy", got)
	}
}

func requestIdentityClientConfig(t *testing.T, srv http.Handler) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/session/client-config", nil)
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, request)
	return response
}

func assertIdentityClientConfig(t *testing.T, response *httptest.ResponseRecorder, want identity.WebClientConfig) {
	t.Helper()
	var got identity.WebClientConfig
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("client config = %#v, want %#v", got, want)
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(strings.ToLower(value), needle) {
			return true
		}
	}
	return false
}
