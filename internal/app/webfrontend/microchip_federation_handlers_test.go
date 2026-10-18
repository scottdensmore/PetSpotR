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

func TestMicrochipFederationHandlers_SeedingCreateTraceLotResolveAndSightingPromotion(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "microchip_federation" topic and Mesh signaling hub
	chipSSECh, unsubSSE := server.ReunionHub().Subscribe("microchip_federation")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("microchip_federation", "microchip-federation-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /microchip-federation should render HTML and seed default microchip scan cases
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/microchip-federation", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /microchip-federation, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Universal Microchip Registry Federation") {
		t.Errorf("expected /microchip-federation HTML to contain Universal Microchip Registry Federation heading")
	}

	// 2. GET /api/v1/microchip-federation/cases should return the 3 seeded cases
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/microchip-federation/cases", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/microchip-federation/cases, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Cases []domain.MicrochipScanCase `json:"cases"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal microchip cases response: %v", err)
	}
	if len(listResp.Cases) != 3 {
		t.Fatalf("expected 3 seeded microchip cases, got %d", len(listResp.Cases))
	}

	byID := make(map[string]domain.MicrochipScanCase)
	for _, c := range listResp.Cases {
		byID[c.CaseID] = c
	}

	c101, ok := byID["chip-case-101"]
	if !ok {
		t.Fatalf("expected seeded chip-case-101")
	}
	if c101.PetName != "Luna" || c101.Status != domain.ChipStatusUnregisteredBatchTraced {
		t.Errorf("expected chip-case-101 Luna to be UNREGISTERED_BATCH_TRACED, got %s / %s", c101.PetName, c101.Status)
	}

	c102, ok := byID["chip-case-102"]
	if !ok {
		t.Fatalf("expected seeded chip-case-102")
	}
	if c102.PetName != "Buster" || c102.Status != domain.ChipStatusSecondaryContactEscalated {
		t.Errorf("expected chip-case-102 Buster to be SECONDARY_CONTACT_ESCALATED, got %s / %s", c102.PetName, c102.Status)
	}

	c103, ok := byID["chip-case-103"]
	if !ok {
		t.Fatalf("expected seeded chip-case-103")
	}
	if c103.PetName != "Cooper" || c103.Status != domain.ChipStatusReunionDispatchedVerified || c103.PromotedSightingID != "sighting-chip-seed-103" {
		t.Errorf("expected chip-case-103 Cooper to be REUNION_DISPATCHED_VERIFIED with sighting-chip-seed-103, got %s / %s / %s", c103.PetName, c103.Status, c103.PromotedSightingID)
	}

	// 2b. Filter by petId and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/microchip-federation/cases?petId=pet-2026-002&status=UNREGISTERED_BATCH_TRACED", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/microchip-federation/cases, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Cases []domain.MicrochipScanCase `json:"cases"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered microchip cases: %v", err)
	}
	if len(filteredResp.Cases) != 1 || filteredResp.Cases[0].CaseID != "chip-case-101" {
		t.Errorf("expected 1 filtered case (chip-case-101), got %+v", filteredResp.Cases)
	}

	// 3. POST /api/v1/microchip-federation/cases with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":        "",
		"rawChipInput": "invalid-chip",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/microchip-federation/cases", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid microchip scan case, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/microchip-federation/cases creates a new federated scan case (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":           "pet-2026-002",
		"petName":         "Luna",
		"species":         "CAT",
		"rawChipInput":    "985-141000-774123",
		"latitude":        47.6640,
		"longitude":       -122.3980,
		"scannedByAgency": "Ballard Emergency Vet RFID Wand",
		"simulateMode":    "REGISTERED_PRIMARY",
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/microchip-federation/cases", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/microchip-federation/cases, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createdCase domain.MicrochipScanCase
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createdCase); err != nil {
		t.Fatalf("failed to unmarshal created microchip case: %v", err)
	}
	if !strings.HasPrefix(createdCase.CaseID, "chip-case-") {
		t.Errorf("expected CaseID prefix 'chip-case-', got %q", createdCase.CaseID)
	}
	if createdCase.Status != domain.ChipStatusRegisteredOwnerFound || createdCase.WinningRegistry != "HomeAgain" {
		t.Errorf("expected new case REGISTERED_OWNER_FOUND in HomeAgain, got %s / %s", createdCase.Status, createdCase.WinningRegistry)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-chipSSECh:
		if evt.Type != domain.ReunionEventType("microchip_case_updated") {
			t.Errorf("expected microchip_case_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for microchip_case_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "microchip_case_updated" {
			t.Errorf("expected microchip_case_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for microchip_case_updated mesh event")
	}

	// 5. POST /api/v1/microchip-federation/cases/chip-case-101/trace-lot traces manufacturer batch lot
	tracePayload, _ := json.Marshal(map[string]any{
		"implantingClinicName":    "Emerald City Spay/Neuter & Wellness Clinic",
		"implantingClinicPhone":   "(206) 555-0148",
		"implantPatientChartId":   "PIMS-CHART-9102",
		"recoveredOwnerName":      "Devon & Priya Patel",
		"recoveredOwnerPhoneMask": "+1 (206) •••-7724",
	})
	recTrace := httptest.NewRecorder()
	reqTrace := httptest.NewRequest(http.MethodPost, "/api/v1/microchip-federation/cases/chip-case-101/trace-lot", bytes.NewReader(tracePayload))
	reqTrace.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTrace, reqTrace)
	if recTrace.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST trace-lot, got %d: %s", recTrace.Code, recTrace.Body.String())
	}

	var tracedCase domain.MicrochipScanCase
	if err := json.Unmarshal(recTrace.Body.Bytes(), &tracedCase); err != nil {
		t.Fatalf("failed to unmarshal traced case: %v", err)
	}
	if tracedCase.Status != domain.ChipStatusUnregisteredBatchTraced {
		t.Errorf("expected status UNREGISTERED_BATCH_TRACED, got %s", tracedCase.Status)
	}
	if tracedCase.ImplantPatientChartID != "PIMS-CHART-9102" || tracedCase.ResolvedOwnerName != "Devon & Priya Patel" {
		t.Errorf("expected chart PIMS-CHART-9102 and owner Devon & Priya Patel, got %s / %s", tracedCase.ImplantPatientChartID, tracedCase.ResolvedOwnerName)
	}

	// 6. POST /api/v1/microchip-federation/cases/chip-case-101/resolve resolves ownership and promotes sighting
	resolvePayload, _ := json.Marshal(map[string]any{
		"verifiedOwnerName":      "Devon & Priya Patel",
		"verifiedOwnerPhoneMask": "+1 (206) •••-7724",
		"verificationMethod":     "CLINIC_PIMS_LOT_TRACE",
		"notes":                  "Confirmed ownership via clinic rabies/microchip certificate and dispatched reunion runner.",
	})
	recResolve := httptest.NewRecorder()
	reqResolve := httptest.NewRequest(http.MethodPost, "/api/v1/microchip-federation/cases/chip-case-101/resolve", bytes.NewReader(resolvePayload))
	reqResolve.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recResolve, reqResolve)
	if recResolve.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST resolve, got %d: %s", recResolve.Code, recResolve.Body.String())
	}

	var resolvedCase domain.MicrochipScanCase
	if err := json.Unmarshal(recResolve.Body.Bytes(), &resolvedCase); err != nil {
		t.Fatalf("failed to unmarshal resolved case: %v", err)
	}
	if resolvedCase.Status != domain.ChipStatusReunionDispatchedVerified {
		t.Errorf("expected status REUNION_DISPATCHED_VERIFIED, got %s", resolvedCase.Status)
	}
	if !strings.HasPrefix(resolvedCase.PromotedSightingID, "sighting-chip-") {
		t.Fatalf("expected PromotedSightingID starting with 'sighting-chip-', got %q", resolvedCase.PromotedSightingID)
	}

	// Verify promoted sighting exists in store.SightingsCollection
	sightingBytes, err := memStore.GetState(context.Background(), store.SightingsCollection, resolvedCase.PromotedSightingID)
	if err != nil {
		t.Fatalf("expected promoted sighting %q in SightingsCollection: %v", resolvedCase.PromotedSightingID, err)
	}
	var promotedSighting domain.Sighting
	if err := json.Unmarshal(sightingBytes, &promotedSighting); err != nil {
		t.Fatalf("failed to unmarshal promoted sighting: %v", err)
	}
	if promotedSighting.PetID != "pet-2026-002" {
		t.Errorf("expected promoted sighting PetID 'pet-2026-002', got %q", promotedSighting.PetID)
	}

	// 7. Unknown case ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/microchip-federation/cases/unknown-case-id/resolve", bytes.NewReader(resolvePayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown case ID, got %d", rec404.Code)
	}
}

func TestMicrochipFederationHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/microchip-federation/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/microchip-federation/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/microchip-federation/stream, got %d", resp.StatusCode)
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
