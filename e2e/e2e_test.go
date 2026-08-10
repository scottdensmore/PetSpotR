package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Mock handlers matching service logic for E2E integration test
type lostPetService struct {
	store  store.StateStore
	broker pubsub.Broker
}

func (s *lostPetService) HandleLostPet(w http.ResponseWriter, r *http.Request) {
	var evt domain.LostPetEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := evt.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data, err := evt.ToJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.SaveState(r.Context(), store.LostPetsCollection, evt.PetID, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.broker.Publish(r.Context(), "lostPet", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "petId": evt.PetID})
}

type foundPetService struct {
	store     store.StateStore
	broker    pubsub.Broker
	blobStore blob.BlobStore
}

func (s *foundPetService) HandleFoundPet(w http.ResponseWriter, r *http.Request) {
	var evt domain.FoundPetEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := evt.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	data, err := evt.ToJSON()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.store.SaveState(r.Context(), store.FoundPetsCollection, evt.PetID, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.broker.Publish(r.Context(), "foundPet", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "success", "petId": evt.PetID})
}

func TestEndToEndPetSpotRWorkflow(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	bs := blob.NewMemoryBlobStore("https://storage.petspotr.io/images")

	// 1. Setup Mock Ollama Server returning realistic Gemma 2 JSON
	mockOllamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode mock response: %v", err)
		}
	}))
	defer mockOllamaServer.Close()

	ollamaClient := ollama.NewClient(ollama.WithBaseURL(mockOllamaServer.URL))

	// 2. Setup Services
	lostSvc := &lostPetService{store: st, broker: ps}
	foundSvc := &foundPetService{store: st, broker: ps, blobStore: bs}

	// 3. Thread-safe channel for receiving dispatched notifications
	notifChan := make(chan *domain.OwnerNotification, 1)

	// 4. Register Notification Worker Subscriber
	err := ps.Subscribe("matchFound", func(ctx context.Context, data []byte) error {
		var matchRes domain.MatchResult
		if err := json.Unmarshal(data, &matchRes); err != nil {
			return err
		}
		if err := matchRes.Validate(); err != nil {
			return err
		}
		notif := &domain.OwnerNotification{
			FromEmail:  "alerts@petspotr.io",
			ToEmail:    "owner@example.com",
			Subject:    fmt.Sprintf("Match Found for Your Pet (%s)", matchRes.MatchedPetID),
			PetName:    matchRes.MatchedPetID,
			MatchScore: matchRes.Score,
		}
		notif.Body = notif.RenderEmailBody()
		notifChan <- notif
		return nil
	})
	if err != nil {
		t.Fatalf("failed to subscribe to matchFound: %v", err)
	}
	if err := ps.Subscribe("lostPet", func(context.Context, []byte) error { return nil }); err != nil {
		t.Fatalf("failed to subscribe to lostPet: %v", err)
	}

	// 5. Register Pet Matcher Worker Subscriber using real scoring pipeline & store lookup
	err = ps.Subscribe("foundPet", func(ctx context.Context, data []byte) error {
		var foundEvt domain.FoundPetEvent
		if err := json.Unmarshal(data, &foundEvt); err != nil {
			return err
		}

		// Ollama trait extraction
		prompt := scoring.BuildGemmaPrompt("Pet", "")
		genReq := &ollama.GenerateRequest{
			Model:  "gemma2:2b",
			Prompt: prompt,
			Images: []string{foundEvt.ImageURL},
		}
		genResp, err := ollamaClient.Generate(ctx, genReq)
		if err != nil {
			return err
		}

		foundTraits, err := scoring.ParseGemmaResponse(genResp.Response)
		if err != nil {
			return err
		}

		// State lookup for lost pet candidate
		lostStateBytes, err := st.GetState(ctx, store.LostPetsCollection, "lost-777")
		if err != nil {
			return err
		}

		var lostEvt domain.LostPetEvent
		if err := json.Unmarshal(lostStateBytes, &lostEvt); err != nil {
			return err
		}

		lostTraits := &scoring.PetTraits{
			Breed:               "Golden Retriever",
			PrimaryColor:        "Golden",
			SecondaryColor:      "Cream",
			DistinctiveMarkings: []string{"White chest patch"},
			EyeColor:            "Brown",
		}

		// Compute similarity using actual domain scoring engine
		matchResult := scoring.ComparePets(lostEvt.PetID, foundEvt.PetID, lostTraits, foundTraits)
		if matchResult != nil && matchResult.IsMatch {
			resBytes, err := matchResult.ToJSON()
			if err != nil {
				return err
			}
			return ps.Publish(ctx, "matchFound", resBytes)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to subscribe to foundPet: %v", err)
	}

	// --- Step A: Report Lost Pet ---
	lostEvt := domain.LostPetEvent{
		PetID:         "lost-777",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Seattle, WA",
	}
	lostBody, err := lostEvt.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize LostPetEvent: %v", err)
	}
	reqLost := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(lostBody))
	recLost := httptest.NewRecorder()
	lostSvc.HandleLostPet(recLost, reqLost)

	if recLost.Code != http.StatusCreated {
		t.Fatalf("LostPet request failed with status %d (body: %s)", recLost.Code, recLost.Body.String())
	}

	// --- Step B: Report Found Pet ---
	foundEvt := domain.FoundPetEvent{
		PetID:    "found-888",
		ImageURL: "https://storage.petspotr.io/images/found-888.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	foundBody, err := foundEvt.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize FoundPetEvent: %v", err)
	}
	reqFound := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(foundBody))
	recFound := httptest.NewRecorder()
	foundSvc.HandleFoundPet(recFound, reqFound)

	if recFound.Code != http.StatusCreated {
		t.Fatalf("FoundPet request failed with status %d (body: %s)", recFound.Code, recFound.Body.String())
	}

	// --- Step C: Assert End-to-End Event Cascade & Notification Generation ---
	select {
	case dispatchedNotif := <-notifChan:
		if dispatchedNotif.ToEmail != "owner@example.com" {
			t.Errorf("notification recipient email mismatch: got %s, want owner@example.com", dispatchedNotif.ToEmail)
		}

		if dispatchedNotif.MatchScore != 1.0 {
			t.Errorf("notification match score mismatch: got %f, want 1.0", dispatchedNotif.MatchScore)
		}

		if dispatchedNotif.Body == "" {
			t.Errorf("expected rendered HTML email body")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for dispatched OwnerNotification")
	}
}
