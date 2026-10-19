package addisoniancrisis_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/addisoniancrisis"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestAddisonianCrisisSeededOperationsAndMetrics(t *testing.T) {
	svc := addisoniancrisis.NewAddisonianService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("AC-OP-101")
	if err != nil {
		t.Fatalf("expected AC-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Curly" || op101.IncidentID != "INC-AC-9601" || op101.Status != domain.AddisonianStatusShockHyperkalemicCritical {
		t.Errorf("unexpected AC-OP-101 metadata: %+v", op101)
	}
	if !op101.OsmoticMyelinolysisAssayAlert || op101.PatientProfile.WeightKg != 23.5 || op101.PatientProfile.SerumSodiumMeqL != 124.0 || op101.PatientProfile.SerumPotassiumMeqL != 7.8 {
		t.Errorf("unexpected AC-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.ResuscitationRegimen == nil || !op101.ResuscitationRegimen.ZeroPrednisolonePreACTHGuard || op101.ResuscitationRegimen.MaxSodiumRiseRateMeqLDay != 10.0 {
		t.Errorf("unexpected AC-OP-101 regimen: %+v", op101.ResuscitationRegimen)
	}
	if op101.ResuscitationRegimen.SodiumPotassiumRatio != 15.9 || op101.ResuscitationRegimen.IsotonicNaCl09BolusMl != 470.0 || op101.ResuscitationRegimen.DexamethasoneSP025MgKgMg != 5.88 || op101.ResuscitationRegimen.CosyntropinACTHStimDoseMcg != 117.5 || op101.ResuscitationRegimen.CalciumGluconate10PctMl != 11.75 {
		t.Errorf("unexpected AC-OP-101 regimen values: %+v", op101.ResuscitationRegimen)
	}
	lowerAlert := strings.ToLower(op101.ResuscitationRegimen.AddisonianPitfallAlert)
	if !strings.Contains(lowerAlert, "prednisone") || !strings.Contains(lowerAlert, "dexamethasone-sp") || !strings.Contains(lowerAlert, "myelinolysis") {
		t.Errorf("expected pitfall alert to warn against prednisone cross-reactivity, dexamethasone-sp, and myelinolysis, got %q", op101.ResuscitationRegimen.AddisonianPitfallAlert)
	}
	if len(op101.ElectrolyteChecks) != 1 {
		t.Errorf("expected 1 initial electrolyte check on AC-OP-101, got %d", len(op101.ElectrolyteChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected AC-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("AC-OP-102")
	if err != nil {
		t.Fatalf("expected AC-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Pippa" || op102.IncidentID != "INC-AC-9602" || op102.Status != domain.AddisonianStatusDexSPNaClACTHStaged {
		t.Errorf("unexpected AC-OP-102 metadata: %+v", op102)
	}
	if op102.ResuscitationRegimen == nil || op102.ResuscitationRegimen.SodiumPotassiumRatio != 21.5 || op102.ResuscitationRegimen.CalciumGluconate10PctMl != 0.0 {
		t.Errorf("expected Na:K ratio 21.5 and 0.0 mL CaGluconate on AC-OP-102, got %+v", op102.ResuscitationRegimen)
	}

	op103, err := svc.GetOperation("AC-OP-103")
	if err != nil {
		t.Fatalf("expected AC-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Archie" || op103.IncidentID != "INC-AC-9603" || op103.Status != domain.AddisonianStatusDOCPEukalemicCleared {
		t.Errorf("unexpected AC-OP-103 metadata: %+v", op103)
	}
	if op103.OsmoticMyelinolysisAssayAlert || op103.DischargeClearance == nil {
		t.Errorf("expected AC-OP-103 to have DischargeClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.AddisonianStatusShockHyperkalemicCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "AC-OP-101" {
		t.Errorf("expected 1 critical op AC-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.CriticalShockCount != 1 || metrics.DexSPACTHStagedCount != 1 || metrics.DOCPEukalemicClearedCount != 1 || metrics.MyelinolysisAssayAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestAddisonianCrisisCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := addisoniancrisis.NewAddisonianService()

	profile := domain.AddisonianPatientProfile{
		Species:                   domain.AddisonianSpeciesCanineTypical,
		Breed:                     "Great Pyrenees",
		WeightKg:                  55.0,
		SerumSodiumMeqL:           122.0,
		SerumPotassiumMeqL:        8.1,
		BloodGlucoseMgDl:          48.0,
		HeartRateBpm:              48,
		SystolicBloodPressureMmHg: 64,
		RestingCortisolMcgDl:      0.1,
	}

	created, err := svc.CreateOperation("INC-AC-9604", "Louie", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "AC-OP-104" || created.Status != domain.AddisonianStatusShockHyperkalemicCritical || !created.OsmoticMyelinolysisAssayAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	// 55 kg * 5 mcg/kg = 275 mcg, capped at 250 mcg
	if created.ResuscitationRegimen.CosyntropinACTHStimDoseMcg != 250.0 || created.ResuscitationRegimen.CalciumGluconate10PctMl != 27.5 {
		t.Errorf("expected Cosyntropin capped at 250 mcg and CaGluconate 27.5 mL, got %+v", created.ResuscitationRegimen)
	}

	// Stage regimen on AC-OP-104 after initial calcium gluconate & fluid aliquot
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.SerumSodiumMeqL = 128.0
	stagedProfile.SerumPotassiumMeqL = 6.2
	stagedProfile.HeartRateBpm = 84
	stagedProfile.SystolicBloodPressureMmHg = 92

	staged, err := svc.StageResuscitationRegimen(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageResuscitationRegimen error: %v", err)
	}
	if staged.Status != domain.AddisonianStatusDexSPNaClACTHStaged {
		t.Errorf("expected status %s, got %s", domain.AddisonianStatusDexSPNaClACTHStaged, staged.Status)
	}
	if staged.ResuscitationRegimen.CalciumGluconate10PctMl != 0.0 || staged.ResuscitationRegimen.SodiumPotassiumRatio != 20.6 {
		t.Errorf("unexpected staged regimen: %+v", staged.ResuscitationRegimen)
	}
	if staged.PatientProfile.Breed != "Great Pyrenees" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record electrolyte check with eukalemia & safe Na+ rise
	checked, err := svc.RecordElectrolyteCheck(created.ID, domain.AddisonianElectrolyteCheck{
		SerumSodiumMeqL:       138.0,
		SerumPotassiumMeqL:    4.8,
		SodiumRise24HrMeqL:    8.0,
		PostACTHCortisolMcgDl: 0.3,
		HeartRateBpm:          90,
		ClinicianCallsign:     "DR-ENDO-55",
		Notes:                 "Na+ rose 8.0 mEq/L/24h (<= 10.0 mEq/L/day target); post-ACTH cortisol 0.3 mcg/dL; K+ normalized to 4.8 mEq/L.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordElectrolyteCheck error: %v", err)
	}
	if len(checked.ElectrolyteChecks) != 1 || checked.OsmoticMyelinolysisAssayAlert {
		t.Errorf("expected 1 electrolyte check and false alert when Na:K >= 27 and K+ 4.8, got %+v", checked)
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.AddisonianDischargeClearance{
		FinalSodiumPotassiumRatio:        31.2,
		FinalSerumPotassiumMeqL:          4.5,
		PostACTHCortisolConfirmedLow:     true,
		DOCP22MgKgSubQAdministered:       true,
		OralPrednisonePhysiologicStarted: true,
		VetLicense:                       "VET-WA-88120",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.AddisonianStatusDOCPEukalemicCleared || cleared.OsmoticMyelinolysisAssayAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse check with severe hyperkalemia & bradycardia reverts to critical
	relapsed, err := svc.RecordElectrolyteCheck(created.ID, domain.AddisonianElectrolyteCheck{
		SerumSodiumMeqL:       125.0,
		SerumPotassiumMeqL:    7.6,
		SodiumRise24HrMeqL:    10.5,
		PostACTHCortisolMcgDl: 0.2,
		HeartRateBpm:          50,
		ClinicianCallsign:     "DR-ENDO-55",
		Notes:                 "Severe hyperkalemic relapse with bradycardia HR 50 bpm.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordElectrolyteCheck error: %v", err)
	}
	if relapsed.Status != domain.AddisonianStatusShockHyperkalemicCritical || !relapsed.OsmoticMyelinolysisAssayAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to ADDISONIAN_SHOCK_HYPERKALEMIA_CRITICAL, got %+v", relapsed)
	}
}

func TestAddisonianCrisisValidationErrors(t *testing.T) {
	svc := addisoniancrisis.NewAddisonianService()

	validProfile := domain.AddisonianPatientProfile{
		Species:                   domain.AddisonianSpeciesCanineAtypical,
		WeightKg:                  18.0,
		SerumSodiumMeqL:           136.0,
		SerumPotassiumMeqL:        4.9,
		BloodGlucoseMgDl:          58.0,
		HeartRateBpm:              78,
		SystolicBloodPressureMmHg: 88,
		RestingCortisolMcgDl:      0.4,
	}

	if _, err := svc.CreateOperation("", "Louie", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.AddisonianPatientProfile)
	}{
		{"invalid species", func(p *domain.AddisonianPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.AddisonianPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.AddisonianPatientProfile) { p.WeightKg = 140 }},
		{"na+ too low", func(p *domain.AddisonianPatientProfile) { p.SerumSodiumMeqL = 80.0 }},
		{"na+ too high", func(p *domain.AddisonianPatientProfile) { p.SerumSodiumMeqL = 195.0 }},
		{"k+ too low", func(p *domain.AddisonianPatientProfile) { p.SerumPotassiumMeqL = 1.5 }},
		{"k+ too high", func(p *domain.AddisonianPatientProfile) { p.SerumPotassiumMeqL = 14.0 }},
		{"bg too low", func(p *domain.AddisonianPatientProfile) { p.BloodGlucoseMgDl = 10.0 }},
		{"bg too high", func(p *domain.AddisonianPatientProfile) { p.BloodGlucoseMgDl = 700.0 }},
		{"hr too low", func(p *domain.AddisonianPatientProfile) { p.HeartRateBpm = 15 }},
		{"hr too high", func(p *domain.AddisonianPatientProfile) { p.HeartRateBpm = 350 }},
		{"sbp too low", func(p *domain.AddisonianPatientProfile) { p.SystolicBloodPressureMmHg = 20 }},
		{"sbp too high", func(p *domain.AddisonianPatientProfile) { p.SystolicBloodPressureMmHg = 300 }},
		{"cortisol negative", func(p *domain.AddisonianPatientProfile) { p.RestingCortisolMcgDl = -0.5 }},
		{"cortisol too high", func(p *domain.AddisonianPatientProfile) { p.RestingCortisolMcgDl = 30.0 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Louie", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageResuscitationRegimen("AC-OP-101", p); err == nil {
				t.Errorf("expected StageResuscitationRegimen error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageResuscitationRegimen("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging regimen on unknown op")
	}

	validCheck := domain.AddisonianElectrolyteCheck{
		SerumSodiumMeqL:       132.0,
		SerumPotassiumMeqL:    5.4,
		SodiumRise24HrMeqL:    6.5,
		PostACTHCortisolMcgDl: 0.4,
		HeartRateBpm:          84,
		ClinicianCallsign:     "DR-1",
	}
	if _, err := svc.RecordElectrolyteCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording electrolyte check on unknown op")
	}

	badChecks := []func(c *domain.AddisonianElectrolyteCheck){
		func(c *domain.AddisonianElectrolyteCheck) { c.SerumSodiumMeqL = 80.0 },
		func(c *domain.AddisonianElectrolyteCheck) { c.SerumPotassiumMeqL = 1.0 },
		func(c *domain.AddisonianElectrolyteCheck) { c.SodiumRise24HrMeqL = -1.0 },
		func(c *domain.AddisonianElectrolyteCheck) { c.SodiumRise24HrMeqL = 13.5 },
		func(c *domain.AddisonianElectrolyteCheck) { c.PostACTHCortisolMcgDl = -0.2 },
		func(c *domain.AddisonianElectrolyteCheck) { c.PostACTHCortisolMcgDl = 30.0 },
		func(c *domain.AddisonianElectrolyteCheck) { c.HeartRateBpm = 10 },
		func(c *domain.AddisonianElectrolyteCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordElectrolyteCheck("AC-OP-101", c); err == nil {
			t.Errorf("expected RecordElectrolyteCheck validation error for %+v", c)
		}
	}

	validClearance := domain.AddisonianDischargeClearance{
		FinalSodiumPotassiumRatio:        30.5,
		FinalSerumPotassiumMeqL:          4.5,
		PostACTHCortisolConfirmedLow:     true,
		DOCP22MgKgSubQAdministered:       true,
		OralPrednisonePhysiologicStarted: true,
		VetLicense:                       "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.AddisonianDischargeClearance){
		func(d *domain.AddisonianDischargeClearance) { d.FinalSodiumPotassiumRatio = 25.0 },
		func(d *domain.AddisonianDischargeClearance) { d.FinalSodiumPotassiumRatio = 55.0 },
		func(d *domain.AddisonianDischargeClearance) { d.FinalSerumPotassiumMeqL = 3.1 },
		func(d *domain.AddisonianDischargeClearance) { d.FinalSerumPotassiumMeqL = 6.0 },
		func(d *domain.AddisonianDischargeClearance) { d.PostACTHCortisolConfirmedLow = false },
		func(d *domain.AddisonianDischargeClearance) { d.DOCP22MgKgSubQAdministered = false },
		func(d *domain.AddisonianDischargeClearance) { d.OralPrednisonePhysiologicStarted = false },
		func(d *domain.AddisonianDischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("AC-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}

	// Validate AddisonianCrisisOperation error paths
	op := domain.AddisonianCrisisOperation{
		ID:             "AC-OP-101",
		IncidentID:     "INC-AC-9601",
		PetName:        "Curly",
		Status:         domain.AddisonianStatusShockHyperkalemicCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.AddisonianCrisisOperation){
		func(o *domain.AddisonianCrisisOperation) { o.ID = "" },
		func(o *domain.AddisonianCrisisOperation) { o.IncidentID = "" },
		func(o *domain.AddisonianCrisisOperation) { o.PetName = "" },
		func(o *domain.AddisonianCrisisOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.AddisonianCrisisOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected AddisonianCrisisOperation.Validate error for %+v", cp)
		}
	}
}
