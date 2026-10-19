package domain

import (
	"fmt"
	"strings"
	"time"
)

// RabiesQuarantineStatus represents the statutory & clinical workflow state of an Acute Rabies Quarantine,
// Lyssavirus Post-Exposure Prophylaxis (PEP), Strict 10-Day Domestic Bite Observation vs. 45-Day/4-Month
// Unvaccinated Quarantine, Biosecurity Double-Barrier Isolation & Public Health Rabies Control Officer (RCO)
// Statutory Reporting triage operation.
type RabiesQuarantineStatus string

const (
	RabiesStatusBiosecurityIsolationRCOHoldStaged    RabiesQuarantineStatus = "BIOSECURITY_ISOLATION_RCO_HOLD_STAGED"
	RabiesStatusSerialNeuroSalivarySheddingMonitored RabiesQuarantineStatus = "SERIAL_NEURO_SALIVARY_SHEDDING_MONITORED"
	RabiesStatusStatutoryQuarantineSurvivedCleared   RabiesQuarantineStatus = "STATUTORY_QUARANTINE_SURVIVED_CLEARED"
)

// IsValid returns true when RabiesQuarantineStatus is a recognized lifecycle state.
func (s RabiesQuarantineStatus) IsValid() bool {
	switch s {
	case RabiesStatusBiosecurityIsolationRCOHoldStaged,
		RabiesStatusSerialNeuroSalivarySheddingMonitored,
		RabiesStatusStatutoryQuarantineSurvivedCleared:
		return true
	default:
		return false
	}
}

// RabiesStatutoryCategory classifies the NASPHV Compendium on Animal Rabies Prevention and Control
// statutory confinement or quarantine category.
type RabiesStatutoryCategory string

const (
	RabiesCategoryTenDayDomesticBiteConfinement     RabiesStatutoryCategory = "TEN_DAY_DOMESTIC_BITE_CONFINEMENT"
	RabiesCategoryFortyFiveDayVaccinatedObservation RabiesStatutoryCategory = "FORTY_FIVE_DAY_VACCINATED_BOOSTER_OBSERVATION"
	RabiesCategoryFourMonthStrictDoubleBarrier      RabiesStatutoryCategory = "FOUR_MONTH_STRICT_DOUBLE_BARRIER_QUARANTINE"
	RabiesCategorySixMonthOverdueSerologyQuarantine RabiesStatutoryCategory = "SIX_MONTH_OVERDUE_SEROLOGY_QUARANTINE"
)

// IsValid returns true when RabiesStatutoryCategory is a recognized statutory category.
func (c RabiesStatutoryCategory) IsValid() bool {
	switch c {
	case RabiesCategoryTenDayDomesticBiteConfinement,
		RabiesCategoryFortyFiveDayVaccinatedObservation,
		RabiesCategoryFourMonthStrictDoubleBarrier,
		RabiesCategorySixMonthOverdueSerologyQuarantine:
		return true
	default:
		return false
	}
}

// RabiesBiosecurityProtocol classifies the biosecurity isolation, wound virucidal lavage,
// post-exposure booster, or prospective serologic RFFIT monitoring protocol.
type RabiesBiosecurityProtocol string

const (
	RabiesProtocolImmediateIMBooster96HrWoundLavage   RabiesBiosecurityProtocol = "IMMEDIATE_IM_BOOSTER_WITHIN_96HR_WOUND_LAVAGE"
	RabiesProtocolStrictTenDayNoVaccineDuringBiteHold RabiesBiosecurityProtocol = "STRICT_TEN_DAY_NO_VACCINE_DURING_BITE_HOLD"
	RabiesProtocolDoubleBarrierLockedCagePPEHandling  RabiesBiosecurityProtocol = "DOUBLE_BARRIER_LOCKED_CAGE_PPE_POLE_HANDLING"
	RabiesProtocolProspectiveSerologicRFFITBooster    RabiesBiosecurityProtocol = "PROSPECTIVE_SEROLOGIC_RFFIT_DAY0_DAY5_BOOSTER"
)

// IsValid returns true when RabiesBiosecurityProtocol is a recognized biosecurity/PEP protocol.
func (p RabiesBiosecurityProtocol) IsValid() bool {
	switch p {
	case RabiesProtocolImmediateIMBooster96HrWoundLavage,
		RabiesProtocolStrictTenDayNoVaccineDuringBiteHold,
		RabiesProtocolDoubleBarrierLockedCagePPEHandling,
		RabiesProtocolProspectiveSerologicRFFITBooster:
		return true
	default:
		return false
	}
}

// RabiesQuarantineStaging captures statutory biosecurity double-barrier isolation,
// NASPHV 10-day human-bite vaccine prohibition guard, 15-minute virucidal povidone-iodine/soap wound lavage,
// <= 96-hour IM rabies booster or Day 0 / Day 5 prospective RFFIT serologic paired-serum plan,
// Public Health Rabies Control Officer (RCO) statutory case filing, and DFA brainstem/hippocampus/cerebellum
// preservation protocol (refrigerate at 4°C, NEVER freeze with dry ice or damage skull).
type RabiesQuarantineStaging struct {
	Protocol                             RabiesBiosecurityProtocol `json:"protocol"`
	RCOCaseFilingNumber                  string                    `json:"rco_case_filing_number"`
	RequiredQuarantineDays               int                       `json:"required_quarantine_days"`
	NoVaccineDuring10DayBiteHoldVerified bool                      `json:"no_vaccine_during_10day_bite_hold_verified"`
	TenDayBiteHoldVaccineGuardNote       string                    `json:"ten_day_bite_hold_vaccine_guard_note"`
	DoubleBarrierLockedEnclosureVerified bool                      `json:"double_barrier_locked_enclosure_verified"`
	DoubleBarrierBiosecurityPPEPlan      string                    `json:"double_barrier_biosecurity_ppe_plan"`
	VirucidalWoundLavageBoosterPEPPlan   string                    `json:"virucidal_wound_lavage_booster_pep_plan"`
	ProspectiveSerologicRFFITPlan        string                    `json:"prospective_serologic_rffit_plan"`
	BrainStemHippocampusPreservedGuard   bool                      `json:"brain_stem_hippocampus_preserved_guard"`
	DFANeuropathologyPreservationNote    string                    `json:"dfa_neuropathology_preservation_note"`
	StagedBy                             string                    `json:"staged_by"`
	StagedAt                             time.Time                 `json:"staged_at"`
}

// RabiesNeuroObservationCheck records a serial contactless/double-barrier neurological & salivary
// shedding observation check (Quarantine Day Elapsed, RFFIT Titer IU/mL, Cranial Nerve / Bulbar Palsy Absent,
// Hypersalivation / Dysphagia Absent, Double-Barrier Locked Enclosure Verified).
type RabiesNeuroObservationCheck struct {
	ID                                   string    `json:"id"`
	QuarantineDayElapsed                 int       `json:"quarantine_day_elapsed"`
	RFFITTiterIUML                       float64   `json:"rffit_titer_iu_ml"`
	CranialNerveBulbarPalsyAbsent        bool      `json:"cranial_nerve_bulbar_palsy_absent"`
	HypersalivationDysphagiaAbsent       bool      `json:"hypersalivation_dysphagia_absent"`
	DoubleBarrierLockedEnclosureVerified bool      `json:"double_barrier_locked_enclosure_verified"`
	NoVaccineDuring10DayBiteHoldVerified bool      `json:"no_vaccine_during_10day_bite_hold_verified"`
	RecordedBy                           string    `json:"recorded_by"`
	ClinicalNote                         string    `json:"clinical_note"`
	RecordedAt                           time.Time `json:"recorded_at"`
}

// RabiesQuarantineClearance captures statutory quarantine survival and Public Health RCO release
// verification once QuarantineDayElapsed >= RequiredQuarantineDays, CranialNerveBulbarPalsyAbsent == true,
// RCOWrittenReleaseSigned == true, and ExitRabiesVaccinationVerified == true.
type RabiesQuarantineClearance struct {
	QuarantineDayElapsed          int       `json:"quarantine_day_elapsed"`
	RequiredQuarantineDays        int       `json:"required_quarantine_days"`
	CranialNerveBulbarPalsyAbsent bool      `json:"cranial_nerve_bulbar_palsy_absent"`
	RCOWrittenReleaseSigned       bool      `json:"rco_written_release_signed"`
	ExitRabiesVaccinationVerified bool      `json:"exit_rabies_vaccination_verified"`
	RFFITTiterIUML                float64   `json:"rffit_titer_iu_ml"`
	ClearedBy                     string    `json:"cleared_by"`
	ClearanceSummary              string    `json:"clearance_summary"`
	ClearedAt                     time.Time `json:"cleared_at"`
}

// RabiesQuarantineOperation represents an Acute Rabies Quarantine, Lyssavirus Post-Exposure Prophylaxis (PEP),
// Strict 10-Day Domestic Bite Observation vs. 45-Day/4-Month Unvaccinated Quarantine, Biosecurity Double-Barrier
// Isolation & Public Health Rabies Control Officer (RCO) Statutory Reporting triage operation.
type RabiesQuarantineOperation struct {
	ID                                   string                        `json:"id"`
	CaseID                               string                        `json:"case_id"`
	PetName                              string                        `json:"pet_name"`
	Species                              string                        `json:"species"`
	Breed                                string                        `json:"breed"`
	WeightKg                             float64                       `json:"weight_kg"`
	Status                               RabiesQuarantineStatus        `json:"status"`
	StatutoryCategory                    RabiesStatutoryCategory       `json:"statutory_category"`
	Protocol                             RabiesBiosecurityProtocol     `json:"protocol"`
	RCOCaseFilingNumber                  string                        `json:"rco_case_filing_number"`
	QuarantineDayElapsed                 int                           `json:"quarantine_day_elapsed"`
	RequiredQuarantineDays               int                           `json:"required_quarantine_days"`
	RFFITTiterIUML                       float64                       `json:"rffit_titer_iu_ml"`
	NoVaccineDuring10DayBiteHoldVerified bool                          `json:"no_vaccine_during_10day_bite_hold_verified"`
	DoubleBarrierLockedEnclosureVerified bool                          `json:"double_barrier_locked_enclosure_verified"`
	CranialNerveBulbarPalsyAbsent        bool                          `json:"cranial_nerve_bulbar_palsy_absent"`
	HypersalivationDysphagiaAbsent       bool                          `json:"hypersalivation_dysphagia_absent"`
	BrainStemHippocampusPreservedGuard   bool                          `json:"brain_stem_hippocampus_preserved_guard"`
	SeroprotectiveTiterVerified          bool                          `json:"seroprotective_titer_verified"`
	PathophysiologySummary               string                        `json:"pathophysiology_summary"`
	StatutoryQuarantineRCONote           string                        `json:"statutory_quarantine_rco_note"`
	NeuroSalivarySheddingNote            string                        `json:"neuro_salivary_shedding_note"`
	QuarantineStaging                    RabiesQuarantineStaging       `json:"quarantine_staging"`
	NeuroObservationChecks               []RabiesNeuroObservationCheck `json:"neuro_observation_checks"`
	Clearance                            *RabiesQuarantineClearance    `json:"clearance,omitempty"`
	DischargedBy                         string                        `json:"discharged_by,omitempty"`
	DischargeSummary                     string                        `json:"discharge_summary,omitempty"`
	DischargedAt                         *time.Time                    `json:"discharged_at,omitempty"`
	CreatedAt                            time.Time                     `json:"created_at"`
	UpdatedAt                            time.Time                     `json:"updated_at"`
}

// Validate checks domain invariants on RabiesQuarantineOperation.
func (op RabiesQuarantineOperation) Validate() error {
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
	if !op.StatutoryCategory.IsValid() {
		return fmt.Errorf("invalid statutory_category: %q", op.StatutoryCategory)
	}
	if !op.Protocol.IsValid() {
		return fmt.Errorf("invalid protocol: %q", op.Protocol)
	}
	if strings.TrimSpace(op.RCOCaseFilingNumber) == "" {
		return fmt.Errorf("rco_case_filing_number is required")
	}
	return nil
}

// RabiesQuarantineOverview summarizes aggregate telemetry counts across Rabies Quarantine operations.
type RabiesQuarantineOverview struct {
	TotalOperations                         int `json:"total_operations"`
	BiosecurityIsolationRCOHoldStagedCount  int `json:"biosecurity_isolation_rco_hold_staged_count"`
	SerialNeuroSalivaryMonitoredCount       int `json:"serial_neuro_salivary_monitored_count"`
	StatutoryQuarantineSurvivedClearedCount int `json:"statutory_quarantine_survived_cleared_count"`
	TenDayDomesticBiteHoldCount             int `json:"ten_day_domestic_bite_hold_count"`
	FourMonthStrictUnvaccinatedCount        int `json:"four_month_strict_unvaccinated_count"`
	ProtectiveRFFITTiterCount               int `json:"protective_rffit_titer_count"`
}
