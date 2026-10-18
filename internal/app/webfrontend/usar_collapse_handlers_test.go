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

func TestUSARCollapseHandlers_SeededEvaluateStepClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	usarSSECh, unsubSSE := server.ReunionHub().Subscribe("usar_collapse")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("usar_collapse", "usar-collapse-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /usar-structural-collapse should render HTML with seeded operations and summary stats
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/usar-structural-collapse", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /usar-structural-collapse, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Seismic Rubble, Urban Search",
		"stat-total-usar-ops",
		"stat-critical-collapse-alerts",
		"stat-staged-shoring-rigs",
		"stat-extracted-usar-count",
		"pancake-slab-aftershock",
		"leanto-truss-void",
		"vshape-masonry-pocket",
		"USAR-OP-101",
		"USAR-OP-102",
		"USAR-OP-103",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /usar-structural-collapse HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/usar-collapse/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/usar-collapse/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/usar-collapse/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.USARRescueOperation `json:"operations"`
		Summary    map[string]int               `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal USAR collapse operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.USARRescueOperation)
	for _, op := range listResp.Operations {
		byID[op.OperationID] = op
	}

	op101, ok := byID["USAR-OP-101"]
	if !ok || op101.PetName != "Diesel" || op101.Status != domain.USARRescueStatusSecondaryCollapseCrushCritical || !op101.SecondaryCollapseAlert || !op101.CrushRhabdoAlert {
		t.Errorf("unexpected USAR-OP-101 state: %+v", op101)
	}
	op102, ok := byID["USAR-OP-102"]
	if !ok || op102.PetName != "Milo" || op102.Status != domain.USARRescueStatusShoringCribbingDelsarStaged || op102.ShoringReadinessScore != 100 {
		t.Errorf("unexpected USAR-OP-102 state: %+v", op102)
	}
	op103, ok := byID["USAR-OP-103"]
	if !ok || op103.PetName != "Rosie" || op103.Status != domain.USARRescueStatusExtractedUSARDeconSafe || op103.ClearanceCertificateHash != "USAR-SAFE-5C9D41B8E702" {
		t.Errorf("unexpected USAR-OP-103 state: %+v", op103)
	}

	// 2b. Filter by status and voidType
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/usar-collapse/operations?status=SECONDARY_COLLAPSE_CRUSH_CRITICAL&voidType=PANCAKE_FLOOR_SLAB_VOID", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.USARRescueOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].OperationID != "USAR-OP-101" {
		t.Errorf("expected 1 filtered operation USAR-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/usar-collapse/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/usar-collapse/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/usar-collapse/evaluate creates a new USAR collapse operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"petId":                  "PET-USAR-204",
		"petName":                "Brutus",
		"species":                "Canine",
		"weightKg":               32.5,
		"voidType":               "PANCAKE_FLOOR_SLAB_VOID",
		"rubbleOverburdenMeters": 2.9,
		"aftershockPGA":          0.36,
		"silicaDustMgM3":         8.6,
		"hoursEntrapped":         19.5,
		"structuralStabilityIdx": 20,
		"collapseStructureGrid":  "SECTOR-BRAVO-04",
		"leadUsarSpecCallsign":   "USAR-TF1-SPEC-01",
		"notes":                  "Pancake slab collapse simulation",
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/usar-collapse/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/usar-collapse/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.USARRescueOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.OperationID != "USAR-OP-104" || !created.SecondaryCollapseAlert || !created.CrushRhabdoAlert || created.Status != domain.USARRescueStatusSecondaryCollapseCrushCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-usarSSECh:
		if evt.Type != domain.ReunionEventType("usar_collapse_updated") {
			t.Errorf("expected usar_collapse_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for usar_collapse_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "usar_collapse_updated" {
			t.Errorf("expected usar_collapse_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for usar_collapse_updated mesh event")
	}

	// 5. POST /api/v1/usar-collapse/operations/USAR-OP-101/step updates protocol steps
	for _, stepID := range []string{"step-box-cribbing-shoring", "step-rotary-core-breach", "step-crush-rhabdo-decon"} {
		stepPayload, _ := json.Marshal(map[string]any{
			"stepId":    stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/usar-collapse/operations/USAR-OP-101/step", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/usar-collapse/operations/USAR-OP-101/clearance issues certificate
	clearancePayload, _ := json.Marshal(map[string]any{
		"leadUsarSpecCallsign": "USAR-TF1-SPEC-01 (Castellano)",
		"notes":                "Extracted safely from B2 pancake slab void and silica decontaminated",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/usar-collapse/operations/USAR-OP-101/clearance", bytes.NewReader(clearancePayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.USARRescueOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.USARRescueStatusExtractedUSARDeconSafe || cleared.HazardScore != 10 || !strings.HasPrefix(cleared.ClearanceCertificateHash, "USAR-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on step and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/usar-collapse/operations/UNKNOWN-ID/step", bytes.NewReader([]byte(`{"stepId":"step-delsar-triangulation","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on step, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/usar-collapse/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestUSARCollapseHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/usar-collapse/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/usar-collapse/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/usar-collapse/stream, got %d", resp.StatusCode)
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
