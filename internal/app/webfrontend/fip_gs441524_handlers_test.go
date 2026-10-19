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

func TestFIPGS441524Handlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	fipSSECh, unsubSSE := server.ReunionHub().Subscribe("fip_gs441524")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("fip_gs441524", "fip-gs441524-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /fip-gs441524-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/fip-gs441524-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /fip-gs441524-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Feline Infectious Peritonitis (FIP",
		"fip-gs441524-cockpit",
		"fip-alert-banner",
		"fip-clearance-banner",
		"fip-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"fip-operations-grid",
		"fip-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-ag-ratio",
		"active-treatment-day",
		"create-fip-form",
		"stage-antiviral-form",
		"record-progress-check-form",
		"clear-fip-form",
		"FIP-OP-101",
		"FIP-OP-102",
		"FIP-OP-103",
		"Bramble",
		"Cosmo",
		"Saffron",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /fip-gs441524-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/fip-gs441524/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/fip-gs441524/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/fip-gs441524/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.FIPGS441524Operation `json:"operations"`
		Metrics    domain.FIPOverviewMetrics     `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal FIP operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?phenotype=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/fip-gs441524/operations?status=ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE&phenotype=EFFUSIVE_CAVITARY_PLEURAL_ASCITES_FIP", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.FIPGS441524Operation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "FIP-OP-101" {
		t.Errorf("expected 1 filtered operation FIP-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/fip-gs441524/operations/FIP-OP-101 returns Bramble
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/fip-gs441524/operations/FIP-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/fip-gs441524/operations/FIP-OP-101, got %d", recGet.Code)
	}
	var op101 domain.FIPGS441524Operation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal FIP-OP-101: %v", err)
	}
	if op101.PetName != "Bramble" || op101.Status != domain.FIPStatusAcuteTriage || op101.AlbuminGlobulinRatio != 0.28 {
		t.Errorf("unexpected FIP-OP-101 state: %+v", op101)
	}

	// 4. POST /api/fip-gs441524/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/fip-gs441524/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/fip-gs441524/operations creates a new operation "Mochi" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                          "CASE-FIP-104",
		"pet_name":                         "Mochi",
		"species":                          "Feline",
		"breed":                            "Scottish Fold",
		"weight_kg":                        2.8,
		"phenotype":                        "EFFUSIVE_CAVITARY_PLEURAL_ASCITES_FIP",
		"treatment_day":                    1,
		"albumin_globulin_ratio":           0.32,
		"total_bilirubin_mg_dl":            2.1,
		"hematocrit_percent":               23.5,
		"daily_gs441524_mg_kg":             10.0,
		"rivalta_test_positive":            true,
		"pleural_effusion_dyspnea_present": true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/fip-gs441524/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/fip-gs441524/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.FIPGS441524Operation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "FIP-OP-104" || created.PetName != "Mochi" || created.AlbuminGlobulinRatio != 0.32 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-fipSSECh:
		if evt.Type != domain.ReunionEventType("fip_gs441524_updated") {
			t.Errorf("expected fip_gs441524_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for fip_gs441524_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "fip_gs441524_updated" {
			t.Errorf("expected fip_gs441524_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for fip_gs441524_updated mesh event")
	}

	// 6. POST /api/fip-gs441524/operations/FIP-OP-101/stage-antiviral transitions to GS441524_REMDESIVIR_84DAY_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"gs441524_or_remdesivir_dosing":     "GS-441524 10 mg/kg/day (24 mg/day PO divided q12h) for 84-day (12-week) course",
		"weight_adjusted_dose_recalc_note":  "Weigh weekly on pediatric scale and recalculate GS-441524 dose upward; never reduce dose",
		"therapeutic_thoracocentesis_limit": "Immediate thoracocentesis for pleural dyspnea; never drain > 20-30% abdominal ascites",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/fip-gs441524/operations/FIP-OP-101/stage-antiviral", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging antiviral protocol on FIP-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.FIPGS441524Operation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.FIPStatusGS441524Staged {
		t.Errorf("expected status %s, got %s", domain.FIPStatusGS441524Staged, staged.Status)
	}

	// 7. POST /api/fip-gs441524/operations/FIP-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"treatment_day":          84,
		"weight_kg":              3.5,
		"albumin_globulin_ratio": 0.72,
		"total_bilirubin_mg_dl":  0.2,
		"hematocrit_percent":     36.5,
		"recorded_by":            "Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
		"clinical_note":          "Day 84 (Week 12) Check: Effusion completely resolved, weight 3.5 kg, A:G Ratio normalized to 0.72 (> 0.60), Total Bilirubin 0.2 mg/dL, HCT 36.5%.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/fip-gs441524/operations/FIP-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording progress check on FIP-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.FIPGS441524Operation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.ProgressChecks) != 3 || checked.TreatmentDay != 84 || checked.AlbuminGlobulinRatio != 0.72 {
		t.Errorf("expected 3 progress checks and updated Day/A:G Ratio on FIP-OP-101, got %+v", checked)
	}

	// 8. POST /api/fip-gs441524/operations/FIP-OP-101/clear-discharge transitions to AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
		"discharge_summary": "Completed full 84-day GS-441524 protocol; A:G Ratio normalized to 0.72 (> 0.60), effusion resolved, HCT 36.5%, cleared as 84-day FIP clinical cure.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/fip-gs441524/operations/FIP-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on FIP-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.FIPGS441524Operation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.FIPStatusCleared84DayCured || cleared.DischargedBy != "Dr. Linnea Lindholm, DACVIM (Feline Medicine)" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/fip-gs441524/operations/UNKNOWN",
		"/api/fip-gs441524/operations/UNKNOWN/stage-antiviral",
		"/api/fip-gs441524/operations/UNKNOWN/record-check",
		"/api/fip-gs441524/operations/UNKNOWN/clear-discharge",
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

func TestFIPGS441524Handlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/fip-gs441524/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/fip-gs441524/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/fip-gs441524/stream, got %d", resp.StatusCode)
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
