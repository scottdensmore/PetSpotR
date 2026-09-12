package notification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTwilioSMSProvider(t *testing.T) {
	var requestCount int32
	var receivedUser, receivedPass string
	var receivedContentType string
	var receivedFormTo, receivedFormFrom, receivedFormBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		receivedUser, receivedPass, _ = r.BasicAuth()
		receivedContentType = r.Header.Get("Content-Type")

		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}
		receivedFormTo = r.Form.Get("To")
		receivedFormFrom = r.Form.Get("From")
		receivedFormBody = r.Form.Get("Body")

		if !strings.HasPrefix(r.URL.Path, "/2010-04-01/Accounts/") || !strings.HasSuffix(r.URL.Path, "/Messages.json") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if receivedUser == "ACfail" {
			http.Error(w, `{"code": 20003, "message": "Authentication Error"}`, http.StatusUnauthorized)
			return
		}
		if receivedUser == "AC500" {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	t.Run("sends SMS successfully with basic auth and urlencoded form", func(t *testing.T) {
		provider, err := NewTwilioSMSProvider(
			"ACtest",
			"secret-auth-token",
			WithTwilioBaseURL(server.URL),
			WithTwilioFromNumber("+15005550006"),
		)
		if err != nil {
			t.Fatalf("NewTwilioSMSProvider failed: %v", err)
		}

		msg := SMSMessage{
			To:             "+12065550199",
			Body:           "PetSpotR match alert for Buddy",
			IdempotencyKey: "twilio-key-1",
		}

		if err := provider.SendSMS(context.Background(), msg); err != nil {
			t.Fatalf("SendSMS failed: %v", err)
		}

		if receivedUser != "ACtest" || receivedPass != "secret-auth-token" {
			t.Errorf("expected basic auth ACtest:secret-auth-token, got %s:%s", receivedUser, receivedPass)
		}
		if !strings.Contains(receivedContentType, "application/x-www-form-urlencoded") {
			t.Errorf("expected urlencoded content type, got %s", receivedContentType)
		}
		if receivedFormTo != "+12065550199" {
			t.Errorf("expected form To '+12065550199', got %q", receivedFormTo)
		}
		if receivedFormFrom != "+15005550006" {
			t.Errorf("expected form From '+15005550006', got %q", receivedFormFrom)
		}
		if receivedFormBody != "PetSpotR match alert for Buddy" {
			t.Errorf("expected form Body 'PetSpotR match alert for Buddy', got %q", receivedFormBody)
		}
	})

	t.Run("handles idempotency replay without duplicate HTTP requests", func(t *testing.T) {
		provider, err := NewTwilioSMSProvider(
			"ACtest",
			"secret-token",
			WithTwilioBaseURL(server.URL),
		)
		if err != nil {
			t.Fatalf("NewTwilioSMSProvider failed: %v", err)
		}

		msg := SMSMessage{
			To:             "+12065550199",
			Body:           "Alert text",
			IdempotencyKey: "twilio-duplicate-key",
		}

		startCount := atomic.LoadInt32(&requestCount)
		for range 3 {
			if err := provider.SendSMS(context.Background(), msg); err != nil {
				t.Fatalf("SendSMS retry failed: %v", err)
			}
		}
		endCount := atomic.LoadInt32(&requestCount)
		if endCount-startCount != 1 {
			t.Errorf("expected exactly 1 network request for idempotent replays, got %d", endCount-startCount)
		}
	})

	t.Run("returns error on Twilio API failure", func(t *testing.T) {
		provider, err := NewTwilioSMSProvider(
			"ACfail",
			"bad-token",
			WithTwilioBaseURL(server.URL),
		)
		if err != nil {
			t.Fatalf("NewTwilioSMSProvider failed: %v", err)
		}

		msg := SMSMessage{
			To:   "+12065550199",
			Body: "Alert",
		}

		err = provider.SendSMS(context.Background(), msg)
		if err == nil {
			t.Fatal("expected error on 401 response, got nil")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Errorf("expected error containing 401, got %v", err)
		}
	})
}
