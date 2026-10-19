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

func TestPancreatitisCPLHandlers_SeededCRUDStageCheckClearanceAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	pancreatitisSSECh, unsubSSE := server.ReunionHub().Subscribe("pancreatitis_cpl")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("pancreatitis_cpl", "pancreatitis-cpl-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /pancreatitis-cpl-triage renders HTML with seeded operations and required DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/pancreatitis-cpl-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /pancreatitis-cpl-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"pancreatitis-cpl-cockpit",
		"pancreatitis-alert-banner",
		"pancreatitis-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"pancreatitis-operations-grid",
		"pancreatitis-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-spec-pl",
		"active-enteral-rer",
		"stage-nutrition-form",
		"stage-nutrition-btn",
		"record-enteral-check-form",
		"enteral-checks-list",
		"clear-pancreatitis-form",
		"pancreatitis-clearance-banner",
		"PANC-OP-101",
		"PANC-OP-102",
		"PANC-OP-103",
		"Schnitzel",
		"Miso",
		"Biscuit",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /pancreatitis-cpl-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/pancreatitis-cpl/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/pancreatitis-cpl/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/pancreatitis-cpl/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.PancreatitisOperation     `json:"operations"`
		Metrics    domain.PancreatitisOverviewMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal pancreatitis operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status=ACUTE_NECROTIZING_PANCREATITIS_TRIAGE
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/pancreatitis-cpl/operations?status=ACUTE_NECROTIZING_PANCREATITIS_TRIAGE", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.PancreatitisOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "PANC-OP-101" {
		t.Errorf("expected 1 filtered operation PANC-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/pancreatitis-cpl/operations/PANC-OP-101 returns Schnitzel
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/pancreatitis-cpl/operations/PANC-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/pancreatitis-cpl/operations/PANC-OP-101, got %d", recGet.Code)
	}
	var op101 domain.PancreatitisOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal PANC-OP-101: %v", err)
	}
	if op101.PetName != "Schnitzel" || op101.SpecPLUgL != 940 || op101.Status != domain.PancreatitisStatusAcuteNecrotizingTriage {
		t.Errorf("unexpected PANC-OP-101 state: %+v", op101)
	}

	// 4. POST /api/pancreatitis-cpl/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/pancreatitis-cpl/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/pancreatitis-cpl/operations creates a new operation "Waffles" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                        "CASE-PANC-104",
		"pet_name":                       "Waffles",
		"species":                        "Canine",
		"breed":                          "Cavalier King Charles Spaniel",
		"weight_kg":                      7.4,
		"severity_class":                 "SEVERE_NECROTIZING_SIRS_MODS",
		"spec_pl_ug_l":                   780,
		"modified_glasgow_pain_score":    13,
		"enteral_rer_percent":            0,
		"pocus_hypoechoic_pancreas":      true,
		"hyperechoic_peripancreatic_fat": true,
		"prayer_position_present":        true,
		"feline_triaditis_suspected":     false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/pancreatitis-cpl/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/pancreatitis-cpl/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.PancreatitisOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "PANC-OP-104" || created.PetName != "Waffles" || created.Status != domain.PancreatitisStatusAcuteNecrotizingTriage {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-pancreatitisSSECh:
		if evt.Type != domain.ReunionEventType("pancreatitis_cpl_updated") {
			t.Errorf("expected pancreatitis_cpl_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for pancreatitis_cpl_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "pancreatitis_cpl_updated" {
			t.Errorf("expected pancreatitis_cpl_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for pancreatitis_cpl_updated mesh event")
	}

	// 6. POST /api/pancreatitis-cpl/operations/PANC-OP-101/stage-nutrition transitions to FUZAPLADIB_MAROPITANT_ENTERAL_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"fuzapladib_protocol":       "Fuzapladib sodium 0.4 mg/kg IV q24h x 3d",
		"antiemetic_dual_protocol":  "Maropitant 1 mg/kg IV q24h + Ondansetron 0.5 mg/kg IV q8h",
		"visceral_analgesia_cri":    "Fentanyl + Ketamine + Lidocaine CRI (Zero NSAIDs)",
		"early_enteral_tube_plan":   "8 Fr Nasoesophageal tube ultra-low-fat polymeric liquid diet",
		"rer_caloric_ramp_schedule": "25% -> 50% -> 75% -> 100% RER over 72h",
		"fluid_crystalloid_plan":    "Lactated Ringer's + 20 mEq/L KCl",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/pancreatitis-cpl/operations/PANC-OP-101/stage-nutrition", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging nutrition on PANC-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.PancreatitisOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.PancreatitisStatusFuzapladibEnteralStaged {
		t.Errorf("expected status %s, got %s", domain.PancreatitisStatusFuzapladibEnteralStaged, staged.Status)
	}

	// 7. POST /api/pancreatitis-cpl/operations/PANC-OP-101/record-check appends check and updates telemetry
	checkPayload, _ := json.Marshal(map[string]any{
		"spec_pl_ug_l":                310,
		"modified_glasgow_pain_score": 4,
		"enteral_rer_percent":         75,
		"vomiting_regurg_status":      "ZERO_EMESIS_TOLERATING_NE_TUBE",
		"recorded_by":                 "Dr. Elena Rostova, DACVECC",
		"clinical_note":               "Spec cPL decreased from 940 to 310 ug/L; tolerating 75% RER via NE tube with zero emesis.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/pancreatitis-cpl/operations/PANC-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording check on PANC-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.PancreatitisOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if checked.SpecPLUgL != 310 || checked.ModifiedGlasgowPainScore != 4 || checked.EnteralRERPercent != 75 || len(checked.EnteralChecks) != 3 {
		t.Errorf("unexpected checked operation state: %+v", checked)
	}

	// 8. POST /api/pancreatitis-cpl/operations/PANC-OP-101/clear-discharge clears for discharge
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by": "Dr. Elena Rostova, DACVECC",
		"summary":       "Completed 3-day Fuzapladib sodium & Maropitant/Ondansetron course; tolerating 100% RER ultra-low-fat diet pain-free.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/pancreatitis-cpl/operations/PANC-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on PANC-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.PancreatitisOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.PancreatitisStatusToleratingEnteralCleared || cleared.DischargedBy != "Dr. Elena Rostova, DACVECC" || cleared.DischargedAt == nil {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/pancreatitis-cpl/operations/UNKNOWN",
		"/api/pancreatitis-cpl/operations/UNKNOWN/stage-nutrition",
		"/api/pancreatitis-cpl/operations/UNKNOWN/record-check",
		"/api/pancreatitis-cpl/operations/UNKNOWN/clear-discharge",
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

func TestPancreatitisCPLHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/pancreatitis-cpl/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/pancreatitis-cpl/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/pancreatitis-cpl/stream, got %d", resp.StatusCode)
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
