package domain

import (
	"fmt"
	"strings"
)

// NeonatalLitterType classifies the neonatal orphan litter category and clinical physiology profile.
type NeonatalLitterType string

const (
	NeonatalLitterUnweanedKitten0To2Wk             NeonatalLitterType = "UNWEANED_KITTEN_LITTER_0_2WK"
	NeonatalLitterTransitionalKitten2To4Wk         NeonatalLitterType = "TRANSITIONAL_KITTEN_LITTER_2_4WK"
	NeonatalLitterPuppyCleft                       NeonatalLitterType = "NEONATAL_PUPPY_LITTER_CLEFT"
	NeonatalLitterSingletonOrphanColostrumDeprived NeonatalLitterType = "SINGLETON_ORPHAN_COLOSTRUM_DEPRIVED"
	NeonatalLitterMaternalAgalactiaRejected        NeonatalLitterType = "MATERNAL_AGALACTIA_REJECTED"
)

// IsValid returns true when NeonatalLitterType is a supported enum value.
func (t NeonatalLitterType) IsValid() bool {
	switch t {
	case NeonatalLitterUnweanedKitten0To2Wk,
		NeonatalLitterTransitionalKitten2To4Wk,
		NeonatalLitterPuppyCleft,
		NeonatalLitterSingletonOrphanColostrumDeprived,
		NeonatalLitterMaternalAgalactiaRejected:
		return true
	default:
		return false
	}
}

// NeonatalTriageStatus represents the clinical triage lifecycle state of a neonatal litter operation.
type NeonatalTriageStatus string

const (
	NeonatalTriageStatusFadingTriadHypothermicCritical NeonatalTriageStatus = "FADING_TRIAD_HYPOTHERMIC_CRITICAL"
	NeonatalTriageStatusIncubatorWarmedDextroseStaged  NeonatalTriageStatus = "INCUBATOR_WARMED_DEXTROSE_STAGED"
	NeonatalTriageStatusStabilizedNursingGainingSafe   NeonatalTriageStatus = "STABILIZED_NURSING_GAINING_SAFE"
)

// IsValid returns true when NeonatalTriageStatus is a supported lifecycle state.
func (s NeonatalTriageStatus) IsValid() bool {
	switch s {
	case NeonatalTriageStatusFadingTriadHypothermicCritical,
		NeonatalTriageStatusIncubatorWarmedDextroseStaged,
		NeonatalTriageStatusStabilizedNursingGainingSafe:
		return true
	default:
		return false
	}
}

// NeonatalProtocolStep represents an actionable incubator warming, sublingual dextrose, gavage, or stimulation checklist step.
type NeonatalProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// NeonatalTriageOperation represents a neonatal orphan litter and incubator critical care triage operation.
type NeonatalTriageOperation struct {
	ID                        string                 `json:"id"`
	PetID                     string                 `json:"pet_id"`
	PetName                   string                 `json:"pet_name"`
	Species                   string                 `json:"species"`
	WeightGrams               float64                `json:"weight_grams"`
	LitterCount               int                    `json:"litter_count"`
	AgeDays                   int                    `json:"age_days"`
	LitterType                NeonatalLitterType     `json:"litter_type"`
	Status                    NeonatalTriageStatus   `json:"status"`
	NeonateRectalTempCelsius  float64                `json:"neonate_rectal_temp_celsius"`
	BloodGlucoseMgDl          float64                `json:"blood_glucose_mg_dl"`
	IncubatorHumidityPct      float64                `json:"incubator_humidity_pct"`
	SucklingReflexPresent     bool                   `json:"suckling_reflex_present"`
	WarmedBeforeFeedingGate   bool                   `json:"warmed_before_feeding_gate"`
	SublingualDextroseApplied bool                   `json:"sublingual_dextrose_applied"`
	RequiredIncubatorProtocol string                 `json:"required_incubator_protocol"`
	IleusWarmingGateNote      string                 `json:"ileus_warming_gate_note"`
	ProtocolSteps             []NeonatalProtocolStep `json:"protocol_steps"`
	ClearanceToken            string                 `json:"clearance_token,omitempty"`
	CreatedAt                 string                 `json:"created_at"`
	UpdatedAt                 string                 `json:"updated_at"`
}

// NeonatalEvaluationRequest represents input parameters for evaluating a neonatal incubator triage operation.
type NeonatalEvaluationRequest struct {
	ID                        string             `json:"id,omitempty"`
	PetID                     string             `json:"pet_id,omitempty"`
	PetName                   string             `json:"pet_name"`
	Species                   string             `json:"species"`
	WeightGrams               float64            `json:"weight_grams"`
	LitterCount               int                `json:"litter_count"`
	AgeDays                   int                `json:"age_days"`
	LitterType                NeonatalLitterType `json:"litter_type"`
	NeonateRectalTempCelsius  float64            `json:"neonate_rectal_temp_celsius"`
	BloodGlucoseMgDl          float64            `json:"blood_glucose_mg_dl"`
	IncubatorHumidityPct      float64            `json:"incubator_humidity_pct"`
	SucklingReflexPresent     bool               `json:"suckling_reflex_present"`
	WarmedBeforeFeedingGate   bool               `json:"warmed_before_feeding_gate"`
	SublingualDextroseApplied bool               `json:"sublingual_dextrose_applied"`
}

// NeonatalStepUpdateRequest updates a protocol step on an existing neonatal triage operation.
type NeonatalStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on NeonatalTriageOperation.
func (op NeonatalTriageOperation) Validate() error {
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
	if op.WeightGrams < 30.0 || op.WeightGrams > 5000.0 {
		return fmt.Errorf("weight_grams must be in [30.0, 5000.0], got %v", op.WeightGrams)
	}
	if op.LitterCount < 1 || op.LitterCount > 16 {
		return fmt.Errorf("litter_count must be in [1, 16], got %d", op.LitterCount)
	}
	if op.AgeDays < 0 || op.AgeDays > 42 {
		return fmt.Errorf("age_days must be in [0, 42], got %d", op.AgeDays)
	}
	if !op.LitterType.IsValid() {
		return fmt.Errorf("invalid litter_type: %q", op.LitterType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.NeonateRectalTempCelsius < 28.0 || op.NeonateRectalTempCelsius > 40.5 {
		return fmt.Errorf("neonate_rectal_temp_celsius must be in [28.0, 40.5], got %v", op.NeonateRectalTempCelsius)
	}
	if op.BloodGlucoseMgDl < 15.0 || op.BloodGlucoseMgDl > 300.0 {
		return fmt.Errorf("blood_glucose_mg_dl must be in [15.0, 300.0], got %v", op.BloodGlucoseMgDl)
	}
	if op.IncubatorHumidityPct < 20.0 || op.IncubatorHumidityPct > 95.0 {
		return fmt.Errorf("incubator_humidity_pct must be in [20.0, 95.0], got %v", op.IncubatorHumidityPct)
	}
	if strings.TrimSpace(op.RequiredIncubatorProtocol) == "" {
		return fmt.Errorf("required_incubator_protocol is required")
	}
	if strings.TrimSpace(op.IleusWarmingGateNote) == "" {
		return fmt.Errorf("ileus_warming_gate_note is required")
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("expected 4 protocol_steps, got %d", len(op.ProtocolSteps))
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.ID) == "" || strings.TrimSpace(step.Title) == "" || strings.TrimSpace(step.Description) == "" {
			return fmt.Errorf("protocol_steps[%d] must have non-empty id, title, and description", i)
		}
	}
	if op.Status == NeonatalTriageStatusStabilizedNursingGainingSafe && strings.TrimSpace(op.ClearanceToken) == "" {
		return fmt.Errorf("clearance_token is required when status is STABILIZED_NURSING_GAINING_SAFE")
	}
	if strings.TrimSpace(op.CreatedAt) == "" || strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("created_at and updated_at are required")
	}
	return nil
}
