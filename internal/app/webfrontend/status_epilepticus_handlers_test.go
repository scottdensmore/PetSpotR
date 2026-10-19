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
	"github.com/scottdensmore/petspotr/pkg/statusepilepticus"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestStatusEpilepticusHandlers_SeededCRUDRegimenCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	seSSECh, unsubSSE := server.ReunionHub().Subscribe("status_epilepticus")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("status_epilepticus", "status-epilepticus-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /status-epilepticus-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/status-epilepticus-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /status-epilepticus-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Status Epilepticus, Cluster Seizures, Bromethalin/Metaldehyde Neuro-Tremor",
		"kpi-total-ops",
		"kpi-critical-ictal",
		"kpi-staged-cri",
		"kpi-cleared-normothermic",
		"kpi-hyperthermia-alerts",
		"epilepticus-alert-banner",
		"epilepticus-clearance-banner",
		"SE-OP-101",
		"SE-OP-102",
		"SE-OP-103",
		"Koda",
		"Mochi",
		"Bowie",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /status-epilepticus-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/status-epilepticus/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/status-epilepticus/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/status-epilepticus/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.StatusEpilepticusOperation  `json:"operations"`
		Metrics    statusepilepticus.EpilepticusMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal status epilepticus operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?neuro_etiology=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/status-epilepticus/operations?status=ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL&neuro_etiology=IDIOPATHIC_STATUS_EPILEPTICUS", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.StatusEpilepticusOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "SE-OP-101" {
		t.Errorf("expected 1 filtered operation SE-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/status-epilepticus/operations/SE-OP-101 returns Koda
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/status-epilepticus/operations/SE-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/status-epilepticus/operations/SE-OP-101, got %d", recGet.Code)
	}
	var op101 domain.StatusEpilepticusOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal SE-OP-101: %v", err)
	}
	if op101.PetName != "Koda" || op101.Status != domain.EpilepticusStatusActiveIctalCritical {
		t.Errorf("unexpected SE-OP-101 state: %+v", op101)
	}

	// 4. POST /api/status-epilepticus/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/status-epilepticus/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/status-epilepticus/operations creates a new operation "Nova" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-SE-9204",
		"pet_name":    "Nova",
		"patient_profile": map[string]any{
			"species":                     "CANINE",
			"breed":                       "Belgian Malinois",
			"weight_kg":                   26.0,
			"neuro_etiology":              "BROMETHALIN_CEREBRAL_EDEMA",
			"continuous_ictal_minutes":    12,
			"cluster_count_24hr":          4,
			"rectal_temp_fahrenheit":      105.8,
			"blood_glucose_mg_dl":         58.0,
			"modified_glasgow_coma_score": 10,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/status-epilepticus/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/status-epilepticus/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.StatusEpilepticusOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "SE-OP-104" || created.PetName != "Nova" || created.Status != domain.EpilepticusStatusActiveIctalCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-seSSECh:
		if evt.Type != domain.ReunionEventType("status_epilepticus_updated") {
			t.Errorf("expected status_epilepticus_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for status_epilepticus_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "status_epilepticus_updated" {
			t.Errorf("expected status_epilepticus_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for status_epilepticus_updated mesh event")
	}

	// 6. POST /api/status-epilepticus/operations/SE-OP-101/stage-regimen transitions to MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                     "CANINE",
		"weight_kg":                   24.0,
		"neuro_etiology":              "IDIOPATHIC_STATUS_EPILEPTICUS",
		"continuous_ictal_minutes":    14,
		"cluster_count_24hr":          5,
		"rectal_temp_fahrenheit":      105.2,
		"blood_glucose_mg_dl":         84.0,
		"modified_glasgow_coma_score": 11,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/status-epilepticus/operations/SE-OP-101/stage-regimen", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging regimen on SE-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.StatusEpilepticusOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.EpilepticusStatusAnticonvulsantCRIStaged {
		t.Errorf("expected status %s, got %s", domain.EpilepticusStatusAnticonvulsantCRIStaged, staged.Status)
	}

	// 7. POST /api/status-epilepticus/operations/SE-OP-101/neuro-check records serial neuro check
	checkPayload, _ := json.Marshal(map[string]any{
		"seizure_activity_observed":   false,
		"rectal_temp_fahrenheit":      101.6,
		"modified_glasgow_coma_score": 16,
		"pupillary_light_reflex":      "NORMAL_BILATERAL",
		"blood_glucose_mg_dl":         96.0,
		"clinician_callsign":          "DR-NEURO-41",
		"notes":                       "Normothermic after stopping active cooling at 103.0°F; seizure-free on Keppra.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/status-epilepticus/operations/SE-OP-101/neuro-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording neuro check on SE-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.StatusEpilepticusOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.NeuroChecks) != 2 {
		t.Errorf("expected 2 neuro checks on SE-OP-101, got %d", len(checked.NeuroChecks))
	}

	// 8. POST /api/status-epilepticus/operations/SE-OP-101/discharge-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"seizure_free_hours":                    26,
		"final_modified_glasgow_score":          18,
		"final_rectal_temp_fahrenheit":          101.2,
		"ambulatory_menace_intact":              true,
		"rescue_intranasal_midazolam_dispensed": true,
		"vet_license":                           "VET-NEURO-90411",
		"summary":                               "26h seizure-free normothermic recovery.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/status-epilepticus/operations/SE-OP-101/discharge-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing discharge clearance on SE-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.StatusEpilepticusOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.EpilepticusStatusSeizureFreeNormothermic || cleared.NeurogenicHyperthermiaAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/status-epilepticus/operations/UNKNOWN",
		"/api/status-epilepticus/operations/UNKNOWN/stage-regimen",
		"/api/status-epilepticus/operations/UNKNOWN/neuro-check",
		"/api/status-epilepticus/operations/UNKNOWN/discharge-clearance",
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

func TestStatusEpilepticusHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/status-epilepticus/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/status-epilepticus/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/status-epilepticus/stream, got %d", resp.StatusCode)
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
