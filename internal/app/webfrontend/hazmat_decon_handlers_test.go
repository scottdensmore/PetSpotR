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

func TestHazmatDeconHandlers_SeededEvaluateStepClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	hzSSECh, unsubSSE := server.ReunionHub().Subscribe("hazmat_decon")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("hazmat_decon", "hazmat-decon-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /hazmat-decon should render HTML with seeded operations and summary stats
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/hazmat-decon", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /hazmat-decon, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Wildfire Smoke, Ashfall, Hazmat Plume",
		"stat-total-hazmat-ops",
		"stat-critical-smoke-alerts",
		"stat-staged-decon-washes",
		"stat-cleared-safe-count",
		"wildfire-ash-plume",
		"petrochemical-coat-spill",
		"cyanotoxin-pond-bloom",
		"HZ-OP-101",
		"HZ-OP-102",
		"HZ-OP-103",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /hazmat-decon HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/hazmat-decon/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/hazmat-decon/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/hazmat-decon/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.HazmatDeconOperation `json:"operations"`
		Summary    map[string]int                `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal hazmat decon operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.HazmatDeconOperation)
	for _, op := range listResp.Operations {
		byID[op.OperationID] = op
	}

	op101, ok := byID["HZ-OP-101"]
	if !ok || op101.PetName != "Cinder" || op101.Status != domain.HazmatStatusRespiratoryDermalCritical || !op101.SmokeInhalationAlert || !op101.DermalToxinIngestionAlert {
		t.Errorf("unexpected HZ-OP-101 state: %+v", op101)
	}
	op102, ok := byID["HZ-OP-102"]
	if !ok || op102.PetName != "Harbor" || op102.Status != domain.HazmatStatusDeconWashNebulizerStaged || op102.DeconReadinessScore != 100 {
		t.Errorf("unexpected HZ-OP-102 state: %+v", op102)
	}
	op103, ok := byID["HZ-OP-103"]
	if !ok || op103.PetName != "Willow" || op103.Status != domain.HazmatStatusDecontaminatedClearedSafe || op103.ClearanceCertificateHash != "HAZMAT-SAFE-9F4B21E8C305" {
		t.Errorf("unexpected HZ-OP-103 state: %+v", op103)
	}

	// 2b. Filter by status and exposureType
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/hazmat-decon/operations?status=RESPIRATORY_DERMAL_CRITICAL&exposureType=WILDFIRE_SMOKE_ASHFALL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.HazmatDeconOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].OperationID != "HZ-OP-101" {
		t.Errorf("expected 1 filtered operation HZ-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/hazmat-decon/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/hazmat-decon/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/hazmat-decon/evaluate creates a new hazmat decon operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"petId":                "PET-HZ-204",
		"petName":              "Ash",
		"species":              "Feline",
		"weightKg":             4.1,
		"exposureType":         "WILDFIRE_SMOKE_ASHFALL",
		"aqiIndex":             450,
		"pm25UgM3":             410.0,
		"carboxyhemoglobinPct": 17.8,
		"coatContaminationPct": 72.0,
		"respiratoryRateBpm":   72,
		"deconCorridorZone":    "DECON-CORRIDOR-HOT-01",
		"leadToxTechCallsign":  "TOX-VET-01",
		"notes":                "Wildfire ashfall plume simulation",
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/hazmat-decon/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/hazmat-decon/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.HazmatDeconOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.OperationID != "HZ-OP-104" || !created.SmokeInhalationAlert || !created.DermalToxinIngestionAlert || created.Status != domain.HazmatStatusRespiratoryDermalCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-hzSSECh:
		if evt.Type != domain.ReunionEventType("hazmat_decon_updated") {
			t.Errorf("expected hazmat_decon_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for hazmat_decon_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "hazmat_decon_updated" {
			t.Errorf("expected hazmat_decon_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for hazmat_decon_updated mesh event")
	}

	// 5. POST /api/v1/hazmat-decon/operations/HZ-OP-101/step updates protocol steps
	for _, stepID := range []string{"step-warm-surfactant-wash", "step-oxygen-nebulizer", "step-cohb-binder-panel"} {
		stepPayload, _ := json.Marshal(map[string]any{
			"stepId":    stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/hazmat-decon/operations/HZ-OP-101/step", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/hazmat-decon/operations/HZ-OP-101/clearance issues certificate
	clearancePayload, _ := json.Marshal(map[string]any{
		"leadToxTechCallsign": "TOX-VET-01 (Dr. Aris Thorne)",
		"notes":               "Decontaminated and COHb cleared",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/hazmat-decon/operations/HZ-OP-101/clearance", bytes.NewReader(clearancePayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.HazmatDeconOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.HazmatStatusDecontaminatedClearedSafe || cleared.HazardScore != 10 || !strings.HasPrefix(cleared.ClearanceCertificateHash, "HAZMAT-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on step and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/hazmat-decon/operations/UNKNOWN-ID/step", bytes.NewReader([]byte(`{"stepId":"step-ocular-flush","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on step, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/hazmat-decon/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestHazmatDeconHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/hazmat-decon/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/hazmat-decon/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/hazmat-decon/stream, got %d", resp.StatusCode)
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
