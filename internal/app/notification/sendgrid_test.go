package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSendGridEmailProvider(t *testing.T) {
	var requestCount int32
	var receivedAuthHeader string
	var receivedIdempotencyHeader string
	var receivedPayload sendGridPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedIdempotencyHeader = r.Header.Get("X-Entity-Ref-ID")

		if r.URL.Path != "/v3/mail/send" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &receivedPayload); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		if r.Header.Get("Authorization") == "Bearer fail-token" {
			http.Error(w, `{"errors":[{"message":"unauthorized"}]}`, http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") == "Bearer 500-token" {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	t.Run("sends email successfully with proper headers and payload", func(t *testing.T) {
		provider, err := NewSendGridEmailProvider(
			"valid-api-key",
			WithSendGridBaseURL(server.URL),
			WithSendGridFromEmail("test-alerts@petspotr.io"),
		)
		if err != nil {
			t.Fatalf("NewSendGridEmailProvider failed: %v", err)
		}

		msg := EmailMessage{
			To:             "owner@example.com",
			Subject:        "Match Alert",
			TextBody:       "A match was found",
			HTMLBody:       "<h1>Match found</h1>",
			IdempotencyKey: "sg-idem-key-1",
		}

		if err := provider.SendEmail(context.Background(), msg); err != nil {
			t.Fatalf("SendEmail failed: %v", err)
		}

		if receivedAuthHeader != "Bearer valid-api-key" {
			t.Errorf("expected Authorization 'Bearer valid-api-key', got %q", receivedAuthHeader)
		}
		if receivedIdempotencyHeader != "sg-idem-key-1" {
			t.Errorf("expected X-Entity-Ref-ID 'sg-idem-key-1', got %q", receivedIdempotencyHeader)
		}
		if receivedPayload.Subject != "Match Alert" {
			t.Errorf("expected subject 'Match Alert', got %q", receivedPayload.Subject)
		}
		if len(receivedPayload.Personalizations) != 1 || receivedPayload.Personalizations[0].To[0].Email != "owner@example.com" {
			t.Errorf("unexpected recipient in payload: %+v", receivedPayload.Personalizations)
		}
		if len(receivedPayload.Content) != 2 {
			t.Errorf("expected 2 content parts (text and html), got %d", len(receivedPayload.Content))
		}
	})

	t.Run("handles idempotency replay without duplicate HTTP requests", func(t *testing.T) {
		provider, err := NewSendGridEmailProvider(
			"valid-api-key",
			WithSendGridBaseURL(server.URL),
		)
		if err != nil {
			t.Fatalf("NewSendGridEmailProvider failed: %v", err)
		}

		msg := EmailMessage{
			To:             "owner@example.com",
			Subject:        "Match Alert",
			TextBody:       "Text",
			IdempotencyKey: "sg-duplicate-key",
		}

		startCount := atomic.LoadInt32(&requestCount)
		for range 3 {
			if err := provider.SendEmail(context.Background(), msg); err != nil {
				t.Fatalf("SendEmail retry failed: %v", err)
			}
		}
		endCount := atomic.LoadInt32(&requestCount)
		if endCount-startCount != 1 {
			t.Errorf("expected exactly 1 network request for idempotent replays, got %d", endCount-startCount)
		}
	})

	t.Run("returns error on API failure", func(t *testing.T) {
		provider, err := NewSendGridEmailProvider(
			"fail-token",
			WithSendGridBaseURL(server.URL),
		)
		if err != nil {
			t.Fatalf("NewSendGridEmailProvider failed: %v", err)
		}

		msg := EmailMessage{
			To:      "owner@example.com",
			Subject: "Alert",
		}

		err = provider.SendEmail(context.Background(), msg)
		if err == nil {
			t.Fatal("expected error on 401 response, got nil")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("expected error containing 401, got %v", err)
		}
	})
}
