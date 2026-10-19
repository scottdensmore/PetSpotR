package domain

import (
	"fmt"
	"strings"
	"time"
)

// SepticPeritonitisStatus represents the lifecycle state of a Septic Peritonitis,
// Abdominocentesis Blood-Fluid Glucose/Lactate Gradient & Closed-Suction Jackson-Pratt Drain triage operation.
type SepticPeritonitisStatus string

const (
	PeritonitisStatusSepticEffusionCritical   SepticPeritonitisStatus = "SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL"
	PeritonitisStatusAntibioticLavageJPStaged SepticPeritonitisStatus = "EMPIRIC_ANTIBIOTIC_LAVAGE_JP_DRAIN_STAGED"
	PeritonitisStatusNonSepticJPDrainRemoved  SepticPeritonitisStatus = "NON_SEPTIC_CYTOLOGY_JP_DRAIN_REMOVED_CLEARED"
)

// IsValid returns true when SepticPeritonitisStatus is a supported lifecycle state.
func (s SepticPeritonitisStatus) IsValid() bool {
	switch s {
	case PeritonitisStatusSepticEffusionCritical,
		PeritonitisStatusAntibioticLavageJPStaged,
		PeritonitisStatusNonSepticJPDrainRemoved:
		return true
	default:
		return false
	}
}

const (
	PeritonitisSpeciesCanineGIDehiscence     = "CANINE_GI_DEHISCENCE_PERFORATION"
	PeritonitisSpeciesCanineProstaticAbscess = "CANINE_RUPTURED_PROSTATIC_ABSCESS"
	PeritonitisSpeciesFelineBilePeritonitis  = "FELINE_SEPTIC_BILE_PERITONITIS"
)

// IsValidPeritonitisSpecies returns true if species is a recognized septic peritonitis classification.
func IsValidPeritonitisSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case PeritonitisSpeciesCanineGIDehiscence,
		PeritonitisSpeciesCanineProstaticAbscess,
		PeritonitisSpeciesFelineBilePeritonitis:
		return true
	default:
		return false
	}
}

// PeritonitisPatientProfile captures patient signalment, abdominocentesis blood-to-fluid glucose
// and lactate gradients, degenerate neutrophil count, and intracellular bacteria cytology.
type PeritonitisPatientProfile struct {
	Species                      string  `json:"species"`
	Breed                        string  `json:"breed,omitempty"`
	WeightKg                     float64 `json:"weight_kg"`
	BloodGlucoseMgDl             float64 `json:"blood_glucose_mg_dl"`
	EffusionGlucoseMgDl          float64 `json:"effusion_glucose_mg_dl"`
	BloodLactateMmolL            float64 `json:"blood_lactate_mmol_l"`
	EffusionLactateMmolL         float64 `json:"effusion_lactate_mmol_l"`
	DegenerateNeutrophilsCountUl int     `json:"degenerate_neutrophils_count_ul"`
	IntracellularBacteriaSeen    bool    `json:"intracellular_bacteria_seen"`
}

// Validate enforces clinical invariants on PeritonitisPatientProfile.
func (p PeritonitisPatientProfile) Validate() error {
	if !IsValidPeritonitisSpecies(p.Species) {
		return fmt.Errorf("invalid species: %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.BloodGlucoseMgDl < 10.0 || p.BloodGlucoseMgDl > 600.0 {
		return fmt.Errorf("blood_glucose_mg_dl must be in [10.0, 600.0], got %v", p.BloodGlucoseMgDl)
	}
	if p.EffusionGlucoseMgDl < 0.0 || p.EffusionGlucoseMgDl > 600.0 {
		return fmt.Errorf("effusion_glucose_mg_dl must be in [0.0, 600.0], got %v", p.EffusionGlucoseMgDl)
	}
	if p.BloodLactateMmolL < 0.1 || p.BloodLactateMmolL > 30.0 {
		return fmt.Errorf("blood_lactate_mmol_l must be in [0.1, 30.0], got %v", p.BloodLactateMmolL)
	}
	if p.EffusionLactateMmolL < 0.1 || p.EffusionLactateMmolL > 35.0 {
		return fmt.Errorf("effusion_lactate_mmol_l must be in [0.1, 35.0], got %v", p.EffusionLactateMmolL)
	}
	if p.DegenerateNeutrophilsCountUl < 0 || p.DegenerateNeutrophilsCountUl > 500000 {
		return fmt.Errorf("degenerate_neutrophils_count_ul must be in [0, 500000], got %d", p.DegenerateNeutrophilsCountUl)
	}
	return nil
}

// SepticPeritonitisSurgicalPlan captures calculated blood-fluid glucose/lactate gradients,
// empiric IV broad-spectrum antibiotics, warm sterile saline lavage volume, Jackson-Pratt drains,
// and antiseptic lavage prohibition guard.
type SepticPeritonitisSurgicalPlan struct {
	BloodFluidGlucoseGradientMgDl     float64 `json:"blood_fluid_glucose_gradient_mg_dl"`
	FluidBloodLactateGradientMmolL    float64 `json:"fluid_blood_lactate_gradient_mmol_l"`
	AmpicillinSulbactam30MgKgMg       float64 `json:"ampicillin_sulbactam_30mg_kg_mg"`
	Enrofloxacin10MgKgDoseMg          float64 `json:"enrofloxacin_10mg_kg_dose_mg"`
	WarmSterileSalineLavageLiters     float64 `json:"warm_sterile_saline_lavage_liters"`
	JacksonPrattClosedSuctionDrains   int     `json:"jackson_pratt_closed_suction_drains"`
	ZeroAntisepticLavageAdditionGuard bool    `json:"zero_antiseptic_lavage_addition_guard"`
	PeritonitisPitfallAlert           string  `json:"peritonitis_pitfall_alert"`
	ClinicalRationale                 string  `json:"clinical_rationale"`
}

// JacksonPrattDrainCheck records a serial post-operative closed-suction Jackson-Pratt drain
// effusion production, cytology, albumin, and hemodynamic check.
type JacksonPrattDrainCheck struct {
	Timestamp                     time.Time `json:"timestamp"`
	EffusionProductionMlKgDay     float64   `json:"effusion_production_ml_kg_day"`
	CytologyIntracellularBacteria bool      `json:"cytology_intracellular_bacteria"`
	DegenerateNeutrophilsPresent  bool      `json:"degenerate_neutrophils_present"`
	SerumAlbuminGdl               float64   `json:"serum_albumin_g_dl"`
	MeanArterialPressureMmHg      int       `json:"mean_arterial_pressure_mm_hg"`
	ClinicianCallsign             string    `json:"clinician_callsign"`
	Notes                         string    `json:"notes"`
}

// Validate enforces clinical invariants on JacksonPrattDrainCheck.
func (c JacksonPrattDrainCheck) Validate() error {
	if c.EffusionProductionMlKgDay < 0.0 || c.EffusionProductionMlKgDay > 100.0 {
		return fmt.Errorf("effusion_production_ml_kg_day must be in [0.0, 100.0], got %v", c.EffusionProductionMlKgDay)
	}
	if c.SerumAlbuminGdl < 0.5 || c.SerumAlbuminGdl > 6.0 {
		return fmt.Errorf("serum_albumin_g_dl must be in [0.5, 6.0], got %v", c.SerumAlbuminGdl)
	}
	if c.MeanArterialPressureMmHg < 30 || c.MeanArterialPressureMmHg > 220 {
		return fmt.Errorf("mean_arterial_pressure_mm_hg must be in [30, 220], got %d", c.MeanArterialPressureMmHg)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// PeritonitisDrainRemovalClearance certifies non-septic effusion cytology, low physiological
// drain output (<= 2.0 mL/kg/day), oncotic stability, enteral nutrition tolerance, and aseptic JP drain removal.
type PeritonitisDrainRemovalClearance struct {
	ClearedAt                     time.Time `json:"cleared_at"`
	FinalDrainOutputMlKgDay       float64   `json:"final_drain_output_ml_kg_day"`
	FinalCytologyBacteriaNegative bool      `json:"final_cytology_bacteria_negative"`
	FinalSerumAlbuminGdl          float64   `json:"final_serum_albumin_g_dl"`
	EnteralNutritionTolerated     bool      `json:"enteral_nutrition_tolerated"`
	JPDrainAsepticallyRemoved     bool      `json:"jp_drain_aseptically_removed"`
	VetLicense                    string    `json:"vet_license"`
	Summary                       string    `json:"summary"`
}

// Validate enforces strict drain removal safety criteria on PeritonitisDrainRemovalClearance.
func (d PeritonitisDrainRemovalClearance) Validate() error {
	if d.FinalDrainOutputMlKgDay < 0.0 || d.FinalDrainOutputMlKgDay > 2.0 {
		return fmt.Errorf("final_drain_output_ml_kg_day must be in [0.0, 2.0], got %v", d.FinalDrainOutputMlKgDay)
	}
	if !d.FinalCytologyBacteriaNegative {
		return fmt.Errorf("final_cytology_bacteria_negative must be true for drain removal clearance")
	}
	if d.FinalSerumAlbuminGdl < 2.0 || d.FinalSerumAlbuminGdl > 6.0 {
		return fmt.Errorf("final_serum_albumin_g_dl must be >= 2.0 (in [2.0, 6.0]), got %v", d.FinalSerumAlbuminGdl)
	}
	if !d.EnteralNutritionTolerated {
		return fmt.Errorf("enteral_nutrition_tolerated must be true for drain removal clearance")
	}
	if !d.JPDrainAsepticallyRemoved {
		return fmt.Errorf("jp_drain_aseptically_removed must be true for drain removal clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// SepticPeritonitisOperation represents a Septic Peritonitis, Abdominocentesis Blood-Fluid
// Glucose/Lactate Gradient & Closed-Suction Jackson-Pratt Drain triage operation.
type SepticPeritonitisOperation struct {
	ID                                 string                            `json:"id"`
	IncidentID                         string                            `json:"incident_id"`
	PetName                            string                            `json:"pet_name"`
	Status                             SepticPeritonitisStatus           `json:"status"`
	SepticGradientAntibioticDelayAlert bool                              `json:"septic_gradient_antibiotic_delay_alert"`
	PatientProfile                     PeritonitisPatientProfile         `json:"patient_profile"`
	SurgicalPlan                       *SepticPeritonitisSurgicalPlan    `json:"surgical_plan,omitempty"`
	DrainChecks                        []JacksonPrattDrainCheck          `json:"drain_checks"`
	DrainClearance                     *PeritonitisDrainRemovalClearance `json:"drain_clearance,omitempty"`
	CreatedAt                          time.Time                         `json:"created_at"`
	UpdatedAt                          time.Time                         `json:"updated_at"`
}

// Validate checks domain invariants on SepticPeritonitisOperation.
func (op SepticPeritonitisOperation) Validate() error {
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
