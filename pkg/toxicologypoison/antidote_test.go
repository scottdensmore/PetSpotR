package toxicologypoison_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/toxicologypoison"
)

func TestEvaluateToxicologyTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.ToxicologyEvaluationRequest{
		PetID:                     "PET-TP-TEST-01",
		PetName:                   "Waffle",
		Species:                   "Dog - Beagle",
		WeightKg:                  13.4,
		AgentType:                 domain.ToxicologyAgentXylitolInsulinSurgeHepatotoxin,
		EstimatedDoseMgKg:         620.0,
		ProthrombinTimeRatio:      1.8,
		BloodGlucoseMgDl:          42.0,
		TimeSinceIngestionMins:    45,
		CharcoalAvoidedForXylitol: true,
		VitaminK3MenadioneAvoided: true,
	}

	// 1. Critical Xylitol Hypoglycemia & Coagulopathy evaluation
	op, err := toxicologypoison.EvaluateToxicologyTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro {
		t.Errorf("expected status %s, got %s", domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	lowerNote := strings.ToLower(op.ToxicologyPitfallWarningNote)
	if !strings.Contains(lowerNote, "charcoal") || !strings.Contains(lowerNote, "xylitol") || !strings.Contains(lowerNote, "menadione") || !strings.Contains(lowerNote, "phytonadione") {
		t.Errorf("expected warning note to mention charcoal/xylitol and Vitamin K3 menadione/phytonadione, got %q", op.ToxicologyPitfallWarningNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged vs Critical across all 5 agent types
	stagedAgents := []domain.ToxicologyAgentType{
		domain.ToxicologyAgentSecondGenAnticoagulantBrodifacoum,
		domain.ToxicologyAgentXylitolInsulinSurgeHepatotoxin,
		domain.ToxicologyAgentIvermectinPermethrinLipophilic,
	}
	for _, agent := range stagedAgents {
		stagedReq := validBase
		stagedReq.ID = ""
		stagedReq.PetID = ""
		stagedReq.PetName = "Bandit"
		stagedReq.Species = ""
		stagedReq.AgentType = agent
		stagedReq.EstimatedDoseMgKg = 2.0
		stagedReq.ProthrombinTimeRatio = 1.1
		stagedReq.BloodGlucoseMgDl = 96.0
		stagedReq.TimeSinceIngestionMins = 60
		stagedReq.CharcoalAvoidedForXylitol = true
		stagedReq.VitaminK3MenadioneAvoided = true

		stagedOp, err := toxicologypoison.EvaluateToxicologyTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for agent %s: %v", agent, err)
		}
		if stagedOp.Status != domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon {
			t.Errorf("expected STAGED status for %s, got %s", agent, stagedOp.Status)
		}
	}

	// Critical agents even when PT and glucose are normal
	criticalAgents := []domain.ToxicologyAgentType{
		domain.ToxicologyAgentLiliumSppFelineNephrotoxin,
		domain.ToxicologyAgentBromethalinNeurotoxicCerebralEdema,
	}
	for _, agent := range criticalAgents {
		critReq := validBase
		critReq.AgentType = agent
		critReq.ProthrombinTimeRatio = 1.0
		critReq.BloodGlucoseMgDl = 100.0
		critReq.CharcoalAvoidedForXylitol = true
		critReq.VitaminK3MenadioneAvoided = true

		critOp, err := toxicologypoison.EvaluateToxicologyTriage(critReq)
		if err != nil {
			t.Fatalf("unexpected error for agent %s: %v", agent, err)
		}
		if critOp.Status != domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro {
			t.Errorf("expected CRITICAL status for %s, got %s", agent, critOp.Status)
		}
	}

	// 3. Pitfall flags (!CharcoalAvoidedForXylitol or !VitaminK3MenadioneAvoided) trigger Critical status
	pitfallReq := validBase
	pitfallReq.AgentType = domain.ToxicologyAgentSecondGenAnticoagulantBrodifacoum
	pitfallReq.ProthrombinTimeRatio = 1.1
	pitfallReq.BloodGlucoseMgDl = 95.0
	pitfallReq.CharcoalAvoidedForXylitol = false
	pitfallReq.VitaminK3MenadioneAvoided = false
	pitfallOp, err := toxicologypoison.EvaluateToxicologyTriage(pitfallReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pitfallOp.Status != domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro {
		t.Errorf("expected critical status when charcoal or K3 menadione pitfall flags are false, got %s", pitfallOp.Status)
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.ToxicologyEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.ToxicologyEvaluationRequest) { r.PetName = "  " }},
		{"weight < 0.8", func(r *domain.ToxicologyEvaluationRequest) { r.WeightKg = 0.4 }},
		{"weight > 100.0", func(r *domain.ToxicologyEvaluationRequest) { r.WeightKg = 115.0 }},
		{"invalid agent type", func(r *domain.ToxicologyEvaluationRequest) { r.AgentType = "UNKNOWN_TOXIN" }},
		{"dose <= 0", func(r *domain.ToxicologyEvaluationRequest) { r.EstimatedDoseMgKg = 0 }},
		{"dose > 5000.0", func(r *domain.ToxicologyEvaluationRequest) { r.EstimatedDoseMgKg = 5500.0 }},
		{"pt ratio < 0.8", func(r *domain.ToxicologyEvaluationRequest) { r.ProthrombinTimeRatio = 0.5 }},
		{"pt ratio > 10.0", func(r *domain.ToxicologyEvaluationRequest) { r.ProthrombinTimeRatio = 12.0 }},
		{"glucose < 15.0", func(r *domain.ToxicologyEvaluationRequest) { r.BloodGlucoseMgDl = 10.0 }},
		{"glucose > 400.0", func(r *domain.ToxicologyEvaluationRequest) { r.BloodGlucoseMgDl = 420.0 }},
		{"time < 1", func(r *domain.ToxicologyEvaluationRequest) { r.TimeSinceIngestionMins = 0 }},
		{"time > 10080", func(r *domain.ToxicologyEvaluationRequest) { r.TimeSinceIngestionMins = 12000 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := toxicologypoison.EvaluateToxicologyTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueToxicologyClearance(t *testing.T) {
	op, err := toxicologypoison.EvaluateToxicologyTriage(domain.ToxicologyEvaluationRequest{
		ID:                        "TP-OP-TEST",
		PetID:                     "PET-TP-TEST",
		PetName:                   "Waffle",
		Species:                   "Dog - Beagle",
		WeightKg:                  13.4,
		AgentType:                 domain.ToxicologyAgentXylitolInsulinSurgeHepatotoxin,
		EstimatedDoseMgKg:         620.0,
		ProthrombinTimeRatio:      1.8,
		BloodGlucoseMgDl:          42.0,
		TimeSinceIngestionMins:    45,
		CharcoalAvoidedForXylitol: true,
		VitaminK3MenadioneAvoided: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil / invalid step checks
	if err := toxicologypoison.UpdateProtocolStep(nil, toxicologypoison.StepApomorphineRopiniroleEmesisGate, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := toxicologypoison.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := toxicologypoison.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	// Clearance before completing all steps should fail
	if err := toxicologypoison.IssueToxicologyClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := toxicologypoison.IssueToxicologyClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	// Complete all 4 steps
	for _, s := range op.ProtocolSteps {
		if err := toxicologypoison.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon, op.Status)
	}

	// Issue clearance
	if err := toxicologypoison.IssueToxicologyClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.ToxicologyStatusClearedDecontaminatedPTGlucose {
		t.Errorf("expected status %s, got %s", domain.ToxicologyStatusClearedDecontaminatedPTGlucose, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "POISON-SAFE-") || len(op.ClearanceToken) != len("POISON-SAFE-")+12 {
		t.Errorf("expected POISON-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to Staged and clears token
	if err := toxicologypoison.UpdateProtocolStep(op, toxicologypoison.Step48HrPTRecheckRenalHepaticPanel, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := toxicologypoison.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("TP-OP-101")
	if !ok || op101.PetName != "Waffle" || op101.Status != domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro {
		t.Errorf("unexpected TP-OP-101: %+v", op101)
	}
	op102, ok := st.Get("TP-OP-102")
	if !ok || op102.PetName != "Bandit" || op102.Status != domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon {
		t.Errorf("unexpected TP-OP-102: %+v", op102)
	}
	op103, ok := st.Get("TP-OP-103")
	if !ok || op103.PetName != "Tigger" || op103.Status != domain.ToxicologyStatusClearedDecontaminatedPTGlucose || !strings.HasPrefix(op103.ClearanceToken, "POISON-SAFE-") {
		t.Errorf("unexpected TP-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}
	if _, err := st.UpdateStep("UNKNOWN", toxicologypoison.StepApomorphineRopiniroleEmesisGate, true); err == nil {
		t.Errorf("expected error updating step on unknown op")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	created, err := st.EvaluateAndSave(domain.ToxicologyEvaluationRequest{
		PetName:                   "Cleo",
		Species:                   "Cat - Domestic Shorthair",
		WeightKg:                  4.2,
		AgentType:                 domain.ToxicologyAgentLiliumSppFelineNephrotoxin,
		EstimatedDoseMgKg:         85.0,
		ProthrombinTimeRatio:      1.0,
		BloodGlucoseMgDl:          108.0,
		TimeSinceIngestionMins:    35,
		CharcoalAvoidedForXylitol: true,
		VitaminK3MenadioneAvoided: true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "TP-OP-104" {
		t.Errorf("expected TP-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.ToxicologyStatusClearedDecontaminatedPTGlucose || !strings.HasPrefix(cleared.ClearanceToken, "POISON-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
