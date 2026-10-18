package avalanchewinter_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/avalanchewinter"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvaluateWinterRescue_AvalancheBurialAndAfterdropHypothermia(t *testing.T) {
	req := domain.WinterEvaluationRequest{
		OperationID:           "AW-OP-901",
		PetID:                 "PET-901",
		PetName:               "Kodiak",
		Species:               "Canine",
		WeightKg:              31.0,
		TerrainType:           domain.WinterTerrainAvalancheDebrisChute,
		BurialDepthCm:         90.0,
		WindChillCelsius:      -24.0,
		CoreTempCelsius:       33.5,
		MinutesBuried:         28,
		AvalancheDangerScale:  4,
		SnowpackSectorGrid:    "CHUTE-NORTH-04A",
		LeadSkiPatrolCallsign: "PATROL-ALPINE-01",
		Notes:                 "Buried in wind-slab debris toe",
	}

	op, err := avalanchewinter.EvaluateWinterRescue(req)
	if err != nil {
		t.Fatalf("unexpected error evaluating winter rescue: %v", err)
	}

	if op.Status != domain.WinterRescueStatusBurialHypothermiaCritical {
		t.Errorf("expected BURIAL_HYPOTHERMIA_CRITICAL, got %s", op.Status)
	}
	if !op.AsphyxiaAirPocketAlert {
		t.Errorf("expected AsphyxiaAirPocketAlert=true for BurialDepthCm=90.0 and MinutesBuried=28")
	}
	if !op.SevereFrostbiteAfterdropAlert {
		t.Errorf("expected SevereFrostbiteAfterdropAlert=true for CoreTempCelsius=33.5 and WindChillCelsius=-24.0")
	}
	if !slices.Contains(op.RiskFlags, "EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK") {
		t.Errorf("expected EXTREME_SNOWPACK_ASPHYXIA_AIR_POCKET_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "CRITICAL_HYPOTHERMIC_AFTERDROP_ARRHYTHMIA_RISK") {
		t.Errorf("expected CRITICAL_HYPOTHERMIC_AFTERDROP_ARRHYTHMIA_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "SECONDARY_SLAB_TRIGGER_HAZARD") {
		t.Errorf("expected SECONDARY_SLAB_TRIGGER_HAZARD in %v", op.RiskFlags)
	}
	if op.HazardScore < 85 {
		t.Errorf("expected high HazardScore >= 85, got %d", op.HazardScore)
	}
	if len(op.ProtocolSteps) != 5 {
		t.Errorf("expected 5 protocol steps, got %d", len(op.ProtocolSteps))
	}
}

func TestEvaluateWinterRescue_TerrainTypesAndAutoIDs(t *testing.T) {
	cases := []struct {
		name          string
		req           domain.WinterEvaluationRequest
		wantAsphyxia  bool
		wantAfterdrop bool
	}{
		{
			name: "tree well powder pit triggers asphyxia alert even at shallow depth",
			req: domain.WinterEvaluationRequest{
				PetName:              "Timber",
				WeightKg:             19.5,
				TerrainType:          domain.WinterTerrainTreeWellPowderPit,
				BurialDepthCm:        35.0,
				WindChillCelsius:     -8.0,
				CoreTempCelsius:      36.5,
				MinutesBuried:        8,
				AvalancheDangerScale: 3,
			},
			wantAsphyxia:  true,
			wantAfterdrop: false,
		},
		{
			name: "blizzard snowdrift trench mild conditions staged status",
			req: domain.WinterEvaluationRequest{
				PetName:              "Lumi",
				Species:              "Feline",
				WeightKg:             4.5,
				TerrainType:          domain.WinterTerrainBlizzardSnowdriftTrench,
				BurialDepthCm:        20.0,
				WindChillCelsius:     -6.0,
				CoreTempCelsius:      37.2,
				MinutesBuried:        5,
				AvalancheDangerScale: 2,
			},
			wantAsphyxia:  false,
			wantAfterdrop: false,
		},
		{
			name: "glacial crevasse ledge extreme cold",
			req: domain.WinterEvaluationRequest{
				PetName:              "Glacier",
				Species:              "Canine",
				WeightKg:             24.0,
				TerrainType:          domain.WinterTerrainGlacialCrevasseLedge,
				BurialDepthCm:        30.0,
				WindChillCelsius:     -18.0,
				CoreTempCelsius:      34.6,
				MinutesBuried:        12,
				AvalancheDangerScale: 3,
			},
			wantAsphyxia:  false,
			wantAfterdrop: true,
		},
		{
			name: "subzero frozen trail",
			req: domain.WinterEvaluationRequest{
				PetName:              "Sitka",
				Species:              "Canine",
				WeightKg:             16.0,
				TerrainType:          domain.WinterTerrainSubzeroFrozenTrail,
				BurialDepthCm:        10.0,
				WindChillCelsius:     -9.0,
				CoreTempCelsius:      37.0,
				MinutesBuried:        0,
				AvalancheDangerScale: 1,
			},
			wantAsphyxia:  false,
			wantAfterdrop: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := avalanchewinter.EvaluateWinterRescue(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.AsphyxiaAirPocketAlert != tc.wantAsphyxia {
				t.Errorf("expected AsphyxiaAirPocketAlert=%v, got %v", tc.wantAsphyxia, op.AsphyxiaAirPocketAlert)
			}
			if op.SevereFrostbiteAfterdropAlert != tc.wantAfterdrop {
				t.Errorf("expected SevereFrostbiteAfterdropAlert=%v, got %v", tc.wantAfterdrop, op.SevereFrostbiteAfterdropAlert)
			}
			if !strings.HasPrefix(op.OperationID, "AW-OP-") {
				t.Errorf("expected auto-generated AW-OP- prefix, got %q", op.OperationID)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueWinterClearance(t *testing.T) {
	store := avalanchewinter.NewStore()
	ops := store.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := store.Get("AW-OP-101")
	if !ok {
		t.Fatalf("expected AW-OP-101 in store")
	}
	if op101.RewarmingReadinessScore != 40 {
		t.Errorf("expected initial readiness 40, got %d", op101.RewarmingReadinessScore)
	}

	// Issuing clearance before all required steps are complete should fail
	if _, err := store.IssueClearance("AW-OP-101", "PATROL-ALPINE-01", "premature clearance"); err == nil {
		t.Errorf("expected error issuing clearance with incomplete steps")
	}

	// Uncheck a step and recheck all 5 steps
	if updated, err := store.UpdateStep("AW-OP-101", domain.WinterStepUpdateRequest{
		StepID:    "step-beacon-pinpoint",
		Completed: false,
	}); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	} else if updated.RewarmingReadinessScore != 20 {
		t.Errorf("expected readiness 20 after unchecking step, got %d", updated.RewarmingReadinessScore)
	}

	for _, stepID := range []string{
		"step-beacon-pinpoint",
		"step-probe-spiral",
		"step-v-conveyor-trench",
		"step-horizontal-afterdrop-lift",
		"step-active-core-rewarm",
	} {
		updated, err := store.UpdateStep("AW-OP-101", domain.WinterStepUpdateRequest{
			StepID:    stepID,
			Completed: true,
			Notes:     "Completed " + stepID,
		})
		if err != nil {
			t.Fatalf("unexpected error completing step %s: %v", stepID, err)
		}
		op101 = updated
	}

	if op101.RewarmingReadinessScore != 100 {
		t.Errorf("expected RewarmingReadinessScore=100, got %d", op101.RewarmingReadinessScore)
	}
	if op101.Status != domain.WinterRescueStatusProbeTrenchRewarmingStaged {
		t.Errorf("expected PROBE_TRENCH_REWARMING_STAGED, got %s", op101.Status)
	}

	// Issue clearance now that all steps are completed
	cleared, err := store.IssueClearance("AW-OP-101", "PATROL-ALPINE-01", "Yukon extracted and rewarmed safely")
	if err != nil {
		t.Fatalf("unexpected error issuing winter clearance: %v", err)
	}
	if cleared.Status != domain.WinterRescueStatusExtractedNormothermicSafe {
		t.Errorf("expected EXTRACTED_NORMOTHERMIC_SAFE, got %s", cleared.Status)
	}
	if cleared.HazardScore != 10 {
		t.Errorf("expected HazardScore=10, got %d", cleared.HazardScore)
	}
	if cleared.AsphyxiaAirPocketAlert || cleared.SevereFrostbiteAfterdropAlert {
		t.Errorf("expected alerts cleared on extraction, got asphyxia=%v afterdrop=%v", cleared.AsphyxiaAirPocketAlert, cleared.SevereFrostbiteAfterdropAlert)
	}
	if !strings.HasPrefix(cleared.ClearanceCertificateHash, "WINTER-SAFE-") || len(cleared.ClearanceCertificateHash) != len("WINTER-SAFE-")+12 {
		t.Errorf("expected WINTER-SAFE-<12HEX> hash, got %q", cleared.ClearanceCertificateHash)
	}

	// EvaluateAndSave in store
	created, err := store.EvaluateAndSave(domain.WinterEvaluationRequest{
		PetName:              "Balto",
		Species:              "Canine",
		WeightKg:             29.0,
		TerrainType:          domain.WinterTerrainAvalancheDebrisChute,
		BurialDepthCm:        80.0,
		WindChillCelsius:     -20.0,
		CoreTempCelsius:      34.0,
		MinutesBuried:        22,
		AvalancheDangerScale: 4,
	})
	if err != nil {
		t.Fatalf("unexpected error in EvaluateAndSave: %v", err)
	}
	if created.OperationID != "AW-OP-104" {
		t.Errorf("expected AW-OP-104, got %s", created.OperationID)
	}
}

func TestValidationAndErrorPaths(t *testing.T) {
	store := avalanchewinter.NewStore()

	if _, ok := store.Get("NON-EXISTENT"); ok {
		t.Errorf("expected false for unknown operation")
	}
	if _, err := store.UpdateStep("NON-EXISTENT", domain.WinterStepUpdateRequest{StepID: "step-beacon-pinpoint", Completed: true}); err == nil {
		t.Errorf("expected error for unknown operation on UpdateStep")
	}
	if _, err := store.IssueClearance("NON-EXISTENT", "PATROL", ""); err == nil {
		t.Errorf("expected error for unknown operation on IssueClearance")
	}

	op102, _ := store.Get("AW-OP-102")
	if _, err := avalanchewinter.UpdateProtocolStep(op102, domain.WinterStepUpdateRequest{StepID: ""}); err == nil {
		t.Errorf("expected error for empty stepId")
	}
	if _, err := avalanchewinter.UpdateProtocolStep(op102, domain.WinterStepUpdateRequest{StepID: "invalid-step"}); err == nil {
		t.Errorf("expected error for unknown stepId")
	}
	if _, err := avalanchewinter.IssueWinterClearance(domain.WinterRescueOperation{}, "PATROL", ""); err == nil {
		t.Errorf("expected error for empty operationId")
	}

	invalidReqs := []domain.WinterEvaluationRequest{
		{PetName: ""},
		{PetName: "Yukon", TerrainType: "INVALID_TERRAIN", WeightKg: 25, CoreTempCelsius: 36, AvalancheDangerScale: 3},
		{PetName: "Yukon", TerrainType: domain.WinterTerrainAvalancheDebrisChute, WeightKg: 0, CoreTempCelsius: 36, AvalancheDangerScale: 3},
		{PetName: "Yukon", TerrainType: domain.WinterTerrainAvalancheDebrisChute, WeightKg: 25, BurialDepthCm: -5, CoreTempCelsius: 36, AvalancheDangerScale: 3},
		{PetName: "Yukon", TerrainType: domain.WinterTerrainAvalancheDebrisChute, WeightKg: 25, WindChillCelsius: -90, CoreTempCelsius: 36, AvalancheDangerScale: 3},
		{PetName: "Yukon", TerrainType: domain.WinterTerrainAvalancheDebrisChute, WeightKg: 25, CoreTempCelsius: 10, AvalancheDangerScale: 3},
		{PetName: "Yukon", TerrainType: domain.WinterTerrainAvalancheDebrisChute, WeightKg: 25, CoreTempCelsius: 36, MinutesBuried: -1, AvalancheDangerScale: 3},
		{PetName: "Yukon", TerrainType: domain.WinterTerrainAvalancheDebrisChute, WeightKg: 25, CoreTempCelsius: 36, AvalancheDangerScale: 6},
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
	missingCertOp.Status = domain.WinterRescueStatusExtractedNormothermicSafe
	missingCertOp.ClearanceCertificateHash = ""
	if err := missingCertOp.Validate(); err == nil {
		t.Errorf("expected error for missing clearanceCertificateHash on EXTRACTED_NORMOTHERMIC_SAFE")
	}
}
