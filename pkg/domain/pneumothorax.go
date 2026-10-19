package domain

import (
	"fmt"
	"strings"
	"time"
)

// PneumothoraxStatus represents the lifecycle state of a Tension Pneumothorax,
// Flail Chest, 7th–9th ICS Thoracocentesis & Heimlich/Pleural Seal Chest Tube operation.
type PneumothoraxStatus string

const (
	PneumothoraxStatusTensionAbsentGlideCritical PneumothoraxStatus = "TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL"
	PneumothoraxStatusThoracocentesisTubeStaged  PneumothoraxStatus = "ICS_7_9_THORACOCENTESIS_CHEST_TUBE_STAGED"
	PneumothoraxStatusPleuralSealTubeRemoved     PneumothoraxStatus = "PLEURAL_SEAL_GLIDE_RESTORED_TUBE_REMOVED"
)

// IsValid returns true when PneumothoraxStatus is a supported lifecycle state.
func (s PneumothoraxStatus) IsValid() bool {
	switch s {
	case PneumothoraxStatusTensionAbsentGlideCritical,
		PneumothoraxStatusThoracocentesisTubeStaged,
		PneumothoraxStatusPleuralSealTubeRemoved:
		return true
	default:
		return false
	}
}

const (
	PneumothoraxSpeciesCanineSpontaneousBullae     = "CANINE_SPONTANEOUS_BULLAE"
	PneumothoraxSpeciesCanineTraumaticFlailChest   = "CANINE_TRAUMATIC_FLAIL_CHEST"
	PneumothoraxSpeciesFelineTraumaticPneumothorax = "FELINE_TRAUMATIC_PNEUMOTHORAX"
)

// IsValidPneumothoraxSpecies returns true if species is a recognized pneumothorax / pleural space classification.
func IsValidPneumothoraxSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case PneumothoraxSpeciesCanineSpontaneousBullae,
		PneumothoraxSpeciesCanineTraumaticFlailChest,
		PneumothoraxSpeciesFelineTraumaticPneumothorax:
		return true
	default:
		return false
	}
}

// PneumothoraxPatientProfile captures signalment, respiratory rate, SpO2, TFAST glide sign,
// cumulative air aspirated (mL/kg), 24-hour tap count, and open sucking chest wound status.
type PneumothoraxPatientProfile struct {
	Species                    string  `json:"species"`
	Breed                      string  `json:"breed,omitempty"`
	WeightKg                   float64 `json:"weight_kg"`
	RespiratoryRateBpm         int     `json:"respiratory_rate_bpm"`
	SpO2Percent                float64 `json:"spo2_percent"`
	TfastGlideSignPresent      bool    `json:"tfast_glide_sign_present"`
	CumulativeAirAspiratedMlKg float64 `json:"cumulative_air_aspirated_ml_kg"`
	TapCountLast24Hr           int     `json:"tap_count_last_24hr"`
	OpenSuckingChestWound      bool    `json:"open_sucking_chest_wound"`
}

// Validate enforces clinical invariants on PneumothoraxPatientProfile.
func (p PneumothoraxPatientProfile) Validate() error {
	if !IsValidPneumothoraxSpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_SPONTANEOUS_BULLAE, CANINE_TRAUMATIC_FLAIL_CHEST, or FELINE_TRAUMATIC_PNEUMOTHORAX, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.RespiratoryRateBpm < 6 || p.RespiratoryRateBpm > 180 {
		return fmt.Errorf("respiratory_rate_bpm must be in [6, 180], got %d", p.RespiratoryRateBpm)
	}
	if p.SpO2Percent < 50.0 || p.SpO2Percent > 100.0 {
		return fmt.Errorf("spo2_percent must be in [50.0, 100.0], got %v", p.SpO2Percent)
	}
	if p.CumulativeAirAspiratedMlKg < 0.0 || p.CumulativeAirAspiratedMlKg > 500.0 {
		return fmt.Errorf("cumulative_air_aspirated_ml_kg must be in [0.0, 500.0], got %v", p.CumulativeAirAspiratedMlKg)
	}
	if p.TapCountLast24Hr < 0 || p.TapCountLast24Hr > 30 {
		return fmt.Errorf("tap_count_last_24hr must be in [0, 30], got %d", p.TapCountLast24Hr)
	}
	return nil
}

// PneumothoraxDecompressionPlan captures 7th-9th ICS dorsal third needle thoracocentesis landmark,
// cranial rib margin intercostal vessel guard, expected tap volume, thoracostomy chest tube indication,
// continuous pleural suction (-15 cmH2O), autologous blood patch pleurodesis dose, and pitfall guard.
type PneumothoraxDecompressionPlan struct {
	NeedleLandmarkIntercostalSpace      string  `json:"needle_landmark_intercostal_space"`
	CranialRibMarginVesselGuard         bool    `json:"cranial_rib_margin_vessel_guard"`
	ExpectedNegativePressureTapMl       float64 `json:"expected_negative_pressure_tap_ml"`
	ThoracostomyTubeIndicated           bool    `json:"thoracostomy_tube_indicated"`
	ContinuousSuctionCmH2O              int     `json:"continuous_suction_cm_h2o"`
	AutologousBloodPatchMlKg            float64 `json:"autologous_blood_patch_ml_kg"`
	ZeroPreDecompressionRadiographGuard bool    `json:"zero_pre_decompression_radiograph_guard"`
	PneumothoraxPitfallAlert            string  `json:"pneumothorax_pitfall_alert"`
	ClinicalRationale                   string  `json:"clinical_rationale"`
}

// PleuralDrainageSerialCheck records a serial pleural air/fluid drainage rate,
// TFAST glide sign status, SpO2, and subcutaneous emphysema check.
type PleuralDrainageSerialCheck struct {
	Timestamp                    time.Time `json:"timestamp"`
	AirAspiratedRateMlKgHr       float64   `json:"air_aspirated_rate_ml_kg_hr"`
	FluidAspiratedRateMlKgDay    float64   `json:"fluid_aspirated_rate_ml_kg_day"`
	TfastGlideSignRestored       bool      `json:"tfast_glide_sign_restored"`
	SpO2Percent                  float64   `json:"spo2_percent"`
	SubcutaneousEmphysemaPresent bool      `json:"subcutaneous_emphysema_present"`
	ClinicianCallsign            string    `json:"clinician_callsign"`
	Notes                        string    `json:"notes"`
}

// Validate enforces clinical invariants on PleuralDrainageSerialCheck.
func (c PleuralDrainageSerialCheck) Validate() error {
	if c.AirAspiratedRateMlKgHr < 0.0 || c.AirAspiratedRateMlKgHr > 200.0 {
		return fmt.Errorf("air_aspirated_rate_ml_kg_hr must be in [0.0, 200.0], got %v", c.AirAspiratedRateMlKgHr)
	}
	if c.FluidAspiratedRateMlKgDay < 0.0 || c.FluidAspiratedRateMlKgDay > 100.0 {
		return fmt.Errorf("fluid_aspirated_rate_ml_kg_day must be in [0.0, 100.0], got %v", c.FluidAspiratedRateMlKgDay)
	}
	if c.SpO2Percent < 50.0 || c.SpO2Percent > 100.0 {
		return fmt.Errorf("spo2_percent must be in [50.0, 100.0], got %v", c.SpO2Percent)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// PneumothoraxRemovalClearance certifies >= 24h zero air production, fluid output <= 2.0 mL/kg/day,
// bilateral TFAST glide sign confirmation, room-air SpO2 >= 95%, and aseptic chest tube removal.
type PneumothoraxRemovalClearance struct {
	ClearedAt                   time.Time `json:"cleared_at"`
	ZeroAirProductionHours      int       `json:"zero_air_production_hours"`
	FinalFluidOutputMlKgDay     float64   `json:"final_fluid_output_ml_kg_day"`
	BilateralGlideSignConfirmed bool      `json:"bilateral_glide_sign_confirmed"`
	RoomAirSpO2Percent          float64   `json:"room_air_spo2_percent"`
	ChestTubeAsepticallyRemoved bool      `json:"chest_tube_aseptically_removed"`
	VetLicense                  string    `json:"vet_license"`
	Summary                     string    `json:"summary"`
}

// Validate enforces strict chest tube removal & pleural seal safety criteria on PneumothoraxRemovalClearance.
func (r PneumothoraxRemovalClearance) Validate() error {
	if r.ZeroAirProductionHours < 24 || r.ZeroAirProductionHours > 240 {
		return fmt.Errorf("zero_air_production_hours must be >= 24 (in [24, 240]), got %d", r.ZeroAirProductionHours)
	}
	if r.FinalFluidOutputMlKgDay < 0.0 || r.FinalFluidOutputMlKgDay > 2.0 {
		return fmt.Errorf("final_fluid_output_ml_kg_day must be in [0.0, 2.0], got %v", r.FinalFluidOutputMlKgDay)
	}
	if !r.BilateralGlideSignConfirmed {
		return fmt.Errorf("bilateral_glide_sign_confirmed must be true for chest tube removal clearance")
	}
	if r.RoomAirSpO2Percent < 95.0 || r.RoomAirSpO2Percent > 100.0 {
		return fmt.Errorf("room_air_spo2_percent must be in [95.0, 100.0], got %v", r.RoomAirSpO2Percent)
	}
	if !r.ChestTubeAsepticallyRemoved {
		return fmt.Errorf("chest_tube_aseptically_removed must be true for chest tube removal clearance")
	}
	if strings.TrimSpace(r.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// PneumothoraxOperation represents a Tension Pneumothorax, Flail Chest,
// 7th–9th ICS Thoracocentesis & Heimlich/Pleural Seal Chest Tube triage operation.
type PneumothoraxOperation struct {
	ID                          string                         `json:"id"`
	IncidentID                  string                         `json:"incident_id"`
	PetName                     string                         `json:"pet_name"`
	Status                      PneumothoraxStatus             `json:"status"`
	TensionRadiographDelayAlert bool                           `json:"tension_radiograph_delay_alert"`
	PatientProfile              PneumothoraxPatientProfile     `json:"patient_profile"`
	DecompressionPlan           *PneumothoraxDecompressionPlan `json:"decompression_plan,omitempty"`
	DrainageChecks              []PleuralDrainageSerialCheck   `json:"drainage_checks"`
	RemovalClearance            *PneumothoraxRemovalClearance  `json:"removal_clearance,omitempty"`
	CreatedAt                   time.Time                      `json:"created_at"`
	UpdatedAt                   time.Time                      `json:"updated_at"`
}

// Validate checks domain invariants on PneumothoraxOperation.
func (op PneumothoraxOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.IncidentID) == "" {
		return fmt.Errorf("incident_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if err := op.PatientProfile.Validate(); err != nil {
		return fmt.Errorf("invalid patient_profile: %w", err)
	}
	return nil
}
