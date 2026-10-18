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

func TestSwiftwaterRescueHandlers_SeedingCreateExtractAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	swSSECh, unsubSSE := server.ReunionHub().Subscribe("swiftwater_rescue")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("swiftwater_rescue", "swiftwater-rescue-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /swiftwater-rescue should render HTML and seed default ops
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/swiftwater-rescue", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /swiftwater-rescue, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Aquatic, Floodwater &amp; Ice-Rescue Swiftwater") &&
		!strings.Contains(recPage.Body.String(), "Aquatic, Floodwater & Ice-Rescue Swiftwater") {
		t.Errorf("expected /swiftwater-rescue HTML to contain Aquatic, Floodwater & Ice-Rescue Swiftwater heading")
	}

	// 2. GET /api/v1/swiftwater-rescue/ops should return the 3 seeded ops
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/swiftwater-rescue/ops", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/swiftwater-rescue/ops, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Ops []domain.SwiftwaterRescueOp `json:"ops"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal swiftwater rescue ops response: %v", err)
	}
	if len(listResp.Ops) != 3 {
		t.Fatalf("expected 3 seeded swiftwater rescue ops, got %d", len(listResp.Ops))
	}

	byID := make(map[string]domain.SwiftwaterRescueOp)
	for _, op := range listResp.Ops {
		byID[op.OpID] = op
	}

	o101, ok := byID["sw-op-101"]
	if !ok {
		t.Fatalf("expected seeded sw-op-101")
	}
	if o101.PetName != "Yukon" || o101.ExtractionStatus != domain.SwiftwaterStatusStrandedInCurrent || o101.HydroRisk != domain.HydroRiskExtremeIceHypothermia || !o101.ColdShockAlert {
		t.Errorf("expected sw-op-101 Yukon in STRANDED_IN_SWIFTWATER / EXTREME_ICE_HYPOTHERMIA_RISK with ColdShockAlert=true, got %+v", o101)
	}

	o102, ok := byID["sw-op-102"]
	if !ok {
		t.Fatalf("expected seeded sw-op-102")
	}
	if o102.PetName != "Misty" || o102.ExtractionStatus != domain.SwiftwaterStatusThrowBagRigged || o102.HydroRisk != domain.HydroRiskElevatedFloodWashout {
		t.Errorf("expected sw-op-102 Misty in THROW_BAG_RIGGED_APPROACH / ELEVATED_FLOOD_CURRENT_RISK, got %+v", o102)
	}

	o103, ok := byID["sw-op-103"]
	if !ok {
		t.Fatalf("expected seeded sw-op-103")
	}
	if o103.PetName != "Harbor" || o103.ExtractionStatus != domain.SwiftwaterStatusExtractedWarming || o103.HydroRisk != domain.HydroRiskShorelineStabilized || o103.ExtractionClearanceCode != "SWIFTWATER-SAFE-SW-OP-103" {
		t.Errorf("expected sw-op-103 Harbor in EXTRACTED_WARMING_STABLE / SHORELINE_REWARMED_SAFE with SWIFTWATER-SAFE-SW-OP-103, got %+v", o103)
	}

	// 2b. Filter by petId and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/swiftwater-rescue/ops?petId=pet-2026-001&status=STRANDED_IN_SWIFTWATER", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/swiftwater-rescue/ops, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Ops []domain.SwiftwaterRescueOp `json:"ops"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered ops: %v", err)
	}
	if len(filteredResp.Ops) != 1 || filteredResp.Ops[0].OpID != "sw-op-101" {
		t.Errorf("expected 1 filtered op (sw-op-101), got %+v", filteredResp.Ops)
	}

	// 3. POST /api/v1/swiftwater-rescue/ops with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":   "",
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/swiftwater-rescue/ops", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid swiftwater op, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/swiftwater-rescue/ops creates a new swiftwater op (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":                "pet-2026-001",
		"petName":              "Glacier",
		"species":              "DOG",
		"waterwayName":         "Bitter Lake Fractured Ice Shelf",
		"hazardEnvironment":    "FRACTURED_LAKE_ICE_SHELF",
		"waterTempFahrenheit":  33,
		"currentVelocityKnots": 2.0,
		"pfdThrowBagDeployed":  false,
		"warmingBlanketReady":  false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/swiftwater-rescue/ops", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/swiftwater-rescue/ops, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.SwiftwaterRescueOp
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created swiftwater op: %v", err)
	}
	if !strings.HasPrefix(created.OpID, "sw-op-") {
		t.Errorf("expected OpID prefix 'sw-op-', got %q", created.OpID)
	}
	if !created.ColdShockAlert || created.ExtractionStatus != domain.SwiftwaterStatusStrandedInCurrent {
		t.Errorf("expected new op ColdShockAlert=true and STRANDED_IN_SWIFTWATER, got %+v", created)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-swSSECh:
		if evt.Type != domain.ReunionEventType("swiftwater_op_updated") {
			t.Errorf("expected swiftwater_op_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for swiftwater_op_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "swiftwater_op_updated" {
			t.Errorf("expected swiftwater_op_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for swiftwater_op_updated mesh event")
	}

	// 5. POST /api/v1/swiftwater-rescue/ops/sw-op-101/extract completes extraction
	extractPayload, _ := json.Marshal(map[string]any{
		"completeShorelineExtraction": true,
	})
	recExtract := httptest.NewRecorder()
	reqExtract := httptest.NewRequest(http.MethodPost, "/api/v1/swiftwater-rescue/ops/sw-op-101/extract", bytes.NewReader(extractPayload))
	reqExtract.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recExtract, reqExtract)
	if recExtract.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST extract, got %d: %s", recExtract.Code, recExtract.Body.String())
	}

	var updated domain.SwiftwaterRescueOp
	if err := json.Unmarshal(recExtract.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to unmarshal updated swiftwater op: %v", err)
	}
	if updated.ExtractionStatus != domain.SwiftwaterStatusExtractedWarming {
		t.Errorf("expected EXTRACTED_WARMING_STABLE, got %s", updated.ExtractionStatus)
	}
	if updated.HydroRisk != domain.HydroRiskShorelineStabilized {
		t.Errorf("expected SHORELINE_REWARMED_SAFE, got %s", updated.HydroRisk)
	}
	if updated.ExtractionClearanceCode != "SWIFTWATER-SAFE-SW-OP-101" {
		t.Errorf("expected SWIFTWATER-SAFE-SW-OP-101, got %s", updated.ExtractionClearanceCode)
	}

	// 6. Unknown op ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/swiftwater-rescue/ops/unknown-op-id/extract", bytes.NewReader(extractPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op ID, got %d", rec404.Code)
	}
}

func TestSwiftwaterRescueHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/swiftwater-rescue/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/swiftwater-rescue/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/swiftwater-rescue/stream, got %d", resp.StatusCode)
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
