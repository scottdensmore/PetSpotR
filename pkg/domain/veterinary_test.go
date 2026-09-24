package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestVeterinaryPassport_Serialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	passport := domain.VeterinaryPassport{
		PassportID:       "vp-12345",
		PetID:            "pet-dog-1",
		PetName:          "Buster",
		Species:          "Dog",
		Breed:            "Labrador",
		MicrochipID:      "985141000123456",
		RabiesTagID:      "RAB-2026-99",
		WeightKg:         28.5,
		EmergencyContact: "555-0199",
		PrimaryClinic:    "Metro Veterinary Trauma",
		IssuedAt:         now,
		ExpiresAt:        now.AddDate(1, 0, 0),
		Vaccinations: []domain.VaccinationRecord{
			{
				VaccineName:      "Rabies 3-Yr",
				AdministeredDate: now.AddDate(-1, 0, 0),
				ExpirationDate:   now.AddDate(2, 0, 0),
				ClinicName:       "Metro Vet",
				Verified:         true,
			},
		},
		Allergies: []domain.ClinicalAllergy{
			{
				Allergen:            "Penicillin",
				Severity:            domain.AllergySeverityAnaphylactic,
				ReactionDescription: "Acute facial angioedema and bronchospasm",
			},
		},
		ChronicConditions: []domain.ChronicCondition{
			{
				ConditionName: "Osteoarthritis",
				DiagnosedDate: now.AddDate(-2, 0, 0),
				Medications:   []string{"Carprofen 75mg PO BID"},
				CriticalFlag:  false,
			},
		},
	}

	data, err := json.Marshal(passport)
	if err != nil {
		t.Fatalf("failed to marshal VeterinaryPassport: %v", err)
	}

	var decoded domain.VeterinaryPassport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal VeterinaryPassport: %v", err)
	}

	if decoded.PassportID != passport.PassportID {
		t.Errorf("expected PassportID %s, got %s", passport.PassportID, decoded.PassportID)
	}
	if len(decoded.Allergies) != 1 || decoded.Allergies[0].Severity != domain.AllergySeverityAnaphylactic {
		t.Errorf("expected 1 anaphylactic allergy, got %+v", decoded.Allergies)
	}
}

func TestTriageAssessment_Serialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	assessment := domain.TriageAssessment{
		AssessmentID: "triage-001",
		PetID:        "pet-dog-1",
		HubID:        "hub-mobile-triage-1",
		MedicID:      "medic-42",
		MedicName:    "Dr. Aris Thorne",
		Species:      "Dog",
		WeightKg:     28.5,
		Category:     domain.TriageCategoryRed,
		Vitals: domain.VitalSigns{
			HeartRateBPM:       195,
			RespiratoryRateBPM: 55,
			TemperatureF:       103.8,
			CapillaryRefillSec: 3.2,
			MucousMembrane:     domain.MMColorCyanotic,
			GlasgowComaScale:   8,
		},
		TraumaNotes: "Suspected smoke inhalation and partial thickness burns on paws",
		AdministeredTreatments: []domain.ClinicalTreatment{
			{
				TreatmentID:    "treat-001",
				MedicationName: "Oxygen 100% via flow-by",
				Dosage:         "5 L/min",
				Route:          domain.RouteTopical,
				AdministeredBy: "Dr. Aris Thorne",
				AdministeredAt: now,
			},
		},
		AssessedAt: now,
	}

	data, err := json.Marshal(assessment)
	if err != nil {
		t.Fatalf("failed to marshal TriageAssessment: %v", err)
	}

	var decoded domain.TriageAssessment
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal TriageAssessment: %v", err)
	}

	if decoded.Category != domain.TriageCategoryRed {
		t.Errorf("expected category RED, got %s", decoded.Category)
	}
	if decoded.Vitals.MucousMembrane != domain.MMColorCyanotic {
		t.Errorf("expected cyanotic MM, got %s", decoded.Vitals.MucousMembrane)
	}
}

func TestVeterinaryStoreCollectionConstants(t *testing.T) {
	if store.CollectionVeterinaryPassports != "veterinary_passports" {
		t.Errorf("unexpected CollectionVeterinaryPassports: %s", store.CollectionVeterinaryPassports)
	}
	if store.CollectionTriageAssessments != "triage_assessments" {
		t.Errorf("unexpected CollectionTriageAssessments: %s", store.CollectionTriageAssessments)
	}
}
