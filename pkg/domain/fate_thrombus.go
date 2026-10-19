package domain

import (
	"fmt"
	"strings"
	"time"
)

// FATEThrombusStatus represents the clinical workflow state of a Feline Aortic Thromboembolism
// (FATE / Saddle Thrombus), 5 Ps (Pain, Pallor, Pulselessness, Paresis, Poikilothermy),
// Paired Glucose/Lactate Limb Gradient & Clopidogrel/Rivaroxaban triage operation.
type FATEThrombusStatus string

const (
	FATEStatusAcuteSaddle5PsTriage          FATEThrombusStatus = "ACUTE_SADDLE_THROMBUS_5PS_TRIAGE"
	FATEStatusMethadoneClopidogrelStaged    FATEThrombusStatus = "METHADONE_CLOPIDOGREL_ANTICOAG_STAGED"
	FATEStatusReperfusedNormokalemicCleared FATEThrombusStatus = "REPERFUSED_NORMOKALEMIC_DUAL_THERAPY_CLEARED"
)

// IsValid returns true when FATEThrombusStatus is a recognized lifecycle state.
func (s FATEThrombusStatus) IsValid() bool {
	switch s {
	case FATEStatusAcuteSaddle5PsTriage,
		FATEStatusMethadoneClopidogrelStaged,
		FATEStatusReperfusedNormokalemicCleared:
		return true
	default:
		return false
	}
}

// ThrombusDistributionType classifies anatomical distribution of arterial thromboembolism.
type ThrombusDistributionType string

const (
	ThrombusDistBilateralAorticSaddle  ThrombusDistributionType = "BILATERAL_AORTIC_TRIFURCATION_SADDLE"
	ThrombusDistUnilateralIliacFemoral ThrombusDistributionType = "UNILATERAL_ILIAC_FEMORAL_THROMBUS"
	ThrombusDistRightForeBrachial      ThrombusDistributionType = "RIGHT_FORE_BRACHIAL_THROMBOEMBOLISM"
)

// IsValid returns true when ThrombusDistributionType is a recognized anatomical distribution.
func (d ThrombusDistributionType) IsValid() bool {
	switch d {
	case ThrombusDistBilateralAorticSaddle,
		ThrombusDistUnilateralIliacFemoral,
		ThrombusDistRightForeBrachial:
		return true
	default:
		return false
	}
}

// MetabolicGradientPlan captures the 5 Ps clinical presentation, paired systemic-to-affected-limb
// blood glucose delta (> 30 mg/dL diagnostic), affected-limb-to-systemic lactate gradient,
// left atrial spontaneous echocardiographic contrast ("smoke") & LA:Ao ratio, reperfusion
// hyperkalemia washout alert, and strict passive warming (never direct heating pad on ischemic limbs) guidance.
type MetabolicGradientPlan struct {
	FivePsClinicalSummary          string `json:"five_ps_clinical_summary"`
	SystemicToLimbGlucoseDelta     string `json:"systemic_to_limb_glucose_delta"`
	LimbToSystemicLactateDelta     string `json:"limb_to_systemic_lactate_delta"`
	LeftAtrialSmokeLA2AoRatio      string `json:"left_atrial_smoke_la2ao_ratio"`
	ReperfusionHyperkalemiaAlert   string `json:"reperfusion_hyperkalemia_alert"`
	PassiveWarmingContraindication string `json:"passive_warming_contraindication"`
}

// AntithromboticAnalgesiaPlan captures full-mu opioid analgesia (Fentanyl CRI / Methadone IV —
// never relying solely on partial-agonist Buprenorphine for acute ischemic pain), Clopidogrel 75 mg
// PO oral loading dose (FAT CAT Trial) followed by 18.75 mg PO q24h, Factor Xa inhibitor (Rivaroxaban)
// or Low-Molecular-Weight Heparin (Enoxaparin / Dalteparin) dual therapy, concurrent cardiogenic CHF
// Furosemide + oxygen cage management, serial potassium / ECG monitoring, and Pimobendan LVOT caution.
type AntithromboticAnalgesiaPlan struct {
	FullMuOpioidCRIProtocol     string `json:"full_mu_opioid_cri_protocol"`
	ClopidogrelLoadingProtocol  string `json:"clopidogrel_loading_protocol"`
	FactorXaOrHeparinProtocol   string `json:"factor_xa_or_heparin_protocol"`
	ConcurrentCHFFurosemidePlan string `json:"concurrent_chf_furosemide_plan"`
	SerialPotassiumMonitoring   string `json:"serial_potassium_monitoring"`
	PimobendanLVOTNote          string `json:"pimobendan_lvot_note"`
}

// LimbPerfusionCheck records a serial paired glucose delta, lactate delta, serum potassium (K+),
// distal limb temperature, femoral/dorsal pedal pulse & Doppler flow check.
type LimbPerfusionCheck struct {
	ID                  string    `json:"id"`
	GlucoseDeltaMgDL    float64   `json:"glucose_delta_mg_dl"`
	LactateDeltaMmolL   float64   `json:"lactate_delta_mmol_l"`
	SerumPotassiumMmolL float64   `json:"serum_potassium_mmol_l"`
	DistalLimbTempC     float64   `json:"distal_limb_temp_c"`
	FemoralPulseStatus  string    `json:"femoral_pulse_status"`
	RecordedBy          string    `json:"recorded_by"`
	ClinicalNote        string    `json:"clinical_note"`
	RecordedAt          time.Time `json:"recorded_at"`
}

// FATEThrombusOperation represents a Feline Aortic Thromboembolism (FATE / Saddle Thrombus)
// triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type FATEThrombusOperation struct {
	ID                       string                      `json:"id"`
	CaseID                   string                      `json:"case_id"`
	PetName                  string                      `json:"pet_name"`
	Species                  string                      `json:"species"`
	Breed                    string                      `json:"breed"`
	WeightKg                 float64                     `json:"weight_kg"`
	Status                   FATEThrombusStatus          `json:"status"`
	DistributionType         ThrombusDistributionType    `json:"distribution_type"`
	SystemicGlucoseMgDL      float64                     `json:"systemic_glucose_mg_dl"`
	AffectedLimbGlucoseMgDL  float64                     `json:"affected_limb_glucose_mg_dl"`
	GlucoseDeltaMgDL         float64                     `json:"glucose_delta_mg_dl"`
	SystemicLactateMmolL     float64                     `json:"systemic_lactate_mmol_l"`
	AffectedLimbLactateMmolL float64                     `json:"affected_limb_lactate_mmol_l"`
	LactateDeltaMmolL        float64                     `json:"lactate_delta_mmol_l"`
	SerumPotassiumMmolL      float64                     `json:"serum_potassium_mmol_l"`
	DistalLimbTempC          float64                     `json:"distal_limb_temp_c"`
	LA2AoRatio               float64                     `json:"la2ao_ratio"`
	ConcurrentCardiogenicCHF bool                        `json:"concurrent_cardiogenic_chf"`
	GradientPlan             MetabolicGradientPlan       `json:"gradient_plan"`
	AntithromboticPlan       AntithromboticAnalgesiaPlan `json:"antithrombotic_plan"`
	PerfusionChecks          []LimbPerfusionCheck        `json:"perfusion_checks"`
	DischargedBy             string                      `json:"discharged_by,omitempty"`
	DischargeSummary         string                      `json:"discharge_summary,omitempty"`
	DischargedAt             *time.Time                  `json:"discharged_at,omitempty"`
	CreatedAt                time.Time                   `json:"created_at"`
	UpdatedAt                time.Time                   `json:"updated_at"`
}

// Validate checks domain invariants on FATEThrombusOperation.
func (op FATEThrombusOperation) Validate() error {
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
	if !op.DistributionType.IsValid() {
		return fmt.Errorf("invalid distribution_type: %q", op.DistributionType)
	}
	return nil
}

// FATEOverviewMetrics summarizes aggregate counts across FATE / Saddle Thrombus operations.
type FATEOverviewMetrics struct {
	TotalOperations        int `json:"total_operations"`
	AcuteSaddleTriageCount int `json:"acute_saddle_triage_count"`
	AnticoagStagedCount    int `json:"anticoag_staged_count"`
	ClearedReperfusedCount int `json:"cleared_reperfused_count"`
	BilateralSaddleCount   int `json:"bilateral_saddle_count"`
	ConcurrentCHFCount     int `json:"concurrent_chf_count"`
}
