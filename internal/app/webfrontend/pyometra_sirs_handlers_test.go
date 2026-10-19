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

func TestPyometraSIRSHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	pyoSSECh, unsubSSE := server.ReunionHub().Subscribe("pyometra_sirs")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("pyometra_sirs", "pyometra-sirs-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /pyometra-sirs-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/pyometra-sirs-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /pyometra-sirs-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Pyometra (Open vs. Closed Cervix)",
		"pyometra-sirs-cockpit",
		"pyometra-alert-banner",
		"pyometra-clearance-banner",
		"pyometra-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"pyometra-operations-grid",
		"pyometra-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-uterine-diameter",
		"active-blood-lactate",
		"create-pyometra-form",
		"stage-ohe-form",
		"record-hemodynamic-check-form",
		"clear-pyometra-form",
		"PYO-OP-101",
		"PYO-OP-102",
		"PYO-OP-103",
		"Bella",
		"Duchess",
		"Daisy",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /pyometra-sirs-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/pyometra-sirs/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/pyometra-sirs/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/pyometra-sirs/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.PyometraOperation     `json:"operations"`
		Metrics    domain.PyometraOverviewMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal Pyometra operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?cervix_type=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/pyometra-sirs/operations?status=CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE&cervix_type=CLOSED_CERVIX_RUPTURE_RISK", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.PyometraOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "PYO-OP-101" {
		t.Errorf("expected 1 filtered operation PYO-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/pyometra-sirs/operations/PYO-OP-101 returns Bella
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/pyometra-sirs/operations/PYO-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/pyometra-sirs/operations/PYO-OP-101, got %d", recGet.Code)
	}
	var op101 domain.PyometraOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal PYO-OP-101: %v", err)
	}
	if op101.PetName != "Bella" || op101.Status != domain.PyometraStatusClosedCervixSIRSTriage || op101.UterineHornDiameterCm != 5.4 {
		t.Errorf("unexpected PYO-OP-101 state: %+v", op101)
	}

	// 4. POST /api/pyometra-sirs/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/pyometra-sirs/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/pyometra-sirs/operations creates a new operation "Sadie" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                     "CASE-PYO-104",
		"pet_name":                    "Sadie",
		"species":                     "Canine",
		"breed":                       "Bernese Mountain Dog",
		"weight_kg":                   38.2,
		"cervix_type":                 "CLOSED_CERVIX_RUPTURE_RISK",
		"uterine_horn_diameter_cm":    6.1,
		"blood_lactate_mmol_l":        5.1,
		"wbc_count_k_ul":              36.4,
		"band_neutrophil_percent":     20.0,
		"map_mm_hg":                   56,
		"pocus_free_peritoneal_fluid": false,
		"polyuria_polydipsia_present": true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/pyometra-sirs/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/pyometra-sirs/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.PyometraOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "PYO-OP-104" || created.PetName != "Sadie" || created.UterineHornDiameterCm != 6.1 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-pyoSSECh:
		if evt.Type != domain.ReunionEventType("pyometra_sirs_updated") {
			t.Errorf("expected pyometra_sirs_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for pyometra_sirs_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "pyometra_sirs_updated" {
			t.Errorf("expected pyometra_sirs_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for pyometra_sirs_updated mesh event")
	}

	// 6. POST /api/pyometra-sirs/operations/PYO-OP-101/stage-ohe transitions to RESUSCITATED_EMPIRIC_ABX_OHE_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"crystalloid_resuscitation_plan": "Normosol-R 20 mL/kg IV shock bolus + Norepinephrine 0.1 mcg/kg/min CRI",
		"empiric_bactericidal_abx_plan":  "Enrofloxacin 10 mg/kg slow IV diluted q24h + Ampicillin-Sulbactam 30 mg/kg IV q8h",
		"surgical_ohe_handling_protocol": "Emergent midline laparotomy OHE with moistened laparotomy sponge isolation and double-ligated cervical stump",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/pyometra-sirs/operations/PYO-OP-101/stage-ohe", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging OHE stabilization protocol on PYO-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.PyometraOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.PyometraStatusResuscitatedOHEStaged {
		t.Errorf("expected status %s, got %s", domain.PyometraStatusResuscitatedOHEStaged, staged.Status)
	}

	// 7. POST /api/pyometra-sirs/operations/PYO-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"uterine_horn_diameter_cm": 0.0,
		"blood_lactate_mmol_l":     1.6,
		"wbc_count_k_ul":           15.2,
		"band_neutrophil_percent":  3.0,
		"map_mm_hg":                82,
		"recorded_by":              "Dr. Hannah Lin, DACVS",
		"clinical_note":            "Post-OHE ICU telemetry: distended uterine horns excised (0.0 cm), lactate cleared to 1.6 mmol/L, band neutrophils down to 3.0%, MAP 82 mmHg.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/pyometra-sirs/operations/PYO-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording hemodynamic check on PYO-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.PyometraOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.HemodynamicChecks) != 3 || checked.UterineHornDiameterCm != 0.0 || checked.BloodLactateMmolL != 1.6 || checked.MAPMmHg != 82 {
		t.Errorf("expected 3 hemodynamic checks and updated vitals on PYO-OP-101, got %+v", checked)
	}

	// 8. POST /api/pyometra-sirs/operations/PYO-OP-101/clear-discharge transitions to POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Hannah Lin, DACVS",
		"discharge_summary": "Emergent OHE completed without peritoneal contamination, normotensive (MAP 82 mmHg), afebrile, lactate 1.6 mmol/L, resolved PU/PD, discharged on oral Amoxicillin-Clavulanate + Enrofloxacin.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/pyometra-sirs/operations/PYO-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on PYO-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.PyometraOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.PyometraStatusPostopOHECleared || cleared.DischargedBy != "Dr. Hannah Lin, DACVS" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/pyometra-sirs/operations/UNKNOWN",
		"/api/pyometra-sirs/operations/UNKNOWN/stage-ohe",
		"/api/pyometra-sirs/operations/UNKNOWN/record-check",
		"/api/pyometra-sirs/operations/UNKNOWN/clear-discharge",
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

func TestPyometraSIRSHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/pyometra-sirs/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/pyometra-sirs/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/pyometra-sirs/stream, got %d", resp.StatusCode)
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
