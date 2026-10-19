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
	"github.com/scottdensmore/petspotr/pkg/gdvbloat"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestGDVBloatHandlers_SeededEvaluateStepsClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	gdvSSECh, unsubSSE := server.ReunionHub().Subscribe("gdv_bloat")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("gdv_bloat", "gdv-bloat-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /gdv-bloat-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/gdv-bloat-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /gdv-bloat-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Gastric Dilatation-Volvulus (GDV / Bloat)",
		"kpi-total-ops",
		"kpi-critical-gdv",
		"kpi-staged-trocar",
		"kpi-cleared-safe",
		"filter-presentation-type",
		"filter-status",
		"gdv-operations-list",
		"active-operation-workspace",
		"gdv-eval-form",
		"GDV-OP-101",
		"GDV-OP-102",
		"GDV-OP-103",
		"Titan",
		"Baron",
		"Duchess",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /gdv-bloat-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/v1/gdv-bloat/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/gdv-bloat/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/gdv-bloat/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.GDVBloatOperation `json:"operations"`
		Summary    map[string]int             `json:"summary"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal GDV bloat operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	byID := make(map[string]domain.GDVBloatOperation)
	for _, op := range listResp.Operations {
		byID[op.ID] = op
	}

	op101, ok := byID["GDV-OP-101"]
	if !ok || op101.PetName != "Titan" || op101.Status != domain.GDVStatusCriticalNecrosisVPCShock || !op101.VentricularArrhythmiaVPCs {
		t.Errorf("unexpected GDV-OP-101 state: %+v", op101)
	}
	op102, ok := byID["GDV-OP-102"]
	if !ok || op102.PetName != "Baron" || op102.Status != domain.GDVStatusStagedTrocarDualIV {
		t.Errorf("unexpected GDV-OP-102 state: %+v", op102)
	}
	op103, ok := byID["GDV-OP-103"]
	if !ok || op103.PetName != "Duchess" || op103.Status != domain.GDVStatusClearedGastropexySinus || !strings.HasPrefix(op103.ClearanceToken, "GDV-SAFE-") {
		t.Errorf("unexpected GDV-OP-103 state: %+v", op103)
	}

	// 2b. Filter by ?presentation_type= and ?status=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/gdv-bloat/operations?presentation_type=ACUTE_GDV_360_PYLORIC_TORSION&status=GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.GDVBloatOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "GDV-OP-101" {
		t.Errorf("expected 1 filtered operation GDV-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/v1/gdv-bloat/evaluate with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"pet_name": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/gdv-bloat/evaluate", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid evaluate payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/gdv-bloat/evaluate creates a new GDV bloat operation (201 Created)
	evalPayload, _ := json.Marshal(map[string]any{
		"pet_id":                          "PET-GDV-204",
		"pet_name":                        "Goliath",
		"species":                         "Dog - German Shepherd",
		"weight_kg":                       42.0,
		"presentation_type":               "ACUTE_GDV_360_PYLORIC_TORSION",
		"plasma_lactate_mmol_l":           8.6,
		"heart_rate_bpm":                  204,
		"tympanic_abdominal_girth_cm":     116.0,
		"non_productive_retching_present": true,
		"cranial_cephalic_dual_iv_access": true,
		"ventricular_arrhythmia_vpcs":     true,
	})
	recEval := httptest.NewRecorder()
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/gdv-bloat/evaluate", bytes.NewReader(evalPayload))
	reqEval.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recEval, reqEval)
	if recEval.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/gdv-bloat/evaluate, got %d: %s", recEval.Code, recEval.Body.String())
	}

	var created domain.GDVBloatOperation
	if err := json.Unmarshal(recEval.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "GDV-OP-104" || created.Status != domain.GDVStatusCriticalNecrosisVPCShock {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-gdvSSECh:
		if evt.Type != domain.ReunionEventType("gdv_bloat_updated") {
			t.Errorf("expected gdv_bloat_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for gdv_bloat_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "gdv_bloat_updated" {
			t.Errorf("expected gdv_bloat_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for gdv_bloat_updated mesh event")
	}

	// 5. POST /api/v1/gdv-bloat/operations/GDV-OP-101/steps updates all 4 protocol steps
	for _, stepID := range []string{
		gdvbloat.StepBilateralCephalicIVBolus,
		gdvbloat.StepTrocarOrogastricDecompress,
		gdvbloat.StepRightLateralDoubleBubble,
		gdvbloat.StepLidocaineVPCGastropexyPrep,
	} {
		stepPayload, _ := json.Marshal(map[string]any{
			"step_id":   stepID,
			"completed": true,
		})
		recStep := httptest.NewRecorder()
		reqStep := httptest.NewRequest(http.MethodPost, "/api/v1/gdv-bloat/operations/GDV-OP-101/steps", bytes.NewReader(stepPayload))
		reqStep.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(recStep, reqStep)
		if recStep.Code != http.StatusOK {
			t.Fatalf("expected 200 OK updating step %s, got %d: %s", stepID, recStep.Code, recStep.Body.String())
		}
	}

	// 6. POST /api/v1/gdv-bloat/operations/GDV-OP-101/clearance issues GDV-SAFE-<12HEX> token
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/v1/gdv-bloat/operations/GDV-OP-101/clearance", bytes.NewReader([]byte(`{}`)))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.GDVBloatOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.GDVStatusClearedGastropexySinus || !strings.HasPrefix(cleared.ClearanceToken, "GDV-SAFE-") {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 7. 404 Not Found for unknown operation ID on steps and clearance
	rec404Step := httptest.NewRecorder()
	req404Step := httptest.NewRequest(http.MethodPost, "/api/v1/gdv-bloat/operations/UNKNOWN-ID/steps", bytes.NewReader([]byte(`{"step_id":"STEP-BILATERAL-CEPHALIC-IV-BOLUS","completed":true}`)))
	server.ServeHTTP(rec404Step, req404Step)
	if rec404Step.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on steps, got %d", rec404Step.Code)
	}

	rec404Clear := httptest.NewRecorder()
	req404Clear := httptest.NewRequest(http.MethodPost, "/api/v1/gdv-bloat/operations/UNKNOWN-ID/clearance", bytes.NewReader([]byte(`{}`)))
	server.ServeHTTP(rec404Clear, req404Clear)
	if rec404Clear.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown op on clearance, got %d", rec404Clear.Code)
	}
}

func TestGDVBloatHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/gdv-bloat/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/gdv-bloat/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/gdv-bloat/stream, got %d", resp.StatusCode)
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
