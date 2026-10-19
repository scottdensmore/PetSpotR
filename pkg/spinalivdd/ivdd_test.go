package spinalivdd_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/spinalivdd"
)

func TestIVDDServiceSeededOperationsAndMetrics(t *testing.T) {
	svc := spinalivdd.NewIVDDService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("IVDD-OP-101")
	if err != nil {
		t.Fatalf("expected IVDD-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Barnaby" || op101.Breed != "French Bulldog" || op101.WeightKg != 12.4 || op101.Status != domain.SpinalStatusAcuteParaplegiaTriage {
		t.Errorf("unexpected IVDD-OP-101 metadata: %+v", op101)
	}
	if op101.FrankelGrade != domain.FrankelGrade5DeepPainAbsent || op101.DeepPainPresent || !op101.SchiffSherringtonPresent {
		t.Errorf("unexpected IVDD-OP-101 neuro status: %+v", op101)
	}
	if len(op101.NeuroChecks) != 2 {
		t.Errorf("expected 2 seeded neuro checks on IVDD-OP-101, got %d", len(op101.NeuroChecks))
	}
	if !strings.Contains(strings.ToLower(op101.NociceptionPlan.WithdrawalReflexNote), "withdrawal reflex") ||
		!strings.Contains(strings.ToLower(op101.NociceptionPlan.SteroidNSAIDWashoutAlert), "steroid") {
		t.Errorf("unexpected IVDD-OP-101 nociception plan: %+v", op101.NociceptionPlan)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected IVDD-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("IVDD-OP-102")
	if err != nil {
		t.Fatalf("expected IVDD-OP-102 to exist, got error: %v", err)
	}
	if !strings.EqualFold(op102.PetName, "strudel") || op102.WeightKg != 5.8 || op102.Status != domain.SpinalStatusHemilaminectomyStaged {
		t.Errorf("unexpected IVDD-OP-102 metadata: %+v", op102)
	}
	if len(op102.NeuroChecks) != 1 {
		t.Errorf("expected 1 seeded neuro check on IVDD-OP-102, got %d", len(op102.NeuroChecks))
	}

	op103, err := svc.GetOperation("IVDD-OP-103")
	if err != nil {
		t.Fatalf("expected IVDD-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Pickles" || op103.WeightKg != 11.2 || op103.Status != domain.SpinalStatusPostopBladderCleared {
		t.Errorf("unexpected IVDD-OP-103 metadata: %+v", op103)
	}
	if op103.DischargedBy != "Dr. Elena Rostova, DACVIM (Neurology)" || op103.DischargedAt == nil || len(op103.NeuroChecks) != 2 {
		t.Errorf("unexpected IVDD-OP-103 discharge state: %+v", op103)
	}

	triageOps := svc.ListOperations(string(domain.SpinalStatusAcuteParaplegiaTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "IVDD-OP-101" {
		t.Errorf("expected 1 filtered operation IVDD-OP-101, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcuteParaplegiaTriageCount != 1 ||
		metrics.HemilaminectomyStagedCount != 1 ||
		metrics.ClearedPostopCount != 1 ||
		metrics.DeepPainAbsentEmergencyCount != 1 ||
		metrics.SchiffSherringtonCount != 1 {
		t.Errorf("unexpected overview metrics: %+v", metrics)
	}
	if aliasMetrics := svc.ComputeMetrics(); aliasMetrics != metrics {
		t.Errorf("expected ComputeMetrics to match GetOverviewMetrics")
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestIVDDServiceCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := spinalivdd.NewIVDDService()

	// Create cervical operation with default fields
	createdCervical, err := svc.CreateOperation(spinalivdd.CreateOperationInput{
		PetName:                  "Waffles",
		WeightKg:                 9.5,
		FrankelGrade:             domain.FrankelGrade3NonAmbulatoryParaparesis,
		LocalizationZone:         domain.LocalizationC1C5CervicalTetraparesis,
		OnsetHours:               6.0,
		DeepPainPresent:          true,
		SchiffSherringtonPresent: false,
	})
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if createdCervical.ID != "IVDD-OP-104" || createdCervical.Status != domain.SpinalStatusAcuteParaplegiaTriage {
		t.Errorf("unexpected created cervical op: %+v", createdCervical)
	}
	if !strings.Contains(createdCervical.DecompressionPlan.HemilaminectomySite, "Ventral Slot") {
		t.Errorf("expected Cervical Ventral Slot plan, got %+v", createdCervical.DecompressionPlan)
	}

	// Create lumbosacral Grade 5 operation
	createdLS, err := svc.CreateOperation(spinalivdd.CreateOperationInput{
		CaseID:                   "CASE-IVDD-2299",
		PetName:                  "Biscuit",
		Species:                  "Canine",
		Breed:                    "Beagle",
		WeightKg:                 13.0,
		FrankelGrade:             domain.FrankelGrade5DeepPainAbsent,
		LocalizationZone:         domain.LocalizationL4S3LumbosacralLMN,
		OnsetHours:               3.0,
		CutaneousTrunciVertebra:  "L3",
		DeepPainPresent:          true, // should be forced false by Grade 5
		SchiffSherringtonPresent: true,
		PriorSteroidOrNSAID:      "Prednisone 1 mg/kg PO given 4h ago",
		SurgicalWindowHours:      0,
		TargetMAPMmHg:            88,
	})
	if err != nil {
		t.Fatalf("unexpected CreateOperation LS error: %v", err)
	}
	if createdLS.DeepPainPresent || createdLS.SurgicalWindowHours != 12.0 {
		t.Errorf("expected DeepPainPresent=false and SurgicalWindowHours=12.0 for Grade 5, got %+v", createdLS)
	}
	if !strings.Contains(createdLS.DecompressionPlan.BladderManagementProtocol, "LMN Bladder") {
		t.Errorf("expected LMN bladder protocol for L4-S3, got %q", createdLS.DecompressionPlan.BladderManagementProtocol)
	}

	// Stage decompression on IVDD-OP-101 with explicit fields and fallback empty fields
	staged, err := svc.StageDecompression(
		"IVDD-OP-101",
		"Emergent Decompression < 12h (MAP >= 85 mmHg)",
		"3D Helical CT + 1.5T Thoracolumbar MRI",
		"Left T13–L1 Hemilaminectomy + Fenestration",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageDecompression error: %v", err)
	}
	if staged.Status != domain.SpinalStatusHemilaminectomyStaged ||
		staged.DecompressionPlan.HemilaminectomySite != "Left T13–L1 Hemilaminectomy + Fenestration" ||
		staged.DecompressionPlan.BladderManagementProtocol == "" {
		t.Errorf("unexpected staged decompression state: %+v", staged)
	}

	// Record neuro check on IVDD-OP-101 showing return of deep pain post-op
	checked, err := svc.RecordNeuroCheck(
		"IVDD-OP-101",
		domain.FrankelGrade3NonAmbulatoryParaparesis,
		"Intact Bilaterally — Conscious Head Turn & Whimper on Medial/Lateral Digit Clamp",
		"L2 (Caudal Improvement)",
		"UMN Tone Improving — Manual Expression Smooth",
		"Dr. Elena Rostova, DACVIM (Neurology)",
		"Post-hemilaminectomy recovery: deep pain perception restored bilaterally, cutaneous trunci improved caudally to L2.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordNeuroCheck error: %v", err)
	}
	if !checked.DeepPainPresent || checked.FrankelGrade != domain.FrankelGrade3NonAmbulatoryParaparesis || checked.CutaneousTrunciVertebra != "L2 (Caudal Improvement)" || len(checked.NeuroChecks) != 3 {
		t.Errorf("unexpected checked state: %+v", checked)
	}

	// Clear neurology discharge on IVDD-OP-101
	cleared, err := svc.ClearNeurologyDischarge(
		"IVDD-OP-101",
		"Dr. Elena Rostova, DACVIM (Neurology)",
		"Post-op T13–L1 hemilaminectomy nociception intact bilaterally, bladder expression/voiding protocol cleared, 4-week strict crate rest.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearNeurologyDischarge error: %v", err)
	}
	if cleared.Status != domain.SpinalStatusPostopBladderCleared || cleared.DischargedAt == nil || cleared.DischargedBy == "" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}

	// Also test ClearNeurologyDischarge upgrading Grade 5 directly
	clearedLS, err := svc.ClearNeurologyDischarge(
		createdLS.ID,
		"Dr. Marcus Vance, DACVECC",
		"Recovered nociception and voluntary voiding post-laminectomy.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearNeurologyDischarge on LS error: %v", err)
	}
	if clearedLS.FrankelGrade != domain.FrankelGrade3NonAmbulatoryParaparesis || !clearedLS.DeepPainPresent {
		t.Errorf("expected FrankelGrade upgraded to Grade 3 and DeepPainPresent true on discharge, got %+v", clearedLS)
	}
}

func TestIVDDServiceValidationErrors(t *testing.T) {
	svc := spinalivdd.NewIVDDService()

	validInput := spinalivdd.CreateOperationInput{
		PetName:          "Rocky",
		Species:          "Canine",
		Breed:            "Dachshund",
		WeightKg:         7.2,
		FrankelGrade:     domain.FrankelGrade4DeepPainIntact,
		LocalizationZone: domain.LocalizationT3L3ThoracolumbarUMN,
		OnsetHours:       5.0,
		TargetMAPMmHg:    85,
	}

	badInputs := []struct {
		name   string
		mutate func(in *spinalivdd.CreateOperationInput)
	}{
		{"empty pet_name", func(in *spinalivdd.CreateOperationInput) { in.PetName = "  " }},
		{"weight <= 0", func(in *spinalivdd.CreateOperationInput) { in.WeightKg = 0 }},
		{"weight > 120", func(in *spinalivdd.CreateOperationInput) { in.WeightKg = 150 }},
		{"invalid frankel grade", func(in *spinalivdd.CreateOperationInput) { in.FrankelGrade = "GRADE_99" }},
		{"invalid localization zone", func(in *spinalivdd.CreateOperationInput) { in.LocalizationZone = "UNKNOWN_ZONE" }},
		{"negative onset hours", func(in *spinalivdd.CreateOperationInput) { in.OnsetHours = -1 }},
		{"onset hours > 720", func(in *spinalivdd.CreateOperationInput) { in.OnsetHours = 800 }},
		{"target MAP too low", func(in *spinalivdd.CreateOperationInput) { in.TargetMAPMmHg = 30 }},
		{"target MAP too high", func(in *spinalivdd.CreateOperationInput) { in.TargetMAPMmHg = 200 }},
	}

	for _, tc := range badInputs {
		t.Run(tc.name, func(t *testing.T) {
			in := validInput
			tc.mutate(&in)
			if _, err := svc.CreateOperation(in); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageDecompression("UNKNOWN", "a", "b", "c", "d", "e", "f"); err == nil {
		t.Errorf("expected StageDecompression error for unknown ID")
	}

	if _, err := svc.RecordNeuroCheck("UNKNOWN", domain.FrankelGrade4DeepPainIntact, "Intact", "L1", "UMN", "Dr. A", "note"); err == nil {
		t.Errorf("expected RecordNeuroCheck error for unknown ID")
	}
	if _, err := svc.RecordNeuroCheck("IVDD-OP-101", "BAD_GRADE", "Intact", "L1", "UMN", "Dr. A", "note"); err == nil {
		t.Errorf("expected RecordNeuroCheck error for bad grade")
	}
	if _, err := svc.RecordNeuroCheck("IVDD-OP-101", domain.FrankelGrade4DeepPainIntact, "", "L1", "UMN", "Dr. A", "note"); err == nil {
		t.Errorf("expected RecordNeuroCheck error for empty deep_pain_status")
	}
	if _, err := svc.RecordNeuroCheck("IVDD-OP-101", domain.FrankelGrade4DeepPainIntact, "Intact", "", "UMN", "Dr. A", "note"); err == nil {
		t.Errorf("expected RecordNeuroCheck error for empty cutaneous_trunci_level")
	}
	if _, err := svc.RecordNeuroCheck("IVDD-OP-101", domain.FrankelGrade4DeepPainIntact, "Intact", "L1", "", "Dr. A", "note"); err == nil {
		t.Errorf("expected RecordNeuroCheck error for empty bladder_tone_status")
	}
	if _, err := svc.RecordNeuroCheck("IVDD-OP-101", domain.FrankelGrade4DeepPainIntact, "Intact", "L1", "UMN", "", "note"); err == nil {
		t.Errorf("expected RecordNeuroCheck error for empty recorded_by")
	}

	if _, err := svc.ClearNeurologyDischarge("UNKNOWN", "Dr. A", "summary"); err == nil {
		t.Errorf("expected ClearNeurologyDischarge error for unknown ID")
	}
	if _, err := svc.ClearNeurologyDischarge("IVDD-OP-101", "", "summary"); err == nil {
		t.Errorf("expected ClearNeurologyDischarge error for empty discharged_by")
	}
	if _, err := svc.ClearNeurologyDischarge("IVDD-OP-101", "Dr. A", ""); err == nil {
		t.Errorf("expected ClearNeurologyDischarge error for empty summary")
	}

	// Domain Validate() coverage
	op101, _ := svc.GetOperation("IVDD-OP-101")
	badOps := []func(o *domain.SpinalIVDDOperation){
		func(o *domain.SpinalIVDDOperation) { o.ID = "" },
		func(o *domain.SpinalIVDDOperation) { o.CaseID = "" },
		func(o *domain.SpinalIVDDOperation) { o.PetName = "" },
		func(o *domain.SpinalIVDDOperation) { o.Species = "" },
		func(o *domain.SpinalIVDDOperation) { o.WeightKg = 0 },
		func(o *domain.SpinalIVDDOperation) { o.Status = "BAD" },
		func(o *domain.SpinalIVDDOperation) { o.FrankelGrade = "BAD" },
		func(o *domain.SpinalIVDDOperation) { o.LocalizationZone = "BAD" },
		func(o *domain.SpinalIVDDOperation) { o.OnsetHours = -1 },
		func(o *domain.SpinalIVDDOperation) { o.TargetMAPMmHg = 20 },
	}
	for _, mutate := range badOps {
		cp := *op101
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected SpinalIVDDOperation.Validate error for %+v", cp)
		}
	}

	chk := op101.NeuroChecks[0]
	if err := chk.Validate(); err != nil {
		t.Errorf("expected valid NeuroNociceptionCheck, got %v", err)
	}
	badChks := []func(c *domain.NeuroNociceptionCheck){
		func(c *domain.NeuroNociceptionCheck) { c.FrankelGrade = "BAD" },
		func(c *domain.NeuroNociceptionCheck) { c.DeepPainStatus = "" },
		func(c *domain.NeuroNociceptionCheck) { c.CutaneousTrunciLevel = "" },
		func(c *domain.NeuroNociceptionCheck) { c.BladderToneStatus = "" },
		func(c *domain.NeuroNociceptionCheck) { c.RecordedBy = "" },
	}
	for _, mutate := range badChks {
		cp := chk
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected NeuroNociceptionCheck.Validate error for %+v", cp)
		}
	}
}
