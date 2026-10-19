package anaphylaxishalo_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/anaphylaxishalo"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestAnaphylaxisHaloSeededOperationsAndMetrics(t *testing.T) {
	svc := anaphylaxishalo.NewAnaphylaxisService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("AH-OP-101")
	if err != nil {
		t.Fatalf("expected AH-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Waffle" || op101.IncidentID != "INC-AH-9901" || op101.Status != domain.AnaphylaxisStatusShockHaloCritical {
		t.Errorf("unexpected AH-OP-101 metadata: %+v", op101)
	}
	if !op101.HepaticCongestionCoagulopathyAlert || op101.PatientProfile.WeightKg != 6.8 || op101.PatientProfile.GallbladderWallThicknessMm != 6.4 || !op101.PatientProfile.GallbladderHaloSignPresent {
		t.Errorf("unexpected AH-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.ResuscitationPlan == nil || !op101.ResuscitationPlan.ZeroIVUndilutedEpiSurgicalCholecystectomyGuard {
		t.Errorf("unexpected AH-OP-101 plan: %+v", op101.ResuscitationPlan)
	}
	if op101.ResuscitationPlan.EpinephrineIM001MgKgDoseMg != 0.07 || op101.ResuscitationPlan.EpinephrineCRI005McgKgMin != 0.34 || op101.ResuscitationPlan.IsotonicCrystalloidBolusMl != 136.0 || op101.ResuscitationPlan.FreshFrozenPlasma15MlKgMl != 102.0 || op101.ResuscitationPlan.DiphenhydramineIM2MgKgMg != 13.6 || op101.ResuscitationPlan.FamotidineIV05MgKgMg != 3.4 {
		t.Errorf("unexpected AH-OP-101 plan values: %+v", op101.ResuscitationPlan)
	}
	lowerAlert := strings.ToLower(op101.ResuscitationPlan.AnaphylaxisPitfallAlert)
	if !strings.Contains(lowerAlert, "halo sign") || !strings.Contains(lowerAlert, "cholecystectomy") || !strings.Contains(lowerAlert, "diphenhydramine") || !strings.Contains(lowerAlert, "24h") {
		t.Errorf("expected pitfall alert to warn against halo sign cholecystectomy, diphenhydramine IV bolus, and 24h biphasic observation, got %q", op101.ResuscitationPlan.AnaphylaxisPitfallAlert)
	}
	if len(op101.BiphasicChecks) != 1 {
		t.Errorf("expected 1 initial biphasic check on AH-OP-101, got %d", len(op101.BiphasicChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected AH-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("AH-OP-102")
	if err != nil {
		t.Fatalf("expected AH-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Bramble" || op102.IncidentID != "INC-AH-9902" || op102.Status != domain.AnaphylaxisStatusEpinephrineFFPStaged {
		t.Errorf("unexpected AH-OP-102 metadata: %+v", op102)
	}
	if op102.ResuscitationPlan == nil || op102.ResuscitationPlan.IsotonicCrystalloidBolusMl != 44.0 || op102.ResuscitationPlan.FreshFrozenPlasma15MlKgMl != 0.0 || op102.ResuscitationPlan.EpinephrineCRI005McgKgMin != 0.0 {
		t.Errorf("expected feline 10 mL/kg (44.0 mL) crystalloid and 0 FFP/CRI on AH-OP-102, got %+v", op102.ResuscitationPlan)
	}

	op103, err := svc.GetOperation("AH-OP-103")
	if err != nil {
		t.Fatalf("expected AH-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Clover" || op103.IncidentID != "INC-AH-9903" || op103.Status != domain.AnaphylaxisStatusBiphasic24HrCleared {
		t.Errorf("unexpected AH-OP-103 metadata: %+v", op103)
	}
	if op103.HepaticCongestionCoagulopathyAlert || op103.DischargeClearance == nil {
		t.Errorf("expected AH-OP-103 to have DischargeClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.AnaphylaxisStatusShockHaloCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "AH-OP-101" {
		t.Errorf("expected 1 critical op AH-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.CriticalShockHaloCount != 1 || metrics.EpinephrineFFPStagedCount != 1 || metrics.Biphasic24HrClearedCount != 1 || metrics.HepaticCoagulopathyAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestAnaphylaxisHaloCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := anaphylaxishalo.NewAnaphylaxisService()

	// Test Epinephrine cap under 20 kg (e.g. 19.5 kg -> 0.195 -> 0.20 mg; or wait: 0.01 * 35 kg = 0.35 mg, >= 20 kg capped at 0.5 mg; 60 kg -> 0.60 mg capped at 0.50 mg)
	profileHeavy := domain.AnaphylaxisPatientProfile{
		Species:                    domain.AnaphylaxisSpeciesCanineHepaticSplanchnic,
		Breed:                      "Great Dane",
		WeightKg:                   60.0,
		GallbladderWallThicknessMm: 5.8,
		GallbladderHaloSignPresent: true,
		MeanArterialPressureMmHg:   52,
		AlanineAminotransferaseUL:  1950,
		PackedCellVolumePercent:    65.0,
		ProthrombinTimeSeconds:     24.0,
	}

	created, err := svc.CreateOperation("INC-AH-9904", "Peanut", profileHeavy)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "AH-OP-104" || created.Status != domain.AnaphylaxisStatusShockHaloCritical || !created.HepaticCongestionCoagulopathyAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	// 60 kg * 0.01 mg/kg = 0.60 mg, capped at 0.50 mg for >= 20 kg
	if created.ResuscitationPlan.EpinephrineIM001MgKgDoseMg != 0.50 || created.ResuscitationPlan.FreshFrozenPlasma15MlKgMl != 900.0 || created.ResuscitationPlan.EpinephrineCRI005McgKgMin != 3.0 {
		t.Errorf("expected Epinephrine IM capped at 0.50 mg, FFP 900.0 mL, CRI 3.0 mcg/min, got %+v", created.ResuscitationPlan)
	}

	// Stage plan on AH-OP-104 after initial IM Epinephrine & fluid bolus
	stagedProfile := profileHeavy
	stagedProfile.Breed = ""
	stagedProfile.GallbladderWallThicknessMm = 3.6
	stagedProfile.MeanArterialPressureMmHg = 72
	stagedProfile.PackedCellVolumePercent = 54.0
	stagedProfile.ProthrombinTimeSeconds = 15.0

	staged, err := svc.StageResuscitationPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageResuscitationPlan error: %v", err)
	}
	if staged.Status != domain.AnaphylaxisStatusEpinephrineFFPStaged {
		t.Errorf("expected status %s, got %s", domain.AnaphylaxisStatusEpinephrineFFPStaged, staged.Status)
	}
	if staged.ResuscitationPlan.EpinephrineCRI005McgKgMin != 0.0 || staged.ResuscitationPlan.FreshFrozenPlasma15MlKgMl != 0.0 {
		t.Errorf("expected 0 CRI and 0 FFP when MAP 72 and PT 15.0, got %+v", staged.ResuscitationPlan)
	}
	if staged.PatientProfile.Breed != "Great Dane" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record biphasic check with normotension & resolved gallbladder wall edema
	checked, err := svc.RecordBiphasicCheck(created.ID, domain.AnaphylaxisBiphasicCheck{
		MeanArterialPressureMmHg:   86,
		GallbladderWallThicknessMm: 2.2,
		PackedCellVolumePercent:    46.0,
		AlanineAminotransferaseUL:  380,
		BiphasicRelapseSignsNoted:  false,
		ClinicianCallsign:          "DR-ECC-55",
		Notes:                      "Gallbladder wall edema resolved to 2.2 mm; normotensive MAP 86 mmHg; PCV 46%.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordBiphasicCheck error: %v", err)
	}
	if len(checked.BiphasicChecks) != 1 || checked.HepaticCongestionCoagulopathyAlert {
		t.Errorf("expected 1 biphasic check and false alert when MAP >= 75 and GB wall <= 3.0, got %+v", checked)
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.AnaphylaxisDischargeClearance{
		BiphasicObservationHours:   24,
		FinalMAPMmHg:               88,
		GallbladderHaloResolved:    true,
		CoagulopathyNormalized:     true,
		EpiPenAutoInjectorEducated: true,
		VetLicense:                 "VET-WA-99120",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.AnaphylaxisStatusBiphasic24HrCleared || cleared.HepaticCongestionCoagulopathyAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse check with biphasic hypotension & recurrent Halo Sign reverts to critical
	relapsed, err := svc.RecordBiphasicCheck(created.ID, domain.AnaphylaxisBiphasicCheck{
		MeanArterialPressureMmHg:   54,
		GallbladderWallThicknessMm: 5.5,
		PackedCellVolumePercent:    63.0,
		AlanineAminotransferaseUL:  1600,
		BiphasicRelapseSignsNoted:  true,
		ClinicianCallsign:          "DR-ECC-55",
		Notes:                      "Biphasic anaphylactic relapse at 14h with recurrent hypotension MAP 54 mmHg and GB wall 5.5 mm.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordBiphasicCheck error: %v", err)
	}
	if relapsed.Status != domain.AnaphylaxisStatusShockHaloCritical || !relapsed.HepaticCongestionCoagulopathyAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to ANAPHYLACTIC_SHOCK_HALO_SIGN_CRITICAL, got %+v", relapsed)
	}

	// Issue discharge clearance when GB wall > 2.5 mm normalizes GB wall thickness to 2.0 mm
	clearedAgain, err := svc.IssueDischargeClearance(created.ID, domain.AnaphylaxisDischargeClearance{
		BiphasicObservationHours:   28,
		FinalMAPMmHg:               90,
		GallbladderHaloResolved:    true,
		CoagulopathyNormalized:     true,
		EpiPenAutoInjectorEducated: true,
		VetLicense:                 "VET-WA-99120",
		Summary:                    "Cleared after secondary 28h observation.",
	})
	if err != nil {
		t.Fatalf("unexpected second IssueDischargeClearance error: %v", err)
	}
	if clearedAgain.PatientProfile.GallbladderWallThicknessMm != 2.0 {
		t.Errorf("expected GallbladderWallThicknessMm normalized to 2.0 on clearance, got %v", clearedAgain.PatientProfile.GallbladderWallThicknessMm)
	}
}

func TestAnaphylaxisHaloValidationErrors(t *testing.T) {
	svc := anaphylaxishalo.NewAnaphylaxisService()

	validProfile := domain.AnaphylaxisPatientProfile{
		Species:                    domain.AnaphylaxisSpeciesCanineHepaticSplanchnic,
		WeightKg:                   12.0,
		GallbladderWallThicknessMm: 4.5,
		GallbladderHaloSignPresent: true,
		MeanArterialPressureMmHg:   58,
		AlanineAminotransferaseUL:  980,
		PackedCellVolumePercent:    62.0,
		ProthrombinTimeSeconds:     19.5,
	}

	if _, err := svc.CreateOperation("", "Peanut", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.AnaphylaxisPatientProfile)
	}{
		{"invalid species", func(p *domain.AnaphylaxisPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.AnaphylaxisPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.AnaphylaxisPatientProfile) { p.WeightKg = 140 }},
		{"gb wall too low", func(p *domain.AnaphylaxisPatientProfile) { p.GallbladderWallThicknessMm = 0.2 }},
		{"gb wall too high", func(p *domain.AnaphylaxisPatientProfile) { p.GallbladderWallThicknessMm = 18.0 }},
		{"map too low", func(p *domain.AnaphylaxisPatientProfile) { p.MeanArterialPressureMmHg = 10 }},
		{"map too high", func(p *domain.AnaphylaxisPatientProfile) { p.MeanArterialPressureMmHg = 230 }},
		{"alt too low", func(p *domain.AnaphylaxisPatientProfile) { p.AlanineAminotransferaseUL = 5 }},
		{"alt too high", func(p *domain.AnaphylaxisPatientProfile) { p.AlanineAminotransferaseUL = 30000 }},
		{"pcv too low", func(p *domain.AnaphylaxisPatientProfile) { p.PackedCellVolumePercent = 5.0 }},
		{"pcv too high", func(p *domain.AnaphylaxisPatientProfile) { p.PackedCellVolumePercent = 90.0 }},
		{"pt too low", func(p *domain.AnaphylaxisPatientProfile) { p.ProthrombinTimeSeconds = 2.0 }},
		{"pt too high", func(p *domain.AnaphylaxisPatientProfile) { p.ProthrombinTimeSeconds = 150.0 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Peanut", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageResuscitationPlan("AH-OP-101", p); err == nil {
				t.Errorf("expected StageResuscitationPlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageResuscitationPlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging plan on unknown op")
	}

	validCheck := domain.AnaphylaxisBiphasicCheck{
		MeanArterialPressureMmHg:   78,
		GallbladderWallThicknessMm: 2.8,
		PackedCellVolumePercent:    48.0,
		AlanineAminotransferaseUL:  450,
		BiphasicRelapseSignsNoted:  false,
		ClinicianCallsign:          "DR-1",
	}
	if _, err := svc.RecordBiphasicCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording biphasic check on unknown op")
	}

	badChecks := []func(c *domain.AnaphylaxisBiphasicCheck){
		func(c *domain.AnaphylaxisBiphasicCheck) { c.MeanArterialPressureMmHg = 10 },
		func(c *domain.AnaphylaxisBiphasicCheck) { c.GallbladderWallThicknessMm = 0.1 },
		func(c *domain.AnaphylaxisBiphasicCheck) { c.PackedCellVolumePercent = 5.0 },
		func(c *domain.AnaphylaxisBiphasicCheck) { c.AlanineAminotransferaseUL = 2 },
		func(c *domain.AnaphylaxisBiphasicCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordBiphasicCheck("AH-OP-101", c); err == nil {
			t.Errorf("expected RecordBiphasicCheck validation error for %+v", c)
		}
	}

	validClearance := domain.AnaphylaxisDischargeClearance{
		BiphasicObservationHours:   24,
		FinalMAPMmHg:               85,
		GallbladderHaloResolved:    true,
		CoagulopathyNormalized:     true,
		EpiPenAutoInjectorEducated: true,
		VetLicense:                 "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.AnaphylaxisDischargeClearance){
		func(d *domain.AnaphylaxisDischargeClearance) { d.BiphasicObservationHours = 18 },
		func(d *domain.AnaphylaxisDischargeClearance) { d.BiphasicObservationHours = 200 },
		func(d *domain.AnaphylaxisDischargeClearance) { d.FinalMAPMmHg = 68 },
		func(d *domain.AnaphylaxisDischargeClearance) { d.FinalMAPMmHg = 135 },
		func(d *domain.AnaphylaxisDischargeClearance) { d.GallbladderHaloResolved = false },
		func(d *domain.AnaphylaxisDischargeClearance) { d.CoagulopathyNormalized = false },
		func(d *domain.AnaphylaxisDischargeClearance) { d.EpiPenAutoInjectorEducated = false },
		func(d *domain.AnaphylaxisDischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("AH-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}

	// Validate AnaphylaxisHaloOperation error paths
	op := domain.AnaphylaxisHaloOperation{
		ID:             "AH-OP-101",
		IncidentID:     "INC-AH-9901",
		PetName:        "Waffle",
		Status:         domain.AnaphylaxisStatusShockHaloCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.AnaphylaxisHaloOperation){
		func(o *domain.AnaphylaxisHaloOperation) { o.ID = "" },
		func(o *domain.AnaphylaxisHaloOperation) { o.IncidentID = "" },
		func(o *domain.AnaphylaxisHaloOperation) { o.PetName = "" },
		func(o *domain.AnaphylaxisHaloOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.AnaphylaxisHaloOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected AnaphylaxisHaloOperation.Validate error for %+v", cp)
		}
	}
}
