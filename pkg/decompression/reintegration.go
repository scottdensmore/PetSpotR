package decompression

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreatePlanInput struct {
	PlanID              string
	PetID               string
	PetName             string
	Species             string
	DaysMissing         int
	WeightLossPct       float64
	EscapeVector        string
	GpsCollarEquipped   bool
	DoubleLeashVerified bool
}

type ProgressUpdateInput struct {
	CompleteAllChecklist  bool
	RemediateAllPerimeter bool
	CompleteItemID        string
	RemediateFindingID    string
	GpsCollarEquipped     *bool
	DoubleLeashVerified   *bool
	Notes                 string
}

func GenerateDecompressionPlan(input CreatePlanInput, now time.Time) (*domain.ReunionDecompressionPlan, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	planID := strings.TrimSpace(input.PlanID)
	petID := strings.TrimSpace(input.PetID)
	petName := strings.TrimSpace(input.PetName)
	if planID == "" {
		return nil, fmt.Errorf("planId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return nil, fmt.Errorf("petName is required")
	}
	if input.DaysMissing < 1 || input.DaysMissing > 3650 {
		return nil, fmt.Errorf("daysMissing must be in [1, 3650], got %d", input.DaysMissing)
	}
	if input.WeightLossPct < 0.0 || input.WeightLossPct > 50.0 {
		return nil, fmt.Errorf("weightLossPct must be in [0.0, 50.0], got %.2f", input.WeightLossPct)
	}

	species := strings.ToUpper(strings.TrimSpace(input.Species))
	if species == "" {
		species = "DOG"
	}

	escapeVector := strings.ToUpper(strings.TrimSpace(input.EscapeVector))
	if escapeVector == "" {
		escapeVector = "UNLATCHED_SIDE_GATE"
	}

	refeedingWarning := input.DaysMissing >= 5 || input.WeightLossPct >= 10.0

	refeedingTitle := "Gradual gastrointestinal hydration & bland recovery diet over 48 hours"
	if refeedingWarning {
		refeedingTitle = "CRITICAL REFEEDING PROTOCOL: 6 micro-meals/day of warm electrolyte slurry to prevent hypophosphatemia"
	}

	checklist := []domain.DecompressionChecklistItem{
		{
			ItemID:   "chk-quiet-refuge",
			Category: "CORTISOL_QUIET_ZONE",
			Title:    "Enforce 72-hour decompression sanctuary room (zero visitors, low-stimulation lighting, pheromone diffuser)",
		},
		{
			ItemID:   "chk-refeeding",
			Category: "REFEEDING_HYDRATION",
			Title:    refeedingTitle,
		},
		{
			ItemID:   "chk-vet-panel",
			Category: "VET_PARASITE_SCREEN",
			Title:    "Complete post-recovery veterinary tick/fecal panel, paw pad inspection & microchip scan verification",
		},
		{
			ItemID:   "chk-leash-protocol",
			Category: "PERIMETER_FORTIFICATION",
			Title:    "Fit escape-proof 3-strap martingale harness + dual-clip safety tether before any outdoor yard break",
		},
	}

	primaryFinding := domain.PerimeterAuditFinding{
		FindingID: "aud-primary-vector",
	}
	switch escapeVector {
	case "DOOR_DASH_BOLT":
		primaryFinding.ZoneLabel = "Front Entryway Threshold"
		primaryFinding.VulnerabilityType = "SINGLE_DOOR_BOLT_CORRIDOR"
		primaryFinding.RemediationAction = "Install interior tension-mounted airlock baby gate & boundary tether anchor"
	case "DIG_UNDER_FENCE":
		primaryFinding.ZoneLabel = "Backyard Cedar Fence Line"
		primaryFinding.VulnerabilityType = "SOIL_EROSION_UNDER_FENCE_GAP"
		primaryFinding.RemediationAction = "Anchor 16-gauge L-footer dig-defence galvanized mesh along fence base"
	case "FIREWORKS_PANIC_LEASH_SLIP":
		primaryFinding.ZoneLabel = "Walking Collar & Clasp Assembly"
		primaryFinding.VulnerabilityType = "SINGLE_BUCKLE_PANIC_SLIP"
		primaryFinding.RemediationAction = "Upgrade to 3-point escape-proof Ruffwear Web Master harness + waist safety backup"
	default:
		primaryFinding.ZoneLabel = "North Side Yard Gate"
		primaryFinding.VulnerabilityType = "GRAVITY_LATCH_UNSECURED"
		primaryFinding.RemediationAction = "Install self-closing hydraulic hinge + dual-sided carabiner padlock"
	}

	findings := []domain.PerimeterAuditFinding{
		primaryFinding,
		{
			FindingID:         "aud-secondary-perimeter",
			ZoneLabel:         "Rear Deck & Utility Gap",
			VulnerabilityType: "UNSCREENED_6_INCH_GAP",
			RemediationAction: "Install lattice barrier & verify ID tag + LTE GPS collar signal",
		},
	}

	plan := &domain.ReunionDecompressionPlan{
		PlanID:                   planID,
		PetID:                    petID,
		PetName:                  petName,
		Species:                  species,
		DaysMissing:              input.DaysMissing,
		WeightLossPct:            input.WeightLossPct,
		EscapeVector:             escapeVector,
		RefeedingSyndromeWarning: refeedingWarning,
		GpsCollarEquipped:        input.GpsCollarEquipped,
		DoubleLeashVerified:      input.DoubleLeashVerified,
		ChecklistItems:           checklist,
		PerimeterFindings:        findings,
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	RecomputePlanMetrics(plan)

	if err := plan.Validate(); err != nil {
		return nil, err
	}

	return plan, nil
}

func RecomputePlanMetrics(plan *domain.ReunionDecompressionPlan) {
	if plan == nil {
		return
	}

	plan.RefeedingSyndromeWarning = plan.DaysMissing >= 5 || plan.WeightLossPct >= 10.0

	totalChk := len(plan.ChecklistItems)
	completedChk := 0
	for _, item := range plan.ChecklistItems {
		if item.Completed {
			completedChk++
		}
	}

	totalAud := len(plan.PerimeterFindings)
	remediatedAud := 0
	for _, finding := range plan.PerimeterFindings {
		if finding.Remediated {
			remediatedAud++
		}
	}

	// Cortisol Stress Index (0..100)
	baseStress := 35 + plan.DaysMissing*5 + int(math.Round(plan.WeightLossPct*2.0))
	if baseStress > 95 {
		baseStress = 95
	}
	chkDenominator := totalChk
	if chkDenominator < 1 {
		chkDenominator = 1
	}
	chkReduction := (completedChk * 50) / chkDenominator
	gearReduction := 0
	if plan.GpsCollarEquipped {
		gearReduction += 10
	}
	if plan.DoubleLeashVerified {
		gearReduction += 10
	}
	cortisol := baseStress - chkReduction - gearReduction
	if cortisol < 10 {
		cortisol = 10
	}
	if cortisol > 100 {
		cortisol = 100
	}
	plan.CortisolStressIndex = cortisol

	// Collar Readiness Score (0..100)
	collarScore := 20
	if plan.GpsCollarEquipped {
		collarScore += 45
	}
	if plan.DoubleLeashVerified {
		collarScore += 35
	}
	if collarScore < 0 {
		collarScore = 0
	}
	if collarScore > 100 {
		collarScore = 100
	}
	plan.CollarReadinessScore = collarScore

	// Recidivism Risk Score (0..100)
	unremediatedPenalty := (totalAud - remediatedAud) * 28
	incompletePenalty := (totalChk - completedChk) * 8
	collarPenalty := 0
	if !plan.GpsCollarEquipped {
		collarPenalty = 18
	}
	leashPenalty := 0
	if !plan.DoubleLeashVerified {
		leashPenalty = 14
	}
	riskScore := 8 + unremediatedPenalty + incompletePenalty + collarPenalty + leashPenalty
	if riskScore < 5 {
		riskScore = 5
	}
	if riskScore > 100 {
		riskScore = 100
	}
	plan.RecidivismRiskScore = riskScore

	// Recidivism Risk Level
	if riskScore >= 65 {
		plan.RecidivismRisk = domain.RecidivismRiskCriticalFlight
	} else if riskScore >= 30 {
		plan.RecidivismRisk = domain.RecidivismRiskElevatedPerimeter
	} else {
		plan.RecidivismRisk = domain.RecidivismRiskFortifiedLow
	}

	// Decompression Phase
	if totalChk > 0 && completedChk == totalChk && totalAud > 0 && remediatedAud == totalAud && plan.GpsCollarEquipped && plan.DoubleLeashVerified {
		plan.Phase = domain.DecompPhaseGraduatedStable
		if plan.GraduationCertificateID == "" {
			plan.GraduationCertificateID = "CERT-DECOMP-" + strings.ToUpper(plan.PlanID)
		}
	} else if completedChk >= 3 && remediatedAud >= 1 {
		plan.Phase = domain.DecompPhaseRoutineReintegration14
	} else if completedChk >= 1 || remediatedAud >= 1 {
		plan.Phase = domain.DecompPhaseCortisolTaperDay4To7
	} else {
		plan.Phase = domain.DecompPhaseAcuteRefuge72H
	}

	refeedingLabel := "standard hydration & bland diet"
	if plan.RefeedingSyndromeWarning {
		refeedingLabel = "CRITICAL refeeding syndrome watch (electrolyte micro-meals active)"
	}

	plan.ClinicalSummary = fmt.Sprintf(
		"Phase %s | Cortisol Stress %d/100 | Nutrition: %s | Perimeter Audit %d/%d remediated | Recidivism Risk: %s (%d/100).",
		plan.Phase,
		plan.CortisolStressIndex,
		refeedingLabel,
		remediatedAud,
		totalAud,
		plan.RecidivismRisk,
		plan.RecidivismRiskScore,
	)
}

func ApplyDecompressionProgress(plan *domain.ReunionDecompressionPlan, input ProgressUpdateInput, now time.Time) error {
	if plan == nil {
		return fmt.Errorf("plan is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	if input.CompleteAllChecklist {
		for i := range plan.ChecklistItems {
			plan.ChecklistItems[i].Completed = true
		}
	}
	if itemID := strings.TrimSpace(input.CompleteItemID); itemID != "" {
		found := false
		for i := range plan.ChecklistItems {
			if plan.ChecklistItems[i].ItemID == itemID {
				plan.ChecklistItems[i].Completed = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("checklist item %q not found", itemID)
		}
	}

	if input.RemediateAllPerimeter {
		for i := range plan.PerimeterFindings {
			plan.PerimeterFindings[i].Remediated = true
		}
	}
	if findingID := strings.TrimSpace(input.RemediateFindingID); findingID != "" {
		found := false
		for i := range plan.PerimeterFindings {
			if plan.PerimeterFindings[i].FindingID == findingID {
				plan.PerimeterFindings[i].Remediated = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("perimeter finding %q not found", findingID)
		}
	}

	if input.GpsCollarEquipped != nil {
		plan.GpsCollarEquipped = *input.GpsCollarEquipped
	}
	if input.DoubleLeashVerified != nil {
		plan.DoubleLeashVerified = *input.DoubleLeashVerified
	}

	RecomputePlanMetrics(plan)

	if notes := strings.TrimSpace(input.Notes); notes != "" {
		plan.ClinicalSummary += " Note: " + notes
	}

	plan.UpdatedAt = now
	return nil
}
