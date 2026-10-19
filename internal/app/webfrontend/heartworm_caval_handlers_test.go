package webfrontend

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestHeartwormCavalTriagePageAndOverview(t *testing.T) {
	srv := NewServer()

	// GET /heartworm-caval-triage
	req := httptest.NewRequest(http.MethodGet, "/heartworm-caval-triage", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /heartworm-caval-triage, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"heartworm-caval-cockpit",
		"HWC-OP-101",
		"Boudreaux",
		"HWC-OP-102",
		"Magnolia",
		"HWC-OP-103",
		"Cypress",
		"hwc-alert-banner",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected page HTML to contain %q", want)
		}
	}

	// HEAD /heartworm-caval-triage
	reqHead := httptest.NewRequest(http.MethodHead, "/heartworm-caval-triage", nil)
	recHead := httptest.NewRecorder()
	srv.ServeHTTP(recHead, reqHead)
	if recHead.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HEAD /heartworm-caval-triage, got %d", recHead.Code)
	}

	// Direct method not allowed check
	reqPostPage := httptest.NewRequest(http.MethodPost, "/heartworm-caval-triage", nil)
	recPostPage := httptest.NewRecorder()
	srv.handleHeartwormCavalTriagePage(recPostPage, reqPostPage)
	if recPostPage.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST handleHeartwormCavalTriagePage, got %d", recPostPage.Code)
	}

	// GET /api/heartworm-caval/overview
	reqOv := httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/overview", nil)
	recOv := httptest.NewRecorder()
	srv.ServeHTTP(recOv, reqOv)
	if recOv.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for overview, got %d", recOv.Code)
	}
	var ov domain.HeartwormOverview
	if err := json.NewDecoder(recOv.Body).Decode(&ov); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if ov.TotalOperations != 3 || ov.ClassIVCavalSyndromeCount != 1 {
		t.Fatalf("unexpected overview: %+v", ov)
	}

	reqOvPost := httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/overview", nil)
	recOvPost := httptest.NewRecorder()
	srv.handleHeartwormCavalOverview(recOvPost, reqOvPost)
	if recOvPost.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST handleHeartwormCavalOverview, got %d", recOvPost.Code)
	}
}

func TestHeartwormCavalOperationsCRUDAndLifecycle(t *testing.T) {
	srv := NewServer()
	srv.heartwormService = nil // test lazy initialization in ensureHeartwormService

	// List operations with status and severity_class filters
	reqList := httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/operations?status=JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED&severity_class=CLASS_IV_CAVAL_SYNDROME", nil)
	recList := httptest.NewRecorder()
	srv.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list operations, got %d", recList.Code)
	}

	// Get single operation
	reqGet := httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/operations/HWC-OP-101", nil)
	recGet := httptest.NewRecorder()
	srv.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get operation, got %d", recGet.Code)
	}

	reqGet404 := httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/operations/HWC-OP-999", nil)
	recGet404 := httptest.NewRecorder()
	srv.ServeHTTP(recGet404, reqGet404)
	if recGet404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown operation, got %d", recGet404.Code)
	}

	// Create new operation
	createBody := map[string]any{
		"pet_name":                         "Thibodaux",
		"species":                          "Canine",
		"breed":                            "Catahoula Leopard Dog",
		"weight_kg":                        26.0,
		"severity_class":                   "CLASS_IV_CAVAL_SYNDROME",
		"protocol":                         "RIGHT_JUGULAR_ISHIHARA_FORCEPS_EXTRACTION",
		"tr_jet_velocity_ms":               4.05,
		"worms_extracted_count":            0,
		"pcv_percent":                      19.5,
		"free_hemoglobinuria_present":      true,
		"parallel_hyperechoic_caval_lines": true,
	}
	createBytes, _ := json.Marshal(createBody)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations", bytes.NewReader(createBytes))
	recCreate := httptest.NewRecorder()
	srv.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var created domain.HeartwormCavalOperation
	if err := json.NewDecoder(recCreate.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID != "HWC-OP-104" {
		t.Fatalf("expected HWC-OP-104, got %s", created.ID)
	}

	// Stage intervention — first test immediate Melarsomine contraindication guard (400 Bad Request)
	badStageBody := map[string]any{
		"protocol":                              "SPLIT_DOSE_MELARSOMINE_DAY60_90_91",
		"immediate_melarsomine_contraindicated": false,
	}
	badStageBytes, _ := json.Marshal(badStageBody)
	reqBadStage := httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-104/intervention", bytes.NewReader(badStageBytes))
	recBadStage := httptest.NewRecorder()
	srv.ServeHTTP(recBadStage, reqBadStage)
	if recBadStage.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for immediate Melarsomine in active Class IV Caval Syndrome, got %d", recBadStage.Code)
	}

	// Valid Stage intervention
	stageBody := map[string]any{
		"protocol":                               "RIGHT_JUGULAR_ISHIHARA_FORCEPS_EXTRACTION",
		"right_jugular_venotomy_extraction_plan": "Right external jugular venotomy with Ishihara alligator forceps",
		"immediate_melarsomine_contraindicated":  true,
		"wolbachia_doxycycline_mg_kg_day":        20.0,
		"wolbachia_doxycycline_28day_plan":       "Doxycycline 10 mg/kg PO q12h for 28 days",
		"macrocyclic_lactone_pretreat_plan":      "Ivermectin/Milbemycin preventive + Diphenhydramine/Prednisone",
		"prednisone_taper_mg_kg_day":             1.0,
		"strict_cage_rest_weeks":                 8,
		"staged_by":                              "Dr. Thibodaux Broussard, DACVIM (Cardiology)",
	}
	stageBytes, _ := json.Marshal(stageBody)
	reqStage := httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-104/intervention", bytes.NewReader(stageBytes))
	recStage := httptest.NewRecorder()
	srv.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stage intervention, got %d: %s", recStage.Code, recStage.Body.String())
	}

	// Record hemodynamic check
	chkBody := map[string]any{
		"tr_jet_velocity_ms":               2.35,
		"worms_extracted_count":            31,
		"pcv_percent":                      36.0,
		"free_hemoglobinuria_present":      false,
		"parallel_hyperechoic_caval_lines": false,
		"sildenafil_mg_kg_dose":            1.5,
		"recorded_by":                      "Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		"clinical_note":                    "31 adult worms extracted via right jugular venotomy; caval lumen clear and hemoglobinuria resolved.",
	}
	chkBytes, _ := json.Marshal(chkBody)
	reqChk := httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-104/hemodynamic-check", bytes.NewReader(chkBytes))
	recChk := httptest.NewRecorder()
	srv.ServeHTTP(recChk, reqChk)
	if recChk.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for hemodynamic check, got %d: %s", recChk.Code, recChk.Body.String())
	}

	// Evaluate clearance — first test safety guard when parallel_hyperechoic_caval_lines is true
	badClearBody := map[string]any{
		"parallel_hyperechoic_caval_lines": true,
		"free_hemoglobinuria_present":      false,
		"tr_jet_velocity_ms":               2.35,
		"strict_cage_rest_verified":        true,
		"cleared_by":                       "Dr. Broussard",
		"clearance_summary":                "Attempt clearance with caval worms still present",
	}
	badClearBytes, _ := json.Marshal(badClearBody)
	reqBadClear := httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-104/clearance", bytes.NewReader(badClearBytes))
	recBadClear := httptest.NewRecorder()
	srv.ServeHTTP(recBadClear, reqBadClear)
	if recBadClear.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for clearance with caval lines present, got %d", recBadClear.Code)
	}

	// Valid clearance
	clearBody := map[string]any{
		"parallel_hyperechoic_caval_lines": false,
		"free_hemoglobinuria_present":      false,
		"tr_jet_velocity_ms":               2.35,
		"strict_cage_rest_verified":        true,
		"cleared_by":                       "Dr. Thibodaux Broussard, DACVIM (Cardiology)",
		"clearance_summary":                "Caval lumen patent, hemoglobinuria resolved, TR jet 2.35 m/s, 8-week strict cage rest verified; cleared.",
	}
	clearBytes, _ := json.Marshal(clearBody)
	reqClear := httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-104/clearance", bytes.NewReader(clearBytes))
	recClear := httptest.NewRecorder()
	srv.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var clearedOp domain.HeartwormCavalOperation
	if err := json.NewDecoder(recClear.Body).Decode(&clearedOp); err != nil {
		t.Fatalf("decode clearedOp: %v", err)
	}
	if clearedOp.Status != domain.HeartwormStatusCavalPatentAntigenNegativeCleared {
		t.Fatalf("expected status CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED, got %s", clearedOp.Status)
	}
}

func TestHeartwormCavalErrorPathsAndSSEStream(t *testing.T) {
	srv := NewServer()
	srv.reunionPingInterval = 15 * time.Millisecond

	// Method not allowed & invalid JSON checks on handlers
	r1 := httptest.NewRecorder()
	srv.handleListHeartwormCavalOperations(r1, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations", nil))
	if r1.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r1.Code)
	}

	r2 := httptest.NewRecorder()
	srv.handleGetHeartwormCavalOperation(r2, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-101", nil))
	if r2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r2.Code)
	}

	r3 := httptest.NewRecorder()
	srv.handleGetHeartwormCavalOperation(r3, httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/operations/", nil))
	if r3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", r3.Code)
	}

	r4 := httptest.NewRecorder()
	srv.handleCreateHeartwormCavalOperation(r4, httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/operations", nil))
	if r4.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r4.Code)
	}

	r5 := httptest.NewRecorder()
	srv.handleCreateHeartwormCavalOperation(r5, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations", strings.NewReader("{bad")))
	if r5.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON, got %d", r5.Code)
	}

	r6 := httptest.NewRecorder()
	srv.handleStageHeartwormIntervention(r6, httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/operations/HWC-OP-101/intervention", nil))
	if r6.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r6.Code)
	}

	r7 := httptest.NewRecorder()
	srv.handleStageHeartwormIntervention(r7, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations//intervention", nil))
	if r7.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", r7.Code)
	}

	r8 := httptest.NewRecorder()
	srv.ServeHTTP(r8, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/UNKNOWN/intervention", strings.NewReader("{}")))
	if r8.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r8.Code)
	}

	r9 := httptest.NewRecorder()
	srv.ServeHTTP(r9, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-101/intervention", strings.NewReader("{bad")))
	if r9.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r9.Code)
	}

	r10 := httptest.NewRecorder()
	srv.handleRecordHeartwormHemodynamicCheck(r10, httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/operations/HWC-OP-101/hemodynamic-check", nil))
	if r10.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r10.Code)
	}

	r11 := httptest.NewRecorder()
	srv.handleRecordHeartwormHemodynamicCheck(r11, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations//hemodynamic-check", nil))
	if r11.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r11.Code)
	}

	r12 := httptest.NewRecorder()
	srv.ServeHTTP(r12, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/UNKNOWN/hemodynamic-check", strings.NewReader("{}")))
	if r12.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r12.Code)
	}

	r13 := httptest.NewRecorder()
	srv.ServeHTTP(r13, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-101/hemodynamic-check", strings.NewReader("{bad")))
	if r13.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r13.Code)
	}

	r14 := httptest.NewRecorder()
	srv.handleEvaluateHeartwormClearance(r14, httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/operations/HWC-OP-101/clearance", nil))
	if r14.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r14.Code)
	}

	r15 := httptest.NewRecorder()
	srv.handleEvaluateHeartwormClearance(r15, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations//clearance", nil))
	if r15.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r15.Code)
	}

	r16 := httptest.NewRecorder()
	srv.ServeHTTP(r16, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/UNKNOWN/clearance", strings.NewReader("{}")))
	if r16.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r16.Code)
	}

	r17 := httptest.NewRecorder()
	srv.ServeHTTP(r17, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/operations/HWC-OP-101/clearance", strings.NewReader("{bad")))
	if r17.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r17.Code)
	}

	r18 := httptest.NewRecorder()
	srv.handleHeartwormCavalStream(r18, httptest.NewRequest(http.MethodPost, "/api/heartworm-caval/stream", nil))
	if r18.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r18.Code)
	}

	// SSE stream test
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()
	reqSSE := httptest.NewRequest(http.MethodGet, "/api/heartworm-caval/stream", nil).WithContext(ctx)
	recSSE := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.ServeHTTP(recSSE, reqSSE)
	}()

	time.Sleep(10 * time.Millisecond)
	srv.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{"id": "HWC-OP-101"}, time.Now().UTC())
	<-done

	if !strings.Contains(recSSE.Body.String(), ": connected") {
		t.Fatalf("expected SSE connected comment, got %s", recSSE.Body.String())
	}
}
