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

func TestWildlifeCorridorHandlers_SeedingCreateMitigateAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "wildlife_corridors" topic and Mesh signaling hub
	corridorSSECh, unsubSSE := server.ReunionHub().Subscribe("wildlife_corridors")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("wildlife_corridors", "wildlife-corridors-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /wildlife-corridors should render HTML and seed default reports
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/wildlife-corridors", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /wildlife-corridors, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Predator & Wildlife Conflict Corridor") && !strings.Contains(recPage.Body.String(), "Predator &amp; Wildlife Conflict Corridor") {
		t.Errorf("expected /wildlife-corridors HTML to contain Predator & Wildlife Conflict Corridor heading")
	}

	// 2. GET /api/v1/wildlife-corridors/reports should return the 3 seeded reports
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/wildlife-corridors/reports", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/wildlife-corridors/reports, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Reports []domain.WildlifeCorridorReport `json:"reports"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal wildlife corridor reports response: %v", err)
	}
	if len(listResp.Reports) != 3 {
		t.Fatalf("expected 3 seeded wildlife corridor reports, got %d", len(listResp.Reports))
	}

	byID := make(map[string]domain.WildlifeCorridorReport)
	for _, r := range listResp.Reports {
		byID[r.ReportID] = r
	}

	r101, ok := byID["corridor-rep-101"]
	if !ok {
		t.Fatalf("expected seeded corridor-rep-101")
	}
	if r101.PetName != "Mochi" || r101.Phase != domain.CorridorPhaseActivePredatorWatch || r101.ThreatLevel != domain.PredatorThreatExtremeNocturnal || !r101.SmallPreyAlert {
		t.Errorf("expected corridor-rep-101 Mochi in ACTIVE_PREDATOR_WATCH / EXTREME_NOCTURNAL_PREDATION_RISK with SmallPreyAlert=true, got %+v", r101)
	}

	r102, ok := byID["corridor-rep-102"]
	if !ok {
		t.Fatalf("expected seeded corridor-rep-102")
	}
	if r102.PetName != "Pippin" || r102.Phase != domain.CorridorPhaseHazingDeterrentDeployed || r102.ThreatLevel != domain.PredatorThreatElevatedCrepuscular {
		t.Errorf("expected corridor-rep-102 Pippin in HAZING_DETERRENT_DEPLOYED / ELEVATED_CREPUSCULAR_CORRIDOR, got %+v", r102)
	}

	r103, ok := byID["corridor-rep-103"]
	if !ok {
		t.Fatalf("expected seeded corridor-rep-103")
	}
	if r103.PetName != "Shadow" || r103.Phase != domain.CorridorPhaseCorridorCleared || r103.ThreatLevel != domain.PredatorThreatMitigatedSafeZone || r103.ClearanceCertificate != "CERT-CORRIDOR-CORRIDOR-REP-103" {
		t.Errorf("expected corridor-rep-103 Shadow in CORRIDOR_CLEARED / MITIGATED_SAFE_ZONE with CERT-CORRIDOR-CORRIDOR-REP-103, got %+v", r103)
	}

	// 2b. Filter by petId and phase
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/wildlife-corridors/reports?petId=pet-2026-002&phase=ACTIVE_PREDATOR_WATCH", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/wildlife-corridors/reports, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Reports []domain.WildlifeCorridorReport `json:"reports"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered reports: %v", err)
	}
	if len(filteredResp.Reports) != 1 || filteredResp.Reports[0].ReportID != "corridor-rep-101" {
		t.Errorf("expected 1 filtered report (corridor-rep-101), got %+v", filteredResp.Reports)
	}

	// 3. POST /api/v1/wildlife-corridors/reports with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":       "",
		"petWeightKg": 0,
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/wildlife-corridors/reports", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid wildlife corridor report, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/wildlife-corridors/reports creates a new report (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":                 "pet-2026-009",
		"petName":               "Peanut",
		"petSpecies":            "DOG",
		"petWeightKg":           3.2,
		"predatorSpecies":       "GREAT_HORNED_OWL",
		"packCount":             1,
		"solarPhase":            "DAWN_DUSK_CREPUSCULAR",
		"corridorTerrain":       "BRUSH_CANYON_INTERFACE",
		"distanceMeters":        95,
		"strobeDeterrentActive": false,
		"attractantsCleared":    false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/wildlife-corridors/reports", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/wildlife-corridors/reports, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createdReport domain.WildlifeCorridorReport
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createdReport); err != nil {
		t.Fatalf("failed to unmarshal created report: %v", err)
	}
	if !strings.HasPrefix(createdReport.ReportID, "corridor-rep-") {
		t.Errorf("expected ReportID prefix 'corridor-rep-', got %q", createdReport.ReportID)
	}
	if !createdReport.SmallPreyAlert || createdReport.Phase != domain.CorridorPhaseActivePredatorWatch || createdReport.ThreatLevel != domain.PredatorThreatExtremeNocturnal {
		t.Errorf("expected new report SmallPreyAlert=true, ACTIVE_PREDATOR_WATCH, EXTREME_NOCTURNAL_PREDATION_RISK, got %+v", createdReport)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-corridorSSECh:
		if evt.Type != domain.ReunionEventType("wildlife_corridor_updated") {
			t.Errorf("expected wildlife_corridor_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for wildlife_corridor_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "wildlife_corridor_updated" {
			t.Errorf("expected wildlife_corridor_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for wildlife_corridor_updated mesh event")
	}

	// 5. POST /api/v1/wildlife-corridors/reports/corridor-rep-101/mitigate updates mitigation steps and clears corridor
	mitigatePayload, _ := json.Marshal(map[string]any{
		"completeAllMitigations": true,
	})
	recMit := httptest.NewRecorder()
	reqMit := httptest.NewRequest(http.MethodPost, "/api/v1/wildlife-corridors/reports/corridor-rep-101/mitigate", bytes.NewReader(mitigatePayload))
	reqMit.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recMit, reqMit)
	if recMit.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST mitigate, got %d: %s", recMit.Code, recMit.Body.String())
	}

	var updatedReport domain.WildlifeCorridorReport
	if err := json.Unmarshal(recMit.Body.Bytes(), &updatedReport); err != nil {
		t.Fatalf("failed to unmarshal updated report: %v", err)
	}
	if updatedReport.Phase != domain.CorridorPhaseCorridorCleared {
		t.Errorf("expected phase CORRIDOR_CLEARED, got %s", updatedReport.Phase)
	}
	if updatedReport.ThreatLevel != domain.PredatorThreatMitigatedSafeZone {
		t.Errorf("expected threatLevel MITIGATED_SAFE_ZONE, got %s", updatedReport.ThreatLevel)
	}
	if updatedReport.ClearanceCertificate != "CERT-CORRIDOR-CORRIDOR-REP-101" {
		t.Errorf("expected ClearanceCertificate CERT-CORRIDOR-CORRIDOR-REP-101, got %q", updatedReport.ClearanceCertificate)
	}

	// 6. Unknown report ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/wildlife-corridors/reports/unknown-rep-id/mitigate", bytes.NewReader(mitigatePayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown report ID, got %d", rec404.Code)
	}
}

func TestWildlifeCorridorHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/wildlife-corridors/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/wildlife-corridors/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/wildlife-corridors/stream, got %d", resp.StatusCode)
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
