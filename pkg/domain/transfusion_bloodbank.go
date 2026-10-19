package domain

import (
	"fmt"
	"strings"
)

// TransfusionBloodGroupType classifies the canine DEA 1.1, feline AB blood typing, or emergency canine-to-feline xenotransfusion profile.
type TransfusionBloodGroupType string

const (
	TransfusionBloodGroupFelineTypeBAlloantibodyRisk   TransfusionBloodGroupType = "FELINE_TYPE_B_ALLOANTIBODY_RISK"
	TransfusionBloodGroupFelineTypeACrossmatched       TransfusionBloodGroupType = "FELINE_TYPE_A_CROSSMATCHED"
	TransfusionBloodGroupCanineDEA11NegativeUniversal  TransfusionBloodGroupType = "CANINE_DEA_1_1_NEGATIVE_UNIVERSAL"
	TransfusionBloodGroupCanineDEA11PositiveSensitized TransfusionBloodGroupType = "CANINE_DEA_1_1_POSITIVE_SENSITIZED"
	TransfusionBloodGroupEmergencyCanineToFelineXeno   TransfusionBloodGroupType = "EMERGENCY_CANINE_TO_FELINE_XENOTRANSFUSION"
)

// IsValid returns true when TransfusionBloodGroupType is a supported blood group enum value.
func (b TransfusionBloodGroupType) IsValid() bool {
	switch b {
	case TransfusionBloodGroupFelineTypeBAlloantibodyRisk,
		TransfusionBloodGroupFelineTypeACrossmatched,
		TransfusionBloodGroupCanineDEA11NegativeUniversal,
		TransfusionBloodGroupCanineDEA11PositiveSensitized,
		TransfusionBloodGroupEmergencyCanineToFelineXeno:
		return true
	default:
		return false
	}
}

// TransfusionTriageStatus represents the lifecycle state of a Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction triage operation.
type TransfusionTriageStatus string

const (
	TransfusionStatusCriticalHemolyticAnaphylaxis   TransfusionTriageStatus = "ALLOANTIBODY_HEMOLYTIC_ANAPHYLAXIS_CRITICAL"
	TransfusionStatusStagedCrossmatchedMicronFilter TransfusionTriageStatus = "CROSSMATCHED_MICRON_FILTER_RATE_STAGED"
	TransfusionStatusSafePCVRestoredNormothermic    TransfusionTriageStatus = "TRANSFUSED_PCV_RESTORED_NORMOTHERMIC_SAFE"
)

// IsValid returns true when TransfusionTriageStatus is a supported lifecycle state.
func (s TransfusionTriageStatus) IsValid() bool {
	switch s {
	case TransfusionStatusCriticalHemolyticAnaphylaxis,
		TransfusionStatusStagedCrossmatchedMicronFilter,
		TransfusionStatusSafePCVRestoredNormothermic:
		return true
	default:
		return false
	}
}

// TransfusionProtocolStep represents an actionable AB/DEA 1.1 blood typing & slide crossmatch, 170-micron in-line filter + 0.9% NaCl line setup, 0.25 mL/kg/hr 15-min biologic trial, or post-transfusion PCV/TS & hemolysis check step.
type TransfusionProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// TransfusionBloodbankOperation represents a Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction triage operation.
type TransfusionBloodbankOperation struct {
	ID                               string                    `json:"id"`
	PetID                            string                    `json:"pet_id"`
	PetName                          string                    `json:"pet_name"`
	Species                          string                    `json:"species"`
	WeightKg                         float64                   `json:"weight_kg"`
	BloodGroupType                   TransfusionBloodGroupType `json:"blood_group_type"`
	Status                           TransfusionTriageStatus   `json:"status"`
	RecipientPCVPct                  float64                   `json:"recipient_pcv_pct"`
	DonorPCVPct                      float64                   `json:"donor_pcv_pct"`
	TargetPCVPct                     float64                   `json:"target_pcv_pct"`
	MajorMinorCrossmatchCompatible   bool                      `json:"major_minor_crossmatch_compatible"`
	InLine170MicronFilterAttached    bool                      `json:"in_line_170_micron_filter_attached"`
	LactatedRingersCoInfusionAvoided bool                      `json:"lactated_ringers_co_infusion_avoided"`
	RequiredTransfusionRegimen       string                    `json:"required_transfusion_regimen"`
	AlloantibodyCalciumWarningNote   string                    `json:"alloantibody_calcium_warning_note"`
	ProtocolSteps                    []TransfusionProtocolStep `json:"protocol_steps"`
	ClearanceToken                   string                    `json:"clearance_token,omitempty"`
	CreatedAt                        string                    `json:"created_at"`
	UpdatedAt                        string                    `json:"updated_at"`
}

// TransfusionEvaluationRequest represents input parameters for evaluating a Canine/Feline Xenotransfusion, DEA 1.1 / AB Blood Typing & Acute Hemolytic Transfusion Reaction triage operation.
type TransfusionEvaluationRequest struct {
	ID                               string                    `json:"id,omitempty"`
	PetID                            string                    `json:"pet_id"`
	PetName                          string                    `json:"pet_name"`
	Species                          string                    `json:"species"`
	WeightKg                         float64                   `json:"weight_kg"`
	BloodGroupType                   TransfusionBloodGroupType `json:"blood_group_type"`
	RecipientPCVPct                  float64                   `json:"recipient_pcv_pct"`
	DonorPCVPct                      float64                   `json:"donor_pcv_pct"`
	TargetPCVPct                     float64                   `json:"target_pcv_pct"`
	MajorMinorCrossmatchCompatible   bool                      `json:"major_minor_crossmatch_compatible"`
	InLine170MicronFilterAttached    bool                      `json:"in_line_170_micron_filter_attached"`
	LactatedRingersCoInfusionAvoided bool                      `json:"lactated_ringers_co_infusion_avoided"`
}

// TransfusionStepUpdateRequest updates a protocol step on an existing transfusion bloodbank triage operation.
type TransfusionStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on TransfusionBloodbankOperation.
func (op TransfusionBloodbankOperation) Validate() error {
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
	if op.WeightKg < 1.0 || op.WeightKg > 100.0 {
		return fmt.Errorf("weight_kg must be in [1.0, 100.0], got %v", op.WeightKg)
	}
	if !op.BloodGroupType.IsValid() {
		return fmt.Errorf("invalid blood_group_type: %q", op.BloodGroupType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.RecipientPCVPct < 4.0 || op.RecipientPCVPct > 45.0 {
		return fmt.Errorf("recipient_pcv_pct must be in [4.0, 45.0], got %v", op.RecipientPCVPct)
	}
	if op.DonorPCVPct < 30.0 || op.DonorPCVPct > 65.0 {
		return fmt.Errorf("donor_pcv_pct must be in [30.0, 65.0], got %v", op.DonorPCVPct)
	}
	if op.TargetPCVPct <= op.RecipientPCVPct || op.TargetPCVPct > 55.0 {
		return fmt.Errorf("target_pcv_pct must be > recipient_pcv_pct and <= 55.0, got %v", op.TargetPCVPct)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
