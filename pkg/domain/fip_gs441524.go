package domain

import (
	"fmt"
	"strings"
	"time"
)

// FIPTriageStatus represents the clinical workflow state of an Acute Feline Infectious Peritonitis
// (FIP — Effusive Wet & Non-Effusive Dry/Ocular/Neurological), Rivalta Test, Albumin:Globulin Ratio < 0.4,
// GS-441524 / Remdesivir Nucleoside Analog & Blood-Brain/Ocular Barrier Dosing triage operation.
type FIPTriageStatus string

const (
	FIPStatusAcuteTriage       FIPTriageStatus = "ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE"
	FIPStatusGS441524Staged    FIPTriageStatus = "GS441524_REMDESIVIR_84DAY_STAGED"
	FIPStatusCleared84DayCured FIPTriageStatus = "AG_RATIO_NORMALIZED_84DAY_CURED_CLEARED"
)

// IsValid returns true when FIPTriageStatus is a recognized lifecycle state.
func (s FIPTriageStatus) IsValid() bool {
	switch s {
	case FIPStatusAcuteTriage,
		FIPStatusGS441524Staged,
		FIPStatusCleared84DayCured:
		return true
	default:
		return false
	}
}

// FIPClinicalPhenotype classifies the clinical presentation of Feline Infectious Peritonitis (FIP).
type FIPClinicalPhenotype string

const (
	FIPPhenotypeEffusiveCavitary FIPClinicalPhenotype = "EFFUSIVE_CAVITARY_PLEURAL_ASCITES_FIP"
	FIPPhenotypeNeurologicalCNS  FIPClinicalPhenotype = "NEUROLOGICAL_ATAXIA_SEIZURE_CNS_FIP"
	FIPPhenotypeOcularUveitis    FIPClinicalPhenotype = "OCULAR_ANTERIOR_UVEITIS_KERATIC_FIP"
)

// IsValid returns true when FIPClinicalPhenotype is a recognized FIP clinical phenotype.
func (p FIPClinicalPhenotype) IsValid() bool {
	switch p {
	case FIPPhenotypeEffusiveCavitary,
		FIPPhenotypeNeurologicalCNS,
		FIPPhenotypeOcularUveitis:
		return true
	default:
		return false
	}
}

// RivaltaBiomarkerAssessmentPlan captures the FIP clinical phenotype pathophysiology,
// Rivalta test and Albumin:Globulin (A:G) ratio (< 0.4) interpretation, Alpha-1-Acid Glycoprotein (AGP)
// and FCoV RT-PCR / immunohistochemistry status, strict contraindication of complete abdominal ascites
// drainage, Blood-Brain/Ocular barrier nucleoside dose escalation, and L-Lysine / Fluoroquinolone warning.
type RivaltaBiomarkerAssessmentPlan struct {
	PhenotypePathophysiology                string `json:"phenotype_pathophysiology"`
	RivaltaAGRatioInterpretation            string `json:"rivalta_ag_ratio_interpretation"`
	Alpha1AcidGlycoproteinFCoVPCR           string `json:"alpha1_acid_glycoprotein_fcov_pcr"`
	CompleteAscitesDrainageContraindication string `json:"complete_ascites_drainage_contraindication"`
	BarrierPenetrationDoseEscalation        string `json:"barrier_penetration_dose_escalation"`
	FluoroquinoloneRetinalToxicityWarning   string `json:"fluoroquinolone_retinal_toxicity_warning"`
}

// NucleosideAntiviralProtocolPlan captures GS-441524 / Remdesivir 84-day (12-week) nucleoside analog
// dosing, weekly weight-adjusted dose up-titration (never reduce dose), therapeutic thoracocentesis
// and partial ascites limit, ocular topical atropine/prednisolone acetate support, hepatoprotectant
// (SAMe/Silybin) and Cobalamin (Vitamin B12) supplementation, and Day 28/56/84 biomarker monitoring schedule.
type NucleosideAntiviralProtocolPlan struct {
	GS441524OrRemdesivirDosing      string `json:"gs441524_or_remdesivir_dosing"`
	WeightAdjustedDoseRecalcNote    string `json:"weight_adjusted_dose_recalc_note"`
	TherapeuticThoracocentesisLimit string `json:"therapeutic_thoracocentesis_limit"`
	OcularTopicalAtropinePredNote   string `json:"ocular_topical_atropine_pred_note"`
	HepatoprotectantB12SupportPlan  string `json:"hepatoprotectant_b12_support_plan"`
	Day28Day56Day84MonitoringPlan   string `json:"day28_day56_day84_monitoring_plan"`
}

// FIPAntiviralProgressCheck records a serial 84-day antiviral progress check
// (Treatment Day, Weight kg, Albumin:Globulin Ratio, Total Bilirubin mg/dL, Hematocrit %).
type FIPAntiviralProgressCheck struct {
	ID                   string    `json:"id"`
	TreatmentDay         int       `json:"treatment_day"`
	WeightKg             float64   `json:"weight_kg"`
	AlbuminGlobulinRatio float64   `json:"albumin_globulin_ratio"`
	TotalBilirubinMgDL   float64   `json:"total_bilirubin_mg_dl"`
	HematocritPercent    float64   `json:"hematocrit_percent"`
	RecordedBy           string    `json:"recorded_by"`
	ClinicalNote         string    `json:"clinical_note"`
	RecordedAt           time.Time `json:"recorded_at"`
}

// FIPGS441524Operation represents an Acute Feline Infectious Peritonitis (FIP) & GS-441524 / Remdesivir
// Nucleoside Analog 84-day triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type FIPGS441524Operation struct {
	ID                            string                          `json:"id"`
	CaseID                        string                          `json:"case_id"`
	PetName                       string                          `json:"pet_name"`
	Species                       string                          `json:"species"`
	Breed                         string                          `json:"breed"`
	WeightKg                      float64                         `json:"weight_kg"`
	Status                        FIPTriageStatus                 `json:"status"`
	Phenotype                     FIPClinicalPhenotype            `json:"phenotype"`
	TreatmentDay                  int                             `json:"treatment_day"`
	AlbuminGlobulinRatio          float64                         `json:"albumin_globulin_ratio"`
	TotalBilirubinMgDL            float64                         `json:"total_bilirubin_mg_dl"`
	HematocritPercent             float64                         `json:"hematocrit_percent"`
	DailyGS441524MgKg             float64                         `json:"daily_gs441524_mg_kg"`
	RivaltaTestPositive           bool                            `json:"rivalta_test_positive"`
	PleuralEffusionDyspneaPresent bool                            `json:"pleural_effusion_dyspnea_present"`
	BiomarkerPlan                 RivaltaBiomarkerAssessmentPlan  `json:"biomarker_plan"`
	AntiviralPlan                 NucleosideAntiviralProtocolPlan `json:"antiviral_plan"`
	ProgressChecks                []FIPAntiviralProgressCheck     `json:"progress_checks"`
	DischargedBy                  string                          `json:"discharged_by,omitempty"`
	DischargeSummary              string                          `json:"discharge_summary,omitempty"`
	DischargedAt                  *time.Time                      `json:"discharged_at,omitempty"`
	CreatedAt                     time.Time                       `json:"created_at"`
	UpdatedAt                     time.Time                       `json:"updated_at"`
}

// Validate checks domain invariants on FIPGS441524Operation.
func (op FIPGS441524Operation) Validate() error {
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
	if !op.Phenotype.IsValid() {
		return fmt.Errorf("invalid phenotype: %q", op.Phenotype)
	}
	return nil
}

// FIPOverviewMetrics summarizes aggregate counts across FIP & GS-441524 Antiviral operations.
type FIPOverviewMetrics struct {
	TotalOperations          int `json:"total_operations"`
	AcuteFIPTriageCount      int `json:"acute_fip_triage_count"`
	GS441524StagedCount      int `json:"gs441524_staged_count"`
	Cleared84DayCuredCount   int `json:"cleared_84day_cured_count"`
	NeuroOcularHighDoseCount int `json:"neuro_ocular_high_dose_count"`
	AGRatioUnder04Count      int `json:"ag_ratio_under_04_count"`
}
