package decompression

import (
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestGenerateDecompressionPlan_ValidationAndDefaults(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.FixedZone("PDT", -7*3600))

	tests := []struct {
		name    string
		input   CreatePlanInput
		wantErr bool
	}{
		{
			name: "missing planId",
			input: CreatePlanInput{
				PetID:       "pet-1",
				PetName:     "Luna",
				DaysMissing: 2,
			},
			wantErr: true,
		},
		{
			name: "missing petId",
			input: CreatePlanInput{
				PlanID:      "plan-1",
				PetName:     "Luna",
				DaysMissing: 2,
			},
			wantErr: true,
		},
		{
			name: "missing petName",
			input: CreatePlanInput{
				PlanID:      "plan-1",
				PetID:       "pet-1",
				DaysMissing: 2,
			},
			wantErr: true,
		},
		{
			name: "daysMissing < 1",
			input: CreatePlanInput{
				PlanID:      "plan-1",
				PetID:       "pet-1",
				PetName:     "Luna",
				DaysMissing: 0,
			},
			wantErr: true,
		},
		{
			name: "weightLossPct negative",
			input: CreatePlanInput{
				PlanID:        "plan-1",
				PetID:         "pet-1",
				PetName:       "Luna",
				DaysMissing:   2,
				WeightLossPct: -1.0,
			},
			wantErr: true,
		},
		{
			name: "weightLossPct > 50",
			input: CreatePlanInput{
				PlanID:        "plan-1",
				PetID:         "pet-1",
				PetName:       "Luna",
				DaysMissing:   2,
				WeightLossPct: 51.0,
			},
			wantErr: true,
		},
		{
			name: "valid defaults when species and escapeVector omitted",
			input: CreatePlanInput{
				PlanID:        "plan-default",
				PetID:         "pet-1",
				PetName:       "Buster",
				DaysMissing:   2,
				WeightLossPct: 3.0,
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := GenerateDecompressionPlan(tc.input, now)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if plan.Species != "DOG" {
				t.Errorf("expected default species DOG, got %q", plan.Species)
			}
			if plan.EscapeVector != "UNLATCHED_SIDE_GATE" {
				t.Errorf("expected default escapeVector UNLATCHED_SIDE_GATE, got %q", plan.EscapeVector)
			}
			if plan.CreatedAt.Location() != time.UTC || plan.UpdatedAt.Location() != time.UTC {
				t.Errorf("expected CreatedAt and UpdatedAt in UTC")
			}
			if plan.RefeedingSyndromeWarning {
				t.Errorf("expected RefeedingSyndromeWarning=false for 2 days missing and 3%% weight loss")
			}
			if len(plan.ChecklistItems) != 4 {
				t.Fatalf("expected 4 checklist items, got %d", len(plan.ChecklistItems))
			}
			if len(plan.PerimeterFindings) != 2 {
				t.Fatalf("expected 2 perimeter findings, got %d", len(plan.PerimeterFindings))
			}
		})
	}
}

func TestGenerateDecompressionPlan_RefeedingAndEscapeVectors(t *testing.T) {
	now := time.Now().UTC()

	vectors := []struct {
		vector       string
		wantZone     string
		wantVulnType string
	}{
		{
			vector:       "DOOR_DASH_BOLT",
			wantZone:     "Front Entryway Threshold",
			wantVulnType: "SINGLE_DOOR_BOLT_CORRIDOR",
		},
		{
			vector:       "DIG_UNDER_FENCE",
			wantZone:     "Backyard Cedar Fence Line",
			wantVulnType: "SOIL_EROSION_UNDER_FENCE_GAP",
		},
		{
			vector:       "FIREWORKS_PANIC_LEASH_SLIP",
			wantZone:     "Walking Collar & Clasp Assembly",
			wantVulnType: "SINGLE_BUCKLE_PANIC_SLIP",
		},
		{
			vector:       "UNLATCHED_SIDE_GATE",
			wantZone:     "North Side Yard Gate",
			wantVulnType: "GRAVITY_LATCH_UNSECURED",
		},
	}

	for _, vc := range vectors {
		plan, err := GenerateDecompressionPlan(CreatePlanInput{
			PlanID:        "decomp-plan-101",
			PetID:         "pet-2026-002",
			PetName:       "Luna",
			Species:       "CAT",
			DaysMissing:   9,
			WeightLossPct: 14.5,
			EscapeVector:  vc.vector,
		}, now)
		if err != nil {
			t.Fatalf("vector %s: unexpected error: %v", vc.vector, err)
		}
		if !plan.RefeedingSyndromeWarning {
			t.Errorf("vector %s: expected RefeedingSyndromeWarning=true", vc.vector)
		}
		if !strings.Contains(plan.ChecklistItems[1].Title, "CRITICAL REFEEDING PROTOCOL") {
			t.Errorf("expected critical refeeding title, got %q", plan.ChecklistItems[1].Title)
		}
		if plan.PerimeterFindings[0].ZoneLabel != vc.wantZone {
			t.Errorf("vector %s: expected zone %q, got %q", vc.vector, vc.wantZone, plan.PerimeterFindings[0].ZoneLabel)
		}
		if plan.PerimeterFindings[0].VulnerabilityType != vc.wantVulnType {
			t.Errorf("vector %s: expected vuln %q, got %q", vc.vector, vc.wantVulnType, plan.PerimeterFindings[0].VulnerabilityType)
		}
		if plan.Phase != domain.DecompPhaseAcuteRefuge72H {
			t.Errorf("expected initial phase ACUTE_REFUGE_72H, got %q", plan.Phase)
		}
		if plan.RecidivismRisk != domain.RecidivismRiskCriticalFlight {
			t.Errorf("expected initial risk CRITICAL_FLIGHT_RISK, got %q", plan.RecidivismRisk)
		}
	}
}

func TestApplyDecompressionProgress_LifecycleAndGraduation(t *testing.T) {
	now := time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC)

	// Start with Buster's plan (matching seed 102 in Task 3)
	plan, err := GenerateDecompressionPlan(CreatePlanInput{
		PlanID:              "decomp-plan-102",
		PetID:               "pet-2026-001",
		PetName:             "Buster",
		Species:             "DOG",
		DaysMissing:         3,
		WeightLossPct:       4.0,
		EscapeVector:        "UNLATCHED_SIDE_GATE",
		GpsCollarEquipped:   true,
		DoubleLeashVerified: false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Step 1: Progress 1 checklist item and 1 perimeter finding -> CORTISOL_TAPER_DAY_4_7
	t1 := now.Add(24 * time.Hour)
	err = ApplyDecompressionProgress(plan, ProgressUpdateInput{
		CompleteItemID:     "chk-quiet-refuge",
		RemediateFindingID: "aud-primary-vector",
	}, t1)
	if err != nil {
		t.Fatalf("step 1 unexpected error: %v", err)
	}
	if plan.Phase != domain.DecompPhaseCortisolTaperDay4To7 {
		t.Errorf("step 1: expected phase CORTISOL_TAPER_DAY_4_7, got %q", plan.Phase)
	}
	if plan.RecidivismRiskScore != 74 {
		t.Errorf("step 1: expected RecidivismRiskScore 74, got %d", plan.RecidivismRiskScore)
	}

	// Step 2: Complete 2 more checklist items (total 3 completed, 1 finding remediated) -> ROUTINE_REINTEGRATION_DAY_8_14 & ELEVATED_PERIMETER_RISK
	err = ApplyDecompressionProgress(plan, ProgressUpdateInput{
		CompleteItemID: "chk-refeeding",
	}, t1.Add(time.Hour))
	if err != nil {
		t.Fatalf("step 2a unexpected error: %v", err)
	}
	err = ApplyDecompressionProgress(plan, ProgressUpdateInput{
		CompleteItemID: "chk-vet-panel",
	}, t1.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("step 2b unexpected error: %v", err)
	}
	if plan.Phase != domain.DecompPhaseRoutineReintegration14 {
		t.Errorf("step 2: expected phase ROUTINE_REINTEGRATION_DAY_8_14, got %q", plan.Phase)
	}
	if plan.RecidivismRisk != domain.RecidivismRiskElevatedPerimeter {
		t.Errorf("step 2: expected risk ELEVATED_PERIMETER_RISK, got %q (score=%d)", plan.RecidivismRisk, plan.RecidivismRiskScore)
	}

	// Step 3: Complete all checklist, remediate all perimeter, verify double leash & notes -> GRADUATED_STABLE & FORTIFIED_LOW_RISK
	leashTrue := true
	t3 := now.Add(14 * 24 * time.Hour)
	err = ApplyDecompressionProgress(plan, ProgressUpdateInput{
		CompleteAllChecklist:  true,
		RemediateAllPerimeter: true,
		DoubleLeashVerified:   &leashTrue,
		Notes:                 "All 14-day cortisol & perimeter fortification milestones verified.",
	}, t3)
	if err != nil {
		t.Fatalf("step 3 unexpected error: %v", err)
	}
	if plan.Phase != domain.DecompPhaseGraduatedStable {
		t.Errorf("step 3: expected phase GRADUATED_STABLE, got %q", plan.Phase)
	}
	if plan.RecidivismRisk != domain.RecidivismRiskFortifiedLow {
		t.Errorf("step 3: expected risk FORTIFIED_LOW_RISK, got %q (score=%d)", plan.RecidivismRisk, plan.RecidivismRiskScore)
	}
	if plan.GraduationCertificateID != "CERT-DECOMP-DECOMP-PLAN-102" {
		t.Errorf("step 3: expected GraduationCertificateID CERT-DECOMP-DECOMP-PLAN-102, got %q", plan.GraduationCertificateID)
	}
	if plan.CollarReadinessScore != 100 {
		t.Errorf("step 3: expected CollarReadinessScore 100, got %d", plan.CollarReadinessScore)
	}
	if !strings.Contains(plan.ClinicalSummary, "Note: All 14-day cortisol & perimeter fortification milestones verified.") {
		t.Errorf("step 3: expected ClinicalSummary to contain appended note, got %q", plan.ClinicalSummary)
	}
	if err := plan.Validate(); err != nil {
		t.Errorf("step 3: expected valid plan after graduation, got error: %v", err)
	}
}

func TestApplyDecompressionProgress_Errors(t *testing.T) {
	now := time.Now().UTC()
	if err := ApplyDecompressionProgress(nil, ProgressUpdateInput{}, now); err == nil {
		t.Fatalf("expected error for nil plan")
	}

	plan, err := GenerateDecompressionPlan(CreatePlanInput{
		PlanID:      "decomp-plan-err",
		PetID:       "pet-1",
		PetName:     "Max",
		DaysMissing: 2,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := ApplyDecompressionProgress(plan, ProgressUpdateInput{CompleteItemID: "non-existent"}, now); err == nil {
		t.Errorf("expected error for unknown CompleteItemID")
	}
	if err := ApplyDecompressionProgress(plan, ProgressUpdateInput{RemediateFindingID: "non-existent"}, now); err == nil {
		t.Errorf("expected error for unknown RemediateFindingID")
	}
}
