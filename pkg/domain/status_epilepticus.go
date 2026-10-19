package domain

import (
	"fmt"
	"strings"
	"time"
)

// StatusEpilepticusStatus represents the lifecycle state of a Status Epilepticus, Cluster Seizures,
// Bromethalin/Metaldehyde Neuro-Tremor & Levetiracetam/Propofol CRI triage operation.
type StatusEpilepticusStatus string

const (
	EpilepticusStatusActiveIctalCritical     StatusEpilepticusStatus = "ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL"
	EpilepticusStatusAnticonvulsantCRIStaged StatusEpilepticusStatus = "MIDAZOLAM_KEPPRA_METHOCARBAMOL_STAGED"
	EpilepticusStatusSeizureFreeNormothermic StatusEpilepticusStatus = "SEIZURE_FREE_24H_NORMOTHERMIC_CLEARED"
)

// IsValid returns true when StatusEpilepticusStatus is a supported lifecycle state.
func (s StatusEpilepticusStatus) IsValid() bool {
	switch s {
	case EpilepticusStatusActiveIctalCritical,
		EpilepticusStatusAnticonvulsantCRIStaged,
		EpilepticusStatusSeizureFreeNormothermic:
		return true
	default:
		return false
	}
}

const (
	EpilepticusSpeciesCanine = "CANINE"
	EpilepticusSpeciesFeline = "FELINE"

	NeuroEtiologyIdiopathicStatusEpilepticus = "IDIOPATHIC_STATUS_EPILEPTICUS"
	NeuroEtiologyMetaldehydeSnailBaitTremor  = "METALDEHYDE_SNAIL_BAIT_TREMOR"
	NeuroEtiologyPyrethroidFelinePermethrin  = "PYRETHROID_FELINE_PERMETHRIN"
	NeuroEtiologyBromethalinCerebralEdema    = "BROMETHALIN_CEREBRAL_EDEMA"

	PupillaryReflexNormalBilateral         = "NORMAL_BILATERAL"
	PupillaryReflexMioticPinpoint          = "MIOTIC_PINPOINT"
	PupillaryReflexAnisocoriaCushingReflex = "ANISOCORIA_CUSHING_REFLEX"
)

// IsValidEpilepticusSpecies returns true if species is CANINE or FELINE.
func IsValidEpilepticusSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case EpilepticusSpeciesCanine, EpilepticusSpeciesFeline:
		return true
	default:
		return false
	}
}

// IsValidNeuroEtiology returns true if etiology is a recognized neurological emergency etiology.
func IsValidNeuroEtiology(etiology string) bool {
	switch strings.ToUpper(strings.TrimSpace(etiology)) {
	case NeuroEtiologyIdiopathicStatusEpilepticus,
		NeuroEtiologyMetaldehydeSnailBaitTremor,
		NeuroEtiologyPyrethroidFelinePermethrin,
		NeuroEtiologyBromethalinCerebralEdema:
		return true
	default:
		return false
	}
}

// IsValidPupillaryLightReflex returns true if plr is a supported pupillary reflex classification.
func IsValidPupillaryLightReflex(plr string) bool {
	switch strings.ToUpper(strings.TrimSpace(plr)) {
	case PupillaryReflexNormalBilateral,
		PupillaryReflexMioticPinpoint,
		PupillaryReflexAnisocoriaCushingReflex:
		return true
	default:
		return false
	}
}

// EpilepticusPatientProfile captures patient signalment, neuro-etiology, ictal duration, cluster frequency,
// rectal temperature, blood glucose, and Modified Glasgow Coma Scale (MGCS) score.
type EpilepticusPatientProfile struct {
	Species                  string  `json:"species"`
	Breed                    string  `json:"breed,omitempty"`
	WeightKg                 float64 `json:"weight_kg"`
	NeuroEtiology            string  `json:"neuro_etiology"`
	ContinuousIctalMinutes   int     `json:"continuous_ictal_minutes"`
	ClusterCount24Hr         int     `json:"cluster_count_24hr"`
	RectalTempFahrenheit     float64 `json:"rectal_temp_fahrenheit"`
	BloodGlucoseMgDl         float64 `json:"blood_glucose_mg_dl"`
	ModifiedGlasgowComaScore int     `json:"modified_glasgow_coma_score"`
}

// Validate enforces clinical invariants on EpilepticusPatientProfile.
func (p EpilepticusPatientProfile) Validate() error {
	if !IsValidEpilepticusSpecies(p.Species) {
		return fmt.Errorf("species must be CANINE or FELINE, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if !IsValidNeuroEtiology(p.NeuroEtiology) {
		return fmt.Errorf("invalid neuro_etiology: %q", p.NeuroEtiology)
	}
	if p.ContinuousIctalMinutes < 0 || p.ContinuousIctalMinutes > 1440 {
		return fmt.Errorf("continuous_ictal_minutes must be in [0, 1440], got %d", p.ContinuousIctalMinutes)
	}
	if p.ClusterCount24Hr < 0 || p.ClusterCount24Hr > 100 {
		return fmt.Errorf("cluster_count_24hr must be in [0, 100], got %d", p.ClusterCount24Hr)
	}
	if p.RectalTempFahrenheit < 94.0 || p.RectalTempFahrenheit > 112.0 {
		return fmt.Errorf("rectal_temp_fahrenheit must be in [94.0, 112.0], got %v", p.RectalTempFahrenheit)
	}
	if p.BloodGlucoseMgDl < 15.0 || p.BloodGlucoseMgDl > 500.0 {
		return fmt.Errorf("blood_glucose_mg_dl must be in [15.0, 500.0], got %v", p.BloodGlucoseMgDl)
	}
	if p.ModifiedGlasgowComaScore < 3 || p.ModifiedGlasgowComaScore > 18 {
		return fmt.Errorf("modified_glasgow_coma_score must be in [3, 18], got %d", p.ModifiedGlasgowComaScore)
	}
	return nil
}

// AnticonvulsantTitrationRegimen captures calculated anticonvulsant, muscle relaxant, osmotherapy,
// anesthetic CRI, and active cooling cutoffs for status epilepticus & neurotoxic tremor triage.
type AnticonvulsantTitrationRegimen struct {
	MidazolamFirstLineBolusMg       float64 `json:"midazolam_first_line_bolus_mg"`
	LevetiracetamLoading60MgKgMg    float64 `json:"levetiracetam_loading_60mg_kg_mg"`
	PhenobarbitalLoadingDoseMg      float64 `json:"phenobarbital_loading_dose_mg"`
	Methocarbamol55MgKgDoseMg       float64 `json:"methocarbamol_55mg_kg_dose_mg"`
	PropofolCRIRateMgKgMin          float64 `json:"propofol_cri_rate_mg_kg_min"`
	Mannitol05GKgCerebralEdemaGrams float64 `json:"mannitol_05g_kg_cerebral_edema_grams"`
	StopCoolingThresholdTempF       float64 `json:"stop_cooling_threshold_temp_f"`
	ZeroOralGagAspirationGuard      bool    `json:"zero_oral_gag_aspiration_guard"`
	NeuroprotectionPitfallAlert     string  `json:"neuroprotection_pitfall_alert"`
	ClinicalRationale               string  `json:"clinical_rationale"`
}

// NeuroIcuSerialCheck records a serial neurological ICU assessment.
type NeuroIcuSerialCheck struct {
	Timestamp                time.Time `json:"timestamp"`
	SeizureActivityObserved  bool      `json:"seizure_activity_observed"`
	RectalTempFahrenheit     float64   `json:"rectal_temp_fahrenheit"`
	ModifiedGlasgowComaScore int       `json:"modified_glasgow_coma_score"`
	PupillaryLightReflex     string    `json:"pupillary_light_reflex"`
	BloodGlucoseMgDl         float64   `json:"blood_glucose_mg_dl"`
	ClinicianCallsign        string    `json:"clinician_callsign"`
	Notes                    string    `json:"notes"`
}

// Validate enforces clinical invariants on NeuroIcuSerialCheck.
func (c NeuroIcuSerialCheck) Validate() error {
	if c.RectalTempFahrenheit < 94.0 || c.RectalTempFahrenheit > 112.0 {
		return fmt.Errorf("rectal_temp_fahrenheit must be in [94.0, 112.0], got %v", c.RectalTempFahrenheit)
	}
	if c.ModifiedGlasgowComaScore < 3 || c.ModifiedGlasgowComaScore > 18 {
		return fmt.Errorf("modified_glasgow_coma_score must be in [3, 18], got %d", c.ModifiedGlasgowComaScore)
	}
	if !IsValidPupillaryLightReflex(c.PupillaryLightReflex) {
		return fmt.Errorf("invalid pupillary_light_reflex: %q", c.PupillaryLightReflex)
	}
	if c.BloodGlucoseMgDl < 15.0 || c.BloodGlucoseMgDl > 500.0 {
		return fmt.Errorf("blood_glucose_mg_dl must be in [15.0, 500.0], got %v", c.BloodGlucoseMgDl)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// EpilepticusDischargeClearance certifies 24-hour seizure-free normothermic recovery and home rescue readiness.
type EpilepticusDischargeClearance struct {
	ClearedAt                          time.Time `json:"cleared_at"`
	SeizureFreeHours                   int       `json:"seizure_free_hours"`
	FinalModifiedGlasgowScore          int       `json:"final_modified_glasgow_score"`
	FinalRectalTempFahrenheit          float64   `json:"final_rectal_temp_fahrenheit"`
	AmbulatoryMenaceIntact             bool      `json:"ambulatory_menace_intact"`
	RescueIntranasalMidazolamDispensed bool      `json:"rescue_intranasal_midazolam_dispensed"`
	VetLicense                         string    `json:"vet_license"`
	Summary                            string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on EpilepticusDischargeClearance.
func (d EpilepticusDischargeClearance) Validate() error {
	if d.SeizureFreeHours < 24 {
		return fmt.Errorf("seizure_free_hours must be >= 24, got %d", d.SeizureFreeHours)
	}
	if d.FinalModifiedGlasgowScore < 16 || d.FinalModifiedGlasgowScore > 18 {
		return fmt.Errorf("final_modified_glasgow_score must be in [16, 18], got %d", d.FinalModifiedGlasgowScore)
	}
	if d.FinalRectalTempFahrenheit < 99.0 || d.FinalRectalTempFahrenheit > 102.5 {
		return fmt.Errorf("final_rectal_temp_fahrenheit must be in [99.0, 102.5], got %v", d.FinalRectalTempFahrenheit)
	}
	if !d.AmbulatoryMenaceIntact {
		return fmt.Errorf("ambulatory_menace_intact must be true for discharge clearance")
	}
	if !d.RescueIntranasalMidazolamDispensed {
		return fmt.Errorf("rescue_intranasal_midazolam_dispensed must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// StatusEpilepticusOperation represents a Status Epilepticus, Cluster Seizures, Bromethalin/Metaldehyde
// Neuro-Tremor & Levetiracetam/Propofol CRI triage operation.
type StatusEpilepticusOperation struct {
	ID                          string                          `json:"id"`
	IncidentID                  string                          `json:"incident_id"`
	PetName                     string                          `json:"pet_name"`
	Status                      StatusEpilepticusStatus         `json:"status"`
	NeurogenicHyperthermiaAlert bool                            `json:"neurogenic_hyperthermia_alert"`
	PatientProfile              EpilepticusPatientProfile       `json:"patient_profile"`
	TitrationRegimen            *AnticonvulsantTitrationRegimen `json:"titration_regimen,omitempty"`
	NeuroChecks                 []NeuroIcuSerialCheck           `json:"neuro_checks"`
	DischargeClearance          *EpilepticusDischargeClearance  `json:"discharge_clearance,omitempty"`
	CreatedAt                   time.Time                       `json:"created_at"`
	UpdatedAt                   time.Time                       `json:"updated_at"`
}

// Validate checks domain invariants on StatusEpilepticusOperation.
func (op StatusEpilepticusOperation) Validate() error {
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
