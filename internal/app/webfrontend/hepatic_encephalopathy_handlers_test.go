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

func TestHepaticEncephalopathyHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	hepaticSSECh, unsubSSE := server.ReunionHub().Subscribe("hepatic_encephalopathy")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("hepatic_encephalopathy", "hepatic-encephalopathy-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /hepatic-encephalopathy-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/hepatic-encephalopathy-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /hepatic-encephalopathy-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Hepatic Failure",
		"hepatic-encephalopathy-cockpit",
		"hepatic-alert-banner",
		"hepatic-clearance-banner",
		"hepatic-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"hepatic-operations-grid",
		"hepatic-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-blood-ammonia",
		"active-alt-value",
		"create-hepatic-form",
		"stage-support-form",
		"record-metabolic-check-form",
		"clear-hepatic-form",
		"HEP-OP-101",
		"HEP-OP-102",
		"HEP-OP-103",
		"Kona",
		"Marmalade",
		"Scout",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /hepatic-encephalopathy-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/hepatic-encephalopathy/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/hepatic-encephalopathy/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/hepatic-encephalopathy/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.HepaticOperation     `json:"operations"`
		Metrics    domain.HepaticOverviewMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal Hepatic operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?etiology=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/hepatic-encephalopathy/operations?status=ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE&etiology=SAGO_PALM_CYCASIN_HEPATOTOXICITY", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.HepaticOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "HEP-OP-101" {
		t.Errorf("expected 1 filtered operation HEP-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/hepatic-encephalopathy/operations/HEP-OP-101 returns Kona
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/hepatic-encephalopathy/operations/HEP-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/hepatic-encephalopathy/operations/HEP-OP-101, got %d", recGet.Code)
	}
	var op101 domain.HepaticOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal HEP-OP-101: %v", err)
	}
	if op101.PetName != "Kona" || op101.Status != domain.HepaticStatusFulminantHETriage || op101.BloodAmmoniaUmolL != 245.0 {
		t.Errorf("unexpected HEP-OP-101 state: %+v", op101)
	}

	// 4. POST /api/hepatic-encephalopathy/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/hepatic-encephalopathy/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/hepatic-encephalopathy/operations creates a new operation "Bodie" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                        "CASE-HEP-104",
		"pet_name":                       "Bodie",
		"species":                        "Canine",
		"breed":                          "Golden Retriever",
		"weight_kg":                      29.5,
		"etiology_type":                  "SAGO_PALM_CYCASIN_HEPATOTOXICITY",
		"alt_iu_l":                       3120,
		"total_bilirubin_mg_dl":          7.2,
		"blood_ammonia_umol_l":           260.0,
		"blood_glucose_mg_dl":            52.0,
		"pt_seconds":                     27.8,
		"encephalopathy_grade":           3,
		"head_pressing_seizures_present": true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/hepatic-encephalopathy/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/hepatic-encephalopathy/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.HepaticOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "HEP-OP-104" || created.PetName != "Bodie" || created.BloodAmmoniaUmolL != 260.0 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-hepaticSSECh:
		if evt.Type != domain.ReunionEventType("hepatic_encephalopathy_updated") {
			t.Errorf("expected hepatic_encephalopathy_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for hepatic_encephalopathy_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "hepatic_encephalopathy_updated" {
			t.Errorf("expected hepatic_encephalopathy_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for hepatic_encephalopathy_updated mesh event")
	}

	// 6. POST /api/hepatic-encephalopathy/operations/HEP-OP-101/stage-support transitions to NAC_LACTULOSE_VITAMIN_K1_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"nac_glutathione_loading_protocol": "N-Acetylcysteine (NAC) 140 mg/kg IV loading dose via 0.2-micron filter, followed by 70 mg/kg IV q6h x 7 doses",
		"lactulose_enema_oral_protocol":    "Lactulose 1:3 warm water retention enema (15 mL/kg retained 20–30 min) + oral Lactulose 0.5 mL/kg PO q8h + Metronidazole 7.5 mg/kg IV q12h",
		"levetiracetam_neuro_protocol":     "Levetiracetam 60 mg/kg IV loading dose, then 20 mg/kg IV q8h (strictly avoiding Diazepam/Midazolam)",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/hepatic-encephalopathy/operations/HEP-OP-101/stage-support", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging NAC/Lactulose support protocol on HEP-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.HepaticOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.HepaticStatusNACLactuloseStaged {
		t.Errorf("expected status %s, got %s", domain.HepaticStatusNACLactuloseStaged, staged.Status)
	}

	// 7. POST /api/hepatic-encephalopathy/operations/HEP-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"alt_iu_l":              480,
		"total_bilirubin_mg_dl": 1.6,
		"blood_ammonia_umol_l":  42.0,
		"blood_glucose_mg_dl":   96.0,
		"pt_seconds":            12.8,
		"recorded_by":           "Dr. Priya Nair, DACVECC",
		"clinical_note":         "Post-NAC & Lactulose retention enemas: blood ammonia decreased to 42.0 umol/L, ALT reduced to 480 IU/L, euglycemic 96.0 mg/dL, PT 12.8 sec.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/hepatic-encephalopathy/operations/HEP-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording metabolic check on HEP-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.HepaticOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.MetabolicChecks) != 3 || checked.BloodAmmoniaUmolL != 42.0 || checked.ALTIUL != 480 {
		t.Errorf("expected 3 metabolic checks and updated ammonia/ALT on HEP-OP-101, got %+v", checked)
	}

	// 8. POST /api/hepatic-encephalopathy/operations/HEP-OP-101/clear-discharge transitions to AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Priya Nair, DACVECC",
		"discharge_summary": "Sago palm hepatotoxicity resolved; blood ammonia normalized (42.0 umol/L), HE Grade 0, PT normalized (12.8 sec), discharged on oral SAMe/Silybin + Lactulose.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/hepatic-encephalopathy/operations/HEP-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on HEP-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.HepaticOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.HepaticStatusAmmoniaNormalizedCleared || cleared.DischargedBy != "Dr. Priya Nair, DACVECC" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/hepatic-encephalopathy/operations/UNKNOWN",
		"/api/hepatic-encephalopathy/operations/UNKNOWN/stage-support",
		"/api/hepatic-encephalopathy/operations/UNKNOWN/record-check",
		"/api/hepatic-encephalopathy/operations/UNKNOWN/clear-discharge",
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

func TestHepaticEncephalopathyHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/hepatic-encephalopathy/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/hepatic-encephalopathy/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/hepatic-encephalopathy/stream, got %d", resp.StatusCode)
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
