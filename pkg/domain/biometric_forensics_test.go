package domain_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func validSampleForensicCase() domain.BiometricForensicCase {
	return domain.BiometricForensicCase{
		CaseID:                   "forensic-case-101",
		PetID:                    "pet-2026-001",
		PetName:                  "Barnaby",
		Species:                  "DOG",
		AgeYears:                 14,
		SensoryCondition:         "CCD_SUNDOWNING_DEMENTIA",
		BiometricModality:        "NASAL_PLANUM_RIDGE_PRINT",
		NearestHazardType:        "RETENTION_POND_CULVERT",
		EntrapmentRiskScore:      95,
		EntrapmentRisk:           domain.EntrapmentRiskCriticalSundowningHazard,
		ResolutionStatus:         domain.ForensicStatusPendingBiometricScan,
		SundowningWanderAlert:    true,
		NasalPrintMatched:        false,
		WaterCulvertCleared:      false,
		BiometricConfidenceScore: 0,
		VerificationSteps: []domain.ForensicVerificationStep{
			{
				StepID:    "for-step-nasal-ridge",
				Title:     "Execute Gabor-wavelet ridge matching on nasal planum dermatoglyphic macro photo",
				Category:  "NASAL_PLANUM_BIOMETRICS",
				Completed: false,
			},
		},
		CreatedAt: "2026-10-18T12:00:00Z",
		UpdatedAt: "2026-10-18T12:00:00Z",
	}
}

func TestBiometricForensicCase_Validate(t *testing.T) {
	base := validSampleForensicCase()
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid sample forensic case, got error: %v", err)
	}

	if !domain.ForensicStatusPendingBiometricScan.IsValid() ||
		!domain.ForensicStatusDermatoglyphicCorrelated.IsValid() ||
		!domain.ForensicStatusIdentityConfirmedSafe.IsValid() {
		t.Errorf("expected canonical ForensicResolutionStatus values to be valid")
	}
	if domain.ForensicResolutionStatus("UNKNOWN").IsValid() {
		t.Errorf("expected unknown ForensicResolutionStatus to be invalid")
	}

	if !domain.EntrapmentRiskCriticalSundowningHazard.IsValid() ||
		!domain.EntrapmentRiskElevatedSensoryWander.IsValid() ||
		!domain.EntrapmentRiskContainedSafePeriphery.IsValid() {
		t.Errorf("expected canonical CognitiveEntrapmentRisk values to be valid")
	}
	if domain.CognitiveEntrapmentRisk("UNKNOWN").IsValid() {
		t.Errorf("expected unknown CognitiveEntrapmentRisk to be invalid")
	}

	tests := []struct {
		name   string
		mutate func(*domain.BiometricForensicCase)
	}{
		{
			name:   "empty caseId",
			mutate: func(c *domain.BiometricForensicCase) { c.CaseID = "" },
		},
		{
			name:   "empty petId",
			mutate: func(c *domain.BiometricForensicCase) { c.PetID = "" },
		},
		{
			name:   "empty petName",
			mutate: func(c *domain.BiometricForensicCase) { c.PetName = "" },
		},
		{
			name:   "invalid species",
			mutate: func(c *domain.BiometricForensicCase) { c.Species = "BIRD" },
		},
		{
			name:   "ageYears too low",
			mutate: func(c *domain.BiometricForensicCase) { c.AgeYears = 0 },
		},
		{
			name:   "ageYears too high",
			mutate: func(c *domain.BiometricForensicCase) { c.AgeYears = 29 },
		},
		{
			name:   "invalid sensoryCondition",
			mutate: func(c *domain.BiometricForensicCase) { c.SensoryCondition = "UNKNOWN_COND" },
		},
		{
			name:   "invalid biometricModality",
			mutate: func(c *domain.BiometricForensicCase) { c.BiometricModality = "UNKNOWN_MOD" },
		},
		{
			name:   "invalid nearestHazardType",
			mutate: func(c *domain.BiometricForensicCase) { c.NearestHazardType = "UNKNOWN_HAZARD" },
		},
		{
			name:   "entrapmentRiskScore out of range",
			mutate: func(c *domain.BiometricForensicCase) { c.EntrapmentRiskScore = 105 },
		},
		{
			name:   "invalid entrapmentRisk",
			mutate: func(c *domain.BiometricForensicCase) { c.EntrapmentRisk = "BAD_RISK" },
		},
		{
			name:   "invalid resolutionStatus",
			mutate: func(c *domain.BiometricForensicCase) { c.ResolutionStatus = "BAD_STATUS" },
		},
		{
			name:   "biometricConfidenceScore out of range",
			mutate: func(c *domain.BiometricForensicCase) { c.BiometricConfidenceScore = -5 },
		},
		{
			name:   "empty verificationSteps",
			mutate: func(c *domain.BiometricForensicCase) { c.VerificationSteps = nil },
		},
		{
			name: "invalid step category",
			mutate: func(c *domain.BiometricForensicCase) {
				c.VerificationSteps[0].Category = "BAD_CAT"
			},
		},
		{
			name: "confirmed extraction without certificate ID",
			mutate: func(c *domain.BiometricForensicCase) {
				c.ResolutionStatus = domain.ForensicStatusIdentityConfirmedSafe
				c.ForensicCertificateID = ""
			},
		},
		{
			name:   "empty createdAt",
			mutate: func(c *domain.BiometricForensicCase) { c.CreatedAt = "" },
		},
		{
			name:   "empty updatedAt",
			mutate: func(c *domain.BiometricForensicCase) { c.UpdatedAt = "" },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validSampleForensicCase()
			tc.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}
