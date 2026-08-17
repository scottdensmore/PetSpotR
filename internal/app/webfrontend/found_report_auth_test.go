package webfrontend

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type recordingFoundPetReporter struct {
	calls   int
	command foundpet.ReportCommand
}

func (r *recordingFoundPetReporter) ReportFoundPet(
	_ context.Context,
	command foundpet.ReportCommand,
	_ foundpet.ReportMetadata,
) (foundpet.ReportResult, error) {
	r.calls++
	r.command = command
	return foundpet.ReportResult{PetID: command.PetID, EventID: "event-authenticated"}, nil
}

func TestFoundPetReportRequiresVerifiedSessionWhenIdentityIsEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		manager       *stubSessionManager
		sessionCookie string
		wantCleared   bool
	}{
		{
			name:    "missing session",
			manager: &stubSessionManager{},
		},
		{
			name: "invalid session",
			manager: &stubSessionManager{
				verifyErr: errors.Join(identity.ErrUnauthenticated, errors.New("provider detail")),
			},
			sessionCookie: "expired-session",
			wantCleared:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reporter := &recordingFoundPetReporter{}
			srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
				IdentitySessions: tt.manager,
				FoundPetReporter: reporter,
			})
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/found-pets",
				strings.NewReader(`{"petId":"found-auth","imageUrl":"https://storage.petspotr.io/found.jpg","finderEmail":"spoofed@example.com","location":"Seattle, WA"}`),
			)
			req.Header.Set("Content-Type", "application/json")
			if tt.sessionCookie != "" {
				req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: tt.sessionCookie})
			}
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("POST found report status = %d, want %d; body = %q", rec.Code, http.StatusUnauthorized, rec.Body.String())
			}
			if reporter.calls != 0 {
				t.Fatalf("ReportFoundPet() calls = %d, want 0", reporter.calls)
			}
			if strings.Contains(rec.Body.String(), "provider detail") {
				t.Fatalf("authentication response leaked provider error: %q", rec.Body.String())
			}
			if tt.wantCleared {
				cleared := responseCookie(t, rec, localSessionCookieName)
				if cleared.Value != "" || cleared.MaxAge >= 0 {
					t.Fatalf("cleared session cookie = %#v, want empty expired cookie", cleared)
				}
			}
		})
	}
}

func TestFoundPetReportRequiresCSRFAndUsesVerifiedOwnership(t *testing.T) {
	t.Parallel()

	principal := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "user-404",
		Email: "verified-finder@example.com", EmailVerified: true,
	}
	manager := &stubSessionManager{verified: principal}
	reporter := &recordingFoundPetReporter{}
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions: manager,
		FoundPetReporter: reporter,
	})
	payload := `{"petId":"found-auth","imageUrl":"https://storage.petspotr.io/found.jpg","finderEmail":"spoofed@example.com","location":"Seattle, WA"}`

	missingCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
	missingCSRF.Header.Set("Content-Type", "application/json")
	missingCSRF.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "verified-session"})
	missingCSRFRec := httptest.NewRecorder()
	srv.ServeHTTP(missingCSRFRec, missingCSRF)
	if missingCSRFRec.Code != http.StatusForbidden || reporter.calls != 0 {
		t.Fatalf("POST without CSRF = %d, reporter calls = %d; want %d/0", missingCSRFRec.Code, reporter.calls, http.StatusForbidden)
	}

	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	wrongCSRF := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
	wrongCSRF.Header.Set("Content-Type", "application/json")
	wrongCSRF.Header.Set(csrfHeaderName, "wrong-token")
	wrongCSRF.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "verified-session"})
	wrongCSRF.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	wrongCSRFRec := httptest.NewRecorder()
	srv.ServeHTTP(wrongCSRFRec, wrongCSRF)
	if wrongCSRFRec.Code != http.StatusForbidden || reporter.calls != 0 {
		t.Fatalf("POST with wrong CSRF = %d, reporter calls = %d; want %d/0", wrongCSRFRec.Code, reporter.calls, http.StatusForbidden)
	}

	authorized := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(payload))
	authorized.Header.Set("Content-Type", "application/json")
	authorized.Header.Set(csrfHeaderName, csrfToken)
	authorized.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "verified-session"})
	authorized.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	authorizedRec := httptest.NewRecorder()
	srv.ServeHTTP(authorizedRec, authorized)

	if authorizedRec.Code != http.StatusCreated {
		t.Fatalf("authenticated POST status = %d, want %d; body = %q", authorizedRec.Code, http.StatusCreated, authorizedRec.Body.String())
	}
	if reporter.calls != 1 {
		t.Fatalf("ReportFoundPet() calls = %d, want 1", reporter.calls)
	}
	if reporter.command.FinderEmail != principal.Email {
		t.Fatalf("finder email = %q, want verified email %q", reporter.command.FinderEmail, principal.Email)
	}
	wantOwner := domain.PrincipalRef{Issuer: principal.Issuer, Subject: principal.Subject}
	if reporter.command.OwnedBy == nil || *reporter.command.OwnedBy != wantOwner {
		t.Fatalf("report owner = %#v, want %#v", reporter.command.OwnedBy, wantOwner)
	}
	if manager.verifyToken != "verified-session" {
		t.Fatalf("verified session = %q, want verified-session", manager.verifyToken)
	}
}
