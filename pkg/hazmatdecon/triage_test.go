package hazmatdecon_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/hazmatdecon"
)

func TestEvaluateHazmatDecon_WildfireSmokeAndDermalAshfall(t *testing.T) {
	req := domain.HazmatEvaluationRequest{
		OperationID:          "HZ-OP-901",
		PetID:                "PET-901",
		PetName:              "Ember",
		Species:              "Feline",
		WeightKg:             4.2,
		ExposureType:         domain.HazmatExposureWildfireSmokeAshfall,
		AQIIndex:             440,
		PM25UgM3:             395.0,
		CarboxyhemoglobinPct: 17.2,
		CoatContaminationPct: 68.0,
		RespiratoryRateBpm:   70,
		DeconCorridorZone:    "DECON-CORRIDOR-HOT-01",
		LeadToxTechCallsign:  "TOX-VET-01",
		Notes:                "Feline pulled from active wildfire ashfall zone",
	}

	op, err := hazmatdecon.EvaluateHazmatDecon(req)
	if err != nil {
		t.Fatalf("unexpected error evaluating hazmat decon: %v", err)
	}

	if op.Status != domain.HazmatStatusRespiratoryDermalCritical {
		t.Errorf("expected RESPIRATORY_DERMAL_CRITICAL, got %s", op.Status)
	}
	if !op.SmokeInhalationAlert {
		t.Errorf("expected SmokeInhalationAlert=true for AQI=440, PM2.5=395, COHb=17.2%%")
	}
	if !op.DermalToxinIngestionAlert {
		t.Errorf("expected DermalToxinIngestionAlert=true for CoatContaminationPct=68.0%%")
	}
	if !slices.Contains(op.RiskFlags, "EXTREME_SMOKE_CYANIDE_HYPOXIA_RISK") {
		t.Errorf("expected EXTREME_SMOKE_CYANIDE_HYPOXIA_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "CRITICAL_COAT_GROOMING_TOXICOSIS_RISK") {
		t.Errorf("expected CRITICAL_COAT_GROOMING_TOXICOSIS_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "CORNEAL_ASH_ABRASION_HAZARD") {
		t.Errorf("expected CORNEAL_ASH_ABRASION_HAZARD in %v", op.RiskFlags)
	}
	if op.HazardScore < 85 {
		t.Errorf("expected high HazardScore >= 85, got %d", op.HazardScore)
	}
	if len(op.ProtocolSteps) != 5 {
		t.Errorf("expected 5 protocol steps, got %d", len(op.ProtocolSteps))
	}
}

func TestEvaluateHazmatDecon_ExposureTypesAndAutoIDs(t *testing.T) {
	cases := []struct {
		name       string
		req        domain.HazmatEvaluationRequest
		wantSmoke  bool
		wantDermal bool
	}{
		{
			name: "petrochemical spill coat triggers dermal alert",
			req: domain.HazmatEvaluationRequest{
				PetName:              "Diesel",
				WeightKg:             19.5,
				ExposureType:         domain.HazmatExposurePetrochemicalSpillCoat,
				AQIIndex:             85,
				PM25UgM3:             28.0,
				CarboxyhemoglobinPct: 2.1,
				CoatContaminationPct: 52.0,
				RespiratoryRateBpm:   34,
			},
			wantSmoke:  false,
			wantDermal: true,
		},
		{
			name: "toxic algae cyanotoxin bloom triggers dermal & bloom alert",
			req: domain.HazmatEvaluationRequest{
				PetName:              "splash",
				Species:              "Canine",
				WeightKg:             16.0,
				ExposureType:         domain.HazmatExposureToxicAlgaeCyanotoxin,
				AQIIndex:             40,
				PM25UgM3:             10.0,
				CarboxyhemoglobinPct: 1.0,
				CoatContaminationPct: 20.0,
				RespiratoryRateBpm:   28,
			},
			wantSmoke:  false,
			wantDermal: true,
		},
		{
			name: "tear gas aerosol plume moderate exposure staged",
			req: domain.HazmatEvaluationRequest{
				PetName:              "Scout",
				Species:              "Canine",
				WeightKg:             14.0,
				ExposureType:         domain.HazmatExposureTearGasAerosolPlume,
				AQIIndex:             160,
				PM25UgM3:             65.0,
				CarboxyhemoglobinPct: 2.5,
				CoatContaminationPct: 15.0,
				RespiratoryRateBpm:   42,
			},
			wantSmoke:  false,
			wantDermal: false,
		},
		{
			name: "industrial solvent vapor high tachypnea triggers smoke/respiratory alert",
			req: domain.HazmatEvaluationRequest{
				PetName:              "Vapor",
				Species:              "Feline",
				WeightKg:             3.8,
				ExposureType:         domain.HazmatExposureIndustrialSolventVapor,
				AQIIndex:             180,
				PM25UgM3:             70.0,
				CarboxyhemoglobinPct: 4.0,
				CoatContaminationPct: 10.0,
				RespiratoryRateBpm:   60,
			},
			wantSmoke:  true,
			wantDermal: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := hazmatdecon.EvaluateHazmatDecon(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.SmokeInhalationAlert != tc.wantSmoke {
				t.Errorf("expected SmokeInhalationAlert=%v, got %v", tc.wantSmoke, op.SmokeInhalationAlert)
			}
			if op.DermalToxinIngestionAlert != tc.wantDermal {
				t.Errorf("expected DermalToxinIngestionAlert=%v, got %v", tc.wantDermal, op.DermalToxinIngestionAlert)
			}
			if !strings.HasPrefix(op.OperationID, "HZ-OP-") {
				t.Errorf("expected auto-generated HZ-OP- prefix, got %q", op.OperationID)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueDeconClearance(t *testing.T) {
	store := hazmatdecon.NewStore()
	ops := store.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := store.Get("HZ-OP-101")
	if !ok {
		t.Fatalf("expected HZ-OP-101 in store")
	}
	if op101.DeconReadinessScore != 40 {
		t.Errorf("expected initial readiness 40, got %d", op101.DeconReadinessScore)
	}

	// Issuing clearance before all required steps are complete should fail
	if _, err := store.IssueClearance("HZ-OP-101", "TOX-VET-01", "premature clearance"); err == nil {
		t.Errorf("expected error issuing clearance with incomplete steps")
	}

	// Uncheck a step and recheck all 5 steps
	if updated, err := store.UpdateStep("HZ-OP-101", domain.HazmatStepUpdateRequest{
		StepID:    "step-ocular-flush",
		Completed: false,
	}); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	} else if updated.DeconReadinessScore != 20 {
		t.Errorf("expected readiness 20 after unchecking step, got %d", updated.DeconReadinessScore)
	}

	for _, stepID := range []string{
		"step-ocular-flush",
		"step-elizabethan-collar",
		"step-warm-surfactant-wash",
		"step-oxygen-nebulizer",
		"step-cohb-binder-panel",
	} {
		updated, err := store.UpdateStep("HZ-OP-101", domain.HazmatStepUpdateRequest{
			StepID:    stepID,
			Completed: true,
			Notes:     "Completed " + stepID,
		})
		if err != nil {
			t.Fatalf("unexpected error completing step %s: %v", stepID, err)
		}
		op101 = updated
	}

	if op101.DeconReadinessScore != 100 {
		t.Errorf("expected DeconReadinessScore=100, got %d", op101.DeconReadinessScore)
	}
	if op101.Status != domain.HazmatStatusDeconWashNebulizerStaged {
		t.Errorf("expected DECON_WASH_NEBULIZER_STAGED, got %s", op101.Status)
	}

	// Issue clearance now that all steps are completed
	cleared, err := store.IssueClearance("HZ-OP-101", "TOX-VET-01", "Cinder decontaminated and COHb normalized")
	if err != nil {
		t.Fatalf("unexpected error issuing hazmat decon clearance: %v", err)
	}
	if cleared.Status != domain.HazmatStatusDecontaminatedClearedSafe {
		t.Errorf("expected DECONTAMINATED_CLEARED_SAFE, got %s", cleared.Status)
	}
	if cleared.HazardScore != 10 {
		t.Errorf("expected HazardScore=10, got %d", cleared.HazardScore)
	}
	if cleared.SmokeInhalationAlert || cleared.DermalToxinIngestionAlert {
		t.Errorf("expected alerts cleared on decon clearance, got smoke=%v dermal=%v", cleared.SmokeInhalationAlert, cleared.DermalToxinIngestionAlert)
	}
	if !strings.HasPrefix(cleared.ClearanceCertificateHash, "HAZMAT-SAFE-") || len(cleared.ClearanceCertificateHash) != len("HAZMAT-SAFE-")+12 {
		t.Errorf("expected HAZMAT-SAFE-<12HEX> hash, got %q", cleared.ClearanceCertificateHash)
	}

	// EvaluateAndSave in store
	created, err := store.EvaluateAndSave(domain.HazmatEvaluationRequest{
		PetName:              "Soot",
		Species:              "Feline",
		WeightKg:             3.9,
		ExposureType:         domain.HazmatExposureWildfireSmokeAshfall,
		AQIIndex:             390,
		PM25UgM3:             340.0,
		CarboxyhemoglobinPct: 14.5,
		CoatContaminationPct: 55.0,
		RespiratoryRateBpm:   64,
	})
	if err != nil {
		t.Fatalf("unexpected error in EvaluateAndSave: %v", err)
	}
	if created.OperationID != "HZ-OP-104" {
		t.Errorf("expected HZ-OP-104, got %s", created.OperationID)
	}
}

func TestValidationAndErrorPaths(t *testing.T) {
	store := hazmatdecon.NewStore()

	if _, ok := store.Get("NON-EXISTENT"); ok {
		t.Errorf("expected false for unknown operation")
	}
	if _, err := store.UpdateStep("NON-EXISTENT", domain.HazmatStepUpdateRequest{StepID: "step-ocular-flush", Completed: true}); err == nil {
		t.Errorf("expected error for unknown operation on UpdateStep")
	}
	if _, err := store.IssueClearance("NON-EXISTENT", "TOX-VET", ""); err == nil {
		t.Errorf("expected error for unknown operation on IssueClearance")
	}

	op102, _ := store.Get("HZ-OP-102")
	if _, err := hazmatdecon.UpdateProtocolStep(op102, domain.HazmatStepUpdateRequest{StepID: ""}); err == nil {
		t.Errorf("expected error for empty stepId")
	}
	if _, err := hazmatdecon.UpdateProtocolStep(op102, domain.HazmatStepUpdateRequest{StepID: "invalid-step"}); err == nil {
		t.Errorf("expected error for unknown stepId")
	}
	if _, err := hazmatdecon.IssueDeconClearance(domain.HazmatDeconOperation{}, "TOX-VET", ""); err == nil {
		t.Errorf("expected error for empty operationId")
	}

	invalidReqs := []domain.HazmatEvaluationRequest{
		{PetName: ""},
		{PetName: "Cinder", ExposureType: "INVALID_EXPOSURE", WeightKg: 4, AQIIndex: 100, PM25UgM3: 30, RespiratoryRateBpm: 30},
		{PetName: "Cinder", ExposureType: domain.HazmatExposureWildfireSmokeAshfall, WeightKg: 0, AQIIndex: 100, PM25UgM3: 30, RespiratoryRateBpm: 30},
		{PetName: "Cinder", ExposureType: domain.HazmatExposureWildfireSmokeAshfall, WeightKg: 4, AQIIndex: -10, PM25UgM3: 30, RespiratoryRateBpm: 30},
		{PetName: "Cinder", ExposureType: domain.HazmatExposureWildfireSmokeAshfall, WeightKg: 4, AQIIndex: 100, PM25UgM3: -5, RespiratoryRateBpm: 30},
		{PetName: "Cinder", ExposureType: domain.HazmatExposureWildfireSmokeAshfall, WeightKg: 4, AQIIndex: 100, PM25UgM3: 30, CarboxyhemoglobinPct: 120, RespiratoryRateBpm: 30},
		{PetName: "Cinder", ExposureType: domain.HazmatExposureWildfireSmokeAshfall, WeightKg: 4, AQIIndex: 100, PM25UgM3: 30, CoatContaminationPct: -1, RespiratoryRateBpm: 30},
		{PetName: "Cinder", ExposureType: domain.HazmatExposureWildfireSmokeAshfall, WeightKg: 4, AQIIndex: 100, PM25UgM3: 30, RespiratoryRateBpm: 0},
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
	missingCertOp.Status = domain.HazmatStatusDecontaminatedClearedSafe
	missingCertOp.ClearanceCertificateHash = ""
	if err := missingCertOp.Validate(); err == nil {
		t.Errorf("expected error for missing clearanceCertificateHash on DECONTAMINATED_CLEARED_SAFE")
	}
}
