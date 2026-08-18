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

func TestMediatedMatchThreadRequiresParticipantsAndHidesIdentity(t *testing.T) {
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
	seedAuthorizedMatch(t, state, "match-thread", "lost-101", "found-202", &reporterRef, &finderRef)
	seedAuthorizedMatch(t, state, "match-incomplete-thread", "lost-303", "found-404", &reporterRef, nil)
	seedAuthorizedMatch(t, state, "match-rejected-thread", "lost-505", "found-606", &reporterRef, &finderRef)
	seedAuthorizedMatch(t, state, "match-shared-owner-thread", "lost-707", "found-808", &reporterRef, &reporterRef)
	rejectedData, err := state.GetState(context.Background(), store.MatchesCollection, "match-rejected-thread")
	if err != nil {
		t.Fatal(err)
	}
	var rejected domain.MatchRecord
	if err := json.Unmarshal(rejectedData, &rejected); err != nil {
		t.Fatal(err)
	}
	rejected.Status = domain.MatchStatusRejected
	rejectedData, err = json.Marshal(rejected)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(context.Background(), store.MatchesCollection, rejected.MatchID, rejectedData); err != nil {
		t.Fatal(err)
	}

	manager := &stubSessionManager{verified: reporter}
	srv := NewServerWithOptions(state, ServerOptions{IdentitySessions: manager})
	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	reporterBody := `{"matchId":"match-thread","senderEmail":"spoofed@example.com","message":"Can we compare identifying marks?"}`

	anonymous := performMediatedRequest(srv, http.MethodPost, reporterBody, "", csrfToken, "request-101", "")
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous POST status = %d, want %d", anonymous.Code, http.StatusUnauthorized)
	}
	missingCSRF := performMediatedRequest(srv, http.MethodPost, reporterBody, "reporter-session", "", "request-101", "")
	if missingCSRF.Code != http.StatusForbidden {
		t.Fatalf("missing-CSRF POST status = %d, want %d", missingCSRF.Code, http.StatusForbidden)
	}
	missingKey := performMediatedRequest(srv, http.MethodPost, reporterBody, "reporter-session", csrfToken, "", "")
	if missingKey.Code != http.StatusBadRequest {
		t.Fatalf("missing-key POST status = %d, want %d", missingKey.Code, http.StatusBadRequest)
	}
	oversized := performMediatedRequest(
		srv, http.MethodPost,
		`{"matchId":"match-thread","message":"`+strings.Repeat("x", domain.MaxMediatedMessageRunes+1)+`"}`,
		"reporter-session", csrfToken, "request-oversized", "",
	)
	if oversized.Code != http.StatusBadRequest {
		t.Fatalf("oversized POST status = %d, want %d", oversized.Code, http.StatusBadRequest)
	}

	created := performMediatedRequest(
		srv, http.MethodPost, reporterBody, "reporter-session", csrfToken, "request-101", "",
	)
	assertMediatedResponse(t, created, http.StatusCreated, domain.MatchParticipantRoleReporter, "Can we compare identifying marks?")
	if strings.Contains(created.Body.String(), reporter.Email) || strings.Contains(created.Body.String(), reporter.Subject) ||
		strings.Contains(created.Body.String(), finder.Email) || strings.Contains(created.Body.String(), finder.Subject) {
		t.Fatalf("created response exposed private identity: %s", created.Body.String())
	}

	retry := performMediatedRequest(
		srv, http.MethodPost, reporterBody, "reporter-session", csrfToken, "request-101", "",
	)
	assertMediatedResponse(t, retry, http.StatusOK, domain.MatchParticipantRoleReporter, "Can we compare identifying marks?")
	changedRetry := performMediatedRequest(
		srv, http.MethodPost,
		`{"matchId":"match-thread","message":"Different message"}`,
		"reporter-session", csrfToken, "request-101", "",
	)
	if changedRetry.Code != http.StatusConflict {
		t.Fatalf("changed retry status = %d, want %d", changedRetry.Code, http.StatusConflict)
	}

	manager.verified = finder
	finderCreated := performMediatedRequest(
		srv, http.MethodPost,
		`{"matchId":"match-thread","message":"Yes, there is a white spot on the left paw."}`,
		"finder-session", csrfToken, "request-202", "",
	)
	assertMediatedResponse(
		t, finderCreated, http.StatusCreated, domain.MatchParticipantRoleFinder,
		"Yes, there is a white spot on the left paw.",
	)

	matchData, err := state.GetState(context.Background(), store.MatchesCollection, "match-thread")
	if err != nil {
		t.Fatal(err)
	}
	var terminalMatch domain.MatchRecord
	if err := json.Unmarshal(matchData, &terminalMatch); err != nil {
		t.Fatal(err)
	}
	terminalMatch.Status = domain.MatchStatusReunited
	matchData, err = json.Marshal(terminalMatch)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(context.Background(), store.MatchesCollection, terminalMatch.MatchID, matchData); err != nil {
		t.Fatal(err)
	}
	manager.verified = reporter
	terminalRetry := performMediatedRequest(
		srv, http.MethodPost, reporterBody, "reporter-session", csrfToken, "request-101", "",
	)
	assertMediatedResponse(
		t, terminalRetry, http.StatusOK, domain.MatchParticipantRoleReporter, "Can we compare identifying marks?",
	)
	terminalNewMessage := performMediatedRequest(
		srv, http.MethodPost,
		`{"matchId":"match-thread","message":"This message is new."}`,
		"reporter-session", csrfToken, "request-terminal-new", "",
	)
	if terminalNewMessage.Code != http.StatusConflict {
		t.Fatalf("terminal new-message status = %d, want %d", terminalNewMessage.Code, http.StatusConflict)
	}

	for name, session := range map[string]string{"reporter": "reporter-session", "finder": "finder-session"} {
		if name == "reporter" {
			manager.verified = reporter
		} else {
			manager.verified = finder
		}
		thread := performMediatedRequest(srv, http.MethodGet, "", session, "", "", "match-thread")
		if thread.Code != http.StatusOK || thread.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s thread response = %d Cache-Control %q", name, thread.Code, thread.Header().Get("Cache-Control"))
		}
		var response struct {
			MatchID  string                        `json:"matchId"`
			Messages []domain.MediatedMatchMessage `json:"messages"`
		}
		if err := json.Unmarshal(thread.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.MatchID != "match-thread" || len(response.Messages) != 2 {
			t.Fatalf("%s thread = %#v", name, response)
		}
		for _, privateValue := range []string{reporter.Email, reporter.Subject, finder.Email, finder.Subject, "senderEmail"} {
			if strings.Contains(thread.Body.String(), privateValue) {
				t.Fatalf("%s thread exposed %q: %s", name, privateValue, thread.Body.String())
			}
		}
	}

	manager.verified.Subject = "stranger-303"
	wrongOwner := performMediatedRequest(srv, http.MethodGet, "", "stranger-session", "", "", "match-thread")
	missing := performMediatedRequest(srv, http.MethodGet, "", "stranger-session", "", "", "match-missing")
	incomplete := performMediatedRequest(srv, http.MethodGet, "", "stranger-session", "", "", "match-incomplete-thread")
	for name, response := range map[string]*httptest.ResponseRecorder{
		"wrong owner": wrongOwner, "missing": missing, "incomplete": incomplete,
	} {
		if response.Code != http.StatusNotFound {
			t.Fatalf("%s GET status = %d, want %d", name, response.Code, http.StatusNotFound)
		}
		if response.Body.String() != missing.Body.String() {
			t.Fatalf("%s GET body = %q, want non-enumerating %q", name, response.Body.String(), missing.Body.String())
		}
	}

	manager.verified = reporter
	incompleteOwner := performMediatedRequest(
		srv, http.MethodGet, "", "reporter-session", "", "", "match-incomplete-thread",
	)
	if incompleteOwner.Code != http.StatusNotFound {
		t.Fatalf("known incomplete-owner GET status = %d, want %d", incompleteOwner.Code, http.StatusNotFound)
	}
	sharedOwnerWrite := performMediatedRequest(
		srv, http.MethodPost,
		`{"matchId":"match-shared-owner-thread","message":"hello"}`,
		"reporter-session", csrfToken, "request-shared-owner", "",
	)
	if sharedOwnerWrite.Code != http.StatusConflict {
		t.Fatalf("shared-owner POST status = %d, want %d", sharedOwnerWrite.Code, http.StatusConflict)
	}
	terminalWrite := performMediatedRequest(
		srv, http.MethodPost,
		`{"matchId":"match-rejected-thread","message":"hello"}`,
		"reporter-session", csrfToken, "request-terminal", "",
	)
	if terminalWrite.Code != http.StatusConflict {
		t.Fatalf("terminal-thread POST status = %d, want %d", terminalWrite.Code, http.StatusConflict)
	}
	terminalRead := performMediatedRequest(
		srv, http.MethodGet, "", "reporter-session", "", "", "match-rejected-thread",
	)
	if terminalRead.Code != http.StatusOK {
		t.Fatalf("terminal-thread GET status = %d, want %d", terminalRead.Code, http.StatusOK)
	}

	_, participants := loadAuthorizedMatch(t, state, "match-thread")
	if len(participants.Messages) != 2 {
		t.Fatalf("stored message count = %d, want 2", len(participants.Messages))
	}
	participantsData, err := json.Marshal(participants)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(participantsData), "spoofed@example.com") {
		t.Fatalf("private thread trusted senderEmail: %s", participantsData)
	}
}

func performMediatedRequest(
	srv http.Handler,
	method, body, session, csrfToken, idempotencyKey, matchID string,
) *httptest.ResponseRecorder {
	target := "/api/v1/reunions/contact"
	if matchID != "" {
		target += "?matchId=" + matchID
	}
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if session != "" {
		request.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: session})
	}
	if csrfToken != "" {
		request.Header.Set(csrfHeaderName, csrfToken)
		request.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	srv.ServeHTTP(response, request)
	return response
}

func assertMediatedResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
	wantCode int,
	wantRole domain.MatchParticipantRole,
	wantMessage string,
) {
	t.Helper()
	if response.Code != wantCode || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("mediated response = %d Cache-Control %q; body = %q",
			response.Code, response.Header().Get("Cache-Control"), response.Body.String())
	}
	var body struct {
		Status  string                      `json:"status"`
		Message domain.MediatedMatchMessage `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "accepted" || body.Message.MessageID == "" ||
		body.Message.SenderRole != wantRole || body.Message.Message != wantMessage {
		t.Fatalf("mediated response body = %#v", body)
	}
}
