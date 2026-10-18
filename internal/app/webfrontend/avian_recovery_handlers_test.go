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

func TestAvianRecoveryHandlers_SeededEvaluateStepClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	avSSECh, unsubSSE := server.ReunionHub().Subscribe("avian_recovery")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("avian_recovery", "avian-recovery-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /avian-recovery should render HTML with seeded operations and summary stats
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/avian-recovery", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /avian-recovery, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Avian, Exotic Parrot &amp; Falconry",
		"stat-total-avian-ops",
		"stat-critical-thermal-alerts",
		"stat-staged-acoustic-lures",
		"stat-recovered-aviary-count",
		"macaw-thermal-ridge",
		"falcon-vhf-beacon",
		"cockatiel-courtyard-perch",
		"AV-OP-101",
		"AV-OP-102",
		"AV-OP-103",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /avian-recovery HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/avian-recovery/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/avian-recovery/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/avian-recovery/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.AvianRecoveryOperation `json:"operations"`
		Summary    map[string]int                  `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal avian recovery operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.AvianRecoveryOperation)
	for _, op := range listResp.Operations {
		byID[op.OperationID] = op
	}

	op101, ok := byID["AV-OP-101"]
	if !ok || op101.PetName != "Rio" || op101.Status != domain.AvianStatusThermalDriftPredationCritical || !op101.ThermalDriftAlert || !op101.CropStarvationColdAlert {
		t.Errorf("unexpected AV-OP-101 state: %+v", op101)
	}
	op102, ok := byID["AV-OP-102"]
	if !ok || op102.PetName != "Zephyr" || op102.Status != domain.AvianStatusAcousticLurePerchStaged || op102.LureReadinessScore != 100 {
		t.Errorf("unexpected AV-OP-102 state: %+v", op102)
	}
	op103, ok := byID["AV-OP-103"]
	if !ok || op103.PetName != "Sunny" || op103.Status != domain.AvianStatusRecoveredAviarySafe || op103.ClearanceCertificateHash != "AVIAN-SAFE-4D8E12A9F603" {
		t.Errorf("unexpected AV-OP-103 state: %+v", op103)
	}

	// 2b. Filter by status and avianClass
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/avian-recovery/operations?status=THERMAL_DRIFT_PREDATION_CRITICAL&avianClass=PSITTACINE_PARROT_MACAW", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.AvianRecoveryOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].OperationID != "AV-OP-101" {
		t.Errorf("expected 1 filtered operation AV-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/avian-recovery/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/avian-recovery/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/avian-recovery/evaluate creates a new avian recovery operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"petId":                 "PET-AV-204",
		"petName":               "Kona",
		"species":               "Avian",
		"weightGrams":           980.0,
		"avianClass":            "PSITTACINE_PARROT_MACAW",
		"perchAltitudeMeters":   26.0,
		"thermalUpdraftKnots":   22.5,
		"ambientTempCelsius":    7.5,
		"hoursAloft":            24.0,
		"telemetrySignalDbm":    -78,
		"flightCorridorBearing": "CORRIDOR-SSE-162°",
		"leadAvianTechCallsign": "AVIAN-TECH-01",
		"notes":                 "Macaw thermal ridge simulation",
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/avian-recovery/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/avian-recovery/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.AvianRecoveryOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.OperationID != "AV-OP-104" || !created.ThermalDriftAlert || !created.CropStarvationColdAlert || created.Status != domain.AvianStatusThermalDriftPredationCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-avSSECh:
		if evt.Type != domain.ReunionEventType("avian_recovery_updated") {
			t.Errorf("expected avian_recovery_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for avian_recovery_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "avian_recovery_updated" {
			t.Errorf("expected avian_recovery_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for avian_recovery_updated mesh event")
	}

	// 5. POST /api/v1/avian-recovery/operations/AV-OP-101/step updates protocol steps
	for _, stepID := range []string{"step-flock-acoustic-lure", "step-forage-perch-rig", "step-kevlar-towel-transfer"} {
		stepPayload, _ := json.Marshal(map[string]any{
			"stepId":    stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/avian-recovery/operations/AV-OP-101/step", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/avian-recovery/operations/AV-OP-101/clearance issues certificate
	clearancePayload, _ := json.Marshal(map[string]any{
		"leadAvianTechCallsign": "AVIAN-TECH-01 (Ortega)",
		"notes":                 "Recovered safely into heated aviary carrier",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/avian-recovery/operations/AV-OP-101/clearance", bytes.NewReader(clearancePayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.AvianRecoveryOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.AvianStatusRecoveredAviarySafe || cleared.HazardScore != 10 || !strings.HasPrefix(cleared.ClearanceCertificateHash, "AVIAN-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on step and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/avian-recovery/operations/UNKNOWN-ID/step", bytes.NewReader([]byte(`{"stepId":"step-vhf-triangulate","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on step, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/avian-recovery/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestAvianRecoveryHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/avian-recovery/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/avian-recovery/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/avian-recovery/stream, got %d", resp.StatusCode)
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
