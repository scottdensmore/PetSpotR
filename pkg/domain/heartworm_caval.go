package domain

import (
	"fmt"
	"strings"
	"time"
)

// HeartwormCavalStatus represents the clinical workflow state of an Acute Heartworm Caval Syndrome
// (Dirofilaria immitis), Vena Cava Worm Burden Echocardiography, Intravascular Hemolysis (Hemoglobinuria),
// Jugular Venotomy Extraction & Melarsomine Split-Dose / Doxycycline / Macrocyclic Lactone
// Thromboembolism Prevention triage operation.
type HeartwormCavalStatus string

const (
	HeartwormStatusJugularVenotomyWolbachiaDoxyStaged HeartwormCavalStatus = "JUGULAR_VENOTOMY_WOLBACHIA_DOXY_STAGED"
	HeartwormStatusPulmonaryHypertensionPTEMonitored  HeartwormCavalStatus = "PULMONARY_HYPERTENSION_PTE_MONITORED"
	HeartwormStatusCavalPatentAntigenNegativeCleared  HeartwormCavalStatus = "CAVAL_PATENT_ANTIGEN_NEGATIVE_CLEARED"
)

// IsValid returns true when HeartwormCavalStatus is a recognized lifecycle state.
func (s HeartwormCavalStatus) IsValid() bool {
	switch s {
	case HeartwormStatusJugularVenotomyWolbachiaDoxyStaged,
		HeartwormStatusPulmonaryHypertensionPTEMonitored,
		HeartwormStatusCavalPatentAntigenNegativeCleared:
		return true
	default:
		return false
	}
}

// HeartwormClass classifies the American Heartworm Society (AHS) clinical severity class
// or feline HARD (Heartworm-Associated Respiratory Disease) presentation.
type HeartwormClass string

const (
	HeartwormClassIVCavalSyndrome          HeartwormClass = "CLASS_IV_CAVAL_SYNDROME"
	HeartwormClassIIISeverePTERCHF         HeartwormClass = "CLASS_III_SEVERE_PTE_RCHF"
	HeartwormClassIIModerateAntigenPos     HeartwormClass = "CLASS_II_MODERATE_ANTIGEN_POS"
	HeartwormClassFelineHARDDirofilariasis HeartwormClass = "FELINE_HARD_RESPIRATORY_DIROFILARIASIS"
)

// IsValid returns true when HeartwormClass is a recognized severity classification.
func (c HeartwormClass) IsValid() bool {
	switch c {
	case HeartwormClassIVCavalSyndrome,
		HeartwormClassIIISeverePTERCHF,
		HeartwormClassIIModerateAntigenPos,
		HeartwormClassFelineHARDDirofilariasis:
		return true
	default:
		return false
	}
}

// HeartwormProtocol classifies the transvenous surgical extraction or medical adulticide/Wolbachia protocol.
type HeartwormProtocol string

const (
	HeartwormProtocolRightJugularIshiharaExtraction HeartwormProtocol = "RIGHT_JUGULAR_ISHIHARA_FORCEPS_EXTRACTION"
	HeartwormProtocolSplitDoseMelarsomineDay609091  HeartwormProtocol = "SPLIT_DOSE_MELARSOMINE_DAY60_90_91"
	HeartwormProtocolDoxycyclineWolbachia28Day      HeartwormProtocol = "DOXYCYCLINE_WOLBACHIA_28DAY_PRETREAT"
	HeartwormProtocolSildenafilPrednisoneCageRest   HeartwormProtocol = "SILDENAFIL_PREDNISONE_CAGE_REST"
)

// IsValid returns true when HeartwormProtocol is a recognized extraction or adulticide protocol.
func (p HeartwormProtocol) IsValid() bool {
	switch p {
	case HeartwormProtocolRightJugularIshiharaExtraction,
		HeartwormProtocolSplitDoseMelarsomineDay609091,
		HeartwormProtocolDoxycyclineWolbachia28Day,
		HeartwormProtocolSildenafilPrednisoneCageRest:
		return true
	default:
		return false
	}
}

// HeartwormInterventionStaging captures transvenous right external jugular venotomy extraction
// (Ishihara flexible alligator forceps or endoscopic retrieval basket), Wolbachia doxycycline
// 28-day pretreatment, macrocyclic lactone + antihistamine/steroid microfilaricidal shock prophylaxis,
// AHS 3-dose split Melarsomine schedule (Day 60, Day 90, Day 91), Sildenafil pulmonary hypertension
// therapy, and strict cage-rest thromboembolism prevention.
type HeartwormInterventionStaging struct {
	Protocol                            HeartwormProtocol `json:"protocol"`
	RightJugularVenotomyExtractionPlan  string            `json:"right_jugular_venotomy_extraction_plan"`
	ImmediateMelarsomineContraindicated bool              `json:"immediate_melarsomine_contraindicated"`
	AdulticideContraindicationGuardNote string            `json:"adulticide_contraindication_guard_note"`
	WolbachiaDoxycyclineMgKgDay         float64           `json:"wolbachia_doxycycline_mg_kg_day"`
	WolbachiaDoxycycline28DayPlan       string            `json:"wolbachia_doxycycline_28day_plan"`
	MacrocyclicLactonePretreatPlan      string            `json:"macrocyclic_lactone_pretreat_plan"`
	PrednisoneTaperMgKgDay              float64           `json:"prednisone_taper_mg_kg_day"`
	PrednisoneAntiInflammatoryTaperPlan string            `json:"prednisone_anti_inflammatory_taper_plan"`
	SildenafilPulmonaryHypertensionPlan string            `json:"sildenafil_pulmonary_hypertension_plan"`
	SplitDoseMelarsomineScheduleNote    string            `json:"split_dose_melarsomine_schedule_note"`
	StrictCageRestWeeks                 int               `json:"strict_cage_rest_weeks"`
	StagedBy                            string            `json:"staged_by"`
	StagedAt                            time.Time         `json:"staged_at"`
}

// HeartwormHemodynamicCheck records a serial echocardiographic & intravascular hemolysis telemetry check
// (TR Jet Velocity m/s, Estimated RVSP mmHg, Worms Extracted Count, PCV %, Free Hemoglobinuria, Parallel Hyperechoic Caval Lines).
type HeartwormHemodynamicCheck struct {
	ID                            string    `json:"id"`
	TRJetVelocityMS               float64   `json:"tr_jet_velocity_ms"`
	EstimatedRVSPMmHg             float64   `json:"estimated_rvsp_mmhg"`
	WormsExtractedCount           int       `json:"worms_extracted_count"`
	PCVPercent                    float64   `json:"pcv_percent"`
	FreeHemoglobinuriaPresent     bool      `json:"free_hemoglobinuria_present"`
	ParallelHyperechoicCavalLines bool      `json:"parallel_hyperechoic_caval_lines"`
	SildenafilMgKgDose            float64   `json:"sildenafil_mg_kg_dose"`
	RecordedBy                    string    `json:"recorded_by"`
	ClinicalNote                  string    `json:"clinical_note"`
	RecordedAt                    time.Time `json:"recorded_at"`
}

// HeartwormClearance captures discharge/step-down clearance verification once caval patency is restored,
// intravascular hemolysis (hemoglobinuria) has resolved, tricuspid regurgitation velocity is <= 2.8 m/s,
// and strict cage rest is verified.
type HeartwormClearance struct {
	ParallelHyperechoicCavalLines bool      `json:"parallel_hyperechoic_caval_lines"`
	FreeHemoglobinuriaPresent     bool      `json:"free_hemoglobinuria_present"`
	TRJetVelocityMS               float64   `json:"tr_jet_velocity_ms"`
	StrictCageRestVerified        bool      `json:"strict_cage_rest_verified"`
	AntigenNegativeOrStagedCure   bool      `json:"antigen_negative_or_staged_cure"`
	ClearedBy                     string    `json:"cleared_by"`
	ClearanceSummary              string    `json:"clearance_summary"`
	ClearedAt                     time.Time `json:"cleared_at"`
}

// HeartwormCavalOperation represents an Acute Heartworm Caval Syndrome (Dirofilaria immitis),
// Vena Cava Worm Burden Echocardiography, Intravascular Hemolysis (Hemoglobinuria),
// Jugular Venotomy Extraction & Melarsomine Split-Dose / Doxycycline / Macrocyclic Lactone
// Thromboembolism Prevention triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type HeartwormCavalOperation struct {
	ID                                  string                       `json:"id"`
	CaseID                              string                       `json:"case_id"`
	PetName                             string                       `json:"pet_name"`
	Species                             string                       `json:"species"`
	Breed                               string                       `json:"breed"`
	WeightKg                            float64                      `json:"weight_kg"`
	Status                              HeartwormCavalStatus         `json:"status"`
	SeverityClass                       HeartwormClass               `json:"severity_class"`
	Protocol                            HeartwormProtocol            `json:"protocol"`
	TRJetVelocityMS                     float64                      `json:"tr_jet_velocity_ms"`
	EstimatedRVSPMmHg                   float64                      `json:"estimated_rvsp_mmhg"`
	WormsExtractedCount                 int                          `json:"worms_extracted_count"`
	PCVPercent                          float64                      `json:"pcv_percent"`
	FreeHemoglobinuriaPresent           bool                         `json:"free_hemoglobinuria_present"`
	ParallelHyperechoicCavalLines       bool                         `json:"parallel_hyperechoic_caval_lines"`
	ImmediateMelarsomineContraindicated bool                         `json:"immediate_melarsomine_contraindicated"`
	WolbachiaDoxycyclineMgKgDay         float64                      `json:"wolbachia_doxycycline_mg_kg_day"`
	PrednisoneTaperMgKgDay              float64                      `json:"prednisone_taper_mg_kg_day"`
	StrictCageRestWeeks                 int                          `json:"strict_cage_rest_weeks"`
	SildenafilRequiredAlert             bool                         `json:"sildenafil_required_alert"`
	PathophysiologySummary              string                       `json:"pathophysiology_summary"`
	EchocardiographyCavalNote           string                       `json:"echocardiography_caval_note"`
	HemolysisHemoglobinuriaNote         string                       `json:"hemolysis_hemoglobinuria_note"`
	InterventionStaging                 HeartwormInterventionStaging `json:"intervention_staging"`
	HemodynamicChecks                   []HeartwormHemodynamicCheck  `json:"hemodynamic_checks"`
	Clearance                           *HeartwormClearance          `json:"clearance,omitempty"`
	DischargedBy                        string                       `json:"discharged_by,omitempty"`
	DischargeSummary                    string                       `json:"discharge_summary,omitempty"`
	DischargedAt                        *time.Time                   `json:"discharged_at,omitempty"`
	CreatedAt                           time.Time                    `json:"created_at"`
	UpdatedAt                           time.Time                    `json:"updated_at"`
}

// Validate checks domain invariants on HeartwormCavalOperation.
func (op HeartwormCavalOperation) Validate() error {
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
	if !op.SeverityClass.IsValid() {
		return fmt.Errorf("invalid severity_class: %q", op.SeverityClass)
	}
	if !op.Protocol.IsValid() {
		return fmt.Errorf("invalid protocol: %q", op.Protocol)
	}
	return nil
}

// HeartwormOverview summarizes aggregate telemetry counts across Heartworm Caval Syndrome operations.
type HeartwormOverview struct {
	TotalOperations                        int `json:"total_operations"`
	JugularVenotomyWolbachiaStagedCount    int `json:"jugular_venotomy_wolbachia_staged_count"`
	PulmonaryHypertensionPTEMonitoredCount int `json:"pulmonary_hypertension_pte_monitored_count"`
	CavalPatentClearedCount                int `json:"caval_patent_cleared_count"`
	ClassIVCavalSyndromeCount              int `json:"class_iv_caval_syndrome_count"`
	SeverePulmonaryHypertensionCount       int `json:"severe_pulmonary_hypertension_count"`
	ActiveHemoglobinuriaCount              int `json:"active_hemoglobinuria_count"`
}
