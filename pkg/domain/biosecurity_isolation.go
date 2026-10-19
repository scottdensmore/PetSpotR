package domain

import (
	"fmt"
	"strings"
)

// BiosecurityPathogenType classifies the contagious viral or bacterial pathogen requiring cohort biosecurity isolation.
type BiosecurityPathogenType string

const (
	BiosecurityPathogenCanineParvovirusCPV2Enteritis BiosecurityPathogenType = "CANINE_PARVOVIRUS_CPV2_ENTERITIS"
	BiosecurityPathogenFelinePanleukopeniaFPV        BiosecurityPathogenType = "FELINE_PANLEUKOPENIA_FPV"
	BiosecurityPathogenCanineDistemperCDVNeuro       BiosecurityPathogenType = "CANINE_DISTEMPER_CDV_NEURO"
	BiosecurityPathogenLeptospirosisZoonoticRenal    BiosecurityPathogenType = "LEPTOSPIROSIS_ZOONOTIC_RENAL"
	BiosecurityPathogenStrepZooepidemicusHemorrhagic BiosecurityPathogenType = "STREP_ZOOEPIDEMICUS_HEMORRHAGIC"
)

// IsValid returns true when BiosecurityPathogenType is a supported pathogen enum value.
func (p BiosecurityPathogenType) IsValid() bool {
	switch p {
	case BiosecurityPathogenCanineParvovirusCPV2Enteritis,
		BiosecurityPathogenFelinePanleukopeniaFPV,
		BiosecurityPathogenCanineDistemperCDVNeuro,
		BiosecurityPathogenLeptospirosisZoonoticRenal,
		BiosecurityPathogenStrepZooepidemicusHemorrhagic:
		return true
	default:
		return false
	}
}

// BiosecurityTriageStatus represents the lifecycle state of a biosecurity isolation cohort triage operation.
type BiosecurityTriageStatus string

const (
	BiosecurityStatusCriticalNeutropenicSepsisBreach BiosecurityTriageStatus = "NEUTROPENIC_SEPSIS_BREACH_CRITICAL"
	BiosecurityStatusStagedNegativePressureIsoCohort BiosecurityTriageStatus = "NEGATIVE_PRESSURE_ISO_COHORT_STAGED"
	BiosecurityStatusClearedSafePCRNegativeDecon     BiosecurityTriageStatus = "PCR_NEGATIVE_DECON_CLEARED"
)

// IsValid returns true when BiosecurityTriageStatus is a supported lifecycle state.
func (s BiosecurityTriageStatus) IsValid() bool {
	switch s {
	case BiosecurityStatusCriticalNeutropenicSepsisBreach,
		BiosecurityStatusStagedNegativePressureIsoCohort,
		BiosecurityStatusClearedSafePCRNegativeDecon:
		return true
	default:
		return false
	}
}

// BiosecurityProtocolStep represents an actionable anteroom PPE/footbath lock, AHP/Virkon 10-min decon, nasojejunal/maropitant sepsis care, or PCR shedding clearance step.
type BiosecurityProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// BiosecurityIsolationOperation represents a canine parvovirus, panleukopenia, distemper & biosecurity isolation cohort triage operation.
type BiosecurityIsolationOperation struct {
	ID                                 string                    `json:"id"`
	PetID                              string                    `json:"pet_id"`
	PetName                            string                    `json:"pet_name"`
	Species                            string                    `json:"species"`
	WeightKg                           float64                   `json:"weight_kg"`
	PathogenType                       BiosecurityPathogenType   `json:"pathogen_type"`
	Status                             BiosecurityTriageStatus   `json:"status"`
	NeutrophilCountCellsUl             int                       `json:"neutrophil_count_cells_ul"`
	RoomPressureDifferentialPa         float64                   `json:"room_pressure_differential_pa"`
	DisinfectantContactMins            int                       `json:"disinfectant_contact_mins"`
	HemorrhagicDiarrheaPresent         bool                      `json:"hemorrhagic_diarrhea_present"`
	PotassiumPeroxymonosulfateFootbath bool                      `json:"potassium_peroxymonosulfate_footbath"`
	DedicatedPPEGownGlovesEnforced     bool                      `json:"dedicated_ppe_gown_gloves_enforced"`
	RequiredIsolationRegimen           string                    `json:"required_isolation_regimen"`
	QuatDisinfectantFailureNote        string                    `json:"quat_disinfectant_failure_note"`
	ProtocolSteps                      []BiosecurityProtocolStep `json:"protocol_steps"`
	ClearanceToken                     string                    `json:"clearance_token,omitempty"`
	CreatedAt                          string                    `json:"created_at"`
	UpdatedAt                          string                    `json:"updated_at"`
}

// BiosecurityEvaluationRequest represents input parameters for evaluating a biosecurity isolation cohort triage operation.
type BiosecurityEvaluationRequest struct {
	ID                                 string                  `json:"id,omitempty"`
	PetID                              string                  `json:"pet_id"`
	PetName                            string                  `json:"pet_name"`
	Species                            string                  `json:"species"`
	WeightKg                           float64                 `json:"weight_kg"`
	PathogenType                       BiosecurityPathogenType `json:"pathogen_type"`
	NeutrophilCountCellsUl             int                     `json:"neutrophil_count_cells_ul"`
	RoomPressureDifferentialPa         float64                 `json:"room_pressure_differential_pa"`
	DisinfectantContactMins            int                     `json:"disinfectant_contact_mins"`
	HemorrhagicDiarrheaPresent         bool                    `json:"hemorrhagic_diarrhea_present"`
	PotassiumPeroxymonosulfateFootbath bool                    `json:"potassium_peroxymonosulfate_footbath"`
	DedicatedPPEGownGlovesEnforced     bool                    `json:"dedicated_ppe_gown_gloves_enforced"`
}

// BiosecurityStepUpdateRequest updates a protocol step on an existing biosecurity isolation operation.
type BiosecurityStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on BiosecurityIsolationOperation.
func (op BiosecurityIsolationOperation) Validate() error {
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
	if op.WeightKg <= 0 || op.WeightKg > 100.0 {
		return fmt.Errorf("weight_kg must be in (0, 100.0], got %v", op.WeightKg)
	}
	if !op.PathogenType.IsValid() {
		return fmt.Errorf("invalid pathogen_type: %q", op.PathogenType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.NeutrophilCountCellsUl < 50 || op.NeutrophilCountCellsUl > 40000 {
		return fmt.Errorf("neutrophil_count_cells_ul must be in [50, 40000], got %d", op.NeutrophilCountCellsUl)
	}
	if op.RoomPressureDifferentialPa < -50.0 || op.RoomPressureDifferentialPa > 20.0 {
		return fmt.Errorf("room_pressure_differential_pa must be in [-50.0, 20.0], got %v", op.RoomPressureDifferentialPa)
	}
	if op.DisinfectantContactMins < 1 || op.DisinfectantContactMins > 60 {
		return fmt.Errorf("disinfectant_contact_mins must be in [1, 60], got %d", op.DisinfectantContactMins)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
