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

func TestMassCasualtyVMATTriagePageAndOverview(t *testing.T) {
	srv := NewServer()

	// GET /mass-casualty-vmat-triage
	req := httptest.NewRequest(http.MethodGet, "/mass-casualty-vmat-triage", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /mass-casualty-vmat-triage, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"mass-casualty-vmat-cockpit",
		"MCI-OP-101",
		"Kodiak",
		"MCI-OP-102",
		"Cinder",
		"MCI-OP-103",
		"Shasta",
		"mci-alert-banner",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected page HTML to contain %q", want)
		}
	}

	// HEAD /mass-casualty-vmat-triage
	reqHead := httptest.NewRequest(http.MethodHead, "/mass-casualty-vmat-triage", nil)
	recHead := httptest.NewRecorder()
	srv.ServeHTTP(recHead, reqHead)
	if recHead.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HEAD /mass-casualty-vmat-triage, got %d", recHead.Code)
	}

	// Direct method not allowed check
	reqPostPage := httptest.NewRequest(http.MethodPost, "/mass-casualty-vmat-triage", nil)
	recPostPage := httptest.NewRecorder()
	srv.handleMassCasualtyVMATTriagePage(recPostPage, reqPostPage)
	if recPostPage.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST handleMassCasualtyVMATTriagePage, got %d", recPostPage.Code)
	}

	// GET /api/mass-casualty-vmat/overview
	reqOv := httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/overview", nil)
	recOv := httptest.NewRecorder()
	srv.ServeHTTP(recOv, reqOv)
	if recOv.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for overview, got %d", recOv.Code)
	}
	var ov domain.MassCasualtyVMATOverview
	if err := json.NewDecoder(recOv.Body).Decode(&ov); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if ov.TotalOperations != 3 || ov.RedImmediatePriority1Count != 1 {
		t.Fatalf("unexpected overview: %+v", ov)
	}

	reqOvPost := httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/overview", nil)
	recOvPost := httptest.NewRecorder()
	srv.handleMassCasualtyVMATOverview(recOvPost, reqOvPost)
	if recOvPost.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for POST handleMassCasualtyVMATOverview, got %d", recOvPost.Code)
	}
}

func TestMassCasualtyVMATOperationsCRUDAndLifecycle(t *testing.T) {
	srv := NewServer()
	srv.massCasualtyVMATService = nil // test lazy initialization in ensureMassCasualtyVMATService

	// List operations with status and triage_tag filters
	reqList := httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/operations?status=START_SALT_TAGGED_DECON_STAGED&triage_tag=RED_IMMEDIATE_PRIORITY_1_LIFE_THREAT", nil)
	recList := httptest.NewRecorder()
	srv.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for list operations, got %d", recList.Code)
	}

	// Get single operation
	reqGet := httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/operations/MCI-OP-101", nil)
	recGet := httptest.NewRecorder()
	srv.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get operation, got %d", recGet.Code)
	}

	reqGet404 := httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/operations/MCI-OP-999", nil)
	recGet404 := httptest.NewRecorder()
	srv.ServeHTTP(recGet404, reqGet404)
	if recGet404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown operation, got %d", recGet404.Code)
	}

	// Create new operation
	createBody := map[string]any{
		"pet_name":                             "Cascade",
		"species":                              "Canine",
		"breed":                                "German Shepherd (SAR K9)",
		"weight_kg":                            32.0,
		"triage_tag":                           "RED_IMMEDIATE_PRIORITY_1_LIFE_THREAT",
		"protocol":                             "HOT_WARM_COLD_ZONE_DAWN_DECON_CORRIDOR",
		"animal_triage_score_ats":              18,
		"lactate_mmol_l":                       6.4,
		"spo2_percent":                         88.0,
		"surge_bed_occupancy_percent":          92.0,
		"waterproof_rfid_triage_collar_number": "VMAT-RFID-9104",
		"ics214_incident_number":               "ICS214-FEMA-2026-104",
		"gross_decontamination_prior_to_cold_zone_verified": true,
		"contagious_cohort_segregated":                      true,
	}
	createBytes, _ := json.Marshal(createBody)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations", bytes.NewReader(createBytes))
	recCreate := httptest.NewRecorder()
	srv.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}
	var created domain.MassCasualtyVMATOperation
	if err := json.NewDecoder(recCreate.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID != "MCI-OP-104" {
		t.Fatalf("expected MCI-OP-104, got %s", created.ID)
	}

	// Stage VMAT field triage — first test Hot-to-Cold Zone Cross-Contamination Guard (400 Bad Request)
	badStageBody := map[string]any{
		"protocol": "HOT_WARM_COLD_ZONE_DAWN_DECON_CORRIDOR",
		"gross_decontamination_prior_to_cold_zone_verified": false,
	}
	badStageBytes, _ := json.Marshal(badStageBody)
	reqBadStage := httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-104/staging", bytes.NewReader(badStageBytes))
	recBadStage := httptest.NewRecorder()
	srv.ServeHTTP(recBadStage, reqBadStage)
	if recBadStage.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for unverified gross decon, got %d", recBadStage.Code)
	}

	// Valid Stage VMAT field triage
	stageBody := map[string]any{
		"protocol":   "HOT_WARM_COLD_ZONE_DAWN_DECON_CORRIDOR",
		"triage_tag": "RED_IMMEDIATE_PRIORITY_1_LIFE_THREAT",
		"warm_zone_decontamination_corridor_plan":           "Warm-zone Dawn surfactant wash & sterile saline ocular flush",
		"damage_control_resuscitation_plan":                 "Direct pressure + TXA 10 mg/kg IV + Hypertonic Saline 4 mL/kg IV",
		"waterproof_rfid_triage_collar_number":              "VMAT-RFID-9104",
		"ics214_incident_number":                            "ICS214-FEMA-2026-104",
		"surge_bed_occupancy_percent":                       92.0,
		"gross_decontamination_prior_to_cold_zone_verified": true,
		"contagious_cohort_segregated":                      true,
		"staged_by":                                         "Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
	}
	stageBytes, _ := json.Marshal(stageBody)
	reqStage := httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-104/staging", bytes.NewReader(stageBytes))
	recStage := httptest.NewRecorder()
	srv.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for stage VMAT triage, got %d: %s", recStage.Code, recStage.Body.String())
	}

	// Record reassessment check
	chkBody := map[string]any{
		"triage_tag":                  "YELLOW_DELAYED_PRIORITY_2_URGENT_SURGERY",
		"animal_triage_score_ats":     5,
		"lactate_mmol_l":              1.85,
		"spo2_percent":                97.5,
		"surge_bed_occupancy_percent": 78.0,
		"gross_decontamination_prior_to_cold_zone_verified": true,
		"contagious_cohort_segregated":                      true,
		"recorded_by":                                       "Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		"clinical_note":                                     "Post-resuscitation check: arterial hemorrhage controlled, ATS 5/25, lactate 1.85 mmol/L, SpO2 97.5%.",
	}
	chkBytes, _ := json.Marshal(chkBody)
	reqChk := httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-104/reassessment", bytes.NewReader(chkBytes))
	recChk := httptest.NewRecorder()
	srv.ServeHTTP(recChk, reqChk)
	if recChk.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for reassessment check, got %d: %s", recChk.Code, recChk.Body.String())
	}

	// Evaluate clearance — first test safety guard when waterproof_rfid_matched_verified is false
	badClearBody := map[string]any{
		"lactate_mmol_l":                   1.85,
		"spo2_percent":                     97.5,
		"waterproof_rfid_matched_verified": false,
		"ics214_chain_of_custody_signed":   true,
		"cleared_by":                       "Dr. Vance",
		"clearance_summary":                "Attempt clearance without RFID match",
	}
	badClearBytes, _ := json.Marshal(badClearBody)
	reqBadClear := httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-104/clearance", bytes.NewReader(badClearBytes))
	recBadClear := httptest.NewRecorder()
	srv.ServeHTTP(recBadClear, reqBadClear)
	if recBadClear.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for clearance without RFID match, got %d", recBadClear.Code)
	}

	// Valid clearance
	clearBody := map[string]any{
		"lactate_mmol_l":                   1.85,
		"spo2_percent":                     97.5,
		"waterproof_rfid_matched_verified": true,
		"ics214_chain_of_custody_signed":   true,
		"cleared_by":                       "Dr. Cheyenne Vance, DVM, DACVECC (FEMA VMAT Incident Commander)",
		"clearance_summary":                "Perfusion normalized (lactate 1.85 mmol/L), SpO2 97.5%, RFID collar VMAT-RFID-9104 matched, and FEMA ICS-214 signed; cleared.",
	}
	clearBytes, _ := json.Marshal(clearBody)
	reqClear := httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-104/clearance", bytes.NewReader(clearBytes))
	recClear := httptest.NewRecorder()
	srv.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for clearance, got %d: %s", recClear.Code, recClear.Body.String())
	}
	var clearedOp domain.MassCasualtyVMATOperation
	if err := json.NewDecoder(recClear.Body).Decode(&clearedOp); err != nil {
		t.Fatalf("decode clearedOp: %v", err)
	}
	if clearedOp.Status != domain.MassCasualtyVMATStatusReunifiedEvacTransportCleared {
		t.Fatalf("expected status REUNIFIED_EVAC_TRANSPORT_CLEARED, got %s", clearedOp.Status)
	}
}

func TestMassCasualtyVMATErrorPathsAndSSEStream(t *testing.T) {
	srv := NewServer()
	srv.reunionPingInterval = 15 * time.Millisecond

	// Method not allowed & invalid JSON checks on handlers
	r1 := httptest.NewRecorder()
	srv.handleListMassCasualtyVMATOperations(r1, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations", nil))
	if r1.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r1.Code)
	}

	r2 := httptest.NewRecorder()
	srv.handleGetMassCasualtyVMATOperation(r2, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-101", nil))
	if r2.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r2.Code)
	}

	r3 := httptest.NewRecorder()
	srv.handleGetMassCasualtyVMATOperation(r3, httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/operations/", nil))
	if r3.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", r3.Code)
	}

	r4 := httptest.NewRecorder()
	srv.handleCreateMassCasualtyVMATOperation(r4, httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/operations", nil))
	if r4.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r4.Code)
	}

	r5 := httptest.NewRecorder()
	srv.handleCreateMassCasualtyVMATOperation(r5, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations", strings.NewReader("{bad")))
	if r5.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON, got %d", r5.Code)
	}

	r6 := httptest.NewRecorder()
	srv.handleStageMassCasualtyVMAT(r6, httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/operations/MCI-OP-101/staging", nil))
	if r6.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r6.Code)
	}

	r7 := httptest.NewRecorder()
	srv.handleStageMassCasualtyVMAT(r7, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations//staging", nil))
	if r7.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty id, got %d", r7.Code)
	}

	r8 := httptest.NewRecorder()
	srv.ServeHTTP(r8, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/UNKNOWN/staging", strings.NewReader("{}")))
	if r8.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r8.Code)
	}

	r9 := httptest.NewRecorder()
	srv.ServeHTTP(r9, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-101/staging", strings.NewReader("{bad")))
	if r9.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r9.Code)
	}

	r10 := httptest.NewRecorder()
	srv.handleRecordMassCasualtyVMATReassessment(r10, httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/operations/MCI-OP-101/reassessment", nil))
	if r10.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r10.Code)
	}

	r11 := httptest.NewRecorder()
	srv.handleRecordMassCasualtyVMATReassessment(r11, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations//reassessment", nil))
	if r11.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r11.Code)
	}

	r12 := httptest.NewRecorder()
	srv.ServeHTTP(r12, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/UNKNOWN/reassessment", strings.NewReader("{}")))
	if r12.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r12.Code)
	}

	r13 := httptest.NewRecorder()
	srv.ServeHTTP(r13, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-101/reassessment", strings.NewReader("{bad")))
	if r13.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r13.Code)
	}

	r14 := httptest.NewRecorder()
	srv.handleEvaluateMassCasualtyVMATClearance(r14, httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/operations/MCI-OP-101/clearance", nil))
	if r14.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r14.Code)
	}

	r15 := httptest.NewRecorder()
	srv.handleEvaluateMassCasualtyVMATClearance(r15, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations//clearance", nil))
	if r15.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r15.Code)
	}

	r16 := httptest.NewRecorder()
	srv.ServeHTTP(r16, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/UNKNOWN/clearance", strings.NewReader("{}")))
	if r16.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", r16.Code)
	}

	r17 := httptest.NewRecorder()
	srv.ServeHTTP(r17, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/operations/MCI-OP-101/clearance", strings.NewReader("{bad")))
	if r17.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", r17.Code)
	}

	r18 := httptest.NewRecorder()
	srv.handleMassCasualtyVMATStream(r18, httptest.NewRequest(http.MethodPost, "/api/mass-casualty-vmat/stream", nil))
	if r18.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", r18.Code)
	}

	// SSE stream test
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()
	reqSSE := httptest.NewRequest(http.MethodGet, "/api/mass-casualty-vmat/stream", nil).WithContext(ctx)
	recSSE := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.ServeHTTP(recSSE, reqSSE)
	}()

	time.Sleep(10 * time.Millisecond)
	srv.broadcastMassCasualtyVMATEvent("mass_casualty_vmat_updated", map[string]any{"id": "MCI-OP-101"}, time.Now().UTC())
	<-done

	if !strings.Contains(recSSE.Body.String(), ": connected") {
		t.Fatalf("expected SSE connected comment, got %s", recSSE.Body.String())
	}
}
