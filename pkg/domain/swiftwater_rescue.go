package domain

import (
	"fmt"
	"strings"
)

type SwiftwaterExtractionStatus string

const (
	SwiftwaterStatusStrandedInCurrent SwiftwaterExtractionStatus = "STRANDED_IN_SWIFTWATER"
	SwiftwaterStatusThrowBagRigged    SwiftwaterExtractionStatus = "THROW_BAG_RIGGED_APPROACH"
	SwiftwaterStatusExtractedWarming  SwiftwaterExtractionStatus = "EXTRACTED_WARMING_STABLE"
)

func (s SwiftwaterExtractionStatus) IsValid() bool {
	switch s {
	case SwiftwaterStatusStrandedInCurrent,
		SwiftwaterStatusThrowBagRigged,
		SwiftwaterStatusExtractedWarming:
		return true
	default:
		return false
	}
}

type HypothermiaHydroRisk string

const (
	HydroRiskExtremeIceHypothermia HypothermiaHydroRisk = "EXTREME_ICE_HYPOTHERMIA_RISK"
	HydroRiskElevatedFloodWashout  HypothermiaHydroRisk = "ELEVATED_FLOOD_CURRENT_RISK"
	HydroRiskShorelineStabilized   HypothermiaHydroRisk = "SHORELINE_REWARMED_SAFE"
)

func (r HypothermiaHydroRisk) IsValid() bool {
	switch r {
	case HydroRiskExtremeIceHypothermia,
		HydroRiskElevatedFloodWashout,
		HydroRiskShorelineStabilized:
		return true
	default:
		return false
	}
}

type SwiftwaterProtocolStep struct {
	StepID      string `json:"stepId"`
	Title       string `json:"title"`
	Category    string `json:"category"` // "NFPA_1670_ROPE_RIGGING", "ICE_SHELF_FLOTATION", "ACTIVE_CORE_REWARMING"
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type SwiftwaterRescueOp struct {
	OpID                    string                     `json:"opId"`
	PetID                   string                     `json:"petId"`
	PetName                 string                     `json:"petName"`
	Species                 string                     `json:"species"` // "DOG" or "CAT"
	WaterwayName            string                     `json:"waterwayName"`
	HazardEnvironment       string                     `json:"hazardEnvironment"`    // "FRACTURED_LAKE_ICE_SHELF", "FLASH_FLOOD_STORM_CHANNEL", "TIDAL_MUDFLAT_ESTUARY"
	WaterTempFahrenheit     int                        `json:"waterTempFahrenheit"`  // 28 to 85
	CurrentVelocityKnots    float64                    `json:"currentVelocityKnots"` // 0.0 to 25.0
	HypothermiaRiskScore    int                        `json:"hypothermiaRiskScore"` // 0 to 100
	HydroRisk               HypothermiaHydroRisk       `json:"hydroRisk"`
	ExtractionStatus        SwiftwaterExtractionStatus `json:"extractionStatus"`
	ColdShockAlert          bool                       `json:"coldShockAlert"`
	PFDThrowBagDeployed     bool                       `json:"pfdThrowBagDeployed"`
	WarmingBlanketReady     bool                       `json:"warmingBlanketReady"`
	RiggingReadinessScore   int                        `json:"riggingReadinessScore"` // 0 to 100
	ProtocolSteps           []SwiftwaterProtocolStep   `json:"protocolSteps"`
	ExtractionClearanceCode string                     `json:"extractionClearanceCode,omitempty"`
	CreatedAt               string                     `json:"createdAt"`
	UpdatedAt               string                     `json:"updatedAt"`
}

func validSwiftwaterHazardEnvironment(env string) bool {
	switch env {
	case "FRACTURED_LAKE_ICE_SHELF", "FLASH_FLOOD_STORM_CHANNEL", "TIDAL_MUDFLAT_ESTUARY":
		return true
	default:
		return false
	}
}

func validSwiftwaterStepCategory(cat string) bool {
	switch cat {
	case "NFPA_1670_ROPE_RIGGING", "ICE_SHELF_FLOTATION", "ACTIVE_CORE_REWARMING":
		return true
	default:
		return false
	}
}

// Validate checks invariants for SwiftwaterRescueOp.
func (op SwiftwaterRescueOp) Validate() error {
	if strings.TrimSpace(op.OpID) == "" {
		return fmt.Errorf("opId is required")
	}
	if strings.TrimSpace(op.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	species := strings.TrimSpace(op.Species)
	if species != "DOG" && species != "CAT" {
		return fmt.Errorf("species must be DOG or CAT, got %q", op.Species)
	}
	if strings.TrimSpace(op.WaterwayName) == "" {
		return fmt.Errorf("waterwayName is required")
	}
	if !validSwiftwaterHazardEnvironment(strings.TrimSpace(op.HazardEnvironment)) {
		return fmt.Errorf("invalid hazardEnvironment: %q", op.HazardEnvironment)
	}
	if op.WaterTempFahrenheit < 28 || op.WaterTempFahrenheit > 85 {
		return fmt.Errorf("waterTempFahrenheit must be in [28, 85], got %d", op.WaterTempFahrenheit)
	}
	if op.CurrentVelocityKnots < 0.0 || op.CurrentVelocityKnots > 25.0 {
		return fmt.Errorf("currentVelocityKnots must be in [0.0, 25.0], got %v", op.CurrentVelocityKnots)
	}
	if op.HypothermiaRiskScore < 0 || op.HypothermiaRiskScore > 100 {
		return fmt.Errorf("hypothermiaRiskScore must be in [0, 100], got %d", op.HypothermiaRiskScore)
	}
	if !op.HydroRisk.IsValid() {
		return fmt.Errorf("invalid hydroRisk: %q", op.HydroRisk)
	}
	if !op.ExtractionStatus.IsValid() {
		return fmt.Errorf("invalid extractionStatus: %q", op.ExtractionStatus)
	}
	if op.RiggingReadinessScore < 0 || op.RiggingReadinessScore > 100 {
		return fmt.Errorf("riggingReadinessScore must be in [0, 100], got %d", op.RiggingReadinessScore)
	}
	if len(op.ProtocolSteps) < 1 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.StepID) == "" ||
			strings.TrimSpace(step.Title) == "" ||
			!validSwiftwaterStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty stepId, title, and valid category", i)
		}
	}
	if op.ExtractionStatus == SwiftwaterStatusExtractedWarming && strings.TrimSpace(op.ExtractionClearanceCode) == "" {
		return fmt.Errorf("extractionClearanceCode is required when extractionStatus is EXTRACTED_WARMING_STABLE")
	}
	if strings.TrimSpace(op.CreatedAt) == "" {
		return fmt.Errorf("createdAt is required")
	}
	if strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
