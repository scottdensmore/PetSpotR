package usarcollapse_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/usarcollapse"
)

func TestEvaluateUSARCollapse_PancakeSlabSecondaryCollapseAndCrushRhabdo(t *testing.T) {
	req := domain.USAREvaluationRequest{
		OperationID:            "USAR-OP-901",
		PetID:                  "PET-901",
		PetName:                "Brutus",
		Species:                "Canine",
		WeightKg:               33.0,
		VoidType:               domain.USARVoidPancakeFloorSlabVoid,
		RubbleOverburdenMeters: 2.8,
		AftershockPGA:          0.34,
		SilicaDustMgM3:         8.5,
		HoursEntrapped:         19.0,
		StructuralStabilityIdx: 22,
		CollapseStructureGrid:  "SECTOR-BRAVO-04",
		LeadUSARSpecCallsign:   "USAR-TF1-SPEC-01",
		Notes:                  "Entrapped under pancake floor slab",
	}

	op, err := usarcollapse.EvaluateUSARCollapse(req)
	if err != nil {
		t.Fatalf("unexpected error evaluating USAR collapse: %v", err)
	}

	if op.Status != domain.USARRescueStatusSecondaryCollapseCrushCritical {
		t.Errorf("expected SECONDARY_COLLAPSE_CRUSH_CRITICAL, got %s", op.Status)
	}
	if !op.SecondaryCollapseAlert {
		t.Errorf("expected SecondaryCollapseAlert=true for StructuralStabilityIdx=22 and AftershockPGA=0.34")
	}
	if !op.CrushRhabdoAlert {
		t.Errorf("expected CrushRhabdoAlert=true for HoursEntrapped=19.0 and RubbleOverburdenMeters=2.8")
	}
	if !slices.Contains(op.RiskFlags, "EXTREME_UNSHORED_SECONDARY_COLLAPSE_RISK") {
		t.Errorf("expected EXTREME_UNSHORED_SECONDARY_COLLAPSE_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "CRITICAL_CRUSH_SYNDROME_HYPERKALEMIA_RISK") {
		t.Errorf("expected CRITICAL_CRUSH_SYNDROME_HYPERKALEMIA_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "UTILITY_GAS_RUPTURE_SPARK_HAZARD") {
		t.Errorf("expected UTILITY_GAS_RUPTURE_SPARK_HAZARD in %v", op.RiskFlags)
	}
	if op.HazardScore < 85 {
		t.Errorf("expected high HazardScore >= 85, got %d", op.HazardScore)
	}
	if len(op.ProtocolSteps) != 5 {
		t.Errorf("expected 5 protocol steps, got %d", len(op.ProtocolSteps))
	}
}

func TestEvaluateUSARCollapse_VoidTypesAndAutoIDs(t *testing.T) {
	cases := []struct {
		name         string
		req          domain.USAREvaluationRequest
		wantCollapse bool
		wantCrush    bool
	}{
		{
			name: "cantilever beam overhang triggers secondary collapse alert even with high stability index",
			req: domain.USAREvaluationRequest{
				PetName:                "Rex",
				WeightKg:               24.0,
				VoidType:               domain.USARVoidCantileverBeamOverhang,
				RubbleOverburdenMeters: 1.0,
				AftershockPGA:          0.10,
				SilicaDustMgM3:         2.0,
				HoursEntrapped:         4.0,
				StructuralStabilityIdx: 70,
			},
			wantCollapse: true,
			wantCrush:    false,
		},
		{
			name: "lean-to timber truss cavity mild conditions staged status",
			req: domain.USAREvaluationRequest{
				PetName:                "Cleo",
				Species:                "Feline",
				WeightKg:               4.2,
				VoidType:               domain.USARVoidLeanToTimberTrussCavity,
				RubbleOverburdenMeters: 0.9,
				AftershockPGA:          0.08,
				SilicaDustMgM3:         1.5,
				HoursEntrapped:         4.5,
				StructuralStabilityIdx: 78,
			},
			wantCollapse: false,
			wantCrush:    false,
		},
		{
			name: "sub-basement utility vault high silica dust triggers crush/rhabdo alert",
			req: domain.USAREvaluationRequest{
				PetName:                "bandit",
				Species:                "Canine",
				WeightKg:               18.0,
				VoidType:               domain.USARVoidSubBasementUtilityVault,
				RubbleOverburdenMeters: 1.5,
				AftershockPGA:          0.15,
				SilicaDustMgM3:         6.5,
				HoursEntrapped:         8.0,
				StructuralStabilityIdx: 60,
			},
			wantCollapse: false,
			wantCrush:    true,
		},
		{
			name: "v-shape masonry rubble mild conditions",
			req: domain.USAREvaluationRequest{
				PetName:                "Bella",
				Species:                "Canine",
				WeightKg:               15.0,
				VoidType:               domain.USARVoidVShapeMasonryRubble,
				RubbleOverburdenMeters: 0.6,
				AftershockPGA:          0.05,
				SilicaDustMgM3:         0.8,
				HoursEntrapped:         3.0,
				StructuralStabilityIdx: 85,
			},
			wantCollapse: false,
			wantCrush:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := usarcollapse.EvaluateUSARCollapse(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.SecondaryCollapseAlert != tc.wantCollapse {
				t.Errorf("expected SecondaryCollapseAlert=%v, got %v", tc.wantCollapse, op.SecondaryCollapseAlert)
			}
			if op.CrushRhabdoAlert != tc.wantCrush {
				t.Errorf("expected CrushRhabdoAlert=%v, got %v", tc.wantCrush, op.CrushRhabdoAlert)
			}
			if !strings.HasPrefix(op.OperationID, "USAR-OP-") {
				t.Errorf("expected auto-generated USAR-OP- prefix, got %q", op.OperationID)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueUSARClearance(t *testing.T) {
	store := usarcollapse.NewStore()
	ops := store.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := store.Get("USAR-OP-101")
	if !ok {
		t.Fatalf("expected USAR-OP-101 in store")
	}
	if op101.ShoringReadinessScore != 40 {
		t.Errorf("expected initial readiness 40, got %d", op101.ShoringReadinessScore)
	}

	// Issuing clearance before all required steps are complete should fail
	if _, err := store.IssueClearance("USAR-OP-101", "USAR-TF1-SPEC-01", "premature clearance"); err == nil {
		t.Errorf("expected error issuing clearance with incomplete steps")
	}

	// Uncheck a step and recheck all 5 steps
	if updated, err := store.UpdateStep("USAR-OP-101", domain.USARStepUpdateRequest{
		StepID:    "step-delsar-triangulation",
		Completed: false,
	}); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	} else if updated.ShoringReadinessScore != 20 {
		t.Errorf("expected readiness 20 after unchecking step, got %d", updated.ShoringReadinessScore)
	}

	for _, stepID := range []string{
		"step-delsar-triangulation",
		"step-utility-lockout-monitor",
		"step-box-cribbing-shoring",
		"step-rotary-core-breach",
		"step-crush-rhabdo-decon",
	} {
		updated, err := store.UpdateStep("USAR-OP-101", domain.USARStepUpdateRequest{
			StepID:    stepID,
			Completed: true,
			Notes:     "Completed " + stepID,
		})
		if err != nil {
			t.Fatalf("unexpected error completing step %s: %v", stepID, err)
		}
		op101 = updated
	}

	if op101.ShoringReadinessScore != 100 {
		t.Errorf("expected ShoringReadinessScore=100, got %d", op101.ShoringReadinessScore)
	}
	if op101.Status != domain.USARRescueStatusShoringCribbingDelsarStaged {
		t.Errorf("expected SHORING_CRIBBING_DELSAR_STAGED, got %s", op101.Status)
	}

	// Issue clearance now that all steps are completed
	cleared, err := store.IssueClearance("USAR-OP-101", "USAR-TF1-SPEC-01", "Diesel extracted and decontaminated safely")
	if err != nil {
		t.Fatalf("unexpected error issuing USAR clearance: %v", err)
	}
	if cleared.Status != domain.USARRescueStatusExtractedUSARDeconSafe {
		t.Errorf("expected EXTRACTED_USAR_DECON_SAFE, got %s", cleared.Status)
	}
	if cleared.HazardScore != 10 {
		t.Errorf("expected HazardScore=10, got %d", cleared.HazardScore)
	}
	if cleared.SecondaryCollapseAlert || cleared.CrushRhabdoAlert {
		t.Errorf("expected alerts cleared on extraction, got collapse=%v crush=%v", cleared.SecondaryCollapseAlert, cleared.CrushRhabdoAlert)
	}
	if !strings.HasPrefix(cleared.ClearanceCertificateHash, "USAR-SAFE-") || len(cleared.ClearanceCertificateHash) != len("USAR-SAFE-")+12 {
		t.Errorf("expected USAR-SAFE-<12HEX> hash, got %q", cleared.ClearanceCertificateHash)
	}

	// EvaluateAndSave in store
	created, err := store.EvaluateAndSave(domain.USAREvaluationRequest{
		PetName:                "Titan",
		Species:                "Canine",
		WeightKg:               30.0,
		VoidType:               domain.USARVoidPancakeFloorSlabVoid,
		RubbleOverburdenMeters: 2.5,
		AftershockPGA:          0.32,
		SilicaDustMgM3:         7.8,
		HoursEntrapped:         15.0,
		StructuralStabilityIdx: 25,
	})
	if err != nil {
		t.Fatalf("unexpected error in EvaluateAndSave: %v", err)
	}
	if created.OperationID != "USAR-OP-104" {
		t.Errorf("expected USAR-OP-104, got %s", created.OperationID)
	}
}

func TestValidationAndErrorPaths(t *testing.T) {
	store := usarcollapse.NewStore()

	if _, ok := store.Get("NON-EXISTENT"); ok {
		t.Errorf("expected false for unknown operation")
	}
	if _, err := store.UpdateStep("NON-EXISTENT", domain.USARStepUpdateRequest{StepID: "step-delsar-triangulation", Completed: true}); err == nil {
		t.Errorf("expected error for unknown operation on UpdateStep")
	}
	if _, err := store.IssueClearance("NON-EXISTENT", "USAR-SPEC", ""); err == nil {
		t.Errorf("expected error for unknown operation on IssueClearance")
	}

	op102, _ := store.Get("USAR-OP-102")
	if _, err := usarcollapse.UpdateProtocolStep(op102, domain.USARStepUpdateRequest{StepID: ""}); err == nil {
		t.Errorf("expected error for empty stepId")
	}
	if _, err := usarcollapse.UpdateProtocolStep(op102, domain.USARStepUpdateRequest{StepID: "invalid-step"}); err == nil {
		t.Errorf("expected error for unknown stepId")
	}
	if _, err := usarcollapse.IssueUSARClearance(domain.USARRescueOperation{}, "USAR-SPEC", ""); err == nil {
		t.Errorf("expected error for empty operationId")
	}

	invalidReqs := []domain.USAREvaluationRequest{
		{PetName: ""},
		{PetName: "Diesel", VoidType: "INVALID_VOID", WeightKg: 25, StructuralStabilityIdx: 50},
		{PetName: "Diesel", VoidType: domain.USARVoidPancakeFloorSlabVoid, WeightKg: 0, StructuralStabilityIdx: 50},
		{PetName: "Diesel", VoidType: domain.USARVoidPancakeFloorSlabVoid, WeightKg: 25, RubbleOverburdenMeters: -1, StructuralStabilityIdx: 50},
		{PetName: "Diesel", VoidType: domain.USARVoidPancakeFloorSlabVoid, WeightKg: 25, AftershockPGA: -0.5, StructuralStabilityIdx: 50},
		{PetName: "Diesel", VoidType: domain.USARVoidPancakeFloorSlabVoid, WeightKg: 25, SilicaDustMgM3: -2, StructuralStabilityIdx: 50},
		{PetName: "Diesel", VoidType: domain.USARVoidPancakeFloorSlabVoid, WeightKg: 25, HoursEntrapped: -1, StructuralStabilityIdx: 50},
		{PetName: "Diesel", VoidType: domain.USARVoidPancakeFloorSlabVoid, WeightKg: 25, StructuralStabilityIdx: 120},
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
	missingCertOp.Status = domain.USARRescueStatusExtractedUSARDeconSafe
	missingCertOp.ClearanceCertificateHash = ""
	if err := missingCertOp.Validate(); err == nil {
		t.Errorf("expected error for missing clearanceCertificateHash on EXTRACTED_USAR_DECON_SAFE")
	}
}
