package domain

import (
	"testing"
	"time"
)

func validReunionDecompressionPlan() ReunionDecompressionPlan {
	now := time.Date(2026, 10, 17, 14, 0, 0, 0, time.UTC)
	return ReunionDecompressionPlan{
		PlanID:                   "rdp-001",
		PetID:                    "pet-luna-01",
		PetName:                  "Luna",
		Species:                  "DOG",
		DaysMissing:              8,
		WeightLossPct:            12.5,
		EscapeVector:             "UNLATCHED_SIDE_GATE",
		Phase:                    DecompPhaseAcuteRefuge72H,
		CortisolStressIndex:      84,
		RefeedingSyndromeWarning: true,
		RecidivismRiskScore:      88,
		RecidivismRisk:           RecidivismRiskCriticalFlight,
		CollarReadinessScore:     20,
		GpsCollarEquipped:        false,
		DoubleLeashVerified:      false,
		ChecklistItems: []DecompressionChecklistItem{
			{
				ItemID:    "chk-refeeding-01",
				Category:  "REFEEDING_HYDRATION",
				Title:     "Small bland meals every 4-6 hours to prevent refeeding syndrome",
				Completed: false,
			},
			{
				ItemID:    "chk-cortisol-02",
				Category:  "CORTISOL_QUIET_ZONE",
				Title:     "Enforce 72-hour quiet refuge room with zero visitor greetings",
				Completed: false,
			},
		},
		PerimeterFindings: []PerimeterAuditFinding{
			{
				FindingID:         "paf-gate-01",
				ZoneLabel:         "East Side Gate",
				VulnerabilityType: "SINGLE_GRAVITY_LATCH",
				RemediationAction: "Install self-closing spring hinge + secondary carabiner pin",
				Remediated:        false,
			},
		},
		ClinicalSummary: "Elevated cortisol decompression protocol with refeeding syndrome monitoring.",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestReunionDecompressionValidate(t *testing.T) {
	t.Run("valid acute refuge plan", func(t *testing.T) {
		p := validReunionDecompressionPlan()
		if err := p.Validate(); err != nil {
			t.Fatalf("expected valid plan, got error: %v", err)
		}
	})

	t.Run("valid graduated stable plan with certificate", func(t *testing.T) {
		p := validReunionDecompressionPlan()
		p.Phase = DecompPhaseGraduatedStable
		p.CortisolStressIndex = 12
		p.RecidivismRiskScore = 8
		p.RecidivismRisk = RecidivismRiskFortifiedLow
		p.CollarReadinessScore = 100
		p.GpsCollarEquipped = true
		p.DoubleLeashVerified = true
		p.GraduationCertificateID = "CERT-DECOMP-LUNA-2026"
		if err := p.Validate(); err != nil {
			t.Fatalf("expected valid graduated plan, got error: %v", err)
		}
	})

	t.Run("rejects empty required fields", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*ReunionDecompressionPlan)
		}{
			{"empty planId", func(p *ReunionDecompressionPlan) { p.PlanID = " " }},
			{"empty petId", func(p *ReunionDecompressionPlan) { p.PetID = "" }},
			{"empty petName", func(p *ReunionDecompressionPlan) { p.PetName = "  " }},
			{"invalid species", func(p *ReunionDecompressionPlan) { p.Species = "turtle" }},
			{"zero daysMissing", func(p *ReunionDecompressionPlan) { p.DaysMissing = 0 }},
			{"excessive daysMissing", func(p *ReunionDecompressionPlan) { p.DaysMissing = 4000 }},
			{"negative weightLossPct", func(p *ReunionDecompressionPlan) { p.WeightLossPct = -1.0 }},
			{"excessive weightLossPct", func(p *ReunionDecompressionPlan) { p.WeightLossPct = 55.0 }},
			{"empty escapeVector", func(p *ReunionDecompressionPlan) { p.EscapeVector = "" }},
			{"invalid phase", func(p *ReunionDecompressionPlan) { p.Phase = DecompressionPhase("UNKNOWN_PHASE") }},
			{"negative cortisolStressIndex", func(p *ReunionDecompressionPlan) { p.CortisolStressIndex = -5 }},
			{"excessive cortisolStressIndex", func(p *ReunionDecompressionPlan) { p.CortisolStressIndex = 105 }},
			{"negative recidivismRiskScore", func(p *ReunionDecompressionPlan) { p.RecidivismRiskScore = -1 }},
			{"excessive recidivismRiskScore", func(p *ReunionDecompressionPlan) { p.RecidivismRiskScore = 101 }},
			{"invalid recidivismRisk", func(p *ReunionDecompressionPlan) { p.RecidivismRisk = RecidivismRiskLevel("NONE") }},
			{"negative collarReadinessScore", func(p *ReunionDecompressionPlan) { p.CollarReadinessScore = -1 }},
			{"excessive collarReadinessScore", func(p *ReunionDecompressionPlan) { p.CollarReadinessScore = 101 }},
			{"empty checklistItems", func(p *ReunionDecompressionPlan) { p.ChecklistItems = nil }},
			{"checklist item missing fields", func(p *ReunionDecompressionPlan) {
				p.ChecklistItems[0].Title = " "
			}},
			{"empty perimeterFindings", func(p *ReunionDecompressionPlan) { p.PerimeterFindings = nil }},
			{"perimeter finding missing fields", func(p *ReunionDecompressionPlan) {
				p.PerimeterFindings[0].RemediationAction = ""
			}},
			{"empty clinicalSummary", func(p *ReunionDecompressionPlan) { p.ClinicalSummary = "" }},
			{"graduated without certificate", func(p *ReunionDecompressionPlan) {
				p.Phase = DecompPhaseGraduatedStable
				p.GraduationCertificateID = ""
			}},
			{"zero createdAt", func(p *ReunionDecompressionPlan) { p.CreatedAt = time.Time{} }},
			{"zero updatedAt", func(p *ReunionDecompressionPlan) { p.UpdatedAt = time.Time{} }},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				p := validReunionDecompressionPlan()
				tc.mutate(&p)
				if err := p.Validate(); err == nil {
					t.Fatalf("expected validation error for %s, got nil", tc.name)
				}
			})
		}
	})
}
