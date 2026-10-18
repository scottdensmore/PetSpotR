package domain_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func validSampleServiceAnimalCase() domain.ServiceAnimalCase {
	return domain.ServiceAnimalCase{
		CaseID:                     "sa-case-101",
		PetID:                      "pet-2026-001",
		PetName:                    "Apollo",
		Species:                    "DOG",
		HandlerName:                "Maya Lin (T1D Handler)",
		ServiceTaskType:            "DIABETIC_HYPOGLYCEMIA_ALERT",
		VestEquipmentState:         "ADA_CAPE_ENTANGLED_HAZARD",
		HoursSeparated:             8,
		MedicalUrgencyScore:        90,
		DependencyUrgency:          domain.DependencyUrgencyLifeCriticalAlert,
		RecoveryStatus:             domain.ServiceStatusEmergencyEscalated,
		HandlerMedicalStandbyAlert: true,
		HandlerStandbyLinked:       false,
		RecallCueBroadcasted:       false,
		InterceptReadinessScore:    0,
		RecoverySteps: []domain.ServiceRecoveryStep{
			{
				StepID:    "sa-step-ems-standby",
				Title:     "Link handler to paramedic/CGM telemetry standby during working-animal separation",
				Category:  "HANDLER_MEDICAL_STANDBY",
				Completed: false,
			},
		},
		CreatedAt: "2026-10-18T12:00:00Z",
		UpdatedAt: "2026-10-18T12:00:00Z",
	}
}

func TestServiceAnimalCase_Validate(t *testing.T) {
	base := validSampleServiceAnimalCase()
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid sample service animal case, got error: %v", err)
	}

	if !domain.ServiceStatusEmergencyEscalated.IsValid() ||
		!domain.ServiceStatusTacticalInterceptStaged.IsValid() ||
		!domain.ServiceStatusReunitedHandlerSafe.IsValid() {
		t.Errorf("expected canonical ServiceRecoveryStatus values to be valid")
	}
	if domain.ServiceRecoveryStatus("UNKNOWN").IsValid() {
		t.Errorf("expected unknown ServiceRecoveryStatus to be invalid")
	}

	if !domain.DependencyUrgencyLifeCriticalAlert.IsValid() ||
		!domain.DependencyUrgencyMobilityGuideHigh.IsValid() ||
		!domain.DependencyUrgencyTeamStabilized.IsValid() {
		t.Errorf("expected canonical HandlerDependencyUrgency values to be valid")
	}
	if domain.HandlerDependencyUrgency("UNKNOWN").IsValid() {
		t.Errorf("expected unknown HandlerDependencyUrgency to be invalid")
	}

	tests := []struct {
		name   string
		mutate func(*domain.ServiceAnimalCase)
	}{
		{
			name:   "empty caseId",
			mutate: func(c *domain.ServiceAnimalCase) { c.CaseID = "" },
		},
		{
			name:   "empty petId",
			mutate: func(c *domain.ServiceAnimalCase) { c.PetID = "" },
		},
		{
			name:   "empty petName",
			mutate: func(c *domain.ServiceAnimalCase) { c.PetName = "" },
		},
		{
			name:   "invalid species",
			mutate: func(c *domain.ServiceAnimalCase) { c.Species = "CAT" },
		},
		{
			name:   "empty handlerName",
			mutate: func(c *domain.ServiceAnimalCase) { c.HandlerName = "" },
		},
		{
			name:   "invalid serviceTaskType",
			mutate: func(c *domain.ServiceAnimalCase) { c.ServiceTaskType = "UNKNOWN_TASK" },
		},
		{
			name:   "invalid vestEquipmentState",
			mutate: func(c *domain.ServiceAnimalCase) { c.VestEquipmentState = "UNKNOWN_VEST" },
		},
		{
			name:   "hoursSeparated too low",
			mutate: func(c *domain.ServiceAnimalCase) { c.HoursSeparated = 0 },
		},
		{
			name:   "hoursSeparated too high",
			mutate: func(c *domain.ServiceAnimalCase) { c.HoursSeparated = 169 },
		},
		{
			name:   "medicalUrgencyScore out of range",
			mutate: func(c *domain.ServiceAnimalCase) { c.MedicalUrgencyScore = 105 },
		},
		{
			name:   "invalid dependencyUrgency",
			mutate: func(c *domain.ServiceAnimalCase) { c.DependencyUrgency = "BAD_URGENCY" },
		},
		{
			name:   "invalid recoveryStatus",
			mutate: func(c *domain.ServiceAnimalCase) { c.RecoveryStatus = "BAD_STATUS" },
		},
		{
			name:   "interceptReadinessScore out of range",
			mutate: func(c *domain.ServiceAnimalCase) { c.InterceptReadinessScore = -5 },
		},
		{
			name:   "empty recoverySteps",
			mutate: func(c *domain.ServiceAnimalCase) { c.RecoverySteps = nil },
		},
		{
			name: "invalid step category",
			mutate: func(c *domain.ServiceAnimalCase) {
				c.RecoverySteps[0].Category = "BAD_CAT"
			},
		},
		{
			name: "reunited handler safe without ADA restoration certificate",
			mutate: func(c *domain.ServiceAnimalCase) {
				c.RecoveryStatus = domain.ServiceStatusReunitedHandlerSafe
				c.ADARestorationCertificate = ""
			},
		},
		{
			name:   "empty createdAt",
			mutate: func(c *domain.ServiceAnimalCase) { c.CreatedAt = "" },
		},
		{
			name:   "empty updatedAt",
			mutate: func(c *domain.ServiceAnimalCase) { c.UpdatedAt = "" },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validSampleServiceAnimalCase()
			tc.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}
