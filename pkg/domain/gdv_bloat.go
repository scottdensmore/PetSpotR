package domain

import (
	"fmt"
	"strings"
)

// GDVPresentationType classifies the gastric dilatation-volvulus (GDV / bloat), splenic torsion, or deep-chest aerophagia presentation.
type GDVPresentationType string

const (
	GDVPresentationAcuteGDV360PyloricTorsion        GDVPresentationType = "ACUTE_GDV_360_PYLORIC_TORSION"
	GDVPresentationPartialGDV180Volvulus            GDVPresentationType = "PARTIAL_GDV_180_VOLVULUS"
	GDVPresentationSimpleGastricDilatationFoodBloat GDVPresentationType = "SIMPLE_GASTRIC_DILATATION_FOOD_BLOAT"
	GDVPresentationSplenicTorsionHemoabdomen        GDVPresentationType = "SPLENIC_TORSION_HEMOABDOMEN"
	GDVPresentationPostExerciseDeepChestAerophagia  GDVPresentationType = "POST_EXERCISE_DEEP_CHEST_AEROPHAGIA"
)

// IsValid returns true when GDVPresentationType is a supported presentation enum value.
func (p GDVPresentationType) IsValid() bool {
	switch p {
	case GDVPresentationAcuteGDV360PyloricTorsion,
		GDVPresentationPartialGDV180Volvulus,
		GDVPresentationSimpleGastricDilatationFoodBloat,
		GDVPresentationSplenicTorsionHemoabdomen,
		GDVPresentationPostExerciseDeepChestAerophagia:
		return true
	default:
		return false
	}
}

// GDVTriageStatus represents the lifecycle state of a Gastric Dilatation-Volvulus (GDV / Bloat), Splenic Torsion & Trocar Decompression triage operation.
type GDVTriageStatus string

const (
	GDVStatusCriticalNecrosisVPCShock GDVTriageStatus = "GASTRIC_NECROSIS_VPC_SHOCK_CRITICAL"
	GDVStatusStagedTrocarDualIV       GDVTriageStatus = "TROCAR_DECOMPRESSED_DUAL_IV_STAGED"
	GDVStatusClearedGastropexySinus   GDVTriageStatus = "GASTROPEXY_SINUS_RHYTHM_CLEARED"
)

// IsValid returns true when GDVTriageStatus is a supported lifecycle state.
func (s GDVTriageStatus) IsValid() bool {
	switch s {
	case GDVStatusCriticalNecrosisVPCShock,
		GDVStatusStagedTrocarDualIV,
		GDVStatusClearedGastropexySinus:
		return true
	default:
		return false
	}
}

// GDVProtocolStep represents an actionable cranial cephalic IV bolus, trocar/orogastric decompression, right-lateral double-bubble radiograph, or lidocaine VPC/gastropexy step.
type GDVProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// GDVBloatOperation represents a Gastric Dilatation-Volvulus (GDV / Bloat), Splenic Torsion & Trocar Decompression triage operation.
type GDVBloatOperation struct {
	ID                           string              `json:"id"`
	PetID                        string              `json:"pet_id"`
	PetName                      string              `json:"pet_name"`
	Species                      string              `json:"species"`
	WeightKg                     float64             `json:"weight_kg"`
	PresentationType             GDVPresentationType `json:"presentation_type"`
	Status                       GDVTriageStatus     `json:"status"`
	PlasmaLactateMmolL           float64             `json:"plasma_lactate_mmol_l"`
	HeartRateBpm                 int                 `json:"heart_rate_bpm"`
	TympanicAbdominalGirthCm     float64             `json:"tympanic_abdominal_girth_cm"`
	NonProductiveRetchingPresent bool                `json:"non_productive_retching_present"`
	CranialCephalicDualIVAccess  bool                `json:"cranial_cephalic_dual_iv_access"`
	VentricularArrhythmiaVPCs    bool                `json:"ventricular_arrhythmia_vpcs"`
	RequiredDecompressionRegimen string              `json:"required_decompression_regimen"`
	CaudalVenaCavaWarningNote    string              `json:"caudal_vena_cava_warning_note"`
	ProtocolSteps                []GDVProtocolStep   `json:"protocol_steps"`
	ClearanceToken               string              `json:"clearance_token,omitempty"`
	CreatedAt                    string              `json:"created_at"`
	UpdatedAt                    string              `json:"updated_at"`
}

// GDVEvaluationRequest represents input parameters for evaluating a GDV / Bloat, Splenic Torsion & Trocar Decompression triage operation.
type GDVEvaluationRequest struct {
	ID                           string              `json:"id,omitempty"`
	PetID                        string              `json:"pet_id"`
	PetName                      string              `json:"pet_name"`
	Species                      string              `json:"species"`
	WeightKg                     float64             `json:"weight_kg"`
	PresentationType             GDVPresentationType `json:"presentation_type"`
	PlasmaLactateMmolL           float64             `json:"plasma_lactate_mmol_l"`
	HeartRateBpm                 int                 `json:"heart_rate_bpm"`
	TympanicAbdominalGirthCm     float64             `json:"tympanic_abdominal_girth_cm"`
	NonProductiveRetchingPresent bool                `json:"non_productive_retching_present"`
	CranialCephalicDualIVAccess  bool                `json:"cranial_cephalic_dual_iv_access"`
	VentricularArrhythmiaVPCs    bool                `json:"ventricular_arrhythmia_vpcs"`
}

// GDVStepUpdateRequest updates a protocol step on an existing GDV bloat triage operation.
type GDVStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on GDVBloatOperation.
func (op GDVBloatOperation) Validate() error {
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
	if op.WeightKg < 5.0 || op.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in [5.0, 120.0], got %v", op.WeightKg)
	}
	if !op.PresentationType.IsValid() {
		return fmt.Errorf("invalid presentation_type: %q", op.PresentationType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.PlasmaLactateMmolL < 0.5 || op.PlasmaLactateMmolL > 25.0 {
		return fmt.Errorf("plasma_lactate_mmol_l must be in [0.5, 25.0], got %v", op.PlasmaLactateMmolL)
	}
	if op.HeartRateBpm < 40 || op.HeartRateBpm > 280 {
		return fmt.Errorf("heart_rate_bpm must be in [40, 280], got %d", op.HeartRateBpm)
	}
	if op.TympanicAbdominalGirthCm < 30.0 || op.TympanicAbdominalGirthCm > 180.0 {
		return fmt.Errorf("tympanic_abdominal_girth_cm must be in [30.0, 180.0], got %v", op.TympanicAbdominalGirthCm)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
