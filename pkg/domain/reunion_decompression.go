package domain

import (
	"fmt"
	"strings"
	"time"
)

type DecompressionPhase string

const (
	DecompPhaseAcuteRefuge72H         DecompressionPhase = "ACUTE_REFUGE_72H"
	DecompPhaseCortisolTaperDay4To7   DecompressionPhase = "CORTISOL_TAPER_DAY_4_7"
	DecompPhaseRoutineReintegration14 DecompressionPhase = "ROUTINE_REINTEGRATION_DAY_8_14"
	DecompPhaseGraduatedStable        DecompressionPhase = "GRADUATED_STABLE"
)

func ValidDecompressionPhase(p DecompressionPhase) bool {
	switch p {
	case DecompPhaseAcuteRefuge72H,
		DecompPhaseCortisolTaperDay4To7,
		DecompPhaseRoutineReintegration14,
		DecompPhaseGraduatedStable:
		return true
	default:
		return false
	}
}

type RecidivismRiskLevel string

const (
	RecidivismRiskCriticalFlight    RecidivismRiskLevel = "CRITICAL_FLIGHT_RISK"
	RecidivismRiskElevatedPerimeter RecidivismRiskLevel = "ELEVATED_PERIMETER_RISK"
	RecidivismRiskFortifiedLow      RecidivismRiskLevel = "FORTIFIED_LOW_RISK"
)

func ValidRecidivismRiskLevel(r RecidivismRiskLevel) bool {
	switch r {
	case RecidivismRiskCriticalFlight,
		RecidivismRiskElevatedPerimeter,
		RecidivismRiskFortifiedLow:
		return true
	default:
		return false
	}
}

type DecompressionChecklistItem struct {
	ItemID    string `json:"itemId"`
	Category  string `json:"category"` // e.g. "REFEEDING_HYDRATION", "CORTISOL_QUIET_ZONE", "VET_PARASITE_SCREEN", "PERIMETER_FORTIFICATION"
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

type PerimeterAuditFinding struct {
	FindingID         string `json:"findingId"`
	ZoneLabel         string `json:"zoneLabel"`
	VulnerabilityType string `json:"vulnerabilityType"`
	RemediationAction string `json:"remediationAction"`
	Remediated        bool   `json:"remediated"`
}

type ReunionDecompressionPlan struct {
	PlanID                   string                       `json:"planId"`
	PetID                    string                       `json:"petId"`
	PetName                  string                       `json:"petName"`
	Species                  string                       `json:"species"`       // "DOG" or "CAT"
	DaysMissing              int                          `json:"daysMissing"`   // >= 1
	WeightLossPct            float64                      `json:"weightLossPct"` // 0..50
	EscapeVector             string                       `json:"escapeVector"`  // e.g. "UNLATCHED_SIDE_GATE", "DOOR_DASH_BOLT", "DIG_UNDER_FENCE", "FIREWORKS_PANIC_LEASH_SLIP"
	Phase                    DecompressionPhase           `json:"phase"`
	CortisolStressIndex      int                          `json:"cortisolStressIndex"`      // 0..100
	RefeedingSyndromeWarning bool                         `json:"refeedingSyndromeWarning"` // true when DaysMissing >= 5 or WeightLossPct >= 10.0
	RecidivismRiskScore      int                          `json:"recidivismRiskScore"`      // 0..100
	RecidivismRisk           RecidivismRiskLevel          `json:"recidivismRisk"`
	CollarReadinessScore     int                          `json:"collarReadinessScore"` // 0..100
	GpsCollarEquipped        bool                         `json:"gpsCollarEquipped"`
	DoubleLeashVerified      bool                         `json:"doubleLeashVerified"`
	ChecklistItems           []DecompressionChecklistItem `json:"checklistItems"`
	PerimeterFindings        []PerimeterAuditFinding      `json:"perimeterFindings"`
	ClinicalSummary          string                       `json:"clinicalSummary"`
	GraduationCertificateID  string                       `json:"graduationCertificateId,omitempty"`
	CreatedAt                time.Time                    `json:"createdAt"`
	UpdatedAt                time.Time                    `json:"updatedAt"`
}

// Validate checks invariants for ReunionDecompressionPlan.
func (p ReunionDecompressionPlan) Validate() error {
	if strings.TrimSpace(p.PlanID) == "" {
		return fmt.Errorf("planId is required")
	}
	if strings.TrimSpace(p.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(p.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	species := strings.TrimSpace(p.Species)
	if species != "DOG" && species != "CAT" {
		return fmt.Errorf("species must be DOG or CAT, got %q", p.Species)
	}
	if p.DaysMissing < 1 || p.DaysMissing > 3650 {
		return fmt.Errorf("daysMissing must be in [1, 3650], got %d", p.DaysMissing)
	}
	if p.WeightLossPct < 0.0 || p.WeightLossPct > 50.0 {
		return fmt.Errorf("weightLossPct must be in [0.0, 50.0], got %.2f", p.WeightLossPct)
	}
	if strings.TrimSpace(p.EscapeVector) == "" {
		return fmt.Errorf("escapeVector is required")
	}
	if !ValidDecompressionPhase(p.Phase) {
		return fmt.Errorf("invalid decompression phase: %q", p.Phase)
	}
	if p.CortisolStressIndex < 0 || p.CortisolStressIndex > 100 {
		return fmt.Errorf("cortisolStressIndex must be in [0, 100], got %d", p.CortisolStressIndex)
	}
	if p.RecidivismRiskScore < 0 || p.RecidivismRiskScore > 100 {
		return fmt.Errorf("recidivismRiskScore must be in [0, 100], got %d", p.RecidivismRiskScore)
	}
	if !ValidRecidivismRiskLevel(p.RecidivismRisk) {
		return fmt.Errorf("invalid recidivismRisk: %q", p.RecidivismRisk)
	}
	if p.CollarReadinessScore < 0 || p.CollarReadinessScore > 100 {
		return fmt.Errorf("collarReadinessScore must be in [0, 100], got %d", p.CollarReadinessScore)
	}
	if len(p.ChecklistItems) < 1 {
		return fmt.Errorf("at least one checklistItem is required")
	}
	for i, item := range p.ChecklistItems {
		if strings.TrimSpace(item.ItemID) == "" ||
			strings.TrimSpace(item.Category) == "" ||
			strings.TrimSpace(item.Title) == "" {
			return fmt.Errorf("checklistItems[%d] must have non-empty itemId, category, and title", i)
		}
	}
	if len(p.PerimeterFindings) < 1 {
		return fmt.Errorf("at least one perimeterFinding is required")
	}
	for i, finding := range p.PerimeterFindings {
		if strings.TrimSpace(finding.FindingID) == "" ||
			strings.TrimSpace(finding.ZoneLabel) == "" ||
			strings.TrimSpace(finding.VulnerabilityType) == "" ||
			strings.TrimSpace(finding.RemediationAction) == "" {
			return fmt.Errorf("perimeterFindings[%d] must have non-empty findingId, zoneLabel, vulnerabilityType, and remediationAction", i)
		}
	}
	if strings.TrimSpace(p.ClinicalSummary) == "" {
		return fmt.Errorf("clinicalSummary is required")
	}
	if p.Phase == DecompPhaseGraduatedStable && strings.TrimSpace(p.GraduationCertificateID) == "" {
		return fmt.Errorf("graduationCertificateId is required when phase is GRADUATED_STABLE")
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("createdAt is required")
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
