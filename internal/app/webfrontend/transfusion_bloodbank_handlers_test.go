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
	"github.com/scottdensmore/petspotr/pkg/transfusionbloodbank"
)

func TestTransfusionBloodbankHandlers_SeededEvaluateStepsClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	tbSSECh, unsubSSE := server.ReunionHub().Subscribe("transfusion_bloodbank")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("transfusion_bloodbank", "transfusion-bloodbank-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /transfusion-bloodbank-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/transfusion-bloodbank-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /transfusion-bloodbank-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing",
		"kpi-total-ops",
		"kpi-critical-hemolytic",
		"kpi-staged-crossmatch",
		"kpi-cleared-safe",
		"filter-blood-group",
		"filter-status",
		"bloodbank-operations-list",
		"active-operation-workspace",
		"bloodbank-eval-form",
		"TB-OP-101",
		"TB-OP-102",
		"TB-OP-103",
		"Saffron",
		"Ranger",
		"Cleo",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /transfusion-bloodbank-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/transfusion-bloodbank/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/transfusion-bloodbank/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/transfusion-bloodbank/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.TransfusionBloodbankOperation `json:"operations"`
		Summary    map[string]int                         `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal transfusion bloodbank operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.TransfusionBloodbankOperation)
	for _, op := range listResp.Operations {
		byID[op.ID] = op
	}

	op101, ok := byID["TB-OP-101"]
	if !ok || op101.PetName != "Saffron" || op101.Status != domain.TransfusionStatusCriticalHemolyticAnaphylaxis || op101.BloodGroupType != domain.TransfusionBloodGroupFelineTypeBAlloantibodyRisk {
		t.Errorf("unexpected TB-OP-101 state: %+v", op101)
	}
	op102, ok := byID["TB-OP-102"]
	if !ok || op102.PetName != "Ranger" || op102.Status != domain.TransfusionStatusStagedCrossmatchedMicronFilter {
		t.Errorf("unexpected TB-OP-102 state: %+v", op102)
	}
	op103, ok := byID["TB-OP-103"]
	if !ok || op103.PetName != "Cleo" || op103.Status != domain.TransfusionStatusSafePCVRestoredNormothermic || !strings.HasPrefix(op103.ClearanceToken, "BLOODBANK-SAFE-") {
		t.Errorf("unexpected TB-OP-103 state: %+v", op103)
	}

	// 2b. Filter by ?blood_group_type= and ?status=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/transfusion-bloodbank/operations?blood_group_type=FELINE_TYPE_B_ALLOANTIBODY_RISK&status=ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.TransfusionBloodbankOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "TB-OP-101" {
		t.Errorf("expected 1 filtered operation TB-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/transfusion-bloodbank/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"pet_name": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/transfusion-bloodbank/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/transfusion-bloodbank/evaluate creates a new transfusion bloodbank operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"pet_id":                               "PET-TB-204",
		"pet_name":                             "Bramble",
		"species":                              "Cat - British Shorthair",
		"weight_kg":                            4.1,
		"blood_group_type":                     "FELINE_TYPE_B_ALLOANTIBODY_RISK",
		"recipient_pcv_pct":                    9.5,
		"donor_pcv_pct":                        42.0,
		"target_pcv_pct":                       24.0,
		"major_minor_crossmatch_compatible":    true,
		"in_line_170_micron_filter_attached":   true,
		"lactated_ringers_co_infusion_avoided": true,
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/transfusion-bloodbank/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/transfusion-bloodbank/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.TransfusionBloodbankOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "TB-OP-104" || created.Status != domain.TransfusionStatusCriticalHemolyticAnaphylaxis {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-tbSSECh:
		if evt.Type != domain.ReunionEventType("transfusion_bloodbank_updated") {
			t.Errorf("expected transfusion_bloodbank_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for transfusion_bloodbank_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "transfusion_bloodbank_updated" {
			t.Errorf("expected transfusion_bloodbank_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for transfusion_bloodbank_updated mesh event")
	}

	// 5. POST /api/v1/transfusion-bloodbank/operations/TB-OP-101/steps updates all 4 protocol steps
	for _, stepID := range []string{
		transfusionbloodbank.StepABDEA11TypingSlideCrossmatch,
		transfusionbloodbank.Step170MicronFilter09NaClOnlyLine,
		transfusionbloodbank.StepSlow025MlKg15MinBiologicTrial,
		transfusionbloodbank.StepPostPCVTotalSolidsHemolysisCheck,
	} {
		stepPayload, _ := json.Marshal(map[string]any{
			"step_id":   stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/transfusion-bloodbank/operations/TB-OP-101/steps", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/transfusion-bloodbank/operations/TB-OP-101/clearance issues BLOODBANK-SAFE-<12HEX> token
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/transfusion-bloodbank/operations/TB-OP-101/clearance", bytes.NewReader([]byte(`{}`)))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.TransfusionBloodbankOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.TransfusionStatusSafePCVRestoredNormothermic || !strings.HasPrefix(cleared.ClearanceToken, "BLOODBANK-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on steps and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/transfusion-bloodbank/operations/UNKNOWN-ID/steps", bytes.NewReader([]byte(`{"step_id":"STEP-AB-DEA11-TYPING-SLIDE-CROSSMATCH","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on steps, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/transfusion-bloodbank/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestTransfusionBloodbankHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/transfusion-bloodbank/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/transfusion-bloodbank/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/transfusion-bloodbank/stream, got %d", resp.StatusCode)
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
