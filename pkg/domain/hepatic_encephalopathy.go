package domain

import (
	"fmt"
	"strings"
	"time"
)

// HepaticTriageStatus represents the clinical workflow state of an Acute Hepatic Failure,
// Hepatic Encephalopathy (HE), Sago Palm (Cycad) / Amanita / Blue-Green Algae (Microcystin)
// Hepatotoxicity, N-Acetylcysteine (NAC) & Lactulose/Levetiracetam triage operation.
type HepaticTriageStatus string

const (
	HepaticStatusFulminantHETriage        HepaticTriageStatus = "ACUTE_FULMINANT_HEPATOTOXICITY_HE_TRIAGE"
	HepaticStatusNACLactuloseStaged       HepaticTriageStatus = "NAC_LACTULOSE_VITAMIN_K1_STAGED"
	HepaticStatusAmmoniaNormalizedCleared HepaticTriageStatus = "AMMONIA_NORMALIZED_SYNTHETIC_STABLE_CLEARED"
)

// IsValid returns true when HepaticTriageStatus is a recognized lifecycle state.
func (s HepaticTriageStatus) IsValid() bool {
	switch s {
	case HepaticStatusFulminantHETriage,
		HepaticStatusNACLactuloseStaged,
		HepaticStatusAmmoniaNormalizedCleared:
		return true
	default:
		return false
	}
}

// HepatotoxinEtiologyType classifies the primary hepatotoxic or metabolic etiology of acute hepatic failure / HE.
type HepatotoxinEtiologyType string

const (
	HepatotoxinEtiologySagoPalmCycasin       HepatotoxinEtiologyType = "SAGO_PALM_CYCASIN_HEPATOTOXICITY"
	HepatotoxinEtiologyBlueGreenAlgaeAmanita HepatotoxinEtiologyType = "BLUE_GREEN_ALGAE_MICROCYSTIN_AMANITA"
	HepatotoxinEtiologyFelineLipidosisPSS    HepatotoxinEtiologyType = "FELINE_HEPATIC_LIPIDOSIS_PSS_HE"
)

// IsValid returns true when HepatotoxinEtiologyType is a recognized hepatotoxicity etiology.
func (e HepatotoxinEtiologyType) IsValid() bool {
	switch e {
	case HepatotoxinEtiologySagoPalmCycasin,
		HepatotoxinEtiologyBlueGreenAlgaeAmanita,
		HepatotoxinEtiologyFelineLipidosisPSS:
		return true
	default:
		return false
	}
}

// HepatotoxinEncephalopathyPlan captures etiology pathophysiology (Sago Palm cycasin/MAM centrilobular
// necrosis, Blue-Green Algae microcystin-LR PP1/PP2A inhibition & Amanita alpha-amanitin RNA Pol II
// inhibition, or Feline Hepatic Lipidosis / Portosystemic Shunt HE), hyperammonemia neurological grading,
// synthetic failure hypoglycemia/coagulopathy, lactulose colonic NH3 -> NH4+ ion trapping, strict
// benzodiazepine (diazepam/midazolam) & methionine contraindications, and feline refeeding electrolyte alert.
type HepatotoxinEncephalopathyPlan struct {
	EtiologyPathophysiology         string `json:"etiology_pathophysiology"`
	HyperammonemiaNeuroStatus       string `json:"hyperammonemia_neuro_status"`
	HypoglycemiaCoagulopathyNote    string `json:"hypoglycemia_coagulopathy_note"`
	LactuloseAmmoniumTrappingPlan   string `json:"lactulose_ammonium_trapping_plan"`
	BenzodiazepineMethionineWarning string `json:"benzodiazepine_methionine_warning"`
	RefeedingElectrolyteAlert       string `json:"refeeding_electrolyte_alert"`
}

// NACLactuloseSupportPlan captures N-Acetylcysteine (NAC) 140 mg/kg IV loading + 70 mg/kg IV q6h x 7 doses
// (0.2 micron in-line filter) for glutathione replenishment, SAMe + Silybin (Silymarin) hepatoprotectants,
// Lactulose warm retention enema (1:3 warm water) + oral lactulose / neomycin / low-dose metronidazole,
// Levetiracetam 60 mg/kg IV loading for HE seizures, Dextrose CRI + Vitamin K1 + Fresh Frozen Plasma (FFP)
// synthetic/coagulopathy support, and potassium/phosphate/thiamine supplementation.
type NACLactuloseSupportPlan struct {
	NACGlutathioneLoadingProtocol  string `json:"nac_glutathione_loading_protocol"`
	SAMeSilybinHepatoprotectant    string `json:"same_silybin_hepatoprotectant"`
	LactuloseEnemaOralProtocol     string `json:"lactulose_enema_oral_protocol"`
	LevetiracetamNeuroProtocol     string `json:"levetiracetam_neuro_protocol"`
	DextroseVitaminK1FFPPlan       string `json:"dextrose_vitamin_k1_ffp_plan"`
	PotassiumPhosphateThiaminePlan string `json:"potassium_phosphate_thiamine_plan"`
}

// HepaticMetabolicCheck records a serial ALT (IU/L), Total Bilirubin (mg/dL), Blood Ammonia (umol/L),
// Blood Glucose (mg/dL), and Prothrombin Time (PT seconds) metabolic & synthetic liver check.
type HepaticMetabolicCheck struct {
	ID                 string    `json:"id"`
	ALTIUL             int       `json:"alt_iu_l"`
	TotalBilirubinMgDL float64   `json:"total_bilirubin_mg_dl"`
	BloodAmmoniaUmolL  float64   `json:"blood_ammonia_umol_l"`
	BloodGlucoseMgDL   float64   `json:"blood_glucose_mg_dl"`
	PTSeconds          float64   `json:"pt_seconds"`
	RecordedBy         string    `json:"recorded_by"`
	ClinicalNote       string    `json:"clinical_note"`
	RecordedAt         time.Time `json:"recorded_at"`
}

// HepaticOperation represents an Acute Hepatic Failure, Hepatic Encephalopathy & Hepatotoxicity
// triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type HepaticOperation struct {
	ID                          string                        `json:"id"`
	CaseID                      string                        `json:"case_id"`
	PetName                     string                        `json:"pet_name"`
	Species                     string                        `json:"species"`
	Breed                       string                        `json:"breed"`
	WeightKg                    float64                       `json:"weight_kg"`
	Status                      HepaticTriageStatus           `json:"status"`
	EtiologyType                HepatotoxinEtiologyType       `json:"etiology_type"`
	ALTIUL                      int                           `json:"alt_iu_l"`
	TotalBilirubinMgDL          float64                       `json:"total_bilirubin_mg_dl"`
	BloodAmmoniaUmolL           float64                       `json:"blood_ammonia_umol_l"`
	BloodGlucoseMgDL            float64                       `json:"blood_glucose_mg_dl"`
	PTSeconds                   float64                       `json:"pt_seconds"`
	EncephalopathyGrade         int                           `json:"encephalopathy_grade"`
	HeadPressingSeizuresPresent bool                          `json:"head_pressing_seizures_present"`
	EncephalopathyPlan          HepatotoxinEncephalopathyPlan `json:"encephalopathy_plan"`
	SupportPlan                 NACLactuloseSupportPlan       `json:"support_plan"`
	MetabolicChecks             []HepaticMetabolicCheck       `json:"metabolic_checks"`
	DischargedBy                string                        `json:"discharged_by,omitempty"`
	DischargeSummary            string                        `json:"discharge_summary,omitempty"`
	DischargedAt                *time.Time                    `json:"discharged_at,omitempty"`
	CreatedAt                   time.Time                     `json:"created_at"`
	UpdatedAt                   time.Time                     `json:"updated_at"`
}

// Validate checks domain invariants on HepaticOperation.
func (op HepaticOperation) Validate() error {
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
	if !op.EtiologyType.IsValid() {
		return fmt.Errorf("invalid etiology_type: %q", op.EtiologyType)
	}
	if op.EncephalopathyGrade < 0 || op.EncephalopathyGrade > 4 {
		return fmt.Errorf("encephalopathy_grade must be in [0, 4], got %d", op.EncephalopathyGrade)
	}
	return nil
}

// HepaticOverviewMetrics summarizes aggregate counts across Hepatic Encephalopathy & Hepatotoxicity operations.
type HepaticOverviewMetrics struct {
	TotalOperations                   int `json:"total_operations"`
	FulminantHETriageCount            int `json:"fulminant_he_triage_count"`
	NACLactuloseStagedCount           int `json:"nac_lactulose_staged_count"`
	ClearedHepaticCount               int `json:"cleared_hepatic_count"`
	SagoPalmMicrocystinCount          int `json:"sago_palm_microcystin_count"`
	HyperammonemicEncephalopathyCount int `json:"hyperammonemic_encephalopathy_count"`
}
