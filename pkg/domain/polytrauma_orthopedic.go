package domain

import (
	"fmt"
	"strings"
	"time"
)

// PolytraumaOrthoStatus represents the clinical workflow state of an Acute Bite Wound,
// Vehicular Polytrauma, Degloving/Shearing Injury, Open Fracture Gustilo-Anderson Classification,
// Wet-to-Dry/NPWT Vacuum-Assisted Closure & Damage-Control Orthopedics triage operation.
type PolytraumaOrthoStatus string

const (
	OrthoStatusAcuteOpenFractureTriage    PolytraumaOrthoStatus = "ACUTE_OPEN_FRACTURE_DEGLOVING_TRIAGE"
	OrthoStatusNPWTSplintStaged           PolytraumaOrthoStatus = "LAVAGE_NPWT_ROBERT_JONES_SPLINT_STAGED"
	OrthoStatusGranulationFixationCleared PolytraumaOrthoStatus = "GRANULATION_FIXATION_STABLE_CLEARED"
)

// IsValid returns true when PolytraumaOrthoStatus is a recognized lifecycle state.
func (s PolytraumaOrthoStatus) IsValid() bool {
	switch s {
	case OrthoStatusAcuteOpenFractureTriage,
		OrthoStatusNPWTSplintStaged,
		OrthoStatusGranulationFixationCleared:
		return true
	default:
		return false
	}
}

// GustiloInjuryClassification classifies the open fracture, vehicular shearing/degloving,
// or Big-Dog-Little-Dog (BDLD) crushing thoracic bite wound injury.
type GustiloInjuryClassification string

const (
	GustiloClassGradeIIIShearingDegloving  GustiloInjuryClassification = "GUSTILO_GRADE_III_SHEARING_DEGLOVING"
	GustiloClassBDLDThoracicBiteFlail      GustiloInjuryClassification = "BIG_DOG_LITTLE_DOG_THORACIC_BITE_FLAIL"
	GustiloClassGradeIOpenLongBoneFracture GustiloInjuryClassification = "GUSTILO_GRADE_I_II_OPEN_LONG_BONE_FRACTURE"
)

// IsValid returns true when GustiloInjuryClassification is a recognized injury classification.
func (c GustiloInjuryClassification) IsValid() bool {
	switch c {
	case GustiloClassGradeIIIShearingDegloving,
		GustiloClassBDLDThoracicBiteFlail,
		GustiloClassGradeIOpenLongBoneFracture:
		return true
	default:
		return false
	}
}

// GustiloWoundAssessmentPlan captures the Gustilo-Anderson open fracture & degloving pathophysiology,
// "Iceberg Effect" of BDLD (Big-Dog-Little-Dog) crushing bite wounds, 7–8 PSI pulsatile warm LRS/0.9% NaCl
// lavage note, strict contraindication of primary closure over contaminated/shearing wounds,
// joint-above-and-joint-below immobilization rule, and distal limb neurovascular/toe-web temperature assessment.
type GustiloWoundAssessmentPlan struct {
	GustiloGradePathophysiology    string `json:"gustilo_grade_pathophysiology"`
	IcebergBiteDeadSpaceAlert      string `json:"iceberg_bite_dead_space_alert"`
	HighPressureLavage7to8PSINote  string `json:"high_pressure_lavage_7_to_8_psi_note"`
	PrimaryClosureContraindication string `json:"primary_closure_contraindication"`
	JointAboveJointBelowSplintRule string `json:"joint_above_joint_below_splint_rule"`
	VascularNeuroDistalLimbNote    string `json:"vascular_neuro_distal_limb_note"`
}

// DebridementFixationPlan captures sterile water-soluble gel wound protection prior to wide surgical clip,
// copious 7–8 PSI warm isotonic lavage (1 L bag + 300 mmHg pressure cuff + 18G needle or 35 mL syringe + 18G needle),
// Negative Pressure Wound Therapy (NPWT at -125 mmHg) or sterile tie-over / wet-to-dry dressing plan,
// deep tissue culture before post-lavage empiric antimicrobials, Robert Jones / Spica splint immobilization,
// and definitive External Skeletal Fixator (ESF) or internal bone plating plan.
type DebridementFixationPlan struct {
	SterileWaterSolubleGelClipPlan    string `json:"sterile_water_soluble_gel_clip_plan"`
	CopiousLavage35mLSyringe18GPlan   string `json:"copious_lavage_35_ml_syringe_18g_plan"`
	NPWTVacuumOrTieOverDressingPlan   string `json:"npwt_vacuum_or_tie_over_dressing_plan"`
	EmpiricAbxCultureBeforeLavage     string `json:"empiric_abx_culture_before_lavage"`
	RobertJonesOrSpicaSplintPlan      string `json:"robert_jones_or_spica_splint_plan"`
	DefinitiveExternalSkeletalFixator string `json:"definitive_external_skeletal_fixator"`
}

// OrthopedicWoundCheck records a serial orthopedic & wound bed assessment check
// (Animal Trauma Triage score, lavage volume, NPWT vacuum pressure, blood lactate, distal toe temperature delta).
type OrthopedicWoundCheck struct {
	ID                      string    `json:"id"`
	AnimalTraumaTriageScore int       `json:"animal_trauma_triage_score"`
	LavageVolumeML          int       `json:"lavage_volume_ml"`
	NPWTVacuumMmHg          int       `json:"npwt_vacuum_mmhg"`
	BloodLactateMmolL       float64   `json:"blood_lactate_mmol_l"`
	DistalToeTempDeltaC     float64   `json:"distal_toe_temp_delta_c"`
	RecordedBy              string    `json:"recorded_by"`
	ClinicalNote            string    `json:"clinical_note"`
	RecordedAt              time.Time `json:"recorded_at"`
}

// PolytraumaOrthoOperation represents an Acute Bite Wound, Vehicular Polytrauma,
// Degloving/Shearing Injury, Open Fracture Gustilo-Anderson & NPWT triage operation
// in the PetSpotR Veterinary Emergency ICU cockpit.
type PolytraumaOrthoOperation struct {
	ID                               string                      `json:"id"`
	CaseID                           string                      `json:"case_id"`
	PetName                          string                      `json:"pet_name"`
	Species                          string                      `json:"species"`
	Breed                            string                      `json:"breed"`
	WeightKg                         float64                     `json:"weight_kg"`
	Status                           PolytraumaOrthoStatus       `json:"status"`
	GustiloClass                     GustiloInjuryClassification `json:"gustilo_class"`
	AnimalTraumaTriageScore          int                         `json:"animal_trauma_triage_score"`
	LavageVolumeML                   int                         `json:"lavage_volume_ml"`
	NPWTVacuumMmHg                   int                         `json:"npwt_vacuum_mmhg"`
	BloodLactateMmolL                float64                     `json:"blood_lactate_mmol_l"`
	DistalToeTempDeltaC              float64                     `json:"distal_toe_temp_delta_c"`
	ExposedCorticalBoneTendon        bool                        `json:"exposed_cortical_bone_tendon"`
	ThoracicWallPenetrationSuspected bool                        `json:"thoracic_wall_penetration_suspected"`
	AssessmentPlan                   GustiloWoundAssessmentPlan  `json:"assessment_plan"`
	FixationPlan                     DebridementFixationPlan     `json:"fixation_plan"`
	WoundChecks                      []OrthopedicWoundCheck      `json:"wound_checks"`
	DischargedBy                     string                      `json:"discharged_by,omitempty"`
	DischargeSummary                 string                      `json:"discharge_summary,omitempty"`
	DischargedAt                     *time.Time                  `json:"discharged_at,omitempty"`
	CreatedAt                        time.Time                   `json:"created_at"`
	UpdatedAt                        time.Time                   `json:"updated_at"`
}

// Validate checks domain invariants on PolytraumaOrthoOperation.
func (op PolytraumaOrthoOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.CaseID) == "" {
		return fmt.Errorf("case_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if op.WeightKg <= 0 {
		return fmt.Errorf("weight_kg must be positive, got %v", op.WeightKg)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if !op.GustiloClass.IsValid() {
		return fmt.Errorf("invalid gustilo_class: %q", op.GustiloClass)
	}
	return nil
}

// PolytraumaOrthoOverviewMetrics summarizes aggregate counts across Polytrauma & Orthopedic operations.
type PolytraumaOrthoOverviewMetrics struct {
	TotalOperations              int `json:"total_operations"`
	AcuteOpenFractureTriageCount int `json:"acute_open_fracture_triage_count"`
	NPWTSplintStagedCount        int `json:"npwt_splint_staged_count"`
	ClearedFixationCount         int `json:"cleared_fixation_count"`
	GustiloGradeIIIShearingCount int `json:"gustilo_grade_iii_shearing_count"`
	ThoracicBitePenetrationCount int `json:"thoracic_bite_penetration_count"`
}
