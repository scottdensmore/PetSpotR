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
	"github.com/scottdensmore/petspotr/pkg/acousticpanic"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestAcousticPanicHandlers_SeededEvaluateStepsClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	acousticSSECh, unsubSSE := server.ReunionHub().Subscribe("acoustic_panic")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("acoustic_panic", "acoustic-panic-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /acoustic-panic-trajectory renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/acoustic-panic-trajectory", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /acoustic-panic-trajectory, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acoustic Panic",
		"kpi-total-ops",
		"kpi-critical-bolt",
		"kpi-staged-lure",
		"kpi-decompressed-safe",
		"filter-trigger-type",
		"filter-status",
		"acoustic-operations-list",
		"active-operation-workspace",
		"acoustic-eval-form",
		"AP-OP-101",
		"AP-OP-102",
		"AP-OP-103",
		"Boomer",
		"Willow",
		"Milo",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /acoustic-panic-trajectory HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/acoustic-panic/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/acoustic-panic/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/acoustic-panic/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.AcousticPanicOperation `json:"operations"`
		Summary    map[string]int                  `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal acoustic panic operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.AcousticPanicOperation)
	for _, op := range listResp.Operations {
		byID[op.ID] = op
	}

	op101, ok := byID["AP-OP-101"]
	if !ok || op101.PetName != "Boomer" || op101.Status != domain.AcousticStatusCriticalBolt || !op101.BarrierBreachInjurySuspected {
		t.Errorf("unexpected AP-OP-101 state: %+v", op101)
	}
	op102, ok := byID["AP-OP-102"]
	if !ok || op102.PetName != "Willow" || op102.Status != domain.AcousticStatusStagedSniff {
		t.Errorf("unexpected AP-OP-102 state: %+v", op102)
	}
	op103, ok := byID["AP-OP-103"]
	if !ok || op103.PetName != "Milo" || op103.Status != domain.AcousticStatusDecompressedSafe || !strings.HasPrefix(op103.ClearanceToken, "ACOUSTIC-SAFE-") {
		t.Errorf("unexpected AP-OP-103 state: %+v", op103)
	}

	// 2b. Filter by ?trigger_type= and ?status=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/acoustic-panic/operations?trigger_type=JULY4_NYE_FIREWORKS_BARRAGE&status=ADRENAL_FLIGHT_BOLT_CRITICAL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.AcousticPanicOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "AP-OP-101" {
		t.Errorf("expected 1 filtered operation AP-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/acoustic-panic/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"pet_name": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/acoustic-panic/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/acoustic-panic/evaluate creates a new acoustic panic operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"pet_id":                          "PET-AP-204",
		"pet_name":                        "Ranger",
		"species":                         "Dog - Belgian Malinois",
		"weight_kg":                       28.4,
		"trigger_type":                    "JULY4_NYE_FIREWORKS_BARRAGE",
		"peak_decibels_db":                136.0,
		"barometric_drop_hpa":             2.0,
		"bolt_heading_degrees":            225,
		"barrier_breach_injury_suspected": true,
		"no_chase_passive_lure_enforced":  true,
		"adaptil_feliway_station_staged":  true,
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/acoustic-panic/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/acoustic-panic/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.AcousticPanicOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "AP-OP-104" || created.Status != domain.AcousticStatusCriticalBolt {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-acousticSSECh:
		if evt.Type != domain.ReunionEventType("acoustic_panic_updated") {
			t.Errorf("expected acoustic_panic_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for acoustic_panic_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "acoustic_panic_updated" {
			t.Errorf("expected acoustic_panic_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for acoustic_panic_updated mesh event")
	}

	// 5. POST /api/v1/acoustic-panic/operations/AP-OP-101/steps updates all 4 protocol steps
	for _, stepID := range []string{
		acousticpanic.StepSilentNoChasePerimeter,
		acousticpanic.StepScentPheromoneRefugeLure,
		acousticpanic.StepBarrierLacerationClawCheck,
		acousticpanic.StepThundershirtAcousticDecompress,
	} {
		stepPayload, _ := json.Marshal(map[string]any{
			"step_id":   stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/acoustic-panic/operations/AP-OP-101/steps", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/acoustic-panic/operations/AP-OP-101/clearance issues ACOUSTIC-SAFE-<12HEX> token
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/acoustic-panic/operations/AP-OP-101/clearance", bytes.NewReader([]byte(`{}`)))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.AcousticPanicOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.AcousticStatusDecompressedSafe || !strings.HasPrefix(cleared.ClearanceToken, "ACOUSTIC-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on steps and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/acoustic-panic/operations/UNKNOWN-ID/steps", bytes.NewReader([]byte(`{"step_id":"STEP-SILENT-NO-CHASE-PERIMETER","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on steps, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/acoustic-panic/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestAcousticPanicHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/acoustic-panic/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/acoustic-panic/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/acoustic-panic/stream, got %d", resp.StatusCode)
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
