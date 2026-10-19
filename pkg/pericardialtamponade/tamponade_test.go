package pericardialtamponade_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pericardialtamponade"
)

func TestPericardialTamponadeSeededOperationsAndMetrics(t *testing.T) {
	svc := pericardialtamponade.NewTamponadeService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("PT-OP-101")
	if err != nil {
		t.Fatalf("expected PT-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Brutus" || op101.IncidentID != "INC-PT-9921" || op101.Status != domain.TamponadeStatusRADiastolicCollapseCritical {
		t.Errorf("unexpected PT-OP-101 metadata: %+v", op101)
	}
	if !op101.FurosemideLeftCorpnaryHazardAlert || op101.PatientProfile.WeightKg != 33.0 || op101.PatientProfile.HeartRateBpm != 185 || op101.PatientProfile.MeanArterialPressureMmHg != 52 {
		t.Errorf("unexpected PT-OP-101 profile: %+v", op101.PatientProfile)
	}
	if !op101.PatientProfile.RightAtrialDiastolicCollapse || !op101.PatientProfile.ElectricalAlternansPresent || op101.PatientProfile.PulsusParadoxusMmHg != 18 || op101.PatientProfile.EffusionPCVPercent != 28.0 {
		t.Errorf("unexpected PT-OP-101 tamponade flags: %+v", op101.PatientProfile)
	}
	if op101.CentresisPlan == nil || op101.CentresisPlan.ApproachSideAndICS != "Right 4th-5th ICS Cardiac Notch" || !op101.CentresisPlan.ClotTubeIatrogenicPunctureCheck || !op101.CentresisPlan.ZeroFurosemideDiureticGuard {
		t.Errorf("unexpected PT-OP-101 plan: %+v", op101.CentresisPlan)
	}
	if op101.CentresisPlan.Lidocaine2PctInfiltrationMl != 3.3 || op101.CentresisPlan.LidocaineIV2MgKgArrhythmiaMg != 66.0 || op101.CentresisPlan.CrystalloidPreloadExpansionMl != 330.0 {
		t.Errorf("unexpected PT-OP-101 plan doses: %+v", op101.CentresisPlan)
	}
	lowerAlert := strings.ToLower(op101.CentresisPlan.TamponadePitfallAlert)
	if !strings.Contains(lowerAlert, "furosemide") || !strings.Contains(lowerAlert, "coronary") || !strings.Contains(lowerAlert, "clot tube") {
		t.Errorf("expected pitfall alert to warn against furosemide, left coronary artery, and clot tube, got %q", op101.CentresisPlan.TamponadePitfallAlert)
	}
	if len(op101.TapChecks) != 1 {
		t.Errorf("expected 1 initial tap check on PT-OP-101, got %d", len(op101.TapChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected PT-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("PT-OP-102")
	if err != nil {
		t.Fatalf("expected PT-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Mugsy" || op102.IncidentID != "INC-PT-9922" || op102.Status != domain.TamponadeStatusRightICS45TapStaged {
		t.Errorf("unexpected PT-OP-102 metadata: %+v", op102)
	}
	if op102.CentresisPlan == nil || op102.CentresisPlan.LidocaineIV2MgKgArrhythmiaMg != 27.0 || op102.CentresisPlan.CrystalloidPreloadExpansionMl != 135.0 {
		t.Errorf("unexpected PT-OP-102 plan: %+v", op102.CentresisPlan)
	}

	op103, err := svc.GetOperation("PT-OP-103")
	if err != nil {
		t.Fatalf("expected PT-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Samson" || op103.IncidentID != "INC-PT-9923" || op103.Status != domain.TamponadeStatusDecompressedSinusCleared {
		t.Errorf("unexpected PT-OP-103 metadata: %+v", op103)
	}
	if op103.FurosemideLeftCorpnaryHazardAlert || op103.DischargeClearance == nil {
		t.Errorf("expected PT-OP-103 to have DischargeClearance and false alert, got %+v", op103)
	}
	// 58 kg * 0.1 mL/kg = 5.8 mL, capped at 4.0 mL
	if op103.CentresisPlan.Lidocaine2PctInfiltrationMl != 4.0 {
		t.Errorf("expected Lidocaine2PctInfiltrationMl capped at 4.0 mL for 58 kg dog, got %v", op103.CentresisPlan.Lidocaine2PctInfiltrationMl)
	}

	criticalOps := svc.ListOperations(string(domain.TamponadeStatusRADiastolicCollapseCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "PT-OP-101" {
		t.Errorf("expected 1 critical op PT-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.CriticalTamponadeCount != 1 || metrics.RightICS45TapStagedCount != 1 || metrics.DecompressedSinusClearedCount != 1 || metrics.FurosemideLeftCoronaryHazardAlerts != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestPericardialTamponadeCreateStageTapCheckAndClearanceLifecycle(t *testing.T) {
	svc := pericardialtamponade.NewTamponadeService()

	profile := domain.TamponadePatientProfile{
		Species:                      domain.TamponadeSpeciesCanineRAHemangiosarcoma,
		Breed:                        "German Shepherd",
		WeightKg:                     36.0,
		HeartRateBpm:                 190,
		MeanArterialPressureMmHg:     50,
		RightAtrialDiastolicCollapse: true,
		ElectricalAlternansPresent:   true,
		PulsusParadoxusMmHg:          20,
		EffusionPCVPercent:           30.0,
	}

	created, err := svc.CreateOperation("INC-PT-9924", "Goliath", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "PT-OP-104" || created.Status != domain.TamponadeStatusRADiastolicCollapseCritical || !created.FurosemideLeftCorpnaryHazardAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if created.CentresisPlan.LidocaineIV2MgKgArrhythmiaMg != 72.0 || created.CentresisPlan.CrystalloidPreloadExpansionMl != 360.0 {
		t.Errorf("expected IV Lidocaine 72.0 mg and Crystalloid 360.0 mL, got %+v", created.CentresisPlan)
	}

	// Stage pericardiocentesis plan on PT-OP-104
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.HeartRateBpm = 165
	stagedProfile.MeanArterialPressureMmHg = 64

	staged, err := svc.StagePericardiocentesisPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StagePericardiocentesisPlan error: %v", err)
	}
	if staged.Status != domain.TamponadeStatusRightICS45TapStaged {
		t.Errorf("expected status %s, got %s", domain.TamponadeStatusRightICS45TapStaged, staged.Status)
	}
	if staged.PatientProfile.Breed != "German Shepherd" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Record tap check with full decompression & normotensive MAP
	checked, err := svc.RecordTapCheck(created.ID, domain.PericardiocentesisTapCheck{
		VolumeAspiratedMl:           280.0,
		SampleNonClottingVerified:   true,
		VentricularArrhythmiasNoted: false,
		PostTapMAPMmHg:              92,
		PostTapHeartRateBpm:         105,
		ClinicianCallsign:           "DR-CARDIO-44",
		Notes:                       "280 mL non-clotting hemorrhagic fluid aspirated via right 4th-5th ICS; immediate normalization of QRS complexes and MAP 92 mmHg.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordTapCheck error: %v", err)
	}
	if len(checked.TapChecks) != 1 || checked.FurosemideLeftCorpnaryHazardAlert {
		t.Errorf("expected 1 tap check and false hazard alert when MAP 92 and HR 105, got %+v", checked)
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.TamponadeDischargeClearance{
		FinalMAPMmHg:                       94,
		RightAtrialCollapseResolved:        true,
		RepeatTfast24HrNoReaccumulation:    true,
		SinusRhythmNoVPCs:                  true,
		YunnanBaiyaoOrOncologyPlanEducated: true,
		VetLicense:                         "VET-CARDIO-88120",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.TamponadeStatusDecompressedSinusCleared || cleared.FurosemideLeftCorpnaryHazardAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse tap check with recurrent tamponade hypotension & tachycardia reverts to critical
	relapsed, err := svc.RecordTapCheck(created.ID, domain.PericardiocentesisTapCheck{
		VolumeAspiratedMl:           120.0,
		SampleNonClottingVerified:   true,
		VentricularArrhythmiasNoted: true,
		PostTapMAPMmHg:              54,
		PostTapHeartRateBpm:         184,
		ClinicianCallsign:           "DR-CARDIO-44",
		Notes:                       "Acute reaccumulation with recurrent hypotension MAP 54 mmHg and VPCs.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordTapCheck error: %v", err)
	}
	if relapsed.Status != domain.TamponadeStatusRADiastolicCollapseCritical || !relapsed.FurosemideLeftCorpnaryHazardAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to TAMPONADE_RA_COLLAPSE_ALTERNANS_CRITICAL, got %+v", relapsed)
	}
}

func TestPericardialTamponadeValidationErrors(t *testing.T) {
	svc := pericardialtamponade.NewTamponadeService()

	validProfile := domain.TamponadePatientProfile{
		Species:                      domain.TamponadeSpeciesCanineHeartBaseChemodectoma,
		WeightKg:                     16.0,
		HeartRateBpm:                 148,
		MeanArterialPressureMmHg:     68,
		RightAtrialDiastolicCollapse: true,
		ElectricalAlternansPresent:   false,
		PulsusParadoxusMmHg:          14,
		EffusionPCVPercent:           24.0,
	}

	if _, err := svc.CreateOperation("", "Goliath", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.TamponadePatientProfile)
	}{
		{"invalid species", func(p *domain.TamponadePatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.TamponadePatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.TamponadePatientProfile) { p.WeightKg = 140 }},
		{"hr too low", func(p *domain.TamponadePatientProfile) { p.HeartRateBpm = 15 }},
		{"hr too high", func(p *domain.TamponadePatientProfile) { p.HeartRateBpm = 350 }},
		{"map too low", func(p *domain.TamponadePatientProfile) { p.MeanArterialPressureMmHg = 15 }},
		{"map too high", func(p *domain.TamponadePatientProfile) { p.MeanArterialPressureMmHg = 240 }},
		{"pulsus paradoxus negative", func(p *domain.TamponadePatientProfile) { p.PulsusParadoxusMmHg = -5 }},
		{"pulsus paradoxus too high", func(p *domain.TamponadePatientProfile) { p.PulsusParadoxusMmHg = 80 }},
		{"effusion pcv negative", func(p *domain.TamponadePatientProfile) { p.EffusionPCVPercent = -2.0 }},
		{"effusion pcv too high", func(p *domain.TamponadePatientProfile) { p.EffusionPCVPercent = 85.0 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Goliath", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StagePericardiocentesisPlan("PT-OP-101", p); err == nil {
				t.Errorf("expected StagePericardiocentesisPlan error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StagePericardiocentesisPlan("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging plan on unknown op")
	}

	validCheck := domain.PericardiocentesisTapCheck{
		VolumeAspiratedMl:           180.0,
		SampleNonClottingVerified:   true,
		VentricularArrhythmiasNoted: false,
		PostTapMAPMmHg:              88,
		PostTapHeartRateBpm:         112,
		ClinicianCallsign:           "DR-1",
	}
	if _, err := svc.RecordTapCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording tap check on unknown op")
	}

	badChecks := []func(c *domain.PericardiocentesisTapCheck){
		func(c *domain.PericardiocentesisTapCheck) { c.VolumeAspiratedMl = 0.0 },
		func(c *domain.PericardiocentesisTapCheck) { c.VolumeAspiratedMl = 3000.0 },
		func(c *domain.PericardiocentesisTapCheck) { c.SampleNonClottingVerified = false },
		func(c *domain.PericardiocentesisTapCheck) { c.PostTapMAPMmHg = 15 },
		func(c *domain.PericardiocentesisTapCheck) { c.PostTapMAPMmHg = 240 },
		func(c *domain.PericardiocentesisTapCheck) { c.PostTapHeartRateBpm = 10 },
		func(c *domain.PericardiocentesisTapCheck) { c.PostTapHeartRateBpm = 350 },
		func(c *domain.PericardiocentesisTapCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordTapCheck("PT-OP-101", c); err == nil {
			t.Errorf("expected RecordTapCheck validation error for %+v", c)
		}
	}

	validClearance := domain.TamponadeDischargeClearance{
		FinalMAPMmHg:                       92,
		RightAtrialCollapseResolved:        true,
		RepeatTfast24HrNoReaccumulation:    true,
		SinusRhythmNoVPCs:                  true,
		YunnanBaiyaoOrOncologyPlanEducated: true,
		VetLicense:                         "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.TamponadeDischargeClearance){
		func(d *domain.TamponadeDischargeClearance) { d.FinalMAPMmHg = 68 },
		func(d *domain.TamponadeDischargeClearance) { d.FinalMAPMmHg = 135 },
		func(d *domain.TamponadeDischargeClearance) { d.RightAtrialCollapseResolved = false },
		func(d *domain.TamponadeDischargeClearance) { d.RepeatTfast24HrNoReaccumulation = false },
		func(d *domain.TamponadeDischargeClearance) { d.SinusRhythmNoVPCs = false },
		func(d *domain.TamponadeDischargeClearance) { d.YunnanBaiyaoOrOncologyPlanEducated = false },
		func(d *domain.TamponadeDischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("PT-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}

	// Validate PericardialTamponadeOperation error paths
	op := domain.PericardialTamponadeOperation{
		ID:             "PT-OP-101",
		IncidentID:     "INC-PT-9921",
		PetName:        "Brutus",
		Status:         domain.TamponadeStatusRADiastolicCollapseCritical,
		PatientProfile: validProfile,
	}
	badOps := []func(o *domain.PericardialTamponadeOperation){
		func(o *domain.PericardialTamponadeOperation) { o.ID = "" },
		func(o *domain.PericardialTamponadeOperation) { o.IncidentID = "" },
		func(o *domain.PericardialTamponadeOperation) { o.PetName = "" },
		func(o *domain.PericardialTamponadeOperation) { o.Status = "INVALID_STATUS" },
		func(o *domain.PericardialTamponadeOperation) { o.PatientProfile.WeightKg = -1 },
	}
	for _, mutate := range badOps {
		cp := op
		mutate(&cp)
		if err := cp.Validate(); err == nil {
			t.Errorf("expected PericardialTamponadeOperation.Validate error for %+v", cp)
		}
	}
}
