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
	"github.com/scottdensmore/petspotr/pkg/pneumothorax"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestPneumothoraxHandlers_SeededCRUDPlanCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	pneumothoraxSSECh, unsubSSE := server.ReunionHub().Subscribe("pneumothorax")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("pneumothorax", "pneumothorax-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /pneumothorax-thoracocentesis-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/pneumothorax-thoracocentesis-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /pneumothorax-thoracocentesis-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Tension Pneumothorax, Flail Chest, 7th–9th ICS Thoracocentesis",
		"kpi-total-ops",
		"kpi-critical-tension",
		"kpi-staged-thoracocentesis",
		"kpi-cleared-pleural-seal",
		"kpi-tension-radiograph-alerts",
		"pneumothorax-alert-banner",
		"pneumothorax-clearance-banner",
		"PTX-OP-101",
		"PTX-OP-102",
		"PTX-OP-103",
		"Maverick",
		"Bandit",
		"Tango",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /pneumothorax-thoracocentesis-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/pneumothorax-thoracocentesis/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/pneumothorax-thoracocentesis/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/pneumothorax-thoracocentesis/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.PneumothoraxOperation   `json:"operations"`
		Metrics    pneumothorax.PneumothoraxMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal pneumothorax operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/pneumothorax-thoracocentesis/operations?status=TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL&species=CANINE_SPONTANEOUS_BULLAE", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.PneumothoraxOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "PTX-OP-101" {
		t.Errorf("expected 1 filtered operation PTX-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/pneumothorax-thoracocentesis/operations/PTX-OP-101 returns Maverick
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/pneumothorax-thoracocentesis/operations/PTX-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/pneumothorax-thoracocentesis/operations/PTX-OP-101, got %d", recGet.Code)
	}
	var op101 domain.PneumothoraxOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal PTX-OP-101: %v", err)
	}
	if op101.PetName != "Maverick" || op101.Status != domain.PneumothoraxStatusTensionAbsentGlideCritical {
		t.Errorf("unexpected PTX-OP-101 state: %+v", op101)
	}

	// 4. POST /api/pneumothorax-thoracocentesis/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/pneumothorax-thoracocentesis/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/pneumothorax-thoracocentesis/operations creates a new operation "Goose" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-PTX-9914",
		"pet_name":    "Goose",
		"patient_profile": map[string]any{
			"species":                        "CANINE_SPONTANEOUS_BULLAE",
			"breed":                          "Alaskan Malamute",
			"weight_kg":                      34.0,
			"respiratory_rate_bpm":           68,
			"spo2_percent":                   85.0,
			"tfast_glide_sign_present":       false,
			"cumulative_air_aspirated_ml_kg": 54.0,
			"tap_count_last_24hr":            3,
			"open_sucking_chest_wound":       false,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/pneumothorax-thoracocentesis/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/pneumothorax-thoracocentesis/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.PneumothoraxOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "PTX-OP-104" || created.PetName != "Goose" || created.Status != domain.PneumothoraxStatusTensionAbsentGlideCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-pneumothoraxSSECh:
		if evt.Type != domain.ReunionEventType("pneumothorax_updated") {
			t.Errorf("expected pneumothorax_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for pneumothorax_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "pneumothorax_updated" {
			t.Errorf("expected pneumothorax_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for pneumothorax_updated mesh event")
	}

	// 6. POST /api/pneumothorax-thoracocentesis/operations/PTX-OP-101/stage-plan transitions to ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                        "CANINE_SPONTANEOUS_BULLAE",
		"weight_kg":                      25.0,
		"respiratory_rate_bpm":           48,
		"spo2_percent":                   92.0,
		"tfast_glide_sign_present":       true,
		"cumulative_air_aspirated_ml_kg": 62.0,
		"tap_count_last_24hr":            3,
		"open_sucking_chest_wound":       false,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/pneumothorax-thoracocentesis/operations/PTX-OP-101/stage-plan", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging plan on PTX-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.PneumothoraxOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.PneumothoraxStatusThoracocentesisTubeStaged {
		t.Errorf("expected status %s, got %s", domain.PneumothoraxStatusThoracocentesisTubeStaged, staged.Status)
	}

	// 7. POST /api/pneumothorax-thoracocentesis/operations/PTX-OP-101/drainage-check records drainage check
	checkPayload, _ := json.Marshal(map[string]any{
		"air_aspirated_rate_ml_kg_hr":    0.0,
		"fluid_aspirated_rate_ml_kg_day": 1.4,
		"tfast_glide_sign_restored":      true,
		"spo2_percent":                   97.0,
		"subcutaneous_emphysema_present": false,
		"clinician_callsign":             "DR-PULM-11",
		"notes":                          "Zero air production on -15 cmH2O continuous suction; bilateral TFAST glide sign restored.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/pneumothorax-thoracocentesis/operations/PTX-OP-101/drainage-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording drainage check on PTX-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.PneumothoraxOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.DrainageChecks) != 2 {
		t.Errorf("expected 2 drainage checks on PTX-OP-101, got %d", len(checked.DrainageChecks))
	}

	// 8. POST /api/pneumothorax-thoracocentesis/operations/PTX-OP-101/removal-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"zero_air_production_hours":      26,
		"final_fluid_output_ml_kg_day":   1.2,
		"bilateral_glide_sign_confirmed": true,
		"room_air_spo2_percent":          97.5,
		"chest_tube_aseptically_removed": true,
		"vet_license":                    "VET-PULM-99140",
		"summary":                        "Zero air production for 26 hours; fluid output 1.2 mL/kg/day; bilateral TFAST glide sign confirmed; chest tube aseptically removed.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/pneumothorax-thoracocentesis/operations/PTX-OP-101/removal-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing removal clearance on PTX-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.PneumothoraxOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.PneumothoraxStatusPleuralSealTubeRemoved || cleared.TensionRadiographDelayAlert || cleared.RemovalClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/pneumothorax-thoracocentesis/operations/UNKNOWN",
		"/api/pneumothorax-thoracocentesis/operations/UNKNOWN/stage-plan",
		"/api/pneumothorax-thoracocentesis/operations/UNKNOWN/drainage-check",
		"/api/pneumothorax-thoracocentesis/operations/UNKNOWN/removal-clearance",
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

func TestPneumothoraxHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/pneumothorax-thoracocentesis/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/pneumothorax-thoracocentesis/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/pneumothorax-thoracocentesis/stream, got %d", resp.StatusCode)
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
