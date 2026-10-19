package statusepilepticus_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/statusepilepticus"
)

func TestStatusEpilepticusSeededOperationsAndMetrics(t *testing.T) {
	svc := statusepilepticus.NewEpilepticusService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("SE-OP-101")
	if err != nil {
		t.Fatalf("expected SE-OP-101 to exist, got error: %v", err)
	}
	if op101.PetName != "Koda" || op101.IncidentID != "INC-SE-9201" || op101.Status != domain.EpilepticusStatusActiveIctalCritical {
		t.Errorf("unexpected SE-OP-101 metadata: %+v", op101)
	}
	if !op101.NeurogenicHyperthermiaAlert || op101.PatientProfile.WeightKg != 24.0 || op101.PatientProfile.RectalTempFahrenheit != 106.4 {
		t.Errorf("unexpected SE-OP-101 profile: %+v", op101.PatientProfile)
	}
	if op101.TitrationRegimen == nil || !op101.TitrationRegimen.ZeroOralGagAspirationGuard || op101.TitrationRegimen.StopCoolingThresholdTempF != 103.0 {
		t.Errorf("unexpected SE-OP-101 regimen: %+v", op101.TitrationRegimen)
	}
	lowerAlert := strings.ToLower(op101.TitrationRegimen.NeuroprotectionPitfallAlert)
	if !strings.Contains(lowerAlert, "mouth") || !strings.Contains(lowerAlert, "diazepam") || !strings.Contains(lowerAlert, "103.0") {
		t.Errorf("expected pitfall alert to warn against mouth objects, feline oral diazepam, and 103.0°F cooling threshold, got %q", op101.TitrationRegimen.NeuroprotectionPitfallAlert)
	}
	if len(op101.NeuroChecks) != 1 {
		t.Errorf("expected 1 initial neuro check on SE-OP-101, got %d", len(op101.NeuroChecks))
	}
	if err := op101.Validate(); err != nil {
		t.Errorf("expected SE-OP-101 to validate, got %v", err)
	}

	op102, err := svc.GetOperation("SE-OP-102")
	if err != nil {
		t.Fatalf("expected SE-OP-102 to exist, got error: %v", err)
	}
	if op102.PetName != "Mochi" || op102.IncidentID != "INC-SE-9202" || op102.Status != domain.EpilepticusStatusAnticonvulsantCRIStaged {
		t.Errorf("unexpected SE-OP-102 metadata: %+v", op102)
	}
	if op102.TitrationRegimen == nil || op102.TitrationRegimen.Methocarbamol55MgKgDoseMg <= 0 {
		t.Errorf("expected Methocarbamol dose > 0 for feline permethrin on SE-OP-102, got %+v", op102.TitrationRegimen)
	}

	op103, err := svc.GetOperation("SE-OP-103")
	if err != nil {
		t.Fatalf("expected SE-OP-103 to exist, got error: %v", err)
	}
	if op103.PetName != "Bowie" || op103.IncidentID != "INC-SE-9203" || op103.Status != domain.EpilepticusStatusSeizureFreeNormothermic {
		t.Errorf("unexpected SE-OP-103 metadata: %+v", op103)
	}
	if op103.NeurogenicHyperthermiaAlert || op103.DischargeClearance == nil {
		t.Errorf("expected SE-OP-103 to have DischargeClearance and false hyperthermia alert, got %+v", op103)
	}

	criticalOps := svc.ListOperations(string(domain.EpilepticusStatusActiveIctalCritical))
	if len(criticalOps) != 1 || criticalOps[0].ID != "SE-OP-101" {
		t.Errorf("expected 1 critical op SE-OP-101, got %+v", criticalOps)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.ActiveIctalCriticalCount != 1 || metrics.AnticonvulsantCRIStagedCount != 1 || metrics.SeizureFreeClearedCount != 1 || metrics.HyperthermiaAlertCount != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("NON-EXISTENT"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestStatusEpilepticusCreateStageCheckAndClearanceLifecycle(t *testing.T) {
	svc := statusepilepticus.NewEpilepticusService()

	profile := domain.EpilepticusPatientProfile{
		Species:                  domain.EpilepticusSpeciesCanine,
		Breed:                    "Belgian Malinois",
		WeightKg:                 28.0,
		NeuroEtiology:            domain.NeuroEtiologyBromethalinCerebralEdema,
		ContinuousIctalMinutes:   11,
		ClusterCount24Hr:         4,
		RectalTempFahrenheit:     105.8,
		BloodGlucoseMgDl:         58.0,
		ModifiedGlasgowComaScore: 10,
	}

	created, err := svc.CreateOperation("INC-SE-9204", "Nova", profile)
	if err != nil {
		t.Fatalf("unexpected CreateOperation error: %v", err)
	}
	if created.ID != "SE-OP-104" || created.Status != domain.EpilepticusStatusActiveIctalCritical || !created.NeurogenicHyperthermiaAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}
	if created.TitrationRegimen == nil || created.TitrationRegimen.Mannitol05GKgCerebralEdemaGrams != 14.0 {
		t.Errorf("expected mannitol 14.0 g for 28 kg bromethalin dog, got %+v", created.TitrationRegimen)
	}

	// Stage regimen on SE-OP-104 with Metaldehyde tremor etiology and mild idiopathic profile test
	stagedProfile := profile
	stagedProfile.Breed = ""
	stagedProfile.NeuroEtiology = domain.NeuroEtiologyMetaldehydeSnailBaitTremor
	stagedProfile.ContinuousIctalMinutes = 6
	stagedProfile.RectalTempFahrenheit = 104.2
	stagedProfile.BloodGlucoseMgDl = 88.0
	stagedProfile.ModifiedGlasgowComaScore = 13

	staged, err := svc.StageTitrationRegimen(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("unexpected StageTitrationRegimen error: %v", err)
	}
	if staged.Status != domain.EpilepticusStatusAnticonvulsantCRIStaged {
		t.Errorf("expected status %s, got %s", domain.EpilepticusStatusAnticonvulsantCRIStaged, staged.Status)
	}
	if staged.TitrationRegimen.MidazolamFirstLineBolusMg != 5.6 ||
		staged.TitrationRegimen.LevetiracetamLoading60MgKgMg != 1680.0 ||
		staged.TitrationRegimen.Methocarbamol55MgKgDoseMg != 1540.0 ||
		staged.TitrationRegimen.PropofolCRIRateMgKgMin != 0.1 {
		t.Errorf("unexpected staged regimen doses: %+v", staged.TitrationRegimen)
	}
	if staged.PatientProfile.Breed != "Belgian Malinois" {
		t.Errorf("expected breed to be preserved when empty on stage, got %q", staged.PatientProfile.Breed)
	}

	// Also verify mild idiopathic regimen branch (propofol 0.05)
	mildRegimen := statusepilepticus.BuildTitrationRegimen(domain.EpilepticusPatientProfile{
		Species:                  domain.EpilepticusSpeciesCanine,
		WeightKg:                 10.0,
		NeuroEtiology:            domain.NeuroEtiologyIdiopathicStatusEpilepticus,
		ContinuousIctalMinutes:   2,
		ClusterCount24Hr:         1,
		RectalTempFahrenheit:     101.5,
		BloodGlucoseMgDl:         95.0,
		ModifiedGlasgowComaScore: 16,
	})
	if mildRegimen.PropofolCRIRateMgKgMin != 0.05 {
		t.Errorf("expected 0.05 propofol CRI for mild cluster, got %v", mildRegimen.PropofolCRIRateMgKgMin)
	}

	// Record serial neuro check (normothermic, no seizure)
	withCheck, err := svc.RecordNeuroCheck(created.ID, domain.NeuroIcuSerialCheck{
		SeizureActivityObserved:  false,
		RectalTempFahrenheit:     101.6,
		ModifiedGlasgowComaScore: 16,
		PupillaryLightReflex:     domain.PupillaryReflexNormalBilateral,
		BloodGlucoseMgDl:         96.0,
		ClinicianCallsign:        "DR-NEURO-88",
		Notes:                    "Tremors resolved following methocarbamol and levetiracetam; cooling stopped at 103.0°F; now 101.6°F.",
	})
	if err != nil {
		t.Fatalf("unexpected RecordNeuroCheck error: %v", err)
	}
	if len(withCheck.NeuroChecks) != 1 || withCheck.NeurogenicHyperthermiaAlert {
		t.Errorf("expected 1 neuro check and false hyperthermia alert at 101.6°F, got %+v", withCheck)
	}

	// Issue discharge clearance
	cleared, err := svc.IssueDischargeClearance(created.ID, domain.EpilepticusDischargeClearance{
		SeizureFreeHours:                   24,
		FinalModifiedGlasgowScore:          18,
		FinalRectalTempFahrenheit:          101.0,
		AmbulatoryMenaceIntact:             true,
		RescueIntranasalMidazolamDispensed: true,
		VetLicense:                         "VET-OR-55210",
	})
	if err != nil {
		t.Fatalf("unexpected IssueDischargeClearance error: %v", err)
	}
	if cleared.Status != domain.EpilepticusStatusSeizureFreeNormothermic || cleared.NeurogenicHyperthermiaAlert || cleared.DischargeClearance == nil {
		t.Errorf("unexpected cleared state: %+v", cleared)
	}
	if cleared.DischargeClearance.Summary == "" {
		t.Errorf("expected auto-populated discharge summary when empty")
	}

	// Relapse neuro check with active seizure transitions back to critical
	relapsed, err := svc.RecordNeuroCheck(created.ID, domain.NeuroIcuSerialCheck{
		SeizureActivityObserved:  true,
		RectalTempFahrenheit:     104.8,
		ModifiedGlasgowComaScore: 10,
		PupillaryLightReflex:     domain.PupillaryReflexAnisocoriaCushingReflex,
		BloodGlucoseMgDl:         72.0,
		ClinicianCallsign:        "DR-NEURO-88",
		Notes:                    "Breakthrough cluster seizure observed.",
	})
	if err != nil {
		t.Fatalf("unexpected relapse RecordNeuroCheck error: %v", err)
	}
	if relapsed.Status != domain.EpilepticusStatusActiveIctalCritical || !relapsed.NeurogenicHyperthermiaAlert || relapsed.DischargeClearance != nil {
		t.Errorf("expected relapse to revert to ACTIVE_ICTAL_HYPERTHERMIA_CRITICAL, got %+v", relapsed)
	}
}

func TestStatusEpilepticusValidationErrors(t *testing.T) {
	svc := statusepilepticus.NewEpilepticusService()

	validProfile := domain.EpilepticusPatientProfile{
		Species:                  domain.EpilepticusSpeciesCanine,
		WeightKg:                 20.0,
		NeuroEtiology:            domain.NeuroEtiologyIdiopathicStatusEpilepticus,
		ContinuousIctalMinutes:   6,
		ClusterCount24Hr:         3,
		RectalTempFahrenheit:     104.5,
		BloodGlucoseMgDl:         85.0,
		ModifiedGlasgowComaScore: 12,
	}

	if _, err := svc.CreateOperation("", "Koda", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	badProfiles := []struct {
		name   string
		mutate func(p *domain.EpilepticusPatientProfile)
	}{
		{"invalid species", func(p *domain.EpilepticusPatientProfile) { p.Species = "EQUINE" }},
		{"weight <= 0", func(p *domain.EpilepticusPatientProfile) { p.WeightKg = 0 }},
		{"weight > 120", func(p *domain.EpilepticusPatientProfile) { p.WeightKg = 150 }},
		{"invalid etiology", func(p *domain.EpilepticusPatientProfile) { p.NeuroEtiology = "UNKNOWN" }},
		{"negative ictal minutes", func(p *domain.EpilepticusPatientProfile) { p.ContinuousIctalMinutes = -1 }},
		{"negative cluster count", func(p *domain.EpilepticusPatientProfile) { p.ClusterCount24Hr = -1 }},
		{"temp too low", func(p *domain.EpilepticusPatientProfile) { p.RectalTempFahrenheit = 88.0 }},
		{"temp too high", func(p *domain.EpilepticusPatientProfile) { p.RectalTempFahrenheit = 115.0 }},
		{"glucose too low", func(p *domain.EpilepticusPatientProfile) { p.BloodGlucoseMgDl = 5.0 }},
		{"mgcs < 3", func(p *domain.EpilepticusPatientProfile) { p.ModifiedGlasgowComaScore = 2 }},
		{"mgcs > 18", func(p *domain.EpilepticusPatientProfile) { p.ModifiedGlasgowComaScore = 19 }},
	}

	for _, tc := range badProfiles {
		t.Run(tc.name, func(t *testing.T) {
			p := validProfile
			tc.mutate(&p)
			if _, err := svc.CreateOperation("INC-1", "Koda", p); err == nil {
				t.Errorf("expected CreateOperation error for %s", tc.name)
			}
			if _, err := svc.StageTitrationRegimen("SE-OP-101", p); err == nil {
				t.Errorf("expected StageTitrationRegimen error for %s", tc.name)
			}
		})
	}

	if _, err := svc.StageTitrationRegimen("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging regimen on unknown op")
	}

	validCheck := domain.NeuroIcuSerialCheck{
		RectalTempFahrenheit:     101.5,
		ModifiedGlasgowComaScore: 15,
		PupillaryLightReflex:     domain.PupillaryReflexNormalBilateral,
		BloodGlucoseMgDl:         90.0,
		ClinicianCallsign:        "DR-1",
	}
	if _, err := svc.RecordNeuroCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error recording neuro check on unknown op")
	}

	badChecks := []func(c *domain.NeuroIcuSerialCheck){
		func(c *domain.NeuroIcuSerialCheck) { c.RectalTempFahrenheit = 90.0 },
		func(c *domain.NeuroIcuSerialCheck) { c.ModifiedGlasgowComaScore = 1 },
		func(c *domain.NeuroIcuSerialCheck) { c.PupillaryLightReflex = "INVALID_PLR" },
		func(c *domain.NeuroIcuSerialCheck) { c.BloodGlucoseMgDl = 10.0 },
		func(c *domain.NeuroIcuSerialCheck) { c.ClinicianCallsign = "" },
	}
	for _, mutate := range badChecks {
		c := validCheck
		mutate(&c)
		if _, err := svc.RecordNeuroCheck("SE-OP-101", c); err == nil {
			t.Errorf("expected RecordNeuroCheck validation error for %+v", c)
		}
	}

	validClearance := domain.EpilepticusDischargeClearance{
		SeizureFreeHours:                   24,
		FinalModifiedGlasgowScore:          17,
		FinalRectalTempFahrenheit:          101.2,
		AmbulatoryMenaceIntact:             true,
		RescueIntranasalMidazolamDispensed: true,
		VetLicense:                         "VET-123",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error issuing clearance on unknown op")
	}

	badClearances := []func(d *domain.EpilepticusDischargeClearance){
		func(d *domain.EpilepticusDischargeClearance) { d.SeizureFreeHours = 12 },
		func(d *domain.EpilepticusDischargeClearance) { d.FinalModifiedGlasgowScore = 14 },
		func(d *domain.EpilepticusDischargeClearance) { d.FinalRectalTempFahrenheit = 103.2 },
		func(d *domain.EpilepticusDischargeClearance) { d.FinalRectalTempFahrenheit = 97.5 },
		func(d *domain.EpilepticusDischargeClearance) { d.AmbulatoryMenaceIntact = false },
		func(d *domain.EpilepticusDischargeClearance) { d.RescueIntranasalMidazolamDispensed = false },
		func(d *domain.EpilepticusDischargeClearance) { d.VetLicense = "  " },
	}
	for _, mutate := range badClearances {
		d := validClearance
		mutate(&d)
		if _, err := svc.IssueDischargeClearance("SE-OP-101", d); err == nil {
			t.Errorf("expected IssueDischargeClearance validation error for %+v", d)
		}
	}
}
