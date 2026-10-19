package biosecurityiso_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/biosecurityiso"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvaluateBiosecurityTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.BiosecurityEvaluationRequest{
		PetID:                              "PET-TEST-01",
		PetName:                            "Baxter",
		Species:                            "Dog - Rottweiler Juvenile",
		WeightKg:                           14.2,
		PathogenType:                       domain.BiosecurityPathogenCanineParvovirusCPV2Enteritis,
		NeutrophilCountCellsUl:             620,
		RoomPressureDifferentialPa:         -5.0,
		DisinfectantContactMins:            10,
		HemorrhagicDiarrheaPresent:         true,
		PotassiumPeroxymonosulfateFootbath: true,
		DedicatedPPEGownGlovesEnforced:     true,
	}

	// 1. Critical Neutropenic Sepsis / Breach evaluation
	op, err := biosecurityiso.EvaluateBiosecurityTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.BiosecurityStatusCriticalNeutropenicSepsisBreach {
		t.Errorf("expected status %s, got %s", domain.BiosecurityStatusCriticalNeutropenicSepsisBreach, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	if !strings.Contains(strings.ToLower(op.QuatDisinfectantFailureNote), "quaternary ammonium") || !strings.Contains(op.QuatDisinfectantFailureNote, "Virkon") {
		t.Errorf("expected quat disinfectant failure note to mention quaternary ammonium resistance and Virkon, got %q", op.QuatDisinfectantFailureNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged Negative-Pressure Isolation Cohort evaluation across all pathogen types
	pathogens := []domain.BiosecurityPathogenType{
		domain.BiosecurityPathogenCanineParvovirusCPV2Enteritis,
		domain.BiosecurityPathogenFelinePanleukopeniaFPV,
		domain.BiosecurityPathogenCanineDistemperCDVNeuro,
		domain.BiosecurityPathogenLeptospirosisZoonoticRenal,
		domain.BiosecurityPathogenStrepZooepidemicusHemorrhagic,
	}
	for _, pt := range pathogens {
		stagedReq := validBase
		stagedReq.PetName = "Mittens"
		stagedReq.Species = ""
		stagedReq.PetID = ""
		stagedReq.PathogenType = pt
		stagedReq.NeutrophilCountCellsUl = 2450
		stagedReq.RoomPressureDifferentialPa = -6.5
		stagedReq.DisinfectantContactMins = 12
		stagedReq.HemorrhagicDiarrheaPresent = false
		stagedReq.PotassiumPeroxymonosulfateFootbath = true
		stagedReq.DedicatedPPEGownGlovesEnforced = true

		stagedOp, err := biosecurityiso.EvaluateBiosecurityTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for pathogen %s: %v", pt, err)
		}
		if stagedOp.Status != domain.BiosecurityStatusStagedNegativePressureIsoCohort {
			t.Errorf("expected STAGED status for %s, got %s", pt, stagedOp.Status)
		}
	}

	// 3. Individual critical breach triggers (pressure > -2.5, contact < 10, missing footbath, missing PPE)
	breachTriggers := []struct {
		name   string
		mutate func(r *domain.BiosecurityEvaluationRequest)
	}{
		{"pressure > -2.5 Pa", func(r *domain.BiosecurityEvaluationRequest) { r.RoomPressureDifferentialPa = -1.0 }},
		{"contact < 10 mins", func(r *domain.BiosecurityEvaluationRequest) { r.DisinfectantContactMins = 5 }},
		{"missing footbath", func(r *domain.BiosecurityEvaluationRequest) { r.PotassiumPeroxymonosulfateFootbath = false }},
		{"missing PPE", func(r *domain.BiosecurityEvaluationRequest) { r.DedicatedPPEGownGlovesEnforced = false }},
	}
	for _, bt := range breachTriggers {
		t.Run(bt.name, func(t *testing.T) {
			req := validBase
			req.NeutrophilCountCellsUl = 2500
			req.RoomPressureDifferentialPa = -5.0
			req.DisinfectantContactMins = 10
			req.PotassiumPeroxymonosulfateFootbath = true
			req.DedicatedPPEGownGlovesEnforced = true
			bt.mutate(&req)

			bOp, err := biosecurityiso.EvaluateBiosecurityTriage(req)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", bt.name, err)
			}
			if bOp.Status != domain.BiosecurityStatusCriticalNeutropenicSepsisBreach {
				t.Errorf("expected critical status for %s, got %s", bt.name, bOp.Status)
			}
		})
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.BiosecurityEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.BiosecurityEvaluationRequest) { r.PetName = "   " }},
		{"non-positive weight", func(r *domain.BiosecurityEvaluationRequest) { r.WeightKg = 0 }},
		{"weight > 100", func(r *domain.BiosecurityEvaluationRequest) { r.WeightKg = 105.0 }},
		{"invalid pathogen type", func(r *domain.BiosecurityEvaluationRequest) { r.PathogenType = "UNKNOWN_PATHOGEN" }},
		{"neutrophils < 50", func(r *domain.BiosecurityEvaluationRequest) { r.NeutrophilCountCellsUl = 20 }},
		{"neutrophils > 40000", func(r *domain.BiosecurityEvaluationRequest) { r.NeutrophilCountCellsUl = 45000 }},
		{"pressure < -50", func(r *domain.BiosecurityEvaluationRequest) { r.RoomPressureDifferentialPa = -55.0 }},
		{"pressure > 20", func(r *domain.BiosecurityEvaluationRequest) { r.RoomPressureDifferentialPa = 25.0 }},
		{"contact < 1", func(r *domain.BiosecurityEvaluationRequest) { r.DisinfectantContactMins = 0 }},
		{"contact > 60", func(r *domain.BiosecurityEvaluationRequest) { r.DisinfectantContactMins = 65 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := biosecurityiso.EvaluateBiosecurityTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueBiosecurityClearance(t *testing.T) {
	op, err := biosecurityiso.EvaluateBiosecurityTriage(domain.BiosecurityEvaluationRequest{
		ID:                                 "BI-OP-TEST",
		PetID:                              "PET-BI-TEST",
		PetName:                            "Baxter",
		Species:                            "Dog - Rottweiler Juvenile",
		WeightKg:                           14.2,
		PathogenType:                       domain.BiosecurityPathogenCanineParvovirusCPV2Enteritis,
		NeutrophilCountCellsUl:             620,
		RoomPressureDifferentialPa:         -5.0,
		DisinfectantContactMins:            10,
		HemorrhagicDiarrheaPresent:         true,
		PotassiumPeroxymonosulfateFootbath: true,
		DedicatedPPEGownGlovesEnforced:     true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil / invalid step checks
	if err := biosecurityiso.UpdateProtocolStep(nil, biosecurityiso.StepAnteroomPPEFootbathLock, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := biosecurityiso.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := biosecurityiso.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	// Clearance before completing all steps should fail
	if err := biosecurityiso.IssueBiosecurityClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := biosecurityiso.IssueBiosecurityClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	// Complete all 4 steps
	for _, s := range op.ProtocolSteps {
		if err := biosecurityiso.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.BiosecurityStatusStagedNegativePressureIsoCohort {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.BiosecurityStatusStagedNegativePressureIsoCohort, op.Status)
	}

	// Issue clearance
	if err := biosecurityiso.IssueBiosecurityClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.BiosecurityStatusClearedSafePCRNegativeDecon {
		t.Errorf("expected status %s, got %s", domain.BiosecurityStatusClearedSafePCRNegativeDecon, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "BIOSEC-SAFE-") || len(op.ClearanceToken) != len("BIOSEC-SAFE-")+12 {
		t.Errorf("expected BIOSEC-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to Staged and clears token
	if err := biosecurityiso.UpdateProtocolStep(op, biosecurityiso.StepPCRSheddingClearanceBath, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.BiosecurityStatusStagedNegativePressureIsoCohort || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.BiosecurityStatusStagedNegativePressureIsoCohort, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := biosecurityiso.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("BI-OP-101")
	if !ok || op101.PetName != "Baxter" || op101.Status != domain.BiosecurityStatusCriticalNeutropenicSepsisBreach {
		t.Errorf("unexpected BI-OP-101: %+v", op101)
	}
	op102, ok := st.Get("BI-OP-102")
	if !ok || op102.PetName != "Mittens" || op102.Status != domain.BiosecurityStatusStagedNegativePressureIsoCohort {
		t.Errorf("unexpected BI-OP-102: %+v", op102)
	}
	op103, ok := st.Get("BI-OP-103")
	if !ok || op103.PetName != "Otis" || op103.Status != domain.BiosecurityStatusClearedSafePCRNegativeDecon || !strings.HasPrefix(op103.ClearanceToken, "BIOSEC-SAFE-") {
		t.Errorf("unexpected BI-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}
	if _, err := st.UpdateStep("UNKNOWN", biosecurityiso.StepAnteroomPPEFootbathLock, true); err == nil {
		t.Errorf("expected error updating step on unknown op")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	created, err := st.EvaluateAndSave(domain.BiosecurityEvaluationRequest{
		PetName:                            "Freya",
		Species:                            "Dog - Husky Juvenile",
		WeightKg:                           11.5,
		PathogenType:                       domain.BiosecurityPathogenCanineDistemperCDVNeuro,
		NeutrophilCountCellsUl:             890,
		RoomPressureDifferentialPa:         -6.0,
		DisinfectantContactMins:            10,
		HemorrhagicDiarrheaPresent:         false,
		PotassiumPeroxymonosulfateFootbath: true,
		DedicatedPPEGownGlovesEnforced:     true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "BI-OP-104" {
		t.Errorf("expected BI-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.BiosecurityStatusClearedSafePCRNegativeDecon || !strings.HasPrefix(cleared.ClearanceToken, "BIOSEC-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
