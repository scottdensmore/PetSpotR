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
	ownerRef := domain.PrincipalRef{Issuer: owner.Issuer, Subject: owner.Subject}
	_, err := reports.ReportLostPet(ctx, lostpet.ReportCommand{
		PetID: "lost-status-101", ReporterEmail: owner.Email,
		ReportedAt: time.Date(2026, time.August, 20, 17, 0, 0, 0, time.UTC),
		Location:   "Seattle, WA",
		OwnedBy:    &ownerRef,
	}, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	grantRoleForTest(t, state, ownerRef, domain.RoleScope{Kind: domain.RoleScopeGlobal})
	_, changed, err := state.RevokeRoleAssignment(ctx, domain.RoleAssignmentChange{
		Target: ownerRef, Role: domain.RoleOperator, Scope: domain.RoleScope{Kind: domain.RoleScopeGlobal},
		Actor:       domain.PrincipalRef{Issuer: owner.Issuer, Subject: "bootstrap-admin"},
		OperationID: "revoke-owner-status-101",
		OccurredAt:  time.Date(2026, time.August, 20, 13, 0, 0, 0, time.UTC),
	})
	if err != nil || !changed {
		t.Fatalf("revoke owner's operator role = changed %t, error %v", changed, err)
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

func TestGlobalOperatorCanMarkLostReportReunitedWithPinnedRoleAudit(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	broker := pubsub.NewMemoryPubSub()
	reports := lostpet.NewService(state, broker)
	owner := domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-operator-target",
	}
	operator := identity.Principal{
		Issuer: owner.Issuer, Subject: "global-lost-operator", Email: "operator@example.com", EmailVerified: true,
	}
	operatorRef := domain.PrincipalRef{Issuer: operator.Issuer, Subject: operator.Subject}
	assignment := grantRoleForTest(t, state, operatorRef, domain.RoleScope{Kind: domain.RoleScopeGlobal})
	_, err := reports.ReportLostPet(ctx, lostpet.ReportCommand{
		PetID: "lost-operator-status", ReporterEmail: "owner@example.com",
		ReportedAt: time.Date(2026, time.August, 20, 18, 0, 0, 0, time.UTC),
		Location:   "Seattle, WA", OwnedBy: &owner,
	}, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = reports.ReportLostPet(ctx, lostpet.ReportCommand{
		PetID: "lost-operator-revoked", ReporterEmail: "owner@example.com",
		ReportedAt: time.Date(2026, time.August, 20, 18, 5, 0, 0, time.UTC),
		Location:   "Seattle, WA", OwnedBy: &owner,
	}, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}

	srv := NewServerWithOptions(state, ServerOptions{
		LostPetReporter: reports, IdentitySessions: &stubSessionManager{verified: operator},
	})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	response := performLostStatusRequest(
		srv, "lost-operator-status", "operator-reunite-lost", "operator-session", csrfToken,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("global operator lifecycle status = %d, want %d; body = %q",
			response.Code, http.StatusOK, response.Body.String())
	}

	recordData, err := state.GetState(ctx, store.LostPetsCollection, "lost-operator-status")
	if err != nil {
		t.Fatal(err)
	}
	var persisted map[string]any
	if err := json.Unmarshal(recordData, &persisted); err != nil {
		t.Fatal(err)
	}
	audit, ok := persisted["lifecycleAudit"].(map[string]any)
	if !ok || audit["authorizedAs"] != "operator" || audit["assignmentId"] != assignment.AssignmentID ||
		audit["assignmentRevision"] != float64(assignment.Revision) {
		t.Fatalf("operator lifecycle audit = %#v", persisted["lifecycleAudit"])
	}
	auditData, err := json.Marshal(audit)
	if err != nil {
		t.Fatal(err)
	}
	for _, privateValue := range []string{operator.Issuer, operator.Subject, operator.Email} {
		if strings.Contains(string(auditData), privateValue) || strings.Contains(response.Body.String(), privateValue) {
			t.Fatalf("operator lifecycle exposed raw identity %q", privateValue)
		}
	}
	legacyReport := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: "lost-operator-ownerless", ReporterEmail: "legacy@example.com",
		ReportedAt: time.Date(2026, time.August, 20, 18, 10, 0, 0, time.UTC),
		Location:   "Tacoma, WA",
	})
	legacyRecord, _ := legacyReport.Persisted()
	legacyData, err := json.Marshal(legacyRecord)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.LostPetsCollection, legacyRecord.PetID, legacyData); err != nil {
		t.Fatal(err)
	}
	legacyResponse := performLostStatusRequest(
		srv, legacyRecord.PetID, "operator-reunite-ownerless", "operator-session", csrfToken,
	)
	if legacyResponse.Code != http.StatusOK {
		t.Fatalf("ownerless operator lifecycle status = %d, want %d; body = %q",
			legacyResponse.Code, http.StatusOK, legacyResponse.Body.String())
	}
	legacyData, err = state.GetState(ctx, store.LostPetsCollection, legacyRecord.PetID)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(legacyData, &legacyRecord); err != nil || legacyRecord.LifecycleAudit == nil ||
		legacyRecord.LifecycleAudit.AuthorizedAs != domain.LostPetLifecycleAuthorizationOperator ||
		legacyRecord.LifecycleAudit.AssignmentID != assignment.AssignmentID {
		t.Fatalf("ownerless operator lifecycle audit = %#v, %v", legacyRecord.LifecycleAudit, err)
	}
	changedOperation := performLostStatusRequest(
		srv, "lost-operator-status", "operator-reunite-changed", "operator-session", csrfToken,
	)
	if changedOperation.Code != http.StatusConflict {
		t.Fatalf("changed operator operation status = %d, want %d; body = %q",
			changedOperation.Code, http.StatusConflict, changedOperation.Body.String())
	}

	_, changed, err := state.RevokeRoleAssignment(ctx, domain.RoleAssignmentChange{
		Target: operatorRef, Role: domain.RoleOperator, Scope: domain.RoleScope{Kind: domain.RoleScopeGlobal},
		Actor:       domain.PrincipalRef{Issuer: owner.Issuer, Subject: "bootstrap-admin"},
		OperationID: "revoke-global-lost-operator",
		OccurredAt:  time.Date(2026, time.August, 20, 19, 0, 0, 0, time.UTC),
	})
	if err != nil || !changed {
		t.Fatalf("revoke global operator = changed %t, error %v", changed, err)
	}
	revokedRetry := performLostStatusRequest(
		srv, "lost-operator-status", "operator-reunite-lost", "operator-session", csrfToken,
	)
	if revokedRetry.Code != http.StatusNotFound {
		t.Fatalf("revoked exact retry status = %d, want %d; body = %q",
			revokedRetry.Code, http.StatusNotFound, revokedRetry.Body.String())
	}
	revokedNewTarget := performLostStatusRequest(
		srv, "lost-operator-revoked", "operator-reunite-revoked", "operator-session", csrfToken,
	)
	if revokedNewTarget.Code != http.StatusNotFound || revokedNewTarget.Body.String() != revokedRetry.Body.String() {
		t.Fatalf("revoked new target status = %d, want non-enumerating 404; body = %q",
			revokedNewTarget.Code, revokedNewTarget.Body.String())
	}
	revokedRecordData, err := state.GetState(ctx, store.LostPetsCollection, "lost-operator-revoked")
	if err != nil {
		t.Fatal(err)
	}
	var revokedRecord domain.LostPetRecord
	if err := json.Unmarshal(revokedRecordData, &revokedRecord); err != nil ||
		revokedRecord.Status != domain.LostPetStatusLost || revokedRecord.LifecycleAudit != nil {
		t.Fatalf("revoked operator mutated target = %#v, %v", revokedRecord, err)
	}

	regranted, changed, err := state.GrantRoleAssignment(ctx, domain.RoleAssignmentChange{
		Target: operatorRef, Role: domain.RoleOperator, Scope: domain.RoleScope{Kind: domain.RoleScopeGlobal},
		Actor:       domain.PrincipalRef{Issuer: owner.Issuer, Subject: "bootstrap-admin"},
		OperationID: "regrant-global-lost-operator",
		OccurredAt:  time.Date(2026, time.August, 20, 20, 0, 0, 0, time.UTC),
	})
	if err != nil || !changed || regranted.Revision == assignment.Revision {
		t.Fatalf("regrant global operator = %#v, changed %t, error %v", regranted, changed, err)
	}
	regrantedRetry := performLostStatusRequest(
		srv, "lost-operator-status", "operator-reunite-lost", "operator-session", csrfToken,
	)
	if regrantedRetry.Code != http.StatusOK {
		t.Fatalf("regranted exact retry status = %d, want %d; body = %q",
			regrantedRetry.Code, http.StatusOK, regrantedRetry.Body.String())
	}
	stableData, err := state.GetState(ctx, store.LostPetsCollection, "lost-operator-status")
	if err != nil {
		t.Fatal(err)
	}
	if string(stableData) != string(recordData) {
		t.Fatalf("exact retry after regrant changed original audit:\n got %s\nwant %s", stableData, recordData)
	}

	corruptReport := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID: "lost-operator-corrupt", ReporterEmail: "legacy@example.com",
		ReportedAt: time.Date(2026, time.August, 20, 20, 5, 0, 0, time.UTC),
	})
	corruptRecord, _ := corruptReport.Persisted()
	corruptRecord.OwnerIdentityRef = "contact_spliced"
	corruptData, err := json.Marshal(corruptRecord)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.LostPetsCollection, corruptRecord.PetID, corruptData); err != nil {
		t.Fatal(err)
	}
	corrupt := performLostStatusRequest(
		srv, corruptRecord.PetID, "operator-corrupt", "operator-session", csrfToken,
	)
	missing := performLostStatusRequest(
		srv, "lost-operator-missing", "operator-missing", "operator-session", csrfToken,
	)
	if corrupt.Code != http.StatusNotFound || missing.Code != http.StatusNotFound ||
		corrupt.Body.String() != missing.Body.String() {
		t.Fatalf("corrupt/missing lifecycle = %d/%q and %d/%q",
			corrupt.Code, corrupt.Body.String(), missing.Code, missing.Body.String())
	}
	unchangedCorrupt, err := state.GetState(ctx, store.LostPetsCollection, corruptRecord.PetID)
	if err != nil || string(unchangedCorrupt) != string(corruptData) {
		t.Fatalf("corrupt lifecycle target changed: %s, %v", unchangedCorrupt, err)
	}
}

func TestLostPetLifecycleHidesShelterScopedOperator(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	reports := lostpet.NewService(state, pubsub.NewMemoryPubSub())
	issuer := "https://securetoken.google.com/petspotr-test"
	owner := domain.PrincipalRef{Issuer: issuer, Subject: "shelter-target-owner"}
	operator := identity.Principal{
		Issuer: issuer, Subject: "shelter-only-operator", Email: "shelter@example.com", EmailVerified: true,
	}
	_, err := reports.ReportLostPet(ctx, lostpet.ReportCommand{
		PetID: "lost-shelter-scope", ReporterEmail: "owner@example.com",
		ReportedAt: time.Date(2026, time.August, 20, 18, 0, 0, 0, time.UTC), OwnedBy: &owner,
	}, lostpet.ReportMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	grantRoleForTest(t, state, domain.PrincipalRef{Issuer: issuer, Subject: operator.Subject}, domain.RoleScope{
		Kind: domain.RoleScopeShelter, ShelterID: "shelter-101",
	})
	srv := NewServerWithOptions(state, ServerOptions{
		LostPetReporter: reports, IdentitySessions: &stubSessionManager{verified: operator},
	})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	denied := performLostStatusRequest(srv, "lost-shelter-scope", "shelter-denied", "session", csrfToken)
	missing := performLostStatusRequest(srv, "lost-shelter-missing", "shelter-missing", "session", csrfToken)
	if denied.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || denied.Body.String() != missing.Body.String() {
		t.Fatalf("shelter/missing lifecycle = %d/%q and %d/%q",
			denied.Code, denied.Body.String(), missing.Code, missing.Body.String())
	}
	recordData, err := state.GetState(ctx, store.LostPetsCollection, "lost-shelter-scope")
	if err != nil {
		t.Fatal(err)
	}
	var record domain.LostPetRecord
	if err := json.Unmarshal(recordData, &record); err != nil ||
		record.Status != domain.LostPetStatusLost || record.LifecycleAudit != nil {
		t.Fatalf("shelter-only operator mutated target = %#v, %v", record, err)
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
