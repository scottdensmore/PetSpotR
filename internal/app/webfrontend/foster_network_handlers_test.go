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

func TestFosterNetworkHandlers_SeedingCreateTransitionAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	fosterSSECh, unsubSSE := server.ReunionHub().Subscribe("foster_network")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("foster_network", "foster-network-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /foster-network should render HTML and seed default placements
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/foster-network", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /foster-network, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Emergency Foster Network Capacity") {
		t.Errorf("expected /foster-network HTML to contain Emergency Foster Network Capacity heading")
	}

	// 2. GET /api/v1/foster-network/placements should return the 3 seeded placements
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/foster-network/placements", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/foster-network/placements, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Placements []domain.FosterPlacementMatch `json:"placements"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal foster placements response: %v", err)
	}
	if len(listResp.Placements) != 3 {
		t.Fatalf("expected 3 seeded foster placements, got %d", len(listResp.Placements))
	}

	byID := make(map[string]domain.FosterPlacementMatch)
	for _, p := range listResp.Placements {
		byID[p.PlacementID] = p
	}

	p101, ok := byID["foster-match-101"]
	if !ok {
		t.Fatalf("expected seeded foster-match-101")
	}
	if p101.PetName != "Barnaby" || p101.PlacementStatus != domain.FosterStatusPendingBiosecurity || p101.BiosecurityTier != domain.BiosecurityTierIsoQuarantine || !p101.IsoQuarantineAlert {
		t.Errorf("expected foster-match-101 Barnaby in PENDING_BIOSECURITY_CLEARANCE / ISO_QUARANTINE_CONTAGION with IsoQuarantineAlert=true, got %+v", p101)
	}

	p102, ok := byID["foster-match-102"]
	if !ok {
		t.Fatalf("expected seeded foster-match-102")
	}
	if p102.PetName != "Willow" || p102.PlacementStatus != domain.FosterStatusQuarantineVerified || p102.BiosecurityTier != domain.BiosecurityTierMedicalSinglePet {
		t.Errorf("expected foster-match-102 Willow in QUARANTINE_VERIFIED_STAGED / MEDICAL_RECOVERY_SINGLE_PET, got %+v", p102)
	}

	p103, ok := byID["foster-match-103"]
	if !ok {
		t.Fatalf("expected seeded foster-match-103")
	}
	if p103.PetName != "Otis" || p103.PlacementStatus != domain.FosterStatusPlacedInFosterHome || p103.BiosecurityTier != domain.BiosecurityTierClearedStandard || p103.PlacementAuthorizationCode != "FOSTER-AUTH-FOSTER-MATCH-103" {
		t.Errorf("expected foster-match-103 Otis in ACTIVE_IN_FOSTER_HOME / BIOSECURITY_CLEARED_STANDARD with PlacementAuthorizationCode, got %+v", p103)
	}

	// 2b. Filter by petId and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/foster-network/placements?petId=pet-2026-002&status=PENDING_BIOSECURITY_CLEARANCE", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/foster-network/placements, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Placements []domain.FosterPlacementMatch `json:"placements"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered placements: %v", err)
	}
	if len(filteredResp.Placements) != 1 || filteredResp.Placements[0].PlacementID != "foster-match-101" {
		t.Errorf("expected 1 filtered placement (foster-match-101), got %+v", filteredResp.Placements)
	}

	// 3. POST /api/v1/foster-network/placements with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":   "",
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/foster-network/placements", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid foster placement, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/foster-network/placements creates a new foster placement (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":              "pet-2026-002",
		"petName":            "Barnaby",
		"species":            "DOG",
		"fosterHostName":     "Elena Vance (RN / Vet Tech)",
		"contagionFlag":      "PARVO_SUSPECTED",
		"medicalNeedLevel":   "INJECTABLE_SUBQ_FLUIDS",
		"residentPetsCount":  1,
		"hepaIsolationReady": false,
		"medCertVerified":    false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/foster-network/placements", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/foster-network/placements, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.FosterPlacementMatch
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created placement: %v", err)
	}
	if !strings.HasPrefix(created.PlacementID, "foster-match-") {
		t.Errorf("expected PlacementID prefix 'foster-match-', got %q", created.PlacementID)
	}
	if !created.IsoQuarantineAlert || created.PlacementStatus != domain.FosterStatusPendingBiosecurity {
		t.Errorf("expected new placement IsoQuarantineAlert=true and PENDING_BIOSECURITY_CLEARANCE, got %+v", created)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-fosterSSECh:
		if evt.Type != domain.ReunionEventType("foster_placement_updated") {
			t.Errorf("expected foster_placement_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for foster_placement_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "foster_placement_updated" {
			t.Errorf("expected foster_placement_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for foster_placement_updated mesh event")
	}

	// 5. POST /api/v1/foster-network/placements/foster-match-101/transition activates placement
	transPayload, _ := json.Marshal(map[string]any{
		"activateFosterPlacement": true,
	})
	recTrans := httptest.NewRecorder()
	reqTrans := httptest.NewRequest(http.MethodPost, "/api/v1/foster-network/placements/foster-match-101/transition", bytes.NewReader(transPayload))
	reqTrans.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTrans, reqTrans)
	if recTrans.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST transition, got %d: %s", recTrans.Code, recTrans.Body.String())
	}

	var updated domain.FosterPlacementMatch
	if err := json.Unmarshal(recTrans.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to unmarshal updated placement: %v", err)
	}
	if updated.PlacementStatus != domain.FosterStatusPlacedInFosterHome {
		t.Errorf("expected ACTIVE_IN_FOSTER_HOME, got %s", updated.PlacementStatus)
	}
	if updated.BiosecurityTier != domain.BiosecurityTierClearedStandard {
		t.Errorf("expected BIOSECURITY_CLEARED_STANDARD, got %s", updated.BiosecurityTier)
	}
	if updated.PlacementAuthorizationCode != "FOSTER-AUTH-FOSTER-MATCH-101" {
		t.Errorf("expected FOSTER-AUTH-FOSTER-MATCH-101, got %s", updated.PlacementAuthorizationCode)
	}

	// 6. Unknown placement ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/foster-network/placements/unknown-placement-id/transition", bytes.NewReader(transPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown placement ID, got %d", rec404.Code)
	}
}

func TestFosterNetworkHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/foster-network/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/foster-network/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/foster-network/stream, got %d", resp.StatusCode)
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
