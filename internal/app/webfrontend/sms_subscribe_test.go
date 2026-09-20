package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/sms"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestApiSmsSubscribe(t *testing.T) {
	st := store.NewMemoryStore()
	mockSMS := sms.NewMockProvider()

	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
		DisableRateLimiting:      true,
		SMSProvider:              mockSMS,
	})

	t.Run("valid phone number sends verification test SMS", func(t *testing.T) {
		mockSMS.Reset()
		body, _ := json.Marshal(map[string]string{"phone": "(206) 555-0199"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/subscribe", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Status  string `json:"status"`
			Phone   string `json:"phone"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.Status != "success" || resp.Phone != "+12065550199" {
			t.Errorf("unexpected response: %+v", resp)
		}

		// Verify SMS was sent
		sent := mockSMS.SentMessages()
		if len(sent) != 1 {
			t.Fatalf("expected 1 sent SMS, got %d", len(sent))
		}
		if sent[0].To != "+12065550199" || !strings.Contains(sent[0].Body, "PetSpotR") {
			t.Errorf("unexpected sent SMS: %+v", sent[0])
		}
	})

	t.Run("invalid phone number returns 400 Bad Request", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"phone": "invalid-phone"})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/subscribe", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("missing phone number returns 400 Bad Request", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/subscribe", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("GET method returns 405 Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sms/subscribe", nil)
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed, got %d", w.Code)
		}
	})

	t.Run("subscribing clears prior opt-out flag", func(t *testing.T) {
		phone := "+12065550299"
		optMgr := sms.NewOptOutManager(st, nil)
		if err := optMgr.OptOut(context.Background(), phone, "STOP"); err != nil {
			t.Fatalf("failed to opt out: %v", err)
		}

		body, _ := json.Marshal(map[string]string{"phone": phone})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/sms/subscribe", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		srv.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		optedOut, err := optMgr.IsOptedOut(context.Background(), phone)
		if err != nil || optedOut {
			t.Errorf("expected phone to no longer be opted out, got %v (err: %v)", optedOut, err)
		}
	})
}
