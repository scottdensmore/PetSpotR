package pneumothorax_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pneumothorax"
)

func TestPneumothoraxSeededOperationsAndMetrics(t *testing.T) {
	svc := pneumothorax.NewPneumothoraxService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("PTX-OP-101")
	if err != nil {
		t.Fatalf("expected PTX-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Maverick" || op101.IncidentID != "INC-PTX-9911" || op101.Status != domain.PneumothoraxStatusTensionAbsentGlideCritical {
		t.Errorf("unexpected PTX-OP-101 metadata: %+v", op101)
	}
	if !op101.TensionRadiographDelayAlert || op101.PatientProfile.WeightKg != 25.0 || op101.PatientProfile.RespiratoryRateBpm != 76 || op101.PatientProfile.SpO2Percent != 83.0 || op101.PatientProfile.TfastGlideSignPresent {
		t.Errorf("unexpected PTX-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.DecompressionPlan == nil || !op101.DecompressionPlan.CranialRibMarginVesselGuard || !op101.DecompressionPlan.ZeroPreDecompressionRadiographGuard || !op101.DecompressionPlan.ThoracostomyTubeIndicated {
		t.Errorf("unexpected PTX-OP-101 plan guards: %+v", op101.DecompressionPlan)
	}
	if op101.DecompressionPlan.NeedleLandmarkIntercostalSpace != "7th-9th ICS Dorsal Third" || op101.DecompressionPlan.ExpectedNegativePressureTapMl != 750.0 || op101.DecompressionPlan.ContinuousSuctionCmH2O != -15 || op101.DecompressionPlan.AutologousBloodPatchMlKg != 5.0 {
		t.Errorf("unexpected PTX-OP-101 plan values: %+v", op101.DecompressionPlan)
	}
	lowerAlert := strings.ToLower(op101.DecompressionPlan.PneumothoraxPitfallAlert)
	if !strings.Contains(lowerAlert, "radiograph") || !strings.Contains(lowerAlert, "cranial") || !strings.Contains(lowerAlert, "positive-pressure") {
		t.Errorf("expected pitfall alert to warn against pre-decompression radiograph, caudal vs cranial rib margin, and positive-pressure ventilation, got %q", op101.DecompressionPlan.PneumothoraxPitfallAlert)
	}
	if len(op101.DrainageChecks) != 1 {
		t.Errorf("expected 1 initial drainage check on PTX-OP-101, got %d", len(op101.DrainageChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected PTX-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("PTX-OP-102")
	if err != nil {
		t.Fatalf("expected PTX-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Bandit" || op102.IncidentID != "INC-PTX-9912" || op102.Status != domain.PneumothoraxStatusThoracocentesisTubeStaged {
		t.Errorf("unexpected PTX-OP-102 metadata: %+v", op102)
	}
	if op102.DecompressionPlan == nil || op102.DecompressionPlan.ExpectedNegativePressureTapMl != 126.0 || op102.DecompressionPlan.ThoracostomyTubeIndicated || op102.DecompressionPlan.AutologousBloodPatchMlKg != 0.0 {
		t.Errorf("unexpected PTX-OP-102 plan: %+v", op102.DecompressionPlan)
	}

	op103, err := svc.GetOperation("PTX-OP-103")
	if err != nil {
		t.Fatalf("expected PTX-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Tango" || op103.IncidentID != "INC-PTX-9913" || op103.Status != domain.PneumothoraxStatusPleuralSealTubeRemoved {
		t.Errorf("unexpected PTX-OP-103 metadata: %+v", op103)
	}
	if op103.TensionRadiographDelayAlert || op103.RemovalClearance == nil {
		t.Errorf("expected PTX-OP-103 to have RemovalClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.PneumothoraxStatusTensionAbsentGlideCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "PTX-OP-101" {
		t.Errorf("expected 1 critical op PTX-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.CriticalTensionCount != 1 || metrics.ThoracocentesisTubeStagedCount != 1 || metrics.PleuralSealRemovedCount != 1 || metrics.TensionRadiographAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestPneumothoraxCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := pneumothorax.NewPneumothoraxService()

	profile := domain.PneumothoraxPatientProfile{
		Species:                    domain.PneumothoraxSpeciesCanineSpontaneousBullae,
		Breed:                      "Alaskan Malamute",
		WeightKg:                   34.0,
		RespiratoryRateBpm:         68,
		SpO2Percent:                85.0,
		TfastGlideSignPresent:      false,
		CumulativeAirAspiratedMlKg: 54.0,
		TapCountLast24Hr:           3,
		OpenSuckingChestWound:      true,
	}

	created, err := svc.CreateOperation("INC-PTX-9914", "Goose", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "PTX-OP-104" || created.Status != domain.PneumothoraxStatusTensionAbsentGlideCritical || !created.TensionRadiographDelayAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if created.DecompressionPlan.ExpectedNegativePressureTapMl != 1020.0 || !created.DecompressionPlan.ThoracostomyTubeIndicated || created.DecompressionPlan.AutologousBloodPatchMlKg != 5.0 {
		t.Errorf("unexpected created plan: %+v", created.DecompressionPlan)
	}

	// Stage decompression plan on PTX-OP-104
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.RespiratoryRateBpm = 42
	stagedProfile.SpO2Percent = 94.0
	stagedProfile.TfastGlideSignPresent = true

	staged, err := svc.StageDecompressionPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageDecompressionPlan error: %v", err)
	}
	if staged.Status != domain.PneumothoraxStatusThoracocentesisTubeStaged {
		t.Errorf("expected status %s, got %s", domain.PneumothoraxStatusThoracocentesisTubeStaged, staged.Status)
	}
	if staged.PatientProfile.Breed != "Alaskan Malamute" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record drainage check with zero air production and restored glide sign
	checked, err := svc.RecordDrainageCheck(created.ID, domain.PleuralDrainageSerialCheck{
		AirAspiratedRateMlKgHr:       0.0,
		FluidAspiratedRateMlKgDay:    1.4,
		TfastGlideSignRestored:       true,
		SpO2Percent:                  97.0,
		SubcutaneousEmphysemaPresent: false,
		ClinicianCallsign:            "DR-PULM-34",
		Notes:                        "Zero air leak on -15 cmH2O continuous suction; bilateral TFAST glide sign restored.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordDrainageCheck error: %v", err)
	}
	if len(checked.DrainageChecks) != 1 || checked.TensionRadiographDelayAlert {
		t.Errorf("expected 1 drainage check and false alert when glide sign restored and SpO2 97%%, got %+v", checked)
	}

	// Issue removal clearance
	cleared, err := svc.IssueRemovalClearance(created.ID, domain.PneumothoraxRemovalClearance{
		ZeroAirProductionHours:      26,
		FinalFluidOutputMlKgDay:     1.2,
		BilateralGlideSignConfirmed: true,
		RoomAirSpO2Percent:          97.5,
		ChestTubeAsepticallyRemoved: true,
		VetLicense:                  "VET-WA-99140",
	})
	if err != nil {
		t.Fatalf("unexpected IssueRemovalClearance error: %v", err)
	}
	if cleared.Status != domain.PneumothoraxStatusPleuralSealTubeRemoved || cleared.TensionRadiographDelayAlert || cleared.RemovalClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.RemovalClearance.Summary == "" {
		t.Errorf("expected auto-populated removal clearance summary when empty")
	}

	// Relapse check with absent glide sign and hypoxemia reverts to critical
	relapsed, err := svc.RecordDrainageCheck(created.ID, domain.PleuralDrainageSerialCheck{
		AirAspiratedRateMlKgHr:       30.0,
		FluidAspiratedRateMlKgDay:    2.5,
		TfastGlideSignRestored:       false,
		SpO2Percent:                  82.0,
		SubcutaneousEmphysemaPresent: true,
		ClinicianCallsign:            "DR-PULM-34",
		Notes:                        "Recurrent bullae rupture with absent glide sign and SpO2 82%.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordDrainageCheck error: %v", err)
	}
	if relapsed.Status != domain.PneumothoraxStatusTensionAbsentGlideCritical || !relapsed.TensionRadiographDelayAlert || relapsed.RemovalClearance != nil {
		t.Errorf("expected relapse to revert to TENSION_PNEUMOTHORAX_ABSENT_GLIDE_CRITICAL, got %+v", relapsed)
	}
}

func TestPneumothoraxValidationErrors(t *testing.T) {
	svc := pneumothorax.NewPneumothoraxService()

	validProfile := domain.PneumothoraxPatientProfile{
		Species:                    domain.PneumothoraxSpeciesCanineTraumaticFlailChest,
		WeightKg:                   20.0,
		RespiratoryRateBpm:         48,
		SpO2Percent:                91.0,
		TfastGlideSignPresent:      true,
		CumulativeAirAspiratedMlKg: 35.0,
		TapCountLast24Hr:           2,
		OpenSuckingChestWound:      false,
	}

	if _, err := svc.CreateOperation("", "Goose", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.PneumothoraxPatientProfile)
	}{
		{"invalid species", func(p *domain.PneumothoraxPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.PneumothoraxPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.PneumothoraxPatientProfile) { p.WeightKg = 140 }},
		{"rr too low", func(p *domain.PneumothoraxPatientProfile) { p.RespiratoryRateBpm = 4 }},
		{"rr too high", func(p *domain.PneumothoraxPatientProfile) { p.RespiratoryRateBpm = 200 }},
		{"spo2 too low", func(p *domain.PneumothoraxPatientProfile) { p.SpO2Percent = 40.0 }},
		{"spo2 too high", func(p *domain.PneumothoraxPatientProfile) { p.SpO2Percent = 105.0 }},
		{"cumulative air negative", func(p *domain.PneumothoraxPatientProfile) { p.CumulativeAirAspiratedMlKg = -5.0 }},
		{"cumulative air too high", func(p *domain.PneumothoraxPatientProfile) { p.CumulativeAirAspiratedMlKg = 600.0 }},
		{"tap count negative", func(p *domain.PneumothoraxPatientProfile) { p.TapCountLast24Hr = -1 }},
		{"tap count too high", func(p *domain.PneumothoraxPatientProfile) { p.TapCountLast24Hr = 35 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Goose", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageDecompressionPlan("PTX-OP-101", p); err == nil {
				t.Errorf("expected StageDecompressionPlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageDecompressionPlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging plan on unknown op")
	}

	validCheck := domain.PleuralDrainageSerialCheck{
		AirAspiratedRateMlKgHr:       2.0,
		FluidAspiratedRateMlKgDay:    1.5,
		TfastGlideSignRestored:       true,
		SpO2Percent:                  95.0,
		SubcutaneousEmphysemaPresent: false,
		ClinicianCallsign:            "DR-1",
	}
	if _, err := svc.RecordDrainageCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording drainage check on unknown op")
	}

	badChecks := []func(c *domain.PleuralDrainageSerialCheck){
		func(c *domain.PleuralDrainageSerialCheck) { c.AirAspiratedRateMlKgHr = -1.0 },
		func(c *domain.PleuralDrainageSerialCheck) { c.AirAspiratedRateMlKgHr = 250.0 },
		func(c *domain.PleuralDrainageSerialCheck) { c.FluidAspiratedRateMlKgDay = -0.5 },
		func(c *domain.PleuralDrainageSerialCheck) { c.FluidAspiratedRateMlKgDay = 120.0 },
		func(c *domain.PleuralDrainageSerialCheck) { c.SpO2Percent = 40.0 },
		func(c *domain.PleuralDrainageSerialCheck) { c.SpO2Percent = 102.0 },
		func(c *domain.PleuralDrainageSerialCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordDrainageCheck("PTX-OP-101", c); err == nil {
			t.Errorf("expected RecordDrainageCheck validation error for %+v", c)
		}
	}

	validClearance := domain.PneumothoraxRemovalClearance{
		ZeroAirProductionHours:      24,
		FinalFluidOutputMlKgDay:     1.5,
		BilateralGlideSignConfirmed: true,
		RoomAirSpO2Percent:          96.0,
		ChestTubeAsepticallyRemoved: true,
		VetLicense:                  "VET-123",
	}
	if _, err := svc.IssueRemovalClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.PneumothoraxRemovalClearance){
		func(d *domain.PneumothoraxRemovalClearance) { d.ZeroAirProductionHours = 12 },
		func(d *domain.PneumothoraxRemovalClearance) { d.ZeroAirProductionHours = 300 },
		func(d *domain.PneumothoraxRemovalClearance) { d.FinalFluidOutputMlKgDay = -0.5 },
		func(d *domain.PneumothoraxRemovalClearance) { d.FinalFluidOutputMlKgDay = 2.5 },
		func(d *domain.PneumothoraxRemovalClearance) { d.BilateralGlideSignConfirmed = false },
		func(d *domain.PneumothoraxRemovalClearance) { d.RoomAirSpO2Percent = 92.0 },
		func(d *domain.PneumothoraxRemovalClearance) { d.RoomAirSpO2Percent = 101.0 },
		func(d *domain.PneumothoraxRemovalClearance) { d.ChestTubeAsepticallyRemoved = false },
		func(d *domain.PneumothoraxRemovalClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueRemovalClearance("PTX-OP-101", d); err == nil {
			t.Errorf("expected IssueRemovalClearance validation error for %+v", d)
		}
	}

	// Validate PneumothoraxOperation error paths
	op := domain.PneumothoraxOperation{
		ID:             "PTX-OP-101",
		IncidentID:     "INC-PTX-9911",
		PetName:        "Maverick",
		Status:         domain.PneumothoraxStatusTensionAbsentGlideCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.PneumothoraxOperation){
		func(o *domain.PneumothoraxOperation) { o.ID = "" },
		func(o *domain.PneumothoraxOperation) { o.IncidentID = "" },
		func(o *domain.PneumothoraxOperation) { o.PetName = "" },
		func(o *domain.PneumothoraxOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.PneumothoraxOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected PneumothoraxOperation.Validate error for %+v", cp)
		}
	}
}
