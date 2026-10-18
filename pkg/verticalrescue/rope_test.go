package verticalrescue_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/verticalrescue"
)

func TestEvaluateVerticalRescue_FallShockAndCanopyFailureAlerts(t *testing.T) {
	op, err := verticalrescue.EvaluateVerticalRescue(domain.VerticalRescueEvaluationRequest{
		OperationID:          "VR-TEST-001",
		PetID:                "PET-VR-999",
		PetName:              "Juniper",
		Species:              "Feline",
		WeightKg:             4.2,
		EnvironmentType:      domain.VerticalEnvTreeCanopyCrown,
		ElevationMeters:      24.0,
		SlopeAngleDeg:        85.0,
		WindGustKnots:        29.0,
		HoursStranded:        30.0,
		BranchStabilityIndex: 20,
		LeadRiggerCallsign:   "ARBOR-9",
		Notes:                "Douglas fir deadwood crown in gale gusts",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.Status != domain.VerticalStatusFallShockCritical {
		t.Errorf("expected FALL_SHOCK_HAZARD_CRITICAL, got %s", op.Status)
	}
	if !op.HighRiseFallShockAlert {
		t.Errorf("expected HighRiseFallShockAlert=true")
	}
	if !op.CanopyFailureAlert {
		t.Errorf("expected CanopyFailureAlert=true")
	}
	if len(op.ProtocolSteps) != 5 {
		t.Errorf("expected 5 protocol steps, got %d", len(op.ProtocolSteps))
	}

	hasFallFlag := false
	hasCanopyFlag := false
	hasFelineFlag := false
	for _, f := range op.RiskFlags {
		switch f {
		case "EXTREME_VERTICAL_FALL_TRAUMA_RISK":
			hasFallFlag = true
		case "CRITICAL_DEADWOOD_WIND_SHEAR_HAZARD":
			hasCanopyFlag = true
		case "FELINE_PANIC_LEAP_SYNDROME":
			hasFelineFlag = true
		}
	}
	if !hasFallFlag || !hasCanopyFlag || !hasFelineFlag {
		t.Errorf("missing expected risk flags, got %v", op.RiskFlags)
	}
}

func TestEvaluateVerticalRescue_AllEnvironmentsAndLowHazard(t *testing.T) {
	envs := []domain.VerticalEnvironmentType{
		domain.VerticalEnvTreeCanopyCrown,
		domain.VerticalEnvCliffsideRockLedge,
		domain.VerticalEnvHighRiseBalconyParapet,
		domain.VerticalEnvBridgeOverpassGirder,
		domain.VerticalEnvUtilityTowerCrossarm,
	}

	for _, env := range envs {
		op, err := verticalrescue.EvaluateVerticalRescue(domain.VerticalRescueEvaluationRequest{
			PetID:                "PET-VR-LOW",
			PetName:              "Bramble",
			Species:              "Canine",
			WeightKg:             18.5,
			EnvironmentType:      env,
			ElevationMeters:      6.0,
			SlopeAngleDeg:        45.0,
			WindGustKnots:        5.0,
			HoursStranded:        2.0,
			BranchStabilityIndex: 90,
		})
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", env, err)
		}
		if op.HighRiseFallShockAlert {
			t.Errorf("expected HighRiseFallShockAlert=false for low elevation/slope/hours")
		}
		if op.CanopyFailureAlert {
			t.Errorf("expected CanopyFailureAlert=false for high stability/low wind")
		}
		if op.Status != domain.VerticalStatusTwinTensionRigged {
			t.Errorf("expected TWIN_TENSION_RIGGED_ASCENT for moderate hazard, got %s", op.Status)
		}
		if op.RecommendedRopeSystem == "" || op.RecommendedCaptureBag == "" || op.DropZoneGrid == "" {
			t.Errorf("expected equipment recommendations to be populated")
		}
	}
}

func TestUpdateProtocolStepAndIssueVerticalClearance(t *testing.T) {
	store := verticalrescue.NewStore()
	ops := store.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded ops, got %d", len(ops))
	}

	// Verify seeded VR-OP-101, VR-OP-102, VR-OP-103
	op101, ok := store.Get("VR-OP-101")
	if !ok || op101.PetName != "Cleo" || op101.Status != domain.VerticalStatusFallShockCritical || op101.RiggingReadinessScore != 40 {
		t.Fatalf("unexpected VR-OP-101 seed state: %+v", op101)
	}
	op102, ok := store.Get("VR-OP-102")
	if !ok || op102.PetName != "Koda" || op102.Status != domain.VerticalStatusTwinTensionRigged || op102.RiggingReadinessScore != 100 {
		t.Fatalf("unexpected VR-OP-102 seed state: %+v", op102)
	}
	op103, ok := store.Get("VR-OP-103")
	if !ok || op103.PetName != "Mochi" || op103.Status != domain.VerticalStatusLoweredGroundSafe || op103.ClearanceCertificateHash != "VERTICAL-SAFE-5E8A29C1D407" {
		t.Fatalf("unexpected VR-OP-103 seed state: %+v", op103)
	}

	// Issuing clearance on VR-OP-101 before all steps are completed must fail
	if _, err := store.IssueClearance("VR-OP-101"); err == nil {
		t.Errorf("expected error issuing clearance on incomplete VR-OP-101")
	}

	// Complete remaining 3 steps on VR-OP-101
	for _, step := range op101.ProtocolSteps {
		if !step.Completed {
			updated, err := store.UpdateStep("VR-OP-101", domain.VerticalStepUpdateRequest{
				StepID:    step.ID,
				Completed: true,
			})
			if err != nil {
				t.Fatalf("unexpected error completing step %s: %v", step.ID, err)
			}
			op101 = updated
		}
	}

	if op101.RiggingReadinessScore != 100 {
		t.Errorf("expected RiggingReadinessScore=100, got %d", op101.RiggingReadinessScore)
	}
	if op101.Status != domain.VerticalStatusTwinTensionRigged {
		t.Errorf("expected TWIN_TENSION_RIGGED_ASCENT after all steps completed, got %s", op101.Status)
	}

	// Toggle a step back to false and then true again
	uncheck, err := store.UpdateStep("VR-OP-101", domain.VerticalStepUpdateRequest{
		StepID:    "vr-step-controlled-lower",
		Completed: false,
	})
	if err != nil || uncheck.RiggingReadinessScore != 80 {
		t.Fatalf("expected RiggingReadinessScore=80 after unchecking step, got %d (err=%v)", uncheck.RiggingReadinessScore, err)
	}
	if _, err := store.UpdateStep("VR-OP-101", domain.VerticalStepUpdateRequest{
		StepID:    "vr-step-controlled-lower",
		Completed: true,
	}); err != nil {
		t.Fatalf("unexpected error re-checking step: %v", err)
	}

	// Issue clearance on VR-OP-101
	cleared, err := store.IssueClearance("VR-OP-101")
	if err != nil {
		t.Fatalf("unexpected error issuing clearance: %v", err)
	}
	if cleared.Status != domain.VerticalStatusLoweredGroundSafe {
		t.Errorf("expected LOWERED_GROUND_SAFE, got %s", cleared.Status)
	}
	if cleared.HazardScore != 10 {
		t.Errorf("expected HazardScore=10, got %d", cleared.HazardScore)
	}
	if cleared.HighRiseFallShockAlert || cleared.CanopyFailureAlert {
		t.Errorf("expected alerts cleared on LOWERED_GROUND_SAFE")
	}
	if !strings.HasPrefix(cleared.ClearanceCertificateHash, "VERTICAL-SAFE-") || len(cleared.ClearanceCertificateHash) != len("VERTICAL-SAFE-")+12 {
		t.Errorf("expected VERTICAL-SAFE-<12HEX>, got %q", cleared.ClearanceCertificateHash)
	}
}

func TestValidationAndErrorPaths(t *testing.T) {
	store := verticalrescue.NewStore()

	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "", PetName: "Cleo"}); err == nil {
		t.Errorf("expected error for empty petId")
	}
	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "P1", PetName: ""}); err == nil {
		t.Errorf("expected error for empty petName")
	}
	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "P1", PetName: "Cleo", WeightKg: 500}); err == nil {
		t.Errorf("expected error for excessive weightKg")
	}
	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "P1", PetName: "Cleo", EnvironmentType: "INVALID"}); err == nil {
		t.Errorf("expected error for invalid environmentType")
	}
	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "P1", PetName: "Cleo", ElevationMeters: 0.2}); err == nil {
		t.Errorf("expected error for low elevationMeters")
	}
	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "P1", PetName: "Cleo", ElevationMeters: 15, SlopeAngleDeg: 110}); err == nil {
		t.Errorf("expected error for invalid slopeAngleDeg")
	}
	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "P1", PetName: "Cleo", ElevationMeters: 15, WindGustKnots: -5}); err == nil {
		t.Errorf("expected error for negative windGustKnots")
	}
	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "P1", PetName: "Cleo", ElevationMeters: 15, HoursStranded: -1}); err == nil {
		t.Errorf("expected error for negative hoursStranded")
	}
	if _, err := store.Evaluate(domain.VerticalRescueEvaluationRequest{PetID: "P1", PetName: "Cleo", ElevationMeters: 15, BranchStabilityIndex: 120}); err == nil {
		t.Errorf("expected error for invalid branchStabilityIndex")
	}

	if _, err := store.UpdateStep("NON-EXISTENT", domain.VerticalStepUpdateRequest{StepID: "vr-step-dropzone-net", Completed: true}); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
	if _, err := store.UpdateStep("VR-OP-101", domain.VerticalStepUpdateRequest{StepID: "", Completed: true}); err == nil {
		t.Errorf("expected error for empty step ID")
	}
	if _, err := store.UpdateStep("VR-OP-101", domain.VerticalStepUpdateRequest{StepID: "invalid-step", Completed: true}); err == nil {
		t.Errorf("expected error for unknown step ID")
	}
	if _, err := store.IssueClearance("NON-EXISTENT"); err == nil {
		t.Errorf("expected error issuing clearance on unknown operation ID")
	}
}
