package domain

import (
	"fmt"
	"strings"
)

// ConfinedSpaceType classifies the subterranean or enclosed rescue geometry.
type ConfinedSpaceType string

const (
	ConfinedSpaceStormDrainCulvert       ConfinedSpaceType = "STORM_DRAIN_CULVERT"
	ConfinedSpaceSubterraneanUtilityPipe ConfinedSpaceType = "SUBTERRANEAN_UTILITY_PIPE"
	ConfinedSpaceFoundationCrawlspace    ConfinedSpaceType = "FOUNDATION_CRAWLSPACE"
	ConfinedSpaceChimneyFlueShaft        ConfinedSpaceType = "CHIMNEY_FLUE_SHAFT"
	ConfinedSpaceCollapsedBurrowTrench   ConfinedSpaceType = "COLLAPSED_BURROW_TRENCH"
)

// IsValid returns true when ConfinedSpaceType is a supported enum value.
func (t ConfinedSpaceType) IsValid() bool {
	switch t {
	case ConfinedSpaceStormDrainCulvert,
		ConfinedSpaceSubterraneanUtilityPipe,
		ConfinedSpaceFoundationCrawlspace,
		ConfinedSpaceChimneyFlueShaft,
		ConfinedSpaceCollapsedBurrowTrench:
		return true
	default:
		return false
	}
}

// ConfinedSpaceStatus represents the lifecycle state of a confined-space extraction.
type ConfinedSpaceStatus string

const (
	ConfinedSpaceStatusAtmosphericHazardCritical ConfinedSpaceStatus = "ATMOSPHERIC_HAZARD_CRITICAL"
	ConfinedSpaceStatusBorescopePipeGateRigged   ConfinedSpaceStatus = "BORESCOPE_PIPE_GATE_RIGGED"
	ConfinedSpaceStatusExtractedSubterraneanSafe ConfinedSpaceStatus = "EXTRACTED_SUBTERRANEAN_SAFE"
)

// IsValid returns true when ConfinedSpaceStatus is a supported lifecycle state.
func (s ConfinedSpaceStatus) IsValid() bool {
	switch s {
	case ConfinedSpaceStatusAtmosphericHazardCritical,
		ConfinedSpaceStatusBorescopePipeGateRigged,
		ConfinedSpaceStatusExtractedSubterraneanSafe:
		return true
	default:
		return false
	}
}

// ConfinedSpaceProtocolStep represents an actionable safety or rigging checklist step.
type ConfinedSpaceProtocolStep struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Category    string `json:"category"` // "GAS_VENTILATION", "BORESCOPE_OPTICS", "ONE_WAY_PIPE_GATE"
	Required    bool   `json:"required"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// ConfinedSpaceOperation represents a subterranean, storm-drain, culvert, or confined-space extraction operation.
type ConfinedSpaceOperation struct {
	OperationID              string                      `json:"operationId"`
	PetID                    string                      `json:"petId"`
	PetName                  string                      `json:"petName"`
	Species                  string                      `json:"species"`
	WeightKg                 float64                     `json:"weightKg"`
	SpaceType                ConfinedSpaceType           `json:"spaceType"`
	Status                   ConfinedSpaceStatus         `json:"status"`
	PipeDiameterCm           float64                     `json:"pipeDiameterCm"`
	DepthMeters              float64                     `json:"depthMeters"`
	OxygenPercent            float64                     `json:"oxygenPercent"`
	H2SPpm                   float64                     `json:"h2sPpm"`
	InflowRiskIndex          int                         `json:"inflowRiskIndex"`
	HazardScore              int                         `json:"hazardScore"`
	RiggingReadinessScore    int                         `json:"riggingReadinessScore"`
	ToxicGasAlert            bool                        `json:"toxicGasAlert"`
	FlashInflowAlert         bool                        `json:"flashInflowAlert"`
	RecommendedOpticsRig     string                      `json:"recommendedOpticsRig"`
	RecommendedVentilation   string                      `json:"recommendedVentilation"`
	ManholeSectorGrid        string                      `json:"manholeSectorGrid"`
	RiskFlags                []string                    `json:"riskFlags"`
	ProtocolSteps            []ConfinedSpaceProtocolStep `json:"protocolSteps"`
	ClearanceCertificateHash string                      `json:"clearanceCertificateHash,omitempty"`
	LeadTechCallsign         string                      `json:"leadTechCallsign"`
	Notes                    string                      `json:"notes"`
	UpdatedAt                string                      `json:"updatedAt"`
}

// ConfinedSpaceEvaluationRequest represents input parameters for evaluating a confined-space rescue operation.
type ConfinedSpaceEvaluationRequest struct {
	OperationID       string            `json:"operationId,omitempty"`
	PetID             string            `json:"petId"`
	PetName           string            `json:"petName"`
	Species           string            `json:"species"`
	WeightKg          float64           `json:"weightKg"`
	SpaceType         ConfinedSpaceType `json:"spaceType"`
	PipeDiameterCm    float64           `json:"pipeDiameterCm"`
	DepthMeters       float64           `json:"depthMeters"`
	OxygenPercent     float64           `json:"oxygenPercent"`
	H2SPpm            float64           `json:"h2sPpm"`
	InflowRiskIndex   int               `json:"inflowRiskIndex"`
	ManholeSectorGrid string            `json:"manholeSectorGrid"`
	LeadTechCallsign  string            `json:"leadTechCallsign"`
	Notes             string            `json:"notes"`
}

// ConfinedSpaceStepUpdateRequest updates a protocol step on an existing operation.
type ConfinedSpaceStepUpdateRequest struct {
	StepID    string `json:"stepId"`
	Completed bool   `json:"completed"`
	Notes     string `json:"notes,omitempty"`
}

func validConfinedSpaceStepCategory(cat string) bool {
	switch cat {
	case "GAS_VENTILATION", "BORESCOPE_OPTICS", "ONE_WAY_PIPE_GATE":
		return true
	default:
		return false
	}
}

// Validate checks domain invariants on ConfinedSpaceOperation.
func (op ConfinedSpaceOperation) Validate() error {
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
	if !op.SpaceType.IsValid() {
		return fmt.Errorf("invalid spaceType: %q", op.SpaceType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.PipeDiameterCm <= 0 || op.PipeDiameterCm > 500 {
		return fmt.Errorf("pipeDiameterCm must be in (0, 500], got %v", op.PipeDiameterCm)
	}
	if op.DepthMeters < 0 || op.DepthMeters > 100 {
		return fmt.Errorf("depthMeters must be in [0, 100], got %v", op.DepthMeters)
	}
	if op.OxygenPercent < 0 || op.OxygenPercent > 25 {
		return fmt.Errorf("oxygenPercent must be in [0, 25], got %v", op.OxygenPercent)
	}
	if op.H2SPpm < 0 || op.H2SPpm > 500 {
		return fmt.Errorf("h2sPpm must be in [0, 500], got %v", op.H2SPpm)
	}
	if op.InflowRiskIndex < 0 || op.InflowRiskIndex > 100 {
		return fmt.Errorf("inflowRiskIndex must be in [0, 100], got %d", op.InflowRiskIndex)
	}
	if op.HazardScore < 0 || op.HazardScore > 100 {
		return fmt.Errorf("hazardScore must be in [0, 100], got %d", op.HazardScore)
	}
	if op.RiggingReadinessScore < 0 || op.RiggingReadinessScore > 100 {
		return fmt.Errorf("riggingReadinessScore must be in [0, 100], got %d", op.RiggingReadinessScore)
	}
	if len(op.ProtocolSteps) == 0 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.ID) == "" ||
			strings.TrimSpace(step.Label) == "" ||
			!validConfinedSpaceStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty id, label, and valid category", i)
		}
	}
	if op.Status == ConfinedSpaceStatusExtractedSubterraneanSafe && strings.TrimSpace(op.ClearanceCertificateHash) == "" {
		return fmt.Errorf("clearanceCertificateHash is required when status is EXTRACTED_SUBTERRANEAN_SAFE")
	}
	if strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
