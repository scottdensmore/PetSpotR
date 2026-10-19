package fipgs441524_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/fipgs441524"
)

func TestFIPSeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := fipgs441524.NewFIPService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded FIP operations, got %d", len(ops))
	}

	// Verify FIP-OP-101 ("Bramble")
	op101, err := svc.GetOperation("FIP-OP-101")
	if err != nil {
		t.Fatalf("expected FIP-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Bramble" || op101.Species != "Feline" || op101.Breed != "British Shorthair Kitten" || op101.WeightKg != 2.4 {
		t.Errorf("unexpected FIP-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.FIPStatusAcuteTriage || op101.Phenotype != domain.FIPPhenotypeEffusiveCavitary {
		t.Errorf("unexpected FIP-OP-101 status/phenotype: %s / %s", op101.Status, op101.Phenotype)
	}
	if op101.TreatmentDay != 1 || op101.AlbuminGlobulinRatio != 0.28 || op101.TotalBilirubinMgDL != 2.6 || op101.HematocritPercent != 21.0 || op101.DailyGS441524MgKg != 10.0 {
		t.Errorf("unexpected FIP-OP-101 metrics: %+v", op101)
	}
	if !op101.RivaltaTestPositive || !op101.PleuralEffusionDyspneaPresent {
		t.Errorf("expected RivaltaTestPositive=true and PleuralEffusionDyspneaPresent=true on FIP-OP-101")
	}
	if len(op101.ProgressChecks) != 2 {
		t.Errorf("expected 2 seeded ProgressChecks on FIP-OP-101, got %d", len(op101.ProgressChecks))
	}
	if !strings.Contains(op101.BiomarkerPlan.FluoroquinoloneRetinalToxicityWarning, "L-Lysine") {
		t.Errorf("expected L-Lysine & Enrofloxacin warning on FIP-OP-101, got %q", op101.BiomarkerPlan.FluoroquinoloneRetinalToxicityWarning)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected FIP-OP-101 to validate, got %v", err)
	}

	// Verify FIP-OP-102 ("Cosmo")
	op102, err := svc.GetOperation("FIP-OP-102")
	if err != nil {
		t.Fatalf("expected FIP-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Cosmo" || op102.Breed != "Ragdoll" || op102.WeightKg != 3.6 {
		t.Errorf("unexpected FIP-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.FIPStatusGS441524Staged || op102.Phenotype != domain.FIPPhenotypeNeurologicalCNS {
		t.Errorf("unexpected FIP-OP-102 status/phenotype: %s / %s", op102.Status, op102.Phenotype)
	}
	if op102.TreatmentDay != 28 || op102.AlbuminGlobulinRatio != 0.48 || op102.TotalBilirubinMgDL != 0.6 || op102.HematocritPercent != 29.0 || op102.DailyGS441524MgKg != 20.0 {
		t.Errorf("unexpected FIP-OP-102 metrics: %+v", op102)
	}
	if !op102.RivaltaTestPositive || op102.PleuralEffusionDyspneaPresent {
		t.Errorf("unexpected FIP-OP-102 flags: %+v", op102)
	}
	if len(op102.ProgressChecks) != 1 {
		t.Errorf("expected 1 seeded ProgressCheck on FIP-OP-102, got %d", len(op102.ProgressChecks))
	}

	// Verify FIP-OP-103 ("Saffron")
	op103, err := svc.GetOperation("FIP-OP-103")
	if err != nil {
		t.Fatalf("expected FIP-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Saffron" || op103.Species != "Feline" || op103.Breed != "Abyssinian" || op103.WeightKg != 4.1 {
		t.Errorf("unexpected FIP-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.FIPStatusCleared84DayCured || op103.Phenotype != domain.FIPPhenotypeOcularUveitis {
		t.Errorf("unexpected FIP-OP-103 status/phenotype: %s / %s", op103.Status, op103.Phenotype)
	}
	if op103.TreatmentDay != 84 || op103.AlbuminGlobulinRatio != 0.78 || op103.TotalBilirubinMgDL != 0.2 || op103.HematocritPercent != 38.0 || op103.DailyGS441524MgKg != 15.0 {
		t.Errorf("unexpected FIP-OP-103 metrics: %+v", op103)
	}
	if op103.DischargedBy != "Dr. Linnea Lindholm, DACVIM (Feline Medicine)" || op103.DischargedAt == nil || len(op103.ProgressChecks) != 2 {
		t.Errorf("expected FIP-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.FIPStatusAcuteTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "FIP-OP-101" {
		t.Errorf("expected 1 ACUTE_EFFUSIVE_NEURO_FIP_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcuteFIPTriageCount != 1 ||
		metrics.GS441524StagedCount != 1 ||
		metrics.Cleared84DayCuredCount != 1 ||
		metrics.NeuroOcularHighDoseCount != 2 ||
		metrics.AGRatioUnder04Count != 1 {
		t.Errorf("unexpected FIPOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown FIP operation ID")
	}
}

func TestFIPCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := fipgs441524.NewFIPService()

	// Create operation with default species/breed fallback
	created, err := svc.CreateOperation(
		"CASE-FIP-104",
		"Mochi",
		"",
		"",
		2.8,
		domain.FIPPhenotypeEffusiveCavitary,
		1,
		0.32,
		2.1,
		23.5,
		10.0,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "FIP-OP-104" || created.Species != "Feline" || created.Breed != "Domestic Shorthair" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.AlbuminGlobulinRatio != 0.32 || created.TreatmentDay != 1 {
		t.Errorf("expected AG=0.32 and TreatmentDay=1, got %.2f / %d", created.AlbuminGlobulinRatio, created.TreatmentDay)
	}

	// Stage antiviral protocol with empty optional fields (tests fallback defaults)
	staged, err := svc.StageAntiviralProtocol(
		created.ID,
		"GS-441524 10 mg/kg/day (28 mg PO q24h or 14 mg PO q12h) for 84 days",
		"Weigh weekly on pediatric scale and recalculate GS-441524 dose upward; never reduce dose",
		"Immediate therapeutic thoracocentesis for pleural dyspnea; never drain > 20-30% abdominal ascites",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageAntiviralProtocol error: %v", err)
	}
	if staged.Status != domain.FIPStatusGS441524Staged {
		t.Errorf("expected status %s, got %s", domain.FIPStatusGS441524Staged, staged.Status)
	}
	if staged.AntiviralPlan.OcularTopicalAtropinePredNote == "" ||
		staged.AntiviralPlan.HepatoprotectantB12SupportPlan == "" ||
		staged.AntiviralPlan.Day28Day56Day84MonitoringPlan == "" {
		t.Errorf("expected default ocular, hepatoprotectant, and monitoring notes to be populated: %+v", staged.AntiviralPlan)
	}

	// Stage antiviral protocol with explicit optional fields
	stagedExplicit, err := svc.StageAntiviralProtocol(
		created.ID,
		"GS-441524 12 mg/kg/day PO divided q12h",
		"Recalculate weekly upward for weight gain",
		"Thoracocentesis only; avoid complete ascites drainage",
		"Topical Prednisolone Acetate 1% q8h + Atropine 1% ophthalmic",
		"SAMe/Silybin (Denamarin) 90 mg PO q24h + Cyanocobalamin 250 ug SC weekly",
		"Day 28, Day 56, and Day 84 CBC/Chem/A:G Ratio/AGP checkpoints",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageAntiviralProtocol error: %v", err)
	}
	if stagedExplicit.AntiviralPlan.OcularTopicalAtropinePredNote != "Topical Prednisolone Acetate 1% q8h + Atropine 1% ophthalmic" {
		t.Errorf("unexpected explicit ocular support note: %q", stagedExplicit.AntiviralPlan.OcularTopicalAtropinePredNote)
	}

	// Record progress check and verify operation metrics update
	checked, err := svc.RecordProgressCheck(
		created.ID,
		84,
		3.9,
		0.74,
		0.2,
		37.0,
		"Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
		"Day 84 (Week 12) Check: Effusion resolved, weight increased from 2.8 to 3.9 kg, A:G Ratio normalized to 0.74 (> 0.60), Total Bilirubin 0.2 mg/dL, HCT 37.0%.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordProgressCheck error: %v", err)
	}
	if len(checked.ProgressChecks) != 1 {
		t.Fatalf("expected 1 ProgressCheck on FIP-OP-104, got %d", len(checked.ProgressChecks))
	}
	if checked.TreatmentDay != 84 || checked.WeightKg != 3.9 || checked.AlbuminGlobulinRatio != 0.74 || checked.TotalBilirubinMgDL != 0.2 || checked.HematocritPercent != 37.0 {
		t.Errorf("expected operation metrics updated from ProgressCheck, got %+v", checked)
	}

	// Clear FIP discharge
	cleared, err := svc.ClearFIPDischarge(
		created.ID,
		"Dr. Linnea Lindholm, DACVIM (Feline Medicine)",
		"Completed 84-day GS-441524 nucleoside analog protocol; A:G Ratio normalized to 0.74, bilirubin 0.2 mg/dL, HCT 37.0%, cleared for 84-day FIP clinical cure.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearFIPDischarge error: %v", err)
	}
	if cleared.Status != domain.FIPStatusCleared84DayCured || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Linnea Lindholm, DACVIM (Feline Medicine)" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestFIPValidationErrors(t *testing.T) {
	svc := fipgs441524.NewFIPService()

	createCases := []struct {
		name         string
		caseID       string
		petName      string
		weightKg     float64
		phenotype    domain.FIPClinicalPhenotype
		treatmentDay int
		agRatio      float64
		bilirubin    float64
		hct          float64
		gsDose       float64
	}{
		{"empty case_id", "", "Bramble", 2.4, domain.FIPPhenotypeEffusiveCavitary, 1, 0.28, 2.6, 21.0, 10.0},
		{"empty pet_name", "CASE-1", "", 2.4, domain.FIPPhenotypeEffusiveCavitary, 1, 0.28, 2.6, 21.0, 10.0},
		{"weight <= 0", "CASE-1", "Bramble", 0, domain.FIPPhenotypeEffusiveCavitary, 1, 0.28, 2.6, 21.0, 10.0},
		{"weight > 25", "CASE-1", "Bramble", 30.0, domain.FIPPhenotypeEffusiveCavitary, 1, 0.28, 2.6, 21.0, 10.0},
		{"invalid phenotype", "CASE-1", "Bramble", 2.4, "INVALID_PHENOTYPE", 1, 0.28, 2.6, 21.0, 10.0},
		{"treatmentDay < 1", "CASE-1", "Bramble", 2.4, domain.FIPPhenotypeEffusiveCavitary, 0, 0.28, 2.6, 21.0, 10.0},
		{"treatmentDay > 120", "CASE-1", "Bramble", 2.4, domain.FIPPhenotypeEffusiveCavitary, 125, 0.28, 2.6, 21.0, 10.0},
		{"agRatio <= 0", "CASE-1", "Bramble", 2.4, domain.FIPPhenotypeEffusiveCavitary, 1, 0, 2.6, 21.0, 10.0},
		{"bilirubin < 0", "CASE-1", "Bramble", 2.4, domain.FIPPhenotypeEffusiveCavitary, 1, 0.28, -0.5, 21.0, 10.0},
		{"hct <= 0", "CASE-1", "Bramble", 2.4, domain.FIPPhenotypeEffusiveCavitary, 1, 0.28, 2.6, 0, 10.0},
		{"gsDose <= 0", "CASE-1", "Bramble", 2.4, domain.FIPPhenotypeEffusiveCavitary, 1, 0.28, 2.6, 21.0, 0},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Feline", "British Shorthair",
				tc.weightKg, tc.phenotype, tc.treatmentDay,
				tc.agRatio, tc.bilirubin, tc.hct, tc.gsDose,
				true, true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageAntiviralProtocol validation errors
	if _, err := svc.StageAntiviralProtocol("FIP-OP-101", "", "Recalc", "Centesis", "", "", ""); err == nil {
		t.Errorf("expected error for empty gs441524Dosing")
	}
	if _, err := svc.StageAntiviralProtocol("FIP-OP-101", "Dosing", "", "Centesis", "", "", ""); err == nil {
		t.Errorf("expected error for empty weightRecalcNote")
	}
	if _, err := svc.StageAntiviralProtocol("FIP-OP-101", "Dosing", "Recalc", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty centesisLimit")
	}
	if _, err := svc.StageAntiviralProtocol("NON-EXISTENT", "Dosing", "Recalc", "Centesis", "", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageAntiviralProtocol")
	}

	// RecordProgressCheck validation errors
	if _, err := svc.RecordProgressCheck("FIP-OP-101", 0, 2.5, 0.45, 0.8, 28.0, "Dr. Lindholm", "note"); err == nil {
		t.Errorf("expected error for invalid treatment_day")
	}
	if _, err := svc.RecordProgressCheck("FIP-OP-101", 14, 0, 0.45, 0.8, 28.0, "Dr. Lindholm", "note"); err == nil {
		t.Errorf("expected error for non-positive weight_kg")
	}
	if _, err := svc.RecordProgressCheck("FIP-OP-101", 14, 2.5, 0, 0.8, 28.0, "Dr. Lindholm", "note"); err == nil {
		t.Errorf("expected error for non-positive ag_ratio")
	}
	if _, err := svc.RecordProgressCheck("FIP-OP-101", 14, 2.5, 0.45, -1.0, 28.0, "Dr. Lindholm", "note"); err == nil {
		t.Errorf("expected error for negative bilirubin")
	}
	if _, err := svc.RecordProgressCheck("FIP-OP-101", 14, 2.5, 0.45, 0.8, 0, "Dr. Lindholm", "note"); err == nil {
		t.Errorf("expected error for non-positive hematocrit")
	}
	if _, err := svc.RecordProgressCheck("FIP-OP-101", 14, 2.5, 0.45, 0.8, 28.0, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordProgressCheck("NON-EXISTENT", 14, 2.5, 0.45, 0.8, 28.0, "Dr. Lindholm", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordProgressCheck")
	}

	// ClearFIPDischarge validation errors
	if _, err := svc.ClearFIPDischarge("FIP-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearFIPDischarge("FIP-OP-101", "Dr. Lindholm", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearFIPDischarge("NON-EXISTENT", "Dr. Lindholm", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearFIPDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("FIP-OP-101")
	badOps := []func(op *domain.FIPGS441524Operation){
		func(op *domain.FIPGS441524Operation) { op.ID = "" },
		func(op *domain.FIPGS441524Operation) { op.CaseID = "" },
		func(op *domain.FIPGS441524Operation) { op.PetName = "" },
		func(op *domain.FIPGS441524Operation) { op.WeightKg = 0 },
		func(op *domain.FIPGS441524Operation) { op.Status = "BAD_STATUS" },
		func(op *domain.FIPGS441524Operation) { op.Phenotype = "BAD_PHENOTYPE" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildRivaltaBiomarkerAssessmentPlan & BuildDefaultNucleosideAntiviralProtocolPlan default branches
	planDefault := fipgs441524.BuildRivaltaBiomarkerAssessmentPlan("CUSTOM_PHENOTYPE", 0.50, 0.5, 32.0, 10.0, false, false)
	if !strings.Contains(planDefault.PhenotypePathophysiology, "Feline Infectious Peritonitis") {
		t.Errorf("expected default pathophysiology summary, got %q", planDefault.PhenotypePathophysiology)
	}
	for _, ph := range []domain.FIPClinicalPhenotype{
		domain.FIPPhenotypeEffusiveCavitary,
		domain.FIPPhenotypeNeurologicalCNS,
		domain.FIPPhenotypeOcularUveitis,
	} {
		defAnti := fipgs441524.BuildDefaultNucleosideAntiviralProtocolPlan(3.0, ph, 0, false)
		if defAnti.GS441524OrRemdesivirDosing == "" {
			t.Errorf("expected non-empty default dosing for phenotype %s", ph)
		}
	}
}
