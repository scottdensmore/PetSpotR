package domain

import (
	"fmt"
	"strings"
)

// EnvenomationToxinType classifies the wildlife venom, amphibian cardiotoxin, retrograde quill, or cyanotoxin vector.
type EnvenomationToxinType string

const (
	ToxinPitViperCrotalidHEI      EnvenomationToxinType = "PIT_VIPER_CROTALID_HEI"
	ToxinElapidCoralNeurotoxin    EnvenomationToxinType = "ELAPID_CORAL_NEUROTOXIN"
	ToxinBufoMarinusCardiotoxin   EnvenomationToxinType = "BUFO_MARINUS_CARDIOTOXIN"
	ToxinPorcupineRetrogradeQuill EnvenomationToxinType = "PORCUPINE_RETROGRADE_QUILLS"
	ToxinBlueGreenAlgaeCyanotoxin EnvenomationToxinType = "BLUE_GREEN_ALGAE_CYANOTOXIN"
)

// IsValid returns true when EnvenomationToxinType is a supported toxin enum value.
func (t EnvenomationToxinType) IsValid() bool {
	switch t {
	case ToxinPitViperCrotalidHEI,
		ToxinElapidCoralNeurotoxin,
		ToxinBufoMarinusCardiotoxin,
		ToxinPorcupineRetrogradeQuill,
		ToxinBlueGreenAlgaeCyanotoxin:
		return true
	default:
		return false
	}
}

// EnvenomationTriageStatus represents the lifecycle state of an envenomation, bufotoxin, quill, or toxic flora triage operation.
type EnvenomationTriageStatus string

const (
	EnvenomationStatusCritical   EnvenomationTriageStatus = "COAGULOPATHY_NEUROTOXIC_CRITICAL"
	EnvenomationStatusStaged     EnvenomationTriageStatus = "ANTIVENOM_LAVAGE_SEDATION_STAGED"
	EnvenomationStatusStabilized EnvenomationTriageStatus = "STABILIZED_NEUTRALIZED_CLEARED"
)

// IsValid returns true when EnvenomationTriageStatus is a supported lifecycle state.
func (s EnvenomationTriageStatus) IsValid() bool {
	switch s {
	case EnvenomationStatusCritical,
		EnvenomationStatusStaged,
		EnvenomationStatusStabilized:
		return true
	default:
		return false
	}
}

// EnvenomationProtocolStep represents an actionable demarcation perimeter, forward oral lavage, sedated quill extraction, or coagulopathy/echinocyte recheck step.
type EnvenomationProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// EnvenomationTriageOperation represents a pit-viper envenomation, Bufo toad bufotoxin, porcupine quill, or toxic flora triage operation.
type EnvenomationTriageOperation struct {
	ID                            string                     `json:"id"`
	PetID                         string                     `json:"pet_id"`
	PetName                       string                     `json:"pet_name"`
	Species                       string                     `json:"species"`
	WeightKg                      float64                    `json:"weight_kg"`
	ToxinType                     EnvenomationToxinType      `json:"toxin_type"`
	Status                        EnvenomationTriageStatus   `json:"status"`
	EdemaSpreadCmHr               float64                    `json:"edema_spread_cm_hr"`
	EchinocytePct                 float64                    `json:"echinocyte_pct"`
	QuillCountMuzzleThorax        int                        `json:"quill_count_muzzle_thorax"`
	OralMucosaFlushedForward      bool                       `json:"oral_mucosa_flushed_forward"`
	QuillsCutOrPulledAwakeAvoided bool                       `json:"quills_cut_or_pulled_awake_avoided"`
	AntivenomVialsStaged          int                        `json:"antivenom_vials_staged"`
	RequiredAntidoteProtocol      string                     `json:"required_antidote_protocol"`
	ContraindicationWarningNote   string                     `json:"contraindication_warning_note"`
	ProtocolSteps                 []EnvenomationProtocolStep `json:"protocol_steps"`
	ClearanceToken                string                     `json:"clearance_token,omitempty"`
	CreatedAt                     string                     `json:"created_at"`
	UpdatedAt                     string                     `json:"updated_at"`
}

// EnvenomationEvaluationRequest represents input parameters for evaluating an envenomation / toxin triage operation.
type EnvenomationEvaluationRequest struct {
	ID                            string                `json:"id,omitempty"`
	PetID                         string                `json:"pet_id"`
	PetName                       string                `json:"pet_name"`
	Species                       string                `json:"species"`
	WeightKg                      float64               `json:"weight_kg"`
	ToxinType                     EnvenomationToxinType `json:"toxin_type"`
	EdemaSpreadCmHr               float64               `json:"edema_spread_cm_hr"`
	EchinocytePct                 float64               `json:"echinocyte_pct"`
	QuillCountMuzzleThorax        int                   `json:"quill_count_muzzle_thorax"`
	OralMucosaFlushedForward      bool                  `json:"oral_mucosa_flushed_forward"`
	QuillsCutOrPulledAwakeAvoided bool                  `json:"quills_cut_or_pulled_awake_avoided"`
	AntivenomVialsStaged          int                   `json:"antivenom_vials_staged"`
}

// EnvenomationStepUpdateRequest updates a protocol step on an existing envenomation toxin triage operation.
type EnvenomationStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on EnvenomationTriageOperation.
func (op EnvenomationTriageOperation) Validate() error {
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
	if op.WeightKg <= 0 || op.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", op.WeightKg)
	}
	if !op.ToxinType.IsValid() {
		return fmt.Errorf("invalid toxin_type: %q", op.ToxinType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.EdemaSpreadCmHr < 0 || op.EdemaSpreadCmHr > 50.0 {
		return fmt.Errorf("edema_spread_cm_hr must be in [0, 50.0], got %v", op.EdemaSpreadCmHr)
	}
	if op.EchinocytePct < 0 || op.EchinocytePct > 100.0 {
		return fmt.Errorf("echinocyte_pct must be in [0, 100.0], got %v", op.EchinocytePct)
	}
	if op.QuillCountMuzzleThorax < 0 || op.QuillCountMuzzleThorax > 500 {
		return fmt.Errorf("quill_count_muzzle_thorax must be in [0, 500], got %d", op.QuillCountMuzzleThorax)
	}
	if op.AntivenomVialsStaged < 0 || op.AntivenomVialsStaged > 20 {
		return fmt.Errorf("antivenom_vials_staged must be in [0, 20], got %d", op.AntivenomVialsStaged)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
