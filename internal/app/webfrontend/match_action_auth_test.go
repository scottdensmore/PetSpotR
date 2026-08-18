package webfrontend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMatchActionRequiresParticipantsAndPersistsImmutableDecisions(t *testing.T) {
	state := store.NewMemoryStore()
	reporter := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-101",
		Email: "reporter@example.com", EmailVerified: true,
	}
	finder := identity.Principal{
		Issuer: reporter.Issuer, Subject: "finder-202", Email: "finder@example.com", EmailVerified: true,
	}
	reporterRef := domain.PrincipalRef{Issuer: reporter.Issuer, Subject: reporter.Subject}
	finderRef := domain.PrincipalRef{Issuer: finder.Issuer, Subject: finder.Subject}
	seedAuthorizedMatch(t, state, "match-shared", "lost-101", "found-202", &reporterRef, &finderRef)
	seedAuthorizedMatch(t, state, "match-rejected", "lost-303", "found-404", &reporterRef, &finderRef)
	seedAuthorizedMatch(t, state, "match-incomplete", "lost-505", "found-606", &reporterRef, nil)

	manager := &stubSessionManager{verified: reporter}
	srv := NewServerWithOptions(state, ServerOptions{IdentitySessions: manager})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"

	missingCSRF := performMatchAction(srv, "match-shared", "confirm", "reporter-session", "")
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d, want %d", missingCSRF.Code, http.StatusForbidden)
	}
	anonymous := performMatchAction(srv, "match-shared", "confirm", "", csrfToken)
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want %d", anonymous.Code, http.StatusUnauthorized)
	}

	reporterConfirmation := performMatchAction(srv, "match-shared", "confirm", "reporter-session", csrfToken)
	assertMatchActionResponse(t, reporterConfirmation, http.StatusOK, domain.MatchStatusPendingReview)
	match, participants := loadAuthorizedMatch(t, state, "match-shared")
	if match.Status != domain.MatchStatusPendingReview || participants.ReporterDecision != domain.MatchDecisionConfirm ||
		participants.FinderDecision != "" || len(participants.DecisionAudit) != 1 {
		t.Fatalf("after reporter confirmation = %#v / %#v", match, participants)
	}
	if strings.Contains(reporterConfirmation.Body.String(), reporter.Subject) ||
		strings.Contains(reporterConfirmation.Body.String(), finder.Subject) {
		t.Fatalf("public response exposed private participants: %s", reporterConfirmation.Body.String())
	}

	exactRetry := performMatchAction(srv, "match-shared", "confirm", "reporter-session", csrfToken)
	assertMatchActionResponse(t, exactRetry, http.StatusOK, domain.MatchStatusPendingReview)
	_, participants = loadAuthorizedMatch(t, state, "match-shared")
	if len(participants.DecisionAudit) != 1 {
		t.Fatalf("exact retry audit count = %d, want 1", len(participants.DecisionAudit))
	}

	conflict := performMatchAction(srv, "match-shared", "reject", "reporter-session", csrfToken)
	if conflict.Code != http.StatusConflict {
		t.Fatalf("changed decision status = %d, want %d; body = %q", conflict.Code, http.StatusConflict, conflict.Body.String())
	}

	manager.verified = finder
	finderConfirmation := performMatchAction(srv, "match-shared", "confirm", "finder-session", csrfToken)
	assertMatchActionResponse(t, finderConfirmation, http.StatusOK, domain.MatchStatusConfirmed)
	match, participants = loadAuthorizedMatch(t, state, "match-shared")
	if match.Status != domain.MatchStatusConfirmed || participants.FinderDecision != domain.MatchDecisionConfirm ||
		len(participants.DecisionAudit) != 2 {
		t.Fatalf("after finder confirmation = %#v / %#v", match, participants)
	}

	rejection := performMatchAction(srv, "match-rejected", "reject", "finder-session", csrfToken)
	assertMatchActionResponse(t, rejection, http.StatusOK, domain.MatchStatusRejected)
	match, participants = loadAuthorizedMatch(t, state, "match-rejected")
	if match.Status != domain.MatchStatusRejected || participants.FinderDecision != domain.MatchDecisionReject ||
		len(participants.DecisionAudit) != 1 {
		t.Fatalf("after rejection = %#v / %#v", match, participants)
	}

	manager.verified.Subject = "stranger-303"
	wrongOwner := performMatchAction(srv, "match-shared", "confirm", "stranger-session", csrfToken)
	missing := performMatchAction(srv, "match-missing", "confirm", "stranger-session", csrfToken)
	incomplete := performMatchAction(srv, "match-incomplete", "confirm", "stranger-session", csrfToken)
	for name, response := range map[string]*httptest.ResponseRecorder{
		"wrong owner": wrongOwner, "missing": missing, "incomplete": incomplete,
	} {
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want %d; body = %q", name, response.Code, http.StatusNotFound, response.Body.String())
		}
	}
}

func seedAuthorizedMatch(
	t *testing.T,
	state store.StateStore,
	matchID, lostPetID, foundPetID string,
	reporter, finder *domain.PrincipalRef,
) {
	t.Helper()
	matchData, err := json.Marshal(domain.MatchRecord{
		MatchID: matchID, MatchedPetID: lostPetID, FoundPetID: foundPetID, Status: domain.MatchStatusPendingReview,
	})
	if err != nil {
		t.Fatal(err)
	}
	participantsData, err := json.Marshal(domain.MatchParticipantRecord{
		MatchID: matchID, LostPetID: lostPetID, FoundPetID: foundPetID, Reporter: reporter, Finder: finder,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(context.Background(), store.MatchesCollection, matchID, matchData); err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(context.Background(), store.MatchParticipantsCollection, matchID, participantsData); err != nil {
		t.Fatal(err)
	}
}

func performMatchAction(srv http.Handler, matchID, action, session, csrfToken string) *httptest.ResponseRecorder {
	payload := `{"matchId":"` + matchID + `","action":"` + action + `"}`
	request := httptest.NewRequest(http.MethodPost, "/api/v1/matches/action", strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	if session != "" {
		request.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: session})
	}
	if csrfToken != "" {
		request.Header.Set(csrfHeaderName, csrfToken)
		request.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	}
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, request)
	return response
}

func assertMatchActionResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	wantCode int,
	wantStatus domain.MatchStatus,
) {
	t.Helper()
	if response.Code != wantCode || response.Header().Get("Cache-Control") != "no-store" ||
		!strings.Contains(response.Body.String(), string(wantStatus)) {
		t.Fatalf("match action response = %d Cache-Control %q %q; want %d/no-store/%s",
			response.Code, response.Header().Get("Cache-Control"), response.Body.String(), wantCode, wantStatus)
	}
}

func loadAuthorizedMatch(t *testing.T, state store.StateStore, matchID string) (domain.MatchRecord, domain.MatchParticipantRecord) {
	t.Helper()
	matchData, err := state.GetState(context.Background(), store.MatchesCollection, matchID)
	if err != nil {
		t.Fatal(err)
	}
	participantsData, err := state.GetState(context.Background(), store.MatchParticipantsCollection, matchID)
	if err != nil {
		t.Fatal(err)
	}
	var match domain.MatchRecord
	var participants domain.MatchParticipantRecord
	if err := json.Unmarshal(matchData, &match); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(participantsData, &participants); err != nil {
		t.Fatal(err)
	}
	return match, participants
}
