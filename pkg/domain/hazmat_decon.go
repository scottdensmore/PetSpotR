package domain

import (
	"fmt"
	"strings"
)

// HazmatExposureType classifies the environmental smoke, ashfall, or chemical/biological exposure agent.
type HazmatExposureType string

const (
	HazmatExposureWildfireSmokeAshfall   HazmatExposureType = "WILDFIRE_SMOKE_ASHFALL"
	HazmatExposurePetrochemicalSpillCoat HazmatExposureType = "PETROCHEMICAL_SPILL_COAT"
	HazmatExposureToxicAlgaeCyanotoxin   HazmatExposureType = "TOXIC_ALGAE_CYANOTOXIN"
	HazmatExposureTearGasAerosolPlume    HazmatExposureType = "TEAR_GAS_AEROSOL_PLUME"
	HazmatExposureIndustrialSolventVapor HazmatExposureType = "INDUSTRIAL_SOLVENT_VAPOR"
)

// IsValid returns true when HazmatExposureType is a supported enum value.
func (t HazmatExposureType) IsValid() bool {
	switch t {
	case HazmatExposureWildfireSmokeAshfall,
		HazmatExposurePetrochemicalSpillCoat,
		HazmatExposureToxicAlgaeCyanotoxin,
		HazmatExposureTearGasAerosolPlume,
		HazmatExposureIndustrialSolventVapor:
		return true
	default:
		return false
	}
}

// HazmatDeconStatus represents the lifecycle state of a hazmat & respiratory decontamination operation.
type HazmatDeconStatus string

const (
	HazmatStatusRespiratoryDermalCritical HazmatDeconStatus = "RESPIRATORY_DERMAL_CRITICAL"
	HazmatStatusDeconWashNebulizerStaged  HazmatDeconStatus = "DECON_WASH_NEBULIZER_STAGED"
	HazmatStatusDecontaminatedClearedSafe HazmatDeconStatus = "DECONTAMINATED_CLEARED_SAFE"
)

// IsValid returns true when HazmatDeconStatus is a supported lifecycle state.
func (s HazmatDeconStatus) IsValid() bool {
	switch s {
	case HazmatStatusRespiratoryDermalCritical,
		HazmatStatusDeconWashNebulizerStaged,
		HazmatStatusDecontaminatedClearedSafe:
		return true
	default:
		return false
	}
}

// HazmatProtocolStep represents an actionable decontamination, nebulization, or toxin-binder checklist step.
type HazmatProtocolStep struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Category    string `json:"category"` // "OCULAR_DERMAL_FLUSH", "OXYGEN_NEBULIZATION", "TOXIN_BINDER_MONITORING"
	Required    bool   `json:"required"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// HazmatDeconOperation represents a wildfire smoke, ashfall, hazmat plume, or cyanotoxin decontamination operation.
type HazmatDeconOperation struct {
	OperationID               string               `json:"operationId"`
	PetID                     string               `json:"petId"`
	PetName                   string               `json:"petName"`
	Species                   string               `json:"species"`
	WeightKg                  float64              `json:"weightKg"`
	ExposureType              HazmatExposureType   `json:"exposureType"`
	Status                    HazmatDeconStatus    `json:"status"`
	AQIIndex                  int                  `json:"aqiIndex"`
	PM25UgM3                  float64              `json:"pm25UgM3"`
	CarboxyhemoglobinPct      float64              `json:"carboxyhemoglobinPct"`
	CoatContaminationPct      float64              `json:"coatContaminationPct"`
	RespiratoryRateBpm        int                  `json:"respiratoryRateBpm"`
	HazardScore               int                  `json:"hazardScore"`
	DeconReadinessScore       int                  `json:"deconReadinessScore"`
	SmokeInhalationAlert      bool                 `json:"smokeInhalationAlert"`
	DermalToxinIngestionAlert bool                 `json:"dermalToxinIngestionAlert"`
	RecommendedWashAgent      string               `json:"recommendedWashAgent"`
	RecommendedOxygenTherapy  string               `json:"recommendedOxygenTherapy"`
	DeconCorridorZone         string               `json:"deconCorridorZone"`
	RiskFlags                 []string             `json:"riskFlags"`
	ProtocolSteps             []HazmatProtocolStep `json:"protocolSteps"`
	ClearanceCertificateHash  string               `json:"clearanceCertificateHash,omitempty"`
	LeadToxTechCallsign       string               `json:"leadToxTechCallsign"`
	Notes                     string               `json:"notes"`
	UpdatedAt                 string               `json:"updatedAt"`
}

// HazmatEvaluationRequest represents input parameters for evaluating a hazmat & respiratory decon operation.
type HazmatEvaluationRequest struct {
	OperationID          string             `json:"operationId,omitempty"`
	PetID                string             `json:"petId"`
	PetName              string             `json:"petName"`
	Species              string             `json:"species"`
	WeightKg             float64            `json:"weightKg"`
	ExposureType         HazmatExposureType `json:"exposureType"`
	AQIIndex             int                `json:"aqiIndex"`
	PM25UgM3             float64            `json:"pm25UgM3"`
	CarboxyhemoglobinPct float64            `json:"carboxyhemoglobinPct"`
	CoatContaminationPct float64            `json:"coatContaminationPct"`
	RespiratoryRateBpm   int                `json:"respiratoryRateBpm"`
	DeconCorridorZone    string             `json:"deconCorridorZone"`
	LeadToxTechCallsign  string             `json:"leadToxTechCallsign"`
	Notes                string             `json:"notes"`
}

// HazmatStepUpdateRequest updates a protocol step on an existing hazmat decon operation.
type HazmatStepUpdateRequest struct {
	StepID    string `json:"stepId"`
	Completed bool   `json:"completed"`
	Notes     string `json:"notes,omitempty"`
}

func validHazmatStepCategory(cat string) bool {
	switch cat {
	case "OCULAR_DERMAL_FLUSH", "OXYGEN_NEBULIZATION", "TOXIN_BINDER_MONITORING":
		return true
	default:
		return false
	}
}

// Validate checks domain invariants on HazmatDeconOperation.
func (op HazmatDeconOperation) Validate() error {
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
	if !op.ExposureType.IsValid() {
		return fmt.Errorf("invalid exposureType: %q", op.ExposureType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.AQIIndex < 0 || op.AQIIndex > 1000 {
		return fmt.Errorf("aqiIndex must be in [0, 1000], got %d", op.AQIIndex)
	}
	if op.PM25UgM3 < 0 || op.PM25UgM3 > 2000 {
		return fmt.Errorf("pm25UgM3 must be in [0, 2000], got %v", op.PM25UgM3)
	}
	if op.CarboxyhemoglobinPct < 0 || op.CarboxyhemoglobinPct > 100 {
		return fmt.Errorf("carboxyhemoglobinPct must be in [0, 100], got %v", op.CarboxyhemoglobinPct)
	}
	if op.CoatContaminationPct < 0 || op.CoatContaminationPct > 100 {
		return fmt.Errorf("coatContaminationPct must be in [0, 100], got %v", op.CoatContaminationPct)
	}
	if op.RespiratoryRateBpm <= 0 || op.RespiratoryRateBpm > 250 {
		return fmt.Errorf("respiratoryRateBpm must be in (0, 250], got %d", op.RespiratoryRateBpm)
	}
	if op.HazardScore < 0 || op.HazardScore > 100 {
		return fmt.Errorf("hazardScore must be in [0, 100], got %d", op.HazardScore)
	}
	if op.DeconReadinessScore < 0 || op.DeconReadinessScore > 100 {
		return fmt.Errorf("deconReadinessScore must be in [0, 100], got %d", op.DeconReadinessScore)
	}
	if len(op.ProtocolSteps) == 0 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.ID) == "" ||
			strings.TrimSpace(step.Label) == "" ||
			!validHazmatStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty id, label, and valid category", i)
		}
	}
	if op.Status == HazmatStatusDecontaminatedClearedSafe && strings.TrimSpace(op.ClearanceCertificateHash) == "" {
		return fmt.Errorf("clearanceCertificateHash is required when status is DECONTAMINATED_CLEARED_SAFE")
	}
	if strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
