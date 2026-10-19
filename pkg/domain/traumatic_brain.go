package domain

import (
	"fmt"
	"strings"
	"time"
)

// TraumaticBrainStatus represents the lifecycle state of a Traumatic Brain Injury (TBI),
// Modified Glasgow Coma Scale (MGCS), Cushing's Reflex & Hypertonic Saline/Mannitol triage operation.
type TraumaticBrainStatus string

const (
	TBIStatusCushingHerniationCritical       TraumaticBrainStatus = "CUSHING_REFLEX_ICP_HERNIATION_CRITICAL"
	TBIStatusHypertonicMannitolStaged        TraumaticBrainStatus = "HTS_MANNITOL_30DEG_HEAD_ELEVATION_STAGED"
	TBIStatusMGCSRecoveredNormocapnicCleared TraumaticBrainStatus = "MGCS_RECOVERED_NORMOCAPNIC_CLEARED"
)

// IsValid returns true when TraumaticBrainStatus is a supported lifecycle state.
func (s TraumaticBrainStatus) IsValid() bool {
	switch s {
	case TBIStatusCushingHerniationCritical,
		TBIStatusHypertonicMannitolStaged,
		TBIStatusMGCSRecoveredNormocapnicCleared:
		return true
	default:
		return false
	}
}

const (
	TBISpeciesCanineBluntVehicular       = "CANINE_BLUNT_VEHICULAR_TBI"
	TBISpeciesCanineHighVelocityCranial  = "CANINE_HIGH_VELOCITY_CRANIAL"
	TBISpeciesFelineMaxillofacialCranial = "FELINE_MAXILLOFACIAL_CRANIAL_TRAUMA"
)

// IsValidTBISpecies returns true if species is a recognized TBI presentation classification.
func IsValidTBISpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case TBISpeciesCanineBluntVehicular,
		TBISpeciesCanineHighVelocityCranial,
		TBISpeciesFelineMaxillofacialCranial:
		return true
	default:
		return false
	}
}

const (
	TBIPupilsAnisocoriaUnilateralMydriasis = "ANISOCORIA_UNILATERAL_MYDRIASIS"
	TBIPupilsMioticPinpointBilateral       = "MIOTIC_PINPOINT_BILATERAL"
	TBIPupilsResponsiveIsocoric            = "RESPONSIVE_ISOCORIC"
)

// IsValidTBIPupillaryStatus returns true if pupillaryStatus is a recognized PLR/pupil classification.
func IsValidTBIPupillaryStatus(pupillaryStatus string) bool {
	switch strings.ToUpper(strings.TrimSpace(pupillaryStatus)) {
	case TBIPupilsAnisocoriaUnilateralMydriasis,
		TBIPupilsMioticPinpointBilateral,
		TBIPupilsResponsiveIsocoric:
		return true
	default:
		return false
	}
}

// TBIPatientProfile captures signalment, Modified Glasgow Coma Scale (MGCS 3-18),
// Mean Arterial Pressure (MAP), heart rate, EtCO2, pupillary light reflex status, and Cushing's reflex telemetry.
type TBIPatientProfile struct {
	Species                  string  `json:"species"`
	Breed                    string  `json:"breed,omitempty"`
	WeightKg                 float64 `json:"weight_kg"`
	ModifiedGlasgowComaScore int     `json:"modified_glasgow_coma_score"`
	MeanArterialPressureMmHg int     `json:"mean_arterial_pressure_mmhg"`
	HeartRateBpm             int     `json:"heart_rate_bpm"`
	EndTidalCO2MmHg          float64 `json:"end_tidal_co2_mmhg"`
	PupillaryStatus          string  `json:"pupillary_status"`
	CushingReflexPresent     bool    `json:"cushing_reflex_present"`
}

// Validate enforces clinical invariants on TBIPatientProfile.
func (p TBIPatientProfile) Validate() error {
	if !IsValidTBISpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_BLUNT_VEHICULAR_TBI, CANINE_HIGH_VELOCITY_CRANIAL, or FELINE_MAXILLOFACIAL_CRANIAL_TRAUMA, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.ModifiedGlasgowComaScore < 3 || p.ModifiedGlasgowComaScore > 18 {
		return fmt.Errorf("modified_glasgow_coma_score must be in [3, 18], got %d", p.ModifiedGlasgowComaScore)
	}
	if p.MeanArterialPressureMmHg < 30 || p.MeanArterialPressureMmHg > 240 {
		return fmt.Errorf("mean_arterial_pressure_mmhg must be in [30, 240], got %d", p.MeanArterialPressureMmHg)
	}
	if p.HeartRateBpm < 20 || p.HeartRateBpm > 300 {
		return fmt.Errorf("heart_rate_bpm must be in [20, 300], got %d", p.HeartRateBpm)
	}
	if p.EndTidalCO2MmHg < 15.0 || p.EndTidalCO2MmHg > 90.0 {
		return fmt.Errorf("end_tidal_co2_mmhg must be in [15.0, 90.0], got %v", p.EndTidalCO2MmHg)
	}
	if !IsValidTBIPupillaryStatus(p.PupillaryStatus) {
		return fmt.Errorf("pupillary_status must be ANISOCORIA_UNILATERAL_MYDRIASIS, MIOTIC_PINPOINT_BILATERAL, or RESPONSIVE_ISOCORIC, got %q", p.PupillaryStatus)
	}
	return nil
}

// TBINeuroprotectivePlan captures calculated 7.5% Hypertonic Saline (HTS), Mannitol 0.5 g/kg slow IV via filter,
// 30-degree slant-board head elevation, Cerebral Perfusion Pressure (CPP = MAP - ICP) MAP >= 80 mmHg target,
// normocapnic EtCO2 35-40 mmHg target, and corticosteroid/jugular-compression contraindication guards.
type TBINeuroprotectivePlan struct {
	HypertonicSaline75PctMlKg                 float64 `json:"hypertonic_saline_75pct_ml_kg"`
	HypertonicSalineTotalDoseMl               float64 `json:"hypertonic_saline_total_dose_ml"`
	Mannitol05GKgSlow20MinGrams               float64 `json:"mannitol_05_g_kg_slow_20min_grams"`
	HeadElevationAngleDegrees                 int     `json:"head_elevation_angle_degrees"`
	TargetMAPMinimumMmHg                      int     `json:"target_map_minimum_mmhg"`
	TargetEtCO2RangeMmHg                      string  `json:"target_etco2_range_mmhg"`
	ZeroJugularCompressionCorticosteroidGuard bool    `json:"zero_jugular_compression_corticosteroid_guard"`
	TBIPitfallAlert                           string  `json:"tbi_pitfall_alert"`
	ClinicalRationale                         string  `json:"clinical_rationale"`
}

// TBINeuroSerialCheck records a serial q30-60m Modified Glasgow Coma Scale, MAP, HR, EtCO2, and pupillary check.
type TBINeuroSerialCheck struct {
	Timestamp                time.Time `json:"timestamp"`
	ModifiedGlasgowComaScore int       `json:"modified_glasgow_coma_score"`
	MeanArterialPressureMmHg int       `json:"mean_arterial_pressure_mmhg"`
	HeartRateBpm             int       `json:"heart_rate_bpm"`
	EndTidalCO2MmHg          float64   `json:"end_tidal_co2_mmhg"`
	PupillaryStatus          string    `json:"pupillary_status"`
	ClinicianCallsign        string    `json:"clinician_callsign"`
	Notes                    string    `json:"notes"`
}

// Validate enforces clinical invariants on TBINeuroSerialCheck.
func (c TBINeuroSerialCheck) Validate() error {
	if c.ModifiedGlasgowComaScore < 3 || c.ModifiedGlasgowComaScore > 18 {
		return fmt.Errorf("modified_glasgow_coma_score must be in [3, 18], got %d", c.ModifiedGlasgowComaScore)
	}
	if c.MeanArterialPressureMmHg < 30 || c.MeanArterialPressureMmHg > 240 {
		return fmt.Errorf("mean_arterial_pressure_mmhg must be in [30, 240], got %d", c.MeanArterialPressureMmHg)
	}
	if c.HeartRateBpm < 20 || c.HeartRateBpm > 300 {
		return fmt.Errorf("heart_rate_bpm must be in [20, 300], got %d", c.HeartRateBpm)
	}
	if c.EndTidalCO2MmHg < 15.0 || c.EndTidalCO2MmHg > 90.0 {
		return fmt.Errorf("end_tidal_co2_mmhg must be in [15.0, 90.0], got %v", c.EndTidalCO2MmHg)
	}
	if !IsValidTBIPupillaryStatus(c.PupillaryStatus) {
		return fmt.Errorf("pupillary_status must be ANISOCORIA_UNILATERAL_MYDRIASIS, MIOTIC_PINPOINT_BILATERAL, or RESPONSIVE_ISOCORIC, got %q", c.PupillaryStatus)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// TBIDischargeClearance certifies MGCS recovery (16-18), normotension (MAP 80-120 mmHg),
// responsive isocoric pupils, intact gag/swallow reflex, and ambulatory normocapnia.
type TBIDischargeClearance struct {
	ClearedAt                     time.Time `json:"cleared_at"`
	FinalModifiedGlasgowScore     int       `json:"final_modified_glasgow_score"`
	FinalMAPMmHg                  int       `json:"final_map_mmhg"`
	PupilsIsocoricResponsive      bool      `json:"pupils_isocoric_responsive"`
	GagSwallowReflexIntact        bool      `json:"gag_swallow_reflex_intact"`
	AmbulatoryNormocapnicVerified bool      `json:"ambulatory_normocapnic_verified"`
	VetLicense                    string    `json:"vet_license"`
	Summary                       string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on TBIDischargeClearance.
func (d TBIDischargeClearance) Validate() error {
	if d.FinalModifiedGlasgowScore < 16 || d.FinalModifiedGlasgowScore > 18 {
		return fmt.Errorf("final_modified_glasgow_score must be in [16, 18], got %d", d.FinalModifiedGlasgowScore)
	}
	if d.FinalMAPMmHg < 80 || d.FinalMAPMmHg > 120 {
		return fmt.Errorf("final_map_mmhg must be in [80, 120], got %d", d.FinalMAPMmHg)
	}
	if !d.PupilsIsocoricResponsive {
		return fmt.Errorf("pupils_isocoric_responsive must be true for discharge clearance")
	}
	if !d.GagSwallowReflexIntact {
		return fmt.Errorf("gag_swallow_reflex_intact must be true for discharge clearance")
	}
	if !d.AmbulatoryNormocapnicVerified {
		return fmt.Errorf("ambulatory_normocapnic_verified must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// TraumaticBrainOperation represents a Traumatic Brain Injury (TBI), Modified Glasgow Coma Scale (MGCS),
// Cushing's Reflex & Hypertonic Saline/Mannitol triage operation.
type TraumaticBrainOperation struct {
	ID                            string                  `json:"id"`
	IncidentID                    string                  `json:"incident_id"`
	PetName                       string                  `json:"pet_name"`
	Status                        TraumaticBrainStatus    `json:"status"`
	CushingHerniationSteroidAlert bool                    `json:"cushing_herniation_steroid_alert"`
	PatientProfile                TBIPatientProfile       `json:"patient_profile"`
	NeuroprotectivePlan           *TBINeuroprotectivePlan `json:"neuroprotective_plan,omitempty"`
	NeuroChecks                   []TBINeuroSerialCheck   `json:"neuro_checks"`
	DischargeClearance            *TBIDischargeClearance  `json:"discharge_clearance,omitempty"`
	CreatedAt                     time.Time               `json:"created_at"`
	UpdatedAt                     time.Time               `json:"updated_at"`
}

// Validate checks domain invariants on TraumaticBrainOperation.
func (op TraumaticBrainOperation) Validate() error {
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
