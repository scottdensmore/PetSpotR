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
	"github.com/scottdensmore/petspotr/pkg/envenomationtoxin"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestEnvenomationToxinHandlers_SeededEvaluateStepsClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	toxinSSECh, unsubSSE := server.ReunionHub().Subscribe("envenomation_toxin")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("envenomation_toxin", "envenomation-toxin-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /envenomation-toxin-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/envenomation-toxin-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /envenomation-toxin-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Pit-Viper Envenomation",
		"kpi-total-ops",
		"kpi-critical-toxin",
		"kpi-staged-antivenom",
		"kpi-stabilized-safe",
		"filter-toxin-type",
		"filter-status",
		"toxin-operations-list",
		"active-operation-workspace",
		"toxin-eval-form",
		"ET-OP-101",
		"ET-OP-102",
		"ET-OP-103",
		"Copper",
		"Bramble",
		"Mango",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /envenomation-toxin-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/envenomation-toxin/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/envenomation-toxin/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/envenomation-toxin/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.EnvenomationTriageOperation `json:"operations"`
		Summary    map[string]int                       `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal envenomation toxin operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.EnvenomationTriageOperation)
	for _, op := range listResp.Operations {
		byID[op.ID] = op
	}

	op101, ok := byID["ET-OP-101"]
	if !ok || op101.PetName != "Copper" || op101.Status != domain.EnvenomationStatusCritical || op101.AntivenomVialsStaged != 2 {
		t.Errorf("unexpected ET-OP-101 state: %+v", op101)
	}
	op102, ok := byID["ET-OP-102"]
	if !ok || op102.PetName != "Bramble" || op102.Status != domain.EnvenomationStatusStaged || op102.QuillCountMuzzleThorax != 14 {
		t.Errorf("unexpected ET-OP-102 state: %+v", op102)
	}
	op103, ok := byID["ET-OP-103"]
	if !ok || op103.PetName != "Mango" || op103.Status != domain.EnvenomationStatusStabilized || !strings.HasPrefix(op103.ClearanceToken, "TOXIN-SAFE-") {
		t.Errorf("unexpected ET-OP-103 state: %+v", op103)
	}

	// 2b. Filter by ?toxin_type= and ?status=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/envenomation-toxin/operations?toxin_type=PIT_VIPER_CROTALID_HEI&status=COAGULOPATHY_NEUROTOXIC_CRITICAL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.EnvenomationTriageOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "ET-OP-101" {
		t.Errorf("expected 1 filtered operation ET-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/envenomation-toxin/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"pet_name": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/envenomation-toxin/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/envenomation-toxin/evaluate creates a new envenomation operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"pet_id":                             "PET-ET-204",
		"pet_name":                           "Sidewinder",
		"species":                            "Dog - Catahoula Leopard Dog",
		"weight_kg":                          24.8,
		"toxin_type":                         "PIT_VIPER_CROTALID_HEI",
		"edema_spread_cm_hr":                 7.2,
		"echinocyte_pct":                     42.0,
		"quill_count_muzzle_thorax":          0,
		"oral_mucosa_flushed_forward":        true,
		"quills_cut_or_pulled_awake_avoided": true,
		"antivenom_vials_staged":             3,
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/envenomation-toxin/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/envenomation-toxin/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.EnvenomationTriageOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "ET-OP-104" || created.Status != domain.EnvenomationStatusCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-toxinSSECh:
		if evt.Type != domain.ReunionEventType("envenomation_toxin_updated") {
			t.Errorf("expected envenomation_toxin_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for envenomation_toxin_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "envenomation_toxin_updated" {
			t.Errorf("expected envenomation_toxin_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for envenomation_toxin_updated mesh event")
	}

	// 5. POST /api/v1/envenomation-toxin/operations/ET-OP-101/steps updates all 4 protocol steps
	for _, stepID := range []string{
		envenomationtoxin.StepDemarcationPerimeterMark,
		envenomationtoxin.StepOralForwardLavageAntivenom,
		envenomationtoxin.StepSedatedQuillExtraction,
		envenomationtoxin.StepCoagEchinocyteRecheck,
	} {
		stepPayload, _ := json.Marshal(map[string]any{
			"step_id":   stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/envenomation-toxin/operations/ET-OP-101/steps", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/envenomation-toxin/operations/ET-OP-101/clearance issues TOXIN-SAFE-<12HEX> token
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/envenomation-toxin/operations/ET-OP-101/clearance", bytes.NewReader([]byte(`{}`)))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.EnvenomationTriageOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.EnvenomationStatusStabilized || !strings.HasPrefix(cleared.ClearanceToken, "TOXIN-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on steps and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/envenomation-toxin/operations/UNKNOWN-ID/steps", bytes.NewReader([]byte(`{"step_id":"STEP-DEMARCATION-PERIMETER-MARK","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on steps, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/envenomation-toxin/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestEnvenomationToxinHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/envenomation-toxin/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/envenomation-toxin/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/envenomation-toxin/stream, got %d", resp.StatusCode)
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
