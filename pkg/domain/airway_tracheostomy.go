package domain

import (
	"fmt"
	"strings"
	"time"
)

// AirwayObstructionStatus represents the clinical triage lifecycle state of a
// Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis & Temporary Tracheostomy operation.
type AirwayObstructionStatus string

const (
	AirwayStatusStridorCyanosisCritical     AirwayObstructionStatus = "STRIDOR_CYANOSIS_DYSPNEA_CRITICAL"
	AirwayStatusSedationDexamethasoneStaged AirwayObstructionStatus = "BUTORPHANOL_DEX_INTUBATION_STAGED"
	AirwayStatusPatentNormothermicExtubated AirwayObstructionStatus = "PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED"
)

// IsValid returns true if the AirwayObstructionStatus is a recognized enum value.
func (s AirwayObstructionStatus) IsValid() bool {
	switch s {
	case AirwayStatusStridorCyanosisCritical,
		AirwayStatusSedationDexamethasoneStaged,
		AirwayStatusPatentNormothermicExtubated:
		return true
	default:
		return false
	}
}

const (
	AirwaySpeciesCanineBrachycephalicBOAS  = "CANINE_BRACHYCEPHALIC_BOAS"
	AirwaySpeciesCanineGeriatricLarPar     = "CANINE_GERIATRIC_LARPAR"
	AirwaySpeciesFelineNasopharyngealPolyp = "FELINE_NASOPHARYNGEAL_POLYP"
)

const (
	AirwayEffortSevereParadoxicalCyanotic = "SEVERE_PARADOXICAL_CYANOTIC"
	AirwayEffortModerateStridorOrthopnea  = "MODERATE_STRIDOR_ORTHOPNEA"
	AirwayEffortMildStertorEupneic        = "MILD_STERTOR_EUPNEIC"
)

const (
	AirwayEdemaGradeIIIEvertedSacculesCollapse = "GRADE_III_EVERTED_SACCULES_COLLAPSE"
	AirwayEdemaGradeIIElongatedPalateEdema     = "GRADE_II_ELONGATED_PALATE_EDEMA"
	AirwayEdemaGradeIStenoticNaresOnly         = "GRADE_I_STENOTIC_NARES_ONLY"
)

// AirwayPatientProfile captures upper-airway obstruction telemetry, thermal load, oxygenation, and laryngeal pathology.
type AirwayPatientProfile struct {
	Species              string  `json:"species"`
	Breed                string  `json:"breed"`
	WeightKg             float64 `json:"weight_kg"`
	RectalTempFahrenheit float64 `json:"rectal_temp_fahrenheit"`
	SpO2Percent          float64 `json:"spo2_percent"`
	InspiratoryEffort    string  `json:"inspiratory_effort"`
	LaryngealEdemaGrade  string  `json:"laryngeal_edema_grade"`
	GagReflexPresent     bool    `json:"gag_reflex_present"`
}

// Validate checks clinical invariants on AirwayPatientProfile.
func (p AirwayPatientProfile) Validate() error {
	switch strings.TrimSpace(p.Species) {
	case AirwaySpeciesCanineBrachycephalicBOAS,
		AirwaySpeciesCanineGeriatricLarPar,
		AirwaySpeciesFelineNasopharyngealPolyp:
	default:
		return fmt.Errorf("invalid species %q: must be CANINE_BRACHYCEPHALIC_BOAS, CANINE_GERIATRIC_LARPAR, or FELINE_NASOPHARYNGEAL_POLYP", p.Species)
	}
	if strings.TrimSpace(p.Breed) == "" {
		return fmt.Errorf("breed is required")
	}
	if p.WeightKg < 0.5 || p.WeightKg > 100.0 {
		return fmt.Errorf("weight_kg must be between 0.5 and 100.0 kg, got %.2f", p.WeightKg)
	}
	if p.RectalTempFahrenheit < 95.0 || p.RectalTempFahrenheit > 110.0 {
		return fmt.Errorf("rectal_temp_fahrenheit must be between 95.0 and 110.0°F, got %.1f", p.RectalTempFahrenheit)
	}
	if p.SpO2Percent < 50.0 || p.SpO2Percent > 100.0 {
		return fmt.Errorf("spo2_percent must be between 50.0 and 100.0%%, got %.1f", p.SpO2Percent)
	}
	switch strings.TrimSpace(p.InspiratoryEffort) {
	case AirwayEffortSevereParadoxicalCyanotic,
		AirwayEffortModerateStridorOrthopnea,
		AirwayEffortMildStertorEupneic:
	default:
		return fmt.Errorf("invalid inspiratory_effort %q", p.InspiratoryEffort)
	}
	switch strings.TrimSpace(p.LaryngealEdemaGrade) {
	case AirwayEdemaGradeIIIEvertedSacculesCollapse,
		AirwayEdemaGradeIIElongatedPalateEdema,
		AirwayEdemaGradeIStenoticNaresOnly:
	default:
		return fmt.Errorf("invalid laryngeal_edema_grade %q", p.LaryngealEdemaGrade)
	}
	return nil
}

// AirwayStabilizationPlan contains calculated anxiolysis, corticosteroid edema reduction, rapid-sequence induction,
// downsized endotracheal tube sizing, active cooling ceiling, and temporary tracheostomy indications.
type AirwayStabilizationPlan struct {
	ButorphanolAcepromazineDoseMg  float64 `json:"butorphanol_acepromazine_dose_mg"`
	DexamethasoneSP015MgKgDoseMg   float64 `json:"dexamethasone_sp_015_mg_kg_dose_mg"`
	PropofolInduction2MgKgDoseMg   float64 `json:"propofol_induction_2_mg_kg_dose_mg"`
	EndotrachealTubeSizeMm         float64 `json:"endotracheal_tube_size_mm"`
	TemporaryTracheostomyIndicated bool    `json:"temporary_tracheostomy_indicated"`
	ActiveCoolingTargetMaxTempF    float64 `json:"active_cooling_target_max_temp_f"`
	ZeroNeckLeadRestraintGuard     bool    `json:"zero_neck_lead_restraint_guard"`
	AirwayHazardAlert              string  `json:"airway_hazard_alert"`
	ClinicalRationale              string  `json:"clinical_rationale"`
}

// TracheostomyCareLog records aseptic temporary tracheostomy or endotracheal tube maintenance,
// pre-oxygenation duration, sterile saline instillation, and suctioning duration guardrails.
type TracheostomyCareLog struct {
	Timestamp                   time.Time `json:"timestamp"`
	SterileSalineInstillationMl float64   `json:"sterile_saline_instillation_ml"`
	PreOxygenationDurationSec   int       `json:"pre_oxygenation_duration_sec"`
	SuctionDurationMaxSec       int       `json:"suction_duration_max_sec"`
	InnerCannulaCleaned         bool      `json:"inner_cannula_cleaned"`
	SubcutaneousEmphysemaNoted  bool      `json:"subcutaneous_emphysema_noted"`
	PostSuctionSpO2Percent      float64   `json:"post_suction_spo2_percent"`
	TechnicianCallsign          string    `json:"technician_callsign"`
	Notes                       string    `json:"notes"`
}

// Validate enforces clinical safety invariants on TracheostomyCareLog.
func (l TracheostomyCareLog) Validate() error {
	if l.PreOxygenationDurationSec < 60 {
		return fmt.Errorf("pre_oxygenation_duration_sec must be at least 60 seconds prior to tracheal suctioning, got %d", l.PreOxygenationDurationSec)
	}
	if l.SuctionDurationMaxSec <= 0 || l.SuctionDurationMaxSec > 10 {
		return fmt.Errorf("suction_duration_max_sec must be in [1, 10] seconds to prevent vagal bradycardia and acute hypoxia, got %d", l.SuctionDurationMaxSec)
	}
	if l.SterileSalineInstillationMl < 0 || l.SterileSalineInstillationMl > 10.0 {
		return fmt.Errorf("sterile_saline_instillation_ml must be between 0.0 and 10.0 mL, got %.2f", l.SterileSalineInstillationMl)
	}
	if l.PostSuctionSpO2Percent < 50.0 || l.PostSuctionSpO2Percent > 100.0 {
		return fmt.Errorf("post_suction_spo2_percent must be between 50.0 and 100.0%%, got %.1f", l.PostSuctionSpO2Percent)
	}
	if strings.TrimSpace(l.TechnicianCallsign) == "" {
		return fmt.Errorf("technician_callsign is required")
	}
	return nil
}

// AirwayExtubationClearance verifies normothermia, room-air oxygenation, vigorous gag/swallow reflex,
// tracheostomy occlusion trial tolerance, and harness-only discharge prior to extubation/decannulation.
type AirwayExtubationClearance struct {
	ClearedAt                    time.Time `json:"cleared_at"`
	FinalSpO2OnRoomAirPercent    float64   `json:"final_spo2_on_room_air_percent"`
	FinalRectalTempFahrenheit    float64   `json:"final_rectal_temp_fahrenheit"`
	GagSwallowReflexVigorous     bool      `json:"gag_swallow_reflex_vigorous"`
	TrachTubeOccludedTrialPassed bool      `json:"trach_tube_occluded_trial_passed"`
	HarnessOnlyDischargeVerified bool      `json:"harness_only_discharge_verified"`
	VetLicense                   string    `json:"vet_license"`
	Summary                      string    `json:"summary"`
}

// Validate enforces clinical extubation/decannulation invariants on AirwayExtubationClearance.
func (c AirwayExtubationClearance) Validate() error {
	if c.FinalSpO2OnRoomAirPercent < 95.0 || c.FinalSpO2OnRoomAirPercent > 100.0 {
		return fmt.Errorf("final_spo2_on_room_air_percent must be >= 95.0%% on room air for safe extubation, got %.1f%%", c.FinalSpO2OnRoomAirPercent)
	}
	if c.FinalRectalTempFahrenheit < 99.0 || c.FinalRectalTempFahrenheit > 102.5 {
		return fmt.Errorf("final_rectal_temp_fahrenheit must be normothermic in [99.0, 102.5]°F, got %.1f°F", c.FinalRectalTempFahrenheit)
	}
	if !c.GagSwallowReflexVigorous {
		return fmt.Errorf("gag_swallow_reflex_vigorous must be true prior to extubation to prevent aspiration")
	}
	if !c.TrachTubeOccludedTrialPassed {
		return fmt.Errorf("trach_tube_occluded_trial_passed must be true prior to decannulation/extubation")
	}
	if !c.HarnessOnlyDischargeVerified {
		return fmt.Errorf("harness_only_discharge_verified must be true (strictly no neck leads or choke collars)")
	}
	if strings.TrimSpace(c.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	if strings.TrimSpace(c.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

// AirwayTracheostomyOperation represents a complete Brachycephalic Obstructive Airway Syndrome (BOAS),
// Laryngeal Paralysis & Temporary Tracheostomy triage operation.
type AirwayTracheostomyOperation struct {
	ID                  string                     `json:"id"`
	IncidentID          string                     `json:"incident_id"`
	PetName             string                     `json:"pet_name"`
	Status              AirwayObstructionStatus    `json:"status"`
	CriticalAirwayAlert bool                       `json:"critical_airway_alert"`
	PatientProfile      AirwayPatientProfile       `json:"patient_profile"`
	StabilizationPlan   *AirwayStabilizationPlan   `json:"stabilization_plan,omitempty"`
	TrachCareLogs       []TracheostomyCareLog      `json:"trach_care_logs"`
	ExtubationClearance *AirwayExtubationClearance `json:"extubation_clearance,omitempty"`
	CreatedAt           time.Time                  `json:"created_at"`
	UpdatedAt           time.Time                  `json:"updated_at"`
}
