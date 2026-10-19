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
	"github.com/scottdensmore/petspotr/pkg/septicperitonitis"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestSepticPeritonitisHandlers_SeededCRUDPlanCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	spSSECh, unsubSSE := server.ReunionHub().Subscribe("septic_peritonitis")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("septic_peritonitis", "septic-peritonitis-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /septic-peritonitis-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/septic-peritonitis-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /septic-peritonitis-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Septic Peritonitis, Abdominocentesis Blood-Fluid Glucose/Lactate Gradient",
		"kpi-total-ops",
		"kpi-critical-septic",
		"kpi-staged-lavage",
		"kpi-cleared-jp-removed",
		"kpi-septic-gradient-alerts",
		"peritonitis-alert-banner",
		"peritonitis-clearance-banner",
		"SP-OP-101",
		"SP-OP-102",
		"SP-OP-103",
		"Baxter",
		"Nala",
		"Gumbo",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /septic-peritonitis-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/septic-peritonitis/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/septic-peritonitis/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/septic-peritonitis/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.SepticPeritonitisOperation  `json:"operations"`
		Metrics    septicperitonitis.PeritonitisMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal septic peritonitis operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/septic-peritonitis/operations?status=SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL&species=CANINE_GI_DEHISCENCE_PERFORATION", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.SepticPeritonitisOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "SP-OP-101" {
		t.Errorf("expected 1 filtered operation SP-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/septic-peritonitis/operations/SP-OP-101 returns Baxter
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/septic-peritonitis/operations/SP-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/septic-peritonitis/operations/SP-OP-101, got %d", recGet.Code)
	}
	var op101 domain.SepticPeritonitisOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal SP-OP-101: %v", err)
	}
	if op101.PetName != "Baxter" || op101.Status != domain.PeritonitisStatusSepticEffusionCritical {
		t.Errorf("unexpected SP-OP-101 state: %+v", op101)
	}

	// 4. POST /api/septic-peritonitis/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/septic-peritonitis/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/septic-peritonitis/operations creates a new operation "Roscoe" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-SP-9504",
		"pet_name":    "Roscoe",
		"patient_profile": map[string]any{
			"species":                         "CANINE_RUPTURED_PROSTATIC_ABSCESS",
			"breed":                           "German Shepherd",
			"weight_kg":                       34.0,
			"blood_glucose_mg_dl":             124.0,
			"effusion_glucose_mg_dl":          46.0,
			"blood_lactate_mmol_l":            2.2,
			"effusion_lactate_mmol_l":         6.4,
			"degenerate_neutrophils_count_ul": 42000,
			"intracellular_bacteria_seen":     true,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/septic-peritonitis/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/septic-peritonitis/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.SepticPeritonitisOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "SP-OP-104" || created.PetName != "Roscoe" || created.Status != domain.PeritonitisStatusSepticEffusionCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-spSSECh:
		if evt.Type != domain.ReunionEventType("septic_peritonitis_updated") {
			t.Errorf("expected septic_peritonitis_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for septic_peritonitis_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "septic_peritonitis_updated" {
			t.Errorf("expected septic_peritonitis_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for septic_peritonitis_updated mesh event")
	}

	// 6. POST /api/septic-peritonitis/operations/SP-OP-101/stage-plan transitions to EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                 "CANINE_GI_DEHISCENCE_PERFORATION",
		"weight_kg":               31.0,
		"blood_glucose_mg_dl":     118.0,
		"effusion_glucose_mg_dl":  42.0,
		"blood_lactate_mmol_l":    2.4,
		"effusion_lactate_mmol_l": 6.8,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/septic-peritonitis/operations/SP-OP-101/stage-plan", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging surgical plan on SP-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.SepticPeritonitisOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.PeritonitisStatusAntibioticLavageJPStaged {
		t.Errorf("expected status %s, got %s", domain.PeritonitisStatusAntibioticLavageJPStaged, staged.Status)
	}

	// 7. POST /api/septic-peritonitis/operations/SP-OP-101/drain-check records serial JP drain check
	checkPayload, _ := json.Marshal(map[string]any{
		"effusion_production_ml_kg_day":   1.8,
		"cytology_intracellular_bacteria": false,
		"degenerate_neutrophils_present":  false,
		"serum_albumin_g_dl":              2.4,
		"mean_arterial_pressure_mm_hg":    82,
		"clinician_callsign":              "DR-SURG-14",
		"notes":                           "JP drain output decreased to 1.8 mL/kg/day; negative intracellular bacteria.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/septic-peritonitis/operations/SP-OP-101/drain-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording drain check on SP-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.SepticPeritonitisOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.DrainChecks) != 2 {
		t.Errorf("expected 2 drain checks on SP-OP-101, got %d", len(checked.DrainChecks))
	}

	// 8. POST /api/septic-peritonitis/operations/SP-OP-101/drain-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_drain_output_ml_kg_day":     1.5,
		"final_cytology_bacteria_negative": true,
		"final_serum_albumin_g_dl":         2.5,
		"enteral_nutrition_tolerated":      true,
		"jp_drain_aseptically_removed":     true,
		"vet_license":                      "VET-SURG-90512",
		"summary":                          "Non-septic cytology verified; JP drains aseptically pulled.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/septic-peritonitis/operations/SP-OP-101/drain-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing drain clearance on SP-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.SepticPeritonitisOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.PeritonitisStatusNonSepticJPDrainRemoved || cleared.SepticGradientAntibioticDelayAlert || cleared.DrainClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/septic-peritonitis/operations/UNKNOWN",
		"/api/septic-peritonitis/operations/UNKNOWN/stage-plan",
		"/api/septic-peritonitis/operations/UNKNOWN/drain-check",
		"/api/septic-peritonitis/operations/UNKNOWN/drain-clearance",
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

func TestSepticPeritonitisHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/septic-peritonitis/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/septic-peritonitis/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/septic-peritonitis/stream, got %d", resp.StatusCode)
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
