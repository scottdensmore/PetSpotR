package domain_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func validSampleFosterMatch() domain.FosterPlacementMatch {
	return domain.FosterPlacementMatch{
		PlacementID:        "foster-match-101",
		PetID:              "pet-2026-002",
		PetName:            "Barnaby",
		Species:            "DOG",
		FosterHostName:     "Elena Vance (RN / Vet Tech)",
		ContagionFlag:      "PARVO_SUSPECTED",
		MedicalNeedLevel:   "INJECTABLE_SUBQ_FLUIDS",
		ResidentPetsCount:  2,
		ContagionRiskScore: 95,
		BiosecurityTier:    domain.BiosecurityTierIsoQuarantine,
		PlacementStatus:    domain.FosterStatusPendingBiosecurity,
		IsoQuarantineAlert: true,
		HEPAIsolationReady: false,
		MedCertVerified:    false,
		HostReadinessScore: 0,
		BiosecurityChecks: []domain.FosterBiosecurityCheckItem{
			{
				CheckID:   "bio-chk-bleach-wash",
				Title:     "Complete rescue-grade accelerated hydrogen peroxide (AHP) surface sterilization",
				Category:  "PATHOGEN_ISOLATION",
				Completed: false,
			},
		},
		CreatedAt: "2026-10-18T12:00:00Z",
		UpdatedAt: "2026-10-18T12:00:00Z",
	}
}

func TestFosterPlacementMatch_Validate(t *testing.T) {
	base := validSampleFosterMatch()
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid sample match, got error: %v", err)
	}

	if !domain.FosterStatusPendingBiosecurity.IsValid() ||
		!domain.FosterStatusQuarantineVerified.IsValid() ||
		!domain.FosterStatusPlacedInFosterHome.IsValid() {
		t.Errorf("expected canonical FosterPlacementStatus values to be valid")
	}
	if domain.FosterPlacementStatus("UNKNOWN").IsValid() {
		t.Errorf("expected unknown FosterPlacementStatus to be invalid")
	}

	if !domain.BiosecurityTierIsoQuarantine.IsValid() ||
		!domain.BiosecurityTierMedicalSinglePet.IsValid() ||
		!domain.BiosecurityTierClearedStandard.IsValid() {
		t.Errorf("expected canonical FosterBiosecurityTier values to be valid")
	}
	if domain.FosterBiosecurityTier("UNKNOWN").IsValid() {
		t.Errorf("expected unknown FosterBiosecurityTier to be invalid")
	}

	tests := []struct {
		name   string
		mutate func(*domain.FosterPlacementMatch)
	}{
		{
			name:   "empty placementId",
			mutate: func(m *domain.FosterPlacementMatch) { m.PlacementID = "" },
		},
		{
			name:   "empty petId",
			mutate: func(m *domain.FosterPlacementMatch) { m.PetID = "" },
		},
		{
			name:   "empty petName",
			mutate: func(m *domain.FosterPlacementMatch) { m.PetName = "" },
		},
		{
			name:   "invalid species",
			mutate: func(m *domain.FosterPlacementMatch) { m.Species = "RABBIT" },
		},
		{
			name:   "empty fosterHostName",
			mutate: func(m *domain.FosterPlacementMatch) { m.FosterHostName = "" },
		},
		{
			name:   "invalid contagionFlag",
			mutate: func(m *domain.FosterPlacementMatch) { m.ContagionFlag = "UNKNOWN_VIRUS" },
		},
		{
			name:   "invalid medicalNeedLevel",
			mutate: func(m *domain.FosterPlacementMatch) { m.MedicalNeedLevel = "UNKNOWN_MED" },
		},
		{
			name:   "negative residentPetsCount",
			mutate: func(m *domain.FosterPlacementMatch) { m.ResidentPetsCount = -1 },
		},
		{
			name:   "contagionRiskScore out of range",
			mutate: func(m *domain.FosterPlacementMatch) { m.ContagionRiskScore = 105 },
		},
		{
			name:   "invalid biosecurityTier",
			mutate: func(m *domain.FosterPlacementMatch) { m.BiosecurityTier = "BAD_TIER" },
		},
		{
			name:   "invalid placementStatus",
			mutate: func(m *domain.FosterPlacementMatch) { m.PlacementStatus = "BAD_STATUS" },
		},
		{
			name:   "hostReadinessScore out of range",
			mutate: func(m *domain.FosterPlacementMatch) { m.HostReadinessScore = -5 },
		},
		{
			name:   "empty biosecurityChecks",
			mutate: func(m *domain.FosterPlacementMatch) { m.BiosecurityChecks = nil },
		},
		{
			name: "invalid check item category",
			mutate: func(m *domain.FosterPlacementMatch) {
				m.BiosecurityChecks[0].Category = "BAD_CAT"
			},
		},
		{
			name: "active placement without authorization code",
			mutate: func(m *domain.FosterPlacementMatch) {
				m.PlacementStatus = domain.FosterStatusPlacedInFosterHome
				m.PlacementAuthorizationCode = ""
			},
		},
		{
			name:   "empty createdAt",
			mutate: func(m *domain.FosterPlacementMatch) { m.CreatedAt = "" },
		},
		{
			name:   "empty updatedAt",
			mutate: func(m *domain.FosterPlacementMatch) { m.UpdatedAt = "" },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validSampleFosterMatch()
			tc.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}
