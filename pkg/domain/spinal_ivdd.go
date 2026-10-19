package domain

import (
	"fmt"
	"strings"
	"time"
)

// SpinalIVDDStatus represents the lifecycle state of an Acute Spinal Cord Injury,
// Intervertebral Disc Extrusion (IVDD), Deep Pain Nociception, Schiff-Sherrington Posture
// & Hemilaminectomy triage operation.
type SpinalIVDDStatus string

const (
	SpinalStatusAcuteParaplegiaTriage SpinalIVDDStatus = "ACUTE_PARAPLEGIA_NOCICEPTION_TRIAGE"
	SpinalStatusHemilaminectomyStaged SpinalIVDDStatus = "CT_MYELOGRAM_HEMILAMINECTOMY_STAGED"
	SpinalStatusPostopBladderCleared  SpinalIVDDStatus = "POSTOP_NOCICEPTION_INTACT_BLADDER_CLEARED"
)

// IsValid returns true when SpinalIVDDStatus is a supported lifecycle state.
func (s SpinalIVDDStatus) IsValid() bool {
	switch s {
	case SpinalStatusAcuteParaplegiaTriage,
		SpinalStatusHemilaminectomyStaged,
		SpinalStatusPostopBladderCleared:
		return true
	default:
		return false
	}
}

// ModifiedFrankelGrade classifies thoracolumbar/lumbosacral spinal cord injury severity
// based on motor function and conscious deep pain nociception.
type ModifiedFrankelGrade string

const (
	FrankelGrade5DeepPainAbsent           ModifiedFrankelGrade = "GRADE_5_PARAPLEGIA_DEEP_PAIN_ABSENT"
	FrankelGrade4DeepPainIntact           ModifiedFrankelGrade = "GRADE_4_PARAPLEGIA_DEEP_PAIN_INTACT"
	FrankelGrade3NonAmbulatoryParaparesis ModifiedFrankelGrade = "GRADE_3_NON_AMBULATORY_PARAPARESIS"
	FrankelGrade2AmbulatoryParaparesis    ModifiedFrankelGrade = "GRADE_2_AMBULATORY_PARAPARESIS"
)

// IsValid returns true when ModifiedFrankelGrade is a recognized Frankel grade.
func (g ModifiedFrankelGrade) IsValid() bool {
	switch g {
	case FrankelGrade5DeepPainAbsent,
		FrankelGrade4DeepPainIntact,
		FrankelGrade3NonAmbulatoryParaparesis,
		FrankelGrade2AmbulatoryParaparesis:
		return true
	default:
		return false
	}
}

// NeurolocalizationZone identifies the neuroanatomical spinal cord segment and UMN/LMN phenotype.
type NeurolocalizationZone string

const (
	LocalizationT3L3ThoracolumbarUMN     NeurolocalizationZone = "T3_L3_THORACOLUMBAR_UMN"
	LocalizationL4S3LumbosacralLMN       NeurolocalizationZone = "L4_S3_LUMBOSACRAL_LMN"
	LocalizationC1C5CervicalTetraparesis NeurolocalizationZone = "C1_C5_CERVICAL_TETRAPARESIS"
)

// IsValid returns true when NeurolocalizationZone is a recognized spinal segment localization.
func (z NeurolocalizationZone) IsValid() bool {
	switch z {
	case LocalizationT3L3ThoracolumbarUMN,
		LocalizationL4S3LumbosacralLMN,
		LocalizationC1C5CervicalTetraparesis:
		return true
	default:
		return false
	}
}

// NociceptionAssessmentPlan captures conscious deep pain perception vs local spinal withdrawal reflex,
// Schiff-Sherrington border-cell disinhibition posture, cutaneous trunci (panniculus) cutoff level,
// ascending/descending myelomalacia risk, and steroid + NSAID washout contraindication alerts.
type NociceptionAssessmentPlan struct {
	DeepPainPerception       string `json:"deep_pain_perception"`
	WithdrawalReflexNote     string `json:"withdrawal_reflex_note"`
	SchiffSherringtonPosture string `json:"schiff_sherrington_posture"`
	CutaneousTrunciCutoff    string `json:"cutaneous_trunci_cutoff"`
	MyelomalaciaRiskWarning  string `json:"myelomalacia_risk_warning"`
	SteroidNSAIDWashoutAlert string `json:"steroid_nsaid_washout_alert"`
}

// DecompressionBladderPlan captures surgical decompression window (<12-24h for Grade 5),
// advanced imaging modality (3D CT / High-Field MRI), hemilaminectomy/ventral slot target site,
// UMN vs LMN neurogenic bladder management, multi-modal CRI analgesia, and strict crate rest.
type DecompressionBladderPlan struct {
	SurgicalWindowTarget      string `json:"surgical_window_target"`
	ImagingModality           string `json:"imaging_modality"`
	HemilaminectomySite       string `json:"hemilaminectomy_site"`
	BladderManagementProtocol string `json:"bladder_management_protocol"`
	AnalgesiaCRIProtocol      string `json:"analgesia_cri_protocol"`
	StrictCrateRestProtocol   string `json:"strict_crate_rest_protocol"`
}

// NeuroNociceptionCheck records a serial neurological nociception, cutaneous trunci cutoff,
// and bladder tone evaluation.
type NeuroNociceptionCheck struct {
	ID                   string               `json:"id"`
	FrankelGrade         ModifiedFrankelGrade `json:"frankel_grade"`
	DeepPainStatus       string               `json:"deep_pain_status"`
	CutaneousTrunciLevel string               `json:"cutaneous_trunci_level"`
	BladderToneStatus    string               `json:"bladder_tone_status"`
	RecordedBy           string               `json:"recorded_by"`
	ClinicalNote         string               `json:"clinical_note"`
	RecordedAt           time.Time            `json:"recorded_at"`
}

// Validate checks domain invariants on NeuroNociceptionCheck.
func (c NeuroNociceptionCheck) Validate() error {
	if !c.FrankelGrade.IsValid() {
		return fmt.Errorf("invalid frankel_grade: %q", c.FrankelGrade)
	}
	if strings.TrimSpace(c.DeepPainStatus) == "" {
		return fmt.Errorf("deep_pain_status is required")
	}
	if strings.TrimSpace(c.CutaneousTrunciLevel) == "" {
		return fmt.Errorf("cutaneous_trunci_level is required")
	}
	if strings.TrimSpace(c.BladderToneStatus) == "" {
		return fmt.Errorf("bladder_tone_status is required")
	}
	if strings.TrimSpace(c.RecordedBy) == "" {
		return fmt.Errorf("recorded_by is required")
	}
	return nil
}

// SpinalIVDDOperation represents an Acute Spinal Cord Injury, Intervertebral Disc Extrusion (IVDD),
// Deep Pain Nociception, Schiff-Sherrington Posture & Hemilaminectomy triage operation.
type SpinalIVDDOperation struct {
	ID                       string                    `json:"id"`
	CaseID                   string                    `json:"case_id"`
	PetName                  string                    `json:"pet_name"`
	Species                  string                    `json:"species"`
	Breed                    string                    `json:"breed"`
	WeightKg                 float64                   `json:"weight_kg"`
	Status                   SpinalIVDDStatus          `json:"status"`
	FrankelGrade             ModifiedFrankelGrade      `json:"frankel_grade"`
	LocalizationZone         NeurolocalizationZone     `json:"localization_zone"`
	OnsetHours               float64                   `json:"onset_hours"`
	CutaneousTrunciVertebra  string                    `json:"cutaneous_trunci_vertebra"`
	DeepPainPresent          bool                      `json:"deep_pain_present"`
	SchiffSherringtonPresent bool                      `json:"schiff_sherrington_present"`
	PriorSteroidOrNSAID      string                    `json:"prior_steroid_or_nsaid"`
	SurgicalWindowHours      float64                   `json:"surgical_window_hours"`
	TargetMAPMmHg            int                       `json:"target_map_mmhg"`
	NociceptionPlan          NociceptionAssessmentPlan `json:"nociception_plan"`
	DecompressionPlan        DecompressionBladderPlan  `json:"decompression_plan"`
	NeuroChecks              []NeuroNociceptionCheck   `json:"neuro_checks"`
	DischargedBy             string                    `json:"discharged_by,omitempty"`
	DischargeSummary         string                    `json:"discharge_summary,omitempty"`
	DischargedAt             *time.Time                `json:"discharged_at,omitempty"`
	CreatedAt                time.Time                 `json:"created_at"`
	UpdatedAt                time.Time                 `json:"updated_at"`
}

// Validate checks domain invariants on SpinalIVDDOperation.
func (op SpinalIVDDOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.CaseID) == "" {
		return fmt.Errorf("case_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if strings.TrimSpace(op.Species) == "" {
		return fmt.Errorf("species is required")
	}
	if op.WeightKg <= 0 || op.WeightKg > 120.0 {
		return fmt.Errorf("weight_kg must be in (0, 120.0], got %v", op.WeightKg)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if !op.FrankelGrade.IsValid() {
		return fmt.Errorf("invalid frankel_grade: %q", op.FrankelGrade)
	}
	if !op.LocalizationZone.IsValid() {
		return fmt.Errorf("invalid localization_zone: %q", op.LocalizationZone)
	}
	if op.OnsetHours < 0 || op.OnsetHours > 720 {
		return fmt.Errorf("onset_hours must be in [0, 720], got %v", op.OnsetHours)
	}
	if op.TargetMAPMmHg < 50 || op.TargetMAPMmHg > 180 {
		return fmt.Errorf("target_map_mmhg must be in [50, 180], got %d", op.TargetMAPMmHg)
	}
	return nil
}

// SpinalIVDDOverviewMetrics summarizes operational KPI counts across Acute Spinal Cord Injury & IVDD operations.
type SpinalIVDDOverviewMetrics struct {
	TotalOperations              int `json:"total_operations"`
	AcuteParaplegiaTriageCount   int `json:"acute_paraplegia_triage_count"`
	HemilaminectomyStagedCount   int `json:"hemilaminectomy_staged_count"`
	ClearedPostopCount           int `json:"cleared_postop_count"`
	DeepPainAbsentEmergencyCount int `json:"deep_pain_absent_emergency_count"`
	SchiffSherringtonCount       int `json:"schiff_sherrington_count"`
}
