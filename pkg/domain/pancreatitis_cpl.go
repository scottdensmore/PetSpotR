package domain

import (
	"fmt"
	"strings"
	"time"
)

// PancreatitisStatus represents the clinical lifecycle state of an Acute Pancreatitis,
// Canine/Feline Spec cPL/fPL, POCUS Hypoechoic Pancreas/Hyperechoic Mesentery,
// Maropitant/Fuzapladib & Early Enteral Nutrition triage operation.
type PancreatitisStatus string

const (
	PancreatitisStatusAcuteNecrotizingTriage   PancreatitisStatus = "ACUTE_NECROTIZING_PANCREATITIS_TRIAGE"
	PancreatitisStatusFuzapladibEnteralStaged  PancreatitisStatus = "FUZAPLADIB_MAROPITANT_ENTERAL_STAGED"
	PancreatitisStatusToleratingEnteralCleared PancreatitisStatus = "TOLERATING_ENTERAL_PAIN_FREE_CLEARED"
)

// IsValid returns true when PancreatitisStatus is a supported lifecycle state.
func (s PancreatitisStatus) IsValid() bool {
	switch s {
	case PancreatitisStatusAcuteNecrotizingTriage,
		PancreatitisStatusFuzapladibEnteralStaged,
		PancreatitisStatusToleratingEnteralCleared:
		return true
	default:
		return false
	}
}

// PancreatitisSeverityClass classifies acute pancreatitis severity and feline triaditis comorbidity.
type PancreatitisSeverityClass string

const (
	PancreatitisSeveritySevereNecrotizingSIRSMODS PancreatitisSeverityClass = "SEVERE_NECROTIZING_SIRS_MODS"
	PancreatitisSeverityModerateAcute             PancreatitisSeverityClass = "MODERATE_ACUTE_PANCREATITIS"
	PancreatitisSeverityFelineTriaditis           PancreatitisSeverityClass = "FELINE_TRIADITIS_CHOLANGIOHEPATITIS"
)

// IsValid returns true when PancreatitisSeverityClass is a supported severity class.
func (c PancreatitisSeverityClass) IsValid() bool {
	switch c {
	case PancreatitisSeveritySevereNecrotizingSIRSMODS,
		PancreatitisSeverityModerateAcute,
		PancreatitisSeverityFelineTriaditis:
		return true
	default:
		return false
	}
}

// DiagnosticPOCUSPlan captures Spec cPL/fPL quantitative assay interpretation,
// abdominal POCUS sonographic hallmarks (hypoechoic enlarged pancreas, hyperechoic peripancreatic fat saponification),
// duodenal corrugation, extrahepatic biliary duct obstruction (EHBDO), feline triaditis + cobalamin (Vitamin B12)
// supplementation note, and explicit warning against the outdated prolonged NPO ("resting the pancreas") myth.
type DiagnosticPOCUSPlan struct {
	SpecPLAssayResult         string `json:"spec_pl_assay_result"`
	POCUSSonographicSign      string `json:"pocus_sonographic_sign"`
	DuodenalCorrugationStatus string `json:"duodenal_corrugation_status"`
	BiliaryObstructionEHBDO   string `json:"biliary_obstruction_ehbdo"`
	TriaditisCobalaminNote    string `json:"triaditis_cobalamin_note"`
	OutdatedNPOMythWarning    string `json:"outdated_npo_myth_warning"`
}

// AntiemeticNutritionPlan captures Fuzapladib sodium (Panoquell-CA1 0.4 mg/kg IV q24h x 3d LFA-1 leukocyte extravasation inhibitor),
// dual antiemetic therapy (Maropitant NK-1 antagonist 1 mg/kg IV q24h + Ondansetron 5-HT3 antagonist 0.5 mg/kg IV q8h),
// visceral analgesia CRI (Fentanyl/Ketamine/Lidocaine CRI; avoiding NSAIDs in hypovolemic/SIRS patients),
// early enteral nutrition via Nasoesophageal (NE) or Esophagostomy (E-tube) feeding,
// Resting Energy Requirement (RER = 70 x WeightKg^0.75) caloric ramp schedule (25% -> 50% -> 75% -> 100% RER),
// and balanced isotonic crystalloid microcirculatory resuscitation plan.
type AntiemeticNutritionPlan struct {
	FuzapladibProtocol     string `json:"fuzapladib_protocol"`
	AntiemeticDualProtocol string `json:"antiemetic_dual_protocol"`
	VisceralAnalgesiaCRI   string `json:"visceral_analgesia_cri"`
	EarlyEnteralTubePlan   string `json:"early_enteral_tube_plan"`
	RERCaloricRampSchedule string `json:"rer_caloric_ramp_schedule"`
	FluidCrystalloidPlan   string `json:"fluid_crystalloid_plan"`
}

// EnteralPainCheck records a serial Spec cPL/fPL assay, Modified Glasgow Composite Measure Pain Scale (CMPS-SF) score,
// enteral caloric tolerance (% of RER), vomiting/regurgitation status, and clinician telemetry note.
type EnteralPainCheck struct {
	ID                       string    `json:"id"`
	SpecPLUgL                int       `json:"spec_pl_ug_l"`
	ModifiedGlasgowPainScore int       `json:"modified_glasgow_pain_score"`
	EnteralRERPercent        int       `json:"enteral_rer_percent"`
	VomitingRegurgStatus     string    `json:"vomiting_regurg_status"`
	RecordedBy               string    `json:"recorded_by"`
	ClinicalNote             string    `json:"clinical_note"`
	RecordedAt               time.Time `json:"recorded_at"`
}

// PancreatitisOperation represents an Acute Pancreatitis, Canine/Feline Spec cPL/fPL,
// POCUS Hypoechoic Pancreas/Hyperechoic Mesentery, Maropitant/Fuzapladib & Early Enteral Nutrition triage operation.
type PancreatitisOperation struct {
	ID                           string                    `json:"id"`
	CaseID                       string                    `json:"case_id"`
	PetName                      string                    `json:"pet_name"`
	Species                      string                    `json:"species"`
	Breed                        string                    `json:"breed"`
	WeightKg                     float64                   `json:"weight_kg"`
	Status                       PancreatitisStatus        `json:"status"`
	SeverityClass                PancreatitisSeverityClass `json:"severity_class"`
	SpecPLUgL                    int                       `json:"spec_pl_ug_l"`
	ModifiedGlasgowPainScore     int                       `json:"modified_glasgow_pain_score"`
	EnteralRERPercent            int                       `json:"enteral_rer_percent"`
	POCUSHypoechoicPancreas      bool                      `json:"pocus_hypoechoic_pancreas"`
	HyperechoicPeripancreaticFat bool                      `json:"hyperechoic_peripancreatic_fat"`
	PrayerPositionPresent        bool                      `json:"prayer_position_present"`
	FelineTriaditisSuspected     bool                      `json:"feline_triaditis_suspected"`
	DiagnosticPlan               DiagnosticPOCUSPlan       `json:"diagnostic_plan"`
	NutritionPlan                AntiemeticNutritionPlan   `json:"nutrition_plan"`
	EnteralChecks                []EnteralPainCheck        `json:"enteral_checks"`
	DischargedBy                 string                    `json:"discharged_by,omitempty"`
	DischargeSummary             string                    `json:"discharge_summary,omitempty"`
	DischargedAt                 *time.Time                `json:"discharged_at,omitempty"`
	CreatedAt                    time.Time                 `json:"created_at"`
	UpdatedAt                    time.Time                 `json:"updated_at"`
}

// Validate checks domain invariants on PancreatitisOperation.
func (op PancreatitisOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.CaseID) == "" {
		return fmt.Errorf("case_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if strings.TrimSpace(op.Species) == "" {
		return fmt.Errorf("species is required")
	}
	if op.WeightKg <= 0 || op.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", op.WeightKg)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if !op.SeverityClass.IsValid() {
		return fmt.Errorf("invalid severity_class: %q", op.SeverityClass)
	}
	if op.SpecPLUgL < 0 || op.SpecPLUgL > 5000 {
		return fmt.Errorf("spec_pl_ug_l must be in [0, 5000], got %d", op.SpecPLUgL)
	}
	if op.ModifiedGlasgowPainScore < 0 || op.ModifiedGlasgowPainScore > 24 {
		return fmt.Errorf("modified_glasgow_pain_score must be in [0, 24], got %d", op.ModifiedGlasgowPainScore)
	}
	if op.EnteralRERPercent < 0 || op.EnteralRERPercent > 150 {
		return fmt.Errorf("enteral_rer_percent must be in [0, 150], got %d", op.EnteralRERPercent)
	}
	return nil
}

// PancreatitisOverviewMetrics summarizes operational KPI telemetry across Acute Pancreatitis & Early Enteral operations.
type PancreatitisOverviewMetrics struct {
	TotalOperations              int `json:"total_operations"`
	AcutePancreatitisTriageCount int `json:"acute_pancreatitis_triage_count"`
	EnteralNutritionStagedCount  int `json:"enteral_nutrition_staged_count"`
	ClearedEnteralCount          int `json:"cleared_enteral_count"`
	SevereSpecPLElevatedCount    int `json:"severe_spec_pl_elevated_count"`
	FelineTriaditisCount         int `json:"feline_triaditis_count"`
}
