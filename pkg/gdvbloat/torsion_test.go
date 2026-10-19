package gdvbloat_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/gdvbloat"
)

func TestEvaluateGDVTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.GDVEvaluationRequest{
		PetID:                        "PET-GDV-TEST-01",
		PetName:                      "Titan",
		Species:                      "Dog - Great Dane",
		WeightKg:                     64.0,
		PresentationType:             domain.GDVPresentationAcuteGDV360PyloricTorsion,
		PlasmaLactateMmolL:           7.8,
		HeartRateBpm:                 196,
		TympanicAbdominalGirthCm:     112.0,
		NonProductiveRetchingPresent: true,
		CranialCephalicDualIVAccess:  true,
		VentricularArrhythmiaVPCs:    true,
	}

	// 1. Critical GDV / Gastric Necrosis & VPC Shock evaluation
	op, err := gdvbloat.EvaluateGDVTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.GDVStatusCriticalNecrosisVPCShock {
		t.Errorf("expected status %s, got %s", domain.GDVStatusCriticalNecrosisVPCShock, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	lowerNote := strings.ToLower(op.CaudalVenaCavaWarningNote)
	if !strings.Contains(lowerNote, "saphenous") || !strings.Contains(lowerNote, "caudal vena cava") || !strings.Contains(lowerNote, "double-bubble") {
		t.Errorf("expected warning note to mention saphenous/caudal vena cava occlusion and double-bubble radiograph, got %q", op.CaudalVenaCavaWarningNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged trocar / dual IV evaluation across non-acute presentation types
	stagedPresentations := []domain.GDVPresentationType{
		domain.GDVPresentationPartialGDV180Volvulus,
		domain.GDVPresentationSimpleGastricDilatationFoodBloat,
		domain.GDVPresentationSplenicTorsionHemoabdomen,
		domain.GDVPresentationPostExerciseDeepChestAerophagia,
	}
	for _, pres := range stagedPresentations {
		stagedReq := validBase
		stagedReq.ID = ""
		stagedReq.PetID = ""
		stagedReq.PetName = "Baron"
		stagedReq.Species = ""
		stagedReq.PresentationType = pres
		stagedReq.PlasmaLactateMmolL = 3.4
		stagedReq.HeartRateBpm = 148
		stagedReq.TympanicAbdominalGirthCm = 86.0
		stagedReq.NonProductiveRetchingPresent = true
		stagedReq.CranialCephalicDualIVAccess = true
		stagedReq.VentricularArrhythmiaVPCs = false

		stagedOp, err := gdvbloat.EvaluateGDVTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for presentation %s: %v", pres, err)
		}
		if stagedOp.Status != domain.GDVStatusStagedTrocarDualIV {
			t.Errorf("expected STAGED status for %s, got %s", pres, stagedOp.Status)
		}
	}

	// 3. Missing cranial cephalic dual IV access triggers Critical status
	missingIVReq := validBase
	missingIVReq.PresentationType = domain.GDVPresentationPartialGDV180Volvulus
	missingIVReq.PlasmaLactateMmolL = 2.8
	missingIVReq.HeartRateBpm = 135
	missingIVReq.VentricularArrhythmiaVPCs = false
	missingIVReq.CranialCephalicDualIVAccess = false
	missingIVOp, err := gdvbloat.EvaluateGDVTriage(missingIVReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missingIVOp.Status != domain.GDVStatusCriticalNecrosisVPCShock {
		t.Errorf("expected critical status when cranial cephalic dual IV access is false, got %s", missingIVOp.Status)
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.GDVEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.GDVEvaluationRequest) { r.PetName = "  " }},
		{"weight < 5.0", func(r *domain.GDVEvaluationRequest) { r.WeightKg = 3.5 }},
		{"weight > 120.0", func(r *domain.GDVEvaluationRequest) { r.WeightKg = 125.0 }},
		{"invalid presentation type", func(r *domain.GDVEvaluationRequest) { r.PresentationType = "UNKNOWN_PRESENTATION" }},
		{"lactate < 0.5", func(r *domain.GDVEvaluationRequest) { r.PlasmaLactateMmolL = 0.2 }},
		{"lactate > 25.0", func(r *domain.GDVEvaluationRequest) { r.PlasmaLactateMmolL = 28.0 }},
		{"heart rate < 40", func(r *domain.GDVEvaluationRequest) { r.HeartRateBpm = 30 }},
		{"heart rate > 280", func(r *domain.GDVEvaluationRequest) { r.HeartRateBpm = 295 }},
		{"girth < 30.0", func(r *domain.GDVEvaluationRequest) { r.TympanicAbdominalGirthCm = 22.0 }},
		{"girth > 180.0", func(r *domain.GDVEvaluationRequest) { r.TympanicAbdominalGirthCm = 195.0 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := gdvbloat.EvaluateGDVTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueGDVClearance(t *testing.T) {
	op, err := gdvbloat.EvaluateGDVTriage(domain.GDVEvaluationRequest{
		ID:                           "GDV-OP-TEST",
		PetID:                        "PET-GDV-TEST",
		PetName:                      "Titan",
		Species:                      "Dog - Great Dane",
		WeightKg:                     64.0,
		PresentationType:             domain.GDVPresentationAcuteGDV360PyloricTorsion,
		PlasmaLactateMmolL:           7.8,
		HeartRateBpm:                 196,
		TympanicAbdominalGirthCm:     112.0,
		NonProductiveRetchingPresent: true,
		CranialCephalicDualIVAccess:  true,
		VentricularArrhythmiaVPCs:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil / invalid step checks
	if err := gdvbloat.UpdateProtocolStep(nil, gdvbloat.StepBilateralCephalicIVBolus, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := gdvbloat.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := gdvbloat.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	// Clearance before completing all steps should fail
	if err := gdvbloat.IssueGDVClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := gdvbloat.IssueGDVClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	// Complete all 4 steps
	for _, s := range op.ProtocolSteps {
		if err := gdvbloat.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.GDVStatusStagedTrocarDualIV {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.GDVStatusStagedTrocarDualIV, op.Status)
	}

	// Issue clearance
	if err := gdvbloat.IssueGDVClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.GDVStatusClearedGastropexySinus {
		t.Errorf("expected status %s, got %s", domain.GDVStatusClearedGastropexySinus, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "GDV-SAFE-") || len(op.ClearanceToken) != len("GDV-SAFE-")+12 {
		t.Errorf("expected GDV-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to Staged and clears token
	if err := gdvbloat.UpdateProtocolStep(op, gdvbloat.StepLidocaineVPCGastropexyPrep, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.GDVStatusStagedTrocarDualIV || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.GDVStatusStagedTrocarDualIV, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := gdvbloat.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("GDV-OP-101")
	if !ok || op101.PetName != "Titan" || op101.Status != domain.GDVStatusCriticalNecrosisVPCShock {
		t.Errorf("unexpected GDV-OP-101: %+v", op101)
	}
	op102, ok := st.Get("GDV-OP-102")
	if !ok || op102.PetName != "Baron" || op102.Status != domain.GDVStatusStagedTrocarDualIV {
		t.Errorf("unexpected GDV-OP-102: %+v", op102)
	}
	op103, ok := st.Get("GDV-OP-103")
	if !ok || op103.PetName != "Duchess" || op103.Status != domain.GDVStatusClearedGastropexySinus || !strings.HasPrefix(op103.ClearanceToken, "GDV-SAFE-") {
		t.Errorf("unexpected GDV-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}
	if _, err := st.UpdateStep("UNKNOWN", gdvbloat.StepBilateralCephalicIVBolus, true); err == nil {
		t.Errorf("expected error updating step on unknown op")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	created, err := st.EvaluateAndSave(domain.GDVEvaluationRequest{
		PetName:                      "Atlas",
		Species:                      "Dog - Irish Wolfhound",
		WeightKg:                     72.5,
		PresentationType:             domain.GDVPresentationSplenicTorsionHemoabdomen,
		PlasmaLactateMmolL:           8.4,
		HeartRateBpm:                 192,
		TympanicAbdominalGirthCm:     118.0,
		NonProductiveRetchingPresent: true,
		CranialCephalicDualIVAccess:  true,
		VentricularArrhythmiaVPCs:    true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "GDV-OP-104" {
		t.Errorf("expected GDV-OP-104, got %s", created.ID)
	}

	for _, step := range created.ProtocolSteps {
		if _, err := st.UpdateStep(created.ID, step.ID, true); err != nil {
			t.Fatalf("failed updating step %s: %v", step.ID, err)
		}
	}
	cleared, err := st.IssueClearance(created.ID)
	if err != nil {
		t.Fatalf("failed issuing clearance: %v", err)
	}
	if cleared.Status != domain.GDVStatusClearedGastropexySinus || !strings.HasPrefix(cleared.ClearanceToken, "GDV-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
