package cardiogenicedema_test

import (
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/cardiogenicedema"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestCardiogenicEdemaService_SeededOperationsAndMetrics(t *testing.T) {
	svc := cardiogenicedema.NewCardiogenicService()

	ops := svc.ListOperations("")
	if len(ops) != 3 {
		t.Fatalf("expected 3 seeded operations, got %d", len(ops))
	}

	op101, err := svc.GetOperation("CE-OP-101")
	if err != nil {
		t.Fatalf("expected CE-OP-101, got err: %v", err)
	}
	if op101.PetName != "Oliver" || op101.Status != domain.CardiogenicStatusFulminantEdemaCritical || !op101.FluidOverloadContraindicationAlert {
		t.Errorf("unexpected CE-OP-101 state: %+v", op101)
	}
	if op101.DiureticRegimen == nil || !op101.DiureticRegimen.ZeroIVCrystalloidFluidGuard {
		t.Errorf("expected ZeroIVCrystalloidFluidGuard on CE-OP-101")
	}
	if !strings.Contains(op101.DiureticRegimen.CardiogenicPitfallAlert, "CRYSTALLOID") {
		t.Errorf("expected pitfall alert on CE-OP-101, got %q", op101.DiureticRegimen.CardiogenicPitfallAlert)
	}

	op102, err := svc.GetOperation("CE-OP-102")
	if err != nil {
		t.Fatalf("expected CE-OP-102, got err: %v", err)
	}
	if op102.PetName != "Cleo" || op102.Status != domain.CardiogenicStatusFurosemideO2CageStaged || !op102.PatientProfile.PleuralEffusionPresent {
		t.Errorf("unexpected CE-OP-102 state: %+v", op102)
	}

	op103, err := svc.GetOperation("CE-OP-103")
	if err != nil {
		t.Fatalf("expected CE-OP-103, got err: %v", err)
	}
	if op103.PetName != "Teddy" || op103.Status != domain.CardiogenicStatusEupneicBLineResolved || op103.FluidOverloadContraindicationAlert || op103.DischargeClearance == nil {
		t.Errorf("unexpected CE-OP-103 state: %+v", op103)
	}

	filtered := svc.ListOperations(string(domain.CardiogenicStatusFulminantEdemaCritical))
	if len(filtered) != 1 || filtered[0].ID != "CE-OP-101" {
		t.Errorf("expected 1 filtered operation CE-OP-101, got %+v", filtered)
	}

	metrics := svc.ComputeMetrics()
	if metrics.TotalOperations != 3 || metrics.FulminantCritical != 1 || metrics.FurosemideO2Staged != 1 || metrics.EupneicCleared != 1 || metrics.ActiveFluidGuardLock != 2 {
		t.Errorf("unexpected metrics: %+v", metrics)
	}

	if _, err := svc.GetOperation("UNKNOWN"); err == nil {
		t.Errorf("expected error for unknown operation ID")
	}
}

func TestCardiogenicEdemaService_CreateStageDiuresisAndDischarge(t *testing.T) {
	svc := cardiogenicedema.NewCardiogenicService()

	canineProfile := domain.CardiogenicPatientProfile{
		Species:                domain.CardiogenicSpeciesCanineDCM,
		WeightKg:               28.0,
		RespiratoryRateBpm:     76,
		HeartRateBpm:           185,
		MurmurGrade:            domain.MurmurGradeVSystolicApical,
		TfastBLinesPerSite:     6,
		LaAoRatio:              2.30,
		PleuralEffusionPresent: false,
	}

	created, err := svc.CreateOperation("INC-CE-9200", "Darcy", canineProfile)
	if err != nil {
		t.Fatalf("failed to create operation: %v", err)
	}
	if created.ID != "CE-OP-104" || created.Status != domain.CardiogenicStatusFulminantEdemaCritical || !created.FluidOverloadContraindicationAlert {
		t.Errorf("unexpected created state: %+v", created)
	}
	if err := created.Validate(); err != nil {
		t.Errorf("expected created operation to validate, got %v", err)
	}

	// Stage feline severe profile
	felineSevereProfile := domain.CardiogenicPatientProfile{
		Species:                domain.CardiogenicSpeciesFelineHCM,
		WeightKg:               5.0,
		RespiratoryRateBpm:     74,
		HeartRateBpm:           220,
		MurmurGrade:            domain.MurmurGradeIVGallopRhythm,
		TfastBLinesPerSite:     6,
		LaAoRatio:              2.15,
		PleuralEffusionPresent: true,
	}
	stagedFeline, err := svc.StageDiureticRegimen(created.ID, felineSevereProfile)
	if err != nil {
		t.Fatalf("failed to stage feline regimen: %v", err)
	}
	if stagedFeline.Status != domain.CardiogenicStatusFurosemideO2CageStaged {
		t.Errorf("expected staged status, got %q", stagedFeline.Status)
	}
	// Feline 2.0 mg/kg * 5.0 kg = 10.0 mg
	if stagedFeline.DiureticRegimen.FurosemideInitialBolusMg != 10.0 || !stagedFeline.DiureticRegimen.ThoracocentesisIndicated {
		t.Errorf("unexpected feline regimen: %+v", stagedFeline.DiureticRegimen)
	}

	// Record Diuresis Check on CE-OP-101 (also transitions from FULMINANT_PULMONARY_EDEMA_CRITICAL to FUROSEMIDE_PIMOBENDAN_O2_CAGE_STAGED)
	updated101, err := svc.RecordDiuresisCheck("CE-OP-101", domain.DiuresisTelemetryCheck{
		RestingRespiratoryRateBpm: 36,
		UrineOutputMlKgHr:         4.2,
		TfastBLinesRemaining:      2,
		SystolicBloodPressureMmHg: 116,
		CreatinineMgDl:            1.3,
		PotassiumMeqL:             3.9,
		ClinicianCallsign:         "DR-CARDIO-22",
		Notes:                     "Robust diuresis response to furosemide CRI.",
	})
	if err != nil {
		t.Fatalf("failed to record diuresis check: %v", err)
	}
	if updated101.Status != domain.CardiogenicStatusFurosemideO2CageStaged || len(updated101.DiuresisChecks) != 2 {
		t.Errorf("unexpected updated101 state: %+v", updated101)
	}

	// Issue Discharge Clearance on CE-OP-101
	cleared101, err := svc.IssueDischargeClearance("CE-OP-101", domain.CardiogenicDischargeClearance{
		FinalSleepingRespRateBpm:  22,
		FinalTfastBLinesCount:     0,
		RoomAirSpO2Percent:        97.5,
		RenalAzotemiaStable:       true,
		HomeSRRMonitoringEducated: true,
		VetLicense:                "VET-OR-55120",
		Summary:                   "Eupneic on room air, B-lines resolved, owner educated on SRR < 30 bpm.",
	})
	if err != nil {
		t.Fatalf("failed to issue discharge clearance: %v", err)
	}
	if cleared101.Status != domain.CardiogenicStatusEupneicBLineResolved || cleared101.FluidOverloadContraindicationAlert {
		t.Errorf("unexpected cleared101 state: %+v", cleared101)
	}
}

func TestCardiogenicEdemaService_ValidationErrors(t *testing.T) {
	svc := cardiogenicedema.NewCardiogenicService()

	validProfile := domain.CardiogenicPatientProfile{
		Species:                domain.CardiogenicSpeciesCanineMMVD,
		WeightKg:               10.0,
		RespiratoryRateBpm:     60,
		HeartRateBpm:           160,
		MurmurGrade:            domain.MurmurGradeVSystolicApical,
		TfastBLinesPerSite:     5,
		LaAoRatio:              2.0,
		PleuralEffusionPresent: false,
	}

	if _, err := svc.CreateOperation("", "Oliver", validProfile); err == nil {
		t.Errorf("expected error for empty incident_id")
	}
	if _, err := svc.CreateOperation("INC-1", "", validProfile); err == nil {
		t.Errorf("expected error for empty pet_name")
	}
	badProfile := validProfile
	badProfile.Species = "INVALID"
	if _, err := svc.CreateOperation("INC-1", "Oliver", badProfile); err == nil {
		t.Errorf("expected error for invalid species")
	}
	if _, err := svc.StageDiureticRegimen("CE-OP-101", badProfile); err == nil {
		t.Errorf("expected error staging invalid profile")
	}
	if _, err := svc.StageDiureticRegimen("UNKNOWN", validProfile); err == nil {
		t.Errorf("expected error staging unknown operation")
	}

	badCheck := domain.DiuresisTelemetryCheck{
		RestingRespiratoryRateBpm: 2,
	}
	if _, err := svc.RecordDiuresisCheck("CE-OP-101", badCheck); err == nil {
		t.Errorf("expected error for invalid diuresis check")
	}
	validCheck := domain.DiuresisTelemetryCheck{
		RestingRespiratoryRateBpm: 32,
		UrineOutputMlKgHr:         3.5,
		TfastBLinesRemaining:      1,
		SystolicBloodPressureMmHg: 115,
		CreatinineMgDl:            1.2,
		PotassiumMeqL:             4.0,
		ClinicianCallsign:         "DR-1",
	}
	if _, err := svc.RecordDiuresisCheck("UNKNOWN", validCheck); err == nil {
		t.Errorf("expected error for unknown op on diuresis check")
	}

	badClearance := domain.CardiogenicDischargeClearance{
		FinalSleepingRespRateBpm: 45,
	}
	if _, err := svc.IssueDischargeClearance("CE-OP-101", badClearance); err == nil {
		t.Errorf("expected error for invalid discharge clearance")
	}
	validClearance := domain.CardiogenicDischargeClearance{
		FinalSleepingRespRateBpm:  24,
		FinalTfastBLinesCount:     1,
		RoomAirSpO2Percent:        96.0,
		RenalAzotemiaStable:       true,
		HomeSRRMonitoringEducated: true,
		VetLicense:                "VET-1",
		Summary:                   "Cleared",
	}
	if _, err := svc.IssueDischargeClearance("UNKNOWN", validClearance); err == nil {
		t.Errorf("expected error for unknown op on discharge clearance")
	}

	// Domain validation edge cases
	if domain.CardiogenicEdemaStatus("BAD").IsValid() {
		t.Errorf("expected BAD status to be invalid")
	}
	if err := domain.CardiogenicEdemaStatus("BAD").Validate(); err == nil {
		t.Errorf("expected error validating BAD status")
	}
}
