package domain

import (
	"fmt"
	"strings"
	"time"
)

// SystemicMycosesStatus represents the clinical workflow state of an Acute Systemic Mycoses
// & Fungal Emergencies (Blastomycosis, Histoplasmosis, Coccidioidomycosis, Cryptococcosis
// & Sinonasal Aspergillosis), Urine/Serum MiraVista Antigen EIA, Itraconazole/Fluconazole,
// Amphotericin B Lipid Complex Nephrotoxicity Guard & Clotrimazole Trephination triage operation.
type SystemicMycosesStatus string

const (
	MycosesStatusAzoleAmphotericinTrephinationStaged SystemicMycosesStatus = "AZOLE_AMPHOTERICIN_TREPHINATION_STAGED"
	MycosesStatusAntigenEIACreaOphthalmicMonitored   SystemicMycosesStatus = "ANTIGEN_EIA_CREA_OPHTHALMIC_MONITORED"
	MycosesStatusTwoNegativeAntigenEIACleared        SystemicMycosesStatus = "TWO_NEGATIVE_ANTIGEN_EIA_CLEARED"
)

// IsValid returns true when SystemicMycosesStatus is a recognized lifecycle state.
func (s SystemicMycosesStatus) IsValid() bool {
	switch s {
	case MycosesStatusAzoleAmphotericinTrephinationStaged,
		MycosesStatusAntigenEIACreaOphthalmicMonitored,
		MycosesStatusTwoNegativeAntigenEIACleared:
		return true
	default:
		return false
	}
}

// FungalSyndrome classifies the dimorphic systemic mycosis or opportunistic fungal syndrome.
type FungalSyndrome string

const (
	FungalSyndromeBlastomycosisBroadBasedBuddingYeast FungalSyndrome = "BLASTOMYCOSIS_BROAD_BASED_BUDDING_YEAST"
	FungalSyndromeHistoplasmosisIntracellular         FungalSyndrome = "HISTOPLASMOSIS_INTRACELLULAR_MACROPHAGE"
	FungalSyndromeCoccidioidomycosisSpheruleValley    FungalSyndrome = "COCCIDIOIDOMYCOSIS_SPHERULE_VALLEY_FEVER"
	FungalSyndromeCryptococcosisEncapsulatedCNSOcular FungalSyndrome = "CRYPTOCOCCOSIS_ENCAPSULATED_CNS_OCULAR"
	FungalSyndromeSinonasalAspergillosisCribriform    FungalSyndrome = "SINONASAL_ASPERGILLOSIS_CRIBRIFORM_CHECK"
)

// IsValid returns true when FungalSyndrome is a recognized systemic fungal classification.
func (f FungalSyndrome) IsValid() bool {
	switch f {
	case FungalSyndromeBlastomycosisBroadBasedBuddingYeast,
		FungalSyndromeHistoplasmosisIntracellular,
		FungalSyndromeCoccidioidomycosisSpheruleValley,
		FungalSyndromeCryptococcosisEncapsulatedCNSOcular,
		FungalSyndromeSinonasalAspergillosisCribriform:
		return true
	default:
		return false
	}
}

// AntifungalProtocol classifies the systemic azole, lipid-complex polyene, or local sinus trephination regimen.
type AntifungalProtocol string

const (
	AntifungalProtocolItraconazole5MgKgBrandOrSolution     AntifungalProtocol = "ITRACONAZOLE_5MG_KG_BRAND_OR_SOLUTION"
	AntifungalProtocolFluconazole10MgKgBIDCNSOcular        AntifungalProtocol = "FLUCONAZOLE_10MG_KG_BID_CNS_OCULAR"
	AntifungalProtocolAmphotericinBLipidComplexSaline      AntifungalProtocol = "AMPHOTERICIN_B_LIPID_COMPLEX_SALINE_DIURESIS"
	AntifungalProtocolClotrimazole1PctFrontalSinusInfusion AntifungalProtocol = "CLOTRIMAZOLE_1PCT_FRONTAL_SINUS_INFUSION"
)

// IsValid returns true when AntifungalProtocol is a recognized antifungal treatment regimen.
func (p AntifungalProtocol) IsValid() bool {
	switch p {
	case AntifungalProtocolItraconazole5MgKgBrandOrSolution,
		AntifungalProtocolFluconazole10MgKgBIDCNSOcular,
		AntifungalProtocolAmphotericinBLipidComplexSaline,
		AntifungalProtocolClotrimazole1PctFrontalSinusInfusion:
		return true
	default:
		return false
	}
}

// AntifungalTherapyStaging captures systemic Itraconazole (5 mg/kg/day innovator capsules or oral solution,
// blocking compounded bulk powder), Fluconazole (10 mg/kg PO q12h for CNS/Ocular/Prostatic penetration),
// Amphotericin B Lipid Complex (1–3 mg/kg IV 3x/wk with mandatory 0.9% NaCl pre/post-diuresis and
// Creatinine <= 2.5 mg/dL guard), 1% Clotrimazole frontal sinus trephination (requiring CT verification of
// intact cribriform plate), and short-course Dexamethasone-SP (0.1 mg/kg) for pulmonary Blastomycosis yeast die-off ARDS.
type AntifungalTherapyStaging struct {
	Protocol                                  AntifungalProtocol `json:"protocol"`
	ItraconazoleMgKgDay                       float64            `json:"itraconazole_mg_kg_day"`
	FluconazoleMgKgDay                        float64            `json:"fluconazole_mg_kg_day"`
	AmphotericinBLipidMgKgDose                float64            `json:"amphotericin_b_lipid_mg_kg_dose"`
	CompoundedBulkItraconazoleBlocked         bool               `json:"compounded_bulk_itraconazole_blocked"`
	CompoundedItraconazoleGuardNote           string             `json:"compounded_itraconazole_guard_note"`
	CribriformPlateIntactOnCT                 bool               `json:"cribriform_plate_intact_on_ct"`
	CribriformPlateTrephinationGuardNote      string             `json:"cribriform_plate_trephination_guard_note"`
	AmphotericinLipidSalineDiuresisVerified   bool               `json:"amphotericin_lipid_saline_diuresis_verified"`
	AmphotericinNephrotoxicityGuardNote       string             `json:"amphotericin_nephrotoxicity_guard_note"`
	DieOffDexamethasoneAntiInflammatoryStaged bool               `json:"die_off_dexamethasone_anti_inflammatory_staged"`
	PulmonaryDieOffARDSPlan                   string             `json:"pulmonary_die_off_ards_plan"`
	SystemicAzoleAndPolyeneRegimenPlan        string             `json:"systemic_azole_and_polyene_regimen_plan"`
	OphthalmicChorioretinitisNeuroPlan        string             `json:"ophthalmic_chorioretinitis_neuro_plan"`
	MinimumAzoleDurationMonths                int                `json:"minimum_azole_duration_months"`
	StagedBy                                  string             `json:"staged_by"`
	StagedAt                                  time.Time          `json:"staged_at"`
}

// MycosesAntigenRenalCheck records a serial Urine/Serum MiraVista Fungal Antigen EIA (ng/mL),
// renal nephrotoxicity (Serum Creatinine mg/dL), hepatic azole toxicity (ALT U/L), and oxygenation (SpO2 %) check.
type MycosesAntigenRenalCheck struct {
	ID                                string    `json:"id"`
	MiraVistaAntigenNgML              float64   `json:"miravista_antigen_ng_ml"`
	SerumCreatinineMgDL               float64   `json:"serum_creatinine_mg_dl"`
	ALTActivityUL                     float64   `json:"alt_activity_u_l"`
	SpO2Percent                       float64   `json:"spo2_percent"`
	CribriformPlateIntactOnCT         bool      `json:"cribriform_plate_intact_on_ct"`
	AmphotericinDoseHeldForAzotemia   bool      `json:"amphotericin_dose_held_for_azotemia"`
	OphthalmicChorioretinitisResolved bool      `json:"ophthalmic_chorioretinitis_resolved"`
	RecordedBy                        string    `json:"recorded_by"`
	ClinicalNote                      string    `json:"clinical_note"`
	RecordedAt                        time.Time `json:"recorded_at"`
}

// SystemicMycosesClearance captures antifungal cure clearance verification once MiraVista Urine/Serum
// Antigen EIA reaches 0.0 ng/mL across two consecutive negative tests 1 month apart, Serum Creatinine is <= 1.8 mg/dL,
// and a minimum 6-month systemic azole course (continuing >= 1 month past radiographic/antigen resolution) is verified.
type SystemicMycosesClearance struct {
	MiraVistaAntigenNgML              float64   `json:"miravista_antigen_ng_ml"`
	SerumCreatinineMgDL               float64   `json:"serum_creatinine_mg_dl"`
	TwoConsecutiveNegativeEIAVerified bool      `json:"two_consecutive_negative_eia_verified"`
	Minimum6MonthAzoleCourseVerified  bool      `json:"minimum_6month_azole_course_verified"`
	ClearedBy                         string    `json:"cleared_by"`
	ClearanceSummary                  string    `json:"clearance_summary"`
	ClearedAt                         time.Time `json:"cleared_at"`
}

// SystemicMycosesOperation represents an Acute Systemic Mycoses & Fungal Emergencies
// (Blastomycosis, Histoplasmosis, Coccidioidomycosis, Cryptococcosis & Sinonasal Aspergillosis),
// Urine/Serum MiraVista Antigen EIA, Itraconazole/Fluconazole, Amphotericin B Lipid Complex
// Nephrotoxicity Guard & Clotrimazole Trephination triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type SystemicMycosesOperation struct {
	ID                                        string                     `json:"id"`
	CaseID                                    string                     `json:"case_id"`
	PetName                                   string                     `json:"pet_name"`
	Species                                   string                     `json:"species"`
	Breed                                     string                     `json:"breed"`
	WeightKg                                  float64                    `json:"weight_kg"`
	Status                                    SystemicMycosesStatus      `json:"status"`
	Syndrome                                  FungalSyndrome             `json:"syndrome"`
	Protocol                                  AntifungalProtocol         `json:"protocol"`
	MiraVistaAntigenNgML                      float64                    `json:"miravista_antigen_ng_ml"`
	SerumCreatinineMgDL                       float64                    `json:"serum_creatinine_mg_dl"`
	ALTActivityUL                             float64                    `json:"alt_activity_u_l"`
	SpO2Percent                               float64                    `json:"spo2_percent"`
	CribriformPlateIntactOnCT                 bool                       `json:"cribriform_plate_intact_on_ct"`
	CompoundedBulkItraconazoleBlocked         bool                       `json:"compounded_bulk_itraconazole_blocked"`
	AmphotericinLipidSalineDiuresisVerified   bool                       `json:"amphotericin_lipid_saline_diuresis_verified"`
	DieOffDexamethasoneAntiInflammatoryStaged bool                       `json:"die_off_dexamethasone_anti_inflammatory_staged"`
	AmphotericinNephrotoxicityHoldAlert       bool                       `json:"amphotericin_nephrotoxicity_hold_alert"`
	PathophysiologySummary                    string                     `json:"pathophysiology_summary"`
	CytologyAntigenDiagnosticNote             string                     `json:"cytology_antigen_diagnostic_note"`
	RenalHepaticSafetyNote                    string                     `json:"renal_hepatic_safety_note"`
	TherapyStaging                            AntifungalTherapyStaging   `json:"therapy_staging"`
	AntigenRenalChecks                        []MycosesAntigenRenalCheck `json:"antigen_renal_checks"`
	Clearance                                 *SystemicMycosesClearance  `json:"clearance,omitempty"`
	DischargedBy                              string                     `json:"discharged_by,omitempty"`
	DischargeSummary                          string                     `json:"discharge_summary,omitempty"`
	DischargedAt                              *time.Time                 `json:"discharged_at,omitempty"`
	CreatedAt                                 time.Time                  `json:"created_at"`
	UpdatedAt                                 time.Time                  `json:"updated_at"`
}

// Validate checks domain invariants on SystemicMycosesOperation.
func (op SystemicMycosesOperation) Validate() error {
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
	if !op.Protocol.IsValid() {
		return fmt.Errorf("invalid protocol: %q", op.Protocol)
	}
	return nil
}

// SystemicMycosesOverview summarizes aggregate telemetry counts across Systemic Mycoses operations.
type SystemicMycosesOverview struct {
	TotalOperations                          int `json:"total_operations"`
	AzoleAmphotericinTrephinationStagedCount int `json:"azole_amphotericin_trephination_staged_count"`
	AntigenEIACreaOphthalmicMonitoredCount   int `json:"antigen_eia_crea_ophthalmic_monitored_count"`
	TwoNegativeAntigenEIAClearedCount        int `json:"two_negative_antigen_eia_cleared_count"`
	BlastomycosisHistoplasmosisCount         int `json:"blastomycosis_histoplasmosis_count"`
	CribriformBreachContraindicatedCount     int `json:"cribriform_breach_contraindicated_count"`
	AzotemiaNephrotoxicityAlertCount         int `json:"azotemia_nephrotoxicity_alert_count"`
}
