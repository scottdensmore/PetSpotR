package domain

import (
	"fmt"
	"strings"
)

// UrethralObstructionCauseType classifies the feline urethral obstruction (blocked tom) or canine os penis calculus cause.
type UrethralObstructionCauseType string

const (
	ObstructionCauseStruwiteCrystallineMatrixPlug       UrethralObstructionCauseType = "STRUWITE_CRYSTALLINE_MATRIX_PLUG"
	ObstructionCauseCalciumOxalateUrethrolith           UrethralObstructionCauseType = "CALCIUM_OXALATE_URETHROLITH"
	ObstructionCauseIdiopathicFICUrethralSpasm          UrethralObstructionCauseType = "IDIOPATHIC_FIC_URETHRAL_SPASM"
	ObstructionCauseCanineOsPenisCalculusLodgment       UrethralObstructionCauseType = "CANINE_OS_PENIS_CALCULUS_LODGMENT"
	ObstructionCausePostCatheterStricturedReobstruction UrethralObstructionCauseType = "POST_CATHETER_STRICTURED_REOBSTRUCTION"
)

// IsValid returns true when UrethralObstructionCauseType is a supported obstruction cause enum value.
func (c UrethralObstructionCauseType) IsValid() bool {
	switch c {
	case ObstructionCauseStruwiteCrystallineMatrixPlug,
		ObstructionCauseCalciumOxalateUrethrolith,
		ObstructionCauseIdiopathicFICUrethralSpasm,
		ObstructionCauseCanineOsPenisCalculusLodgment,
		ObstructionCausePostCatheterStricturedReobstruction:
		return true
	default:
		return false
	}
}

// UrethralTriageStatus represents the lifecycle state of a Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection triage operation.
type UrethralTriageStatus string

const (
	UrethralStatusCriticalHyperkalemicBradycardia   UrethralTriageStatus = "HYPERKALEMIC_BRADYCARDIA_ATRIAL_STANDSTILL_CRITICAL"
	UrethralStatusStagedCalciumRetrohydropropulsion UrethralTriageStatus = "CALCIUM_CARDIOPROTECT_RETROHYDROPROPULSION_STAGED"
	UrethralStatusClearedIndwellingNormokalemic     UrethralTriageStatus = "INDWELLING_CLOSED_DIURESIS_NORMOKALEMIC_CLEARED"
)

// IsValid returns true when UrethralTriageStatus is a supported lifecycle state.
func (s UrethralTriageStatus) IsValid() bool {
	switch s {
	case UrethralStatusCriticalHyperkalemicBradycardia,
		UrethralStatusStagedCalciumRetrohydropropulsion,
		UrethralStatusClearedIndwellingNormokalemic:
		return true
	default:
		return false
	}
}

// UrethralProtocolStep represents an actionable 10% calcium gluconate + insulin/dextrose cardioprotection, sacrococcygeal epidural & retrohydropropulsion, closed urine collection post-obstructive diuresis, or prazosin/normokalemia clearance step.
type UrethralProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// UrethralObstructionOperation represents a Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection triage operation.
type UrethralObstructionOperation struct {
	ID                             string                       `json:"id"`
	PetID                          string                       `json:"pet_id"`
	PetName                        string                       `json:"pet_name"`
	Species                        string                       `json:"species"`
	WeightKg                       float64                      `json:"weight_kg"`
	ObstructionCause               UrethralObstructionCauseType `json:"obstruction_cause"`
	Status                         UrethralTriageStatus         `json:"status"`
	SerumPotassiumMmolL            float64                      `json:"serum_potassium_mmol_l"`
	HeartRateBpm                   int                          `json:"heart_rate_bpm"`
	BladderPalpationDiameterCm     float64                      `json:"bladder_palpation_diameter_cm"`
	AbsentPWaveTentedTWave         bool                         `json:"absent_p_wave_tented_t_wave"`
	CalciumGluconateBeforeSedation bool                         `json:"calcium_gluconate_before_sedation"`
	ManualBladderExpressionAvoided bool                         `json:"manual_bladder_expression_avoided"`
	RequiredUnblockingProtocol     string                       `json:"required_unblocking_protocol"`
	CardioprotectionWarningNote    string                       `json:"cardioprotection_warning_note"`
	ProtocolSteps                  []UrethralProtocolStep       `json:"protocol_steps"`
	ClearanceToken                 string                       `json:"clearance_token,omitempty"`
	CreatedAt                      string                       `json:"created_at"`
	UpdatedAt                      string                       `json:"updated_at"`
}

// UrethralEvaluationRequest represents input parameters for evaluating a Feline Urethral Obstruction (Blocked Tom), Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection triage operation.
type UrethralEvaluationRequest struct {
	ID                             string                       `json:"id,omitempty"`
	PetID                          string                       `json:"pet_id"`
	PetName                        string                       `json:"pet_name"`
	Species                        string                       `json:"species"`
	WeightKg                       float64                      `json:"weight_kg"`
	ObstructionCause               UrethralObstructionCauseType `json:"obstruction_cause"`
	SerumPotassiumMmolL            float64                      `json:"serum_potassium_mmol_l"`
	HeartRateBpm                   int                          `json:"heart_rate_bpm"`
	BladderPalpationDiameterCm     float64                      `json:"bladder_palpation_diameter_cm"`
	AbsentPWaveTentedTWave         bool                         `json:"absent_p_wave_tented_t_wave"`
	CalciumGluconateBeforeSedation bool                         `json:"calcium_gluconate_before_sedation"`
	ManualBladderExpressionAvoided bool                         `json:"manual_bladder_expression_avoided"`
}

// UrethralStepUpdateRequest updates a protocol step on an existing urethral obstruction triage operation.
type UrethralStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on UrethralObstructionOperation.
func (op UrethralObstructionOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.PetID) == "" {
		return fmt.Errorf("pet_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if strings.TrimSpace(op.Species) == "" {
		return fmt.Errorf("species is required")
	}
	if op.WeightKg < 1.5 || op.WeightKg > 65.0 {
		return fmt.Errorf("weight_kg must be in [1.5, 65.0], got %v", op.WeightKg)
	}
	if !op.ObstructionCause.IsValid() {
		return fmt.Errorf("invalid obstruction_cause: %q", op.ObstructionCause)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.SerumPotassiumMmolL < 2.5 || op.SerumPotassiumMmolL > 12.5 {
		return fmt.Errorf("serum_potassium_mmol_l must be in [2.5, 12.5], got %v", op.SerumPotassiumMmolL)
	}
	if op.HeartRateBpm < 60 || op.HeartRateBpm > 260 {
		return fmt.Errorf("heart_rate_bpm must be in [60, 260], got %d", op.HeartRateBpm)
	}
	if op.BladderPalpationDiameterCm < 2.0 || op.BladderPalpationDiameterCm > 20.0 {
		return fmt.Errorf("bladder_palpation_diameter_cm must be in [2.0, 20.0], got %v", op.BladderPalpationDiameterCm)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
