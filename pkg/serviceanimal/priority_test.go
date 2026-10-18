package serviceanimal_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/serviceanimal"
)

func TestEvaluateServiceAnimalCase_DiabeticAlertDog(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	sc, err := serviceanimal.EvaluateServiceAnimalCase(serviceanimal.CreateServiceCaseInput{
		CaseID:               "sa-case-101",
		PetID:                "pet-2026-001",
		PetName:              "Apollo",
		Species:              "DOG",
		HandlerName:          "Maya Lin (T1D Handler)",
		ServiceTaskType:      "DIABETIC_HYPOGLYCEMIA_ALERT",
		VestEquipmentState:   "ADA_CAPE_ENTANGLED_HAZARD",
		HoursSeparated:       8,
		HandlerStandbyLinked: false,
		RecallCueBroadcasted: false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating service animal case: %v", err)
	}

	if sc.RecoveryStatus != domain.ServiceStatusEmergencyEscalated {
		t.Errorf("expected HANDLER_MEDICAL_ESCALATED, got %s", sc.RecoveryStatus)
	}
	if sc.DependencyUrgency != domain.DependencyUrgencyLifeCriticalAlert {
		t.Errorf("expected LIFE_CRITICAL_MEDICAL_DEPENDENCY, got %s", sc.DependencyUrgency)
	}
	if !sc.HandlerMedicalStandbyAlert {
		t.Errorf("expected HandlerMedicalStandbyAlert=true for diabetic alert dog with entangled ADA cape")
	}
	if sc.MedicalUrgencyScore != 90 {
		t.Errorf("expected MedicalUrgencyScore=90, got %d", sc.MedicalUrgencyScore)
	}
	if len(sc.RecoverySteps) != 4 {
		t.Errorf("expected 4 recovery steps, got %d", len(sc.RecoverySteps))
	}
}

func TestApplyServiceCaseUpdate_StageAndRestore(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	sc, err := serviceanimal.EvaluateServiceAnimalCase(serviceanimal.CreateServiceCaseInput{
		CaseID:               "sa-case-102",
		PetID:                "pet-2026-002",
		PetName:              "Pax",
		Species:              "DOG",
		HandlerName:          "Jordan Vance",
		ServiceTaskType:      "BLIND_GUIDE_NAVIGATION",
		VestEquipmentState:   "HIGH_VIS_WORKING_HARNESS",
		HoursSeparated:       3,
		HandlerStandbyLinked: false,
		RecallCueBroadcasted: false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error evaluating service animal case: %v", err)
	}

	// 1. StageTacticalIntercept -> TACTICAL_VEST_INTERCEPT_STAGED
	if err := serviceanimal.ApplyServiceCaseUpdate(sc, serviceanimal.ServiceCaseUpdateInput{
		StageTacticalIntercept: true,
	}, now.Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error staging tactical intercept: %v", err)
	}

	if sc.RecoveryStatus != domain.ServiceStatusTacticalInterceptStaged {
		t.Errorf("expected TACTICAL_VEST_INTERCEPT_STAGED, got %s", sc.RecoveryStatus)
	}
	if sc.DependencyUrgency != domain.DependencyUrgencyMobilityGuideHigh {
		t.Errorf("expected HIGH_MOBILITY_GUIDE_DEPENDENCY, got %s", sc.DependencyUrgency)
	}
	if sc.InterceptReadinessScore != 100 {
		t.Errorf("expected InterceptReadinessScore=100, got %d", sc.InterceptReadinessScore)
	}

	// 2. Complete individual step
	if err := serviceanimal.ApplyServiceCaseUpdate(sc, serviceanimal.ServiceCaseUpdateInput{
		CompleteStepID: "sa-step-acoustic-cue",
	}, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing step: %v", err)
	}

	// 3. Unknown step ID returns error
	if err := serviceanimal.ApplyServiceCaseUpdate(sc, serviceanimal.ServiceCaseUpdateInput{
		CompleteStepID: "unknown-step",
	}, now.Add(3*time.Minute)); err == nil {
		t.Errorf("expected error for unknown step ID")
	}

	// 4. RestoreADATeamReunion -> REUNITED_ADA_TEAM_RESTORED
	if err := serviceanimal.ApplyServiceCaseUpdate(sc, serviceanimal.ServiceCaseUpdateInput{
		RestoreADATeamReunion: true,
	}, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("unexpected error restoring ADA team reunion: %v", err)
	}

	if sc.RecoveryStatus != domain.ServiceStatusReunitedHandlerSafe {
		t.Errorf("expected REUNITED_ADA_TEAM_RESTORED, got %s", sc.RecoveryStatus)
	}
	if sc.DependencyUrgency != domain.DependencyUrgencyTeamStabilized {
		t.Errorf("expected HANDLER_TEAM_STABILIZED, got %s", sc.DependencyUrgency)
	}
	if sc.MedicalUrgencyScore != 5 {
		t.Errorf("expected MedicalUrgencyScore=5, got %d", sc.MedicalUrgencyScore)
	}
	if sc.ADARestorationCertificate != "ADA-RESTORED-SA-CASE-102" {
		t.Errorf("expected ADA-RESTORED-SA-CASE-102, got %s", sc.ADARestorationCertificate)
	}
}

func TestEvaluateServiceAnimalCase_ValidationErrors(t *testing.T) {
	now := time.Now().UTC()

	if _, err := serviceanimal.EvaluateServiceAnimalCase(serviceanimal.CreateServiceCaseInput{
		CaseID:         "",
		PetID:          "pet-1",
		PetName:        "Buddy",
		HandlerName:    "Alex",
		HoursSeparated: 2,
	}, now); err == nil {
		t.Errorf("expected error for missing caseId")
	}

	if _, err := serviceanimal.EvaluateServiceAnimalCase(serviceanimal.CreateServiceCaseInput{
		CaseID:         "sa-1",
		PetID:          "pet-1",
		PetName:        "Buddy",
		HandlerName:    "Alex",
		HoursSeparated: 0,
	}, now); err == nil {
		t.Errorf("expected error for out-of-range hoursSeparated")
	}

	if err := serviceanimal.ApplyServiceCaseUpdate(nil, serviceanimal.ServiceCaseUpdateInput{}, now); err == nil {
		t.Errorf("expected error for nil service animal case")
	}
}
