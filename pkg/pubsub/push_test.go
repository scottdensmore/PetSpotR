package pubsub_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

type stubPushAuthorizer struct {
	err      error
	audience string
	called   bool
}

func (a *stubPushAuthorizer) Authorize(_ context.Context, authorization, audience string) error {
	a.called = true
	a.audience = audience
	if authorization != "Bearer signed-token" {
		return errors.New("unexpected authorization header")
	}
	return a.err
}

func TestPushHandlerAcknowledgesOnlySuccessfulAuthenticatedProcessing(t *testing.T) {
	const subscription = "projects/petspotr-test/subscriptions/found-pet-matcher"
	payload := []byte(`{"envelopeVersion":1,"id":"evt-101"}`)
	body := pushBody(t, subscription, "message-101", payload)

	tests := []struct {
		name          string
		authErr       error
		handlerErr    error
		body          []byte
		subscription  string
		wantStatus    int
		wantProcessed bool
	}{
		{name: "success acknowledges", body: body, subscription: subscription, wantStatus: http.StatusNoContent, wantProcessed: true},
		{name: "invalid identity is rejected", authErr: errors.New("invalid identity"), body: body, subscription: subscription, wantStatus: http.StatusUnauthorized},
		{name: "transient processing failure requests redelivery", handlerErr: errors.New("ollama unavailable"), body: body, subscription: subscription, wantStatus: http.StatusInternalServerError, wantProcessed: true},
		{name: "wrong subscription is poison", body: body, subscription: "projects/petspotr-test/subscriptions/other", wantStatus: http.StatusBadRequest},
		{name: "malformed delivery is poison", body: []byte(`{"message":{"data":"%%%"}}`), subscription: subscription, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizer := &stubPushAuthorizer{err: tt.authErr}
			processed := false
			handler := pubsub.NewPushHandler(authorizer, tt.subscription, func(_ context.Context, got []byte) error {
				processed = true
				if !bytes.Equal(got, payload) {
					t.Fatalf("handler payload = %s, want %s", got, payload)
				}
				return tt.handlerErr
			})
			req := httptest.NewRequest(http.MethodPost, "https://pet-matcher.example.test/pubsub/found-pet", bytes.NewReader(tt.body))
			req.Header.Set("Authorization", "Bearer signed-token")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if processed != tt.wantProcessed {
				t.Fatalf("processed = %t, want %t", processed, tt.wantProcessed)
			}
			if !authorizer.called || authorizer.audience != "https://pet-matcher.example.test" {
				t.Fatalf("authorizer called = %t, audience = %q", authorizer.called, authorizer.audience)
			}
		})
	}
}

func TestPushHandlerRejectsNonPOST(t *testing.T) {
	handler := pubsub.NewPushHandler(&stubPushAuthorizer{}, "subscription", func(context.Context, []byte) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "https://pet-matcher.example.test/pubsub/found-pet", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

func pushBody(t *testing.T, subscription, messageID string, payload []byte) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"subscription": subscription,
		"message": map[string]any{
			"messageId": messageID,
			"data":      base64.StdEncoding.EncodeToString(payload),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
