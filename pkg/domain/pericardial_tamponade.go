package domain

import (
	"fmt"
	"strings"
	"time"
)

// PericardialTamponadeStatus represents the lifecycle state of a Pericardial Effusion,
// Cardiac Tamponade, Electrical Alternans & Right-Sided 4th–5th ICS Pericardiocentesis triage operation.
type PericardialTamponadeStatus string

const (
	TamponadeStatusRADiastolicCollapseCritical PericardialTamponadeStatus = "TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL"
	TamponadeStatusRightICS45TapStaged         PericardialTamponadeStatus = "RIGHT_ICS_4_5_PERICARDIOCENTESIS_STAGED"
	TamponadeStatusDecompressedSinusCleared    PericardialTamponadeStatus = "DECOMPRESSED_RA_EXPANDED_SINUS_CLEARED"
)

// IsValid returns true when PericardialTamponadeStatus is a supported lifecycle state.
func (s PericardialTamponadeStatus) IsValid() bool {
	switch s {
	case TamponadeStatusRADiastolicCollapseCritical,
		TamponadeStatusRightICS45TapStaged,
		TamponadeStatusDecompressedSinusCleared:
		return true
	default:
		return false
	}
}

const (
	TamponadeSpeciesCanineRAHemangiosarcoma      = "CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA"
	TamponadeSpeciesCanineHeartBaseChemodectoma  = "CANINE_HEART_BASE_CHEMODECTOMA"
	TamponadeSpeciesCanineIdiopathicPericarditis = "CANINE_IDIOPATHIC_PERICARDITIS"
)

// IsValidTamponadeSpecies returns true if species is a recognized pericardial effusion etiology.
func IsValidTamponadeSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case TamponadeSpeciesCanineRAHemangiosarcoma,
		TamponadeSpeciesCanineHeartBaseChemodectoma,
		TamponadeSpeciesCanineIdiopathicPericarditis:
		return true
	default:
		return false
	}
}

// TamponadePatientProfile captures signalment, heart rate, mean arterial pressure (MAP),
// right atrial diastolic collapse on TFAST/echocardiography, electrical alternans on ECG,
// pulsus paradoxus, and pericardial effusion PCV.
type TamponadePatientProfile struct {
	Species                      string  `json:"species"`
	Breed                        string  `json:"breed,omitempty"`
	WeightKg                     float64 `json:"weight_kg"`
	HeartRateBpm                 int     `json:"heart_rate_bpm"`
	MeanArterialPressureMmHg     int     `json:"mean_arterial_pressure_mmhg"`
	RightAtrialDiastolicCollapse bool    `json:"right_atrial_diastolic_collapse"`
	ElectricalAlternansPresent   bool    `json:"electrical_alternans_present"`
	PulsusParadoxusMmHg          int     `json:"pulsus_paradoxus_mmhg"`
	EffusionPCVPercent           float64 `json:"effusion_pcv_percent"`
}

// Validate enforces clinical invariants on TamponadePatientProfile.
func (p TamponadePatientProfile) Validate() error {
	if !IsValidTamponadeSpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_RIGHT_ATRIAL_HEMANGIOSARCOMA, CANINE_HEART_BASE_CHEMODECTOMA, or CANINE_IDIOPATHIC_PERICARDITIS, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.HeartRateBpm < 30 || p.HeartRateBpm > 300 {
		return fmt.Errorf("heart_rate_bpm must be in [30, 300], got %d", p.HeartRateBpm)
	}
	if p.MeanArterialPressureMmHg < 25 || p.MeanArterialPressureMmHg > 200 {
		return fmt.Errorf("mean_arterial_pressure_mmhg must be in [25, 200], got %d", p.MeanArterialPressureMmHg)
	}
	if p.PulsusParadoxusMmHg < 0 || p.PulsusParadoxusMmHg > 60 {
		return fmt.Errorf("pulsus_paradoxus_mmhg must be in [0, 60], got %d", p.PulsusParadoxusMmHg)
	}
	if p.EffusionPCVPercent < 0.0 || p.EffusionPCVPercent > 75.0 {
		return fmt.Errorf("effusion_pcv_percent must be in [0.0, 75.0], got %v", p.EffusionPCVPercent)
	}
	return nil
}

// PericardiocentesisPlan captures the right 4th–5th ICS cardiac notch approach,
// cranial to 5th rib at costochondral junction landmark, 2% Lidocaine local block,
// IV Lidocaine 2 mg/kg arrhythmia standby, crystalloid preload expansion (10 mL/kg),
// clot-tube iatrogenic chamber puncture check, and zero-furosemide diuretic guard.
type PericardiocentesisPlan struct {
	ApproachSideAndICS              string  `json:"approach_side_and_ics"`
	CostochondralJunctionLandmark   string  `json:"costochondral_junction_landmark"`
	Lidocaine2PctInfiltrationMl     float64 `json:"lidocaine_2_pct_infiltration_ml"`
	LidocaineIV2MgKgArrhythmiaMg    float64 `json:"lidocaine_iv_2_mg_kg_arrhythmia_mg"`
	CrystalloidPreloadExpansionMl   float64 `json:"crystalloid_preload_expansion_ml"`
	ClotTubeIatrogenicPunctureCheck bool    `json:"clot_tube_iatrogenic_puncture_check"`
	ZeroFurosemideDiureticGuard     bool    `json:"zero_furosemide_diuretic_guard"`
	TamponadePitfallAlert           string  `json:"tamponade_pitfall_alert"`
	ClinicalRationale               string  `json:"clinical_rationale"`
}

// PericardiocentesisTapCheck records a pericardiocentesis tap aspiration volume,
// red-top clot tube non-clotting verification, ECG ventricular arrhythmia status,
// and post-tap hemodynamic response.
type PericardiocentesisTapCheck struct {
	Timestamp                   time.Time `json:"timestamp"`
	VolumeAspiratedMl           float64   `json:"volume_aspirated_ml"`
	SampleNonClottingVerified   bool      `json:"sample_non_clotting_verified"`
	VentricularArrhythmiasNoted bool      `json:"ventricular_arrhythmias_noted"`
	PostTapMAPMmHg              int       `json:"post_tap_map_mmhg"`
	PostTapHeartRateBpm         int       `json:"post_tap_heart_rate_bpm"`
	ClinicianCallsign           string    `json:"clinician_callsign"`
	Notes                       string    `json:"notes"`
}

// Validate enforces clinical invariants on PericardiocentesisTapCheck, including
// SampleNonClottingVerified == true to rule out iatrogenic intracardiac chamber puncture.
func (c PericardiocentesisTapCheck) Validate() error {
	if c.VolumeAspiratedMl <= 0.0 || c.VolumeAspiratedMl > 2500.0 {
		return fmt.Errorf("volume_aspirated_ml must be in (0.0, 2500.0], got %v", c.VolumeAspiratedMl)
	}
	if !c.SampleNonClottingVerified {
		return fmt.Errorf("sample_non_clotting_verified must be true in red-top clot tube to rule out iatrogenic intracardiac chamber puncture")
	}
	if c.PostTapMAPMmHg < 25 || c.PostTapMAPMmHg > 200 {
		return fmt.Errorf("post_tap_map_mmhg must be in [25, 200], got %d", c.PostTapMAPMmHg)
	}
	if c.PostTapHeartRateBpm < 30 || c.PostTapHeartRateBpm > 300 {
		return fmt.Errorf("post_tap_heart_rate_bpm must be in [30, 300], got %d", c.PostTapHeartRateBpm)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// TamponadeDischargeClearance certifies resolved right atrial diastolic collapse,
// normotensive MAP (75–120 mmHg), 24h repeat TFAST showing no pericardial effusion reaccumulation,
// stable sinus rhythm without VPCs, and Yunnan Baiyao / oncology follow-up owner education.
type TamponadeDischargeClearance struct {
	ClearedAt                          time.Time `json:"cleared_at"`
	FinalMAPMmHg                       int       `json:"final_map_mmhg"`
	RightAtrialCollapseResolved        bool      `json:"right_atrial_collapse_resolved"`
	RepeatTfast24HrNoReaccumulation    bool      `json:"repeat_tfast_24hr_no_reaccumulation"`
	SinusRhythmNoVPCs                  bool      `json:"sinus_rhythm_no_vpcs"`
	YunnanBaiyaoOrOncologyPlanEducated bool      `json:"yunnan_baiyao_or_oncology_plan_educated"`
	VetLicense                         string    `json:"vet_license"`
	Summary                            string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on TamponadeDischargeClearance.
func (d TamponadeDischargeClearance) Validate() error {
	if d.FinalMAPMmHg < 75 || d.FinalMAPMmHg > 120 {
		return fmt.Errorf("final_map_mmhg must be in [75, 120], got %d", d.FinalMAPMmHg)
	}
	if !d.RightAtrialCollapseResolved {
		return fmt.Errorf("right_atrial_collapse_resolved must be true for discharge clearance")
	}
	if !d.RepeatTfast24HrNoReaccumulation {
		return fmt.Errorf("repeat_tfast_24hr_no_reaccumulation must be true for discharge clearance")
	}
	if !d.SinusRhythmNoVPCs {
		return fmt.Errorf("sinus_rhythm_no_vpcs must be true for discharge clearance")
	}
	if !d.YunnanBaiyaoOrOncologyPlanEducated {
		return fmt.Errorf("yunnan_baiyao_or_oncology_plan_educated must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// PericardialTamponadeOperation represents a Pericardial Effusion, Cardiac Tamponade,
// Electrical Alternans & Right-Sided 4th–5th ICS Pericardiocentesis triage operation.
type PericardialTamponadeOperation struct {
	ID                                string                       `json:"id"`
	IncidentID                        string                       `json:"incident_id"`
	PetName                           string                       `json:"pet_name"`
	Status                            PericardialTamponadeStatus   `json:"status"`
	FurosemideLeftCorpnaryHazardAlert bool                         `json:"furosemide_left_coronary_hazard_alert"`
	PatientProfile                    TamponadePatientProfile      `json:"patient_profile"`
	CentresisPlan                     *PericardiocentesisPlan      `json:"centesis_plan,omitempty"`
	TapChecks                         []PericardiocentesisTapCheck `json:"tap_checks"`
	DischargeClearance                *TamponadeDischargeClearance `json:"discharge_clearance,omitempty"`
	CreatedAt                         time.Time                    `json:"created_at"`
	UpdatedAt                         time.Time                    `json:"updated_at"`
}

// Validate checks domain invariants on PericardialTamponadeOperation.
func (op PericardialTamponadeOperation) Validate() error {
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
