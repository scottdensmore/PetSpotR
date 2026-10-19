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

func TestMaritimeCoastalHandlers_SeededEvaluateStepClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	mcSSECh, unsubSSE := server.ReunionHub().Subscribe("maritime_rescue")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("maritime_rescue", "maritime-rescue-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /maritime-coastal-rescue renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/maritime-coastal-rescue", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /maritime-coastal-rescue, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Maritime, Coastal &amp; Tidal Mudflat Pet Rescue Operations Cockpit",
		"kpi-total-ops",
		"kpi-critical-tide",
		"kpi-staged-vessel",
		"kpi-extracted-safe",
		"filter-zone",
		"filter-status",
		"maritime-operations-list",
		"active-operation-workspace",
		"maritime-eval-form",
		"MC-OP-101",
		"MC-OP-102",
		"MC-OP-103",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /maritime-coastal-rescue HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/maritime-rescue/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/maritime-rescue/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/maritime-rescue/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.MaritimeRescueOperation `json:"operations"`
		Total      int                              `json:"total"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal maritime rescue operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.MaritimeRescueOperation)
	for _, op := range listResp.Operations {
		byID[op.ID] = op
	}

	op101, ok := byID["MC-OP-101"]
	if !ok || op101.PetName != "Anchor" || op101.Status != domain.MaritimeStatusTidalFloodAspirationCritical {
		t.Errorf("unexpected MC-OP-101 state: %+v", op101)
	}
	op102, ok := byID["MC-OP-102"]
	if !ok || op102.PetName != "Coral" || op102.Status != domain.MaritimeStatusHovercraftMudSledStaged {
		t.Errorf("unexpected MC-OP-102 state: %+v", op102)
	}
	op103, ok := byID["MC-OP-103"]
	if !ok || op103.PetName != "Skipper" || op103.Status != domain.MaritimeStatusExtractedHarborDeckSafe || !strings.HasPrefix(op103.ClearanceToken, "MARITIME-SAFE-") {
		t.Errorf("unexpected MC-OP-103 state: %+v", op103)
	}

	// 2b. Filter by zone and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/maritime-rescue/operations?zone=TIDAL_MUDFLAT_ESTUARY&status=TIDAL_FLOOD_ASPIRATION_CRITICAL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.MaritimeRescueOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "MC-OP-101" {
		t.Errorf("expected 1 filtered operation MC-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/maritime-rescue/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"pet_name": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/maritime-rescue/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/maritime-rescue/evaluate creates a new maritime rescue operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"pet_name":                       "Harbor",
		"species":                        "Dog - Chesapeake Bay Retriever",
		"weight_kg":                      34.0,
		"maritime_zone":                  "TIDAL_MUDFLAT_ESTUARY",
		"tide_surge_rate_cm_hr":          55.0,
		"swell_height_meters":            1.4,
		"water_temp_celsius":             9.0,
		"saltwater_aspiration_suspected": true,
		"mud_suction_depth_cm":           42.0,
		"mud_walkers_air_lance_deployed": false,
		"canine_pfd_harness_secured":     false,
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/maritime-rescue/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/maritime-rescue/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.MaritimeRescueOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "MC-OP-104" || created.Status != domain.MaritimeStatusTidalFloodAspirationCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-mcSSECh:
		if evt.Type != domain.ReunionEventType("maritime_rescue_updated") {
			t.Errorf("expected maritime_rescue_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for maritime_rescue_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "maritime_rescue_updated" {
			t.Errorf("expected maritime_rescue_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for maritime_rescue_updated mesh event")
	}

	// 5. POST /api/v1/maritime-rescue/operations/MC-OP-101/steps updates protocol steps
	for _, stepID := range []string{
		"STEP-TIDE-SUCTION-RELIEF",
		"STEP-VESSEL-PFD-RIGGING",
		"STEP-SALTWATER-ASPIRATION-O2",
		"STEP-HARBOR-DECK-DECON-WARM",
	} {
		stepPayload, _ := json.Marshal(map[string]any{
			"step_id":   stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/maritime-rescue/operations/MC-OP-101/steps", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/maritime-rescue/operations/MC-OP-101/clearance issues token
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/maritime-rescue/operations/MC-OP-101/clearance", bytes.NewReader([]byte(`{}`)))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.MaritimeRescueOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.MaritimeStatusExtractedHarborDeckSafe || !strings.HasPrefix(cleared.ClearanceToken, "MARITIME-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on steps and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/maritime-rescue/operations/UNKNOWN-ID/steps", bytes.NewReader([]byte(`{"step_id":"STEP-TIDE-SUCTION-RELIEF","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on steps, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/maritime-rescue/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestMaritimeCoastalHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/maritime-rescue/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/maritime-rescue/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/maritime-rescue/stream, got %d", resp.StatusCode)
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
