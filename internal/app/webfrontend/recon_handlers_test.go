package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestReconMissionsEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Step 1: POST /api/v1/recon/missions (Create)
	missionReq := map[string]interface{}{
		"petId":         "test-pet-101",
		"pilotCallsign": "SkyWatcher-1",
		"droneModel":    "DJI Matrice 30T",
		"searchPartyId": "party-pet-101",
	}
	body, _ := json.Marshal(missionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recon/missions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("expected 200/201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var mission domain.DroneMission
	if err := json.Unmarshal(w.Body.Bytes(), &mission); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if mission.PilotCallsign != "SkyWatcher-1" || mission.PetID != "test-pet-101" {
		t.Errorf("unexpected mission response: %+v", mission)
	}
	if mission.ID == "" {
		t.Error("expected non-empty mission ID")
	}
	if mission.Status != domain.FlightStatusActive {
		t.Errorf("expected ACTIVE status, got %q", mission.Status)
	}

	// Validation: missing required fields
	invalidReq := map[string]interface{}{
		"pilotCallsign": "SkyWatcher-1",
	}
	invBody, _ := json.Marshal(invalidReq)
	reqInv := httptest.NewRequest(http.MethodPost, "/api/v1/recon/missions", bytes.NewReader(invBody))
	reqInv.Header.Set("Content-Type", "application/json")
	wInv := httptest.NewRecorder()
	server.ServeHTTP(wInv, reqInv)
	if wInv.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing petId, got %d", wInv.Code)
	}

	// Step 2: GET /api/v1/recon/missions (List for pet)
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/recon/missions?petId=test-pet-101", nil)
	wGet := httptest.NewRecorder()
	server.ServeHTTP(wGet, reqGet)
	if wGet.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", wGet.Code, wGet.Body.String())
	}

	var missions []domain.DroneMission
	if err := json.Unmarshal(wGet.Body.Bytes(), &missions); err != nil {
		t.Fatalf("failed to decode missions list: %v", err)
	}
	if len(missions) != 1 {
		t.Fatalf("expected 1 mission, got %d", len(missions))
	}
	if missions[0].ID != mission.ID {
		t.Errorf("expected mission ID %s, got %s", mission.ID, missions[0].ID)
	}
}

func TestReconTelemetryParseEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Seed search party with sector near 37.7749, -122.4194
	party := searchparty.SearchParty{
		PartyID:           "party-test-1",
		LostPetID:         "test-pet-101",
		CenterCoordinates: domain.LocationPoint{Latitude: 37.7749, Longitude: -122.4194},
		RadiusMeters:      1000,
		CreatedAt:         time.Now().UTC(),
		Sectors: []searchparty.SearchSector{
			{
				SectorID: "sector-1",
				Name:     "Alpha Sector",
				Status:   searchparty.SectorStatusUnassigned,
				PolygonPoints: []domain.LocationPoint{
					{Latitude: 37.7740, Longitude: -122.4210},
					{Latitude: 37.7760, Longitude: -122.4210},
					{Latitude: 37.7760, Longitude: -122.4180},
					{Latitude: 37.7740, Longitude: -122.4180},
				},
			},
		},
	}
	partyBytes, _ := json.Marshal(party)
	_ = memStore.SaveState(context.Background(), store.SearchPartiesCollection, party.PartyID, partyBytes)

	// Sample DJI SRT log
	sampleSRT := `1
00:00:01,000 --> 00:00:02,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude : 37.774929] [longitude : -122.419416] [rel_alt: 45.200] [heading: 0.0] [pitch: -90.0] [roll: 0.0] [yaw: 0.0]

2
00:00:02,000 --> 00:00:03,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude : 37.775129] [longitude : -122.419416] [rel_alt: 45.200] [heading: 0.0] [pitch: -90.0] [roll: 0.0] [yaw: 0.0]
`

	// 1. JSON body request
	reqPayload := map[string]interface{}{
		"content":       sampleSRT,
		"filename":      "flight.srt",
		"searchPartyId": "party-test-1",
		"petId":         "test-pet-101",
	}
	body, _ := json.Marshal(reqPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recon/telemetry/parse", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var telemResp struct {
		Waypoints         []domain.DroneWaypoint `json:"waypoints"`
		FootprintPolygon  [][]float64            `json:"footprintPolygon"`
		SweptAreaSqMeters float64                `json:"sweptAreaSqMeters"`
		CoveredSectorIDs  []string               `json:"coveredSectorIds"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &telemResp); err != nil {
		t.Fatalf("failed to decode telemetry parse response: %v", err)
	}

	if len(telemResp.Waypoints) != 2 {
		t.Errorf("expected 2 waypoints, got %d", len(telemResp.Waypoints))
	}
	if len(telemResp.FootprintPolygon) < 3 {
		t.Errorf("expected footprint polygon with at least 3 vertices, got %d", len(telemResp.FootprintPolygon))
	}
	if telemResp.SweptAreaSqMeters <= 0 {
		t.Errorf("expected positive swept area, got %f", telemResp.SweptAreaSqMeters)
	}
	if len(telemResp.CoveredSectorIDs) == 0 || telemResp.CoveredSectorIDs[0] != "sector-1" {
		t.Errorf("expected covered sector-1, got %+v", telemResp.CoveredSectorIDs)
	}

	// 2. Multipart form upload
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	part, _ := mw.CreateFormFile("file", "mission_log.srt")
	_, _ = part.Write([]byte(sampleSRT))
	_ = mw.WriteField("searchPartyId", "party-test-1")
	_ = mw.Close()

	reqMP := httptest.NewRequest(http.MethodPost, "/api/v1/recon/telemetry/parse", &b)
	reqMP.Header.Set("Content-Type", mw.FormDataContentType())
	wMP := httptest.NewRecorder()
	server.ServeHTTP(wMP, reqMP)
	if wMP.Code != http.StatusOK {
		t.Fatalf("expected 200 for multipart upload, got %d (body: %s)", wMP.Code, wMP.Body.String())
	}
}

func TestReconThermalScanEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Create test image: 100x100 cold dark background with 6x6 hot white cluster
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
		}
	}
	for y := 47; y <= 53; y++ {
		for x := 47; x <= 53; x++ {
			img.Set(x, y, color.RGBA{R: 245, G: 245, B: 245, A: 255})
		}
	}
	var imgBuf bytes.Buffer
	if err := jpeg.Encode(&imgBuf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("failed to encode jpeg: %v", err)
	}

	// Create mission in store
	mission := domain.DroneMission{
		ID:            "mission-thermal-1",
		PetID:         "test-pet-101",
		PilotCallsign: "SkyWatcher-1",
		DroneModel:    "DJI M30T",
		Status:        domain.FlightStatusActive,
		StartTime:     time.Now().UTC(),
	}
	mBytes, _ := json.Marshal(mission)
	_ = memStore.SaveState(context.Background(), store.CollectionReconMissions, mission.ID, mBytes)

	// Multipart request
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	filePart, _ := writer.CreateFormFile("image", "frame_001.jpg")
	_, _ = filePart.Write(imgBuf.Bytes())
	_ = writer.WriteField("missionId", mission.ID)
	_ = writer.WriteField("petId", mission.PetID)
	_ = writer.WriteField("altitudeMetersAGL", "30.0")
	_ = writer.WriteField("latitude", "37.7749")
	_ = writer.WriteField("longitude", "-122.4194")
	_ = writer.WriteField("gimbalPitchDeg", "-90.0")
	_ = writer.WriteField("palette", "WHITE_HOT")
	_ = writer.Close()

	reqBytes := body.Bytes()
	contentType := writer.FormDataContentType()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/recon/thermal/scan", bytes.NewReader(reqBytes))
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("expected 200/201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var hotspots []domain.ThermalHotspot
	if err := json.Unmarshal(w.Body.Bytes(), &hotspots); err != nil {
		t.Fatalf("failed to decode hotspots response: %v", err)
	}

	if len(hotspots) == 0 {
		t.Fatal("expected at least 1 detected hotspot, got 0")
	}
	h := hotspots[0]
	if h.ID == "" {
		t.Error("expected non-empty hotspot ID")
	}
	if h.MissionID != mission.ID {
		t.Errorf("expected missionId %s, got %s", mission.ID, h.MissionID)
	}
	if h.ConfidenceScore < 0.60 {
		t.Errorf("expected confidence score >= 0.60, got %f", h.ConfidenceScore)
	}

	// Verify hotspot was persisted in store
	savedHotspotBytes, err := memStore.GetState(context.Background(), store.CollectionReconHotspots, h.ID)
	if err != nil {
		t.Fatalf("expected hotspot saved in store: %v", err)
	}
	var savedH domain.ThermalHotspot
	_ = json.Unmarshal(savedHotspotBytes, &savedH)
	if savedH.ID != h.ID {
		t.Errorf("stored hotspot ID mismatch: %s vs %s", savedH.ID, h.ID)
	}

	// Verify deduplication on duplicate thermal frame scan
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/recon/thermal/scan", bytes.NewReader(reqBytes))
	req2.Header.Set("Content-Type", contentType)
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 on second scan, got %d", w2.Code)
	}

	savedMissionBytes, err := memStore.GetState(context.Background(), store.CollectionReconMissions, mission.ID)
	if err != nil {
		t.Fatalf("expected mission in store: %v", err)
	}
	var savedM domain.DroneMission
	_ = json.Unmarshal(savedMissionBytes, &savedM)
	if len(savedM.Hotspots) != len(hotspots) {
		t.Errorf("expected deduplicated hotspots count %d, got %d", len(hotspots), len(savedM.Hotspots))
	}
}

func TestReconHotspotStatusEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Seed lost pet
	lostPet := domain.LostPetRecord{
		PetID:      "test-pet-101",
		PetName:    "Buddy",
		Species:    "Dog",
		Status:     domain.LostPetStatusLost,
		ReportedAt: time.Now().UTC(),
	}
	lpBytes, _ := json.Marshal(lostPet)
	_ = memStore.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lpBytes)

	// Seed search party with sector containing the hotspot coords (37.7750, -122.4190)
	partyID := "party-test-101"
	party := searchparty.SearchParty{
		PartyID:           partyID,
		LostPetID:         lostPet.PetID,
		CenterCoordinates: domain.LocationPoint{Latitude: 37.7750, Longitude: -122.4190},
		RadiusMeters:      1000,
		CreatedAt:         time.Now().UTC(),
		Sectors: []searchparty.SearchSector{
			{
				SectorID: "sector-alpha",
				Name:     "Alpha Sector",
				Status:   searchparty.SectorStatusActiveSearch,
				PolygonPoints: []domain.LocationPoint{
					{Latitude: 37.7740, Longitude: -122.4200},
					{Latitude: 37.7760, Longitude: -122.4200},
					{Latitude: 37.7760, Longitude: -122.4180},
					{Latitude: 37.7740, Longitude: -122.4180},
				},
			},
		},
	}
	pBytes, _ := json.Marshal(party)
	_ = memStore.SaveState(context.Background(), store.SearchPartiesCollection, partyID, pBytes)

	// Subscribe to SignalingHub to verify mesh:thermal-hotspot broadcast
	sigHub := server.SignalingHub()
	msgCh, unsubscribe := sigHub.Subscribe(partyID, "node-ground-searcher")
	defer unsubscribe()

	// Seed hotspot
	hotspot := domain.ThermalHotspot{
		ID:              "hotspot-xyz-1",
		MissionID:       "mission-101",
		PetID:           lostPet.PetID,
		Timestamp:       time.Now().UTC(),
		Latitude:        37.7750,
		Longitude:       -122.4190,
		EstimatedTempC:  37.5,
		ConfidenceScore: 0.92,
		Palette:         domain.PaletteIronbow,
		Status:          domain.HotspotUnverified,
		Classification:  "Thermal Heat Anomaly",
	}
	hBytes, _ := json.Marshal(hotspot)
	_ = memStore.SaveState(context.Background(), store.CollectionReconHotspots, hotspot.ID, hBytes)

	// PUT /api/v1/recon/hotspots/{id}/status (CONFIRMED)
	statusReq := map[string]string{
		"status": "CONFIRMED",
	}
	reqBody, _ := json.Marshal(statusReq)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/recon/hotspots/%s/status", hotspot.ID), bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var updatedHotspot domain.ThermalHotspot
	if err := json.Unmarshal(w.Body.Bytes(), &updatedHotspot); err != nil {
		t.Fatalf("failed to decode updated hotspot: %v", err)
	}

	if updatedHotspot.Status != domain.HotspotConfirmed {
		t.Errorf("expected status CONFIRMED, got %s", updatedHotspot.Status)
	}
	if updatedHotspot.LinkedSightingID == "" {
		t.Error("expected linked sighting ID on confirmed hotspot")
	}

	// 1. Verify official community sighting registered
	sightingBytes, err := memStore.GetState(context.Background(), store.SightingsCollection, updatedHotspot.LinkedSightingID)
	if err != nil {
		t.Fatalf("expected sighting saved in store: %v", err)
	}
	var sighting domain.PetSightingRecord
	_ = json.Unmarshal(sightingBytes, &sighting)
	if sighting.LostPetID != lostPet.PetID {
		t.Errorf("sighting pet ID mismatch: %s vs %s", sighting.LostPetID, lostPet.PetID)
	}
	if sighting.Coordinates == nil || sighting.Coordinates.Latitude != hotspot.Latitude {
		t.Errorf("sighting coordinates mismatch: %+v", sighting.Coordinates)
	}

	// 2. Verify sector updated to sighting_reported
	savedPartyBytes, err := memStore.GetState(context.Background(), store.SearchPartiesCollection, partyID)
	if err != nil {
		t.Fatalf("failed to get updated search party: %v", err)
	}
	var updatedParty searchparty.SearchParty
	_ = json.Unmarshal(savedPartyBytes, &updatedParty)
	if len(updatedParty.Sectors) == 0 || updatedParty.Sectors[0].Status != searchparty.SectorStatusSightingReported {
		t.Errorf("expected sector status sighting_reported, got %s", updatedParty.Sectors[0].Status)
	}

	// 3. Verify mesh broadcast received
	select {
	case env := <-msgCh:
		if env.Type != mesh.SignalThermalHotspot && env.Type != mesh.SignalingType("mesh:thermal-hotspot") {
			t.Errorf("unexpected signaling type: %s", env.Type)
		}
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for mesh:thermal-hotspot broadcast")
	}
}

func TestReconMissionExportEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	mission := domain.DroneMission{
		ID:            "mission-export-1",
		PetID:         "test-pet-101",
		PilotCallsign: "SkyWatcher-1",
		DroneModel:    "DJI Matrice 30T",
		Status:        domain.FlightStatusCompleted,
		StartTime:     time.Now().Add(-1 * time.Hour).UTC(),
		Waypoints: []domain.DroneWaypoint{
			{Latitude: 37.7749, Longitude: -122.4194, AltitudeAGL: 35.0},
			{Latitude: 37.7755, Longitude: -122.4190, AltitudeAGL: 35.0},
		},
		FootprintPoly: [][]float64{
			{-122.4196, 37.7748},
			{-122.4188, 37.7748},
			{-122.4188, 37.7756},
			{-122.4196, 37.7756},
		},
		Hotspots: []domain.ThermalHotspot{
			{
				ID:              "hotspot-1",
				Latitude:        37.7752,
				Longitude:       -122.4192,
				EstimatedTempC:  36.8,
				ConfidenceScore: 0.88,
				Status:          domain.HotspotConfirmed,
			},
		},
	}
	mBytes, _ := json.Marshal(mission)
	_ = memStore.SaveState(context.Background(), store.CollectionReconMissions, mission.ID, mBytes)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/recon/missions/%s/export", mission.ID), nil)
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var geoJSON struct {
		Type     string                   `json:"type"`
		Features []map[string]interface{} `json:"features"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &geoJSON); err != nil {
		t.Fatalf("failed to decode GeoJSON: %v", err)
	}

	if geoJSON.Type != "FeatureCollection" {
		t.Errorf("expected FeatureCollection, got %s", geoJSON.Type)
	}
	if len(geoJSON.Features) < 3 {
		t.Errorf("expected at least 3 features (path, footprint, hotspot), got %d", len(geoJSON.Features))
	}

	// Verify linear ring closure for Polygon feature
	var footprintFeature map[string]interface{}
	for _, f := range geoJSON.Features {
		geom, ok := f["geometry"].(map[string]interface{})
		if ok && geom["type"] == "Polygon" {
			footprintFeature = f
			break
		}
	}
	if footprintFeature == nil {
		t.Fatal("expected polygon footprint feature in export")
	}
	geom := footprintFeature["geometry"].(map[string]interface{})
	coordsRaw := geom["coordinates"].([]interface{})
	ring := coordsRaw[0].([]interface{})
	if len(ring) != 5 {
		t.Fatalf("expected 5 coordinates in closed linear ring, got %d", len(ring))
	}
	firstCoord := ring[0].([]interface{})
	lastCoord := ring[4].([]interface{})
	if firstCoord[0] != lastCoord[0] || firstCoord[1] != lastCoord[1] {
		t.Errorf("linear ring not closed: first %v != last %v", firstCoord, lastCoord)
	}

	// Test 404 for unknown mission
	req404 := httptest.NewRequest(http.MethodGet, "/api/v1/recon/missions/unknown-id/export", nil)
	w404 := httptest.NewRecorder()
	server.ServeHTTP(w404, req404)
	if w404.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w404.Code)
	}
}
