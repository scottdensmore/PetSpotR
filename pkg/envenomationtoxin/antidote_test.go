package envenomationtoxin_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/envenomationtoxin"
)

func TestEvaluateEnvenomationTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.EnvenomationEvaluationRequest{
		PetID:                         "PET-ET-TEST-01",
		PetName:                       "Copper",
		Species:                       "Dog - Catahoula Leopard Dog",
		WeightKg:                      26.5,
		ToxinType:                     domain.ToxinPitViperCrotalidHEI,
		EdemaSpreadCmHr:               6.5,
		EchinocytePct:                 38.0,
		QuillCountMuzzleThorax:        0,
		OralMucosaFlushedForward:      true,
		QuillsCutOrPulledAwakeAvoided: true,
		AntivenomVialsStaged:          2,
	}

	// 1. Critical pit-viper evaluation
	op, err := envenomationtoxin.EvaluateEnvenomationTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.EnvenomationStatusCritical {
		t.Errorf("expected status %s, got %s", domain.EnvenomationStatusCritical, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	lowerNote := strings.ToLower(op.ContraindicationWarningNote)
	if !strings.Contains(lowerNote, "never cut porcupine quills") || !strings.Contains(lowerNote, "bufo") {
		t.Errorf("expected contraindication warning note to mention never cutting porcupine quills and Bufo oral lavage, got %q", op.ContraindicationWarningNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged evaluation across non-critical toxin types
	stagedTypes := []domain.EnvenomationToxinType{
		domain.ToxinPitViperCrotalidHEI,
		domain.ToxinElapidCoralNeurotoxin,
		domain.ToxinBufoMarinusCardiotoxin,
		domain.ToxinPorcupineRetrogradeQuill,
	}
	for _, tox := range stagedTypes {
		stagedReq := validBase
		stagedReq.ID = ""
		stagedReq.PetID = ""
		stagedReq.Species = ""
		stagedReq.PetName = "Bramble"
		stagedReq.ToxinType = tox
		stagedReq.EdemaSpreadCmHr = 1.2
		stagedReq.EchinocytePct = 5.0
		stagedReq.QuillCountMuzzleThorax = 12
		stagedReq.OralMucosaFlushedForward = true
		stagedReq.QuillsCutOrPulledAwakeAvoided = true
		stagedReq.AntivenomVialsStaged = 1

		stagedOp, err := envenomationtoxin.EvaluateEnvenomationTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for toxin %s: %v", tox, err)
		}
		if stagedOp.Status != domain.EnvenomationStatusStaged {
			t.Errorf("expected STAGED status for %s, got %s", tox, stagedOp.Status)
		}
	}

	// 3. Critical triggers: blue-green algae, Bufo without forward flush, cut quills, quill count >= 20
	algaeReq := validBase
	algaeReq.ToxinType = domain.ToxinBlueGreenAlgaeCyanotoxin
	algaeReq.EdemaSpreadCmHr = 0.0
	algaeReq.EchinocytePct = 0.0
	algaeOp, err := envenomationtoxin.EvaluateEnvenomationTriage(algaeReq)
	if err != nil || algaeOp.Status != domain.EnvenomationStatusCritical {
		t.Errorf("expected critical status for blue-green algae cyanotoxin, got op=%+v err=%v", algaeOp, err)
	}

	bufoUnflushed := validBase
	bufoUnflushed.ToxinType = domain.ToxinBufoMarinusCardiotoxin
	bufoUnflushed.EdemaSpreadCmHr = 0.5
	bufoUnflushed.EchinocytePct = 1.0
	bufoUnflushed.OralMucosaFlushedForward = false
	bufoOp, err := envenomationtoxin.EvaluateEnvenomationTriage(bufoUnflushed)
	if err != nil || bufoOp.Status != domain.EnvenomationStatusCritical {
		t.Errorf("expected critical status for Bufo without forward flush, got op=%+v err=%v", bufoOp, err)
	}

	cutQuills := validBase
	cutQuills.ToxinType = domain.ToxinPorcupineRetrogradeQuill
	cutQuills.EdemaSpreadCmHr = 0.5
	cutQuills.EchinocytePct = 0.0
	cutQuills.QuillCountMuzzleThorax = 8
	cutQuills.QuillsCutOrPulledAwakeAvoided = false
	cutOp, err := envenomationtoxin.EvaluateEnvenomationTriage(cutQuills)
	if err != nil || cutOp.Status != domain.EnvenomationStatusCritical {
		t.Errorf("expected critical status when quills cut or pulled awake not avoided, got op=%+v err=%v", cutOp, err)
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.EnvenomationEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.EnvenomationEvaluationRequest) { r.PetName = "   " }},
		{"non-positive weight", func(r *domain.EnvenomationEvaluationRequest) { r.WeightKg = 0 }},
		{"weight > 120", func(r *domain.EnvenomationEvaluationRequest) { r.WeightKg = 125.0 }},
		{"invalid toxin type", func(r *domain.EnvenomationEvaluationRequest) { r.ToxinType = "UNKNOWN_TOXIN" }},
		{"edema negative", func(r *domain.EnvenomationEvaluationRequest) { r.EdemaSpreadCmHr = -0.5 }},
		{"edema > 50", func(r *domain.EnvenomationEvaluationRequest) { r.EdemaSpreadCmHr = 55.0 }},
		{"echinocyte negative", func(r *domain.EnvenomationEvaluationRequest) { r.EchinocytePct = -1.0 }},
		{"echinocyte > 100", func(r *domain.EnvenomationEvaluationRequest) { r.EchinocytePct = 105.0 }},
		{"quills negative", func(r *domain.EnvenomationEvaluationRequest) { r.QuillCountMuzzleThorax = -1 }},
		{"quills > 500", func(r *domain.EnvenomationEvaluationRequest) { r.QuillCountMuzzleThorax = 505 }},
		{"vials negative", func(r *domain.EnvenomationEvaluationRequest) { r.AntivenomVialsStaged = -1 }},
		{"vials > 20", func(r *domain.EnvenomationEvaluationRequest) { r.AntivenomVialsStaged = 25 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := envenomationtoxin.EvaluateEnvenomationTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueEnvenomationClearance(t *testing.T) {
	op, err := envenomationtoxin.EvaluateEnvenomationTriage(domain.EnvenomationEvaluationRequest{
		ID:                            "ET-OP-TEST",
		PetID:                         "PET-ET-TEST",
		PetName:                       "Copper",
		Species:                       "Dog - Catahoula Leopard Dog",
		WeightKg:                      26.5,
		ToxinType:                     domain.ToxinPitViperCrotalidHEI,
		EdemaSpreadCmHr:               6.5,
		EchinocytePct:                 38.0,
		QuillCountMuzzleThorax:        0,
		OralMucosaFlushedForward:      true,
		QuillsCutOrPulledAwakeAvoided: true,
		AntivenomVialsStaged:          2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := envenomationtoxin.UpdateProtocolStep(nil, envenomationtoxin.StepDemarcationPerimeterMark, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := envenomationtoxin.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := envenomationtoxin.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	if err := envenomationtoxin.IssueEnvenomationClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := envenomationtoxin.IssueEnvenomationClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	for _, s := range op.ProtocolSteps {
		if err := envenomationtoxin.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.EnvenomationStatusStaged {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.EnvenomationStatusStaged, op.Status)
	}

	if err := envenomationtoxin.IssueEnvenomationClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.EnvenomationStatusStabilized {
		t.Errorf("expected status %s, got %s", domain.EnvenomationStatusStabilized, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "TOXIN-SAFE-") || len(op.ClearanceToken) != len("TOXIN-SAFE-")+12 {
		t.Errorf("expected TOXIN-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	if err := envenomationtoxin.UpdateProtocolStep(op, envenomationtoxin.StepCoagEchinocyteRecheck, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.EnvenomationStatusStaged || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.EnvenomationStatusStaged, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := envenomationtoxin.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("ET-OP-101")
	if !ok || op101.PetName != "Copper" || op101.Status != domain.EnvenomationStatusCritical {
		t.Errorf("unexpected ET-OP-101: %+v", op101)
	}
	op102, ok := st.Get("ET-OP-102")
	if !ok || op102.PetName != "Bramble" || op102.Status != domain.EnvenomationStatusStaged {
		t.Errorf("unexpected ET-OP-102: %+v", op102)
	}
	op103, ok := st.Get("ET-OP-103")
	if !ok || op103.PetName != "Mango" || op103.Status != domain.EnvenomationStatusStabilized || !strings.HasPrefix(op103.ClearanceToken, "TOXIN-SAFE-") {
		t.Errorf("unexpected ET-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}

	created, err := st.EvaluateAndSave(domain.EnvenomationEvaluationRequest{
		PetName:                       "Timber",
		Species:                       "Dog - Australian Cattle Dog",
		WeightKg:                      19.8,
		ToxinType:                     domain.ToxinElapidCoralNeurotoxin,
		EdemaSpreadCmHr:               2.0,
		EchinocytePct:                 12.0,
		QuillCountMuzzleThorax:        0,
		OralMucosaFlushedForward:      true,
		QuillsCutOrPulledAwakeAvoided: true,
		AntivenomVialsStaged:          3,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "ET-OP-104" {
		t.Errorf("expected ET-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.EnvenomationStatusStabilized || !strings.HasPrefix(cleared.ClearanceToken, "TOXIN-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}

	if _, err := st.UpdateStep("UNKNOWN", envenomationtoxin.StepDemarcationPerimeterMark, true); err == nil {
		t.Errorf("expected error updating step on unknown op")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}
}
