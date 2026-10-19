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

func TestFATEThrombusHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	fateSSECh, unsubSSE := server.ReunionHub().Subscribe("fate_thrombus")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("fate_thrombus", "fate-thrombus-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /fate-thrombus-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/fate-thrombus-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /fate-thrombus-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Feline Aortic Thromboembolism (FATE / Saddle Thrombus)",
		"fate-thrombus-cockpit",
		"fate-alert-banner",
		"fate-clearance-banner",
		"fate-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"fate-operations-grid",
		"fate-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-glucose-delta",
		"active-limb-temp",
		"stage-antithrombotic-form",
		"record-perfusion-check-form",
		"clear-fate-form",
		"FATE-OP-101",
		"FATE-OP-102",
		"FATE-OP-103",
		"Jasper",
		"Cleo",
		"Figaro",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /fate-thrombus-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/fate-thrombus/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/fate-thrombus/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/fate-thrombus/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.FATEThrombusOperation `json:"operations"`
		Metrics    domain.FATEOverviewMetrics     `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal FATE operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?distribution=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/fate-thrombus/operations?status=ACUTE_SADDLE_THROMBUS_5PS_TRIAGE&distribution=BILATERAL_AORTIC_TRIFURCATION_SADDLE", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.FATEThrombusOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "FATE-OP-101" {
		t.Errorf("expected 1 filtered operation FATE-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/fate-thrombus/operations/FATE-OP-101 returns Jasper
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/fate-thrombus/operations/FATE-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/fate-thrombus/operations/FATE-OP-101, got %d", recGet.Code)
	}
	var op101 domain.FATEThrombusOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal FATE-OP-101: %v", err)
	}
	if op101.PetName != "Jasper" || op101.Status != domain.FATEStatusAcuteSaddle5PsTriage || op101.GlucoseDeltaMgDL != 93.0 {
		t.Errorf("unexpected FATE-OP-101 state: %+v", op101)
	}

	// 4. POST /api/fate-thrombus/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/fate-thrombus/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/fate-thrombus/operations creates a new operation "Oliver" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                      "CASE-FATE-104",
		"pet_name":                     "Oliver",
		"species":                      "Feline",
		"breed":                        "Domestic Shorthair",
		"weight_kg":                    5.6,
		"distribution_type":            "BILATERAL_AORTIC_TRIFURCATION_SADDLE",
		"systemic_glucose_mg_dl":       192.0,
		"affected_limb_glucose_mg_dl":  88.0,
		"systemic_lactate_mmol_l":      2.2,
		"affected_limb_lactate_mmol_l": 12.6,
		"serum_potassium_mmol_l":       4.8,
		"distal_limb_temp_c":           28.9,
		"la2ao_ratio":                  2.40,
		"concurrent_cardiogenic_chf":   true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/fate-thrombus/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/fate-thrombus/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.FATEThrombusOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "FATE-OP-104" || created.PetName != "Oliver" || created.GlucoseDeltaMgDL != 104.0 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-fateSSECh:
		if evt.Type != domain.ReunionEventType("fate_thrombus_updated") {
			t.Errorf("expected fate_thrombus_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for fate_thrombus_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "fate_thrombus_updated" {
			t.Errorf("expected fate_thrombus_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for fate_thrombus_updated mesh event")
	}

	// 6. POST /api/fate-thrombus/operations/FATE-OP-101/stage-antithrombotic transitions to METHADONE_CLOPIDOGREL_ANTICOAG_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"full_mu_opioid_cri_protocol":   "Methadone 0.25 mg/kg IV q4h transitioning to Fentanyl CRI 3–5 mcg/kg/hr IV",
		"clopidogrel_loading_protocol":  "Clopidogrel 75 mg PO one-time oral loading dose (FAT CAT Trial), followed by 18.75 mg PO q24h",
		"factor_xa_or_heparin_protocol": "Enoxaparin 0.9 mg/kg SC q8h transitioning to Rivaroxaban 2.5 mg PO q24h dual therapy",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/fate-thrombus/operations/FATE-OP-101/stage-antithrombotic", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging antithrombotic protocol on FATE-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.FATEThrombusOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.FATEStatusMethadoneClopidogrelStaged {
		t.Errorf("expected status %s, got %s", domain.FATEStatusMethadoneClopidogrelStaged, staged.Status)
	}

	// 7. POST /api/fate-thrombus/operations/FATE-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"glucose_delta_mg_dl":    18.0,
		"lactate_delta_mmol_l":   1.5,
		"serum_potassium_mmol_l": 4.4,
		"distal_limb_temp_c":     36.8,
		"femoral_pulse_status":   "BILATERAL_DOPPLER_AUDIBLE_WARMING_PADS",
		"recorded_by":            "Dr. Maya Lin, DVM, DACVECC",
		"clinical_note":          "Collateral reperfusion progressing; glucose delta decreased to 18.0 mg/dL, limb temp rose to 36.8°C, normokalemic K+ 4.4 mmol/L.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/fate-thrombus/operations/FATE-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording perfusion check on FATE-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.FATEThrombusOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.PerfusionChecks) != 3 || checked.GlucoseDeltaMgDL != 18.0 || checked.DistalLimbTempC != 36.8 {
		t.Errorf("expected 3 perfusion checks and updated glucoseDelta/limbTemp on FATE-OP-101, got %+v", checked)
	}

	// 8. POST /api/fate-thrombus/operations/FATE-OP-101/clear-discharge transitions to REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Julian Vance, DACVIM (Cardiology)",
		"discharge_summary": "Bilateral pelvic limb perfusion restored, normokalemia verified (K+ 4.4 mmol/L), discharged on Clopidogrel 18.75 mg PO q24h + Rivaroxaban 2.5 mg PO q24h.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/fate-thrombus/operations/FATE-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on FATE-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.FATEThrombusOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.FATEStatusReperfusedNormokalemicCleared || cleared.DischargedBy != "Dr. Julian Vance, DACVIM (Cardiology)" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/fate-thrombus/operations/UNKNOWN",
		"/api/fate-thrombus/operations/UNKNOWN/stage-antithrombotic",
		"/api/fate-thrombus/operations/UNKNOWN/record-check",
		"/api/fate-thrombus/operations/UNKNOWN/clear-discharge",
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

func TestFATEThrombusHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/fate-thrombus/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/fate-thrombus/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/fate-thrombus/stream, got %d", resp.StatusCode)
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
