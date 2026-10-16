package veterinary

import (
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func sampleAssessmentAndPassport() (domain.TriageAssessment, *domain.VeterinaryPassport, domain.VeterinaryClinic) {
	now := time.Date(2026, 10, 16, 15, 0, 0, 0, time.UTC)
	assessment := domain.TriageAssessment{
		AssessmentID: "assess-101",
		PetID:        "pet-901",
		HubID:        "hub-alpha",
		MedicID:      "medic-1",
		MedicName:    "Dr. Sarah Chen",
		Species:      "Dog",
		WeightKg:     26.5,
		Category:     domain.TriageCategoryRed,
		Vitals: domain.VitalSigns{
			HeartRateBPM:       165,
			HeartRateAssessed:  true,
			RespiratoryRateBPM: 42,
			RespRateAssessed:   true,
			TemperatureF:       103.8,
			CapillaryRefillSec: 2.8,
			MucousMembrane:     domain.MMColorPale,
			GlasgowComaScale:   12,
		},
		TraumaNotes: "Smoke inhalation and singed paw pads from wildfire evacuation zone",
		AdministeredTreatments: []domain.ClinicalTreatment{
			{
				TreatmentID:    "tx-1",
				MedicationName: "Lactated Ringers",
				Dosage:         "400mL",
				Route:          domain.RouteIV,
				AdministeredBy: "Dr. Sarah Chen",
				AdministeredAt: now,
			},
			{
				TreatmentID:    "tx-2",
				MedicationName: "Buprenorphine",
				Dosage:         "0.5mg",
				Route:          domain.RouteIM,
				AdministeredBy: "Dr. Sarah Chen",
				AdministeredAt: now,
			},
		},
		AssessedAt: now,
	}

	passport := &domain.VeterinaryPassport{
		PassportID:  "pass-501",
		PetID:       "pet-901",
		PetName:     "Koda",
		Species:     "Dog",
		Breed:       "Australian Shepherd",
		MicrochipID: "985141002394812",
		RabiesTagID: "RAB-2026-991",
		BloodType:   "DEA 1.1+",
		WeightKg:    26.5,
		Allergies: []domain.ClinicalAllergy{
			{
				Allergen:            "Penicillin",
				Severity:            domain.AllergySeverityAnaphylactic,
				ReactionDescription: "Acute laryngeal edema and hypotension",
			},
			{
				Allergen:            "Carprofen",
				Severity:            domain.AllergySeverityMild,
				ReactionDescription: "GI upset",
			},
		},
		Vaccinations: []domain.VaccinationRecord{
			{
				VaccineName:      "Rabies 3-Year",
				AdministeredDate: now.AddDate(-1, 0, 0),
				ExpirationDate:   now.AddDate(2, 0, 0),
				ClinicName:       "Pacific Vet Med",
				Verified:         true,
			},
		},
		IssuedAt:  now.AddDate(-1, 0, 0),
		ExpiresAt: now.AddDate(1, 0, 0),
	}

	clinic := domain.VeterinaryClinic{
		ClinicID:          "clinic-pdx-1",
		Name:              "DoveLewis Emergency Animal Hospital",
		PIMS:              domain.PIMSEzyVet,
		Emergency24Hr:     true,
		Phone:             "503-228-7281",
		Address:           "1945 NW Pettygrove St, Portland, OR",
		Latitude:          45.5335,
		Longitude:         -122.6915,
		AcceptingPatients: true,
		CreatedAt:         now,
	}

	return assessment, passport, clinic
}

func TestFHIRBuildAndValidateBundle(t *testing.T) {
	assessment, passport, clinic := sampleAssessmentAndPassport()

	bundle, err := BuildFHIRVeterinaryBundle(assessment, passport, clinic)
	if err != nil {
		t.Fatalf("BuildFHIRVeterinaryBundle unexpected error: %v", err)
	}

	if bundle.ResourceType != "Bundle" {
		t.Fatalf("expected ResourceType Bundle, got %q", bundle.ResourceType)
	}
	if bundle.Type != "document" {
		t.Fatalf("expected Type document, got %q", bundle.Type)
	}
	if bundle.ID != "fhir-bundle-assess-101" {
		t.Fatalf("expected ID fhir-bundle-assess-101, got %q", bundle.ID)
	}
	if bundle.BundleDigestHex == "" {
		t.Fatal("expected non-empty BundleDigestHex")
	}

	// Validate in-memory bundle
	if err := ValidateFHIRVeterinaryBundle(bundle); err != nil {
		t.Fatalf("ValidateFHIRVeterinaryBundle failed on in-memory bundle: %v", err)
	}

	// Verify resource types present
	resourceCounts := make(map[string]int)
	for _, entry := range bundle.Entry {
		rt, _ := entry.Resource["resourceType"].(string)
		resourceCounts[rt]++
	}

	expectedCounts := map[string]int{
		"Patient":                  1,
		"Encounter":                1,
		"Observation":              5, // weight, HR, RR, temp, panel
		"AllergyIntolerance":       2,
		"Immunization":             1,
		"MedicationAdministration": 2,
	}
	for rt, want := range expectedCounts {
		if got := resourceCounts[rt]; got != want {
			t.Errorf("resourceType %q count = %d, want %d", rt, got, want)
		}
	}

	// Validate JSON round-trip preserves digest validation
	rawJSON, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	var unmarshaled domain.FHIRVeterinaryBundle
	if err := json.Unmarshal(rawJSON, &unmarshaled); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	if err := ValidateFHIRVeterinaryBundle(unmarshaled); err != nil {
		t.Fatalf("ValidateFHIRVeterinaryBundle failed after JSON round-trip: %v", err)
	}

	// Tamper with an entry and ensure validation fails
	unmarshaled.Entry[1].Resource["status"] = "cancelled"
	if err := ValidateFHIRVeterinaryBundle(unmarshaled); err == nil {
		t.Fatal("expected ValidateFHIRVeterinaryBundle to fail on tampered bundle")
	}
}

func TestFHIRBuildBundleWithoutPassportAndValidationErrors(t *testing.T) {
	assessment, _, clinic := sampleAssessmentAndPassport()

	// Missing petId should error
	invalidAssessment := assessment
	invalidAssessment.PetID = "   "
	if _, err := BuildFHIRVeterinaryBundle(invalidAssessment, nil, clinic); err == nil {
		t.Fatal("expected error when assessment.PetID is empty")
	}

	// Nil passport should still build valid bundle with Patient, Encounter, Observations, Meds
	bundle, err := BuildFHIRVeterinaryBundle(assessment, nil, clinic)
	if err != nil {
		t.Fatalf("BuildFHIRVeterinaryBundle with nil passport failed: %v", err)
	}
	if err := ValidateFHIRVeterinaryBundle(bundle); err != nil {
		t.Fatalf("ValidateFHIRVeterinaryBundle with nil passport failed: %v", err)
	}

	// Invalid bundle resourceType
	badBundle := bundle
	badBundle.ResourceType = "Patient"
	if err := ValidateFHIRVeterinaryBundle(badBundle); err == nil {
		t.Fatal("expected error for invalid bundle ResourceType")
	}

	// Missing patient-animal extension
	noExtBundle := bundle
	noExtBundle.Entry = append([]domain.FHIRResourceEntry{}, bundle.Entry...)
	noExtBundle.Entry[0] = domain.FHIRResourceEntry{
		FullURL: "urn:uuid:patient-pet-901",
		Resource: map[string]any{
			"resourceType": "Patient",
			"id":           "pet-901",
		},
	}
	if err := ValidateFHIRVeterinaryBundle(noExtBundle); err == nil {
		t.Fatal("expected error when patient-animal extension is missing")
	}
}

func TestFHIRPIMSIntakeRecordAndCSVExport(t *testing.T) {
	assessment, passport, clinic := sampleAssessmentAndPassport()

	record := BuildPIMSIntakeRecord("handoff-777", assessment, passport, clinic)
	if record.HandoffID != "handoff-777" {
		t.Fatalf("expected HandoffID handoff-777, got %q", record.HandoffID)
	}
	if record.PetName != "Koda" || record.Breed != "Australian Shepherd" {
		t.Fatalf("unexpected pet identity in PIMS record: %+v", record)
	}
	if record.MicrochipID != "985141002394812" {
		t.Fatalf("expected microchip 985141002394812, got %q", record.MicrochipID)
	}
	if len(record.CriticalAllergies) != 2 || record.CriticalAllergies[0] != "Penicillin (ANAPHYLACTIC)" {
		t.Fatalf("unexpected CriticalAllergies: %+v", record.CriticalAllergies)
	}
	if len(record.TreatmentsGiven) != 2 || record.TreatmentsGiven[0] != "Lactated Ringers 400mL IV" {
		t.Fatalf("unexpected TreatmentsGiven: %+v", record.TreatmentsGiven)
	}

	record.AttendingVet = "Dr. Gregory House, DVM"
	record.DischargeSummary = "Oxygen therapy completed; cleared for shelter reunification"

	csvOut, err := FormatPIMSRecordCSV([]domain.PIMSIntakeRecord{record})
	if err != nil {
		t.Fatalf("FormatPIMSRecordCSV failed: %v", err)
	}

	rows, err := csv.NewReader(strings.NewReader(csvOut)).ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV output: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 CSV rows (header + 1 record), got %d", len(rows))
	}

	expectedHeader := "handoff_id,clinic_id,clinic_name,pims_provider,pet_id,pet_name,species,breed,weight_kg,microchip_id,triage_category,heart_rate_bpm,respiratory_rate_bpm,temperature_f,critical_allergies,treatments_given,attending_vet,discharge_summary"
	if strings.Join(rows[0], ",") != expectedHeader {
		t.Fatalf("CSV header mismatch:\ngot:  %s\nwant: %s", strings.Join(rows[0], ","), expectedHeader)
	}

	dataRow := rows[1]
	if dataRow[0] != "handoff-777" || dataRow[3] != string(domain.PIMSEzyVet) || dataRow[5] != "Koda" {
		t.Fatalf("unexpected CSV row values: %+v", dataRow)
	}
	if dataRow[14] != "Penicillin (ANAPHYLACTIC); Carprofen (MILD)" {
		t.Fatalf("unexpected critical_allergies column: %q", dataRow[14])
	}
	if dataRow[15] != "Lactated Ringers 400mL IV; Buprenorphine 0.5mg IM" {
		t.Fatalf("unexpected treatments_given column: %q", dataRow[15])
	}
	if dataRow[16] != "Dr. Gregory House, DVM" {
		t.Fatalf("unexpected attending_vet column: %q", dataRow[16])
	}
}
