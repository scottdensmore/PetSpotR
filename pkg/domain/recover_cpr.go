package domain

import (
	"fmt"
	"strings"
	"time"
)

// RecoverCPRStatus represents the lifecycle state of a Cardiopulmonary Resuscitation (RECOVER Guidelines),
// Open/Closed-Chest Compression, EtCO2 > 15 mmHg & Defibrillation triage operation.
type RecoverCPRStatus string

const (
	CPRStatusArrestInProgressCritical RecoverCPRStatus = "CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL"
	CPRStatusEpiDefibCycleStaged      RecoverCPRStatus = "RECOVER_2MIN_CYCLE_EPI_DEFIB_STAGED"
	CPRStatusROSCPostArrestCleared    RecoverCPRStatus = "ROSC_ACHIEVED_NORMOCAPNIC_PCA_STABLE"
)

// IsValid returns true when RecoverCPRStatus is a supported lifecycle state.
func (s RecoverCPRStatus) IsValid() bool {
	switch s {
	case CPRStatusArrestInProgressCritical,
		CPRStatusEpiDefibCycleStaged,
		CPRStatusROSCPostArrestCleared:
		return true
	default:
		return false
	}
}

const (
	CPRSpeciesCanineRoundChestCardiac      = "CANINE_ROUND_CHEST_CARDIAC_PUMP"
	CPRSpeciesCanineKeelChestThoracic      = "CANINE_KEEL_CHEST_THORACIC_PUMP"
	CPRSpeciesCanineBarrelChestSupine      = "CANINE_BARREL_CHEST_SUPINE"
	CPRSpeciesFelineCircumferentialCardiac = "FELINE_CIRCUMFERENTIAL_CARDIAC"
)

// IsValidCPRSpecies returns true if species is a recognized RECOVER thoracic conformation classification.
func IsValidCPRSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case CPRSpeciesCanineRoundChestCardiac,
		CPRSpeciesCanineKeelChestThoracic,
		CPRSpeciesCanineBarrelChestSupine,
		CPRSpeciesFelineCircumferentialCardiac:
		return true
	default:
		return false
	}
}

const (
	CPRRhythmAsystolePEANonShockable  = "ASYSTOLE_PEA_NON_SHOCKABLE"
	CPRRhythmVFibPulselessVTShockable = "VFIB_PULSELESS_VT_SHOCKABLE"
	CPRRhythmVagalBradycardicArrest   = "VAGAL_BRADYCARDIC_ARREST"
)

// IsValidCPRArrestRhythm returns true if rhythm is a recognized RECOVER arrest ECG rhythm.
func IsValidCPRArrestRhythm(rhythm string) bool {
	switch strings.ToUpper(strings.TrimSpace(rhythm)) {
	case CPRRhythmAsystolePEANonShockable,
		CPRRhythmVFibPulselessVTShockable,
		CPRRhythmVagalBradycardicArrest:
		return true
	default:
		return false
	}
}

// CPRPatientProfile captures thoracic conformation, weight, arrest rhythm, capnography EtCO2 (mmHg),
// completed 2-minute compression cycles, and presence of reversible opioid/alpha-2/benzodiazepine agents.
type CPRPatientProfile struct {
	Species                       string  `json:"species"`
	Breed                         string  `json:"breed,omitempty"`
	WeightKg                      float64 `json:"weight_kg"`
	ArrestRhythm                  string  `json:"arrest_rhythm"`
	EndTidalCO2MmHg               float64 `json:"end_tidal_co2_mmhg"`
	CompressionCyclesCompleted    int     `json:"compression_cycles_completed"`
	ReversibleOpioidAlpha2Present bool    `json:"reversible_opioid_alpha2_present"`
}

// Validate enforces clinical invariants on CPRPatientProfile.
func (p CPRPatientProfile) Validate() error {
	if !IsValidCPRSpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_ROUND_CHEST_CARDIAC_PUMP, CANINE_KEEL_CHEST_THORACIC_PUMP, CANINE_BARREL_CHEST_SUPINE, or FELINE_CIRCUMFERENTIAL_CARDIAC, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if !IsValidCPRArrestRhythm(p.ArrestRhythm) {
		return fmt.Errorf("arrest_rhythm must be ASYSTOLE_PEA_NON_SHOCKABLE, VFIB_PULSELESS_VT_SHOCKABLE, or VAGAL_BRADYCARDIC_ARREST, got %q", p.ArrestRhythm)
	}
	if p.EndTidalCO2MmHg < 0.0 || p.EndTidalCO2MmHg > 100.0 {
		return fmt.Errorf("end_tidal_co2_mmhg must be in [0.0, 100.0], got %v", p.EndTidalCO2MmHg)
	}
	if p.CompressionCyclesCompleted < 0 || p.CompressionCyclesCompleted > 30 {
		return fmt.Errorf("compression_cycles_completed must be in [0, 30], got %d", p.CompressionCyclesCompleted)
	}
	return nil
}

// RecoverALSProtocolPlan captures conformation-specific hand placement, 100-120/min compression rate,
// strict 10 breaths/min ventilation rate, low-dose Epinephrine (0.01 mg/kg IV/IO every other 2-min cycle),
// Atropine (0.04 mg/kg IV/IO), biphasic defibrillation energy (2-4 J/kg), Amiodarone (5 mg/kg IV/IO),
// Naloxone/Atipamezole/Flumazenil reversal flag, and anti-hyperventilation / high-dose Epinephrine guard.
type RecoverALSProtocolPlan struct {
	HandPlacementTechnique                      string  `json:"hand_placement_technique"`
	TargetCompressionRateBpm                    int     `json:"target_compression_rate_bpm"`
	TargetVentilationRateBpm                    int     `json:"target_ventilation_rate_bpm"`
	LowDoseEpinephrine001MgKgMg                 float64 `json:"low_dose_epinephrine_001_mg_kg_mg"`
	Atropine004MgKgDoseMg                       float64 `json:"atropine_004_mg_kg_dose_mg"`
	BiphasicDefibJoules                         float64 `json:"biphasic_defib_joules"`
	Amiodarone5MgKgDoseMg                       float64 `json:"amiodarone_5_mg_kg_dose_mg"`
	NaloxoneAtipamezoleReversalIndicated        bool    `json:"naloxone_atipamezole_reversal_indicated"`
	ZeroHyperventilationEveryOtherCycleEpiGuard bool    `json:"zero_hyperventilation_every_other_cycle_epi_guard"`
	RecoverPitfallAlert                         string  `json:"recover_pitfall_alert"`
	ClinicalRationale                           string  `json:"clinical_rationale"`
}

// CPRTwoMinuteCycleCheck records an uninterrupted 2-minute BLS/ALS compression cycle check,
// capnography EtCO2 (target > 15 mmHg for adequate perfusion, abrupt jump > 35 mmHg indicating ROSC),
// compressor rotation, and rhythm check pause duration (<= 10 seconds).
type CPRTwoMinuteCycleCheck struct {
	Timestamp            time.Time `json:"timestamp"`
	CycleNumber          int       `json:"cycle_number"`
	EndTidalCO2MmHg      float64   `json:"end_tidal_co2_mmhg"`
	RhythmAt2MinPause    string    `json:"rhythm_at_2_min_pause"`
	CompressorRotated    bool      `json:"compressor_rotated"`
	PauseDurationSeconds int       `json:"pause_duration_seconds"`
	ClinicianCallsign    string    `json:"clinician_callsign"`
	Notes                string    `json:"notes"`
}

// Validate enforces RECOVER 2-minute cycle check invariants, including PauseDurationSeconds <= 10.
func (c CPRTwoMinuteCycleCheck) Validate() error {
	if c.CycleNumber < 1 || c.CycleNumber > 30 {
		return fmt.Errorf("cycle_number must be in [1, 30], got %d", c.CycleNumber)
	}
	if c.EndTidalCO2MmHg < 0.0 || c.EndTidalCO2MmHg > 100.0 {
		return fmt.Errorf("end_tidal_co2_mmhg must be in [0.0, 100.0], got %v", c.EndTidalCO2MmHg)
	}
	if strings.TrimSpace(c.RhythmAt2MinPause) == "" {
		return fmt.Errorf("rhythm_at_2_min_pause is required")
	}
	if !c.CompressorRotated {
		return fmt.Errorf("compressor_rotated must be true every 2-minute BLS cycle per RECOVER guidelines")
	}
	if c.PauseDurationSeconds < 1 || c.PauseDurationSeconds > 10 {
		return fmt.Errorf("pause_duration_seconds must be in [1, 10] (rhythm checks must never interrupt chest compressions > 10 seconds), got %d", c.PauseDurationSeconds)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// ROSCDischargeClearance certifies sustained Return of Spontaneous Circulation (ROSC >= 20 min),
// normocapnic ventilation (EtCO2 35-45 mmHg), normotensive perfusion (MAP 80-120 mmHg),
// SpO2 titrated to 94-98% avoiding hyperoxic free-radical injury, and post-cardiac arrest temperature control.
type ROSCDischargeClearance struct {
	ClearedAt                                     time.Time `json:"cleared_at"`
	SustainedROSCMinutes                          int       `json:"sustained_rosc_minutes"`
	FinalEtCO2MmHg                                float64   `json:"final_etco2_mmhg"`
	FinalMAPMmHg                                  int       `json:"final_map_mmhg"`
	SpO2Titrated94To98NoHyperoxia                 bool      `json:"spo2_titrated_94_to_98_no_hyperoxia"`
	PermissiveHypothermiaOrNormothermiaMaintained bool      `json:"permissive_hypothermia_or_normothermia_maintained"`
	VetLicense                                    string    `json:"vet_license"`
	Summary                                       string    `json:"summary"`
}

// Validate enforces post-cardiac arrest care (PCA) clearance invariants on ROSCDischargeClearance.
func (d ROSCDischargeClearance) Validate() error {
	if d.SustainedROSCMinutes < 20 || d.SustainedROSCMinutes > 1440 {
		return fmt.Errorf("sustained_rosc_minutes must be >= 20 (in [20, 1440]), got %d", d.SustainedROSCMinutes)
	}
	if d.FinalEtCO2MmHg < 35.0 || d.FinalEtCO2MmHg > 45.0 {
		return fmt.Errorf("final_etco2_mmhg must be in [35.0, 45.0] for normocapnic post-arrest cerebral perfusion, got %v", d.FinalEtCO2MmHg)
	}
	if d.FinalMAPMmHg < 80 || d.FinalMAPMmHg > 120 {
		return fmt.Errorf("final_map_mmhg must be in [80, 120], got %d", d.FinalMAPMmHg)
	}
	if !d.SpO2Titrated94To98NoHyperoxia {
		return fmt.Errorf("spo2_titrated_94_to_98_no_hyperoxia must be true to prevent hyperoxic reperfusion injury")
	}
	if !d.PermissiveHypothermiaOrNormothermiaMaintained {
		return fmt.Errorf("permissive_hypothermia_or_normothermia_maintained must be true for post-cardiac arrest neuroprotection")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// RecoverCPROperation represents a Cardiopulmonary Resuscitation (RECOVER Guidelines),
// Open/Closed-Chest Compression, EtCO2 > 15 mmHg & Defibrillation triage operation.
type RecoverCPROperation struct {
	ID                               string                   `json:"id"`
	IncidentID                       string                   `json:"incident_id"`
	PetName                          string                   `json:"pet_name"`
	Status                           RecoverCPRStatus         `json:"status"`
	HyperventilationHighDoseEpiAlert bool                     `json:"hyperventilation_high_dose_epi_alert"`
	PatientProfile                   CPRPatientProfile        `json:"patient_profile"`
	ALSProtocolPlan                  *RecoverALSProtocolPlan  `json:"als_protocol_plan,omitempty"`
	CycleChecks                      []CPRTwoMinuteCycleCheck `json:"cycle_checks"`
	ROSCClearance                    *ROSCDischargeClearance  `json:"rosc_clearance,omitempty"`
	CreatedAt                        time.Time                `json:"created_at"`
	UpdatedAt                        time.Time                `json:"updated_at"`
}

// Validate checks domain invariants on RecoverCPROperation.
func (op RecoverCPROperation) Validate() error {
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
