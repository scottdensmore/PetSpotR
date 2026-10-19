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
)

func TestSpinalIVDDHandlers_Page(t *testing.T) {
	srv := NewServer()

	req := httptest.NewRequest(http.MethodGet, "/spinal-ivdd-triage", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	body := rec.Body.String()
	requiredIDs := []string{
		"spinal-ivdd-cockpit",
		"ivdd-alert-banner",
		"ivdd-status-filter",
		"connect-sse-btn",
		"sse-status-badge",
		"ivdd-operations-grid",
		"ivdd-active-workspace",
		"active-op-title",
		"active-op-status-badge",
		"active-frankel-grade",
		"active-cutaneous-trunci",
		"stage-decompression-form",
		"stage-decompression-btn",
		"record-neuro-check-form",
		"neuro-frankel-input",
		"neuro-deep-pain-input",
		"neuro-trunci-input",
		"neuro-bladder-input",
		"neuro-recorded-by-input",
		"neuro-note-input",
		"record-neuro-check-btn",
		"neuro-checks-list",
		"clear-ivdd-form",
		"clear-discharged-by-input",
		"clear-summary-input",
		"clear-ivdd-btn",
		"ivdd-clearance-banner",
		"clearance-summary-text",
	}
	for _, id := range requiredIDs {
		if !strings.Contains(body, `id="`+id+`"`) {
			t.Errorf("expected HTML to contain id=%q", id)
		}
	}
}

func TestSpinalIVDDHandlers_APIRoutes(t *testing.T) {
	srv := NewServer()

	// 1. GET /api/spinal-ivdd/operations
	req := httptest.NewRequest(http.MethodGet, "/api/spinal-ivdd/operations", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on list operations, got %d", rec.Code)
	}

	// Filter by status and localization_zone
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/spinal-ivdd/operations?status=ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE&localization_zone=T3_L3_THORACOLUMBAR_UMN", nil)
	recFilter := httptest.NewRecorder()
	srv.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on filtered list, got %d", recFilter.Code)
	}

	// 2. POST /api/spinal-ivdd/operations
	createPayload := map[string]any{
		"case_id":                    "CASE-IVDD-900",
		"pet_name":                   "Buster",
		"species":                    "Canine",
		"breed":                      "French Bulldog",
		"weight_kg":                  11.5,
		"frankel_grade":              "GRADE_5_PARAPLEGIA_DEEP_PAIN_ABSENT",
		"localization_zone":          "T3_L3_THORACOLUMBAR_UMN",
		"onset_hours":                5.0,
		"cutaneous_trunci_vertebra":  "L1 (Caudal Absent)",
		"deep_pain_present":          false,
		"schiff_sherrington_present": true,
		"prior_steroid_or_nsaid":     "Carprofen 2.2 mg/kg PO 4h ago",
		"surgical_window_hours":      12.0,
		"target_map_mmhg":            85,
	}
	b, _ := json.Marshal(createPayload)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/spinal-ivdd/operations", bytes.NewReader(b))
	recCreate := httptest.NewRecorder()
	srv.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	// Invalid POST
	reqBadPost := httptest.NewRequest(http.MethodPost, "/api/spinal-ivdd/operations", strings.NewReader("{bad"))
	recBadPost := httptest.NewRecorder()
	srv.ServeHTTP(recBadPost, reqBadPost)
	if recBadPost.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", recBadPost.Code)
	}

	// 3. GET /api/spinal-ivdd/operations/IVDD-OP-101
	reqGet := httptest.NewRequest(http.MethodGet, "/api/spinal-ivdd/operations/IVDD-OP-101", nil)
	recGet := httptest.NewRecorder()
	srv.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on get operation, got %d", recGet.Code)
	}

	// Not found GET
	reqNotFound := httptest.NewRequest(http.MethodGet, "/api/spinal-ivdd/operations/NON-EXISTENT", nil)
	recNotFound := httptest.NewRecorder()
	srv.ServeHTTP(recNotFound, reqNotFound)
	if recNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", recNotFound.Code)
	}

	// 4. POST /api/spinal-ivdd/operations/IVDD-OP-101/stage-decompression
	stagePayload := map[string]any{
		"surgical_window_target":      "Emergent Decompression < 12h (Grade 5 Deep Pain Absent); Target MAP >= 85 mmHg",
		"imaging_modality":            "High-Field 1.5T Thoracolumbar MRI + Helical CT",
		"hemilaminectomy_site":        "Left T13-L1 Hemilaminectomy + Prophylactic L1-L2 Fenestration",
		"bladder_management_protocol": "UMN Bladder: Prazosin 0.05 mg/kg PO q8h + Diazepam 0.2 mg/kg IV pre-expression q6h",
		"analgesia_cri_protocol":      "Fentanyl CRI 3 mcg/kg/h + Ketamine CRI 2 mcg/kg/min + Gabapentin 10 mg/kg PO q8h",
		"strict_crate_rest_protocol":  "Strict 4-6 week confined crate rest, padded turning q4h, PROM physiotherapy",
	}
	stageBytes, _ := json.Marshal(stagePayload)
	reqStage := httptest.NewRequest(http.MethodPost, "/api/spinal-ivdd/operations/IVDD-OP-101/stage-decompression", bytes.NewReader(stageBytes))
	recStage := httptest.NewRecorder()
	srv.ServeHTTP(recStage, reqStage)
	if recStage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on stage decompression, got %d: %s", recStage.Code, recStage.Body.String())
	}

	// 5. POST /api/spinal-ivdd/operations/IVDD-OP-101/record-check
	checkPayload := map[string]any{
		"frankel_grade":          "GRADE_3_NON_AMBULATORY_PARAPARESIS",
		"deep_pain_status":       "Intact Bilaterally — Conscious Head Turn & Vocalization Restored",
		"cutaneous_trunci_level": "L2 (Caudal Improvement — Zero Cranial Migration)",
		"bladder_tone_status":    "Normotonic Detrusor — Voluntary Voiding Stream Observed",
		"recorded_by":            "Dr. Elena Rostova, DACVIM (Neurology)",
		"clinical_note":          "Early deep pain response returning in left pelvic digits",
	}
	checkBytes, _ := json.Marshal(checkPayload)
	reqCheck := httptest.NewRequest(http.MethodPost, "/api/spinal-ivdd/operations/IVDD-OP-101/record-check", bytes.NewReader(checkBytes))
	recCheck := httptest.NewRecorder()
	srv.ServeHTTP(recCheck, reqCheck)
	if recCheck.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on record check, got %d: %s", recCheck.Code, recCheck.Body.String())
	}

	// 6. POST /api/spinal-ivdd/operations/IVDD-OP-101/clear-discharge
	clearPayload := map[string]any{
		"discharged_by":     "Dr. Elena Rostova, DACVIM (Neurology)",
		"discharge_summary": "Deep pain positive, voluntary urination restored, cleared for step-down crate rest and outpatient Underwater Treadmill rehab.",
	}
	clearBytes, _ := json.Marshal(clearPayload)
	reqClear := httptest.NewRequest(http.MethodPost, "/api/spinal-ivdd/operations/IVDD-OP-101/clear-discharge", bytes.NewReader(clearBytes))
	recClear := httptest.NewRecorder()
	srv.ServeHTTP(recClear, reqClear)
	if recClear.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on clear discharge, got %d: %s", recClear.Code, recClear.Body.String())
	}
}

func TestSpinalIVDDHandlers_SSEStream(t *testing.T) {
	srv := NewServer()

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/spinal-ivdd/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		srv.ServeHTTP(rec, req)
		close(done)
	}()

	time.Sleep(30 * time.Millisecond)
	srv.broadcastSpinalIVDDEvent("spinal_ivdd_updated", map[string]any{"id": "IVDD-OP-101"}, time.Now().UTC())
	<-done

	body := rec.Body.String()
	if !strings.Contains(body, ": connected") {
		t.Errorf("expected SSE connected comment, got %s", body)
	}
	if !strings.Contains(body, "event: spinal_ivdd_updated") {
		t.Errorf("expected SSE spinal_ivdd_updated event, got %s", body)
	}
}
