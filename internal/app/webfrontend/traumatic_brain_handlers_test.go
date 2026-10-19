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
	"github.com/scottdensmore/petspotr/pkg/traumaticbrain"
)

func TestTraumaticBrainHandlers_SeededCRUDPlanCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	tbiSSECh, unsubSSE := server.ReunionHub().Subscribe("traumatic_brain")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("traumatic_brain", "traumatic-brain-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /traumatic-brain-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/traumatic-brain-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /traumatic-brain-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Traumatic Brain Injury (TBI), Modified Glasgow Coma Scale (MGCS)",
		"kpi-total-ops",
		"kpi-cushing-critical",
		"kpi-staged-hts-mannitol",
		"kpi-cleared-mgcs",
		"kpi-cushing-steroid-alerts",
		"tbi-alert-banner",
		"tbi-clearance-banner",
		"TBI-OP-101",
		"TBI-OP-102",
		"TBI-OP-103",
		"Atlas",
		"Milo",
		"Sierra",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /traumatic-brain-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/traumatic-brain/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/traumatic-brain/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/traumatic-brain/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.TraumaticBrainOperation `json:"operations"`
		Metrics    traumaticbrain.TBIMetrics        `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal traumatic brain operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/traumatic-brain/operations?status=CUSHING_REFLEX_ICP_HERNIATION_CRITICAL&species=CANINE_BLUNT_VEHICULAR_TBI", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.TraumaticBrainOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "TBI-OP-101" {
		t.Errorf("expected 1 filtered operation TBI-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/traumatic-brain/operations/TBI-OP-101 returns Atlas
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/traumatic-brain/operations/TBI-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/traumatic-brain/operations/TBI-OP-101, got %d", recGet.Code)
	}
	var op101 domain.TraumaticBrainOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal TBI-OP-101: %v", err)
	}
	if op101.PetName != "Atlas" || op101.Status != domain.TBIStatusCushingHerniationCritical {
		t.Errorf("unexpected TBI-OP-101 state: %+v", op101)
	}

	// 4. POST /api/traumatic-brain/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/traumatic-brain/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/traumatic-brain/operations creates a new operation "Zephyr" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-TBI-9804",
		"pet_name":    "Zephyr",
		"patient_profile": map[string]any{
			"species":                     "CANINE_HIGH_VELOCITY_CRANIAL",
			"breed":                       "Belgian Malinois",
			"weight_kg":                   29.0,
			"modified_glasgow_coma_score": 9,
			"mean_arterial_pressure_mmhg": 146,
			"heart_rate_bpm":              48,
			"end_tidal_co2_mmhg":          46.0,
			"pupillary_status":            "ANISOCORIA_UNILATERAL_MYDRIASIS",
			"cushing_reflex_present":      true,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/traumatic-brain/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/traumatic-brain/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.TraumaticBrainOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "TBI-OP-104" || created.PetName != "Zephyr" || created.Status != domain.TBIStatusCushingHerniationCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-tbiSSECh:
		if evt.Type != domain.ReunionEventType("traumatic_brain_updated") {
			t.Errorf("expected traumatic_brain_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for traumatic_brain_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "traumatic_brain_updated" {
			t.Errorf("expected traumatic_brain_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for traumatic_brain_updated mesh event")
	}

	// 6. POST /api/traumatic-brain/operations/TBI-OP-101/stage-plan transitions to HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                     "CANINE_BLUNT_VEHICULAR_TBI",
		"weight_kg":                   42.0,
		"modified_glasgow_coma_score": 12,
		"mean_arterial_pressure_mmhg": 104,
		"heart_rate_bpm":              82,
		"end_tidal_co2_mmhg":          38.0,
		"pupillary_status":            "MIOTIC_PINPOINT_BILATERAL",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/traumatic-brain/operations/TBI-OP-101/stage-plan", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging plan on TBI-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.TraumaticBrainOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.TBIStatusHypertonicMannitolStaged {
		t.Errorf("expected status %s, got %s", domain.TBIStatusHypertonicMannitolStaged, staged.Status)
	}

	// 7. POST /api/traumatic-brain/operations/TBI-OP-101/neuro-check records serial check
	checkPayload, _ := json.Marshal(map[string]any{
		"modified_glasgow_coma_score": 15,
		"mean_arterial_pressure_mmhg": 96,
		"heart_rate_bpm":              88,
		"end_tidal_co2_mmhg":          37.0,
		"pupillary_status":            "RESPONSIVE_ISOCORIC",
		"clinician_callsign":          "DR-NEURO-11",
		"notes":                       "Cushing's reflex resolved; MGCS improved to 15/18; pupils isocoric.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/traumatic-brain/operations/TBI-OP-101/neuro-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording neuro check on TBI-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.TraumaticBrainOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.NeuroChecks) != 2 {
		t.Errorf("expected 2 neuro checks on TBI-OP-101, got %d", len(checked.NeuroChecks))
	}

	// 8. POST /api/traumatic-brain/operations/TBI-OP-101/discharge-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_modified_glasgow_score":    17,
		"final_map_mmhg":                  96,
		"pupils_isocoric_responsive":      true,
		"gag_swallow_reflex_intact":       true,
		"ambulatory_normocapnic_verified": true,
		"vet_license":                     "VET-NEURO-98104",
		"summary":                         "MGCS recovered to 17/18; normotensive MAP 96 mmHg; responsive isocoric pupils; intact gag/swallow reflex.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/traumatic-brain/operations/TBI-OP-101/discharge-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing discharge clearance on TBI-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.TraumaticBrainOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.TBIStatusMGCSRecoveredNormocapnicCleared || cleared.CushingHerniationSteroidAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/traumatic-brain/operations/UNKNOWN",
		"/api/traumatic-brain/operations/UNKNOWN/stage-plan",
		"/api/traumatic-brain/operations/UNKNOWN/neuro-check",
		"/api/traumatic-brain/operations/UNKNOWN/discharge-clearance",
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

func TestTraumaticBrainHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/traumatic-brain/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/traumatic-brain/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/traumatic-brain/stream, got %d", resp.StatusCode)
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
