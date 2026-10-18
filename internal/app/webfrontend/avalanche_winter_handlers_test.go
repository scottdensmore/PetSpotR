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

func TestAvalancheWinterHandlers_SeededEvaluateStepClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	awSSECh, unsubSSE := server.ReunionHub().Subscribe("winter_rescue")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("winter_rescue", "winter-rescue-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /avalanche-winter-rescue should render HTML with seeded operations and summary stats
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/avalanche-winter-rescue", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /avalanche-winter-rescue, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Cold-Weather Avalanche, Deep-Snow Drift",
		"stat-total-winter-ops",
		"stat-critical-burial-alerts",
		"stat-staged-rewarm-bivies",
		"stat-normothermic-safe-count",
		"avalanche-slab-burial",
		"tree-well-immersion",
		"blizzard-drift-shelter",
		"AW-OP-101",
		"AW-OP-102",
		"AW-OP-103",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /avalanche-winter-rescue HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/winter-rescue/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/winter-rescue/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/winter-rescue/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.WinterRescueOperation `json:"operations"`
		Summary    map[string]int                 `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal winter rescue operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.WinterRescueOperation)
	for _, op := range listResp.Operations {
		byID[op.OperationID] = op
	}

	op101, ok := byID["AW-OP-101"]
	if !ok || op101.PetName != "Yukon" || op101.Status != domain.WinterRescueStatusBurialHypothermiaCritical || !op101.AsphyxiaAirPocketAlert || !op101.SevereFrostbiteAfterdropAlert {
		t.Errorf("unexpected AW-OP-101 state: %+v", op101)
	}
	op102, ok := byID["AW-OP-102"]
	if !ok || op102.PetName != "Aspen" || op102.Status != domain.WinterRescueStatusProbeTrenchRewarmingStaged || op102.RewarmingReadinessScore != 100 {
		t.Errorf("unexpected AW-OP-102 state: %+v", op102)
	}
	op103, ok := byID["AW-OP-103"]
	if !ok || op103.PetName != "Freya" || op103.Status != domain.WinterRescueStatusExtractedNormothermicSafe || op103.ClearanceCertificateHash != "WINTER-SAFE-6B2C84E1A904" {
		t.Errorf("unexpected AW-OP-103 state: %+v", op103)
	}

	// 2b. Filter by status and terrainType
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/winter-rescue/operations?status=BURIAL_HYPOTHERMIA_CRITICAL&terrainType=AVALANCHE_DEBRIS_CHUTE", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.WinterRescueOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].OperationID != "AW-OP-101" {
		t.Errorf("expected 1 filtered operation AW-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/winter-rescue/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/winter-rescue/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/winter-rescue/evaluate creates a new winter rescue operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"petId":                 "PET-AW-204",
		"petName":               "Kodiak",
		"species":               "Canine",
		"weightKg":              29.0,
		"terrainType":           "AVALANCHE_DEBRIS_CHUTE",
		"burialDepthCm":         90.0,
		"windChillCelsius":      -24.0,
		"coreTempCelsius":       33.4,
		"minutesBuried":         26,
		"avalancheDangerScale":  4,
		"snowpackSectorGrid":    "CHUTE-NORTH-04A",
		"leadSkiPatrolCallsign": "PATROL-ALPINE-01",
		"notes":                 "Avalanche slab burial simulation",
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/winter-rescue/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/winter-rescue/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.WinterRescueOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.OperationID != "AW-OP-104" || !created.AsphyxiaAirPocketAlert || !created.SevereFrostbiteAfterdropAlert || created.Status != domain.WinterRescueStatusBurialHypothermiaCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-awSSECh:
		if evt.Type != domain.ReunionEventType("winter_rescue_updated") {
			t.Errorf("expected winter_rescue_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for winter_rescue_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "winter_rescue_updated" {
			t.Errorf("expected winter_rescue_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for winter_rescue_updated mesh event")
	}

	// 5. POST /api/v1/winter-rescue/operations/AW-OP-101/step updates protocol steps
	for _, stepID := range []string{"step-v-conveyor-trench", "step-horizontal-afterdrop-lift", "step-active-core-rewarm"} {
		stepPayload, _ := json.Marshal(map[string]any{
			"stepId":    stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/winter-rescue/operations/AW-OP-101/step", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/winter-rescue/operations/AW-OP-101/clearance issues certificate
	clearancePayload, _ := json.Marshal(map[string]any{
		"leadSkiPatrolCallsign": "PATROL-ALPINE-01 (Sorensen)",
		"notes":                 "Extracted safely from avalanche debris toe and core rewarmed",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/winter-rescue/operations/AW-OP-101/clearance", bytes.NewReader(clearancePayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.WinterRescueOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.WinterRescueStatusExtractedNormothermicSafe || cleared.HazardScore != 10 || !strings.HasPrefix(cleared.ClearanceCertificateHash, "WINTER-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on step and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/winter-rescue/operations/UNKNOWN-ID/step", bytes.NewReader([]byte(`{"stepId":"step-beacon-pinpoint","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on step, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/winter-rescue/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestAvalancheWinterHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/winter-rescue/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/winter-rescue/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/winter-rescue/stream, got %d", resp.StatusCode)
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
