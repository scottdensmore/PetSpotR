package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestLostPetService_HandleLostPet(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	svc := NewService(st, ps)

	var publishedEvent domain.LostPetEvent
	var published bool
	_ = ps.Subscribe("lostPet", func(ctx context.Context, data []byte) error {
		published = true
		return json.Unmarshal(data, &publishedEvent)
	})

	t.Run("valid lost pet submission saves state, publishes event, and returns 201 JSON", func(t *testing.T) {
		evt := domain.LostPetEvent{
			PetID:         "pet-123",
			ReporterEmail: "owner@example.com",
			ReportedAt:    time.Now().UTC(),
			Location:      "Seattle, WA",
		}

		body, _ := json.Marshal(evt)
		req := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		svc.HandleLostPet(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 status code, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		if contentType := rec.Header().Get("Content-Type"); contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		var resp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response JSON: %v", err)
		}
		if resp["status"] != "success" || resp["petId"] != "pet-123" {
			t.Errorf("unexpected response body: %+v", resp)
		}

		// Verify state persistence
		data, err := st.GetState(context.Background(), "lostPets", "pet-123")
		if err != nil {
			t.Fatalf("failed to retrieve saved state: %v", err)
		}

		var savedEvt domain.LostPetEvent
		_ = json.Unmarshal(data, &savedEvt)
		if savedEvt.PetID != "pet-123" {
			t.Errorf("saved pet ID mismatch: got %s, want pet-123", savedEvt.PetID)
		}

		// Verify pubsub event publication
		if !published {
			t.Fatal("expected lostPet event to be published")
		}
		if publishedEvent.PetID != "pet-123" {
			t.Errorf("published pet ID mismatch: got %s, want pet-123", publishedEvent.PetID)
		}
	})

	t.Run("non-POST method returns 405 Method Not Allowed JSON error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/lostPet", nil)
		rec := httptest.NewRecorder()

		svc.HandleLostPet(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 status code, got %d", rec.Code)
		}

		var errResp ErrorResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
		if !strings.Contains(errResp.Error, "Method not allowed") {
			t.Errorf("expected Method not allowed error message, got %s", errResp.Error)
		}
	})

	t.Run("malformed JSON returns 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader([]byte(`{bad-json`)))
		rec := httptest.NewRecorder()

		svc.HandleLostPet(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 status code, got %d", rec.Code)
		}
	})

	t.Run("invalid domain payload returns 400 Bad Request JSON error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader([]byte(`{"petId":""}`)))
		rec := httptest.NewRecorder()

		svc.HandleLostPet(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 status code, got %d", rec.Code)
		}

		var errResp ErrorResponse
		_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
		if !strings.Contains(errResp.Error, "validation failed") {
			t.Errorf("expected validation failed error message, got %s", errResp.Error)
		}
	})
}
