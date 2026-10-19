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
	"github.com/scottdensmore/petspotr/pkg/imhathrombosis"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestIMHAThrombosisHandlers_SeededCRUDRegimenCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	imhaSSECh, unsubSSE := server.ReunionHub().Subscribe("imha_thrombosis")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("imha_thrombosis", "imha-thrombosis-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /imha-thrombosis-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/imha-thrombosis-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /imha-thrombosis-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Immune-Mediated Hemolytic Anemia (IMHA)",
		"kpi-total-ops",
		"kpi-critical-hemolytic",
		"kpi-staged-regimen",
		"kpi-cleared-stable",
		"kpi-pte-alerts",
		"imha-alert-banner",
		"imha-clearance-banner",
		"IM-OP-101",
		"IM-OP-102",
		"IM-OP-103",
		"Penny",
		"Scout",
		"Maple",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /imha-thrombosis-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/imha-thrombosis/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/imha-thrombosis/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/imha-thrombosis/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.IMHAThrombosisOperation `json:"operations"`
		Metrics    imhathrombosis.IMHAMetrics       `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal imha thrombosis operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/imha-thrombosis/operations?status=HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL&species=CANINE_PRIMARY_IMHA", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.IMHAThrombosisOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "IM-OP-101" {
		t.Errorf("expected 1 filtered operation IM-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/imha-thrombosis/operations/IM-OP-101 returns Penny
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/imha-thrombosis/operations/IM-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/imha-thrombosis/operations/IM-OP-101, got %d", recGet.Code)
	}
	var op101 domain.IMHAThrombosisOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal IM-OP-101: %v", err)
	}
	if op101.PetName != "Penny" || op101.Status != domain.IMHAStatusHemolyticCrisisCritical {
		t.Errorf("unexpected IM-OP-101 state: %+v", op101)
	}

	// 4. POST /api/imha-thrombosis/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/imha-thrombosis/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/imha-thrombosis/operations creates a new operation "Poppy" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-IM-9704",
		"pet_name":    "Poppy",
		"patient_profile": map[string]any{
			"species":                              "CANINE_PRIMARY_IMHA",
			"breed":                                "Cocker Spaniel",
			"weight_kg":                            12.5,
			"packed_cell_volume_percent":           13.5,
			"total_bilirubin_mg_dl":                5.2,
			"platelet_count_k_ul":                  92.0,
			"spherocytes_per_hpf":                  16,
			"saline_agglutination_4_to_1_positive": true,
			"intravascular_hemoglobinuria":         true,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/imha-thrombosis/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/imha-thrombosis/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.IMHAThrombosisOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "IM-OP-104" || created.PetName != "Poppy" || created.Status != domain.IMHAStatusHemolyticCrisisCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-imhaSSECh:
		if evt.Type != domain.ReunionEventType("imha_thrombosis_updated") {
			t.Errorf("expected imha_thrombosis_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for imha_thrombosis_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "imha_thrombosis_updated" {
			t.Errorf("expected imha_thrombosis_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for imha_thrombosis_updated mesh event")
	}

	// 6. POST /api/imha-thrombosis/operations/IM-OP-101/stage-regimen transitions to PREDNISOLONE_CLOPIDOGREL_PRBC_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                    "CANINE_PRIMARY_IMHA",
		"weight_kg":                  11.5,
		"packed_cell_volume_percent": 15.0,
		"total_bilirubin_mg_dl":      4.8,
		"platelet_count_k_ul":        95.0,
		"spherocytes_per_hpf":        14,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/imha-thrombosis/operations/IM-OP-101/stage-regimen", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging regimen on IM-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.IMHAThrombosisOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.IMHAStatusImmunosuppressantAnticoagStaged {
		t.Errorf("expected status %s, got %s", domain.IMHAStatusImmunosuppressantAnticoagStaged, staged.Status)
	}

	// 7. POST /api/imha-thrombosis/operations/IM-OP-101/hemolysis-check records hemolysis check
	checkPayload, _ := json.Marshal(map[string]any{
		"packed_cell_volume_percent":      23.0,
		"reticulocyte_count_k_ul":         110.0,
		"total_bilirubin_mg_dl":           1.8,
		"saline_agglutination_persistent": false,
		"resting_respiratory_rate_bpm":    24,
		"clinician_callsign":              "DR-HEME-11",
		"notes":                           "Post-pRBC PCV rose to 23.0%, regenerative reticulocytosis 110 K/uL, plasma clearing, eupneic.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/imha-thrombosis/operations/IM-OP-101/hemolysis-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording hemolysis check on IM-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.IMHAThrombosisOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.HemolysisChecks) != 2 {
		t.Errorf("expected 2 hemolysis checks on IM-OP-101, got %d", len(checked.HemolysisChecks))
	}

	// 8. POST /api/imha-thrombosis/operations/IM-OP-101/discharge-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_pcv_stable_48_hr_percent":       28.5,
		"final_reticulocyte_count_k_ul":        138.0,
		"saline_agglutination_resolved":        true,
		"no_pulmonary_thromboembolism_dyspnea": true,
		"thromboprophylaxis_dispensed":         true,
		"vet_license":                          "VET-IM-97410",
		"summary":                              "PCV stable at 28.5% for 48h; reticulocytes 138 K/uL; 1:4 saline agglutination resolved; Clopidogrel + Rivaroxaban & slow Prednisolone taper dispensed.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/imha-thrombosis/operations/IM-OP-101/discharge-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing discharge clearance on IM-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.IMHAThrombosisOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.IMHAStatusPCVStableReticulocytosisCleared || cleared.PulmonaryThromboembolismAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/imha-thrombosis/operations/UNKNOWN",
		"/api/imha-thrombosis/operations/UNKNOWN/stage-regimen",
		"/api/imha-thrombosis/operations/UNKNOWN/hemolysis-check",
		"/api/imha-thrombosis/operations/UNKNOWN/discharge-clearance",
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

func TestIMHAThrombosisHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/imha-thrombosis/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/imha-thrombosis/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/imha-thrombosis/stream, got %d", resp.StatusCode)
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
