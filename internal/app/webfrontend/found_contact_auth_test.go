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

func TestFoundPetContactIsVisibleOnlyToTheAuthenticatedOwner(t *testing.T) {
	memory := store.NewMemoryStore()
	legacyServer := NewServerWithStore(memory)
	legacyRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/found-pets",
		strings.NewReader(`{"petId":"found-legacy-contact","imageUrl":"https://storage.petspotr.io/legacy.jpg","finderEmail":"legacy@example.com","location":"Seattle, WA"}`),
	)
	legacyRequest.Header.Set("Content-Type", "application/json")
	legacyRecorder := httptest.NewRecorder()
	legacyServer.ServeHTTP(legacyRecorder, legacyRequest)
	if legacyRecorder.Code != http.StatusCreated {
		t.Fatalf("create legacy report status = %d, want %d; body = %q", legacyRecorder.Code, http.StatusCreated, legacyRecorder.Body.String())
	}

	owner := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "finder-606",
		Email: "verified-finder@example.com", EmailVerified: true,
	}
	manager := &stubSessionManager{verified: owner}
	srv := NewServerWithOptions(memory, ServerOptions{IdentitySessions: manager})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	create := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/found-pets",
		strings.NewReader(`{"petId":"found-private-contact","imageUrl":"https://storage.petspotr.io/found.jpg","finderEmail":"spoofed@example.com","location":"Seattle, WA"}`),
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

	contactPath := "/api/v1/found-pets/found-private-contact/contact"
	ownerRecorder := requestFoundContact(t, srv, contactPath, "owner-session")
	if ownerRecorder.Code != http.StatusOK {
		t.Fatalf("owner contact status = %d, want %d; body = %q", ownerRecorder.Code, http.StatusOK, ownerRecorder.Body.String())
	}
	if ownerRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("owner contact Cache-Control = %q, want no-store", ownerRecorder.Header().Get("Cache-Control"))
	}
	var contact map[string]string
	if err := json.Unmarshal(ownerRecorder.Body.Bytes(), &contact); err != nil {
		t.Fatalf("decode owner contact: %v", err)
	}
	if contact["email"] != owner.Email || contact["phone"] != "" {
		t.Fatalf("owner contact = %#v, want verified email without phone", contact)
	}
	for _, privateValue := range []string{owner.Issuer, owner.Subject, "identityRef", "finderIdentityRef"} {
		if strings.Contains(ownerRecorder.Body.String(), privateValue) {
			t.Fatalf("contact response exposed private identity value %q: %s", privateValue, ownerRecorder.Body.String())
		}
	}

	manager.verified.Subject = "wrong-owner"
	wrongOwner := requestFoundContact(t, srv, contactPath, "wrong-owner-session")
	missing := requestFoundContact(t, srv, "/api/v1/found-pets/found-missing/contact", "wrong-owner-session")
	legacy := requestFoundContact(t, srv, "/api/v1/found-pets/found-legacy-contact/contact", "wrong-owner-session")
	for name, recorder := range map[string]*httptest.ResponseRecorder{
		"wrong owner": wrongOwner,
		"missing":     missing,
		"legacy":      legacy,
	} {
		if recorder.Code != http.StatusNotFound || recorder.Body.String() != wrongOwner.Body.String() {
			t.Fatalf("%s response = %d %q, want non-enumerating %d %q", name, recorder.Code, recorder.Body.String(), http.StatusNotFound, wrongOwner.Body.String())
		}
	}

	anonymous := requestFoundContact(t, srv, contactPath, "")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous contact status = %d, want %d", anonymous.Code, http.StatusUnauthorized)
	}
	disabled := requestFoundContact(t, legacyServer, contactPath, "owner-session")
	if disabled.Code != http.StatusNotFound {
		t.Fatalf("identity-disabled contact status = %d, want %d", disabled.Code, http.StatusNotFound)
	}
}

func requestFoundContact(t *testing.T, srv *Server, path, session string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if session != "" {
		req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: session})
	}
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, req)
	return recorder
}
