package webfrontend_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestVeterinaryEndpoints_Lifecycle(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	petID := "pet-vet-001"

	// Set up SSE subscriber for triage events
	reunionCh, unsubReunion := server.ReunionHub().Subscribe(petID)
	defer unsubReunion()

	// Set up Mesh subscriber for peer mesh events
	meshCh, unsubMesh := server.SignalingHub().Subscribe(petID, "mobile-medic-node-1")
	defer unsubMesh()

	// Drain initial join event from mesh channel
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. Issue passport
	reqBody, _ := json.Marshal(map[string]any{
		"petId":       petID,
		"petName":     "Milo",
		"species":     "Cat",
		"breed":       "Siamese",
		"microchipId": "985141000445566",
		"rabiesTagId": "RAB-2026-9912",
		"bloodType":   "A",
		"weightKg":    4.2,
		"allergies": []map[string]any{
			{"allergen": "Amoxicillin", "severity": "MODERATE", "reactionDescription": "Facial swelling and hives"},
		},
		"vaccinations": []map[string]any{
			{
				"vaccineName":      "Rabies",
				"administeredDate": time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339),
				"expirationDate":   time.Now().AddDate(2, 0, 0).UTC().Format(time.RFC3339),
				"clinicName":       "Central Emergency Vet",
				"verified":         true,
			},
		},
		"emergencyContact": "Sarah Connor (+1-555-0199)",
		"primaryClinic":    "Central Emergency Vet",
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for passport issuance, got %d: %s", rec.Code, rec.Body.String())
	}

	var passportRes map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &passportRes); err != nil {
		t.Fatalf("failed to unmarshal passport response: %v", err)
	}

	passportID, ok := passportRes["passportId"].(string)
	if !ok || passportID == "" {
		t.Fatalf("missing passportId in response: %+v", passportRes)
	}

	qrPayload, ok := passportRes["qrPayload"].(string)
	if !ok || len(qrPayload) == 0 {
		t.Fatalf("missing qrPayload in response: %+v", passportRes)
	}

	qrDataURI, ok := passportRes["qrDataUri"].(string)
	if !ok || len(qrDataURI) == 0 {
		t.Fatalf("missing qrDataUri in response: %+v", passportRes)
	}

	// 2. Verify passport payload
	verifyBody, _ := json.Marshal(map[string]string{
		"qrPayload": qrPayload,
	})
	recVerify := httptest.NewRecorder()
	reqVerify := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports/verify", bytes.NewReader(verifyBody))
	reqVerify.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recVerify, reqVerify)

	if recVerify.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for verify, got %d: %s", recVerify.Code, recVerify.Body.String())
	}

	var verifyRes struct {
		Verified bool                      `json:"verified"`
		Passport domain.VeterinaryPassport `json:"passport"`
	}
	if err := json.Unmarshal(recVerify.Body.Bytes(), &verifyRes); err != nil {
		t.Fatalf("failed to parse verify response: %v", err)
	}
	if !verifyRes.Verified {
		t.Errorf("expected verified true, got false")
	}
	if verifyRes.Passport.PetID != petID || verifyRes.Passport.PetName != "Milo" {
		t.Errorf("unexpected verified passport: %+v", verifyRes.Passport)
	}

	// 3. Retrieve passport by ID
	recGetPass := httptest.NewRecorder()
	reqGetPass := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/passports/"+passportID, nil)
	server.ServeHTTP(recGetPass, reqGetPass)

	if recGetPass.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get passport, got %d: %s", recGetPass.Code, recGetPass.Body.String())
	}

	var retrievedPassport domain.VeterinaryPassport
	if err := json.Unmarshal(recGetPass.Body.Bytes(), &retrievedPassport); err != nil {
		t.Fatalf("failed to unmarshal retrieved passport: %v", err)
	}
	if retrievedPassport.PassportID != passportID || retrievedPassport.PetID != petID {
		t.Errorf("retrieved passport mismatch: got %+v", retrievedPassport)
	}

	// 4. Submit Emergency Triage Assessment
	triageBody, _ := json.Marshal(map[string]any{
		"petId":     petID,
		"medicId":   "medic-9",
		"medicName": "Sarah Jenks, RVT",
		"species":   "Cat",
		"weightKg":  4.2,
		"vitals": map[string]any{
			"heartRateBpm":       240,
			"respiratoryRateBpm": 60,
			"temperatureF":       103.5,
			"capillaryRefillSec": 3.2,
			"mucousMembrane":     "MM_CYANOTIC",
			"glasgowComaScale":   8,
		},
		"traumaNotes": "Smoke inhalation and lethargy",
	})

	recTriage := httptest.NewRecorder()
	reqTriage := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage", bytes.NewReader(triageBody))
	reqTriage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTriage, reqTriage)

	if recTriage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for triage, got %d: %s", recTriage.Code, recTriage.Body.String())
	}

	var triageRes domain.TriageAssessment
	if err := json.Unmarshal(recTriage.Body.Bytes(), &triageRes); err != nil {
		t.Fatalf("failed to unmarshal triage response: %v", err)
	}
	if triageRes.Category != domain.TriageCategoryRed {
		t.Errorf("expected TRIAGE_RED, got %s", triageRes.Category)
	}
	if triageRes.AssessmentID == "" {
		t.Errorf("expected non-empty assessment ID")
	}

	// 5. Verify SSE distribution on reunionHub
	select {
	case sseEvt := <-reunionCh:
		if sseEvt.MatchID != petID {
			t.Errorf("expected SSE event MatchID %q, got %q", petID, sseEvt.MatchID)
		}
		if sseEvt.Type != "triage_assessment_created" {
			t.Errorf("expected SSE event type triage_assessment_created, got %s", sseEvt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for SSE broadcast on reunionHub")
	}

	// 6. Verify Mesh distribution on signalingHub
	select {
	case meshEnv := <-meshCh:
		if meshEnv.SearchPartyID != petID {
			t.Errorf("expected mesh envelope SearchPartyID %q, got %q", petID, meshEnv.SearchPartyID)
		}
		if meshEnv.Type != mesh.SignalingType("triage_assessment_created") {
			t.Errorf("expected mesh envelope type triage_assessment_created, got %s", meshEnv.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for mesh broadcast on signalingHub")
	}

	// 7. Retrieve assessments for pet
	recGetTriage := httptest.NewRecorder()
	reqGetTriage := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/triage/"+petID, nil)
	server.ServeHTTP(recGetTriage, reqGetTriage)

	if recGetTriage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get triage assessments, got %d: %s", recGetTriage.Code, recGetTriage.Body.String())
	}

	var assessmentsList []domain.TriageAssessment
	if err := json.Unmarshal(recGetTriage.Body.Bytes(), &assessmentsList); err != nil {
		t.Fatalf("failed to unmarshal assessments list: %v", err)
	}
	if len(assessmentsList) != 1 {
		t.Fatalf("expected 1 assessment, got %d", len(assessmentsList))
	}
	if assessmentsList[0].AssessmentID != triageRes.AssessmentID {
		t.Errorf("expected assessment ID %s, got %s", triageRes.AssessmentID, assessmentsList[0].AssessmentID)
	}

	// 8. Append Clinical Treatment
	treatBody, _ := json.Marshal(map[string]any{
		"medicationName": "Oxygen Flow-By & Butorphanol",
		"dosage":         "0.8 mg",
		"route":          "IM",
		"administeredBy": "Sarah Jenks, RVT",
		"notes":          "Sedation and respiratory stabilization",
	})
	recTreat := httptest.NewRecorder()
	reqTreat := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage/"+triageRes.AssessmentID+"/treatments", bytes.NewReader(treatBody))
	reqTreat.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTreat, reqTreat)

	if recTreat.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for treatment, got %d: %s", recTreat.Code, recTreat.Body.String())
	}

	var updatedAssessment domain.TriageAssessment
	if err := json.Unmarshal(recTreat.Body.Bytes(), &updatedAssessment); err != nil {
		t.Fatalf("failed to unmarshal updated assessment: %v", err)
	}
	if len(updatedAssessment.AdministeredTreatments) != 1 {
		t.Fatalf("expected 1 treatment, got %d", len(updatedAssessment.AdministeredTreatments))
	}
	treatment := updatedAssessment.AdministeredTreatments[0]
	if treatment.MedicationName != "Oxygen Flow-By & Butorphanol" || treatment.Dosage != "0.8 mg" {
		t.Errorf("unexpected treatment content: %+v", treatment)
	}
	if updatedAssessment.ReassessedAt == nil {
		t.Errorf("expected ReassessedAt to be set after treatment")
	}

	// Verify treatment SSE distribution on reunionHub
	select {
	case sseEvt := <-reunionCh:
		if sseEvt.MatchID != petID {
			t.Errorf("expected SSE treatment event MatchID %q, got %q", petID, sseEvt.MatchID)
		}
		if sseEvt.Type != "triage_treatment_administered" {
			t.Errorf("expected SSE event type triage_treatment_administered, got %s", sseEvt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for treatment SSE broadcast on reunionHub")
	}

	// Verify treatment Mesh distribution on signalingHub
	select {
	case meshEnv := <-meshCh:
		if meshEnv.SearchPartyID != petID {
			t.Errorf("expected mesh treatment envelope SearchPartyID %q, got %q", petID, meshEnv.SearchPartyID)
		}
		if meshEnv.Type != mesh.SignalingType("triage_treatment_administered") {
			t.Errorf("expected mesh envelope type triage_treatment_administered, got %s", meshEnv.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for treatment mesh broadcast on signalingHub")
	}

	// 9. Re-fetch via GET triage by petID to confirm persisted state
	recGetTriage2 := httptest.NewRecorder()
	reqGetTriage2 := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/triage/"+petID, nil)
	server.ServeHTTP(recGetTriage2, reqGetTriage2)

	if recGetTriage2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for second get triage, got %d: %s", recGetTriage2.Code, recGetTriage2.Body.String())
	}

	var assessmentsList2 []domain.TriageAssessment
	if err := json.Unmarshal(recGetTriage2.Body.Bytes(), &assessmentsList2); err != nil {
		t.Fatalf("failed to unmarshal assessments list: %v", err)
	}
	if len(assessmentsList2) != 1 || len(assessmentsList2[0].AdministeredTreatments) != 1 {
		t.Errorf("persisted assessment missing treatment: %+v", assessmentsList2)
	}
}

func TestVeterinaryEndpoints_ValidationAndErrorHandling(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	t.Run("Passport Issuance Missing Required Fields", func(t *testing.T) {
		// Missing petName
		reqBody, _ := json.Marshal(map[string]any{
			"petId": "pet-001",
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Passport Issuance Invalid JSON", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports", bytes.NewReader([]byte("{invalid-json")))
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Passport Verification Corrupted Payload", func(t *testing.T) {
		verifyBody, _ := json.Marshal(map[string]string{
			"qrPayload": "VP1:corrupt-data",
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports/verify", bytes.NewReader(verifyBody))
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 Unprocessable Entity, got %d", rec.Code)
		}
	})

	t.Run("Passport Verification Empty Payload", func(t *testing.T) {
		verifyBody, _ := json.Marshal(map[string]string{
			"qrPayload": "",
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports/verify", bytes.NewReader(verifyBody))
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Get Nonexistent Passport", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/passports/nonexistent-id", nil)
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("Get Triage Unknown Pet Returns Empty Array", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/triage/unknown-pet", nil)
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var list []domain.TriageAssessment
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if list == nil || len(list) != 0 {
			t.Errorf("expected empty non-nil slice, got %+v", list)
		}
	})

	t.Run("Append Treatment Nonexistent Assessment", func(t *testing.T) {
		treatBody, _ := json.Marshal(map[string]any{
			"medicationName": "Fluids",
			"dosage":         "100 mL",
			"route":          "IV",
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage/nonexistent-assessment/treatments", bytes.NewReader(treatBody))
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", rec.Code)
		}
	})

	t.Run("Append Treatment Empty Medication Name", func(t *testing.T) {
		treatBody, _ := json.Marshal(map[string]any{
			"medicationName": "   ",
			"dosage":         "100 mL",
			"route":          "IV",
		})
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage/some-assessment/treatments", bytes.NewReader(treatBody))
		req.Header.Set("Content-Type", "application/json")
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", rec.Code)
		}
	})

	t.Run("Method Not Allowed Check", func(t *testing.T) {
		// GET on /api/v1/veterinary/passports
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/passports", nil)
		server.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed, got %d", rec.Code)
		}
	})
}

func TestVeterinaryEndpoints_ConcurrentTreatments(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	petID := "pet-concurrent-treat"

	// Create assessment
	triageBody, _ := json.Marshal(map[string]any{
		"petId":    petID,
		"medicId":  "medic-concurrent",
		"species":  "Dog",
		"weightKg": 25.0,
		"vitals": map[string]any{
			"heartRateBpm": 100,
		},
	})
	recTriage := httptest.NewRecorder()
	reqTriage := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage", bytes.NewReader(triageBody))
	reqTriage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTriage, reqTriage)

	if recTriage.Code != http.StatusOK {
		t.Fatalf("failed to create triage assessment: %d", recTriage.Code)
	}

	var assessment domain.TriageAssessment
	if err := json.Unmarshal(recTriage.Body.Bytes(), &assessment); err != nil {
		t.Fatalf("failed to parse triage response: %v", err)
	}

	const numTreatments = 10
	var wg sync.WaitGroup
	wg.Add(numTreatments)

	for i := 0; i < numTreatments; i++ {
		go func(idx int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]any{
				"medicationName": fmt.Sprintf("Medication-%d", idx),
				"dosage":         fmt.Sprintf("%d mg", idx+1),
				"route":          "IV",
				"administeredBy": fmt.Sprintf("Medic-%d", idx),
			})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage/"+assessment.AssessmentID+"/treatments", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			server.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("concurrent treatment %d failed with code %d: %s", idx, rec.Code, rec.Body.String())
			}
		}(i)
	}

	wg.Wait()

	// Verify all treatments are atomically preserved
	recGet := httptest.NewRecorder()
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/triage/"+petID, nil)
	server.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("failed to get triage: %d", recGet.Code)
	}

	var list []domain.TriageAssessment
	if err := json.Unmarshal(recGet.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to parse triage list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 assessment, got %d", len(list))
	}
	if len(list[0].AdministeredTreatments) != numTreatments {
		t.Fatalf("expected %d administered treatments, got %d (data race / lost update)", numTreatments, len(list[0].AdministeredTreatments))
	}
}
