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

func TestMunicipalCADHandlers_SeedingCreateDispatchAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "municipal_cad" topic and Mesh signaling hub
	cadSSECh, unsubSSE := server.ReunionHub().Subscribe("municipal_cad")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("municipal_cad", "municipal-cad-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /municipal-cad should render HTML and seed default incidents
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/municipal-cad", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /municipal-cad, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Municipal 311") {
		t.Errorf("expected /municipal-cad HTML to contain Municipal 311 heading")
	}

	// 2. GET /api/v1/municipal-cad/incidents should return the 3 seeded incidents
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/municipal-cad/incidents", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/municipal-cad/incidents, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Incidents []domain.MunicipalCADIncident `json:"incidents"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal municipal CAD incidents response: %v", err)
	}
	if len(listResp.Incidents) != 3 {
		t.Fatalf("expected 3 seeded municipal CAD incidents, got %d", len(listResp.Incidents))
	}

	byID := make(map[string]domain.MunicipalCADIncident)
	for _, inc := range listResp.Incidents {
		byID[inc.IncidentID] = inc
	}

	inc101, ok := byID["cad-inc-101"]
	if !ok {
		t.Fatalf("expected seeded cad-inc-101")
	}
	if inc101.PetName != "Cooper" || inc101.DispatchStatus != domain.CADStatusInbound311Triage || inc101.HazardSeverity != domain.RoadwayHazardFreewayCritical || !inc101.DoNotChaseAlert {
		t.Errorf("expected cad-inc-101 Cooper in INBOUND_311_TRIAGE / FREEWAY_MEDIAN_CRITICAL with DoNotChaseAlert=true, got %+v", inc101)
	}

	inc102, ok := byID["cad-inc-102"]
	if !ok {
		t.Fatalf("expected seeded cad-inc-102")
	}
	if inc102.PetName != "Hazel" || inc102.DispatchStatus != domain.CADStatusHighwayContainment || inc102.HazardSeverity != domain.RoadwayHazardArterialHigh {
		t.Errorf("expected cad-inc-102 Hazel in HIGHWAY_CONTAINMENT_DISPATCHED / ARTERIAL_TRAFFIC_HIGH, got %+v", inc102)
	}

	inc103, ok := byID["cad-inc-103"]
	if !ok {
		t.Fatalf("expected seeded cad-inc-103")
	}
	if inc103.PetName != "Biscuit" || inc103.DispatchStatus != domain.CADStatusResolvedSafeCustody || inc103.HazardSeverity != domain.RoadwayHazardContainedSafe || inc103.CustodyResolutionCode != "CAD-SAFE-CAD-INC-103" {
		t.Errorf("expected cad-inc-103 Biscuit in RESOLVED_SAFE_CUSTODY / CONTAINED_CORRIDOR_SAFE with CustodyResolutionCode CAD-SAFE-CAD-INC-103, got %+v", inc103)
	}

	// 2b. Filter by petId and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/municipal-cad/incidents?petId=pet-2026-001&status=INBOUND_311_TRIAGE", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/municipal-cad/incidents, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Incidents []domain.MunicipalCADIncident `json:"incidents"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered incidents: %v", err)
	}
	if len(filteredResp.Incidents) != 1 || filteredResp.Incidents[0].IncidentID != "cad-inc-101" {
		t.Errorf("expected 1 filtered incident (cad-inc-101), got %+v", filteredResp.Incidents)
	}

	// 3. POST /api/v1/municipal-cad/incidents with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":         "",
		"speedLimitMph": 5,
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/municipal-cad/incidents", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid municipal CAD incident, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/municipal-cad/incidents creates a new municipal CAD incident (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"cadCallNumber":         "311-SEA-9100",
		"petId":                 "pet-2026-009",
		"petName":               "Rex",
		"species":               "DOG",
		"roadwayName":           "I-90 Floating Bridge Eastbound Span",
		"speedLimitMph":         60,
		"laneCount":             4,
		"petBehaviorState":      "BOLTING_INTO_TRAFFIC",
		"acoUnitAssigned":       "",
		"dotSignAdvisoryActive": false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/municipal-cad/incidents", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/municipal-cad/incidents, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createdIncident domain.MunicipalCADIncident
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createdIncident); err != nil {
		t.Fatalf("failed to unmarshal created incident: %v", err)
	}
	if !strings.HasPrefix(createdIncident.IncidentID, "cad-inc-") {
		t.Errorf("expected IncidentID prefix 'cad-inc-', got %q", createdIncident.IncidentID)
	}
	if !createdIncident.DoNotChaseAlert || createdIncident.DispatchStatus != domain.CADStatusInbound311Triage {
		t.Errorf("expected new incident DoNotChaseAlert=true and INBOUND_311_TRIAGE, got %+v", createdIncident)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-cadSSECh:
		if evt.Type != domain.ReunionEventType("municipal_cad_updated") {
			t.Errorf("expected municipal_cad_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for municipal_cad_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "municipal_cad_updated" {
			t.Errorf("expected municipal_cad_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for municipal_cad_updated mesh event")
	}

	// 5. POST /api/v1/municipal-cad/incidents/cad-inc-101/dispatch resolves safe custody
	dispatchPayload, _ := json.Marshal(map[string]any{
		"resolveSafeCustody": true,
		"acoUnitAssigned":    "ACO-UNIT-19",
	})
	recDisp := httptest.NewRecorder()
	reqDisp := httptest.NewRequest(http.MethodPost, "/api/v1/municipal-cad/incidents/cad-inc-101/dispatch", bytes.NewReader(dispatchPayload))
	reqDisp.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recDisp, reqDisp)
	if recDisp.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST dispatch, got %d: %s", recDisp.Code, recDisp.Body.String())
	}

	var updatedIncident domain.MunicipalCADIncident
	if err := json.Unmarshal(recDisp.Body.Bytes(), &updatedIncident); err != nil {
		t.Fatalf("failed to unmarshal updated incident: %v", err)
	}
	if updatedIncident.DispatchStatus != domain.CADStatusResolvedSafeCustody {
		t.Errorf("expected status RESOLVED_SAFE_CUSTODY, got %s", updatedIncident.DispatchStatus)
	}
	if updatedIncident.HazardSeverity != domain.RoadwayHazardContainedSafe {
		t.Errorf("expected severity CONTAINED_CORRIDOR_SAFE, got %s", updatedIncident.HazardSeverity)
	}
	if updatedIncident.CustodyResolutionCode != "CAD-SAFE-CAD-INC-101" {
		t.Errorf("expected CustodyResolutionCode CAD-SAFE-CAD-INC-101, got %q", updatedIncident.CustodyResolutionCode)
	}

	// 6. Unknown incident ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/municipal-cad/incidents/unknown-cad-id/dispatch", bytes.NewReader(dispatchPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown incident ID, got %d", rec404.Code)
	}
}

func TestMunicipalCADHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/municipal-cad/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/municipal-cad/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/municipal-cad/stream, got %d", resp.StatusCode)
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
