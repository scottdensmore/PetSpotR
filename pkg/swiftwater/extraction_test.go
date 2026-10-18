package swiftwater_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/swiftwater"
)

func TestEvaluateSwiftwaterRescueOp_IceShelfBreakthrough(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	op, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 "sw-op-101",
		PetID:                "pet-2026-001",
		PetName:              "Yukon",
		Species:              "DOG",
		WaterwayName:         "Green Lake North Ice Shelf",
		HazardEnvironment:    "FRACTURED_LAKE_ICE_SHELF",
		WaterTempFahrenheit:  34,
		CurrentVelocityKnots: 1.5,
		PFDThrowBagDeployed:  false,
		WarmingBlanketReady:  false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating swiftwater op: %v", err)
	}

	if op.ExtractionStatus != domain.SwiftwaterStatusStrandedInCurrent {
		t.Errorf("expected STRANDED_IN_SWIFTWATER, got %s", op.ExtractionStatus)
	}
	if op.HydroRisk != domain.HydroRiskExtremeIceHypothermia {
		t.Errorf("expected EXTREME_ICE_HYPOTHERMIA_RISK, got %s", op.HydroRisk)
	}
	if !op.ColdShockAlert {
		t.Errorf("expected ColdShockAlert=true for 34F ice shelf")
	}
	if op.HypothermiaRiskScore != 80 {
		t.Errorf("expected HypothermiaRiskScore=80, got %d", op.HypothermiaRiskScore)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Errorf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
}

func TestApplySwiftwaterOpUpdate_DeployRiggingAndCompleteExtraction(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	op, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 "sw-op-102",
		PetID:                "pet-2026-002",
		PetName:              "Misty",
		Species:              "CAT",
		WaterwayName:         "Thornton Creek Flood Weir",
		HazardEnvironment:    "FLASH_FLOOD_STORM_CHANNEL",
		WaterTempFahrenheit:  47,
		CurrentVelocityKnots: 5.5,
		PFDThrowBagDeployed:  false,
		WarmingBlanketReady:  false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating swiftwater op: %v", err)
	}

	// 1. DeployRiggingAndFlotation -> THROW_BAG_RIGGED_APPROACH
	if err := swiftwater.ApplySwiftwaterOpUpdate(op, swiftwater.SwiftwaterOpUpdateInput{
		DeployRiggingAndFlotation: true,
	}, now.Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error deploying rigging: %v", err)
	}

	if op.ExtractionStatus != domain.SwiftwaterStatusThrowBagRigged {
		t.Errorf("expected THROW_BAG_RIGGED_APPROACH, got %s", op.ExtractionStatus)
	}
	if op.HydroRisk != domain.HydroRiskElevatedFloodWashout {
		t.Errorf("expected ELEVATED_FLOOD_CURRENT_RISK, got %s", op.HydroRisk)
	}
	if op.RiggingReadinessScore != 100 {
		t.Errorf("expected RiggingReadinessScore=100, got %d", op.RiggingReadinessScore)
	}

	// 2. Complete individual step
	if err := swiftwater.ApplySwiftwaterOpUpdate(op, swiftwater.SwiftwaterOpUpdateInput{
		CompleteStepID: "sw-step-horizontal-haul",
	}, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing step: %v", err)
	}

	// 3. Unknown step ID returns error
	if err := swiftwater.ApplySwiftwaterOpUpdate(op, swiftwater.SwiftwaterOpUpdateInput{
		CompleteStepID: "unknown-step",
	}, now.Add(3*time.Minute)); err == nil {
		t.Errorf("expected error for unknown step ID")
	}

	// 4. CompleteShorelineExtraction -> EXTRACTED_WARMING_STABLE
	if err := swiftwater.ApplySwiftwaterOpUpdate(op, swiftwater.SwiftwaterOpUpdateInput{
		CompleteShorelineExtraction: true,
	}, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing shoreline extraction: %v", err)
	}

	if op.ExtractionStatus != domain.SwiftwaterStatusExtractedWarming {
		t.Errorf("expected EXTRACTED_WARMING_STABLE, got %s", op.ExtractionStatus)
	}
	if op.HydroRisk != domain.HydroRiskShorelineStabilized {
		t.Errorf("expected SHORELINE_REWARMED_SAFE, got %s", op.HydroRisk)
	}
	if op.HypothermiaRiskScore != 5 {
		t.Errorf("expected HypothermiaRiskScore=5, got %d", op.HypothermiaRiskScore)
	}
	if op.ExtractionClearanceCode != "SWIFTWATER-SAFE-SW-OP-102" {
		t.Errorf("expected SWIFTWATER-SAFE-SW-OP-102, got %s", op.ExtractionClearanceCode)
	}
}

func TestEvaluateSwiftwaterRescueOp_ValidationErrors(t *testing.T) {
	now := time.Now().UTC()

	if _, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 "",
		PetID:                "pet-1",
		PetName:              "Yukon",
		WaterwayName:         "Green Lake",
		WaterTempFahrenheit:  40,
		CurrentVelocityKnots: 2.0,
	}, now); err == nil {
		t.Errorf("expected error for missing opId")
	}

	if _, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 "sw-1",
		PetID:                "pet-1",
		PetName:              "Yukon",
		WaterwayName:         "Green Lake",
		WaterTempFahrenheit:  10,
		CurrentVelocityKnots: 2.0,
	}, now); err == nil {
		t.Errorf("expected error for out-of-range waterTempFahrenheit")
	}

	if _, err := swiftwater.EvaluateSwiftwaterRescueOp(swiftwater.CreateSwiftwaterOpInput{
		OpID:                 "sw-1",
		PetID:                "pet-1",
		PetName:              "Yukon",
		WaterwayName:         "Green Lake",
		WaterTempFahrenheit:  40,
		CurrentVelocityKnots: 30.0,
	}, now); err == nil {
		t.Errorf("expected error for out-of-range currentVelocityKnots")
	}

	if err := swiftwater.ApplySwiftwaterOpUpdate(nil, swiftwater.SwiftwaterOpUpdateInput{}, now); err == nil {
		t.Errorf("expected error for nil swiftwater op")
	}
}
