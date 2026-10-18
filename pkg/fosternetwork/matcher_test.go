package fosternetwork_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/fosternetwork"
)

func TestEvaluateFosterPlacement_ParvoIsolation(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	match, err := fosternetwork.EvaluateFosterPlacement(fosternetwork.CreateFosterPlacementInput{
		PlacementID:        "foster-match-101",
		PetID:              "pet-2026-002",
		PetName:            "Barnaby",
		Species:            "DOG",
		FosterHostName:     "Elena Vance (RN / Vet Tech)",
		ContagionFlag:      "PARVO_SUSPECTED",
		MedicalNeedLevel:   "INJECTABLE_SUBQ_FLUIDS",
		ResidentPetsCount:  2,
		HEPAIsolationReady: false,
		MedCertVerified:    false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating foster placement: %v", err)
	}

	if match.PlacementStatus != domain.FosterStatusPendingBiosecurity {
		t.Errorf("expected PENDING_BIOSECURITY_CLEARANCE, got %s", match.PlacementStatus)
	}
	if match.BiosecurityTier != domain.BiosecurityTierIsoQuarantine {
		t.Errorf("expected ISO_QUARANTINE_CONTAGION, got %s", match.BiosecurityTier)
	}
	if !match.IsoQuarantineAlert {
		t.Errorf("expected IsoQuarantineAlert=true for PARVO_SUSPECTED")
	}
	if match.ContagionRiskScore != 95 {
		t.Errorf("expected ContagionRiskScore=95, got %d", match.ContagionRiskScore)
	}
	if len(match.BiosecurityChecks) != 4 {
		t.Errorf("expected 4 biosecurity checks, got %d", len(match.BiosecurityChecks))
	}
}

func TestApplyFosterPlacementTransition_VerifyAndActivate(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	match, err := fosternetwork.EvaluateFosterPlacement(fosternetwork.CreateFosterPlacementInput{
		PlacementID:        "foster-match-102",
		PetID:              "pet-2026-001",
		PetName:            "Willow",
		Species:            "CAT",
		FosterHostName:     "Marcus Sterling",
		ContagionFlag:      "RINGWORM_DERMATOPHYTE",
		MedicalNeedLevel:   "WOUND_DRESSING_BID",
		ResidentPetsCount:  0,
		HEPAIsolationReady: false,
		MedCertVerified:    false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating placement: %v", err)
	}

	// 1. VerifyBiosecurityRig -> QUARANTINE_VERIFIED_STAGED
	if err := fosternetwork.ApplyFosterPlacementTransition(match, fosternetwork.FosterPlacementUpdateInput{
		VerifyBiosecurityRig: true,
	}, now.Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error verifying biosecurity rig: %v", err)
	}

	if match.PlacementStatus != domain.FosterStatusQuarantineVerified {
		t.Errorf("expected QUARANTINE_VERIFIED_STAGED, got %s", match.PlacementStatus)
	}
	if match.BiosecurityTier != domain.BiosecurityTierMedicalSinglePet {
		t.Errorf("expected MEDICAL_RECOVERY_SINGLE_PET, got %s", match.BiosecurityTier)
	}
	if match.HostReadinessScore != 100 {
		t.Errorf("expected HostReadinessScore=100, got %d", match.HostReadinessScore)
	}

	// 2. Complete individual check item
	if err := fosternetwork.ApplyFosterPlacementTransition(match, fosternetwork.FosterPlacementUpdateInput{
		CompleteCheckID: "bio-chk-resident-sep",
	}, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing check item: %v", err)
	}

	// 3. Unknown check item returns error
	if err := fosternetwork.ApplyFosterPlacementTransition(match, fosternetwork.FosterPlacementUpdateInput{
		CompleteCheckID: "unknown-chk",
	}, now.Add(3*time.Minute)); err == nil {
		t.Errorf("expected error for unknown check ID")
	}

	// 4. ActivateFosterPlacement -> ACTIVE_IN_FOSTER_HOME
	if err := fosternetwork.ApplyFosterPlacementTransition(match, fosternetwork.FosterPlacementUpdateInput{
		ActivateFosterPlacement: true,
	}, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("unexpected error activating foster placement: %v", err)
	}

	if match.PlacementStatus != domain.FosterStatusPlacedInFosterHome {
		t.Errorf("expected ACTIVE_IN_FOSTER_HOME, got %s", match.PlacementStatus)
	}
	if match.BiosecurityTier != domain.BiosecurityTierClearedStandard {
		t.Errorf("expected BIOSECURITY_CLEARED_STANDARD, got %s", match.BiosecurityTier)
	}
	if match.ContagionRiskScore != 5 {
		t.Errorf("expected ContagionRiskScore=5, got %d", match.ContagionRiskScore)
	}
	if match.PlacementAuthorizationCode != "FOSTER-AUTH-FOSTER-MATCH-102" {
		t.Errorf("expected FOSTER-AUTH-FOSTER-MATCH-102, got %s", match.PlacementAuthorizationCode)
	}
}

func TestEvaluateFosterPlacement_ValidationErrors(t *testing.T) {
	now := time.Now().UTC()

	if _, err := fosternetwork.EvaluateFosterPlacement(fosternetwork.CreateFosterPlacementInput{
		PlacementID: "",
		PetID:       "pet-1",
		PetName:     "Buddy",
	}, now); err == nil {
		t.Errorf("expected error for missing placementId")
	}

	if _, err := fosternetwork.EvaluateFosterPlacement(fosternetwork.CreateFosterPlacementInput{
		PlacementID:    "foster-1",
		PetID:          "pet-1",
		PetName:        "Buddy",
		FosterHostName: "",
	}, now); err == nil {
		t.Errorf("expected error for missing fosterHostName")
	}

	if err := fosternetwork.ApplyFosterPlacementTransition(nil, fosternetwork.FosterPlacementUpdateInput{}, now); err == nil {
		t.Errorf("expected error for nil placement")
	}
}
