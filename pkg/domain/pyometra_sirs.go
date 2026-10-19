package domain

import (
	"fmt"
	"strings"
	"time"
)

// PyometraTriageStatus represents the clinical workflow state of an Acute Pyometra
// (Open vs. Closed Cervix), POCUS Fluid-Filled Uterine Horns, E. coli Endotoxemia/SIRS,
// Stabilization & Ovariohysterectomy (OHE) triage operation.
type PyometraTriageStatus string

const (
	PyometraStatusClosedCervixSIRSTriage PyometraTriageStatus = "CLOSED_CERVIX_ENDOTOXIC_SIRS_TRIAGE"
	PyometraStatusResuscitatedOHEStaged  PyometraTriageStatus = "RESUSCITATED_EMPIRIC_ABX_OHE_STAGED"
	PyometraStatusPostopOHECleared       PyometraTriageStatus = "POSTOP_OHE_NORMOTENSIVE_AFEBRILE_CLEARED"
)

// IsValid returns true when PyometraTriageStatus is a recognized lifecycle state.
func (s PyometraTriageStatus) IsValid() bool {
	switch s {
	case PyometraStatusClosedCervixSIRSTriage,
		PyometraStatusResuscitatedOHEStaged,
		PyometraStatusPostopOHECleared:
		return true
	default:
		return false
	}
}

// CervixPatencyType classifies anatomical cervical patency and uterine pathology.
type CervixPatencyType string

const (
	CervixPatencyClosedRuptureRisk     CervixPatencyType = "CLOSED_CERVIX_RUPTURE_RISK"
	CervixPatencyOpenPurulentDischarge CervixPatencyType = "OPEN_CERVIX_PURULENT_DISCHARGE"
	CervixPatencyStumpOvarianRemnant   CervixPatencyType = "STUMP_PYOMETRA_OVARIAN_REMNANT"
)

// IsValid returns true when CervixPatencyType is a recognized cervical classification.
func (c CervixPatencyType) IsValid() bool {
	switch c {
	case CervixPatencyClosedRuptureRisk,
		CervixPatencyOpenPurulentDischarge,
		CervixPatencyStumpOvarianRemnant:
		return true
	default:
		return false
	}
}

// EndotoxemiaSIRSPlan captures cervical patency & uterine rupture risk, POCUS/AFAST fluid-filled
// convoluted uterine horns, E. coli lipopolysaccharide (LPS) endotoxin renal medullary ADH
// antagonism (secondary nephrogenic diabetes insipidus causing PU/PD), strict contraindication of
// blind percutaneous cystocentesis and forceful abdominal palpation, prostaglandin F2-alpha /
// aglepristone luteolysis contraindication in closed-cervix or SIRS patients, and SIRS/lactate shock summary.
type EndotoxemiaSIRSPlan struct {
	CervixPatencySummary           string `json:"cervix_patency_summary"`
	POCUSUterineHornFinding        string `json:"pocus_uterine_horn_finding"`
	EColiADHResistancePolyuriaNote string `json:"e_coli_adh_resistance_polyuria_note"`
	CystocentesisContraindication  string `json:"cystocentesis_contraindication"`
	ProstaglandinLuteolysisWarning string `json:"prostaglandin_luteolysis_warning"`
	SIRSLactateShockSummary        string `json:"sirs_lactate_shock_summary"`
}

// StabilizationOHEPlan captures pre-operative buffered isotonic crystalloid shock resuscitation,
// empiric bactericidal IV antibiotic coverage (Enrofloxacin + Ampicillin-Sulbactam / Amoxicillin-Clavulanate),
// atraumatic surgical ovariohysterectomy (OHE) laparotomy sponge isolation, warm sterile saline
// abdominal lavage if uterine rupture/peritonitis is present, multimodal perioperative analgesia,
// and post-operative sepsis/DIC/normotension surveillance.
type StabilizationOHEPlan struct {
	CrystalloidResuscitationPlan string `json:"crystalloid_resuscitation_plan"`
	EmpiricBactericidalAbxPlan   string `json:"empiric_bactericidal_abx_plan"`
	SurgicalOHEHandlingProtocol  string `json:"surgical_ohe_handling_protocol"`
	AbdominalLavagePlan          string `json:"abdominal_lavage_plan"`
	PerioperativeAnalgesiaPlan   string `json:"perioperative_analgesia_plan"`
	PostopSepsisMonitoring       string `json:"postop_sepsis_monitoring"`
}

// PeriopHemodynamicCheck records a serial perioperative POCUS uterine horn diameter,
// blood lactate, total WBC count, band neutrophil percentage (degenerative left shift),
// and Mean Arterial Pressure (MAP) check.
type PeriopHemodynamicCheck struct {
	ID                    string    `json:"id"`
	UterineHornDiameterCm float64   `json:"uterine_horn_diameter_cm"`
	BloodLactateMmolL     float64   `json:"blood_lactate_mmol_l"`
	WBCCountKUl           float64   `json:"wbc_count_k_ul"`
	BandNeutrophilPercent float64   `json:"band_neutrophil_percent"`
	MAPMmHg               int       `json:"map_mm_hg"`
	RecordedBy            string    `json:"recorded_by"`
	ClinicalNote          string    `json:"clinical_note"`
	RecordedAt            time.Time `json:"recorded_at"`
}

// PyometraOperation represents an Acute Pyometra, E. coli Endotoxemia/SIRS & Ovariohysterectomy (OHE)
// triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type PyometraOperation struct {
	ID                        string                   `json:"id"`
	CaseID                    string                   `json:"case_id"`
	PetName                   string                   `json:"pet_name"`
	Species                   string                   `json:"species"`
	Breed                     string                   `json:"breed"`
	WeightKg                  float64                  `json:"weight_kg"`
	Status                    PyometraTriageStatus     `json:"status"`
	CervixType                CervixPatencyType        `json:"cervix_type"`
	UterineHornDiameterCm     float64                  `json:"uterine_horn_diameter_cm"`
	BloodLactateMmolL         float64                  `json:"blood_lactate_mmol_l"`
	WBCCountKUl               float64                  `json:"wbc_count_k_ul"`
	BandNeutrophilPercent     float64                  `json:"band_neutrophil_percent"`
	MAPMmHg                   int                      `json:"map_mm_hg"`
	POCUSFreePeritonealFluid  bool                     `json:"pocus_free_peritoneal_fluid"`
	PolyuriaPolydipsiaPresent bool                     `json:"polyuria_polydipsia_present"`
	SIRSPlan                  EndotoxemiaSIRSPlan      `json:"sirs_plan"`
	OHEPlan                   StabilizationOHEPlan     `json:"ohe_plan"`
	HemodynamicChecks         []PeriopHemodynamicCheck `json:"hemodynamic_checks"`
	DischargedBy              string                   `json:"discharged_by,omitempty"`
	DischargeSummary          string                   `json:"discharge_summary,omitempty"`
	DischargedAt              *time.Time               `json:"discharged_at,omitempty"`
	CreatedAt                 time.Time                `json:"created_at"`
	UpdatedAt                 time.Time                `json:"updated_at"`
}

// Validate checks domain invariants on PyometraOperation.
func (op PyometraOperation) Validate() error {
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
	if !op.CervixType.IsValid() {
		return fmt.Errorf("invalid cervix_type: %q", op.CervixType)
	}
	return nil
}

// PyometraOverviewMetrics summarizes aggregate counts across Pyometra / Endotoxic SIRS operations.
type PyometraOverviewMetrics struct {
	TotalOperations              int `json:"total_operations"`
	ClosedCervixSIRSTriageCount  int `json:"closed_cervix_sirs_triage_count"`
	OHEStagedCount               int `json:"ohe_staged_count"`
	ClearedPostopOHECount        int `json:"cleared_postop_ohe_count"`
	ClosedCervixRuptureRiskCount int `json:"closed_cervix_rupture_risk_count"`
	PeritonealEffusionAlertCount int `json:"peritoneal_effusion_alert_count"`
}
