package ophthalmicglobe_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ophthalmicglobe"
)

func TestOphthalmicGlobeSeededOperationsAndMetrics(t *testing.T) {
	svc := ophthalmicglobe.NewOphthalmicService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("OG-OP-101")
	if err != nil {
		t.Fatalf("expected OG-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Buster" || op101.IncidentID != "INC-OG-9951" || op101.Status != domain.OphthalmicStatusVisionThreateningCritical {
		t.Errorf("unexpected OG-OP-101 metadata: %+v", op101)
	}
	if !op101.LatanoprostLuxationSteroidAlert || op101.PatientProfile.WeightKg != 12.5 || op101.PatientProfile.IntraocularPressureMmHg != 64 || !op101.PatientProfile.AnteriorLensLuxationPresent {
		t.Errorf("unexpected OG-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.PreservationPlan == nil || !op101.PreservationPlan.ZeroTopicalSteroidOintmentNeckLeadGuard {
		t.Errorf("unexpected OG-OP-101 plan: %+v", op101.PreservationPlan)
	}
	if op101.PreservationPlan.Latanoprost0005PctAllowed || op101.PreservationPlan.Mannitol1GKgIVGrams != 12.5 || op101.PreservationPlan.DorzolamideTimololDropsFreq != "q8h" {
		t.Errorf("expected Latanoprost false (lens luxation), Mannitol 12.5 g, Dorzolamide/Timolol q8h on OG-OP-101, got %+v", op101.PreservationPlan)
	}
	lowerAlert := strings.ToLower(op101.PreservationPlan.OphthalmicPitfallAlert)
	if !strings.Contains(lowerAlert, "latanoprost") || !strings.Contains(lowerAlert, "corticosteroids") || !strings.Contains(lowerAlert, "jugular") {
		t.Errorf("expected pitfall alert to warn against latanoprost in lens luxation, topical corticosteroids, and manual jugular compression, got %q", op101.PreservationPlan.OphthalmicPitfallAlert)
	}
	if len(op101.TonometryChecks) != 1 {
		t.Errorf("expected 1 initial tonometry check on OG-OP-101, got %d", len(op101.TonometryChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected OG-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("OG-OP-102")
	if err != nil {
		t.Fatalf("expected OG-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Pugsley" || op102.IncidentID != "INC-OG-9952" || op102.Status != domain.OphthalmicStatusLatanoprostTarsorrhaphyStaged {
		t.Errorf("unexpected OG-OP-102 metadata: %+v", op102)
	}
	if op102.PreservationPlan == nil || !op102.PreservationPlan.TemporaryTarsorrhaphyIndicated || !op102.PreservationPlan.AutologousSerumDropsQ1H {
		t.Errorf("expected TemporaryTarsorrhaphyIndicated and AutologousSerumDropsQ1H on OG-OP-102, got %+v", op102.PreservationPlan)
	}

	op103, err := svc.GetOperation("OG-OP-103")
	if err != nil {
		t.Fatalf("expected OG-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Misty" || op103.IncidentID != "INC-OG-9953" || op103.Status != domain.OphthalmicStatusIOPNormalCorneaProtected {
		t.Errorf("unexpected OG-OP-103 metadata: %+v", op103)
	}
	if op103.LatanoprostLuxationSteroidAlert || op103.DischargeClearance == nil {
		t.Errorf("expected OG-OP-103 to have DischargeClearance and false alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.OphthalmicStatusVisionThreateningCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "OG-OP-101" {
		t.Errorf("expected 1 critical op OG-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.VisionThreateningCriticalCount != 1 || metrics.LatanoprostTarsorrhaphyStagedCount != 1 || metrics.IOPNormalizedClearedCount != 1 || metrics.LatanoprostSteroidAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestOphthalmicGlobeCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := ophthalmicglobe.NewOphthalmicService()

	// Create primary congestive glaucoma without lens luxation (Latanoprost allowed!)
	profilePrimary := domain.OphthalmicPatientProfile{
		Species:                        domain.OphthalmicSpeciesCaninePrimaryCongestiveGlaucoma,
		Breed:                          "Basset Hound",
		WeightKg:                       22.0,
		IntraocularPressureMmHg:        58,
		FluoresceinStainPattern:        domain.OphthalmicFluoresceinNegativeIntact,
		AnteriorLensLuxationPresent:    false,
		ExtraocularMusclesAvulsedCount: 0,
		ConsensualPLRPresent:           true,
	}

	created, err := svc.CreateOperation("INC-OG-9954", "Gizmo", profilePrimary)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "OG-OP-104" || created.Status != domain.OphthalmicStatusVisionThreateningCritical || !created.LatanoprostLuxationSteroidAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if !created.PreservationPlan.Latanoprost0005PctAllowed || created.PreservationPlan.Mannitol1GKgIVGrams != 22.0 || created.PreservationPlan.DorzolamideTimololDropsFreq != "q8h" {
		t.Errorf("expected Latanoprost true, Mannitol 22.0 g, Dorzolamide q8h, got %+v", created.PreservationPlan)
	}

	// Stage plan on OG-OP-104 with normotensive IOP & intact cornea
	stagedProfile := profilePrimary
	stagedProfile.Breed = ""
	stagedProfile.IntraocularPressureMmHg = 20

	staged, err := svc.StagePreservationPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StagePreservationPlan error: %v", err)
	}
	if staged.Status != domain.OphthalmicStatusLatanoprostTarsorrhaphyStaged {
		t.Errorf("expected status %s, got %s", domain.OphthalmicStatusLatanoprostTarsorrhaphyStaged, staged.Status)
	}
	if staged.PreservationPlan.Latanoprost0005PctAllowed || staged.PreservationPlan.Mannitol1GKgIVGrams != 0.0 {
		t.Errorf("expected Latanoprost false and Mannitol 0.0 g when IOP 20, got %+v", staged.PreservationPlan)
	}
	if staged.PatientProfile.Breed != "Basset Hound" {
		t.Errorf("expected breed preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record tonometry check with normalized IOP, negative Seidel, dazzle+, and rigid E-collar
	checked, err := svc.RecordTonometryCheck(created.ID, domain.OphthalmicTonometryCheck{
		IntraocularPressureMmHg: 18,
		SeidelTestNegative:      true,
		DazzleReflexPresent:     true,
		RigidECollarVerified:    true,
		ClinicianCallsign:       "DR-OPHTHO-21",
		Notes:                   "IOP normalized to 18 mmHg; Seidel test negative; dazzle reflex intact; rigid E-collar verified.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordTonometryCheck error: %v", err)
	}
	if len(checked.TonometryChecks) != 1 || checked.LatanoprostLuxationSteroidAlert {
		t.Errorf("expected 1 tonometry check and false alert when IOP 18 and Seidel negative, got %+v", checked)
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.OphthalmicDischargeClearance{
		FinalIOPMmHg:                    17,
		GlobeTectonicIntegrityConfirmed: true,
		HardPlasticECollarFitted:        true,
		TopicalDrop5MinSpacingEducated:  true,
		VetLicense:                      "VET-OPHTHO-88120",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.OphthalmicStatusIOPNormalCorneaProtected || cleared.LatanoprostLuxationSteroidAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse check with Seidel positive leak & IOP spike > 40 reverts to critical
	relapsed, err := svc.RecordTonometryCheck(created.ID, domain.OphthalmicTonometryCheck{
		IntraocularPressureMmHg: 48,
		SeidelTestNegative:      false,
		DazzleReflexPresent:     true,
		RigidECollarVerified:    false,
		ClinicianCallsign:       "DR-OPHTHO-21",
		Notes:                   "IOP spike to 48 mmHg and Seidel positive aqueous leak after E-collar removal.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordTonometryCheck error: %v", err)
	}
	if relapsed.Status != domain.OphthalmicStatusVisionThreateningCritical || !relapsed.LatanoprostLuxationSteroidAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to VISION_THREATENING_IOP_DESCEMETOCELE_CRITICAL, got %+v", relapsed)
	}
	if relapsed.PatientProfile.FluoresceinStainPattern != domain.OphthalmicFluoresceinSeidelPositiveLeak {
		t.Errorf("expected Seidel positive stain pattern on relapse, got %q", relapsed.PatientProfile.FluoresceinStainPattern)
	}

	// Re-clearing resets Seidel positive leak to FLUORESCEIN_NEGATIVE_INTACT
	clearedAgain, err := svc.IssueDischargeClearance(created.ID, domain.OphthalmicDischargeClearance{
		FinalIOPMmHg:                    16,
		GlobeTectonicIntegrityConfirmed: true,
		HardPlasticECollarFitted:        true,
		TopicalDrop5MinSpacingEducated:  true,
		VetLicense:                      "VET-OPHTHO-88120",
		Summary:                         "Cleared after conjunctival graft repair.",
	})
	if err != nil {
		t.Fatalf("unexpected second IssueDischargeClearance error: %v", err)
	}
	if clearedAgain.PatientProfile.FluoresceinStainPattern != domain.OphthalmicFluoresceinNegativeIntact {
		t.Errorf("expected FluoresceinStainPattern reset to FLUORESCEIN_NEGATIVE_INTACT on clearance, got %q", clearedAgain.PatientProfile.FluoresceinStainPattern)
	}
}

func TestOphthalmicGlobeValidationErrors(t *testing.T) {
	svc := ophthalmicglobe.NewOphthalmicService()

	validProfile := domain.OphthalmicPatientProfile{
		Species:                        domain.OphthalmicSpeciesCaninePrimaryCongestiveGlaucoma,
		WeightKg:                       10.0,
		IntraocularPressureMmHg:        50,
		FluoresceinStainPattern:        domain.OphthalmicFluoresceinDonutRingDescemetocele,
		AnteriorLensLuxationPresent:    false,
		ExtraocularMusclesAvulsedCount: 0,
		ConsensualPLRPresent:           true,
	}

	if _, err := svc.CreateOperation("", "Gizmo", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.OphthalmicPatientProfile)
	}{
		{"invalid species", func(p *domain.OphthalmicPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.OphthalmicPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.OphthalmicPatientProfile) { p.WeightKg = 140 }},
		{"iop too low", func(p *domain.OphthalmicPatientProfile) { p.IntraocularPressureMmHg = 1 }},
		{"iop too high", func(p *domain.OphthalmicPatientProfile) { p.IntraocularPressureMmHg = 150 }},
		{"invalid fluorescein pattern", func(p *domain.OphthalmicPatientProfile) { p.FluoresceinStainPattern = "UNKNOWN_STAIN" }},
		{"muscles avulsed < 0", func(p *domain.OphthalmicPatientProfile) { p.ExtraocularMusclesAvulsedCount = -1 }},
		{"muscles avulsed > 6", func(p *domain.OphthalmicPatientProfile) { p.ExtraocularMusclesAvulsedCount = 7 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Gizmo", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StagePreservationPlan("OG-OP-101", p); err == nil {
				t.Errorf("expected StagePreservationPlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StagePreservationPlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging plan on unknown op")
	}

	validCheck := domain.OphthalmicTonometryCheck{
		IntraocularPressureMmHg: 18,
		SeidelTestNegative:      true,
		DazzleReflexPresent:     true,
		RigidECollarVerified:    true,
		ClinicianCallsign:       "DR-OPHTHO-1",
	}
	if _, err := svc.RecordTonometryCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording tonometry check on unknown op")
	}

	badChecks := []func(c *domain.OphthalmicTonometryCheck){
		func(c *domain.OphthalmicTonometryCheck) { c.IntraocularPressureMmHg = 0 },
		func(c *domain.OphthalmicTonometryCheck) { c.IntraocularPressureMmHg = 140 },
		func(c *domain.OphthalmicTonometryCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordTonometryCheck("OG-OP-101", c); err == nil {
			t.Errorf("expected RecordTonometryCheck validation error for %+v", c)
		}
	}

	validClearance := domain.OphthalmicDischargeClearance{
		FinalIOPMmHg:                    16,
		GlobeTectonicIntegrityConfirmed: true,
		HardPlasticECollarFitted:        true,
		TopicalDrop5MinSpacingEducated:  true,
		VetLicense:                      "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.OphthalmicDischargeClearance){
		func(d *domain.OphthalmicDischargeClearance) { d.FinalIOPMmHg = 8 },
		func(d *domain.OphthalmicDischargeClearance) { d.FinalIOPMmHg = 28 },
		func(d *domain.OphthalmicDischargeClearance) { d.GlobeTectonicIntegrityConfirmed = false },
		func(d *domain.OphthalmicDischargeClearance) { d.HardPlasticECollarFitted = false },
		func(d *domain.OphthalmicDischargeClearance) { d.TopicalDrop5MinSpacingEducated = false },
		func(d *domain.OphthalmicDischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("OG-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}

	// Validate OphthalmicGlobeOperation error paths
	op := domain.OphthalmicGlobeOperation{
		ID:             "OG-OP-101",
		IncidentID:     "INC-OG-9951",
		PetName:        "Buster",
		Status:         domain.OphthalmicStatusVisionThreateningCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.OphthalmicGlobeOperation){
		func(o *domain.OphthalmicGlobeOperation) { o.ID = "" },
		func(o *domain.OphthalmicGlobeOperation) { o.IncidentID = "" },
		func(o *domain.OphthalmicGlobeOperation) { o.PetName = "" },
		func(o *domain.OphthalmicGlobeOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.OphthalmicGlobeOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected OphthalmicGlobeOperation.Validate error for %+v", cp)
		}
	}
}
