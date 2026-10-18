package domain

import (
	"testing"
)

func validMunicipalCADIncident() MunicipalCADIncident {
	return MunicipalCADIncident{
		IncidentID:                "cad-inc-101",
		CADCallNumber:             "311-SEA-8841",
		PetID:                     "pet-2026-001",
		PetName:                   "Cooper",
		Species:                   "DOG",
		RoadwayName:               "I-5 Express Lanes Milepost 167 Median",
		SpeedLimitMPH:             60,
		LaneCount:                 4,
		PetBehaviorState:          "BOLTING_INTO_TRAFFIC",
		TrafficStrikeRiskScore:    91,
		HazardSeverity:            RoadwayHazardFreewayCritical,
		DispatchStatus:            CADStatusInbound311Triage,
		DoNotChaseAlert:           true,
		DOTSignAdvisoryActive:     false,
		ACOUnitAssigned:           "",
		ContainmentReadinessScore: 0,
		ProtocolSteps: []CADContainmentProtocolStep{
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
		},
		CreatedAt: "2026-10-18T12:00:00Z",
		UpdatedAt: "2026-10-18T12:00:00Z",
	}
}

func TestMunicipalCADIncidentValidate(t *testing.T) {
	t.Run("valid inbound 311 triage incident", func(t *testing.T) {
		inc := validMunicipalCADIncident()
		if err := inc.Validate(); err != nil {
			t.Fatalf("expected valid incident, got error: %v", err)
		}
	})

	t.Run("valid resolved safe custody incident with resolution code", func(t *testing.T) {
		inc := validMunicipalCADIncident()
		inc.DispatchStatus = CADStatusResolvedSafeCustody
		inc.HazardSeverity = RoadwayHazardContainedSafe
		inc.TrafficStrikeRiskScore = 5
		inc.DOTSignAdvisoryActive = true
		inc.ACOUnitAssigned = "ACO-UNIT-14"
		inc.ContainmentReadinessScore = 100
		inc.CustodyResolutionCode = "CAD-SAFE-CAD-INC-101"
		if err := inc.Validate(); err != nil {
			t.Fatalf("expected valid resolved incident, got error: %v", err)
		}
	})

	t.Run("rejects invalid or missing fields", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*MunicipalCADIncident)
		}{
			{"empty incidentId", func(m *MunicipalCADIncident) { m.IncidentID = " " }},
			{"empty cadCallNumber", func(m *MunicipalCADIncident) { m.CADCallNumber = "" }},
			{"empty petId", func(m *MunicipalCADIncident) { m.PetID = "" }},
			{"empty petName", func(m *MunicipalCADIncident) { m.PetName = " " }},
			{"invalid species", func(m *MunicipalCADIncident) { m.Species = "BIRD" }},
			{"empty roadwayName", func(m *MunicipalCADIncident) { m.RoadwayName = "" }},
			{"speedLimitMph too low", func(m *MunicipalCADIncident) { m.SpeedLimitMPH = 10 }},
			{"speedLimitMph too high", func(m *MunicipalCADIncident) { m.SpeedLimitMPH = 95 }},
			{"laneCount too low", func(m *MunicipalCADIncident) { m.LaneCount = 0 }},
			{"laneCount too high", func(m *MunicipalCADIncident) { m.LaneCount = 15 }},
			{"invalid petBehaviorState", func(m *MunicipalCADIncident) { m.PetBehaviorState = "SLEEPING" }},
			{"negative trafficStrikeRiskScore", func(m *MunicipalCADIncident) { m.TrafficStrikeRiskScore = -1 }},
			{"excessive trafficStrikeRiskScore", func(m *MunicipalCADIncident) { m.TrafficStrikeRiskScore = 101 }},
			{"invalid hazardSeverity", func(m *MunicipalCADIncident) { m.HazardSeverity = RoadwayHazardSeverity("UNKNOWN") }},
			{"invalid dispatchStatus", func(m *MunicipalCADIncident) { m.DispatchStatus = CADDispatchStatus("UNKNOWN") }},
			{"negative containmentReadinessScore", func(m *MunicipalCADIncident) { m.ContainmentReadinessScore = -5 }},
			{"excessive containmentReadinessScore", func(m *MunicipalCADIncident) { m.ContainmentReadinessScore = 105 }},
			{"empty protocolSteps", func(m *MunicipalCADIncident) { m.ProtocolSteps = nil }},
			{"step missing instruction", func(m *MunicipalCADIncident) { m.ProtocolSteps[0].Instruction = "" }},
			{"step invalid role", func(m *MunicipalCADIncident) { m.ProtocolSteps[0].Role = "INVALID_ROLE" }},
			{"resolved without custodyResolutionCode", func(m *MunicipalCADIncident) {
				m.DispatchStatus = CADStatusResolvedSafeCustody
				m.CustodyResolutionCode = ""
			}},
			{"empty createdAt", func(m *MunicipalCADIncident) { m.CreatedAt = "" }},
			{"empty updatedAt", func(m *MunicipalCADIncident) { m.UpdatedAt = " " }},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				inc := validMunicipalCADIncident()
				tc.mutate(&inc)
				if err := inc.Validate(); err == nil {
					t.Fatalf("expected validation error for %s, got nil", tc.name)
				}
			})
		}
	})
}
