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
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func setupTestServerWithParty(t *testing.T) (*webfrontend.Server, store.StateStore, string, string) {
	t.Helper()

	memStore := store.NewMemoryStore()
	petID := "pet-breadcrumbs-123"
	sectorID := "sector-1"

	// Seed lost pet
	pet := domain.LostPetRecord{
		PetID:       petID,
		PetName:     "Kona",
		Coordinates: &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330},
		ReportedAt:  time.Now().UTC(),
		Status:      domain.LostPetStatusLost,
	}
	petBytes, err := json.Marshal(pet)
	if err != nil {
		t.Fatalf("failed to marshal pet: %v", err)
	}
	if err := memStore.SaveState(context.Background(), store.LostPetsCollection, petID, petBytes); err != nil {
		t.Fatalf("failed to save pet: %v", err)
	}

	// Seed search party
	partyID := "party-" + petID
	center := *pet.Coordinates
	sectors := searchparty.DecomposePerimeter(&center, 1000.0, 4)
	party := searchparty.SearchParty{
		PartyID:           partyID,
		LostPetID:         petID,
		CenterCoordinates: center,
		RadiusMeters:      1000.0,
		CreatedAt:         time.Now().UTC(),
		Sectors:           sectors,
	}
	partyBytes, err := json.Marshal(party)
	if err != nil {
		t.Fatalf("failed to marshal party: %v", err)
	}
	if err := memStore.SaveState(context.Background(), store.SearchPartiesCollection, partyID, partyBytes); err != nil {
		t.Fatalf("failed to save party: %v", err)
	}

	server := webfrontend.NewServerWithOptions(memStore, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	return server, memStore, petID, sectorID
}

func TestApiSectorBreadcrumbs_POST_Success(t *testing.T) {
	t.Parallel()

	server, _, petID, sectorID := setupTestServerWithParty(t)

	now := time.Now().UTC()
	reqPayload := map[string]any{
		"volunteerAlias": "Trail Scout #42",
		"points": []map[string]any{
			{
				"latitude":       47.6010,
				"longitude":      -122.3290,
				"timestamp":      now.Format(time.RFC3339Nano),
				"accuracyMeters": 4.5,
			},
			{
				"latitude":       47.6020,
				"longitude":      -122.3280,
				"timestamp":      now.Add(15 * time.Second).Format(time.RFC3339Nano),
				"accuracyMeters": 4.2,
			},
		},
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d, body: %s", w.Code, w.Body.String())
	}

	var trail searchparty.VolunteerBreadcrumbTrail
	if err := json.Unmarshal(w.Body.Bytes(), &trail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if trail.TrailID == "" {
		t.Errorf("expected non-empty trailId")
	}
	if trail.SectorID != sectorID {
		t.Errorf("expected sectorId %s, got %s", sectorID, trail.SectorID)
	}
	if trail.VolunteerAlias != "Trail Scout #42" {
		t.Errorf("expected alias %s, got %s", "Trail Scout #42", trail.VolunteerAlias)
	}
	if len(trail.Points) != 2 {
		t.Errorf("expected 2 points, got %d", len(trail.Points))
	}
	if trail.TotalDistanceM <= 0 {
		t.Errorf("expected positive totalDistanceM, got %f", trail.TotalDistanceM)
	}
	if trail.DurationSeconds != 15 {
		t.Errorf("expected durationSeconds 15, got %d", trail.DurationSeconds)
	}
}

func TestApiSectorBreadcrumbs_POST_AppendToExistingTrail(t *testing.T) {
	t.Parallel()

	server, _, petID, sectorID := setupTestServerWithParty(t)

	now := time.Now().UTC()
	firstBatch := map[string]any{
		"trailId":        "trail-kona-001",
		"volunteerAlias": "Trail Scout #42",
		"points": []map[string]any{
			{
				"latitude":       47.6010,
				"longitude":      -122.3290,
				"timestamp":      now.Format(time.RFC3339Nano),
				"accuracyMeters": 4.5,
			},
		},
	}
	firstBody, _ := json.Marshal(firstBatch)
	req1 := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
		bytes.NewReader(firstBody),
	)
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	server.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first batch expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	// Send second batch with same trailId
	secondBatch := map[string]any{
		"trailId":        "trail-kona-001",
		"volunteerAlias": "Trail Scout #42",
		"points": []map[string]any{
			{
				"latitude":       47.6025,
				"longitude":      -122.3275,
				"timestamp":      now.Add(30 * time.Second).Format(time.RFC3339Nano),
				"accuracyMeters": 3.8,
			},
		},
	}
	secondBody, _ := json.Marshal(secondBatch)
	req2 := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
		bytes.NewReader(secondBody),
	)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("second batch expected 201, got %d: %s", w2.Code, w2.Body.String())
	}

	var trail searchparty.VolunteerBreadcrumbTrail
	if err := json.Unmarshal(w2.Body.Bytes(), &trail); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if trail.TrailID != "trail-kona-001" {
		t.Errorf("expected trailId trail-kona-001, got %s", trail.TrailID)
	}
	if len(trail.Points) != 2 {
		t.Errorf("expected 2 points after append, got %d", len(trail.Points))
	}
	if trail.TotalDistanceM <= 0 {
		t.Errorf("expected positive distance, got %f", trail.TotalDistanceM)
	}
	if trail.DurationSeconds != 30 {
		t.Errorf("expected durationSeconds 30, got %d", trail.DurationSeconds)
	}
}

func TestApiSectorBreadcrumbs_POST_ValidationErrors(t *testing.T) {
	t.Parallel()

	server, _, petID, sectorID := setupTestServerWithParty(t)

	t.Run("empty points array", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"points": []any{},
		})
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
			bytes.NewReader(body),
		)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for empty points, got %d", w.Code)
		}
	})

	t.Run("invalid point coordinates", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"points": []map[string]any{
				{
					"latitude":  95.0,
					"longitude": -122.3,
					"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				},
			},
		})
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
			bytes.NewReader(body),
		)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for invalid coordinates, got %d", w.Code)
		}
	})

	t.Run("non-existent pet / search party", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"points": []map[string]any{
				{
					"latitude":  47.6,
					"longitude": -122.3,
					"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				},
			},
		})
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/search-parties/non-existent-pet/sectors/"+sectorID+"/breadcrumbs",
			bytes.NewReader(body),
		)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for non-existent pet, got %d", w.Code)
		}
	})

	t.Run("non-existent sector ID", func(t *testing.T) {
		body, _ := json.Marshal(map[string]any{
			"points": []map[string]any{
				{
					"latitude":  47.6,
					"longitude": -122.3,
					"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
				},
			},
		})
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/v1/search-parties/"+petID+"/sectors/non-existent-sector/breadcrumbs",
			bytes.NewReader(body),
		)
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for non-existent sector, got %d", w.Code)
		}
	})
}

func TestApiSectorBreadcrumbs_GET_Success(t *testing.T) {
	t.Parallel()

	server, _, petID, sectorID := setupTestServerWithParty(t)

	// Before recording, should return empty array
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
		nil,
	)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var trails []searchparty.VolunteerBreadcrumbTrail
	if err := json.Unmarshal(w.Body.Bytes(), &trails); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(trails) != 0 {
		t.Errorf("expected 0 trails, got %d", len(trails))
	}

	// Now post a trail
	now := time.Now().UTC()
	postPayload, _ := json.Marshal(map[string]any{
		"trailId":        "trail-get-test",
		"volunteerAlias": "Trail Scout #1",
		"points": []map[string]any{
			{
				"latitude":       47.601,
				"longitude":      -122.329,
				"timestamp":      now.Format(time.RFC3339Nano),
				"accuracyMeters": 4.0,
			},
			{
				"latitude":       47.602,
				"longitude":      -122.328,
				"timestamp":      now.Add(10 * time.Second).Format(time.RFC3339Nano),
				"accuracyMeters": 4.0,
			},
		},
	})
	postReq := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
		bytes.NewReader(postPayload),
	)
	postReq.Header.Set("Content-Type", "application/json")
	postW := httptest.NewRecorder()
	server.ServeHTTP(postW, postReq)
	if postW.Code != http.StatusCreated {
		t.Fatalf("expected 201 on post, got %d: %s", postW.Code, postW.Body.String())
	}

	// GET again
	getReq := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
		nil,
	)
	getW := httptest.NewRecorder()
	server.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 on get, got %d: %s", getW.Code, getW.Body.String())
	}

	if err := json.Unmarshal(getW.Body.Bytes(), &trails); err != nil {
		t.Fatalf("failed to decode GET response: %v", err)
	}
	if len(trails) != 1 {
		t.Fatalf("expected 1 trail, got %d", len(trails))
	}
	if trails[0].TrailID != "trail-get-test" {
		t.Errorf("expected trailId trail-get-test, got %s", trails[0].TrailID)
	}
}

func TestApiSectorBreadcrumbs_SSE_Broadcast(t *testing.T) {
	t.Parallel()

	hub := webfrontend.NewReunionHub()
	memStore := store.NewMemoryStore()
	petID := "pet-sse-123"
	sectorID := "sector-1"

	// Seed pet & party
	pet := domain.LostPetRecord{
		PetID:       petID,
		PetName:     "Milo",
		Coordinates: &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330},
		ReportedAt:  time.Now().UTC(),
		Status:      domain.LostPetStatusLost,
	}
	petBytes, _ := json.Marshal(pet)
	_ = memStore.SaveState(context.Background(), store.LostPetsCollection, petID, petBytes)

	partyID := "party-" + petID
	party := searchparty.SearchParty{
		PartyID:           partyID,
		LostPetID:         petID,
		CenterCoordinates: *pet.Coordinates,
		RadiusMeters:      1000.0,
		CreatedAt:         time.Now().UTC(),
		Sectors:           searchparty.DecomposePerimeter(pet.Coordinates, 1000.0, 4),
	}
	partyBytes, _ := json.Marshal(party)
	_ = memStore.SaveState(context.Background(), store.SearchPartiesCollection, partyID, partyBytes)

	server := webfrontend.NewServerWithOptions(memStore, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
		ReunionHub:               hub,
	})

	// Subscribe to petID on hub
	eventCh, unsubscribe := hub.Subscribe(petID)
	defer unsubscribe()

	now := time.Now().UTC()
	reqPayload := map[string]any{
		"volunteerAlias": "Trail Scout #42",
		"points": []map[string]any{
			{
				"latitude":       47.6010,
				"longitude":      -122.3290,
				"timestamp":      now.Format(time.RFC3339Nano),
				"accuracyMeters": 4.5,
			},
		},
	}
	body, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/search-parties/"+petID+"/sectors/"+sectorID+"/breadcrumbs",
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	select {
	case event := <-eventCh:
		if event.Type != domain.ReunionEventBreadcrumbUpdated {
			t.Errorf("expected event type %s, got %s", domain.ReunionEventBreadcrumbUpdated, event.Type)
		}
		if event.MatchID != petID {
			t.Errorf("expected matchID %s, got %s", petID, event.MatchID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SSE breadcrumb_updated event")
	}
}
