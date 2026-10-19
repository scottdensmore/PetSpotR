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

func TestSystemicMycosesTriagePageAndOverview(t *testing.T) {
	srv := NewServer()

	// GET /systemic-mycoses-fungal-triage
	req := httptest.NewRequest(http.MethodGet, "/systemic-mycoses-fungal-triage", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /systemic-mycoses-fungal-triage, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"systemic-mycoses-cockpit",
		"MYC-OP-101",
		"Northwoods",
		"MYC-OP-102",
		"Sonora",
		"MYC-OP-103",
		"Sedona",
		"myc-alert-banner",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected page HTML to contain %q", want)
		}
	}

	// HEAD /systemic-mycoses-fungal-triage
	reqHead := httptest.NewRequest(http.MethodHead, "/systemic-mycoses-fungal-triage", nil)
	recHead := httptest.NewRecorder()
	srv.ServeHTTP(recHead, reqHead)
	if recHead.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HEAD /systemic-mycoses-fungal-triage, got %d", recHead.Code)
	}

	// Direct method not allowed check
	reqPostPage := httptest.NewRequest(http.MethodPost, "/systemic-mycoses-fungal-triage", nil)
	recPostPage := httptest.NewRecorder()
	srv.handleSystemicMycosesTriagePage(recPostPage, reqPostPage)
	if recPostPage.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST handleSystemicMycosesTriagePage, got %d", recPostPage.Code)
	}

	// GET /api/systemic-mycoses/overview
	reqOv := httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/overview", nil)
	recOv := httptest.NewRecorder()
	srv.ServeHTTP(recOv, reqOv)
	if recOv.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for overview, got %d", recOv.Code)
	}
	var ov domain.SystemicMycosesOverview
	if err := json.NewDecoder(recOv.Body).Decode(&ov); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if ov.TotalOperations != 3 || ov.CribriformBreachContraindicatedCount != 1 {
		t.Fatalf("unexpected overview: %+v", ov)
	}

	reqOvPost := httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/overview", nil)
	recOvPost := httptest.NewRecorder()
	srv.handleSystemicMycosesOverview(recOvPost, reqOvPost)
	if recOvPost.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST handleSystemicMycosesOverview, got %d", recOvPost.Code)
	}
}

func TestSystemicMycosesOperationsCRUDAndLifecycle(t *testing.T) {
	srv := NewServer()
	srv.systemicMycosesService = nil // test lazy initialization in ensureSystemicMycosesService

	// List operations with status and syndrome filters
	reqList := httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/operations?status=AZOLE_AMPHOTERICIN_TREPHINATION_STAGED&syndrome=BLASTOMYCOSIS_BROAD_BASED_BUDDING_YEAST", nil)
	recList := httptest.NewRecorder()
	srv.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list operations, got %d", recList.Code)
	}

	// Get single operation
	reqGet := httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/operations/MYC-OP-101", nil)
	recGet := httptest.NewRecorder()
	srv.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get operation, got %d", recGet.Code)
	}

	reqGet404 := httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/operations/MYC-OP-999", nil)
	recGet404 := httptest.NewRecorder()
	srv.ServeHTTP(recGet404, reqGet404)
	if recGet404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown operation, got %d", recGet404.Code)
	}

	// Create new operation
	createBody := map[string]any{
		"pet_name":                             "Voyageur",
		"species":                              "Canine",
		"breed":                                "German Shorthaired Pointer",
		"weight_kg":                            27.5,
		"syndrome":                             "BLASTOMYCOSIS_BROAD_BASED_BUDDING_YEAST",
		"protocol":                             "ITRACONAZOLE_5MG_KG_BRAND_OR_SOLUTION",
		"miravista_antigen_ng_ml":              12.6,
		"serum_creatinine_mg_dl":               1.25,
		"alt_activity_u_l":                     74.0,
		"spo2_percent":                         90.5,
		"cribriform_plate_intact_on_ct":        true,
		"compounded_bulk_itraconazole_blocked": true,
		"amphotericin_lipid_saline_diuresis_verified": true,
	}
	createBytes, _ := json.Marshal(createBody)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations", bytes.NewReader(createBytes))
	recCreate := httptest.NewRecorder()
	srv.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var created domain.SystemicMycosesOperation
	if err := json.NewDecoder(recCreate.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID != "MYC-OP-104" {
		t.Fatalf("expected MYC-OP-104, got %s", created.ID)
	}

	// Stage therapy — first test cribriform plate breach guard (400 Bad Request)
	badStageBody := map[string]any{
		"protocol":                             "CLOTRIMAZOLE_1PCT_FRONTAL_SINUS_INFUSION",
		"cribriform_plate_intact_on_ct":        false,
		"compounded_bulk_itraconazole_blocked": true,
	}
	badStageBytes, _ := json.Marshal(badStageBody)
	reqBadStage := httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-104/therapy", bytes.NewReader(badStageBytes))
	recBadStage := httptest.NewRecorder()
	srv.ServeHTTP(recBadStage, reqBadStage)
	if recBadStage.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for Clotrimazole trephination with cribriform plate breach, got %d", recBadStage.Code)
	}

	// Valid Stage therapy
	stageBody := map[string]any{
		"protocol":                                       "ITRACONAZOLE_5MG_KG_BRAND_OR_SOLUTION",
		"itraconazole_mg_kg_day":                         5.0,
		"fluconazole_mg_kg_day":                          20.0,
		"amphotericin_b_lipid_mg_kg_dose":                2.0,
		"compounded_bulk_itraconazole_blocked":           true,
		"cribriform_plate_intact_on_ct":                  true,
		"amphotericin_lipid_saline_diuresis_verified":    true,
		"die_off_dexamethasone_anti_inflammatory_staged": true,
		"systemic_azole_and_polyene_regimen_plan":        "FDA-approved innovator Itraconazole 5 mg/kg/day PO with food + Amphotericin B Lipid Complex 2 mg/kg IV 3x/wk with 0.9% NaCl diuresis",
		"pulmonary_die_off_ards_plan":                    "Dexamethasone-SP 0.1 mg/kg IV q24h for 3 days + oxygen cage support",
		"ophthalmic_chorioretinitis_neuro_plan":          "Serial indirect ophthalmoscopy + tonometry",
		"minimum_azole_duration_months":                  6,
		"staged_by":                                      "Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
	}
	stageBytes, _ := json.Marshal(stageBody)
	reqStage := httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-104/therapy", bytes.NewReader(stageBytes))
	recStage := httptest.NewRecorder()
	srv.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stage therapy, got %d: %s", recStage.Code, recStage.Body.String())
	}

	// Record antigen & renal check
	chkBody := map[string]any{
		"miravista_antigen_ng_ml":             0.0,
		"serum_creatinine_mg_dl":              1.15,
		"alt_activity_u_l":                    58.0,
		"spo2_percent":                        98.0,
		"cribriform_plate_intact_on_ct":       true,
		"amphotericin_dose_held_for_azotemia": false,
		"ophthalmic_chorioretinitis_resolved": true,
		"recorded_by":                         "Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
		"clinical_note":                       "Month 6 & Month 7 MiraVista urine antigen EIA both 0.00 ng/mL, Creatinine 1.15 mg/dL, ALT 58 U/L, SpO2 98.0%.",
	}
	chkBytes, _ := json.Marshal(chkBody)
	reqChk := httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-104/antigen-check", bytes.NewReader(chkBytes))
	recChk := httptest.NewRecorder()
	srv.ServeHTTP(recChk, reqChk)
	if recChk.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for antigen check, got %d: %s", recChk.Code, recChk.Body.String())
	}

	// Evaluate clearance — first test safety guard when miravista_antigen_ng_ml > 0
	badClearBody := map[string]any{
		"miravista_antigen_ng_ml":               1.2,
		"serum_creatinine_mg_dl":                1.15,
		"two_consecutive_negative_eia_verified": true,
		"minimum_6month_azole_course_verified":  true,
		"cleared_by":                            "Dr. Thibodeaux",
		"clearance_summary":                     "Attempt clearance with positive MiraVista antigen",
	}
	badClearBytes, _ := json.Marshal(badClearBody)
	reqBadClear := httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-104/clearance", bytes.NewReader(badClearBytes))
	recBadClear := httptest.NewRecorder()
	srv.ServeHTTP(recBadClear, reqBadClear)
	if recBadClear.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for clearance with positive antigen, got %d", recBadClear.Code)
	}

	// Valid clearance
	clearBody := map[string]any{
		"miravista_antigen_ng_ml":               0.0,
		"serum_creatinine_mg_dl":                1.15,
		"two_consecutive_negative_eia_verified": true,
		"minimum_6month_azole_course_verified":  true,
		"cleared_by":                            "Dr. Arlette Thibodeaux, DACVIM (Internal Medicine & Infectious Disease)",
		"clearance_summary":                     "Two consecutive negative MiraVista antigen EIA tests (0.00 ng/mL) 30 days apart, Creatinine 1.15 mg/dL, and 6-month FDA-approved Itraconazole course verified; cleared.",
	}
	clearBytes, _ := json.Marshal(clearBody)
	reqClear := httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-104/clearance", bytes.NewReader(clearBytes))
	recClear := httptest.NewRecorder()
	srv.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var clearedOp domain.SystemicMycosesOperation
	if err := json.NewDecoder(recClear.Body).Decode(&clearedOp); err != nil {
		t.Fatalf("decode clearedOp: %v", err)
	}
	if clearedOp.Status != domain.MycosesStatusTwoNegativeAntigenEIACleared {
		t.Fatalf("expected status TWO_NEGATIVE_ANTIGEN_EIA_CLEARED, got %s", clearedOp.Status)
	}
}

func TestSystemicMycosesErrorPathsAndSSEStream(t *testing.T) {
	srv := NewServer()
	srv.reunionPingInterval = 15 * time.Millisecond

	// Method not allowed & invalid JSON checks on handlers
	r1 := httptest.NewRecorder()
	srv.handleListSystemicMycosesOperations(r1, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations", nil))
	if r1.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r1.Code)
	}

	r2 := httptest.NewRecorder()
	srv.handleGetSystemicMycosesOperation(r2, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-101", nil))
	if r2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r2.Code)
	}

	r3 := httptest.NewRecorder()
	srv.handleGetSystemicMycosesOperation(r3, httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/operations/", nil))
	if r3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", r3.Code)
	}

	r4 := httptest.NewRecorder()
	srv.handleCreateSystemicMycosesOperation(r4, httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/operations", nil))
	if r4.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r4.Code)
	}

	r5 := httptest.NewRecorder()
	srv.handleCreateSystemicMycosesOperation(r5, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations", strings.NewReader("{bad")))
	if r5.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON, got %d", r5.Code)
	}

	r6 := httptest.NewRecorder()
	srv.handleStageSystemicMycosesTherapy(r6, httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/operations/MYC-OP-101/therapy", nil))
	if r6.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r6.Code)
	}

	r7 := httptest.NewRecorder()
	srv.handleStageSystemicMycosesTherapy(r7, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations//therapy", nil))
	if r7.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", r7.Code)
	}

	r8 := httptest.NewRecorder()
	srv.ServeHTTP(r8, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/UNKNOWN/therapy", strings.NewReader("{}")))
	if r8.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r8.Code)
	}

	r9 := httptest.NewRecorder()
	srv.ServeHTTP(r9, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-101/therapy", strings.NewReader("{bad")))
	if r9.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r9.Code)
	}

	r10 := httptest.NewRecorder()
	srv.handleRecordMycosesAntigenRenalCheck(r10, httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/operations/MYC-OP-101/antigen-check", nil))
	if r10.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r10.Code)
	}

	r11 := httptest.NewRecorder()
	srv.handleRecordMycosesAntigenRenalCheck(r11, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations//antigen-check", nil))
	if r11.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r11.Code)
	}

	r12 := httptest.NewRecorder()
	srv.ServeHTTP(r12, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/UNKNOWN/antigen-check", strings.NewReader("{}")))
	if r12.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r12.Code)
	}

	r13 := httptest.NewRecorder()
	srv.ServeHTTP(r13, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-101/antigen-check", strings.NewReader("{bad")))
	if r13.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r13.Code)
	}

	r14 := httptest.NewRecorder()
	srv.handleEvaluateSystemicMycosesClearance(r14, httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/operations/MYC-OP-101/clearance", nil))
	if r14.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r14.Code)
	}

	r15 := httptest.NewRecorder()
	srv.handleEvaluateSystemicMycosesClearance(r15, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations//clearance", nil))
	if r15.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r15.Code)
	}

	r16 := httptest.NewRecorder()
	srv.ServeHTTP(r16, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/UNKNOWN/clearance", strings.NewReader("{}")))
	if r16.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r16.Code)
	}

	r17 := httptest.NewRecorder()
	srv.ServeHTTP(r17, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/operations/MYC-OP-101/clearance", strings.NewReader("{bad")))
	if r17.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r17.Code)
	}

	r18 := httptest.NewRecorder()
	srv.handleSystemicMycosesStream(r18, httptest.NewRequest(http.MethodPost, "/api/systemic-mycoses/stream", nil))
	if r18.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r18.Code)
	}

	// SSE stream test
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()
	reqSSE := httptest.NewRequest(http.MethodGet, "/api/systemic-mycoses/stream", nil).WithContext(ctx)
	recSSE := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.ServeHTTP(recSSE, reqSSE)
	}()

	time.Sleep(10 * time.Millisecond)
	srv.broadcastSystemicMycosesEvent("systemic_mycoses_updated", map[string]any{"id": "MYC-OP-101"}, time.Now().UTC())
	<-done

	if !strings.Contains(recSSE.Body.String(), ": connected") {
		t.Fatalf("expected SSE connected comment, got %s", recSSE.Body.String())
	}
}
