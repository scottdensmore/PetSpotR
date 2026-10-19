package fatethrombus_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/fatethrombus"
)

func TestFATESeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := fatethrombus.NewFATEService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded FATE operations, got %d", len(ops))
	}

	// Verify FATE-OP-101 ("Jasper")
	op101, err := svc.GetOperation("FATE-OP-101")
	if err != nil {
		t.Fatalf("expected FATE-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Jasper" || op101.Species != "Feline" || op101.Breed != "Maine Coon" || op101.WeightKg != 6.8 {
		t.Errorf("unexpected FATE-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.FATEStatusAcuteSaddle5PsTriage || op101.DistributionType != domain.ThrombusDistBilateralAorticSaddle {
		t.Errorf("unexpected FATE-OP-101 status/distribution: %s / %s", op101.Status, op101.DistributionType)
	}
	if op101.SystemicGlucoseMgDL != 185.0 || op101.AffectedLimbGlucoseMgDL != 92.0 || op101.GlucoseDeltaMgDL != 93.0 {
		t.Errorf("unexpected FATE-OP-101 glucose values: sys=%.1f limb=%.1f delta=%.1f", op101.SystemicGlucoseMgDL, op101.AffectedLimbGlucoseMgDL, op101.GlucoseDeltaMgDL)
	}
	if op101.SystemicLactateMmolL != 2.4 || op101.AffectedLimbLactateMmolL != 11.8 || op101.LactateDeltaMmolL != 9.4 {
		t.Errorf("unexpected FATE-OP-101 lactate values: sys=%.1f limb=%.1f delta=%.1f", op101.SystemicLactateMmolL, op101.AffectedLimbLactateMmolL, op101.LactateDeltaMmolL)
	}
	if op101.SerumPotassiumMmolL != 4.6 || op101.DistalLimbTempC != 29.2 || op101.LA2AoRatio != 2.35 || !op101.ConcurrentCardiogenicCHF {
		t.Errorf("unexpected FATE-OP-101 electrolytes/temp/echo: %+v", op101)
	}
	if len(op101.PerfusionChecks) != 2 {
		t.Errorf("expected 2 seeded PerfusionChecks on FATE-OP-101, got %d", len(op101.PerfusionChecks))
	}
	if !strings.Contains(op101.GradientPlan.SystemicToLimbGlucoseDelta, "DIAGNOSTIC FOR FATE") {
		t.Errorf("expected diagnostic glucose delta note on FATE-OP-101, got %q", op101.GradientPlan.SystemicToLimbGlucoseDelta)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected FATE-OP-101 to validate, got %v", err)
	}

	// Verify FATE-OP-102 ("Cleo")
	op102, err := svc.GetOperation("FATE-OP-102")
	if err != nil {
		t.Fatalf("expected FATE-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Cleo" || op102.Breed != "Ragdoll" || op102.WeightKg != 4.9 {
		t.Errorf("unexpected FATE-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.FATEStatusMethadoneClopidogrelStaged || op102.DistributionType != domain.ThrombusDistUnilateralIliacFemoral {
		t.Errorf("unexpected FATE-OP-102 status/distribution: %s / %s", op102.Status, op102.DistributionType)
	}
	if op102.GlucoseDeltaMgDL != 44.0 || op102.LactateDeltaMmolL != 4.4 || op102.SerumPotassiumMmolL != 5.1 || op102.DistalLimbTempC != 34.1 || op102.LA2AoRatio != 1.95 || op102.ConcurrentCardiogenicCHF {
		t.Errorf("unexpected FATE-OP-102 metrics: %+v", op102)
	}
	if len(op102.PerfusionChecks) != 1 {
		t.Errorf("expected 1 seeded PerfusionCheck on FATE-OP-102, got %d", len(op102.PerfusionChecks))
	}

	// Verify FATE-OP-103 ("Figaro")
	op103, err := svc.GetOperation("FATE-OP-103")
	if err != nil {
		t.Fatalf("expected FATE-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Figaro" || op103.Breed != "Domestic Longhair" || op103.WeightKg != 5.2 {
		t.Errorf("unexpected FATE-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.FATEStatusReperfusedNormokalemicCleared || op103.DistributionType != domain.ThrombusDistRightForeBrachial {
		t.Errorf("unexpected FATE-OP-103 status/distribution: %s / %s", op103.Status, op103.DistributionType)
	}
	if op103.DischargedBy != "Dr. Julian Vance, DACVIM (Cardiology)" || op103.DischargedAt == nil || len(op103.PerfusionChecks) != 2 {
		t.Errorf("expected FATE-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.FATEStatusAcuteSaddle5PsTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "FATE-OP-101" {
		t.Errorf("expected 1 ACUTE_SADDLE_THROMBUS_5PS_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcuteSaddleTriageCount != 1 ||
		metrics.AnticoagStagedCount != 1 ||
		metrics.ClearedReperfusedCount != 1 ||
		metrics.BilateralSaddleCount != 1 ||
		metrics.ConcurrentCHFCount != 1 {
		t.Errorf("unexpected FATEOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown FATE operation ID")
	}
}

func TestFATECreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := fatethrombus.NewFATEService()

	// Create operation with default species/breed fallback
	created, err := svc.CreateOperation(
		"CASE-FATE-104",
		"Oliver",
		"",
		"",
		5.6,
		domain.ThrombusDistBilateralAorticSaddle,
		192.0, 88.0,
		2.2, 12.6,
		4.8, 28.9, 2.40,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "FATE-OP-104" || created.Species != "Feline" || created.Breed != "Domestic Shorthair" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	if created.GlucoseDeltaMgDL != 104.0 || created.LactateDeltaMmolL != 10.4 {
		t.Errorf("expected glucoseDelta=104.0 and lactateDelta=10.4, got %.1f / %.1f", created.GlucoseDeltaMgDL, created.LactateDeltaMmolL)
	}

	// Stage antithrombotic protocol with empty optional fields (tests fallback defaults)
	staged, err := svc.StageAntithrombotic(
		created.ID,
		"Fentanyl 3 mcg/kg IV bolus + 4 mcg/kg/hr CRI",
		"Clopidogrel 75 mg PO loading dose (FAT CAT Trial), then 18.75 mg PO q24h",
		"Enoxaparin 0.9 mg/kg SC q8h transitioning to Rivaroxaban 2.5 mg PO q24h",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageAntithrombotic error: %v", err)
	}
	if staged.Status != domain.FATEStatusMethadoneClopidogrelStaged {
		t.Errorf("expected status %s, got %s", domain.FATEStatusMethadoneClopidogrelStaged, staged.Status)
	}
	if staged.AntithromboticPlan.ConcurrentCHFFurosemidePlan == "" ||
		staged.AntithromboticPlan.SerialPotassiumMonitoring == "" ||
		staged.AntithromboticPlan.PimobendanLVOTNote == "" {
		t.Errorf("expected default CHF, potassium, and pimobendan notes to be populated: %+v", staged.AntithromboticPlan)
	}

	// Stage antithrombotic protocol with explicit optional fields
	stagedExplicit, err := svc.StageAntithrombotic(
		created.ID,
		"Methadone 0.25 mg/kg IV q4h",
		"Clopidogrel 75 mg PO load",
		"Rivaroxaban 2.5 mg PO q24h",
		"Furosemide 2 mg/kg IV q6h + Oxygen Cage 45%",
		"Serial K+ q2h on venous blood gas",
		"Hold Pimobendan until DLVOTO ruled out",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageAntithrombotic error: %v", err)
	}
	if stagedExplicit.AntithromboticPlan.ConcurrentCHFFurosemidePlan != "Furosemide 2 mg/kg IV q6h + Oxygen Cage 45%" {
		t.Errorf("unexpected explicit CHF plan: %q", stagedExplicit.AntithromboticPlan.ConcurrentCHFFurosemidePlan)
	}

	// Record perfusion check and verify operation metrics update
	checked, err := svc.RecordPerfusionCheck(
		created.ID,
		14.0,
		1.2,
		4.2,
		37.4,
		"BILATERAL_FEMORAL_DOPPLER_RESTORED",
		"Dr. Maya Lin, DVM, DACVECC",
		"Reperfusion progressing without hyperkalemic washout; bilateral femoral Doppler flow audible and nail beds pink.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordPerfusionCheck error: %v", err)
	}
	if len(checked.PerfusionChecks) != 1 {
		t.Fatalf("expected 1 PerfusionCheck on FATE-OP-104, got %d", len(checked.PerfusionChecks))
	}
	if checked.GlucoseDeltaMgDL != 14.0 || checked.LactateDeltaMmolL != 1.2 || checked.SerumPotassiumMmolL != 4.2 || checked.DistalLimbTempC != 37.4 {
		t.Errorf("expected operation vitals updated from PerfusionCheck, got %+v", checked)
	}

	// Clear FATE discharge
	cleared, err := svc.ClearFATEDischarge(
		created.ID,
		"Dr. Julian Vance, DACVIM (Cardiology)",
		"Bilateral pelvic limb motor function restored, normokalemic K+ 4.2 mmol/L, glucose delta 14.0 mg/dL, discharged on Clopidogrel 18.75 mg PO q24h + Rivaroxaban 2.5 mg PO q24h.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearFATEDischarge error: %v", err)
	}
	if cleared.Status != domain.FATEStatusReperfusedNormokalemicCleared || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Julian Vance, DACVIM (Cardiology)" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestFATEValidationErrors(t *testing.T) {
	svc := fatethrombus.NewFATEService()

	// CreateOperation validation errors
	createCases := []struct {
		name        string
		caseID      string
		petName     string
		weightKg    float64
		distType    domain.ThrombusDistributionType
		sysGlucose  float64
		limbGlucose float64
		sysLactate  float64
		limbLactate float64
		potassium   float64
		limbTempC   float64
		la2ao       float64
	}{
		{"empty case_id", "", "Jasper", 5.0, domain.ThrombusDistBilateralAorticSaddle, 180, 90, 2.0, 9.0, 4.5, 30.0, 2.1},
		{"empty pet_name", "CASE-1", "", 5.0, domain.ThrombusDistBilateralAorticSaddle, 180, 90, 2.0, 9.0, 4.5, 30.0, 2.1},
		{"weight <= 0", "CASE-1", "Jasper", 0, domain.ThrombusDistBilateralAorticSaddle, 180, 90, 2.0, 9.0, 4.5, 30.0, 2.1},
		{"weight > 50", "CASE-1", "Jasper", 60, domain.ThrombusDistBilateralAorticSaddle, 180, 90, 2.0, 9.0, 4.5, 30.0, 2.1},
		{"invalid distType", "CASE-1", "Jasper", 5.0, "INVALID_DIST", 180, 90, 2.0, 9.0, 4.5, 30.0, 2.1},
		{"glucose <= 0", "CASE-1", "Jasper", 5.0, domain.ThrombusDistBilateralAorticSaddle, 0, 90, 2.0, 9.0, 4.5, 30.0, 2.1},
		{"lactate < 0", "CASE-1", "Jasper", 5.0, domain.ThrombusDistBilateralAorticSaddle, 180, 90, -1.0, 9.0, 4.5, 30.0, 2.1},
		{"potassium out of range", "CASE-1", "Jasper", 5.0, domain.ThrombusDistBilateralAorticSaddle, 180, 90, 2.0, 9.0, 1.5, 30.0, 2.1},
		{"temp out of range", "CASE-1", "Jasper", 5.0, domain.ThrombusDistBilateralAorticSaddle, 180, 90, 2.0, 9.0, 4.5, 15.0, 2.1},
		{"la2ao out of range", "CASE-1", "Jasper", 5.0, domain.ThrombusDistBilateralAorticSaddle, 180, 90, 2.0, 9.0, 4.5, 30.0, 0.5},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Feline", "Maine Coon",
				tc.weightKg, tc.distType,
				tc.sysGlucose, tc.limbGlucose,
				tc.sysLactate, tc.limbLactate,
				tc.potassium, tc.limbTempC, tc.la2ao,
				true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageAntithrombotic validation errors
	if _, err := svc.StageAntithrombotic("FATE-OP-101", "", "Clopidogrel", "Rivaroxaban", "", "", ""); err == nil {
		t.Errorf("expected error for empty opioidProtocol")
	}
	if _, err := svc.StageAntithrombotic("FATE-OP-101", "Methadone", "", "Rivaroxaban", "", "", ""); err == nil {
		t.Errorf("expected error for empty clopidogrelLoading")
	}
	if _, err := svc.StageAntithrombotic("FATE-OP-101", "Methadone", "Clopidogrel", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty factorXaProtocol")
	}
	if _, err := svc.StageAntithrombotic("NON-EXISTENT", "Methadone", "Clopidogrel", "Rivaroxaban", "", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageAntithrombotic")
	}

	// RecordPerfusionCheck validation errors
	if _, err := svc.RecordPerfusionCheck("FATE-OP-101", -1, 2.0, 4.5, 35.0, "PRESENT", "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for negative glucose delta")
	}
	if _, err := svc.RecordPerfusionCheck("FATE-OP-101", 10, -1.0, 4.5, 35.0, "PRESENT", "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for negative lactate delta")
	}
	if _, err := svc.RecordPerfusionCheck("FATE-OP-101", 10, 2.0, 1.0, 35.0, "PRESENT", "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for potassium out of range")
	}
	if _, err := svc.RecordPerfusionCheck("FATE-OP-101", 10, 2.0, 4.5, 10.0, "PRESENT", "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for limb temp out of range")
	}
	if _, err := svc.RecordPerfusionCheck("FATE-OP-101", 10, 2.0, 4.5, 35.0, "", "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for empty femoral pulse status")
	}
	if _, err := svc.RecordPerfusionCheck("FATE-OP-101", 10, 2.0, 4.5, 35.0, "PRESENT", "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordPerfusionCheck("NON-EXISTENT", 10, 2.0, 4.5, 35.0, "PRESENT", "Dr. Lin", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordPerfusionCheck")
	}

	// ClearFATEDischarge validation errors
	if _, err := svc.ClearFATEDischarge("FATE-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearFATEDischarge("FATE-OP-101", "Dr. Vance", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearFATEDischarge("NON-EXISTENT", "Dr. Vance", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearFATEDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("FATE-OP-101")
	badOps := []func(op *domain.FATEThrombusOperation){
		func(op *domain.FATEThrombusOperation) { op.ID = "" },
		func(op *domain.FATEThrombusOperation) { op.CaseID = "" },
		func(op *domain.FATEThrombusOperation) { op.PetName = "" },
		func(op *domain.FATEThrombusOperation) { op.WeightKg = 0 },
		func(op *domain.FATEThrombusOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.FATEThrombusOperation) { op.DistributionType = "BAD_DIST" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildMetabolicGradientPlan default switch branch coverage
	planDefault := fatethrombus.BuildMetabolicGradientPlan("CUSTOM_DIST", 120, 100, 20, 1.5, 2.5, 1.0, 4.2, 36.5, 1.5, false)
	if !strings.Contains(planDefault.FivePsClinicalSummary, "5 Ps") {
		t.Errorf("expected default 5 Ps summary, got %q", planDefault.FivePsClinicalSummary)
	}
}
