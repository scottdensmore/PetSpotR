package transfusionbloodbank_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/transfusionbloodbank"
)

func TestEvaluateTransfusionTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.TransfusionEvaluationRequest{
		PetID:                            "PET-TB-TEST-01",
		PetName:                          "Saffron",
		Species:                          "Cat - Devon Rex",
		WeightKg:                         3.2,
		BloodGroupType:                   domain.TransfusionBloodGroupFelineTypeBAlloantibodyRisk,
		RecipientPCVPct:                  10.5,
		DonorPCVPct:                      38.0,
		TargetPCVPct:                     24.0,
		MajorMinorCrossmatchCompatible:   true,
		InLine170MicronFilterAttached:    true,
		LactatedRingersCoInfusionAvoided: true,
	}

	// 1. Critical Feline Type B Alloantibody / Low PCV evaluation
	op, err := transfusionbloodbank.EvaluateTransfusionTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.TransfusionStatusCriticalHemolyticAnaphylaxis {
		t.Errorf("expected status %s, got %s", domain.TransfusionStatusCriticalHemolyticAnaphylaxis, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	lowerNote := strings.ToLower(op.AlloantibodyCalciumWarningNote)
	if !strings.Contains(lowerNote, "alloantibodies") || !strings.Contains(lowerNote, "1 ml") || !strings.Contains(lowerNote, "lactated ringer") || !strings.Contains(lowerNote, "cpda-1") {
		t.Errorf("expected warning note to mention alloantibodies, 1 mL Type A to Type B anaphylaxis, and Lactated Ringer's / CPDA-1 clot formation, got %q", op.AlloantibodyCalciumWarningNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged evaluation across non-critical blood groups
	stagedBloodGroups := []domain.TransfusionBloodGroupType{
		domain.TransfusionBloodGroupFelineTypeACrossmatched,
		domain.TransfusionBloodGroupCanineDEA11NegativeUniversal,
		domain.TransfusionBloodGroupCanineDEA11PositiveSensitized,
	}
	for _, bg := range stagedBloodGroups {
		stagedReq := validBase
		stagedReq.ID = ""
		stagedReq.PetID = ""
		stagedReq.PetName = "Ranger"
		stagedReq.Species = ""
		stagedReq.BloodGroupType = bg
		stagedReq.RecipientPCVPct = 17.5
		stagedReq.DonorPCVPct = 48.0
		stagedReq.TargetPCVPct = 30.0
		stagedReq.MajorMinorCrossmatchCompatible = true
		stagedReq.InLine170MicronFilterAttached = true
		stagedReq.LactatedRingersCoInfusionAvoided = true

		stagedOp, err := transfusionbloodbank.EvaluateTransfusionTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for blood group %s: %v", bg, err)
		}
		if stagedOp.Status != domain.TransfusionStatusStagedCrossmatchedMicronFilter {
			t.Errorf("expected STAGED status for %s, got %s", bg, stagedOp.Status)
		}
	}

	// 3. Emergency Canine-to-Feline Xenotransfusion triggers Critical status
	xenoReq := validBase
	xenoReq.BloodGroupType = domain.TransfusionBloodGroupEmergencyCanineToFelineXeno
	xenoReq.RecipientPCVPct = 16.0
	xenoReq.TargetPCVPct = 28.0
	xenoOp, err := transfusionbloodbank.EvaluateTransfusionTriage(xenoReq)
	if err != nil {
		t.Fatalf("unexpected error for xenotransfusion: %v", err)
	}
	if xenoOp.Status != domain.TransfusionStatusCriticalHemolyticAnaphylaxis {
		t.Errorf("expected critical status for xenotransfusion, got %s", xenoOp.Status)
	}

	// 4. Incompatible crossmatch, missing filter, or LRS co-infusion triggers Critical status
	for _, tc := range []struct {
		name   string
		mutate func(r *domain.TransfusionEvaluationRequest)
	}{
		{"incompatible crossmatch", func(r *domain.TransfusionEvaluationRequest) { r.MajorMinorCrossmatchCompatible = false }},
		{"missing 170 micron filter", func(r *domain.TransfusionEvaluationRequest) { r.InLine170MicronFilterAttached = false }},
		{"LRS co-infusion not avoided", func(r *domain.TransfusionEvaluationRequest) { r.LactatedRingersCoInfusionAvoided = false }},
	} {
		req := validBase
		req.BloodGroupType = domain.TransfusionBloodGroupCanineDEA11NegativeUniversal
		req.RecipientPCVPct = 18.0
		req.TargetPCVPct = 30.0
		tc.mutate(&req)
		critOp, err := transfusionbloodbank.EvaluateTransfusionTriage(req)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tc.name, err)
		}
		if critOp.Status != domain.TransfusionStatusCriticalHemolyticAnaphylaxis {
			t.Errorf("expected critical status for %s, got %s", tc.name, critOp.Status)
		}
	}

	// 5. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.TransfusionEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.TransfusionEvaluationRequest) { r.PetName = "  " }},
		{"weight < 1.0", func(r *domain.TransfusionEvaluationRequest) { r.WeightKg = 0.5 }},
		{"weight > 100.0", func(r *domain.TransfusionEvaluationRequest) { r.WeightKg = 105.0 }},
		{"invalid blood group", func(r *domain.TransfusionEvaluationRequest) { r.BloodGroupType = "UNKNOWN_BLOOD_GROUP" }},
		{"recipient PCV < 4.0", func(r *domain.TransfusionEvaluationRequest) { r.RecipientPCVPct = 3.0 }},
		{"recipient PCV > 45.0", func(r *domain.TransfusionEvaluationRequest) { r.RecipientPCVPct = 48.0 }},
		{"donor PCV < 30.0", func(r *domain.TransfusionEvaluationRequest) { r.DonorPCVPct = 25.0 }},
		{"donor PCV > 65.0", func(r *domain.TransfusionEvaluationRequest) { r.DonorPCVPct = 68.0 }},
		{"target PCV <= recipient PCV", func(r *domain.TransfusionEvaluationRequest) { r.TargetPCVPct = 10.5 }},
		{"target PCV > 55.0", func(r *domain.TransfusionEvaluationRequest) { r.TargetPCVPct = 58.0 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := transfusionbloodbank.EvaluateTransfusionTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueTransfusionClearance(t *testing.T) {
	op, err := transfusionbloodbank.EvaluateTransfusionTriage(domain.TransfusionEvaluationRequest{
		ID:                               "TB-OP-TEST",
		PetID:                            "PET-TB-TEST",
		PetName:                          "Saffron",
		Species:                          "Cat - Devon Rex",
		WeightKg:                         3.2,
		BloodGroupType:                   domain.TransfusionBloodGroupFelineTypeBAlloantibodyRisk,
		RecipientPCVPct:                  10.5,
		DonorPCVPct:                      38.0,
		TargetPCVPct:                     24.0,
		MajorMinorCrossmatchCompatible:   true,
		InLine170MicronFilterAttached:    true,
		LactatedRingersCoInfusionAvoided: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := transfusionbloodbank.UpdateProtocolStep(nil, transfusionbloodbank.StepABDEA11TypingSlideCrossmatch, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := transfusionbloodbank.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := transfusionbloodbank.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	if err := transfusionbloodbank.IssueTransfusionClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := transfusionbloodbank.IssueTransfusionClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	for _, s := range op.ProtocolSteps {
		if err := transfusionbloodbank.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.TransfusionStatusStagedCrossmatchedMicronFilter {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.TransfusionStatusStagedCrossmatchedMicronFilter, op.Status)
	}

	if err := transfusionbloodbank.IssueTransfusionClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.TransfusionStatusSafePCVRestoredNormothermic {
		t.Errorf("expected status %s, got %s", domain.TransfusionStatusSafePCVRestoredNormothermic, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "BLOODBANK-SAFE-") || len(op.ClearanceToken) != len("BLOODBANK-SAFE-")+12 {
		t.Errorf("expected BLOODBANK-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to Staged and clears token
	if err := transfusionbloodbank.UpdateProtocolStep(op, transfusionbloodbank.StepPostPCVTotalSolidsHemolysisCheck, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.TransfusionStatusStagedCrossmatchedMicronFilter || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.TransfusionStatusStagedCrossmatchedMicronFilter, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := transfusionbloodbank.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("TB-OP-101")
	if !ok || op101.PetName != "Saffron" || op101.Status != domain.TransfusionStatusCriticalHemolyticAnaphylaxis {
		t.Errorf("unexpected TB-OP-101: %+v", op101)
	}
	op102, ok := st.Get("TB-OP-102")
	if !ok || op102.PetName != "Ranger" || op102.Status != domain.TransfusionStatusStagedCrossmatchedMicronFilter {
		t.Errorf("unexpected TB-OP-102: %+v", op102)
	}
	op103, ok := st.Get("TB-OP-103")
	if !ok || op103.PetName != "Cleo" || op103.Status != domain.TransfusionStatusSafePCVRestoredNormothermic || !strings.HasPrefix(op103.ClearanceToken, "BLOODBANK-SAFE-") {
		t.Errorf("unexpected TB-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}
	if _, err := st.UpdateStep("UNKNOWN", transfusionbloodbank.StepABDEA11TypingSlideCrossmatch, true); err == nil {
		t.Errorf("expected error updating step on unknown op")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	created, err := st.EvaluateAndSave(domain.TransfusionEvaluationRequest{
		PetName:                          "Bramble",
		Species:                          "Cat - British Shorthair",
		WeightKg:                         4.1,
		BloodGroupType:                   domain.TransfusionBloodGroupEmergencyCanineToFelineXeno,
		RecipientPCVPct:                  9.5,
		DonorPCVPct:                      45.0,
		TargetPCVPct:                     22.0,
		MajorMinorCrossmatchCompatible:   true,
		InLine170MicronFilterAttached:    true,
		LactatedRingersCoInfusionAvoided: true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "TB-OP-104" {
		t.Errorf("expected TB-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.TransfusionStatusSafePCVRestoredNormothermic || !strings.HasPrefix(cleared.ClearanceToken, "BLOODBANK-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
