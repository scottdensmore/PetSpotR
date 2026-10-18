package webfrontend_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestK9Handlers_SeedingCreateWaypointsSightingAndGPX(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "k9" topic and Mesh signaling hub
	k9SSECh, unsubSSE := server.ReunionHub().Subscribe("k9")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("k9", "k9-command-post")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /k9-ops should render HTML and seed default K9 missions
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/k9-ops", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /k9-ops, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "K9 Atlas") {
		t.Errorf("expected /k9-ops HTML to contain seeded K9 Atlas mission")
	}

	// 2. GET /api/v1/k9/missions should return the 2 seeded K9 missions
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/k9/missions", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/k9/missions, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Missions []domain.K9ScentMission `json:"missions"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal K9 missions response: %v", err)
	}
	if len(listResp.Missions) != 2 {
		t.Fatalf("expected 2 seeded K9 missions, got %d", len(listResp.Missions))
	}

	missionsByID := make(map[string]domain.K9ScentMission)
	for _, m := range listResp.Missions {
		missionsByID[m.MissionID] = m
	}

	m101, ok := missionsByID["mission-k9-101"]
	if !ok {
		t.Fatalf("expected seeded mission-k9-101")
	}
	if m101.CanineName != "K9 Atlas" || m101.ViabilityRating != domain.ScentViabilityHigh {
		t.Errorf("expected mission-k9-101 K9 Atlas with HIGH_VIABILITY, got %s / %s", m101.CanineName, m101.ViabilityRating)
	}
	if len(m101.Waypoints) != 2 {
		t.Errorf("expected mission-k9-101 to have 2 seeded waypoints, got %d", len(m101.Waypoints))
	}
	if m101.PlumeCone.DownwindAzimuthDeg != 90.0 {
		t.Errorf("expected mission-k9-101 downwind azimuth 90.0, got %.1f", m101.PlumeCone.DownwindAzimuthDeg)
	}

	m102, ok := missionsByID["mission-k9-102"]
	if !ok {
		t.Fatalf("expected seeded mission-k9-102")
	}
	if !m102.PawThermalHazard || m102.ViabilityRating != domain.ScentViabilityDegraded {
		t.Errorf("expected mission-k9-102 to flag PawThermalHazard and DEGRADED_THERMAL_HAZARD, got hazard=%v rating=%s", m102.PawThermalHazard, m102.ViabilityRating)
	}

	// 3. POST /api/v1/k9/missions with invalid payload returns 400 Bad Request
	invalidBody, _ := json.Marshal(map[string]any{
		"petId":       "",
		"handlerName": "",
		"canineName":  "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/k9/missions", bytes.NewReader(invalidBody))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid K9 mission, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/k9/missions creates a new K9 scent mission
	createPayload, _ := json.Marshal(map[string]any{
		"incidentId":         "INC-2026-WILDFIRE-01",
		"petId":              "pet-k9-target-01",
		"handlerVolunteerId": "vol-sar-101",
		"handlerName":        "Maya Lin",
		"canineName":         "K9 Rex",
		"canineCallsign":     "K9-REX-9",
		"discipline":         "MANTRAILING_SCENT_SPECIFIC",
		"scentArticleDesc":   "Unwashed wool toy",
		"lkpOriginLat":       47.6062,
		"lkpOriginLng":       -122.3321,
		"environment": map[string]any{
			"windSpeedKmh":        14.0,
			"windFromDeg":         270.0,
			"temperatureC":        15.0,
			"relativeHumidityPct": 75.0,
			"groundSurface":       "LUSH_VEGETATION",
			"trailAgeHours":       2.0,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/k9/missions", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/k9/missions, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createdMission domain.K9ScentMission
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createdMission); err != nil {
		t.Fatalf("failed to unmarshal created mission: %v", err)
	}
	if !strings.HasPrefix(createdMission.MissionID, "mission-k9-") {
		t.Errorf("expected generated missionId starting with 'mission-k9-', got %q", createdMission.MissionID)
	}
	if createdMission.ViabilityRating != domain.ScentViabilityHigh {
		t.Errorf("expected HIGH_VIABILITY for created mission, got %q", createdMission.ViabilityRating)
	}
	if createdMission.PlumeCone.DownwindAzimuthDeg != 90.0 || len(createdMission.PlumeCone.Vertices) < 8 {
		t.Errorf("unexpected plume cone on created mission: %+v", createdMission.PlumeCone)
	}
	initialPOD := createdMission.EstimatedPOD

	// Verify SSE event "k9_mission_updated"
	select {
	case evt := <-k9SSECh:
		if evt.Type != domain.ReunionEventType("k9_mission_updated") {
			t.Errorf("expected k9_mission_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for k9_mission_updated SSE event")
	}

	// 5. POST /api/v1/k9/missions/{id}/waypoints with promoteToSighting: true
	wpPayload, _ := json.Marshal(map[string]any{
		"lat":               47.6062,
		"lng":               -122.3290,
		"behavior":          "STRONG_SCENT_PULL",
		"notes":             "Strong scent pull along creek bed",
		"promoteToSighting": true,
	})
	recWP := httptest.NewRecorder()
	reqWP := httptest.NewRequest(http.MethodPost, "/api/v1/k9/missions/"+createdMission.MissionID+"/waypoints", bytes.NewReader(wpPayload))
	reqWP.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recWP, reqWP)
	if recWP.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST waypoint, got %d: %s", recWP.Code, recWP.Body.String())
	}

	var updatedMission domain.K9ScentMission
	if err := json.Unmarshal(recWP.Body.Bytes(), &updatedMission); err != nil {
		t.Fatalf("failed to unmarshal updated mission after waypoint: %v", err)
	}
	if len(updatedMission.Waypoints) != 1 {
		t.Fatalf("expected 1 waypoint on mission, got %d", len(updatedMission.Waypoints))
	}
	if !updatedMission.Waypoints[0].InScentPlume {
		t.Errorf("expected downwind East waypoint to be marked InScentPlume = true")
	}
	if updatedMission.PlumeIntersectionPct != 100.0 {
		t.Errorf("expected PlumeIntersectionPct = 100.0, got %.1f", updatedMission.PlumeIntersectionPct)
	}
	if updatedMission.EstimatedPOD <= initialPOD {
		t.Errorf("expected EstimatedPOD to increase from %.1f, got %.1f", initialPOD, updatedMission.EstimatedPOD)
	}
	if !strings.HasPrefix(updatedMission.PromotedSightingID, "sighting-k9-") {
		t.Fatalf("expected PromotedSightingID starting with 'sighting-k9-', got %q", updatedMission.PromotedSightingID)
	}

	// Verify Sighting was saved in store.SightingsCollection
	sightingBytes, err := memStore.GetState(context.Background(), store.SightingsCollection, updatedMission.PromotedSightingID)
	if err != nil {
		t.Fatalf("expected promoted sighting %q in store.SightingsCollection: %v", updatedMission.PromotedSightingID, err)
	}
	var promotedSighting domain.Sighting
	if err := json.Unmarshal(sightingBytes, &promotedSighting); err != nil {
		t.Fatalf("failed to unmarshal promoted sighting: %v", err)
	}
	if promotedSighting.PetID != "pet-k9-target-01" {
		t.Errorf("expected promoted sighting PetID 'pet-k9-target-01', got %q", promotedSighting.PetID)
	}
	if !strings.Contains(promotedSighting.Notes, "[K9 K9 Rex STRONG_SCENT_PULL] Strong scent pull along creek bed") {
		t.Errorf("unexpected promoted sighting notes: %q", promotedSighting.Notes)
	}

	// 6. GET /api/v1/k9/missions/{id}/gpx exports GPX 1.1 XML
	recGPX := httptest.NewRecorder()
	reqGPX := httptest.NewRequest(http.MethodGet, "/api/v1/k9/missions/"+createdMission.MissionID+"/gpx", nil)
	server.ServeHTTP(recGPX, reqGPX)
	if recGPX.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GPX export, got %d: %s", recGPX.Code, recGPX.Body.String())
	}
	if !strings.Contains(recGPX.Header().Get("Content-Type"), "application/gpx+xml") {
		t.Errorf("expected application/gpx+xml Content-Type, got %q", recGPX.Header().Get("Content-Type"))
	}
	expectedDisposition := `attachment; filename="k9-mission-` + createdMission.MissionID + `.gpx"`
	if recGPX.Header().Get("Content-Disposition") != expectedDisposition {
		t.Errorf("expected Content-Disposition %q, got %q", expectedDisposition, recGPX.Header().Get("Content-Disposition"))
	}
	gpxBody := recGPX.Body.String()
	for _, token := range []string{
		`<gpx version="1.1"`,
		`<wpt lat="47.606200" lon="-122.332100">`,
		`<trkpt lat="47.606200" lon="-122.329000">`,
		`STRONG_SCENT_PULL`,
	} {
		if !strings.Contains(gpxBody, token) {
			t.Errorf("expected GPX XML to contain %q, got:\n%s", token, gpxBody)
		}
	}

	// 7. GET /api/v1/k9/missions/unknown-id/gpx returns 404 Not Found
	recGPX404 := httptest.NewRecorder()
	reqGPX404 := httptest.NewRequest(http.MethodGet, "/api/v1/k9/missions/unknown-mission-id/gpx", nil)
	server.ServeHTTP(recGPX404, reqGPX404)
	if recGPX404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown mission GPX export, got %d", recGPX404.Code)
	}
}

func TestK9Handlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/k9/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/k9/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/k9/stream, got %d", resp.StatusCode)
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Errorf("expected text/event-stream Content-Type, got %q", resp.Header.Get("Content-Type"))
	}

	reader := bufio.NewReader(resp.Body)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed reading initial SSE comment: %v", err)
	}
	if !strings.Contains(line, ": connected") {
		t.Errorf("expected ': connected' prologue, got %q", line)
	}
}
