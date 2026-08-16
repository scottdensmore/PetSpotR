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

func TestWebFrontendLostPetSubmissionUsesCanonicalService(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	lostReports := lostpet.NewService(st, ps)
	frontend := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		LostPetReporter: lostReports,
	})

	published := make(chan domain.LostPetEvent, 1)
	if err := ps.Subscribe("lostPet", func(_ context.Context, data []byte) error {
		var event domain.LostPetEvent
		if _, err := domain.DecodeEventPayload(data, domain.EventTypeLostPetReported, &event); err != nil {
			return err
		}
		published <- event
		return nil
	}); err != nil {
		t.Fatalf("subscribe to lostPet: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/lost-pets",
		strings.NewReader(`{"petName":"Buddy","reporterEmail":"owner@example.com","location":"Seattle, WA"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	frontend.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("browser lost-pet status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	var response struct {
		Status string `json:"status"`
		PetID  string `json:"petId"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode browser lost-pet response: %v", err)
	}
	if response.Status != "success" || response.PetID == "" {
		t.Fatalf("browser lost-pet response = %#v", response)
	}

	select {
	case event := <-published:
		if event.PetID != response.PetID || event.ReporterEmail != "owner@example.com" || event.Location != "Seattle, WA" {
			t.Fatalf("published lost-pet event = %#v; response pet ID = %q", event, response.PetID)
		}
	case <-time.After(time.Second):
		t.Fatal("browser lost-pet submission did not publish the canonical lostPet event")
	}

	if _, err := st.GetState(ctx, store.LostPetsCollection, response.PetID); err != nil {
		t.Fatalf("load canonical lost-pet state: %v", err)
	}
	outboxRecords, err := st.ListState(ctx, store.OutboxCollection)
	if err != nil {
		t.Fatalf("list canonical lost-pet outbox: %v", err)
	}
	if len(outboxRecords) != 1 {
		t.Fatalf("lost-pet outbox record count = %d, want 1", len(outboxRecords))
	}
}

func TestLostPetHTTPAdaptersShareCanonicalContract(t *testing.T) {
	reportedAt := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name            string
		path            string
		body            func(petID, reporterEmail, location string) string
		invalidResponse string
	}{
		{
			name: "direct service",
			path: "/lostPet",
			body: func(petID, reporterEmail, location string) string {
				data, err := json.Marshal(domain.LostPetEvent{
					PetID:         petID,
					ReporterEmail: reporterEmail,
					ReportedAt:    reportedAt,
					Location:      location,
				})
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			},
			invalidResponse: `{"error":"Event validation failed: domain: reporterEmail is required"}` + "\n",
		},
		{
			name: "browser service",
			path: "/api/v1/lost-pets",
			body: func(petID, reporterEmail, location string) string {
				data, err := json.Marshal(map[string]any{
					"petId":         petID,
					"petName":       "Buddy",
					"reporterEmail": reporterEmail,
					"reportedAt":    reportedAt,
					"location":      location,
				})
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			},
			invalidResponse: "reporterEmail is required\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			st := store.NewMemoryStore()
			ps := pubsub.NewMemoryPubSub()
			lostReports := lostpet.NewService(st, ps)
			frontend := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
				LostPetReporter: lostReports,
			})

			published := make(chan domain.LostPetEvent, 4)
			if err := ps.Subscribe("lostPet", func(_ context.Context, data []byte) error {
				var event domain.LostPetEvent
				if _, err := domain.DecodeEventPayload(data, domain.EventTypeLostPetReported, &event); err != nil {
					return err
				}
				published <- event
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			handler := http.Handler(http.HandlerFunc(lostReports.HandleLostPet))
			if tt.path == "/api/v1/lost-pets" {
				handler = frontend
			}
			submit := func(body string) *httptest.ResponseRecorder {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				handler.ServeHTTP(recorder, request)
				return recorder
			}

			invalid := submit(tt.body("lost-invalid", "", "Seattle, WA"))
			if invalid.Code != http.StatusBadRequest || invalid.Body.String() != tt.invalidResponse {
				t.Fatalf("invalid response = %d %q, want %d %q", invalid.Code, invalid.Body.String(), http.StatusBadRequest, tt.invalidResponse)
			}

			validBody := tt.body("lost-stable", "owner@example.com", "Seattle, WA")
			first := submit(validBody)
			if first.Code != http.StatusCreated {
				t.Fatalf("first create status = %d, want %d; body = %s", first.Code, http.StatusCreated, first.Body.String())
			}
			retry := submit(validBody)
			if retry.Code != http.StatusCreated {
				t.Fatalf("exact retry status = %d, want %d; body = %s", retry.Code, http.StatusCreated, retry.Body.String())
			}
			conflict := submit(tt.body("lost-stable", "owner@example.com", "Portland, OR"))
			if conflict.Code != http.StatusConflict {
				t.Fatalf("conflicting retry status = %d, want %d; body = %s", conflict.Code, http.StatusConflict, conflict.Body.String())
			}
			sameName := submit(tt.body("lost-distinct", "second@example.com", "Seattle, WA"))
			if sameName.Code != http.StatusCreated {
				t.Fatalf("same-name distinct report status = %d, want %d; body = %s", sameName.Code, http.StatusCreated, sameName.Body.String())
			}

			if got := len(published); got != 2 {
				t.Fatalf("published event count = %d, want 2", got)
			}
			outboxRecords, err := st.ListState(ctx, store.OutboxCollection)
			if err != nil {
				t.Fatal(err)
			}
			if len(outboxRecords) != 2 {
				t.Fatalf("outbox record count = %d, want 2", len(outboxRecords))
			}
		})
	}
}

func TestWebFrontendFoundPetSubmissionUsesCanonicalService(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	foundReports := foundpet.NewService(st, ps, blob.NewMemoryBlobStore("https://storage.petspotr.io/images"))
	frontend := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		FoundPetReporter: foundReports,
	})

	published := make(chan domain.FoundPetEvent, 1)
	if err := ps.Subscribe("foundPet", func(_ context.Context, data []byte) error {
		var event domain.FoundPetEvent
		if _, err := domain.DecodeEventPayload(data, domain.EventTypeFoundPetReported, &event); err != nil {
			return err
		}
		published <- event
		return nil
	}); err != nil {
		t.Fatalf("subscribe to foundPet: %v", err)
	}

	foundAt := time.Date(2026, time.August, 15, 13, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(map[string]any{
		"petId":       "found-browser-stable",
		"foundAt":     foundAt,
		"imageUrl":    "https://storage.petspotr.io/images/found-browser-stable.jpg",
		"location":    "Seattle, WA",
		"finderEmail": "finder@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	frontend.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("browser found-pet status = %d, want %d; body = %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	var response struct {
		Status string `json:"status"`
		PetID  string `json:"petId"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode browser found-pet response: %v", err)
	}
	if response.Status != "success" || response.PetID != "found-browser-stable" {
		t.Fatalf("browser found-pet response = %#v", response)
	}

	select {
	case event := <-published:
		if event.PetID != response.PetID || event.ImageURL != "https://storage.petspotr.io/images/found-browser-stable.jpg" || event.Location != "Seattle, WA" {
			t.Fatalf("published found-pet event = %#v; response pet ID = %q", event, response.PetID)
		}
	case <-time.After(time.Second):
		t.Fatal("browser found-pet submission did not publish the canonical foundPet event")
	}

	stateData, err := st.GetState(ctx, store.FoundPetsCollection, response.PetID)
	if err != nil {
		t.Fatalf("load canonical found-pet state: %v", err)
	}
	if strings.Contains(string(stateData), "finder@example.com") || strings.Contains(string(stateData), "finderEmail") {
		t.Fatalf("canonical found-pet state exposed finder contact: %s", stateData)
	}
	outboxRecords, err := st.ListState(ctx, store.OutboxCollection)
	if err != nil {
		t.Fatalf("list canonical found-pet outbox: %v", err)
	}
	if len(outboxRecords) != 1 {
		t.Fatalf("found-pet outbox record count = %d, want 1", len(outboxRecords))
	}
}

func TestFoundPetHTTPAdaptersShareCanonicalContract(t *testing.T) {
	foundAt := time.Date(2026, time.August, 15, 14, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		path string
		body func(petID, imageURL, location string) string
	}{
		{
			name: "direct service",
			path: "/foundPet",
			body: func(petID, imageURL, location string) string {
				data, err := json.Marshal(domain.FoundPetEvent{
					PetID: petID, ImageURL: imageURL, FoundAt: foundAt, Location: location,
				})
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			},
		},
		{
			name: "browser service",
			path: "/api/v1/found-pets",
			body: func(petID, imageURL, location string) string {
				data, err := json.Marshal(map[string]any{
					"petId": petID, "imageUrl": imageURL, "foundAt": foundAt,
					"location": location, "finderEmail": "finder@example.com",
				})
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			st := store.NewMemoryStore()
			ps := pubsub.NewMemoryPubSub()
			foundReports := foundpet.NewService(st, ps, blob.NewMemoryBlobStore("https://storage.petspotr.io/images"))
			frontend := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
				FoundPetReporter: foundReports,
			})

			published := make(chan domain.FoundPetEvent, 4)
			if err := ps.Subscribe("foundPet", func(_ context.Context, data []byte) error {
				var event domain.FoundPetEvent
				if _, err := domain.DecodeEventPayload(data, domain.EventTypeFoundPetReported, &event); err != nil {
					return err
				}
				published <- event
				return nil
			}); err != nil {
				t.Fatal(err)
			}

			handler := http.Handler(http.HandlerFunc(foundReports.HandleFoundPet))
			if tt.path == "/api/v1/found-pets" {
				handler = frontend
			}
			submit := func(body string) *httptest.ResponseRecorder {
				recorder := httptest.NewRecorder()
				request := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				handler.ServeHTTP(recorder, request)
				return recorder
			}

			invalidCases := []struct {
				name     string
				imageURL string
				location string
			}{
				{name: "missing image", location: "Seattle, WA"},
				{name: "missing location", imageURL: "https://storage.petspotr.io/images/found-invalid.jpg"},
			}
			for _, invalidCase := range invalidCases {
				invalid := submit(tt.body("found-invalid-"+strings.ReplaceAll(invalidCase.name, " ", "-"), invalidCase.imageURL, invalidCase.location))
				if invalid.Code != http.StatusBadRequest {
					t.Fatalf("%s response status = %d, want %d; body = %s", invalidCase.name, invalid.Code, http.StatusBadRequest, invalid.Body.String())
				}
			}

			validBody := tt.body("found-stable", "https://storage.petspotr.io/images/found-stable.jpg", "Seattle, WA")
			first := submit(validBody)
			if first.Code != http.StatusCreated {
				t.Fatalf("first create status = %d, want %d; body = %s", first.Code, http.StatusCreated, first.Body.String())
			}
			retry := submit(validBody)
			if retry.Code != http.StatusCreated {
				t.Fatalf("exact retry status = %d, want %d; body = %s", retry.Code, http.StatusCreated, retry.Body.String())
			}
			conflict := submit(tt.body("found-stable", "https://storage.petspotr.io/images/found-stable.jpg", "Portland, OR"))
			if conflict.Code != http.StatusConflict {
				t.Fatalf("conflicting retry status = %d, want %d; body = %s", conflict.Code, http.StatusConflict, conflict.Body.String())
			}
			distinct := submit(tt.body("found-distinct", "https://storage.petspotr.io/images/found-stable.jpg", "Seattle, WA"))
			if distinct.Code != http.StatusCreated {
				t.Fatalf("distinct report status = %d, want %d; body = %s", distinct.Code, http.StatusCreated, distinct.Body.String())
			}

			if got := len(published); got != 2 {
				t.Fatalf("published event count = %d, want 2", got)
			}
			outboxRecords, err := st.ListState(ctx, store.OutboxCollection)
			if err != nil {
				t.Fatal(err)
			}
			if len(outboxRecords) != 2 {
				t.Fatalf("outbox record count = %d, want 2", len(outboxRecords))
			}
		})
	}
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
