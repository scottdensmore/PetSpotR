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
	"github.com/scottdensmore/petspotr/pkg/beacon"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestBeaconPings_RESTEndpoints(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// 1. Ingest ping via POST
	payload := map[string]interface{}{
		"volunteerAlias": "Volunteer Alpha",
		"observerCoords": map[string]float64{"latitude": 47.6062, "longitude": -122.3321},
		"rssi":           -64,
		"txPower1m":      -59,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/pet-test-1/beacon-pings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	var postResp struct {
		Ping          beacon.BeaconPing          `json:"ping"`
		Triangulation beacon.TriangulationResult `json:"triangulation"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &postResp); err != nil {
		t.Fatalf("failed to parse POST response: %v", err)
	}

	if postResp.Ping.PetID != "pet-test-1" {
		t.Errorf("expected PetID pet-test-1, got %q", postResp.Ping.PetID)
	}
	if postResp.Ping.PingID == "" {
		t.Error("expected non-empty PingID")
	}
	if postResp.Ping.RSSI != -64 {
		t.Errorf("expected RSSI -64, got %d", postResp.Ping.RSSI)
	}
	if postResp.Ping.TxPower1m != -59 {
		t.Errorf("expected TxPower1m -59, got %d", postResp.Ping.TxPower1m)
	}
	if postResp.Ping.DistanceMeters <= 0 {
		t.Errorf("expected positive distance, got %f", postResp.Ping.DistanceMeters)
	}
	if postResp.Triangulation.ObservationCount != 1 {
		t.Errorf("expected 1 observation, got %d", postResp.Triangulation.ObservationCount)
	}
	if postResp.Triangulation.Proximity != beacon.ProximityNear {
		t.Errorf("expected near proximity, got %s", postResp.Triangulation.Proximity)
	}

	// 2. Fetch triangulation via GET
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/search-parties/pet-test-1/beacon-triangulation", nil)
	getW := httptest.NewRecorder()
	server.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", getW.Code, getW.Body.String())
	}

	var getResp struct {
		PetID         string                     `json:"petId"`
		Triangulation beacon.TriangulationResult `json:"triangulation"`
		RecentPings   []beacon.BeaconPing        `json:"recentPings"`
	}
	if err := json.Unmarshal(getW.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to parse GET response: %v", err)
	}

	if getResp.PetID != "pet-test-1" {
		t.Errorf("expected petId pet-test-1, got %q", getResp.PetID)
	}
	if getResp.Triangulation.ObservationCount != 1 {
		t.Errorf("expected 1 observation in GET, got %d", getResp.Triangulation.ObservationCount)
	}
	if len(getResp.RecentPings) != 1 {
		t.Fatalf("expected 1 recent ping, got %d", len(getResp.RecentPings))
	}
	if getResp.RecentPings[0].PingID != postResp.Ping.PingID {
		t.Errorf("expected recent ping ID %q, got %q", postResp.Ping.PingID, getResp.RecentPings[0].PingID)
	}
}

func TestBeaconPings_Validation(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	testCases := []struct {
		name       string
		method     string
		path       string
		payload    interface{}
		expectCode int
	}{
		{
			name:       "invalid observer coordinates",
			method:     http.MethodPost,
			path:       "/api/v1/search-parties/pet-val-1/beacon-pings",
			payload:    map[string]interface{}{"observerCoords": map[string]float64{"latitude": 95.0, "longitude": -122.3321}, "rssi": -65},
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "rssi too high (positive)",
			method:     http.MethodPost,
			path:       "/api/v1/search-parties/pet-val-1/beacon-pings",
			payload:    map[string]interface{}{"observerCoords": map[string]float64{"latitude": 47.6062, "longitude": -122.3321}, "rssi": 10},
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "rssi too low (below -120)",
			method:     http.MethodPost,
			path:       "/api/v1/search-parties/pet-val-1/beacon-pings",
			payload:    map[string]interface{}{"observerCoords": map[string]float64{"latitude": 47.6062, "longitude": -122.3321}, "rssi": -130},
			expectCode: http.StatusBadRequest,
		},
		{
			name:       "invalid method for pings endpoint",
			method:     http.MethodPut,
			path:       "/api/v1/search-parties/pet-val-1/beacon-pings",
			payload:    map[string]interface{}{},
			expectCode: http.StatusMethodNotAllowed,
		},
		{
			name:       "invalid method for triangulation endpoint",
			method:     http.MethodPost,
			path:       "/api/v1/search-parties/pet-val-1/beacon-triangulation",
			payload:    map[string]interface{}{},
			expectCode: http.StatusMethodNotAllowed,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			if tc.payload != nil {
				body, _ = json.Marshal(tc.payload)
			}
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			server.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("expected HTTP %d, got %d (body: %s)", tc.expectCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestBeaconPings_MultiPingTriangulation(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)
	petID := "pet-multi-tri"

	pings := []struct {
		lat  float64
		lon  float64
		rssi int
	}{
		{lat: 47.6000, lon: -122.3300, rssi: -65},
		{lat: 47.6001, lon: -122.3300, rssi: -62},
		{lat: 47.60005, lon: -122.3301, rssi: -60},
	}

	for i, p := range pings {
		body, _ := json.Marshal(map[string]interface{}{
			"volunteerAlias": "Volunteer",
			"observerCoords": map[string]float64{"latitude": p.lat, "longitude": p.lon},
			"rssi":           p.rssi,
			"txPower1m":      -59,
		})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+petID+"/beacon-pings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("ping %d failed with code %d: %s", i, w.Code, w.Body.String())
		}
	}

	// Fetch triangulation
	req := httptest.NewRequest(http.MethodGet, "/api/v1/search-parties/"+petID+"/beacon-triangulation", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		PetID         string                     `json:"petId"`
		Triangulation beacon.TriangulationResult `json:"triangulation"`
		RecentPings   []beacon.BeaconPing        `json:"recentPings"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Triangulation.ObservationCount != 3 {
		t.Errorf("expected 3 observations, got %d", resp.Triangulation.ObservationCount)
	}
	if resp.Triangulation.ConfidenceScore < 0.7 {
		t.Errorf("expected confidence score >= 0.7, got %f", resp.Triangulation.ConfidenceScore)
	}
	if len(resp.RecentPings) != 3 {
		t.Errorf("expected 3 recent pings, got %d", len(resp.RecentPings))
	}
}

func TestBeaconPings_SSEBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	hub := webfrontend.NewReunionHub()
	server := webfrontend.NewServerWithOptions(memStore, webfrontend.ServerOptions{
		ReunionHub:               hub,
		AllowPrivilegedMutations: true,
		DisableRateLimiting:      true,
	})

	petID := "pet-sse-123"
	subCh, unsub := hub.Subscribe(petID)
	defer unsub()

	// Post ping
	body, _ := json.Marshal(map[string]interface{}{
		"volunteerAlias": "Volunteer Gamma",
		"observerCoords": map[string]float64{"latitude": 47.6062, "longitude": -122.3321},
		"rssi":           -64,
		"txPower1m":      -59,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/"+petID+"/beacon-pings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	select {
	case event := <-subCh:
		if event.Type != domain.ReunionEventBeaconPing {
			t.Errorf("expected event type %q, got %q", domain.ReunionEventBeaconPing, event.Type)
		}
		if event.MatchID != petID {
			t.Errorf("expected MatchID %q, got %q", petID, event.MatchID)
		}
		payload, ok := event.Payload.(domain.BeaconPingEventPayload)
		if !ok {
			t.Fatalf("expected payload type domain.BeaconPingEventPayload, got %T", event.Payload)
		}
		if payload.Type != string(domain.ReunionEventBeaconPing) {
			t.Errorf("expected payload.Type %q, got %q", domain.ReunionEventBeaconPing, payload.Type)
		}
		if payload.PetID != petID {
			t.Errorf("expected payload.PetID %q, got %q", petID, payload.PetID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for beacon_ping SSE event")
	}
}

func TestSearchParty_IncludesCollarBeacon(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	petID := "pet-with-beacon-tag"
	major := uint16(100)
	minor := uint16(200)
	pet := domain.LostPetRecord{
		PetID:       petID,
		PetName:     "Ziggy",
		Coordinates: &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		ReportedAt:  time.Now().UTC(),
		Status:      domain.LostPetStatusLost,
		CollarBeacon: &domain.CollarBeaconConfig{
			Protocol:       domain.BeaconProtocolIBeacon,
			UUID:           "FDA50693-A4E2-4FB1-AFCF-C6EB07647825",
			Major:          &major,
			Minor:          &minor,
			DeviceName:     "PetSpotR-Ziggy",
			CalibratedRSSI: -59,
		},
	}
	petBytes, _ := json.Marshal(pet)
	_ = memStore.SaveState(context.Background(), store.LostPetsCollection, petID, petBytes)

	// Create search party
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/"+petID+"/search-party", nil)
	createW := httptest.NewRecorder()
	server.ServeHTTP(createW, createReq)

	if createW.Code != http.StatusCreated {
		t.Fatalf("failed to create search party: %d: %s", createW.Code, createW.Body.String())
	}

	var createResp webfrontend.SearchPartyResponse
	if err := json.Unmarshal(createW.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to unmarshal create response: %v", err)
	}
	if createResp.CollarBeacon == nil {
		t.Fatal("expected CollarBeacon in create response, got nil")
	}
	if createResp.CollarBeacon.DeviceName != "PetSpotR-Ziggy" {
		t.Errorf("expected DeviceName PetSpotR-Ziggy, got %q", createResp.CollarBeacon.DeviceName)
	}

	// GET search party
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/"+petID+"/search-party", nil)
	getW := httptest.NewRecorder()
	server.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("failed to get search party: %d: %s", getW.Code, getW.Body.String())
	}

	var getResp webfrontend.SearchPartyResponse
	if err := json.Unmarshal(getW.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("failed to unmarshal get response: %v", err)
	}
	if getResp.CollarBeacon == nil {
		t.Fatal("expected CollarBeacon in GET response, got nil")
	}
	if getResp.CollarBeacon.UUID != "FDA50693-A4E2-4FB1-AFCF-C6EB07647825" {
		t.Errorf("expected UUID 'FDA50693-A4E2-4FB1-AFCF-C6EB07647825', got %q", getResp.CollarBeacon.UUID)
	}
}
