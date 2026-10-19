package domain

import (
	"fmt"
	"strings"
	"time"
)

// OphthalmicGlobeStatus represents the lifecycle state of an Acute Ophthalmic Emergencies,
// Descemetoceles, Proptosed Globe, Acute Congestive Glaucoma (IOP > 40 mmHg) & Mannitol/Latanoprost triage operation.
type OphthalmicGlobeStatus string

const (
	OphthalmicStatusVisionThreateningCritical     OphthalmicGlobeStatus = "VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL"
	OphthalmicStatusLatanoprostTarsorrhaphyStaged OphthalmicGlobeStatus = "LATANOPROST_MANNITOL_TARSORRHAPHY_STAGED"
	OphthalmicStatusIOPNormalCorneaProtected      OphthalmicGlobeStatus = "IOP_NORMALIZED_GLOBE_PROTECTED_CLEARED"
)

// IsValid returns true when OphthalmicGlobeStatus is a supported lifecycle state.
func (s OphthalmicGlobeStatus) IsValid() bool {
	switch s {
	case OphthalmicStatusVisionThreateningCritical,
		OphthalmicStatusLatanoprostTarsorrhaphyStaged,
		OphthalmicStatusIOPNormalCorneaProtected:
		return true
	default:
		return false
	}
}

const (
	OphthalmicSpeciesCaninePrimaryCongestiveGlaucoma    = "CANINE_PRIMARY_CONGESTIVE_GLAUCOMA"
	OphthalmicSpeciesCanineBrachycephalicGlobeProptosis = "CANINE_BRACHYCEPHALIC_GLOBE_PROPTOSIS"
	OphthalmicSpeciesFelineMeltingDescemetoceleUlcer    = "FELINE_MELTING_DESCEMETOCELE_ULCER"
)

// IsValidOphthalmicSpecies returns true if species is a recognized ophthalmic emergency classification.
func IsValidOphthalmicSpecies(species string) bool {
	switch strings.ToUpper(strings.TrimSpace(species)) {
	case OphthalmicSpeciesCaninePrimaryCongestiveGlaucoma,
		OphthalmicSpeciesCanineBrachycephalicGlobeProptosis,
		OphthalmicSpeciesFelineMeltingDescemetoceleUlcer:
		return true
	default:
		return false
	}
}

const (
	OphthalmicFluoresceinNegativeIntact         = "FLUORESCEIN_NEGATIVE_INTACT"
	OphthalmicFluoresceinStromalPositiveMelting = "STROMAL_POSITIVE_MELTING"
	OphthalmicFluoresceinDonutRingDescemetocele = "DONUT_RING_CLEAR_CENTER_DESCEMETOCELE"
	OphthalmicFluoresceinSeidelPositiveLeak     = "SEIDEL_POSITIVE_AQUEOUS_LEAK"
)

// IsValidFluoresceinStainPattern returns true if pattern is a recognized fluorescein corneal stain pattern.
func IsValidFluoresceinStainPattern(pattern string) bool {
	switch strings.ToUpper(strings.TrimSpace(pattern)) {
	case OphthalmicFluoresceinNegativeIntact,
		OphthalmicFluoresceinStromalPositiveMelting,
		OphthalmicFluoresceinDonutRingDescemetocele,
		OphthalmicFluoresceinSeidelPositiveLeak:
		return true
	default:
		return false
	}
}

// OphthalmicPatientProfile captures signalment, rebound/applanation tonometry IOP (mmHg),
// fluorescein corneal stain pattern, anterior lens luxation status, avulsed extraocular muscle count,
// and consensual pupillary light reflex (PLR).
type OphthalmicPatientProfile struct {
	Species                        string  `json:"species"`
	Breed                          string  `json:"breed,omitempty"`
	WeightKg                       float64 `json:"weight_kg"`
	IntraocularPressureMmHg        int     `json:"intraocular_pressure_mmhg"`
	FluoresceinStainPattern        string  `json:"fluorescein_stain_pattern"`
	AnteriorLensLuxationPresent    bool    `json:"anterior_lens_luxation_present"`
	ExtraocularMusclesAvulsedCount int     `json:"extraocular_muscles_avulsed_count"`
	ConsensualPLRPresent           bool    `json:"consensual_plr_present"`
}

// Validate enforces clinical invariants on OphthalmicPatientProfile.
func (p OphthalmicPatientProfile) Validate() error {
	if !IsValidOphthalmicSpecies(p.Species) {
		return fmt.Errorf("species must be CANINE_PRIMARY_CONGESTIVE_GLAUCOMA, CANINE_BRACHYCEPHALIC_GLOBE_PROPTOSIS, or FELINE_MELTING_DESCEMETOCELE_ULCER, got %q", p.Species)
	}
	if p.WeightKg <= 0 || p.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", p.WeightKg)
	}
	if p.IntraocularPressureMmHg < 2 || p.IntraocularPressureMmHg > 120 {
		return fmt.Errorf("intraocular_pressure_mmhg must be in [2, 120], got %d", p.IntraocularPressureMmHg)
	}
	if !IsValidFluoresceinStainPattern(p.FluoresceinStainPattern) {
		return fmt.Errorf("fluorescein_stain_pattern must be FLUORESCEIN_NEGATIVE_INTACT, STROMAL_POSITIVE_MELTING, DONUT_RING_CLEAR_CENTER_DESCEMETOCELE, or SEIDEL_POSITIVE_AQUEOUS_LEAK, got %q", p.FluoresceinStainPattern)
	}
	if p.ExtraocularMusclesAvulsedCount < 0 || p.ExtraocularMusclesAvulsedCount > 6 {
		return fmt.Errorf("extraocular_muscles_avulsed_count must be in [0, 6], got %d", p.ExtraocularMusclesAvulsedCount)
	}
	return nil
}

// OphthalmicPreservationPlan captures Latanoprost 0.005% eligibility (contraindicated in anterior lens luxation
// and feline uveitic/melting ulcers), Dorzolamide/Timolol carbonic anhydrase inhibitor / beta-blocker frequency,
// Mannitol 1.0 g/kg slow IV over 20 min (when IOP > 40 mmHg), autologous serum drops q1h (anti-collagenase MMP inhibition
// for melting ulcers / descemetoceles), temporary tarsorrhaphy indication (proptosed globe with < 3 avulsed extraocular muscles),
// and zero topical corticosteroid / petroleum ointment / neck-lead restraint guard.
type OphthalmicPreservationPlan struct {
	Latanoprost0005PctAllowed               bool    `json:"latanoprost_0005_pct_allowed"`
	DorzolamideTimololDropsFreq             string  `json:"dorzolamide_timolol_drops_freq"`
	Mannitol1GKgIVGrams                     float64 `json:"mannitol_1_g_kg_iv_grams"`
	AutologousSerumDropsQ1H                 bool    `json:"autologous_serum_drops_q1h"`
	TemporaryTarsorrhaphyIndicated          bool    `json:"temporary_tarsorrhaphy_indicated"`
	ZeroTopicalSteroidOintmentNeckLeadGuard bool    `json:"zero_topical_steroid_ointment_neck_lead_guard"`
	OphthalmicPitfallAlert                  string  `json:"ophthalmic_pitfall_alert"`
	ClinicalRationale                       string  `json:"clinical_rationale"`
}

// OphthalmicTonometryCheck records a serial rebound/applanation tonometry, Seidel aqueous leak test,
// dazzle reflex, and rigid E-collar verification check.
type OphthalmicTonometryCheck struct {
	Timestamp               time.Time `json:"timestamp"`
	IntraocularPressureMmHg int       `json:"intraocular_pressure_mmhg"`
	SeidelTestNegative      bool      `json:"seidel_test_negative"`
	DazzleReflexPresent     bool      `json:"dazzle_reflex_present"`
	RigidECollarVerified    bool      `json:"rigid_e_collar_verified"`
	ClinicianCallsign       string    `json:"clinician_callsign"`
	Notes                   string    `json:"notes"`
}

// Validate enforces clinical invariants on OphthalmicTonometryCheck.
func (c OphthalmicTonometryCheck) Validate() error {
	if c.IntraocularPressureMmHg < 2 || c.IntraocularPressureMmHg > 120 {
		return fmt.Errorf("intraocular_pressure_mmhg must be in [2, 120], got %d", c.IntraocularPressureMmHg)
	}
	if strings.TrimSpace(c.ClinicianCallsign) == "" {
		return fmt.Errorf("clinician_callsign is required")
	}
	return nil
}

// OphthalmicDischargeClearance certifies normalized IOP (10-25 mmHg), confirmed globe tectonic integrity,
// fitted hard plastic Elizabethan collar, and owner education on >= 5-minute spacing between topical ophthalmic drops.
type OphthalmicDischargeClearance struct {
	ClearedAt                       time.Time `json:"cleared_at"`
	FinalIOPMmHg                    int       `json:"final_iop_mmhg"`
	GlobeTectonicIntegrityConfirmed bool      `json:"globe_tectonic_integrity_confirmed"`
	HardPlasticECollarFitted        bool      `json:"hard_plastic_e_collar_fitted"`
	TopicalDrop5MinSpacingEducated  bool      `json:"topical_drop_5_min_spacing_educated"`
	VetLicense                      string    `json:"vet_license"`
	Summary                         string    `json:"summary"`
}

// Validate enforces strict discharge safety criteria on OphthalmicDischargeClearance.
func (d OphthalmicDischargeClearance) Validate() error {
	if d.FinalIOPMmHg < 10 || d.FinalIOPMmHg > 25 {
		return fmt.Errorf("final_iop_mmhg must be in [10, 25], got %d", d.FinalIOPMmHg)
	}
	if !d.GlobeTectonicIntegrityConfirmed {
		return fmt.Errorf("globe_tectonic_integrity_confirmed must be true for discharge clearance")
	}
	if !d.HardPlasticECollarFitted {
		return fmt.Errorf("hard_plastic_e_collar_fitted must be true for discharge clearance")
	}
	if !d.TopicalDrop5MinSpacingEducated {
		return fmt.Errorf("topical_drop_5_min_spacing_educated must be true for discharge clearance")
	}
	if strings.TrimSpace(d.VetLicense) == "" {
		return fmt.Errorf("vet_license is required")
	}
	return nil
}

// OphthalmicGlobeOperation represents an Acute Ophthalmic Emergencies, Descemetoceles,
// Proptosed Globe, Acute Congestive Glaucoma (IOP > 40 mmHg) & Mannitol/Latanoprost triage operation.
type OphthalmicGlobeOperation struct {
	ID                              string                        `json:"id"`
	IncidentID                      string                        `json:"incident_id"`
	PetName                         string                        `json:"pet_name"`
	Status                          OphthalmicGlobeStatus         `json:"status"`
	LatanoprostLuxationSteroidAlert bool                          `json:"latanoprost_luxation_steroid_alert"`
	PatientProfile                  OphthalmicPatientProfile      `json:"patient_profile"`
	PreservationPlan                *OphthalmicPreservationPlan   `json:"preservation_plan,omitempty"`
	TonometryChecks                 []OphthalmicTonometryCheck    `json:"tonometry_checks"`
	DischargeClearance              *OphthalmicDischargeClearance `json:"discharge_clearance,omitempty"`
	CreatedAt                       time.Time                     `json:"created_at"`
	UpdatedAt                       time.Time                     `json:"updated_at"`
}

// Validate checks domain invariants on OphthalmicGlobeOperation.
func (op OphthalmicGlobeOperation) Validate() error {
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
