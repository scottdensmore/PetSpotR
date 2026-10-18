package serviceanimal

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreateServiceCaseInput struct {
	CaseID               string
	PetID                string
	PetName              string
	Species              string
	HandlerName          string
	ServiceTaskType      string
	VestEquipmentState   string
	HoursSeparated       int
	HandlerStandbyLinked bool
	RecallCueBroadcasted bool
}

type ServiceCaseUpdateInput struct {
	StageTacticalIntercept bool
	RestoreADATeamReunion  bool
	CompleteStepID         string
	HandlerStandbyLinked   *bool
	RecallCueBroadcasted   *bool
}

func EvaluateServiceAnimalCase(input CreateServiceCaseInput, now time.Time) (*domain.ServiceAnimalCase, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	caseID := strings.TrimSpace(input.CaseID)
	petID := strings.TrimSpace(input.PetID)
	petName := strings.TrimSpace(input.PetName)
	handlerName := strings.TrimSpace(input.HandlerName)

	if caseID == "" {
		return nil, fmt.Errorf("caseId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return nil, fmt.Errorf("petName is required")
	}
	if handlerName == "" {
		return nil, fmt.Errorf("handlerName is required")
	}
	if input.HoursSeparated < 1 || input.HoursSeparated > 168 {
		return nil, fmt.Errorf("hoursSeparated must be in [1, 168], got %d", input.HoursSeparated)
	}

	species := strings.ToUpper(strings.TrimSpace(input.Species))
	if species == "" {
		species = "DOG"
	}

	serviceTaskType := strings.ToUpper(strings.TrimSpace(input.ServiceTaskType))
	if serviceTaskType == "" {
		serviceTaskType = "DIABETIC_HYPOGLYCEMIA_ALERT"
	}

	vestEquipmentState := strings.ToUpper(strings.TrimSpace(input.VestEquipmentState))
	if vestEquipmentState == "" {
		vestEquipmentState = "ADA_CAPE_ENTANGLED_HAZARD"
	}

	steps := []domain.ServiceRecoveryStep{
		{
			StepID:    "sa-step-ems-standby",
			Title:     "Link handler to paramedic/CGM telemetry standby during working-animal separation",
			Category:  "HANDLER_MEDICAL_STANDBY",
			Completed: false,
		},
		{
			StepID:    "sa-step-cape-snag-sweep",
			Title:     "Sweep brush & fence lines for entangled ADA cape/rigid guide handle snags",
			Category:  "NON_CONFRONTATIONAL_VEST_PROTOCOL",
			Completed: false,
		},
		{
			StepID:    "sa-step-acoustic-cue",
			Title:     "Broadcast handler-recorded conditioned acoustic recall whistle/clicker sequence",
			Category:  "NON_CONFRONTATIONAL_VEST_PROTOCOL",
			Completed: false,
		},
		{
			StepID:    "sa-step-ada-reunion",
			Title:     "Verify task-cue responsiveness & restore protected ADA handler-animal team",
			Category:  "ADA_PUBLIC_ACCESS_HANDOFF",
			Completed: false,
		},
	}

	serviceCase := &domain.ServiceAnimalCase{
		CaseID:               caseID,
		PetID:                petID,
		PetName:              petName,
		Species:              species,
		HandlerName:          handlerName,
		ServiceTaskType:      serviceTaskType,
		VestEquipmentState:   vestEquipmentState,
		HoursSeparated:       input.HoursSeparated,
		HandlerStandbyLinked: input.HandlerStandbyLinked,
		RecallCueBroadcasted: input.RecallCueBroadcasted,
		RecoverySteps:        steps,
		CreatedAt:            nowStr,
		UpdatedAt:            nowStr,
	}

	RecomputeServiceMetrics(serviceCase, now)

	if err := serviceCase.Validate(); err != nil {
		return nil, err
	}

	return serviceCase, nil
}

func RecomputeServiceMetrics(sc *domain.ServiceAnimalCase, now time.Time) {
	if sc == nil {
		return
	}
	_ = now

	sc.HandlerMedicalStandbyAlert = sc.ServiceTaskType == "DIABETIC_HYPOGLYCEMIA_ALERT" ||
		sc.ServiceTaskType == "CARDIAC_SYNC_SEIZURE_RESPONSE" ||
		sc.VestEquipmentState == "ADA_CAPE_ENTANGLED_HAZARD"

	readiness := 0
	if sc.HandlerStandbyLinked {
		readiness += 50
	}
	if sc.RecallCueBroadcasted {
		readiness += 50
	}
	sc.InterceptReadinessScore = readiness

	totalSteps := len(sc.RecoverySteps)
	completedSteps := 0
	for _, step := range sc.RecoverySteps {
		if step.Completed {
			completedSteps++
		}
	}

	baseUrgency := 40
	switch sc.ServiceTaskType {
	case "DIABETIC_HYPOGLYCEMIA_ALERT":
		baseUrgency = 55
	case "CARDIAC_SYNC_SEIZURE_RESPONSE":
		baseUrgency = 55
	case "BLIND_GUIDE_NAVIGATION":
		baseUrgency = 45
	case "PTSD_PSYCHIATRIC_DEEP_PRESSURE":
		baseUrgency = 40
	}

	equipmentPenalty := 0
	switch sc.VestEquipmentState {
	case "ADA_CAPE_ENTANGLED_HAZARD":
		equipmentPenalty = 20
	case "HIGH_VIS_WORKING_HARNESS":
		equipmentPenalty = 10
	case "COLLAR_ONLY_SEPARATED":
		equipmentPenalty = 15
	}

	separationPenalty := 0
	if sc.HoursSeparated >= 6 {
		separationPenalty = 15
	} else if sc.HoursSeparated >= 2 {
		separationPenalty = 10
	}

	urgency := baseUrgency + equipmentPenalty + separationPenalty - (completedSteps * 15) - (readiness / 4)
	if urgency < 5 {
		urgency = 5
	}
	if urgency > 100 {
		urgency = 100
	}
	sc.MedicalUrgencyScore = urgency

	if totalSteps > 0 && completedSteps == totalSteps && sc.HandlerStandbyLinked && sc.RecallCueBroadcasted {
		sc.RecoveryStatus = domain.ServiceStatusReunitedHandlerSafe
		sc.DependencyUrgency = domain.DependencyUrgencyTeamStabilized
		sc.MedicalUrgencyScore = 5
		if sc.ADARestorationCertificate == "" {
			sc.ADARestorationCertificate = "ADA-RESTORED-" + strings.ToUpper(sc.CaseID)
		}
	} else if completedSteps >= 1 || (sc.HandlerStandbyLinked && sc.RecallCueBroadcasted) {
		sc.RecoveryStatus = domain.ServiceStatusTacticalInterceptStaged
		if sc.MedicalUrgencyScore >= 70 {
			sc.DependencyUrgency = domain.DependencyUrgencyLifeCriticalAlert
		} else {
			sc.DependencyUrgency = domain.DependencyUrgencyMobilityGuideHigh
		}
	} else {
		sc.RecoveryStatus = domain.ServiceStatusEmergencyEscalated
		if sc.HandlerMedicalStandbyAlert || sc.MedicalUrgencyScore >= 65 {
			sc.DependencyUrgency = domain.DependencyUrgencyLifeCriticalAlert
		} else {
			sc.DependencyUrgency = domain.DependencyUrgencyMobilityGuideHigh
		}
	}
}

func ApplyServiceCaseUpdate(sc *domain.ServiceAnimalCase, input ServiceCaseUpdateInput, now time.Time) error {
	if sc == nil {
		return fmt.Errorf("service animal case is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	if input.HandlerStandbyLinked != nil {
		sc.HandlerStandbyLinked = *input.HandlerStandbyLinked
	}
	if input.RecallCueBroadcasted != nil {
		sc.RecallCueBroadcasted = *input.RecallCueBroadcasted
	}

	if stepID := strings.TrimSpace(input.CompleteStepID); stepID != "" {
		found := false
		for i := range sc.RecoverySteps {
			if sc.RecoverySteps[i].StepID == stepID {
				sc.RecoverySteps[i].Completed = true
				if sc.RecoverySteps[i].CompletedAt == "" {
					sc.RecoverySteps[i].CompletedAt = nowStr
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("recovery step %q not found", stepID)
		}
	}

	if input.StageTacticalIntercept {
		for i := range sc.RecoverySteps {
			if i < 2 {
				sc.RecoverySteps[i].Completed = true
				if sc.RecoverySteps[i].CompletedAt == "" {
					sc.RecoverySteps[i].CompletedAt = nowStr
				}
			}
		}
		sc.HandlerStandbyLinked = true
		sc.RecallCueBroadcasted = true
	}

	if input.RestoreADATeamReunion {
		for i := range sc.RecoverySteps {
			sc.RecoverySteps[i].Completed = true
			if sc.RecoverySteps[i].CompletedAt == "" {
				sc.RecoverySteps[i].CompletedAt = nowStr
			}
		}
		sc.HandlerStandbyLinked = true
		sc.RecallCueBroadcasted = true
	}

	RecomputeServiceMetrics(sc, now)
	sc.UpdatedAt = nowStr

	return sc.Validate()
}
