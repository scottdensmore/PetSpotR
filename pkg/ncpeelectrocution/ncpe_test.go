package ncpeelectrocution_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ncpeelectrocution"
)

func TestNCPESeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := ncpeelectrocution.NewNCPEService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded NCPE operations, got %d", len(ops))
	}

	// Verify NCPE-OP-101 ("Sparky")
	op101, err := svc.GetOperation("NCPE-OP-101")
	if err != nil {
		t.Fatalf("expected NCPE-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Sparky" || op101.Species != "Canine" || op101.Breed != "Golden Retriever Puppy" || op101.WeightKg != 6.4 {
		t.Errorf("unexpected NCPE-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.NCPEStatusAcuteCaudodorsalTriage || op101.EtiologyClass != domain.NCPEEtiologyOralElectricalCordBite {
		t.Errorf("unexpected NCPE-OP-101 status/etiology_class: %s / %s", op101.Status, op101.EtiologyClass)
	}
	if op101.PaO2FiO2Ratio != 165.0 || op101.SpO2Percent != 86.0 || op101.RespiratoryRateBpm != 68 || op101.CaudodorsalBLinesCount != 14 || op101.LA2AoRatio != 1.22 {
		t.Errorf("unexpected NCPE-OP-101 metrics: %+v", op101)
	}
	if !op101.OralElectricalEscharPresent || !op101.PinkFrothyAirwayFluid {
		t.Errorf("expected OralElectricalEscharPresent=true and PinkFrothyAirwayFluid=true on NCPE-OP-101")
	}
	if len(op101.RespiratoryChecks) != 2 {
		t.Errorf("expected 2 seeded RespiratoryChecks on NCPE-OP-101, got %d", len(op101.RespiratoryChecks))
	}
	if !strings.Contains(op101.AssessmentPlan.HighDoseFurosemideContraindication, "Furosemide") {
		t.Errorf("expected high-dose furosemide contraindication on NCPE-OP-101, got %q", op101.AssessmentPlan.HighDoseFurosemideContraindication)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected NCPE-OP-101 to validate, got %v", err)
	}

	// Verify NCPE-OP-102 ("Milo")
	op102, err := svc.GetOperation("NCPE-OP-102")
	if err != nil {
		t.Fatalf("expected NCPE-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Milo" || op102.Breed != "Pug" || op102.WeightKg != 8.9 {
		t.Errorf("unexpected NCPE-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.NCPEStatusOxygenCPAPStaged || op102.EtiologyClass != domain.NCPEEtiologyPostObstructiveChoking {
		t.Errorf("unexpected NCPE-OP-102 status/etiology_class: %s / %s", op102.Status, op102.EtiologyClass)
	}
	if op102.PaO2FiO2Ratio != 235.0 || op102.SpO2Percent != 93.0 || op102.RespiratoryRateBpm != 42 || op102.CaudodorsalBLinesCount != 7 || op102.LA2AoRatio != 1.18 {
		t.Errorf("unexpected NCPE-OP-102 metrics: %+v", op102)
	}
	if op102.OralElectricalEscharPresent || op102.PinkFrothyAirwayFluid {
		t.Errorf("expected OralElectricalEscharPresent=false and PinkFrothyAirwayFluid=false on NCPE-OP-102")
	}
	if len(op102.RespiratoryChecks) != 1 {
		t.Errorf("expected 1 seeded RespiratoryCheck on NCPE-OP-102, got %d", len(op102.RespiratoryChecks))
	}

	// Verify NCPE-OP-103 ("Luna")
	op103, err := svc.GetOperation("NCPE-OP-103")
	if err != nil {
		t.Fatalf("expected NCPE-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Luna" || op103.Species != "Feline" || op103.Breed != "Siamese" || op103.WeightKg != 3.8 {
		t.Errorf("unexpected NCPE-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.NCPEStatusBLinesResolvedCleared || op103.EtiologyClass != domain.NCPEEtiologyPostIctalCatecholamineSurge {
		t.Errorf("unexpected NCPE-OP-103 status/etiology_class: %s / %s", op103.Status, op103.EtiologyClass)
	}
	if op103.PaO2FiO2Ratio != 410.0 || op103.SpO2Percent != 98.0 || op103.RespiratoryRateBpm != 26 || op103.CaudodorsalBLinesCount != 0 || op103.LA2AoRatio != 1.15 {
		t.Errorf("unexpected NCPE-OP-103 metrics: %+v", op103)
	}
	if op103.DischargedBy != "Dr. Nadia Varma, DACVECC" || op103.DischargedAt == nil || len(op103.RespiratoryChecks) != 2 {
		t.Errorf("expected NCPE-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.NCPEStatusAcuteCaudodorsalTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "NCPE-OP-101" {
		t.Errorf("expected 1 ACUTE_CAUDODORSAL_NCPE_HYPOXEMIA_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcuteNCPETriageCount != 1 ||
		metrics.CPAPOxygenStagedCount != 1 ||
		metrics.ClearedRoomAirCount != 1 ||
		metrics.OralCordElectrocutionCount != 1 ||
		metrics.SeverePaO2FiO2Under200Count != 1 {
		t.Errorf("unexpected NCPEOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown NCPE operation ID")
	}
}

func TestNCPECreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := ncpeelectrocution.NewNCPEService()

	// Create operation with default species/breed fallback
	created, err := svc.CreateOperation(
		"CASE-NCPE-104",
		"Volt",
		"",
		"",
		7.2,
		domain.NCPEEtiologyOralElectricalCordBite,
		155.0,
		85.0,
		72,
		16,
		1.20,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "NCPE-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.PaO2FiO2Ratio != 155.0 || created.CaudodorsalBLinesCount != 16 {
		t.Errorf("expected PaO2/FiO2=155.0 and B-lines=16, got %.1f / %d", created.PaO2FiO2Ratio, created.CaudodorsalBLinesCount)
	}

	// Stage CPAP ventilation with empty optional fields (tests fallback defaults)
	staged, err := svc.StageCPAPVentilation(
		created.ID,
		"High-Flow Nasal Cannula (HFNC) at 2.0 L/kg/min warm humidified O2 (FiO2 60%)",
		"Butorphanol 0.2 mg/kg IV q4h + Acepromazine 0.01 mg/kg IV for anxiolysis",
		"",
		"Helmet CPAP (6 cmH2O PEEP) / Mechanical Ventilation with PEEP 5–10 cmH2O for PaO2/FiO2 < 200",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageCPAPVentilation error: %v", err)
	}
	if staged.Status != domain.NCPEStatusOxygenCPAPStaged {
		t.Errorf("expected status %s, got %s", domain.NCPEStatusOxygenCPAPStaged, staged.Status)
	}
	if staged.VentilationPlan.LowDoseFurosemideBronchodilatorNote == "" ||
		staged.VentilationPlan.ConservativeFluidMaintenancePlan == "" ||
		staged.VentilationPlan.OralCommissureBurnAnalgesiaPlan == "" {
		t.Errorf("expected default furosemide, fluid, and oral burn notes to be populated: %+v", staged.VentilationPlan)
	}

	// Stage CPAP ventilation with explicit optional fields
	stagedExplicit, err := svc.StageCPAPVentilation(
		created.ID,
		"HFNC 2.0 L/kg/min FiO2 60%",
		"Butorphanol 0.2 mg/kg IV q4h",
		"Strictly avoid 4–8 mg/kg Furosemide; optional 0.5 mg/kg IV once for bronchodilation",
		"Helmet CPAP 7 cmH2O PEEP",
		"Restrict IV fluids to 1.5 mL/kg/hr",
		"Methadone 0.2 mg/kg IV q4h + 0.12% Chlorhexidine oral rinse + Sucralfate slurry",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageCPAPVentilation error: %v", err)
	}
	if stagedExplicit.VentilationPlan.LowDoseFurosemideBronchodilatorNote != "Strictly avoid 4–8 mg/kg Furosemide; optional 0.5 mg/kg IV once for bronchodilation" {
		t.Errorf("unexpected explicit low-dose furosemide note: %q", stagedExplicit.VentilationPlan.LowDoseFurosemideBronchodilatorNote)
	}

	// Record respiratory check and verify operation metrics update
	checked, err := svc.RecordRespiratoryCheck(
		created.ID,
		395.0,
		98.0,
		28,
		0,
		1.20,
		"Dr. Nadia Varma, DACVECC",
		"Post-HFNC, Helmet CPAP & Butorphanol anxiolysis: caudodorsal B-lines resolved (0 B-lines), PaO2/FiO2 395, SpO2 98% on room air.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordRespiratoryCheck error: %v", err)
	}
	if len(checked.RespiratoryChecks) != 1 {
		t.Fatalf("expected 1 RespiratoryCheck on NCPE-OP-104, got %d", len(checked.RespiratoryChecks))
	}
	if checked.PaO2FiO2Ratio != 395.0 || checked.SpO2Percent != 98.0 || checked.RespiratoryRateBpm != 28 || checked.CaudodorsalBLinesCount != 0 || checked.LA2AoRatio != 1.20 {
		t.Errorf("expected operation vitals updated from RespiratoryCheck, got %+v", checked)
	}

	// Clear NCPE discharge
	cleared, err := svc.ClearNCPEDischarge(
		created.ID,
		"Dr. Nadia Varma, DACVECC",
		"Caudodorsal B-lines completely resolved (0 B-lines), normoxemic on room air (PaO2/FiO2 395, SpO2 98%, RR 28 bpm), 5–7 day oral eschar monitoring scheduled.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearNCPEDischarge error: %v", err)
	}
	if cleared.Status != domain.NCPEStatusBLinesResolvedCleared || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Nadia Varma, DACVECC" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestNCPEValidationErrors(t *testing.T) {
	svc := ncpeelectrocution.NewNCPEService()

	createCases := []struct {
		name          string
		caseID        string
		petName       string
		weightKg      float64
		etiologyClass domain.NCPEEtiologyClassification
		pfRatio       float64
		spo2          float64
		rr            int
		blines        int
		la2ao         float64
	}{
		{"empty case_id", "", "Sparky", 6.4, domain.NCPEEtiologyOralElectricalCordBite, 165.0, 86.0, 68, 14, 1.22},
		{"empty pet_name", "CASE-1", "", 6.4, domain.NCPEEtiologyOralElectricalCordBite, 165.0, 86.0, 68, 14, 1.22},
		{"weight <= 0", "CASE-1", "Sparky", 0, domain.NCPEEtiologyOralElectricalCordBite, 165.0, 86.0, 68, 14, 1.22},
		{"weight > 120", "CASE-1", "Sparky", 130, domain.NCPEEtiologyOralElectricalCordBite, 165.0, 86.0, 68, 14, 1.22},
		{"invalid etiology_class", "CASE-1", "Sparky", 6.4, "INVALID_ETIOLOGY", 165.0, 86.0, 68, 14, 1.22},
		{"pfRatio <= 0", "CASE-1", "Sparky", 6.4, domain.NCPEEtiologyOralElectricalCordBite, 0, 86.0, 68, 14, 1.22},
		{"spo2 < 40", "CASE-1", "Sparky", 6.4, domain.NCPEEtiologyOralElectricalCordBite, 165.0, 20.0, 68, 14, 1.22},
		{"rr <= 0", "CASE-1", "Sparky", 6.4, domain.NCPEEtiologyOralElectricalCordBite, 165.0, 86.0, 0, 14, 1.22},
		{"blines < 0", "CASE-1", "Sparky", 6.4, domain.NCPEEtiologyOralElectricalCordBite, 165.0, 86.0, 68, -1, 1.22},
		{"la2ao <= 0", "CASE-1", "Sparky", 6.4, domain.NCPEEtiologyOralElectricalCordBite, 165.0, 86.0, 68, 14, 0},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "Golden Retriever",
				tc.weightKg, tc.etiologyClass,
				tc.pfRatio, tc.spo2, tc.rr, tc.blines, tc.la2ao,
				true, true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageCPAPVentilation validation errors
	if _, err := svc.StageCPAPVentilation("NCPE-OP-101", "", "Butorphanol", "", "CPAP", "", ""); err == nil {
		t.Errorf("expected error for empty oxygenProtocol")
	}
	if _, err := svc.StageCPAPVentilation("NCPE-OP-101", "O2", "", "", "CPAP", "", ""); err == nil {
		t.Errorf("expected error for empty anxiolyticPlan")
	}
	if _, err := svc.StageCPAPVentilation("NCPE-OP-101", "O2", "Butorphanol", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty cpapPeepPlan")
	}
	if _, err := svc.StageCPAPVentilation("NON-EXISTENT", "O2", "Butorphanol", "", "CPAP", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageCPAPVentilation")
	}

	// RecordRespiratoryCheck validation errors
	if _, err := svc.RecordRespiratoryCheck("NCPE-OP-101", 0, 95.0, 30, 2, 1.2, "Dr. Varma", "note"); err == nil {
		t.Errorf("expected error for non-positive pfRatio")
	}
	if _, err := svc.RecordRespiratoryCheck("NCPE-OP-101", 300.0, 30.0, 30, 2, 1.2, "Dr. Varma", "note"); err == nil {
		t.Errorf("expected error for spo2 < 40")
	}
	if _, err := svc.RecordRespiratoryCheck("NCPE-OP-101", 300.0, 95.0, 0, 2, 1.2, "Dr. Varma", "note"); err == nil {
		t.Errorf("expected error for non-positive respiratory rate")
	}
	if _, err := svc.RecordRespiratoryCheck("NCPE-OP-101", 300.0, 95.0, 30, -1, 1.2, "Dr. Varma", "note"); err == nil {
		t.Errorf("expected error for negative caudodorsal B-lines")
	}
	if _, err := svc.RecordRespiratoryCheck("NCPE-OP-101", 300.0, 95.0, 30, 2, 0, "Dr. Varma", "note"); err == nil {
		t.Errorf("expected error for non-positive la2ao ratio")
	}
	if _, err := svc.RecordRespiratoryCheck("NCPE-OP-101", 300.0, 95.0, 30, 2, 1.2, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordRespiratoryCheck("NON-EXISTENT", 300.0, 95.0, 30, 2, 1.2, "Dr. Varma", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordRespiratoryCheck")
	}

	// ClearNCPEDischarge validation errors
	if _, err := svc.ClearNCPEDischarge("NCPE-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearNCPEDischarge("NCPE-OP-101", "Dr. Varma", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearNCPEDischarge("NON-EXISTENT", "Dr. Varma", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearNCPEDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("NCPE-OP-101")
	badOps := []func(op *domain.NCPEElectrocutionOperation){
		func(op *domain.NCPEElectrocutionOperation) { op.ID = "" },
		func(op *domain.NCPEElectrocutionOperation) { op.CaseID = "" },
		func(op *domain.NCPEElectrocutionOperation) { op.PetName = "" },
		func(op *domain.NCPEElectrocutionOperation) { op.WeightKg = 0 },
		func(op *domain.NCPEElectrocutionOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.NCPEElectrocutionOperation) { op.EtiologyClass = "BAD_ETIOLOGY" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildCaudodorsalNCPEAssessmentPlan default & borderline LA:Ao switch branch coverage
	planDefault := ncpeelectrocution.BuildCaudodorsalNCPEAssessmentPlan("CUSTOM_ETIOLOGY", 250.0, 95.0, 32, 2, 1.65, false, false)
	if !strings.Contains(planDefault.EtiologyPathophysiology, "Non-Cardiogenic") {
		t.Errorf("expected default pathophysiology summary, got %q", planDefault.EtiologyPathophysiology)
	}
}
