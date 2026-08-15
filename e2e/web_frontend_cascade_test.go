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
	"github.com/scottdensmore/petspotr/internal/app/petmatcher"
	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestWebFrontendFullEventCascadeJourney(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	bs := blob.NewMemoryBlobStore("https://storage.petspotr.io/images")

	mockOllamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode mock response: %v", err)
		}
	}))
	defer mockOllamaServer.Close()

	matchResults := make(chan domain.MatchResult, 1)
	if err := ps.Subscribe("matchFound", func(_ context.Context, data []byte) error {
		var result domain.MatchResult
		if _, err := domain.DecodeEventPayload(data, domain.EventTypeMatchFound, &result); err != nil {
			return err
		}
		matchResults <- result
		return nil
	}); err != nil {
		t.Fatalf("subscribe to matchFound: %v", err)
	}

	matcher := petmatcher.NewWorker(st, ps, ollama.NewClient(ollama.WithBaseURL(mockOllamaServer.URL)))
	if err := matcher.Start(ctx); err != nil {
		t.Fatalf("start matcher: %v", err)
	}
	lostReports := lostpet.NewService(st, ps)
	foundReports := foundpet.NewService(st, ps, bs)

	lostEvent := domain.LostPetEvent{
		PetID:         "lost-101",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Capitol Hill, Seattle, WA",
	}
	lostBody, err := lostEvent.ToJSON()
	if err != nil {
		t.Fatalf("encode lost-pet event: %v", err)
	}
	lostRequest := httptest.NewRequest(http.MethodPost, "/lostPet", bytes.NewReader(lostBody))
	lostRecorder := httptest.NewRecorder()
	lostReports.HandleLostPet(lostRecorder, lostRequest)
	if lostRecorder.Code != http.StatusCreated {
		t.Fatalf("lost-pet report status = %d, want %d; body = %s", lostRecorder.Code, http.StatusCreated, lostRecorder.Body.String())
	}

	foundEvent := domain.FoundPetEvent{
		PetID:    "found-999",
		ImageURL: "https://storage.petspotr.io/images/found-999.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Capitol Hill, Seattle, WA",
	}
	foundBody, err := foundEvent.ToJSON()
	if err != nil {
		t.Fatalf("encode found-pet event: %v", err)
	}
	foundRequest := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(foundBody))
	foundRecorder := httptest.NewRecorder()
	foundReports.HandleFoundPet(foundRecorder, foundRequest)
	if foundRecorder.Code != http.StatusCreated {
		t.Fatalf("found-pet report status = %d, want %d; body = %s", foundRecorder.Code, http.StatusCreated, foundRecorder.Body.String())
	}

	select {
	case match := <-matchResults:
		if match.MatchedPetID != lostEvent.PetID || match.FoundPetID != foundEvent.PetID {
			t.Fatalf("match pet IDs = %s/%s, want %s/%s", match.MatchedPetID, match.FoundPetID, lostEvent.PetID, foundEvent.PetID)
		}
		if match.Score < 0.70 {
			t.Fatalf("match score = %f, want at least 0.70", match.Score)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for matchFound event")
	}

	frontend := webfrontend.NewServer()
	actionRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/matches/action",
		strings.NewReader(`{"matchId":"match-101","action":"confirm"}`),
	)
	actionRecorder := httptest.NewRecorder()
	frontend.ServeHTTP(actionRecorder, actionRequest)
	if actionRecorder.Code != http.StatusOK {
		t.Fatalf("match action status = %d, want %d; body = %s", actionRecorder.Code, http.StatusOK, actionRecorder.Body.String())
	}
	assertFrontendMatchStatus(t, frontend, "match-101", "CONFIRMED")

	resolveRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/reunions/resolve",
		strings.NewReader(`{"matchId":"match-101","petId":"lost-101","rating":5,"feedback":"Reunited via Gemma 4 AI!"}`),
	)
	resolveRecorder := httptest.NewRecorder()
	frontend.ServeHTTP(resolveRecorder, resolveRequest)
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("reunion resolution status = %d, want %d; body = %s", resolveRecorder.Code, http.StatusOK, resolveRecorder.Body.String())
	}
	assertFrontendMatchStatus(t, frontend, "match-101", "REUNITED")
}

func assertFrontendMatchStatus(t *testing.T, frontend http.Handler, matchID, wantStatus string) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/matches", nil)
	recorder := httptest.NewRecorder()
	frontend.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("match list status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var matches []webfrontend.MatchRecord
	if err := json.NewDecoder(recorder.Body).Decode(&matches); err != nil {
		t.Fatalf("decode match list: %v", err)
	}
	for _, match := range matches {
		if match.MatchID == matchID {
			if match.Status != wantStatus {
				t.Fatalf("match %s status = %q, want %q", matchID, match.Status, wantStatus)
			}
			return
		}
	}
	t.Fatalf("match %s not found", matchID)
}
