package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const notificationTestSubscription = "projects/local/subscriptions/match-found-notification"

func TestNotificationHTTPHandlerAcknowledgesDurableIdempotentDelivery(t *testing.T) {
	email := newIdempotentTestSender(ChannelEmail, 0)
	sms := newIdempotentTestSender(ChannelSMS, 0)
	push := newIdempotentTestSender(ChannelPush, 0)
	worker := NewWorkerWithStoreAndDispatcher(
		store.NewMemoryStore(),
		nil,
		NewMultiChannelDispatcher(email, sms, push),
	)
	handler := newNotificationHTTPHandler(
		worker,
		pubsub.NewStaticPushAuthorizer("local-secret"),
		notificationTestSubscription,
	)
	body := notificationPushBody(t, notificationTestSubscription, "message-1", matchFoundEnvelope(t, "found-push", "lost-push"))

	unauthorized := httptest.NewRequest(http.MethodPost, "/pubsub/match-found", bytes.NewReader(body))
	unauthorizedRecorder := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedRecorder, unauthorized)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorizedRecorder.Code, http.StatusUnauthorized)
	}

	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodPost, "/pubsub/match-found", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer local-secret")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusNoContent {
			t.Fatalf("delivery %d status = %d, want %d; body=%q", i+1, recorder.Code, http.StatusNoContent, recorder.Body.String())
		}
	}

	for channel, sender := range map[Channel]*idempotentTestSender{
		ChannelEmail: email,
		ChannelSMS:   sms,
		ChannelPush:  push,
	} {
		if calls, effects := sender.snapshot(); calls != 1 || effects != 1 {
			t.Errorf("%s calls/effects = %d/%d, want 1/1", channel, calls, effects)
		}
	}
}

func TestNotificationHTTPHandlerRequestsRedeliveryAfterTransientFailure(t *testing.T) {
	email := newIdempotentTestSender(ChannelEmail, 0)
	sms := newIdempotentTestSender(ChannelSMS, 1)
	push := newIdempotentTestSender(ChannelPush, 0)
	worker := NewWorkerWithStoreAndDispatcher(
		store.NewMemoryStore(),
		nil,
		NewMultiChannelDispatcher(email, sms, push),
	)
	handler := newNotificationHTTPHandler(
		worker,
		pubsub.NewStaticPushAuthorizer("local-secret"),
		notificationTestSubscription,
	)
	body := notificationPushBody(t, notificationTestSubscription, "message-redelivery", matchFoundEnvelope(t, "found-retry", "lost-retry"))

	for attempt, wantStatus := range []int{http.StatusInternalServerError, http.StatusNoContent} {
		request := httptest.NewRequest(http.MethodPost, "/pubsub/match-found", bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer local-secret")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)
		if recorder.Code != wantStatus {
			t.Fatalf("attempt %d status = %d, want %d; body=%q", attempt+1, recorder.Code, wantStatus, recorder.Body.String())
		}
	}

	if calls, effects := email.snapshot(); calls != 1 || effects != 1 {
		t.Errorf("email calls/effects = %d/%d, want 1/1", calls, effects)
	}
	if calls, effects := sms.snapshot(); calls != 2 || effects != 1 {
		t.Errorf("sms calls/effects = %d/%d, want 2/1", calls, effects)
	}
	if calls, effects := push.snapshot(); calls != 1 || effects != 1 {
		t.Errorf("push calls/effects = %d/%d, want 1/1", calls, effects)
	}
}

func TestNotificationHTTPHandlerRejectsWrongSubscription(t *testing.T) {
	handler := newNotificationHTTPHandler(
		NewWorkerWithStore(store.NewMemoryStore(), nil),
		pubsub.NewStaticPushAuthorizer("local-secret"),
		notificationTestSubscription,
	)
	body := notificationPushBody(t, "projects/local/subscriptions/other", "message-wrong-subscription", []byte(`{}`))
	request := httptest.NewRequest(http.MethodPost, "/pubsub/match-found", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer local-secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestNotificationHTTPHandlerRequestsRedeliveryForPoisonPayload(t *testing.T) {
	handler := newNotificationHTTPHandler(
		NewWorkerWithStore(store.NewMemoryStore(), nil),
		pubsub.NewStaticPushAuthorizer("local-secret"),
		notificationTestSubscription,
	)
	body := notificationPushBody(t, notificationTestSubscription, "message-poison", []byte(`{"not":"a match"}`))
	request := httptest.NewRequest(http.MethodPost, "/pubsub/match-found", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer local-secret")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestNotificationHTTPHandlerHealth(t *testing.T) {
	handler := newNotificationHTTPHandler(nil, nil, "")
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "OK" {
		t.Fatalf("health response = %d %q, want 200 OK", recorder.Code, recorder.Body.String())
	}
}

func notificationPushBody(t *testing.T, subscription, messageID string, payload []byte) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"subscription": subscription,
		"message": map[string]string{
			"messageId": messageID,
			"data":      base64.StdEncoding.EncodeToString(payload),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}
