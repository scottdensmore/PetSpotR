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

func TestServiceAnimalHandlers_SeedingCreateRecoverAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	saSSECh, unsubSSE := server.ReunionHub().Subscribe("service_animal_priority")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("service_animal_priority", "service-animal-priority-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /service-animal-priority should render HTML and seed default cases
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/service-animal-priority", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /service-animal-priority, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Working Dog &amp; Service Animal ADA") && !strings.Contains(recPage.Body.String(), "Working Dog & Service Animal ADA") {
		t.Errorf("expected /service-animal-priority HTML to contain Working Dog & Service Animal ADA heading")
	}

	// 2. GET /api/v1/service-animal-priority/cases should return the 3 seeded cases
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/service-animal-priority/cases", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/service-animal-priority/cases, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Cases []domain.ServiceAnimalCase `json:"cases"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal service animal cases response: %v", err)
	}
	if len(listResp.Cases) != 3 {
		t.Fatalf("expected 3 seeded service animal cases, got %d", len(listResp.Cases))
	}

	byID := make(map[string]domain.ServiceAnimalCase)
	for _, c := range listResp.Cases {
		byID[c.CaseID] = c
	}

	c101, ok := byID["sa-case-101"]
	if !ok {
		t.Fatalf("expected seeded sa-case-101")
	}
	if c101.PetName != "Apollo" || c101.RecoveryStatus != domain.ServiceStatusEmergencyEscalated || c101.DependencyUrgency != domain.DependencyUrgencyLifeCriticalAlert || !c101.HandlerMedicalStandbyAlert {
		t.Errorf("expected sa-case-101 Apollo in HANDLER_MEDICAL_ESCALATED / LIFE_CRITICAL_MEDICAL_DEPENDENCY with HandlerMedicalStandbyAlert=true, got %+v", c101)
	}

	c102, ok := byID["sa-case-102"]
	if !ok {
		t.Fatalf("expected seeded sa-case-102")
	}
	if c102.PetName != "Pax" || c102.RecoveryStatus != domain.ServiceStatusTacticalInterceptStaged || c102.DependencyUrgency != domain.DependencyUrgencyMobilityGuideHigh {
		t.Errorf("expected sa-case-102 Pax in TACTICAL_VEST_INTERCEPT_STAGED / HIGH_MOBILITY_GUIDE_DEPENDENCY, got %+v", c102)
	}

	c103, ok := byID["sa-case-103"]
	if !ok {
		t.Fatalf("expected seeded sa-case-103")
	}
	if c103.PetName != "Valor" || c103.RecoveryStatus != domain.ServiceStatusReunitedHandlerSafe || c103.DependencyUrgency != domain.DependencyUrgencyTeamStabilized || c103.ADARestorationCertificate != "ADA-RESTORED-SA-CASE-103" {
		t.Errorf("expected sa-case-103 Valor in REUNITED_ADA_TEAM_RESTORED / HANDLER_TEAM_STABILIZED with ADARestorationCertificate, got %+v", c103)
	}

	// 2b. Filter by petId and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/service-animal-priority/cases?petId=pet-2026-001&status=HANDLER_MEDICAL_ESCALATED", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/service-animal-priority/cases, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Cases []domain.ServiceAnimalCase `json:"cases"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered cases: %v", err)
	}
	if len(filteredResp.Cases) != 1 || filteredResp.Cases[0].CaseID != "sa-case-101" {
		t.Errorf("expected 1 filtered case (sa-case-101), got %+v", filteredResp.Cases)
	}

	// 3. POST /api/v1/service-animal-priority/cases with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":   "",
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/service-animal-priority/cases", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid service animal case, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/service-animal-priority/cases creates a new service animal case (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":                "pet-2026-001",
		"petName":              "Apollo",
		"species":              "DOG",
		"handlerName":          "Maya Lin (T1D Handler)",
		"serviceTaskType":      "DIABETIC_HYPOGLYCEMIA_ALERT",
		"vestEquipmentState":   "ADA_CAPE_ENTANGLED_HAZARD",
		"hoursSeparated":       6,
		"handlerStandbyLinked": false,
		"recallCueBroadcasted": false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/service-animal-priority/cases", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/service-animal-priority/cases, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.ServiceAnimalCase
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created service animal case: %v", err)
	}
	if !strings.HasPrefix(created.CaseID, "sa-case-") {
		t.Errorf("expected CaseID prefix 'sa-case-', got %q", created.CaseID)
	}
	if !created.HandlerMedicalStandbyAlert || created.RecoveryStatus != domain.ServiceStatusEmergencyEscalated {
		t.Errorf("expected new case HandlerMedicalStandbyAlert=true and HANDLER_MEDICAL_ESCALATED, got %+v", created)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-saSSECh:
		if evt.Type != domain.ReunionEventType("service_animal_case_updated") {
			t.Errorf("expected service_animal_case_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for service_animal_case_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "service_animal_case_updated" {
			t.Errorf("expected service_animal_case_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for service_animal_case_updated mesh event")
	}

	// 5. POST /api/v1/service-animal-priority/cases/sa-case-101/recover restores ADA team
	recoverPayload, _ := json.Marshal(map[string]any{
		"restoreAdaTeamReunion": true,
	})
	recRecover := httptest.NewRecorder()
	reqRecover := httptest.NewRequest(http.MethodPost, "/api/v1/service-animal-priority/cases/sa-case-101/recover", bytes.NewReader(recoverPayload))
	reqRecover.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recRecover, reqRecover)
	if recRecover.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST recover, got %d: %s", recRecover.Code, recRecover.Body.String())
	}

	var updated domain.ServiceAnimalCase
	if err := json.Unmarshal(recRecover.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to unmarshal updated service animal case: %v", err)
	}
	if updated.RecoveryStatus != domain.ServiceStatusReunitedHandlerSafe {
		t.Errorf("expected REUNITED_ADA_TEAM_RESTORED, got %s", updated.RecoveryStatus)
	}
	if updated.DependencyUrgency != domain.DependencyUrgencyTeamStabilized {
		t.Errorf("expected HANDLER_TEAM_STABILIZED, got %s", updated.DependencyUrgency)
	}
	if updated.ADARestorationCertificate != "ADA-RESTORED-SA-CASE-101" {
		t.Errorf("expected ADA-RESTORED-SA-CASE-101, got %s", updated.ADARestorationCertificate)
	}

	// 6. Unknown case ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/service-animal-priority/cases/unknown-case-id/recover", bytes.NewReader(recoverPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown case ID, got %d", rec404.Code)
	}
}

func TestServiceAnimalHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/service-animal-priority/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/service-animal-priority/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/service-animal-priority/stream, got %d", resp.StatusCode)
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
