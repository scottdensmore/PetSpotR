package thermalburnparkland_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/thermalburnparkland"
)

func TestBurnSeededOperationsAndOverviewMetrics(t *testing.T) {
	svc := thermalburnparkland.NewBurnService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded Thermal Burn operations, got %d", len(ops))
	}

	// Verify BURN-OP-101 ("Ember")
	op101, err := svc.GetOperation("BURN-OP-101")
	if err != nil {
		t.Fatalf("expected BURN-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Ember" || op101.Species != "Canine" || op101.Breed != "Australian Cattle Dog" || op101.WeightKg != 20.0 {
		t.Errorf("unexpected BURN-OP-101 signalment: %+v", op101)
	}
	if op101.Status != domain.ThermalBurnStatusAcuteTriage || op101.DepthClass != domain.BurnDepthFullThicknessCircumferentialEschar {
		t.Errorf("unexpected BURN-OP-101 status/depth: %s / %s", op101.Status, op101.DepthClass)
	}
	if op101.TBSAPercent != 30.0 || op101.Total24hParklandML != 2400.0 || op101.First8hRateMLHr != 150.0 || op101.Next16hRateMLHr != 75.0 {
		t.Errorf("unexpected BURN-OP-101 Parkland parameters: tbsa=%.1f total=%.1f 8h=%.1f 16h=%.1f",
			op101.TBSAPercent, op101.Total24hParklandML, op101.First8hRateMLHr, op101.Next16hRateMLHr)
	}
	if op101.CurrentFluidRateMLHr != 150.0 || op101.UrineOutputMLKgHr != 0.8 || op101.SerumAlbuminGDL != 1.7 || op101.PCVPercent != 58.0 {
		t.Errorf("unexpected BURN-OP-101 fluid/UOP/albumin/PCV: %+v", op101)
	}
	if !op101.CircumferentialThoracicEschar || !op101.SingedVibrissaeSootAirway {
		t.Errorf("expected CircumferentialThoracicEschar=true and SingedVibrissaeSootAirway=true on BURN-OP-101")
	}
	if len(op101.BurnChecks) != 2 {
		t.Errorf("expected 2 seeded BurnChecks on BURN-OP-101, got %d", len(op101.BurnChecks))
	}
	if !strings.Contains(op101.ParklandPlan.Colloid24hCapillaryLeakWarning, "COLLOID") {
		t.Errorf("expected colloid warning on BURN-OP-101, got %q", op101.ParklandPlan.Colloid24hCapillaryLeakWarning)
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected BURN-OP-101 to validate, got %v", err)
	}

	// Verify BURN-OP-102 ("Cinder")
	op102, err := svc.GetOperation("BURN-OP-102")
	if err != nil {
		t.Fatalf("expected BURN-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Cinder" || op102.Species != "Feline" || op102.Breed != "Domestic Shorthair" || op102.WeightKg != 4.0 {
		t.Errorf("unexpected BURN-OP-102 signalment: %+v", op102)
	}
	if op102.Status != domain.ThermalBurnStatusParklandStaged || op102.DepthClass != domain.BurnDepthDeepPartialThicknessDermal {
		t.Errorf("unexpected BURN-OP-102 status/depth: %s / %s", op102.Status, op102.DepthClass)
	}
	if op102.TBSAPercent != 22.0 || op102.Total24hParklandML != 220.0 || op102.First8hRateMLHr != 13.8 || op102.Next16hRateMLHr != 6.9 {
		t.Errorf("unexpected BURN-OP-102 feline Parkland parameters: %+v", op102)
	}
	if op102.CurrentFluidRateMLHr != 13.8 || op102.UrineOutputMLKgHr != 1.4 || op102.SerumAlbuminGDL != 2.1 || op102.PCVPercent != 44.0 {
		t.Errorf("unexpected BURN-OP-102 vitals: %+v", op102)
	}
	if op102.CircumferentialThoracicEschar || !op102.SingedVibrissaeSootAirway {
		t.Errorf("unexpected BURN-OP-102 flags: %+v", op102)
	}
	if len(op102.BurnChecks) != 1 {
		t.Errorf("expected 1 seeded BurnCheck on BURN-OP-102, got %d", len(op102.BurnChecks))
	}

	// Verify BURN-OP-103 ("Blaze")
	op103, err := svc.GetOperation("BURN-OP-103")
	if err != nil {
		t.Fatalf("expected BURN-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Blaze" || op103.Species != "Canine" || op103.Breed != "Dalmatian" || op103.WeightKg != 24.0 {
		t.Errorf("unexpected BURN-OP-103 signalment: %+v", op103)
	}
	if op103.Status != domain.ThermalBurnStatusGranulationCleared || op103.DepthClass != domain.BurnDepthThermalAirwayCornealFlash {
		t.Errorf("unexpected BURN-OP-103 status/depth: %s / %s", op103.Status, op103.DepthClass)
	}
	if op103.TBSAPercent != 8.0 || op103.Total24hParklandML != 768.0 || op103.First8hRateMLHr != 48.0 || op103.Next16hRateMLHr != 24.0 {
		t.Errorf("unexpected BURN-OP-103 Parkland parameters: %+v", op103)
	}
	if op103.CurrentFluidRateMLHr != 35.0 || op103.UrineOutputMLKgHr != 1.9 || op103.SerumAlbuminGDL != 2.8 || op103.PCVPercent != 41.0 {
		t.Errorf("unexpected BURN-OP-103 vitals: %+v", op103)
	}
	if op103.DischargedBy != "Dr. Marcus Thorne, DACVS / Burn Reconstruction" || op103.DischargedAt == nil || len(op103.BurnChecks) != 2 {
		t.Errorf("expected BURN-OP-103 to have DischargedBy, DischargedAt, and 2 checks, got %+v", op103)
	}

	// Filter by status
	triageOps := svc.ListOperations(string(domain.ThermalBurnStatusAcuteTriage))
	if len(triageOps) != 1 || triageOps[0].ID != "BURN-OP-101" {
		t.Errorf("expected 1 ACUTE_TBSA_BURN_SMOKE_EDEMA_TRIAGE operation, got %+v", triageOps)
	}

	metrics := svc.GetOverviewMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.AcuteBurnTriageCount != 1 ||
		metrics.ParklandStagedCount != 1 ||
		metrics.ClearedGranulationCount != 1 ||
		metrics.SevereTBSAOver20PctCount != 2 ||
		metrics.CircumferentialEscharCount != 1 {
		t.Errorf("unexpected ThermalBurnOverviewMetrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN-ID"); err == nil {
		t.Errorf("expected error for unknown Thermal Burn operation ID")
	}
}

func TestBurnCreateStageCheckAndClearLifecycle(t *testing.T) {
	svc := thermalburnparkland.NewBurnService()

	// Create operation with default species/breed fallback and 0 currentFluidRateMLHr (auto-populated from First8hRateMLHr)
	created, err := svc.CreateOperation(
		"CASE-BURN-104",
		"Phoenix",
		"",
		"",
		18.0,
		domain.BurnDepthFullThicknessCircumferentialEschar,
		25.0,
		0,
		0.7,
		1.8,
		56.0,
		true,
		true,
	)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "BURN-OP-104" || created.Species != "Canine" || created.Breed != "Mixed Breed" {
		t.Errorf("unexpected created operation metadata: %+v", created)
	}
	// 4 mL * 18 kg * 25% TBSA = 1800 mL; First 8h = 900 / 8 = 112.5 mL/hr; Next 16h = 900 / 16 = 56.3 mL/hr
	if created.Total24hParklandML != 1800.0 || created.First8hRateMLHr != 112.5 || created.CurrentFluidRateMLHr != 112.5 {
		t.Errorf("expected Total24hParklandML=1800.0 and First8hRateMLHr=112.5, got %+v", created)
	}

	// Stage Parkland & wound plan with empty optional fields (tests fallback defaults)
	staged, err := svc.StageParklandWoundPlan(
		created.ID,
		"Parkland LRS 1800 mL/24h (First 8h at 112.5 mL/hr, Next 16h at 56.3 mL/hr); colloids withheld in first 24h",
		"Topical 1% Silver Sulfadiazine (SSD) + sterile Manuka Honey UMF 15+ barrier q12h",
		"Immediate bilateral lateral thoracic escharotomy through full-thickness eschar + staged wet-to-dry debridement",
		"",
		"",
		"",
	)
	if err != nil {
		t.Fatalf("unexpected StageParklandWoundPlan error: %v", err)
	}
	if staged.Status != domain.ThermalBurnStatusParklandStaged {
		t.Errorf("expected status %s, got %s", domain.ThermalBurnStatusParklandStaged, staged.Status)
	}
	if staged.WoundPlan.MultimodalBurnAnalgesiaCRI == "" ||
		staged.WoundPlan.AlbuminOncotic24hTransition == "" ||
		staged.WoundPlan.CornealThermalAirwayProtection == "" {
		t.Errorf("expected default analgesia, albumin transition, and corneal/airway notes to be populated: %+v", staged.WoundPlan)
	}

	// Stage Parkland & wound plan with explicit optional fields
	stagedExplicit, err := svc.StageParklandWoundPlan(
		created.ID,
		"Parkland LRS 112.5 mL/hr first 8h",
		"1% Silver Sulfadiazine cream q12h",
		"Thoracic escharotomy + hydrotherapy debridement",
		"Fentanyl 4 mcg/kg/hr + Ketamine 5 mcg/kg/min + Lidocaine 30 mcg/kg/min CRI",
		"Canine Albumin 5% infusion after 24h capillary seal",
		"Fluorescein corneal check + humidified O2 40% + N-acetylcysteine nebulization",
	)
	if err != nil {
		t.Fatalf("unexpected explicit StageParklandWoundPlan error: %v", err)
	}
	if stagedExplicit.WoundPlan.AlbuminOncotic24hTransition != "Canine Albumin 5% infusion after 24h capillary seal" {
		t.Errorf("unexpected explicit albumin transition plan: %q", stagedExplicit.WoundPlan.AlbuminOncotic24hTransition)
	}

	// Record burn check and verify operation metrics update
	checked, err := svc.RecordBurnCheck(
		created.ID,
		15.0,
		65.0,
		1.8,
		2.6,
		42.0,
		"Dr. Marcus Thorne, DACVS / Burn Reconstruction",
		"Post-escharotomy & 24h Parkland transition: chest wall compliance restored, UOP 1.8 mL/kg/hr, albumin improved to 2.6 g/dL, PCV normalized to 42.0%.",
	)
	if err != nil {
		t.Fatalf("unexpected RecordBurnCheck error: %v", err)
	}
	if len(checked.BurnChecks) != 1 {
		t.Fatalf("expected 1 BurnCheck on BURN-OP-104, got %d", len(checked.BurnChecks))
	}
	if checked.TBSAPercent != 15.0 || checked.CurrentFluidRateMLHr != 65.0 || checked.UrineOutputMLKgHr != 1.8 || checked.SerumAlbuminGDL != 2.6 || checked.PCVPercent != 42.0 {
		t.Errorf("expected operation vitals updated from BurnCheck, got %+v", checked)
	}

	// Clear burn discharge
	cleared, err := svc.ClearBurnDischarge(
		created.ID,
		"Dr. Marcus Thorne, DACVS / Burn Reconstruction",
		"Healthy granulation bed established over residual 15.0% TBSA, Urine Output 1.8 mL/kg/hr, Serum Albumin 2.6 g/dL, PCV 42.0%, cleared for reconstructive step-down.",
	)
	if err != nil {
		t.Fatalf("unexpected ClearBurnDischarge error: %v", err)
	}
	if cleared.Status != domain.ThermalBurnStatusGranulationCleared || cleared.DischargedAt == nil || cleared.DischargedBy != "Dr. Marcus Thorne, DACVS / Burn Reconstruction" {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
}

func TestBurnValidationErrors(t *testing.T) {
	svc := thermalburnparkland.NewBurnService()

	createCases := []struct {
		name       string
		caseID     string
		petName    string
		weightKg   float64
		depthClass domain.BurnDepthClassification
		tbsa       float64
		fluidRate  float64
		uop        float64
		albumin    float64
		pcv        float64
	}{
		{"empty case_id", "", "Ember", 20.0, domain.BurnDepthFullThicknessCircumferentialEschar, 30, 150, 0.8, 1.7, 58},
		{"empty pet_name", "CASE-1", "", 20.0, domain.BurnDepthFullThicknessCircumferentialEschar, 30, 150, 0.8, 1.7, 58},
		{"weight <= 0", "CASE-1", "Ember", 0, domain.BurnDepthFullThicknessCircumferentialEschar, 30, 150, 0.8, 1.7, 58},
		{"weight > 120", "CASE-1", "Ember", 130, domain.BurnDepthFullThicknessCircumferentialEschar, 30, 150, 0.8, 1.7, 58},
		{"invalid depth_class", "CASE-1", "Ember", 20.0, "INVALID_DEPTH", 30, 150, 0.8, 1.7, 58},
		{"tbsa <= 0", "CASE-1", "Ember", 20.0, domain.BurnDepthFullThicknessCircumferentialEschar, 0, 150, 0.8, 1.7, 58},
		{"tbsa > 100", "CASE-1", "Ember", 20.0, domain.BurnDepthFullThicknessCircumferentialEschar, 105, 150, 0.8, 1.7, 58},
		{"fluidRate < 0", "CASE-1", "Ember", 20.0, domain.BurnDepthFullThicknessCircumferentialEschar, 30, -5, 0.8, 1.7, 58},
		{"uop < 0", "CASE-1", "Ember", 20.0, domain.BurnDepthFullThicknessCircumferentialEschar, 30, 150, -0.5, 1.7, 58},
		{"albumin <= 0", "CASE-1", "Ember", 20.0, domain.BurnDepthFullThicknessCircumferentialEschar, 30, 150, 0.8, 0, 58},
		{"pcv <= 0", "CASE-1", "Ember", 20.0, domain.BurnDepthFullThicknessCircumferentialEschar, 30, 150, 0.8, 1.7, 0},
	}

	for _, tc := range createCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateOperation(
				tc.caseID, tc.petName, "Canine", "Cattle Dog",
				tc.weightKg, tc.depthClass,
				tc.tbsa, tc.fluidRate, tc.uop, tc.albumin, tc.pcv,
				true, true,
			)
			if err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
		})
	}

	// StageParklandWoundPlan validation errors
	if _, err := svc.StageParklandWoundPlan("BURN-OP-101", "", "SSD", "Escharotomy", "", "", ""); err == nil {
		t.Errorf("expected error for empty parklandSchedule")
	}
	if _, err := svc.StageParklandWoundPlan("BURN-OP-101", "Parkland", "", "Escharotomy", "", "", ""); err == nil {
		t.Errorf("expected error for empty ssdHoneyPlan")
	}
	if _, err := svc.StageParklandWoundPlan("BURN-OP-101", "Parkland", "SSD", "", "", "", ""); err == nil {
		t.Errorf("expected error for empty escharotomyPlan")
	}
	if _, err := svc.StageParklandWoundPlan("NON-EXISTENT", "Parkland", "SSD", "Escharotomy", "", "", ""); err == nil {
		t.Errorf("expected error for unknown operation ID in StageParklandWoundPlan")
	}

	// RecordBurnCheck validation errors
	if _, err := svc.RecordBurnCheck("BURN-OP-101", 0, 100, 1.5, 2.5, 45, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for non-positive tbsa_percent")
	}
	if _, err := svc.RecordBurnCheck("BURN-OP-101", 20, -10, 1.5, 2.5, 45, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for negative current_fluid_rate_ml_hr")
	}
	if _, err := svc.RecordBurnCheck("BURN-OP-101", 20, 100, -1.0, 2.5, 45, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for negative urine_output_ml_kg_hr")
	}
	if _, err := svc.RecordBurnCheck("BURN-OP-101", 20, 100, 1.5, 0, 45, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for non-positive serum_albumin_g_dl")
	}
	if _, err := svc.RecordBurnCheck("BURN-OP-101", 20, 100, 1.5, 2.5, 0, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for non-positive pcv_percent")
	}
	if _, err := svc.RecordBurnCheck("BURN-OP-101", 20, 100, 1.5, 2.5, 45, "", "note"); err == nil {
		t.Errorf("expected error for empty recorded_by")
	}
	if _, err := svc.RecordBurnCheck("NON-EXISTENT", 20, 100, 1.5, 2.5, 45, "Dr. Thorne", "note"); err == nil {
		t.Errorf("expected error for unknown operation ID in RecordBurnCheck")
	}

	// ClearBurnDischarge validation errors
	if _, err := svc.ClearBurnDischarge("BURN-OP-101", "", "summary"); err == nil {
		t.Errorf("expected error for empty dischargedBy")
	}
	if _, err := svc.ClearBurnDischarge("BURN-OP-101", "Dr. Thorne", ""); err == nil {
		t.Errorf("expected error for empty discharge summary")
	}
	if _, err := svc.ClearBurnDischarge("NON-EXISTENT", "Dr. Thorne", "summary"); err == nil {
		t.Errorf("expected error for unknown operation ID in ClearBurnDischarge")
	}

	// Domain Validate() coverage
	validOp, _ := svc.GetOperation("BURN-OP-101")
	badOps := []func(op *domain.ThermalBurnOperation){
		func(op *domain.ThermalBurnOperation) { op.ID = "" },
		func(op *domain.ThermalBurnOperation) { op.CaseID = "" },
		func(op *domain.ThermalBurnOperation) { op.PetName = "" },
		func(op *domain.ThermalBurnOperation) { op.WeightKg = 0 },
		func(op *domain.ThermalBurnOperation) { op.Status = "BAD_STATUS" },
		func(op *domain.ThermalBurnOperation) { op.DepthClass = "BAD_DEPTH" },
	}
	for _, mutate := range badOps {
		cp := *validOp
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected domain Validate() error for mutated op: %+v", cp)
		}
	}

	// BuildTBSAParklandAssessmentPlan default switch branch coverage
	planDefault := thermalburnparkland.BuildTBSAParklandAssessmentPlan("Canine", 20.0, "CUSTOM_DEPTH", 15, 1200, 75, 37.5, 75, 1.5, 2.5, 45, false, false)
	if !strings.Contains(planDefault.BurnDepthPathophysiology, "Thermal Burn") {
		t.Errorf("expected default pathophysiology summary, got %q", planDefault.BurnDepthPathophysiology)
	}
}
