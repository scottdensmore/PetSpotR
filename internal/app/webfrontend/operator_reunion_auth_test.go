package webfrontend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestGlobalOperatorCanResolveConfirmedMatchWithAudit(t *testing.T) {
	state := store.NewMemoryStore()
	operator := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "operator-101",
		Email: "operator@example.com", EmailVerified: true,
	}
	operatorRef := domain.PrincipalRef{Issuer: operator.Issuer, Subject: operator.Subject}
	assignment := grantRoleForTest(t, state, operatorRef, domain.RoleScope{Kind: domain.RoleScopeGlobal})
	matchID := seedMatchForReunion(t, state, "operator", "lost-101", "found-202", domain.MatchStatusConfirmed)

	manager := &stubSessionManager{verified: operator}
	srv := NewServerWithOptions(state, ServerOptions{IdentitySessions: manager})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"

	missingCSRF := performOperatorReunion(srv, matchID, "lost-101", "operator-session", "", "resolve-101")
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d, want %d", missingCSRF.Code, http.StatusForbidden)
	}
	anonymous := performOperatorReunion(srv, matchID, "lost-101", "", csrfToken, "resolve-101")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want %d", anonymous.Code, http.StatusUnauthorized)
	}

	resolved := performOperatorReunion(srv, matchID, "lost-101", "operator-session", csrfToken, "resolve-101")
	if resolved.Code != http.StatusOK {
		t.Fatalf("global operator status = %d, want %d; body = %q", resolved.Code, http.StatusOK, resolved.Body.String())
	}
	match, participants := loadAuthorizedMatch(t, state, matchID)
	if match.Status != domain.MatchStatusReunited {
		t.Fatalf("match status = %q, want %q", match.Status, domain.MatchStatusReunited)
	}
	participantData, err := json.Marshal(participants)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"reunionAudit"`, `"operationId":"resolve-101"`, `"role":"operator"`, `"kind":"global"`} {
		if !strings.Contains(string(participantData), want) {
			t.Fatalf("private participant audit %s does not contain %s", participantData, want)
		}
	}
	auditData, err := json.Marshal(participants.ReunionAudit)
	if err != nil {
		t.Fatal(err)
	}
	for _, privateValue := range []string{operator.Issuer, operator.Subject, operator.Email} {
		if strings.Contains(string(auditData), privateValue) || strings.Contains(resolved.Body.String(), privateValue) {
			t.Fatalf("operator mutation exposed raw identity %q", privateValue)
		}
	}

	exactRetry := performOperatorReunion(srv, matchID, "lost-101", "operator-session", csrfToken, "resolve-101")
	if exactRetry.Code != http.StatusOK {
		t.Fatalf("exact retry status = %d, want %d; body = %q", exactRetry.Code, http.StatusOK, exactRetry.Body.String())
	}
	_, retriedParticipants := loadAuthorizedMatch(t, state, matchID)
	retriedData, err := json.Marshal(retriedParticipants)
	if err != nil {
		t.Fatal(err)
	}
	if string(retriedData) != string(participantData) {
		t.Fatalf("exact retry changed audit: got %s, want %s", retriedData, participantData)
	}
	roleChange := domain.RoleAssignmentChange{
		Target: operatorRef, Role: domain.RoleOperator, Scope: domain.RoleScope{Kind: domain.RoleScopeGlobal},
		Actor:       domain.PrincipalRef{Issuer: operator.Issuer, Subject: "bootstrap-admin"},
		OperationID: "revoke-operator-101",
		OccurredAt:  time.Date(2026, time.August, 20, 13, 0, 0, 0, time.UTC),
	}
	if _, changed, err := state.RevokeRoleAssignment(context.Background(), roleChange); err != nil || !changed {
		t.Fatalf("revoke before exact retry = changed %t, error %v", changed, err)
	}
	roleChange.OperationID = "regrant-operator-101"
	roleChange.OccurredAt = roleChange.OccurredAt.Add(time.Hour)
	if _, changed, err := state.GrantRoleAssignment(context.Background(), roleChange); err != nil || !changed {
		t.Fatalf("regrant before exact retry = changed %t, error %v", changed, err)
	}
	afterRegrantRetry := performOperatorReunion(
		srv, matchID, "lost-101", "operator-session", csrfToken, "resolve-101",
	)
	if afterRegrantRetry.Code != http.StatusOK {
		t.Fatalf("exact retry after regrant status = %d, want %d; body = %q",
			afterRegrantRetry.Code, http.StatusOK, afterRegrantRetry.Body.String())
	}
	_, afterRegrantParticipants := loadAuthorizedMatch(t, state, matchID)
	if afterRegrantParticipants.ReunionAudit == nil ||
		afterRegrantParticipants.ReunionAudit.AssignmentRevision != assignment.Revision {
		t.Fatalf("exact retry after regrant replaced original audit: %#v", afterRegrantParticipants.ReunionAudit)
	}
}

func TestOperatorReunionFailsClosedForUnauthorizedOrInvalidTargets(t *testing.T) {
	issuer := "https://securetoken.google.com/petspotr-test"
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"

	tests := []struct {
		name       string
		principal  identity.Principal
		prepare    func(*testing.T, *store.MemoryStore, domain.PrincipalRef)
		matchID    string
		petID      string
		wantStatus int
	}{
		{
			name:      "ordinary user",
			principal: identity.Principal{Issuer: issuer, Subject: "ordinary-201", Email: "ordinary@example.com", EmailVerified: true},
			matchID:   "match-ordinary", petID: "lost-ordinary", wantStatus: http.StatusForbidden,
		},
		{
			name:      "shelter operator without trusted resource scope",
			principal: identity.Principal{Issuer: issuer, Subject: "shelter-202", Email: "shelter@example.com", EmailVerified: true},
			prepare: func(t *testing.T, state *store.MemoryStore, target domain.PrincipalRef) {
				grantRoleForTest(t, state, target, domain.RoleScope{Kind: domain.RoleScopeShelter, ShelterID: "shelter-1"})
			},
			matchID: "match-shelter", petID: "lost-shelter", wantStatus: http.StatusForbidden,
		},
		{
			name:      "revoked global operator",
			principal: identity.Principal{Issuer: issuer, Subject: "revoked-203", Email: "revoked@example.com", EmailVerified: true},
			prepare: func(t *testing.T, state *store.MemoryStore, target domain.PrincipalRef) {
				grantRoleForTest(t, state, target, domain.RoleScope{Kind: domain.RoleScopeGlobal})
				_, changed, err := state.RevokeRoleAssignment(context.Background(), domain.RoleAssignmentChange{
					Target: target, Role: domain.RoleOperator, Scope: domain.RoleScope{Kind: domain.RoleScopeGlobal},
					Actor:       domain.PrincipalRef{Issuer: issuer, Subject: "bootstrap-admin"},
					OperationID: "revoke-" + target.Subject,
					OccurredAt:  time.Date(2026, time.August, 20, 13, 0, 0, 0, time.UTC),
				})
				if err != nil || !changed {
					t.Fatalf("revoke operator role = changed %t, error %v", changed, err)
				}
			},
			matchID: "match-revoked", petID: "lost-revoked", wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := store.NewMemoryStore()
			ref := domain.PrincipalRef{Issuer: tt.principal.Issuer, Subject: tt.principal.Subject}
			if tt.prepare != nil {
				tt.prepare(t, state, ref)
			}
			matchID := seedMatchForReunion(t, state, tt.matchID, tt.petID, "found-unauthorized", domain.MatchStatusConfirmed)
			srv := NewServerWithOptions(state, ServerOptions{IdentitySessions: &stubSessionManager{verified: tt.principal}})
			response := performOperatorReunion(srv, matchID, tt.petID, "session", csrfToken, "resolve-denied")
			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", response.Code, tt.wantStatus, response.Body.String())
			}
			match, participants := loadAuthorizedMatch(t, state, matchID)
			if match.Status != domain.MatchStatusConfirmed || participants.ReunionAudit != nil {
				t.Fatalf("denied request mutated match: %#v / %#v", match, participants.ReunionAudit)
			}
		})
	}

	operator := identity.Principal{Issuer: issuer, Subject: "operator-204", Email: "operator@example.com", EmailVerified: true}
	state := store.NewMemoryStore()
	grantRoleForTest(t, state, domain.PrincipalRef{Issuer: issuer, Subject: operator.Subject}, domain.RoleScope{Kind: domain.RoleScopeGlobal})
	matchID := seedMatchForReunion(t, state, "invalid", "lost-valid", "found-valid", domain.MatchStatusConfirmed)
	srv := NewServerWithOptions(state, ServerOptions{IdentitySessions: &stubSessionManager{verified: operator}})

	missingKey := performOperatorReunion(srv, matchID, "lost-valid", "session", csrfToken, "")
	if missingKey.Code != http.StatusBadRequest {
		t.Fatalf("missing idempotency key status = %d, want %d", missingKey.Code, http.StatusBadRequest)
	}
	invalidRating := performOperatorReunionWithFeedback(
		srv, matchID, "lost-valid", "session", csrfToken, "resolve-invalid", 0, "Invalid rating",
	)
	if invalidRating.Code != http.StatusBadRequest {
		t.Fatalf("invalid rating status = %d, want %d; body = %q", invalidRating.Code, http.StatusBadRequest, invalidRating.Body.String())
	}
	wrongPet := performOperatorReunion(srv, matchID, "lost-wrong", "session", csrfToken, "resolve-wrong")
	missingMatch := performOperatorReunion(srv, "match-missing", "lost-valid", "session", csrfToken, "resolve-missing")
	for name, response := range map[string]*httptest.ResponseRecorder{"wrong pet": wrongPet, "missing match": missingMatch} {
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d", name, response.Code, http.StatusNotFound)
		}
	}

	resolved := performOperatorReunion(srv, matchID, "lost-valid", "session", csrfToken, "resolve-stable")
	if resolved.Code != http.StatusOK {
		t.Fatalf("initial resolution status = %d, want %d", resolved.Code, http.StatusOK)
	}
	changedRetry := performOperatorReunionWithFeedback(
		srv, matchID, "lost-valid", "session", csrfToken, "resolve-stable", 4, "Changed retry",
	)
	if changedRetry.Code != http.StatusConflict {
		t.Fatalf("changed retry status = %d, want %d; body = %q", changedRetry.Code, http.StatusConflict, changedRetry.Body.String())
	}

	pendingMatchID := seedMatchForReunion(
		t, state, "pending", "lost-pending", "found-pending", domain.MatchStatusPendingReview,
	)
	pending := performOperatorReunion(srv, pendingMatchID, "lost-pending", "session", csrfToken, "resolve-pending")
	if pending.Code != http.StatusConflict {
		t.Fatalf("pending match status = %d, want %d; body = %q", pending.Code, http.StatusConflict, pending.Body.String())
	}
}

func grantRoleForTest(
	t *testing.T,
	roles store.RoleAssignmentStore,
	target domain.PrincipalRef,
	scope domain.RoleScope,
) domain.RoleAssignment {
	t.Helper()
	assignment, changed, err := roles.GrantRoleAssignment(context.Background(), domain.RoleAssignmentChange{
		Target: target,
		Role:   domain.RoleOperator,
		Scope:  scope,
		Actor: domain.PrincipalRef{
			Issuer: "https://securetoken.google.com/petspotr-test", Subject: "bootstrap-admin",
		},
		OperationID: "grant-" + target.Subject,
		OccurredAt:  time.Date(2026, time.August, 20, 12, 0, 0, 0, time.UTC),
	})
	if err != nil || !changed {
		t.Fatalf("grant operator role = changed %t, error %v", changed, err)
	}
	return assignment
}

func seedMatchForReunion(
	t *testing.T,
	state store.StateStore,
	label, lostPetID, foundPetID string,
	status domain.MatchStatus,
) string {
	t.Helper()
	sourceEventID := "event-reunion-" + label
	matchID, err := domain.StableMatchID(sourceEventID, foundPetID, lostPetID)
	if err != nil {
		t.Fatal(err)
	}
	reporter := &domain.PrincipalRef{Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-101"}
	finder := &domain.PrincipalRef{Issuer: reporter.Issuer, Subject: "finder-202"}
	decidedAt := time.Date(2026, time.August, 20, 12, 30, 0, 0, time.UTC)
	match := domain.MatchRecord{
		MatchID: matchID, MatchedPetID: lostPetID, FoundPetID: foundPetID,
		Score: 0.91, Status: status, MatchedAt: decidedAt.Add(-time.Hour),
		Scores: domain.MatchScoreBreakdown{
			Visual: 0.95, Color: 1, Spatial: 0.82, DistanceMiles: 2.7, Threshold: 0.7,
		},
		LostPet:       domain.MatchPetDetail{PetID: lostPetID, Breed: "Golden Retriever"},
		FoundPet:      domain.MatchPetDetail{PetID: foundPetID, Breed: "Golden Retriever"},
		SourceEventID: sourceEventID, Model: "gemma4:e2b", ThresholdVersion: "visual-spatial-v1",
		Explanation: "High visual and geographic similarity",
	}
	participants := domain.MatchParticipantRecord{
		MatchID: matchID, LostPetID: lostPetID, FoundPetID: foundPetID, Reporter: reporter, Finder: finder,
	}
	if status == domain.MatchStatusConfirmed {
		participants.ReporterDecision = domain.MatchDecisionConfirm
		participants.FinderDecision = domain.MatchDecisionConfirm
		participants.DecisionAudit = []domain.MatchDecisionAudit{
			{Role: domain.MatchParticipantRoleReporter, Decision: domain.MatchDecisionConfirm, DecidedAt: decidedAt},
			{Role: domain.MatchParticipantRoleFinder, Decision: domain.MatchDecisionConfirm, DecidedAt: decidedAt},
		}
	}
	matchData, err := json.Marshal(match)
	if err != nil {
		t.Fatal(err)
	}
	participantData, err := json.Marshal(participants)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(context.Background(), store.MatchesCollection, matchID, matchData); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(context.Background(), store.MatchParticipantsCollection, matchID, participantData); err != nil {
		t.Fatal(err)
	}
	return matchID
}

func performOperatorReunion(
	srv http.Handler,
	matchID, petID, session, csrfToken, operationID string,
) *httptest.ResponseRecorder {
	return performOperatorReunionWithFeedback(srv, matchID, petID, session, csrfToken, operationID, 5, "Reunited safely")
}

func performOperatorReunionWithFeedback(
	srv http.Handler,
	matchID, petID, session, csrfToken, operationID string,
	rating int,
	feedback string,
) *httptest.ResponseRecorder {
	payload, err := json.Marshal(ReunionResolveRequest{
		MatchID: matchID, PetID: petID, Rating: rating, Feedback: feedback,
	})
	if err != nil {
		panic(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reunions/resolve", strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", operationID)
	if session != "" {
		request.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: session})
	}
	if csrfToken != "" {
		request.Header.Set(csrfHeaderName, csrfToken)
		request.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	}
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, request)
	return recorder
}
