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
	"github.com/scottdensmore/petspotr/pkg/hemoabdomen"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestHemoabdomenHandlers_SeededCRUDPlanCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	haSSECh, unsubSSE := server.ReunionHub().Subscribe("hemoabdomen")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("hemoabdomen", "hemoabdomen-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /hemoabdomen-autotransfusion-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/hemoabdomen-autotransfusion-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /hemoabdomen-autotransfusion-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Traumatic Hemoabdomen, Autotransfusion, Centesis Non-Clotting",
		"kpi-total-ops",
		"kpi-critical-shock",
		"kpi-staged-autotransfusion",
		"kpi-cleared-hemostasis",
		"kpi-clot-neoplasia-alerts",
		"hemoabdomen-alert-banner",
		"hemoabdomen-clearance-banner",
		"HA-OP-101",
		"HA-OP-102",
		"HA-OP-103",
		"Apollo",
		"Sadie",
		"Ranger",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /hemoabdomen-autotransfusion-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/hemoabdomen-autotransfusion/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/hemoabdomen-autotransfusion/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/hemoabdomen-autotransfusion/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.HemoabdomenOperation  `json:"operations"`
		Metrics    hemoabdomen.HemoabdomenMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal hemoabdomen operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?species=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/hemoabdomen-autotransfusion/operations?status=HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL&species=CANINE_BLUNT_TRAUMA_HEPATIC", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.HemoabdomenOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "HA-OP-101" {
		t.Errorf("expected 1 filtered operation HA-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/hemoabdomen-autotransfusion/operations/HA-OP-101 returns Apollo
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/hemoabdomen-autotransfusion/operations/HA-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/hemoabdomen-autotransfusion/operations/HA-OP-101, got %d", recGet.Code)
	}
	var op101 domain.HemoabdomenOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal HA-OP-101: %v", err)
	}
	if op101.PetName != "Apollo" || op101.Status != domain.HemoabdomenStatusHemorrhagicShockCritical {
		t.Errorf("unexpected HA-OP-101 state: %+v", op101)
	}

	// 4. POST /api/hemoabdomen-autotransfusion/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/hemoabdomen-autotransfusion/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/hemoabdomen-autotransfusion/operations creates a new operation "Titan" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-HA-9404",
		"pet_name":    "Titan",
		"patient_profile": map[string]any{
			"species":                        "CANINE_BLUNT_TRAUMA_HEPATIC",
			"breed":                          "Rottweiler",
			"weight_kg":                      38.0,
			"afast_afs4_score":               4,
			"effusion_pcv_percent":           40.0,
			"peripheral_pcv_percent":         30.0,
			"mean_arterial_pressure_mmhg":    50,
			"plasma_lactate_mmol_l":          6.8,
			"effusion_non_clotting_verified": true,
			"suspected_neoplasia":            false,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/hemoabdomen-autotransfusion/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/hemoabdomen-autotransfusion/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.HemoabdomenOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "HA-OP-104" || created.PetName != "Titan" || created.Status != domain.HemoabdomenStatusHemorrhagicShockCritical {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-haSSECh:
		if evt.Type != domain.ReunionEventType("hemoabdomen_updated") {
			t.Errorf("expected hemoabdomen_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for hemoabdomen_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "hemoabdomen_updated" {
			t.Errorf("expected hemoabdomen_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for hemoabdomen_updated mesh event")
	}

	// 6. POST /api/hemoabdomen-autotransfusion/operations/HA-OP-101/stage-plan transitions to AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"species":                        "CANINE_BLUNT_TRAUMA_HEPATIC",
		"weight_kg":                      34.0,
		"afast_afs4_score":               4,
		"effusion_pcv_percent":           38.0,
		"peripheral_pcv_percent":         32.0,
		"mean_arterial_pressure_mmhg":    52,
		"plasma_lactate_mmol_l":          6.2,
		"effusion_non_clotting_verified": true,
		"suspected_neoplasia":            false,
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/hemoabdomen-autotransfusion/operations/HA-OP-101/stage-plan", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging plan on HA-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.HemoabdomenOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.HemoabdomenStatusAutotransfusionTXAStaged {
		t.Errorf("expected status %s, got %s", domain.HemoabdomenStatusAutotransfusionTXAStaged, staged.Status)
	}

	// 7. POST /api/hemoabdomen-autotransfusion/operations/HA-OP-101/hemodynamic-check records check
	checkPayload, _ := json.Marshal(map[string]any{
		"mean_arterial_pressure_mmhg": 68,
		"heart_rate_bpm":              118,
		"peripheral_pcv_percent":      33.0,
		"total_solids_gdl":            5.4,
		"plasma_lactate_mmol_l":       2.3,
		"autotransfused_volume_ml":    350.0,
		"clinician_callsign":          "DR-TRAUMA-14",
		"notes":                       "350 mL autologous shed blood reinfused via 170 µm filter; MAP stabilized at 68 mmHg.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/hemoabdomen-autotransfusion/operations/HA-OP-101/hemodynamic-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording hemodynamic check on HA-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.HemoabdomenOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.HemodynamicChecks) != 2 {
		t.Errorf("expected 2 hemodynamic checks on HA-OP-101, got %d", len(checked.HemodynamicChecks))
	}

	// 8. POST /api/hemoabdomen-autotransfusion/operations/HA-OP-101/hemostasis-clearance issues clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_map_mmhg":                   84,
		"final_peripheral_pcv_percent":     34.0,
		"final_plasma_lactate_mmol_l":      1.5,
		"surgical_hemostasis_confirmed":    true,
		"repeat_afast_free_fluid_resolved": true,
		"vet_license":                      "VET-SURG-90814",
		"summary":                          "Surgical hemostasis confirmed; repeat AFAST AFS 0/4 free fluid resolved.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/hemoabdomen-autotransfusion/operations/HA-OP-101/hemostasis-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK issuing hemostasis clearance on HA-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.HemoabdomenOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.HemoabdomenStatusHemostasisRestoredCleared || cleared.ClotPoppingNeoplasiaAlert || cleared.HemostasisClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/hemoabdomen-autotransfusion/operations/UNKNOWN",
		"/api/hemoabdomen-autotransfusion/operations/UNKNOWN/stage-plan",
		"/api/hemoabdomen-autotransfusion/operations/UNKNOWN/hemodynamic-check",
		"/api/hemoabdomen-autotransfusion/operations/UNKNOWN/hemostasis-clearance",
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

func TestHemoabdomenHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/hemoabdomen-autotransfusion/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/hemoabdomen-autotransfusion/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/hemoabdomen-autotransfusion/stream, got %d", resp.StatusCode)
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
