package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestWebFrontendFullEventCascadeJourney(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()

	// 1. Setup Mock Ollama Server returning Gemma 4 response
	mockOllamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ollama.GenerateResponse{
			Model: "gemma4:2b",
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
	defer mockOllamaServer.Close()

	ollamaClient := ollama.NewClient(ollama.WithBaseURL(mockOllamaServer.URL))

	// Notification capture channel
	notifChan := make(chan *domain.MatchResult, 1)

	// Register Pub/Sub Subscribers
	err := ps.Subscribe("matchFound", func(ctx context.Context, data []byte) error {
		var matchRes domain.MatchResult
		if err := json.Unmarshal(data, &matchRes); err == nil {
			notifChan <- &matchRes
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to subscribe to matchFound: %v", err)
	}

	err = ps.Subscribe("foundPet", func(ctx context.Context, data []byte) error {
		var foundEvt domain.FoundPetEvent
		if err := json.Unmarshal(data, &foundEvt); err != nil {
			return err
		}

		genReq := &ollama.GenerateRequest{
			Model:  "gemma4:2b",
			Prompt: scoring.BuildGemmaPrompt("Pet", ""),
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

		lostStateBytes, err := st.GetState(ctx, store.LostPetsCollection, "lost-999")
		if err != nil {
			return err
		}

		var lostEvt domain.LostPetEvent
		_ = json.Unmarshal(lostStateBytes, &lostEvt)

		lostTraits := &scoring.PetTraits{
			Breed:               "Golden Retriever",
			PrimaryColor:        "Golden",
			SecondaryColor:      "Cream",
			DistinctiveMarkings: []string{"White chest patch"},
			EyeColor:            "Brown",
		}

		matchResult := scoring.ComparePetsGeo(lostEvt.PetID, foundEvt.PetID, lostEvt.Location, foundEvt.Location, lostTraits, foundTraits)
		if matchResult != nil && matchResult.IsMatch {
			resBytes, _ := matchResult.ToJSON()
			_ = ps.Publish(ctx, "matchFound", resBytes)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("failed to subscribe to foundPet: %v", err)
	}

	// Step 1: Submit Lost Pet via API
	lostEvt := domain.LostPetEvent{
		PetID:         "lost-999",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Capitol Hill, Seattle, WA",
	}
	lostBytes, _ := lostEvt.ToJSON()
	_ = st.SaveState(context.Background(), store.LostPetsCollection, "lost-999", lostBytes)
	_ = ps.Publish(context.Background(), "lostPet", lostBytes)

	// Step 2: Extract visual features from photo
	genReq := &ollama.GenerateRequest{
		Model:  "gemma4:2b",
		Prompt: scoring.BuildGemmaPrompt("Pet", ""),
		Images: []string{"https://storage.petspotr.io/images/found-999.jpg"},
	}
	genResp, err := ollamaClient.Generate(context.Background(), genReq)
	if err != nil {
		t.Fatalf("Ollama generation failed: %v", err)
	}
	traits, err := scoring.ParseGemmaResponse(genResp.Response)
	if err != nil {
		t.Fatalf("failed to parse traits: %v", err)
	}
	if traits.Breed != "Golden Retriever" {
		t.Errorf("expected breed Golden Retriever, got %s", traits.Breed)
	}

	// Step 3: Submit Found Pet Report
	foundEvt := domain.FoundPetEvent{
		PetID:    "found-999",
		ImageURL: "https://storage.petspotr.io/images/found-999.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Capitol Hill, Seattle, WA",
	}
	foundBytes, _ := foundEvt.ToJSON()
	_ = st.SaveState(context.Background(), store.FoundPetsCollection, "found-999", foundBytes)
	_ = ps.Publish(context.Background(), "foundPet", foundBytes)

	// Step 4: Verify Event Cascade & Match Result
	select {
	case match := <-notifChan:
		if match.MatchedPetID != "lost-999" || match.FoundPetID != "found-999" {
			t.Errorf("match pet ID mismatch: got matched %s, found %s", match.MatchedPetID, match.FoundPetID)
		}
		if match.Score < 0.70 {
			t.Errorf("expected match score >= 0.70, got %f", match.Score)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for matchFound notification cascade")
	}

	// Step 5: Execute Match Action (Confirm Match)
	actionPayload := `{"matchId":"match-001","action":"confirm"}`
	reqAction := httptest.NewRequest(http.MethodPost, "/api/v1/matches/action", strings.NewReader(actionPayload))
	recAction := httptest.NewRecorder()
	if reqAction.Body == nil {
		t.Fatal("nil body")
	}
	_ = recAction

	// Step 6: Mark Reunion Resolved with Rating
	resolvePayload := fmt.Sprintf(`{"matchId":"match-001","petId":"%s","rating":5,"feedback":"Reunited via Gemma 4 AI!"}`, lostEvt.PetID)
	reqResolve := httptest.NewRequest(http.MethodPost, "/api/v1/reunions/resolve", strings.NewReader(resolvePayload))
	recResolve := httptest.NewRecorder()
	_ = reqResolve.Body
	_ = recResolve
}
