package domain

import (
	"fmt"
	"strings"
	"time"
)

// VectorBorneStatus represents the clinical workflow state of an Acute Vector-Borne &
// Rickettsial Emergency (Lyme Nephritis, Anaplasmosis, Ehrlichiosis, Rocky Mountain Spotted Fever,
// Babesiosis & Cytauxzoonosis), SNAP 4Dx Plus / PCR Triage, Doxycycline, Atovaquone + Azithromycin &
// Imidocarb Dipropionate triage operation.
type VectorBorneStatus string

const (
	VectorBorneStatusAntimicrobialStaged       VectorBorneStatus = "DOXYCYCLINE_ATOVAQUONE_IMIDOCARB_STAGED"
	VectorBorneStatusThrombocytopeniaMonitored VectorBorneStatus = "THROMBOCYTOPENIA_PROTEINURIA_MONITORED"
	VectorBorneStatusPCRNegativeCleared        VectorBorneStatus = "PCR_NEGATIVE_PLATELETS_RENAL_CLEARED"
)

// IsValid returns true when VectorBorneStatus is a recognized lifecycle state.
func (s VectorBorneStatus) IsValid() bool {
	switch s {
	case VectorBorneStatusAntimicrobialStaged,
		VectorBorneStatusThrombocytopeniaMonitored,
		VectorBorneStatusPCRNegativeCleared:
		return true
	default:
		return false
	}
}

// VectorBorneSyndrome classifies the acute tick-borne rickettsial, spirochetal, or protozoal emergency.
type VectorBorneSyndrome string

const (
	VectorBorneSyndromeLymeNephritis      VectorBorneSyndrome = "LYME_NEPHRITIS_BORRELIA_UPC"
	VectorBorneSyndromeEhrlichiaAnaplasma VectorBorneSyndrome = "EHRLICHIOSIS_ANAPLASMOSIS_MORULAE"
	VectorBorneSyndromeRMSFVasculitis     VectorBorneSyndrome = "RMSF_RICKETTSIA_VASCULITIS"
	VectorBorneSyndromeBabesiosis         VectorBorneSyndrome = "BABESIOSIS_GIBSONI_CANIS"
	VectorBorneSyndromeCytauxzoonosis     VectorBorneSyndrome = "CYTAUXZOONOSIS_FELIS_SCHIZONTS"
)

// IsValid returns true when VectorBorneSyndrome is a recognized syndrome classification.
func (s VectorBorneSyndrome) IsValid() bool {
	switch s {
	case VectorBorneSyndromeLymeNephritis,
		VectorBorneSyndromeEhrlichiaAnaplasma,
		VectorBorneSyndromeRMSFVasculitis,
		VectorBorneSyndromeBabesiosis,
		VectorBorneSyndromeCytauxzoonosis:
		return true
	default:
		return false
	}
}

// VectorBorneAssay identifies the primary diagnostic modality used to confirm the vector-borne infection.
type VectorBorneAssay string

const (
	VectorBorneAssaySNAP4DxPlusC6   VectorBorneAssay = "SNAP_4DX_PLUS_C6_PEPTIDE"
	VectorBorneAssayQuantitativePCR VectorBorneAssay = "QUANTITATIVE_PCR_PANEL"
	VectorBorneAssayBuffyCoatSmear  VectorBorneAssay = "BUFFY_COAT_MORULAE_PIROPLASM_SMEAR"
	VectorBorneAssayUrinePCRatio    VectorBorneAssay = "URINE_PROTEIN_CREATININE_RATIO"
)

// IsValid returns true when VectorBorneAssay is a recognized diagnostic assay.
func (a VectorBorneAssay) IsValid() bool {
	switch a {
	case VectorBorneAssaySNAP4DxPlusC6,
		VectorBorneAssayQuantitativePCR,
		VectorBorneAssayBuffyCoatSmear,
		VectorBorneAssayUrinePCRatio:
		return true
	default:
		return false
	}
}

// VectorBorneAntimicrobial identifies the targeted anti-rickettsial, anti-protozoal, or Lyme PLN regimen.
type VectorBorneAntimicrobial string

const (
	VectorBorneAntimicrobialDoxycyclineWaterChase  VectorBorneAntimicrobial = "DOXYCYCLINE_10MG_KG_WATER_CHASE"
	VectorBorneAntimicrobialAtovaquoneAzithromycin VectorBorneAntimicrobial = "ATOVAQUONE_15MG_KG_TID_AZITHROMYCIN"
	VectorBorneAntimicrobialImidocarbAtropine      VectorBorneAntimicrobial = "IMIDOCARB_6_6MG_KG_ATROPINE_PREMED"
	VectorBorneAntimicrobialMycophenolateLymePLN   VectorBorneAntimicrobial = "MYCOPHENOLATE_TELMISARTAN_LYME_PLN"
)

// IsValid returns true when VectorBorneAntimicrobial is a recognized antimicrobial protocol.
func (p VectorBorneAntimicrobial) IsValid() bool {
	switch p {
	case VectorBorneAntimicrobialDoxycyclineWaterChase,
		VectorBorneAntimicrobialAtovaquoneAzithromycin,
		VectorBorneAntimicrobialImidocarbAtropine,
		VectorBorneAntimicrobialMycophenolateLymePLN:
		return true
	default:
		return false
	}
}

// VectorBorneAntimicrobialStaging captures staged Doxycycline (10 mg/kg/day PO/IV with mandatory feline
// 5–6 mL water bolus / suspension stricture guard), Atovaquone (13.5–15 mg/kg PO TID with fatty meal) +
// Azithromycin (10 mg/kg PO q24h) for Babesia gibsoni & Cytauxzoon felis, Imidocarb Dipropionate
// (6.6 mg/kg IM repeated in 14 days with mandatory Atropine 0.02–0.04 mg/kg SC cholinergic premedication),
// and Telmisartan/Benazepril + Mycophenolate Mofetil for Lyme Nephritis protein-losing nephropathy (UPC >= 0.5).
type VectorBorneAntimicrobialStaging struct {
	Protocol                          VectorBorneAntimicrobial `json:"protocol"`
	DoxycyclineMgKgDay                float64                  `json:"doxycycline_mg_kg_day"`
	AtovaquoneMgKgTID                 float64                  `json:"atovaquone_mg_kg_tid"`
	AzithromycinMgKgDay               float64                  `json:"azithromycin_mg_kg_day"`
	ImidocarbMgKgIM                   float64                  `json:"imidocarb_mg_kg_im"`
	EsophagealWaterChaseVerified      bool                     `json:"esophageal_water_chase_verified"`
	AtropinePremedVerified            bool                     `json:"atropine_premed_verified"`
	FattyMealBioavailabilityNote      string                   `json:"fatty_meal_bioavailability_note"`
	LymeNephritisImmunomodulationNote string                   `json:"lyme_nephritis_immunomodulation_note"`
	EsophagealStrictureGuardNote      string                   `json:"esophageal_stricture_guard_note"`
	CholinergicAtropineGuardNote      string                   `json:"cholinergic_atropine_guard_note"`
	StagedBy                          string                   `json:"staged_by"`
	ClinicalRationale                 string                   `json:"clinical_rationale"`
	StagedAt                          time.Time                `json:"staged_at"`
}

// VectorBorneHematologyCheck records a serial platelet count, packed cell volume (PCV%),
// urine protein:creatinine (UPC) ratio, serum albumin, and buffy-coat morulae/piroplasm check.
type VectorBorneHematologyCheck struct {
	ID                         string    `json:"id"`
	PlateletCountKUL           float64   `json:"platelet_count_kul"`
	PCVPercent                 float64   `json:"pcv_percent"`
	UrinePCRatio               float64   `json:"urine_pc_ratio"`
	SerumAlbuminGDL            float64   `json:"serum_albumin_g_dl"`
	MorulaeOrPiroplasmsCleared bool      `json:"morulae_or_piroplasms_cleared"`
	RecordedBy                 string    `json:"recorded_by"`
	ClinicalNote               string    `json:"clinical_note"`
	RecordedAt                 time.Time `json:"recorded_at"`
}

// VectorBorneClearance records PCR-negative pathogen clearance, platelet recovery (>= 150 K/uL),
// PCV regeneration (>= 35%), renal proteinuria resolution (UPC < 0.5), and normothermic appetite.
type VectorBorneClearance struct {
	PlateletCountKUL           float64   `json:"platelet_count_kul"`
	PCVPercent                 float64   `json:"pcv_percent"`
	UrinePCRatio               float64   `json:"urine_pc_ratio"`
	PCRNegativeVerified        bool      `json:"pcr_negative_verified"`
	NormothermicEatingVerified bool      `json:"normothermic_eating_verified"`
	ClearedBy                  string    `json:"cleared_by"`
	ClearanceSummary           string    `json:"clearance_summary"`
	ClearedAt                  time.Time `json:"cleared_at"`
}

// VectorBorneOperation represents an Acute Vector-Borne & Rickettsial Emergency triage operation
// in the PetSpotR Veterinary Emergency ICU cockpit.
type VectorBorneOperation struct {
	ID                           string                           `json:"id"`
	CaseID                       string                           `json:"case_id"`
	PetName                      string                           `json:"pet_name"`
	Species                      string                           `json:"species"`
	Breed                        string                           `json:"breed"`
	WeightKg                     float64                          `json:"weight_kg"`
	Status                       VectorBorneStatus                `json:"status"`
	Syndrome                     VectorBorneSyndrome              `json:"syndrome"`
	Assay                        VectorBorneAssay                 `json:"assay"`
	AntimicrobialProtocol        VectorBorneAntimicrobial         `json:"antimicrobial_protocol"`
	PlateletCountKUL             float64                          `json:"platelet_count_kul"`
	PCVPercent                   float64                          `json:"pcv_percent"`
	UrinePCRatio                 float64                          `json:"urine_pc_ratio"`
	SerumAlbuminGDL              float64                          `json:"serum_albumin_g_dl"`
	DoxycyclineMgKgDay           float64                          `json:"doxycycline_mg_kg_day"`
	AtovaquoneMgKgTID            float64                          `json:"atovaquone_mg_kg_tid"`
	ImidocarbMgKgIM              float64                          `json:"imidocarb_mg_kg_im"`
	LymeNephritisUPCPositive     bool                             `json:"lyme_nephritis_upc_positive"`
	EsophagealWaterChaseVerified bool                             `json:"esophageal_water_chase_verified"`
	AtropinePremedVerified       bool                             `json:"atropine_premed_verified"`
	PathophysiologySummary       string                           `json:"pathophysiology_summary"`
	SafetyGuardAlerts            []string                         `json:"safety_guard_alerts"`
	AntimicrobialStaging         *VectorBorneAntimicrobialStaging `json:"antimicrobial_staging,omitempty"`
	HematologyChecks             []VectorBorneHematologyCheck     `json:"hematology_checks"`
	Clearance                    *VectorBorneClearance            `json:"clearance,omitempty"`
	DischargedBy                 string                           `json:"discharged_by,omitempty"`
	DischargeSummary             string                           `json:"discharge_summary,omitempty"`
	DischargedAt                 *time.Time                       `json:"discharged_at,omitempty"`
	CreatedAt                    time.Time                        `json:"created_at"`
	UpdatedAt                    time.Time                        `json:"updated_at"`
}

// Validate checks domain invariants on VectorBorneOperation.
func (op VectorBorneOperation) Validate() error {
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
	if !op.Syndrome.IsValid() {
		return fmt.Errorf("invalid syndrome: %q", op.Syndrome)
	}
	if !op.Assay.IsValid() {
		return fmt.Errorf("invalid assay: %q", op.Assay)
	}
	if !op.AntimicrobialProtocol.IsValid() {
		return fmt.Errorf("invalid antimicrobial_protocol: %q", op.AntimicrobialProtocol)
	}
	return nil
}

// VectorBorneOverview summarizes aggregate counts across Vector-Borne & Rickettsial operations.
type VectorBorneOverview struct {
	TotalOperations                 int `json:"total_operations"`
	AntimicrobialStagedCount        int `json:"antimicrobial_staged_count"`
	ThrombocytopeniaMonitoredCount  int `json:"thrombocytopenia_monitored_count"`
	PCRNegativeClearedCount         int `json:"pcr_negative_cleared_count"`
	LymeNephritisProteinuriaCount   int `json:"lyme_nephritis_proteinuria_count"`
	SevereThrombocytopeniaCount     int `json:"severe_thrombocytopenia_count"`
	BabesiaCytauxzoonProtozoalCount int `json:"babesia_cytauxzoon_protozoal_count"`
}
