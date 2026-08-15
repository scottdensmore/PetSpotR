package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/foundpet"
	"github.com/scottdensmore/petspotr/internal/app/lostpet"
	"github.com/scottdensmore/petspotr/internal/app/notification"
	"github.com/scottdensmore/petspotr/internal/app/petmatcher"
	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

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
	lostSvc := lostpet.NewService(st, ps)
	foundSvc := foundpet.NewService(st, ps, bs)

	// 3. Register the production Notification worker with an observable dev sender.
	emailSender := notification.NewMockEmailSender()
	notificationWorker := notification.NewWorkerWithStoreAndDispatcher(
		st,
		ps,
		notification.NewMultiChannelDispatcher(
			emailSender,
			notification.NewMockSMSSender(),
			notification.NewMockWebPushSender(),
		),
	)
	if err := notificationWorker.Start(context.Background()); err != nil {
		t.Fatalf("failed to start notification worker: %v", err)
	}

	// 4. Register the production Pet Matcher worker.
	matcherWorker := petmatcher.NewWorker(st, ps, ollamaClient)
	if err := matcherWorker.Start(context.Background()); err != nil {
		t.Fatalf("failed to start pet matcher: %v", err)
	}

	// --- Step A: Report Lost Pet ---
	lostEvt := domain.LostPetEvent{
		PetID:         "lost-101",
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

	// --- Step C: Assert End-to-End Event Cascade & Notification Dispatch ---
	var ownerMessage *notification.NotificationMessage
	for _, message := range emailSender.SentMessages {
		if message.Email == "owner@example.com" {
			ownerMessage = message
			break
		}
	}
	if ownerMessage == nil {
		t.Fatal("expected owner notification to be dispatched")
	}
	if ownerMessage.Subject != "Match Found for Your Pet (lost-101)" {
		t.Errorf("notification subject mismatch: got %q", ownerMessage.Subject)
	}
	if !strings.Contains(ownerMessage.Body, "<strong>100%</strong>") {
		t.Errorf("expected rendered 100%% match confidence, got %q", ownerMessage.Body)
	}
}
