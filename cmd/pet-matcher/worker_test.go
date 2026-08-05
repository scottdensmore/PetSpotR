package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMatcherWorker_ProcessFoundPet(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()

	// Mock Ollama server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollama.GenerateResponse{
			Model: "gemma2:2b",
			Response: "{\n" +
				"  \"breed\": \"Golden Retriever\",\n" +
				"  \"primaryColor\": \"Golden\",\n" +
				"  \"secondaryColor\": \"Cream\",\n" +
				"  \"distinctiveMarkings\": [\"White chest patch\"],\n" +
				"  \"eyeColor\": \"Brown\"\n" +
				"}",
			Done: true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	ollamaClient := ollama.NewClient(ollama.WithBaseURL(ts.URL))
	worker := NewWorker(st, ps, ollamaClient)

	// Save lost pet in store
	lostEvt := domain.LostPetEvent{
		PetID:         "lost-101",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Seattle, WA",
	}
	lostData, _ := lostEvt.ToJSON()
	_ = st.SaveState(context.Background(), "lostPets", "lost-101", lostData)

	var matchFoundEvent domain.MatchResult
	var matchPublished bool
	_ = ps.Subscribe("matchFound", func(ctx context.Context, data []byte) error {
		matchPublished = true
		return json.Unmarshal(data, &matchFoundEvent)
	})

	t.Run("found pet matching lost pet publishes matchFound event", func(t *testing.T) {
		foundEvt := domain.FoundPetEvent{
			PetID:    "found-202",
			ImageURL: "https://storage.petspotr.io/found-202.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Seattle, WA",
		}
		foundData, _ := foundEvt.ToJSON()

		err := worker.ProcessFoundPet(context.Background(), foundData)
		if err != nil {
			t.Fatalf("ProcessFoundPet failed: %v", err)
		}

		if !matchPublished {
			t.Fatal("expected matchFound event to be published")
		}

		if matchFoundEvent.FoundPetID != "found-202" || matchFoundEvent.MatchedPetID != "lost-101" {
			t.Errorf("match event IDs mismatch: got found %s, matched %s", matchFoundEvent.FoundPetID, matchFoundEvent.MatchedPetID)
		}

		if !matchFoundEvent.IsMatch {
			t.Errorf("expected IsMatch true, got false")
		}
	})

	t.Run("invalid json event returns error", func(t *testing.T) {
		err := worker.ProcessFoundPet(context.Background(), []byte("{invalid-json"))
		if err == nil {
			t.Error("expected error for invalid json, got nil")
		}
	})

	t.Run("invalid event validation returns error", func(t *testing.T) {
		invalidEvt := domain.FoundPetEvent{PetID: ""} // missing required fields
		data, _ := invalidEvt.ToJSON()
		err := worker.ProcessFoundPet(context.Background(), data)
		if err == nil {
			t.Error("expected error for unvalidated event, got nil")
		}
	})

	t.Run("no candidate in lost pet store returns nil without error", func(t *testing.T) {
		emptyStore := store.NewMemoryStore()
		emptyWorker := NewWorker(emptyStore, ps, ollamaClient)

		foundEvt := domain.FoundPetEvent{
			PetID:    "found-203",
			ImageURL: "https://storage.petspotr.io/found-203.jpg",
		}
		data, _ := foundEvt.ToJSON()
		err := emptyWorker.ProcessFoundPet(context.Background(), data)
		if err != nil {
			t.Errorf("expected nil error when no candidate available, got %v", err)
		}
	})

	t.Run("ollama error returned when generate fails", func(t *testing.T) {
		badOllama := ollama.NewClient(ollama.WithBaseURL("http://invalid-host-12345"))
		badWorker := NewWorker(st, ps, badOllama)

		foundEvt := domain.FoundPetEvent{
			PetID:    "found-204",
			ImageURL: "https://storage.petspotr.io/found-204.jpg",
		}
		data, _ := foundEvt.ToJSON()
		err := badWorker.ProcessFoundPet(context.Background(), data)
		if err == nil {
			t.Error("expected error when Ollama client fails, got nil")
		}
	})
}

func TestMatcherWorker_Start(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	oc := ollama.NewClient()
	worker := NewWorker(st, ps, oc)

	t.Run("Start registers foundPet subscription successfully", func(t *testing.T) {
		ctx := context.Background()
		if err := worker.Start(ctx); err != nil {
			t.Fatalf("worker.Start failed: %v", err)
		}
	})

	t.Run("Start returns error on cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := worker.Start(ctx); err == nil {
			t.Error("expected error on cancelled context, got nil")
		}
	})
}
