package domain

import (
	"fmt"
	"strings"
	"time"
)

// NCPEElectrocutionStatus represents the clinical workflow state of an Acute Non-Cardiogenic
// Pulmonary Edema (NCPE), Electrocution (Oral Cord Bite), Upper Airway Obstruction /
// Post-Ictal Neurogenic Edema, Low-Dose Furosemide Controversy & CPAP/PEEP Mechanical
// Ventilation triage operation.
type NCPEElectrocutionStatus string

const (
	NCPEStatusAcuteCaudodorsalTriage NCPEElectrocutionStatus = "ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE"
	NCPEStatusOxygenCPAPStaged       NCPEElectrocutionStatus = "OXYGEN_BUTORPHANOL_CPAP_PEEP_STAGED"
	NCPEStatusBLinesResolvedCleared  NCPEElectrocutionStatus = "BLINES_RESOLVED_ROOM_AIR_NORMOCAPNIC_CLEARED"
)

// IsValid returns true when NCPEElectrocutionStatus is a recognized lifecycle state.
func (s NCPEElectrocutionStatus) IsValid() bool {
	switch s {
	case NCPEStatusAcuteCaudodorsalTriage,
		NCPEStatusOxygenCPAPStaged,
		NCPEStatusBLinesResolvedCleared:
		return true
	default:
		return false
	}
}

// NCPEEtiologyClassification classifies the primary pathophysiological etiology of
// Acute Non-Cardiogenic Pulmonary Edema (NCPE).
type NCPEEtiologyClassification string

const (
	NCPEEtiologyOralElectricalCordBite      NCPEEtiologyClassification = "ORAL_ELECTRICAL_CORD_BITE_NEUROGENIC"
	NCPEEtiologyPostObstructiveChoking      NCPEEtiologyClassification = "POST_OBSTRUCTIVE_CHOKING_STRANGULATION_NCPE"
	NCPEEtiologyPostIctalCatecholamineSurge NCPEEtiologyClassification = "POST_ICTAL_SEIZURE_TBI_CATECHOLAMINE_SURGE"
)

// IsValid returns true when NCPEEtiologyClassification is a recognized NCPE etiology.
func (e NCPEEtiologyClassification) IsValid() bool {
	switch e {
	case NCPEEtiologyOralElectricalCordBite,
		NCPEEtiologyPostObstructiveChoking,
		NCPEEtiologyPostIctalCatecholamineSurge:
		return true
	default:
		return false
	}
}

// CaudodorsalNCPEAssessmentPlan captures the NCPE etiology pathophysiology, caudodorsal vs.
// perihilar TFAST/VetBLUE B-lines POCUS distinction, normal LA:Ao (< 1.5) cardiogenic rule-out,
// strict contraindication of high-dose furosemide diuresis (4-8 mg/kg), oral electrical burn
// delayed eschar necrosis (5-7 days) & oronasal fistula risk, and capillary leak fluid overload warning.
type CaudodorsalNCPEAssessmentPlan struct {
	EtiologyPathophysiology            string `json:"etiology_pathophysiology"`
	CaudodorsalVsPerihilarPOCUSNote    string `json:"caudodorsal_vs_perihilar_pocus_note"`
	NormalLA2AoCardiogenicRuleOut      string `json:"normal_la2ao_cardiogenic_rule_out"`
	HighDoseFurosemideContraindication string `json:"high_dose_furosemide_contraindication"`
	OralElectricalBurnDelayedNecrosis  string `json:"oral_electrical_burn_delayed_necrosis"`
	FluidOverloadCapillaryLeakWarning  string `json:"fluid_overload_capillary_leak_warning"`
}

// OxygenCPAPVentilationPlan captures oxygen cage / High-Flow Nasal Cannula (HFNC) protocol,
// Butorphanol / Acepromazine anxiolysis to break negative-pressure Müller maneuver, low-dose
// furosemide bronchodilator/venodilator controversy note, Helmet CPAP or PEEP mechanical
// ventilation protocol, conservative IV fluid maintenance plan, and oral commissure burn analgesia plan.
type OxygenCPAPVentilationPlan struct {
	OxygenFiO2OrHFNCProtocol            string `json:"oxygen_fio2_or_hfnc_protocol"`
	AnxiolyticButorphanolAcepromazine   string `json:"anxiolytic_butorphanol_acepromazine"`
	LowDoseFurosemideBronchodilatorNote string `json:"low_dose_furosemide_bronchodilator_note"`
	CPAPOrPEEPMechanicalVentilation     string `json:"cpap_or_peep_mechanical_ventilation"`
	ConservativeFluidMaintenancePlan    string `json:"conservative_fluid_maintenance_plan"`
	OralCommissureBurnAnalgesiaPlan     string `json:"oral_commissure_burn_analgesia_plan"`
}

// NCPERespiratoryCheck records a serial oxygenation, VetBLUE POCUS B-line, and echocardiographic LA:Ao check.
type NCPERespiratoryCheck struct {
	ID                     string    `json:"id"`
	PaO2FiO2Ratio          float64   `json:"pao2_fio2_ratio"`
	SpO2Percent            float64   `json:"spo2_percent"`
	RespiratoryRateBpm     int       `json:"respiratory_rate_bpm"`
	CaudodorsalBLinesCount int       `json:"caudodorsal_blines_count"`
	LA2AoRatio             float64   `json:"la2ao_ratio"`
	RecordedBy             string    `json:"recorded_by"`
	ClinicalNote           string    `json:"clinical_note"`
	RecordedAt             time.Time `json:"recorded_at"`
}

// NCPEElectrocutionOperation represents an Acute Non-Cardiogenic Pulmonary Edema (NCPE) &
// Electrocution (Oral Cord Bite) triage operation in the PetSpotR Veterinary Emergency ICU cockpit.
type NCPEElectrocutionOperation struct {
	ID                          string                        `json:"id"`
	CaseID                      string                        `json:"case_id"`
	PetName                     string                        `json:"pet_name"`
	Species                     string                        `json:"species"`
	Breed                       string                        `json:"breed"`
	WeightKg                    float64                       `json:"weight_kg"`
	Status                      NCPEElectrocutionStatus       `json:"status"`
	EtiologyClass               NCPEEtiologyClassification    `json:"etiology_class"`
	PaO2FiO2Ratio               float64                       `json:"pao2_fio2_ratio"`
	SpO2Percent                 float64                       `json:"spo2_percent"`
	RespiratoryRateBpm          int                           `json:"respiratory_rate_bpm"`
	CaudodorsalBLinesCount      int                           `json:"caudodorsal_blines_count"`
	LA2AoRatio                  float64                       `json:"la2ao_ratio"`
	OralElectricalEscharPresent bool                          `json:"oral_electrical_eschar_present"`
	PinkFrothyAirwayFluid       bool                          `json:"pink_frothy_airway_fluid"`
	AssessmentPlan              CaudodorsalNCPEAssessmentPlan `json:"assessment_plan"`
	VentilationPlan             OxygenCPAPVentilationPlan     `json:"ventilation_plan"`
	RespiratoryChecks           []NCPERespiratoryCheck        `json:"respiratory_checks"`
	DischargedBy                string                        `json:"discharged_by,omitempty"`
	DischargeSummary            string                        `json:"discharge_summary,omitempty"`
	DischargedAt                *time.Time                    `json:"discharged_at,omitempty"`
	CreatedAt                   time.Time                     `json:"created_at"`
	UpdatedAt                   time.Time                     `json:"updated_at"`
}

// Validate checks domain invariants on NCPEElectrocutionOperation.
func (op NCPEElectrocutionOperation) Validate() error {
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
	if !op.EtiologyClass.IsValid() {
		return fmt.Errorf("invalid etiology_class: %q", op.EtiologyClass)
	}
	return nil
}

// NCPEOverviewMetrics summarizes aggregate counts across NCPE & Electrocution operations.
type NCPEOverviewMetrics struct {
	TotalOperations             int `json:"total_operations"`
	AcuteNCPETriageCount        int `json:"acute_ncpe_triage_count"`
	CPAPOxygenStagedCount       int `json:"cpap_oxygen_staged_count"`
	ClearedRoomAirCount         int `json:"cleared_room_air_count"`
	OralCordElectrocutionCount  int `json:"oral_cord_electrocution_count"`
	SeverePaO2FiO2Under200Count int `json:"severe_pao2_fio2_under_200_count"`
}
