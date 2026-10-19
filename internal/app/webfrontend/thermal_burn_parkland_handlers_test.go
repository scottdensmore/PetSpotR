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

func TestThermalBurnParklandHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	burnSSECh, unsubSSE := server.ReunionHub().Subscribe("thermal_burn_parkland")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("thermal_burn_parkland", "thermal-burn-parkland-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /thermal-burn-parkland-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/thermal-burn-parkland-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /thermal-burn-parkland-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Parkland Crystalloid Resuscitation Formula",
		"thermal-burn-parkland-cockpit",
		"burn-alert-banner",
		"burn-clearance-banner",
		"burn-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"burn-operations-grid",
		"burn-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-tbsa-percent",
		"active-urine-output",
		"create-burn-form",
		"stage-parkland-form",
		"record-burn-check-form",
		"clear-burn-form",
		"BURN-OP-101",
		"BURN-OP-102",
		"BURN-OP-103",
		"Ember",
		"Cinder",
		"Blaze",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /thermal-burn-parkland-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/thermal-burn-parkland/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/thermal-burn-parkland/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/thermal-burn-parkland/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.ThermalBurnOperation     `json:"operations"`
		Metrics    domain.ThermalBurnOverviewMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal Thermal Burn operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?depth_class=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/thermal-burn-parkland/operations?status=ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE&depth_class=FULL_THICKNESS_CIRCUMFERENTIAL_ESCHAR", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.ThermalBurnOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "BURN-OP-101" {
		t.Errorf("expected 1 filtered operation BURN-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/thermal-burn-parkland/operations/BURN-OP-101 returns Ember
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/thermal-burn-parkland/operations/BURN-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/thermal-burn-parkland/operations/BURN-OP-101, got %d", recGet.Code)
	}
	var op101 domain.ThermalBurnOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal BURN-OP-101: %v", err)
	}
	if op101.PetName != "Ember" || op101.Status != domain.ThermalBurnStatusAcuteTriage || op101.TBSAPercent != 30.0 {
		t.Errorf("unexpected BURN-OP-101 state: %+v", op101)
	}

	// 4. POST /api/thermal-burn-parkland/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/thermal-burn-parkland/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/thermal-burn-parkland/operations creates a new operation "Phoenix" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                         "CASE-BURN-104",
		"pet_name":                        "Phoenix",
		"species":                         "Canine",
		"breed":                           "Belgian Malinois",
		"weight_kg":                       25.0,
		"depth_class":                     "FULL_THICKNESS_CIRCUMFERENTIAL_ESCHAR",
		"tbsa_percent":                    28.0,
		"current_fluid_rate_ml_hr":        175.0,
		"urine_output_ml_kg_hr":           0.9,
		"serum_albumin_g_dl":              1.8,
		"pcv_percent":                     56.0,
		"circumferential_thoracic_eschar": true,
		"singed_vibrissae_soot_airway":    true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/thermal-burn-parkland/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/thermal-burn-parkland/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.ThermalBurnOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "BURN-OP-104" || created.PetName != "Phoenix" || created.TBSAPercent != 28.0 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-burnSSECh:
		if evt.Type != domain.ReunionEventType("thermal_burn_parkland_updated") {
			t.Errorf("expected thermal_burn_parkland_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for thermal_burn_parkland_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "thermal_burn_parkland_updated" {
			t.Errorf("expected thermal_burn_parkland_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for thermal_burn_parkland_updated mesh event")
	}

	// 6. POST /api/thermal-burn-parkland/operations/BURN-OP-101/stage-parkland transitions to PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"parkland_crystalloid_schedule":        "Parkland LRS 2400 mL/24h (First 8h at 150.0 mL/hr, Next 16h at 75.0 mL/hr); colloids withheld in first 24h",
		"silver_sulfadiazine_honey_wound_plan": "1% Silver Sulfadiazine (SSD) + sterile Manuka Honey UMF 15+ barrier q12h",
		"escharotomy_wet_debridement_plan":     "Immediate bilateral lateral thoracic escharotomy incisions through full-thickness eschar + staged debridement",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/thermal-burn-parkland/operations/BURN-OP-101/stage-parkland", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging Parkland plan on BURN-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.ThermalBurnOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.ThermalBurnStatusParklandStaged {
		t.Errorf("expected status %s, got %s", domain.ThermalBurnStatusParklandStaged, staged.Status)
	}

	// 7. POST /api/thermal-burn-parkland/operations/BURN-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"tbsa_percent":             18.0,
		"current_fluid_rate_ml_hr": 75.0,
		"urine_output_ml_kg_hr":    1.7,
		"serum_albumin_g_dl":       2.5,
		"pcv_percent":              43.0,
		"recorded_by":              "Dr. Marcus Thorne, DACVS / Burn Reconstruction",
		"clinical_note":            "Post-escharotomy & Parkland transition: thoracic compliance restored, Urine Output 1.7 mL/kg/hr, Albumin 2.5 g/dL, residual TBSA 18.0%.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/thermal-burn-parkland/operations/BURN-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording burn check on BURN-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.ThermalBurnOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.BurnChecks) != 3 || checked.TBSAPercent != 18.0 || checked.UrineOutputMLKgHr != 1.7 {
		t.Errorf("expected 3 burn checks and updated TBSA%%/UOP on BURN-OP-101, got %+v", checked)
	}

	// 8. POST /api/thermal-burn-parkland/operations/BURN-OP-101/clear-discharge transitions to GRANULATION_BED_NORMOALBUMINEMIC_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Marcus Thorne, DACVS / Burn Reconstruction",
		"discharge_summary": "Healthy granulation bed established over residual 18.0% TBSA, Urine Output 1.7 mL/kg/hr, Serum Albumin 2.5 g/dL, PCV 43.0%, cleared for reconstructive step-down.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/thermal-burn-parkland/operations/BURN-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on BURN-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.ThermalBurnOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.ThermalBurnStatusGranulationCleared || cleared.DischargedBy != "Dr. Marcus Thorne, DACVS / Burn Reconstruction" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/thermal-burn-parkland/operations/UNKNOWN",
		"/api/thermal-burn-parkland/operations/UNKNOWN/stage-parkland",
		"/api/thermal-burn-parkland/operations/UNKNOWN/record-check",
		"/api/thermal-burn-parkland/operations/UNKNOWN/clear-discharge",
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

func TestThermalBurnParklandHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/thermal-burn-parkland/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/thermal-burn-parkland/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/thermal-burn-parkland/stream, got %d", resp.StatusCode)
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
