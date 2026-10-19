package imhathrombosis_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/imhathrombosis"
)

func TestIMHAThrombosisSeededOperationsAndMetrics(t *testing.T) {
	svc := imhathrombosis.NewIMHAService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("IM-OP-101")
	if err != nil {
		t.Fatalf("expected IM-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Penny" || op101.IncidentID != "INC-IM-9701" || op101.Status != domain.IMHAStatusHemolyticCrisisCritical {
		t.Errorf("unexpected IM-OP-101 metadata: %+v", op101)
	}
	if !op101.PulmonaryThromboembolismAlert || op101.PatientProfile.WeightKg != 11.5 || op101.PatientProfile.PackedCellVolumePercent != 12.0 || op101.PatientProfile.TotalBilirubinMgDl != 6.4 || op101.PatientProfile.PlateletCountKUl != 85.0 || op101.PatientProfile.SpherocytesPerHPF != 18 || !op101.PatientProfile.SalineAgglutination4To1Positive || !op101.PatientProfile.IntravascularHemoglobinuria {
		t.Errorf("unexpected IM-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.ImmunosuppressiveRegimen == nil || !op101.ImmunosuppressiveRegimen.ZeroJugularVenipunctureGuard {
		t.Errorf("unexpected IM-OP-101 regimen: %+v", op101.ImmunosuppressiveRegimen)
	}
	if op101.ImmunosuppressiveRegimen.Prednisolone2MgKgDayMg != 23.0 || op101.ImmunosuppressiveRegimen.Mycophenolate10MgKgBIDMg != 115.0 || op101.ImmunosuppressiveRegimen.Clopidogrel2MgKgDayMg != 23.0 || op101.ImmunosuppressiveRegimen.Rivaroxaban1MgKgDayMg != 11.5 || op101.ImmunosuppressiveRegimen.PackedRBCTransfusionMl != 172.5 {
		t.Errorf("unexpected IM-OP-101 doses: %+v", op101.ImmunosuppressiveRegimen)
	}
	lowerAlert := strings.ToLower(op101.ImmunosuppressiveRegimen.IMHAPitfallAlert)
	if !strings.Contains(lowerAlert, "thromboprophylaxis") || !strings.Contains(lowerAlert, "jugular") || !strings.Contains(lowerAlert, "1:4") {
		t.Errorf("expected pitfall alert to warn against omitting thromboprophylaxis, jugular venipuncture, and 1:4 saline wash, got %q", op101.ImmunosuppressiveRegimen.IMHAPitfallAlert)
	}
	if len(op101.HemolysisChecks) != 1 {
		t.Errorf("expected 1 initial hemolysis check on IM-OP-101, got %d", len(op101.HemolysisChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected IM-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("IM-OP-102")
	if err != nil {
		t.Fatalf("expected IM-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Scout" || op102.IncidentID != "INC-IM-9702" || op102.Status != domain.IMHAStatusImmunosuppressantAnticoagStaged {
		t.Errorf("unexpected IM-OP-102 metadata: %+v", op102)
	}
	if op102.ImmunosuppressiveRegimen == nil || op102.ImmunosuppressiveRegimen.Clopidogrel2MgKgDayMg != 0.0 || op102.ImmunosuppressiveRegimen.Vincristine002MgKgIMTPMg != 0.36 {
		t.Errorf("expected held anticoagulants and Vincristine 0.36 mg on IM-OP-102 (platelets 14 K/uL), got %+v", op102.ImmunosuppressiveRegimen)
	}

	op103, err := svc.GetOperation("IM-OP-103")
	if err != nil {
		t.Fatalf("expected IM-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Maple" || op103.IncidentID != "INC-IM-9703" || op103.Status != domain.IMHAStatusPCVStableReticulocytosisCleared {
		t.Errorf("unexpected IM-OP-103 metadata: %+v", op103)
	}
	if op103.PulmonaryThromboembolismAlert || op103.DischargeClearance == nil {
		t.Errorf("expected IM-OP-103 to have DischargeClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.IMHAStatusHemolyticCrisisCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "IM-OP-101" {
		t.Errorf("expected 1 critical op IM-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.CriticalHemolyticCount != 1 || metrics.StagedRegimenCount != 1 || metrics.ClearedStableCount != 1 || metrics.PTEAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestIMHAThrombosisCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := imhathrombosis.NewIMHAService()

	profile := domain.IMHAPatientProfile{
		Species:                         domain.IMHASpeciesFelineMycoplasmaSecondaryIMHA,
		Breed:                           "Domestic Shorthair",
		WeightKg:                        4.8,
		PackedCellVolumePercent:         14.0,
		TotalBilirubinMgDl:              4.2,
		PlateletCountKUl:                90.0,
		SpherocytesPerHPF:               4,
		SalineAgglutination4To1Positive: true,
		IntravascularHemoglobinuria:     false,
	}

	created, err := svc.CreateOperation("INC-IM-9704", "Poppy", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "IM-OP-104" || created.Status != domain.IMHAStatusHemolyticCrisisCritical || !created.PulmonaryThromboembolismAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if created.ImmunosuppressiveRegimen.Prednisolone2MgKgDayMg != 9.6 || created.ImmunosuppressiveRegimen.PackedRBCTransfusionMl != 72.0 {
		t.Errorf("expected 9.6 mg Prednisolone and 72.0 mL pRBC for 4.8 kg feline at PCV 14%%, got %+v", created.ImmunosuppressiveRegimen)
	}
	if !strings.Contains(created.ImmunosuppressiveRegimen.ClinicalRationale, "Doxycycline") {
		t.Errorf("expected feline secondary IMHA rationale to mention Doxycycline, got %q", created.ImmunosuppressiveRegimen.ClinicalRationale)
	}

	// Stage regimen on IM-OP-104 after initial pRBC transfusion
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.PackedCellVolumePercent = 22.0
	stagedProfile.TotalBilirubinMgDl = 2.1

	staged, err := svc.StageImmunosuppressiveRegimen(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageImmunosuppressiveRegimen error: %v", err)
	}
	if staged.Status != domain.IMHAStatusImmunosuppressantAnticoagStaged {
		t.Errorf("expected status %s, got %s", domain.IMHAStatusImmunosuppressantAnticoagStaged, staged.Status)
	}
	if staged.ImmunosuppressiveRegimen.PackedRBCTransfusionMl != 0.0 {
		t.Errorf("expected 0.0 mL pRBC when PCV 22%% >= 18%%, got %+v", staged.ImmunosuppressiveRegimen)
	}
	if staged.PatientProfile.Breed != "Domestic Shorthair" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record hemolysis check with resolving bilirubin, strong reticulocytosis, negative saline agglutination
	checked, err := svc.RecordHemolysisCheck(created.ID, domain.IMHAHemolysisSerialCheck{
		PackedCellVolumePercent:       27.0,
		ReticulocyteCountKUl:          112.0,
		TotalBilirubinMgDl:            0.9,
		SalineAgglutinationPersistent: false,
		RestingRespiratoryRateBpm:     24,
		ClinicianCallsign:             "DR-HEME-44",
		Notes:                         "PCV climbed to 27.0%, reticulocytes 112 K/uL, 1:4 saline wash negative, eupneic.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordHemolysisCheck error: %v", err)
	}
	if len(checked.HemolysisChecks) != 1 {
		t.Errorf("expected 1 hemolysis check, got %d", len(checked.HemolysisChecks))
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.IMHADischargeClearance{
		FinalPCVStable48HrPercent:         29.0,
		FinalReticulocyteCountKUl:         135.0,
		SalineAgglutinationResolved:       true,
		NoPulmonaryThromboembolismDyspnea: true,
		ThromboprophylaxisDispensed:       true,
		VetLicense:                        "VET-WA-88104",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.IMHAStatusPCVStableReticulocytosisCleared || cleared.PulmonaryThromboembolismAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse check with severe anemia & tachypnea reverts to critical
	relapsed, err := svc.RecordHemolysisCheck(created.ID, domain.IMHAHemolysisSerialCheck{
		PackedCellVolumePercent:       13.0,
		ReticulocyteCountKUl:          30.0,
		TotalBilirubinMgDl:            5.5,
		SalineAgglutinationPersistent: true,
		RestingRespiratoryRateBpm:     54,
		ClinicianCallsign:             "DR-HEME-44",
		Notes:                         "Acute hemolytic relapse with PCV 13% and tachypnea 54 bpm.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordHemolysisCheck error: %v", err)
	}
	if relapsed.Status != domain.IMHAStatusHemolyticCrisisCritical || !relapsed.PulmonaryThromboembolismAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to HEMOLYTIC_AGGLUTINATION_CRISIS_CRITICAL, got %+v", relapsed)
	}
}

func TestIMHAThrombosisValidationErrors(t *testing.T) {
	svc := imhathrombosis.NewIMHAService()

	validProfile := domain.IMHAPatientProfile{
		Species:                         domain.IMHASpeciesCaninePrimaryIMHA,
		WeightKg:                        11.5,
		PackedCellVolumePercent:         14.0,
		TotalBilirubinMgDl:              4.5,
		PlateletCountKUl:                95.0,
		SpherocytesPerHPF:               12,
		SalineAgglutination4To1Positive: true,
		IntravascularHemoglobinuria:     true,
	}

	if _, err := svc.CreateOperation("", "Penny", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.IMHAPatientProfile)
	}{
		{"invalid species", func(p *domain.IMHAPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.IMHAPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.IMHAPatientProfile) { p.WeightKg = 140 }},
		{"pcv too low", func(p *domain.IMHAPatientProfile) { p.PackedCellVolumePercent = 2.0 }},
		{"pcv too high", func(p *domain.IMHAPatientProfile) { p.PackedCellVolumePercent = 75.0 }},
		{"bili negative", func(p *domain.IMHAPatientProfile) { p.TotalBilirubinMgDl = -1.0 }},
		{"bili too high", func(p *domain.IMHAPatientProfile) { p.TotalBilirubinMgDl = 60.0 }},
		{"platelets negative", func(p *domain.IMHAPatientProfile) { p.PlateletCountKUl = -5.0 }},
		{"platelets too high", func(p *domain.IMHAPatientProfile) { p.PlateletCountKUl = 1200.0 }},
		{"spherocytes negative", func(p *domain.IMHAPatientProfile) { p.SpherocytesPerHPF = -1 }},
		{"spherocytes > 100", func(p *domain.IMHAPatientProfile) { p.SpherocytesPerHPF = 120 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Penny", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageImmunosuppressiveRegimen("IM-OP-101", p); err == nil {
				t.Errorf("expected StageImmunosuppressiveRegimen error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageImmunosuppressiveRegimen("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging regimen on unknown op")
	}

	validCheck := domain.IMHAHemolysisSerialCheck{
		PackedCellVolumePercent:       22.0,
		ReticulocyteCountKUl:          95.0,
		TotalBilirubinMgDl:            1.8,
		SalineAgglutinationPersistent: false,
		RestingRespiratoryRateBpm:     24,
		ClinicianCallsign:             "DR-1",
	}
	if _, err := svc.RecordHemolysisCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording hemolysis check on unknown op")
	}

	badChecks := []func(c *domain.IMHAHemolysisSerialCheck){
		func(c *domain.IMHAHemolysisSerialCheck) { c.PackedCellVolumePercent = 2.0 },
		func(c *domain.IMHAHemolysisSerialCheck) { c.ReticulocyteCountKUl = -10.0 },
		func(c *domain.IMHAHemolysisSerialCheck) { c.TotalBilirubinMgDl = -0.5 },
		func(c *domain.IMHAHemolysisSerialCheck) { c.RestingRespiratoryRateBpm = 2 },
		func(c *domain.IMHAHemolysisSerialCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordHemolysisCheck("IM-OP-101", c); err == nil {
			t.Errorf("expected RecordHemolysisCheck validation error for %+v", c)
		}
	}

	validClearance := domain.IMHADischargeClearance{
		FinalPCVStable48HrPercent:         28.0,
		FinalReticulocyteCountKUl:         115.0,
		SalineAgglutinationResolved:       true,
		NoPulmonaryThromboembolismDyspnea: true,
		ThromboprophylaxisDispensed:       true,
		VetLicense:                        "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.IMHADischargeClearance){
		func(d *domain.IMHADischargeClearance) { d.FinalPCVStable48HrPercent = 20.0 },
		func(d *domain.IMHADischargeClearance) { d.FinalPCVStable48HrPercent = 70.0 },
		func(d *domain.IMHADischargeClearance) { d.FinalReticulocyteCountKUl = 45.0 },
		func(d *domain.IMHADischargeClearance) { d.FinalReticulocyteCountKUl = 1200.0 },
		func(d *domain.IMHADischargeClearance) { d.SalineAgglutinationResolved = false },
		func(d *domain.IMHADischargeClearance) { d.NoPulmonaryThromboembolismDyspnea = false },
		func(d *domain.IMHADischargeClearance) { d.ThromboprophylaxisDispensed = false },
		func(d *domain.IMHADischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("IM-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}

	// Validate IMHAThrombosisOperation error paths
	op := domain.IMHAThrombosisOperation{
		ID:             "IM-OP-101",
		IncidentID:     "INC-IM-9701",
		PetName:        "Penny",
		Status:         domain.IMHAStatusHemolyticCrisisCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.IMHAThrombosisOperation){
		func(o *domain.IMHAThrombosisOperation) { o.ID = "" },
		func(o *domain.IMHAThrombosisOperation) { o.IncidentID = "" },
		func(o *domain.IMHAThrombosisOperation) { o.PetName = "" },
		func(o *domain.IMHAThrombosisOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.IMHAThrombosisOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected IMHAThrombosisOperation.Validate error for %+v", cp)
		}
	}
}
