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
	"github.com/scottdensmore/petspotr/pkg/anaphylaxishalo"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestAnaphylaxisHaloHandlers_SeededCRUDPlanCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	anaphylaxisSSECh, unsubSSE := server.ReunionHub().Subscribe("anaphylaxis_halo")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("anaphylaxis_halo", "anaphylaxis-halo-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /anaphylaxis-halo-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/anaphylaxis-halo-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /anaphylaxis-halo-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Anaphylaxis, Hemoabdomen Halo Sign",
		"kpi-total-ops",
		"kpi-critical-shock",
		"kpi-staged-epinephrine",
		"kpi-cleared-biphasic",
		"kpi-hepatic-alerts",
		"anaphylaxis-alert-banner",
		"anaphylaxis-clearance-banner",
		"AH-OP-101",
		"AH-OP-102",
		"AH-OP-103",
		"Waffle",
		"Bramble",
		"Clover",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /anaphylaxis-halo-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/anaphylaxis-halo/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/anaphylaxis-halo/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/anaphylaxis-halo/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.AnaphylaxisHaloOperation  `json:"operations"`
		Metrics    anaphylaxishalo.AnaphylaxisMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal anaphylaxis halo operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/anaphylaxis-halo/operations?status=ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL&species=CANINE_HYMENOPTERA_HEMOABDOMEN", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.AnaphylaxisHaloOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "AH-OP-101" {
		t.Errorf("expected 1 filtered operation AH-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/anaphylaxis-halo/operations/AH-OP-101 returns Waffle
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/anaphylaxis-halo/operations/AH-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/anaphylaxis-halo/operations/AH-OP-101, got %d", recGet.Code)
	}
	var op101 domain.AnaphylaxisHaloOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal AH-OP-101: %v", err)
	}
	if op101.PetName != "Waffle" || op101.Status != domain.AnaphylaxisStatusShockHaloCritical {
		t.Errorf("unexpected AH-OP-101 state: %+v", op101)
	}

	// 4. POST /api/anaphylaxis-halo/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/anaphylaxis-halo/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/anaphylaxis-halo/operations creates a new operation "Peanut" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-AH-9904",
		"pet_name":    "Peanut",
		"patient_profile": map[string]any{
			"species":                       "CANINE_HYMENOPTERA_HEMOABDOMEN",
			"breed":                         "French Bulldog",
			"weight_kg":                     11.5,
			"gallbladder_wall_thickness_mm": 5.8,
			"gallbladder_halo_sign_present": true,
			"mean_arterial_pressure_mmhg":   48,
			"alanine_aminotransferase_u_l":  1850,
			"packed_cell_volume_percent":    66.0,
			"prothrombin_time_seconds":      24.5,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/anaphylaxis-halo/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/anaphylaxis-halo/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.AnaphylaxisHaloOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "AH-OP-104" || created.PetName != "Peanut" || created.Status != domain.AnaphylaxisStatusShockHaloCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-anaphylaxisSSECh:
		if evt.Type != domain.ReunionEventType("anaphylaxis_halo_updated") {
			t.Errorf("expected anaphylaxis_halo_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for anaphylaxis_halo_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "anaphylaxis_halo_updated" {
			t.Errorf("expected anaphylaxis_halo_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for anaphylaxis_halo_updated mesh event")
	}

	// 6. POST /api/anaphylaxis-halo/operations/AH-OP-101/stage-plan transitions to EPINEPHRINE_IM_CRI_CRYSTALLOID_FFP_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                       "CANINE_HYMENOPTERA_HEMOABDOMEN",
		"weight_kg":                     6.8,
		"gallbladder_wall_thickness_mm": 6.4,
		"mean_arterial_pressure_mmhg":   46,
		"packed_cell_volume_percent":    68.0,
		"prothrombin_time_seconds":      28.5,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/anaphylaxis-halo/operations/AH-OP-101/stage-plan", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging plan on AH-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.AnaphylaxisHaloOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.AnaphylaxisStatusEpinephrineFFPStaged {
		t.Errorf("expected status %s, got %s", domain.AnaphylaxisStatusEpinephrineFFPStaged, staged.Status)
	}

	// 7. POST /api/anaphylaxis-halo/operations/AH-OP-101/biphasic-check records biphasic check
	checkPayload, _ := json.Marshal(map[string]any{
		"mean_arterial_pressure_mmhg":   84,
		"gallbladder_wall_thickness_mm": 2.3,
		"packed_cell_volume_percent":    47.0,
		"alanine_aminotransferase_u_l":  420,
		"biphasic_relapse_signs_noted":  false,
		"clinician_callsign":            "DR-ECC-41",
		"notes":                         "AFAST gallbladder wall edema resolved from 6.4 mm to 2.3 mm; MAP 84 mmHg off Epinephrine CRI; PCV normalized to 47%.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/anaphylaxis-halo/operations/AH-OP-101/biphasic-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording biphasic check on AH-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.AnaphylaxisHaloOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.BiphasicChecks) != 2 {
		t.Errorf("expected 2 biphasic checks on AH-OP-101, got %d", len(checked.BiphasicChecks))
	}

	// 8. POST /api/anaphylaxis-halo/operations/AH-OP-101/discharge-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"biphasic_observation_hours":    26,
		"final_map_mmhg":                88,
		"gallbladder_halo_resolved":     true,
		"coagulopathy_normalized":       true,
		"epipen_auto_injector_educated": true,
		"vet_license":                   "VET-ECC-99108",
		"summary":                       "26h biphasic observation completed without relapse; AFAST gallbladder Halo Sign resolved; PT/PCV normalized; owner educated on EpiPen auto-injector.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/anaphylaxis-halo/operations/AH-OP-101/discharge-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing discharge clearance on AH-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.AnaphylaxisHaloOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.AnaphylaxisStatusBiphasic24HrCleared || cleared.HepaticCongestionCoagulopathyAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/anaphylaxis-halo/operations/UNKNOWN",
		"/api/anaphylaxis-halo/operations/UNKNOWN/stage-plan",
		"/api/anaphylaxis-halo/operations/UNKNOWN/biphasic-check",
		"/api/anaphylaxis-halo/operations/UNKNOWN/discharge-clearance",
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

func TestAnaphylaxisHaloHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/anaphylaxis-halo/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/anaphylaxis-halo/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/anaphylaxis-halo/stream, got %d", resp.StatusCode)
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
