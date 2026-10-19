package domain

import (
	"fmt"
	"strings"
	"time"
)

// ThermalBurnTriageStatus represents the clinical workflow state of an Acute Smoke Inhalation,
// Thermal Burn Classification (Superficial/Partial/Full-Thickness TBSA%), Parkland Crystalloid
// Resuscitation Formula, Silver Sulfadiazine & Escharotomy triage operation.
type ThermalBurnTriageStatus string

const (
	ThermalBurnStatusAcuteTriage        ThermalBurnTriageStatus = "ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE"
	ThermalBurnStatusParklandStaged     ThermalBurnTriageStatus = "PARKLAND_FIRST_8H_ESCHAROTOMY_STAGED"
	ThermalBurnStatusGranulationCleared ThermalBurnTriageStatus = "GRANULATION_BED_NORMOALBUMINEMIC_CLEARED"
)

// IsValid returns true when ThermalBurnTriageStatus is a recognized lifecycle state.
func (s ThermalBurnTriageStatus) IsValid() bool {
	switch s {
	case ThermalBurnStatusAcuteTriage,
		ThermalBurnStatusParklandStaged,
		ThermalBurnStatusGranulationCleared:
		return true
	default:
		return false
	}
}

// BurnDepthClassification classifies the primary depth and anatomic distribution of a thermal burn injury.
type BurnDepthClassification string

const (
	BurnDepthFullThicknessCircumferentialEschar BurnDepthClassification = "FULL_THICKNESS_CIRCUMFERENTIAL_ESCHAR"
	BurnDepthDeepPartialThicknessDermal         BurnDepthClassification = "DEEP_PARTIAL_THICKNESS_DERMAL_BURN"
	BurnDepthThermalAirwayCornealFlash          BurnDepthClassification = "THERMAL_AIRWAY_CORNEAL_FLASH_BURN"
)

// IsValid returns true when BurnDepthClassification is a recognized classification.
func (c BurnDepthClassification) IsValid() bool {
	switch c {
	case BurnDepthFullThicknessCircumferentialEschar,
		BurnDepthDeepPartialThicknessDermal,
		BurnDepthThermalAirwayCornealFlash:
		return true
	default:
		return false
	}
}

// TBSAParklandAssessmentPlan captures the burn depth pathophysiology, 24h Parkland fluid calculation
// summary, first 8h vs. next 16h resuscitation split note, circumferential escharotomy alert,
// 24h colloid capillary leak contraindication warning, and systemic steroid/prophylactic antibiotic warning.
type TBSAParklandAssessmentPlan struct {
	BurnDepthPathophysiology              string `json:"burn_depth_pathophysiology"`
	Parkland24hCalculationSummary         string `json:"parkland_24h_calculation_summary"`
	First8hVsNext16hSplitNote             string `json:"first_8h_vs_next_16h_split_note"`
	CircumferentialEscharotomyAlert       string `json:"circumferential_escharotomy_alert"`
	Colloid24hCapillaryLeakWarning        string `json:"colloid_24h_capillary_leak_warning"`
	SystemicSteroidProphylacticAbxWarning string `json:"systemic_steroid_prophylactic_abx_warning"`
}

// ResuscitationWoundCarePlan captures the Parkland crystalloid schedule, Silver Sulfadiazine 1% /
// medical-grade Manuka honey wound protocol, escharotomy & staged debridement plan, multimodal burn
// analgesia CRI, albumin/oncotic >24h transition plan, and corneal/thermal airway protection.
type ResuscitationWoundCarePlan struct {
	ParklandCrystalloidSchedule      string `json:"parkland_crystalloid_schedule"`
	SilverSulfadiazineHoneyWoundPlan string `json:"silver_sulfadiazine_honey_wound_plan"`
	EscharotomyWetDebridementPlan    string `json:"escharotomy_wet_debridement_plan"`
	MultimodalBurnAnalgesiaCRI       string `json:"multimodal_burn_analgesia_cri"`
	AlbuminOncotic24hTransition      string `json:"albumin_oncotic_24h_transition"`
	CornealThermalAirwayProtection   string `json:"corneal_thermal_airway_protection"`
}

// BurnFluidWoundCheck records a serial TBSA%, fluid rate, urine output, serum albumin, and PCV check.
type BurnFluidWoundCheck struct {
	ID                   string    `json:"id"`
	TBSAPercent          float64   `json:"tbsa_percent"`
	CurrentFluidRateMLHr float64   `json:"current_fluid_rate_ml_hr"`
	UrineOutputMLKgHr    float64   `json:"urine_output_ml_kg_hr"`
	SerumAlbuminGDL      float64   `json:"serum_albumin_g_dl"`
	PCVPercent           float64   `json:"pcv_percent"`
	RecordedBy           string    `json:"recorded_by"`
	ClinicalNote         string    `json:"clinical_note"`
	RecordedAt           time.Time `json:"recorded_at"`
}

// ThermalBurnOperation represents an Acute Smoke Inhalation, Thermal Burn Classification,
// Parkland Crystalloid Resuscitation Formula, Silver Sulfadiazine & Escharotomy triage operation
// in the PetSpotR Veterinary Emergency ICU cockpit.
type ThermalBurnOperation struct {
	ID                            string                     `json:"id"`
	CaseID                        string                     `json:"case_id"`
	PetName                       string                     `json:"pet_name"`
	Species                       string                     `json:"species"`
	Breed                         string                     `json:"breed"`
	WeightKg                      float64                    `json:"weight_kg"`
	Status                        ThermalBurnTriageStatus    `json:"status"`
	DepthClass                    BurnDepthClassification    `json:"depth_class"`
	TBSAPercent                   float64                    `json:"tbsa_percent"`
	Total24hParklandML            float64                    `json:"total_24h_parkland_ml"`
	First8hRateMLHr               float64                    `json:"first_8h_rate_ml_hr"`
	Next16hRateMLHr               float64                    `json:"next_16h_rate_ml_hr"`
	CurrentFluidRateMLHr          float64                    `json:"current_fluid_rate_ml_hr"`
	UrineOutputMLKgHr             float64                    `json:"urine_output_ml_kg_hr"`
	SerumAlbuminGDL               float64                    `json:"serum_albumin_g_dl"`
	PCVPercent                    float64                    `json:"pcv_percent"`
	CircumferentialThoracicEschar bool                       `json:"circumferential_thoracic_eschar"`
	SingedVibrissaeSootAirway     bool                       `json:"singed_vibrissae_soot_airway"`
	ParklandPlan                  TBSAParklandAssessmentPlan `json:"parkland_plan"`
	WoundPlan                     ResuscitationWoundCarePlan `json:"wound_plan"`
	BurnChecks                    []BurnFluidWoundCheck      `json:"burn_checks"`
	DischargedBy                  string                     `json:"discharged_by,omitempty"`
	DischargeSummary              string                     `json:"discharge_summary,omitempty"`
	DischargedAt                  *time.Time                 `json:"discharged_at,omitempty"`
	CreatedAt                     time.Time                  `json:"created_at"`
	UpdatedAt                     time.Time                  `json:"updated_at"`
}

// Validate checks domain invariants on ThermalBurnOperation.
func (op ThermalBurnOperation) Validate() error {
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
	if !op.DepthClass.IsValid() {
		return fmt.Errorf("invalid depth_class: %q", op.DepthClass)
	}
	return nil
}

// ThermalBurnOverviewMetrics summarizes aggregate counts across Thermal Burn & Parkland ICU operations.
type ThermalBurnOverviewMetrics struct {
	TotalOperations            int `json:"total_operations"`
	AcuteBurnTriageCount       int `json:"acute_burn_triage_count"`
	ParklandStagedCount        int `json:"parkland_staged_count"`
	ClearedGranulationCount    int `json:"cleared_granulation_count"`
	SevereTBSAOver20PctCount   int `json:"severe_tbsa_over_20_pct_count"`
	CircumferentialEscharCount int `json:"circumferential_eschar_count"`
}
