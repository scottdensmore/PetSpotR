package confinedspace_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/confinedspace"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvaluateConfinedSpace_CulvertToxicGasAndInflowSurge(t *testing.T) {
	req := domain.ConfinedSpaceEvaluationRequest{
		OperationID:       "CS-OP-901",
		PetID:             "PET-901",
		PetName:           "Milo",
		Species:           "Feline",
		WeightKg:          4.1,
		SpaceType:         domain.ConfinedSpaceStormDrainCulvert,
		PipeDiameterCm:    32.0,
		DepthMeters:       3.8,
		OxygenPercent:     18.1,
		H2SPpm:            15.0,
		InflowRiskIndex:   85,
		ManholeSectorGrid: "MH-RAVENNA-42B",
		LeadTechCallsign:  "TECH-SUB-01",
		Notes:             "Wedged in corrugated culvert bend",
	}

	op, err := confinedspace.EvaluateConfinedSpace(req)
	if err != nil {
		t.Fatalf("unexpected error evaluating confined space: %v", err)
	}

	if op.Status != domain.ConfinedSpaceStatusAtmosphericHazardCritical {
		t.Errorf("expected ATMOSPHERIC_HAZARD_CRITICAL, got %s", op.Status)
	}
	if !op.ToxicGasAlert {
		t.Errorf("expected ToxicGasAlert=true for O2=18.1%% and H2S=15.0ppm")
	}
	if !op.FlashInflowAlert {
		t.Errorf("expected FlashInflowAlert=true for InflowRiskIndex=85 and 32cm culvert")
	}
	if !slices.Contains(op.RiskFlags, "EXTREME_ATMOSPHERIC_ASPHYXIATION_RISK") {
		t.Errorf("expected EXTREME_ATMOSPHERIC_ASPHYXIATION_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "CRITICAL_CULVERT_INFLOW_SURGE_RISK") {
		t.Errorf("expected CRITICAL_CULVERT_INFLOW_SURGE_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "NARROW_DIAMETER_WEDGING_HAZARD") {
		t.Errorf("expected NARROW_DIAMETER_WEDGING_HAZARD in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "DEEP_SHAFT_VERTICAL_HAUL_REQUIRED") {
		t.Errorf("expected DEEP_SHAFT_VERTICAL_HAUL_REQUIRED in %v", op.RiskFlags)
	}
	if op.HazardScore < 85 {
		t.Errorf("expected high HazardScore >= 85, got %d", op.HazardScore)
	}
	if len(op.ProtocolSteps) != 5 {
		t.Errorf("expected 5 protocol steps, got %d", len(op.ProtocolSteps))
	}
}

func TestEvaluateConfinedSpace_SpaceTypesAndAutoIDs(t *testing.T) {
	cases := []struct {
		name      string
		req       domain.ConfinedSpaceEvaluationRequest
		wantGas   bool
		wantSurge bool
	}{
		{
			name: "deep utility pipe triggers gas alert",
			req: domain.ConfinedSpaceEvaluationRequest{
				PetName:         "Scout",
				WeightKg:        9.5,
				SpaceType:       domain.ConfinedSpaceSubterraneanUtilityPipe,
				PipeDiameterCm:  55.0,
				DepthMeters:     3.4,
				OxygenPercent:   20.4,
				H2SPpm:          2.0,
				InflowRiskIndex: 25,
			},
			wantGas:   true,
			wantSurge: false,
		},
		{
			name: "crawlspace normal atmosphere rigged status",
			req: domain.ConfinedSpaceEvaluationRequest{
				PetName:         "Barnaby",
				Species:         "Canine",
				WeightKg:        12.0,
				SpaceType:       domain.ConfinedSpaceFoundationCrawlspace,
				PipeDiameterCm:  65.0,
				DepthMeters:     1.5,
				OxygenPercent:   20.8,
				H2SPpm:          0.5,
				InflowRiskIndex: 15,
			},
			wantGas:   false,
			wantSurge: false,
		},
		{
			name: "chimney flue shaft",
			req: domain.ConfinedSpaceEvaluationRequest{
				PetName:         "Cinder",
				Species:         "Feline",
				WeightKg:        3.8,
				SpaceType:       domain.ConfinedSpaceChimneyFlueShaft,
				PipeDiameterCm:  48.0,
				DepthMeters:     4.2,
				OxygenPercent:   20.9,
				H2SPpm:          0.0,
				InflowRiskIndex: 75,
			},
			wantGas:   false,
			wantSurge: true,
		},
		{
			name: "collapsed burrow trench",
			req: domain.ConfinedSpaceEvaluationRequest{
				PetName:         "Digby",
				Species:         "Canine",
				WeightKg:        8.0,
				SpaceType:       domain.ConfinedSpaceCollapsedBurrowTrench,
				PipeDiameterCm:  45.0,
				DepthMeters:     2.2,
				OxygenPercent:   20.5,
				H2SPpm:          1.0,
				InflowRiskIndex: 20,
			},
			wantGas:   false,
			wantSurge: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := confinedspace.EvaluateConfinedSpace(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.ToxicGasAlert != tc.wantGas {
				t.Errorf("expected ToxicGasAlert=%v, got %v", tc.wantGas, op.ToxicGasAlert)
			}
			if op.FlashInflowAlert != tc.wantSurge {
				t.Errorf("expected FlashInflowAlert=%v, got %v", tc.wantSurge, op.FlashInflowAlert)
			}
			if !strings.HasPrefix(op.OperationID, "CS-OP-") {
				t.Errorf("expected auto-generated CS-OP- prefix, got %q", op.OperationID)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueSubterraneanClearance(t *testing.T) {
	store := confinedspace.NewStore()
	ops := store.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := store.Get("CS-OP-101")
	if !ok {
		t.Fatalf("expected CS-OP-101 in store")
	}
	if op101.RiggingReadinessScore != 40 {
		t.Errorf("expected initial readiness 40, got %d", op101.RiggingReadinessScore)
	}

	// Issuing clearance before all required steps are complete should fail
	if _, err := store.IssueClearance("CS-OP-101", "TECH-SUB-01", "premature clearance"); err == nil {
		t.Errorf("expected error issuing clearance with incomplete steps")
	}

	// Uncheck a step and recheck all 5 steps
	if updated, err := store.UpdateStep("CS-OP-101", domain.ConfinedSpaceStepUpdateRequest{
		StepID:    "step-gas-purge",
		Completed: false,
	}); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	} else if updated.RiggingReadinessScore != 20 {
		t.Errorf("expected readiness 20 after unchecking step, got %d", updated.RiggingReadinessScore)
	}

	for _, stepID := range []string{
		"step-gas-purge",
		"step-inflow-dam",
		"step-borescope-loc",
		"step-oneway-gate",
		"step-haul-extract",
	} {
		updated, err := store.UpdateStep("CS-OP-101", domain.ConfinedSpaceStepUpdateRequest{
			StepID:    stepID,
			Completed: true,
			Notes:     "Completed " + stepID,
		})
		if err != nil {
			t.Fatalf("unexpected error completing step %s: %v", stepID, err)
		}
		op101 = updated
	}

	if op101.RiggingReadinessScore != 100 {
		t.Errorf("expected RiggingReadinessScore=100, got %d", op101.RiggingReadinessScore)
	}
	if op101.Status != domain.ConfinedSpaceStatusBorescopePipeGateRigged {
		t.Errorf("expected BORESCOPE_PIPE_GATE_RIGGED, got %s", op101.Status)
	}

	// Issue clearance now that all steps are completed
	cleared, err := store.IssueClearance("CS-OP-101", "TECH-SUB-01", "Pip extracted safely")
	if err != nil {
		t.Fatalf("unexpected error issuing subterranean clearance: %v", err)
	}
	if cleared.Status != domain.ConfinedSpaceStatusExtractedSubterraneanSafe {
		t.Errorf("expected EXTRACTED_SUBTERRANEAN_SAFE, got %s", cleared.Status)
	}
	if cleared.HazardScore != 10 {
		t.Errorf("expected HazardScore=10, got %d", cleared.HazardScore)
	}
	if cleared.ToxicGasAlert || cleared.FlashInflowAlert {
		t.Errorf("expected alerts cleared on extraction, got gas=%v inflow=%v", cleared.ToxicGasAlert, cleared.FlashInflowAlert)
	}
	if !strings.HasPrefix(cleared.ClearanceCertificateHash, "CONFINED-SAFE-") || len(cleared.ClearanceCertificateHash) != len("CONFINED-SAFE-")+12 {
		t.Errorf("expected CONFINED-SAFE-<12HEX> hash, got %q", cleared.ClearanceCertificateHash)
	}

	// EvaluateAndSave in store
	created, err := store.EvaluateAndSave(domain.ConfinedSpaceEvaluationRequest{
		PetName:         "Mochi",
		Species:         "Feline",
		WeightKg:        3.6,
		SpaceType:       domain.ConfinedSpaceStormDrainCulvert,
		PipeDiameterCm:  38.0,
		DepthMeters:     2.9,
		OxygenPercent:   18.9,
		H2SPpm:          11.2,
		InflowRiskIndex: 78,
	})
	if err != nil {
		t.Fatalf("unexpected error in EvaluateAndSave: %v", err)
	}
	if created.OperationID != "CS-OP-104" {
		t.Errorf("expected CS-OP-104, got %s", created.OperationID)
	}
}

func TestValidationAndErrorPaths(t *testing.T) {
	store := confinedspace.NewStore()

	if _, ok := store.Get("NON-EXISTENT"); ok {
		t.Errorf("expected false for unknown operation")
	}
	if _, err := store.UpdateStep("NON-EXISTENT", domain.ConfinedSpaceStepUpdateRequest{StepID: "step-gas-purge", Completed: true}); err == nil {
		t.Errorf("expected error for unknown operation on UpdateStep")
	}
	if _, err := store.IssueClearance("NON-EXISTENT", "TECH", ""); err == nil {
		t.Errorf("expected error for unknown operation on IssueClearance")
	}

	op102, _ := store.Get("CS-OP-102")
	if _, err := confinedspace.UpdateProtocolStep(op102, domain.ConfinedSpaceStepUpdateRequest{StepID: ""}); err == nil {
		t.Errorf("expected error for empty stepId")
	}
	if _, err := confinedspace.UpdateProtocolStep(op102, domain.ConfinedSpaceStepUpdateRequest{StepID: "invalid-step"}); err == nil {
		t.Errorf("expected error for unknown stepId")
	}
	if _, err := confinedspace.IssueSubterraneanClearance(domain.ConfinedSpaceOperation{}, "TECH", ""); err == nil {
		t.Errorf("expected error for empty operationId")
	}

	invalidReqs := []domain.ConfinedSpaceEvaluationRequest{
		{PetName: ""},
		{PetName: "Pip", SpaceType: "INVALID_SPACE", WeightKg: 4, PipeDiameterCm: 35, OxygenPercent: 20},
		{PetName: "Pip", SpaceType: domain.ConfinedSpaceStormDrainCulvert, WeightKg: 0, PipeDiameterCm: 35, OxygenPercent: 20},
		{PetName: "Pip", SpaceType: domain.ConfinedSpaceStormDrainCulvert, WeightKg: 4, PipeDiameterCm: 0, OxygenPercent: 20},
		{PetName: "Pip", SpaceType: domain.ConfinedSpaceStormDrainCulvert, WeightKg: 4, PipeDiameterCm: 35, DepthMeters: -1, OxygenPercent: 20},
		{PetName: "Pip", SpaceType: domain.ConfinedSpaceStormDrainCulvert, WeightKg: 4, PipeDiameterCm: 35, OxygenPercent: 0},
		{PetName: "Pip", SpaceType: domain.ConfinedSpaceStormDrainCulvert, WeightKg: 4, PipeDiameterCm: 35, OxygenPercent: 20, H2SPpm: -5},
		{PetName: "Pip", SpaceType: domain.ConfinedSpaceStormDrainCulvert, WeightKg: 4, PipeDiameterCm: 35, OxygenPercent: 20, InflowRiskIndex: 120},
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
	missingCertOp.Status = domain.ConfinedSpaceStatusExtractedSubterraneanSafe
	missingCertOp.ClearanceCertificateHash = ""
	if err := missingCertOp.Validate(); err == nil {
		t.Errorf("expected error for missing clearanceCertificateHash on EXTRACTED_SUBTERRANEAN_SAFE")
	}
}
