package domain

import (
	"fmt"
	"strings"
	"time"
)

// DICCoagulopathyStatus represents the clinical workflow state of an Acute Coagulopathy &
// Disseminated Intravascular Coagulation (DIC), PT/aPTT Prolongation, D-Dimer/FDP Elevation,
// Fresh Frozen Plasma (FFP) & Tranexamic Acid (TXA) triage operation.
type DICCoagulopathyStatus string

const (
	DICStatusAcuteOvertDICTriage            DICCoagulopathyStatus = "ACUTE_OVERT_DIC_HYPOFIBRINOLYSIS_TRIAGE"
	DICStatusFFPCryoprecipitateTXAStaged    DICCoagulopathyStatus = "FFP_CRYOPRECIPITATE_TXA_STAGED"
	DICStatusHemostasisNormocoagulableClear DICCoagulopathyStatus = "HEMOSTASIS_RESTORED_NORMOCOAGULABLE_CLEARED"
)

// IsValid returns true when DICCoagulopathyStatus is a recognized lifecycle state.
func (s DICCoagulopathyStatus) IsValid() bool {
	switch s {
	case DICStatusAcuteOvertDICTriage,
		DICStatusFFPCryoprecipitateTXAStaged,
		DICStatusHemostasisNormocoagulableClear:
		return true
	default:
		return false
	}
}

// DICPhasePhenotype classifies the pathophysiological phase / phenotype of acute coagulopathy or DIC.
type DICPhasePhenotype string

const (
	DICPhenotypeHypoCoagulableConsumptive  DICPhasePhenotype = "HYPO_COAGULABLE_HEMORRHAGIC_CONSUMPTIVE_DIC"
	DICPhenotypeHyperCoagulableEarly       DICPhasePhenotype = "HYPER_COAGULABLE_MICROTHROMBOTIC_EARLY_DIC"
	DICPhenotypeHyperfibrinolysisGreyhound DICPhasePhenotype = "HYPERFIBRINOLYSIS_GREYHOUND_TRAUMA_COAGULOPATHY"
)

// IsValid returns true when DICPhasePhenotype is a recognized phenotype.
func (p DICPhasePhenotype) IsValid() bool {
	switch p {
	case DICPhenotypeHypoCoagulableConsumptive,
		DICPhenotypeHyperCoagulableEarly,
		DICPhenotypeHyperfibrinolysisGreyhound:
		return true
	default:
		return false
	}
}

// CoagulationTEGProfilePlan captures the phase/phenotype pathophysiology, PT/aPTT prolongation summary,
// D-Dimer/FDP elevation & Fibrinogen/Antithrombin III depletion note, Thromboelastography (TEG/ROTEM)
// viscoelastic interpretation, Tranexamic Acid (TXA) / Epsilon-Aminocaproic Acid (EACA) fibrinolysis
// indication note, and strict contraindication of jugular venipuncture & IM injections in coagulopathic patients.
type CoagulationTEGProfilePlan struct {
	PhenotypePathophysiology            string `json:"phenotype_pathophysiology"`
	PTaPTTProlongationSummary           string `json:"pt_aptt_prolongation_summary"`
	DDimerFibrinogenATIIINote           string `json:"d_dimer_fibrinogen_atiii_note"`
	TEGViscoelasticInterpretation       string `json:"teg_viscoelastic_interpretation"`
	TXAFibrinolysisIndicationNote       string `json:"txa_fibrinolysis_indication_note"`
	JugularVenipunctureContraindication string `json:"jugular_venipuncture_contraindication"`
}

// FFPAntifibrinolyticPlan captures Fresh Frozen Plasma (FFP 10–20 mL/kg IV) clotting factor &
// Antithrombin III replenishment, Cryoprecipitate (1 unit/10 kg) for vWF/Factor VIII/Fibrinogen < 100 mg/dL,
// Tranexamic Acid (TXA 10 mg/kg IV slow over 15–20 min q8h) or Epsilon-Aminocaproic Acid (EACA 50–100 mg/kg IV/PO q8h),
// Antithrombin III / Low-Molecular-Weight Heparin (LMWH) phase-appropriate guidance, underlying trigger
// source control, and serial coagulation panel schedule.
type FFPAntifibrinolyticPlan struct {
	FreshFrozenPlasmaProtocol      string `json:"fresh_frozen_plasma_protocol"`
	CryoprecipitatevWFProtocol     string `json:"cryoprecipitate_vwf_protocol"`
	TranexamicAcidAminocaproicPlan string `json:"tranexamic_acid_aminocaproic_plan"`
	AntithrombinHeparinPhasePlan   string `json:"antithrombin_heparin_phase_plan"`
	UnderlyingTriggerControlPlan   string `json:"underlying_trigger_control_plan"`
	SerialCoagPanelSchedule        string `json:"serial_coag_panel_schedule"`
}

// CoagulationPanelCheck records a serial platelet count, PT, aPTT, D-Dimer, and fibrinogen check.
type CoagulationPanelCheck struct {
	ID               string    `json:"id"`
	PlateletCountKUl float64   `json:"platelet_count_k_ul"`
	PTSeconds        float64   `json:"pt_seconds"`
	APTTSeconds      float64   `json:"aptt_seconds"`
	DDimerNgML       float64   `json:"d_dimer_ng_ml"`
	FibrinogenMgDL   float64   `json:"fibrinogen_mg_dl"`
	RecordedBy       string    `json:"recorded_by"`
	ClinicalNote     string    `json:"clinical_note"`
	RecordedAt       time.Time `json:"recorded_at"`
}

// DICCoagulopathyOperation represents an Acute Coagulopathy & Disseminated Intravascular Coagulation (DIC)
// triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type DICCoagulopathyOperation struct {
	ID                         string                    `json:"id"`
	CaseID                     string                    `json:"case_id"`
	PetName                    string                    `json:"pet_name"`
	Species                    string                    `json:"species"`
	Breed                      string                    `json:"breed"`
	WeightKg                   float64                   `json:"weight_kg"`
	Status                     DICCoagulopathyStatus     `json:"status"`
	PhasePhenotype             DICPhasePhenotype         `json:"phase_phenotype"`
	PlateletCountKUl           float64                   `json:"platelet_count_k_ul"`
	PTSeconds                  float64                   `json:"pt_seconds"`
	APTTSeconds                float64                   `json:"aptt_seconds"`
	DDimerNgML                 float64                   `json:"d_dimer_ng_ml"`
	FibrinogenMgDL             float64                   `json:"fibrinogen_mg_dl"`
	AntithrombinPercent        float64                   `json:"antithrombin_percent"`
	SchistocytesPresent        bool                      `json:"schistocytes_present"`
	SpontaneousMucosalBleeding bool                      `json:"spontaneous_mucosal_bleeding"`
	ProfilePlan                CoagulationTEGProfilePlan `json:"profile_plan"`
	ComponentPlan              FFPAntifibrinolyticPlan   `json:"component_plan"`
	CoagChecks                 []CoagulationPanelCheck   `json:"coag_checks"`
	DischargedBy               string                    `json:"discharged_by,omitempty"`
	DischargeSummary           string                    `json:"discharge_summary,omitempty"`
	DischargedAt               *time.Time                `json:"discharged_at,omitempty"`
	CreatedAt                  time.Time                 `json:"created_at"`
	UpdatedAt                  time.Time                 `json:"updated_at"`
}

// Validate checks domain invariants on DICCoagulopathyOperation.
func (op DICCoagulopathyOperation) Validate() error {
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
	if !op.PhasePhenotype.IsValid() {
		return fmt.Errorf("invalid phase_phenotype: %q", op.PhasePhenotype)
	}
	return nil
}

// DICOverviewMetrics summarizes aggregate counts across DIC & Coagulopathy operations.
type DICOverviewMetrics struct {
	TotalOperations                 int `json:"total_operations"`
	OvertDICTriageCount             int `json:"overt_dic_triage_count"`
	FFPTXAStagedCount               int `json:"ffp_txa_staged_count"`
	ClearedNormocoagulableCount     int `json:"cleared_normocoagulable_count"`
	ConsumptiveHemorrhagicCount     int `json:"consumptive_hemorrhagic_count"`
	HyperfibrinolysisGreyhoundCount int `json:"hyperfibrinolysis_greyhound_count"`
}
