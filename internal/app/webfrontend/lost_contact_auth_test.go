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

func TestLostPetContactIsVisibleOnlyToTheAuthenticatedOwner(t *testing.T) {
	memory := store.NewMemoryStore()
	legacyServer := NewServerWithStore(memory)
	legacyRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/lost-pets",
		strings.NewReader(`{"petId":"lost-legacy-contact","petName":"Legacy","reporterEmail":"legacy@example.com","location":"Seattle, WA"}`),
	)
	legacyRequest.Header.Set("Content-Type", "application/json")
	legacyRecorder := httptest.NewRecorder()
	legacyServer.ServeHTTP(legacyRecorder, legacyRequest)
	if legacyRecorder.Code != http.StatusCreated {
		t.Fatalf("create legacy report status = %d, want %d; body = %q", legacyRecorder.Code, http.StatusCreated, legacyRecorder.Body.String())
	}

	owner := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-505",
		Email: "verified-owner@example.com", EmailVerified: true,
	}
	manager := &stubSessionManager{verified: owner}
	srv := NewServerWithOptions(memory, ServerOptions{IdentitySessions: manager})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	create := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/lost-pets",
		strings.NewReader(`{"petId":"lost-private-contact","petName":"Buddy","reporterEmail":"spoofed@example.com","phone":"(555) 010-0300","location":"Seattle, WA"}`),
	)
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set(csrfHeaderName, csrfToken)
	create.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "owner-session"})
	create.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	createRecorder := httptest.NewRecorder()
	srv.ServeHTTP(createRecorder, create)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create owned report status = %d, want %d; body = %q", createRecorder.Code, http.StatusCreated, createRecorder.Body.String())
	}

	contactPath := "/api/v1/lost-pets/lost-private-contact/contact"
	ownerRecorder := requestLostContact(t, srv, contactPath, "owner-session")
	if ownerRecorder.Code != http.StatusOK {
		t.Fatalf("owner contact status = %d, want %d; body = %q", ownerRecorder.Code, http.StatusOK, ownerRecorder.Body.String())
	}
	if ownerRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("owner contact Cache-Control = %q, want no-store", ownerRecorder.Header().Get("Cache-Control"))
	}
	var contact struct {
		Email string `json:"email"`
		Phone string `json:"phone"`
	}
	if err := json.Unmarshal(ownerRecorder.Body.Bytes(), &contact); err != nil {
		t.Fatalf("decode owner contact: %v", err)
	}
	if contact.Email != owner.Email || contact.Phone != "(555) 010-0300" {
		t.Fatalf("owner contact = %#v, want verified email and stored phone", contact)
	}
	for _, privateValue := range []string{owner.Issuer, owner.Subject, "identityRef", "ownerIdentityRef"} {
		if strings.Contains(ownerRecorder.Body.String(), privateValue) {
			t.Fatalf("contact response exposed private identity value %q: %s", privateValue, ownerRecorder.Body.String())
		}
	}

	manager.verified.Subject = "wrong-owner"
	wrongOwner := requestLostContact(t, srv, contactPath, "wrong-owner-session")
	missing := requestLostContact(t, srv, "/api/v1/lost-pets/lost-missing/contact", "wrong-owner-session")
	legacy := requestLostContact(t, srv, "/api/v1/lost-pets/lost-legacy-contact/contact", "wrong-owner-session")
	for name, recorder := range map[string]*httptest.ResponseRecorder{
		"wrong owner": wrongOwner,
		"missing":     missing,
		"legacy":      legacy,
	} {
		if recorder.Code != http.StatusNotFound || recorder.Body.String() != wrongOwner.Body.String() {
			t.Fatalf("%s response = %d %q, want non-enumerating %d %q", name, recorder.Code, recorder.Body.String(), http.StatusNotFound, wrongOwner.Body.String())
		}
	}

	anonymous := requestLostContact(t, srv, contactPath, "")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous contact status = %d, want %d", anonymous.Code, http.StatusUnauthorized)
	}
	disabled := requestLostContact(t, legacyServer, contactPath, "owner-session")
	if disabled.Code != http.StatusNotFound {
		t.Fatalf("identity-disabled contact status = %d, want %d", disabled.Code, http.StatusNotFound)
	}
}

func requestLostContact(t *testing.T, srv *Server, path, session string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if session != "" {
		req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: session})
	}
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, req)
	return recorder
}
