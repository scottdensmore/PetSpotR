package domain

import (
	"testing"
	"time"
)

func validMicrochipScanCase() MicrochipScanCase {
	now := time.Date(2026, 10, 18, 5, 0, 0, 0, time.UTC)
	return MicrochipScanCase{
		CaseID:                 "chip-case-101",
		PetID:                  "pet-2026-001",
		PetName:                "Barnaby",
		Species:                "DOG",
		RawChipInput:           "985-112-004-991-203",
		NormalizedChipID:       "985112004991203",
		MaskedChipDisplay:      "985-***-***-991-203",
		ChipStandard:           ChipStandardISO11784_15,
		FrequencyKhz:           134.2,
		ManufacturerCode:       "985",
		ManufacturerName:       "HomeAgain / Merck Animal Health",
		ManufacturerLotID:      "LOT-MERCK-2024-88A",
		Status:                 ChipStatusRegisteredOwnerFound,
		ScannerLocation:        GeoPoint{Latitude: 47.6588, Longitude: -122.3851},
		ScannedByAgency:        "Seattle Animal Shelter Intake #2",
		WinningRegistry:        "HomeAgain National Recovery Database",
		ImplantingClinicName:   "Ballard Veterinary Hospital",
		ImplantingClinicPhone:  "(206) 555-0142",
		ImplantPatientChartID:  "BVH-CHART-88412",
		ResolvedOwnerName:      "Elena Vance",
		ResolvedOwnerPhoneMask: "(206) ***-8819",
		ResolutionNotes:        "AAHA Universal Lookup resolved active owner registration on HomeAgain.",
		RegistryHops: []RegistryQueryHop{
			{
				RegistryName:    "HomeAgain National Recovery Database",
				ManufacturerISO: "985",
				Responded:       true,
				RecordFound:     true,
				LastUpdatedYear: 2026,
				StatusNote:      "Active primary owner registration verified.",
			},
			{
				RegistryName:    "AKC Reunite",
				ManufacturerISO: "956",
				Responded:       true,
				RecordFound:     false,
				LastUpdatedYear: 2024,
				StatusNote:      "Prior enrollment superseded by 2026 update.",
			},
		},
		CreatedAt: now.Add(-45 * time.Minute),
		UpdatedAt: now,
	}
}

func TestMicrochipFederationEnums(t *testing.T) {
	t.Parallel()

	for _, std := range []MicrochipStandardType{
		ChipStandardISO11784_15,
		ChipStandardAVID9,
		ChipStandardHex10Legacy,
	} {
		if !ValidMicrochipStandardType(std) {
			t.Errorf("expected ValidMicrochipStandardType(%q) = true", std)
		}
		if !std.IsValid() {
			t.Errorf("expected (%q).IsValid() = true", std)
		}
	}
	if ValidMicrochipStandardType("INVALID_STD") {
		t.Error("expected ValidMicrochipStandardType(INVALID_STD) = false")
	}
	if MicrochipStandardType("INVALID_STD").IsValid() {
		t.Error("expected MicrochipStandardType(INVALID_STD).IsValid() = false")
	}

	for _, st := range []FederationResolutionStatus{
		ChipStatusRegisteredOwnerFound,
		ChipStatusUnregisteredBatchTraced,
		ChipStatusSecondaryContactEscalated,
		ChipStatusReunionDispatchedVerified,
	} {
		if !ValidFederationResolutionStatus(st) {
			t.Errorf("expected ValidFederationResolutionStatus(%q) = true", st)
		}
		if !st.IsValid() {
			t.Errorf("expected (%q).IsValid() = true", st)
		}
	}
	if ValidFederationResolutionStatus("INVALID_STATUS") {
		t.Error("expected ValidFederationResolutionStatus(INVALID_STATUS) = false")
	}
	if FederationResolutionStatus("INVALID_STATUS").IsValid() {
		t.Error("expected FederationResolutionStatus(INVALID_STATUS).IsValid() = false")
	}
}

func TestMicrochipFederationValidation(t *testing.T) {
	t.Parallel()

	base := validMicrochipScanCase()
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid microchip scan case, got error: %v", err)
	}

	reunion := validMicrochipScanCase()
	reunion.Status = ChipStatusReunionDispatchedVerified
	reunion.ResolvedOwnerName = "Elena Vance"
	reunion.ResolvedOwnerPhoneMask = "(206) ***-8819"
	if err := reunion.Validate(); err != nil {
		t.Fatalf("expected valid reunion scan case, got error: %v", err)
	}

	unregistered := validMicrochipScanCase()
	unregistered.Status = ChipStatusUnregisteredBatchTraced
	unregistered.ResolvedOwnerName = ""
	unregistered.ResolvedOwnerPhoneMask = ""
	if err := unregistered.Validate(); err != nil {
		t.Fatalf("expected valid unregistered batch traced scan case without owner fields, got error: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(c *MicrochipScanCase)
	}{
		{"empty CaseID", func(c *MicrochipScanCase) { c.CaseID = "  " }},
		{"empty PetID", func(c *MicrochipScanCase) { c.PetID = "" }},
		{"empty PetName", func(c *MicrochipScanCase) { c.PetName = " " }},
		{"empty RawChipInput", func(c *MicrochipScanCase) { c.RawChipInput = "" }},
		{"empty NormalizedChipID", func(c *MicrochipScanCase) { c.NormalizedChipID = " " }},
		{"empty MaskedChipDisplay", func(c *MicrochipScanCase) { c.MaskedChipDisplay = "" }},
		{"empty ManufacturerCode", func(c *MicrochipScanCase) { c.ManufacturerCode = "" }},
		{"empty ManufacturerName", func(c *MicrochipScanCase) { c.ManufacturerName = " " }},
		{"empty ManufacturerLotID", func(c *MicrochipScanCase) { c.ManufacturerLotID = "" }},
		{"empty ScannedByAgency", func(c *MicrochipScanCase) { c.ScannedByAgency = "" }},
		{"empty ResolutionNotes", func(c *MicrochipScanCase) { c.ResolutionNotes = " " }},
		{"empty Species", func(c *MicrochipScanCase) { c.Species = "" }},
		{"lowercase Species", func(c *MicrochipScanCase) { c.Species = "dog" }},
		{"invalid ChipStandard", func(c *MicrochipScanCase) { c.ChipStandard = "UNKNOWN_STD" }},
		{"too low FrequencyKhz", func(c *MicrochipScanCase) { c.FrequencyKhz = 100.0 }},
		{"too high FrequencyKhz", func(c *MicrochipScanCase) { c.FrequencyKhz = 150.1 }},
		{"invalid Status", func(c *MicrochipScanCase) { c.Status = "UNKNOWN_STATUS" }},
		{"invalid ScannerLocation lat", func(c *MicrochipScanCase) { c.ScannerLocation.Latitude = 92.0 }},
		{"invalid ScannerLocation lng", func(c *MicrochipScanCase) { c.ScannerLocation.Longitude = -182.0 }},
		{"empty RegistryHops", func(c *MicrochipScanCase) { c.RegistryHops = nil }},
		{"hop missing RegistryName", func(c *MicrochipScanCase) {
			c.RegistryHops = []RegistryQueryHop{
				{RegistryName: " ", StatusNote: "Checked"},
			}
		}},
		{"hop missing StatusNote", func(c *MicrochipScanCase) {
			c.RegistryHops = []RegistryQueryHop{
				{RegistryName: "24PetWatch", StatusNote: ""},
			}
		}},
		{"reunion status missing ResolvedOwnerName", func(c *MicrochipScanCase) {
			c.Status = ChipStatusReunionDispatchedVerified
			c.ResolvedOwnerName = " "
			c.ResolvedOwnerPhoneMask = "(206) ***-8819"
		}},
		{"reunion status missing ResolvedOwnerPhoneMask", func(c *MicrochipScanCase) {
			c.Status = ChipStatusReunionDispatchedVerified
			c.ResolvedOwnerName = "Elena Vance"
			c.ResolvedOwnerPhoneMask = ""
		}},
		{"zero CreatedAt", func(c *MicrochipScanCase) { c.CreatedAt = time.Time{} }},
		{"zero UpdatedAt", func(c *MicrochipScanCase) { c.UpdatedAt = time.Time{} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validMicrochipScanCase()
			tc.mutate(&c)
			if err := c.Validate(); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}
