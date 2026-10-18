package domain

import (
	"fmt"
	"strings"
)

type ServiceRecoveryStatus string

const (
	ServiceStatusEmergencyEscalated      ServiceRecoveryStatus = "HANDLER_MEDICAL_ESCALATED"
	ServiceStatusTacticalInterceptStaged ServiceRecoveryStatus = "TACTICAL_VEST_INTERCEPT_STAGED"
	ServiceStatusReunitedHandlerSafe     ServiceRecoveryStatus = "REUNITED_ADA_TEAM_RESTORED"
)

func (s ServiceRecoveryStatus) IsValid() bool {
	switch s {
	case ServiceStatusEmergencyEscalated,
		ServiceStatusTacticalInterceptStaged,
		ServiceStatusReunitedHandlerSafe:
		return true
	default:
		return false
	}
}

type HandlerDependencyUrgency string

const (
	DependencyUrgencyLifeCriticalAlert HandlerDependencyUrgency = "LIFE_CRITICAL_MEDICAL_DEPENDENCY"
	DependencyUrgencyMobilityGuideHigh HandlerDependencyUrgency = "HIGH_MOBILITY_GUIDE_DEPENDENCY"
	DependencyUrgencyTeamStabilized    HandlerDependencyUrgency = "HANDLER_TEAM_STABILIZED"
)

func (u HandlerDependencyUrgency) IsValid() bool {
	switch u {
	case DependencyUrgencyLifeCriticalAlert,
		DependencyUrgencyMobilityGuideHigh,
		DependencyUrgencyTeamStabilized:
		return true
	default:
		return false
	}
}

type ServiceRecoveryStep struct {
	StepID      string `json:"stepId"`
	Title       string `json:"title"`
	Category    string `json:"category"` // "HANDLER_MEDICAL_STANDBY", "NON_CONFRONTATIONAL_VEST_PROTOCOL", "ADA_PUBLIC_ACCESS_HANDOFF"
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type ServiceAnimalCase struct {
	CaseID                     string                   `json:"caseId"`
	PetID                      string                   `json:"petId"`
	PetName                    string                   `json:"petName"`
	Species                    string                   `json:"species"` // "DOG" or "MINIATURE_HORSE"
	HandlerName                string                   `json:"handlerName"`
	ServiceTaskType            string                   `json:"serviceTaskType"`    // "DIABETIC_HYPOGLYCEMIA_ALERT", "CARDIAC_SYNC_SEIZURE_RESPONSE", "BLIND_GUIDE_NAVIGATION", "PTSD_PSYCHIATRIC_DEEP_PRESSURE"
	VestEquipmentState         string                   `json:"vestEquipmentState"` // "ADA_CAPE_ENTANGLED_HAZARD", "HIGH_VIS_WORKING_HARNESS", "COLLAR_ONLY_SEPARATED"
	HoursSeparated             int                      `json:"hoursSeparated"`     // 1 to 168
	MedicalUrgencyScore        int                      `json:"medicalUrgencyScore"`
	DependencyUrgency          HandlerDependencyUrgency `json:"dependencyUrgency"`
	RecoveryStatus             ServiceRecoveryStatus    `json:"recoveryStatus"`
	HandlerMedicalStandbyAlert bool                     `json:"handlerMedicalStandbyAlert"`
	HandlerStandbyLinked       bool                     `json:"handlerStandbyLinked"`
	RecallCueBroadcasted       bool                     `json:"recallCueBroadcasted"`
	InterceptReadinessScore    int                      `json:"interceptReadinessScore"`
	RecoverySteps              []ServiceRecoveryStep    `json:"recoverySteps"`
	ADARestorationCertificate  string                   `json:"adaRestorationCertificate,omitempty"`
	CreatedAt                  string                   `json:"createdAt"`
	UpdatedAt                  string                   `json:"updatedAt"`
}

func validServiceTaskType(task string) bool {
	switch task {
	case "DIABETIC_HYPOGLYCEMIA_ALERT",
		"CARDIAC_SYNC_SEIZURE_RESPONSE",
		"BLIND_GUIDE_NAVIGATION",
		"PTSD_PSYCHIATRIC_DEEP_PRESSURE":
		return true
	default:
		return false
	}
}

func validVestEquipmentState(state string) bool {
	switch state {
	case "ADA_CAPE_ENTANGLED_HAZARD",
		"HIGH_VIS_WORKING_HARNESS",
		"COLLAR_ONLY_SEPARATED":
		return true
	default:
		return false
	}
}

func validServiceStepCategory(cat string) bool {
	switch cat {
	case "HANDLER_MEDICAL_STANDBY",
		"NON_CONFRONTATIONAL_VEST_PROTOCOL",
		"ADA_PUBLIC_ACCESS_HANDOFF":
		return true
	default:
		return false
	}
}

// Validate checks invariants for ServiceAnimalCase.
func (c ServiceAnimalCase) Validate() error {
	if strings.TrimSpace(c.CaseID) == "" {
		return fmt.Errorf("caseId is required")
	}
	if strings.TrimSpace(c.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(c.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	species := strings.TrimSpace(c.Species)
	if species != "DOG" && species != "MINIATURE_HORSE" {
		return fmt.Errorf("species must be DOG or MINIATURE_HORSE, got %q", c.Species)
	}
	if strings.TrimSpace(c.HandlerName) == "" {
		return fmt.Errorf("handlerName is required")
	}
	if !validServiceTaskType(strings.TrimSpace(c.ServiceTaskType)) {
		return fmt.Errorf("invalid serviceTaskType: %q", c.ServiceTaskType)
	}
	if !validVestEquipmentState(strings.TrimSpace(c.VestEquipmentState)) {
		return fmt.Errorf("invalid vestEquipmentState: %q", c.VestEquipmentState)
	}
	if c.HoursSeparated < 1 || c.HoursSeparated > 168 {
		return fmt.Errorf("hoursSeparated must be in [1, 168], got %d", c.HoursSeparated)
	}
	if c.MedicalUrgencyScore < 0 || c.MedicalUrgencyScore > 100 {
		return fmt.Errorf("medicalUrgencyScore must be in [0, 100], got %d", c.MedicalUrgencyScore)
	}
	if !c.DependencyUrgency.IsValid() {
		return fmt.Errorf("invalid dependencyUrgency: %q", c.DependencyUrgency)
	}
	if !c.RecoveryStatus.IsValid() {
		return fmt.Errorf("invalid recoveryStatus: %q", c.RecoveryStatus)
	}
	if c.InterceptReadinessScore < 0 || c.InterceptReadinessScore > 100 {
		return fmt.Errorf("interceptReadinessScore must be in [0, 100], got %d", c.InterceptReadinessScore)
	}
	if len(c.RecoverySteps) < 1 {
		return fmt.Errorf("at least one recoveryStep is required")
	}
	for i, step := range c.RecoverySteps {
		if strings.TrimSpace(step.StepID) == "" ||
			strings.TrimSpace(step.Title) == "" ||
			!validServiceStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("recoverySteps[%d] must have non-empty stepId, title, and valid category", i)
		}
	}
	if c.RecoveryStatus == ServiceStatusReunitedHandlerSafe && strings.TrimSpace(c.ADARestorationCertificate) == "" {
		return fmt.Errorf("adaRestorationCertificate is required when recoveryStatus is REUNITED_ADA_TEAM_RESTORED")
	}
	if strings.TrimSpace(c.CreatedAt) == "" {
		return fmt.Errorf("createdAt is required")
	}
	if strings.TrimSpace(c.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
