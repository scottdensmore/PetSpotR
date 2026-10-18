package avianrecovery_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/avianrecovery"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvaluateAvianRecovery_MacawThermalDriftAndCropStarvation(t *testing.T) {
	req := domain.AvianEvaluationRequest{
		OperationID:           "AV-OP-901",
		PetID:                 "PET-AV-901",
		PetName:               "Mango",
		Species:               "Avian",
		WeightGrams:           1020.0,
		AvianClass:            domain.AvianClassPsittacineParrotMacaw,
		PerchAltitudeMeters:   25.5,
		ThermalUpdraftKnots:   21.0,
		AmbientTempCelsius:    8.0,
		HoursAloft:            20.0,
		TelemetrySignalDbm:    -82,
		FlightCorridorBearing: "CORRIDOR-SSE-160°",
		LeadAvianTechCallsign: "AVIAN-TECH-01",
		Notes:                 "Scarlet macaw in high Douglas fir crown",
	}

	op, err := avianrecovery.EvaluateAvianRecovery(req)
	if err != nil {
		t.Fatalf("unexpected error evaluating avian recovery: %v", err)
	}

	if op.Status != domain.AvianStatusThermalDriftPredationCritical {
		t.Errorf("expected THERMAL_DRIFT_PREDATION_CRITICAL, got %s", op.Status)
	}
	if !op.ThermalDriftAlert {
		t.Errorf("expected ThermalDriftAlert=true for updraft=21.0kts and altitude=25.5m")
	}
	if !op.CropStarvationColdAlert {
		t.Errorf("expected CropStarvationColdAlert=true for hoursAloft=20.0h and temp=8.0C")
	}
	if !slices.Contains(op.RiskFlags, "EXTREME_THERMAL_DOWNWIND_DRIFT_RISK") {
		t.Errorf("expected EXTREME_THERMAL_DOWNWIND_DRIFT_RISK in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "CRITICAL_AVIAN_CROP_DEPLETION_HYPOTHERMIA") {
		t.Errorf("expected CRITICAL_AVIAN_CROP_DEPLETION_HYPOTHERMIA in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "STEEP_DESCENT_WING_STALL_HAZARD") {
		t.Errorf("expected STEEP_DESCENT_WING_STALL_HAZARD in %v", op.RiskFlags)
	}
	if !slices.Contains(op.RiskFlags, "WEAK_VHF_BEACON_MULTIPATH_ATTENUATION") {
		t.Errorf("expected WEAK_VHF_BEACON_MULTIPATH_ATTENUATION in %v", op.RiskFlags)
	}
	if op.HazardScore < 85 {
		t.Errorf("expected high HazardScore >= 85, got %d", op.HazardScore)
	}
	if len(op.ProtocolSteps) != 5 {
		t.Errorf("expected 5 protocol steps, got %d", len(op.ProtocolSteps))
	}
}

func TestEvaluateAvianRecovery_ClassesAndAutoIDs(t *testing.T) {
	cases := []struct {
		name        string
		req         domain.AvianEvaluationRequest
		wantThermal bool
		wantCrop    bool
	}{
		{
			name: "falconry raptor normal conditions staged",
			req: domain.AvianEvaluationRequest{
				PetName:             "Talon",
				WeightGrams:         760.0,
				AvianClass:          domain.AvianClassFalconryRaptorTelemetry,
				PerchAltitudeMeters: 12.0,
				ThermalUpdraftKnots: 10.5,
				AmbientTempCelsius:  17.0,
				HoursAloft:          4.0,
				TelemetrySignalDbm:  -50,
			},
			wantThermal: false,
			wantCrop:    false,
		},
		{
			name: "cockatiel courtyard perch",
			req: domain.AvianEvaluationRequest{
				PetName:             "Pika",
				Species:             "Avian",
				WeightGrams:         95.0,
				AvianClass:          domain.AvianClassCockatielParakeetSmall,
				PerchAltitudeMeters: 5.5,
				ThermalUpdraftKnots: 6.0,
				AmbientTempCelsius:  19.0,
				HoursAloft:          3.5,
				TelemetrySignalDbm:  -48,
			},
			wantThermal: false,
			wantCrop:    false,
		},
		{
			name: "waterfowl domestic goose cold exposure",
			req: domain.AvianEvaluationRequest{
				PetName:             "Gander",
				Species:             "Avian",
				WeightGrams:         4200.0,
				AvianClass:          domain.AvianClassWaterfowlDomesticGoose,
				PerchAltitudeMeters: 2.0,
				ThermalUpdraftKnots: 8.0,
				AmbientTempCelsius:  6.0,
				HoursAloft:          18.0,
				TelemetrySignalDbm:  -65,
			},
			wantThermal: false,
			wantCrop:    true,
		},
		{
			name: "columbiform homing pigeon high updraft",
			req: domain.AvianEvaluationRequest{
				PetName:             "Homer",
				Species:             "Avian",
				WeightGrams:         420.0,
				AvianClass:          domain.AvianClassColumbiformHomingPigeon,
				PerchAltitudeMeters: 21.0,
				ThermalUpdraftKnots: 19.5,
				AmbientTempCelsius:  15.0,
				HoursAloft:          6.0,
				TelemetrySignalDbm:  -60,
			},
			wantThermal: true,
			wantCrop:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := avianrecovery.EvaluateAvianRecovery(tc.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if op.ThermalDriftAlert != tc.wantThermal {
				t.Errorf("expected ThermalDriftAlert=%v, got %v", tc.wantThermal, op.ThermalDriftAlert)
			}
			if op.CropStarvationColdAlert != tc.wantCrop {
				t.Errorf("expected CropStarvationColdAlert=%v, got %v", tc.wantCrop, op.CropStarvationColdAlert)
			}
			if !strings.HasPrefix(op.OperationID, "AV-OP-") {
				t.Errorf("expected auto-generated AV-OP- prefix, got %q", op.OperationID)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueAvianClearance(t *testing.T) {
	store := avianrecovery.NewStore()
	ops := store.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := store.Get("AV-OP-101")
	if !ok {
		t.Fatalf("expected AV-OP-101 in store")
	}
	if op101.LureReadinessScore != 40 {
		t.Errorf("expected initial readiness 40, got %d", op101.LureReadinessScore)
	}

	// Issuing clearance before all required steps are complete should fail
	if _, err := store.IssueClearance("AV-OP-101", "AVIAN-TECH-01", "premature clearance"); err == nil {
		t.Errorf("expected error issuing clearance with incomplete steps")
	}

	// Uncheck a step and recheck all 5 steps
	if updated, err := store.UpdateStep("AV-OP-101", domain.AvianStepUpdateRequest{
		StepID:    "step-vhf-triangulate",
		Completed: false,
	}); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	} else if updated.LureReadinessScore != 20 {
		t.Errorf("expected readiness 20 after unchecking step, got %d", updated.LureReadinessScore)
	}

	for _, stepID := range []string{
		"step-vhf-triangulate",
		"step-raptor-overwatch",
		"step-flock-acoustic-lure",
		"step-forage-perch-rig",
		"step-kevlar-towel-transfer",
	} {
		updated, err := store.UpdateStep("AV-OP-101", domain.AvianStepUpdateRequest{
			StepID:    stepID,
			Completed: true,
			Notes:     "Completed " + stepID,
		})
		if err != nil {
			t.Fatalf("unexpected error completing step %s: %v", stepID, err)
		}
		op101 = updated
	}

	if op101.LureReadinessScore != 100 {
		t.Errorf("expected LureReadinessScore=100, got %d", op101.LureReadinessScore)
	}
	if op101.Status != domain.AvianStatusAcousticLurePerchStaged {
		t.Errorf("expected ACOUSTIC_LURE_PERCH_STAGED, got %s", op101.Status)
	}

	// Issue clearance now that all steps are completed
	cleared, err := store.IssueClearance("AV-OP-101", "AVIAN-TECH-01", "Rio safely transferred to heated aviary carrier")
	if err != nil {
		t.Fatalf("unexpected error issuing avian clearance: %v", err)
	}
	if cleared.Status != domain.AvianStatusRecoveredAviarySafe {
		t.Errorf("expected RECOVERED_AVIARY_SAFE, got %s", cleared.Status)
	}
	if cleared.HazardScore != 10 {
		t.Errorf("expected HazardScore=10, got %d", cleared.HazardScore)
	}
	if cleared.ThermalDriftAlert || cleared.CropStarvationColdAlert {
		t.Errorf("expected alerts cleared on recovery, got thermal=%v crop=%v", cleared.ThermalDriftAlert, cleared.CropStarvationColdAlert)
	}
	if !strings.HasPrefix(cleared.ClearanceCertificateHash, "AVIAN-SAFE-") || len(cleared.ClearanceCertificateHash) != len("AVIAN-SAFE-")+12 {
		t.Errorf("expected AVIAN-SAFE-<12HEX> hash, got %q", cleared.ClearanceCertificateHash)
	}

	// EvaluateAndSave in store
	created, err := store.EvaluateAndSave(domain.AvianEvaluationRequest{
		PetName:             "Kiwi",
		Species:             "Avian",
		WeightGrams:         980.0,
		AvianClass:          domain.AvianClassPsittacineParrotMacaw,
		PerchAltitudeMeters: 24.0,
		ThermalUpdraftKnots: 20.5,
		AmbientTempCelsius:  8.5,
		HoursAloft:          19.0,
		TelemetrySignalDbm:  -76,
	})
	if err != nil {
		t.Fatalf("unexpected error in EvaluateAndSave: %v", err)
	}
	if created.OperationID != "AV-OP-104" {
		t.Errorf("expected AV-OP-104, got %s", created.OperationID)
	}
}

func TestValidationAndErrorPaths(t *testing.T) {
	store := avianrecovery.NewStore()

	if _, ok := store.Get("NON-EXISTENT"); ok {
		t.Errorf("expected false for unknown operation")
	}
	if _, err := store.UpdateStep("NON-EXISTENT", domain.AvianStepUpdateRequest{StepID: "step-vhf-triangulate", Completed: true}); err == nil {
		t.Errorf("expected error for unknown operation on UpdateStep")
	}
	if _, err := store.IssueClearance("NON-EXISTENT", "AVIAN-TECH", ""); err == nil {
		t.Errorf("expected error for unknown operation on IssueClearance")
	}

	op102, _ := store.Get("AV-OP-102")
	if _, err := avianrecovery.UpdateProtocolStep(op102, domain.AvianStepUpdateRequest{StepID: ""}); err == nil {
		t.Errorf("expected error for empty stepId")
	}
	if _, err := avianrecovery.UpdateProtocolStep(op102, domain.AvianStepUpdateRequest{StepID: "invalid-step"}); err == nil {
		t.Errorf("expected error for unknown stepId")
	}
	if _, err := avianrecovery.IssueAvianClearance(domain.AvianRecoveryOperation{}, "AVIAN-TECH", ""); err == nil {
		t.Errorf("expected error for empty operationId")
	}

	invalidReqs := []domain.AvianEvaluationRequest{
		{PetName: ""},
		{PetName: "Rio", AvianClass: "INVALID_CLASS", WeightGrams: 900, TelemetrySignalDbm: -60},
		{PetName: "Rio", AvianClass: domain.AvianClassPsittacineParrotMacaw, WeightGrams: 0, TelemetrySignalDbm: -60},
		{PetName: "Rio", AvianClass: domain.AvianClassPsittacineParrotMacaw, WeightGrams: 900, PerchAltitudeMeters: -5, TelemetrySignalDbm: -60},
		{PetName: "Rio", AvianClass: domain.AvianClassPsittacineParrotMacaw, WeightGrams: 900, ThermalUpdraftKnots: -2, TelemetrySignalDbm: -60},
		{PetName: "Rio", AvianClass: domain.AvianClassPsittacineParrotMacaw, WeightGrams: 900, AmbientTempCelsius: -50, TelemetrySignalDbm: -60},
		{PetName: "Rio", AvianClass: domain.AvianClassPsittacineParrotMacaw, WeightGrams: 900, HoursAloft: -1, TelemetrySignalDbm: -60},
		{PetName: "Rio", AvianClass: domain.AvianClassPsittacineParrotMacaw, WeightGrams: 900, TelemetrySignalDbm: 15},
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
	missingCertOp.Status = domain.AvianStatusRecoveredAviarySafe
	missingCertOp.ClearanceCertificateHash = ""
	if err := missingCertOp.Validate(); err == nil {
		t.Errorf("expected error for missing clearanceCertificateHash on RECOVERED_AVIARY_SAFE")
	}
}
