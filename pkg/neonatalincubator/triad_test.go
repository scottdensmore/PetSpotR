package neonatalincubator_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/neonatalincubator"
)

func TestEvaluateNeonatalTriage_CriticalStagedAndValidation(t *testing.T) {
	// 1. Critical fading triad (hypothermic < 35.5°C, hypoglycemic < 60 mg/dL, !warmedBeforeFeedingGate)
	critReq := domain.NeonatalEvaluationRequest{
		PetName:                   "Clover Litter",
		Species:                   "Cat - Domestic Shorthair Neonates",
		WeightGrams:               92.0,
		LitterCount:               4,
		AgeDays:                   5,
		LitterType:                domain.NeonatalLitterUnweanedKitten0To2Wk,
		NeonateRectalTempCelsius:  33.8,
		BloodGlucoseMgDl:          44.0,
		IncubatorHumidityPct:      62.0,
		SucklingReflexPresent:     false,
		WarmedBeforeFeedingGate:   false,
		SublingualDextroseApplied: true,
	}
	critOp, err := neonatalincubator.EvaluateNeonatalTriage(critReq)
	if err != nil {
		t.Fatalf("unexpected error evaluating critical neonatal triage: %v", err)
	}
	if critOp.Status != domain.NeonatalTriageStatusFadingTriadHypothermicCritical {
		t.Errorf("expected FADING_TRIAD_HYPOTHERMIC_CRITICAL, got %q", critOp.Status)
	}
	if !strings.Contains(critOp.IleusWarmingGateNote, "36.1°C") || !strings.Contains(critOp.IleusWarmingGateNote, "97.0°F") || !strings.Contains(strings.ToLower(critOp.IleusWarmingGateNote), "ileus") {
		t.Errorf("expected IleusWarmingGateNote to reference 36.1°C / 97.0°F and paralytic ileus, got %q", critOp.IleusWarmingGateNote)
	}
	if len(critOp.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(critOp.ProtocolSteps))
	}
	expectedStepIDs := []string{
		"STEP-INCUBATOR-WARMING-GATE",
		"STEP-SUBLINGUAL-DEXTROSE-50",
		"STEP-OROGASTRIC-TUBE-GAVAGE",
		"STEP-ANOGENITAL-STIM-WEIGH",
	}
	for i, wantID := range expectedStepIDs {
		if critOp.ProtocolSteps[i].ID != wantID {
			t.Errorf("step[%d] expected ID %q, got %q", i, wantID, critOp.ProtocolSteps[i].ID)
		}
	}

	// 2. Cleft palate puppy without suckling reflex -> Critical even if normothermic and euglycemic
	cleftReq := domain.NeonatalEvaluationRequest{
		PetName:                   "Biscuit Litter",
		Species:                   "Dog - Bulldog Neonates",
		WeightGrams:               210.0,
		LitterCount:               2,
		AgeDays:                   3,
		LitterType:                domain.NeonatalLitterPuppyCleft,
		NeonateRectalTempCelsius:  36.8,
		BloodGlucoseMgDl:          92.0,
		IncubatorHumidityPct:      60.0,
		SucklingReflexPresent:     false,
		WarmedBeforeFeedingGate:   true,
		SublingualDextroseApplied: true,
	}
	cleftOp, err := neonatalincubator.EvaluateNeonatalTriage(cleftReq)
	if err != nil {
		t.Fatalf("unexpected error evaluating cleft palate litter: %v", err)
	}
	if cleftOp.Status != domain.NeonatalTriageStatusFadingTriadHypothermicCritical {
		t.Errorf("expected FADING_TRIAD_HYPOTHERMIC_CRITICAL for cleft litter without suckling reflex, got %q", cleftOp.Status)
	}

	// 3. Staged operation across remaining litter types
	for _, lt := range []domain.NeonatalLitterType{
		domain.NeonatalLitterSingletonOrphanColostrumDeprived,
		domain.NeonatalLitterTransitionalKitten2To4Wk,
		domain.NeonatalLitterMaternalAgalactiaRejected,
	} {
		stagedReq := domain.NeonatalEvaluationRequest{
			PetName:                   "Pippin",
			Species:                   "Dog - French Bulldog Neonate",
			WeightGrams:               185.0,
			LitterCount:               1,
			AgeDays:                   8,
			LitterType:                lt,
			NeonateRectalTempCelsius:  36.8,
			BloodGlucoseMgDl:          88.0,
			IncubatorHumidityPct:      58.0,
			SucklingReflexPresent:     true,
			WarmedBeforeFeedingGate:   true,
			SublingualDextroseApplied: true,
		}
		stagedOp, err := neonatalincubator.EvaluateNeonatalTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for litter type %s: %v", lt, err)
		}
		if stagedOp.Status != domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged {
			t.Errorf("expected INCUBATOR_WARMED_DEXTROSE_STAGED for %s, got %q", lt, stagedOp.Status)
		}
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(*domain.NeonatalEvaluationRequest)
	}{
		{"empty pet_name", func(r *domain.NeonatalEvaluationRequest) { r.PetName = "  " }},
		{"weight too low", func(r *domain.NeonatalEvaluationRequest) { r.WeightGrams = 25.0 }},
		{"weight too high", func(r *domain.NeonatalEvaluationRequest) { r.WeightGrams = 5500.0 }},
		{"litter count 0", func(r *domain.NeonatalEvaluationRequest) { r.LitterCount = 0 }},
		{"litter count > 16", func(r *domain.NeonatalEvaluationRequest) { r.LitterCount = 18 }},
		{"age < 0", func(r *domain.NeonatalEvaluationRequest) { r.AgeDays = -1 }},
		{"age > 42", func(r *domain.NeonatalEvaluationRequest) { r.AgeDays = 45 }},
		{"invalid litter_type", func(r *domain.NeonatalEvaluationRequest) { r.LitterType = "UNKNOWN_TYPE" }},
		{"rectal temp < 28", func(r *domain.NeonatalEvaluationRequest) { r.NeonateRectalTempCelsius = 26.5 }},
		{"rectal temp > 40.5", func(r *domain.NeonatalEvaluationRequest) { r.NeonateRectalTempCelsius = 41.2 }},
		{"glucose < 15", func(r *domain.NeonatalEvaluationRequest) { r.BloodGlucoseMgDl = 10.0 }},
		{"glucose > 300", func(r *domain.NeonatalEvaluationRequest) { r.BloodGlucoseMgDl = 320.0 }},
		{"humidity < 20", func(r *domain.NeonatalEvaluationRequest) { r.IncubatorHumidityPct = 15.0 }},
		{"humidity > 95", func(r *domain.NeonatalEvaluationRequest) { r.IncubatorHumidityPct = 98.0 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := critReq
			tc.mutate(&req)
			if _, err := neonatalincubator.EvaluateNeonatalTriage(req); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueNeonatalClearance(t *testing.T) {
	store := neonatalincubator.NewStore()
	ops := store.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := store.Get("NI-OP-101")
	if !ok {
		t.Fatalf("expected NI-OP-101 in store")
	}
	if op101.PetName != "Clover Litter" || op101.Status != domain.NeonatalTriageStatusFadingTriadHypothermicCritical {
		t.Errorf("unexpected NI-OP-101 initial state: %+v", op101)
	}

	// Issuing clearance before all 4 steps are completed must fail
	if _, err := store.IssueClearance("NI-OP-101"); err == nil {
		t.Fatalf("expected error issuing clearance on incomplete NI-OP-101")
	}

	// Complete step 1 -> now warming gate & sublingual dextrose are both done -> transitions to INCUBATOR_WARMED_DEXTROSE_STAGED
	updated, err := store.UpdateStep("NI-OP-101", neonatalincubator.StepIDIncubatorWarmingGate, true)
	if err != nil {
		t.Fatalf("unexpected error updating step: %v", err)
	}
	if updated.Status != domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged {
		t.Errorf("expected INCUBATOR_WARMED_DEXTROSE_STAGED after completing warming gate, got %q", updated.Status)
	}

	// Complete remaining 2 steps
	for _, stepID := range []string{
		neonatalincubator.StepIDOrogastricTubeGavage,
		neonatalincubator.StepIDAnogenitalStimWeigh,
	} {
		updated, err = store.UpdateStep("NI-OP-101", stepID, true)
		if err != nil {
			t.Fatalf("unexpected error completing %s: %v", stepID, err)
		}
	}

	// Issue clearance -> STABILIZED_NURSING_GAINING_SAFE with NEONATAL-SAFE-<12HEX>
	cleared, err := store.IssueClearance("NI-OP-101")
	if err != nil {
		t.Fatalf("unexpected error issuing clearance: %v", err)
	}
	if cleared.Status != domain.NeonatalTriageStatusStabilizedNursingGainingSafe {
		t.Errorf("expected STABILIZED_NURSING_GAINING_SAFE, got %q", cleared.Status)
	}
	if !strings.HasPrefix(cleared.ClearanceToken, "NEONATAL-SAFE-") || len(cleared.ClearanceToken) != len("NEONATAL-SAFE-")+12 {
		t.Errorf("unexpected clearance token format: %q", cleared.ClearanceToken)
	}

	// Verify deterministic token generation
	clearedAgain, err := store.IssueClearance("NI-OP-101")
	if err != nil {
		t.Fatalf("unexpected error re-issuing clearance: %v", err)
	}
	if clearedAgain.ClearanceToken != cleared.ClearanceToken {
		t.Errorf("expected deterministic token %q, got %q", cleared.ClearanceToken, clearedAgain.ClearanceToken)
	}

	// Unchecking a step clears token and reverts status
	reverted, err := store.UpdateStep("NI-OP-101", neonatalincubator.StepIDIncubatorWarmingGate, false)
	if err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if reverted.ClearanceToken != "" {
		t.Errorf("expected clearance token to be cleared when step unchecked, got %q", reverted.ClearanceToken)
	}

	// Error cases on UpdateProtocolStep and IssueNeonatalClearance
	if err := neonatalincubator.UpdateProtocolStep(nil, "STEP-INCUBATOR-WARMING-GATE", true); err == nil {
		t.Errorf("expected error for nil op in UpdateProtocolStep")
	}
	if err := neonatalincubator.UpdateProtocolStep(&op101, "", true); err == nil {
		t.Errorf("expected error for empty stepID in UpdateProtocolStep")
	}
	if err := neonatalincubator.UpdateProtocolStep(&op101, "NON-EXISTENT", true); err == nil {
		t.Errorf("expected error for unknown stepID in UpdateProtocolStep")
	}
	if err := neonatalincubator.IssueNeonatalClearance(nil); err == nil {
		t.Errorf("expected error for nil op in IssueNeonatalClearance")
	}
	if _, err := store.UpdateStep("UNKNOWN-OP", neonatalincubator.StepIDIncubatorWarmingGate, true); err == nil {
		t.Errorf("expected error for unknown op ID in store.UpdateStep")
	}
	if _, err := store.IssueClearance("UNKNOWN-OP"); err == nil {
		t.Errorf("expected error for unknown op ID in store.IssueClearance")
	}

	// EvaluateAndSave in store assigns NI-OP-104
	newOp, err := store.EvaluateAndSave(domain.NeonatalEvaluationRequest{
		PetName:                   "Mochi Litter",
		Species:                   "Cat - Siamese Neonates",
		WeightGrams:               110.0,
		LitterCount:               5,
		AgeDays:                   4,
		LitterType:                domain.NeonatalLitterMaternalAgalactiaRejected,
		NeonateRectalTempCelsius:  34.2,
		BloodGlucoseMgDl:          52.0,
		IncubatorHumidityPct:      63.0,
		SucklingReflexPresent:     false,
		WarmedBeforeFeedingGate:   false,
		SublingualDextroseApplied: false,
	})
	if err != nil {
		t.Fatalf("unexpected error in store.EvaluateAndSave: %v", err)
	}
	if newOp.ID != "NI-OP-104" {
		t.Errorf("expected NI-OP-104, got %q", newOp.ID)
	}
}
