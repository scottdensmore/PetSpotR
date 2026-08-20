package webfrontend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestLostPetOwnerCanMarkOwnReportReunitedWithDurableEvent(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	broker := pubsub.NewMemoryPubSub()
	published := make(chan []byte, 1)
	if err := broker.Subscribe("petStatusChanged", func(_ context.Context, data []byte) error {
		published <- append([]byte(nil), data...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reports := lostpet.NewService(state, broker)
	owner := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-status-101",
		Email: "owner@example.com", EmailVerified: true,
	}
	_, err := reports.ReportLostPet(ctx, lostpet.ReportCommand{
		PetID: "lost-status-101", ReporterEmail: owner.Email,
		ReportedAt: time.Date(2026, time.August, 20, 17, 0, 0, 0, time.UTC),
		Location:   "Seattle, WA",
		OwnedBy:    &domain.PrincipalRef{Issuer: owner.Issuer, Subject: owner.Subject},
	}, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServerWithOptions(state, ServerOptions{
		LostPetReporter:  reports,
		IdentitySessions: &stubSessionManager{verified: owner},
	})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/v1/lost-pets/lost-status-101/status",
		strings.NewReader(`{"status":"reunited"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "reunite-lost-status-101")
	req.Header.Set(csrfHeaderName, csrfToken)
	req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "owner-session"})
	req.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("owner lifecycle response = %d Cache-Control %q; body = %q",
			rec.Code, rec.Header().Get("Cache-Control"), rec.Body.String())
	}
	var response map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["petId"] != "lost-status-101" || response["status"] != "reunited" || response["eventId"] == "" {
		t.Fatalf("owner lifecycle result = %#v", response)
	}

	recordData, err := state.GetState(ctx, store.LostPetsCollection, "lost-status-101")
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(recordData, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted["status"] != "reunited" {
		t.Fatalf("persisted status = %#v", persisted["status"])
	}
	audit, ok := persisted["lifecycleAudit"].(map[string]any)
	if !ok || audit["operationId"] != "reunite-lost-status-101" || audit["authorizedAs"] != "owner" ||
		audit["eventId"] != response["eventId"] {
		t.Fatalf("persisted lifecycle audit = %#v", persisted["lifecycleAudit"])
	}
	auditData, err := json.Marshal(audit)
	if err != nil {
		t.Fatal(err)
	}
	for _, privateValue := range []string{owner.Issuer, owner.Subject, owner.Email} {
		if strings.Contains(string(auditData), privateValue) || strings.Contains(rec.Body.String(), privateValue) {
			t.Fatalf("lifecycle boundary exposed raw identity %q", privateValue)
		}
	}

	outboxRecord, err := outbox.GetRecord(ctx, state, response["eventId"])
	if err != nil {
		t.Fatal(err)
	}
	if outboxRecord.Topic != "petStatusChanged" {
		t.Fatalf("status outbox topic = %q", outboxRecord.Topic)
	}
	select {
	case event := <-published:
		if strings.Contains(string(event), owner.Email) || strings.Contains(string(event), owner.Subject) ||
			!strings.Contains(string(event), `"type":"petspotr.pet.status-changed"`) {
			t.Fatalf("published status event = %s", event)
		}
	case <-time.After(time.Second):
		t.Fatal("petStatusChanged event was not published")
	}

	retry := performLostStatusRequest(srv, "lost-status-101", "reunite-lost-status-101", "owner-session", csrfToken)
	if retry.Code != http.StatusOK {
		t.Fatalf("exact retry status = %d; body = %q", retry.Code, retry.Body.String())
	}
	var retryResult map[string]string
	if err := json.Unmarshal(retry.Body.Bytes(), &retryResult); err != nil {
		t.Fatal(err)
	}
	if retryResult["eventId"] != response["eventId"] {
		t.Fatalf("retry event ID = %q, want %q", retryResult["eventId"], response["eventId"])
	}
	select {
	case duplicate := <-published:
		t.Fatalf("exact retry republished event: %s", duplicate)
	default:
	}

	conflict := performLostStatusRequest(srv, "lost-status-101", "different-operation", "owner-session", csrfToken)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("changed operation status = %d, want %d; body = %q", conflict.Code, http.StatusConflict, conflict.Body.String())
	}
	wrongOwnerServer := NewServerWithOptions(state, ServerOptions{
		LostPetReporter: reports,
		IdentitySessions: &stubSessionManager{verified: identity.Principal{
			Issuer: owner.Issuer, Subject: "different-owner", Email: "different@example.com", EmailVerified: true,
		}},
	})
	wrongOwner := performLostStatusRequest(wrongOwnerServer, "lost-status-101", "wrong-owner-operation", "other-session", csrfToken)
	missing := performLostStatusRequest(wrongOwnerServer, "lost-status-missing", "missing-operation", "other-session", csrfToken)
	if wrongOwner.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || wrongOwner.Body.String() != missing.Body.String() {
		t.Fatalf("non-enumerating statuses = %d/%q and %d/%q", wrongOwner.Code, wrongOwner.Body.String(), missing.Code, missing.Body.String())
	}
	disabled := NewServerWithOptions(state, ServerOptions{LostPetReporter: reports})
	if response := performLostStatusRequest(disabled, "lost-status-101", "disabled-operation", "", csrfToken); response.Code != http.StatusNotFound {
		t.Fatalf("identity-disabled status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestLostPetLifecycleHTTPBoundaryRejectsInvalidRequests(t *testing.T) {
	principal := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-boundary",
		Email: "owner@example.com", EmailVerified: true,
	}
	srv := NewServerWithOptions(store.NewMemoryStore(), ServerOptions{
		IdentitySessions: &stubSessionManager{verified: principal},
	})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	request := func(method, body, operationID, session, headerCSRF, cookieCSRF string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/v1/lost-pets/lost-boundary/status", strings.NewReader(body))
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

	if got := request(http.MethodPatch, `{"status":"reunited"}`, "anonymous", "", csrfToken, csrfToken); got.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want %d", got.Code, http.StatusUnauthorized)
	}
	if got := request(http.MethodPatch, `{"status":"reunited"}`, "bad-csrf", "session", "wrong", csrfToken); got.Code != http.StatusForbidden {
		t.Fatalf("bad CSRF status = %d, want %d", got.Code, http.StatusForbidden)
	}
	if got := request(http.MethodGet, "", "", "session", csrfToken, csrfToken); got.Code != http.StatusMethodNotAllowed || got.Header().Get("Allow") != http.MethodPatch {
		t.Fatalf("wrong method = %d Allow %q", got.Code, got.Header().Get("Allow"))
	}
	tests := []struct {
		name, body, operationID string
	}{
		{name: "missing idempotency key", body: `{"status":"reunited"}`},
		{name: "unsupported status", body: `{"status":"closed"}`, operationID: "closed"},
		{name: "unknown field", body: `{"status":"reunited","owner":"spoofed"}`, operationID: "unknown"},
		{name: "multiple values", body: `{"status":"reunited"}{}`, operationID: "multiple"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := request(http.MethodPatch, tt.body, tt.operationID, "session", csrfToken, csrfToken)
			if got.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %q", got.Code, http.StatusBadRequest, got.Body.String())
			}
		})
	}
}

func performLostStatusRequest(
	handler http.Handler,
	petID, operationID, session, csrfToken string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/lost-pets/"+petID+"/status", strings.NewReader(`{"status":"reunited"}`))
	request.Header.Set("Content-Type", "application/json")
	if operationID != "" {
		request.Header.Set("Idempotency-Key", operationID)
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
