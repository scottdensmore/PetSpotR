package septicperitonitis_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/septicperitonitis"
)

func TestSepticPeritonitisSeededOperationsAndMetrics(t *testing.T) {
	svc := septicperitonitis.NewPeritonitisService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("SP-OP-101")
	if err != nil {
		t.Fatalf("expected SP-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Baxter" || op101.IncidentID != "INC-SP-9501" || op101.Status != domain.PeritonitisStatusSepticEffusionCritical {
		t.Errorf("unexpected SP-OP-101 metadata: %+v", op101)
	}
	if !op101.SepticGradientAntibioticDelayAlert || op101.PatientProfile.WeightKg != 31.0 || op101.PatientProfile.BloodGlucoseMgDl != 118.0 || op101.PatientProfile.EffusionGlucoseMgDl != 42.0 {
		t.Errorf("unexpected SP-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.SurgicalPlan == nil ||
		op101.SurgicalPlan.BloodFluidGlucoseGradientMgDl != 76.0 ||
		op101.SurgicalPlan.FluidBloodLactateGradientMmolL != 4.4 ||
		op101.SurgicalPlan.AmpicillinSulbactam30MgKgMg != 930.0 ||
		op101.SurgicalPlan.Enrofloxacin10MgKgDoseMg != 310.0 ||
		op101.SurgicalPlan.WarmSterileSalineLavageLiters != 9.3 ||
		op101.SurgicalPlan.JacksonPrattClosedSuctionDrains != 2 ||
		!op101.SurgicalPlan.ZeroAntisepticLavageAdditionGuard {
		t.Errorf("unexpected SP-OP-101 surgical plan: %+v", op101.SurgicalPlan)
	}
	lowerAlert := strings.ToLower(op101.SurgicalPlan.PeritonitisPitfallAlert)
	if !strings.Contains(lowerAlert, "1 hour") || !strings.Contains(lowerAlert, "chlorhexidine") || !strings.Contains(lowerAlert, "povidone-iodine") || !strings.Contains(lowerAlert, "aspirate") {
		t.Errorf("expected pitfall alert to warn against >1h antibiotic delay, chlorhexidine/povidone-iodine chemical peritonitis, and aspirating lavage fluid, got %q", op101.SurgicalPlan.PeritonitisPitfallAlert)
	}
	if len(op101.DrainChecks) != 1 {
		t.Errorf("expected 1 initial drain check on SP-OP-101, got %d", len(op101.DrainChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected SP-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("SP-OP-102")
	if err != nil {
		t.Fatalf("expected SP-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Nala" || op102.IncidentID != "INC-SP-9502" || op102.Status != domain.PeritonitisStatusAntibioticLavageJPStaged {
		t.Errorf("unexpected SP-OP-102 metadata: %+v", op102)
	}
	if op102.SurgicalPlan == nil || op102.SurgicalPlan.Enrofloxacin10MgKgDoseMg != 22.5 || op102.SurgicalPlan.JacksonPrattClosedSuctionDrains != 1 {
		t.Errorf("expected feline Enrofloxacin 22.5 mg (5 mg/kg) and 1 JP drain on SP-OP-102, got %+v", op102.SurgicalPlan)
	}

	op103, err := svc.GetOperation("SP-OP-103")
	if err != nil {
		t.Fatalf("expected SP-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Gumbo" || op103.IncidentID != "INC-SP-9503" || op103.Status != domain.PeritonitisStatusNonSepticJPDrainRemoved {
		t.Errorf("unexpected SP-OP-103 metadata: %+v", op103)
	}
	if op103.SepticGradientAntibioticDelayAlert || op103.DrainClearance == nil {
		t.Errorf("expected SP-OP-103 to have DrainClearance and false septic alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.PeritonitisStatusSepticEffusionCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "SP-OP-101" {
		t.Errorf("expected 1 critical op SP-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.SepticEffusionCriticalCount != 1 ||
		metrics.AntibioticLavageJPStagedCount != 1 ||
		metrics.NonSepticClearedCount != 1 ||
		metrics.SepticGradientAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestSepticPeritonitisCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := septicperitonitis.NewPeritonitisService()

	profile := domain.PeritonitisPatientProfile{
		Species:                      domain.PeritonitisSpeciesCanineProstaticAbscess,
		Breed:                        "German Shepherd",
		WeightKg:                     34.0,
		BloodGlucoseMgDl:             126.0,
		EffusionGlucoseMgDl:          48.0,
		BloodLactateMmolL:            2.2,
		EffusionLactateMmolL:         6.4,
		DegenerateNeutrophilsCountUl: 41000,
		IntracellularBacteriaSeen:    true,
	}

	created, err := svc.CreateOperation("INC-SP-9504", "Roscoe", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "SP-OP-104" || created.Status != domain.PeritonitisStatusSepticEffusionCritical || !created.SepticGradientAntibioticDelayAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if created.SurgicalPlan == nil ||
		created.SurgicalPlan.BloodFluidGlucoseGradientMgDl != 78.0 ||
		created.SurgicalPlan.FluidBloodLactateGradientMmolL != 4.2 ||
		created.SurgicalPlan.AmpicillinSulbactam30MgKgMg != 1020.0 ||
		created.SurgicalPlan.Enrofloxacin10MgKgDoseMg != 340.0 ||
		created.SurgicalPlan.WarmSterileSalineLavageLiters != 10.2 {
		t.Errorf("unexpected surgical plan on SP-OP-104: %+v", created.SurgicalPlan)
	}

	// Stage surgical plan on SP-OP-104 preserving breed when empty
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.WeightKg = 34.0
	stagedProfile.BloodGlucoseMgDl = 120.0
	stagedProfile.EffusionGlucoseMgDl = 50.0
	stagedProfile.BloodLactateMmolL = 2.0
	stagedProfile.EffusionLactateMmolL = 5.5

	staged, err := svc.StageSurgicalPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageSurgicalPlan error: %v", err)
	}
	if staged.Status != domain.PeritonitisStatusAntibioticLavageJPStaged {
		t.Errorf("expected status %s, got %s", domain.PeritonitisStatusAntibioticLavageJPStaged, staged.Status)
	}
	if staged.PatientProfile.Breed != "German Shepherd" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}
	if staged.SurgicalPlan.BloodFluidGlucoseGradientMgDl != 70.0 || staged.SurgicalPlan.FluidBloodLactateGradientMmolL != 3.5 {
		t.Errorf("unexpected staged gradients: %+v", staged.SurgicalPlan)
	}

	// Record drain check (non-septic improving)
	withCheck, err := svc.RecordDrainCheck(created.ID, domain.JacksonPrattDrainCheck{
		EffusionProductionMlKgDay:     1.8,
		CytologyIntracellularBacteria: false,
		DegenerateNeutrophilsPresent:  false,
		SerumAlbuminGdl:               2.5,
		MeanArterialPressureMmHg:      84,
		ClinicianCallsign:             "DR-SURG-44",
		Notes:                         "JP drain production down to 1.8 mL/kg/day; cytology clear of intracellular bacteria.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordDrainCheck error: %v", err)
	}
	if len(withCheck.DrainChecks) != 1 || withCheck.SepticGradientAntibioticDelayAlert {
		t.Errorf("expected 1 drain check and false septic alert at 1.8 mL/kg/day non-septic, got %+v", withCheck)
	}

	// Issue drain removal clearance (with empty summary to test auto-summary)
	cleared, err := svc.IssueDrainClearance(created.ID, domain.PeritonitisDrainRemovalClearance{
		FinalDrainOutputMlKgDay:       1.5,
		FinalCytologyBacteriaNegative: true,
		FinalSerumAlbuminGdl:          2.6,
		EnteralNutritionTolerated:     true,
		JPDrainAsepticallyRemoved:     true,
		VetLicense:                    "VET-SURG-88412",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDrainClearance error: %v", err)
	}
	if cleared.Status != domain.PeritonitisStatusNonSepticJPDrainRemoved || cleared.SepticGradientAntibioticDelayAlert || cleared.DrainClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DrainClearance.Summary == "" {
		t.Errorf("expected auto-populated drain clearance summary when empty")
	}

	// Relapse drain check with intracellular bacteria transitions back to SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL
	relapsed, err := svc.RecordDrainCheck(created.ID, domain.JacksonPrattDrainCheck{
		EffusionProductionMlKgDay:     7.5,
		CytologyIntracellularBacteria: true,
		DegenerateNeutrophilsPresent:  true,
		SerumAlbuminGdl:               1.7,
		MeanArterialPressureMmHg:      60,
		ClinicianCallsign:             "DR-SURG-44",
		Notes:                         "Recurrent intracellular bacteria on JP bulb cytology.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordDrainCheck error: %v", err)
	}
	if relapsed.Status != domain.PeritonitisStatusSepticEffusionCritical || !relapsed.SepticGradientAntibioticDelayAlert || relapsed.DrainClearance != nil {
		t.Errorf("expected relapse to revert to SEPTIC_EFFUSION_INTRACELLULAR_BACTERIA_CRITICAL, got %+v", relapsed)
	}
}

func TestSepticPeritonitisValidationErrors(t *testing.T) {
	svc := septicperitonitis.NewPeritonitisService()

	validProfile := domain.PeritonitisPatientProfile{
		Species:                      domain.PeritonitisSpeciesCanineGIDehiscence,
		WeightKg:                     25.0,
		BloodGlucoseMgDl:             115.0,
		EffusionGlucoseMgDl:          45.0,
		BloodLactateMmolL:            2.1,
		EffusionLactateMmolL:         5.8,
		DegenerateNeutrophilsCountUl: 32000,
		IntracellularBacteriaSeen:    true,
	}

	if _, err := svc.CreateOperation("", "Baxter", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.PeritonitisPatientProfile)
	}{
		{"invalid species", func(p *domain.PeritonitisPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.PeritonitisPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.PeritonitisPatientProfile) { p.WeightKg = 150 }},
		{"blood glucose too low", func(p *domain.PeritonitisPatientProfile) { p.BloodGlucoseMgDl = 5.0 }},
		{"blood glucose too high", func(p *domain.PeritonitisPatientProfile) { p.BloodGlucoseMgDl = 700.0 }},
		{"effusion glucose negative", func(p *domain.PeritonitisPatientProfile) { p.EffusionGlucoseMgDl = -1.0 }},
		{"blood lactate too low", func(p *domain.PeritonitisPatientProfile) { p.BloodLactateMmolL = 0.0 }},
		{"effusion lactate too low", func(p *domain.PeritonitisPatientProfile) { p.EffusionLactateMmolL = 0.0 }},
		{"degenerate neutrophils negative", func(p *domain.PeritonitisPatientProfile) { p.DegenerateNeutrophilsCountUl = -5 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Baxter", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageSurgicalPlan("SP-OP-101", p); err == nil {
				t.Errorf("expected StageSurgicalPlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageSurgicalPlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging surgical plan on unknown op")
	}

	validCheck := domain.JacksonPrattDrainCheck{
		EffusionProductionMlKgDay:     2.2,
		CytologyIntracellularBacteria: false,
		DegenerateNeutrophilsPresent:  false,
		SerumAlbuminGdl:               2.4,
		MeanArterialPressureMmHg:      80,
		ClinicianCallsign:             "DR-1",
	}
	if _, err := svc.RecordDrainCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording drain check on unknown op")
	}

	badChecks := []func(c *domain.JacksonPrattDrainCheck){
		func(c *domain.JacksonPrattDrainCheck) { c.EffusionProductionMlKgDay = -1.0 },
		func(c *domain.JacksonPrattDrainCheck) { c.EffusionProductionMlKgDay = 120.0 },
		func(c *domain.JacksonPrattDrainCheck) { c.SerumAlbuminGdl = 0.2 },
		func(c *domain.JacksonPrattDrainCheck) { c.MeanArterialPressureMmHg = 20 },
		func(c *domain.JacksonPrattDrainCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordDrainCheck("SP-OP-101", c); err == nil {
			t.Errorf("expected RecordDrainCheck validation error for %+v", c)
		}
	}

	validClearance := domain.PeritonitisDrainRemovalClearance{
		FinalDrainOutputMlKgDay:       1.6,
		FinalCytologyBacteriaNegative: true,
		FinalSerumAlbuminGdl:          2.5,
		EnteralNutritionTolerated:     true,
		JPDrainAsepticallyRemoved:     true,
		VetLicense:                    "VET-123",
	}
	if _, err := svc.IssueDrainClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.PeritonitisDrainRemovalClearance){
		func(d *domain.PeritonitisDrainRemovalClearance) { d.FinalDrainOutputMlKgDay = 2.5 },
		func(d *domain.PeritonitisDrainRemovalClearance) { d.FinalDrainOutputMlKgDay = -0.5 },
		func(d *domain.PeritonitisDrainRemovalClearance) { d.FinalCytologyBacteriaNegative = false },
		func(d *domain.PeritonitisDrainRemovalClearance) { d.FinalSerumAlbuminGdl = 1.6 },
		func(d *domain.PeritonitisDrainRemovalClearance) { d.EnteralNutritionTolerated = false },
		func(d *domain.PeritonitisDrainRemovalClearance) { d.JPDrainAsepticallyRemoved = false },
		func(d *domain.PeritonitisDrainRemovalClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDrainClearance("SP-OP-101", d); err == nil {
			t.Errorf("expected IssueDrainClearance validation error for %+v", d)
		}
	}
}
