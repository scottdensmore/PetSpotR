package domain

import (
	"fmt"
	"strings"
)

// AcousticTriggerType classifies the fireworks, thunderstorm barometric drop, gunshot/blasting, transformer arc, or siren startle trigger.
type AcousticTriggerType string

const (
	TriggerFireworksBarrage            AcousticTriggerType = "JULY4_NYE_FIREWORKS_BARRAGE"
	TriggerThunderstormBarometricDrop  AcousticTriggerType = "SEVERE_THUNDERSTORM_BAROMETRIC_DROP"
	TriggerGunshotConstructionBlasting AcousticTriggerType = "GUNSHOT_CONSTRUCTION_BLASTING"
	TriggerHighVoltageTransformerArc   AcousticTriggerType = "HIGH_VOLTAGE_TRANSFORMER_ARC"
	TriggerSirenSonicBoomStartle       AcousticTriggerType = "SIREN_SONIC_BOOM_STARTLE"
)

// IsValid returns true when AcousticTriggerType is a supported acoustic trigger enum value.
func (t AcousticTriggerType) IsValid() bool {
	switch t {
	case TriggerFireworksBarrage,
		TriggerThunderstormBarometricDrop,
		TriggerGunshotConstructionBlasting,
		TriggerHighVoltageTransformerArc,
		TriggerSirenSonicBoomStartle:
		return true
	default:
		return false
	}
}

// AcousticPanicStatus represents the lifecycle state of an acoustic panic, bolt-trajectory vectoring & calming pheromone SAR operation.
type AcousticPanicStatus string

const (
	AcousticStatusCriticalBolt     AcousticPanicStatus = "ADRENAL_FLIGHT_BOLT_CRITICAL"
	AcousticStatusStagedSniff      AcousticPanicStatus = "CALMING_PERIMETER_SNIFF_STAGED"
	AcousticStatusDecompressedSafe AcousticPanicStatus = "DECOMPRESSED_CONTAINED_SAFE"
)

// IsValid returns true when AcousticPanicStatus is a supported lifecycle state.
func (s AcousticPanicStatus) IsValid() bool {
	switch s {
	case AcousticStatusCriticalBolt,
		AcousticStatusStagedSniff,
		AcousticStatusDecompressedSafe:
		return true
	default:
		return false
	}
}

// AcousticProtocolStep represents an actionable silent no-chase perimeter, scent/pheromone refuge lure, barrier laceration check, or acoustic decompression protocol step.
type AcousticProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// AcousticPanicOperation represents a fireworks/thunderstorm acoustic panic, bolt-trajectory vectoring & calming pheromone SAR operation.
type AcousticPanicOperation struct {
	ID                           string                 `json:"id"`
	PetID                        string                 `json:"pet_id"`
	PetName                      string                 `json:"pet_name"`
	Species                      string                 `json:"species"`
	WeightKg                     float64                `json:"weight_kg"`
	TriggerType                  AcousticTriggerType    `json:"trigger_type"`
	Status                       AcousticPanicStatus    `json:"status"`
	PeakDecibelsDb               float64                `json:"peak_decibels_db"`
	BarometricDropHpa            float64                `json:"barometric_drop_hpa"`
	BoltHeadingDegrees           int                    `json:"bolt_heading_degrees"`
	BarrierBreachInjurySuspected bool                   `json:"barrier_breach_injury_suspected"`
	NoChasePassiveLureEnforced   bool                   `json:"no_chase_passive_lure_enforced"`
	AdaptilFeliwayStationStaged  bool                   `json:"adaptil_feliway_station_staged"`
	PredictedRefugeCorridor      string                 `json:"predicted_refuge_corridor"`
	AdrenalNoChaseAdvisory       string                 `json:"adrenal_no_chase_advisory"`
	ProtocolSteps                []AcousticProtocolStep `json:"protocol_steps"`
	ClearanceToken               string                 `json:"clearance_token,omitempty"`
	CreatedAt                    string                 `json:"created_at"`
	UpdatedAt                    string                 `json:"updated_at"`
}

// AcousticEvaluationRequest represents input parameters for evaluating an acoustic panic & bolt-trajectory vectoring operation.
type AcousticEvaluationRequest struct {
	ID                           string              `json:"id,omitempty"`
	PetID                        string              `json:"pet_id"`
	PetName                      string              `json:"pet_name"`
	Species                      string              `json:"species"`
	WeightKg                     float64             `json:"weight_kg"`
	TriggerType                  AcousticTriggerType `json:"trigger_type"`
	PeakDecibelsDb               float64             `json:"peak_decibels_db"`
	BarometricDropHpa            float64             `json:"barometric_drop_hpa"`
	BoltHeadingDegrees           int                 `json:"bolt_heading_degrees"`
	BarrierBreachInjurySuspected bool                `json:"barrier_breach_injury_suspected"`
	NoChasePassiveLureEnforced   bool                `json:"no_chase_passive_lure_enforced"`
	AdaptilFeliwayStationStaged  bool                `json:"adaptil_feliway_station_staged"`
}

// AcousticStepUpdateRequest updates a protocol step on an existing acoustic panic operation.
type AcousticStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on AcousticPanicOperation.
func (op AcousticPanicOperation) Validate() error {
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
	if !op.TriggerType.IsValid() {
		return fmt.Errorf("invalid trigger_type: %q", op.TriggerType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.PeakDecibelsDb < 60.0 || op.PeakDecibelsDb > 180.0 {
		return fmt.Errorf("peak_decibels_db must be in [60.0, 180.0], got %v", op.PeakDecibelsDb)
	}
	if op.BarometricDropHpa < 0.0 || op.BarometricDropHpa > 60.0 {
		return fmt.Errorf("barometric_drop_hpa must be in [0.0, 60.0], got %v", op.BarometricDropHpa)
	}
	if op.BoltHeadingDegrees < 0 || op.BoltHeadingDegrees > 359 {
		return fmt.Errorf("bolt_heading_degrees must be in [0, 359], got %d", op.BoltHeadingDegrees)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
