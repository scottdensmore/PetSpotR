package urethralobstruction_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/urethralobstruction"
)

func TestEvaluateUrethralTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.UrethralEvaluationRequest{
		PetID:                          "PET-UO-TEST-01",
		PetName:                        "Sylvester",
		Species:                        "Cat - Domestic Shorthair Neutered Male",
		WeightKg:                       5.8,
		ObstructionCause:               domain.ObstructionCauseStruwiteCrystallineMatrixPlug,
		SerumPotassiumMmolL:            8.9,
		HeartRateBpm:                   118,
		BladderPalpationDiameterCm:     9.5,
		AbsentPWaveTentedTWave:         true,
		CalciumGluconateBeforeSedation: true,
		ManualBladderExpressionAvoided: true,
	}

	// 1. Critical Hyperkalemic Bradycardia / Atrial Standstill evaluation
	op, err := urethralobstruction.EvaluateUrethralTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.UrethralStatusCriticalHyperkalemicBradycardia {
		t.Errorf("expected status %s, got %s", domain.UrethralStatusCriticalHyperkalemicBradycardia, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	lowerNote := strings.ToLower(op.CardioprotectionWarningNote)
	if !strings.Contains(lowerNote, "calcium gluconate") || !strings.Contains(lowerNote, "manual expression") {
		t.Errorf("expected warning note to mention calcium gluconate before sedation and avoiding forceful manual expression, got %q", op.CardioprotectionWarningNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged Calcium Cardioprotect & Retrohydropropulsion across all obstruction causes
	allCauses := []domain.UrethralObstructionCauseType{
		domain.ObstructionCauseStruwiteCrystallineMatrixPlug,
		domain.ObstructionCauseCalciumOxalateUrethrolith,
		domain.ObstructionCauseIdiopathicFICUrethralSpasm,
		domain.ObstructionCauseCanineOsPenisCalculusLodgment,
		domain.ObstructionCausePostCatheterStricturedReobstruction,
	}
	for _, cause := range allCauses {
		stagedReq := validBase
		stagedReq.ID = ""
		stagedReq.PetID = ""
		stagedReq.PetName = "Figaro"
		stagedReq.Species = ""
		stagedReq.ObstructionCause = cause
		stagedReq.SerumPotassiumMmolL = 5.4
		stagedReq.HeartRateBpm = 172
		stagedReq.BladderPalpationDiameterCm = 6.8
		stagedReq.AbsentPWaveTentedTWave = false
		stagedReq.CalciumGluconateBeforeSedation = true
		stagedReq.ManualBladderExpressionAvoided = true

		stagedOp, err := urethralobstruction.EvaluateUrethralTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for cause %s: %v", cause, err)
		}
		if stagedOp.Status != domain.UrethralStatusStagedCalciumRetrohydropropulsion {
			t.Errorf("expected STAGED status for %s, got %s", cause, stagedOp.Status)
		}
	}

	// 3. Missing calcium gluconate before sedation or manual bladder expression not avoided triggers Critical status
	missingCaReq := validBase
	missingCaReq.SerumPotassiumMmolL = 5.2
	missingCaReq.HeartRateBpm = 165
	missingCaReq.AbsentPWaveTentedTWave = false
	missingCaReq.CalciumGluconateBeforeSedation = false
	missingCaReq.ManualBladderExpressionAvoided = false
	missingCaOp, err := urethralobstruction.EvaluateUrethralTriage(missingCaReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if missingCaOp.Status != domain.UrethralStatusCriticalHyperkalemicBradycardia {
		t.Errorf("expected critical status when calcium gluconate or manual expression guardrail is false, got %s", missingCaOp.Status)
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.UrethralEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.UrethralEvaluationRequest) { r.PetName = "  " }},
		{"weight < 1.5", func(r *domain.UrethralEvaluationRequest) { r.WeightKg = 1.1 }},
		{"weight > 65.0", func(r *domain.UrethralEvaluationRequest) { r.WeightKg = 70.0 }},
		{"invalid obstruction cause", func(r *domain.UrethralEvaluationRequest) { r.ObstructionCause = "UNKNOWN_CAUSE" }},
		{"potassium < 2.5", func(r *domain.UrethralEvaluationRequest) { r.SerumPotassiumMmolL = 1.9 }},
		{"potassium > 12.5", func(r *domain.UrethralEvaluationRequest) { r.SerumPotassiumMmolL = 13.5 }},
		{"heart rate < 60", func(r *domain.UrethralEvaluationRequest) { r.HeartRateBpm = 45 }},
		{"heart rate > 260", func(r *domain.UrethralEvaluationRequest) { r.HeartRateBpm = 275 }},
		{"bladder < 2.0", func(r *domain.UrethralEvaluationRequest) { r.BladderPalpationDiameterCm = 1.2 }},
		{"bladder > 20.0", func(r *domain.UrethralEvaluationRequest) { r.BladderPalpationDiameterCm = 22.5 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := urethralobstruction.EvaluateUrethralTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueUrethralClearance(t *testing.T) {
	op, err := urethralobstruction.EvaluateUrethralTriage(domain.UrethralEvaluationRequest{
		ID:                             "UO-OP-TEST",
		PetID:                          "PET-UO-TEST",
		PetName:                        "Sylvester",
		Species:                        "Cat - Domestic Shorthair Neutered Male",
		WeightKg:                       5.8,
		ObstructionCause:               domain.ObstructionCauseStruwiteCrystallineMatrixPlug,
		SerumPotassiumMmolL:            8.9,
		HeartRateBpm:                   118,
		BladderPalpationDiameterCm:     9.5,
		AbsentPWaveTentedTWave:         true,
		CalciumGluconateBeforeSedation: true,
		ManualBladderExpressionAvoided: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil / invalid step checks
	if err := urethralobstruction.UpdateProtocolStep(nil, urethralobstruction.StepCalciumGluconateInsulinDextrose, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := urethralobstruction.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := urethralobstruction.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	// Clearance before completing all steps should fail
	if err := urethralobstruction.IssueUrethralClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := urethralobstruction.IssueUrethralClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	// Complete all 4 steps
	for _, s := range op.ProtocolSteps {
		if err := urethralobstruction.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.UrethralStatusStagedCalciumRetrohydropropulsion {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.UrethralStatusStagedCalciumRetrohydropropulsion, op.Status)
	}

	// Issue clearance
	if err := urethralobstruction.IssueUrethralClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.UrethralStatusClearedIndwellingNormokalemic {
		t.Errorf("expected status %s, got %s", domain.UrethralStatusClearedIndwellingNormokalemic, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "URETHRAL-SAFE-") || len(op.ClearanceToken) != len("URETHRAL-SAFE-")+12 {
		t.Errorf("expected URETHRAL-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to Staged and clears token
	if err := urethralobstruction.UpdateProtocolStep(op, urethralobstruction.StepPrazosinNormoKClearance, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.UrethralStatusStagedCalciumRetrohydropropulsion || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.UrethralStatusStagedCalciumRetrohydropropulsion, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := urethralobstruction.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("UO-OP-101")
	if !ok || op101.PetName != "Sylvester" || op101.Status != domain.UrethralStatusCriticalHyperkalemicBradycardia {
		t.Errorf("unexpected UO-OP-101: %+v", op101)
	}
	op102, ok := st.Get("UO-OP-102")
	if !ok || op102.PetName != "Figaro" || op102.Status != domain.UrethralStatusStagedCalciumRetrohydropropulsion {
		t.Errorf("unexpected UO-OP-102: %+v", op102)
	}
	op103, ok := st.Get("UO-OP-103")
	if !ok || op103.PetName != "Chester" || op103.Status != domain.UrethralStatusClearedIndwellingNormokalemic || !strings.HasPrefix(op103.ClearanceToken, "URETHRAL-SAFE-") {
		t.Errorf("unexpected UO-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}
	if _, err := st.UpdateStep("UNKNOWN", urethralobstruction.StepCalciumGluconateInsulinDextrose, true); err == nil {
		t.Errorf("expected error updating step on unknown op")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	created, err := st.EvaluateAndSave(domain.UrethralEvaluationRequest{
		PetName:                        "Barnaby",
		Species:                        "Cat - Maine Coon Neutered Male",
		WeightKg:                       7.4,
		ObstructionCause:               domain.ObstructionCauseStruwiteCrystallineMatrixPlug,
		SerumPotassiumMmolL:            9.2,
		HeartRateBpm:                   112,
		BladderPalpationDiameterCm:     10.2,
		AbsentPWaveTentedTWave:         true,
		CalciumGluconateBeforeSedation: true,
		ManualBladderExpressionAvoided: true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "UO-OP-104" {
		t.Errorf("expected UO-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.UrethralStatusClearedIndwellingNormokalemic || !strings.HasPrefix(cleared.ClearanceToken, "URETHRAL-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
