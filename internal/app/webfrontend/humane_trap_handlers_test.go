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

func TestHumaneTrapHandlers_SeedingCreateTelemetryDispatchAndSightingPromotion(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "humane_traps" topic and Mesh signaling hub
	trapSSECh, unsubSSE := server.ReunionHub().Subscribe("humane_traps")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("humane_traps", "humane-trap-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /humane-traps should render HTML and seed default humane traps
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/humane-traps", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /humane-traps, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Humane Live-Trap IoT Telemetry") {
		t.Errorf("expected /humane-traps HTML to contain Humane Live-Trap IoT Telemetry heading")
	}

	// 2. GET /api/v1/humane-traps should return the 3 seeded traps
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/humane-traps", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/humane-traps, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Traps []domain.HumaneTrapDeployment `json:"traps"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal humane traps response: %v", err)
	}
	if len(listResp.Traps) != 3 {
		t.Fatalf("expected 3 seeded humane traps, got %d", len(listResp.Traps))
	}

	byID := make(map[string]domain.HumaneTrapDeployment)
	for _, tr := range listResp.Traps {
		byID[tr.TrapID] = tr
	}

	t101, ok := byID["trap-ht-101"]
	if !ok {
		t.Fatalf("expected seeded trap-ht-101")
	}
	if t101.PetName != "Luna" || t101.Status != domain.TrapStatusDoorTrippedUrgent {
		t.Errorf("expected trap-ht-101 Luna to be DOOR_TRIPPED_URGENT, got %s / %s", t101.PetName, t101.Status)
	}

	t102, ok := byID["trap-ht-102"]
	if !ok {
		t.Fatalf("expected seeded trap-ht-102")
	}
	if t102.PetName != "Buster" || t102.Status != domain.TrapStatusArmedActive || t102.BaitState != domain.BaitStateDegradingRefresh {
		t.Errorf("expected trap-ht-102 Buster to be ARMED_ACTIVE / DEGRADING_REFRESH_SOON, got %s / %s / %s", t102.PetName, t102.Status, t102.BaitState)
	}

	t103, ok := byID["trap-ht-103"]
	if !ok {
		t.Fatalf("expected seeded trap-ht-103")
	}
	if t103.PetName != "Luna" || t103.Status != domain.TrapStatusThermalLockout {
		t.Errorf("expected trap-ht-103 Luna to be THERMAL_LOCKOUT_DISARMED, got %s / %s", t103.PetName, t103.Status)
	}

	// 2b. Filter by status and petId
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/humane-traps?petId=pet-2026-002&status=DOOR_TRIPPED_URGENT", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/humane-traps, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Traps []domain.HumaneTrapDeployment `json:"traps"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered humane traps: %v", err)
	}
	if len(filteredResp.Traps) != 1 || filteredResp.Traps[0].TrapID != "trap-ht-101" {
		t.Errorf("expected 1 filtered trap (trap-ht-101), got %+v", filteredResp.Traps)
	}

	// 3. POST /api/v1/humane-traps with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":     "",
		"trapLabel": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/humane-traps", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid humane trap, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/humane-traps creates a new armed humane trap deployment (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":          "pet-2026-002",
		"petName":        "Luna",
		"targetSpecies":  "CAT",
		"targetWeightKg": 4.2,
		"trapLabel":      "Tomahawk #409 — Woodland Park Rose Garden",
		"trapType":       "BOX_TRAP_GRAVITY",
		"latitude":       47.6685,
		"longitude":      -122.3510,
		"sectorId":       "SEC-N-08",
		"baitType":       "SARDINES_IN_OIL",
		"internalTempC":  18.5,
		"batteryPct":     98,
		"rssiDbm":        -64,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/humane-traps", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/humane-traps, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createdTrap domain.HumaneTrapDeployment
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createdTrap); err != nil {
		t.Fatalf("failed to unmarshal created trap: %v", err)
	}
	if !strings.HasPrefix(createdTrap.TrapID, "trap-ht-") {
		t.Errorf("expected TrapID prefix 'trap-ht-', got %q", createdTrap.TrapID)
	}
	if createdTrap.Status != domain.TrapStatusArmedActive || createdTrap.BaitFreshnessPct != 100 || createdTrap.BaitState != domain.BaitStateFreshPeak {
		t.Errorf("expected new trap ARMED_ACTIVE with 100%% FRESH_PEAK_SCENT, got %s / %d%% / %s", createdTrap.Status, createdTrap.BaitFreshnessPct, createdTrap.BaitState)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-trapSSECh:
		if evt.Type != domain.ReunionEventType("humane_trap_updated") {
			t.Errorf("expected humane_trap_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for humane_trap_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "humane_trap_updated" {
			t.Errorf("expected humane_trap_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for humane_trap_updated mesh event")
	}

	// 5. POST /api/v1/humane-traps/{id}/telemetry simulates door trip with wildlife bycatch weight mismatch
	telemPayload, _ := json.Marshal(map[string]any{
		"doorClosed":               true,
		"internalTempC":            17.8,
		"batteryPct":               91,
		"rssiDbm":                  -66,
		"accelerometerPeakG":       3.10,
		"estimatedCaptureWeightKg": 1.2, // mismatch vs 4.2kg target
		"baitReset":                false,
		"baitType":                 "",
	})
	recTelem := httptest.NewRecorder()
	reqTelem := httptest.NewRequest(http.MethodPost, "/api/v1/humane-traps/"+createdTrap.TrapID+"/telemetry", bytes.NewReader(telemPayload))
	reqTelem.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTelem, reqTelem)
	if recTelem.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST telemetry, got %d: %s", recTelem.Code, recTelem.Body.String())
	}

	var trippedTrap domain.HumaneTrapDeployment
	if err := json.Unmarshal(recTelem.Body.Bytes(), &trippedTrap); err != nil {
		t.Fatalf("failed to unmarshal tripped trap: %v", err)
	}
	if trippedTrap.Status != domain.TrapStatusDoorTrippedUrgent || !trippedTrap.DoorClosed {
		t.Errorf("expected status DOOR_TRIPPED_URGENT and DoorClosed=true, got %s / %v", trippedTrap.Status, trippedTrap.DoorClosed)
	}
	if trippedTrap.TrippedAt == nil || trippedTrap.CheckSLADeadline == nil {
		t.Errorf("expected TrippedAt and CheckSLADeadline to be set")
	}
	hasBycatchFlag := false
	for _, f := range trippedTrap.AlertFlags {
		if f == domain.TrapAlertBycatchWeight {
			hasBycatchFlag = true
		}
	}
	if !hasBycatchFlag {
		t.Errorf("expected WEIGHT_MISMATCH_POSSIBLE_BYCATCH alert flag, got %v", trippedTrap.AlertFlags)
	}

	// 5b. Resolve bycatch release & re-arm
	bycatchResolvePayload, _ := json.Marshal(map[string]any{
		"action":  "RESOLVE",
		"outcome": "BYCATCH_RELEASED_REARMED",
		"notes":   "Non-target raccoon safely released; fresh sardines placed and trap re-armed.",
	})
	recBycatch := httptest.NewRecorder()
	reqBycatch := httptest.NewRequest(http.MethodPost, "/api/v1/humane-traps/"+createdTrap.TrapID+"/dispatch", bytes.NewReader(bycatchResolvePayload))
	reqBycatch.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recBycatch, reqBycatch)
	if recBycatch.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for bycatch resolution, got %d: %s", recBycatch.Code, recBycatch.Body.String())
	}
	var rearmedTrap domain.HumaneTrapDeployment
	if err := json.Unmarshal(recBycatch.Body.Bytes(), &rearmedTrap); err != nil {
		t.Fatalf("failed to unmarshal rearmed trap: %v", err)
	}
	if rearmedTrap.Status != domain.TrapStatusArmedActive || rearmedTrap.DoorClosed || rearmedTrap.BaitFreshnessPct != 100 {
		t.Errorf("expected rearmed trap ARMED_ACTIVE with 100%% bait, got %s / door=%v / bait=%d%%", rearmedTrap.Status, rearmedTrap.DoorClosed, rearmedTrap.BaitFreshnessPct)
	}

	// 6. POST /api/v1/humane-traps/trap-ht-101/dispatch Mode A (DISPATCH)
	dispatchPayload, _ := json.Marshal(map[string]any{
		"action":        "DISPATCH",
		"responderId":   "vol-sar-104",
		"responderName": "Maya Lin (SAR Tech)",
		"etaMinutes":    6,
	})
	recDispatch := httptest.NewRecorder()
	reqDispatch := httptest.NewRequest(http.MethodPost, "/api/v1/humane-traps/trap-ht-101/dispatch", bytes.NewReader(dispatchPayload))
	reqDispatch.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recDispatch, reqDispatch)
	if recDispatch.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for responder dispatch, got %d: %s", recDispatch.Code, recDispatch.Body.String())
	}
	var dispatchedTrap domain.HumaneTrapDeployment
	if err := json.Unmarshal(recDispatch.Body.Bytes(), &dispatchedTrap); err != nil {
		t.Fatalf("failed to unmarshal dispatched trap: %v", err)
	}
	if dispatchedTrap.Status != domain.TrapStatusResponderDispatched || dispatchedTrap.AssignedResponderID != "vol-sar-104" {
		t.Errorf("expected RESPONDER_DISPATCHED with vol-sar-104, got %s / %s", dispatchedTrap.Status, dispatchedTrap.AssignedResponderID)
	}

	// 7. POST /api/v1/humane-traps/trap-ht-101/dispatch Mode B (RESOLVE: TARGET_PET_CAPTURED) -> promotes sighting
	resolvePayload, _ := json.Marshal(map[string]any{
		"action":  "RESOLVE",
		"outcome": "TARGET_PET_CAPTURED",
		"notes":   "Verified Luna inside Tomahawk #404; microchip scanned and transfer carrier secured.",
	})
	recResolve := httptest.NewRecorder()
	reqResolve := httptest.NewRequest(http.MethodPost, "/api/v1/humane-traps/trap-ht-101/dispatch", bytes.NewReader(resolvePayload))
	reqResolve.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recResolve, reqResolve)
	if recResolve.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for TARGET_PET_CAPTURED resolution, got %d: %s", recResolve.Code, recResolve.Body.String())
	}
	var capturedTrap domain.HumaneTrapDeployment
	if err := json.Unmarshal(recResolve.Body.Bytes(), &capturedTrap); err != nil {
		t.Fatalf("failed to unmarshal captured trap: %v", err)
	}
	if capturedTrap.Status != domain.TrapStatusTargetCaptured {
		t.Errorf("expected status TARGET_CAPTURED_SECURED, got %s", capturedTrap.Status)
	}
	if !strings.HasPrefix(capturedTrap.PromotedSightingID, "sighting-trap-") {
		t.Fatalf("expected PromotedSightingID starting with 'sighting-trap-', got %q", capturedTrap.PromotedSightingID)
	}

	// Verify promoted sighting exists in store.SightingsCollection
	sightingBytes, err := memStore.GetState(context.Background(), store.SightingsCollection, capturedTrap.PromotedSightingID)
	if err != nil {
		t.Fatalf("expected promoted sighting %q in SightingsCollection: %v", capturedTrap.PromotedSightingID, err)
	}
	var promotedSighting domain.Sighting
	if err := json.Unmarshal(sightingBytes, &promotedSighting); err != nil {
		t.Fatalf("failed to unmarshal promoted sighting: %v", err)
	}
	if promotedSighting.PetID != "pet-2026-002" {
		t.Errorf("expected promoted sighting PetID 'pet-2026-002', got %q", promotedSighting.PetID)
	}

	// 8. Unknown trap ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/humane-traps/unknown-trap-id/dispatch", bytes.NewReader(resolvePayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown trap ID, got %d", rec404.Code)
	}
}

func TestHumaneTrapHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/humane-traps/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/humane-traps/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/humane-traps/stream, got %d", resp.StatusCode)
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
