package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

	type publishedLostPet struct {
		data     []byte
		envelope *domain.EventEnvelope
		event    domain.LostPetReportedV2
	}
	published := make(chan publishedLostPet, 1)
	if err := ps.Subscribe("lostPet", func(_ context.Context, data []byte) error {
		var event domain.LostPetReportedV2
		envelope, err := domain.DecodeEventPayload(data, domain.EventTypeLostPetReported, &event)
		if err != nil {
			return err
		}
		published <- publishedLostPet{data: append([]byte(nil), data...), envelope: envelope, event: event}
		return nil
	}); err != nil {
		t.Fatalf("subscribe to lostPet: %v", err)
	}

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/lost-pets",
		strings.NewReader(`{"petId":"lost-buddy-schema","petName":"Buddy","species":"Dog","breed":"Golden Retriever","primaryColor":"Golden","description":"White chest patch","reporterEmail":" Owner@Example.COM ","phone":" (555) 019-2834 ","reportedAt":"2026-08-15T12:00:00Z","location":"Seattle, WA"}`),
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
	case got := <-published:
		if got.envelope == nil || got.envelope.PayloadVersion != domain.LostPetReportedPayloadVersion {
			t.Fatalf("published envelope = %#v", got.envelope)
		}
		if got.event.PetID != response.PetID || got.event.PetName != "Buddy" || got.event.Species != "Dog" ||
			got.event.Breed != "Golden Retriever" || got.event.PrimaryColor != "Golden" ||
			got.event.Description != "White chest patch" || got.event.ReporterEmail != "owner@example.com" ||
			got.event.Location != "Seattle, WA" || got.event.GeocodingStatus != domain.GeocodingPending {
			t.Fatalf("published lost-pet event = %#v; response pet ID = %q", got.event, response.PetID)
		}
		if strings.Contains(string(got.data), `"phone"`) {
			t.Fatalf("published lost-pet event exposed phone data: %s", got.data)
		}

		var legacy domain.LostPetEvent
		if _, err := domain.DecodeEventPayload(got.data, domain.EventTypeLostPetReported, &legacy); err != nil {
			t.Fatalf("legacy reader rejected payload-v2 event: %v", err)
		}
		if legacy.PetID != response.PetID || legacy.ReporterEmail != "owner@example.com" || legacy.Location != "Seattle, WA" {
			t.Fatalf("legacy decoded event = %#v", legacy)
		}
	case <-time.After(time.Second):
		t.Fatal("browser lost-pet submission did not publish the canonical lostPet event")
	}

	stateData, err := st.GetState(ctx, store.LostPetsCollection, response.PetID)
	if err != nil {
		t.Fatalf("load canonical lost-pet state: %v", err)
	}
	var report domain.LostPetReport
	if err := json.Unmarshal(stateData, &report); err != nil {
		t.Fatalf("decode canonical lost-pet state: %v", err)
	}
	if report.PetName != "Buddy" || report.Species != "Dog" || report.Breed != "Golden Retriever" ||
		report.PrimaryColor != "Golden" || report.Description != "White chest patch" ||
		report.ReporterEmail != "owner@example.com" || report.Phone != "(555) 019-2834" ||
		report.Status != domain.LostPetStatusLost || report.GeocodingStatus != domain.GeocodingPending ||
		report.Coordinates != nil {
		t.Fatalf("persisted lost-pet report = %#v", report)
	}

	publicRequest := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets", nil)
	publicRecorder := httptest.NewRecorder()
	frontend.ServeHTTP(publicRecorder, publicRequest)
	if publicRecorder.Code != http.StatusOK {
		t.Fatalf("public lost-pet status = %d, want 200; body = %s", publicRecorder.Code, publicRecorder.Body.String())
	}
	publicData := append([]byte(nil), publicRecorder.Body.Bytes()...)
	var publicReports []domain.PublicLostPetReport
	if err := json.Unmarshal(publicData, &publicReports); err != nil {
		t.Fatalf("decode public lost-pet reports: %v", err)
	}
	if len(publicReports) != 1 || publicReports[0].PetName != "Buddy" || publicReports[0].Species != "Dog" ||
		publicReports[0].GeocodingStatus != domain.GeocodingPending {
		t.Fatalf("public lost-pet reports = %#v", publicReports)
	}
	if strings.Contains(string(publicData), "owner@example.com") ||
		strings.Contains(string(publicData), "reporterEmail") || strings.Contains(string(publicData), "phone") {
		t.Fatalf("public lost-pet response exposed private contact: %s", publicData)
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
				data, err := json.Marshal(map[string]any{
					"petId":         petID,
					"petName":       "Buddy",
					"species":       "Dog",
					"breed":         "Golden Retriever",
					"primaryColor":  "Golden",
					"description":   "White chest patch",
					"reporterEmail": reporterEmail,
					"phone":         "(555) 019-2834",
					"reportedAt":    reportedAt,
					"location":      location,
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
					"species":       "Dog",
					"breed":         "Golden Retriever",
					"primaryColor":  "Golden",
					"description":   "White chest patch",
					"reporterEmail": reporterEmail,
					"phone":         "(555) 019-2834",
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

			published := make(chan domain.LostPetReportedV2, 4)
			if err := ps.Subscribe("lostPet", func(_ context.Context, data []byte) error {
				var event domain.LostPetReportedV2
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

			legacyEmptyLocation := submit(tt.body("lost-no-location", "owner@example.com", ""))
			if legacyEmptyLocation.Code != http.StatusCreated {
				t.Fatalf("empty-location create status = %d, want %d; body = %s", legacyEmptyLocation.Code, http.StatusCreated, legacyEmptyLocation.Body.String())
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

			if got := len(published); got != 3 {
				t.Fatalf("published event count = %d, want 3", got)
			}
			for len(published) > 0 {
				event := <-published
				wantGeocodingStatus := domain.GeocodingPending
				if event.PetID == "lost-no-location" {
					wantGeocodingStatus = domain.GeocodingUnavailable
				}
				if event.PetName != "Buddy" || event.Species != "Dog" || event.Breed != "Golden Retriever" ||
					event.PrimaryColor != "Golden" || event.Description != "White chest patch" ||
					event.GeocodingStatus != wantGeocodingStatus || event.Status != domain.LostPetStatusLost {
					t.Fatalf("published event = %#v", event)
				}
			}
			stateData, err := st.GetState(ctx, store.LostPetsCollection, "lost-stable")
			if err != nil {
				t.Fatal(err)
			}
			var report domain.LostPetReport
			if err := json.Unmarshal(stateData, &report); err != nil {
				t.Fatal(err)
			}
			if report.PetName != "Buddy" || report.Species != "Dog" || report.Breed != "Golden Retriever" ||
				report.PrimaryColor != "Golden" || report.Description != "White chest patch" ||
				report.Phone != "(555) 019-2834" {
				t.Fatalf("persisted report = %#v", report)
			}
			outboxRecords, err := st.ListState(ctx, store.OutboxCollection)
			if err != nil {
				t.Fatal(err)
			}
			if len(outboxRecords) != 3 {
				t.Fatalf("outbox record count = %d, want 3", len(outboxRecords))
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

	type publishedFoundPet struct {
		data     []byte
		envelope *domain.EventEnvelope
		event    domain.FoundPetReportedV2
	}
	published := make(chan publishedFoundPet, 1)
	if err := ps.Subscribe("foundPet", func(_ context.Context, data []byte) error {
		var event domain.FoundPetReportedV2
		envelope, err := domain.DecodeEventPayload(data, domain.EventTypeFoundPetReported, &event)
		if err != nil {
			return err
		}
		published <- publishedFoundPet{data: append([]byte(nil), data...), envelope: envelope, event: event}
		return nil
	}); err != nil {
		t.Fatalf("subscribe to foundPet: %v", err)
	}

	foundAt := time.Date(2026, time.August, 15, 13, 0, 0, 0, time.UTC)
	payload, err := json.Marshal(map[string]any{
		"petId":               "found-browser-stable",
		"foundAt":             foundAt,
		"imageUrl":            "https://storage.petspotr.io/images/found-browser-stable.jpg",
		"location":            "Seattle, WA",
		"finderEmail":         " Finder@Example.COM ",
		"species":             "Dog",
		"breed":               "Golden Retriever",
		"primaryColor":        "Golden",
		"secondaryColor":      "Cream",
		"distinctiveMarkings": []string{"White chest patch"},
		"custodyStatus":       "Local Shelter",
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
	case got := <-published:
		if got.envelope == nil || got.envelope.PayloadVersion != domain.FoundPetReportedPayloadVersion {
			t.Fatalf("published envelope = %#v", got.envelope)
		}
		if got.event.PetID != response.PetID ||
			got.event.ImageURL != "https://storage.petspotr.io/images/found-browser-stable.jpg" ||
			got.event.Location != "Seattle, WA" || got.event.Species != "Dog" ||
			got.event.Breed != "Golden Retriever" || got.event.PrimaryColor != "Golden" ||
			got.event.SecondaryColor != "Cream" || len(got.event.DistinctiveMarkings) != 1 ||
			got.event.DistinctiveMarkings[0] != "White chest patch" ||
			got.event.CustodyStatus != domain.CustodyLocalShelter ||
			got.event.GeocodingStatus != domain.GeocodingPending || got.event.Status != domain.FoundPetStatusFound {
			t.Fatalf("published found-pet event = %#v; response pet ID = %q", got.event, response.PetID)
		}
		if strings.Contains(string(got.data), "finder@example.com") || strings.Contains(string(got.data), "finderEmail") {
			t.Fatalf("published found-pet event exposed finder contact: %s", got.data)
		}
		var legacy domain.FoundPetEvent
		if _, err := domain.DecodeEventPayload(got.data, domain.EventTypeFoundPetReported, &legacy); err != nil {
			t.Fatalf("legacy reader rejected payload-v2 event: %v", err)
		}
		if legacy.PetID != response.PetID || legacy.ImageURL != got.event.ImageURL || legacy.Location != got.event.Location {
			t.Fatalf("legacy decoded event = %#v", legacy)
		}
	case <-time.After(time.Second):
		t.Fatal("browser found-pet submission did not publish the canonical foundPet event")
	}

	stateData, err := st.GetState(ctx, store.FoundPetsCollection, response.PetID)
	if err != nil {
		t.Fatalf("load canonical found-pet state: %v", err)
	}
	var report domain.FoundPetReport
	if err := json.Unmarshal(stateData, &report); err != nil {
		t.Fatalf("decode canonical found-pet state: %v", err)
	}
	if report.FinderEmail != "finder@example.com" || report.Species != "Dog" ||
		report.Breed != "Golden Retriever" || report.PrimaryColor != "Golden" ||
		report.SecondaryColor != "Cream" || len(report.DistinctiveMarkings) != 1 ||
		report.CustodyStatus != domain.CustodyLocalShelter || report.Status != domain.FoundPetStatusFound ||
		report.GeocodingStatus != domain.GeocodingPending || report.Coordinates != nil {
		t.Fatalf("persisted found-pet report = %#v", report)
	}

	publicRequest := httptest.NewRequest(http.MethodGet, "/api/v1/found-pets", nil)
	publicRecorder := httptest.NewRecorder()
	frontend.ServeHTTP(publicRecorder, publicRequest)
	if publicRecorder.Code != http.StatusOK {
		t.Fatalf("public found-pet status = %d, want 200; body = %s", publicRecorder.Code, publicRecorder.Body.String())
	}
	publicData := append([]byte(nil), publicRecorder.Body.Bytes()...)
	var publicReports []domain.PublicFoundPetReport
	if err := json.Unmarshal(publicData, &publicReports); err != nil {
		t.Fatalf("decode public found-pet reports: %v", err)
	}
	if len(publicReports) != 1 || publicReports[0].Species != "Dog" ||
		publicReports[0].SecondaryColor != "Cream" || publicReports[0].CustodyStatus != domain.CustodyLocalShelter {
		t.Fatalf("public found-pet reports = %#v", publicReports)
	}
	if strings.Contains(string(publicData), "finder@example.com") || strings.Contains(string(publicData), "finderEmail") {
		t.Fatalf("public found-pet response exposed private contact: %s", publicData)
	}
	outboxRecords, err := st.ListState(ctx, store.OutboxCollection)
	if err != nil {
		t.Fatalf("list canonical found-pet outbox: %v", err)
	}
	if len(outboxRecords) != 1 {
		t.Fatalf("found-pet outbox record count = %d, want 1", len(outboxRecords))
	}
}

func TestWebFrontendFoundPetSubmissionRejectsPrivilegedFieldInjection(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	foundReports := foundpet.NewReportService(st, pubsub.NewMemoryPubSub())
	frontend := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		FoundPetReporter: foundReports,
	})

	objectOnlyPayload := `{"petId":"found-object-only","imageObject":"images/found-pets/other/image.jpg","location":"Seattle, WA"}`
	objectOnlyRequest := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(objectOnlyPayload))
	objectOnlyRecorder := httptest.NewRecorder()
	frontend.ServeHTTP(objectOnlyRecorder, objectOnlyRequest)
	if objectOnlyRecorder.Code != http.StatusBadRequest {
		t.Fatalf("object-only browser report status = %d, want 400; body = %s", objectOnlyRecorder.Code, objectOnlyRecorder.Body.String())
	}
	if _, err := st.GetState(ctx, store.FoundPetsCollection, "found-object-only"); !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		t.Fatalf("object-only browser report mutated state: %v", err)
	}

	spoofedGeoPayload := `{"petId":"found-spoofed-geo","imageUrl":"https://storage.petspotr.io/found.jpg","imageObject":"images/found-pets/other/image.jpg","location":"Seattle, WA","geocodingStatus":"verified","coordinates":{"latitude":1,"longitude":2}}`
	spoofedGeoRequest := httptest.NewRequest(http.MethodPost, "/api/v1/found-pets", strings.NewReader(spoofedGeoPayload))
	spoofedGeoRecorder := httptest.NewRecorder()
	frontend.ServeHTTP(spoofedGeoRecorder, spoofedGeoRequest)
	if spoofedGeoRecorder.Code != http.StatusCreated {
		t.Fatalf("spoofed-geocoding browser report status = %d, want 201; body = %s", spoofedGeoRecorder.Code, spoofedGeoRecorder.Body.String())
	}
	stateData, err := st.GetState(ctx, store.FoundPetsCollection, "found-spoofed-geo")
	if err != nil {
		t.Fatal(err)
	}
	var report domain.FoundPetReport
	if err := json.Unmarshal(stateData, &report); err != nil {
		t.Fatal(err)
	}
	if report.ImageObject != "" || report.GeocodingStatus != domain.GeocodingPending || report.Coordinates != nil {
		t.Fatalf("browser report accepted privileged fields: %#v", report)
	}
}

func TestFoundPetHTTPAdaptersShareCanonicalContract(t *testing.T) {
	foundAt := time.Date(2026, time.August, 15, 14, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		path string
		body func(petID, imageURL, location, finderEmail string) string
	}{
		{
			name: "direct service",
			path: "/foundPet",
			body: func(petID, imageURL, location, finderEmail string) string {
				data, err := json.Marshal(map[string]any{
					"petId": petID, "imageUrl": imageURL, "foundAt": foundAt, "location": location,
					"finderEmail": finderEmail, "species": "Dog", "breed": "Golden Retriever",
					"primaryColor": "Golden", "secondaryColor": "Cream",
					"distinctiveMarkings": []string{"White chest patch"}, "custodyStatus": "Finder Home",
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
			body: func(petID, imageURL, location, finderEmail string) string {
				data, err := json.Marshal(map[string]any{
					"petId": petID, "imageUrl": imageURL, "foundAt": foundAt,
					"location": location, "finderEmail": finderEmail, "species": "Dog",
					"breed": "Golden Retriever", "primaryColor": "Golden", "secondaryColor": "Cream",
					"distinctiveMarkings": []string{"White chest patch"}, "custodyStatus": "Finder Home",
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

			published := make(chan domain.FoundPetReportedV2, 4)
			if err := ps.Subscribe("foundPet", func(_ context.Context, data []byte) error {
				var event domain.FoundPetReportedV2
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
				name        string
				imageURL    string
				location    string
				finderEmail string
			}{
				{name: "missing image", location: "Seattle, WA", finderEmail: "finder@example.com"},
				{name: "missing location", imageURL: "https://storage.petspotr.io/images/found-invalid.jpg", finderEmail: "finder@example.com"},
				{name: "invalid finder email", imageURL: "https://storage.petspotr.io/images/found-invalid.jpg", location: "Seattle, WA", finderEmail: "invalid"},
			}
			for _, invalidCase := range invalidCases {
				invalid := submit(tt.body(
					"found-invalid-"+strings.ReplaceAll(invalidCase.name, " ", "-"),
					invalidCase.imageURL,
					invalidCase.location,
					invalidCase.finderEmail,
				))
				if invalid.Code != http.StatusBadRequest {
					t.Fatalf("%s response status = %d, want %d; body = %s", invalidCase.name, invalid.Code, http.StatusBadRequest, invalid.Body.String())
				}
			}

			validBody := tt.body("found-stable", "https://storage.petspotr.io/images/found-stable.jpg", "Seattle, WA", "finder@example.com")
			first := submit(validBody)
			if first.Code != http.StatusCreated {
				t.Fatalf("first create status = %d, want %d; body = %s", first.Code, http.StatusCreated, first.Body.String())
			}
			retry := submit(validBody)
			if retry.Code != http.StatusCreated {
				t.Fatalf("exact retry status = %d, want %d; body = %s", retry.Code, http.StatusCreated, retry.Body.String())
			}
			conflict := submit(tt.body("found-stable", "https://storage.petspotr.io/images/found-stable.jpg", "Portland, OR", "finder@example.com"))
			if conflict.Code != http.StatusConflict {
				t.Fatalf("conflicting retry status = %d, want %d; body = %s", conflict.Code, http.StatusConflict, conflict.Body.String())
			}
			distinct := submit(tt.body("found-distinct", "https://storage.petspotr.io/images/found-stable.jpg", "Seattle, WA", "finder@example.com"))
			if distinct.Code != http.StatusCreated {
				t.Fatalf("distinct report status = %d, want %d; body = %s", distinct.Code, http.StatusCreated, distinct.Body.String())
			}

			if got := len(published); got != 2 {
				t.Fatalf("published event count = %d, want 2", got)
			}
			for len(published) > 0 {
				event := <-published
				if event.Species != "Dog" || event.Breed != "Golden Retriever" ||
					event.PrimaryColor != "Golden" || event.SecondaryColor != "Cream" ||
					len(event.DistinctiveMarkings) != 1 || event.CustodyStatus != domain.CustodyFinderHome ||
					event.GeocodingStatus != domain.GeocodingPending || event.Status != domain.FoundPetStatusFound {
					t.Fatalf("published event = %#v", event)
				}
			}
			stateData, err := st.GetState(ctx, store.FoundPetsCollection, "found-stable")
			if err != nil {
				t.Fatal(err)
			}
			var report domain.FoundPetReport
			if err := json.Unmarshal(stateData, &report); err != nil {
				t.Fatal(err)
			}
			if report.FinderEmail != "finder@example.com" || report.Species != "Dog" ||
				report.Breed != "Golden Retriever" || report.SecondaryColor != "Cream" ||
				len(report.DistinctiveMarkings) != 1 || report.CustodyStatus != domain.CustodyFinderHome {
				t.Fatalf("persisted report = %#v", report)
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
