package dystociaeclampsia_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/dystociaeclampsia"
)

func TestEvaluateDystociaTriage_ValidationAndClassification(t *testing.T) {
	validBase := domain.DystociaEvaluationRequest{
		PetID:                          "PET-DE-TEST",
		PetName:                        "Penelope",
		Species:                        "Dog - English Bulldog Dam",
		WeightKg:                       23.5,
		PresentationType:               domain.DystociaObstructiveBrachycephalicFetalOversize,
		LowestFetalHeartRateBpm:        154,
		IonizedCalciumMmolL:            1.12,
		InterPuppyIntervalMins:         145,
		GreenBlackUteroverdinNoFetus:   true,
		PelvicCanalObstructionExcluded: false,
		ECGMonitoredCalciumInfusion:    true,
	}

	// 1. Critical Fetal Distress / Obstructive evaluation
	op, err := dystociaeclampsia.EvaluateDystociaTriage(validBase)
	if err != nil {
		t.Fatalf("expected valid evaluation, got error: %v", err)
	}
	if op.Status != domain.DystociaStatusFetalDistressObstructiveCritical {
		t.Errorf("expected status %s, got %s", domain.DystociaStatusFetalDistressObstructiveCritical, op.Status)
	}
	if len(op.ProtocolSteps) != 4 {
		t.Fatalf("expected 4 protocol steps, got %d", len(op.ProtocolSteps))
	}
	if !strings.Contains(strings.ToLower(op.OxytocinContraindicationNote), "oxytocin") || !strings.Contains(strings.ToLower(op.OxytocinContraindicationNote), "uterine rupture") {
		t.Errorf("expected contraindication note to mention oxytocin and uterine rupture, got %q", op.OxytocinContraindicationNote)
	}
	if !strings.Contains(strings.ToLower(op.OxytocinContraindicationNote), "calcium gluconate") || !strings.Contains(strings.ToLower(op.OxytocinContraindicationNote), "ecg") {
		t.Errorf("expected contraindication note to mention calcium gluconate and ECG, got %q", op.OxytocinContraindicationNote)
	}
	if err := op.Validate(); err != nil {
		t.Errorf("expected op.Validate() to succeed, got %v", err)
	}

	// 2. Staged evaluation across all presentation types
	presentations := []domain.DystociaPresentationType{
		domain.DystociaObstructiveBrachycephalicFetalOversize,
		domain.DystociaPrimaryUterineInertiaNormocalcemic,
		domain.DystociaPuerperalEclampsiaHypocalcemiaTetany,
		domain.DystociaPlacentalDetachmentUteroverdin,
		domain.DystociaMalpresentationBreechTransverse,
	}
	for _, pres := range presentations {
		stagedReq := validBase
		stagedReq.PetName = "Maple"
		stagedReq.Species = ""
		stagedReq.PetID = ""
		stagedReq.PresentationType = pres
		stagedReq.LowestFetalHeartRateBpm = 205
		stagedReq.IonizedCalciumMmolL = 1.18
		stagedReq.InterPuppyIntervalMins = 75
		stagedReq.GreenBlackUteroverdinNoFetus = false
		stagedReq.PelvicCanalObstructionExcluded = true
		stagedReq.ECGMonitoredCalciumInfusion = false

		stagedOp, err := dystociaeclampsia.EvaluateDystociaTriage(stagedReq)
		if err != nil {
			t.Fatalf("unexpected error for presentation %s: %v", pres, err)
		}
		if stagedOp.Status != domain.DystociaStatusCalciumECGWhelpingStaged {
			t.Errorf("expected STAGED status for %s, got %s", pres, stagedOp.Status)
		}
	}

	// 3. Individual critical triggers
	critTriggers := []struct {
		name   string
		mutate func(r *domain.DystociaEvaluationRequest)
	}{
		{"fetal HR < 180", func(r *domain.DystociaEvaluationRequest) { r.LowestFetalHeartRateBpm = 165 }},
		{"ionized calcium < 0.9", func(r *domain.DystociaEvaluationRequest) { r.IonizedCalciumMmolL = 0.72 }},
		{"uteroverdin without fetus", func(r *domain.DystociaEvaluationRequest) { r.GreenBlackUteroverdinNoFetus = true }},
		{"pelvic obstruction not excluded", func(r *domain.DystociaEvaluationRequest) { r.PelvicCanalObstructionExcluded = false }},
		{"inter-puppy interval >= 120", func(r *domain.DystociaEvaluationRequest) { r.InterPuppyIntervalMins = 130 }},
	}
	for _, ct := range critTriggers {
		t.Run(ct.name, func(t *testing.T) {
			req := domain.DystociaEvaluationRequest{
				PetName:                        "Maple",
				Species:                        "Dog - Spaniel",
				WeightKg:                       8.2,
				PresentationType:               domain.DystociaPrimaryUterineInertiaNormocalcemic,
				LowestFetalHeartRateBpm:        205,
				IonizedCalciumMmolL:            1.18,
				InterPuppyIntervalMins:         75,
				GreenBlackUteroverdinNoFetus:   false,
				PelvicCanalObstructionExcluded: true,
				ECGMonitoredCalciumInfusion:    true,
			}
			ct.mutate(&req)
			resOp, err := dystociaeclampsia.EvaluateDystociaTriage(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resOp.Status != domain.DystociaStatusFetalDistressObstructiveCritical {
				t.Errorf("expected critical status for trigger %s, got %s", ct.name, resOp.Status)
			}
		})
	}

	// 4. Validation error cases
	invalidCases := []struct {
		name   string
		mutate func(r *domain.DystociaEvaluationRequest)
	}{
		{"empty pet name", func(r *domain.DystociaEvaluationRequest) { r.PetName = "  " }},
		{"weight < 1.5", func(r *domain.DystociaEvaluationRequest) { r.WeightKg = 1.0 }},
		{"weight > 90.0", func(r *domain.DystociaEvaluationRequest) { r.WeightKg = 95.0 }},
		{"invalid presentation type", func(r *domain.DystociaEvaluationRequest) { r.PresentationType = "UNKNOWN_PRESENTATION" }},
		{"fetal HR < 80", func(r *domain.DystociaEvaluationRequest) { r.LowestFetalHeartRateBpm = 60 }},
		{"fetal HR > 280", func(r *domain.DystociaEvaluationRequest) { r.LowestFetalHeartRateBpm = 300 }},
		{"iCa < 0.4", func(r *domain.DystociaEvaluationRequest) { r.IonizedCalciumMmolL = 0.25 }},
		{"iCa > 1.8", func(r *domain.DystociaEvaluationRequest) { r.IonizedCalciumMmolL = 2.1 }},
		{"interval < 0", func(r *domain.DystociaEvaluationRequest) { r.InterPuppyIntervalMins = -5 }},
		{"interval > 480", func(r *domain.DystociaEvaluationRequest) { r.InterPuppyIntervalMins = 500 }},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			req := validBase
			tc.mutate(&req)
			if _, err := dystociaeclampsia.EvaluateDystociaTriage(req); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestUpdateProtocolStepAndIssueDystociaClearance(t *testing.T) {
	op, err := dystociaeclampsia.EvaluateDystociaTriage(domain.DystociaEvaluationRequest{
		ID:                             "DE-OP-TEST",
		PetID:                          "PET-DE-TEST",
		PetName:                        "Penelope",
		Species:                        "Dog - English Bulldog Dam",
		WeightKg:                       23.5,
		PresentationType:               domain.DystociaObstructiveBrachycephalicFetalOversize,
		LowestFetalHeartRateBpm:        154,
		IonizedCalciumMmolL:            1.12,
		InterPuppyIntervalMins:         145,
		GreenBlackUteroverdinNoFetus:   true,
		PelvicCanalObstructionExcluded: false,
		ECGMonitoredCalciumInfusion:    true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := dystociaeclampsia.UpdateProtocolStep(nil, dystociaeclampsia.StepDopplerFetalHRPelvicXray, true); err == nil {
		t.Errorf("expected error for nil op")
	}
	if err := dystociaeclampsia.UpdateProtocolStep(op, "", true); err == nil {
		t.Errorf("expected error for empty stepID")
	}
	if err := dystociaeclampsia.UpdateProtocolStep(op, "NON-EXISTENT-STEP", true); err == nil {
		t.Errorf("expected error for unknown stepID")
	}

	if err := dystociaeclampsia.IssueDystociaClearance(nil); err == nil {
		t.Errorf("expected error for nil op on clearance")
	}
	if err := dystociaeclampsia.IssueDystociaClearance(op); err == nil {
		t.Errorf("expected error issuing clearance before steps are complete")
	}

	// Complete all 4 steps
	for _, s := range op.ProtocolSteps {
		if err := dystociaeclampsia.UpdateProtocolStep(op, s.ID, true); err != nil {
			t.Fatalf("failed completing step %s: %v", s.ID, err)
		}
	}
	if op.Status != domain.DystociaStatusCalciumECGWhelpingStaged {
		t.Errorf("expected status to transition to %s after all steps completed, got %s", domain.DystociaStatusCalciumECGWhelpingStaged, op.Status)
	}

	// Issue clearance
	if err := dystociaeclampsia.IssueDystociaClearance(op); err != nil {
		t.Fatalf("expected clearance to succeed, got %v", err)
	}
	if op.Status != domain.DystociaStatusDeliveredNormocalcemicSafe {
		t.Errorf("expected status %s, got %s", domain.DystociaStatusDeliveredNormocalcemicSafe, op.Status)
	}
	if !strings.HasPrefix(op.ClearanceToken, "DYSTOCIA-SAFE-") || len(op.ClearanceToken) != len("DYSTOCIA-SAFE-")+12 {
		t.Errorf("expected DYSTOCIA-SAFE-<12HEX> token, got %q", op.ClearanceToken)
	}

	// Unchecking a step reverts status to Staged and clears token
	if err := dystociaeclampsia.UpdateProtocolStep(op, dystociaeclampsia.StepNeonatalResusColostrumLatch, false); err != nil {
		t.Fatalf("unexpected error unchecking step: %v", err)
	}
	if op.Status != domain.DystociaStatusCalciumECGWhelpingStaged || op.ClearanceToken != "" {
		t.Errorf("expected status %s and empty token after unchecking step, got status=%s token=%q", domain.DystociaStatusCalciumECGWhelpingStaged, op.Status, op.ClearanceToken)
	}
}

func TestStore_SeededAndCRUD(t *testing.T) {
	st := dystociaeclampsia.NewStore()
	ops := st.List()
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, ok := st.Get("DE-OP-101")
	if !ok || op101.PetName != "Penelope" || op101.Status != domain.DystociaStatusFetalDistressObstructiveCritical {
		t.Errorf("unexpected DE-OP-101: %+v", op101)
	}
	op102, ok := st.Get("DE-OP-102")
	if !ok || op102.PetName != "Maple" || op102.Status != domain.DystociaStatusCalciumECGWhelpingStaged {
		t.Errorf("unexpected DE-OP-102: %+v", op102)
	}
	op103, ok := st.Get("DE-OP-103")
	if !ok || op103.PetName != "Rosie" || op103.Status != domain.DystociaStatusDeliveredNormocalcemicSafe || !strings.HasPrefix(op103.ClearanceToken, "DYSTOCIA-SAFE-") {
		t.Errorf("unexpected DE-OP-103: %+v", op103)
	}

	if _, ok := st.Get("UNKNOWN"); ok {
		t.Errorf("expected false for unknown operation ID")
	}
	if _, err := st.UpdateStep("UNKNOWN", dystociaeclampsia.StepDopplerFetalHRPelvicXray, true); err == nil {
		t.Errorf("expected error updating step on unknown op")
	}
	if _, err := st.IssueClearance("UNKNOWN"); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	created, err := st.EvaluateAndSave(domain.DystociaEvaluationRequest{
		PetName:                        "Duchess",
		Species:                        "Dog - French Bulldog Dam",
		WeightKg:                       11.4,
		PresentationType:               domain.DystociaPlacentalDetachmentUteroverdin,
		LowestFetalHeartRateBpm:        148,
		IonizedCalciumMmolL:            0.84,
		InterPuppyIntervalMins:         160,
		GreenBlackUteroverdinNoFetus:   true,
		PelvicCanalObstructionExcluded: false,
		ECGMonitoredCalciumInfusion:    true,
	})
	if err != nil {
		t.Fatalf("unexpected EvaluateAndSave error: %v", err)
	}
	if created.ID != "DE-OP-104" {
		t.Errorf("expected DE-OP-104, got %s", created.ID)
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
	if cleared.Status != domain.DystociaStatusDeliveredNormocalcemicSafe || !strings.HasPrefix(cleared.ClearanceToken, "DYSTOCIA-SAFE-") {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}
