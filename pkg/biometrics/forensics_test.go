package biometrics_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/biometrics"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvaluateBiometricForensicCase_SundowningSenior(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	fc, err := biometrics.EvaluateBiometricForensicCase(biometrics.CreateForensicCaseInput{
		CaseID:              "forensic-case-101",
		PetID:               "pet-2026-001",
		PetName:             "Barnaby",
		Species:             "DOG",
		AgeYears:            14,
		SensoryCondition:    "CCD_SUNDOWNING_DEMENTIA",
		BiometricModality:   "NASAL_PLANUM_RIDGE_PRINT",
		NearestHazardType:   "RETENTION_POND_CULVERT",
		NasalPrintMatched:   false,
		WaterCulvertCleared: false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating forensic case: %v", err)
	}

	if fc.ResolutionStatus != domain.ForensicStatusPendingBiometricScan {
		t.Errorf("expected PENDING_BIOMETRIC_SCAN, got %s", fc.ResolutionStatus)
	}
	if fc.EntrapmentRisk != domain.EntrapmentRiskCriticalSundowningHazard {
		t.Errorf("expected CRITICAL_SUNDOWNING_ENTRAPMENT, got %s", fc.EntrapmentRisk)
	}
	if !fc.SundowningWanderAlert {
		t.Errorf("expected SundowningWanderAlert=true for 14yo CCD dog")
	}
	if fc.EntrapmentRiskScore != 95 {
		t.Errorf("expected EntrapmentRiskScore=95, got %d", fc.EntrapmentRiskScore)
	}
	if len(fc.VerificationSteps) != 4 {
		t.Errorf("expected 4 verification steps, got %d", len(fc.VerificationSteps))
	}
}

func TestApplyForensicCaseUpdate_CorrelateAndConfirm(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	fc, err := biometrics.EvaluateBiometricForensicCase(biometrics.CreateForensicCaseInput{
		CaseID:              "forensic-case-102",
		PetID:               "pet-2026-002",
		PetName:             "Clementine",
		Species:             "CAT",
		AgeYears:            12,
		SensoryCondition:    "BILATERAL_CATARACT_BLIND",
		BiometricModality:   "IRIS_MELANIN_TOPOLOGY",
		NearestHazardType:   "THICK_BLACKBERRY_BRAMBLE",
		NasalPrintMatched:   false,
		WaterCulvertCleared: false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating forensic case: %v", err)
	}

	// 1. CorrelateBiometricAndSweepHazard -> DERMATOGLYPHIC_CORRELATED
	if err := biometrics.ApplyForensicCaseUpdate(fc, biometrics.ForensicCaseUpdateInput{
		CorrelateBiometricAndSweepHazard: true,
	}, now.Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error correlating biometric case: %v", err)
	}

	if fc.ResolutionStatus != domain.ForensicStatusDermatoglyphicCorrelated {
		t.Errorf("expected DERMATOGLYPHIC_CORRELATED, got %s", fc.ResolutionStatus)
	}
	if fc.EntrapmentRisk != domain.EntrapmentRiskElevatedSensoryWander {
		t.Errorf("expected ELEVATED_SENSORY_DISORIENTATION, got %s", fc.EntrapmentRisk)
	}
	if fc.BiometricConfidenceScore != 100 {
		t.Errorf("expected BiometricConfidenceScore=100, got %d", fc.BiometricConfidenceScore)
	}

	// 2. Complete individual step
	if err := biometrics.ApplyForensicCaseUpdate(fc, biometrics.ForensicCaseUpdateInput{
		CompleteStepID: "for-step-olfactory-beacon",
	}, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing step: %v", err)
	}

	// 3. Unknown step ID returns error
	if err := biometrics.ApplyForensicCaseUpdate(fc, biometrics.ForensicCaseUpdateInput{
		CompleteStepID: "unknown-step",
	}, now.Add(3*time.Minute)); err == nil {
		t.Errorf("expected error for unknown step ID")
	}

	// 4. ConfirmForensicExtraction -> IDENTITY_CONFIRMED_SAFE_EXTRACTION
	if err := biometrics.ApplyForensicCaseUpdate(fc, biometrics.ForensicCaseUpdateInput{
		ConfirmForensicExtraction: true,
	}, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("unexpected error confirming forensic extraction: %v", err)
	}

	if fc.ResolutionStatus != domain.ForensicStatusIdentityConfirmedSafe {
		t.Errorf("expected IDENTITY_CONFIRMED_SAFE_EXTRACTION, got %s", fc.ResolutionStatus)
	}
	if fc.EntrapmentRisk != domain.EntrapmentRiskContainedSafePeriphery {
		t.Errorf("expected CONTAINED_SAFE_PERIPHERY, got %s", fc.EntrapmentRisk)
	}
	if fc.EntrapmentRiskScore != 5 {
		t.Errorf("expected EntrapmentRiskScore=5, got %d", fc.EntrapmentRiskScore)
	}
	if fc.ForensicCertificateID != "CERT-FORENSIC-FORENSIC-CASE-102" {
		t.Errorf("expected CERT-FORENSIC-FORENSIC-CASE-102, got %s", fc.ForensicCertificateID)
	}
}

func TestEvaluateBiometricForensicCase_ValidationErrors(t *testing.T) {
	now := time.Now().UTC()

	if _, err := biometrics.EvaluateBiometricForensicCase(biometrics.CreateForensicCaseInput{
		CaseID:   "",
		PetID:    "pet-1",
		PetName:  "Buddy",
		AgeYears: 5,
	}, now); err == nil {
		t.Errorf("expected error for missing caseId")
	}

	if _, err := biometrics.EvaluateBiometricForensicCase(biometrics.CreateForensicCaseInput{
		CaseID:   "forensic-1",
		PetID:    "pet-1",
		PetName:  "Buddy",
		AgeYears: 0,
	}, now); err == nil {
		t.Errorf("expected error for out-of-range ageYears")
	}

	if err := biometrics.ApplyForensicCaseUpdate(nil, biometrics.ForensicCaseUpdateInput{}, now); err == nil {
		t.Errorf("expected error for nil forensic case")
	}
}
