package webfrontend_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/veterinary"
)

func TestVeterinaryFHIR_ClinicsSeedingAndRegistration(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// 1. GET /api/v1/veterinary/clinics should seed 3 default 24/7 emergency clinics
	recList := httptest.NewRecorder()
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/clinics", nil)
	server.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/veterinary/clinics, got %d: %s", recList.Code, recList.Body.String())
	}

	var listResp struct {
		Clinics []domain.VeterinaryClinic `json:"clinics"`
	}
	if err := json.Unmarshal(recList.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to unmarshal clinics response: %v", err)
	}
	if len(listResp.Clinics) != 3 {
		t.Fatalf("expected 3 seeded clinics, got %d", len(listResp.Clinics))
	}

	expectedIDs := map[string]domain.PIMSProvider{
		"clinic-er-seattle-1":  domain.PIMSCornerstone,
		"clinic-er-bellevue-2": domain.PIMSEzyVet,
		"clinic-shelter-3":     domain.PIMSShelterluv,
	}
	for _, c := range listResp.Clinics {
		wantPIMS, ok := expectedIDs[c.ClinicID]
		if !ok {
			t.Errorf("unexpected seeded clinic ID %q", c.ClinicID)
			continue
		}
		if c.PIMS != wantPIMS {
			t.Errorf("clinic %s PIMS = %q, want %q", c.ClinicID, c.PIMS, wantPIMS)
		}
		if !c.Emergency24Hr || !c.AcceptingPatients {
			t.Errorf("expected seeded clinic %s to be 24hr emergency and accepting patients", c.ClinicID)
		}
	}

	// 2. POST /api/v1/veterinary/clinics to register custom clinic (default PIMS when omitted)
	createBody, _ := json.Marshal(map[string]any{
		"name":          "Cascade Veterinary Trauma Center",
		"emergency24Hr": true,
		"phone":         "(206) 555-0188",
		"address":       "500 Pine St, Seattle, WA",
		"latitude":      47.6109,
		"longitude":     -122.3364,
	})
	recCreate := httptest.NewRecorder()
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/clinics", bytes.NewReader(createBody))
	reqCreate.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/veterinary/clinics, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var createResp struct {
		Clinic domain.VeterinaryClinic `json:"clinic"`
	}
	if err := json.Unmarshal(recCreate.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create clinic response: %v", err)
	}
	if !strings.HasPrefix(createResp.Clinic.ClinicID, "clinic-") {
		t.Errorf("expected generated clinicId starting with 'clinic-', got %q", createResp.Clinic.ClinicID)
	}
	if createResp.Clinic.PIMS != domain.PIMSCornerstone {
		t.Errorf("expected default PIMS %q, got %q", domain.PIMSCornerstone, createResp.Clinic.PIMS)
	}
	if !createResp.Clinic.AcceptingPatients {
		t.Errorf("expected AcceptingPatients to be true")
	}

	// 3. Invalid clinic registration returns 400 Bad Request
	invalidBody, _ := json.Marshal(map[string]any{
		"name": "",
		"pims": "INVALID_PIMS",
	})
	recInvalid := httptest.NewRecorder()
	reqInvalid := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/clinics", bytes.NewReader(invalidBody))
	reqInvalid.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recInvalid, reqInvalid)

	if recInvalid.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid clinic, got %d", recInvalid.Code)
	}
}

func TestVeterinaryFHIR_HandoffExportAndInboundWebhookLifecycle(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	petID := "pet-fhir-k9-01"

	// Subscribe to SSE "triage" topic and Mesh signaling hub
	triageSSECh, unsubSSE := server.ReunionHub().Subscribe("triage")
	defer unsubSSE()

	meshCh, unsubMesh := server.SignalingHub().Subscribe("triage", "er-clinic-gateway")
	defer unsubMesh()
	select {
	case <-meshCh:
	case <-time.After(50 * time.Millisecond):
	}

	// 1. Create a VeterinaryPassport with critical anaphylactic allergy and rabies vaccination
	passportReq, _ := json.Marshal(map[string]any{
		"petId":       petID,
		"petName":     "Apollo",
		"species":     "Dog",
		"breed":       "German Shepherd",
		"microchipId": "985141002394811",
		"rabiesTagId": "WA-RAB-2026-771",
		"bloodType":   "DEA 1.1 Negative",
		"weightKg":    34.5,
		"allergies": []map[string]any{
			{
				"allergen":            "Penicillin",
				"severity":            "ANAPHYLACTIC",
				"reactionDescription": "Acute anaphylaxis and hypotensive shock",
			},
		},
		"vaccinations": []map[string]any{
			{
				"vaccineName":      "Rabies 3-Year",
				"administeredDate": time.Now().AddDate(-1, 0, 0).UTC().Format(time.RFC3339),
				"expirationDate":   time.Now().AddDate(2, 0, 0).UTC().Format(time.RFC3339),
				"clinicName":       "Northwest Veterinary Critical Care & ER",
				"verified":         true,
			},
		},
		"emergencyContact": "Commander Alex Vance (206-555-0199)",
		"primaryClinic":    "Northwest Veterinary Critical Care & ER",
	})
	recPass := httptest.NewRecorder()
	reqPass := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports", bytes.NewReader(passportReq))
	reqPass.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recPass, reqPass)
	if recPass.Code != http.StatusOK {
		t.Fatalf("failed to create passport: %d %s", recPass.Code, recPass.Body.String())
	}
	var passResp struct {
		PassportID string `json:"passportId"`
	}
	_ = json.Unmarshal(recPass.Body.Bytes(), &passResp)

	// 2. Create a TriageAssessment
	triageReq, _ := json.Marshal(map[string]any{
		"petId":     petID,
		"hubId":     "hub-seattle-center",
		"medicId":   "medic-sar-1",
		"medicName": "Lt. Elena Rostova, RVT",
		"species":   "Dog",
		"weightKg":  34.5,
		"vitals": map[string]any{
			"heartRateBpm":       175,
			"respiratoryRateBpm": 48,
			"temperatureF":       103.8,
			"capillaryRefillSec": 3.0,
			"mucousMembrane":     "MM_PALE",
			"glasgowComaScale":   11,
		},
		"traumaNotes": "Smoke inhalation and thermal paw pad burns during wildfire evacuation",
	})
	recTriage := httptest.NewRecorder()
	reqTriage := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage", bytes.NewReader(triageReq))
	reqTriage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTriage, reqTriage)
	if recTriage.Code != http.StatusOK {
		t.Fatalf("failed to create triage assessment: %d %s", recTriage.Code, recTriage.Body.String())
	}
	var assessment domain.TriageAssessment
	_ = json.Unmarshal(recTriage.Body.Bytes(), &assessment)

	// Drain triage creation event from SSE
	select {
	case <-triageSSECh:
	case <-time.After(100 * time.Millisecond):
	}

	// 3. Append IV clinical treatment to assessment
	txReq, _ := json.Marshal(map[string]any{
		"medicationName": "Lactated Ringer's Bolus",
		"dosage":         "1000 mL",
		"route":          "IV",
		"administeredBy": "Lt. Elena Rostova, RVT",
		"notes":          "Shock resuscitation bolus via cephalic IV catheter",
	})
	recTx := httptest.NewRecorder()
	reqTx := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage/"+assessment.AssessmentID+"/treatments", bytes.NewReader(txReq))
	reqTx.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTx, reqTx)
	if recTx.Code != http.StatusOK {
		t.Fatalf("failed to append IV treatment: %d %s", recTx.Code, recTx.Body.String())
	}

	// Drain treatment event from SSE
	select {
	case <-triageSSECh:
	case <-time.After(100 * time.Millisecond):
	}

	// 4. Dispatch Outbound Pre-Arrival ClinicalHandoff (POST /api/v1/veterinary/handoffs)
	handoffReq, _ := json.Marshal(map[string]any{
		"assessmentId":  assessment.AssessmentID,
		"petId":         petID,
		"passportId":    passResp.PassportID,
		"clinicId":      "clinic-er-seattle-1",
		"transportUnit": "SAR MedEvac Unit #4",
		"etaMinutes":    12,
	})
	recHandoff := httptest.NewRecorder()
	reqHandoff := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/handoffs", bytes.NewReader(handoffReq))
	reqHandoff.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recHandoff, reqHandoff)

	if recHandoff.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for POST /api/v1/veterinary/handoffs, got %d: %s", recHandoff.Code, recHandoff.Body.String())
	}

	var handoff domain.ClinicalHandoff
	if err := json.Unmarshal(recHandoff.Body.Bytes(), &handoff); err != nil {
		t.Fatalf("failed to unmarshal clinical handoff: %v", err)
	}
	if !strings.HasPrefix(handoff.HandoffID, "handoff-") {
		t.Errorf("expected handoffId starting with 'handoff-', got %q", handoff.HandoffID)
	}
	if handoff.Status != domain.HandoffStatusEnRoute {
		t.Errorf("expected status %q, got %q", domain.HandoffStatusEnRoute, handoff.Status)
	}
	if handoff.ClinicID != "clinic-er-seattle-1" || handoff.PIMS != domain.PIMSCornerstone {
		t.Errorf("unexpected clinic details on handoff: %+v", handoff)
	}
	if err := veterinary.ValidateFHIRVeterinaryBundle(handoff.FHIRBundle); err != nil {
		t.Fatalf("expected valid FHIR bundle on handoff, got error: %v", err)
	}

	// Verify FHIRBundle contains Patient (with patient-animal extension), AllergyIntolerance, Observation, MedicationAdministration
	resourceTypes := make(map[string]int)
	for _, entry := range handoff.FHIRBundle.Entry {
		if rt, ok := entry.Resource["resourceType"].(string); ok {
			resourceTypes[rt]++
		}
	}
	for _, requiredType := range []string{"Patient", "Encounter", "AllergyIntolerance", "Observation", "MedicationAdministration", "Immunization"} {
		if resourceTypes[requiredType] == 0 {
			t.Errorf("expected FHIRBundle to include %s resource, got counts: %v", requiredType, resourceTypes)
		}
	}

	// Verify PIMSRecord populated
	if handoff.PIMSRecord.PetName != "Apollo" || handoff.PIMSRecord.MicrochipID != "985141002394811" {
		t.Errorf("unexpected PIMSRecord patient metadata: %+v", handoff.PIMSRecord)
	}
	if len(handoff.PIMSRecord.CriticalAllergies) != 1 || !strings.Contains(handoff.PIMSRecord.CriticalAllergies[0], "Penicillin") {
		t.Errorf("expected PIMSRecord CriticalAllergies to include Penicillin, got %+v", handoff.PIMSRecord.CriticalAllergies)
	}
	if len(handoff.PIMSRecord.TreatmentsGiven) != 1 || !strings.Contains(handoff.PIMSRecord.TreatmentsGiven[0], "Lactated Ringer's Bolus") {
		t.Errorf("expected PIMSRecord TreatmentsGiven to include IV bolus, got %+v", handoff.PIMSRecord.TreatmentsGiven)
	}

	// Verify SSE event "clinical_handoff_dispatched" on topic "triage"
	select {
	case evt := <-triageSSECh:
		if evt.Type != domain.ReunionEventType("clinical_handoff_dispatched") {
			t.Errorf("expected SSE event clinical_handoff_dispatched, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for clinical_handoff_dispatched SSE event")
	}

	// 5. List ClinicalHandoffs (GET /api/v1/veterinary/handoffs?petId=...)
	recListHandoffs := httptest.NewRecorder()
	reqListHandoffs := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/handoffs?petId="+petID, nil)
	server.ServeHTTP(recListHandoffs, reqListHandoffs)
	if recListHandoffs.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /api/v1/veterinary/handoffs, got %d", recListHandoffs.Code)
	}
	var listHandoffsResp struct {
		Handoffs []domain.ClinicalHandoff `json:"handoffs"`
	}
	if err := json.Unmarshal(recListHandoffs.Body.Bytes(), &listHandoffsResp); err != nil {
		t.Fatalf("failed to unmarshal handoffs list: %v", err)
	}
	if len(listHandoffsResp.Handoffs) != 1 || listHandoffsResp.Handoffs[0].HandoffID != handoff.HandoffID {
		t.Fatalf("expected 1 handoff matching %s, got %+v", handoff.HandoffID, listHandoffsResp.Handoffs)
	}

	// 6. Export raw FHIR R4 Bundle (GET /api/v1/veterinary/handoffs/{id}/fhir)
	recExportFHIR := httptest.NewRecorder()
	reqExportFHIR := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/handoffs/"+handoff.HandoffID+"/fhir", nil)
	server.ServeHTTP(recExportFHIR, reqExportFHIR)

	if recExportFHIR.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for FHIR export, got %d: %s", recExportFHIR.Code, recExportFHIR.Body.String())
	}
	if ct := recExportFHIR.Header().Get("Content-Type"); !strings.Contains(ct, "application/fhir+json") {
		t.Errorf("expected Content-Type application/fhir+json, got %q", ct)
	}
	var exportedBundle domain.FHIRVeterinaryBundle
	if err := json.Unmarshal(recExportFHIR.Body.Bytes(), &exportedBundle); err != nil {
		t.Fatalf("failed to unmarshal exported FHIR bundle: %v", err)
	}
	if err := veterinary.ValidateFHIRVeterinaryBundle(exportedBundle); err != nil {
		t.Errorf("exported FHIR bundle failed validation: %v", err)
	}

	// 7. Export PIMS CSV (GET /api/v1/veterinary/handoffs/{id}/fhir?format=csv)
	recExportCSV := httptest.NewRecorder()
	reqExportCSV := httptest.NewRequest(http.MethodGet, "/api/v1/veterinary/handoffs/"+handoff.HandoffID+"/fhir?format=csv", nil)
	server.ServeHTTP(recExportCSV, reqExportCSV)

	if recExportCSV.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for CSV export, got %d: %s", recExportCSV.Code, recExportCSV.Body.String())
	}
	if ct := recExportCSV.Header().Get("Content-Type"); !strings.Contains(ct, "text/csv") {
		t.Errorf("expected Content-Type text/csv, got %q", ct)
	}
	if cd := recExportCSV.Header().Get("Content-Disposition"); !strings.Contains(cd, "pims-intake-"+handoff.HandoffID+".csv") {
		t.Errorf("expected Content-Disposition with filename, got %q", cd)
	}
	csvRows, err := csv.NewReader(strings.NewReader(recExportCSV.Body.String())).ReadAll()
	if err != nil || len(csvRows) != 2 {
		t.Fatalf("expected 2 CSV rows (header + 1 record), got %d (err=%v)", len(csvRows), err)
	}

	// 8. Inbound PIMS Webhook (POST /api/v1/veterinary/handoffs/{id}/pims-webhook)
	webhookBody, _ := json.Marshal(map[string]any{
		"status":              "CLEARED_FOR_REUNIFICATION",
		"verifiedMicrochipId": "985141002394811",
		"attendingVet":        "Dr. Sarah Jenkins, DACVECC",
		"dischargeSummary":    "Patient stabilized on IV fluids and oxygen; burns dressed; cleared for owner reunification.",
	})
	recWebhook := httptest.NewRecorder()
	reqWebhook := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/handoffs/"+handoff.HandoffID+"/pims-webhook", bytes.NewReader(webhookBody))
	reqWebhook.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recWebhook, reqWebhook)

	if recWebhook.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for PIMS webhook, got %d: %s", recWebhook.Code, recWebhook.Body.String())
	}

	var webhookResp struct {
		Handoff domain.ClinicalHandoff `json:"handoff"`
	}
	if err := json.Unmarshal(recWebhook.Body.Bytes(), &webhookResp); err != nil {
		t.Fatalf("failed to unmarshal webhook response: %v", err)
	}
	updated := webhookResp.Handoff
	if updated.Status != domain.HandoffStatusClearedForReunification {
		t.Errorf("expected status %q, got %q", domain.HandoffStatusClearedForReunification, updated.Status)
	}
	if !updated.ReadyForReunion {
		t.Errorf("expected ReadyForReunion == true when CLEARED_FOR_REUNIFICATION")
	}
	if updated.VerifiedMicrochipID != "985141002394811" {
		t.Errorf("expected VerifiedMicrochipID 985141002394811, got %q", updated.VerifiedMicrochipID)
	}
	if updated.AttendingVet != "Dr. Sarah Jenkins, DACVECC" || updated.PIMSRecord.AttendingVet != "Dr. Sarah Jenkins, DACVECC" {
		t.Errorf("expected AttendingVet mirrored into PIMSRecord, got handoff=%q pims=%q", updated.AttendingVet, updated.PIMSRecord.AttendingVet)
	}
	if updated.PIMSRecord.MicrochipID != "985141002394811" {
		t.Errorf("expected VerifiedMicrochipID mirrored into PIMSRecord.MicrochipID, got %q", updated.PIMSRecord.MicrochipID)
	}

	// Verify SSE event "clinical_handoff_updated" on topic "triage"
	select {
	case evt := <-triageSSECh:
		if evt.Type != domain.ReunionEventType("clinical_handoff_updated") {
			t.Errorf("expected SSE event clinical_handoff_updated, got %q", evt.Type)
		}
	case <-time.After(200 * time.Millisecond):
		t.Errorf("timed out waiting for clinical_handoff_updated SSE event")
	}

	// 9. GET /veterinary/fhir page renders 200 OK
	recPage := httptest.NewRecorder()
	reqPage := httptest.NewRequest(http.MethodGet, "/veterinary/fhir", nil)
	server.ServeHTTP(recPage, reqPage)
	if recPage.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /veterinary/fhir, got %d: %s", recPage.Code, recPage.Body.String())
	}
}

func TestVeterinaryFHIR_FallbackResolutionAndErrorCases(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	petID := "pet-fhir-fallback"

	// Create passport and triage assessment
	passportReq, _ := json.Marshal(map[string]any{
		"petId":       petID,
		"petName":     "Luna",
		"species":     "Cat",
		"breed":       "Maine Coon",
		"microchipId": "985141009999000",
		"weightKg":    5.5,
	})
	recPass := httptest.NewRecorder()
	reqPass := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/passports", bytes.NewReader(passportReq))
	reqPass.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recPass, reqPass)

	triageReq, _ := json.Marshal(map[string]any{
		"petId":    petID,
		"species":  "Cat",
		"weightKg": 5.5,
		"vitals": map[string]any{
			"heartRateBpm":       190,
			"respiratoryRateBpm": 36,
			"temperatureF":       101.2,
		},
	})
	recTriage := httptest.NewRecorder()
	reqTriage := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/triage", bytes.NewReader(triageReq))
	reqTriage.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recTriage, reqTriage)

	// Dispatch handoff using only petId (auto-resolving latest assessment and passport, plus default transportUnit and etaMinutes)
	handoffReq, _ := json.Marshal(map[string]any{
		"petId":    petID,
		"clinicId": "clinic-er-bellevue-2",
	})
	recHandoff := httptest.NewRecorder()
	reqHandoff := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/handoffs", bytes.NewReader(handoffReq))
	reqHandoff.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recHandoff, reqHandoff)

	if recHandoff.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for fallback handoff, got %d: %s", recHandoff.Code, recHandoff.Body.String())
	}

	var handoff domain.ClinicalHandoff
	if err := json.Unmarshal(recHandoff.Body.Bytes(), &handoff); err != nil {
		t.Fatalf("failed to decode handoff: %v", err)
	}
	if handoff.TransportUnit != "SAR MedEvac Unit #1" || handoff.ETAMinutes != 15 {
		t.Errorf("expected default transportUnit and etaMinutes=15, got %q and %d", handoff.TransportUnit, handoff.ETAMinutes)
	}
	if handoff.PIMSRecord.PetName != "Luna" || handoff.PIMSRecord.MicrochipID != "985141009999000" {
		t.Errorf("expected auto-resolved passport on PIMSRecord, got %+v", handoff.PIMSRecord)
	}

	// Error case: unknown clinic returns 404
	unknownClinicReq, _ := json.Marshal(map[string]any{
		"petId":    petID,
		"clinicId": "clinic-nonexistent",
	})
	rec404Clinic := httptest.NewRecorder()
	req404Clinic := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/handoffs", bytes.NewReader(unknownClinicReq))
	req404Clinic.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404Clinic, req404Clinic)
	if rec404Clinic.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown clinic, got %d", rec404Clinic.Code)
	}

	// Error case: unknown assessment/pet returns 404
	unknownPetReq, _ := json.Marshal(map[string]any{
		"petId":    "pet-without-triage",
		"clinicId": "clinic-er-seattle-1",
	})
	rec404Pet := httptest.NewRecorder()
	req404Pet := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/handoffs", bytes.NewReader(unknownPetReq))
	req404Pet.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(rec404Pet, req404Pet)
	if rec404Pet.Code != http.StatusNotFound {
		t.Errorf("expected 404 for pet without triage assessment, got %d", rec404Pet.Code)
	}

	// Error case: invalid status on PIMS webhook returns 400
	badWebhookReq, _ := json.Marshal(map[string]any{
		"status": "INVALID_STATUS",
	})
	recBadWebhook := httptest.NewRecorder()
	reqBadWebhook := httptest.NewRequest(http.MethodPost, "/api/v1/veterinary/handoffs/"+handoff.HandoffID+"/pims-webhook", bytes.NewReader(badWebhookReq))
	reqBadWebhook.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(recBadWebhook, reqBadWebhook)
	if recBadWebhook.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid webhook status, got %d", recBadWebhook.Code)
	}
}
