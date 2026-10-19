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
	"github.com/scottdensmore/petspotr/pkg/addisoniancrisis"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestAddisonianCrisisHandlers_SeededCRUDRegimenCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	addisonianSSECh, unsubSSE := server.ReunionHub().Subscribe("addisonian_crisis")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("addisonian_crisis", "addisonian-crisis-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /addisonian-crisis-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/addisonian-crisis-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /addisonian-crisis-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Hypoadrenocorticism (Addisonian Crisis)",
		"kpi-total-ops",
		"kpi-critical-shock",
		"kpi-staged-dex-sp",
		"kpi-cleared-docp",
		"kpi-myelinolysis-alerts",
		"addisonian-alert-banner",
		"addisonian-clearance-banner",
		"AC-OP-101",
		"AC-OP-102",
		"AC-OP-103",
		"Curly",
		"Pippa",
		"Archie",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /addisonian-crisis-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/addisonian-crisis/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/addisonian-crisis/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/addisonian-crisis/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.AddisonianCrisisOperation `json:"operations"`
		Metrics    addisoniancrisis.AddisonianMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal addisonian crisis operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/addisonian-crisis/operations?status=ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL&species=CANINE_TYPICAL_ADDISONS", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.AddisonianCrisisOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "AC-OP-101" {
		t.Errorf("expected 1 filtered operation AC-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/addisonian-crisis/operations/AC-OP-101 returns Curly
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/addisonian-crisis/operations/AC-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/addisonian-crisis/operations/AC-OP-101, got %d", recGet.Code)
	}
	var op101 domain.AddisonianCrisisOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal AC-OP-101: %v", err)
	}
	if op101.PetName != "Curly" || op101.Status != domain.AddisonianStatusShockHyperkalemicCritical {
		t.Errorf("unexpected AC-OP-101 state: %+v", op101)
	}

	// 4. POST /api/addisonian-crisis/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/addisonian-crisis/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/addisonian-crisis/operations creates a new operation "Louie" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-AC-9604",
		"pet_name":    "Louie",
		"patient_profile": map[string]any{
			"species":                      "CANINE_TYPICAL_ADDISONS",
			"breed":                        "Bearded Collie",
			"weight_kg":                    22.0,
			"serum_sodium_meq_l":           123.0,
			"serum_potassium_meq_l":        7.5,
			"blood_glucose_mg_dl":          56.0,
			"heart_rate_bpm":               54,
			"systolic_blood_pressure_mmhg": 70,
			"resting_cortisol_mcg_dl":      0.2,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/addisonian-crisis/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/addisonian-crisis/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.AddisonianCrisisOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "AC-OP-104" || created.PetName != "Louie" || created.Status != domain.AddisonianStatusShockHyperkalemicCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-addisonianSSECh:
		if evt.Type != domain.ReunionEventType("addisonian_crisis_updated") {
			t.Errorf("expected addisonian_crisis_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for addisonian_crisis_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "addisonian_crisis_updated" {
			t.Errorf("expected addisonian_crisis_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for addisonian_crisis_updated mesh event")
	}

	// 6. POST /api/addisonian-crisis/operations/AC-OP-101/stage-regimen transitions to DEX_SP_NACL_ACTH_STIM_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                      "CANINE_TYPICAL_ADDISONS",
		"weight_kg":                    23.5,
		"serum_sodium_meq_l":           124.0,
		"serum_potassium_meq_l":        7.8,
		"heart_rate_bpm":               52,
		"systolic_blood_pressure_mmhg": 68,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/addisonian-crisis/operations/AC-OP-101/stage-regimen", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging regimen on AC-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.AddisonianCrisisOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.AddisonianStatusDexSPNaClACTHStaged {
		t.Errorf("expected status %s, got %s", domain.AddisonianStatusDexSPNaClACTHStaged, staged.Status)
	}

	// 7. POST /api/addisonian-crisis/operations/AC-OP-101/electrolyte-check records electrolyte check
	checkPayload, _ := json.Marshal(map[string]any{
		"serum_sodium_meq_l":        131.0,
		"serum_potassium_meq_l":     5.2,
		"sodium_rise_24hr_meq_l":    7.0,
		"post_acth_cortisol_mcg_dl": 0.3,
		"heart_rate_bpm":            88,
		"clinician_callsign":        "DR-ENDO-19",
		"notes":                     "Post-ACTH cortisol 0.3 mcg/dL; Na+ rose 7.0 mEq/L/24h; sinus rhythm restored.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/addisonian-crisis/operations/AC-OP-101/electrolyte-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording electrolyte check on AC-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.AddisonianCrisisOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.ElectrolyteChecks) != 2 {
		t.Errorf("expected 2 electrolyte checks on AC-OP-101, got %d", len(checked.ElectrolyteChecks))
	}

	// 8. POST /api/addisonian-crisis/operations/AC-OP-101/discharge-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_sodium_potassium_ratio":        31.5,
		"final_serum_potassium_meq_l":         4.4,
		"post_acth_cortisol_confirmed_low":    true,
		"docp_22_mg_kg_subq_administered":     true,
		"oral_prednisone_physiologic_started": true,
		"vet_license":                         "VET-ENDO-96108",
		"summary":                             "Post-ACTH cortisol confirmed < 2.0 mcg/dL; eukalemic; DOCP 2.2 mg/kg SubQ + oral Prednisone started.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/addisonian-crisis/operations/AC-OP-101/discharge-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing discharge clearance on AC-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.AddisonianCrisisOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.AddisonianStatusDOCPEukalemicCleared || cleared.OsmoticMyelinolysisAssayAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/addisonian-crisis/operations/UNKNOWN",
		"/api/addisonian-crisis/operations/UNKNOWN/stage-regimen",
		"/api/addisonian-crisis/operations/UNKNOWN/electrolyte-check",
		"/api/addisonian-crisis/operations/UNKNOWN/discharge-clearance",
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

func TestAddisonianCrisisHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/addisonian-crisis/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/addisonian-crisis/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/addisonian-crisis/stream, got %d", resp.StatusCode)
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
