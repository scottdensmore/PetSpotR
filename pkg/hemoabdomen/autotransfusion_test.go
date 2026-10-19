package hemoabdomen_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/hemoabdomen"
)

func TestHemoabdomenSeededOperationsAndMetrics(t *testing.T) {
	svc := hemoabdomen.NewHemoabdomenService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("HA-OP-101")
	if err != nil {
		t.Fatalf("expected HA-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Apollo" || op101.IncidentID != "INC-HA-9401" || op101.Status != domain.HemoabdomenStatusHemorrhagicShockCritical {
		t.Errorf("unexpected HA-OP-101 metadata: %+v", op101)
	}
	if !op101.ClotPoppingNeoplasiaAlert || op101.PatientProfile.WeightKg != 34.0 || op101.PatientProfile.MeanArterialPressureMmHg != 48 {
		t.Errorf("unexpected HA-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.ResuscitationPlan == nil ||
		op101.ResuscitationPlan.PermissiveHypotensionTargetMAPMmHg != 65 ||
		!op101.ResuscitationPlan.AutotransfusionIndicated ||
		op101.ResuscitationPlan.MicronFilterSizeUm != 170 ||
		!op101.ResuscitationPlan.ZeroAbdominalCompressionWrapGuard ||
		op101.ResuscitationPlan.NorepinephrineCRIMcgKgMin != 0.1 {
		t.Errorf("unexpected HA-OP-101 resuscitation plan: %+v", op101.ResuscitationPlan)
	}
	lowerAlert := strings.ToLower(op101.ResuscitationPlan.HemoabdomenPitfallAlert)
	if !strings.Contains(lowerAlert, "90 ml/kg") || !strings.Contains(lowerAlert, "wrap") || !strings.Contains(lowerAlert, "170") {
		t.Errorf("expected pitfall alert to warn against 90 mL/kg shock boluses, compressive wraps, and 170–260 µm filter, got %q", op101.ResuscitationPlan.HemoabdomenPitfallAlert)
	}
	if len(op101.HemodynamicChecks) != 1 {
		t.Errorf("expected 1 initial hemodynamic check on HA-OP-101, got %d", len(op101.HemodynamicChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected HA-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("HA-OP-102")
	if err != nil {
		t.Fatalf("expected HA-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Sadie" || op102.IncidentID != "INC-HA-9402" || op102.Status != domain.HemoabdomenStatusAutotransfusionTXAStaged {
		t.Errorf("unexpected HA-OP-102 metadata: %+v", op102)
	}
	if op102.ResuscitationPlan == nil || op102.ResuscitationPlan.AutotransfusionIndicated {
		t.Errorf("expected AutotransfusionIndicated=false for suspected splenic hemangiosarcoma on HA-OP-102, got %+v", op102.ResuscitationPlan)
	}

	op103, err := svc.GetOperation("HA-OP-103")
	if err != nil {
		t.Fatalf("expected HA-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Ranger" || op103.IncidentID != "INC-HA-9403" || op103.Status != domain.HemoabdomenStatusHemostasisRestoredCleared {
		t.Errorf("unexpected HA-OP-103 metadata: %+v", op103)
	}
	if op103.ClotPoppingNeoplasiaAlert || op103.HemostasisClearance == nil {
		t.Errorf("expected HA-OP-103 to have HemostasisClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.HemoabdomenStatusHemorrhagicShockCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "HA-OP-101" {
		t.Errorf("expected 1 critical op HA-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 ||
		metrics.HemorrhagicShockCriticalCount != 1 ||
		metrics.AutotransfusionTXAStagedCount != 1 ||
		metrics.HemostasisClearedCount != 1 ||
		metrics.ClotPoppingNeoplasiaAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestHemoabdomenCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := hemoabdomen.NewHemoabdomenService()

	profile := domain.HemoabdomenPatientProfile{
		Species:                     domain.HemoabdomenSpeciesCanineBluntTraumaHepatic,
		Breed:                       "Rottweiler",
		WeightKg:                    38.0,
		AfastAFS4Score:              4,
		EffusionPCVPercent:          40.0,
		PeripheralPCVPercent:        30.0,
		MeanArterialPressureMmHg:    50,
		PlasmaLactateMmolL:          6.8,
		EffusionNonClottingVerified: true,
		SuspectedNeoplasia:          false,
	}

	created, err := svc.CreateOperation("INC-HA-9404", "Titan", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "HA-OP-104" || created.Status != domain.HemoabdomenStatusHemorrhagicShockCritical || !created.ClotPoppingNeoplasiaAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if created.ResuscitationPlan == nil ||
		created.ResuscitationPlan.TranexamicAcid10MgKgDoseMg != 380.0 ||
		!created.ResuscitationPlan.AutotransfusionIndicated ||
		created.ResuscitationPlan.AutotransfusionMaxCollectMl != 570.0 ||
		created.ResuscitationPlan.NorepinephrineCRIMcgKgMin != 0.1 {
		t.Errorf("unexpected created resuscitation plan: %+v", created.ResuscitationPlan)
	}

	// Stage resuscitation plan on HA-OP-104 (preserving Breed when empty)
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.MeanArterialPressureMmHg = 60
	stagedProfile.PlasmaLactateMmolL = 4.2

	staged, err := svc.StageResuscitationPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageResuscitationPlan error: %v", err)
	}
	if staged.Status != domain.HemoabdomenStatusAutotransfusionTXAStaged {
		t.Errorf("expected status %s, got %s", domain.HemoabdomenStatusAutotransfusionTXAStaged, staged.Status)
	}
	if staged.ResuscitationPlan.NorepinephrineCRIMcgKgMin != 0.0 {
		t.Errorf("expected Norepinephrine CRI 0.0 at MAP 60, got %v", staged.ResuscitationPlan.NorepinephrineCRIMcgKgMin)
	}
	if staged.PatientProfile.Breed != "Rottweiler" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Verify splenic hemangiosarcoma auto-sets SuspectedNeoplasia
	neoplasiaOp, err := svc.CreateOperation("INC-HA-9405", "Bella", domain.HemoabdomenPatientProfile{
		Species:                     domain.HemoabdomenSpeciesCanineSplenicHemangiosarcoma,
		WeightKg:                    25.0,
		AfastAFS4Score:              3,
		EffusionPCVPercent:          35.0,
		PeripheralPCVPercent:        26.0,
		MeanArterialPressureMmHg:    62,
		PlasmaLactateMmolL:          3.8,
		EffusionNonClottingVerified: true,
		SuspectedNeoplasia:          false,
	})
	if err != nil {
		t.Fatalf("unexpected CreateOperation error for Bella: %v", err)
	}
	if !neoplasiaOp.PatientProfile.SuspectedNeoplasia || neoplasiaOp.ResuscitationPlan.AutotransfusionIndicated {
		t.Errorf("expected SuspectedNeoplasia=true and AutotransfusionIndicated=false for splenic hemangiosarcoma")
	}

	// Record hemodynamic check (improved MAP 72, lactate 2.2)
	withCheck, err := svc.RecordHemodynamicCheck(created.ID, domain.HemoabdomenHemodynamicCheck{
		MeanArterialPressureMmHg: 72,
		HeartRateBpm:             112,
		PeripheralPCVPercent:     33.0,
		TotalSolidsGdl:           5.6,
		PlasmaLactateMmolL:       2.2,
		AutotransfusedVolumeMl:   420.0,
		ClinicianCallsign:        "DR-TRAUMA-14",
		Notes:                    "420 mL autologous shed blood reinfused via 170 µm filter; MAP stabilized at 72 mmHg.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordHemodynamicCheck error: %v", err)
	}
	if len(withCheck.HemodynamicChecks) != 1 || withCheck.ClotPoppingNeoplasiaAlert {
		t.Errorf("expected 1 hemodynamic check and false alert at MAP 72 / lactate 2.2, got %+v", withCheck)
	}

	// Issue hemostasis clearance
	cleared, err := svc.IssueHemostasisClearance(created.ID, domain.HemoabdomenHemostasisClearance{
		FinalMAPMmHg:                 84,
		FinalPeripheralPCVPercent:    34.0,
		FinalPlasmaLactateMmolL:      1.5,
		SurgicalHemostasisConfirmed:  true,
		RepeatAfastFreeFluidResolved: true,
		VetLicense:                   "VET-OR-77219",
	})
	if err != nil {
		t.Fatalf("unexpected IssueHemostasisClearance error: %v", err)
	}
	if cleared.Status != domain.HemoabdomenStatusHemostasisRestoredCleared || cleared.ClotPoppingNeoplasiaAlert || cleared.HemostasisClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.HemostasisClearance.Summary == "" {
		t.Errorf("expected auto-populated hemostasis clearance summary when empty")
	}

	// Relapse hemodynamic check with severe hypotension (MAP 44) transitions back to critical
	relapsed, err := svc.RecordHemodynamicCheck(created.ID, domain.HemoabdomenHemodynamicCheck{
		MeanArterialPressureMmHg: 44,
		HeartRateBpm:             178,
		PeripheralPCVPercent:     16.0,
		TotalSolidsGdl:           3.8,
		PlasmaLactateMmolL:       8.1,
		AutotransfusedVolumeMl:   0.0,
		ClinicianCallsign:        "DR-TRAUMA-14",
		Notes:                    "Re-bleed hypotension detected.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordHemodynamicCheck error: %v", err)
	}
	if relapsed.Status != domain.HemoabdomenStatusHemorrhagicShockCritical || !relapsed.ClotPoppingNeoplasiaAlert || relapsed.HemostasisClearance != nil {
		t.Errorf("expected relapse to revert to HEMORRHAGIC_SHOCK_HEMOABDOMEN_CRITICAL, got %+v", relapsed)
	}
}

func TestHemoabdomenValidationErrors(t *testing.T) {
	svc := hemoabdomen.NewHemoabdomenService()

	validProfile := domain.HemoabdomenPatientProfile{
		Species:                     domain.HemoabdomenSpeciesCanineBluntTraumaHepatic,
		WeightKg:                    25.0,
		AfastAFS4Score:              3,
		EffusionPCVPercent:          35.0,
		PeripheralPCVPercent:        28.0,
		MeanArterialPressureMmHg:    58,
		PlasmaLactateMmolL:          4.5,
		EffusionNonClottingVerified: true,
		SuspectedNeoplasia:          false,
	}

	if _, err := svc.CreateOperation("", "Apollo", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.HemoabdomenPatientProfile)
	}{
		{"invalid species", func(p *domain.HemoabdomenPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.HemoabdomenPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.HemoabdomenPatientProfile) { p.WeightKg = 140 }},
		{"afs < 0", func(p *domain.HemoabdomenPatientProfile) { p.AfastAFS4Score = -1 }},
		{"afs > 4", func(p *domain.HemoabdomenPatientProfile) { p.AfastAFS4Score = 5 }},
		{"effusion pcv < 0", func(p *domain.HemoabdomenPatientProfile) { p.EffusionPCVPercent = -1 }},
		{"effusion pcv > 75", func(p *domain.HemoabdomenPatientProfile) { p.EffusionPCVPercent = 80 }},
		{"peripheral pcv < 5", func(p *domain.HemoabdomenPatientProfile) { p.PeripheralPCVPercent = 3 }},
		{"peripheral pcv > 75", func(p *domain.HemoabdomenPatientProfile) { p.PeripheralPCVPercent = 80 }},
		{"map < 20", func(p *domain.HemoabdomenPatientProfile) { p.MeanArterialPressureMmHg = 15 }},
		{"map > 180", func(p *domain.HemoabdomenPatientProfile) { p.MeanArterialPressureMmHg = 200 }},
		{"lactate < 0.5", func(p *domain.HemoabdomenPatientProfile) { p.PlasmaLactateMmolL = 0.2 }},
		{"lactate > 25", func(p *domain.HemoabdomenPatientProfile) { p.PlasmaLactateMmolL = 30 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Apollo", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageResuscitationPlan("HA-OP-101", p); err == nil {
				t.Errorf("expected StageResuscitationPlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageResuscitationPlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging plan on unknown op")
	}

	// Stage splenic hemangiosarcoma branch
	splenicProfile := validProfile
	splenicProfile.Species = domain.HemoabdomenSpeciesCanineSplenicHemangiosarcoma
	if _, err := svc.StageResuscitationPlan("HA-OP-101", splenicProfile); err != nil {
		t.Errorf("unexpected error staging splenic profile: %v", err)
	}

	validCheck := domain.HemoabdomenHemodynamicCheck{
		MeanArterialPressureMmHg: 68,
		HeartRateBpm:             120,
		PeripheralPCVPercent:     30.0,
		TotalSolidsGdl:           5.4,
		PlasmaLactateMmolL:       2.4,
		AutotransfusedVolumeMl:   200.0,
		ClinicianCallsign:        "DR-1",
	}
	if _, err := svc.RecordHemodynamicCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording check on unknown op")
	}

	badChecks := []func(c *domain.HemoabdomenHemodynamicCheck){
		func(c *domain.HemoabdomenHemodynamicCheck) { c.MeanArterialPressureMmHg = 10 },
		func(c *domain.HemoabdomenHemodynamicCheck) { c.HeartRateBpm = 20 },
		func(c *domain.HemoabdomenHemodynamicCheck) { c.PeripheralPCVPercent = 2.0 },
		func(c *domain.HemoabdomenHemodynamicCheck) { c.TotalSolidsGdl = 0.5 },
		func(c *domain.HemoabdomenHemodynamicCheck) { c.PlasmaLactateMmolL = 0.1 },
		func(c *domain.HemoabdomenHemodynamicCheck) { c.AutotransfusedVolumeMl = -10 },
		func(c *domain.HemoabdomenHemodynamicCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordHemodynamicCheck("HA-OP-101", c); err == nil {
			t.Errorf("expected RecordHemodynamicCheck validation error for %+v", c)
		}
	}

	validClearance := domain.HemoabdomenHemostasisClearance{
		FinalMAPMmHg:                 82,
		FinalPeripheralPCVPercent:    31.0,
		FinalPlasmaLactateMmolL:      1.6,
		SurgicalHemostasisConfirmed:  true,
		RepeatAfastFreeFluidResolved: true,
		VetLicense:                   "VET-123",
	}
	if _, err := svc.IssueHemostasisClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(c *domain.HemoabdomenHemostasisClearance){
		func(c *domain.HemoabdomenHemostasisClearance) { c.FinalMAPMmHg = 60 },
		func(c *domain.HemoabdomenHemostasisClearance) { c.FinalMAPMmHg = 120 },
		func(c *domain.HemoabdomenHemostasisClearance) { c.FinalPeripheralPCVPercent = 20.0 },
		func(c *domain.HemoabdomenHemostasisClearance) { c.FinalPlasmaLactateMmolL = 3.2 },
		func(c *domain.HemoabdomenHemostasisClearance) { c.SurgicalHemostasisConfirmed = false },
		func(c *domain.HemoabdomenHemostasisClearance) { c.RepeatAfastFreeFluidResolved = false },
		func(c *domain.HemoabdomenHemostasisClearance) { c.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		c := validClearance
		mutate(&c)
		if _, err := svc.IssueHemostasisClearance("HA-OP-101", c); err == nil {
			t.Errorf("expected IssueHemostasisClearance validation error for %+v", c)
		}
	}

	// Domain operation validation branches
	op, _ := svc.GetOperation("HA-OP-101")
	opBadID := *op
	opBadID.ID = ""
	if err := opBadID.Validate(); err == nil {
		t.Errorf("expected error for empty op ID")
	}
	opBadInc := *op
	opBadInc.IncidentID = ""
	if err := opBadInc.Validate(); err == nil {
		t.Errorf("expected error for empty op IncidentID")
	}
	opBadPet := *op
	opBadPet.PetName = ""
	if err := opBadPet.Validate(); err == nil {
		t.Errorf("expected error for empty op PetName")
	}
	opBadStatus := *op
	opBadStatus.Status = "INVALID"
	if err := opBadStatus.Validate(); err == nil {
		t.Errorf("expected error for invalid op Status")
	}
	opBadProfile := *op
	opBadProfile.PatientProfile.WeightKg = -1
	if err := opBadProfile.Validate(); err == nil {
		t.Errorf("expected error for invalid op PatientProfile")
	}
}
