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

func TestVectorBorneRickettsialHandlers_SeededCRUDStageCheckClearAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	vbrSSECh, unsubSSE := server.ReunionHub().Subscribe("vector_borne_rickettsial")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("vector_borne_rickettsial", "vector-borne-rickettsial-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /vector-borne-rickettsial-triage renders HTML with seeded operations and key DOM IDs
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/vector-borne-rickettsial-triage", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /vector-borne-rickettsial-triage, got %d: %s", recPage.Code, recPage.Body.String())
	}
	bodyStr := recPage.Body.String()
	for _, snippet := range []string{
		"Acute Vector-Borne &amp; Rickettsial Emergencies",
		"vbr-triage-cockpit",
		"vbr-alert-banner",
		"vbr-clearance-banner",
		"vbr-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"vbr-operations-grid",
		"vbr-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-platelets",
		"active-pcv",
		"active-upc",
		"create-vbr-form",
		"stage-antimicrobial-form",
		"record-hematology-check-form",
		"clear-vbr-form",
		"VBR-OP-101",
		"VBR-OP-102",
		"VBR-OP-103",
		"Yukon",
		"Willow",
		"Boone",
	} {
		if !strings.Contains(bodyStr, snippet) {
			t.Errorf("expected GET /vector-borne-rickettsial-triage HTML to contain %q", snippet)
		}
	}

	// 1b. GET /api/vector-borne-rickettsial/overview returns aggregate overview metrics
	recOv := httptest.NewRecorder()
	reqOv := httptest.NewRequest(http.MethodGet, "/api/vector-borne-rickettsial/overview", nil)
	server.ServeHTTP(recOv, reqOv)
	if recOv.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/vector-borne-rickettsial/overview, got %d", recOv.Code)
	}
	var ov domain.VectorBorneOverview
	if err := json.Unmarshal(recOv.Body.Bytes(), &ov); err != nil {
		t.Fatalf("failed to unmarshal overview: %v", err)
	}
	if ov.TotalOperations != 3 || ov.AntimicrobialStagedCount != 1 || ov.ThrombocytopeniaMonitoredCount != 1 || ov.PCRNegativeClearedCount != 1 {
		t.Errorf("unexpected overview metrics: %+v", ov)
	}

	// 2. GET /api/vector-borne-rickettsial/operations returns 3 seeded operations
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/vector-borne-rickettsial/operations", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/vector-borne-rickettsial/operations, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Operations []domain.VectorBorneOperation `json:"operations"`
		Overview   domain.VectorBorneOverview    `json:"overview"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal Vector-Borne operations: %v", err)
	}
	if len(listResp.Operations) != 3 || listResp.Overview.TotalOperations != 3 {
		t.Fatalf("expected 3 seeded operations, got %d (overview=%+v)", len(listResp.Operations), listResp.Overview)
	}

	// 2b. Filter by ?status= and ?syndrome=
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/vector-borne-rickettsial/operations?status=DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED&syndrome=LYME_NEPHRITIS_BORRELIA_UPC", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered operations, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Operations []domain.VectorBorneOperation `json:"operations"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered operations: %v", err)
	}
	if len(filteredResp.Operations) != 1 || filteredResp.Operations[0].ID != "VBR-OP-101" {
		t.Errorf("expected 1 filtered operation VBR-OP-101, got %+v", filteredResp.Operations)
	}

	// 3. GET /api/vector-borne-rickettsial/operations/VBR-OP-101 returns Yukon
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/vector-borne-rickettsial/operations/VBR-OP-101", nil)
	server.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/vector-borne-rickettsial/operations/VBR-OP-101, got %d", recGet.Code)
	}
	var op101 domain.VectorBorneOperation
	if err := json.Unmarshal(recGet.Body.Bytes(), &op101); err != nil {
		t.Fatalf("failed to unmarshal VBR-OP-101: %v", err)
	}
	if op101.PetName != "Yukon" || op101.Status != domain.VectorBorneStatusAntimicrobialStaged || op101.UrinePCRatio != 4.8 {
		t.Errorf("unexpected VBR-OP-101 state: %+v", op101)
	}

	// 4. POST /api/vector-borne-rickettsial/operations with invalid payload returns 400
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/vector-borne-rickettsial/operations", bytes.NewReader([]byte(`{"pet_name":""}`)))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid create payload, got %d", recInvalid.Code)
	}

	// 5. POST /api/vector-borne-rickettsial/operations creates a new operation "Scout" (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"case_id":                         "CASE-VBR-104",
		"pet_name":                        "Scout",
		"species":                         "Canine",
		"breed":                           "Australian Shepherd",
		"weight_kg":                       22.5,
		"syndrome":                        "EHRLICHIOSIS_ANAPLASMOSIS_MORULAE",
		"assay":                           "SNAP_4DX_PLUS_C6_PEPTIDE",
		"antimicrobial_protocol":          "DOXYCYCLINE_10MG_KG_WATER_CHASE",
		"platelet_count_kul":              38.0,
		"pcv_percent":                     28.0,
		"urine_pc_ratio":                  0.4,
		"serum_albumin_g_dl":              2.6,
		"doxycycline_mg_kg_day":           10.0,
		"atovaquone_mg_kg_tid":            0.0,
		"imidocarb_mg_kg_im":              0.0,
		"esophageal_water_chase_verified": true,
		"atropine_premed_verified":        false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/vector-borne-rickettsial/operations", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/vector-borne-rickettsial/operations, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.VectorBorneOperation
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created operation: %v", err)
	}
	if created.ID != "VBR-OP-104" || created.PetName != "Scout" || created.PlateletCountKUL != 38.0 {
		t.Errorf("unexpected created operation state: %+v", created)
	}

	// Verify SSE and Mesh broadcast
	select {
	case evt := <-vbrSSECh:
		if evt.Type != domain.ReunionEventType("vector_borne_rickettsial_updated") {
			t.Errorf("expected vector_borne_rickettsial_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for vector_borne_rickettsial_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "vector_borne_rickettsial_updated" {
			t.Errorf("expected vector_borne_rickettsial_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for vector_borne_rickettsial_updated mesh event")
	}

	// 6. POST /api/vector-borne-rickettsial/operations/VBR-OP-104/antimicrobial stages regimen
	stagePayload, _ := json.Marshal(map[string]any{
		"protocol":                        "DOXYCYCLINE_10MG_KG_WATER_CHASE",
		"doxycycline_mg_kg_day":           10.0,
		"esophageal_water_chase_verified": true,
		"staged_by":                       "Dr. Mara Vance, DACVIM (Infectious Disease)",
		"clinical_rationale":              "Doxycycline 10 mg/kg/day PO with water bolus chase staged for Ehrlichia morulae.",
	})
	recStage := httptest.NewRecorder()
	reqStage := httptest.NewRequest(http.MethodPost, "/api/vector-borne-rickettsial/operations/VBR-OP-104/antimicrobial", bytes.NewReader(stagePayload))
	reqStage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK staging antimicrobial on VBR-OP-104, got %d: %s", recStage.Code, recStage.Body.String())
	}
	var staged domain.VectorBorneOperation
	if err := json.Unmarshal(recStage.Body.Bytes(), &staged); err != nil {
		t.Fatalf("failed to unmarshal staged operation: %v", err)
	}
	if staged.Status != domain.VectorBorneStatusAntimicrobialStaged {
		t.Errorf("expected status %s, got %s", domain.VectorBorneStatusAntimicrobialStaged, staged.Status)
	}

	// 7. POST /api/vector-borne-rickettsial/operations/VBR-OP-104/hematology-check transitions to THROMBOCYTOPENIA_PROTEINURIA_MONITORED
	checkPayload, _ := json.Marshal(map[string]any{
		"platelet_count_kul":            185.0,
		"pcv_percent":                   39.0,
		"urine_pc_ratio":                0.25,
		"serum_albumin_g_dl":            3.2,
		"morulae_or_piroplasms_cleared": true,
		"recorded_by":                   "Dr. Mara Vance, DACVIM (Infectious Disease)",
		"clinical_note":                 "Platelets rebounded to 185.0 K/uL, PCV 39.0%, UPC 0.25, buffy coat smear negative.",
	})
	recCheck := httptest.NewRecorder()
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/vector-borne-rickettsial/operations/VBR-OP-104/hematology-check", bytes.NewReader(checkPayload))
	reqCheck.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK recording hematology check on VBR-OP-104, got %d: %s", recCheck.Code, recCheck.Body.String())
	}
	var checked domain.VectorBorneOperation
	if err := json.Unmarshal(recCheck.Body.Bytes(), &checked); err != nil {
		t.Fatalf("failed to unmarshal checked operation: %v", err)
	}
	if checked.Status != domain.VectorBorneStatusThrombocytopeniaMonitored || len(checked.HematologyChecks) != 1 || checked.PlateletCountKUL != 185.0 {
		t.Errorf("expected THROMBOCYTOPENIA_PROTEINURIA_MONITORED and updated vitals on VBR-OP-104, got %+v", checked)
	}

	// 8. POST /api/vector-borne-rickettsial/operations/VBR-OP-104/clearance transitions to PCR_NEGATIVE_PLATELETS_RENAL_CLEARED
	clearPayload, _ := json.Marshal(map[string]any{
		"platelet_count_kul":           195.0,
		"pcv_percent":                  40.0,
		"urine_pc_ratio":               0.20,
		"pcr_negative_verified":        true,
		"normothermic_eating_verified": true,
		"cleared_by":                   "Dr. Mara Vance, DACVIM (Infectious Disease)",
		"clearance_summary":            "Quantitative PCR negative, platelets 195.0 K/uL, PCV 40.0%, UPC 0.20, normothermic and eating.",
	})
	recClear := httptest.NewRecorder()
	reqClear := httptest.NewRequest(http.MethodPost, "/api/vector-borne-rickettsial/operations/VBR-OP-104/clearance", bytes.NewReader(clearPayload))
	reqClear.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK clearing VBR-OP-104, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var cleared domain.VectorBorneOperation
	if err := json.Unmarshal(recClear.Body.Bytes(), &cleared); err != nil {
		t.Fatalf("failed to unmarshal cleared operation: %v", err)
	}
	if cleared.Status != domain.VectorBorneStatusPCRNegativeCleared || cleared.DischargedBy != "Dr. Mara Vance, DACVIM (Infectious Disease)" {
		t.Errorf("unexpected cleared operation state: %+v", cleared)
	}

	// 9. 404 Not Found for unknown operation ID
	for _, path := range []string{
		"/api/vector-borne-rickettsial/operations/UNKNOWN",
		"/api/vector-borne-rickettsial/operations/UNKNOWN/antimicrobial",
		"/api/vector-borne-rickettsial/operations/UNKNOWN/hematology-check",
		"/api/vector-borne-rickettsial/operations/UNKNOWN/clearance",
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

func TestVectorBorneRickettsialHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/vector-borne-rickettsial/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/vector-borne-rickettsial/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/vector-borne-rickettsial/stream, got %d", resp.StatusCode)
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
