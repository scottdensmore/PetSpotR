package reptileectotherm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/reptileectotherm"
)

func TestEvaluateEctothermRescue_ColdTorporAndDesiccationAlerts(t *testing.T) {
	req := domain.EctothermEvaluationRequest{
		OperationID:           "RE-OP-901",
		PetID:                 "PET-901",
		PetName:               "Goliath",
		Species:               "Reptile",
		WeightKg:              22.0,
		TaxonType:             domain.EctothermTaxonChelonianTortoiseBurrow,
		SubstrateTempCelsius:  10.8,
		PreferredOptimumTempC: 31.5,
		RelativeHumidityPct:   26.0,
		HoursDormant:          52.0,
		BurrowDepthCm:         115.0,
		MicroHabitatGrid:      "BURROW-SOUTH-03A",
		LeadHerpTechCallsign:  "HERP-TECH-01",
		Notes:                 "Cold-stunned Sulcata tortoise in deep foundation burrow",
	}

	op, err := reptileectotherm.EvaluateEctothermRescue(req)
	if err != nil {
		t.Fatalf("unexpected error evaluating ectotherm rescue: %v", err)
	}

	if op.Status != domain.EctothermRescueStatusColdTorporBrumationCritical {
		t.Errorf("expected COLD_TORPOR_BRUMATION_CRITICAL, got %s", op.Status)
	}
	if !op.CriticalTorporAlert {
		t.Errorf("expected CriticalTorporAlert=true for SubstrateTempCelsius=10.8 and HoursDormant=52.0")
	}
	if !op.DesiccationDysecdysisAlert {
		t.Errorf("expected DesiccationDysecdysisAlert=true for RelativeHumidityPct=26.0")
	}
	if !slices.Contains(op.RiskFlags, "EXTREME_ECTOTHERMIC_COLD_STUN_TORPOR_RISK") {
		t.Errorf("expected EXTREME_ECTOTHERMIC_COLD_STUN_TORPOR_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "CRITICAL_HYPOVOLEMIC_DESICCATION_HAZARD") {
		t.Errorf("expected CRITICAL_HYPOVOLEMIC_DESICCATION_HAZARD in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "BURROW_ROOF_COLLAPSE_ENTRAPMENT_RISK") {
		t.Errorf("expected BURROW_ROOF_COLLAPSE_ENTRAPMENT_RISK in %v", op.RiskFlags)
	}
	if op.HazardScore < 85 {
		t.Errorf("expected high HazardScore >= 85, got %d", op.HazardScore)
	}
	if len(op.ProtocolSteps) != 5 {
		t.Errorf("expected 5 protocol steps, got %d", len(op.ProtocolSteps))
	}
}

func TestEvaluateEctothermRescue_TaxonTypesAndAutoIDs(t *testing.T) {
	cases := []struct {
		name            string
		req             domain.EctothermEvaluationRequest
		wantTorpor      bool
		wantDesiccation bool
	}{
		{
			name: "amphibian salamander axolotl triggers desiccation alert even at higher humidity",
			req: domain.EctothermEvaluationRequest{
				PetName:               "Mudkip",
				Species:               "Amphibian",
				WeightKg:              0.18,
				TaxonType:             domain.EctothermTaxonAmphibianSalamanderAxolotl,
				SubstrateTempCelsius:  16.0,
				PreferredOptimumTempC: 18.0,
				RelativeHumidityPct:   58.0,
				HoursDormant:          8.0,
				BurrowDepthCm:         15.0,
			},
			wantTorpor:      false,
			wantDesiccation: true,
		},
		{
			name: "boid python constrictor mild appliance crevice staged status",
			req: domain.EctothermEvaluationRequest{
				PetName:               "Monty",
				WeightKg:              2.1,
				TaxonType:             domain.EctothermTaxonBoidPythonConstrictor,
				SubstrateTempCelsius:  23.0,
				PreferredOptimumTempC: 30.5,
				RelativeHumidityPct:   64.0,
				HoursDormant:          12.0,
				BurrowDepthCm:         20.0,
			},
			wantTorpor:      false,
			wantDesiccation: false,
		},
		{
			name: "agamid iguanid arboreal mild basking conditions",
			req: domain.EctothermEvaluationRequest{
				PetName:               "helios",
				Species:               "Reptile",
				WeightKg:              0.52,
				TaxonType:             domain.EctothermTaxonAgamidIguanidArboreal,
				SubstrateTempCelsius:  25.0,
				PreferredOptimumTempC: 33.0,
				RelativeHumidityPct:   48.0,
				HoursDormant:          6.0,
				BurrowDepthCm:         0.0,
			},
			wantTorpor:      false,
			wantDesiccation: false,
		},
		{
			name: "colubrid corn king snake prolonged dormancy triggers torpor alert",
			req: domain.EctothermEvaluationRequest{
				PetName:               "Copper",
				Species:               "Reptile",
				WeightKg:              0.65,
				TaxonType:             domain.EctothermTaxonColubridCornKingSnake,
				SubstrateTempCelsius:  19.0,
				PreferredOptimumTempC: 28.0,
				RelativeHumidityPct:   50.0,
				HoursDormant:          40.0,
				BurrowDepthCm:         30.0,
			},
			wantTorpor:      true,
			wantDesiccation: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := reptileectotherm.EvaluateEctothermRescue(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.CriticalTorporAlert != tc.wantTorpor {
				t.Errorf("expected CriticalTorporAlert=%v, got %v", tc.wantTorpor, op.CriticalTorporAlert)
			}
			if op.DesiccationDysecdysisAlert != tc.wantDesiccation {
				t.Errorf("expected DesiccationDysecdysisAlert=%v, got %v", tc.wantDesiccation, op.DesiccationDysecdysisAlert)
			}
			if !strings.HasPrefix(op.OperationID, "RE-OP-") {
				t.Errorf("expected auto-generated RE-OP- prefix, got %q", op.OperationID)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueEctothermClearance(t *testing.T) {
	store := reptileectotherm.NewStore()
	ops := store.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := store.Get("RE-OP-101")
	if !ok {
		t.Fatalf("expected RE-OP-101 in store")
	}
	if op101.ThermalReadinessScore != 40 {
		t.Errorf("expected initial readiness 40, got %d", op101.ThermalReadinessScore)
	}

	// Issuing clearance before all required steps are complete should fail
	if _, err := store.IssueClearance("RE-OP-101", "HERP-TECH-01", "premature clearance"); err == nil {
		t.Errorf("expected error issuing clearance with incomplete steps")
	}

	// Uncheck a step and recheck all 5 steps
	if updated, err := store.UpdateStep("RE-OP-101", domain.EctothermStepUpdateRequest{
		StepID:    "step-endoscope-crevice-sweep",
		Completed: false,
	}); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	} else if updated.ThermalReadinessScore != 20 {
		t.Errorf("expected readiness 20 after unchecking step, got %d", updated.ThermalReadinessScore)
	}

	for _, stepID := range []string{
		"step-endoscope-crevice-sweep",
		"step-burrow-shoring-probe",
		"step-ceramic-ir-basking-lure",
		"step-humidity-hydration-soak",
		"step-gradual-coelomic-rewarm",
	} {
		updated, err := store.UpdateStep("RE-OP-101", domain.EctothermStepUpdateRequest{
			StepID:    stepID,
			Completed: true,
			Notes:     "Completed " + stepID,
		})
		if err != nil {
			t.Fatalf("unexpected error completing step %s: %v", stepID, err)
		}
		op101 = updated
	}

	if op101.ThermalReadinessScore != 100 {
		t.Errorf("expected ThermalReadinessScore=100, got %d", op101.ThermalReadinessScore)
	}
	if op101.Status != domain.EctothermRescueStatusBaskingLureCreviceStaged {
		t.Errorf("expected BASKING_LURE_CREVICE_STAGED, got %s", op101.Status)
	}

	// Issue clearance now that all steps are completed
	cleared, err := store.IssueClearance("RE-OP-101", "HERP-TECH-01", "Tank extracted from burrow and rewarmed across POTZ gradient")
	if err != nil {
		t.Fatalf("unexpected error issuing ectotherm clearance: %v", err)
	}
	if cleared.Status != domain.EctothermRescueStatusRecoveredThermalGradientSafe {
		t.Errorf("expected RECOVERED_THERMAL_GRADIENT_SAFE, got %s", cleared.Status)
	}
	if cleared.HazardScore != 10 {
		t.Errorf("expected HazardScore=10, got %d", cleared.HazardScore)
	}
	if cleared.CriticalTorporAlert || cleared.DesiccationDysecdysisAlert {
		t.Errorf("expected alerts cleared on recovery, got torpor=%v desiccation=%v", cleared.CriticalTorporAlert, cleared.DesiccationDysecdysisAlert)
	}
	if !strings.HasPrefix(cleared.ClearanceCertificateHash, "ECTOTHERM-SAFE-") || len(cleared.ClearanceCertificateHash) != len("ECTOTHERM-SAFE-")+12 {
		t.Errorf("expected ECTOTHERM-SAFE-<12HEX> hash, got %q", cleared.ClearanceCertificateHash)
	}

	// EvaluateAndSave in store
	created, err := store.EvaluateAndSave(domain.EctothermEvaluationRequest{
		PetName:               "Bramble",
		Species:               "Reptile",
		WeightKg:              19.5,
		TaxonType:             domain.EctothermTaxonChelonianTortoiseBurrow,
		SubstrateTempCelsius:  11.0,
		PreferredOptimumTempC: 31.0,
		RelativeHumidityPct:   29.0,
		HoursDormant:          50.0,
		BurrowDepthCm:         105.0,
	})
	if err != nil {
		t.Fatalf("unexpected error in EvaluateAndSave: %v", err)
	}
	if created.OperationID != "RE-OP-104" {
		t.Errorf("expected RE-OP-104, got %s", created.OperationID)
	}
}

func TestValidationAndErrorPaths(t *testing.T) {
	store := reptileectotherm.NewStore()

	if _, ok := store.Get("NON-EXISTENT"); ok {
		t.Errorf("expected false for unknown operation")
	}
	if _, err := store.UpdateStep("NON-EXISTENT", domain.EctothermStepUpdateRequest{StepID: "step-endoscope-crevice-sweep", Completed: true}); err == nil {
		t.Errorf("expected error for unknown operation on UpdateStep")
	}
	if _, err := store.IssueClearance("NON-EXISTENT", "HERP", ""); err == nil {
		t.Errorf("expected error for unknown operation on IssueClearance")
	}

	op102, _ := store.Get("RE-OP-102")
	if _, err := reptileectotherm.UpdateProtocolStep(op102, domain.EctothermStepUpdateRequest{StepID: ""}); err == nil {
		t.Errorf("expected error for empty stepId")
	}
	if _, err := reptileectotherm.UpdateProtocolStep(op102, domain.EctothermStepUpdateRequest{StepID: "invalid-step"}); err == nil {
		t.Errorf("expected error for unknown stepId")
	}
	if _, err := reptileectotherm.IssueEctothermClearance(domain.EctothermRescueOperation{}, "HERP", ""); err == nil {
		t.Errorf("expected error for empty operationId")
	}

	invalidReqs := []domain.EctothermEvaluationRequest{
		{PetName: ""},
		{PetName: "Tank", TaxonType: "INVALID_TAXON", WeightKg: 18, SubstrateTempCelsius: 12, PreferredOptimumTempC: 31, RelativeHumidityPct: 30},
		{PetName: "Tank", TaxonType: domain.EctothermTaxonChelonianTortoiseBurrow, WeightKg: 0, SubstrateTempCelsius: 12, PreferredOptimumTempC: 31, RelativeHumidityPct: 30},
		{PetName: "Tank", TaxonType: domain.EctothermTaxonChelonianTortoiseBurrow, WeightKg: 18, SubstrateTempCelsius: -25, PreferredOptimumTempC: 31, RelativeHumidityPct: 30},
		{PetName: "Tank", TaxonType: domain.EctothermTaxonChelonianTortoiseBurrow, WeightKg: 18, SubstrateTempCelsius: 12, PreferredOptimumTempC: 5, RelativeHumidityPct: 30},
		{PetName: "Tank", TaxonType: domain.EctothermTaxonChelonianTortoiseBurrow, WeightKg: 18, SubstrateTempCelsius: 12, PreferredOptimumTempC: 31, RelativeHumidityPct: 120},
		{PetName: "Tank", TaxonType: domain.EctothermTaxonChelonianTortoiseBurrow, WeightKg: 18, SubstrateTempCelsius: 12, PreferredOptimumTempC: 31, RelativeHumidityPct: 30, HoursDormant: -2},
		{PetName: "Tank", TaxonType: domain.EctothermTaxonChelonianTortoiseBurrow, WeightKg: 18, SubstrateTempCelsius: 12, PreferredOptimumTempC: 31, RelativeHumidityPct: 30, BurrowDepthCm: -10},
	}

	for i, req := range invalidReqs {
		if _, err := store.EvaluateAndSave(req); err == nil {
			t.Errorf("case %d: expected error for invalid evaluation request %+v", i, req)
		}
	}

	// Test domain Validate edge cases
	validOp := op102
	if err := validOp.Validate(); err != nil {
		t.Errorf("expected valid op102, got %v", err)
	}
	badStatusOp := op102
	badStatusOp.Status = "INVALID_STATUS"
	if err := badStatusOp.Validate(); err == nil {
		t.Errorf("expected error for invalid status")
	}
	missingCertOp := op102
	missingCertOp.Status = domain.EctothermRescueStatusRecoveredThermalGradientSafe
	missingCertOp.ClearanceCertificateHash = ""
	if err := missingCertOp.Validate(); err == nil {
		t.Errorf("expected error for missing clearanceCertificateHash on RECOVERED_THERMAL_GRADIENT_SAFE")
	}
}
