package domain

import (
	"fmt"
	"strings"
)

// COExposureSourceType classifies the carbon monoxide (CO), cyanide smoke, or enclosed exhaust asphyxiation source.
type COExposureSourceType string

const (
	COExposureStructuralHouseFireSynthetics COExposureSourceType = "STRUCTURAL_HOUSE_FIRE_SYNTHETICS"
	COExposureEnclosedGarageExhaustCO       COExposureSourceType = "ENCLOSED_GARAGE_EXHAUST_CO"
	COExposurePortableGeneratorCabinLeak    COExposureSourceType = "PORTABLE_GENERATOR_CABIN_LEAK"
	COExposureSmolderingBarnTorfSmoke       COExposureSourceType = "SMOLDERING_BARN_TORF_SMOKE"
	COExposurePropaneHeaterRVAsphyxia       COExposureSourceType = "PROPANE_HEATER_RV_ASPHYXIA"
)

// IsValid returns true when COExposureSourceType is a supported exposure source enum value.
func (s COExposureSourceType) IsValid() bool {
	switch s {
	case COExposureStructuralHouseFireSynthetics,
		COExposureEnclosedGarageExhaustCO,
		COExposurePortableGeneratorCabinLeak,
		COExposureSmolderingBarnTorfSmoke,
		COExposurePropaneHeaterRVAsphyxia:
		return true
	default:
		return false
	}
}

// HyperbaricCOTriageStatus represents the lifecycle state of a hyperbaric oxygen (HBOT), carboxyhemoglobin & cyanide smoke triage operation.
type HyperbaricCOTriageStatus string

const (
	HyperbaricStatusCriticalCOHbCyanide        HyperbaricCOTriageStatus = "CARBOXYHEMOGLOBIN_CYANIDE_CRITICAL"
	HyperbaricStatusStagedHBOTHydroxocobalamin HyperbaricCOTriageStatus = "HBOT_CHAMBER_HYDROXOCOBALAMIN_STAGED"
	HyperbaricStatusClearedSafeNormoxic        HyperbaricCOTriageStatus = "NORMOXIC_COHB_CLEARED_SAFE"
)

// IsValid returns true when HyperbaricCOTriageStatus is a supported lifecycle state.
func (s HyperbaricCOTriageStatus) IsValid() bool {
	switch s {
	case HyperbaricStatusCriticalCOHbCyanide,
		HyperbaricStatusStagedHBOTHydroxocobalamin,
		HyperbaricStatusClearedSafeNormoxic:
		return true
	default:
		return false
	}
}

// HyperbaricProtocolStep represents an actionable CO-oximetry, hydroxocobalamin cyanide binding, HBOT chamber washout, or delayed neuro sequelae step.
type HyperbaricProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// HyperbaricCOOperation represents a hyperbaric oxygen (HBOT), carbon monoxide carboxyhemoglobin & cyanide smoke triage operation.
type HyperbaricCOOperation struct {
	ID                            string                   `json:"id"`
	PetID                         string                   `json:"pet_id"`
	PetName                       string                   `json:"pet_name"`
	Species                       string                   `json:"species"`
	WeightKg                      float64                  `json:"weight_kg"`
	ExposureSource                COExposureSourceType     `json:"exposure_source"`
	Status                        HyperbaricCOTriageStatus `json:"status"`
	CarboxyhemoglobinPct          float64                  `json:"carboxyhemoglobin_pct"`
	BloodLactateMmolL             float64                  `json:"blood_lactate_mmol_l"`
	ChamberPressureAta            float64                  `json:"chamber_pressure_ata"`
	SingeVibrissaeSootNares       bool                     `json:"singe_vibrissae_soot_nares"`
	PulseOximetryFalseNormalNoted bool                     `json:"pulse_oximetry_false_normal_noted"`
	HydroxocobalaminAdministered  bool                     `json:"hydroxocobalamin_administered"`
	RequiredHBOTRegimen           string                   `json:"required_hbot_regimen"`
	CoOximetryWarningNote         string                   `json:"co_oximetry_warning_note"`
	ProtocolSteps                 []HyperbaricProtocolStep `json:"protocol_steps"`
	ClearanceToken                string                   `json:"clearance_token,omitempty"`
	CreatedAt                     string                   `json:"created_at"`
	UpdatedAt                     string                   `json:"updated_at"`
}

// HyperbaricEvaluationRequest represents input parameters for evaluating a hyperbaric oxygen (HBOT) & CO/cyanide smoke triage operation.
type HyperbaricEvaluationRequest struct {
	ID                            string               `json:"id,omitempty"`
	PetID                         string               `json:"pet_id"`
	PetName                       string               `json:"pet_name"`
	Species                       string               `json:"species"`
	WeightKg                      float64              `json:"weight_kg"`
	ExposureSource                COExposureSourceType `json:"exposure_source"`
	CarboxyhemoglobinPct          float64              `json:"carboxyhemoglobin_pct"`
	BloodLactateMmolL             float64              `json:"blood_lactate_mmol_l"`
	ChamberPressureAta            float64              `json:"chamber_pressure_ata"`
	SingeVibrissaeSootNares       bool                 `json:"singe_vibrissae_soot_nares"`
	PulseOximetryFalseNormalNoted bool                 `json:"pulse_oximetry_false_normal_noted"`
	HydroxocobalaminAdministered  bool                 `json:"hydroxocobalamin_administered"`
}

// HyperbaricStepUpdateRequest updates a protocol step on an existing hyperbaric CO triage operation.
type HyperbaricStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on HyperbaricCOOperation.
func (op HyperbaricCOOperation) Validate() error {
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
	if !op.ExposureSource.IsValid() {
		return fmt.Errorf("invalid exposure_source: %q", op.ExposureSource)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.CarboxyhemoglobinPct < 0.0 || op.CarboxyhemoglobinPct > 85.0 {
		return fmt.Errorf("carboxyhemoglobin_pct must be in [0.0, 85.0], got %v", op.CarboxyhemoglobinPct)
	}
	if op.BloodLactateMmolL < 0.5 || op.BloodLactateMmolL > 30.0 {
		return fmt.Errorf("blood_lactate_mmol_l must be in [0.5, 30.0], got %v", op.BloodLactateMmolL)
	}
	if op.ChamberPressureAta < 1.0 || op.ChamberPressureAta > 3.0 {
		return fmt.Errorf("chamber_pressure_ata must be in [1.0, 3.0], got %v", op.ChamberPressureAta)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
