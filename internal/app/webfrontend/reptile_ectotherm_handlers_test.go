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

func TestReptileEctothermHandlers_SeededEvaluateStepClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	reSSECh, unsubSSE := server.ReunionHub().Subscribe("ectotherm_rescue")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("ectotherm_rescue", "ectotherm-rescue-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /reptile-ectotherm-rescue should render HTML with seeded operations and summary stats
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/reptile-ectotherm-rescue", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /reptile-ectotherm-rescue, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Ectothermic Reptile, Chelonian &amp; Amphibian Thermal-Dormancy",
		"stat-total-ectotherm-ops",
		"stat-critical-torpor-alerts",
		"stat-staged-basking-lures",
		"stat-thermal-safe-count",
		"sulcata-cold-burrow",
		"python-appliance-void",
		"dragon-sunroom-crevice",
		"RE-OP-101",
		"RE-OP-102",
		"RE-OP-103",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /reptile-ectotherm-rescue HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/ectotherm-rescue/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/ectotherm-rescue/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/ectotherm-rescue/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.EctothermRescueOperation `json:"operations"`
		Summary    map[string]int                    `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal ectotherm rescue operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.EctothermRescueOperation)
	for _, op := range listResp.Operations {
		byID[op.OperationID] = op
	}

	op101, ok := byID["RE-OP-101"]
	if !ok || op101.PetName != "Tank" || op101.Status != domain.EctothermRescueStatusColdTorporBrumationCritical || !op101.CriticalTorporAlert || !op101.DesiccationDysecdysisAlert {
		t.Errorf("unexpected RE-OP-101 state: %+v", op101)
	}
	op102, ok := byID["RE-OP-102"]
	if !ok || op102.PetName != "Saffron" || op102.Status != domain.EctothermRescueStatusBaskingLureCreviceStaged || op102.ThermalReadinessScore != 100 {
		t.Errorf("unexpected RE-OP-102 state: %+v", op102)
	}
	op103, ok := byID["RE-OP-103"]
	if !ok || op103.PetName != "Figment" || op103.Status != domain.EctothermRescueStatusRecoveredThermalGradientSafe || op103.ClearanceCertificateHash != "ECTOTHERM-SAFE-8A3F19C4E207" {
		t.Errorf("unexpected RE-OP-103 state: %+v", op103)
	}

	// 2b. Filter by status and taxonType
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/ectotherm-rescue/operations?status=COLD_TORPOR_BRUMATION_CRITICAL&taxonType=CHELONIAN_TORTOISE_BURROW", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.EctothermRescueOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].OperationID != "RE-OP-101" {
		t.Errorf("expected 1 filtered operation RE-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/ectotherm-rescue/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/ectotherm-rescue/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/ectotherm-rescue/evaluate creates a new ectotherm rescue operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"petId":                 "PET-RE-204",
		"petName":               "Goliath",
		"species":               "Reptile",
		"weightKg":              22.0,
		"taxonType":             "CHELONIAN_TORTOISE_BURROW",
		"substrateTempCelsius":  10.8,
		"preferredOptimumTempC": 31.5,
		"relativeHumidityPct":   26.0,
		"hoursDormant":          52.0,
		"burrowDepthCm":         115.0,
		"microHabitatGrid":      "BURROW-SOUTH-03A",
		"leadHerpTechCallsign":  "HERP-TECH-01",
		"notes":                 "Sulcata tortoise cold-stun burrow simulation",
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/ectotherm-rescue/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/ectotherm-rescue/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.EctothermRescueOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.OperationID != "RE-OP-104" || !created.CriticalTorporAlert || !created.DesiccationDysecdysisAlert || created.Status != domain.EctothermRescueStatusColdTorporBrumationCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-reSSECh:
		if evt.Type != domain.ReunionEventType("ectotherm_rescue_updated") {
			t.Errorf("expected ectotherm_rescue_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for ectotherm_rescue_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "ectotherm_rescue_updated" {
			t.Errorf("expected ectotherm_rescue_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for ectotherm_rescue_updated mesh event")
	}

	// 5. POST /api/v1/ectotherm-rescue/operations/RE-OP-101/step updates protocol steps
	for _, stepID := range []string{"step-ceramic-ir-basking-lure", "step-humidity-hydration-soak", "step-gradual-coelomic-rewarm"} {
		stepPayload, _ := json.Marshal(map[string]any{
			"stepId":    stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/ectotherm-rescue/operations/RE-OP-101/step", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/ectotherm-rescue/operations/RE-OP-101/clearance issues certificate
	clearancePayload, _ := json.Marshal(map[string]any{
		"leadHerpTechCallsign": "HERP-TECH-01 (Dr. Alvarez)",
		"notes":                "Extracted safely from foundation burrow and rewarmed across POTZ gradient",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/ectotherm-rescue/operations/RE-OP-101/clearance", bytes.NewReader(clearancePayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.EctothermRescueOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.EctothermRescueStatusRecoveredThermalGradientSafe || cleared.HazardScore != 10 || !strings.HasPrefix(cleared.ClearanceCertificateHash, "ECTOTHERM-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on step and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/ectotherm-rescue/operations/UNKNOWN-ID/step", bytes.NewReader([]byte(`{"stepId":"step-endoscope-crevice-sweep","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on step, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/ectotherm-rescue/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestReptileEctothermHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/ectotherm-rescue/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/ectotherm-rescue/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/ectotherm-rescue/stream, got %d", resp.StatusCode)
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
