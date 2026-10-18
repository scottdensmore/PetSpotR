package domain

import (
	"fmt"
	"strings"
)

type CADDispatchStatus string

const (
	CADStatusInbound311Triage    CADDispatchStatus = "INBOUND_311_TRIAGE"
	CADStatusHighwayContainment  CADDispatchStatus = "HIGHWAY_CONTAINMENT_DISPATCHED"
	CADStatusResolvedSafeCustody CADDispatchStatus = "RESOLVED_SAFE_CUSTODY"
)

func (s CADDispatchStatus) IsValid() bool {
	switch s {
	case CADStatusInbound311Triage,
		CADStatusHighwayContainment,
		CADStatusResolvedSafeCustody:
		return true
	default:
		return false
	}
}

type RoadwayHazardSeverity string

const (
	RoadwayHazardFreewayCritical RoadwayHazardSeverity = "FREEWAY_MEDIAN_CRITICAL"
	RoadwayHazardArterialHigh    RoadwayHazardSeverity = "ARTERIAL_TRAFFIC_HIGH"
	RoadwayHazardContainedSafe   RoadwayHazardSeverity = "CONTAINED_CORRIDOR_SAFE"
)

func (h RoadwayHazardSeverity) IsValid() bool {
	switch h {
	case RoadwayHazardFreewayCritical,
		RoadwayHazardArterialHigh,
		RoadwayHazardContainedSafe:
		return true
	default:
		return false
	}
}

type CADContainmentProtocolStep struct {
	StepID      string `json:"stepId"`
	Instruction string `json:"instruction"`
	Role        string `json:"role"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type MunicipalCADIncident struct {
	IncidentID                string                       `json:"incidentId"`
	CADCallNumber             string                       `json:"cadCallNumber"`
	PetID                     string                       `json:"petId"`
	PetName                   string                       `json:"petName"`
	Species                   string                       `json:"species"`
	RoadwayName               string                       `json:"roadwayName"`
	SpeedLimitMPH             int                          `json:"speedLimitMph"`
	LaneCount                 int                          `json:"laneCount"`
	PetBehaviorState          string                       `json:"petBehaviorState"`
	TrafficStrikeRiskScore    int                          `json:"trafficStrikeRiskScore"`
	HazardSeverity            RoadwayHazardSeverity        `json:"hazardSeverity"`
	DispatchStatus            CADDispatchStatus            `json:"dispatchStatus"`
	DoNotChaseAlert           bool                         `json:"doNotChaseAlert"`
	DOTSignAdvisoryActive     bool                         `json:"dotSignAdvisoryActive"`
	ACOUnitAssigned           string                       `json:"acoUnitAssigned"`
	ContainmentReadinessScore int                          `json:"containmentReadinessScore"`
	ProtocolSteps             []CADContainmentProtocolStep `json:"protocolSteps"`
	CustodyResolutionCode     string                       `json:"custodyResolutionCode,omitempty"`
	CreatedAt                 string                       `json:"createdAt"`
	UpdatedAt                 string                       `json:"updatedAt"`
}

func ValidPetBehaviorState(state string) bool {
	switch state {
	case "BOLTING_INTO_TRAFFIC",
		"HUDDLED_IN_MEDIAN",
		"TROTTING_SHOULDER":
		return true
	default:
		return false
	}
}

func ValidCADProtocolRole(role string) bool {
	switch role {
	case "DOT_TRAFFIC_CONTROL",
		"GOOD_SAMARITAN_CALMING",
		"ACO_FIELD_INTERCEPT":
		return true
	default:
		return false
	}
}

func (m MunicipalCADIncident) Validate() error {
	if strings.TrimSpace(m.IncidentID) == "" {
		return fmt.Errorf("incidentId is required")
	}
	if strings.TrimSpace(m.CADCallNumber) == "" {
		return fmt.Errorf("cadCallNumber is required")
	}
	if strings.TrimSpace(m.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(m.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	species := strings.TrimSpace(m.Species)
	if species != "DOG" && species != "CAT" {
		return fmt.Errorf("species must be DOG or CAT, got %q", m.Species)
	}
	if strings.TrimSpace(m.RoadwayName) == "" {
		return fmt.Errorf("roadwayName is required")
	}
	if m.SpeedLimitMPH < 15 || m.SpeedLimitMPH > 85 {
		return fmt.Errorf("speedLimitMph must be in [15, 85], got %d", m.SpeedLimitMPH)
	}
	if m.LaneCount < 1 || m.LaneCount > 12 {
		return fmt.Errorf("laneCount must be in [1, 12], got %d", m.LaneCount)
	}
	if !ValidPetBehaviorState(strings.TrimSpace(m.PetBehaviorState)) {
		return fmt.Errorf("invalid petBehaviorState: %q", m.PetBehaviorState)
	}
	if m.TrafficStrikeRiskScore < 0 || m.TrafficStrikeRiskScore > 100 {
		return fmt.Errorf("trafficStrikeRiskScore must be in [0, 100], got %d", m.TrafficStrikeRiskScore)
	}
	if !m.HazardSeverity.IsValid() {
		return fmt.Errorf("invalid hazardSeverity: %q", m.HazardSeverity)
	}
	if !m.DispatchStatus.IsValid() {
		return fmt.Errorf("invalid dispatchStatus: %q", m.DispatchStatus)
	}
	if m.ContainmentReadinessScore < 0 || m.ContainmentReadinessScore > 100 {
		return fmt.Errorf("containmentReadinessScore must be in [0, 100], got %d", m.ContainmentReadinessScore)
	}
	if len(m.ProtocolSteps) < 1 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range m.ProtocolSteps {
		if strings.TrimSpace(step.StepID) == "" ||
			strings.TrimSpace(step.Instruction) == "" ||
			!ValidCADProtocolRole(strings.TrimSpace(step.Role)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty stepId, instruction, and valid role", i)
		}
	}
	if m.DispatchStatus == CADStatusResolvedSafeCustody && strings.TrimSpace(m.CustodyResolutionCode) == "" {
		return fmt.Errorf("custodyResolutionCode is required when dispatchStatus is RESOLVED_SAFE_CUSTODY")
	}
	if strings.TrimSpace(m.CreatedAt) == "" {
		return fmt.Errorf("createdAt is required")
	}
	if strings.TrimSpace(m.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
