package domain

import (
	"fmt"
	"strings"
	"time"
)

// CardiogenicEdemaStatus represents the lifecycle state of an Acute Cardiogenic Pulmonary Edema,
// Mitral Valve Chordae Rupture, Furosemide/Pimobendan & Oxygen Cage triage operation.
type CardiogenicEdemaStatus string

const (
	CardiogenicStatusFulminantEdemaCritical CardiogenicEdemaStatus = "FULMINANT_PULMONARY_EDEMA_CRITICAL"
	CardiogenicStatusFurosemideO2CageStaged CardiogenicEdemaStatus = "FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED"
	CardiogenicStatusEupneicBLineResolved   CardiogenicEdemaStatus = "EUPNEIC_BLINES_RESOLVED_CLEARED"
)

// Supported species/pathology profiles for cardiogenic pulmonary edema triage.
const (
	CardiogenicSpeciesCanineMMVD = "CANINE_MMVD_STAGE_C_D"
	CardiogenicSpeciesCanineDCM  = "CANINE_DCM_CONGESTIVE"
	CardiogenicSpeciesFelineHCM  = "FELINE_HCM_PLEURAL_EDEMA"
)

// Supported auscultation murmur and gallop classifications.
const (
	MurmurGradeVIChordaeRupture = "GRADE_VI_THRILL_CHORDAE_RUPTURE"
	MurmurGradeVSystolicApical  = "GRADE_V_SYSTOLIC_APICAL"
	MurmurGradeIVGallopRhythm   = "GRADE_IV_GALLOP_RHYTHM"
)

// IsValid returns true when CardiogenicEdemaStatus is a supported lifecycle state.
func (s CardiogenicEdemaStatus) IsValid() bool {
	switch s {
	case CardiogenicStatusFulminantEdemaCritical,
		CardiogenicStatusFurosemideO2CageStaged,
		CardiogenicStatusEupneicBLineResolved:
		return true
	default:
		return false
	}
}

// Validate checks that CardiogenicEdemaStatus is a supported lifecycle state.
func (s CardiogenicEdemaStatus) Validate() error {
	if !s.IsValid() {
		return fmt.Errorf("invalid cardiogenic edema status: %q", s)
	}
	return nil
}

// CardiogenicPatientProfile captures baseline cardiopulmonary, TFAST B-line, and echocardiographic La:Ao metrics.
type CardiogenicPatientProfile struct {
	Species                string  `json:"species"`
	WeightKg               float64 `json:"weight_kg"`
	RespiratoryRateBpm     int     `json:"respiratory_rate_bpm"`
	HeartRateBpm           int     `json:"heart_rate_bpm"`
	MurmurGrade            string  `json:"murmur_grade"`
	TfastBLinesPerSite     int     `json:"tfast_b_lines_per_site"`
	LaAoRatio              float64 `json:"la_ao_ratio"`
	PleuralEffusionPresent bool    `json:"pleural_effusion_present"`
}

// Validate checks clinical invariants on CardiogenicPatientProfile.
func (p CardiogenicPatientProfile) Validate() error {
	switch strings.TrimSpace(p.Species) {
	case CardiogenicSpeciesCanineMMVD, CardiogenicSpeciesCanineDCM, CardiogenicSpeciesFelineHCM:
	default:
		return fmt.Errorf("invalid species profile: %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.RespiratoryRateBpm < 8 || p.RespiratoryRateBpm > 180 {
		return fmt.Errorf("respiratory_rate_bpm must be in [8, 180], got %d", p.RespiratoryRateBpm)
	}
	if p.HeartRateBpm < 40 || p.HeartRateBpm > 320 {
		return fmt.Errorf("heart_rate_bpm must be in [40, 320], got %d", p.HeartRateBpm)
	}
	switch strings.TrimSpace(p.MurmurGrade) {
	case MurmurGradeVIChordaeRupture, MurmurGradeVSystolicApical, MurmurGradeIVGallopRhythm:
	default:
		return fmt.Errorf("invalid murmur_grade: %q", p.MurmurGrade)
	}
	if p.TfastBLinesPerSite < 0 || p.TfastBLinesPerSite > 20 {
		return fmt.Errorf("tfast_b_lines_per_site must be in [0, 20], got %d", p.TfastBLinesPerSite)
	}
	if p.LaAoRatio < 1.0 || p.LaAoRatio > 4.0 {
		return fmt.Errorf("la_ao_ratio must be in [1.0, 4.0], got %v", p.LaAoRatio)
	}
	return nil
}

// DiureticInotropeRegimen specifies the staged loop diuretic bolus/CRI, inodilator, anxiolytic, oxygen cage, and fluid guard protocol.
type DiureticInotropeRegimen struct {
	FurosemideInitialBolusMg    float64 `json:"furosemide_initial_bolus_mg"`
	FurosemideCRI066MgKgHrMgHr  float64 `json:"furosemide_cri_066_mg_kg_hr_mg_hr"`
	Pimobendan025MgKgDoseMg     float64 `json:"pimobendan_025_mg_kg_dose_mg"`
	ButorphanolAnxiolyticDoseMg float64 `json:"butorphanol_anxiolytic_dose_mg"`
	OxygenCageFiO2Percent       int     `json:"oxygen_cage_fio2_percent"`
	ThoracocentesisIndicated    bool    `json:"thoracocentesis_indicated"`
	ZeroIVCrystalloidFluidGuard bool    `json:"zero_iv_crystalloid_fluid_guard"`
	CardiogenicPitfallAlert     string  `json:"cardiogenic_pitfall_alert"`
	ClinicalRationale           string  `json:"clinical_rationale"`
}

// DiuresisTelemetryCheck records serial respiratory rate, urine output, TFAST B-lines, blood pressure, and renal/electrolyte telemetry.
type DiuresisTelemetryCheck struct {
	Timestamp                 time.Time `json:"timestamp"`
	RestingRespiratoryRateBpm int       `json:"resting_respiratory_rate_bpm"`
	UrineOutputMlKgHr         float64   `json:"urine_output_ml_kg_hr"`
	TfastBLinesRemaining      int       `json:"tfast_b_lines_remaining"`
	SystolicBloodPressureMmHg int       `json:"systolic_blood_pressure_mmhg"`
	CreatinineMgDl            float64   `json:"creatinine_mg_dl"`
	PotassiumMeqL             float64   `json:"potassium_meq_l"`
	ClinicianCallsign         string    `json:"clinician_callsign"`
	Notes                     string    `json:"notes"`
}

// Validate checks clinical invariants on DiuresisTelemetryCheck.
func (c DiuresisTelemetryCheck) Validate() error {
	if c.RestingRespiratoryRateBpm < 8 || c.RestingRespiratoryRateBpm > 180 {
		return fmt.Errorf("resting_respiratory_rate_bpm must be in [8, 180], got %d", c.RestingRespiratoryRateBpm)
	}
	if c.UrineOutputMlKgHr < 0 || c.UrineOutputMlKgHr > 30.0 {
		return fmt.Errorf("urine_output_ml_kg_hr must be in [0, 30.0], got %v", c.UrineOutputMlKgHr)
	}
	if c.TfastBLinesRemaining < 0 || c.TfastBLinesRemaining > 20 {
		return fmt.Errorf("tfast_b_lines_remaining must be in [0, 20], got %d", c.TfastBLinesRemaining)
	}
	if c.SystolicBloodPressureMmHg < 50 || c.SystolicBloodPressureMmHg > 260 {
		return fmt.Errorf("systolic_blood_pressure_mmhg must be in [50, 260], got %d", c.SystolicBloodPressureMmHg)
	}
	if c.CreatinineMgDl < 0.2 || c.CreatinineMgDl > 15.0 {
		return fmt.Errorf("creatinine_mg_dl must be in [0.2, 15.0], got %v", c.CreatinineMgDl)
	}
	if c.PotassiumMeqL < 1.5 || c.PotassiumMeqL > 10.0 {
		return fmt.Errorf("potassium_meq_l must be in [1.5, 10.0], got %v", c.PotassiumMeqL)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// CardiogenicDischargeClearance verifies eupneic resolution, TFAST B-line clearance, room-air normoxia, stable renal azotemia, and owner SRR education.
type CardiogenicDischargeClearance struct {
	ClearedAt                 time.Time `json:"cleared_at"`
	FinalSleepingRespRateBpm  int       `json:"final_sleeping_resp_rate_bpm"`
	FinalTfastBLinesCount     int       `json:"final_tfast_b_lines_count"`
	RoomAirSpO2Percent        float64   `json:"room_air_spo2_percent"`
	RenalAzotemiaStable       bool      `json:"renal_azotemia_stable"`
	HomeSRRMonitoringEducated bool      `json:"home_srr_monitoring_educated"`
	VetLicense                string    `json:"vet_license"`
	Summary                   string    `json:"summary"`
}

// Validate checks discharge safety criteria on CardiogenicDischargeClearance.
func (c CardiogenicDischargeClearance) Validate() error {
	if c.FinalSleepingRespRateBpm < 6 || c.FinalSleepingRespRateBpm > 30 {
		return fmt.Errorf("final_sleeping_resp_rate_bpm must be <= 30 bpm (eupneic target), got %d", c.FinalSleepingRespRateBpm)
	}
	if c.FinalTfastBLinesCount < 0 || c.FinalTfastBLinesCount > 1 {
		return fmt.Errorf("final_tfast_b_lines_count must be <= 1 per site, got %d", c.FinalTfastBLinesCount)
	}
	if c.RoomAirSpO2Percent < 95.0 || c.RoomAirSpO2Percent > 100.0 {
		return fmt.Errorf("room_air_spo2_percent must be >= 95.0%% on room air, got %.1f%%", c.RoomAirSpO2Percent)
	}
	if !c.RenalAzotemiaStable {
		return fmt.Errorf("renal_azotemia_stable must be true prior to cardiogenic edema discharge")
	}
	if !c.HomeSRRMonitoringEducated {
		return fmt.Errorf("home_srr_monitoring_educated must be true prior to cardiogenic edema discharge")
	}
	if strings.TrimSpace(c.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	if strings.TrimSpace(c.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	return nil
}

// CardiogenicEdemaOperation represents an Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture,
// Furosemide/Pimobendan & Oxygen Cage triage operation.
type CardiogenicEdemaOperation struct {
	ID                                 string                         `json:"id"`
	IncidentID                         string                         `json:"incident_id"`
	PetName                            string                         `json:"pet_name"`
	Status                             CardiogenicEdemaStatus         `json:"status"`
	FluidOverloadContraindicationAlert bool                           `json:"fluid_overload_contraindication_alert"`
	PatientProfile                     CardiogenicPatientProfile      `json:"patient_profile"`
	DiureticRegimen                    *DiureticInotropeRegimen       `json:"diuretic_regimen,omitempty"`
	DiuresisChecks                     []DiuresisTelemetryCheck       `json:"diuresis_checks"`
	DischargeClearance                 *CardiogenicDischargeClearance `json:"discharge_clearance,omitempty"`
	CreatedAt                          time.Time                      `json:"created_at"`
	UpdatedAt                          time.Time                      `json:"updated_at"`
}

// Validate checks domain invariants on CardiogenicEdemaOperation.
func (op CardiogenicEdemaOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.IncidentID) == "" {
		return fmt.Errorf("incident_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if err := op.Status.Validate(); err != nil {
		return err
	}
	return op.PatientProfile.Validate()
}
