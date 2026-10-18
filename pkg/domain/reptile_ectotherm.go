package domain

import (
	"fmt"
	"strings"
)

// EctothermTaxonType classifies the reptile, chelonian, or amphibian taxonomic and habitat profile.
type EctothermTaxonType string

const (
	EctothermTaxonChelonianTortoiseBurrow    EctothermTaxonType = "CHELONIAN_TORTOISE_BURROW"
	EctothermTaxonBoidPythonConstrictor      EctothermTaxonType = "BOID_PYTHON_CONSTRICTOR"
	EctothermTaxonAgamidIguanidArboreal      EctothermTaxonType = "AGAMID_IGUANID_ARBOREAL"
	EctothermTaxonColubridCornKingSnake      EctothermTaxonType = "COLUBRID_CORN_KING_SNAKE"
	EctothermTaxonAmphibianSalamanderAxolotl EctothermTaxonType = "AMPHIBIAN_SALAMANDER_AXOLOTL"
)

// IsValid returns true when EctothermTaxonType is a supported enum value.
func (t EctothermTaxonType) IsValid() bool {
	switch t {
	case EctothermTaxonChelonianTortoiseBurrow,
		EctothermTaxonBoidPythonConstrictor,
		EctothermTaxonAgamidIguanidArboreal,
		EctothermTaxonColubridCornKingSnake,
		EctothermTaxonAmphibianSalamanderAxolotl:
		return true
	default:
		return false
	}
}

// EctothermRescueStatus represents the lifecycle state of a reptile/ectotherm thermal-dormancy rescue operation.
type EctothermRescueStatus string

const (
	EctothermRescueStatusColdTorporBrumationCritical  EctothermRescueStatus = "COLD_TORPOR_BRUMATION_CRITICAL"
	EctothermRescueStatusBaskingLureCreviceStaged     EctothermRescueStatus = "BASKING_LURE_CREVICE_STAGED"
	EctothermRescueStatusRecoveredThermalGradientSafe EctothermRescueStatus = "RECOVERED_THERMAL_GRADIENT_SAFE"
)

// IsValid returns true when EctothermRescueStatus is a supported lifecycle state.
func (s EctothermRescueStatus) IsValid() bool {
	switch s {
	case EctothermRescueStatusColdTorporBrumationCritical,
		EctothermRescueStatusBaskingLureCreviceStaged,
		EctothermRescueStatusRecoveredThermalGradientSafe:
		return true
	default:
		return false
	}
}

// EctothermProtocolStep represents an actionable endoscope crevice sweep, ceramic IR basking lure, or gradual coelomic rewarming checklist step.
type EctothermProtocolStep struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Category    string `json:"category"` // "ENDOSCOPE_CREVICE_SWEEP", "CERAMIC_IR_BASKING_LURE", "GRADUAL_COELOMIC_REWARMING"
	Required    bool   `json:"required"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// EctothermRescueOperation represents an ectothermic reptile, chelonian, or amphibian thermal-dormancy recovery operation.
type EctothermRescueOperation struct {
	OperationID                string                  `json:"operationId"`
	PetID                      string                  `json:"petId"`
	PetName                    string                  `json:"petName"`
	Species                    string                  `json:"species"`
	WeightKg                   float64                 `json:"weightKg"`
	TaxonType                  EctothermTaxonType      `json:"taxonType"`
	Status                     EctothermRescueStatus   `json:"status"`
	SubstrateTempCelsius       float64                 `json:"substrateTempCelsius"`
	PreferredOptimumTempC      float64                 `json:"preferredOptimumTempC"`
	RelativeHumidityPct        float64                 `json:"relativeHumidityPct"`
	HoursDormant               float64                 `json:"hoursDormant"`
	BurrowDepthCm              float64                 `json:"burrowDepthCm"`
	HazardScore                int                     `json:"hazardScore"`
	ThermalReadinessScore      int                     `json:"thermalReadinessScore"`
	CriticalTorporAlert        bool                    `json:"criticalTorporAlert"`
	DesiccationDysecdysisAlert bool                    `json:"desiccationDysecdysisAlert"`
	RecommendedThermalLure     string                  `json:"recommendedThermalLure"`
	RecommendedContainment     string                  `json:"recommendedContainment"`
	MicroHabitatGrid           string                  `json:"microHabitatGrid"`
	RiskFlags                  []string                `json:"riskFlags"`
	ProtocolSteps              []EctothermProtocolStep `json:"protocolSteps"`
	ClearanceCertificateHash   string                  `json:"clearanceCertificateHash,omitempty"`
	LeadHerpTechCallsign       string                  `json:"leadHerpTechCallsign"`
	Notes                      string                  `json:"notes"`
	UpdatedAt                  string                  `json:"updatedAt"`
}

// EctothermEvaluationRequest represents input parameters for evaluating an ectotherm thermal-dormancy rescue operation.
type EctothermEvaluationRequest struct {
	OperationID           string             `json:"operationId,omitempty"`
	PetID                 string             `json:"petId"`
	PetName               string             `json:"petName"`
	Species               string             `json:"species"`
	WeightKg              float64            `json:"weightKg"`
	TaxonType             EctothermTaxonType `json:"taxonType"`
	SubstrateTempCelsius  float64            `json:"substrateTempCelsius"`
	PreferredOptimumTempC float64            `json:"preferredOptimumTempC"`
	RelativeHumidityPct   float64            `json:"relativeHumidityPct"`
	HoursDormant          float64            `json:"hoursDormant"`
	BurrowDepthCm         float64            `json:"burrowDepthCm"`
	MicroHabitatGrid      string             `json:"microHabitatGrid"`
	LeadHerpTechCallsign  string             `json:"leadHerpTechCallsign"`
	Notes                 string             `json:"notes"`
}

// EctothermStepUpdateRequest updates a protocol step on an existing ectotherm rescue operation.
type EctothermStepUpdateRequest struct {
	StepID    string `json:"stepId"`
	Completed bool   `json:"completed"`
	Notes     string `json:"notes,omitempty"`
}

func validEctothermStepCategory(cat string) bool {
	switch cat {
	case "ENDOSCOPE_CREVICE_SWEEP", "CERAMIC_IR_BASKING_LURE", "GRADUAL_COELOMIC_REWARMING":
		return true
	default:
		return false
	}
}

// Validate checks domain invariants on EctothermRescueOperation.
func (op EctothermRescueOperation) Validate() error {
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
	if !op.TaxonType.IsValid() {
		return fmt.Errorf("invalid taxonType: %q", op.TaxonType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.SubstrateTempCelsius < -20 || op.SubstrateTempCelsius > 55 {
		return fmt.Errorf("substrateTempCelsius must be in [-20, 55], got %v", op.SubstrateTempCelsius)
	}
	if op.PreferredOptimumTempC < 10 || op.PreferredOptimumTempC > 48 {
		return fmt.Errorf("preferredOptimumTempC must be in [10, 48], got %v", op.PreferredOptimumTempC)
	}
	if op.RelativeHumidityPct < 0 || op.RelativeHumidityPct > 100 {
		return fmt.Errorf("relativeHumidityPct must be in [0, 100], got %v", op.RelativeHumidityPct)
	}
	if op.HoursDormant < 0 || op.HoursDormant > 2160 {
		return fmt.Errorf("hoursDormant must be in [0, 2160], got %v", op.HoursDormant)
	}
	if op.BurrowDepthCm < 0 || op.BurrowDepthCm > 1000 {
		return fmt.Errorf("burrowDepthCm must be in [0, 1000], got %v", op.BurrowDepthCm)
	}
	if op.HazardScore < 0 || op.HazardScore > 100 {
		return fmt.Errorf("hazardScore must be in [0, 100], got %d", op.HazardScore)
	}
	if op.ThermalReadinessScore < 0 || op.ThermalReadinessScore > 100 {
		return fmt.Errorf("thermalReadinessScore must be in [0, 100], got %d", op.ThermalReadinessScore)
	}
	if len(op.ProtocolSteps) == 0 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.ID) == "" ||
			strings.TrimSpace(step.Label) == "" ||
			!validEctothermStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty id, label, and valid category", i)
		}
	}
	if op.Status == EctothermRescueStatusRecoveredThermalGradientSafe && strings.TrimSpace(op.ClearanceCertificateHash) == "" {
		return fmt.Errorf("clearanceCertificateHash is required when status is RECOVERED_THERMAL_GRADIENT_SAFE")
	}
	if strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
