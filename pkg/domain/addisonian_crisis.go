package domain

import (
	"fmt"
	"strings"
	"time"
)

// AddisonianCrisisStatus represents the lifecycle state of a Hypoadrenocorticism
// (Addisonian Crisis), Na:K Ratio < 27, Dexamethasone-SP & DOCP triage operation.
type AddisonianCrisisStatus string

const (
	AddisonianStatusShockHyperkalemicCritical AddisonianCrisisStatus = "ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL"
	AddisonianStatusDexSPNaClACTHStaged       AddisonianCrisisStatus = "DEX_SP_NACL_ACTH_STIM_STAGED"
	AddisonianStatusDOCPEukalemicCleared      AddisonianCrisisStatus = "DOCP_PREDNISONE_EUKALEMIC_CLEARED"
)

// IsValid returns true when AddisonianCrisisStatus is a supported lifecycle state.
func (s AddisonianCrisisStatus) IsValid() bool {
	switch s {
	case AddisonianStatusShockHyperkalemicCritical,
		AddisonianStatusDexSPNaClACTHStaged,
		AddisonianStatusDOCPEukalemicCleared:
		return true
	default:
		return false
	}
}

const (
	AddisonianSpeciesCanineTypical    = "CANINE_TYPICAL_ADDISONS"
	AddisonianSpeciesCanineAtypical   = "CANINE_ATYPICAL_GLUCOCORTICOID"
	AddisonianSpeciesFelineHypoadreno = "FELINE_HYPOADRENOCORTICISM"
)

// IsValidAddisonianSpecies returns true if species is a recognized hypoadrenocorticism syndrome classification.
func IsValidAddisonianSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case AddisonianSpeciesCanineTypical,
		AddisonianSpeciesCanineAtypical,
		AddisonianSpeciesFelineHypoadreno:
		return true
	default:
		return false
	}
}

// AddisonianPatientProfile captures signalment, serum sodium/potassium, blood glucose,
// heart rate, systolic blood pressure, and resting cortisol telemetry.
type AddisonianPatientProfile struct {
	Species                   string  `json:"species"`
	Breed                     string  `json:"breed,omitempty"`
	WeightKg                  float64 `json:"weight_kg"`
	SerumSodiumMeqL           float64 `json:"serum_sodium_meq_l"`
	SerumPotassiumMeqL        float64 `json:"serum_potassium_meq_l"`
	BloodGlucoseMgDl          float64 `json:"blood_glucose_mg_dl"`
	HeartRateBpm              int     `json:"heart_rate_bpm"`
	SystolicBloodPressureMmHg int     `json:"systolic_blood_pressure_mmhg"`
	RestingCortisolMcgDl      float64 `json:"resting_cortisol_mcg_dl"`
}

// Validate enforces clinical invariants on AddisonianPatientProfile.
func (p AddisonianPatientProfile) Validate() error {
	if !IsValidAddisonianSpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_TYPICAL_ADDISONS, CANINE_ATYPICAL_GLUCOCORTICOID, or FELINE_HYPOADRENOCORTICISM, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.SerumSodiumMeqL < 95.0 || p.SerumSodiumMeqL > 180.0 {
		return fmt.Errorf("serum_sodium_meq_l must be in [95.0, 180.0], got %v", p.SerumSodiumMeqL)
	}
	if p.SerumPotassiumMeqL < 2.0 || p.SerumPotassiumMeqL > 12.0 {
		return fmt.Errorf("serum_potassium_meq_l must be in [2.0, 12.0], got %v", p.SerumPotassiumMeqL)
	}
	if p.BloodGlucoseMgDl < 15.0 || p.BloodGlucoseMgDl > 600.0 {
		return fmt.Errorf("blood_glucose_mg_dl must be in [15.0, 600.0], got %v", p.BloodGlucoseMgDl)
	}
	if p.HeartRateBpm < 25 || p.HeartRateBpm > 300 {
		return fmt.Errorf("heart_rate_bpm must be in [25, 300], got %d", p.HeartRateBpm)
	}
	if p.SystolicBloodPressureMmHg < 30 || p.SystolicBloodPressureMmHg > 260 {
		return fmt.Errorf("systolic_blood_pressure_mmhg must be in [30, 260], got %d", p.SystolicBloodPressureMmHg)
	}
	if p.RestingCortisolMcgDl < 0.0 || p.RestingCortisolMcgDl > 25.0 {
		return fmt.Errorf("resting_cortisol_mcg_dl must be in [0.0, 25.0], got %v", p.RestingCortisolMcgDl)
	}
	return nil
}

// AddisonianResuscitationRegimen captures calculated Na:K ratio, 0.9% NaCl shock bolus aliquot,
// Dexamethasone-SP (zero prednisolone assay cross-reactivity), Cosyntropin ACTH stimulation dose,
// 10% Calcium Gluconate cardioprotection, and max sodium correction rate (<= 10-12 mEq/L/day).
type AddisonianResuscitationRegimen struct {
	SodiumPotassiumRatio         float64 `json:"sodium_potassium_ratio"`
	IsotonicNaCl09BolusMl        float64 `json:"isotonic_nacl_09_bolus_ml"`
	DexamethasoneSP025MgKgMg     float64 `json:"dexamethasone_sp_025_mg_kg_mg"`
	CosyntropinACTHStimDoseMcg   float64 `json:"cosyntropin_acth_stim_dose_mcg"`
	CalciumGluconate10PctMl      float64 `json:"calcium_gluconate_10_pct_ml"`
	MaxSodiumRiseRateMeqLDay     float64 `json:"max_sodium_rise_rate_meq_l_day"`
	ZeroPrednisolonePreACTHGuard bool    `json:"zero_prednisolone_pre_acth_guard"`
	AddisonianPitfallAlert       string  `json:"addisonian_pitfall_alert"`
	ClinicalRationale            string  `json:"clinical_rationale"`
}

// AddisonianElectrolyteCheck records a serial electrolyte, 24h sodium rise rate,
// post-ACTH stimulation cortisol, and heart rate check.
type AddisonianElectrolyteCheck struct {
	Timestamp             time.Time `json:"timestamp"`
	SerumSodiumMeqL       float64   `json:"serum_sodium_meq_l"`
	SerumPotassiumMeqL    float64   `json:"serum_potassium_meq_l"`
	SodiumRise24HrMeqL    float64   `json:"sodium_rise_24hr_meq_l"`
	PostACTHCortisolMcgDl float64   `json:"post_acth_cortisol_mcg_dl"`
	HeartRateBpm          int       `json:"heart_rate_bpm"`
	ClinicianCallsign     string    `json:"clinician_callsign"`
	Notes                 string    `json:"notes"`
}

// Validate enforces clinical invariants on AddisonianElectrolyteCheck, including
// SodiumRise24HrMeqL <= 12.0 to prevent central pontine myelinolysis / osmotic demyelination syndrome.
func (c AddisonianElectrolyteCheck) Validate() error {
	if c.SerumSodiumMeqL < 95.0 || c.SerumSodiumMeqL > 180.0 {
		return fmt.Errorf("serum_sodium_meq_l must be in [95.0, 180.0], got %v", c.SerumSodiumMeqL)
	}
	if c.SerumPotassiumMeqL < 2.0 || c.SerumPotassiumMeqL > 12.0 {
		return fmt.Errorf("serum_potassium_meq_l must be in [2.0, 12.0], got %v", c.SerumPotassiumMeqL)
	}
	if c.SodiumRise24HrMeqL < 0.0 || c.SodiumRise24HrMeqL > 12.0 {
		return fmt.Errorf("sodium_rise_24hr_meq_l must be in [0.0, 12.0] to prevent central pontine myelinolysis / osmotic demyelination syndrome, got %v", c.SodiumRise24HrMeqL)
	}
	if c.PostACTHCortisolMcgDl < 0.0 || c.PostACTHCortisolMcgDl > 25.0 {
		return fmt.Errorf("post_acth_cortisol_mcg_dl must be in [0.0, 25.0], got %v", c.PostACTHCortisolMcgDl)
	}
	if c.HeartRateBpm < 25 || c.HeartRateBpm > 300 {
		return fmt.Errorf("heart_rate_bpm must be in [25, 300], got %d", c.HeartRateBpm)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// AddisonianDischargeClearance certifies eukalemia, Na:K ratio >= 27.0, confirmed low post-ACTH cortisol,
// DOCP 2.2 mg/kg SubQ mineralocorticoid depot administration, and physiologic oral Prednisone transition.
type AddisonianDischargeClearance struct {
	ClearedAt                        time.Time `json:"cleared_at"`
	FinalSodiumPotassiumRatio        float64   `json:"final_sodium_potassium_ratio"`
	FinalSerumPotassiumMeqL          float64   `json:"final_serum_potassium_meq_l"`
	PostACTHCortisolConfirmedLow     bool      `json:"post_acth_cortisol_confirmed_low"`
	DOCP22MgKgSubQAdministered       bool      `json:"docp_22_mg_kg_subq_administered"`
	OralPrednisonePhysiologicStarted bool      `json:"oral_prednisone_physiologic_started"`
	VetLicense                       string    `json:"vet_license"`
	Summary                          string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on AddisonianDischargeClearance.
func (d AddisonianDischargeClearance) Validate() error {
	if d.FinalSodiumPotassiumRatio < 27.0 || d.FinalSodiumPotassiumRatio > 50.0 {
		return fmt.Errorf("final_sodium_potassium_ratio must be in [27.0, 50.0], got %v", d.FinalSodiumPotassiumRatio)
	}
	if d.FinalSerumPotassiumMeqL < 3.5 || d.FinalSerumPotassiumMeqL > 5.5 {
		return fmt.Errorf("final_serum_potassium_meq_l must be in [3.5, 5.5], got %v", d.FinalSerumPotassiumMeqL)
	}
	if !d.PostACTHCortisolConfirmedLow {
		return fmt.Errorf("post_acth_cortisol_confirmed_low must be true for discharge clearance")
	}
	if !d.DOCP22MgKgSubQAdministered {
		return fmt.Errorf("docp_22_mg_kg_subq_administered must be true for discharge clearance")
	}
	if !d.OralPrednisonePhysiologicStarted {
		return fmt.Errorf("oral_prednisone_physiologic_started must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// AddisonianCrisisOperation represents a Hypoadrenocorticism (Addisonian Crisis),
// Na:K Ratio < 27, Dexamethasone-SP & DOCP triage operation.
type AddisonianCrisisOperation struct {
	ID                            string                          `json:"id"`
	IncidentID                    string                          `json:"incident_id"`
	PetName                       string                          `json:"pet_name"`
	Status                        AddisonianCrisisStatus          `json:"status"`
	OsmoticMyelinolysisAssayAlert bool                            `json:"osmotic_myelinolysis_assay_alert"`
	PatientProfile                AddisonianPatientProfile        `json:"patient_profile"`
	ResuscitationRegimen          *AddisonianResuscitationRegimen `json:"resuscitation_regimen,omitempty"`
	ElectrolyteChecks             []AddisonianElectrolyteCheck    `json:"electrolyte_checks"`
	DischargeClearance            *AddisonianDischargeClearance   `json:"discharge_clearance,omitempty"`
	CreatedAt                     time.Time                       `json:"created_at"`
	UpdatedAt                     time.Time                       `json:"updated_at"`
}

// Validate checks domain invariants on AddisonianCrisisOperation.
func (op AddisonianCrisisOperation) Validate() error {
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
