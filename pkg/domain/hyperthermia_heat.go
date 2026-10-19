package domain

import (
	"fmt"
	"strings"
)

// HeatExposureType classifies the ambient hyperthermia, solar vehicle, or hot-asphalt thermal exposure vector.
type HeatExposureType string

const (
	HeatExposureEnclosedVehicleSolarCabin      HeatExposureType = "ENCLOSED_VEHICLE_SOLAR_CABIN"
	HeatExposureHotAsphaltPawNecrosis          HeatExposureType = "HOT_ASPHALT_PAW_NECROSIS"
	HeatExposureExertionalBrachycephalicStroke HeatExposureType = "EXERTIONAL_BRACHYCEPHALIC_HEATSTROKE"
	HeatExposureUnshadedYardTetherDome         HeatExposureType = "UNSHADED_YARD_TETHER_DOME"
	HeatExposureGarageAtticVentilationFailure  HeatExposureType = "GARAGE_ATTIC_VENTILATION_FAILURE"
)

// IsValid returns true when HeatExposureType is a supported thermal exposure enum value.
func (t HeatExposureType) IsValid() bool {
	switch t {
	case HeatExposureEnclosedVehicleSolarCabin,
		HeatExposureHotAsphaltPawNecrosis,
		HeatExposureExertionalBrachycephalicStroke,
		HeatExposureUnshadedYardTetherDome,
		HeatExposureGarageAtticVentilationFailure:
		return true
	default:
		return false
	}
}

// HeatTriageStatus represents the lifecycle state of a hyperthermia, heatstroke, or hot-asphalt pad burn triage operation.
type HeatTriageStatus string

const (
	HeatStatusCriticalDIC      HeatTriageStatus = "HEATSTROKE_DIC_COAGULOPATHY_CRITICAL"
	HeatStatusStagedCooling    HeatTriageStatus = "EVAPORATIVE_COOLING_394C_STAGED"
	HeatStatusStabilizedCooled HeatTriageStatus = "STABILIZED_NORMOTHERMIC_COOLED"
)

// IsValid returns true when HeatTriageStatus is a supported lifecycle state.
func (s HeatTriageStatus) IsValid() bool {
	switch s {
	case HeatStatusCriticalDIC,
		HeatStatusStagedCooling,
		HeatStatusStabilizedCooled:
		return true
	default:
		return false
	}
}

// HeatProtocolStep represents an actionable evaporative cooling, 39.4°C cutoff, paw-pad hydrogel debridement, or DIC coagulopathy IV protocol step.
type HeatProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// HeatTriageOperation represents a hyperthermia, heatstroke & hot-asphalt paw pad burn triage operation.
type HeatTriageOperation struct {
	ID                              string             `json:"id"`
	PetID                           string             `json:"pet_id"`
	PetName                         string             `json:"pet_name"`
	Species                         string             `json:"species"`
	WeightKg                        float64            `json:"weight_kg"`
	ExposureType                    HeatExposureType   `json:"exposure_type"`
	Status                          HeatTriageStatus   `json:"status"`
	RectalCoreTempCelsius           float64            `json:"rectal_core_temp_celsius"`
	SurfaceAsphaltTempCelsius       float64            `json:"surface_asphalt_temp_celsius"`
	ExposureDurationMins            int                `json:"exposure_duration_mins"`
	BrachycephalicAirwayObstruction bool               `json:"brachycephalic_airway_obstruction"`
	PawPadSloughingGrade            int                `json:"paw_pad_sloughing_grade"`
	CoolWaterConvectionFanActive    bool               `json:"cool_water_convection_fan_active"`
	IceBathVasoconstrictionAvoided  bool               `json:"ice_bath_vasoconstriction_avoided"`
	RequiredCoolingProtocol         string             `json:"required_cooling_protocol"`
	CoagulopathyCutoffNote          string             `json:"coagulopathy_cutoff_note"`
	ProtocolSteps                   []HeatProtocolStep `json:"protocol_steps"`
	ClearanceToken                  string             `json:"clearance_token,omitempty"`
	CreatedAt                       string             `json:"created_at"`
	UpdatedAt                       string             `json:"updated_at"`
}

// HeatEvaluationRequest represents input parameters for evaluating a hyperthermia / heatstroke & paw burn triage operation.
type HeatEvaluationRequest struct {
	ID                              string           `json:"id,omitempty"`
	PetID                           string           `json:"pet_id"`
	PetName                         string           `json:"pet_name"`
	Species                         string           `json:"species"`
	WeightKg                        float64          `json:"weight_kg"`
	ExposureType                    HeatExposureType `json:"exposure_type"`
	RectalCoreTempCelsius           float64          `json:"rectal_core_temp_celsius"`
	SurfaceAsphaltTempCelsius       float64          `json:"surface_asphalt_temp_celsius"`
	ExposureDurationMins            int              `json:"exposure_duration_mins"`
	BrachycephalicAirwayObstruction bool             `json:"brachycephalic_airway_obstruction"`
	PawPadSloughingGrade            int              `json:"paw_pad_sloughing_grade"`
	CoolWaterConvectionFanActive    bool             `json:"cool_water_convection_fan_active"`
	IceBathVasoconstrictionAvoided  bool             `json:"ice_bath_vasoconstriction_avoided"`
}

// HeatStepUpdateRequest updates a protocol step on an existing hyperthermia heat triage operation.
type HeatStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on HeatTriageOperation.
func (op HeatTriageOperation) Validate() error {
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
	if op.WeightKg <= 0 {
		return fmt.Errorf("weight_kg must be greater than 0, got %v", op.WeightKg)
	}
	if !op.ExposureType.IsValid() {
		return fmt.Errorf("invalid exposure_type: %q", op.ExposureType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.RectalCoreTempCelsius < 37.0 || op.RectalCoreTempCelsius > 45.0 {
		return fmt.Errorf("rectal_core_temp_celsius must be in [37.0, 45.0], got %v", op.RectalCoreTempCelsius)
	}
	if op.SurfaceAsphaltTempCelsius < 15.0 || op.SurfaceAsphaltTempCelsius > 95.0 {
		return fmt.Errorf("surface_asphalt_temp_celsius must be in [15.0, 95.0], got %v", op.SurfaceAsphaltTempCelsius)
	}
	if op.ExposureDurationMins <= 0 {
		return fmt.Errorf("exposure_duration_mins must be greater than 0, got %d", op.ExposureDurationMins)
	}
	if op.PawPadSloughingGrade < 0 || op.PawPadSloughingGrade > 4 {
		return fmt.Errorf("paw_pad_sloughing_grade must be in [0, 4], got %d", op.PawPadSloughingGrade)
	}
	if len(op.ProtocolSteps) != 4 {
		return fmt.Errorf("exactly 4 protocol_steps are required")
	}
	return nil
}
