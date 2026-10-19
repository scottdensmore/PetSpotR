package acousticpanic_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/acousticpanic"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvaluateAcousticPanic_ValidationAndClassification(t *testing.T) {
	validBase := domain.AcousticEvaluationRequest{
		PetID:                        "PET-TEST-01",
		PetName:                      "Boomer",
		Species:                      "Dog - Border Collie",
		WeightKg:                     19.5,
		TriggerType:                  domain.TriggerFireworksBarrage,
		PeakDecibelsDb:               132.0,
		BarometricDropHpa:            2.5,
		BoltHeadingDegrees:           215,
		BarrierBreachInjurySuspected: true,
		NoChasePassiveLureEnforced:   false,
		AdaptilFeliwayStationStaged:  true,
	}

	// 1. Critical acoustic panic bolt evaluation
	op, err := acousticpanic.EvaluateAcousticPanic(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.AcousticStatusCriticalBolt {
		t.Errorf("expected status %s, got %s", domain.AcousticStatusCriticalBolt, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	if !strings.Contains(op.AdrenalNoChaseAdvisory, "NEVER") || !strings.Contains(strings.ToLower(op.AdrenalNoChaseAdvisory), "traffic") {
		t.Errorf("expected adrenal advisory to mention NEVER calling/chasing and roadway traffic, got %q", op.AdrenalNoChaseAdvisory)
	}
	if !strings.Contains(strings.ToLower(op.AdrenalNoChaseAdvisory), "pheromone") {
		t.Errorf("expected adrenal advisory to mention pheromone perimeter, got %q", op.AdrenalNoChaseAdvisory)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged sniff evaluation across all trigger types
	triggerTypes := []domain.AcousticTriggerType{
		domain.TriggerFireworksBarrage,
		domain.TriggerThunderstormBarometricDrop,
		domain.TriggerGunshotConstructionBlasting,
		domain.TriggerHighVoltageTransformerArc,
		domain.TriggerSirenSonicBoomStartle,
	}
	for i, trig := range triggerTypes {
		stagedReq := validBase
		stagedReq.PetName = "Willow"
		stagedReq.Species = ""
		stagedReq.PetID = ""
		stagedReq.TriggerType = trig
		stagedReq.PeakDecibelsDb = 104.0
		stagedReq.BarometricDropHpa = 8.0
		stagedReq.BoltHeadingDegrees = (i * 70) % 360
		stagedReq.BarrierBreachInjurySuspected = false
		stagedReq.NoChasePassiveLureEnforced = true
		stagedReq.AdaptilFeliwayStationStaged = i%2 == 0

		stagedOp, err := acousticpanic.EvaluateAcousticPanic(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for trigger %s: %v", trig, err)
		}
		if stagedOp.Status != domain.AcousticStatusStagedSniff {
			t.Errorf("expected STAGED status for %s, got %s", trig, stagedOp.Status)
		}
	}

	// 3. Barometric drop >= 12.0 triggers Critical Bolt status
	baroReq := validBase
	baroReq.PeakDecibelsDb = 95.0
	baroReq.BarometricDropHpa = 14.5
	baroReq.BarrierBreachInjurySuspected = false
	baroReq.NoChasePassiveLureEnforced = true
	baroOp, err := acousticpanic.EvaluateAcousticPanic(baroReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if baroOp.Status != domain.AcousticStatusCriticalBolt {
		t.Errorf("expected critical status for high barometric drop, got %s", baroOp.Status)
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.AcousticEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.AcousticEvaluationRequest) { r.PetName = "  " }},
		{"non-positive weight", func(r *domain.AcousticEvaluationRequest) { r.WeightKg = 0 }},
		{"excessive weight", func(r *domain.AcousticEvaluationRequest) { r.WeightKg = 125.0 }},
		{"invalid trigger type", func(r *domain.AcousticEvaluationRequest) { r.TriggerType = "UNKNOWN_TRIGGER" }},
		{"peak dB too low", func(r *domain.AcousticEvaluationRequest) { r.PeakDecibelsDb = 55.0 }},
		{"peak dB too high", func(r *domain.AcousticEvaluationRequest) { r.PeakDecibelsDb = 185.0 }},
		{"barometric drop negative", func(r *domain.AcousticEvaluationRequest) { r.BarometricDropHpa = -1.0 }},
		{"barometric drop too high", func(r *domain.AcousticEvaluationRequest) { r.BarometricDropHpa = 65.0 }},
		{"heading negative", func(r *domain.AcousticEvaluationRequest) { r.BoltHeadingDegrees = -5 }},
		{"heading > 359", func(r *domain.AcousticEvaluationRequest) { r.BoltHeadingDegrees = 360 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := acousticpanic.EvaluateAcousticPanic(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueAcousticClearance(t *testing.T) {
	op, err := acousticpanic.EvaluateAcousticPanic(domain.AcousticEvaluationRequest{
		ID:                           "AP-OP-TEST",
		PetID:                        "PET-AP-TEST",
		PetName:                      "Boomer",
		Species:                      "Dog - Border Collie",
		WeightKg:                     19.5,
		TriggerType:                  domain.TriggerFireworksBarrage,
		PeakDecibelsDb:               132.0,
		BarometricDropHpa:            2.5,
		BoltHeadingDegrees:           215,
		BarrierBreachInjurySuspected: true,
		NoChasePassiveLureEnforced:   false,
		AdaptilFeliwayStationStaged:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil / invalid step checks
	if err := acousticpanic.UpdateProtocolStep(nil, acousticpanic.StepSilentNoChasePerimeter, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := acousticpanic.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := acousticpanic.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	// Clearance before completing all steps should fail
	if err := acousticpanic.IssueAcousticClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := acousticpanic.IssueAcousticClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	// Complete all 4 steps
	for _, s := range op.ProtocolSteps {
		if err := acousticpanic.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.AcousticStatusStagedSniff {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.AcousticStatusStagedSniff, op.Status)
	}

	// Issue clearance
	if err := acousticpanic.IssueAcousticClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.AcousticStatusDecompressedSafe {
		t.Errorf("expected status %s, got %s", domain.AcousticStatusDecompressedSafe, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "ACOUSTIC-SAFE-") || len(op.ClearanceToken) != len("ACOUSTIC-SAFE-")+12 {
		t.Errorf("expected ACOUSTIC-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to StagedSniff and clears token
	if err := acousticpanic.UpdateProtocolStep(op, acousticpanic.StepThundershirtAcousticDecompress, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.AcousticStatusStagedSniff || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.AcousticStatusStagedSniff, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := acousticpanic.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("AP-OP-101")
	if !ok || op101.PetName != "Boomer" || op101.Status != domain.AcousticStatusCriticalBolt {
		t.Errorf("unexpected AP-OP-101: %+v", op101)
	}
	op102, ok := st.Get("AP-OP-102")
	if !ok || op102.PetName != "Willow" || op102.Status != domain.AcousticStatusStagedSniff {
		t.Errorf("unexpected AP-OP-102: %+v", op102)
	}
	op103, ok := st.Get("AP-OP-103")
	if !ok || op103.PetName != "Milo" || op103.Status != domain.AcousticStatusDecompressedSafe || !strings.HasPrefix(op103.ClearanceToken, "ACOUSTIC-SAFE-") {
		t.Errorf("unexpected AP-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}
	if _, err := st.UpdateStep("UNKNOWN", acousticpanic.StepSilentNoChasePerimeter, true); err == nil {
		t.Errorf("expected error for unknown op on UpdateStep")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error for unknown op on IssueClearance")
	}
	if _, err := st.EvaluateAndSave(domain.AcousticEvaluationRequest{PetName: ""}); err == nil {
		t.Errorf("expected error on invalid EvaluateAndSave")
	}

	created, err := st.EvaluateAndSave(domain.AcousticEvaluationRequest{
		PetName:                      "Koda",
		Species:                      "Dog - Alaskan Malamute",
		WeightKg:                     36.0,
		TriggerType:                  domain.TriggerGunshotConstructionBlasting,
		PeakDecibelsDb:               128.0,
		BarometricDropHpa:            1.5,
		BoltHeadingDegrees:           140,
		BarrierBreachInjurySuspected: true,
		NoChasePassiveLureEnforced:   true,
		AdaptilFeliwayStationStaged:  true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "AP-OP-104" {
		t.Errorf("expected AP-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.AcousticStatusDecompressedSafe || !strings.HasPrefix(cleared.ClearanceToken, "ACOUSTIC-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
