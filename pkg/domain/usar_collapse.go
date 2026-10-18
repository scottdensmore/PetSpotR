package domain

import (
	"fmt"
	"strings"
)

// USARVoidType classifies the structural collapse void-space geometry in an Urban Search & Rescue operation.
type USARVoidType string

const (
	USARVoidPancakeFloorSlabVoid    USARVoidType = "PANCAKE_FLOOR_SLAB_VOID"
	USARVoidLeanToTimberTrussCavity USARVoidType = "LEAN_TO_TIMBER_TRUSS_CAVITY"
	USARVoidVShapeMasonryRubble     USARVoidType = "V_SHAPE_MASONRY_RUBBLE"
	USARVoidCantileverBeamOverhang  USARVoidType = "CANTILEVER_BEAM_OVERHANG"
	USARVoidSubBasementUtilityVault USARVoidType = "SUB_BASEMENT_UTILITY_VAULT"
)

// IsValid returns true when USARVoidType is a supported enum value.
func (t USARVoidType) IsValid() bool {
	switch t {
	case USARVoidPancakeFloorSlabVoid,
		USARVoidLeanToTimberTrussCavity,
		USARVoidVShapeMasonryRubble,
		USARVoidCantileverBeamOverhang,
		USARVoidSubBasementUtilityVault:
		return true
	default:
		return false
	}
}

// USARRescueStatus represents the lifecycle state of a seismic rubble / USAR structural collapse operation.
type USARRescueStatus string

const (
	USARRescueStatusSecondaryCollapseCrushCritical USARRescueStatus = "SECONDARY_COLLAPSE_CRUSH_CRITICAL"
	USARRescueStatusShoringCribbingDelsarStaged    USARRescueStatus = "SHORING_CRIBBING_DELSAR_STAGED"
	USARRescueStatusExtractedUSARDeconSafe         USARRescueStatus = "EXTRACTED_USAR_DECON_SAFE"
)

// IsValid returns true when USARRescueStatus is a supported lifecycle state.
func (s USARRescueStatus) IsValid() bool {
	switch s {
	case USARRescueStatusSecondaryCollapseCrushCritical,
		USARRescueStatusShoringCribbingDelsarStaged,
		USARRescueStatusExtractedUSARDeconSafe:
		return true
	default:
		return false
	}
}

// USARProtocolStep represents an actionable Delsar acoustic pinpointing, timber box-cribbing shoring, or crush-syndrome extraction step.
type USARProtocolStep struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Category    string `json:"category"` // "DELSAR_SEISMIC_PINPOINT", "BOX_CRIBBING_SHORING", "CRUSH_SYNDROME_EXTRACTION"
	Required    bool   `json:"required"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// USARRescueOperation represents a seismic rubble, urban search & rescue (USAR) structural collapse & void-space extraction operation.
type USARRescueOperation struct {
	OperationID              string             `json:"operationId"`
	PetID                    string             `json:"petId"`
	PetName                  string             `json:"petName"`
	Species                  string             `json:"species"`
	WeightKg                 float64            `json:"weightKg"`
	VoidType                 USARVoidType       `json:"voidType"`
	Status                   USARRescueStatus   `json:"status"`
	RubbleOverburdenMeters   float64            `json:"rubbleOverburdenMeters"`
	AftershockPGA            float64            `json:"aftershockPGA"`
	SilicaDustMgM3           float64            `json:"silicaDustMgM3"`
	HoursEntrapped           float64            `json:"hoursEntrapped"`
	StructuralStabilityIdx   int                `json:"structuralStabilityIdx"`
	HazardScore              int                `json:"hazardScore"`
	ShoringReadinessScore    int                `json:"shoringReadinessScore"`
	SecondaryCollapseAlert   bool               `json:"secondaryCollapseAlert"`
	CrushRhabdoAlert         bool               `json:"crushRhabdoAlert"`
	RecommendedAcousticArray string             `json:"recommendedAcousticArray"`
	RecommendedShoringRig    string             `json:"recommendedShoringRig"`
	CollapseStructureGrid    string             `json:"collapseStructureGrid"`
	RiskFlags                []string           `json:"riskFlags"`
	ProtocolSteps            []USARProtocolStep `json:"protocolSteps"`
	ClearanceCertificateHash string             `json:"clearanceCertificateHash,omitempty"`
	LeadUSARSpecCallsign     string             `json:"leadUsarSpecCallsign"`
	Notes                    string             `json:"notes"`
	UpdatedAt                string             `json:"updatedAt"`
}

// USAREvaluationRequest represents input parameters for evaluating a USAR structural collapse operation.
type USAREvaluationRequest struct {
	OperationID            string       `json:"operationId,omitempty"`
	PetID                  string       `json:"petId"`
	PetName                string       `json:"petName"`
	Species                string       `json:"species"`
	WeightKg               float64      `json:"weightKg"`
	VoidType               USARVoidType `json:"voidType"`
	RubbleOverburdenMeters float64      `json:"rubbleOverburdenMeters"`
	AftershockPGA          float64      `json:"aftershockPGA"`
	SilicaDustMgM3         float64      `json:"silicaDustMgM3"`
	HoursEntrapped         float64      `json:"hoursEntrapped"`
	StructuralStabilityIdx int          `json:"structuralStabilityIdx"`
	CollapseStructureGrid  string       `json:"collapseStructureGrid"`
	LeadUSARSpecCallsign   string       `json:"leadUsarSpecCallsign"`
	Notes                  string       `json:"notes"`
}

// USARStepUpdateRequest updates a protocol step on an existing USAR rescue operation.
type USARStepUpdateRequest struct {
	StepID    string `json:"stepId"`
	Completed bool   `json:"completed"`
	Notes     string `json:"notes,omitempty"`
}

func validUSARStepCategory(cat string) bool {
	switch cat {
	case "DELSAR_SEISMIC_PINPOINT", "BOX_CRIBBING_SHORING", "CRUSH_SYNDROME_EXTRACTION":
		return true
	default:
		return false
	}
}

// Validate checks domain invariants on USARRescueOperation.
func (op USARRescueOperation) Validate() error {
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
	if !op.VoidType.IsValid() {
		return fmt.Errorf("invalid voidType: %q", op.VoidType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.RubbleOverburdenMeters < 0 || op.RubbleOverburdenMeters > 30 {
		return fmt.Errorf("rubbleOverburdenMeters must be in [0, 30], got %v", op.RubbleOverburdenMeters)
	}
	if op.AftershockPGA < 0 || op.AftershockPGA > 3.0 {
		return fmt.Errorf("aftershockPGA must be in [0, 3.0], got %v", op.AftershockPGA)
	}
	if op.SilicaDustMgM3 < 0 || op.SilicaDustMgM3 > 100 {
		return fmt.Errorf("silicaDustMgM3 must be in [0, 100], got %v", op.SilicaDustMgM3)
	}
	if op.HoursEntrapped < 0 || op.HoursEntrapped > 720 {
		return fmt.Errorf("hoursEntrapped must be in [0, 720], got %v", op.HoursEntrapped)
	}
	if op.StructuralStabilityIdx < 0 || op.StructuralStabilityIdx > 100 {
		return fmt.Errorf("structuralStabilityIdx must be in [0, 100], got %d", op.StructuralStabilityIdx)
	}
	if op.HazardScore < 0 || op.HazardScore > 100 {
		return fmt.Errorf("hazardScore must be in [0, 100], got %d", op.HazardScore)
	}
	if op.ShoringReadinessScore < 0 || op.ShoringReadinessScore > 100 {
		return fmt.Errorf("shoringReadinessScore must be in [0, 100], got %d", op.ShoringReadinessScore)
	}
	if len(op.ProtocolSteps) == 0 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.ID) == "" ||
			strings.TrimSpace(step.Label) == "" ||
			!validUSARStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty id, label, and valid category", i)
		}
	}
	if op.Status == USARRescueStatusExtractedUSARDeconSafe && strings.TrimSpace(op.ClearanceCertificateHash) == "" {
		return fmt.Errorf("clearanceCertificateHash is required when status is EXTRACTED_USAR_DECON_SAFE")
	}
	if strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
