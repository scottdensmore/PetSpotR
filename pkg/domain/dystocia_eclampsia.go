package domain

import (
	"fmt"
	"strings"
)

// DystociaPresentationType classifies the obstetric dystocia, maternal C-section, or puerperal eclampsia (hypocalcemia) presentation.
type DystociaPresentationType string

const (
	DystociaObstructiveBrachycephalicFetalOversize DystociaPresentationType = "OBSTRUCTIVE_BRACHYCEPHALIC_FETAL_OVERSIZE"
	DystociaPrimaryUterineInertiaNormocalcemic     DystociaPresentationType = "PRIMARY_UTERINE_INERTIA_NORMOCALCEMIC"
	DystociaPuerperalEclampsiaHypocalcemiaTetany   DystociaPresentationType = "PUERPERAL_ECLAMPSIA_HYPOCALCEMIA_TETANY"
	DystociaPlacentalDetachmentUteroverdin         DystociaPresentationType = "PLACENTAL_DETACHMENT_UTEROVERDIN_DISCHARGE"
	DystociaMalpresentationBreechTransverse        DystociaPresentationType = "MALPRESENTATION_BREECH_TRANSVERSE"
)

// IsValid returns true when DystociaPresentationType is a supported presentation enum value.
func (p DystociaPresentationType) IsValid() bool {
	switch p {
	case DystociaObstructiveBrachycephalicFetalOversize,
		DystociaPrimaryUterineInertiaNormocalcemic,
		DystociaPuerperalEclampsiaHypocalcemiaTetany,
		DystociaPlacentalDetachmentUteroverdin,
		DystociaMalpresentationBreechTransverse:
		return true
	default:
		return false
	}
}

// DystociaTriageStatus represents the lifecycle state of a dystocia, maternal C-section & eclampsia triage operation.
type DystociaTriageStatus string

const (
	DystociaStatusFetalDistressObstructiveCritical DystociaTriageStatus = "FETAL_DISTRESS_OBSTRUCTIVE_CRITICAL"
	DystociaStatusCalciumECGWhelpingStaged         DystociaTriageStatus = "CALCIUM_ECG_WHELPING_STAGED"
	DystociaStatusDeliveredNormocalcemicSafe       DystociaTriageStatus = "DELIVERED_NORMOCALCEMIC_LACTATING_SAFE"
)

// IsValid returns true when DystociaTriageStatus is a supported lifecycle state.
func (s DystociaTriageStatus) IsValid() bool {
	switch s {
	case DystociaStatusFetalDistressObstructiveCritical,
		DystociaStatusCalciumECGWhelpingStaged,
		DystociaStatusDeliveredNormocalcemicSafe:
		return true
	default:
		return false
	}
}

// DystociaProtocolStep represents an actionable Doppler fetal HR / pelvic radiograph, ECG-monitored calcium gluconate, oxytocin gate / C-section, or neonatal colostrum resuscitation step.
type DystociaProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// DystociaEclampsiaOperation represents a dystocia, maternal C-section, eclampsia (hypocalcemia) & oxytocin contraindication triage operation.
type DystociaEclampsiaOperation struct {
	ID                             string                   `json:"id"`
	PetID                          string                   `json:"pet_id"`
	PetName                        string                   `json:"pet_name"`
	Species                        string                   `json:"species"`
	WeightKg                       float64                  `json:"weight_kg"`
	PresentationType               DystociaPresentationType `json:"presentation_type"`
	Status                         DystociaTriageStatus     `json:"status"`
	LowestFetalHeartRateBpm        int                      `json:"lowest_fetal_heart_rate_bpm"`
	IonizedCalciumMmolL            float64                  `json:"ionized_calcium_mmol_l"`
	InterPuppyIntervalMins         int                      `json:"inter_puppy_interval_mins"`
	GreenBlackUteroverdinNoFetus   bool                     `json:"green_black_uteroverdin_no_fetus"`
	PelvicCanalObstructionExcluded bool                     `json:"pelvic_canal_obstruction_excluded"`
	ECGMonitoredCalciumInfusion    bool                     `json:"ecg_monitored_calcium_infusion"`
	RequiredObstetricProtocol      string                   `json:"required_obstetric_protocol"`
	OxytocinContraindicationNote   string                   `json:"oxytocin_contraindication_note"`
	ProtocolSteps                  []DystociaProtocolStep   `json:"protocol_steps"`
	ClearanceToken                 string                   `json:"clearance_token,omitempty"`
	CreatedAt                      string                   `json:"created_at"`
	UpdatedAt                      string                   `json:"updated_at"`
}

// DystociaEvaluationRequest represents input parameters for evaluating a dystocia, maternal C-section & eclampsia triage operation.
type DystociaEvaluationRequest struct {
	ID                             string                   `json:"id,omitempty"`
	PetID                          string                   `json:"pet_id"`
	PetName                        string                   `json:"pet_name"`
	Species                        string                   `json:"species"`
	WeightKg                       float64                  `json:"weight_kg"`
	PresentationType               DystociaPresentationType `json:"presentation_type"`
	LowestFetalHeartRateBpm        int                      `json:"lowest_fetal_heart_rate_bpm"`
	IonizedCalciumMmolL            float64                  `json:"ionized_calcium_mmol_l"`
	InterPuppyIntervalMins         int                      `json:"inter_puppy_interval_mins"`
	GreenBlackUteroverdinNoFetus   bool                     `json:"green_black_uteroverdin_no_fetus"`
	PelvicCanalObstructionExcluded bool                     `json:"pelvic_canal_obstruction_excluded"`
	ECGMonitoredCalciumInfusion    bool                     `json:"ecg_monitored_calcium_infusion"`
}

// DystociaStepUpdateRequest updates a protocol step on an existing dystocia & eclampsia triage operation.
type DystociaStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on DystociaEclampsiaOperation.
func (op DystociaEclampsiaOperation) Validate() error {
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
	if op.WeightKg < 1.5 || op.WeightKg > 90.0 {
		return fmt.Errorf("weight_kg must be in [1.5, 90.0], got %v", op.WeightKg)
	}
	if !op.PresentationType.IsValid() {
		return fmt.Errorf("invalid presentation_type: %q", op.PresentationType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.LowestFetalHeartRateBpm < 80 || op.LowestFetalHeartRateBpm > 280 {
		return fmt.Errorf("lowest_fetal_heart_rate_bpm must be in [80, 280], got %d", op.LowestFetalHeartRateBpm)
	}
	if op.IonizedCalciumMmolL < 0.4 || op.IonizedCalciumMmolL > 1.8 {
		return fmt.Errorf("ionized_calcium_mmol_l must be in [0.4, 1.8], got %v", op.IonizedCalciumMmolL)
	}
	if op.InterPuppyIntervalMins < 0 || op.InterPuppyIntervalMins > 480 {
		return fmt.Errorf("inter_puppy_interval_mins must be in [0, 480], got %d", op.InterPuppyIntervalMins)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
