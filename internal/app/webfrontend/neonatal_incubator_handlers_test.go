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

func TestNeonatalIncubatorHandlers_SeededEvaluateStepsClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	niSSECh, unsubSSE := server.ReunionHub().Subscribe("neonatal_triage")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("neonatal_triage", "neonatal-triage-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /neonatal-incubator-triage renders HTML with seeded operations, KPI telemetry, filters, and form inputs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/neonatal-incubator-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /neonatal-incubator-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Neonatal Orphan Litter &amp; Incubator Critical Care Triage Cockpit",
		"kpi-total-ops",
		"kpi-critical-fading",
		"kpi-staged-incubator",
		"kpi-stabilized-safe",
		"filter-litter-type",
		"filter-status",
		"neonatal-operations-list",
		"active-operation-workspace",
		"neonatal-eval-form",
		"eval-pet-name",
		"eval-species",
		"eval-weight-grams",
		"eval-litter-count",
		"eval-age-days",
		"eval-litter-type",
		"eval-rectal-temp-c",
		"eval-glucose-mgdl",
		"eval-humidity-pct",
		"eval-suckling-reflex",
		"eval-warmed-gate",
		"eval-dextrose-applied",
		"eval-error-banner",
		"NI-OP-101",
		"NI-OP-102",
		"NI-OP-103",
		"Clover Litter (",
		"Pippin (",
		"Buttercup Litter (",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /neonatal-incubator-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/neonatal-triage/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/neonatal-triage/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/neonatal-triage/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.NeonatalTriageOperation `json:"operations"`
		Summary    map[string]int                   `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal neonatal triage operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.NeonatalTriageOperation)
	for _, op := range listResp.Operations {
		byID[op.ID] = op
	}

	op101, ok := byID["NI-OP-101"]
	if !ok || op101.PetName != "Clover Litter" || op101.Status != domain.NeonatalTriageStatusFadingTriadHypothermicCritical || op101.NeonateRectalTempCelsius != 33.8 {
		t.Errorf("unexpected NI-OP-101 state: %+v", op101)
	}
	op102, ok := byID["NI-OP-102"]
	if !ok || op102.PetName != "Pippin" || op102.Status != domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged || op102.WeightGrams != 185.0 {
		t.Errorf("unexpected NI-OP-102 state: %+v", op102)
	}
	op103, ok := byID["NI-OP-103"]
	if !ok || op103.PetName != "Buttercup Litter" || op103.Status != domain.NeonatalTriageStatusStabilizedNursingGainingSafe || !strings.HasPrefix(op103.ClearanceToken, "NEONATAL-SAFE-") {
		t.Errorf("unexpected NI-OP-103 state: %+v", op103)
	}

	// 2b. Filter by litter_type and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/neonatal-triage/operations?litter_type=UNWEANED_KITTEN_LITTER_0_2WK&status=FADING_TRIAD_HYPOTHERMIC_CRITICAL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.NeonatalTriageOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "NI-OP-101" {
		t.Errorf("expected 1 filtered operation NI-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/neonatal-triage/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"pet_name": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/neonatal-triage/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/neonatal-triage/evaluate creates a new neonatal triage operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"pet_name":                    "Thistle Litter",
		"species":                     "Cat - Domestic Shorthair Neonates",
		"weight_grams":                86.0,
		"litter_count":                4,
		"age_days":                    3,
		"litter_type":                 "UNWEANED_KITTEN_LITTER_0_2WK",
		"neonate_rectal_temp_celsius": 33.4,
		"blood_glucose_mg_dl":         41.0,
		"incubator_humidity_pct":      64.0,
		"suckling_reflex_present":     false,
		"warmed_before_feeding_gate":  false,
		"sublingual_dextrose_applied": true,
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/neonatal-triage/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/neonatal-triage/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.NeonatalTriageOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "NI-OP-104" || created.Status != domain.NeonatalTriageStatusFadingTriadHypothermicCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-niSSECh:
		if evt.Type != domain.ReunionEventType("neonatal_triage_updated") {
			t.Errorf("expected neonatal_triage_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for neonatal_triage_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "neonatal_triage_updated" {
			t.Errorf("expected neonatal_triage_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for neonatal_triage_updated mesh event")
	}

	// 5. POST /api/v1/neonatal-triage/operations/NI-OP-101/clearance before all steps are done returns 400
	recPrematureClear := httptest.NewRecorder()
	reqPrematureClear := httptest.NewRequest(http.MethodPost, "/api/v1/neonatal-triage/operations/NI-OP-101/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(recPrematureClear, reqPrematureClear)
	if recPrematureClear.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request issuing clearance with incomplete steps, got %d", recPrematureClear.Code)
	}

	// 6. POST /api/v1/neonatal-triage/operations/NI-OP-101/steps updates protocol steps
	for _, stepID := range []string{
		"STEP-INCUBATOR-WARMING-GATE",
		"STEP-SUBLINGUAL-DEXTROSE-50",
		"STEP-OROGASTRIC-TUBE-GAVAGE",
		"STEP-ANOGENITAL-STIM-WEIGH",
	} {
		stepPayload, _ := json.Marshal(map[string]any{
			"step_id":   stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/neonatal-triage/operations/NI-OP-101/steps", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 7. POST /api/v1/neonatal-triage/operations/NI-OP-101/clearance issues token
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/neonatal-triage/operations/NI-OP-101/clearance", bytes.NewReader([]byte(`{}`)))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.NeonatalTriageOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.NeonatalTriageStatusStabilizedNursingGainingSafe || !strings.HasPrefix(cleared.ClearanceToken, "NEONATAL-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 8. 404 Not Found for unknown operation ID on steps and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/neonatal-triage/operations/UNKNOWN-ID/steps", bytes.NewReader([]byte(`{"step_id":"STEP-INCUBATOR-WARMING-GATE","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on steps, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/neonatal-triage/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestNeonatalIncubatorHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/neonatal-triage/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/neonatal-triage/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/neonatal-triage/stream, got %d", resp.StatusCode)
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
