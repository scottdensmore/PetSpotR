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
	"github.com/scottdensmore/petspotr/pkg/pericardialtamponade"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestPericardialTamponadeHandlers_SeededCRUDPlanTapCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	tamponadeSSECh, unsubSSE := server.ReunionHub().Subscribe("pericardial_tamponade")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("pericardial_tamponade", "pericardial-tamponade-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /pericardial-tamponade-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/pericardial-tamponade-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /pericardial-tamponade-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Pericardial Effusion, Cardiac Tamponade",
		"kpi-total-ops",
		"kpi-critical-tamponade",
		"kpi-staged-centesis",
		"kpi-cleared-decompressed",
		"kpi-furosemide-coronary-alerts",
		"tamponade-alert-banner",
		"tamponade-clearance-banner",
		"PT-OP-101",
		"PT-OP-102",
		"PT-OP-103",
		"Brutus",
		"Mugsy",
		"Samson",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /pericardial-tamponade-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/pericardial-tamponade/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/pericardial-tamponade/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/pericardial-tamponade/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.PericardialTamponadeOperation `json:"operations"`
		Metrics    pericardialtamponade.TamponadeMetrics  `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal pericardial tamponade operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/pericardial-tamponade/operations?status=TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL&species=CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.PericardialTamponadeOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "PT-OP-101" {
		t.Errorf("expected 1 filtered operation PT-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/pericardial-tamponade/operations/PT-OP-101 returns Brutus
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/pericardial-tamponade/operations/PT-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/pericardial-tamponade/operations/PT-OP-101, got %d", recGet.Code)
	}
	var op101 domain.PericardialTamponadeOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal PT-OP-101: %v", err)
	}
	if op101.PetName != "Brutus" || op101.Status != domain.TamponadeStatusRADiastolicCollapseCritical {
		t.Errorf("unexpected PT-OP-101 state: %+v", op101)
	}

	// 4. POST /api/pericardial-tamponade/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/pericardial-tamponade/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/pericardial-tamponade/operations creates a new operation "Goliath" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-PT-9924",
		"pet_name":    "Goliath",
		"patient_profile": map[string]any{
			"species":                         "CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA",
			"breed":                           "German Shepherd",
			"weight_kg":                       36.0,
			"heart_rate_bpm":                  190,
			"mean_arterial_pressure_mmhg":     50,
			"right_atrial_diastolic_collapse": true,
			"electrical_alternans_present":    true,
			"pulsus_paradoxus_mmhg":           20,
			"effusion_pcv_percent":            30.0,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/pericardial-tamponade/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/pericardial-tamponade/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.PericardialTamponadeOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "PT-OP-104" || created.PetName != "Goliath" || created.Status != domain.TamponadeStatusRADiastolicCollapseCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-tamponadeSSECh:
		if evt.Type != domain.ReunionEventType("pericardial_tamponade_updated") {
			t.Errorf("expected pericardial_tamponade_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for pericardial_tamponade_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "pericardial_tamponade_updated" {
			t.Errorf("expected pericardial_tamponade_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for pericardial_tamponade_updated mesh event")
	}

	// 6. POST /api/pericardial-tamponade/operations/PT-OP-101/stage-plan transitions to RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                     "CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA",
		"weight_kg":                   33.0,
		"heart_rate_bpm":              185,
		"mean_arterial_pressure_mmhg": 52,
		"pulsus_paradoxus_mmhg":       18,
		"effusion_pcv_percent":        28.0,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/pericardial-tamponade/operations/PT-OP-101/stage-plan", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging plan on PT-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.PericardialTamponadeOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.TamponadeStatusRightICS45TapStaged {
		t.Errorf("expected status %s, got %s", domain.TamponadeStatusRightICS45TapStaged, staged.Status)
	}

	// 7. POST /api/pericardial-tamponade/operations/PT-OP-101/tap-check records tap check
	checkPayload, _ := json.Marshal(map[string]any{
		"volume_aspirated_ml":           260.0,
		"sample_non_clotting_verified":  true,
		"ventricular_arrhythmias_noted": false,
		"post_tap_map_mmhg":             88,
		"post_tap_heart_rate_bpm":       108,
		"clinician_callsign":            "DR-CARDIO-11",
		"notes":                         "260 mL non-clotting port-wine effusion aspirated via right 4th-5th ICS; RA diastolic collapse and electrical alternans resolved.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/pericardial-tamponade/operations/PT-OP-101/tap-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording tap check on PT-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.PericardialTamponadeOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.TapChecks) != 2 {
		t.Errorf("expected 2 tap checks on PT-OP-101, got %d", len(checked.TapChecks))
	}

	// 8. POST /api/pericardial-tamponade/operations/PT-OP-101/discharge-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_map_mmhg":                          94,
		"right_atrial_collapse_resolved":          true,
		"repeat_tfast_24hr_no_reaccumulation":     true,
		"sinus_rhythm_no_vpcs":                    true,
		"yunnan_baiyao_or_oncology_plan_educated": true,
		"vet_license":                             "VET-CARDIO-99210",
		"summary":                                 "Right 4th-5th ICS pericardiocentesis decompression complete; 24h repeat TFAST negative for reaccumulation; sinus rhythm without VPCs.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/pericardial-tamponade/operations/PT-OP-101/discharge-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing discharge clearance on PT-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.PericardialTamponadeOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.TamponadeStatusDecompressedSinusCleared || cleared.FurosemideLeftCorpnaryHazardAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/pericardial-tamponade/operations/UNKNOWN",
		"/api/pericardial-tamponade/operations/UNKNOWN/stage-plan",
		"/api/pericardial-tamponade/operations/UNKNOWN/tap-check",
		"/api/pericardial-tamponade/operations/UNKNOWN/discharge-clearance",
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

func TestPericardialTamponadeHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/pericardial-tamponade/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/pericardial-tamponade/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/pericardial-tamponade/stream, got %d", resp.StatusCode)
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
