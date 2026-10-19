package akidialysis_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/akidialysis"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestAKIDialysisSeededOperationsAndMetrics(t *testing.T) {
	svc := akidialysis.NewAKIService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("AKI-OP-101")
	if err != nil {
		t.Fatalf("expected AKI-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Lotus" || op101.IncidentID != "INC-AKI-9931" || op101.Status != domain.AKIStatusOliguricAzotemiaCritical {
		t.Errorf("unexpected AKI-OP-101 metadata: %+v", op101)
	}
	if !op101.AnuricFluidOverloadFomepizoleAlert || op101.PatientProfile.WeightKg != 4.5 || op101.PatientProfile.IRISGrade != 5 || op101.PatientProfile.SerumCreatinineMgDl != 11.4 || op101.PatientProfile.SerumPotassiumMeqL != 7.4 || op101.PatientProfile.UrineOutputMlKgHr != 0.2 || op101.PatientProfile.BodyWeightGainPercent != 9.5 || op101.PatientProfile.CalciumOxalateMonohydrateCrystals {
		t.Errorf("unexpected AKI-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.DiuresisPlan == nil || !op101.DiuresisPlan.HemodialysisIndicated || !op101.DiuresisPlan.ZeroForceFluidInAnuriaGuard {
		t.Errorf("unexpected AKI-OP-101 plan: %+v", op101.DiuresisPlan)
	}
	// Feline 4.5 kg * 125 mg/kg = 562.5 mg; InsAndOuts = (0.2*4.5) + (20*4.5/24) = 0.9 + 3.75 = 4.65 mL/hr; Furosemide = 9.0 mg; Diltiazem = 0.3 mcg/kg/min
	if op101.DiuresisPlan.FomepizoleLoadingDoseMg != 562.5 || op101.DiuresisPlan.InsAndOutsFluidRateMlHr != 4.65 || op101.DiuresisPlan.FurosemideConversionBolusMg != 9.0 || op101.DiuresisPlan.DiltiazemCRI03McgKgMin != 0.3 {
		t.Errorf("unexpected AKI-OP-101 plan values: %+v", op101.DiuresisPlan)
	}
	lowerAlert := strings.ToLower(op101.DiuresisPlan.AKIPitfallAlert)
	if !strings.Contains(lowerAlert, "ins-and-outs") || !strings.Contains(lowerAlert, "125 mg/kg") || !strings.Contains(lowerAlert, "hemodialysis") {
		t.Errorf("expected pitfall alert to warn against forced fluids (Ins-and-Outs), feline 125 mg/kg Fomepizole, and hemodialysis delay, got %q", op101.DiuresisPlan.AKIPitfallAlert)
	}
	if len(op101.RenalChecks) != 1 {
		t.Errorf("expected 1 initial renal check on AKI-OP-101, got %d", len(op101.RenalChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected AKI-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("AKI-OP-102")
	if err != nil {
		t.Fatalf("expected AKI-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Diesel" || op102.IncidentID != "INC-AKI-9932" || op102.Status != domain.AKIStatusFomepizoleDiuresisStaged {
		t.Errorf("unexpected AKI-OP-102 metadata: %+v", op102)
	}
	// Canine Ethylene Glycol 28.0 kg * 20 mg/kg = 560.0 mg; InsAndOuts = (1.4*28) + (20*28/24) = 39.2 + 23.33 = 62.53 mL/hr; UOP 1.4 >= 1.0 so Furosemide/Diltiazem = 0
	if op102.DiuresisPlan == nil || op102.DiuresisPlan.FomepizoleLoadingDoseMg != 560.0 || op102.DiuresisPlan.InsAndOutsFluidRateMlHr != 62.53 || op102.DiuresisPlan.FurosemideConversionBolusMg != 0.0 || op102.DiuresisPlan.DiltiazemCRI03McgKgMin != 0.0 || op102.DiuresisPlan.HemodialysisIndicated {
		t.Errorf("expected canine 20 mg/kg (560.0 mg) Fomepizole, 62.53 mL/hr Ins-and-Outs, and 0 Furosemide/Diltiazem on AKI-OP-102, got %+v", op102.DiuresisPlan)
	}

	op103, err := svc.GetOperation("AKI-OP-103")
	if err != nil {
		t.Fatalf("expected AKI-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Merlot" || op103.IncidentID != "INC-AKI-9933" || op103.Status != domain.AKIStatusPolyuricNonAzotemicCleared {
		t.Errorf("unexpected AKI-OP-103 metadata: %+v", op103)
	}
	if op103.AnuricFluidOverloadFomepizoleAlert || op103.DischargeClearance == nil {
		t.Errorf("expected AKI-OP-103 to have DischargeClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.AKIStatusOliguricAzotemiaCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "AKI-OP-101" {
		t.Errorf("expected 1 critical op AKI-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.OliguricAzotemiaCriticalCount != 1 || metrics.FomepizoleDiuresisStagedCount != 1 || metrics.PolyuricClearedCount != 1 || metrics.AnuricOverloadAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestAKIDialysisCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := akidialysis.NewAKIService()

	profilePinot := domain.AKIPatientProfile{
		Species:                           domain.AKISpeciesCanineVitisGrapeLeptospirosis,
		Breed:                             "Golden Retriever",
		WeightKg:                          24.0,
		IRISGrade:                         4,
		SerumCreatinineMgDl:               6.8,
		SerumPotassiumMeqL:                6.6,
		UrineOutputMlKgHr:                 0.4,
		BodyWeightGainPercent:             6.0,
		CalciumOxalateMonohydrateCrystals: true,
	}

	created, err := svc.CreateOperation("INC-AKI-9934", "Pinot", profilePinot)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "AKI-OP-104" || created.Status != domain.AKIStatusOliguricAzotemiaCritical || !created.AnuricFluidOverloadFomepizoleAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	// 24 kg with CalciumOxalateMonohydrateCrystals=true -> Fomepizole 480.0 mg; InsAndOuts = (0.4*24) + (20*24/24) = 9.6 + 20.0 = 29.6 mL/hr; Furosemide = 48.0 mg; Diltiazem = 0.3; HemodialysisIndicated = true
	if created.DiuresisPlan.FomepizoleLoadingDoseMg != 480.0 || created.DiuresisPlan.InsAndOutsFluidRateMlHr != 29.6 || created.DiuresisPlan.FurosemideConversionBolusMg != 48.0 || created.DiuresisPlan.DiltiazemCRI03McgKgMin != 0.3 || !created.DiuresisPlan.HemodialysisIndicated {
		t.Errorf("unexpected plan on created AKI-OP-104: %+v", created.DiuresisPlan)
	}

	// Stage plan on AKI-OP-104 after initial diuresis response
	stagedProfile := profilePinot
	stagedProfile.Breed = ""
	stagedProfile.UrineOutputMlKgHr = 1.6
	stagedProfile.SerumCreatinineMgDl = 3.8
	stagedProfile.SerumPotassiumMeqL = 5.1
	stagedProfile.CalciumOxalateMonohydrateCrystals = false

	staged, err := svc.StageDiuresisPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageDiuresisPlan error: %v", err)
	}
	if staged.Status != domain.AKIStatusFomepizoleDiuresisStaged {
		t.Errorf("expected status %s, got %s", domain.AKIStatusFomepizoleDiuresisStaged, staged.Status)
	}
	if staged.DiuresisPlan.FurosemideConversionBolusMg != 0.0 || staged.DiuresisPlan.DiltiazemCRI03McgKgMin != 0.0 || staged.DiuresisPlan.HemodialysisIndicated {
		t.Errorf("expected 0 Furosemide/Diltiazem and false HemodialysisIndicated when UOP 1.6, Cr 3.8, K+ 5.1, got %+v", staged.DiuresisPlan)
	}
	if staged.PatientProfile.Breed != "Golden Retriever" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record renal check with polyuric recovery & normalized Cr/K+
	checked, err := svc.RecordRenalCheck(created.ID, domain.AKIRenalTelemetryCheck{
		UrineOutputMlKgHr:   2.6,
		SerumCreatinineMgDl: 1.8,
		SerumPotassiumMeqL:  4.4,
		BodyWeightKg:        24.0,
		FluidInOutBalanceMl: 0.0,
		ClinicianCallsign:   "DR-RENAL-55",
		Notes:               "Polyuric conversion achieved (UOP 2.6 mL/kg/hr); Cr decreased to 1.8 mg/dL; K+ 4.4 mEq/L.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordRenalCheck error: %v", err)
	}
	if len(checked.RenalChecks) != 1 || checked.AnuricFluidOverloadFomepizoleAlert {
		t.Errorf("expected 1 renal check and false alert when UOP >= 1.5, Cr <= 2.5, K+ <= 5.5, got %+v", checked)
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.AKIDischargeClearance{
		FinalUrineOutputMlKgHr:       2.5,
		FinalSerumCreatinineMgDl:     1.6,
		FinalSerumPotassiumMeqL:      4.2,
		EuvolemicNoPulmonaryEdema:    true,
		OutpatientSubQFluidsEducated: true,
		VetLicense:                   "VET-WA-99320",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.AKIStatusPolyuricNonAzotemicCleared || cleared.AnuricFluidOverloadFomepizoleAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse check with oliguria & hyperkalemia reverts to critical
	relapsed, err := svc.RecordRenalCheck(created.ID, domain.AKIRenalTelemetryCheck{
		UrineOutputMlKgHr:   0.3,
		SerumCreatinineMgDl: 9.2,
		SerumPotassiumMeqL:  7.1,
		BodyWeightKg:        25.8,
		FluidInOutBalanceMl: 350.0,
		ClinicianCallsign:   "DR-RENAL-55",
		Notes:               "Acute oliguric relapse with hyperkalemia K+ 7.1 mEq/L.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordRenalCheck error: %v", err)
	}
	if relapsed.Status != domain.AKIStatusOliguricAzotemiaCritical || !relapsed.AnuricFluidOverloadFomepizoleAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to OLIGURIC_ANURIC_AZOTEMIA_CRITICAL, got %+v", relapsed)
	}
}

func TestAKIDialysisValidationErrors(t *testing.T) {
	svc := akidialysis.NewAKIService()

	validProfile := domain.AKIPatientProfile{
		Species:                           domain.AKISpeciesFelineLiliumNephrotoxicity,
		WeightKg:                          4.5,
		IRISGrade:                         4,
		SerumCreatinineMgDl:               6.5,
		SerumPotassiumMeqL:                6.2,
		UrineOutputMlKgHr:                 0.4,
		BodyWeightGainPercent:             5.0,
		CalciumOxalateMonohydrateCrystals: false,
	}

	if _, err := svc.CreateOperation("", "Lotus", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.AKIPatientProfile)
	}{
		{"invalid species", func(p *domain.AKIPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.AKIPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.AKIPatientProfile) { p.WeightKg = 140 }},
		{"iris grade too low", func(p *domain.AKIPatientProfile) { p.IRISGrade = 0 }},
		{"iris grade too high", func(p *domain.AKIPatientProfile) { p.IRISGrade = 6 }},
		{"creatinine too low", func(p *domain.AKIPatientProfile) { p.SerumCreatinineMgDl = 0.1 }},
		{"creatinine too high", func(p *domain.AKIPatientProfile) { p.SerumCreatinineMgDl = 35.0 }},
		{"potassium too low", func(p *domain.AKIPatientProfile) { p.SerumPotassiumMeqL = 1.5 }},
		{"potassium too high", func(p *domain.AKIPatientProfile) { p.SerumPotassiumMeqL = 14.0 }},
		{"uop too low", func(p *domain.AKIPatientProfile) { p.UrineOutputMlKgHr = -0.5 }},
		{"uop too high", func(p *domain.AKIPatientProfile) { p.UrineOutputMlKgHr = 30.0 }},
		{"weight gain too low", func(p *domain.AKIPatientProfile) { p.BodyWeightGainPercent = -20.0 }},
		{"weight gain too high", func(p *domain.AKIPatientProfile) { p.BodyWeightGainPercent = 30.0 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Lotus", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageDiuresisPlan("AKI-OP-101", p); err == nil {
				t.Errorf("expected StageDiuresisPlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageDiuresisPlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging plan on unknown op")
	}

	validCheck := domain.AKIRenalTelemetryCheck{
		UrineOutputMlKgHr:   1.8,
		SerumCreatinineMgDl: 2.2,
		SerumPotassiumMeqL:  4.5,
		BodyWeightKg:        4.5,
		FluidInOutBalanceMl: 10.0,
		ClinicianCallsign:   "DR-1",
	}
	if _, err := svc.RecordRenalCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording renal check on unknown op")
	}

	badChecks := []func(c *domain.AKIRenalTelemetryCheck){
		func(c *domain.AKIRenalTelemetryCheck) { c.UrineOutputMlKgHr = -1.0 },
		func(c *domain.AKIRenalTelemetryCheck) { c.SerumCreatinineMgDl = 0.1 },
		func(c *domain.AKIRenalTelemetryCheck) { c.SerumPotassiumMeqL = 1.0 },
		func(c *domain.AKIRenalTelemetryCheck) { c.BodyWeightKg = 0 },
		func(c *domain.AKIRenalTelemetryCheck) { c.FluidInOutBalanceMl = 2500.0 },
		func(c *domain.AKIRenalTelemetryCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordRenalCheck("AKI-OP-101", c); err == nil {
			t.Errorf("expected RecordRenalCheck validation error for %+v", c)
		}
	}

	validClearance := domain.AKIDischargeClearance{
		FinalUrineOutputMlKgHr:       2.2,
		FinalSerumCreatinineMgDl:     1.8,
		FinalSerumPotassiumMeqL:      4.4,
		EuvolemicNoPulmonaryEdema:    true,
		OutpatientSubQFluidsEducated: true,
		VetLicense:                   "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.AKIDischargeClearance){
		func(d *domain.AKIDischargeClearance) { d.FinalUrineOutputMlKgHr = 1.0 },
		func(d *domain.AKIDischargeClearance) { d.FinalUrineOutputMlKgHr = 25.0 },
		func(d *domain.AKIDischargeClearance) { d.FinalSerumCreatinineMgDl = 0.1 },
		func(d *domain.AKIDischargeClearance) { d.FinalSerumCreatinineMgDl = 3.2 },
		func(d *domain.AKIDischargeClearance) { d.FinalSerumPotassiumMeqL = 3.1 },
		func(d *domain.AKIDischargeClearance) { d.FinalSerumPotassiumMeqL = 6.0 },
		func(d *domain.AKIDischargeClearance) { d.EuvolemicNoPulmonaryEdema = false },
		func(d *domain.AKIDischargeClearance) { d.OutpatientSubQFluidsEducated = false },
		func(d *domain.AKIDischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("AKI-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}

	// Validate AKIDialysisOperation error paths
	op := domain.AKIDialysisOperation{
		ID:             "AKI-OP-101",
		IncidentID:     "INC-AKI-9931",
		PetName:        "Lotus",
		Status:         domain.AKIStatusOliguricAzotemiaCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.AKIDialysisOperation){
		func(o *domain.AKIDialysisOperation) { o.ID = "" },
		func(o *domain.AKIDialysisOperation) { o.IncidentID = "" },
		func(o *domain.AKIDialysisOperation) { o.PetName = "" },
		func(o *domain.AKIDialysisOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.AKIDialysisOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected AKIDialysisOperation.Validate error for %+v", cp)
		}
	}
}
