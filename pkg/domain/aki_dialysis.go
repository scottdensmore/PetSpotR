package domain

import (
	"fmt"
	"strings"
	"time"
)

// AKIDialysisStatus represents the lifecycle state of an Acute Kidney Injury (AKI),
// Lily/Ethylene Glycol/Grape Nephrotoxicity, Oliguria/Anuria & Furosemide/Diltiazem/Dialysis triage operation.
type AKIDialysisStatus string

const (
	AKIStatusOliguricAzotemiaCritical   AKIDialysisStatus = "OLIGURIC_ANURIC_AZOTEMIA_CRITICAL"
	AKIStatusFomepizoleDiuresisStaged   AKIDialysisStatus = "FOMEPIZOLE_INS_OUTS_DIURESIS_STAGED"
	AKIStatusPolyuricNonAzotemicCleared AKIDialysisStatus = "POLYURIC_CREATININE_PLATEAU_CLEARED"
)

// IsValid returns true when AKIDialysisStatus is a supported lifecycle state.
func (s AKIDialysisStatus) IsValid() bool {
	switch s {
	case AKIStatusOliguricAzotemiaCritical,
		AKIStatusFomepizoleDiuresisStaged,
		AKIStatusPolyuricNonAzotemicCleared:
		return true
	default:
		return false
	}
}

const (
	AKISpeciesFelineLiliumNephrotoxicity    = "FELINE_LILIUM_NEPHROTOXICITY"
	AKISpeciesCanineEthyleneGlycolOxalate   = "CANINE_ETHYLENE_GLYCOL_OXALATE"
	AKISpeciesCanineVitisGrapeLeptospirosis = "CANINE_VITIS_GRAPE_LEPTOSPIROSIS"
)

// IsValidAKISpecies returns true if species is a recognized AKI nephrotoxicity / etiology classification.
func IsValidAKISpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case AKISpeciesFelineLiliumNephrotoxicity,
		AKISpeciesCanineEthyleneGlycolOxalate,
		AKISpeciesCanineVitisGrapeLeptospirosis:
		return true
	default:
		return false
	}
}

// AKIPatientProfile captures signalment, IRIS AKI grade, serum creatinine, serum potassium,
// closed-system urine output (UOP mL/kg/hr), iatrogenic fluid overload body weight gain %,
// and pathognomonic calcium oxalate monohydrate ("picket-fence") crystalluria.
type AKIPatientProfile struct {
	Species                           string  `json:"species"`
	Breed                             string  `json:"breed,omitempty"`
	WeightKg                          float64 `json:"weight_kg"`
	IRISGrade                         int     `json:"iris_grade"`
	SerumCreatinineMgDl               float64 `json:"serum_creatinine_mg_dl"`
	SerumPotassiumMeqL                float64 `json:"serum_potassium_meq_l"`
	UrineOutputMlKgHr                 float64 `json:"urine_output_ml_kg_hr"`
	BodyWeightGainPercent             float64 `json:"body_weight_gain_percent"`
	CalciumOxalateMonohydrateCrystals bool    `json:"calcium_oxalate_monohydrate_crystals"`
}

// Validate enforces clinical invariants on AKIPatientProfile.
func (p AKIPatientProfile) Validate() error {
	if !IsValidAKISpecies(p.Species) {
		return fmt.Errorf("species must be FELINE_LILIUM_NEPHROTOXICITY, CANINE_ETHYLENE_GLYCOL_OXALATE, or CANINE_VITIS_GRAPE_LEPTOSPIROSIS, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.IRISGrade < 1 || p.IRISGrade > 5 {
		return fmt.Errorf("iris_grade must be in [1, 5], got %d", p.IRISGrade)
	}
	if p.SerumCreatinineMgDl < 0.3 || p.SerumCreatinineMgDl > 30.0 {
		return fmt.Errorf("serum_creatinine_mg_dl must be in [0.3, 30.0], got %v", p.SerumCreatinineMgDl)
	}
	if p.SerumPotassiumMeqL < 2.0 || p.SerumPotassiumMeqL > 12.0 {
		return fmt.Errorf("serum_potassium_meq_l must be in [2.0, 12.0], got %v", p.SerumPotassiumMeqL)
	}
	if p.UrineOutputMlKgHr < 0.0 || p.UrineOutputMlKgHr > 25.0 {
		return fmt.Errorf("urine_output_ml_kg_hr must be in [0.0, 25.0], got %v", p.UrineOutputMlKgHr)
	}
	if p.BodyWeightGainPercent < -15.0 || p.BodyWeightGainPercent > 25.0 {
		return fmt.Errorf("body_weight_gain_percent must be in [-15.0, 25.0], got %v", p.BodyWeightGainPercent)
	}
	return nil
}

// AKIDiuresisDialysisPlan captures calculated Fomepizole (4-MP) loading dose (20 mg/kg canine vs 125 mg/kg feline),
// "Ins-and-Outs" hourly IV fluid rate matching urine output + 20 mL/kg/day insensible loss,
// oliguric-to-polyuric conversion Furosemide 2.0 mg/kg IV + Diltiazem CRI 0.3 mcg/kg/min,
// intermittent hemodialysis / CRRT indication, and strict zero-force-fluid-in-anuria guard.
type AKIDiuresisDialysisPlan struct {
	FomepizoleLoadingDoseMg     float64 `json:"fomepizole_loading_dose_mg"`
	InsAndOutsFluidRateMlHr     float64 `json:"ins_and_outs_fluid_rate_ml_hr"`
	FurosemideConversionBolusMg float64 `json:"furosemide_conversion_bolus_mg"`
	DiltiazemCRI03McgKgMin      float64 `json:"diltiazem_cri_03_mcg_kg_min"`
	HemodialysisIndicated       bool    `json:"hemodialysis_indicated"`
	ZeroForceFluidInAnuriaGuard bool    `json:"zero_force_fluid_in_anuria_guard"`
	AKIPitfallAlert             string  `json:"aki_pitfall_alert"`
	ClinicalRationale           string  `json:"clinical_rationale"`
}

// AKIRenalTelemetryCheck records a serial closed-system urine output (UOP), serum creatinine,
// serum potassium, body weight, and fluid in/out balance telemetry check.
type AKIRenalTelemetryCheck struct {
	Timestamp           time.Time `json:"timestamp"`
	UrineOutputMlKgHr   float64   `json:"urine_output_ml_kg_hr"`
	SerumCreatinineMgDl float64   `json:"serum_creatinine_mg_dl"`
	SerumPotassiumMeqL  float64   `json:"serum_potassium_meq_l"`
	BodyWeightKg        float64   `json:"body_weight_kg"`
	FluidInOutBalanceMl float64   `json:"fluid_in_out_balance_ml"`
	ClinicianCallsign   string    `json:"clinician_callsign"`
	Notes               string    `json:"notes"`
}

// Validate enforces clinical invariants on AKIRenalTelemetryCheck.
func (c AKIRenalTelemetryCheck) Validate() error {
	if c.UrineOutputMlKgHr < 0.0 || c.UrineOutputMlKgHr > 25.0 {
		return fmt.Errorf("urine_output_ml_kg_hr must be in [0.0, 25.0], got %v", c.UrineOutputMlKgHr)
	}
	if c.SerumCreatinineMgDl < 0.3 || c.SerumCreatinineMgDl > 30.0 {
		return fmt.Errorf("serum_creatinine_mg_dl must be in [0.3, 30.0], got %v", c.SerumCreatinineMgDl)
	}
	if c.SerumPotassiumMeqL < 2.0 || c.SerumPotassiumMeqL > 12.0 {
		return fmt.Errorf("serum_potassium_meq_l must be in [2.0, 12.0], got %v", c.SerumPotassiumMeqL)
	}
	if c.BodyWeightKg <= 0 || c.BodyWeightKg > 120.0 {
		return fmt.Errorf("body_weight_kg must be in (0, 120.0], got %v", c.BodyWeightKg)
	}
	if c.FluidInOutBalanceMl < -2000.0 || c.FluidInOutBalanceMl > 2000.0 {
		return fmt.Errorf("fluid_in_out_balance_ml must be in [-2000.0, 2000.0], got %v", c.FluidInOutBalanceMl)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// AKIDischargeClearance certifies polyuric / normouric recovery (UOP >= 1.5 mL/kg/hr),
// non-azotemic / plateaued serum creatinine (<= 2.5 mg/dL), normokalemia (3.5-5.5 mEq/L),
// euvolemia without pulmonary edema, and owner outpatient subcutaneous fluid / renal diet education.
type AKIDischargeClearance struct {
	ClearedAt                    time.Time `json:"cleared_at"`
	FinalUrineOutputMlKgHr       float64   `json:"final_urine_output_ml_kg_hr"`
	FinalSerumCreatinineMgDl     float64   `json:"final_serum_creatinine_mg_dl"`
	FinalSerumPotassiumMeqL      float64   `json:"final_serum_potassium_meq_l"`
	EuvolemicNoPulmonaryEdema    bool      `json:"euvolemic_no_pulmonary_edema"`
	OutpatientSubQFluidsEducated bool      `json:"outpatient_subq_fluids_educated"`
	VetLicense                   string    `json:"vet_license"`
	Summary                      string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on AKIDischargeClearance.
func (d AKIDischargeClearance) Validate() error {
	if d.FinalUrineOutputMlKgHr < 1.5 || d.FinalUrineOutputMlKgHr > 20.0 {
		return fmt.Errorf("final_urine_output_ml_kg_hr must be >= 1.5 (in [1.5, 20.0]), got %v", d.FinalUrineOutputMlKgHr)
	}
	if d.FinalSerumCreatinineMgDl < 0.3 || d.FinalSerumCreatinineMgDl > 2.5 {
		return fmt.Errorf("final_serum_creatinine_mg_dl must be <= 2.5 (in [0.3, 2.5]), got %v", d.FinalSerumCreatinineMgDl)
	}
	if d.FinalSerumPotassiumMeqL < 3.5 || d.FinalSerumPotassiumMeqL > 5.5 {
		return fmt.Errorf("final_serum_potassium_meq_l must be in [3.5, 5.5], got %v", d.FinalSerumPotassiumMeqL)
	}
	if !d.EuvolemicNoPulmonaryEdema {
		return fmt.Errorf("euvolemic_no_pulmonary_edema must be true for discharge clearance")
	}
	if !d.OutpatientSubQFluidsEducated {
		return fmt.Errorf("outpatient_subq_fluids_educated must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// AKIDialysisOperation represents an Acute Kidney Injury (AKI), Lily/Ethylene Glycol/Grape Nephrotoxicity,
// Oliguria/Anuria & Furosemide/Diltiazem/Dialysis triage operation.
type AKIDialysisOperation struct {
	ID                                 string                   `json:"id"`
	IncidentID                         string                   `json:"incident_id"`
	PetName                            string                   `json:"pet_name"`
	Status                             AKIDialysisStatus        `json:"status"`
	AnuricFluidOverloadFomepizoleAlert bool                     `json:"anuric_fluid_overload_fomepizole_alert"`
	PatientProfile                     AKIPatientProfile        `json:"patient_profile"`
	DiuresisPlan                       *AKIDiuresisDialysisPlan `json:"diuresis_plan,omitempty"`
	RenalChecks                        []AKIRenalTelemetryCheck `json:"renal_checks"`
	DischargeClearance                 *AKIDischargeClearance   `json:"discharge_clearance,omitempty"`
	CreatedAt                          time.Time                `json:"created_at"`
	UpdatedAt                          time.Time                `json:"updated_at"`
}

// Validate checks domain invariants on AKIDialysisOperation.
func (op AKIDialysisOperation) Validate() error {
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
