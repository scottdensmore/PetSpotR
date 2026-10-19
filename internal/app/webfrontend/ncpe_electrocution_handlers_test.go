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

func TestNCPEElectrocutionHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ncpeSSECh, unsubSSE := server.ReunionHub().Subscribe("ncpe_electrocution")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("ncpe_electrocution", "ncpe-electrocution-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /ncpe-electrocution-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/ncpe-electrocution-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /ncpe-electrocution-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Non-Cardiogenic Pulmonary Edema (NCPE)",
		"ncpe-electrocution-cockpit",
		"ncpe-alert-banner",
		"ncpe-clearance-banner",
		"ncpe-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"ncpe-operations-grid",
		"ncpe-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-pf-ratio",
		"active-blines-count",
		"create-ncpe-form",
		"stage-cpap-form",
		"record-respiratory-check-form",
		"clear-ncpe-form",
		"NCPE-OP-101",
		"NCPE-OP-102",
		"NCPE-OP-103",
		"Sparky",
		"Milo",
		"Luna",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /ncpe-electrocution-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/ncpe-electrocution/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/ncpe-electrocution/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/ncpe-electrocution/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.NCPEElectrocutionOperation `json:"operations"`
		Metrics    domain.NCPEOverviewMetrics          `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal NCPE operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?etiology_class=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/ncpe-electrocution/operations?status=ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE&etiology_class=ORAL_ELECTRICAL_CORD_BITE_NEUROGENIC", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.NCPEElectrocutionOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "NCPE-OP-101" {
		t.Errorf("expected 1 filtered operation NCPE-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/ncpe-electrocution/operations/NCPE-OP-101 returns Sparky
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/ncpe-electrocution/operations/NCPE-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/ncpe-electrocution/operations/NCPE-OP-101, got %d", recGet.Code)
	}
	var op101 domain.NCPEElectrocutionOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal NCPE-OP-101: %v", err)
	}
	if op101.PetName != "Sparky" || op101.Status != domain.NCPEStatusAcuteCaudodorsalTriage || op101.PaO2FiO2Ratio != 165.0 {
		t.Errorf("unexpected NCPE-OP-101 state: %+v", op101)
	}

	// 4. POST /api/ncpe-electrocution/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/ncpe-electrocution/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/ncpe-electrocution/operations creates a new operation "Volt" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                        "CASE-NCPE-104",
		"pet_name":                       "Volt",
		"species":                        "Canine",
		"breed":                          "Labrador Retriever Puppy",
		"weight_kg":                      7.2,
		"etiology_class":                 "ORAL_ELECTRICAL_CORD_BITE_NEUROGENIC",
		"pao2_fio2_ratio":                155.0,
		"spo2_percent":                   85.0,
		"respiratory_rate_bpm":           72,
		"caudodorsal_blines_count":       16,
		"la2ao_ratio":                    1.20,
		"oral_electrical_eschar_present": true,
		"pink_frothy_airway_fluid":       true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/ncpe-electrocution/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/ncpe-electrocution/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.NCPEElectrocutionOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "NCPE-OP-104" || created.PetName != "Volt" || created.PaO2FiO2Ratio != 155.0 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-ncpeSSECh:
		if evt.Type != domain.ReunionEventType("ncpe_electrocution_updated") {
			t.Errorf("expected ncpe_electrocution_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for ncpe_electrocution_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "ncpe_electrocution_updated" {
			t.Errorf("expected ncpe_electrocution_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for ncpe_electrocution_updated mesh event")
	}

	// 6. POST /api/ncpe-electrocution/operations/NCPE-OP-101/stage-cpap transitions to OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"oxygen_fio2_or_hfnc_protocol":        "High-Flow Nasal Cannula (HFNC) at 2.0 L/kg/min warm humidified O2 (FiO2 60%)",
		"anxiolytic_butorphanol_acepromazine": "Butorphanol 0.2 mg/kg IV q4h + Acepromazine 0.01 mg/kg IV for anxiolysis",
		"cpap_or_peep_mechanical_ventilation": "Helmet CPAP (6 cmH2O PEEP) / Mechanical Ventilation with PEEP 5–10 cmH2O",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/ncpe-electrocution/operations/NCPE-OP-101/stage-cpap", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging CPAP ventilation on NCPE-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.NCPEElectrocutionOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.NCPEStatusOxygenCPAPStaged {
		t.Errorf("expected status %s, got %s", domain.NCPEStatusOxygenCPAPStaged, staged.Status)
	}

	// 7. POST /api/ncpe-electrocution/operations/NCPE-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"pao2_fio2_ratio":          395.0,
		"spo2_percent":             98.0,
		"respiratory_rate_bpm":     28,
		"caudodorsal_blines_count": 0,
		"la2ao_ratio":              1.22,
		"recorded_by":              "Dr. Nadia Varma, DACVECC",
		"clinical_note":            "Post-HFNC, Helmet CPAP & Butorphanol anxiolysis: caudodorsal B-lines resolved (0 B-lines), PaO2/FiO2 395, SpO2 98% on room air.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/ncpe-electrocution/operations/NCPE-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording respiratory check on NCPE-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.NCPEElectrocutionOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.RespiratoryChecks) != 3 || checked.PaO2FiO2Ratio != 395.0 || checked.CaudodorsalBLinesCount != 0 {
		t.Errorf("expected 3 respiratory checks and updated PaO2/FiO2 & B-lines on NCPE-OP-101, got %+v", checked)
	}

	// 8. POST /api/ncpe-electrocution/operations/NCPE-OP-101/clear-discharge transitions to BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Nadia Varma, DACVECC",
		"discharge_summary": "Caudodorsal B-lines completely resolved (0 B-lines), normoxemic on room air (PaO2/FiO2 395, SpO2 98%, RR 28 bpm), 5–7 day oral eschar monitoring scheduled.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/ncpe-electrocution/operations/NCPE-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on NCPE-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.NCPEElectrocutionOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.NCPEStatusBLinesResolvedCleared || cleared.DischargedBy != "Dr. Nadia Varma, DACVECC" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/ncpe-electrocution/operations/UNKNOWN",
		"/api/ncpe-electrocution/operations/UNKNOWN/stage-cpap",
		"/api/ncpe-electrocution/operations/UNKNOWN/record-check",
		"/api/ncpe-electrocution/operations/UNKNOWN/clear-discharge",
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

func TestNCPEElectrocutionHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/ncpe-electrocution/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/ncpe-electrocution/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/ncpe-electrocution/stream, got %d", resp.StatusCode)
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
