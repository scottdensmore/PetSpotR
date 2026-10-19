package domain

import (
	"fmt"
	"strings"
	"time"
)

// DKAStatus represents the lifecycle state of a Diabetic Ketoacidosis (DKA),
// Regular Insulin CRI, Hypokalemia/Hypophosphatemia & Cerebral Edema triage operation.
type DKAStatus string

const (
	DKAStatusAcidosisDehydratedCritical    DKAStatus = "KETOACIDOSIS_DEHYDRATED_CRITICAL"
	DKAStatusKClPhosInsulinCRIStaged       DKAStatus = "KCL_PHOS_REGULAR_INSULIN_CRI_STAGED"
	DKAStatusKetoneResolvedGlargineCleared DKAStatus = "KETONES_CLEARED_SUBQ_GLARGINE_READY"
)

// IsValid returns true when DKAStatus is a supported lifecycle state.
func (s DKAStatus) IsValid() bool {
	switch s {
	case DKAStatusAcidosisDehydratedCritical,
		DKAStatusKClPhosInsulinCRIStaged,
		DKAStatusKetoneResolvedGlargineCleared:
		return true
	default:
		return false
	}
}

const (
	DKASpeciesCaninePancreatitis     = "CANINE_DKA_PANCREATITIS"
	DKASpeciesFelineHepaticLipidosis = "FELINE_DKA_HEPATIC_LIPIDOSIS"
)

// IsValidDKASpecies returns true if species is a recognized DKA presentation classification.
func IsValidDKASpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case DKASpeciesCaninePancreatitis, DKASpeciesFelineHepaticLipidosis:
		return true
	default:
		return false
	}
}

// DKAPatientProfile captures signalment, acid-base, ketone, electrolyte, osmolality, and dehydration telemetry.
type DKAPatientProfile struct {
	Species                   string  `json:"species"`
	Breed                     string  `json:"breed,omitempty"`
	WeightKg                  float64 `json:"weight_kg"`
	BloodGlucoseMgDl          float64 `json:"blood_glucose_mg_dl"`
	BetaHydroxybutyrateMmolL  float64 `json:"beta_hydroxybutyrate_mmol_l"`
	VenousPH                  float64 `json:"venous_ph"`
	SerumPotassiumMeqL        float64 `json:"serum_potassium_meq_l"`
	SerumPhosphorusMgDl       float64 `json:"serum_phosphorus_mg_dl"`
	EffectiveOsmolalityMosmKg float64 `json:"effective_osmolality_mosm_kg"`
	DehydrationPercent        float64 `json:"dehydration_percent"`
}

// Validate enforces clinical invariants on DKAPatientProfile.
func (p DKAPatientProfile) Validate() error {
	if !IsValidDKASpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_DKA_PANCREATITIS or FELINE_DKA_HEPATIC_LIPIDOSIS, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.BloodGlucoseMgDl < 30.0 || p.BloodGlucoseMgDl > 1500.0 {
		return fmt.Errorf("blood_glucose_mg_dl must be in [30.0, 1500.0], got %v", p.BloodGlucoseMgDl)
	}
	if p.BetaHydroxybutyrateMmolL < 0.0 || p.BetaHydroxybutyrateMmolL > 25.0 {
		return fmt.Errorf("beta_hydroxybutyrate_mmol_l must be in [0.0, 25.0], got %v", p.BetaHydroxybutyrateMmolL)
	}
	if p.VenousPH < 6.70 || p.VenousPH > 7.60 {
		return fmt.Errorf("venous_ph must be in [6.70, 7.60], got %v", p.VenousPH)
	}
	if p.SerumPotassiumMeqL < 1.2 || p.SerumPotassiumMeqL > 10.0 {
		return fmt.Errorf("serum_potassium_meq_l must be in [1.2, 10.0], got %v", p.SerumPotassiumMeqL)
	}
	if p.SerumPhosphorusMgDl < 0.3 || p.SerumPhosphorusMgDl > 18.0 {
		return fmt.Errorf("serum_phosphorus_mg_dl must be in [0.3, 18.0], got %v", p.SerumPhosphorusMgDl)
	}
	if p.EffectiveOsmolalityMosmKg < 240.0 || p.EffectiveOsmolalityMosmKg > 420.0 {
		return fmt.Errorf("effective_osmolality_mosm_kg must be in [240.0, 420.0], got %v", p.EffectiveOsmolalityMosmKg)
	}
	if p.DehydrationPercent < 0.0 || p.DehydrationPercent > 15.0 {
		return fmt.Errorf("dehydration_percent must be in [0.0, 15.0], got %v", p.DehydrationPercent)
	}
	return nil
}

// DKAElectrolyteInsulinRegimen captures calculated sliding-scale KCl, potassium phosphate CRI,
// regular insulin CRI, dextrose supplementation, and cerebral edema / hypokalemia guards.
type DKAElectrolyteInsulinRegimen struct {
	FluidRehydrationDelayHours         int     `json:"fluid_rehydration_delay_hours"`
	PotassiumChlorideMeqPerLiter       int     `json:"potassium_chloride_meq_per_liter"`
	PotassiumPhosphateMmolKgHr         float64 `json:"potassium_phosphate_mmol_kg_hr"`
	RegularInsulinCRIUkgDay            float64 `json:"regular_insulin_cri_u_kg_day"`
	DextroseSupplementationPercent     float64 `json:"dextrose_supplementation_percent"`
	MaxGlucoseDropRateMgDlHr           float64 `json:"max_glucose_drop_rate_mg_dl_hr"`
	HoldInsulinIfPotassiumBelow33Guard bool    `json:"hold_insulin_if_potassium_below_33_guard"`
	DKAPitfallAlert                    string  `json:"dka_pitfall_alert"`
	ClinicalRationale                  string  `json:"clinical_rationale"`
}

// DKAGlucoseElectrolyteCheck records a serial q2h blood glucose, BHB ketone, electrolyte, and cerebral edema check.
type DKAGlucoseElectrolyteCheck struct {
	Timestamp                      time.Time `json:"timestamp"`
	BloodGlucoseMgDl               float64   `json:"blood_glucose_mg_dl"`
	BetaHydroxybutyrateMmolL       float64   `json:"beta_hydroxybutyrate_mmol_l"`
	SerumPotassiumMeqL             float64   `json:"serum_potassium_meq_l"`
	SerumPhosphorusMgDl            float64   `json:"serum_phosphorus_mg_dl"`
	DextrosePercentActive          float64   `json:"dextrose_percent_active"`
	MentationNormalNoCerebralEdema bool      `json:"mentation_normal_no_cerebral_edema"`
	ClinicianCallsign              string    `json:"clinician_callsign"`
	Notes                          string    `json:"notes"`
}

// Validate enforces clinical invariants on DKAGlucoseElectrolyteCheck.
func (c DKAGlucoseElectrolyteCheck) Validate() error {
	if c.BloodGlucoseMgDl < 30.0 || c.BloodGlucoseMgDl > 1500.0 {
		return fmt.Errorf("blood_glucose_mg_dl must be in [30.0, 1500.0], got %v", c.BloodGlucoseMgDl)
	}
	if c.BetaHydroxybutyrateMmolL < 0.0 || c.BetaHydroxybutyrateMmolL > 25.0 {
		return fmt.Errorf("beta_hydroxybutyrate_mmol_l must be in [0.0, 25.0], got %v", c.BetaHydroxybutyrateMmolL)
	}
	if c.SerumPotassiumMeqL < 1.2 || c.SerumPotassiumMeqL > 10.0 {
		return fmt.Errorf("serum_potassium_meq_l must be in [1.2, 10.0], got %v", c.SerumPotassiumMeqL)
	}
	if c.SerumPhosphorusMgDl < 0.3 || c.SerumPhosphorusMgDl > 18.0 {
		return fmt.Errorf("serum_phosphorus_mg_dl must be in [0.3, 18.0], got %v", c.SerumPhosphorusMgDl)
	}
	if c.DextrosePercentActive < 0.0 || c.DextrosePercentActive > 10.0 {
		return fmt.Errorf("dextrose_percent_active must be in [0.0, 10.0], got %v", c.DextrosePercentActive)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// DKADischargeClearance certifies resolution of ketonemia/acidosis, normokalemia/normophosphatemia,
// voluntary appetite, and transition to subcutaneous long-acting insulin (Glargine/Vetsulin).
type DKADischargeClearance struct {
	ClearedAt                      time.Time `json:"cleared_at"`
	FinalBetaHydroxybutyrateMmolL  float64   `json:"final_beta_hydroxybutyrate_mmol_l"`
	FinalVenousPH                  float64   `json:"final_venous_ph"`
	FinalSerumPotassiumMeqL        float64   `json:"final_serum_potassium_meq_l"`
	FinalSerumPhosphorusMgDl       float64   `json:"final_serum_phosphorus_mg_dl"`
	VoluntaryEatingNoVomiting      bool      `json:"voluntary_eating_no_vomiting"`
	SubQInsulinTransitionCompleted bool      `json:"subq_insulin_transition_completed"`
	VetLicense                     string    `json:"vet_license"`
	Summary                        string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on DKADischargeClearance.
func (d DKADischargeClearance) Validate() error {
	if d.FinalBetaHydroxybutyrateMmolL < 0.0 || d.FinalBetaHydroxybutyrateMmolL >= 0.6 {
		return fmt.Errorf("final_beta_hydroxybutyrate_mmol_l must be in [0.0, 0.6), got %v", d.FinalBetaHydroxybutyrateMmolL)
	}
	if d.FinalVenousPH < 7.35 || d.FinalVenousPH > 7.45 {
		return fmt.Errorf("final_venous_ph must be in [7.35, 7.45], got %v", d.FinalVenousPH)
	}
	if d.FinalSerumPotassiumMeqL < 3.5 || d.FinalSerumPotassiumMeqL > 5.5 {
		return fmt.Errorf("final_serum_potassium_meq_l must be in [3.5, 5.5], got %v", d.FinalSerumPotassiumMeqL)
	}
	if d.FinalSerumPhosphorusMgDl < 2.5 || d.FinalSerumPhosphorusMgDl > 6.0 {
		return fmt.Errorf("final_serum_phosphorus_mg_dl must be in [2.5, 6.0], got %v", d.FinalSerumPhosphorusMgDl)
	}
	if !d.VoluntaryEatingNoVomiting {
		return fmt.Errorf("voluntary_eating_no_vomiting must be true for discharge clearance")
	}
	if !d.SubQInsulinTransitionCompleted {
		return fmt.Errorf("subq_insulin_transition_completed must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// DKAInsulinOperation represents a Diabetic Ketoacidosis (DKA), Regular Insulin CRI,
// Hypokalemia/Hypophosphatemia & Cerebral Edema triage operation.
type DKAInsulinOperation struct {
	ID                            string                        `json:"id"`
	IncidentID                    string                        `json:"incident_id"`
	PetName                       string                        `json:"pet_name"`
	Status                        DKAStatus                     `json:"status"`
	HypokalemiaCerebralEdemaAlert bool                          `json:"hypokalemia_cerebral_edema_alert"`
	PatientProfile                DKAPatientProfile             `json:"patient_profile"`
	InsulinRegimen                *DKAElectrolyteInsulinRegimen `json:"insulin_regimen,omitempty"`
	SerialChecks                  []DKAGlucoseElectrolyteCheck  `json:"serial_checks"`
	DischargeClearance            *DKADischargeClearance        `json:"discharge_clearance,omitempty"`
	CreatedAt                     time.Time                     `json:"created_at"`
	UpdatedAt                     time.Time                     `json:"updated_at"`
}

// Validate checks domain invariants on DKAInsulinOperation.
func (op DKAInsulinOperation) Validate() error {
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
