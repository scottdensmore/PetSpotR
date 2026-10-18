package municipalcad

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreateCADIncidentInput struct {
	IncidentID            string
	CADCallNumber         string
	PetID                 string
	PetName               string
	Species               string
	RoadwayName           string
	SpeedLimitMPH         int
	LaneCount             int
	PetBehaviorState      string
	ACOUnitAssigned       string
	DOTSignAdvisoryActive bool
}

type CADDispatchUpdateInput struct {
	DeployHighwayContainment bool
	ResolveSafeCustody       bool
	CompleteStepID           string
	DOTSignAdvisoryActive    *bool
	ACOUnitAssigned          string
}

func IngestMunicipal311Call(input CreateCADIncidentInput, now time.Time) (*domain.MunicipalCADIncident, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	incidentID := strings.TrimSpace(input.IncidentID)
	petID := strings.TrimSpace(input.PetID)
	petName := strings.TrimSpace(input.PetName)
	roadwayName := strings.TrimSpace(input.RoadwayName)

	if incidentID == "" {
		return nil, fmt.Errorf("incidentId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return nil, fmt.Errorf("petName is required")
	}
	if roadwayName == "" {
		return nil, fmt.Errorf("roadwayName is required")
	}
	if input.SpeedLimitMPH < 15 || input.SpeedLimitMPH > 85 {
		return nil, fmt.Errorf("speedLimitMph must be in [15, 85], got %d", input.SpeedLimitMPH)
	}
	if input.LaneCount < 1 || input.LaneCount > 12 {
		return nil, fmt.Errorf("laneCount must be in [1, 12], got %d", input.LaneCount)
	}

	species := strings.ToUpper(strings.TrimSpace(input.Species))
	if species == "" {
		species = "DOG"
	}

	behavior := strings.ToUpper(strings.TrimSpace(input.PetBehaviorState))
	if behavior == "" {
		behavior = "HUDDLED_IN_MEDIAN"
	}

	cadCallNumber := strings.TrimSpace(input.CADCallNumber)
	if cadCallNumber == "" {
		cadCallNumber = "311-CAD-" + strings.ToUpper(incidentID)
	}

	steps := []domain.CADContainmentProtocolStep{
		{
			StepID:      "cad-step-no-chase",
			Instruction: "Broadcast Good Samaritan DO-NOT-CHASE calming posture (sit sideways, toss high-odor lures)",
			Role:        "GOOD_SAMARITAN_CALMING",
			Completed:   false,
		},
		{
			StepID:      "cad-step-dot-vms",
			Instruction: "Activate DOT Variable Message Signs (VMS): Caution Loose Animal Ahead - Reduce Speed",
			Role:        "DOT_TRAFFIC_CONTROL",
			Completed:   false,
		},
		{
			StepID:      "cad-step-l-barrier",
			Instruction: "Position ACO & Highway Patrol vehicles in L-shaped funnel toward exit ramp shoulder",
			Role:        "ACO_FIELD_INTERCEPT",
			Completed:   false,
		},
		{
			StepID:      "cad-step-slip-lead",
			Instruction: "Secure pet with double slip-lead & scan microchip inside closed ACO transport bay",
			Role:        "ACO_FIELD_INTERCEPT",
			Completed:   false,
		},
	}

	incident := &domain.MunicipalCADIncident{
		IncidentID:            incidentID,
		CADCallNumber:         cadCallNumber,
		PetID:                 petID,
		PetName:               petName,
		Species:               species,
		RoadwayName:           roadwayName,
		SpeedLimitMPH:         input.SpeedLimitMPH,
		LaneCount:             input.LaneCount,
		PetBehaviorState:      behavior,
		DOTSignAdvisoryActive: input.DOTSignAdvisoryActive,
		ACOUnitAssigned:       strings.TrimSpace(input.ACOUnitAssigned),
		ProtocolSteps:         steps,
		CreatedAt:             nowStr,
		UpdatedAt:             nowStr,
	}

	RecomputeCADMetrics(incident, now)

	if err := incident.Validate(); err != nil {
		return nil, err
	}

	return incident, nil
}

func RecomputeCADMetrics(incident *domain.MunicipalCADIncident, now time.Time) {
	if incident == nil {
		return
	}
	if !now.IsZero() {
		incident.UpdatedAt = now.UTC().Format(time.RFC3339)
	}

	incident.DoNotChaseAlert = incident.SpeedLimitMPH >= 45 ||
		incident.PetBehaviorState == "BOLTING_INTO_TRAFFIC" ||
		incident.PetBehaviorState == "HUDDLED_IN_MEDIAN"

	readiness := 0
	if incident.DOTSignAdvisoryActive {
		readiness += 50
	}
	hasACO := strings.TrimSpace(incident.ACOUnitAssigned) != ""
	if hasACO {
		readiness += 50
	}
	incident.ContainmentReadinessScore = readiness

	totalSteps := len(incident.ProtocolSteps)
	completedSteps := 0
	for _, s := range incident.ProtocolSteps {
		if s.Completed {
			completedSteps++
		}
	}

	baseSpeedRisk := 20
	if incident.SpeedLimitMPH >= 55 {
		baseSpeedRisk = 50
	} else if incident.SpeedLimitMPH >= 35 {
		baseSpeedRisk = 35
	}

	lanePenalty := incident.LaneCount * 4
	if lanePenalty > 20 {
		lanePenalty = 20
	}

	behaviorPenalty := 10
	switch incident.PetBehaviorState {
	case "BOLTING_INTO_TRAFFIC":
		behaviorPenalty = 25
	case "HUDDLED_IN_MEDIAN":
		behaviorPenalty = 15
	case "TROTTING_SHOULDER":
		behaviorPenalty = 10
	}

	risk := baseSpeedRisk + lanePenalty + behaviorPenalty - (completedSteps * 15) - (readiness / 4)
	if risk < 5 {
		risk = 5
	}
	if risk > 100 {
		risk = 100
	}
	incident.TrafficStrikeRiskScore = risk

	if totalSteps > 0 && completedSteps == totalSteps && incident.DOTSignAdvisoryActive && hasACO {
		incident.DispatchStatus = domain.CADStatusResolvedSafeCustody
		incident.HazardSeverity = domain.RoadwayHazardContainedSafe
		incident.TrafficStrikeRiskScore = 5
		if strings.TrimSpace(incident.CustodyResolutionCode) == "" {
			incident.CustodyResolutionCode = "CAD-SAFE-" + strings.ToUpper(incident.IncidentID)
		}
	} else if completedSteps >= 1 || (incident.DOTSignAdvisoryActive && hasACO) {
		incident.DispatchStatus = domain.CADStatusHighwayContainment
		if incident.TrafficStrikeRiskScore >= 70 {
			incident.HazardSeverity = domain.RoadwayHazardFreewayCritical
		} else {
			incident.HazardSeverity = domain.RoadwayHazardArterialHigh
		}
	} else {
		incident.DispatchStatus = domain.CADStatusInbound311Triage
		if incident.TrafficStrikeRiskScore >= 65 || incident.SpeedLimitMPH >= 55 {
			incident.HazardSeverity = domain.RoadwayHazardFreewayCritical
		} else {
			incident.HazardSeverity = domain.RoadwayHazardArterialHigh
		}
	}
}

func ApplyCADDispatchAction(incident *domain.MunicipalCADIncident, input CADDispatchUpdateInput, now time.Time) error {
	if incident == nil {
		return fmt.Errorf("incident is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	if input.DOTSignAdvisoryActive != nil {
		incident.DOTSignAdvisoryActive = *input.DOTSignAdvisoryActive
	}
	if aco := strings.TrimSpace(input.ACOUnitAssigned); aco != "" {
		incident.ACOUnitAssigned = aco
	}

	if stepID := strings.TrimSpace(input.CompleteStepID); stepID != "" {
		found := false
		for i := range incident.ProtocolSteps {
			if incident.ProtocolSteps[i].StepID == stepID {
				incident.ProtocolSteps[i].Completed = true
				incident.ProtocolSteps[i].CompletedAt = nowStr
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("protocol step %q not found", stepID)
		}
	}

	if input.DeployHighwayContainment {
		incident.DOTSignAdvisoryActive = true
		if strings.TrimSpace(incident.ACOUnitAssigned) == "" {
			incident.ACOUnitAssigned = "ACO-UNIT-14"
		}
		for i := range incident.ProtocolSteps {
			if i < 2 {
				incident.ProtocolSteps[i].Completed = true
				if incident.ProtocolSteps[i].CompletedAt == "" {
					incident.ProtocolSteps[i].CompletedAt = nowStr
				}
			}
		}
	}

	if input.ResolveSafeCustody {
		incident.DOTSignAdvisoryActive = true
		if strings.TrimSpace(incident.ACOUnitAssigned) == "" {
			incident.ACOUnitAssigned = "ACO-UNIT-14"
		}
		for i := range incident.ProtocolSteps {
			incident.ProtocolSteps[i].Completed = true
			if incident.ProtocolSteps[i].CompletedAt == "" {
				incident.ProtocolSteps[i].CompletedAt = nowStr
			}
		}
	}

	RecomputeCADMetrics(incident, now)
	incident.UpdatedAt = nowStr

	return incident.Validate()
}
