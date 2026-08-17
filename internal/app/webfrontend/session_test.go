package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type stubSessionManager struct {
	createToken string
	createTTL   time.Duration
	created     identity.Session
	createErr   error
	verifyToken string
	verified    identity.Principal
	verifyErr   error
}

func (s *stubSessionManager) CreateSession(_ context.Context, idToken string, ttl time.Duration) (identity.Session, error) {
	s.createToken = idToken
	s.createTTL = ttl
	return s.created, s.createErr
}

func (s *stubSessionManager) VerifySession(_ context.Context, sessionCookie string) (identity.Principal, error) {
	s.verifyToken = sessionCookie
	return s.verified, s.verifyErr
}

func TestSessionRoutesRemainAbsentWhenIdentityIsDisabled(t *testing.T) {
	t.Parallel()

	srv := NewServerWithStore(store.NewMemoryStore())
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/v1/session", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s /api/v1/session status = %d, want %d", method, rec.Code, http.StatusNotFound)
		}
	}
}

func TestSessionLoginRequiresCSRFAndCreatesSecureCookie(t *testing.T) {
	t.Parallel()

	manager := &stubSessionManager{created: identity.Session{
		Cookie: "signed-session-cookie",
		Principal: identity.Principal{
			Issuer: "https://securetoken.google.com/petspotr-test", Subject: "user-101",
			Email: "owner@example.com", EmailVerified: true,
		},
	}}
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions:    manager,
		SecureSessionCookie: true,
	})

	csrfReq := httptest.NewRequest(http.MethodGet, "/api/v1/session/csrf", nil)
	csrfRec := httptest.NewRecorder()
	srv.ServeHTTP(csrfRec, csrfReq)
	if csrfRec.Code != http.StatusOK {
		t.Fatalf("GET CSRF status = %d, want %d", csrfRec.Code, http.StatusOK)
	}
	csrfCookie := responseCookie(t, csrfRec, csrfCookieName)
	if !csrfCookie.HttpOnly || !csrfCookie.Secure || csrfCookie.Path != "/" || csrfCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("CSRF cookie = %#v, want HttpOnly Secure Path=/ SameSite=Strict", csrfCookie)
	}
	csrfToken := jsonStringField(t, csrfRec.Body.String(), "csrfToken")
	if csrfToken == "" || csrfToken != csrfCookie.Value {
		t.Fatal("CSRF response token must be non-empty and match its cookie")
	}

	missingCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(`{"idToken":"identity-token"}`))
	missingCSRF.Header.Set("Content-Type", "application/json")
	missingRec := httptest.NewRecorder()
	srv.ServeHTTP(missingRec, missingCSRF)
	if missingRec.Code != http.StatusForbidden || manager.createToken != "" {
		t.Fatalf("login without CSRF status = %d, manager token = %q", missingRec.Code, manager.createToken)
	}

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(`{"idToken":"identity-token"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set(csrfHeaderName, csrfToken)
	loginReq.AddCookie(csrfCookie)
	loginRec := httptest.NewRecorder()
	srv.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusCreated {
		t.Fatalf("POST session status = %d, want %d; body = %s", loginRec.Code, http.StatusCreated, loginRec.Body.String())
	}
	if manager.createToken != "identity-token" || manager.createTTL != sessionLifetime {
		t.Fatalf("CreateSession() token/ttl = %q/%s, want identity-token/%s", manager.createToken, manager.createTTL, sessionLifetime)
	}
	sessionCookie := responseCookie(t, loginRec, secureSessionCookieName)
	if sessionCookie.Value != "signed-session-cookie" || !sessionCookie.HttpOnly || !sessionCookie.Secure ||
		sessionCookie.Path != "/" || sessionCookie.SameSite != http.SameSiteStrictMode || sessionCookie.MaxAge <= 0 {
		t.Fatalf("session cookie = %#v, want bounded secure host cookie", sessionCookie)
	}
	if got := jsonStringField(t, loginRec.Body.String(), "subject"); got != "user-101" {
		t.Fatalf("login subject = %q, want user-101", got)
	}
	if strings.Contains(loginRec.Body.String(), "signed-session-cookie") || strings.Contains(loginRec.Body.String(), "identity-token") {
		t.Fatalf("login response leaked a token: %s", loginRec.Body.String())
	}
}

func TestSessionRoutesRejectInvalidTokensAndHideProviderErrors(t *testing.T) {
	t.Parallel()

	providerErr := errors.New("provider detail must not cross the boundary")
	manager := &stubSessionManager{
		createErr: errors.Join(identity.ErrUnauthenticated, providerErr),
		verifyErr: errors.Join(identity.ErrUnauthenticated, providerErr),
	}
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions:    manager,
		SecureSessionCookie: true,
	})

	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(`{"idToken":"invalid-token"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginReq.Header.Set(csrfHeaderName, csrfToken)
	loginReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	loginRec := httptest.NewRecorder()
	srv.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusUnauthorized || strings.Contains(loginRec.Body.String(), providerErr.Error()) {
		t.Fatalf("invalid login response = %d %q", loginRec.Code, loginRec.Body.String())
	}

	currentReq := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	currentReq.AddCookie(&http.Cookie{Name: secureSessionCookieName, Value: "invalid-session"})
	currentRec := httptest.NewRecorder()
	srv.ServeHTTP(currentRec, currentReq)
	if currentRec.Code != http.StatusUnauthorized || strings.Contains(currentRec.Body.String(), providerErr.Error()) {
		t.Fatalf("invalid current-session response = %d %q", currentRec.Code, currentRec.Body.String())
	}
}

func TestCurrentSessionAndLogoutUseVerifiedSession(t *testing.T) {
	t.Parallel()

	manager := &stubSessionManager{verified: identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "user-202",
		Email: "finder@example.com", EmailVerified: true,
	}}
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions:    manager,
		SecureSessionCookie: true,
	})

	currentReq := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	currentReq.AddCookie(&http.Cookie{Name: secureSessionCookieName, Value: "current-session"})
	currentRec := httptest.NewRecorder()
	srv.ServeHTTP(currentRec, currentReq)
	if currentRec.Code != http.StatusOK || manager.verifyToken != "current-session" {
		t.Fatalf("GET session response = %d %q, verified token = %q", currentRec.Code, currentRec.Body.String(), manager.verifyToken)
	}
	if got := jsonStringField(t, currentRec.Body.String(), "email"); got != "finder@example.com" {
		t.Fatalf("current session email = %q, want finder@example.com", got)
	}

	csrfToken := "abcdef0123456789abcdef0123456789abcdef0123456789"
	logoutReq := httptest.NewRequest(http.MethodDelete, "/api/v1/session", nil)
	logoutReq.Header.Set(csrfHeaderName, csrfToken)
	logoutReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	logoutRec := httptest.NewRecorder()
	srv.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("DELETE session status = %d, want %d", logoutRec.Code, http.StatusNoContent)
	}
	cleared := responseCookie(t, logoutRec, secureSessionCookieName)
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Fatalf("cleared session cookie = %#v, want empty expired cookie", cleared)
	}
}

func TestSessionRoutesRejectUnexpectedMethodsAndBodies(t *testing.T) {
	t.Parallel()

	manager := &stubSessionManager{}
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions: manager, SecureSessionCookie: true,
	})

	methodReq := httptest.NewRequest(http.MethodPut, "/api/v1/session", nil)
	methodRec := httptest.NewRecorder()
	srv.ServeHTTP(methodRec, methodReq)
	if methodRec.Code != http.StatusMethodNotAllowed || methodRec.Header().Get("Allow") != "GET, POST, DELETE" {
		t.Fatalf("PUT session response = %d Allow=%q", methodRec.Code, methodRec.Header().Get("Allow"))
	}

	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	badTypeReq := httptest.NewRequest(http.MethodPost, "/api/v1/session", strings.NewReader(`{"idToken":"token"}`))
	badTypeReq.Header.Set(csrfHeaderName, csrfToken)
	badTypeReq.AddCookie(&http.Cookie{Name: csrfCookieName, Value: csrfToken})
	badTypeRec := httptest.NewRecorder()
	srv.ServeHTTP(badTypeRec, badTypeReq)
	if badTypeRec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("login content type response = %d, want %d", badTypeRec.Code, http.StatusUnsupportedMediaType)
	}
}

func responseCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("response cookie %q not found in %v", name, rec.Result().Cookies())
	return nil
}

func jsonStringField(t *testing.T, body string, field string) string {
	t.Helper()
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("decode JSON response: %v; body = %q", err, body)
	}
	value, _ := decoded[field].(string)
	return value
}
