package airwaytracheostomy_test

import (
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/airwaytracheostomy"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestAirwayTracheostomyService_SeededOperationsAndMetrics(t *testing.T) {
	svc := airwaytracheostomy.NewAirwayService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	byID := make(map[string]domain.AirwayTracheostomyOperation)
	for _, op := range ops {
		byID[op.ID] = op
	}

	op101, ok := byID["AT-OP-101"]
	if !ok {
		t.Fatalf("expected AT-OP-101 to exist")
	}
	if op101.PetName != "Barnaby" || op101.IncidentID != "INC-AT-8901" || op101.Status != domain.AirwayStatusStridorCyanosisCritical || !op101.CriticalAirwayAlert {
		t.Errorf("unexpected AT-OP-101 metadata: %+v", op101)
	}
	if op101.StabilizationPlan == nil || !op101.StabilizationPlan.TemporaryTracheostomyIndicated || !op101.StabilizationPlan.ZeroNeckLeadRestraintGuard {
		t.Errorf("expected AT-OP-101 stabilization plan with temporary tracheostomy and zero neck lead guard, got %+v", op101.StabilizationPlan)
	}
	if !strings.Contains(strings.ToLower(op101.StabilizationPlan.AirwayHazardAlert), "butorphanol") || !strings.Contains(op101.StabilizationPlan.AirwayHazardAlert, "10 seconds") {
		t.Errorf("expected AT-OP-101 hazard alert to warn about butorphanol and <=10 seconds suctioning, got %q", op101.StabilizationPlan.AirwayHazardAlert)
	}
	if len(op101.TrachCareLogs) != 1 {
		t.Errorf("expected 1 seeded trach care log on AT-OP-101, got %d", len(op101.TrachCareLogs))
	}

	op102, ok := byID["AT-OP-102"]
	if !ok {
		t.Fatalf("expected AT-OP-102 to exist")
	}
	if op102.PetName != "Yukon" || op102.Status != domain.AirwayStatusSedationDexamethasoneStaged || !op102.CriticalAirwayAlert {
		t.Errorf("unexpected AT-OP-102 state: %+v", op102)
	}

	op103, ok := byID["AT-OP-103"]
	if !ok {
		t.Fatalf("expected AT-OP-103 to exist")
	}
	if op103.PetName != "Pippin" || op103.Status != domain.AirwayStatusPatentNormothermicExtubated || op103.CriticalAirwayAlert || op103.ExtubationClearance == nil {
		t.Errorf("unexpected AT-OP-103 state: %+v", op103)
	}

	filtered := svc.ListOperations(string(domain.AirwayStatusPatentNormothermicExtubated))
	if len(filtered) != 1 || filtered[0].ID != "AT-OP-103" {
		t.Errorf("expected 1 filtered operation AT-OP-103, got %+v", filtered)
	}

	got101, err := svc.GetOperation("AT-OP-101")
	if err != nil || got101.PetName != "Barnaby" {
		t.Errorf("expected GetOperation(AT-OP-101) to succeed, got %v, err=%v", got101, err)
	}

	if _, err := svc.GetOperation("UNKNOWN-OP"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.CriticalStridorCyanosis != 1 || metrics.SedationIntubationStaged != 1 || metrics.PatentNormothermicExtubated != 1 || metrics.CriticalAirwayAlerts != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}
}

func TestAirwayTracheostomyService_CreateStageCareAndClearanceLifecycle(t *testing.T) {
	svc := airwaytracheostomy.NewAirwayService()

	profile := domain.AirwayPatientProfile{
		Species:              domain.AirwaySpeciesCanineBrachycephalicBOAS,
		Breed:                "English Bulldog",
		WeightKg:             24.0,
		RectalTempFahrenheit: 105.4,
		SpO2Percent:          85.0,
		InspiratoryEffort:    domain.AirwayEffortSevereParadoxicalCyanotic,
		LaryngealEdemaGrade:  domain.AirwayEdemaGradeIIIEvertedSacculesCollapse,
		GagReflexPresent:     false,
	}

	created, err := svc.CreateOperation("INC-AT-9001", "Otis", profile)
	if err != nil {
		t.Fatalf("CreateOperation failed: %v", err)
	}
	if created.ID != "AT-OP-104" || created.Status != domain.AirwayStatusStridorCyanosisCritical || !created.CriticalAirwayAlert {
		t.Errorf("unexpected created operation: %+v", created)
	}

	// Stage stabilization plan
	stagedProfile := profile
	stagedProfile.RectalTempFahrenheit = 102.8
	stagedProfile.SpO2Percent = 93.0
	staged, err := svc.StageStabilizationPlan(created.ID, stagedProfile)
	if err != nil {
		t.Fatalf("StageStabilizationPlan failed: %v", err)
	}
	if staged.Status != domain.AirwayStatusSedationDexamethasoneStaged {
		t.Errorf("expected status BUTORPHANOL_DEX_INTUBATION_STAGED, got %s", staged.Status)
	}
	if staged.StabilizationPlan.EndotrachealTubeSizeMm != 7.5 {
		t.Errorf("expected downsized ET tube 7.5 mm for 24kg Bulldog, got %.1f", staged.StabilizationPlan.EndotrachealTubeSizeMm)
	}

	// Attempt clearance before recording trach care log -> error
	_, err = svc.IssueExtubationClearance(created.ID, domain.AirwayExtubationClearance{
		FinalSpO2OnRoomAirPercent:    97.0,
		FinalRectalTempFahrenheit:    101.2,
		GagSwallowReflexVigorous:     true,
		TrachTubeOccludedTrialPassed: true,
		HarnessOnlyDischargeVerified: true,
		VetLicense:                   "DVM-WA-88210",
		Summary:                      "Ready for extubation",
	})
	if err == nil {
		t.Errorf("expected error when issuing clearance without any tracheostomy care logs")
	}

	// Record valid trach care log
	withLog, err := svc.RecordTrachCareLog(created.ID, domain.TracheostomyCareLog{
		SterileSalineInstillationMl: 1.5,
		PreOxygenationDurationSec:   90,
		SuctionDurationMaxSec:       8,
		InnerCannulaCleaned:         true,
		SubcutaneousEmphysemaNoted:  true,
		PostSuctionSpO2Percent:      96.0,
		TechnicianCallsign:          "RVT-KIM-02",
		Notes:                       "Inner cannula cleaned; mild subcutaneous crepitus monitored.",
	})
	if err != nil {
		t.Fatalf("RecordTrachCareLog failed: %v", err)
	}
	if len(withLog.TrachCareLogs) != 1 || !withLog.CriticalAirwayAlert {
		t.Errorf("expected 1 care log and CriticalAirwayAlert=true, got %+v", withLog)
	}

	// Issue extubation clearance
	cleared, err := svc.IssueExtubationClearance(created.ID, domain.AirwayExtubationClearance{
		FinalSpO2OnRoomAirPercent:    97.5,
		FinalRectalTempFahrenheit:    101.0,
		GagSwallowReflexVigorous:     true,
		TrachTubeOccludedTrialPassed: true,
		HarnessOnlyDischargeVerified: true,
		VetLicense:                   "DVM-WA-88210",
		Summary:                      "Vigorous swallow reflex, SpO2 97.5% on room air, normothermic 101.0°F, chest harness verified.",
	})
	if err != nil {
		t.Fatalf("IssueExtubationClearance failed: %v", err)
	}
	if cleared.Status != domain.AirwayStatusPatentNormothermicExtubated || cleared.CriticalAirwayAlert {
		t.Errorf("expected PATENT_AIRWAY_NORMOTHERMIC_EXTUBATED and CriticalAirwayAlert=false, got %+v", cleared)
	}
}

func TestAirwayTracheostomyService_SizingAndValidationBranches(t *testing.T) {
	if !domain.AirwayStatusStridorCyanosisCritical.IsValid() ||
		!domain.AirwayStatusSedationDexamethasoneStaged.IsValid() ||
		!domain.AirwayStatusPatentNormothermicExtubated.IsValid() ||
		domain.AirwayObstructionStatus("INVALID").IsValid() {
		t.Errorf("unexpected AirwayObstructionStatus.IsValid behavior")
	}

	// Test ET tube sizing branches across species and weight brackets
	cases := []struct {
		species string
		weight  float64
		wantET  float64
	}{
		{domain.AirwaySpeciesFelineNasopharyngealPolyp, 2.8, 3.0},
		{domain.AirwaySpeciesFelineNasopharyngealPolyp, 4.2, 3.5},
		{domain.AirwaySpeciesFelineNasopharyngealPolyp, 6.0, 4.0},
		{domain.AirwaySpeciesCanineBrachycephalicBOAS, 5.5, 4.0},
		{domain.AirwaySpeciesCanineBrachycephalicBOAS, 9.0, 4.5},
		{domain.AirwaySpeciesCanineBrachycephalicBOAS, 12.4, 5.5},
		{domain.AirwaySpeciesCanineBrachycephalicBOAS, 18.0, 6.5},
		{domain.AirwaySpeciesCanineBrachycephalicBOAS, 25.0, 7.5},
		{domain.AirwaySpeciesCanineGeriatricLarPar, 8.0, 7.0},
		{domain.AirwaySpeciesCanineGeriatricLarPar, 15.0, 8.5},
		{domain.AirwaySpeciesCanineGeriatricLarPar, 25.0, 9.5},
		{domain.AirwaySpeciesCanineGeriatricLarPar, 32.0, 10.5},
		{domain.AirwaySpeciesCanineGeriatricLarPar, 45.0, 11.5},
	}

	for _, tc := range cases {
		plan, err := airwaytracheostomy.ComputeStabilizationPlan(domain.AirwayPatientProfile{
			Species:              tc.species,
			Breed:                "Test Breed",
			WeightKg:             tc.weight,
			RectalTempFahrenheit: 101.5,
			SpO2Percent:          95.0,
			InspiratoryEffort:    domain.AirwayEffortMildStertorEupneic,
			LaryngealEdemaGrade:  domain.AirwayEdemaGradeIStenoticNaresOnly,
			GagReflexPresent:     true,
		})
		if err != nil {
			t.Fatalf("unexpected error for %s %.1fkg: %v", tc.species, tc.weight, err)
		}
		if plan.EndotrachealTubeSizeMm != tc.wantET {
			t.Errorf("for %s %.1fkg expected ET tube %.1f mm, got %.1f mm", tc.species, tc.weight, tc.wantET, plan.EndotrachealTubeSizeMm)
		}
	}

	svc := airwaytracheostomy.NewAirwayService()
	validProfile := domain.AirwayPatientProfile{
		Species:              domain.AirwaySpeciesCanineBrachycephalicBOAS,
		Breed:                "Pug",
		WeightKg:             9.0,
		RectalTempFahrenheit: 101.5,
		SpO2Percent:          96.0,
		InspiratoryEffort:    domain.AirwayEffortMildStertorEupneic,
		LaryngealEdemaGrade:  domain.AirwayEdemaGradeIStenoticNaresOnly,
		GagReflexPresent:     true,
	}

	if _, err := svc.CreateOperation("", "Puggy", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}

	// Profile validation failures
	invalidProfiles := []domain.AirwayPatientProfile{
		{Species: "INVALID", Breed: "Pug", WeightKg: 9, RectalTempFahrenheit: 101, SpO2Percent: 95, InspiratoryEffort: domain.AirwayEffortMildStertorEupneic, LaryngealEdemaGrade: domain.AirwayEdemaGradeIStenoticNaresOnly},
		{Species: domain.AirwaySpeciesCanineBrachycephalicBOAS, Breed: "", WeightKg: 9, RectalTempFahrenheit: 101, SpO2Percent: 95, InspiratoryEffort: domain.AirwayEffortMildStertorEupneic, LaryngealEdemaGrade: domain.AirwayEdemaGradeIStenoticNaresOnly},
		{Species: domain.AirwaySpeciesCanineBrachycephalicBOAS, Breed: "Pug", WeightKg: 0.2, RectalTempFahrenheit: 101, SpO2Percent: 95, InspiratoryEffort: domain.AirwayEffortMildStertorEupneic, LaryngealEdemaGrade: domain.AirwayEdemaGradeIStenoticNaresOnly},
		{Species: domain.AirwaySpeciesCanineBrachycephalicBOAS, Breed: "Pug", WeightKg: 9, RectalTempFahrenheit: 91.0, SpO2Percent: 95, InspiratoryEffort: domain.AirwayEffortMildStertorEupneic, LaryngealEdemaGrade: domain.AirwayEdemaGradeIStenoticNaresOnly},
		{Species: domain.AirwaySpeciesCanineBrachycephalicBOAS, Breed: "Pug", WeightKg: 9, RectalTempFahrenheit: 101, SpO2Percent: 40.0, InspiratoryEffort: domain.AirwayEffortMildStertorEupneic, LaryngealEdemaGrade: domain.AirwayEdemaGradeIStenoticNaresOnly},
		{Species: domain.AirwaySpeciesCanineBrachycephalicBOAS, Breed: "Pug", WeightKg: 9, RectalTempFahrenheit: 101, SpO2Percent: 95, InspiratoryEffort: "BAD_EFFORT", LaryngealEdemaGrade: domain.AirwayEdemaGradeIStenoticNaresOnly},
		{Species: domain.AirwaySpeciesCanineBrachycephalicBOAS, Breed: "Pug", WeightKg: 9, RectalTempFahrenheit: 101, SpO2Percent: 95, InspiratoryEffort: domain.AirwayEffortMildStertorEupneic, LaryngealEdemaGrade: "BAD_GRADE"},
	}
	for i, ip := range invalidProfiles {
		if _, err := svc.CreateOperation("INC-X", "Test", ip); err == nil {
			t.Errorf("case %d: expected profile validation error", i)
		}
		if _, err := svc.StageStabilizationPlan("AT-OP-101", ip); err == nil {
			t.Errorf("case %d: expected StageStabilizationPlan validation error", i)
		}
	}

	if _, err := svc.StageStabilizationPlan("NON-EXISTENT", validProfile); err == nil {
		t.Errorf("expected error staging non-existent operation")
	}

	// TrachCareLog validation failures (including >10s suction and <60s pre-oxygenation)
	invalidLogs := []domain.TracheostomyCareLog{
		{PreOxygenationDurationSec: 30, SuctionDurationMaxSec: 8, SterileSalineInstillationMl: 1.0, PostSuctionSpO2Percent: 95.0, TechnicianCallsign: "RVT-1"},
		{PreOxygenationDurationSec: 60, SuctionDurationMaxSec: 12, SterileSalineInstillationMl: 1.0, PostSuctionSpO2Percent: 95.0, TechnicianCallsign: "RVT-1"},
		{PreOxygenationDurationSec: 60, SuctionDurationMaxSec: 8, SterileSalineInstillationMl: -1.0, PostSuctionSpO2Percent: 95.0, TechnicianCallsign: "RVT-1"},
		{PreOxygenationDurationSec: 60, SuctionDurationMaxSec: 8, SterileSalineInstillationMl: 1.0, PostSuctionSpO2Percent: 40.0, TechnicianCallsign: "RVT-1"},
		{PreOxygenationDurationSec: 60, SuctionDurationMaxSec: 8, SterileSalineInstillationMl: 1.0, PostSuctionSpO2Percent: 95.0, TechnicianCallsign: ""},
	}
	for i, il := range invalidLogs {
		if _, err := svc.RecordTrachCareLog("AT-OP-101", il); err == nil {
			t.Errorf("log case %d: expected validation error", i)
		}
	}
	if _, err := svc.RecordTrachCareLog("NON-EXISTENT", domain.TracheostomyCareLog{
		Timestamp:                   time.Now(),
		PreOxygenationDurationSec:   60,
		SuctionDurationMaxSec:       8,
		SterileSalineInstillationMl: 1.0,
		PostSuctionSpO2Percent:      95.0,
		TechnicianCallsign:          "RVT-1",
	}); err == nil {
		t.Errorf("expected error recording log on non-existent operation")
	}

	// ExtubationClearance validation failures
	invalidClearances := []domain.AirwayExtubationClearance{
		{FinalSpO2OnRoomAirPercent: 91.0, FinalRectalTempFahrenheit: 100.5, GagSwallowReflexVigorous: true, TrachTubeOccludedTrialPassed: true, HarnessOnlyDischargeVerified: true, VetLicense: "DVM-1", Summary: "ok"},
		{FinalSpO2OnRoomAirPercent: 97.0, FinalRectalTempFahrenheit: 103.5, GagSwallowReflexVigorous: true, TrachTubeOccludedTrialPassed: true, HarnessOnlyDischargeVerified: true, VetLicense: "DVM-1", Summary: "ok"},
		{FinalSpO2OnRoomAirPercent: 97.0, FinalRectalTempFahrenheit: 100.5, GagSwallowReflexVigorous: false, TrachTubeOccludedTrialPassed: true, HarnessOnlyDischargeVerified: true, VetLicense: "DVM-1", Summary: "ok"},
		{FinalSpO2OnRoomAirPercent: 97.0, FinalRectalTempFahrenheit: 100.5, GagSwallowReflexVigorous: true, TrachTubeOccludedTrialPassed: false, HarnessOnlyDischargeVerified: true, VetLicense: "DVM-1", Summary: "ok"},
		{FinalSpO2OnRoomAirPercent: 97.0, FinalRectalTempFahrenheit: 100.5, GagSwallowReflexVigorous: true, TrachTubeOccludedTrialPassed: true, HarnessOnlyDischargeVerified: false, VetLicense: "DVM-1", Summary: "ok"},
		{FinalSpO2OnRoomAirPercent: 97.0, FinalRectalTempFahrenheit: 100.5, GagSwallowReflexVigorous: true, TrachTubeOccludedTrialPassed: true, HarnessOnlyDischargeVerified: true, VetLicense: "", Summary: "ok"},
		{FinalSpO2OnRoomAirPercent: 97.0, FinalRectalTempFahrenheit: 100.5, GagSwallowReflexVigorous: true, TrachTubeOccludedTrialPassed: true, HarnessOnlyDischargeVerified: true, VetLicense: "DVM-1", Summary: ""},
	}
	for i, ic := range invalidClearances {
		if _, err := svc.IssueExtubationClearance("AT-OP-101", ic); err == nil {
			t.Errorf("clearance case %d: expected validation error", i)
		}
	}
	if _, err := svc.IssueExtubationClearance("NON-EXISTENT", domain.AirwayExtubationClearance{
		ClearedAt:                    time.Now(),
		FinalSpO2OnRoomAirPercent:    97.0,
		FinalRectalTempFahrenheit:    100.5,
		GagSwallowReflexVigorous:     true,
		TrachTubeOccludedTrialPassed: true,
		HarnessOnlyDischargeVerified: true,
		VetLicense:                   "DVM-1",
		Summary:                      "ok",
	}); err == nil {
		t.Errorf("expected error issuing clearance on non-existent operation")
	}
}
