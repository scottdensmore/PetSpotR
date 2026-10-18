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

func TestReunionDecompressionHandlers_SeedingCreateProgressAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Subscribe to SSE "reunion_decompression" topic and Mesh signaling hub
	decompSSECh, unsubSSE := server.ReunionHub().Subscribe("reunion_decompression")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("reunion_decompression", "reunion-decompression-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /reunion-decompression should render HTML and seed default plans
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/reunion-decompression", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /reunion-decompression, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Post-Reunion Cortisol Decompression") {
		t.Errorf("expected /reunion-decompression HTML to contain Post-Reunion Cortisol Decompression heading")
	}

	// 2. GET /api/v1/reunion-decompression/plans should return the 3 seeded plans
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/reunion-decompression/plans", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/reunion-decompression/plans, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Plans []domain.ReunionDecompressionPlan `json:"plans"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal decompression plans response: %v", err)
	}
	if len(listResp.Plans) != 3 {
		t.Fatalf("expected 3 seeded decompression plans, got %d", len(listResp.Plans))
	}

	byID := make(map[string]domain.ReunionDecompressionPlan)
	for _, p := range listResp.Plans {
		byID[p.PlanID] = p
	}

	p101, ok := byID["decomp-plan-101"]
	if !ok {
		t.Fatalf("expected seeded decomp-plan-101")
	}
	if p101.PetName != "Luna" || p101.Phase != domain.DecompPhaseAcuteRefuge72H || p101.RecidivismRisk != domain.RecidivismRiskCriticalFlight || !p101.RefeedingSyndromeWarning {
		t.Errorf("expected decomp-plan-101 Luna in ACUTE_REFUGE_72H / CRITICAL_FLIGHT_RISK with RefeedingSyndromeWarning=true, got %+v", p101)
	}

	p102, ok := byID["decomp-plan-102"]
	if !ok {
		t.Fatalf("expected seeded decomp-plan-102")
	}
	if p102.PetName != "Buster" || p102.Phase != domain.DecompPhaseCortisolTaperDay4To7 || p102.RecidivismRisk != domain.RecidivismRiskElevatedPerimeter {
		t.Errorf("expected decomp-plan-102 Buster in CORTISOL_TAPER_DAY_4_7 / ELEVATED_PERIMETER_RISK, got %+v", p102)
	}

	p103, ok := byID["decomp-plan-103"]
	if !ok {
		t.Fatalf("expected seeded decomp-plan-103")
	}
	if p103.PetName != "Cooper" || p103.Phase != domain.DecompPhaseGraduatedStable || p103.RecidivismRisk != domain.RecidivismRiskFortifiedLow || p103.GraduationCertificateID == "" {
		t.Errorf("expected decomp-plan-103 Cooper in GRADUATED_STABLE / FORTIFIED_LOW_RISK with GraduationCertificateID, got %+v", p103)
	}

	// 2b. Filter by petId and phase
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/reunion-decompression/plans?petId=pet-2026-002&phase=ACUTE_REFUGE_72H", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/reunion-decompression/plans, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Plans []domain.ReunionDecompressionPlan `json:"plans"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered plans: %v", err)
	}
	if len(filteredResp.Plans) != 1 || filteredResp.Plans[0].PlanID != "decomp-plan-101" {
		t.Errorf("expected 1 filtered plan (decomp-plan-101), got %+v", filteredResp.Plans)
	}

	// 3. POST /api/v1/reunion-decompression/plans with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":       "",
		"daysMissing": 0,
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/reunion-decompression/plans", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid decompression plan, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/reunion-decompression/plans creates a new decompression plan (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":               "pet-2026-002",
		"petName":             "Luna",
		"species":             "CAT",
		"daysMissing":         7,
		"weightLossPct":       12.0,
		"escapeVector":        "DOOR_DASH_BOLT",
		"gpsCollarEquipped":   false,
		"doubleLeashVerified": false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/reunion-decompression/plans", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/reunion-decompression/plans, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createdPlan domain.ReunionDecompressionPlan
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createdPlan); err != nil {
		t.Fatalf("failed to unmarshal created plan: %v", err)
	}
	if !strings.HasPrefix(createdPlan.PlanID, "decomp-plan-") {
		t.Errorf("expected PlanID prefix 'decomp-plan-', got %q", createdPlan.PlanID)
	}
	if !createdPlan.RefeedingSyndromeWarning || createdPlan.Phase != domain.DecompPhaseAcuteRefuge72H {
		t.Errorf("expected new plan RefeedingSyndromeWarning=true and ACUTE_REFUGE_72H, got %+v", createdPlan)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-decompSSECh:
		if evt.Type != domain.ReunionEventType("decompression_plan_updated") {
			t.Errorf("expected decompression_plan_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for decompression_plan_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "decompression_plan_updated" {
			t.Errorf("expected decompression_plan_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for decompression_plan_updated mesh event")
	}

	// 5. POST /api/v1/reunion-decompression/plans/decomp-plan-101/progress updates checklist & perimeter findings
	progressPayload, _ := json.Marshal(map[string]any{
		"completeAllChecklist":  true,
		"remediateAllPerimeter": true,
		"completeItemId":        "",
		"remediateFindingId":    "",
		"gpsCollarEquipped":     true,
		"doubleLeashVerified":   true,
		"notes":                 "Completed staged refeeding, installed entryway airlock gate, and activated LTE GPS collar.",
	})
	recProg := httptest.NewRecorder()
	reqProg := httptest.NewRequest(http.MethodPost, "/api/v1/reunion-decompression/plans/decomp-plan-101/progress", bytes.NewReader(progressPayload))
	reqProg.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recProg, reqProg)
	if recProg.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST progress, got %d: %s", recProg.Code, recProg.Body.String())
	}

	var updatedPlan domain.ReunionDecompressionPlan
	if err := json.Unmarshal(recProg.Body.Bytes(), &updatedPlan); err != nil {
		t.Fatalf("failed to unmarshal updated plan: %v", err)
	}
	if updatedPlan.Phase != domain.DecompPhaseGraduatedStable {
		t.Errorf("expected phase GRADUATED_STABLE, got %s", updatedPlan.Phase)
	}
	if updatedPlan.RecidivismRisk != domain.RecidivismRiskFortifiedLow {
		t.Errorf("expected recidivismRisk FORTIFIED_LOW_RISK, got %s", updatedPlan.RecidivismRisk)
	}
	if updatedPlan.GraduationCertificateID == "" {
		t.Errorf("expected non-empty GraduationCertificateID upon graduation")
	}
	if !strings.Contains(updatedPlan.ClinicalSummary, "installed entryway airlock gate") {
		t.Errorf("expected ClinicalSummary to contain progress notes, got %q", updatedPlan.ClinicalSummary)
	}

	// 6. Unknown plan ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/reunion-decompression/plans/unknown-plan-id/progress", bytes.NewReader(progressPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown plan ID, got %d", rec404.Code)
	}
}

func TestReunionDecompressionHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/reunion-decompression/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/reunion-decompression/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/reunion-decompression/stream, got %d", resp.StatusCode)
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
