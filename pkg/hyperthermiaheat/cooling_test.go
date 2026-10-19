package hyperthermiaheat_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/hyperthermiaheat"
)

func TestEvaluateHeatTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.HeatEvaluationRequest{
		PetID:                           "PET-TEST-01",
		PetName:                         "Apollo",
		Species:                         "Dog - French Bulldog",
		WeightKg:                        12.8,
		ExposureType:                    domain.HeatExposureEnclosedVehicleSolarCabin,
		RectalCoreTempCelsius:           41.8,
		SurfaceAsphaltTempCelsius:       64.0,
		ExposureDurationMins:            35,
		BrachycephalicAirwayObstruction: true,
		PawPadSloughingGrade:            2,
		CoolWaterConvectionFanActive:    true,
		IceBathVasoconstrictionAvoided:  true,
	}

	// 1. Critical heatstroke evaluation
	op, err := hyperthermiaheat.EvaluateHeatTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.HeatStatusCriticalDIC {
		t.Errorf("expected status %s, got %s", domain.HeatStatusCriticalDIC, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	if !strings.Contains(op.CoagulopathyCutoffNote, "39.4°C") || !strings.Contains(op.CoagulopathyCutoffNote, "103.0°F") {
		t.Errorf("expected cutoff note to mention 39.4°C / 103.0°F, got %q", op.CoagulopathyCutoffNote)
	}
	if !strings.Contains(strings.ToLower(op.CoagulopathyCutoffNote), "vasoconstriction") {
		t.Errorf("expected cutoff note to mention vasoconstriction, got %q", op.CoagulopathyCutoffNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged cooling evaluation (non-critical parameters across all exposure types)
	exposureTypes := []domain.HeatExposureType{
		domain.HeatExposureHotAsphaltPawNecrosis,
		domain.HeatExposureExertionalBrachycephalicStroke,
		domain.HeatExposureUnshadedYardTetherDome,
		domain.HeatExposureGarageAtticVentilationFailure,
	}
	for _, exp := range exposureTypes {
		stagedReq := validBase
		stagedReq.PetName = "Biscuit"
		stagedReq.Species = ""
		stagedReq.PetID = ""
		stagedReq.ExposureType = exp
		stagedReq.RectalCoreTempCelsius = 40.1
		stagedReq.SurfaceAsphaltTempCelsius = 55.0
		stagedReq.ExposureDurationMins = 20
		stagedReq.BrachycephalicAirwayObstruction = false
		stagedReq.PawPadSloughingGrade = 2
		stagedReq.IceBathVasoconstrictionAvoided = true

		stagedOp, err := hyperthermiaheat.EvaluateHeatTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for exposure %s: %v", exp, err)
		}
		if stagedOp.Status != domain.HeatStatusStagedCooling {
			t.Errorf("expected STAGED status for %s, got %s", exp, stagedOp.Status)
		}
	}

	// 3. Ice bath vasoconstriction not avoided triggers Critical DIC status
	iceBathReq := validBase
	iceBathReq.RectalCoreTempCelsius = 39.8
	iceBathReq.ExposureDurationMins = 15
	iceBathReq.BrachycephalicAirwayObstruction = false
	iceBathReq.PawPadSloughingGrade = 0
	iceBathReq.IceBathVasoconstrictionAvoided = false
	iceOp, err := hyperthermiaheat.EvaluateHeatTriage(iceBathReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if iceOp.Status != domain.HeatStatusCriticalDIC {
		t.Errorf("expected critical status when ice bath vasoconstriction not avoided, got %s", iceOp.Status)
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.HeatEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.HeatEvaluationRequest) { r.PetName = "  " }},
		{"non-positive weight", func(r *domain.HeatEvaluationRequest) { r.WeightKg = 0 }},
		{"invalid exposure type", func(r *domain.HeatEvaluationRequest) { r.ExposureType = "UNKNOWN_HEAT" }},
		{"rectal temp too low", func(r *domain.HeatEvaluationRequest) { r.RectalCoreTempCelsius = 36.5 }},
		{"rectal temp too high", func(r *domain.HeatEvaluationRequest) { r.RectalCoreTempCelsius = 45.5 }},
		{"surface temp too low", func(r *domain.HeatEvaluationRequest) { r.SurfaceAsphaltTempCelsius = 10.0 }},
		{"surface temp too high", func(r *domain.HeatEvaluationRequest) { r.SurfaceAsphaltTempCelsius = 99.0 }},
		{"duration non-positive", func(r *domain.HeatEvaluationRequest) { r.ExposureDurationMins = 0 }},
		{"paw grade negative", func(r *domain.HeatEvaluationRequest) { r.PawPadSloughingGrade = -1 }},
		{"paw grade > 4", func(r *domain.HeatEvaluationRequest) { r.PawPadSloughingGrade = 5 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := hyperthermiaheat.EvaluateHeatTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueHeatClearance(t *testing.T) {
	op, err := hyperthermiaheat.EvaluateHeatTriage(domain.HeatEvaluationRequest{
		ID:                              "HT-OP-TEST",
		PetID:                           "PET-HT-TEST",
		PetName:                         "Apollo",
		Species:                         "Dog - French Bulldog",
		WeightKg:                        12.8,
		ExposureType:                    domain.HeatExposureEnclosedVehicleSolarCabin,
		RectalCoreTempCelsius:           41.8,
		SurfaceAsphaltTempCelsius:       64.0,
		ExposureDurationMins:            35,
		BrachycephalicAirwayObstruction: true,
		PawPadSloughingGrade:            3,
		CoolWaterConvectionFanActive:    true,
		IceBathVasoconstrictionAvoided:  true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Nil / invalid step checks
	if err := hyperthermiaheat.UpdateProtocolStep(nil, hyperthermiaheat.StepConvectiveCoolingStart, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := hyperthermiaheat.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := hyperthermiaheat.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	// Clearance before completing all steps should fail
	if err := hyperthermiaheat.IssueHeatClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := hyperthermiaheat.IssueHeatClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	// Complete all 4 steps
	for _, s := range op.ProtocolSteps {
		if err := hyperthermiaheat.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.HeatStatusStagedCooling {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.HeatStatusStagedCooling, op.Status)
	}

	// Issue clearance
	if err := hyperthermiaheat.IssueHeatClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.HeatStatusStabilizedCooled {
		t.Errorf("expected status %s, got %s", domain.HeatStatusStabilizedCooled, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "HEAT-SAFE-") || len(op.ClearanceToken) != len("HEAT-SAFE-")+12 {
		t.Errorf("expected HEAT-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to StagedCooling and clears token
	if err := hyperthermiaheat.UpdateProtocolStep(op, hyperthermiaheat.StepDICCoagCrystalloidIV, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.HeatStatusStagedCooling || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.HeatStatusStagedCooling, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := hyperthermiaheat.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("HT-OP-101")
	if !ok || op101.PetName != "Apollo" || op101.Status != domain.HeatStatusCriticalDIC {
		t.Errorf("unexpected HT-OP-101: %+v", op101)
	}
	op102, ok := st.Get("HT-OP-102")
	if !ok || op102.PetName != "Biscuit" || op102.Status != domain.HeatStatusStagedCooling {
		t.Errorf("unexpected HT-OP-102: %+v", op102)
	}
	op103, ok := st.Get("HT-OP-103")
	if !ok || op103.PetName != "Daisy" || op103.Status != domain.HeatStatusStabilizedCooled || !strings.HasPrefix(op103.ClearanceToken, "HEAT-SAFE-") {
		t.Errorf("unexpected HT-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}

	created, err := st.EvaluateAndSave(domain.HeatEvaluationRequest{
		PetName:                         "Zephyr",
		Species:                         "Dog - Boxer",
		WeightKg:                        27.5,
		ExposureType:                    domain.HeatExposureExertionalBrachycephalicStroke,
		RectalCoreTempCelsius:           41.4,
		SurfaceAsphaltTempCelsius:       58.0,
		ExposureDurationMins:            32,
		BrachycephalicAirwayObstruction: true,
		PawPadSloughingGrade:            1,
		CoolWaterConvectionFanActive:    true,
		IceBathVasoconstrictionAvoided:  true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "HT-OP-104" {
		t.Errorf("expected HT-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.HeatStatusStabilizedCooled || !strings.HasPrefix(cleared.ClearanceToken, "HEAT-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
