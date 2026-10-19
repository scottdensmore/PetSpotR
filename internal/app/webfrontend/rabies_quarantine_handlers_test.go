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

func TestRabiesQuarantineTriagePageAndOverview(t *testing.T) {
	srv := NewServer()

	// GET /rabies-quarantine-pep-triage
	req := httptest.NewRequest(http.MethodGet, "/rabies-quarantine-pep-triage", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /rabies-quarantine-pep-triage, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"rabies-quarantine-cockpit",
		"RBQ-OP-101",
		"Koda",
		"RBQ-OP-102",
		"Vesper",
		"RBQ-OP-103",
		"Artemis",
		"rbq-alert-banner",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected page HTML to contain %q", want)
		}
	}

	// HEAD /rabies-quarantine-pep-triage
	reqHead := httptest.NewRequest(http.MethodHead, "/rabies-quarantine-pep-triage", nil)
	recHead := httptest.NewRecorder()
	srv.ServeHTTP(recHead, reqHead)
	if recHead.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HEAD /rabies-quarantine-pep-triage, got %d", recHead.Code)
	}

	// Direct method not allowed check
	reqPostPage := httptest.NewRequest(http.MethodPost, "/rabies-quarantine-pep-triage", nil)
	recPostPage := httptest.NewRecorder()
	srv.handleRabiesQuarantineTriagePage(recPostPage, reqPostPage)
	if recPostPage.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST handleRabiesQuarantineTriagePage, got %d", recPostPage.Code)
	}

	// GET /api/rabies-quarantine/overview
	reqOv := httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/overview", nil)
	recOv := httptest.NewRecorder()
	srv.ServeHTTP(recOv, reqOv)
	if recOv.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for overview, got %d", recOv.Code)
	}
	var ov domain.RabiesQuarantineOverview
	if err := json.NewDecoder(recOv.Body).Decode(&ov); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if ov.TotalOperations != 3 || ov.TenDayDomesticBiteHoldCount != 1 {
		t.Fatalf("unexpected overview: %+v", ov)
	}

	reqOvPost := httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/overview", nil)
	recOvPost := httptest.NewRecorder()
	srv.handleRabiesQuarantineOverview(recOvPost, reqOvPost)
	if recOvPost.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST handleRabiesQuarantineOverview, got %d", recOvPost.Code)
	}
}

func TestRabiesQuarantineOperationsCRUDAndLifecycle(t *testing.T) {
	srv := NewServer()
	srv.rabiesQuarantineService = nil // test lazy initialization in ensureRabiesQuarantineService

	// List operations with status and statutory_category filters
	reqList := httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/operations?status=BIOSECURITY_ISOLATION_RCO_HOLD_STAGED&statutory_category=TEN_DAY_DOMESTIC_BITE_CONFINEMENT", nil)
	recList := httptest.NewRecorder()
	srv.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list operations, got %d", recList.Code)
	}

	// Get single operation
	reqGet := httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/operations/RBQ-OP-101", nil)
	recGet := httptest.NewRecorder()
	srv.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get operation, got %d", recGet.Code)
	}

	reqGet404 := httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/operations/RBQ-OP-999", nil)
	recGet404 := httptest.NewRecorder()
	srv.ServeHTTP(recGet404, reqGet404)
	if recGet404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown operation, got %d", recGet404.Code)
	}

	// Create new operation
	createBody := map[string]any{
		"pet_name":                 "Zephyr",
		"species":                  "Canine",
		"breed":                    "Belgian Malinois",
		"weight_kg":                29.5,
		"statutory_category":       "TEN_DAY_DOMESTIC_BITE_CONFINEMENT",
		"protocol":                 "STRICT_TEN_DAY_NO_VACCINE_DURING_BITE_HOLD",
		"rco_case_filing_number":   "RCO-STAT-2026-104",
		"quarantine_day_elapsed":   1,
		"required_quarantine_days": 10,
		"rffit_titer_iu_ml":        0.40,
		"no_vaccine_during_10day_bite_hold_verified": true,
		"double_barrier_locked_enclosure_verified":   true,
	}
	createBytes, _ := json.Marshal(createBody)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations", bytes.NewReader(createBytes))
	recCreate := httptest.NewRecorder()
	srv.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var created domain.RabiesQuarantineOperation
	if err := json.NewDecoder(recCreate.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID != "RBQ-OP-104" {
		t.Fatalf("expected RBQ-OP-104, got %s", created.ID)
	}

	// Stage quarantine — first test NASPHV 10-day bite hold vaccine prohibition guard (400 Bad Request)
	badStageBody := map[string]any{
		"protocol": "STRICT_TEN_DAY_NO_VACCINE_DURING_BITE_HOLD",
		"no_vaccine_during_10day_bite_hold_verified": false,
	}
	badStageBytes, _ := json.Marshal(badStageBody)
	reqBadStage := httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-104/staging", bytes.NewReader(badStageBytes))
	recBadStage := httptest.NewRecorder()
	srv.ServeHTTP(recBadStage, reqBadStage)
	if recBadStage.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for NASPHV 10-day bite hold vaccine violation, got %d", recBadStage.Code)
	}

	// Valid Stage quarantine
	stageBody := map[string]any{
		"protocol":                                   "STRICT_TEN_DAY_NO_VACCINE_DURING_BITE_HOLD",
		"rco_case_filing_number":                     "RCO-STAT-2026-104",
		"required_quarantine_days":                   10,
		"no_vaccine_during_10day_bite_hold_verified": true,
		"double_barrier_locked_enclosure_verified":   true,
		"double_barrier_biosecurity_ppe_plan":        "Padlocked double-barrier isolation run + PPE + Kevlar gauntlets",
		"virucidal_wound_lavage_booster_pep_plan":    "15-minute virucidal 1% povidone-iodine lavage; withhold canine vaccine until Day 10 release",
		"prospective_serologic_rffit_plan":           "Paired Day 0 and Day 5 RFFIT neutralizing antibody serum titers >= 0.50 IU/mL",
		"brain_stem_hippocampus_preserved_guard":     true,
		"staged_by":                                  "Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
	}
	stageBytes, _ := json.Marshal(stageBody)
	reqStage := httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-104/staging", bytes.NewReader(stageBytes))
	recStage := httptest.NewRecorder()
	srv.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stage quarantine, got %d: %s", recStage.Code, recStage.Body.String())
	}

	// Record neuro check
	chkBody := map[string]any{
		"quarantine_day_elapsed":                     10,
		"rffit_titer_iu_ml":                          1.45,
		"cranial_nerve_bulbar_palsy_absent":          true,
		"hypersalivation_dysphagia_absent":           true,
		"double_barrier_locked_enclosure_verified":   true,
		"no_vaccine_during_10day_bite_hold_verified": true,
		"recorded_by":                                "Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		"clinical_note":                              "Day 10 of 10 statutory domestic bite observation complete: cranial nerves intact, no bulbar palsy or dysphagia.",
	}
	chkBytes, _ := json.Marshal(chkBody)
	reqChk := httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-104/neuro-check", bytes.NewReader(chkBytes))
	recChk := httptest.NewRecorder()
	srv.ServeHTTP(recChk, reqChk)
	if recChk.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for neuro check, got %d: %s", recChk.Code, recChk.Body.String())
	}

	// Evaluate clearance — first test safety guard when cranial_nerve_bulbar_palsy_absent is false
	badClearBody := map[string]any{
		"quarantine_day_elapsed":            10,
		"cranial_nerve_bulbar_palsy_absent": false,
		"rco_written_release_signed":        true,
		"exit_rabies_vaccination_verified":  true,
		"rffit_titer_iu_ml":                 1.45,
		"cleared_by":                        "Dr. Thibodeaux",
		"clearance_summary":                 "Attempt clearance with bulbar palsy present",
	}
	badClearBytes, _ := json.Marshal(badClearBody)
	reqBadClear := httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-104/clearance", bytes.NewReader(badClearBytes))
	recBadClear := httptest.NewRecorder()
	srv.ServeHTTP(recBadClear, reqBadClear)
	if recBadClear.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for clearance with bulbar palsy present, got %d", recBadClear.Code)
	}

	// Valid clearance
	clearBody := map[string]any{
		"quarantine_day_elapsed":            10,
		"cranial_nerve_bulbar_palsy_absent": true,
		"rco_written_release_signed":        true,
		"exit_rabies_vaccination_verified":  true,
		"rffit_titer_iu_ml":                 1.45,
		"cleared_by":                        "Dr. Celeste Thibodeaux, DVM, MPH, DACVPM (Rabies Control Officer)",
		"clearance_summary":                 "Survived full 10-day NASPHV domestic bite observation (Day 10/10), cranial nerve/bulbar palsy absent, RCO release signed, exit rabies vaccine administered; cleared.",
	}
	clearBytes, _ := json.Marshal(clearBody)
	reqClear := httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-104/clearance", bytes.NewReader(clearBytes))
	recClear := httptest.NewRecorder()
	srv.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var clearedOp domain.RabiesQuarantineOperation
	if err := json.NewDecoder(recClear.Body).Decode(&clearedOp); err != nil {
		t.Fatalf("decode clearedOp: %v", err)
	}
	if clearedOp.Status != domain.RabiesStatusStatutoryQuarantineSurvivedCleared {
		t.Fatalf("expected status STATUTORY_QUARANTINE_SURVIVED_CLEARED, got %s", clearedOp.Status)
	}
}

func TestRabiesQuarantineErrorPathsAndSSEStream(t *testing.T) {
	srv := NewServer()
	srv.reunionPingInterval = 15 * time.Millisecond

	r1 := httptest.NewRecorder()
	srv.handleListRabiesQuarantineOperations(r1, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations", nil))
	if r1.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r1.Code)
	}

	r2 := httptest.NewRecorder()
	srv.handleGetRabiesQuarantineOperation(r2, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-101", nil))
	if r2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r2.Code)
	}

	r3 := httptest.NewRecorder()
	srv.handleGetRabiesQuarantineOperation(r3, httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/operations/", nil))
	if r3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", r3.Code)
	}

	r4 := httptest.NewRecorder()
	srv.handleCreateRabiesQuarantineOperation(r4, httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/operations", nil))
	if r4.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r4.Code)
	}

	r5 := httptest.NewRecorder()
	srv.handleCreateRabiesQuarantineOperation(r5, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations", strings.NewReader("{bad")))
	if r5.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON, got %d", r5.Code)
	}

	r6 := httptest.NewRecorder()
	srv.handleStageRabiesQuarantine(r6, httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/operations/RBQ-OP-101/staging", nil))
	if r6.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r6.Code)
	}

	r7 := httptest.NewRecorder()
	srv.handleStageRabiesQuarantine(r7, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations//staging", nil))
	if r7.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", r7.Code)
	}

	r8 := httptest.NewRecorder()
	srv.ServeHTTP(r8, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/UNKNOWN/staging", strings.NewReader("{}")))
	if r8.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r8.Code)
	}

	r9 := httptest.NewRecorder()
	srv.ServeHTTP(r9, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-101/staging", strings.NewReader("{bad")))
	if r9.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r9.Code)
	}

	r10 := httptest.NewRecorder()
	srv.handleRecordRabiesNeuroCheck(r10, httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/operations/RBQ-OP-101/neuro-check", nil))
	if r10.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r10.Code)
	}

	r11 := httptest.NewRecorder()
	srv.handleRecordRabiesNeuroCheck(r11, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations//neuro-check", nil))
	if r11.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r11.Code)
	}

	r12 := httptest.NewRecorder()
	srv.ServeHTTP(r12, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/UNKNOWN/neuro-check", strings.NewReader("{}")))
	if r12.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r12.Code)
	}

	r13 := httptest.NewRecorder()
	srv.ServeHTTP(r13, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-101/neuro-check", strings.NewReader("{bad")))
	if r13.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r13.Code)
	}

	r14 := httptest.NewRecorder()
	srv.handleEvaluateRabiesClearance(r14, httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/operations/RBQ-OP-101/clearance", nil))
	if r14.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r14.Code)
	}

	r15 := httptest.NewRecorder()
	srv.handleEvaluateRabiesClearance(r15, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations//clearance", nil))
	if r15.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r15.Code)
	}

	r16 := httptest.NewRecorder()
	srv.ServeHTTP(r16, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/UNKNOWN/clearance", strings.NewReader("{}")))
	if r16.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r16.Code)
	}

	r17 := httptest.NewRecorder()
	srv.ServeHTTP(r17, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/operations/RBQ-OP-101/clearance", strings.NewReader("{bad")))
	if r17.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r17.Code)
	}

	r18 := httptest.NewRecorder()
	srv.handleRabiesQuarantineStream(r18, httptest.NewRequest(http.MethodPost, "/api/rabies-quarantine/stream", nil))
	if r18.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r18.Code)
	}

	// SSE stream test
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()
	reqSSE := httptest.NewRequest(http.MethodGet, "/api/rabies-quarantine/stream", nil).WithContext(ctx)
	recSSE := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.ServeHTTP(recSSE, reqSSE)
	}()

	time.Sleep(10 * time.Millisecond)
	srv.broadcastRabiesQuarantineEvent("rabies_quarantine_updated", map[string]any{"id": "RBQ-OP-101"}, time.Now().UTC())
	<-done

	if !strings.Contains(recSSE.Body.String(), ": connected") {
		t.Fatalf("expected SSE connected comment, got %s", recSSE.Body.String())
	}
}
