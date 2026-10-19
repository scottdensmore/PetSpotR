package domain

import (
	"fmt"
	"strings"
	"time"
)

// HemoabdomenStatus represents the lifecycle state of an Acute Traumatic Hemoabdomen,
// Autotransfusion, Centesis Non-Clotting & Norepinephrine/TXA Resuscitation operation.
type HemoabdomenStatus string

const (
	HemoabdomenStatusHemorrhagicShockCritical  HemoabdomenStatus = "HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL"
	HemoabdomenStatusAutotransfusionTXAStaged  HemoabdomenStatus = "AUTOTRANSFUSION_TXA_HYPOTENSIVE_STAGED"
	HemoabdomenStatusHemostasisRestoredCleared HemoabdomenStatus = "HEMOSTASIS_SPLENECTOMY_NORMOTENSIVE_SAFE"
)

// IsValid returns true when HemoabdomenStatus is a supported lifecycle state.
func (s HemoabdomenStatus) IsValid() bool {
	switch s {
	case HemoabdomenStatusHemorrhagicShockCritical,
		HemoabdomenStatusAutotransfusionTXAStaged,
		HemoabdomenStatusHemostasisRestoredCleared:
		return true
	default:
		return false
	}
}

const (
	HemoabdomenSpeciesCanineSplenicHemangiosarcoma = "CANINE_SPLENIC_HEMANGIOSARCOMA"
	HemoabdomenSpeciesCanineBluntTraumaHepatic     = "CANINE_BLUNT_TRAUMA_HEPATIC"
	HemoabdomenSpeciesFelineRetroperitonealTrauma  = "FELINE_RETROPERITONEAL_TRAUMA"
)

// IsValidHemoabdomenSpecies returns true if species is one of the supported hemoabdomen presentations.
func IsValidHemoabdomenSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case HemoabdomenSpeciesCanineSplenicHemangiosarcoma,
		HemoabdomenSpeciesCanineBluntTraumaHepatic,
		HemoabdomenSpeciesFelineRetroperitonealTrauma:
		return true
	default:
		return false
	}
}

// HemoabdomenPatientProfile captures patient signalment, AFAST AFS 0-4 free fluid score,
// effusion vs peripheral PCV, mean arterial pressure (MAP), plasma lactate, non-clotting centesis verification,
// and suspected neoplasia flag.
type HemoabdomenPatientProfile struct {
	Species                     string  `json:"species"`
	Breed                       string  `json:"breed,omitempty"`
	WeightKg                    float64 `json:"weight_kg"`
	AfastAFS4Score              int     `json:"afast_afs4_score"`
	EffusionPCVPercent          float64 `json:"effusion_pcv_percent"`
	PeripheralPCVPercent        float64 `json:"peripheral_pcv_percent"`
	MeanArterialPressureMmHg    int     `json:"mean_arterial_pressure_mmhg"`
	PlasmaLactateMmolL          float64 `json:"plasma_lactate_mmol_l"`
	EffusionNonClottingVerified bool    `json:"effusion_non_clotting_verified"`
	SuspectedNeoplasia          bool    `json:"suspected_neoplasia"`
}

// Validate enforces clinical invariants on HemoabdomenPatientProfile.
func (p HemoabdomenPatientProfile) Validate() error {
	if !IsValidHemoabdomenSpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_SPLENIC_HEMANGIOSARCOMA, CANINE_BLUNT_TRAUMA_HEPATIC, or FELINE_RETROPERITONEAL_TRAUMA, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.AfastAFS4Score < 0 || p.AfastAFS4Score > 4 {
		return fmt.Errorf("afast_afs4_score must be in [0, 4], got %d", p.AfastAFS4Score)
	}
	if p.EffusionPCVPercent < 0.0 || p.EffusionPCVPercent > 75.0 {
		return fmt.Errorf("effusion_pcv_percent must be in [0.0, 75.0], got %v", p.EffusionPCVPercent)
	}
	if p.PeripheralPCVPercent < 5.0 || p.PeripheralPCVPercent > 75.0 {
		return fmt.Errorf("peripheral_pcv_percent must be in [5.0, 75.0], got %v", p.PeripheralPCVPercent)
	}
	if p.MeanArterialPressureMmHg < 20 || p.MeanArterialPressureMmHg > 180 {
		return fmt.Errorf("mean_arterial_pressure_mmhg must be in [20, 180], got %d", p.MeanArterialPressureMmHg)
	}
	if p.PlasmaLactateMmolL < 0.5 || p.PlasmaLactateMmolL > 25.0 {
		return fmt.Errorf("plasma_lactate_mmol_l must be in [0.5, 25.0], got %v", p.PlasmaLactateMmolL)
	}
	return nil
}

// HemoabdomenResuscitationPlan captures calculated permissive hypotensive MAP targets, conservative crystalloid aliquots,
// Tranexamic Acid (TXA) antifibrinolytic dosing, autotransfusion indication & 170-260 um filter requirements,
// Norepinephrine CRI rate, and zero abdominal compression wrap guards.
type HemoabdomenResuscitationPlan struct {
	PermissiveHypotensionTargetMAPMmHg int     `json:"permissive_hypotension_target_map_mmhg"`
	CrystalloidAliquotMlKg             float64 `json:"crystalloid_aliquot_ml_kg"`
	TranexamicAcid10MgKgDoseMg         float64 `json:"tranexamic_acid_10mg_kg_dose_mg"`
	AutotransfusionIndicated           bool    `json:"autotransfusion_indicated"`
	AutotransfusionMaxCollectMl        float64 `json:"autotransfusion_max_collect_ml"`
	MicronFilterSizeUm                 int     `json:"micron_filter_size_um"`
	NorepinephrineCRIMcgKgMin          float64 `json:"norepinephrine_cri_mcg_kg_min"`
	ZeroAbdominalCompressionWrapGuard  bool    `json:"zero_abdominal_compression_wrap_guard"`
	HemoabdomenPitfallAlert            string  `json:"hemoabdomen_pitfall_alert"`
	ClinicalRationale                  string  `json:"clinical_rationale"`
}

// HemoabdomenHemodynamicCheck records a serial hemodynamic and autotransfusion ICU assessment.
type HemoabdomenHemodynamicCheck struct {
	Timestamp                time.Time `json:"timestamp"`
	MeanArterialPressureMmHg int       `json:"mean_arterial_pressure_mmhg"`
	HeartRateBpm             int       `json:"heart_rate_bpm"`
	PeripheralPCVPercent     float64   `json:"peripheral_pcv_percent"`
	TotalSolidsGdl           float64   `json:"total_solids_gdl"`
	PlasmaLactateMmolL       float64   `json:"plasma_lactate_mmol_l"`
	AutotransfusedVolumeMl   float64   `json:"autotransfused_volume_ml"`
	ClinicianCallsign        string    `json:"clinician_callsign"`
	Notes                    string    `json:"notes"`
}

// Validate enforces clinical invariants on HemoabdomenHemodynamicCheck.
func (c HemoabdomenHemodynamicCheck) Validate() error {
	if c.MeanArterialPressureMmHg < 20 || c.MeanArterialPressureMmHg > 180 {
		return fmt.Errorf("mean_arterial_pressure_mmhg must be in [20, 180], got %d", c.MeanArterialPressureMmHg)
	}
	if c.HeartRateBpm < 30 || c.HeartRateBpm > 300 {
		return fmt.Errorf("heart_rate_bpm must be in [30, 300], got %d", c.HeartRateBpm)
	}
	if c.PeripheralPCVPercent < 5.0 || c.PeripheralPCVPercent > 75.0 {
		return fmt.Errorf("peripheral_pcv_percent must be in [5.0, 75.0], got %v", c.PeripheralPCVPercent)
	}
	if c.TotalSolidsGdl < 1.0 || c.TotalSolidsGdl > 15.0 {
		return fmt.Errorf("total_solids_gdl must be in [1.0, 15.0], got %v", c.TotalSolidsGdl)
	}
	if c.PlasmaLactateMmolL < 0.5 || c.PlasmaLactateMmolL > 25.0 {
		return fmt.Errorf("plasma_lactate_mmol_l must be in [0.5, 25.0], got %v", c.PlasmaLactateMmolL)
	}
	if c.AutotransfusedVolumeMl < 0.0 || c.AutotransfusedVolumeMl > 2500.0 {
		return fmt.Errorf("autotransfused_volume_ml must be in [0.0, 2500.0], got %v", c.AutotransfusedVolumeMl)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// HemoabdomenHemostasisClearance certifies surgical/spontaneous hemostasis, normotensive MAP,
// restored PCV, normalized lactate, and resolved free abdominal fluid on repeat AFAST.
type HemoabdomenHemostasisClearance struct {
	ClearedAt                    time.Time `json:"cleared_at"`
	FinalMAPMmHg                 int       `json:"final_map_mmhg"`
	FinalPeripheralPCVPercent    float64   `json:"final_peripheral_pcv_percent"`
	FinalPlasmaLactateMmolL      float64   `json:"final_plasma_lactate_mmol_l"`
	SurgicalHemostasisConfirmed  bool      `json:"surgical_hemostasis_confirmed"`
	RepeatAfastFreeFluidResolved bool      `json:"repeat_afast_free_fluid_resolved"`
	VetLicense                   string    `json:"vet_license"`
	Summary                      string    `json:"summary"`
}

// Validate enforces strict hemostasis clearance safety criteria on HemoabdomenHemostasisClearance.
func (c HemoabdomenHemostasisClearance) Validate() error {
	if c.FinalMAPMmHg < 70 || c.FinalMAPMmHg > 110 {
		return fmt.Errorf("final_map_mmhg must be in [70, 110], got %d", c.FinalMAPMmHg)
	}
	if c.FinalPeripheralPCVPercent < 25.0 || c.FinalPeripheralPCVPercent > 75.0 {
		return fmt.Errorf("final_peripheral_pcv_percent must be >= 25.0, got %v", c.FinalPeripheralPCVPercent)
	}
	if c.FinalPlasmaLactateMmolL <= 0.0 || c.FinalPlasmaLactateMmolL > 2.5 {
		return fmt.Errorf("final_plasma_lactate_mmol_l must be in (0.0, 2.5], got %v", c.FinalPlasmaLactateMmolL)
	}
	if !c.SurgicalHemostasisConfirmed {
		return fmt.Errorf("surgical_hemostasis_confirmed must be true for hemostasis clearance")
	}
	if !c.RepeatAfastFreeFluidResolved {
		return fmt.Errorf("repeat_afast_free_fluid_resolved must be true for hemostasis clearance")
	}
	if strings.TrimSpace(c.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// HemoabdomenOperation represents an Acute Traumatic Hemoabdomen, Autotransfusion, Centesis Non-Clotting
// & Norepinephrine/TXA Resuscitation operation.
type HemoabdomenOperation struct {
	ID                        string                          `json:"id"`
	IncidentID                string                          `json:"incident_id"`
	PetName                   string                          `json:"pet_name"`
	Status                    HemoabdomenStatus               `json:"status"`
	ClotPoppingNeoplasiaAlert bool                            `json:"clot_popping_neoplasia_alert"`
	PatientProfile            HemoabdomenPatientProfile       `json:"patient_profile"`
	ResuscitationPlan         *HemoabdomenResuscitationPlan   `json:"resuscitation_plan,omitempty"`
	HemodynamicChecks         []HemoabdomenHemodynamicCheck   `json:"hemodynamic_checks"`
	HemostasisClearance       *HemoabdomenHemostasisClearance `json:"hemostasis_clearance,omitempty"`
	CreatedAt                 time.Time                       `json:"created_at"`
	UpdatedAt                 time.Time                       `json:"updated_at"`
}

// Validate checks domain invariants on HemoabdomenOperation.
func (op HemoabdomenOperation) Validate() error {
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
