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
	"github.com/scottdensmore/petspotr/pkg/cardiogenicedema"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestCardiogenicEdemaHandlers_SeededCreateStageDiuresisClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ceSSECh, unsubSSE := server.ReunionHub().Subscribe("cardiogenic_edema")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("cardiogenic_edema", "cardiogenic-edema-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /cardiogenic-edema-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/cardiogenic-edema-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /cardiogenic-edema-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture",
		"kpi-total-ops",
		"kpi-fulminant-critical",
		"kpi-furosemide-staged",
		"kpi-eupneic-cleared",
		"kpi-fluid-guard-locks",
		"cardiogenic-alert-banner",
		"cardiogenic-clearance-banner",
		"cardiogenic-operations-list",
		"active-operation-workspace",
		"CE-OP-101",
		"CE-OP-102",
		"CE-OP-103",
		"Oliver",
		"Cleo",
		"Teddy",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /cardiogenic-edema-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/cardiogenic-edema/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/cardiogenic-edema/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/cardiogenic-edema/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.CardiogenicEdemaOperation  `json:"operations"`
		Metrics    cardiogenicedema.CardiogenicMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal cardiogenic edema operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	// 2b. GET /api/cardiogenic-edema/operations/{id}
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/cardiogenic-edema/operations/CE-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/cardiogenic-edema/operations/CE-OP-101, got %d", recGet.Code)
	}
	var op101 domain.CardiogenicEdemaOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal CE-OP-101: %v", err)
	}
	if op101.PetName != "Oliver" || op101.Status != domain.CardiogenicStatusFulminantEdemaCritical {
		t.Errorf("unexpected CE-OP-101: %+v", op101)
	}

	// 2c. GET /api/cardiogenic-edema/operations/UNKNOWN returns 404
	recGet404 := httptest.NewRecorder()
	reqGet404 := httptest.NewRequest(http.MethodGet, "/api/cardiogenic-edema/operations/UNKNOWN", nil)
	server.ServeHTTP(recGet404, reqGet404)
	if recGet404.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown op, got %d", recGet404.Code)
	}

	// 2d. Filter by ?status=FULMINANT_PULMONARY_EDEMA_CRITICAL
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/cardiogenic-edema/operations?status=FULMINANT_PULMONARY_EDEMA_CRITICAL", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.CardiogenicEdemaOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "CE-OP-101" {
		t.Errorf("expected 1 filtered operation CE-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. POST /api/cardiogenic-edema/operations with invalid payload returns 400
	invalidPayload, _ := json.Marshal(map[string]any{
		"incident_id": "",
		"pet_name":    "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/cardiogenic-edema/operations", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 4. POST /api/cardiogenic-edema/operations creates a new operation (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-CE-9104",
		"pet_name":    "Darcy",
		"patient_profile": map[string]any{
			"species":                  "CANINE_MMVD_STAGE_C_D",
			"weight_kg":                8.5,
			"respiratory_rate_bpm":     82,
			"heart_rate_bpm":           185,
			"murmur_grade":             "GRADE_VI_THRILL_CHORDAE_RUPTURE",
			"tfast_b_lines_per_site":   7,
			"la_ao_ratio":              2.35,
			"pleural_effusion_present": false,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/cardiogenic-edema/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/cardiogenic-edema/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.CardiogenicEdemaOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "CE-OP-104" || created.Status != domain.CardiogenicStatusFulminantEdemaCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-ceSSECh:
		if evt.Type != domain.ReunionEventType("cardiogenic_edema_updated") {
			t.Errorf("expected cardiogenic_edema_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for cardiogenic_edema_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "cardiogenic_edema_updated" {
			t.Errorf("expected cardiogenic_edema_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for cardiogenic_edema_updated mesh event")
	}

	// 5. POST /api/cardiogenic-edema/operations/CE-OP-101/stage-regimen
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                  "CANINE_MMVD_STAGE_C_D",
		"weight_kg":                9.2,
		"respiratory_rate_bpm":     88,
		"heart_rate_bpm":           190,
		"murmur_grade":             "GRADE_VI_THRILL_CHORDAE_RUPTURE",
		"tfast_b_lines_per_site":   8,
		"la_ao_ratio":              2.45,
		"pleural_effusion_present": false,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/cardiogenic-edema/operations/CE-OP-101/stage-regimen", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging regimen, got %d: %s", recStage.Code, recStage.Body.String())
	}

	// 6. POST /api/cardiogenic-edema/operations/CE-OP-101/diuresis-check
	checkPayload, _ := json.Marshal(map[string]any{
		"resting_respiratory_rate_bpm": 34,
		"urine_output_ml_kg_hr":        4.2,
		"tfast_b_lines_remaining":      2,
		"systolic_blood_pressure_mmhg": 116,
		"creatinine_mg_dl":             1.3,
		"potassium_meq_l":              3.9,
		"clinician_callsign":           "DR-CARDIO-18",
		"notes":                        "Strong natriuresis response.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/cardiogenic-edema/operations/CE-OP-101/diuresis-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording diuresis check, got %d: %s", recCheck.Code, recCheck.Body.String())
	}

	// 7. POST /api/cardiogenic-edema/operations/CE-OP-101/discharge-clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_sleeping_resp_rate_bpm": 22,
		"final_tfast_b_lines_count":    0,
		"room_air_spo2_percent":        98.0,
		"renal_azotemia_stable":        true,
		"home_srr_monitoring_educated": true,
		"vet_license":                  "VET-CARDIO-99201",
		"summary":                      "Eupneic on room air, B-lines resolved.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/cardiogenic-edema/operations/CE-OP-101/discharge-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing discharge clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}

	var cleared domain.CardiogenicEdemaOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.CardiogenicStatusEupneicBLineResolved || cleared.FluidOverloadContraindicationAlert {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}

	// 8. 404 Not Found for unknown operation IDs
	for _, path := range []string{
		"/api/cardiogenic-edema/operations/UNKNOWN/stage-regimen",
		"/api/cardiogenic-edema/operations/UNKNOWN/diuresis-check",
		"/api/cardiogenic-edema/operations/UNKNOWN/discharge-clearance",
	} {
		rec404 := httptest.NewRecorder()
		req404 := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(`{}`)))
		server.ServeHTTP(rec404, req404)
		if rec404.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found for %s, got %d", path, rec404.Code)
		}
	}
}

func TestCardiogenicEdemaHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/cardiogenic-edema/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/cardiogenic-edema/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/cardiogenic-edema/stream, got %d", resp.StatusCode)
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
