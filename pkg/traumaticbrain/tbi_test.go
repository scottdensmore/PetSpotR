package traumaticbrain_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/traumaticbrain"
)

func TestTraumaticBrainSeededOperationsAndMetrics(t *testing.T) {
	svc := traumaticbrain.NewTBIService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("TBI-OP-101")
	if err != nil {
		t.Fatalf("expected TBI-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Atlas" || op101.IncidentID != "INC-TBI-9801" || op101.Status != domain.TBIStatusCushingHerniationCritical {
		t.Errorf("unexpected TBI-OP-101 metadata: %+v", op101)
	}
	if !op101.CushingHerniationSteroidAlert || op101.PatientProfile.WeightKg != 42.0 || op101.PatientProfile.ModifiedGlasgowComaScore != 8 || !op101.PatientProfile.CushingReflexPresent {
		t.Errorf("unexpected TBI-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.NeuroprotectivePlan == nil || !op101.NeuroprotectivePlan.ZeroJugularCompressionCorticosteroidGuard || op101.NeuroprotectivePlan.HeadElevationAngleDegrees != 30 || op101.NeuroprotectivePlan.TargetMAPMinimumMmHg != 80 {
		t.Errorf("unexpected TBI-OP-101 neuroprotective plan: %+v", op101.NeuroprotectivePlan)
	}
	if op101.NeuroprotectivePlan.HypertonicSaline75PctMlKg != 4.0 || op101.NeuroprotectivePlan.HypertonicSalineTotalDoseMl != 168.0 || op101.NeuroprotectivePlan.Mannitol05GKgSlow20MinGrams != 21.0 {
		t.Errorf("unexpected TBI-OP-101 HTS/Mannitol doses: %+v", op101.NeuroprotectivePlan)
	}
	lowerAlert := strings.ToLower(op101.NeuroprotectivePlan.TBIPitfallAlert)
	if !strings.Contains(lowerAlert, "corticosteroid") || !strings.Contains(lowerAlert, "jugular") || !strings.Contains(lowerAlert, "mannitol") {
		t.Errorf("expected pitfall alert to warn against corticosteroids, jugular compression, and hypotensive Mannitol, got %q", op101.NeuroprotectivePlan.TBIPitfallAlert)
	}
	if len(op101.NeuroChecks) != 1 {
		t.Errorf("expected 1 initial neuro check on TBI-OP-101, got %d", len(op101.NeuroChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected TBI-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("TBI-OP-102")
	if err != nil {
		t.Fatalf("expected TBI-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Milo" || op102.IncidentID != "INC-TBI-9802" || op102.Status != domain.TBIStatusHypertonicMannitolStaged {
		t.Errorf("unexpected TBI-OP-102 metadata: %+v", op102)
	}
	if op102.NeuroprotectivePlan == nil || op102.NeuroprotectivePlan.HypertonicSaline75PctMlKg != 2.0 || op102.NeuroprotectivePlan.HypertonicSalineTotalDoseMl != 9.2 || op102.NeuroprotectivePlan.Mannitol05GKgSlow20MinGrams != 2.3 {
		t.Errorf("expected feline 2.0 mL/kg HTS (9.2 mL) and 2.3 g Mannitol on TBI-OP-102, got %+v", op102.NeuroprotectivePlan)
	}

	op103, err := svc.GetOperation("TBI-OP-103")
	if err != nil {
		t.Fatalf("expected TBI-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Sierra" || op103.IncidentID != "INC-TBI-9803" || op103.Status != domain.TBIStatusMGCSRecoveredNormocapnicCleared {
		t.Errorf("unexpected TBI-OP-103 metadata: %+v", op103)
	}
	if op103.CushingHerniationSteroidAlert || op103.DischargeClearance == nil {
		t.Errorf("expected TBI-OP-103 to have DischargeClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.TBIStatusCushingHerniationCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "TBI-OP-101" {
		t.Errorf("expected 1 critical op TBI-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.CushingHerniationCount != 1 || metrics.HypertonicMannitolStaged != 1 || metrics.MGCSRecoveredClearedCount != 1 || metrics.CushingSteroidAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestTraumaticBrainCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := traumaticbrain.NewTBIService()

	// Create operation with hypotensive MAP (68 mmHg < 80 mmHg) -> Mannitol held (0.0 g)
	profile := domain.TBIPatientProfile{
		Species:                  domain.TBISpeciesCanineHighVelocityCranial,
		Breed:                    "Belgian Malinois",
		WeightKg:                 29.0,
		ModifiedGlasgowComaScore: 9,
		MeanArterialPressureMmHg: 68,
		HeartRateBpm:             142,
		EndTidalCO2MmHg:          46.0,
		PupillaryStatus:          domain.TBIPupilsAnisocoriaUnilateralMydriasis,
		CushingReflexPresent:     false,
	}

	created, err := svc.CreateOperation("INC-TBI-9804", "Zephyr", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "TBI-OP-104" || created.Status != domain.TBIStatusCushingHerniationCritical || !created.CushingHerniationSteroidAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if created.NeuroprotectivePlan.HypertonicSaline75PctMlKg != 4.0 || created.NeuroprotectivePlan.HypertonicSalineTotalDoseMl != 116.0 || created.NeuroprotectivePlan.Mannitol05GKgSlow20MinGrams != 0.0 {
		t.Errorf("expected 4.0 mL/kg HTS (116.0 mL) and 0.0 g Mannitol while hypotensive (MAP 68 < 80), got %+v", created.NeuroprotectivePlan)
	}

	// Stage neuroprotective plan after MAP restored to 94 mmHg
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.ModifiedGlasgowComaScore = 13
	stagedProfile.MeanArterialPressureMmHg = 94
	stagedProfile.HeartRateBpm = 104
	stagedProfile.EndTidalCO2MmHg = 38.0
	stagedProfile.PupillaryStatus = domain.TBIPupilsMioticPinpointBilateral

	staged, err := svc.StageNeuroprotectivePlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageNeuroprotectivePlan error: %v", err)
	}
	if staged.Status != domain.TBIStatusHypertonicMannitolStaged {
		t.Errorf("expected status %s, got %s", domain.TBIStatusHypertonicMannitolStaged, staged.Status)
	}
	if staged.NeuroprotectivePlan.Mannitol05GKgSlow20MinGrams != 14.5 || staged.NeuroprotectivePlan.HeadElevationAngleDegrees != 30 {
		t.Errorf("expected 14.5 g Mannitol and 30 deg head elevation, got %+v", staged.NeuroprotectivePlan)
	}
	if staged.PatientProfile.Breed != "Belgian Malinois" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record serial neuro check with improving MGCS 16 & responsive isocoric pupils
	checked, err := svc.RecordNeuroCheck(created.ID, domain.TBINeuroSerialCheck{
		ModifiedGlasgowComaScore: 16,
		MeanArterialPressureMmHg: 96,
		HeartRateBpm:             88,
		EndTidalCO2MmHg:          37.0,
		PupillaryStatus:          domain.TBIPupilsResponsiveIsocoric,
		ClinicianCallsign:        "DR-NEURO-22",
		Notes:                    "MGCS improved to 16/18; pupils now isocoric and responsive; MAP 96 mmHg; EtCO2 37 mmHg.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordNeuroCheck error: %v", err)
	}
	if len(checked.NeuroChecks) != 1 || checked.CushingHerniationSteroidAlert {
		t.Errorf("expected 1 neuro check and false alert when MGCS 16, MAP 96, EtCO2 37, isocoric pupils, got %+v", checked)
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.TBIDischargeClearance{
		FinalModifiedGlasgowScore:     17,
		FinalMAPMmHg:                  98,
		PupilsIsocoricResponsive:      true,
		GagSwallowReflexIntact:        true,
		AmbulatoryNormocapnicVerified: true,
		VetLicense:                    "VET-WA-88120",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.TBIStatusMGCSRecoveredNormocapnicCleared || cleared.CushingHerniationSteroidAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse check with Cushing's reflex / MGCS <= 9 reverts to CUSHING_REFLEX_ICP_HERNIATION_CRITICAL
	relapsed, err := svc.RecordNeuroCheck(created.ID, domain.TBINeuroSerialCheck{
		ModifiedGlasgowComaScore: 7,
		MeanArterialPressureMmHg: 152,
		HeartRateBpm:             44,
		EndTidalCO2MmHg:          50.0,
		PupillaryStatus:          domain.TBIPupilsAnisocoriaUnilateralMydriasis,
		ClinicianCallsign:        "DR-NEURO-22",
		Notes:                    "Acute ICP spike with Cushing's reflex and anisocoria.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordNeuroCheck error: %v", err)
	}
	if relapsed.Status != domain.TBIStatusCushingHerniationCritical || !relapsed.CushingHerniationSteroidAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to CUSHING_REFLEX_ICP_HERNIATION_CRITICAL, got %+v", relapsed)
	}
}

func TestTraumaticBrainValidationErrors(t *testing.T) {
	svc := traumaticbrain.NewTBIService()

	validProfile := domain.TBIPatientProfile{
		Species:                  domain.TBISpeciesCanineBluntVehicular,
		WeightKg:                 30.0,
		ModifiedGlasgowComaScore: 10,
		MeanArterialPressureMmHg: 110,
		HeartRateBpm:             90,
		EndTidalCO2MmHg:          38.0,
		PupillaryStatus:          domain.TBIPupilsMioticPinpointBilateral,
		CushingReflexPresent:     false,
	}

	if _, err := svc.CreateOperation("", "Atlas", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.TBIPatientProfile)
	}{
		{"invalid species", func(p *domain.TBIPatientProfile) { p.Species = "BOVINE" }},
		{"weight <= 0", func(p *domain.TBIPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.TBIPatientProfile) { p.WeightKg = 150 }},
		{"mgcs < 3", func(p *domain.TBIPatientProfile) { p.ModifiedGlasgowComaScore = 2 }},
		{"mgcs > 18", func(p *domain.TBIPatientProfile) { p.ModifiedGlasgowComaScore = 19 }},
		{"map < 30", func(p *domain.TBIPatientProfile) { p.MeanArterialPressureMmHg = 20 }},
		{"map > 240", func(p *domain.TBIPatientProfile) { p.MeanArterialPressureMmHg = 260 }},
		{"hr < 20", func(p *domain.TBIPatientProfile) { p.HeartRateBpm = 10 }},
		{"hr > 300", func(p *domain.TBIPatientProfile) { p.HeartRateBpm = 320 }},
		{"etco2 < 15", func(p *domain.TBIPatientProfile) { p.EndTidalCO2MmHg = 10.0 }},
		{"etco2 > 90", func(p *domain.TBIPatientProfile) { p.EndTidalCO2MmHg = 95.0 }},
		{"invalid pupils", func(p *domain.TBIPatientProfile) { p.PupillaryStatus = "UNKNOWN_PUPIL" }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Atlas", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageNeuroprotectivePlan("TBI-OP-101", p); err == nil {
				t.Errorf("expected StageNeuroprotectivePlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageNeuroprotectivePlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging plan on unknown op")
	}

	validCheck := domain.TBINeuroSerialCheck{
		ModifiedGlasgowComaScore: 14,
		MeanArterialPressureMmHg: 95,
		HeartRateBpm:             100,
		EndTidalCO2MmHg:          38.0,
		PupillaryStatus:          domain.TBIPupilsResponsiveIsocoric,
		ClinicianCallsign:        "DR-1",
	}
	if _, err := svc.RecordNeuroCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording neuro check on unknown op")
	}

	badChecks := []func(c *domain.TBINeuroSerialCheck){
		func(c *domain.TBINeuroSerialCheck) { c.ModifiedGlasgowComaScore = 1 },
		func(c *domain.TBINeuroSerialCheck) { c.MeanArterialPressureMmHg = 15 },
		func(c *domain.TBINeuroSerialCheck) { c.HeartRateBpm = 10 },
		func(c *domain.TBINeuroSerialCheck) { c.EndTidalCO2MmHg = 10.0 },
		func(c *domain.TBINeuroSerialCheck) { c.PupillaryStatus = "BAD" },
		func(c *domain.TBINeuroSerialCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordNeuroCheck("TBI-OP-101", c); err == nil {
			t.Errorf("expected RecordNeuroCheck validation error for %+v", c)
		}
	}

	validClearance := domain.TBIDischargeClearance{
		FinalModifiedGlasgowScore:     17,
		FinalMAPMmHg:                  96,
		PupilsIsocoricResponsive:      true,
		GagSwallowReflexIntact:        true,
		AmbulatoryNormocapnicVerified: true,
		VetLicense:                    "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.TBIDischargeClearance){
		func(d *domain.TBIDischargeClearance) { d.FinalModifiedGlasgowScore = 15 },
		func(d *domain.TBIDischargeClearance) { d.FinalModifiedGlasgowScore = 19 },
		func(d *domain.TBIDischargeClearance) { d.FinalMAPMmHg = 75 },
		func(d *domain.TBIDischargeClearance) { d.FinalMAPMmHg = 125 },
		func(d *domain.TBIDischargeClearance) { d.PupilsIsocoricResponsive = false },
		func(d *domain.TBIDischargeClearance) { d.GagSwallowReflexIntact = false },
		func(d *domain.TBIDischargeClearance) { d.AmbulatoryNormocapnicVerified = false },
		func(d *domain.TBIDischargeClearance) { d.VetLicense = "   " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("TBI-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}

	// Validate TraumaticBrainOperation error paths
	op := domain.TraumaticBrainOperation{
		ID:             "TBI-OP-101",
		IncidentID:     "INC-TBI-9801",
		PetName:        "Atlas",
		Status:         domain.TBIStatusCushingHerniationCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.TraumaticBrainOperation){
		func(o *domain.TraumaticBrainOperation) { o.ID = "" },
		func(o *domain.TraumaticBrainOperation) { o.IncidentID = "" },
		func(o *domain.TraumaticBrainOperation) { o.PetName = "" },
		func(o *domain.TraumaticBrainOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.TraumaticBrainOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected TraumaticBrainOperation.Validate error for %+v", cp)
		}
	}
}
