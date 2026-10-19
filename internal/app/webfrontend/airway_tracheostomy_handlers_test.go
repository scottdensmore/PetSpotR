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
	"github.com/scottdensmore/petspotr/pkg/airwaytracheostomy"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestAirwayTracheostomyHandlers_SeededCreateStageCareClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	atSSECh, unsubSSE := server.ReunionHub().Subscribe("airway_tracheostomy")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("airway_tracheostomy", "airway-tracheostomy-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /airway-tracheostomy-triage renders HTML with seeded operations and KPI telemetry
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/airway-tracheostomy-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /airway-tracheostomy-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis",
		"kpi-total-ops",
		"kpi-critical-stridor",
		"kpi-staged-intubation",
		"kpi-patent-extubated",
		"airway-alert-banner",
		"airway-clearance-banner",
		"status-pill",
		"AT-OP-101",
		"AT-OP-102",
		"AT-OP-103",
		"Barnaby",
		"Yukon",
		"Pippin",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /airway-tracheostomy-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/airway-tracheostomy/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/airway-tracheostomy/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/airway-tracheostomy/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.AirwayTracheostomyOperation    `json:"operations"`
		Metrics    airwaytracheostomy.AirwayCockpitMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal airway tracheostomy operations: %v", err)
	}
	if len(listResp.Operations) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(listResp.Operations))
	}

	// 2b. Filter by ?status=PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/airway-tracheostomy/operations?status=PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED&species=CANINE_BRACHYCEPHALIC_BOAS", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.AirwayTracheostomyOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "AT-OP-103" {
		t.Errorf("expected 1 filtered operation AT-OP-103, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/airway-tracheostomy/operations/AT-OP-101 returns Barnaby
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/airway-tracheostomy/operations/AT-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/airway-tracheostomy/operations/AT-OP-101, got %d", recGet.Code)
	}
	var got101 domain.AirwayTracheostomyOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &got101); err != nil {
		t.Fatalf("failed to unmarshal AT-OP-101: %v", err)
	}
	if got101.PetName != "Barnaby" || got101.Status != domain.AirwayStatusStridorCyanosisCritical {
		t.Errorf("unexpected AT-OP-101 state: %+v", got101)
	}

	// 3b. GET unknown operation returns 404
	recGet404 := httptest.NewRecorder()
	reqGet404 := httptest.NewRequest(http.MethodGet, "/api/airway-tracheostomy/operations/UNKNOWN-ID", nil)
	server.ServeHTTP(recGet404, reqGet404)
	if recGet404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown operation, got %d", recGet404.Code)
	}

	// 4. POST /api/airway-tracheostomy/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/airway-tracheostomy/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/airway-tracheostomy/operations creates "Otis" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"incident_id": "INC-AT-9004",
		"pet_name":    "Otis",
		"patient_profile": map[string]any{
			"species":                "CANINE_BRACHYCEPHALIC_BOAS",
			"breed":                  "English Bulldog",
			"weight_kg":              24.0,
			"rectal_temp_fahrenheit": 105.4,
			"spo2_percent":           85.0,
			"inspiratory_effort":     "SEVERE_PARADOXICAL_CYANOTIC",
			"laryngeal_edema_grade":  "GRADE_III_EVERTED_SACCULES_COLLAPSE",
			"gag_reflex_present":     false,
		},
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/airway-tracheostomy/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/airway-tracheostomy/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.AirwayTracheostomyOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "AT-OP-104" || created.PetName != "Otis" || created.Status != domain.AirwayStatusStridorCyanosisCritical {
		t.Errorf("unexpected created operation: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-atSSECh:
		if evt.Type != domain.ReunionEventType("airway_tracheostomy_updated") {
			t.Errorf("expected airway_tracheostomy_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for airway_tracheostomy_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "airway_tracheostomy_updated" {
			t.Errorf("expected airway_tracheostomy_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for airway_tracheostomy_updated mesh event")
	}

	// 6. POST /api/airway-tracheostomy/operations/AT-OP-104/stage-stabilization
	stagePayload, _ := json.Marshal(map[string]any{
		"rectal_temp_fahrenheit": 102.6,
		"spo2_percent":           94.0,
		"inspiratory_effort":     "MODERATE_STRIDOR_ORTHOPNEA",
		"laryngeal_edema_grade":  "GRADE_III_EVERTED_SACCULES_COLLAPSE",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/airway-tracheostomy/operations/AT-OP-104/stage-stabilization", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stage-stabilization, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.AirwayTracheostomyOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.AirwayStatusSedationDexamethasoneStaged {
		t.Errorf("expected BUTORPHANOL_DEX_INTUBATION_STAGED, got %s", staged.Status)
	}

	// 7. POST /api/airway-tracheostomy/operations/AT-OP-104/trach-care-log (reject >10s suction, accept <=10s)
	badCarePayload, _ := json.Marshal(map[string]any{
		"sterile_saline_instillation_ml": 1.5,
		"pre_oxygenation_duration_sec":   90,
		"suction_duration_max_sec":       15,
		"inner_cannula_cleaned":          true,
		"subcutaneous_emphysema_noted":   false,
		"post_suction_spo2_percent":      95.0,
		"technician_callsign":            "RVT-LEE-09",
		"notes":                          "Too long suction",
	})
	recBadCare := httptest.NewRecorder()
	reqBadCare := httptest.NewRequest(http.MethodPost, "/api/airway-tracheostomy/operations/AT-OP-104/trach-care-log", bytes.NewReader(badCarePayload))
	reqBadCare.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recBadCare, reqBadCare)
	if recBadCare.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for >10s suction duration, got %d", recBadCare.Code)
	}

	goodCarePayload, _ := json.Marshal(map[string]any{
		"sterile_saline_instillation_ml": 1.5,
		"pre_oxygenation_duration_sec":   90,
		"suction_duration_max_sec":       8,
		"inner_cannula_cleaned":          true,
		"subcutaneous_emphysema_noted":   false,
		"post_suction_spo2_percent":      96.5,
		"technician_callsign":            "RVT-LEE-09",
		"notes":                          "Aseptic inner cannula scrub and 8s suction pass after 90s 100% O2 pre-oxygenation.",
	})
	recCare := httptest.NewRecorder()
	reqCare := httptest.NewRequest(http.MethodPost, "/api/airway-tracheostomy/operations/AT-OP-104/trach-care-log", bytes.NewReader(goodCarePayload))
	reqCare.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCare, reqCare)
	if recCare.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for trach-care-log, got %d: %s", recCare.Code, recCare.Body.String())
	}

	// 8. POST /api/airway-tracheostomy/operations/AT-OP-104/extubation-clearance
	clearPayload, _ := json.Marshal(map[string]any{
		"final_spo2_on_room_air_percent":   97.5,
		"final_rectal_temp_fahrenheit":     101.0,
		"gag_swallow_reflex_vigorous":      true,
		"trach_tube_occluded_trial_passed": true,
		"harness_only_discharge_verified":  true,
		"vet_license":                      "DVM-WA-77410",
		"summary":                          "Vigorous gag reflex restored, SpO2 97.5% on room air, normothermic 101.0°F, chest harness only verified.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/airway-tracheostomy/operations/AT-OP-104/extubation-clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for extubation-clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.AirwayTracheostomyOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.AirwayStatusPatentNormothermicExtubated || cleared.CriticalAirwayAlert || cleared.ExtubationClearance == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}
}

func TestAirwayTracheostomyHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/airway-tracheostomy/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/airway-tracheostomy/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/airway-tracheostomy/stream, got %d", resp.StatusCode)
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
