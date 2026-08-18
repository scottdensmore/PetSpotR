package e2e_test

import (
	"bytes"
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

func TestFirestoreMatchDecisionIsAtomicAcrossServiceRuntimes(t *testing.T) {
	firestoreHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if firestoreHost == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	projectID := "petspotr-match-decision-ownership"
	firstRuntime := newOwnershipStateRuntime(t, ctx, projectID, firestoreHost)
	secondRuntime := newOwnershipStateRuntime(t, ctx, projectID, firestoreHost)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	reporter := identity.Principal{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-" + suffix,
		Email: "reporter@example.com", EmailVerified: true,
	}
	finder := identity.Principal{
		Issuer: reporter.Issuer, Subject: "finder-" + suffix,
		Email: "finder@example.com", EmailVerified: true,
	}
	match := domain.MatchRecord{
		MatchID: "match-decision-" + suffix, FoundPetID: "found-decision-" + suffix,
		MatchedPetID: "lost-decision-" + suffix, Status: domain.MatchStatusPendingReview,
	}
	participants := domain.MatchParticipantRecord{
		MatchID: match.MatchID, FoundPetID: match.FoundPetID, LostPetID: match.MatchedPetID,
		Reporter: &domain.PrincipalRef{Issuer: reporter.Issuer, Subject: reporter.Subject},
		Finder:   &domain.PrincipalRef{Issuer: finder.Issuer, Subject: finder.Subject},
	}
	matchData, err := json.Marshal(match)
	if err != nil {
		t.Fatal(err)
	}
	participantsData, err := json.Marshal(participants)
	if err != nil {
		t.Fatal(err)
	}
	if err := firstRuntime.Store.SaveState(ctx, store.MatchesCollection, match.MatchID, matchData); err != nil {
		t.Fatal(err)
	}
	if err := firstRuntime.Store.SaveState(
		ctx, store.MatchParticipantsCollection, match.MatchID, participantsData,
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = firstRuntime.Store.DeleteState(cleanupCtx, store.MatchesCollection, match.MatchID)
		_ = firstRuntime.Store.DeleteState(cleanupCtx, store.MatchParticipantsCollection, match.MatchID)
	})

	sessions := &firestoreMatchSessions{principals: map[string]identity.Principal{
		"reporter-session": reporter,
		"finder-session":   finder,
	}}
	firstServer := httptest.NewServer(webfrontend.NewServerWithOptions(firstRuntime.Store, webfrontend.ServerOptions{
		IdentitySessions: sessions,
	}))
	defer firstServer.Close()
	secondServer := httptest.NewServer(webfrontend.NewServerWithOptions(secondRuntime.Store, webfrontend.ServerOptions{
		IdentitySessions: sessions,
	}))
	defer secondServer.Close()

	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	reporterResponse := requestFirestoreMatchAction(
		t, secondServer.URL, match.MatchID, "confirm", "reporter-session", csrfToken,
	)
	if reporterResponse.StatusCode != http.StatusOK || reporterResponse.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("reporter action = %d Cache-Control %q", reporterResponse.StatusCode, reporterResponse.Header.Get("Cache-Control"))
	}
	var reporterResult map[string]string
	decodeOwnershipResponse(t, reporterResponse, &reporterResult)
	if reporterResult["status"] != string(domain.MatchStatusPendingReview) {
		t.Fatalf("reporter result = %#v", reporterResult)
	}

	retry := requestFirestoreMatchAction(t, firstServer.URL, match.MatchID, "confirm", "reporter-session", csrfToken)
	if retry.StatusCode != http.StatusOK {
		t.Fatalf("cross-runtime exact retry = %d", retry.StatusCode)
	}
	_ = retry.Body.Close()
	finderResponse := requestFirestoreMatchAction(
		t, secondServer.URL, match.MatchID, "confirm", "finder-session", csrfToken,
	)
	if finderResponse.StatusCode != http.StatusOK {
		t.Fatalf("finder action = %d", finderResponse.StatusCode)
	}
	var finderResult map[string]string
	decodeOwnershipResponse(t, finderResponse, &finderResult)
	if finderResult["status"] != string(domain.MatchStatusConfirmed) {
		t.Fatalf("finder result = %#v", finderResult)
	}

	storedMatchData, err := firstRuntime.Store.GetState(ctx, store.MatchesCollection, match.MatchID)
	if err != nil {
		t.Fatal(err)
	}
	storedParticipantsData, err := secondRuntime.Store.GetState(ctx, store.MatchParticipantsCollection, match.MatchID)
	if err != nil {
		t.Fatal(err)
	}
	var storedMatch domain.MatchRecord
	var storedParticipants domain.MatchParticipantRecord
	if err := json.Unmarshal(storedMatchData, &storedMatch); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(storedParticipantsData, &storedParticipants); err != nil {
		t.Fatal(err)
	}
	if storedMatch.Status != domain.MatchStatusConfirmed ||
		storedParticipants.ReporterDecision != domain.MatchDecisionConfirm ||
		storedParticipants.FinderDecision != domain.MatchDecisionConfirm || len(storedParticipants.DecisionAudit) != 2 {
		t.Fatalf("durable decision = %#v / %#v", storedMatch, storedParticipants)
	}
	if bytes.Contains(storedMatchData, []byte(reporter.Subject)) || bytes.Contains(storedMatchData, []byte(finder.Subject)) ||
		bytes.Contains(storedMatchData, []byte("decisionAudit")) {
		t.Fatalf("public match exposed private decision data: %s", storedMatchData)
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

func requestFirestoreMatchAction(
	t *testing.T,
	baseURL, matchID, action, session, csrfToken string,
) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]string{"matchId": matchID, "action": action})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/matches/action", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-CSRF-Token", csrfToken)
	request.AddCookie(&http.Cookie{Name: "petspotr_session", Value: session})
	request.AddCookie(&http.Cookie{Name: "petspotr_csrf", Value: csrfToken})
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func decodeOwnershipResponse(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

var _ identity.SessionManager = (*firestoreMatchSessions)(nil)
