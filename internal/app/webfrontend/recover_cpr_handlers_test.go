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
	"github.com/scottdensmore/petspotr/pkg/recovercpr"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestRecoverCPRHandlers_SeededCRUDPlanCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	cprSSECh, unsubSSE := server.ReunionHub().Subscribe("recover_cpr")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("recover_cpr", "recover-cpr-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /recover-cpr-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/recover-cpr-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /recover-cpr-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Cardiopulmonary Resuscitation (RECOVER Guidelines)",
		"kpi-total-ops",
		"kpi-critical-arrest",
		"kpi-staged-cycle",
		"kpi-cleared-rosc",
		"kpi-pitfall-alerts",
		"cpr-alert-banner",
		"cpr-clearance-banner",
		"CPR-OP-101",
		"CPR-OP-102",
		"CPR-OP-103",
		"Duke",
		"Gizmo",
		"Pixel",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /recover-cpr-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/recover-cpr/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/recover-cpr/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/recover-cpr/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.RecoverCPROperation `json:"operations"`
		Metrics    recovercpr.RecoverCPRMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal recover cpr operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/recover-cpr/operations?status=CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL&species=CANINE_KEEL_CHEST_THORACIC_PUMP", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.RecoverCPROperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "CPR-OP-101" {
		t.Errorf("expected 1 filtered operation CPR-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/recover-cpr/operations/CPR-OP-101 returns Duke
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/recover-cpr/operations/CPR-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/recover-cpr/operations/CPR-OP-101, got %d", recGet.Code)
	}
	var op101 domain.RecoverCPROperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal CPR-OP-101: %v", err)
	}
	if op101.PetName != "Duke" || op101.Status != domain.CPRStatusArrestInProgressCritical {
		t.Errorf("unexpected CPR-OP-101 state: %+v", op101)
	}

	// 4. POST /api/recover-cpr/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/recover-cpr/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/recover-cpr/operations creates a new operation "Buster" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-CPR-9944",
		"pet_name":    "Buster",
		"patient_profile": map[string]any{
			"species":                          "CANINE_ROUND_CHEST_CARDIAC_PUMP",
			"breed":                            "Golden Retriever",
			"weight_kg":                        28.0,
			"arrest_rhythm":                    "VFIB_PULSELESS_VT_SHOCKABLE",
			"end_tidal_co2_mmhg":               12.5,
			"compression_cycles_completed":     1,
			"reversible_opioid_alpha2_present": true,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/recover-cpr/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/recover-cpr/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.RecoverCPROperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "CPR-OP-104" || created.PetName != "Buster" || created.Status != domain.CPRStatusArrestInProgressCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-cprSSECh:
		if evt.Type != domain.ReunionEventType("recover_cpr_updated") {
			t.Errorf("expected recover_cpr_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for recover_cpr_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "recover_cpr_updated" {
			t.Errorf("expected recover_cpr_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for recover_cpr_updated mesh event")
	}

	// 6. POST /api/recover-cpr/operations/CPR-OP-101/stage-plan transitions to RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                      "CANINE_KEEL_CHEST_THORACIC_PUMP",
		"weight_kg":                    30.0,
		"arrest_rhythm":                "VFIB_PULSELESS_VT_SHOCKABLE",
		"end_tidal_co2_mmhg":           16.5,
		"compression_cycles_completed": 2,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/recover-cpr/operations/CPR-OP-101/stage-plan", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging plan on CPR-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.RecoverCPROperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.CPRStatusEpiDefibCycleStaged || staged.ALSProtocolPlan.BiphasicDefibJoules != 120.0 {
		t.Errorf("expected status %s and 120.0 J defib, got %+v", domain.CPRStatusEpiDefibCycleStaged, staged)
	}

	// 7. POST /api/recover-cpr/operations/CPR-OP-101/cycle-check records 2-min cycle check
	checkPayload, _ := json.Marshal(map[string]any{
		"cycle_number":           2,
		"end_tidal_co2_mmhg":     22.5,
		"rhythm_at_2_min_pause":  "VFIB_PULSELESS_VT_SHOCKABLE",
		"compressor_rotated":     true,
		"pause_duration_seconds": 6,
		"clinician_callsign":     "DR-RECOVER-01",
		"notes":                  "Cycle 2 completed with compressor rotation; EtCO2 improved to 22.5 mmHg.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/recover-cpr/operations/CPR-OP-101/cycle-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording cycle check on CPR-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.RecoverCPROperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.CycleChecks) != 2 {
		t.Errorf("expected 2 cycle checks on CPR-OP-101, got %d", len(checked.CycleChecks))
	}

	// 8. POST /api/recover-cpr/operations/CPR-OP-101/rosc-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"sustained_rosc_minutes":                            30,
		"final_etco2_mmhg":                                  39.0,
		"final_map_mmhg":                                    92,
		"spo2_titrated_94_to_98_no_hyperoxia":               true,
		"permissive_hypothermia_or_normothermia_maintained": true,
		"vet_license":                                       "VET-RECOVER-99410",
		"summary":                                           "Sustained ROSC 30 min; normocapnic EtCO2 39.0 mmHg, MAP 92 mmHg, SpO2 94-98%.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/recover-cpr/operations/CPR-OP-101/rosc-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing ROSC clearance on CPR-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.RecoverCPROperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.CPRStatusROSCPostArrestCleared || cleared.HyperventilationHighDoseEpiAlert || cleared.ROSCClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/recover-cpr/operations/UNKNOWN",
		"/api/recover-cpr/operations/UNKNOWN/stage-plan",
		"/api/recover-cpr/operations/UNKNOWN/cycle-check",
		"/api/recover-cpr/operations/UNKNOWN/rosc-clearance",
	} {
		method := http.MethodPost
		if strings.HasSuffix(path, "/UNKNOWN") {
			method = http.MethodGet
		}
		rec404 := httptest.NewRecorder()
		req404 := httptest.NewRequest(method, path, bytes.NewReader([]byte(`{}`)))
		server.ServeHTTP(rec404, req404)
		if rec404.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for %s %s, got %d", method, path, rec404.Code)
		}
	}
}

func TestRecoverCPRHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/recover-cpr/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/recover-cpr/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/recover-cpr/stream, got %d", resp.StatusCode)
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
