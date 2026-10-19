package hyperbaricco_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/hyperbaricco"
)

func TestEvaluateHyperbaricCOTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.HyperbaricEvaluationRequest{
		PetID:                         "PET-TEST-01",
		PetName:                       "Cinder",
		Species:                       "Cat - Persian",
		WeightKg:                      4.2,
		ExposureSource:                domain.COExposureStructuralHouseFireSynthetics,
		CarboxyhemoglobinPct:          34.5,
		BloodLactateMmolL:             11.2,
		ChamberPressureAta:            2.0,
		SingeVibrissaeSootNares:       true,
		PulseOximetryFalseNormalNoted: true,
		HydroxocobalaminAdministered:  true,
	}

	// 1. Critical COHb / Cyanide evaluation
	op, err := hyperbaricco.EvaluateHyperbaricCOTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.HyperbaricStatusCriticalCOHbCyanide {
		t.Errorf("expected status %s, got %s", domain.HyperbaricStatusCriticalCOHbCyanide, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	if !strings.Contains(op.CoOximetryWarningNote, "98-100%") || !strings.Contains(strings.ToLower(op.CoOximetryWarningNote), "co-oximetry") {
		t.Errorf("expected warning note to mention 98-100%% false SpO2 and CO-oximetry, got %q", op.CoOximetryWarningNote)
	}
	if !strings.Contains(strings.ToLower(op.CoOximetryWarningNote), "hydroxocobalamin") {
		t.Errorf("expected warning note to mention hydroxocobalamin, got %q", op.CoOximetryWarningNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged HBOT evaluation across all exposure sources
	sources := []domain.COExposureSourceType{
		domain.COExposureStructuralHouseFireSynthetics,
		domain.COExposureEnclosedGarageExhaustCO,
		domain.COExposurePortableGeneratorCabinLeak,
		domain.COExposureSmolderingBarnTorfSmoke,
		domain.COExposurePropaneHeaterRVAsphyxia,
	}
	for _, src := range sources {
		stagedReq := validBase
		stagedReq.PetName = "Diesel"
		stagedReq.Species = ""
		stagedReq.PetID = ""
		stagedReq.ExposureSource = src
		stagedReq.CarboxyhemoglobinPct = 14.0
		stagedReq.BloodLactateMmolL = 3.4
		stagedReq.ChamberPressureAta = 1.8
		stagedReq.SingeVibrissaeSootNares = false
		stagedReq.PulseOximetryFalseNormalNoted = true
		stagedReq.HydroxocobalaminAdministered = false

		stagedOp, err := hyperbaricco.EvaluateHyperbaricCOTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for source %s: %v", src, err)
		}
		if stagedOp.Status != domain.HyperbaricStatusStagedHBOTHydroxocobalamin {
			t.Errorf("expected STAGED status for %s, got %s", src, stagedOp.Status)
		}
	}

	// 3. False-normal pulse oximetry not noted triggers Critical status
	spo2Req := validBase
	spo2Req.CarboxyhemoglobinPct = 12.0
	spo2Req.BloodLactateMmolL = 2.5
	spo2Req.SingeVibrissaeSootNares = false
	spo2Req.PulseOximetryFalseNormalNoted = false
	spo2Op, err := hyperbaricco.EvaluateHyperbaricCOTriage(spo2Req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spo2Op.Status != domain.HyperbaricStatusCriticalCOHbCyanide {
		t.Errorf("expected critical status when pulse oximetry false-normal not noted, got %s", spo2Op.Status)
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.HyperbaricEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.HyperbaricEvaluationRequest) { r.PetName = "  " }},
		{"non-positive weight", func(r *domain.HyperbaricEvaluationRequest) { r.WeightKg = 0 }},
		{"weight > 120", func(r *domain.HyperbaricEvaluationRequest) { r.WeightKg = 125.0 }},
		{"invalid exposure source", func(r *domain.HyperbaricEvaluationRequest) { r.ExposureSource = "UNKNOWN_SOURCE" }},
		{"cohb negative", func(r *domain.HyperbaricEvaluationRequest) { r.CarboxyhemoglobinPct = -1.0 }},
		{"cohb > 85", func(r *domain.HyperbaricEvaluationRequest) { r.CarboxyhemoglobinPct = 88.0 }},
		{"lactate < 0.5", func(r *domain.HyperbaricEvaluationRequest) { r.BloodLactateMmolL = 0.2 }},
		{"lactate > 30", func(r *domain.HyperbaricEvaluationRequest) { r.BloodLactateMmolL = 32.0 }},
		{"ata < 1.0", func(r *domain.HyperbaricEvaluationRequest) { r.ChamberPressureAta = 0.8 }},
		{"ata > 3.0", func(r *domain.HyperbaricEvaluationRequest) { r.ChamberPressureAta = 3.5 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := hyperbaricco.EvaluateHyperbaricCOTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueHyperbaricClearance(t *testing.T) {
	op, err := hyperbaricco.EvaluateHyperbaricCOTriage(domain.HyperbaricEvaluationRequest{
		ID:                            "HB-OP-TEST",
		PetID:                         "PET-HB-TEST",
		PetName:                       "Cinder",
		Species:                       "Cat - Persian",
		WeightKg:                      4.2,
		ExposureSource:                domain.COExposureStructuralHouseFireSynthetics,
		CarboxyhemoglobinPct:          34.5,
		BloodLactateMmolL:             11.2,
		ChamberPressureAta:            2.0,
		SingeVibrissaeSootNares:       true,
		PulseOximetryFalseNormalNoted: true,
		HydroxocobalaminAdministered:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil / invalid step checks
	if err := hyperbaricco.UpdateProtocolStep(nil, hyperbaricco.StepCoOximetryFalseSpO2Bypass, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := hyperbaricco.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := hyperbaricco.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	// Clearance before completing all steps should fail
	if err := hyperbaricco.IssueHyperbaricClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := hyperbaricco.IssueHyperbaricClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	// Complete all 4 steps
	for _, s := range op.ProtocolSteps {
		if err := hyperbaricco.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.HyperbaricStatusStagedHBOTHydroxocobalamin {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.HyperbaricStatusStagedHBOTHydroxocobalamin, op.Status)
	}

	// Issue clearance
	if err := hyperbaricco.IssueHyperbaricClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.HyperbaricStatusClearedSafeNormoxic {
		t.Errorf("expected status %s, got %s", domain.HyperbaricStatusClearedSafeNormoxic, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "HBOT-SAFE-") || len(op.ClearanceToken) != len("HBOT-SAFE-")+12 {
		t.Errorf("expected HBOT-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to Staged and clears token
	if err := hyperbaricco.UpdateProtocolStep(op, hyperbaricco.StepDelayedNeuroSequelaeCheck, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.HyperbaricStatusStagedHBOTHydroxocobalamin || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.HyperbaricStatusStagedHBOTHydroxocobalamin, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := hyperbaricco.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("HB-OP-101")
	if !ok || op101.PetName != "Cinder" || op101.Status != domain.HyperbaricStatusCriticalCOHbCyanide {
		t.Errorf("unexpected HB-OP-101: %+v", op101)
	}
	op102, ok := st.Get("HB-OP-102")
	if !ok || op102.PetName != "Diesel" || op102.Status != domain.HyperbaricStatusStagedHBOTHydroxocobalamin {
		t.Errorf("unexpected HB-OP-102: %+v", op102)
	}
	op103, ok := st.Get("HB-OP-103")
	if !ok || op103.PetName != "Pebble" || op103.Status != domain.HyperbaricStatusClearedSafeNormoxic || !strings.HasPrefix(op103.ClearanceToken, "HBOT-SAFE-") {
		t.Errorf("unexpected HB-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}
	if _, err := st.UpdateStep("UNKNOWN", hyperbaricco.StepCoOximetryFalseSpO2Bypass, true); err == nil {
		t.Errorf("expected error updating step on unknown op")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	created, err := st.EvaluateAndSave(domain.HyperbaricEvaluationRequest{
		PetName:                       "Ash",
		Species:                       "Cat - Domestic Shorthair",
		WeightKg:                      4.8,
		ExposureSource:                domain.COExposureSmolderingBarnTorfSmoke,
		CarboxyhemoglobinPct:          26.4,
		BloodLactateMmolL:             9.5,
		ChamberPressureAta:            2.2,
		SingeVibrissaeSootNares:       true,
		PulseOximetryFalseNormalNoted: true,
		HydroxocobalaminAdministered:  true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "HB-OP-104" {
		t.Errorf("expected HB-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.HyperbaricStatusClearedSafeNormoxic || !strings.HasPrefix(cleared.ClearanceToken, "HBOT-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
