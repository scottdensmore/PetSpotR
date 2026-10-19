package domain

import (
	"fmt"
	"strings"
	"time"
)

// OncologicTLSStatus represents the clinical workflow state of an Acute Tumor Lysis Syndrome (TLS) &
// Oncologic Hypercalcemia of Malignancy, Hyperkalemia/Hyperphosphatemia/Hypocalcemia,
// Rasburicase/Allopurinol & Zoledronate/Saline Diuresis triage operation.
type OncologicTLSStatus string

const (
	TLSStatusAcuteMetabolicTriage     OncologicTLSStatus = "ACUTE_TLS_HYPERCALCEMIA_METABOLIC_TRIAGE"
	TLSStatusDiuresisStaged           OncologicTLSStatus = "SALINE_DIURESIS_RASBURICASE_ZOLEDRONATE_STAGED"
	TLSStatusNormoelectrolyticCleared OncologicTLSStatus = "ELECTROLYTES_NORMALIZED_RENAL_PROTECTED_CLEARED"
)

// IsValid returns true when OncologicTLSStatus is a recognized lifecycle state.
func (s OncologicTLSStatus) IsValid() bool {
	switch s {
	case TLSStatusAcuteMetabolicTriage,
		TLSStatusDiuresisStaged,
		TLSStatusNormoelectrolyticCleared:
		return true
	default:
		return false
	}
}

// OncologicEmergencyType classifies the acute oncologic metabolic or hematologic emergency.
type OncologicEmergencyType string

const (
	OncologicEmergencyTumorLysisLymphoma OncologicEmergencyType = "ACUTE_TUMOR_LYSIS_SYNDROME_LYMPHOMA"
	OncologicEmergencyHypercalcemiaPTHrP OncologicEmergencyType = "PARANEOPLASTIC_HYPERCALCEMIA_PTHRP"
	OncologicEmergencyFebrileNeutropenia OncologicEmergencyType = "FEBRILE_NEUTROPENIA_CHEMO_SEPSIS"
)

// IsValid returns true when OncologicEmergencyType is a recognized oncologic emergency classification.
func (e OncologicEmergencyType) IsValid() bool {
	switch e {
	case OncologicEmergencyTumorLysisLymphoma,
		OncologicEmergencyHypercalcemiaPTHrP,
		OncologicEmergencyFebrileNeutropenia:
		return true
	default:
		return false
	}
}

// MetabolicNephroprotectionPlan captures the oncologic emergency pathophysiology, Calcium x Phosphorus
// (Ca x P) product soft-tissue mineralization risk, ionized calcium or potassium arrhythmia alert,
// strict contraindication of glucocorticoids (prednisone/dexamethasone) prior to definitive lymph node
// or bone marrow cytology/biopsy, warning against sodium bicarbonate urine alkalinization in TLS,
// and urate/calcium phosphate tubular nephropathy risk.
type MetabolicNephroprotectionPlan struct {
	EmergencyPathophysiology            string `json:"emergency_pathophysiology"`
	CalciumPhosphorusProductNote        string `json:"calcium_phosphorus_product_note"`
	IonizedCalciumOrPotassiumAlert      string `json:"ionized_calcium_or_potassium_alert"`
	SteroidBeforeBiopsyContraindication string `json:"steroid_before_biopsy_contraindication"`
	SodiumBicarbonateUrineNote          string `json:"sodium_bicarbonate_urine_note"`
	UrateOxalateNephropathyRisk         string `json:"urate_oxalate_nephropathy_risk"`
}

// DiuresisBisphosphonatePlan captures 0.9% NaCl calcium-free saline diuresis, Rasburicase / Allopurinol
// urate reduction protocol, Zoledronate (0.25 mg/kg IV over 15 min) / Calcitonin protocol for refractory
// hypercalcemia, hyperkalemia cardioprotection (10% Calcium Gluconate / Regular Insulin + Dextrose),
// enteral phosphate binder or furosemide loop diuretic (strictly after euvolemia), and serial electrolyte/ECG schedule.
type DiuresisBisphosphonatePlan struct {
	ZeroCalciumSalineDiuresisPlan    string `json:"zero_calcium_saline_diuresis_plan"`
	RasburicaseAllopurinolProtocol   string `json:"rasburicase_allopurinol_protocol"`
	ZoledronateCalcitoninProtocol    string `json:"zoledronate_calcitonin_protocol"`
	HyperkalemiaCardioprotectionPlan string `json:"hyperkalemia_cardioprotection_plan"`
	PhosphateBinderOrLoopDiuretic    string `json:"phosphate_binder_or_loop_diuretic"`
	SerialElectrolyteECGSchedule     string `json:"serial_electrolyte_ecg_schedule"`
}

// OncologicElectrolyteCheck records a serial electrolyte and renal panel check (K+, Phosphorus, iCa, Uric Acid, Creatinine).
type OncologicElectrolyteCheck struct {
	ID                  string    `json:"id"`
	SerumPotassiumMmolL float64   `json:"serum_potassium_mmol_l"`
	SerumPhosphorusMgDL float64   `json:"serum_phosphorus_mg_dl"`
	IonizedCalciumMmolL float64   `json:"ionized_calcium_mmol_l"`
	UricAcidMgDL        float64   `json:"uric_acid_mg_dl"`
	SerumCreatinineMgDL float64   `json:"serum_creatinine_mg_dl"`
	RecordedBy          string    `json:"recorded_by"`
	ClinicalNote        string    `json:"clinical_note"`
	RecordedAt          time.Time `json:"recorded_at"`
}

// OncologicTLSOperation represents an Acute Tumor Lysis Syndrome (TLS) & Oncologic Hypercalcemia
// of Malignancy triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type OncologicTLSOperation struct {
	ID                          string                        `json:"id"`
	CaseID                      string                        `json:"case_id"`
	PetName                     string                        `json:"pet_name"`
	Species                     string                        `json:"species"`
	Breed                       string                        `json:"breed"`
	WeightKg                    float64                       `json:"weight_kg"`
	Status                      OncologicTLSStatus            `json:"status"`
	EmergencyType               OncologicEmergencyType        `json:"emergency_type"`
	SerumPotassiumMmolL         float64                       `json:"serum_potassium_mmol_l"`
	SerumPhosphorusMgDL         float64                       `json:"serum_phosphorus_mg_dl"`
	IonizedCalciumMmolL         float64                       `json:"ionized_calcium_mmol_l"`
	UricAcidMgDL                float64                       `json:"uric_acid_mg_dl"`
	SerumCreatinineMgDL         float64                       `json:"serum_creatinine_mg_dl"`
	CaXPProduct                 float64                       `json:"ca_x_p_product"`
	DefinitiveCytologyConfirmed bool                          `json:"definitive_cytology_confirmed"`
	MetabolicPlan               MetabolicNephroprotectionPlan `json:"metabolic_plan"`
	DiuresisPlan                DiuresisBisphosphonatePlan    `json:"diuresis_plan"`
	ElectrolyteChecks           []OncologicElectrolyteCheck   `json:"electrolyte_checks"`
	DischargedBy                string                        `json:"discharged_by,omitempty"`
	DischargeSummary            string                        `json:"discharge_summary,omitempty"`
	DischargedAt                *time.Time                    `json:"discharged_at,omitempty"`
	CreatedAt                   time.Time                     `json:"created_at"`
	UpdatedAt                   time.Time                     `json:"updated_at"`
}

// Validate checks domain invariants on OncologicTLSOperation.
func (op OncologicTLSOperation) Validate() error {
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
	if !op.EmergencyType.IsValid() {
		return fmt.Errorf("invalid emergency_type: %q", op.EmergencyType)
	}
	return nil
}

// OncologicTLSOverviewMetrics summarizes aggregate counts across Oncologic TLS & Hypercalcemia operations.
type OncologicTLSOverviewMetrics struct {
	TotalOperations               int `json:"total_operations"`
	AcuteMetabolicTriageCount     int `json:"acute_metabolic_triage_count"`
	DiuresisStagedCount           int `json:"diuresis_staged_count"`
	ClearedNormoelectrolyticCount int `json:"cleared_normoelectrolytic_count"`
	TumorLysisSyndromeCount       int `json:"tumor_lysis_syndrome_count"`
	HypercalcemiaMalignancyCount  int `json:"hypercalcemia_malignancy_count"`
}
