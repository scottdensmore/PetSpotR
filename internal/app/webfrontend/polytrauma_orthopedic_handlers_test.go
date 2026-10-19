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

func TestPolytraumaOrthoHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	orthoSSECh, unsubSSE := server.ReunionHub().Subscribe("polytrauma_ortho")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("polytrauma_ortho", "polytrauma-ortho-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /polytrauma-orthopedic-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/polytrauma-orthopedic-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /polytrauma-orthopedic-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Bite Wound, Vehicular Polytrauma",
		"polytrauma-orthopedic-cockpit",
		"ortho-alert-banner",
		"ortho-clearance-banner",
		"ortho-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"ortho-operations-grid",
		"ortho-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-att-score",
		"active-lavage-volume",
		"create-ortho-form",
		"stage-fixation-form",
		"record-wound-check-form",
		"clear-ortho-form",
		"ORTHO-OP-101",
		"ORTHO-OP-102",
		"ORTHO-OP-103",
		"Ranger",
		"Peanut",
		"Toby",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /polytrauma-orthopedic-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/polytrauma-orthopedic/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/polytrauma-orthopedic/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/polytrauma-orthopedic/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.PolytraumaOrthoOperation     `json:"operations"`
		Metrics    domain.PolytraumaOrthoOverviewMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal Polytrauma Ortho operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?gustilo_class=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/polytrauma-orthopedic/operations?status=ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE&gustilo_class=GUSTILO_GRADE_III_SHEARING_DEGLOVING", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.PolytraumaOrthoOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "ORTHO-OP-101" {
		t.Errorf("expected 1 filtered operation ORTHO-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/polytrauma-orthopedic/operations/ORTHO-OP-101 returns Ranger
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/polytrauma-orthopedic/operations/ORTHO-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/polytrauma-orthopedic/operations/ORTHO-OP-101, got %d", recGet.Code)
	}
	var op101 domain.PolytraumaOrthoOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal ORTHO-OP-101: %v", err)
	}
	if op101.PetName != "Ranger" || op101.Status != domain.OrthoStatusAcuteOpenFractureTriage || op101.AnimalTraumaTriageScore != 9 {
		t.Errorf("unexpected ORTHO-OP-101 state: %+v", op101)
	}

	// 4. POST /api/polytrauma-orthopedic/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/polytrauma-orthopedic/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/polytrauma-orthopedic/operations creates a new operation "Bandit" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                             "CASE-ORTHO-104",
		"pet_name":                            "Bandit",
		"species":                             "Canine",
		"breed":                               "Australian Shepherd",
		"weight_kg":                           22.5,
		"gustilo_class":                       "GUSTILO_GRADE_III_SHEARING_DEGLOVING",
		"animal_trauma_triage_score":          8,
		"lavage_volume_ml":                    1800,
		"npwt_vacuum_mmhg":                    0,
		"blood_lactate_mmol_l":                3.9,
		"distal_toe_temp_delta_c":             1.6,
		"exposed_cortical_bone_tendon":        true,
		"thoracic_wall_penetration_suspected": false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/polytrauma-orthopedic/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/polytrauma-orthopedic/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.PolytraumaOrthoOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "ORTHO-OP-104" || created.PetName != "Bandit" || created.AnimalTraumaTriageScore != 8 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-orthoSSECh:
		if evt.Type != domain.ReunionEventType("polytrauma_ortho_updated") {
			t.Errorf("expected polytrauma_ortho_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for polytrauma_ortho_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "polytrauma_ortho_updated" {
			t.Errorf("expected polytrauma_ortho_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for polytrauma_ortho_updated mesh event")
	}

	// 6. POST /api/polytrauma-orthopedic/operations/ORTHO-OP-101/stage-fixation transitions to LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"sterile_water_soluble_gel_clip_plan":   "Pack wound with sterile K-Y water-soluble gel before wide #40 surgical clip",
		"copious_lavage_35_ml_syringe_18g_plan": "Copious 7-8 PSI warm isotonic LRS lavage (2,500 mL via 1 L bag at 300 mmHg + 18G needle)",
		"npwt_vacuum_or_tie_over_dressing_plan": "NPWT Vacuum-Assisted Closure at -125 mmHg continuous suction over polyurethane foam",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/polytrauma-orthopedic/operations/ORTHO-OP-101/stage-fixation", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging fixation on ORTHO-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.PolytraumaOrthoOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.OrthoStatusNPWTSplintStaged {
		t.Errorf("expected status %s, got %s", domain.OrthoStatusNPWTSplintStaged, staged.Status)
	}

	// 7. POST /api/polytrauma-orthopedic/operations/ORTHO-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"animal_trauma_triage_score": 3,
		"lavage_volume_ml":           3000,
		"npwt_vacuum_mmhg":           -125,
		"blood_lactate_mmol_l":       1.4,
		"distal_toe_temp_delta_c":    0.3,
		"recorded_by":                "Dr. Caleb Thorne, DACVS (Orthopedics)",
		"clinical_note":              "72h post-NPWT (-125 mmHg) & transarticular ESF: robust granulation bed covering exposed talus, ATT score 3, Lavage 3000 mL.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/polytrauma-orthopedic/operations/ORTHO-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording wound check on ORTHO-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.PolytraumaOrthoOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.WoundChecks) != 3 || checked.AnimalTraumaTriageScore != 3 || checked.LavageVolumeML != 3000 {
		t.Errorf("expected 3 wound checks and updated ATT/Lavage on ORTHO-OP-101, got %+v", checked)
	}

	// 8. POST /api/polytrauma-orthopedic/operations/ORTHO-OP-101/clear-discharge transitions to GRANULATION_FIXATION_STABLE_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Caleb Thorne, DACVS (Orthopedics)",
		"discharge_summary": "Healthy granulation bed covering exposed cortical bone, transarticular ESF stable, ATT score 3, Lactate 1.4 mmol/L, cleared for orthopedic ward.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/polytrauma-orthopedic/operations/ORTHO-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on ORTHO-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.PolytraumaOrthoOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.OrthoStatusGranulationFixationCleared || cleared.DischargedBy != "Dr. Caleb Thorne, DACVS (Orthopedics)" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/polytrauma-orthopedic/operations/UNKNOWN",
		"/api/polytrauma-orthopedic/operations/UNKNOWN/stage-fixation",
		"/api/polytrauma-orthopedic/operations/UNKNOWN/record-check",
		"/api/polytrauma-orthopedic/operations/UNKNOWN/clear-discharge",
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

func TestPolytraumaOrthoHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/polytrauma-orthopedic/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/polytrauma-orthopedic/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/polytrauma-orthopedic/stream, got %d", resp.StatusCode)
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
