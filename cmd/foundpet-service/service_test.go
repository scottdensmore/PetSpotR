package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFoundPetService_HandleFoundPet(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	bs := blob.NewMemoryBlobStore("https://storage.petspotr.io/images")
	svc := NewService(st, ps, bs)

	var publishedEvent domain.FoundPetEvent
	var published bool
	_ = ps.Subscribe("foundPet", func(ctx context.Context, data []byte) error {
		published = true
		return json.Unmarshal(data, &publishedEvent)
	})

	t.Run("valid found pet submission saves state and publishes event", func(t *testing.T) {
		evt := domain.FoundPetEvent{
			PetID:    "pet-found-555",
			ImageURL: "https://storage.petspotr.io/images/found-555.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Portland, OR",
		}

		body, _ := json.Marshal(evt)
		req := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		svc.HandleFoundPet(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 status code, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		// Verify state persistence
		data, err := st.GetState(context.Background(), store.FoundPetsCollection, "pet-found-555")
		if err != nil {
			t.Fatalf("failed to retrieve saved state: %v", err)
		}

		var savedEvt domain.FoundPetEvent
		_ = json.Unmarshal(data, &savedEvt)
		if savedEvt.PetID != "pet-found-555" {
			t.Errorf("saved pet ID mismatch: got %s, want pet-found-555", savedEvt.PetID)
		}

		// Verify pubsub event publication
		if !published {
			t.Fatal("expected foundPet event to be published")
		}
		if publishedEvent.PetID != "pet-found-555" {
			t.Errorf("published pet ID mismatch: got %s, want pet-found-555", publishedEvent.PetID)
		}
	})

	t.Run("non-POST method returns 405 Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/foundPet", nil)
		rec := httptest.NewRecorder()

		svc.HandleFoundPet(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 status code, got %d", rec.Code)
		}
	})

	t.Run("invalid payload returns 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader([]byte(`{"petId":""}`)))
		rec := httptest.NewRecorder()

		svc.HandleFoundPet(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 status code, got %d", rec.Code)
		}
	})
}
