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

func TestOncologicTLSHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	tlsSSECh, unsubSSE := server.ReunionHub().Subscribe("oncologic_tls")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("oncologic_tls", "oncologic-tls-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /oncologic-tls-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/oncologic-tls-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /oncologic-tls-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Tumor Lysis Syndrome (TLS) &amp; Oncologic Hypercalcemia",
		"oncologic-tls-cockpit",
		"tls-alert-banner",
		"tls-clearance-banner",
		"tls-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"tls-operations-grid",
		"tls-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-potassium",
		"active-ionized-calcium",
		"create-tls-form",
		"stage-diuresis-form",
		"record-electrolyte-check-form",
		"clear-tls-form",
		"TLS-OP-101",
		"TLS-OP-102",
		"TLS-OP-103",
		"Bauer",
		"Penny",
		"Otis",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /oncologic-tls-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/oncologic-tls/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/oncologic-tls/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/oncologic-tls/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.OncologicTLSOperation     `json:"operations"`
		Metrics    domain.OncologicTLSOverviewMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal Oncologic TLS operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?emergency_type=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/oncologic-tls/operations?status=ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE&emergency_type=ACUTE_TUMOR_LYSIS_SYNDROME_LYMPHOMA", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.OncologicTLSOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "TLS-OP-101" {
		t.Errorf("expected 1 filtered operation TLS-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/oncologic-tls/operations/TLS-OP-101 returns Bauer
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/oncologic-tls/operations/TLS-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/oncologic-tls/operations/TLS-OP-101, got %d", recGet.Code)
	}
	var op101 domain.OncologicTLSOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal TLS-OP-101: %v", err)
	}
	if op101.PetName != "Bauer" || op101.Status != domain.TLSStatusAcuteMetabolicTriage || op101.SerumPotassiumMmolL != 7.2 {
		t.Errorf("unexpected TLS-OP-101 state: %+v", op101)
	}

	// 4. POST /api/oncologic-tls/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/oncologic-tls/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/oncologic-tls/operations creates a new operation "Apollo" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                       "CASE-TLS-104",
		"pet_name":                      "Apollo",
		"species":                       "Canine",
		"breed":                         "Golden Retriever",
		"weight_kg":                     30.5,
		"emergency_type":                "ACUTE_TUMOR_LYSIS_SYNDROME_LYMPHOMA",
		"serum_potassium_mmol_l":        6.9,
		"serum_phosphorus_mg_dl":        10.8,
		"ionized_calcium_mmol_l":        0.82,
		"uric_acid_mg_dl":               8.8,
		"serum_creatinine_mg_dl":        2.8,
		"ca_x_p_product":                92.5,
		"definitive_cytology_confirmed": true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/oncologic-tls/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/oncologic-tls/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.OncologicTLSOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "TLS-OP-104" || created.PetName != "Apollo" || created.SerumPotassiumMmolL != 6.9 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-tlsSSECh:
		if evt.Type != domain.ReunionEventType("oncologic_tls_updated") {
			t.Errorf("expected oncologic_tls_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for oncologic_tls_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "oncologic_tls_updated" {
			t.Errorf("expected oncologic_tls_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for oncologic_tls_updated mesh event")
	}

	// 6. POST /api/oncologic-tls/operations/TLS-OP-101/stage-diuresis transitions to SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"zero_calcium_saline_diuresis_plan": "0.9% NaCl Calcium-Free IV Diuresis at 150 mL/hr (2.5x maintenance)",
		"rasburicase_allopurinol_protocol":  "Rasburicase 0.2 mg/kg IV over 30 min + Allopurinol 10 mg/kg PO q8h",
		"zoledronate_calcitonin_protocol":   "Zoledronate 0.25 mg/kg IV over 15 min + Calcitonin 4-6 IU/kg SC q8-12h",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/oncologic-tls/operations/TLS-OP-101/stage-diuresis", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging diuresis therapy on TLS-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.OncologicTLSOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.TLSStatusDiuresisStaged {
		t.Errorf("expected status %s, got %s", domain.TLSStatusDiuresisStaged, staged.Status)
	}

	// 7. POST /api/oncologic-tls/operations/TLS-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"serum_potassium_mmol_l": 4.4,
		"serum_phosphorus_mg_dl": 4.6,
		"ionized_calcium_mmol_l": 1.24,
		"uric_acid_mg_dl":        1.6,
		"serum_creatinine_mg_dl": 1.3,
		"recorded_by":            "Dr. Soren Lindqvist, DACVIM (Oncology)",
		"clinical_note":          "Post-0.9% NaCl diuresis & Rasburicase: K+ normalized to 4.4 mmol/L, iCa restored to 1.24 mmol/L.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/oncologic-tls/operations/TLS-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording electrolyte check on TLS-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.OncologicTLSOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.ElectrolyteChecks) != 3 || checked.SerumPotassiumMmolL != 4.4 || checked.IonizedCalciumMmolL != 1.24 {
		t.Errorf("expected 3 electrolyte checks and updated K+/iCa on TLS-OP-101, got %+v", checked)
	}

	// 8. POST /api/oncologic-tls/operations/TLS-OP-101/clear-discharge transitions to ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Soren Lindqvist, DACVIM (Oncology)",
		"discharge_summary": "Electrolytes normalized (K+ 4.4 mmol/L, iCa 1.24 mmol/L, Phosphorus 4.6 mg/dL), renal function protected, cleared for oncology ward.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/oncologic-tls/operations/TLS-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on TLS-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.OncologicTLSOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.TLSStatusNormoelectrolyticCleared || cleared.DischargedBy != "Dr. Soren Lindqvist, DACVIM (Oncology)" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/oncologic-tls/operations/UNKNOWN",
		"/api/oncologic-tls/operations/UNKNOWN/stage-diuresis",
		"/api/oncologic-tls/operations/UNKNOWN/record-check",
		"/api/oncologic-tls/operations/UNKNOWN/clear-discharge",
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

func TestOncologicTLSHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/oncologic-tls/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/oncologic-tls/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/oncologic-tls/stream, got %d", resp.StatusCode)
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
