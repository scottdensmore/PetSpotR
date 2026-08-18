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

func TestMatchListIsVisibleOnlyToAuthenticatedParticipants(t *testing.T) {
	memory := store.NewMemoryStore()
	owner := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "owner-101",
		Email: "owner@example.com", EmailVerified: true,
	}
	other := domain.PrincipalRef{Issuer: owner.Issuer, Subject: "other-202"}
	ownerRef := domain.PrincipalRef{Issuer: owner.Issuer, Subject: owner.Subject}

	seedMatchListRecord(t, memory, domain.MatchRecord{
		MatchID: "match-reporter", FoundPetID: "found-reporter", MatchedPetID: "lost-reporter",
	})
	seedMatchListParticipants(t, memory, domain.MatchParticipantRecord{
		MatchID: "match-reporter", FoundPetID: "found-reporter", LostPetID: "lost-reporter",
		Reporter: &ownerRef, Finder: &other,
	})
	seedMatchListRecord(t, memory, domain.MatchRecord{
		MatchID: "match-finder", FoundPetID: "found-finder", MatchedPetID: "lost-finder",
	})
	seedMatchListParticipants(t, memory, domain.MatchParticipantRecord{
		MatchID: "match-finder", FoundPetID: "found-finder", LostPetID: "lost-finder",
		Reporter: &other, Finder: &ownerRef,
	})
	seedMatchListRecord(t, memory, domain.MatchRecord{
		MatchID: "match-other", FoundPetID: "found-other", MatchedPetID: "lost-other",
	})
	seedMatchListParticipants(t, memory, domain.MatchParticipantRecord{
		MatchID: "match-other", FoundPetID: "found-other", LostPetID: "lost-other",
		Reporter: &other,
	})
	seedMatchListRecord(t, memory, domain.MatchRecord{
		MatchID: "match-legacy", FoundPetID: "found-legacy", MatchedPetID: "lost-legacy",
	})
	seedMatchListRecord(t, memory, domain.MatchRecord{
		MatchID: "match-tampered", FoundPetID: "found-tampered", MatchedPetID: "lost-tampered",
	})
	seedMatchListParticipants(t, memory, domain.MatchParticipantRecord{
		MatchID: "match-tampered", FoundPetID: "found-other", LostPetID: "lost-tampered",
		Reporter: &ownerRef,
	})
	seedMatchListRecord(t, memory, domain.MatchRecord{
		MatchID: "match-malformed", FoundPetID: "found-malformed", MatchedPetID: "lost-malformed",
	})
	if err := memory.SaveState(
		context.Background(), store.MatchParticipantsCollection, "match-malformed", []byte(`{invalid`),
	); err != nil {
		t.Fatal(err)
	}

	manager := &stubSessionManager{verified: owner}
	srv := NewServerWithOptions(memory, ServerOptions{IdentitySessions: manager})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
	request.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "owner-session"})
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("participant match list status = %d, want %d; body = %q", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("participant match list Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
	var matches []domain.MatchRecord
	if err := json.Unmarshal(recorder.Body.Bytes(), &matches); err != nil {
		t.Fatalf("decode participant matches: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("participant matches = %#v, want reporter and finder matches", matches)
	}
	got := map[string]bool{}
	for _, match := range matches {
		got[match.MatchID] = true
	}
	if !got["match-reporter"] || !got["match-finder"] || got["match-other"] || got["match-legacy"] {
		t.Fatalf("participant match IDs = %#v", got)
	}
	for _, privateValue := range []string{owner.Issuer, owner.Subject, other.Subject, `"reporter":`, `"finder":`} {
		if strings.Contains(recorder.Body.String(), privateValue) {
			t.Fatalf("match list exposed private participant value %q: %s", privateValue, recorder.Body.String())
		}
	}
	if manager.verifyToken != "owner-session" {
		t.Fatalf("verified session = %q, want owner-session", manager.verifyToken)
	}
	manager.verified.Subject = "stranger-303"
	nonParticipantRequest := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
	nonParticipantRequest.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "stranger-session"})
	nonParticipant := httptest.NewRecorder()
	srv.ServeHTTP(nonParticipant, nonParticipantRequest)
	if nonParticipant.Code != http.StatusOK || strings.TrimSpace(nonParticipant.Body.String()) != "[]" {
		t.Fatalf("non-participant match list = %d %q, want 200 []", nonParticipant.Code, nonParticipant.Body.String())
	}

	anonymous := httptest.NewRecorder()
	srv.ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous match list status = %d, want %d", anonymous.Code, http.StatusUnauthorized)
	}

	disabled := NewServerWithStore(memory)
	legacy := httptest.NewRecorder()
	disabled.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil))
	if legacy.Code != http.StatusOK {
		t.Fatalf("identity-disabled match list status = %d, want %d", legacy.Code, http.StatusOK)
	}
	var legacyMatches []domain.MatchRecord
	if err := json.Unmarshal(legacy.Body.Bytes(), &legacyMatches); err != nil {
		t.Fatalf("decode identity-disabled matches: %v", err)
	}
	if len(legacyMatches) != 6 {
		t.Fatalf("identity-disabled matches = %d, want 6", len(legacyMatches))
	}
}

func seedMatchListRecord(t *testing.T, state store.StateStore, match domain.MatchRecord) {
	t.Helper()
	data, err := json.Marshal(match)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(context.Background(), store.MatchesCollection, match.MatchID, data); err != nil {
		t.Fatal(err)
	}
}

func seedMatchListParticipants(t *testing.T, state store.StateStore, participants domain.MatchParticipantRecord) {
	t.Helper()
	data, err := json.Marshal(participants)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(
		context.Background(), store.MatchParticipantsCollection, participants.MatchID, data,
	); err != nil {
		t.Fatal(err)
	}
}
