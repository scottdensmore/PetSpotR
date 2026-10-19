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
	"github.com/scottdensmore/petspotr/pkg/dystociaeclampsia"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestDystociaEclampsiaHandlers_SeededEvaluateStepsClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	dystociaSSECh, unsubSSE := server.ReunionHub().Subscribe("dystocia_eclampsia")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("dystocia_eclampsia", "dystocia-eclampsia-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /dystocia-eclampsia-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/dystocia-eclampsia-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /dystocia-eclampsia-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Dystocia, Maternal C-Section",
		"kpi-total-ops",
		"kpi-critical-fetal",
		"kpi-staged-whelping",
		"kpi-delivered-safe",
		"filter-presentation-type",
		"filter-status",
		"dystocia-operations-list",
		"active-operation-workspace",
		"dystocia-eval-form",
		"DE-OP-101",
		"DE-OP-102",
		"DE-OP-103",
		"Penelope",
		"Maple",
		"Rosie",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /dystocia-eclampsia-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/dystocia-eclampsia/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/dystocia-eclampsia/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/dystocia-eclampsia/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.DystociaEclampsiaOperation `json:"operations"`
		Summary    map[string]int                      `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal dystocia eclampsia operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.DystociaEclampsiaOperation)
	for _, op := range listResp.Operations {
		byID[op.ID] = op
	}

	op101, ok := byID["DE-OP-101"]
	if !ok || op101.PetName != "Penelope" || op101.Status != domain.DystociaStatusFetalDistressObstructiveCritical || !op101.GreenBlackUteroverdinNoFetus {
		t.Errorf("unexpected DE-OP-101 state: %+v", op101)
	}
	op102, ok := byID["DE-OP-102"]
	if !ok || op102.PetName != "Maple" || op102.Status != domain.DystociaStatusCalciumECGWhelpingStaged {
		t.Errorf("unexpected DE-OP-102 state: %+v", op102)
	}
	op103, ok := byID["DE-OP-103"]
	if !ok || op103.PetName != "Rosie" || op103.Status != domain.DystociaStatusDeliveredNormocalcemicSafe || !strings.HasPrefix(op103.ClearanceToken, "DYSTOCIA-SAFE-") {
		t.Errorf("unexpected DE-OP-103 state: %+v", op103)
	}

	// 2b. Filter by ?presentation_type= and ?status=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/dystocia-eclampsia/operations?presentation_type=OBSTRUCTIVE_BRACHYCEPHALIC_FETAL_OVERSIZE&status=FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.DystociaEclampsiaOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "DE-OP-101" {
		t.Errorf("expected 1 filtered operation DE-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/dystocia-eclampsia/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"pet_name": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/dystocia-eclampsia/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/dystocia-eclampsia/evaluate creates a new dystocia eclampsia operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"pet_id":                            "PET-DE-204",
		"pet_name":                          "Duchess",
		"species":                           "Dog - French Bulldog Dam",
		"weight_kg":                         11.4,
		"presentation_type":                 "OBSTRUCTIVE_BRACHYCEPHALIC_FETAL_OVERSIZE",
		"lowest_fetal_heart_rate_bpm":       148,
		"ionized_calcium_mmol_l":            0.82,
		"inter_puppy_interval_mins":         160,
		"green_black_uteroverdin_no_fetus":  true,
		"pelvic_canal_obstruction_excluded": false,
		"ecg_monitored_calcium_infusion":    true,
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/dystocia-eclampsia/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/dystocia-eclampsia/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.DystociaEclampsiaOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "DE-OP-104" || created.Status != domain.DystociaStatusFetalDistressObstructiveCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-dystociaSSECh:
		if evt.Type != domain.ReunionEventType("dystocia_eclampsia_updated") {
			t.Errorf("expected dystocia_eclampsia_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for dystocia_eclampsia_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "dystocia_eclampsia_updated" {
			t.Errorf("expected dystocia_eclampsia_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for dystocia_eclampsia_updated mesh event")
	}

	// 5. POST /api/v1/dystocia-eclampsia/operations/DE-OP-101/steps updates all 4 protocol steps
	for _, stepID := range []string{
		dystociaeclampsia.StepDopplerFetalHRPelvicXray,
		dystociaeclampsia.StepECGCalciumGluconateInfusion,
		dystociaeclampsia.StepOxytocinGateOrEmergencyCSect,
		dystociaeclampsia.StepNeonatalResusColostrumLatch,
	} {
		stepPayload, _ := json.Marshal(map[string]any{
			"step_id":   stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/dystocia-eclampsia/operations/DE-OP-101/steps", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/dystocia-eclampsia/operations/DE-OP-101/clearance issues DYSTOCIA-SAFE-<12HEX> token
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/dystocia-eclampsia/operations/DE-OP-101/clearance", bytes.NewReader([]byte(`{}`)))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.DystociaEclampsiaOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.DystociaStatusDeliveredNormocalcemicSafe || !strings.HasPrefix(cleared.ClearanceToken, "DYSTOCIA-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on steps and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/dystocia-eclampsia/operations/UNKNOWN-ID/steps", bytes.NewReader([]byte(`{"step_id":"STEP-DOPPLER-FETAL-HR-PELVIC-XRAY","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on steps, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/dystocia-eclampsia/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestDystociaEclampsiaHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/dystocia-eclampsia/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/dystocia-eclampsia/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/dystocia-eclampsia/stream, got %d", resp.StatusCode)
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
