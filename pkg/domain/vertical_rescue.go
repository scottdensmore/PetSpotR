package domain

import (
	"fmt"
	"strings"
)

type VerticalEnvironmentType string

const (
	VerticalEnvTreeCanopyCrown        VerticalEnvironmentType = "TREE_CANOPY_CROWN"
	VerticalEnvCliffsideRockLedge     VerticalEnvironmentType = "CLIFFSIDE_ROCK_LEDGE"
	VerticalEnvHighRiseBalconyParapet VerticalEnvironmentType = "HIGH_RISE_BALCONY_PARAPET"
	VerticalEnvBridgeOverpassGirder   VerticalEnvironmentType = "BRIDGE_OVERPASS_GIRDER"
	VerticalEnvUtilityTowerCrossarm   VerticalEnvironmentType = "UTILITY_TOWER_CROSSARM"
)

func (e VerticalEnvironmentType) IsValid() bool {
	switch e {
	case VerticalEnvTreeCanopyCrown,
		VerticalEnvCliffsideRockLedge,
		VerticalEnvHighRiseBalconyParapet,
		VerticalEnvBridgeOverpassGirder,
		VerticalEnvUtilityTowerCrossarm:
		return true
	default:
		return false
	}
}

type VerticalRescueStatus string

const (
	VerticalStatusFallShockCritical VerticalRescueStatus = "FALL_SHOCK_HAZARD_CRITICAL"
	VerticalStatusTwinTensionRigged VerticalRescueStatus = "TWIN_TENSION_RIGGED_ASCENT"
	VerticalStatusLoweredGroundSafe VerticalRescueStatus = "LOWERED_GROUND_SAFE"
)

func (s VerticalRescueStatus) IsValid() bool {
	switch s {
	case VerticalStatusFallShockCritical,
		VerticalStatusTwinTensionRigged,
		VerticalStatusLoweredGroundSafe:
		return true
	default:
		return false
	}
}

type VerticalProtocolStep struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Category    string `json:"category"` // "DROP_ZONE_NETTING", "TWIN_TENSION_ANCHOR", "CALMING_HARNESS_BAG"
	Required    bool   `json:"required"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type VerticalRescueOperation struct {
	OperationID              string                  `json:"operationId"`
	PetID                    string                  `json:"petId"`
	PetName                  string                  `json:"petName"`
	Species                  string                  `json:"species"`
	WeightKg                 float64                 `json:"weightKg"`
	EnvironmentType          VerticalEnvironmentType `json:"environmentType"`
	Status                   VerticalRescueStatus    `json:"status"`
	ElevationMeters          float64                 `json:"elevationMeters"`
	SlopeAngleDeg            float64                 `json:"slopeAngleDeg"`
	WindGustKnots            float64                 `json:"windGustKnots"`
	HoursStranded            float64                 `json:"hoursStranded"`
	BranchStabilityIndex     int                     `json:"branchStabilityIndex"`
	HazardScore              int                     `json:"hazardScore"`
	RiggingReadinessScore    int                     `json:"riggingReadinessScore"`
	HighRiseFallShockAlert   bool                    `json:"highRiseFallShockAlert"`
	CanopyFailureAlert       bool                    `json:"canopyFailureAlert"`
	RecommendedRopeSystem    string                  `json:"recommendedRopeSystem"`
	RecommendedCaptureBag    string                  `json:"recommendedCaptureBag"`
	DropZoneGrid             string                  `json:"dropZoneGrid"`
	RiskFlags                []string                `json:"riskFlags"`
	ProtocolSteps            []VerticalProtocolStep  `json:"protocolSteps"`
	ClearanceCertificateHash string                  `json:"clearanceCertificateHash,omitempty"`
	LeadRiggerCallsign       string                  `json:"leadRiggerCallsign"`
	Notes                    string                  `json:"notes"`
	UpdatedAt                string                  `json:"updatedAt"`
}

type VerticalRescueEvaluationRequest struct {
	OperationID          string                  `json:"operationId,omitempty"`
	PetID                string                  `json:"petId"`
	PetName              string                  `json:"petName"`
	Species              string                  `json:"species"`
	WeightKg             float64                 `json:"weightKg"`
	EnvironmentType      VerticalEnvironmentType `json:"environmentType"`
	ElevationMeters      float64                 `json:"elevationMeters"`
	SlopeAngleDeg        float64                 `json:"slopeAngleDeg"`
	WindGustKnots        float64                 `json:"windGustKnots"`
	HoursStranded        float64                 `json:"hoursStranded"`
	BranchStabilityIndex int                     `json:"branchStabilityIndex"`
	LeadRiggerCallsign   string                  `json:"leadRiggerCallsign"`
	Notes                string                  `json:"notes"`
}

type VerticalStepUpdateRequest struct {
	StepID    string `json:"stepId"`
	Completed bool   `json:"completed"`
}

func validVerticalStepCategory(cat string) bool {
	switch cat {
	case "DROP_ZONE_NETTING", "TWIN_TENSION_ANCHOR", "CALMING_HARNESS_BAG":
		return true
	default:
		return false
	}
}

func (op VerticalRescueOperation) Validate() error {
	if strings.TrimSpace(op.OperationID) == "" {
		return fmt.Errorf("operationId is required")
	}
	if strings.TrimSpace(op.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	if strings.TrimSpace(op.Species) == "" {
		return fmt.Errorf("species is required")
	}
	if op.WeightKg <= 0 || op.WeightKg > 250 {
		return fmt.Errorf("weightKg must be in (0, 250], got %v", op.WeightKg)
	}
	if !op.EnvironmentType.IsValid() {
		return fmt.Errorf("invalid environmentType: %q", op.EnvironmentType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.ElevationMeters < 1.0 || op.ElevationMeters > 500.0 {
		return fmt.Errorf("elevationMeters must be in [1.0, 500.0], got %v", op.ElevationMeters)
	}
	if op.SlopeAngleDeg < 0.0 || op.SlopeAngleDeg > 90.0 {
		return fmt.Errorf("slopeAngleDeg must be in [0.0, 90.0], got %v", op.SlopeAngleDeg)
	}
	if op.WindGustKnots < 0.0 || op.WindGustKnots > 120.0 {
		return fmt.Errorf("windGustKnots must be in [0.0, 120.0], got %v", op.WindGustKnots)
	}
	if op.HoursStranded < 0.0 || op.HoursStranded > 720.0 {
		return fmt.Errorf("hoursStranded must be in [0.0, 720.0], got %v", op.HoursStranded)
	}
	if op.BranchStabilityIndex < 0 || op.BranchStabilityIndex > 100 {
		return fmt.Errorf("branchStabilityIndex must be in [0, 100], got %d", op.BranchStabilityIndex)
	}
	if op.HazardScore < 0 || op.HazardScore > 100 {
		return fmt.Errorf("hazardScore must be in [0, 100], got %d", op.HazardScore)
	}
	if op.RiggingReadinessScore < 0 || op.RiggingReadinessScore > 100 {
		return fmt.Errorf("riggingReadinessScore must be in [0, 100], got %d", op.RiggingReadinessScore)
	}
	if len(op.ProtocolSteps) < 1 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.ID) == "" ||
			strings.TrimSpace(step.Label) == "" ||
			!validVerticalStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty id, label, and valid category", i)
		}
	}
	if op.Status == VerticalStatusLoweredGroundSafe && strings.TrimSpace(op.ClearanceCertificateHash) == "" {
		return fmt.Errorf("clearanceCertificateHash is required when status is LOWERED_GROUND_SAFE")
	}
	if strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
