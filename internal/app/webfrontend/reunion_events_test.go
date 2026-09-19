package webfrontend

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/identity"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type multiUserSessionManager struct {
	principals map[string]identity.Principal
}

func (m *multiUserSessionManager) CreateSession(_ context.Context, _ string, _ time.Duration) (identity.Session, error) {
	return identity.Session{}, nil
}

func (m *multiUserSessionManager) VerifySession(_ context.Context, sessionCookie string) (identity.Principal, error) {
	p, ok := m.principals[sessionCookie]
	if !ok {
		return identity.Principal{}, identity.ErrUnauthenticated
	}
	return p, nil
}

func setupReunionTestEnvironment(t *testing.T, pingInterval time.Duration) (
	*Server,
	*httptest.Server,
	identity.Principal,
	identity.Principal,
	identity.Principal,
	string,
	string,
) {
	t.Helper()

	state := store.NewMemoryStore()
	reporter := identity.Principal{
		Issuer:        "https://securetoken.google.com/petspotr-test",
		Subject:       "reporter-101",
		Email:         "reporter@example.com",
		EmailVerified: true,
	}
	finder := identity.Principal{
		Issuer:        reporter.Issuer,
		Subject:       "finder-202",
		Email:         "finder@example.com",
		EmailVerified: true,
	}
	stranger := identity.Principal{
		Issuer:        reporter.Issuer,
		Subject:       "stranger-999",
		Email:         "stranger@example.com",
		EmailVerified: true,
	}
	operator := identity.Principal{
		Issuer:        reporter.Issuer,
		Subject:       "operator-777",
		Email:         "operator@example.com",
		EmailVerified: true,
	}

	reporterRef := domain.PrincipalRef{Issuer: reporter.Issuer, Subject: reporter.Subject}
	finderRef := domain.PrincipalRef{Issuer: finder.Issuer, Subject: finder.Subject}
	operatorRef := domain.PrincipalRef{Issuer: operator.Issuer, Subject: operator.Subject}

	grantRoleForTest(t, state, operatorRef, domain.RoleScope{Kind: domain.RoleScopeGlobal})

	seedAuthorizedMatch(t, state, "match-thread", "lost-101", "found-202", &reporterRef, &finderRef)
	seedAuthorizedMatch(t, state, "match-incomplete-thread", "lost-303", "found-404", &reporterRef, nil)
	confirmedMatchID := seedMatchForReunion(t, state, "resolve-dispatch", "lost-101", "found-202", domain.MatchStatusConfirmed)

	sessionMgr := &multiUserSessionManager{
		principals: map[string]identity.Principal{
			"reporter-session": reporter,
			"finder-session":   finder,
			"stranger-session": stranger,
			"operator-session": operator,
		},
	}

	hub := NewReunionHub()
	srv := NewServerWithOptions(state, ServerOptions{
		IdentitySessions:    sessionMgr,
		ReunionHub:          hub,
		ReunionPingInterval: pingInterval,
	})

	ts := httptest.NewServer(srv)
	t.Cleanup(func() {
		ts.Close()
		srv.Close()
	})

	csrfToken := "0123456789abcdef0123456789abcdef0123456789abcdef"
	return srv, ts, reporter, finder, stranger, csrfToken, confirmedMatchID
}

type parsedSSEEvent struct {
	id    string
	event string
	data  string
}

func readLineWithTimeout(reader *bufio.Reader, timeout time.Duration) (string, error) {
	type readResult struct {
		line string
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		line, err := reader.ReadString('\n')
		ch <- readResult{line: line, err: err}
	}()
	select {
	case res := <-ch:
		return res.line, res.err
	case <-time.After(timeout):
		return "", errors.New("timed out waiting for line")
	}
}

func readSSEEvent(reader *bufio.Reader, timeout time.Duration) (parsedSSEEvent, error) {
	var result parsedSSEEvent
	deadline := time.Now().Add(timeout)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return result, errors.New("timed out waiting for SSE event")
		}
		line, err := readLineWithTimeout(reader, remaining)
		if err != nil {
			return result, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, ":") {
			// comment line (e.g. : connected or : ping)
			continue
		}
		if trimmed == "" {
			if result.event != "" || result.data != "" {
				return result, nil
			}
			continue
		}
		if strings.HasPrefix(trimmed, "id: ") {
			result.id = strings.TrimPrefix(trimmed, "id: ")
		} else if strings.HasPrefix(trimmed, "event: ") {
			result.event = strings.TrimPrefix(trimmed, "event: ")
		} else if strings.HasPrefix(trimmed, "data: ") {
			result.data = strings.TrimPrefix(trimmed, "data: ")
		}
	}
}

func TestReunionEvents_SSEHandshake(t *testing.T) {
	_, ts, _, _, _, _, _ := setupReunionTestEnvironment(t, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/reunions/events?matchId=match-thread", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "reporter-session"})

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("handshake request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d; body = %s", resp.StatusCode, http.StatusOK, string(body))
	}

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-cache, no-transform" {
		t.Errorf("Cache-Control = %q, want %q", cc, "no-cache, no-transform")
	}
	if conn := resp.Header.Get("Connection"); conn != "keep-alive" {
		t.Errorf("Connection = %q, want %q", conn, "keep-alive")
	}
	if accel := resp.Header.Get("X-Accel-Buffering"); accel != "no" {
		t.Errorf("X-Accel-Buffering = %q, want %q", accel, "no")
	}

	reader := bufio.NewReader(resp.Body)
	firstLine, err := readLineWithTimeout(reader, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read initial comment: %v", err)
	}
	if strings.TrimRight(firstLine, "\r\n") != ": connected" {
		t.Errorf("first line = %q, want %q", firstLine, ": connected")
	}
}

func TestReunionEvents_AccessControl(t *testing.T) {
	_, ts, _, _, _, csrfToken, _ := setupReunionTestEnvironment(t, 0)

	tests := []struct {
		name       string
		method     string
		url        string
		session    string
		csrfToken  string
		body       string
		wantStatus int
	}{
		{
			name:       "SSE unauthenticated receives 404",
			method:     http.MethodGet,
			url:        ts.URL + "/api/v1/reunions/events?matchId=match-thread",
			session:    "",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "SSE non-participant receives 404",
			method:     http.MethodGet,
			url:        ts.URL + "/api/v1/reunions/events?matchId=match-thread",
			session:    "stranger-session",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "SSE non-existent match receives 404",
			method:     http.MethodGet,
			url:        ts.URL + "/api/v1/reunions/events?matchId=match-missing",
			session:    "reporter-session",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "SSE incomplete match participants receives 404",
			method:     http.MethodGet,
			url:        ts.URL + "/api/v1/reunions/events?matchId=match-incomplete-thread",
			session:    "reporter-session",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Presence unauthenticated receives 404",
			method:     http.MethodPost,
			url:        ts.URL + "/api/v1/reunions/presence",
			session:    "",
			csrfToken:  csrfToken,
			body:       `{"matchId":"match-thread","status":"typing"}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Presence non-participant receives 404",
			method:     http.MethodPost,
			url:        ts.URL + "/api/v1/reunions/presence",
			session:    "stranger-session",
			csrfToken:  csrfToken,
			body:       `{"matchId":"match-thread","status":"typing"}`,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Presence missing CSRF receives 403",
			method:     http.MethodPost,
			url:        ts.URL + "/api/v1/reunions/presence",
			session:    "reporter-session",
			csrfToken:  "",
			body:       `{"matchId":"match-thread","status":"typing"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "Presence invalid status receives 400",
			method:     http.MethodPost,
			url:        ts.URL + "/api/v1/reunions/presence",
			session:    "reporter-session",
			csrfToken:  csrfToken,
			body:       `{"matchId":"match-thread","status":"invalid_status"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "Presence empty matchId receives 400",
			method:     http.MethodPost,
			url:        ts.URL + "/api/v1/reunions/presence",
			session:    "reporter-session",
			csrfToken:  csrfToken,
			body:       `{"matchId":"","status":"typing"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var bodyReader io.Reader
			if tc.body != "" {
				bodyReader = strings.NewReader(tc.body)
			}
			req, err := http.NewRequest(tc.method, tc.url, bodyReader)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			if tc.session != "" {
				req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: tc.session})
			}
			if tc.csrfToken != "" {
				req.Header.Set(csrfHeaderName, tc.csrfToken)
				req.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: tc.csrfToken})
			}
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d; body = %s", resp.StatusCode, tc.wantStatus, string(body))
			}
		})
	}
}

func TestReunionEvents_Keepalive(t *testing.T) {
	// Configure short ping interval for fast and deterministic testing
	_, ts, _, _, _, _, _ := setupReunionTestEnvironment(t, 25*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/reunions/events?matchId=match-thread", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "reporter-session"})

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Expect connected line
	line, err := readLineWithTimeout(reader, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read connected line: %v", err)
	}
	if strings.TrimRight(line, "\r\n") != ": connected" {
		t.Fatalf("expected : connected, got %q", line)
	}

	// Expect periodic pings
	pingCount := 0
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) && pingCount < 2 {
		l, err := readLineWithTimeout(reader, 200*time.Millisecond)
		if err != nil {
			break
		}
		if strings.TrimRight(l, "\r\n") == ": ping" {
			pingCount++
		}
	}

	if pingCount < 2 {
		t.Errorf("expected at least 2 ping comments within window, got %d", pingCount)
	}
}

func TestReunionEvents_PresenceDispatch(t *testing.T) {
	_, ts, reporter, finder, _, csrfToken, _ := setupReunionTestEnvironment(t, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Reporter subscribes to SSE stream
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/reunions/events?matchId=match-thread", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "reporter-session"})

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Consume initial connected comment
	initLine, err := readLineWithTimeout(reader, 2*time.Second)
	if err != nil || strings.TrimRight(initLine, "\r\n") != ": connected" {
		t.Fatalf("failed to get connected line: %v (%s)", err, initLine)
	}

	// Finder posts presence
	presenceBody := `{"matchId":"match-thread","status":"typing"}`
	presReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/reunions/presence", strings.NewReader(presenceBody))
	if err != nil {
		t.Fatalf("failed to create presence request: %v", err)
	}
	presReq.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "finder-session"})
	presReq.Header.Set(csrfHeaderName, csrfToken)
	presReq.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	presReq.Header.Set("Content-Type", "application/json")

	presResp, err := ts.Client().Do(presReq)
	if err != nil {
		t.Fatalf("presence request failed: %v", err)
	}
	defer presResp.Body.Close()
	if presResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(presResp.Body)
		t.Fatalf("presence status = %d, want %d; body = %s", presResp.StatusCode, http.StatusOK, string(b))
	}

	// Reporter SSE stream should receive the presence event
	evt, err := readSSEEvent(reader, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read presence event from SSE stream: %v", err)
	}

	if evt.event != string(domain.ReunionEventPresence) {
		t.Errorf("event type = %q, want %q", evt.event, domain.ReunionEventPresence)
	}
	if evt.id == "" {
		t.Errorf("event id should not be empty")
	}

	var payload domain.ReunionPresencePayload
	if err := json.Unmarshal([]byte(evt.data), &payload); err != nil {
		t.Fatalf("failed to decode event payload: %v; raw data = %q", err, evt.data)
	}

	if payload.SenderRole != domain.MatchParticipantRoleFinder {
		t.Errorf("payload.SenderRole = %q, want %q", payload.SenderRole, domain.MatchParticipantRoleFinder)
	}
	if payload.Status != "typing" {
		t.Errorf("payload.Status = %q, want %q", payload.Status, "typing")
	}

	// Zero PII check
	for _, pii := range []string{reporter.Email, reporter.Subject, finder.Email, finder.Subject} {
		if strings.Contains(evt.data, pii) {
			t.Errorf("presence event leaked PII %q: %s", pii, evt.data)
		}
	}
}

func TestReunionEvents_MessageDispatchWithImages(t *testing.T) {
	_, ts, reporter, finder, _, csrfToken, _ := setupReunionTestEnvironment(t, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Finder subscribes to SSE stream
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/reunions/events?matchId=match-thread", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "finder-session"})

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Consume initial connected comment
	initLine, err := readLineWithTimeout(reader, 2*time.Second)
	if err != nil || strings.TrimRight(initLine, "\r\n") != ": connected" {
		t.Fatalf("failed to get connected line: %v (%s)", err, initLine)
	}

	// Reporter creates message with images
	msgBody := `{"matchId":"match-thread","message":"Checking paw marking","images":["images/reunions/match-thread/paw.jpg"]}`
	msgReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/reunions/contact", strings.NewReader(msgBody))
	if err != nil {
		t.Fatalf("failed to create contact request: %v", err)
	}
	msgReq.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "reporter-session"})
	msgReq.Header.Set(csrfHeaderName, csrfToken)
	msgReq.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	msgReq.Header.Set("Idempotency-Key", "msg-key-101")
	msgReq.Header.Set("Content-Type", "application/json")

	msgResp, err := ts.Client().Do(msgReq)
	if err != nil {
		t.Fatalf("contact request failed: %v", err)
	}
	defer msgResp.Body.Close()
	if msgResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(msgResp.Body)
		t.Fatalf("contact status = %d, want %d; body = %s", msgResp.StatusCode, http.StatusCreated, string(b))
	}

	// Finder SSE stream should receive message event
	evt, err := readSSEEvent(reader, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read message event from SSE stream: %v", err)
	}

	if evt.event != string(domain.ReunionEventMessageCreated) {
		t.Errorf("event type = %q, want %q", evt.event, domain.ReunionEventMessageCreated)
	}

	var msg domain.MediatedMatchMessage
	if err := json.Unmarshal([]byte(evt.data), &msg); err != nil {
		t.Fatalf("failed to decode message payload: %v; raw = %q", err, evt.data)
	}

	if msg.SenderRole != domain.MatchParticipantRoleReporter {
		t.Errorf("msg.SenderRole = %q, want %q", msg.SenderRole, domain.MatchParticipantRoleReporter)
	}
	if msg.Message != "Checking paw marking" {
		t.Errorf("msg.Message = %q, want %q", msg.Message, "Checking paw marking")
	}
	if len(msg.Images) != 1 || msg.Images[0] != "images/reunions/match-thread/paw.jpg" {
		t.Errorf("msg.Images = %#v, want ['images/reunions/match-thread/paw.jpg']", msg.Images)
	}

	// Zero PII check
	for _, pii := range []string{reporter.Email, reporter.Subject, finder.Email, finder.Subject} {
		if strings.Contains(evt.data, pii) {
			t.Errorf("message event leaked PII %q: %s", pii, evt.data)
		}
	}
}

func TestReunionEvents_ResolveDispatch(t *testing.T) {
	_, ts, reporter, finder, _, csrfToken, confirmedMatchID := setupReunionTestEnvironment(t, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Finder subscribes to SSE stream for confirmedMatchID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/reunions/events?matchId="+confirmedMatchID, nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}
	req.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "finder-session"})

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("SSE request failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	// Consume initial connected comment
	initLine, err := readLineWithTimeout(reader, 2*time.Second)
	if err != nil || strings.TrimRight(initLine, "\r\n") != ": connected" {
		t.Fatalf("failed to get connected line: %v (%s)", err, initLine)
	}

	// Operator resolves the confirmed match
	resolveBody := `{"matchId":"` + confirmedMatchID + `","petId":"lost-101","rating":5,"feedback":"Reunited successfully!"}`
	resReq, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/reunions/resolve", strings.NewReader(resolveBody))
	if err != nil {
		t.Fatalf("failed to create resolve request: %v", err)
	}
	resReq.AddCookie(&http.Cookie{Name: localSessionCookieName, Value: "operator-session"})
	resReq.Header.Set(csrfHeaderName, csrfToken)
	resReq.AddCookie(&http.Cookie{Name: localCSRFCookieName, Value: csrfToken})
	resReq.Header.Set("Idempotency-Key", "res-key-101")
	resReq.Header.Set("Content-Type", "application/json")

	resResp, err := ts.Client().Do(resReq)
	if err != nil {
		t.Fatalf("resolve request failed: %v", err)
	}
	defer resResp.Body.Close()
	if resResp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resResp.Body)
		t.Fatalf("resolve status = %d, want %d; body = %s", resResp.StatusCode, http.StatusOK, string(b))
	}

	// Finder SSE stream should receive reunion_resolved event
	evt, err := readSSEEvent(reader, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to read resolved event from SSE stream: %v", err)
	}

	if evt.event != string(domain.ReunionEventResolved) {
		t.Errorf("event type = %q, want %q", evt.event, domain.ReunionEventResolved)
	}

	var resPayload map[string]any
	if err := json.Unmarshal([]byte(evt.data), &resPayload); err != nil {
		t.Fatalf("failed to decode resolved payload: %v; raw = %q", err, evt.data)
	}

	if resPayload["status"] != string(domain.MatchStatusReunited) && resPayload["status"] != "REUNITED" {
		t.Errorf("status = %v, want REUNITED", resPayload["status"])
	}

	// Zero PII check
	for _, pii := range []string{reporter.Email, reporter.Subject, finder.Email, finder.Subject} {
		if strings.Contains(evt.data, pii) {
			t.Errorf("resolve event leaked PII %q: %s", pii, evt.data)
		}
	}
}
