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

func TestTetanusBotulismHandlers_SeededCRUDStageNeuroCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	clbSSECh, unsubSSE := server.ReunionHub().Subscribe("tetanus_botulism")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("tetanus_botulism", "tetanus-botulism-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /tetanus-botulism-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/tetanus-botulism-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /tetanus-botulism-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Tetanus (Clostridium tetani) &amp; Botulism (Clostridium botulinum)",
		"tetanus-botulism-cockpit",
		"clb-alert-banner",
		"clb-clearance-banner",
		"clb-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"clb-operations-grid",
		"clb-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-burkitt-class",
		"active-spasm-frequency",
		"create-clb-form",
		"stage-therapy-form",
		"record-neuro-check-form",
		"clear-clb-form",
		"CLB-OP-101",
		"CLB-OP-102",
		"CLB-OP-103",
		"Rex",
		"Otis",
		"Bella",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /tetanus-botulism-triage HTML to contain %q", snippet)
		}
	}

	// 1b. GET /api/tetanus-botulism/overview returns overview metrics
	recOverview := httptest.NewRecorder()
	reqOverview := httptest.NewRequest(http.MethodGet, "/api/tetanus-botulism/overview", nil)
	server.ServeHTTP(recOverview, reqOverview)
	if recOverview.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/tetanus-botulism/overview, got %d", recOverview.Code)
	}
	var overview domain.TetanusBotulismOverview
	if err := json.Unmarshal(recOverview.Body.Bytes(), &overview); err != nil {
		t.Fatalf("failed to unmarshal overview: %v", err)
	}
	if overview.TotalOperations != 3 || overview.MetronidazoleTATStagedCount != 1 || overview.SensoryIsolationMonitoredCount != 1 || overview.NormoreflexicClearedCount != 1 {
		t.Errorf("unexpected overview counts: %+v", overview)
	}

	// 2. GET /api/tetanus-botulism/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/tetanus-botulism/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/tetanus-botulism/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.TetanusBotulismOperation `json:"operations"`
		Overview   domain.TetanusBotulismOverview    `json:"overview"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal Tetanus Botulism operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Overview.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (overview=%+v)", len(listResp.Operations), listResp.Overview)
	}

	// 2b. Filter by ?status= and ?syndrome=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/tetanus-botulism/operations?status=METRONIDAZOLE_TAT_METHOCARBAMOL_STAGED&syndrome=TETANUS_CLASS_III_RECUMBENT_OPISTHOTONOS", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.TetanusBotulismOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "CLB-OP-101" {
		t.Errorf("expected 1 filtered operation CLB-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/tetanus-botulism/operations/CLB-OP-101 returns Rex
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/tetanus-botulism/operations/CLB-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/tetanus-botulism/operations/CLB-OP-101, got %d", recGet.Code)
	}
	var op101 domain.TetanusBotulismOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal CLB-OP-101: %v", err)
	}
	if op101.PetName != "Rex" || op101.Status != domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged || op101.BurkittSeverityClass != 3 {
		t.Errorf("unexpected CLB-OP-101 state: %+v", op101)
	}

	// 4. POST /api/tetanus-botulism/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/tetanus-botulism/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/tetanus-botulism/operations creates a new operation "Apollo" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                 "CASE-CLB-104",
		"pet_name":                "Apollo",
		"species":                 "Canine",
		"breed":                   "Rottweiler",
		"weight_kg":               34.0,
		"syndrome":                "TETANUS_CLASS_II_GENERALIZED_SAWHORSE",
		"protocol":                "EQUINE_TAT_100_1000_IU_KG_AFTER_ID_TEST",
		"burkitt_severity_class":  2,
		"gag_reflex_intact":       false,
		"spasm_frequency_per_hr":  4,
		"core_temp_celsius":       39.6,
		"paco2_mmhg":              46.0,
		"heart_rate_bpm":          136,
		"metronidazole_mg_kg_bid": 10.0,
		"equine_tat_units_kg":     400.0,
		"methocarbamol_mg_kg_iv":  50.0,
		"intradermal_skin_test_negative_verified":    true,
		"penicillin_gaba_warning_acknowledged":       true,
		"dark_quiet_cotton_earplugs_active":          true,
		"aminoglycoside_contraindicated_in_botulism": true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/tetanus-botulism/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/tetanus-botulism/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.TetanusBotulismOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "CLB-OP-104" || created.PetName != "Apollo" || created.BurkittSeverityClass != 2 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-clbSSECh:
		if evt.Type != domain.ReunionEventType("tetanus_botulism_updated") {
			t.Errorf("expected tetanus_botulism_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for tetanus_botulism_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "tetanus_botulism_updated" {
			t.Errorf("expected tetanus_botulism_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for tetanus_botulism_updated mesh event")
	}

	// 6. POST /api/tetanus-botulism/operations/CLB-OP-101/therapy stages therapy and verifies safety guards
	stagePayload, _ := json.Marshal(map[string]any{
		"protocol":                                     "EQUINE_TAT_100_1000_IU_KG_AFTER_ID_TEST",
		"metronidazole_mg_kg_bid":                      10.0,
		"equine_tat_units_kg":                          500.0,
		"methocarbamol_mg_kg_iv":                       55.0,
		"intradermal_skin_test_negative_verified":      true,
		"penicillin_gaba_warning_acknowledged":         true,
		"dark_quiet_cotton_earplugs_active":            true,
		"aminoglycoside_contraindicated_in_botulism":   true,
		"wound_infiltration_prior_to_debridement_note": "0.2 mL Equine TAT intradermal skin test negative at 30 min; local perilesional infiltration prior to wound debridement.",
		"staged_by": "Dr. Ingrid Vance, DACVIM (Neurology)",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/tetanus-botulism/operations/CLB-OP-101/therapy", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging therapy on CLB-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.TetanusBotulismOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged || len(staged.TherapyStagings) != 2 {
		t.Errorf("expected status %s and 2 stagings, got %s (%d)", domain.ClostridialStatusMetronidazoleTATMethocarbamolStaged, staged.Status, len(staged.TherapyStagings))
	}

	// 7. POST /api/tetanus-botulism/operations/CLB-OP-101/neuro-check transitions to SENSORY_ISOLATION_VENTILATION_MONITORED
	checkPayload, _ := json.Marshal(map[string]any{
		"burkitt_severity_class":            1,
		"spasm_frequency_per_hr":            0,
		"gag_reflex_intact":                 true,
		"core_temp_celsius":                 38.5,
		"paco2_mmhg":                        38.0,
		"heart_rate_bpm":                    88,
		"dark_quiet_cotton_earplugs_active": true,
		"recorded_by":                       "Dr. Ingrid Vance, DACVIM (Neurology)",
		"clinical_note":                     "Day 14 sensory-isolation check: Risus sardonicus and sawhorse rigidity resolved, 0 spasms/hr, intact pharyngeal gag reflex, normocapnic PaCO2 38.0 mmHg.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/tetanus-botulism/operations/CLB-OP-101/neuro-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording neuro check on CLB-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.TetanusBotulismOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if checked.Status != domain.ClostridialStatusSensoryIsolationVentilationMonitored || len(checked.NeuroChecks) != 2 || checked.SpasmFrequencyPerHr != 0 {
		t.Errorf("expected SENSORY_ISOLATION_VENTILATION_MONITORED and 2 neuro checks on CLB-OP-101, got %+v", checked)
	}

	// 8. POST /api/tetanus-botulism/operations/CLB-OP-101/clearance transitions to NORMOREFLEXIC_SWALLOWING_INTACT_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"spasm_frequency_per_hr":           0,
		"gag_reflex_intact":                true,
		"paco2_mmhg":                       38.0,
		"ambulatory_normothermic_verified": true,
		"cleared_by":                       "Dr. Ingrid Vance, DACVIM (Neurology)",
		"clearance_summary":                "0 spasms/hr, intact pharyngeal gag reflex, PaCO2 38.0 mmHg, and ambulatory normothermia verified; cleared for step-down.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/tetanus-botulism/operations/CLB-OP-101/clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing CLB-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.TetanusBotulismOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.ClostridialStatusNormoreflexicSwallowingIntactCleared || cleared.ClearedBy != "Dr. Ingrid Vance, DACVIM (Neurology)" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/tetanus-botulism/operations/UNKNOWN",
		"/api/tetanus-botulism/operations/UNKNOWN/therapy",
		"/api/tetanus-botulism/operations/UNKNOWN/neuro-check",
		"/api/tetanus-botulism/operations/UNKNOWN/clearance",
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

func TestTetanusBotulismHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/tetanus-botulism/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/tetanus-botulism/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/tetanus-botulism/stream, got %d", resp.StatusCode)
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
