package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sighting"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestSightingEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	// Seed lost pet
	lostPet := domain.LostPetRecord{
		PetID:      "lost-target-1",
		PetName:    "Buddy",
		ReportedAt: time.Now().UTC().Add(-2 * time.Hour),
		Location:   "Capitol Hill",
		Coordinates: &domain.LocationPoint{
			Latitude:  47.620,
			Longitude: -122.320,
		},
		Status: domain.LostPetStatusLost,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	var createdSightingID string

	t.Run("POST /api/v1/lost-pets/{id}/sightings creates sighting record", func(t *testing.T) {
		body := map[string]interface{}{
			"locationDescription": "Spotted near volunteer park",
			"sightedAt":           time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339),
			"coordinates": map[string]float64{
				"latitude":  47.630,
				"longitude": -122.315,
			},
			"movementDirection": "North",
			"notes":             "Looking energetic",
			"reporterContact": map[string]string{
				"name":  "Jane Witness",
				"phone": "206-555-0199",
			},
		}
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-target-1/sightings", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var record domain.PetSightingRecord
		if err := json.Unmarshal(rec.Body.Bytes(), &record); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if record.SightingID == "" {
			t.Fatal("expected non-empty sightingId")
		}
		if record.LostPetID != "lost-target-1" {
			t.Errorf("expected lostPetId 'lost-target-1', got %q", record.LostPetID)
		}
		if record.Status != domain.SightingStatusActive {
			t.Errorf("expected status active, got %q", record.Status)
		}
		if record.Coordinates == nil || record.Coordinates.Latitude != 47.630 || record.Coordinates.Longitude != -122.315 {
			t.Errorf("unexpected coordinates: %+v", record.Coordinates)
		}
		createdSightingID = record.SightingID

		// Verify persisted in store
		storedBytes, err := st.GetState(context.Background(), store.SightingsCollection, createdSightingID)
		if err != nil {
			t.Fatalf("sighting was not persisted to store: %v", err)
		}
		var storedRecord domain.PetSightingRecord
		if err := json.Unmarshal(storedBytes, &storedRecord); err != nil {
			t.Fatalf("failed to unmarshal stored record: %v", err)
		}
		if storedRecord.SightingID != createdSightingID {
			t.Errorf("stored sighting ID mismatch: got %q, want %q", storedRecord.SightingID, createdSightingID)
		}
	})

	t.Run("GET /api/v1/lost-pets/{id}/sightings lists active sightings chronologically", func(t *testing.T) {
		// Seed an earlier sighting
		earlierSighting := domain.PetSightingRecord{
			SightingID:          "sight-earlier",
			LostPetID:           "lost-target-1",
			ReportedAt:          time.Now().UTC().Add(-30 * time.Minute),
			SightedAt:           time.Now().UTC().Add(-45 * time.Minute),
			LocationDescription: "Near Broadway",
			Coordinates: &domain.LocationPoint{
				Latitude:  47.625,
				Longitude: -122.318,
			},
			Status: domain.SightingStatusActive,
		}
		sBytes, _ := json.Marshal(earlierSighting)
		_ = st.SaveState(context.Background(), store.SightingsCollection, earlierSighting.SightingID, sBytes)

		// Seed a dismissed sighting (should be filtered out)
		dismissedSighting := domain.PetSightingRecord{
			SightingID:          "sight-dismissed",
			LostPetID:           "lost-target-1",
			ReportedAt:          time.Now().UTC().Add(-10 * time.Minute),
			SightedAt:           time.Now().UTC().Add(-20 * time.Minute),
			LocationDescription: "False alarm sighting",
			Coordinates: &domain.LocationPoint{
				Latitude:  47.622,
				Longitude: -122.316,
			},
			Status: domain.SightingStatusDismissed,
		}
		dBytes, _ := json.Marshal(dismissedSighting)
		_ = st.SaveState(context.Background(), store.SightingsCollection, dismissedSighting.SightingID, dBytes)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-target-1/sightings", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		var list []domain.PetSightingRecord
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("failed to decode sightings list: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("expected 2 active sightings, got %d", len(list))
		}
		// SightedAt chronological order
		if list[0].SightingID != "sight-earlier" {
			t.Errorf("expected first sighting to be 'sight-earlier', got %q", list[0].SightingID)
		}
		if list[1].SightingID != createdSightingID {
			t.Errorf("expected second sighting to be %q, got %q", createdSightingID, list[1].SightingID)
		}
	})

	t.Run("GET /api/v1/lost-pets/{id}/sightings returns empty array when none exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-other-pet/sightings", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		if body := bytes.TrimSpace(rec.Body.Bytes()); string(body) != "[]" {
			t.Fatalf("expected empty array '[]', got %s", string(body))
		}
	})

	t.Run("GET /api/v1/lost-pets/{id}/trajectory returns computed path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-target-1/trajectory", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var traj sighting.TrajectoryAnalysis
		if err := json.Unmarshal(rec.Body.Bytes(), &traj); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if traj.LostPetID != "lost-target-1" {
			t.Errorf("expected lostPetId 'lost-target-1', got %q", traj.LostPetID)
		}
		if traj.SightingsCount != 2 {
			t.Errorf("expected sightingsCount 2, got %d", traj.SightingsCount)
		}
		if len(traj.Legs) < 1 {
			t.Errorf("expected at least 1 trajectory leg, got %d", len(traj.Legs))
		}
		if traj.EstimatedPerimeter == nil {
			t.Error("expected estimatedPerimeter in trajectory response")
		}
	})

	t.Run("GET /api/v1/lost-pets/{id}/trajectory returns 404 for missing pet", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/non-existent-pet/trajectory", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})
}

func TestSightingValidation(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	// Seed lost pet
	lostPet := domain.LostPetRecord{
		PetID:      "lost-valid-1",
		PetName:    "Charlie",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{
			Latitude:  47.61,
			Longitude: -122.33,
		},
		Status: domain.LostPetStatusLost,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	validPayload := func() map[string]interface{} {
		return map[string]interface{}{
			"locationDescription": "Valid location",
			"sightedAt":           time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339),
			"coordinates": map[string]float64{
				"latitude":  47.615,
				"longitude": -122.325,
			},
			"movementDirection": "East",
			"notes":             "Spotted near cafe",
		}
	}

	t.Run("POST returns 404 if lost pet does not exist", func(t *testing.T) {
		body := validPayload()
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/unknown-pet/sightings", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("POST returns 400 for invalid JSON", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-valid-1/sightings", bytes.NewReader([]byte("{invalid-json")))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("POST returns 400 for nil coordinates", func(t *testing.T) {
		body := validPayload()
		delete(body, "coordinates")
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-valid-1/sightings", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("POST returns 400 for invalid latitude", func(t *testing.T) {
		body := validPayload()
		body["coordinates"] = map[string]float64{
			"latitude":  95.0,
			"longitude": -122.325,
		}
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-valid-1/sightings", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("POST returns 400 for invalid longitude", func(t *testing.T) {
		body := validPayload()
		body["coordinates"] = map[string]float64{
			"latitude":  47.615,
			"longitude": -195.0,
		}
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-valid-1/sightings", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("POST returns 400 for missing sightedAt", func(t *testing.T) {
		body := validPayload()
		delete(body, "sightedAt")
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-valid-1/sightings", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("POST returns 400 for sightedAt in future", func(t *testing.T) {
		body := validPayload()
		body["sightedAt"] = time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-valid-1/sightings", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Method dispatch on /sightings rejects unsupported methods", func(t *testing.T) {
		for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodPatch} {
			req := httptest.NewRequest(method, "/api/v1/lost-pets/lost-valid-1/sightings", nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s /sightings: expected 405 Method Not Allowed, got %d", method, rec.Code)
			}
		}
	})

	t.Run("Method dispatch on /trajectory rejects non-GET methods", func(t *testing.T) {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			req := httptest.NewRequest(method, "/api/v1/lost-pets/lost-valid-1/trajectory", nil)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s /trajectory: expected 405 Method Not Allowed, got %d", method, rec.Code)
			}
		}
	})
}

func TestSightingRealtimeNotification(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	hub := webfrontend.NewReunionHub()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
		ReunionHub:               hub,
	})

	// Seed lost pet
	lostPet := domain.LostPetRecord{
		PetID:       "lost-target-rt",
		PetName:     "Milo",
		ReportedAt:  time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{Latitude: 47.60, Longitude: -122.33},
		Status:      domain.LostPetStatusLost,
	}
	data, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, data)

	// Seed a match linked to lost pet
	match := domain.MatchRecord{
		MatchID:    "match-rt-1",
		LostPetID:  "lost-target-rt",
		FoundPetID: "found-target-rt",
		Status:     domain.MatchStatusPendingReview,
	}
	mData, _ := json.Marshal(match)
	_ = st.SaveState(context.Background(), store.MatchesCollection, match.MatchID, mData)

	// Subscribe to petID and matchID
	petSubCh, petUnsub := hub.Subscribe("lost-target-rt")
	defer petUnsub()

	matchSubCh, matchUnsub := hub.Subscribe("match-rt-1")
	defer matchUnsub()

	// Post sighting
	payload, _ := json.Marshal(map[string]interface{}{
		"locationDescription": "Spotted near market",
		"sightedAt":           time.Now().UTC().Format(time.RFC3339),
		"coordinates":         map[string]float64{"latitude": 47.61, "longitude": -122.34},
		"movementDirection":   "Northeast",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-target-rt/sightings", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify notification persisted in store.NotificationsCollection
	notifications, err := st.ListState(context.Background(), store.NotificationsCollection)
	if err != nil || len(notifications) == 0 {
		t.Fatalf("expected notification created in store, got %d (err: %v)", len(notifications), err)
	}

	var notif domain.NotificationItem
	for _, raw := range notifications {
		if err := json.Unmarshal(raw, &notif); err != nil {
			t.Fatalf("failed to unmarshal notification: %v", err)
		}
		break
	}
	if notif.PetID != "lost-target-rt" {
		t.Errorf("expected PetID 'lost-target-rt', got %q", notif.PetID)
	}
	if notif.Type != "sighting_reported" {
		t.Errorf("expected Type 'sighting_reported', got %q", notif.Type)
	}
	if notif.Read {
		t.Error("expected Read to be false")
	}
	if notif.Title == "" || notif.Message == "" {
		t.Errorf("expected non-empty Title and Message, got title=%q, message=%q", notif.Title, notif.Message)
	}

	// Verify SSE event broadcast on pet subscription channel
	select {
	case evt := <-petSubCh:
		if evt.Type != "sighting" && evt.Type != domain.ReunionEventSighting {
			t.Errorf("expected event Type 'sighting', got %q", evt.Type)
		}
		if evt.MatchID != "lost-target-rt" {
			t.Errorf("expected event MatchID 'lost-target-rt', got %q", evt.MatchID)
		}
		payload, ok := evt.Payload.(domain.SightingEventPayload)
		if !ok {
			t.Fatalf("expected payload type domain.SightingEventPayload, got %T", evt.Payload)
		}
		if payload.PetID != "lost-target-rt" {
			t.Errorf("expected payload PetID 'lost-target-rt', got %q", payload.PetID)
		}
		if payload.LocationDescription != "Spotted near market" {
			t.Errorf("expected locationDescription 'Spotted near market', got %q", payload.LocationDescription)
		}
		if payload.MovementDirection != "Northeast" {
			t.Errorf("expected movementDirection 'Northeast', got %q", payload.MovementDirection)
		}
		if payload.Coordinates == nil || payload.Coordinates.Latitude != 47.61 || payload.Coordinates.Longitude != -122.34 {
			t.Errorf("unexpected payload coordinates: %+v", payload.Coordinates)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for sighting broadcast on pet subscriber channel")
	}

	// Verify SSE event broadcast on match subscription channel
	select {
	case evt := <-matchSubCh:
		if evt.Type != "sighting" && evt.Type != domain.ReunionEventSighting {
			t.Errorf("expected event Type 'sighting', got %q", evt.Type)
		}
		if evt.MatchID != "match-rt-1" {
			t.Errorf("expected event MatchID 'match-rt-1', got %q", evt.MatchID)
		}
		payload, ok := evt.Payload.(domain.SightingEventPayload)
		if !ok {
			t.Fatalf("expected match payload type domain.SightingEventPayload, got %T", evt.Payload)
		}
		if payload.PetID != "lost-target-rt" {
			t.Errorf("expected match payload PetID 'lost-target-rt', got %q", payload.PetID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for sighting broadcast on match subscriber channel")
	}
}
