package webfrontend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFoundPetFinderCanResolveOwnReportAndFailuresDoNotEnumerate(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	reports := foundpet.NewReportService(state, pubsub.NewMemoryPubSub())
	finder := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "finder-status-101",
		Email: "finder@example.com", EmailVerified: true,
	}
	finderRef := domain.PrincipalRef{Issuer: finder.Issuer, Subject: finder.Subject}
	_, err := reports.ReportFoundPet(ctx, foundpet.ReportCommand{
		PetID: "found-status-101", ImageURL: "https://storage.petspotr.io/found.jpg",
		FoundAt: time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC), Location: "Seattle, WA",
		FinderEmail: finder.Email, OwnedBy: &finderRef,
	}, foundpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServerWithOptions(state, ServerOptions{
		FoundPetReporter: reports, IdentitySessions: &stubSessionManager{verified: finder},
	})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	response := performFoundStatusRequest(
		srv, "found-status-101", "resolve-found-status-101", "finder-session", csrfToken,
	)
	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("finder lifecycle response = %d Cache-Control %q; body = %q",
			response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["petId"] != "found-status-101" || result["status"] != "resolved" || result["eventId"] == "" {
		t.Fatalf("finder lifecycle result = %#v", result)
	}
	retry := performFoundStatusRequest(
		srv, "found-status-101", "resolve-found-status-101", "finder-session", csrfToken,
	)
	var retryResult map[string]string
	if retry.Code != http.StatusOK || json.Unmarshal(retry.Body.Bytes(), &retryResult) != nil ||
		retryResult["eventId"] != result["eventId"] {
		t.Fatalf("exact retry = %d %#v; body = %q", retry.Code, retryResult, retry.Body.String())
	}
	conflict := performFoundStatusRequest(
		srv, "found-status-101", "different-operation", "finder-session", csrfToken,
	)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("changed operation status = %d; body = %q", conflict.Code, conflict.Body.String())
	}

	wrongOwnerServer := NewServerWithOptions(state, ServerOptions{
		FoundPetReporter: reports,
		IdentitySessions: &stubSessionManager{verified: identity.Principal{
			Issuer: finder.Issuer, Subject: "different-finder", Email: "different@example.com", EmailVerified: true,
		}},
	})
	wrongOwner := performFoundStatusRequest(
		wrongOwnerServer, "found-status-101", "wrong-owner", "other-session", csrfToken,
	)
	missing := performFoundStatusRequest(
		wrongOwnerServer, "found-status-missing", "missing", "other-session", csrfToken,
	)
	if wrongOwner.Code != http.StatusNotFound || missing.Code != http.StatusNotFound ||
		wrongOwner.Body.String() != missing.Body.String() {
		t.Fatalf("non-enumerating statuses = %d/%q and %d/%q",
			wrongOwner.Code, wrongOwner.Body.String(), missing.Code, missing.Body.String())
	}

	ownerlessReport := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: "found-ownerless", ImageURL: "https://storage.petspotr.io/legacy.jpg",
		FoundAt: time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC), Location: "Tacoma, WA",
	})
	ownerlessRecord, _ := ownerlessReport.Persisted()
	ownerlessData, err := json.Marshal(ownerlessRecord)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.FoundPetsCollection, ownerlessRecord.PetID, ownerlessData); err != nil {
		t.Fatal(err)
	}
	ownerless := performFoundStatusRequest(
		srv, ownerlessRecord.PetID, "ownerless", "finder-session", csrfToken,
	)
	if ownerless.Code != http.StatusNotFound || ownerless.Body.String() != missing.Body.String() {
		t.Fatalf("ownerless response = %d/%q, want missing %d/%q",
			ownerless.Code, ownerless.Body.String(), missing.Code, missing.Body.String())
	}
	if err := state.SaveState(ctx, store.FoundPetsCollection, "found-corrupt", []byte(`{"petId":"found-corrupt"}`)); err != nil {
		t.Fatal(err)
	}
	corrupt := performFoundStatusRequest(srv, "found-corrupt", "corrupt", "finder-session", csrfToken)
	if corrupt.Code != http.StatusNotFound || corrupt.Body.String() != missing.Body.String() {
		t.Fatalf("corrupt response = %d/%q, want missing %d/%q",
			corrupt.Code, corrupt.Body.String(), missing.Code, missing.Body.String())
	}
	unauditedReport := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: "found-unaudited-resolved", ImageURL: "https://storage.petspotr.io/unaudited.jpg",
		FoundAt: time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC), Location: "Tacoma, WA", OwnedBy: &finderRef,
	})
	unauditedResolved, _ := unauditedReport.Persisted()
	unauditedResolved.Status = domain.FoundPetStatusResolved
	unauditedData, err := json.Marshal(unauditedResolved)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.FoundPetsCollection, unauditedResolved.PetID, unauditedData); err != nil {
		t.Fatal(err)
	}
	unaudited := performFoundStatusRequest(
		srv, unauditedResolved.PetID, "unaudited", "finder-session", csrfToken,
	)
	if unaudited.Code != http.StatusNotFound || unaudited.Body.String() != missing.Body.String() {
		t.Fatalf("unaudited resolved response = %d/%q, want missing %d/%q",
			unaudited.Code, unaudited.Body.String(), missing.Code, missing.Body.String())
	}

	disabled := NewServerWithOptions(state, ServerOptions{FoundPetReporter: reports})
	if got := performFoundStatusRequest(disabled, "found-status-101", "disabled", "", ""); got.Code != http.StatusNotFound {
		t.Fatalf("identity-disabled lifecycle = %d, want %d", got.Code, http.StatusNotFound)
	}
}

func TestFoundPetLifecycleHTTPBoundaryRejectsInvalidRequests(t *testing.T) {
	principal := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "finder-boundary",
		Email: "finder@example.com", EmailVerified: true,
	}
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions: &stubSessionManager{verified: principal},
	})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	request := func(method, body, operationID, session, headerCSRF, cookieCSRF string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/found-pets/found-boundary/status", strings.NewReader(body))
		if operationID != "" {
			req.Header.Set(idempotencyKeyHeader, operationID)
		}
		if session != "" {
			req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: session})
		}
		if headerCSRF != "" {
			req.Header.Set(csrfHeaderName, headerCSRF)
		}
		if cookieCSRF != "" {
			req.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: cookieCSRF})
		}
		response := httptest.NewRecorder()
		srv.ServeHTTP(response, req)
		return response
	}

	if got := request(http.MethodPatch, `{"status":"resolved"}`, "anonymous", "", csrfToken, csrfToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want %d", got.Code, http.StatusUnauthorized)
	}
	if got := request(http.MethodPatch, `{"status":"resolved"}`, "bad-csrf", "session", "wrong", csrfToken); got.Code != http.StatusForbidden {
		t.Fatalf("bad CSRF status = %d, want %d", got.Code, http.StatusForbidden)
	}
	if got := request(http.MethodGet, "", "", "session", csrfToken, csrfToken); got.Code != http.StatusMethodNotAllowed || got.Header().Get("Allow") != http.MethodPatch {
		t.Fatalf("wrong method = %d Allow %q", got.Code, got.Header().Get("Allow"))
	}
	for _, tt := range []struct {
		name, body, operationID string
	}{
		{name: "missing idempotency key", body: `{"status":"resolved"}`},
		{name: "padded idempotency key", body: `{"status":"resolved"}`, operationID: " padded "},
		{name: "unsupported status", body: `{"status":"expired"}`, operationID: "expired"},
		{name: "unknown field", body: `{"status":"resolved","finder":"spoofed"}`, operationID: "unknown"},
		{name: "multiple values", body: `{"status":"resolved"}{}`, operationID: "multiple"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := request(http.MethodPatch, tt.body, tt.operationID, "session", csrfToken, csrfToken)
			if got.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %q", got.Code, http.StatusBadRequest, got.Body.String())
			}
		})
	}
}

func TestFoundPetLifecycleHTTPMapsServiceUnavailable(t *testing.T) {
	principal := identity.Principal{Issuer: "issuer", Subject: "finder", EmailVerified: true}
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions:  &stubSessionManager{verified: principal},
		FoundPetLifecycle: foundLifecycleStub{err: foundpet.ErrLifecycleUnavailable},
	})
	response := performFoundStatusRequest(
		srv, "found-unavailable", "unavailable", "session", "0123456789abcdef0123456789abcdef0123456789abcdef",
	)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable status = %d; body = %q", response.Code, response.Body.String())
	}
}

type foundLifecycleStub struct {
	result foundpet.LifecycleResult
	err    error
}

func (s foundLifecycleStub) ResolveFoundPet(context.Context, foundpet.LifecycleCommand) (foundpet.LifecycleResult, error) {
	return s.result, s.err
}

func performFoundStatusRequest(
	handler http.Handler,
	petID, operationID, session, csrfToken string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/found-pets/"+petID+"/status",
		strings.NewReader(`{"status":"resolved"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	if operationID != "" {
		request.Header.Set(idempotencyKeyHeader, operationID)
	}
	if session != "" {
		request.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: session})
	}
	if csrfToken != "" {
		request.Header.Set(csrfHeaderName, csrfToken)
		request.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

var _ FoundPetLifecycle = foundLifecycleStub{}
