package dkainsulin_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/dkainsulin"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestDKAInsulinSeededOperationsAndMetrics(t *testing.T) {
	svc := dkainsulin.NewDKAService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("DKA-OP-101")
	if err != nil {
		t.Fatalf("expected DKA-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Winston" || op101.IncidentID != "INC-DKA-9301" || op101.Status != domain.DKAStatusAcidosisDehydratedCritical {
		t.Errorf("unexpected DKA-OP-101 metadata: %+v", op101)
	}
	if !op101.HypokalemiaCerebralEdemaAlert || op101.PatientProfile.WeightKg != 5.4 || op101.PatientProfile.SerumPotassiumMeqL != 2.8 {
		t.Errorf("unexpected DKA-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.InsulinRegimen == nil || !op101.InsulinRegimen.HoldInsulinIfPotassiumBelow33Guard || op101.InsulinRegimen.MaxGlucoseDropRateMgDlHr != 50.0 {
		t.Errorf("unexpected DKA-OP-101 regimen: %+v", op101.InsulinRegimen)
	}
	if op101.InsulinRegimen.PotassiumChlorideMeqPerLiter != 40 || op101.InsulinRegimen.PotassiumPhosphateMmolKgHr != 0.06 || op101.InsulinRegimen.RegularInsulinCRIUkgDay != 1.1 {
		t.Errorf("unexpected DKA-OP-101 KCl/KPhos/Insulin regimen: %+v", op101.InsulinRegimen)
	}
	lowerAlert := strings.ToLower(op101.InsulinRegimen.DKAPitfallAlert)
	if !strings.Contains(lowerAlert, "3.3") || !strings.Contains(lowerAlert, "cerebral edema") || !strings.Contains(lowerAlert, "heinz body") {
		t.Errorf("expected pitfall alert to warn against K+ < 3.3, cerebral edema, and Heinz body anemia, got %q", op101.InsulinRegimen.DKAPitfallAlert)
	}
	if len(op101.SerialChecks) != 1 {
		t.Errorf("expected 1 initial serial check on DKA-OP-101, got %d", len(op101.SerialChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected DKA-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("DKA-OP-102")
	if err != nil {
		t.Fatalf("expected DKA-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Daisy" || op102.IncidentID != "INC-DKA-9302" || op102.Status != domain.DKAStatusKClPhosInsulinCRIStaged {
		t.Errorf("unexpected DKA-OP-102 metadata: %+v", op102)
	}
	if op102.InsulinRegimen == nil || op102.InsulinRegimen.RegularInsulinCRIUkgDay != 2.2 || op102.InsulinRegimen.DextroseSupplementationPercent != 2.5 {
		t.Errorf("expected canine 2.2 U/kg/day and 2.5%% dextrose for BG 220 mg/dL on DKA-OP-102, got %+v", op102.InsulinRegimen)
	}

	op103, err := svc.GetOperation("DKA-OP-103")
	if err != nil {
		t.Fatalf("expected DKA-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Chester" || op103.IncidentID != "INC-DKA-9303" || op103.Status != domain.DKAStatusKetoneResolvedGlargineCleared {
		t.Errorf("unexpected DKA-OP-103 metadata: %+v", op103)
	}
	if op103.HypokalemiaCerebralEdemaAlert || op103.DischargeClearance == nil {
		t.Errorf("expected DKA-OP-103 to have DischargeClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.DKAStatusAcidosisDehydratedCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "DKA-OP-101" {
		t.Errorf("expected 1 critical op DKA-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.CriticalAcidosisCount != 1 || metrics.InsulinCRIStagedCount != 1 || metrics.KetoneClearedCount != 1 || metrics.HypokalemiaEdemaAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestDKAInsulinCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := dkainsulin.NewDKAService()

	profile := domain.DKAPatientProfile{
		Species:                   domain.DKASpeciesCaninePancreatitis,
		Breed:                     "Golden Retriever",
		WeightKg:                  28.0,
		BloodGlucoseMgDl:          510.0,
		BetaHydroxybutyrateMmolL:  6.4,
		VenousPH:                  7.09,
		SerumPotassiumMeqL:        1.9,
		SerumPhosphorusMgDl:       1.2,
		EffectiveOsmolalityMosmKg: 334.0,
		DehydrationPercent:        9.0,
	}

	created, err := svc.CreateOperation("INC-DKA-9304", "Biscuit", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "DKA-OP-104" || created.Status != domain.DKAStatusAcidosisDehydratedCritical || !created.HypokalemiaCerebralEdemaAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if created.InsulinRegimen.PotassiumChlorideMeqPerLiter != 80 || created.InsulinRegimen.PotassiumPhosphateMmolKgHr != 0.12 {
		t.Errorf("expected 80 mEq/L KCl and 0.12 mmol/kg/hr KPhos for severe hypokalemia/hypophosphatemia, got %+v", created.InsulinRegimen)
	}

	// Test sliding-scale tiers in BuildInsulinRegimen
	tier2 := dkainsulin.BuildInsulinRegimen(domain.DKAPatientProfile{
		Species:                   domain.DKASpeciesFelineHepaticLipidosis,
		WeightKg:                  5.0,
		BloodGlucoseMgDl:          135.0,
		BetaHydroxybutyrateMmolL:  2.2,
		VenousPH:                  7.28,
		SerumPotassiumMeqL:        2.3,
		SerumPhosphorusMgDl:       2.2,
		EffectiveOsmolalityMosmKg: 298.0,
		DehydrationPercent:        5.0,
	})
	if tier2.PotassiumChlorideMeqPerLiter != 60 || tier2.PotassiumPhosphateMmolKgHr != 0.03 || tier2.DextroseSupplementationPercent != 5.0 {
		t.Errorf("expected 60 mEq/L KCl, 0.03 KPhos, and 5.0%% dextrose, got %+v", tier2)
	}

	tier3 := dkainsulin.BuildInsulinRegimen(domain.DKAPatientProfile{
		Species:                   domain.DKASpeciesCaninePancreatitis,
		WeightKg:                  12.0,
		BloodGlucoseMgDl:          210.0,
		BetaHydroxybutyrateMmolL:  2.0,
		VenousPH:                  7.30,
		SerumPotassiumMeqL:        3.2,
		SerumPhosphorusMgDl:       3.1,
		EffectiveOsmolalityMosmKg: 296.0,
		DehydrationPercent:        4.0,
	})
	if tier3.PotassiumChlorideMeqPerLiter != 30 || tier3.PotassiumPhosphateMmolKgHr != 0.0 || tier3.DextroseSupplementationPercent != 2.5 {
		t.Errorf("expected 30 mEq/L KCl, 0.0 KPhos, and 2.5%% dextrose, got %+v", tier3)
	}

	// Stage regimen on DKA-OP-104 after initial K+ correction
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.BloodGlucoseMgDl = 230.0
	stagedProfile.SerumPotassiumMeqL = 3.6
	stagedProfile.SerumPhosphorusMgDl = 2.8
	stagedProfile.DehydrationPercent = 4.0

	staged, err := svc.StageInsulinRegimen(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageInsulinRegimen error: %v", err)
	}
	if staged.Status != domain.DKAStatusKClPhosInsulinCRIStaged {
		t.Errorf("expected status %s, got %s", domain.DKAStatusKClPhosInsulinCRIStaged, staged.Status)
	}
	if staged.InsulinRegimen.PotassiumChlorideMeqPerLiter != 20 || staged.InsulinRegimen.DextroseSupplementationPercent != 2.5 || staged.InsulinRegimen.RegularInsulinCRIUkgDay != 2.2 {
		t.Errorf("unexpected staged regimen: %+v", staged.InsulinRegimen)
	}
	if staged.PatientProfile.Breed != "Golden Retriever" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record serial check with resolving ketones & normal electrolytes
	checked, err := svc.RecordSerialCheck(created.ID, domain.DKAGlucoseElectrolyteCheck{
		BloodGlucoseMgDl:               175.0,
		BetaHydroxybutyrateMmolL:       1.1,
		SerumPotassiumMeqL:             4.1,
		SerumPhosphorusMgDl:            3.6,
		DextrosePercentActive:          2.5,
		MentationNormalNoCerebralEdema: true,
		ClinicianCallsign:              "DR-ENDO-55",
		Notes:                          "BG falling 35 mg/dL/hr (< 50 mg/dL/hr limit); mentation bright; BHB down to 1.1 mmol/L.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordSerialCheck error: %v", err)
	}
	if len(checked.SerialChecks) != 1 || checked.HypokalemiaCerebralEdemaAlert {
		t.Errorf("expected 1 serial check and false alert when K+ 4.1, Phos 3.6, BHB 1.1, got %+v", checked)
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.DKADischargeClearance{
		FinalBetaHydroxybutyrateMmolL:  0.3,
		FinalVenousPH:                  7.40,
		FinalSerumPotassiumMeqL:        4.3,
		FinalSerumPhosphorusMgDl:       3.9,
		VoluntaryEatingNoVomiting:      true,
		SubQInsulinTransitionCompleted: true,
		VetLicense:                     "VET-WA-77410",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.DKAStatusKetoneResolvedGlargineCleared || cleared.HypokalemiaCerebralEdemaAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse check with cerebral edema / severe hypokalemia reverts to critical
	relapsed, err := svc.RecordSerialCheck(created.ID, domain.DKAGlucoseElectrolyteCheck{
		BloodGlucoseMgDl:               390.0,
		BetaHydroxybutyrateMmolL:       4.2,
		SerumPotassiumMeqL:             2.7,
		SerumPhosphorusMgDl:            1.4,
		DextrosePercentActive:          0.0,
		MentationNormalNoCerebralEdema: false,
		ClinicianCallsign:              "DR-ENDO-55",
		Notes:                          "Severe relapse with hypokalemia 2.7 mEq/L and obtunded mentation.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordSerialCheck error: %v", err)
	}
	if relapsed.Status != domain.DKAStatusAcidosisDehydratedCritical || !relapsed.HypokalemiaCerebralEdemaAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to KETOACIDOSIS_DEHYDRATED_CRITICAL, got %+v", relapsed)
	}
}

func TestDKAInsulinValidationErrors(t *testing.T) {
	svc := dkainsulin.NewDKAService()

	validProfile := domain.DKAPatientProfile{
		Species:                   domain.DKASpeciesFelineHepaticLipidosis,
		WeightKg:                  5.2,
		BloodGlucoseMgDl:          420.0,
		BetaHydroxybutyrateMmolL:  5.1,
		VenousPH:                  7.15,
		SerumPotassiumMeqL:        3.1,
		SerumPhosphorusMgDl:       1.8,
		EffectiveOsmolalityMosmKg: 320.0,
		DehydrationPercent:        7.0,
	}

	if _, err := svc.CreateOperation("", "Winston", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.DKAPatientProfile)
	}{
		{"invalid species", func(p *domain.DKAPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.DKAPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.DKAPatientProfile) { p.WeightKg = 140 }},
		{"bg too low", func(p *domain.DKAPatientProfile) { p.BloodGlucoseMgDl = 10.0 }},
		{"bg too high", func(p *domain.DKAPatientProfile) { p.BloodGlucoseMgDl = 1800.0 }},
		{"bhb negative", func(p *domain.DKAPatientProfile) { p.BetaHydroxybutyrateMmolL = -1.0 }},
		{"bhb too high", func(p *domain.DKAPatientProfile) { p.BetaHydroxybutyrateMmolL = 30.0 }},
		{"ph too low", func(p *domain.DKAPatientProfile) { p.VenousPH = 6.50 }},
		{"ph too high", func(p *domain.DKAPatientProfile) { p.VenousPH = 7.80 }},
		{"k+ too low", func(p *domain.DKAPatientProfile) { p.SerumPotassiumMeqL = 0.5 }},
		{"k+ too high", func(p *domain.DKAPatientProfile) { p.SerumPotassiumMeqL = 12.0 }},
		{"phos too low", func(p *domain.DKAPatientProfile) { p.SerumPhosphorusMgDl = 0.1 }},
		{"phos too high", func(p *domain.DKAPatientProfile) { p.SerumPhosphorusMgDl = 20.0 }},
		{"osmolality too low", func(p *domain.DKAPatientProfile) { p.EffectiveOsmolalityMosmKg = 200.0 }},
		{"osmolality too high", func(p *domain.DKAPatientProfile) { p.EffectiveOsmolalityMosmKg = 450.0 }},
		{"dehydration negative", func(p *domain.DKAPatientProfile) { p.DehydrationPercent = -1.0 }},
		{"dehydration > 15", func(p *domain.DKAPatientProfile) { p.DehydrationPercent = 18.0 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Winston", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageInsulinRegimen("DKA-OP-101", p); err == nil {
				t.Errorf("expected StageInsulinRegimen error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageInsulinRegimen("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging regimen on unknown op")
	}

	validCheck := domain.DKAGlucoseElectrolyteCheck{
		BloodGlucoseMgDl:               210.0,
		BetaHydroxybutyrateMmolL:       2.4,
		SerumPotassiumMeqL:             3.8,
		SerumPhosphorusMgDl:            2.9,
		DextrosePercentActive:          2.5,
		MentationNormalNoCerebralEdema: true,
		ClinicianCallsign:              "DR-1",
	}
	if _, err := svc.RecordSerialCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording serial check on unknown op")
	}

	badChecks := []func(c *domain.DKAGlucoseElectrolyteCheck){
		func(c *domain.DKAGlucoseElectrolyteCheck) { c.BloodGlucoseMgDl = 10.0 },
		func(c *domain.DKAGlucoseElectrolyteCheck) { c.BetaHydroxybutyrateMmolL = -0.5 },
		func(c *domain.DKAGlucoseElectrolyteCheck) { c.SerumPotassiumMeqL = 0.8 },
		func(c *domain.DKAGlucoseElectrolyteCheck) { c.SerumPhosphorusMgDl = 0.1 },
		func(c *domain.DKAGlucoseElectrolyteCheck) { c.DextrosePercentActive = 15.0 },
		func(c *domain.DKAGlucoseElectrolyteCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordSerialCheck("DKA-OP-101", c); err == nil {
			t.Errorf("expected RecordSerialCheck validation error for %+v", c)
		}
	}

	validClearance := domain.DKADischargeClearance{
		FinalBetaHydroxybutyrateMmolL:  0.4,
		FinalVenousPH:                  7.39,
		FinalSerumPotassiumMeqL:        4.2,
		FinalSerumPhosphorusMgDl:       3.8,
		VoluntaryEatingNoVomiting:      true,
		SubQInsulinTransitionCompleted: true,
		VetLicense:                     "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.DKADischargeClearance){
		func(d *domain.DKADischargeClearance) { d.FinalBetaHydroxybutyrateMmolL = 0.8 },
		func(d *domain.DKADischargeClearance) { d.FinalVenousPH = 7.20 },
		func(d *domain.DKADischargeClearance) { d.FinalVenousPH = 7.55 },
		func(d *domain.DKADischargeClearance) { d.FinalSerumPotassiumMeqL = 3.1 },
		func(d *domain.DKADischargeClearance) { d.FinalSerumPotassiumMeqL = 6.0 },
		func(d *domain.DKADischargeClearance) { d.FinalSerumPhosphorusMgDl = 1.9 },
		func(d *domain.DKADischargeClearance) { d.FinalSerumPhosphorusMgDl = 7.0 },
		func(d *domain.DKADischargeClearance) { d.VoluntaryEatingNoVomiting = false },
		func(d *domain.DKADischargeClearance) { d.SubQInsulinTransitionCompleted = false },
		func(d *domain.DKADischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("DKA-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}

	// Validate DKAInsulinOperation error paths
	op := domain.DKAInsulinOperation{
		ID:             "DKA-OP-101",
		IncidentID:     "INC-DKA-9301",
		PetName:        "Winston",
		Status:         domain.DKAStatusAcidosisDehydratedCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.DKAInsulinOperation){
		func(o *domain.DKAInsulinOperation) { o.ID = "" },
		func(o *domain.DKAInsulinOperation) { o.IncidentID = "" },
		func(o *domain.DKAInsulinOperation) { o.PetName = "" },
		func(o *domain.DKAInsulinOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.DKAInsulinOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected DKAInsulinOperation.Validate error for %+v", cp)
		}
	}
}
