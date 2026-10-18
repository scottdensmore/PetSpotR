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

func TestBiometricForensicsHandlers_SeedingCreateVerifyAndBroadcast(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	forensicSSECh, unsubSSE := server.ReunionHub().Subscribe("biometric_forensics")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("biometric_forensics", "biometric-forensics-node")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. GET /biometric-forensics should render HTML and seed default cases
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/biometric-forensics", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /biometric-forensics, got %d: %s", recPage.Code, recPage.Body.String())
	}
	if !strings.Contains(recPage.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %q", recPage.Header().Get("Content-Type"))
	}
	if !strings.Contains(recPage.Body.String(), "Forensic DNA, Coat-Pattern Biometrics") {
		t.Errorf("expected /biometric-forensics HTML to contain Forensic DNA, Coat-Pattern Biometrics heading")
	}

	// 2. GET /api/v1/biometric-forensics/cases should return the 3 seeded cases
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/biometric-forensics/cases", nil)
	server.ServeHTTP(recList, reqList)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/biometric-forensics/cases, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Cases []domain.BiometricForensicCase `json:"cases"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal biometric forensic cases response: %v", err)
	}
	if len(listResp.Cases) != 3 {
		t.Fatalf("expected 3 seeded biometric forensic cases, got %d", len(listResp.Cases))
	}

	byID := make(map[string]domain.BiometricForensicCase)
	for _, c := range listResp.Cases {
		byID[c.CaseID] = c
	}

	c101, ok := byID["forensic-case-101"]
	if !ok {
		t.Fatalf("expected seeded forensic-case-101")
	}
	if c101.PetName != "Barnaby" || c101.ResolutionStatus != domain.ForensicStatusPendingBiometricScan || c101.EntrapmentRisk != domain.EntrapmentRiskCriticalSundowningHazard || !c101.SundowningWanderAlert {
		t.Errorf("expected forensic-case-101 Barnaby in PENDING_BIOMETRIC_SCAN / CRITICAL_SUNDOWNING_ENTRAPMENT with SundowningWanderAlert=true, got %+v", c101)
	}

	c102, ok := byID["forensic-case-102"]
	if !ok {
		t.Fatalf("expected seeded forensic-case-102")
	}
	if c102.PetName != "Clementine" || c102.ResolutionStatus != domain.ForensicStatusDermatoglyphicCorrelated || c102.EntrapmentRisk != domain.EntrapmentRiskElevatedSensoryWander {
		t.Errorf("expected forensic-case-102 Clementine in DERMATOGLYPHIC_CORRELATED / ELEVATED_SENSORY_DISORIENTATION, got %+v", c102)
	}

	c103, ok := byID["forensic-case-103"]
	if !ok {
		t.Fatalf("expected seeded forensic-case-103")
	}
	if c103.PetName != "Jasper" || c103.ResolutionStatus != domain.ForensicStatusIdentityConfirmedSafe || c103.EntrapmentRisk != domain.EntrapmentRiskContainedSafePeriphery || c103.ForensicCertificateID != "CERT-FORENSIC-FORENSIC-CASE-103" {
		t.Errorf("expected forensic-case-103 Jasper in IDENTITY_CONFIRMED_SAFE_EXTRACTION / CONTAINED_SAFE_PERIPHERY with ForensicCertificateID, got %+v", c103)
	}

	// 2b. Filter by petId and status
	recFilter := httptest.NewRecorder()
	reqFilter := httptest.NewRequest(http.MethodGet, "/api/v1/biometric-forensics/cases?petId=pet-2026-001&status=PENDING_BIOMETRIC_SCAN", nil)
	server.ServeHTTP(recFilter, reqFilter)
	if recFilter.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for filtered GET /api/v1/biometric-forensics/cases, got %d", recFilter.Code)
	}
	var filteredResp struct {
		Cases []domain.BiometricForensicCase `json:"cases"`
	}
	if err := json.Unmarshal(recFilter.Body.Bytes(), &filteredResp); err != nil {
		t.Fatalf("failed to unmarshal filtered cases: %v", err)
	}
	if len(filteredResp.Cases) != 1 || filteredResp.Cases[0].CaseID != "forensic-case-101" {
		t.Errorf("expected 1 filtered case (forensic-case-101), got %+v", filteredResp.Cases)
	}

	// 3. POST /api/v1/biometric-forensics/cases with invalid payload returns 400 Bad Request
	invalidPayload, _ := json.Marshal(map[string]any{
		"petId":   "",
		"petName": "",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/biometric-forensics/cases", bytes.NewReader(invalidPayload))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)
	if recInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid biometric forensic case, got %d", recInvalid.Code)
	}

	// 4. POST /api/v1/biometric-forensics/cases creates a new forensic case (201 Created)
	createPayload, _ := json.Marshal(map[string]any{
		"petId":               "pet-2026-001",
		"petName":             "Barnaby",
		"species":             "DOG",
		"ageYears":            15,
		"sensoryCondition":    "CCD_SUNDOWNING_DEMENTIA",
		"biometricModality":   "NASAL_PLANUM_RIDGE_PRINT",
		"nearestHazardType":   "RETENTION_POND_CULVERT",
		"nasalPrintMatched":   false,
		"waterCulvertCleared": false,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/biometric-forensics/cases", bytes.NewReader(createPayload))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)
	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/biometric-forensics/cases, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created domain.BiometricForensicCase
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to unmarshal created forensic case: %v", err)
	}
	if !strings.HasPrefix(created.CaseID, "forensic-case-") {
		t.Errorf("expected CaseID prefix 'forensic-case-', got %q", created.CaseID)
	}
	if !created.SundowningWanderAlert || created.ResolutionStatus != domain.ForensicStatusPendingBiometricScan {
		t.Errorf("expected new case SundowningWanderAlert=true and PENDING_BIOMETRIC_SCAN, got %+v", created)
	}

	// Verify SSE and Mesh broadcast on creation
	select {
	case evt := <-forensicSSECh:
		if evt.Type != domain.ReunionEventType("forensic_case_updated") {
			t.Errorf("expected forensic_case_updated SSE event, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for forensic_case_updated SSE event")
	}

	select {
	case env := <-meshCh:
		if string(env.Type) != "forensic_case_updated" {
			t.Errorf("expected forensic_case_updated mesh event, got %q", env.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for forensic_case_updated mesh event")
	}

	// 5. POST /api/v1/biometric-forensics/cases/forensic-case-101/verify confirms extraction
	verifyPayload, _ := json.Marshal(map[string]any{
		"confirmForensicExtraction": true,
	})
	recVerify := httptest.NewRecorder()
	reqVerify := httptest.NewRequest(http.MethodPost, "/api/v1/biometric-forensics/cases/forensic-case-101/verify", bytes.NewReader(verifyPayload))
	reqVerify.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recVerify, reqVerify)
	if recVerify.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for POST verify, got %d: %s", recVerify.Code, recVerify.Body.String())
	}

	var updated domain.BiometricForensicCase
	if err := json.Unmarshal(recVerify.Body.Bytes(), &updated); err != nil {
		t.Fatalf("failed to unmarshal updated forensic case: %v", err)
	}
	if updated.ResolutionStatus != domain.ForensicStatusIdentityConfirmedSafe {
		t.Errorf("expected IDENTITY_CONFIRMED_SAFE_EXTRACTION, got %s", updated.ResolutionStatus)
	}
	if updated.EntrapmentRisk != domain.EntrapmentRiskContainedSafePeriphery {
		t.Errorf("expected CONTAINED_SAFE_PERIPHERY, got %s", updated.EntrapmentRisk)
	}
	if updated.ForensicCertificateID != "CERT-FORENSIC-FORENSIC-CASE-101" {
		t.Errorf("expected CERT-FORENSIC-FORENSIC-CASE-101, got %s", updated.ForensicCertificateID)
	}

	// 6. Unknown case ID returns 404 Not Found
	rec404 := httptest.NewRecorder()
	req404 := httptest.NewRequest(http.MethodPost, "/api/v1/biometric-forensics/cases/unknown-case-id/verify", bytes.NewReader(verifyPayload))
	req404.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404, req404)
	if rec404.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for unknown case ID, got %d", rec404.Code)
	}
}

func TestBiometricForensicsHandlers_StreamSSE(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	ts := httptest.NewServer(server)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/v1/biometric-forensics/stream", nil)
	if err != nil {
		t.Fatalf("failed to create SSE request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to connect to /api/v1/biometric-forensics/stream: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/biometric-forensics/stream, got %d", resp.StatusCode)
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
