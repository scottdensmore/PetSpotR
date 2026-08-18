package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreMatchListFiltersParticipantsAcrossServiceRuntimes(t *testing.T) {
	firestoreHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if firestoreHost == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	projectID := "petspotr-match-list-ownership"
	writer := newOwnershipStateRuntime(t, ctx, projectID, firestoreHost)
	reader := newOwnershipStateRuntime(t, ctx, projectID, firestoreHost)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	reporter := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-" + suffix,
		Email: "reporter@example.com", EmailVerified: true,
	}
	finder := identity.Principal{
		Issuer: reporter.Issuer, Subject: "finder-" + suffix,
		Email: "finder@example.com", EmailVerified: true,
	}
	shared := domain.MatchRecord{
		MatchID: "match-shared-" + suffix, FoundPetID: "found-shared-" + suffix,
		MatchedPetID: "lost-shared-" + suffix,
	}
	sharedParticipants := domain.MatchParticipantRecord{
		MatchID: shared.MatchID, FoundPetID: shared.FoundPetID, LostPetID: shared.MatchedPetID,
		Reporter: &domain.PrincipalRef{Issuer: reporter.Issuer, Subject: reporter.Subject},
		Finder:   &domain.PrincipalRef{Issuer: finder.Issuer, Subject: finder.Subject},
	}
	unrelated := domain.MatchRecord{
		MatchID: "match-unrelated-" + suffix, FoundPetID: "found-unrelated-" + suffix,
		MatchedPetID: "lost-unrelated-" + suffix,
	}
	unrelatedParticipants := domain.MatchParticipantRecord{
		MatchID: unrelated.MatchID, FoundPetID: unrelated.FoundPetID, LostPetID: unrelated.MatchedPetID,
		Reporter: &domain.PrincipalRef{Issuer: reporter.Issuer, Subject: "unrelated-" + suffix},
	}
	for _, fixture := range []struct {
		match        domain.MatchRecord
		participants domain.MatchParticipantRecord
	}{
		{match: shared, participants: sharedParticipants},
		{match: unrelated, participants: unrelatedParticipants},
	} {
		matchData, err := json.Marshal(fixture.match)
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Store.SaveState(ctx, store.MatchesCollection, fixture.match.MatchID, matchData); err != nil {
			t.Fatal(err)
		}
		participantsData, err := json.Marshal(fixture.participants)
		if err != nil {
			t.Fatal(err)
		}
		if err := writer.Store.SaveState(
			ctx, store.MatchParticipantsCollection, fixture.participants.MatchID, participantsData,
		); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		for _, matchID := range []string{shared.MatchID, unrelated.MatchID} {
			_ = writer.Store.DeleteState(cleanupCtx, store.MatchesCollection, matchID)
			_ = writer.Store.DeleteState(cleanupCtx, store.MatchParticipantsCollection, matchID)
		}
	})

	sessions := &firestoreMatchSessions{principals: map[string]identity.Principal{
		"reporter-session": reporter,
		"finder-session":   finder,
		"stranger-session": {
			Issuer: reporter.Issuer, Subject: "stranger-" + suffix,
			Email: "stranger@example.com", EmailVerified: true,
		},
	}}
	server := httptest.NewServer(webfrontend.NewServerWithOptions(reader.Store, webfrontend.ServerOptions{
		IdentitySessions: sessions,
	}))
	defer server.Close()

	for name, session := range map[string]string{"reporter": "reporter-session", "finder": "finder-session"} {
		matches := requestFirestoreMatches(t, server.URL, session)
		if len(matches) != 1 || matches[0].MatchID != shared.MatchID {
			t.Fatalf("%s matches = %#v, want only %s", name, matches, shared.MatchID)
		}
	}
	if matches := requestFirestoreMatches(t, server.URL, "stranger-session"); len(matches) != 0 {
		t.Fatalf("stranger matches = %#v, want none", matches)
	}

	httpClient := &http.Client{Timeout: 5 * time.Second}
	anonymous, err := httpClient.Get(server.URL + "/api/v1/matches")
	if err != nil {
		t.Fatal(err)
	}
	defer anonymous.Body.Close()
	if anonymous.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want %d", anonymous.StatusCode, http.StatusUnauthorized)
	}
}

type firestoreMatchSessions struct {
	principals map[string]identity.Principal
}

func (s *firestoreMatchSessions) CreateSession(context.Context, string, time.Duration) (identity.Session, error) {
	return identity.Session{}, identity.ErrUnauthenticated
}

func (s *firestoreMatchSessions) VerifySession(_ context.Context, cookie string) (identity.Principal, error) {
	principal, ok := s.principals[cookie]
	if !ok {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return principal, nil
}

func requestFirestoreMatches(t *testing.T, baseURL, session string) []domain.MatchRecord {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, baseURL+"/api/v1/matches", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.AddCookie(&http.Cookie{Name: "petspotr_session", Value: session})
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("match list response = %d Cache-Control %q", response.StatusCode, response.Header.Get("Cache-Control"))
	}
	var matches []domain.MatchRecord
	if err := json.NewDecoder(response.Body).Decode(&matches); err != nil {
		t.Fatal(err)
	}
	return matches
}

var _ identity.SessionManager = (*firestoreMatchSessions)(nil)
