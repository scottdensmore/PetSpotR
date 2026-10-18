package domain_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func validSampleSwiftwaterOp() domain.SwiftwaterRescueOp {
	return domain.SwiftwaterRescueOp{
		OpID:                  "sw-op-101",
		PetID:                 "pet-2026-001",
		PetName:               "Yukon",
		Species:               "DOG",
		WaterwayName:          "Green Lake North Ice Shelf",
		HazardEnvironment:     "FRACTURED_LAKE_ICE_SHELF",
		WaterTempFahrenheit:   34,
		CurrentVelocityKnots:  1.5,
		HypothermiaRiskScore:  80,
		HydroRisk:             domain.HydroRiskExtremeIceHypothermia,
		ExtractionStatus:      domain.SwiftwaterStatusStrandedInCurrent,
		ColdShockAlert:        true,
		PFDThrowBagDeployed:   false,
		WarmingBlanketReady:   false,
		RiggingReadinessScore: 0,
		ProtocolSteps: []domain.SwiftwaterProtocolStep{
			{
				StepID:    "sw-step-downstream-spotter",
				Title:     "Station downstream safety spotter with floating 75ft spectra throw-bag line",
				Category:  "NFPA_1670_ROPE_RIGGING",
				Completed: false,
			},
		},
		CreatedAt: "2026-10-18T12:00:00Z",
		UpdatedAt: "2026-10-18T12:00:00Z",
	}
}

func TestSwiftwaterRescueOp_Validate(t *testing.T) {
	base := validSampleSwiftwaterOp()
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid sample swiftwater op, got error: %v", err)
	}

	if !domain.SwiftwaterStatusStrandedInCurrent.IsValid() ||
		!domain.SwiftwaterStatusThrowBagRigged.IsValid() ||
		!domain.SwiftwaterStatusExtractedWarming.IsValid() {
		t.Errorf("expected canonical SwiftwaterExtractionStatus values to be valid")
	}
	if domain.SwiftwaterExtractionStatus("UNKNOWN").IsValid() {
		t.Errorf("expected unknown SwiftwaterExtractionStatus to be invalid")
	}

	if !domain.HydroRiskExtremeIceHypothermia.IsValid() ||
		!domain.HydroRiskElevatedFloodWashout.IsValid() ||
		!domain.HydroRiskShorelineStabilized.IsValid() {
		t.Errorf("expected canonical HypothermiaHydroRisk values to be valid")
	}
	if domain.HypothermiaHydroRisk("UNKNOWN").IsValid() {
		t.Errorf("expected unknown HypothermiaHydroRisk to be invalid")
	}

	tests := []struct {
		name   string
		mutate func(*domain.SwiftwaterRescueOp)
	}{
		{
			name:   "empty opId",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.OpID = "" },
		},
		{
			name:   "empty petId",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.PetID = "" },
		},
		{
			name:   "empty petName",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.PetName = "" },
		},
		{
			name:   "invalid species",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.Species = "BIRD" },
		},
		{
			name:   "empty waterwayName",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.WaterwayName = "" },
		},
		{
			name:   "invalid hazardEnvironment",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.HazardEnvironment = "UNKNOWN_ENV" },
		},
		{
			name:   "waterTempFahrenheit too low",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.WaterTempFahrenheit = 27 },
		},
		{
			name:   "waterTempFahrenheit too high",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.WaterTempFahrenheit = 86 },
		},
		{
			name:   "currentVelocityKnots negative",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.CurrentVelocityKnots = -0.1 },
		},
		{
			name:   "currentVelocityKnots too high",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.CurrentVelocityKnots = 25.5 },
		},
		{
			name:   "hypothermiaRiskScore out of range",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.HypothermiaRiskScore = 105 },
		},
		{
			name:   "invalid hydroRisk",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.HydroRisk = "BAD_RISK" },
		},
		{
			name:   "invalid extractionStatus",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.ExtractionStatus = "BAD_STATUS" },
		},
		{
			name:   "riggingReadinessScore out of range",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.RiggingReadinessScore = -5 },
		},
		{
			name:   "empty protocolSteps",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.ProtocolSteps = nil },
		},
		{
			name: "invalid step category",
			mutate: func(o *domain.SwiftwaterRescueOp) {
				o.ProtocolSteps[0].Category = "BAD_CAT"
			},
		},
		{
			name: "extracted warming without clearance code",
			mutate: func(o *domain.SwiftwaterRescueOp) {
				o.ExtractionStatus = domain.SwiftwaterStatusExtractedWarming
				o.ExtractionClearanceCode = ""
			},
		},
		{
			name:   "empty createdAt",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.CreatedAt = "" },
		},
		{
			name:   "empty updatedAt",
			mutate: func(o *domain.SwiftwaterRescueOp) { o.UpdatedAt = "" },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validSampleSwiftwaterOp()
			tc.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}
