package swiftwater

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreateSwiftwaterOpInput struct {
	OpID                 string
	PetID                string
	PetName              string
	Species              string
	WaterwayName         string
	HazardEnvironment    string
	WaterTempFahrenheit  int
	CurrentVelocityKnots float64
	PFDThrowBagDeployed  bool
	WarmingBlanketReady  bool
}

type SwiftwaterOpUpdateInput struct {
	DeployRiggingAndFlotation   bool
	CompleteShorelineExtraction bool
	CompleteStepID              string
	PFDThrowBagDeployed         *bool
	WarmingBlanketReady         *bool
}

func EvaluateSwiftwaterRescueOp(input CreateSwiftwaterOpInput, now time.Time) (*domain.SwiftwaterRescueOp, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	opID := strings.TrimSpace(input.OpID)
	petID := strings.TrimSpace(input.PetID)
	petName := strings.TrimSpace(input.PetName)
	waterwayName := strings.TrimSpace(input.WaterwayName)

	if opID == "" {
		return nil, fmt.Errorf("opId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return nil, fmt.Errorf("petName is required")
	}
	if waterwayName == "" {
		return nil, fmt.Errorf("waterwayName is required")
	}
	if input.WaterTempFahrenheit < 28 || input.WaterTempFahrenheit > 85 {
		return nil, fmt.Errorf("waterTempFahrenheit must be in [28, 85], got %d", input.WaterTempFahrenheit)
	}
	if input.CurrentVelocityKnots < 0.0 || input.CurrentVelocityKnots > 25.0 {
		return nil, fmt.Errorf("currentVelocityKnots must be in [0.0, 25.0], got %v", input.CurrentVelocityKnots)
	}

	species := strings.ToUpper(strings.TrimSpace(input.Species))
	if species == "" {
		species = "DOG"
	}

	hazardEnvironment := strings.ToUpper(strings.TrimSpace(input.HazardEnvironment))
	if hazardEnvironment == "" {
		hazardEnvironment = "FRACTURED_LAKE_ICE_SHELF"
	}

	steps := []domain.SwiftwaterProtocolStep{
		{
			StepID:    "sw-step-downstream-spotter",
			Title:     "Station downstream safety spotter with floating 75ft spectra throw-bag line",
			Category:  "NFPA_1670_ROPE_RIGGING",
			Completed: false,
		},
		{
			StepID:    "sw-step-inflatable-sled",
			Title:     "Deploy low-PSI inflatable ice/flood rescue sled & canine dorsal-lift flotation harness",
			Category:  "ICE_SHELF_FLOTATION",
			Completed: false,
		},
		{
			StepID:    "sw-step-horizontal-haul",
			Title:     "Execute horizontal low-stress extraction to prevent circum-rescue cardiovascular collapse",
			Category:  "ICE_SHELF_FLOTATION",
			Completed: false,
		},
		{
			StepID:    "sw-step-bair-hugger",
			Title:     "Initiate forced-air Bair Hugger active core rewarming & warm IV dextrose check",
			Category:  "ACTIVE_CORE_REWARMING",
			Completed: false,
		},
	}

	op := &domain.SwiftwaterRescueOp{
		OpID:                 opID,
		PetID:                petID,
		PetName:              petName,
		Species:              species,
		WaterwayName:         waterwayName,
		HazardEnvironment:    hazardEnvironment,
		WaterTempFahrenheit:  input.WaterTempFahrenheit,
		CurrentVelocityKnots: input.CurrentVelocityKnots,
		PFDThrowBagDeployed:  input.PFDThrowBagDeployed,
		WarmingBlanketReady:  input.WarmingBlanketReady,
		ProtocolSteps:        steps,
		CreatedAt:            nowStr,
		UpdatedAt:            nowStr,
	}

	RecomputeSwiftwaterMetrics(op, now)

	if err := op.Validate(); err != nil {
		return nil, err
	}

	return op, nil
}

func RecomputeSwiftwaterMetrics(op *domain.SwiftwaterRescueOp, now time.Time) {
	if op == nil {
		return
	}
	_ = now

	op.ColdShockAlert = op.WaterTempFahrenheit <= 48 ||
		op.HazardEnvironment == "FRACTURED_LAKE_ICE_SHELF" ||
		op.CurrentVelocityKnots >= 6.0

	readiness := 0
	if op.PFDThrowBagDeployed {
		readiness += 50
	}
	if op.WarmingBlanketReady {
		readiness += 50
	}
	op.RiggingReadinessScore = readiness

	totalSteps := len(op.ProtocolSteps)
	completedSteps := 0
	for _, step := range op.ProtocolSteps {
		if step.Completed {
			completedSteps++
		}
	}

	baseRisk := 35
	switch op.HazardEnvironment {
	case "FRACTURED_LAKE_ICE_SHELF":
		baseRisk = 55
	case "FLASH_FLOOD_STORM_CHANNEL":
		baseRisk = 50
	case "TIDAL_MUDFLAT_ESTUARY":
		baseRisk = 35
	}

	tempPenalty := 0
	if op.WaterTempFahrenheit <= 40 {
		tempPenalty = 25
	} else if op.WaterTempFahrenheit <= 50 {
		tempPenalty = 15
	}

	currentPenalty := 0
	if op.CurrentVelocityKnots >= 6.0 {
		currentPenalty = 20
	} else if op.CurrentVelocityKnots >= 3.0 {
		currentPenalty = 10
	}

	risk := baseRisk + tempPenalty + currentPenalty - (completedSteps * 15) - (readiness / 4)
	if risk < 5 {
		risk = 5
	}
	if risk > 100 {
		risk = 100
	}
	op.HypothermiaRiskScore = risk

	if totalSteps > 0 && completedSteps == totalSteps && op.PFDThrowBagDeployed && op.WarmingBlanketReady {
		op.ExtractionStatus = domain.SwiftwaterStatusExtractedWarming
		op.HydroRisk = domain.HydroRiskShorelineStabilized
		op.HypothermiaRiskScore = 5
		if op.ExtractionClearanceCode == "" {
			op.ExtractionClearanceCode = "SWIFTWATER-SAFE-" + strings.ToUpper(op.OpID)
		}
	} else if completedSteps >= 1 || (op.PFDThrowBagDeployed && op.WarmingBlanketReady) {
		op.ExtractionStatus = domain.SwiftwaterStatusThrowBagRigged
		if op.HypothermiaRiskScore >= 70 {
			op.HydroRisk = domain.HydroRiskExtremeIceHypothermia
		} else {
			op.HydroRisk = domain.HydroRiskElevatedFloodWashout
		}
	} else {
		op.ExtractionStatus = domain.SwiftwaterStatusStrandedInCurrent
		if op.ColdShockAlert || op.HypothermiaRiskScore >= 65 {
			op.HydroRisk = domain.HydroRiskExtremeIceHypothermia
		} else {
			op.HydroRisk = domain.HydroRiskElevatedFloodWashout
		}
	}
}

func ApplySwiftwaterOpUpdate(op *domain.SwiftwaterRescueOp, input SwiftwaterOpUpdateInput, now time.Time) error {
	if op == nil {
		return fmt.Errorf("swiftwater rescue op is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	if input.PFDThrowBagDeployed != nil {
		op.PFDThrowBagDeployed = *input.PFDThrowBagDeployed
	}
	if input.WarmingBlanketReady != nil {
		op.WarmingBlanketReady = *input.WarmingBlanketReady
	}

	if stepID := strings.TrimSpace(input.CompleteStepID); stepID != "" {
		found := false
		for i := range op.ProtocolSteps {
			if op.ProtocolSteps[i].StepID == stepID {
				op.ProtocolSteps[i].Completed = true
				if op.ProtocolSteps[i].CompletedAt == "" {
					op.ProtocolSteps[i].CompletedAt = nowStr
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("protocol step %q not found", stepID)
		}
	}

	if input.DeployRiggingAndFlotation {
		for i := range op.ProtocolSteps {
			if i < 2 {
				op.ProtocolSteps[i].Completed = true
				if op.ProtocolSteps[i].CompletedAt == "" {
					op.ProtocolSteps[i].CompletedAt = nowStr
				}
			}
		}
		op.PFDThrowBagDeployed = true
		op.WarmingBlanketReady = true
	}

	if input.CompleteShorelineExtraction {
		for i := range op.ProtocolSteps {
			op.ProtocolSteps[i].Completed = true
			if op.ProtocolSteps[i].CompletedAt == "" {
				op.ProtocolSteps[i].CompletedAt = nowStr
			}
		}
		op.PFDThrowBagDeployed = true
		op.WarmingBlanketReady = true
	}

	RecomputeSwiftwaterMetrics(op, now)
	op.UpdatedAt = nowStr

	return op.Validate()
}
