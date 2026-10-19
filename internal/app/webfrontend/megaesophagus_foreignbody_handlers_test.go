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

func TestMegaesophagusForeignBodyHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	megaSSECh, unsubSSE := server.ReunionHub().Subscribe("megaesophagus_foreignbody")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("megaesophagus_foreignbody", "megaesophagus-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /megaesophagus-foreignbody-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/megaesophagus-foreignbody-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /megaesophagus-foreignbody-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Esophageal Foreign Body, Megaesophagus, Myasthenia Gravis",
		"megaesophagus-foreignbody-cockpit",
		"mega-alert-banner",
		"mega-clearance-banner",
		"mega-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"mega-operations-grid",
		"mega-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-spo2",
		"active-regurg-episodes",
		"create-mega-form",
		"stage-bailey-form",
		"record-esophageal-check-form",
		"clear-mega-form",
		"MEGA-OP-101",
		"MEGA-OP-102",
		"MEGA-OP-103",
		"Waffles",
		"Freya",
		"Pippin",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /megaesophagus-foreignbody-triage HTML to contain %q", snippet)
		}
	}

	// 2. GET /api/megaesophagus-foreignbody/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/megaesophagus-foreignbody/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/megaesophagus-foreignbody/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.MegaesophagusOperation     `json:"operations"`
		Metrics    domain.MegaesophagusOverviewMetrics `json:"metrics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal Megaesophagus operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Metrics.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (metrics=%+v)", len(listResp.Operations), listResp.Metrics)
	}

	// 2b. Filter by ?status= and ?pathology=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/megaesophagus-foreignbody/operations?status=ACUTE_ESOPHAGEAL_OBSTRUCTION_ASPIRATION_TRIAGE&pathology=ACUTE_ESOPHAGEAL_BONE_FISHHOOK_FOREIGN_BODY", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.MegaesophagusOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "MEGA-OP-101" {
		t.Errorf("expected 1 filtered operation MEGA-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/megaesophagus-foreignbody/operations/MEGA-OP-101 returns Waffles
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/megaesophagus-foreignbody/operations/MEGA-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/megaesophagus-foreignbody/operations/MEGA-OP-101, got %d", recGet.Code)
	}
	var op101 domain.MegaesophagusOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal MEGA-OP-101: %v", err)
	}
	if op101.PetName != "Waffles" || op101.Status != domain.MegaStatusAcuteObstructionAspirationTriage || op101.SpO2Percent != 91.0 {
		t.Errorf("unexpected MEGA-OP-101 state: %+v", op101)
	}

	// 4. POST /api/megaesophagus-foreignbody/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/megaesophagus-foreignbody/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/megaesophagus-foreignbody/operations creates a new operation "Paddington" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                         "CASE-MEGA-104",
		"pet_name":                        "Paddington",
		"species":                         "Canine",
		"breed":                           "Golden Retriever",
		"weight_kg":                       28.2,
		"pathology_type":                  "ACUTE_ESOPHAGEAL_BONE_FISHHOOK_FOREIGN_BODY",
		"spo2_percent":                    92.0,
		"alveolar_arterial_gradient_mmhg": 31.0,
		"regurgitation_episodes_24h":      5,
		"upright_hold_minutes":            0,
		"mucosal_esophagitis_grade":       2,
		"achr_antibody_titer_nmol_l":      0.15,
		"aspiration_pneumonia_present":    true,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/megaesophagus-foreignbody/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/megaesophagus-foreignbody/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.MegaesophagusOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "MEGA-OP-104" || created.PetName != "Paddington" || created.SpO2Percent != 92.0 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-megaSSECh:
		if evt.Type != domain.ReunionEventType("megaesophagus_foreignbody_updated") {
			t.Errorf("expected megaesophagus_foreignbody_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for megaesophagus_foreignbody_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "megaesophagus_foreignbody_updated" {
			t.Errorf("expected megaesophagus_foreignbody_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for megaesophagus_foreignbody_updated mesh event")
	}

	// 6. POST /api/megaesophagus-foreignbody/operations/MEGA-OP-101/stage-bailey transitions to ENDOSCOPIC_RETRIEVAL_PYRIDOSTIGMINE_STAGED
	stagePayload, _ := json.Marshal(map[string]any{
		"endoscopic_retrieval_or_gastric_push":  "Flexible esophagoscopy under cuffed endotracheal tube anesthesia; alligator forceps extraction of wedged rib bone",
		"post_removal_sucralfate_omeprazole":    "Sucralfate oral suspension slurry 0.5 g PO q8h + Omeprazole 1 mg/kg PO q12h",
		"upright_bailey_chair_feeding_protocol": "Upright Bailey Chair 90-degree vertical feeding of Knox gelatin / meatball slurry with strict 30-min hold",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/megaesophagus-foreignbody/operations/MEGA-OP-101/stage-bailey", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging endoscopy/bailey plan on MEGA-OP-101, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.MegaesophagusOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.MegaStatusEndoscopicPyridostigmineStaged {
		t.Errorf("expected status %s, got %s", domain.MegaStatusEndoscopicPyridostigmineStaged, staged.Status)
	}

	// 7. POST /api/megaesophagus-foreignbody/operations/MEGA-OP-101/record-check appends check and updates operation vitals
	checkPayload, _ := json.Marshal(map[string]any{
		"spo2_percent":                    98.0,
		"alveolar_arterial_gradient_mmhg": 10.0,
		"regurgitation_episodes_24h":      0,
		"upright_hold_minutes":            30,
		"mucosal_esophagitis_grade":       1,
		"recorded_by":                     "Dr. Clara Sterling, DACVIM (SAIM)",
		"clinical_note":                   "Post-endoscopic bone extraction, Sucralfate/Omeprazole & 30-min Upright Bailey Chair feeding: 0 regurgitation episodes over 24h, SpO2 98.0%.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/megaesophagus-foreignbody/operations/MEGA-OP-101/record-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording esophageal check on MEGA-OP-101, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.MegaesophagusOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if len(checked.EsophagealChecks) != 3 || checked.SpO2Percent != 98.0 || checked.RegurgitationEpisodes24h != 0 || checked.UprightHoldMinutes != 30 {
		t.Errorf("expected 3 esophageal checks and updated SpO2/Regurg/BaileyHold on MEGA-OP-101, got %+v", checked)
	}

	// 8. POST /api/megaesophagus-foreignbody/operations/MEGA-OP-101/clear-discharge transitions to UPRIGHT_BAILEY_CHAIR_NORMOTHERMIC_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"discharged_by":     "Dr. Clara Sterling, DACVIM (SAIM)",
		"discharge_summary": "Esophageal obstruction resolved, 0 passive regurgitation episodes over 24h on 30-min Upright Bailey Chair feeding protocol, SpO2 98.0% on room air.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/megaesophagus-foreignbody/operations/MEGA-OP-101/clear-discharge", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing discharge on MEGA-OP-101, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.MegaesophagusOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.MegaStatusUprightBaileyChairCleared || cleared.DischargedBy != "Dr. Clara Sterling, DACVIM (SAIM)" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/megaesophagus-foreignbody/operations/UNKNOWN",
		"/api/megaesophagus-foreignbody/operations/UNKNOWN/stage-bailey",
		"/api/megaesophagus-foreignbody/operations/UNKNOWN/record-check",
		"/api/megaesophagus-foreignbody/operations/UNKNOWN/clear-discharge",
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

func TestMegaesophagusForeignBodyHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/megaesophagus-foreignbody/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/megaesophagus-foreignbody/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/megaesophagus-foreignbody/stream, got %d", resp.StatusCode)
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
