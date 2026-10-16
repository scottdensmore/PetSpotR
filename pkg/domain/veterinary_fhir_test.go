package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestVeterinaryFHIR_PIMSProviderIsValid(t *testing.T) {
	validProviders := []PIMSProvider{
		PIMSCornerstone,
		PIMSEzyVet,
		PIMSAvimark,
		PIMSShelterluv,
		PIMSGenericFHIR,
	}
	for _, p := range validProviders {
		if !p.IsValid() {
			t.Errorf("expected PIMSProvider %q to be valid", p)
		}
	}

	invalidProviders := []PIMSProvider{
		"",
		"UNKNOWN_PIMS",
		"idexx_cornerstone",
	}
	for _, p := range invalidProviders {
		if p.IsValid() {
			t.Errorf("expected PIMSProvider %q to be invalid", p)
		}
	}
}

func TestVeterinaryFHIR_HandoffStatusIsValid(t *testing.T) {
	validStatuses := []HandoffStatus{
		HandoffStatusEnRoute,
		HandoffStatusReceivedInTreatment,
		HandoffStatusStabilized,
		HandoffStatusClearedForReunification,
	}
	for _, s := range validStatuses {
		if !s.IsValid() {
			t.Errorf("expected HandoffStatus %q to be valid", s)
		}
	}

	invalidStatuses := []HandoffStatus{
		"",
		"PENDING",
		"en_route_pre_arrival",
	}
	for _, s := range invalidStatuses {
		if s.IsValid() {
			t.Errorf("expected HandoffStatus %q to be invalid", s)
		}
	}
}

func TestVeterinaryFHIR_ClinicValidate(t *testing.T) {
	validClinic := func() *VeterinaryClinic {
		return &VeterinaryClinic{
			ClinicID:          "clinic-101",
			Name:              "Cascadia Emergency Vet Hospital",
			PIMS:              PIMSCornerstone,
			Emergency24Hr:     true,
			Phone:             "+1-503-555-0199",
			Address:           "1200 NW 14th Ave, Portland, OR",
			Latitude:          45.5315,
			Longitude:         -122.6855,
			AcceptingPatients: true,
			CreatedAt:         time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC),
		}
	}

	if err := validClinic().Validate(); err != nil {
		t.Fatalf("expected valid clinic to pass validation, got: %v", err)
	}

	var nilClinic *VeterinaryClinic
	if err := nilClinic.Validate(); err == nil {
		t.Fatal("expected nil clinic validation to fail")
	}

	emptyName := validClinic()
	emptyName.Name = "   "
	if err := emptyName.Validate(); err == nil {
		t.Fatal("expected empty clinic name validation to fail")
	}

	invalidPIMS := validClinic()
	invalidPIMS.PIMS = "UNSUPPORTED"
	if err := invalidPIMS.Validate(); err == nil {
		t.Fatal("expected invalid PIMS validation to fail")
	}

	latLow := validClinic()
	latLow.Latitude = -90.1
	if err := latLow.Validate(); err == nil {
		t.Fatal("expected latitude < -90 validation to fail")
	}

	latHigh := validClinic()
	latHigh.Latitude = 90.1
	if err := latHigh.Validate(); err == nil {
		t.Fatal("expected latitude > 90 validation to fail")
	}

	lonLow := validClinic()
	lonLow.Longitude = -180.1
	if err := lonLow.Validate(); err == nil {
		t.Fatal("expected longitude < -180 validation to fail")
	}

	lonHigh := validClinic()
	lonHigh.Longitude = 180.1
	if err := lonHigh.Validate(); err == nil {
		t.Fatal("expected longitude > 180 validation to fail")
	}
}

func TestVeterinaryFHIR_JSONSerialization(t *testing.T) {
	now := time.Date(2026, 10, 16, 15, 30, 0, 0, time.UTC)

	handoff := ClinicalHandoff{
		HandoffID:           "handoff-001",
		AssessmentID:        "assess-001",
		PetID:               "pet-77",
		PassportID:          "pass-77",
		ClinicID:            "clinic-101",
		ClinicName:          "Cascadia Emergency Vet Hospital",
		PIMS:                PIMSEzyVet,
		TriageCategory:      TriageCategoryRed,
		TransportUnit:       "RESCUE-VAN-4",
		ETAMinutes:          12,
		Status:              HandoffStatusEnRoute,
		VerifiedMicrochipID: "985112009988776",
		AttendingVet:        "Dr. Lin",
		DischargeSummary:    "Stabilized on IV fluids",
		ReadyForReunion:     false,
		FHIRBundle: FHIRVeterinaryBundle{
			ResourceType:    "Bundle",
			ID:              "bundle-handoff-001",
			Type:            "collection",
			Timestamp:       now,
			BundleDigestHex: "abcdef0123456789",
			Entry: []FHIRResourceEntry{
				{
					FullURL: "urn:uuid:patient-pet-77",
					Resource: map[string]any{
						"resourceType": "Patient",
						"id":           "pet-77",
					},
				},
			},
		},
		PIMSRecord: PIMSIntakeRecord{
			HandoffID:          "handoff-001",
			ClinicID:           "clinic-101",
			ClinicName:         "Cascadia Emergency Vet Hospital",
			PIMS:               PIMSEzyVet,
			PetID:              "pet-77",
			PetName:            "Buster",
			Species:            "Canine",
			Breed:              "Australian Shepherd",
			WeightKg:           22.5,
			MicrochipID:        "985112009988776",
			TriageCategory:     TriageCategoryRed,
			HeartRateBPM:       145,
			RespiratoryRateBPM: 36,
			TemperatureF:       102.8,
			CriticalAllergies:  []string{"Penicillin (ANAPHYLACTIC)"},
			TreatmentsGiven:    []string{"Lactated Ringers 250mL (IV)"},
			AttendingVet:       "Dr. Lin",
			DischargeSummary:   "Stabilized on IV fluids",
		},
		DispatchedAt: now,
		UpdatedAt:    now,
	}

	raw, err := json.Marshal(handoff)
	if err != nil {
		t.Fatalf("marshal clinical handoff: %v", err)
	}

	var decoded ClinicalHandoff
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal clinical handoff: %v", err)
	}

	if decoded.HandoffID != handoff.HandoffID {
		t.Errorf("expected HandoffID %q, got %q", handoff.HandoffID, decoded.HandoffID)
	}
	if decoded.PIMS != PIMSEzyVet {
		t.Errorf("expected PIMS %q, got %q", PIMSEzyVet, decoded.PIMS)
	}
	if decoded.FHIRBundle.BundleDigestHex != "abcdef0123456789" {
		t.Errorf("expected BundleDigestHex %q, got %q", "abcdef0123456789", decoded.FHIRBundle.BundleDigestHex)
	}
	if len(decoded.FHIRBundle.Entry) != 1 || decoded.FHIRBundle.Entry[0].Resource["resourceType"] != "Patient" {
		t.Errorf("unexpected FHIR bundle entries: %+v", decoded.FHIRBundle.Entry)
	}
	if decoded.PIMSRecord.PetName != "Buster" || len(decoded.PIMSRecord.CriticalAllergies) != 1 {
		t.Errorf("unexpected PIMSRecord after roundtrip: %+v", decoded.PIMSRecord)
	}
}
