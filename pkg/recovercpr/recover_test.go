package recovercpr_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/recovercpr"
)

func TestRecoverCPRSeededOperationsAndMetrics(t *testing.T) {
	svc := recovercpr.NewRecoverService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("CPR-OP-101")
	if err != nil {
		t.Fatalf("expected CPR-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Duke" || op101.IncidentID != "INC-CPR-9941" || op101.Status != domain.CPRStatusArrestInProgressCritical {
		t.Errorf("unexpected CPR-OP-101 metadata: %+v", op101)
	}
	if !op101.HyperventilationHighDoseEpiAlert || op101.PatientProfile.WeightKg != 30.0 || op101.PatientProfile.EndTidalCO2MmHg != 11.0 || op101.PatientProfile.CompressionCyclesCompleted != 1 || !op101.PatientProfile.ReversibleOpioidAlpha2Present {
		t.Errorf("unexpected CPR-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.ALSProtocolPlan == nil || !op101.ALSProtocolPlan.ZeroHyperventilationEveryOtherCycleEpiGuard {
		t.Fatalf("unexpected CPR-OP-101 plan: %+v", op101.ALSProtocolPlan)
	}
	if op101.ALSProtocolPlan.HandPlacementTechnique != "Directly Over Heart (4th-6th ICS) Lateral Recumbency" ||
		op101.ALSProtocolPlan.TargetCompressionRateBpm != 110 ||
		op101.ALSProtocolPlan.TargetVentilationRateBpm != 10 ||
		op101.ALSProtocolPlan.LowDoseEpinephrine001MgKgMg != 0.30 ||
		op101.ALSProtocolPlan.Atropine004MgKgDoseMg != 1.20 ||
		op101.ALSProtocolPlan.BiphasicDefibJoules != 60.0 ||
		op101.ALSProtocolPlan.Amiodarone5MgKgDoseMg != 150.0 ||
		!op101.ALSProtocolPlan.NaloxoneAtipamezoleReversalIndicated {
		t.Errorf("unexpected CPR-OP-101 plan values: %+v", op101.ALSProtocolPlan)
	}
	lowerAlert := strings.ToLower(op101.ALSProtocolPlan.RecoverPitfallAlert)
	if !strings.Contains(lowerAlert, "10 breaths/min") || !strings.Contains(lowerAlert, "every other") || !strings.Contains(lowerAlert, "0.1 mg/kg") {
		t.Errorf("expected pitfall alert to warn against >10 breaths/min hyperventilation, every-other-cycle epinephrine, and 0.1 mg/kg high-dose epi, got %q", op101.ALSProtocolPlan.RecoverPitfallAlert)
	}
	if len(op101.CycleChecks) != 1 {
		t.Errorf("expected 1 initial cycle check on CPR-OP-101, got %d", len(op101.CycleChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected CPR-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("CPR-OP-102")
	if err != nil {
		t.Fatalf("expected CPR-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Gizmo" || op102.IncidentID != "INC-CPR-9942" || op102.Status != domain.CPRStatusEpiDefibCycleStaged {
		t.Errorf("unexpected CPR-OP-102 metadata: %+v", op102)
	}
	if op102.ALSProtocolPlan == nil || !strings.Contains(op102.ALSProtocolPlan.HandPlacementTechnique, "Sternal Supine") || op102.ALSProtocolPlan.BiphasicDefibJoules != 0.0 {
		t.Errorf("expected barrel-chest Sternal Supine and 0 J defib on non-shockable CPR-OP-102, got %+v", op102.ALSProtocolPlan)
	}

	op103, err := svc.GetOperation("CPR-OP-103")
	if err != nil {
		t.Fatalf("expected CPR-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Pixel" || op103.IncidentID != "INC-CPR-9943" || op103.Status != domain.CPRStatusROSCPostArrestCleared {
		t.Errorf("unexpected CPR-OP-103 metadata: %+v", op103)
	}
	if op103.HyperventilationHighDoseEpiAlert || op103.ROSCClearance == nil || !strings.Contains(op103.ALSProtocolPlan.HandPlacementTechnique, "One-Hand Circumferential Cardiac") {
		t.Errorf("expected CPR-OP-103 to have ROSCClearance, false alert, and feline technique, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.CPRStatusArrestInProgressCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "CPR-OP-101" {
		t.Errorf("expected 1 critical op CPR-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.ArrestInProgressCriticalCount != 1 || metrics.EpiDefibCycleStagedCount != 1 || metrics.ROSCPostArrestClearedCount != 1 || metrics.HyperventilationHighDoseAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}

	if tech := recovercpr.ComputeHandPlacementTechnique("UNKNOWN"); tech != "Directly Over Heart (4th-6th ICS) Lateral Recumbency" {
		t.Errorf("unexpected fallback hand placement technique: %q", tech)
	}
}

func TestRecoverCPRCreateStageCycleCheckAndROSCClearanceLifecycle(t *testing.T) {
	svc := recovercpr.NewRecoverService()

	profileRound := domain.CPRPatientProfile{
		Species:                       domain.CPRSpeciesCanineRoundChestCardiac,
		Breed:                         "Golden Retriever",
		WeightKg:                      28.0,
		ArrestRhythm:                  domain.CPRRhythmVFibPulselessVTShockable,
		EndTidalCO2MmHg:               12.5,
		CompressionCyclesCompleted:    1,
		ReversibleOpioidAlpha2Present: true,
	}

	created, err := svc.CreateOperation("INC-CPR-9944", "Buster", profileRound)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "CPR-OP-104" || created.Status != domain.CPRStatusArrestInProgressCritical || !created.HyperventilationHighDoseEpiAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if !strings.Contains(created.ALSProtocolPlan.HandPlacementTechnique, "Widest Part of Thorax") ||
		created.ALSProtocolPlan.LowDoseEpinephrine001MgKgMg != 0.28 ||
		created.ALSProtocolPlan.Atropine004MgKgDoseMg != 1.12 ||
		created.ALSProtocolPlan.BiphasicDefibJoules != 56.0 ||
		created.ALSProtocolPlan.Amiodarone5MgKgDoseMg != 140.0 {
		t.Errorf("unexpected ALS plan on cycle 1 shockable 28 kg dog: %+v", created.ALSProtocolPlan)
	}

	// Stage plan at cycle 2 (escalates BiphasicDefibJoules to 4.0 J/kg = 112.0 J)
	stagedProfile := profileRound
	stagedProfile.Breed = ""
	stagedProfile.CompressionCyclesCompleted = 2
	stagedProfile.EndTidalCO2MmHg = 19.5

	staged, err := svc.StageALSProtocolPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageALSProtocolPlan error: %v", err)
	}
	if staged.Status != domain.CPRStatusEpiDefibCycleStaged {
		t.Errorf("expected status %s, got %s", domain.CPRStatusEpiDefibCycleStaged, staged.Status)
	}
	if staged.ALSProtocolPlan.BiphasicDefibJoules != 112.0 {
		t.Errorf("expected 4.0 J/kg (112.0 J) at cycle >= 2, got %v", staged.ALSProtocolPlan.BiphasicDefibJoules)
	}
	if staged.PatientProfile.Breed != "Golden Retriever" {
		t.Errorf("expected breed preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record 2-min cycle check with ROSC EtCO2 surge (>= 35 mmHg)
	checked, err := svc.RecordCycleCheck(created.ID, domain.CPRTwoMinuteCycleCheck{
		CycleNumber:          3,
		EndTidalCO2MmHg:      39.0,
		RhythmAt2MinPause:    domain.CPRRhythmAsystolePEANonShockable,
		CompressorRotated:    true,
		PauseDurationSeconds: 7,
		ClinicianCallsign:    "DR-RECOVER-22",
		Notes:                "EtCO2 surged to 39.0 mmHg after biphasic defibrillation and Amiodarone 140 mg IV.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordCycleCheck error: %v", err)
	}
	if len(checked.CycleChecks) != 1 || checked.HyperventilationHighDoseEpiAlert || checked.PatientProfile.CompressionCyclesCompleted != 3 {
		t.Errorf("expected 1 check, false alert on EtCO2 >= 35, and 3 cycles completed, got %+v", checked)
	}

	// Issue ROSC discharge clearance
	cleared, err := svc.IssueROSCClearance(created.ID, domain.ROSCDischargeClearance{
		SustainedROSCMinutes:          25,
		FinalEtCO2MmHg:                39.0,
		FinalMAPMmHg:                  92,
		SpO2Titrated94To98NoHyperoxia: true,
		PermissiveHypothermiaOrNormothermiaMaintained: true,
		VetLicense: "VET-RECOVER-88120",
	})
	if err != nil {
		t.Fatalf("unexpected IssueROSCClearance error: %v", err)
	}
	if cleared.Status != domain.CPRStatusROSCPostArrestCleared || cleared.HyperventilationHighDoseEpiAlert || cleared.ROSCClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.ROSCClearance.Summary == "" {
		t.Errorf("expected auto-populated ROSC clearance summary when empty")
	}

	// Re-arrest check with EtCO2 < 15.0 mmHg reverts to critical
	rearrest, err := svc.RecordCycleCheck(created.ID, domain.CPRTwoMinuteCycleCheck{
		CycleNumber:          4,
		EndTidalCO2MmHg:      10.5,
		RhythmAt2MinPause:    domain.CPRRhythmVFibPulselessVTShockable,
		CompressorRotated:    true,
		PauseDurationSeconds: 6,
		ClinicianCallsign:    "DR-RECOVER-22",
		Notes:                "Recurrent VFib with EtCO2 drop to 10.5 mmHg; resumed chest compressions.",
	})
	if err != nil {
		t.Fatalf("unexpected re-arrest RecordCycleCheck error: %v", err)
	}
	if rearrest.Status != domain.CPRStatusArrestInProgressCritical || !rearrest.HyperventilationHighDoseEpiAlert || rearrest.ROSCClearance != nil {
		t.Errorf("expected re-arrest to revert to CARDIOPULMONARY_ARREST_BLS_ALS_CRITICAL, got %+v", rearrest)
	}
}

func TestRecoverCPRValidationErrors(t *testing.T) {
	svc := recovercpr.NewRecoverService()

	validProfile := domain.CPRPatientProfile{
		Species:                       domain.CPRSpeciesCanineKeelChestThoracic,
		WeightKg:                      25.0,
		ArrestRhythm:                  domain.CPRRhythmVFibPulselessVTShockable,
		EndTidalCO2MmHg:               14.0,
		CompressionCyclesCompleted:    1,
		ReversibleOpioidAlpha2Present: false,
	}

	if _, err := svc.CreateOperation("", "Buster", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.CPRPatientProfile)
	}{
		{"invalid species", func(p *domain.CPRPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.CPRPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.CPRPatientProfile) { p.WeightKg = 140 }},
		{"invalid rhythm", func(p *domain.CPRPatientProfile) { p.ArrestRhythm = "TORSADES" }},
		{"etco2 < 0", func(p *domain.CPRPatientProfile) { p.EndTidalCO2MmHg = -1.0 }},
		{"etco2 > 100", func(p *domain.CPRPatientProfile) { p.EndTidalCO2MmHg = 110.0 }},
		{"cycles < 0", func(p *domain.CPRPatientProfile) { p.CompressionCyclesCompleted = -1 }},
		{"cycles > 30", func(p *domain.CPRPatientProfile) { p.CompressionCyclesCompleted = 35 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Buster", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageALSProtocolPlan("CPR-OP-101", p); err == nil {
				t.Errorf("expected StageALSProtocolPlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageALSProtocolPlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging plan on unknown op")
	}

	validCheck := domain.CPRTwoMinuteCycleCheck{
		CycleNumber:          2,
		EndTidalCO2MmHg:      21.0,
		RhythmAt2MinPause:    domain.CPRRhythmVFibPulselessVTShockable,
		CompressorRotated:    true,
		PauseDurationSeconds: 6,
		ClinicianCallsign:    "DR-1",
	}
	if _, err := svc.RecordCycleCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording cycle check on unknown op")
	}

	badChecks := []func(c *domain.CPRTwoMinuteCycleCheck){
		func(c *domain.CPRTwoMinuteCycleCheck) { c.CycleNumber = 0 },
		func(c *domain.CPRTwoMinuteCycleCheck) { c.CycleNumber = 35 },
		func(c *domain.CPRTwoMinuteCycleCheck) { c.EndTidalCO2MmHg = -1.0 },
		func(c *domain.CPRTwoMinuteCycleCheck) { c.EndTidalCO2MmHg = 105.0 },
		func(c *domain.CPRTwoMinuteCycleCheck) { c.RhythmAt2MinPause = "" },
		func(c *domain.CPRTwoMinuteCycleCheck) { c.CompressorRotated = false },
		func(c *domain.CPRTwoMinuteCycleCheck) { c.PauseDurationSeconds = 0 },
		func(c *domain.CPRTwoMinuteCycleCheck) { c.PauseDurationSeconds = 11 },
		func(c *domain.CPRTwoMinuteCycleCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordCycleCheck("CPR-OP-101", c); err == nil {
			t.Errorf("expected RecordCycleCheck validation error for %+v", c)
		}
	}

	validClearance := domain.ROSCDischargeClearance{
		SustainedROSCMinutes:          25,
		FinalEtCO2MmHg:                38.0,
		FinalMAPMmHg:                  90,
		SpO2Titrated94To98NoHyperoxia: true,
		PermissiveHypothermiaOrNormothermiaMaintained: true,
		VetLicense: "VET-123",
	}
	if _, err := svc.IssueROSCClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.ROSCDischargeClearance){
		func(d *domain.ROSCDischargeClearance) { d.SustainedROSCMinutes = 15 },
		func(d *domain.ROSCDischargeClearance) { d.SustainedROSCMinutes = 2000 },
		func(d *domain.ROSCDischargeClearance) { d.FinalEtCO2MmHg = 30.0 },
		func(d *domain.ROSCDischargeClearance) { d.FinalEtCO2MmHg = 50.0 },
		func(d *domain.ROSCDischargeClearance) { d.FinalMAPMmHg = 70 },
		func(d *domain.ROSCDischargeClearance) { d.FinalMAPMmHg = 135 },
		func(d *domain.ROSCDischargeClearance) { d.SpO2Titrated94To98NoHyperoxia = false },
		func(d *domain.ROSCDischargeClearance) { d.PermissiveHypothermiaOrNormothermiaMaintained = false },
		func(d *domain.ROSCDischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueROSCClearance("CPR-OP-101", d); err == nil {
			t.Errorf("expected IssueROSCClearance validation error for %+v", d)
		}
	}

	op := domain.RecoverCPROperation{
		ID:             "CPR-OP-101",
		IncidentID:     "INC-CPR-9941",
		PetName:        "Duke",
		Status:         domain.CPRStatusArrestInProgressCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.RecoverCPROperation){
		func(o *domain.RecoverCPROperation) { o.ID = "" },
		func(o *domain.RecoverCPROperation) { o.IncidentID = "" },
		func(o *domain.RecoverCPROperation) { o.PetName = "" },
		func(o *domain.RecoverCPROperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.RecoverCPROperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected RecoverCPROperation.Validate error for %+v", cp)
		}
	}
}
