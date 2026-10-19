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
	"github.com/scottdensmore/petspotr/pkg/dkainsulin"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestDKAInsulinHandlers_SeededCRUDRegimenCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	dkaSSECh, unsubSSE := server.ReunionHub().Subscribe("dka_insulin")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("dka_insulin", "dka-insulin-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /dka-insulin-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/dka-insulin-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /dka-insulin-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Diabetic Ketoacidosis (DKA), Regular Insulin CRI, Hypokalemia/Hypophosphatemia",
		"kpi-total-ops",
		"kpi-critical-acidosis",
		"kpi-staged-insulin-cri",
		"kpi-cleared-glargine",
		"kpi-hypokalemia-alerts",
		"dka-alert-banner",
		"dka-clearance-banner",
		"DKA-OP-101",
		"DKA-OP-102",
		"DKA-OP-103",
		"Winston",
		"Daisy",
		"Chester",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /dka-insulin-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/dka-insulin/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/dka-insulin/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/dka-insulin/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.DKAInsulinOperation `json:"operations"`
		Metrics    dkainsulin.DKAMetrics        `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal dka insulin operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/dka-insulin/operations?status=KETOACIDOSIS_DEHYDRATED_CRITICAL&species=FELINE_DKA_HEPATIC_LIPIDOSIS", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.DKAInsulinOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "DKA-OP-101" {
		t.Errorf("expected 1 filtered operation DKA-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/dka-insulin/operations/DKA-OP-101 returns Winston
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/dka-insulin/operations/DKA-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/dka-insulin/operations/DKA-OP-101, got %d", recGet.Code)
	}
	var op101 domain.DKAInsulinOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal DKA-OP-101: %v", err)
	}
	if op101.PetName != "Winston" || op101.Status != domain.DKAStatusAcidosisDehydratedCritical {
		t.Errorf("unexpected DKA-OP-101 state: %+v", op101)
	}

	// 4. POST /api/dka-insulin/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/dka-insulin/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/dka-insulin/operations creates a new operation "Biscuit" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-DKA-9304",
		"pet_name":    "Biscuit",
		"patient_profile": map[string]any{
			"species":                      "CANINE_DKA_PANCREATITIS",
			"breed":                        "Golden Retriever",
			"weight_kg":                    24.0,
			"blood_glucose_mg_dl":          460.0,
			"beta_hydroxybutyrate_mmol_l":  5.4,
			"venous_ph":                    7.14,
			"serum_potassium_meq_l":        2.9,
			"serum_phosphorus_mg_dl":       1.7,
			"effective_osmolality_mosm_kg": 324.0,
			"dehydration_percent":          8.0,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/dka-insulin/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/dka-insulin/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.DKAInsulinOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "DKA-OP-104" || created.PetName != "Biscuit" || created.Status != domain.DKAStatusAcidosisDehydratedCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-dkaSSECh:
		if evt.Type != domain.ReunionEventType("dka_insulin_updated") {
			t.Errorf("expected dka_insulin_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for dka_insulin_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "dka_insulin_updated" {
			t.Errorf("expected dka_insulin_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for dka_insulin_updated mesh event")
	}

	// 6. POST /api/dka-insulin/operations/DKA-OP-101/stage-regimen transitions to KCL_PHOS_REGULAR_INSULIN_CRI_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                "FELINE_DKA_HEPATIC_LIPIDOSIS",
		"weight_kg":              5.4,
		"blood_glucose_mg_dl":    210.0,
		"serum_potassium_meq_l":  2.8,
		"serum_phosphorus_mg_dl": 1.6,
		"dehydration_percent":    6.0,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/dka-insulin/operations/DKA-OP-101/stage-regimen", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging regimen on DKA-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.DKAInsulinOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.DKAStatusKClPhosInsulinCRIStaged {
		t.Errorf("expected status %s, got %s", domain.DKAStatusKClPhosInsulinCRIStaged, staged.Status)
	}

	// 7. POST /api/dka-insulin/operations/DKA-OP-101/serial-check records serial check
	checkPayload, _ := json.Marshal(map[string]any{
		"blood_glucose_mg_dl":                185.0,
		"beta_hydroxybutyrate_mmol_l":        1.6,
		"serum_potassium_meq_l":              3.9,
		"serum_phosphorus_mg_dl":             3.1,
		"dextrose_percent_active":            2.5,
		"mentation_normal_no_cerebral_edema": true,
		"clinician_callsign":                 "DR-ENDO-14",
		"notes":                              "BG falling <= 50 mg/dL/hr; 2.5% dextrose running; K+ corrected to 3.9 mEq/L.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/dka-insulin/operations/DKA-OP-101/serial-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording serial check on DKA-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.DKAInsulinOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.SerialChecks) != 2 {
		t.Errorf("expected 2 serial checks on DKA-OP-101, got %d", len(checked.SerialChecks))
	}

	// 8. POST /api/dka-insulin/operations/DKA-OP-101/discharge-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_beta_hydroxybutyrate_mmol_l": 0.4,
		"final_venous_ph":                   7.39,
		"final_serum_potassium_meq_l":       4.2,
		"final_serum_phosphorus_mg_dl":      3.8,
		"voluntary_eating_no_vomiting":      true,
		"subq_insulin_transition_completed": true,
		"vet_license":                       "VET-ENDO-91042",
		"summary":                           "Ketones cleared; eating voluntarily; transitioned to SubQ Glargine.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/dka-insulin/operations/DKA-OP-101/discharge-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing discharge clearance on DKA-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.DKAInsulinOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.DKAStatusKetoneResolvedGlargineCleared || cleared.HypokalemiaCerebralEdemaAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/dka-insulin/operations/UNKNOWN",
		"/api/dka-insulin/operations/UNKNOWN/stage-regimen",
		"/api/dka-insulin/operations/UNKNOWN/serial-check",
		"/api/dka-insulin/operations/UNKNOWN/discharge-clearance",
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

func TestDKAInsulinHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/dka-insulin/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/dka-insulin/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/dka-insulin/stream, got %d", resp.StatusCode)
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
