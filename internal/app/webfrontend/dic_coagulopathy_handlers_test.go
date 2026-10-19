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

func TestDICCoagulopathyHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	dicSSECh, unsubSSE := server.ReunionHub().Subscribe("dic_coagulopathy")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("dic_coagulopathy", "dic-coagulopathy-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /dic-coagulopathy-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/dic-coagulopathy-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /dic-coagulopathy-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Coagulopathy &amp; Disseminated Intravascular Coagulation (DIC)",
		"dic-coagulopathy-cockpit",
		"dic-alert-banner",
		"dic-clearance-banner",
		"dic-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"dic-operations-grid",
		"dic-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-d-dimer",
		"active-fibrinogen",
		"create-dic-form",
		"stage-therapy-form",
		"record-coag-check-form",
		"clear-dic-form",
		"DIC-OP-101",
		"DIC-OP-102",
		"DIC-OP-103",
		"Flash",
		"Rocket",
		"Willow",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /dic-coagulopathy-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/dic-coagulopathy/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/dic-coagulopathy/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/dic-coagulopathy/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.DICCoagulopathyOperation `json:"operations"`
		Metrics    domain.DICOverviewMetrics         `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal DIC operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?phenotype=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/dic-coagulopathy/operations?status=ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE&phenotype=HYPO_COAGULABLE_HEMORRHAGIC_CONSUMPTIVE_DIC", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.DICCoagulopathyOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "DIC-OP-101" {
		t.Errorf("expected 1 filtered operation DIC-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/dic-coagulopathy/operations/DIC-OP-101 returns Flash
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/dic-coagulopathy/operations/DIC-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/dic-coagulopathy/operations/DIC-OP-101, got %d", recGet.Code)
	}
	var op101 domain.DICCoagulopathyOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal DIC-OP-101: %v", err)
	}
	if op101.PetName != "Flash" || op101.Status != domain.DICStatusAcuteOvertDICTriage || op101.DDimerNgML != 1850.0 {
		t.Errorf("unexpected DIC-OP-101 state: %+v", op101)
	}

	// 4. POST /api/dic-coagulopathy/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/dic-coagulopathy/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/dic-coagulopathy/operations creates a new operation "Zephyr" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                      "CASE-DIC-104",
		"pet_name":                     "Zephyr",
		"species":                      "Canine",
		"breed":                        "Retired Racing Greyhound",
		"weight_kg":                    29.5,
		"phase_phenotype":              "HYPO_COAGULABLE_HEMORRHAGIC_CONSUMPTIVE_DIC",
		"platelet_count_k_ul":          42.0,
		"pt_seconds":                   23.2,
		"aptt_seconds":                 138.0,
		"d_dimer_ng_ml":                1720.0,
		"fibrinogen_mg_dl":             74.0,
		"antithrombin_percent":         51.0,
		"schistocytes_present":         true,
		"spontaneous_mucosal_bleeding": true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/dic-coagulopathy/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/dic-coagulopathy/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.DICCoagulopathyOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "DIC-OP-104" || created.PetName != "Zephyr" || created.DDimerNgML != 1720.0 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-dicSSECh:
		if evt.Type != domain.ReunionEventType("dic_coagulopathy_updated") {
			t.Errorf("expected dic_coagulopathy_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for dic_coagulopathy_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "dic_coagulopathy_updated" {
			t.Errorf("expected dic_coagulopathy_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for dic_coagulopathy_updated mesh event")
	}

	// 6. POST /api/dic-coagulopathy/operations/DIC-OP-101/stage-therapy transitions to FFP_CRYOPRECIPITATE_TXA_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"fresh_frozen_plasma_protocol":      "Fresh Frozen Plasma (FFP) 15 mL/kg IV over 3–4h via blood filter",
		"cryoprecipitate_vwf_protocol":      "Cryoprecipitate 3 units IV (1 unit/10 kg for Fibrinogen < 100 mg/dL)",
		"tranexamic_acid_aminocaproic_plan": "Tranexamic Acid (TXA) 10 mg/kg IV slow infusion over 20 min q8h",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/dic-coagulopathy/operations/DIC-OP-101/stage-therapy", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging component therapy on DIC-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.DICCoagulopathyOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.DICStatusFFPCryoprecipitateTXAStaged {
		t.Errorf("expected status %s, got %s", domain.DICStatusFFPCryoprecipitateTXAStaged, staged.Status)
	}

	// 7. POST /api/dic-coagulopathy/operations/DIC-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"platelet_count_k_ul": 145.0,
		"pt_seconds":          11.6,
		"aptt_seconds":        72.0,
		"d_dimer_ng_ml":       240.0,
		"fibrinogen_mg_dl":    195.0,
		"recorded_by":         "Dr. Callum Vance, DACVECC",
		"clinical_note":       "Post-FFP, Cryoprecipitate & TXA: mucosal bleeding resolved, Fibrinogen restored to 195.0 mg/dL, D-Dimer down to 240.0 ng/mL.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/dic-coagulopathy/operations/DIC-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording coagulation check on DIC-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.DICCoagulopathyOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.CoagChecks) != 3 || checked.DDimerNgML != 240.0 || checked.FibrinogenMgDL != 195.0 {
		t.Errorf("expected 3 coag checks and updated D-Dimer/Fibrinogen on DIC-OP-101, got %+v", checked)
	}

	// 8. POST /api/dic-coagulopathy/operations/DIC-OP-101/clear-discharge transitions to HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Callum Vance, DACVECC",
		"discharge_summary": "Hemostasis restored, PT/aPTT normalized (11.6s / 72.0s), Fibrinogen 195.0 mg/dL, D-Dimer 240.0 ng/mL, cleared for step-down recovery.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/dic-coagulopathy/operations/DIC-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on DIC-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.DICCoagulopathyOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.DICStatusHemostasisNormocoagulableClear || cleared.DischargedBy != "Dr. Callum Vance, DACVECC" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/dic-coagulopathy/operations/UNKNOWN",
		"/api/dic-coagulopathy/operations/UNKNOWN/stage-therapy",
		"/api/dic-coagulopathy/operations/UNKNOWN/record-check",
		"/api/dic-coagulopathy/operations/UNKNOWN/clear-discharge",
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

func TestDICCoagulopathyHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/dic-coagulopathy/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/dic-coagulopathy/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/dic-coagulopathy/stream, got %d", resp.StatusCode)
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
