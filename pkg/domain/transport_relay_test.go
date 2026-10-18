package domain_test

import (
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func validSampleTransportRelayMission() domain.TransportRelayMission {
	return domain.TransportRelayMission{
		MissionID:            "relay-msn-101",
		PetID:                "pet-2026-001",
		PetName:              "Winston",
		Species:              "DOG",
		OriginHub:            "KBOI Boise Air Terminal FBO",
		DestinationHub:       "KBFI Boeing Field Seattle",
		TransportMode:        "GENERAL_AVIATION_AIRLIFT",
		TotalDistanceMiles:   410,
		CruiseAltitudeFeet:   8500,
		BrachycephalicFlag:   true,
		TransportStressScore: 100,
		ThermalRisk:          domain.ThermalRiskBrachycephalicHypoxia,
		RelayStatus:          domain.RelayStatusPreFlightStaging,
		HypoxiaCabinAlert:    true,
		USDACVICertified:     false,
		IATACrateVerified:    false,
		FlightReadinessScore: 0,
		RelayLegChecks: []domain.TransportRelayLegCheck{
			{
				LegID:     "leg-cvi-interstate",
				Title:     "Verify USDA APHIS Form 7001 Certificate of Veterinary Inspection (CVI) & Rabies titer",
				Category:  "USDA_APHIS_CVI_CLEARANCE",
				Completed: false,
			},
		},
		CreatedAt: "2026-10-18T12:00:00Z",
		UpdatedAt: "2026-10-18T12:00:00Z",
	}
}

func TestTransportRelayMission_Validate(t *testing.T) {
	base := validSampleTransportRelayMission()
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid sample mission, got error: %v", err)
	}

	if !domain.RelayStatusPreFlightStaging.IsValid() ||
		!domain.RelayStatusInTransitActiveRelay.IsValid() ||
		!domain.RelayStatusDeliveredDestination.IsValid() {
		t.Errorf("expected canonical TransportRelayStatus values to be valid")
	}
	if domain.TransportRelayStatus("UNKNOWN").IsValid() {
		t.Errorf("expected unknown TransportRelayStatus to be invalid")
	}

	if !domain.ThermalRiskBrachycephalicHypoxia.IsValid() ||
		!domain.ThermalRiskTarmacExcursionWatch.IsValid() ||
		!domain.ThermalRiskPressurizedSafeEnvelope.IsValid() {
		t.Errorf("expected canonical TransportThermalRisk values to be valid")
	}
	if domain.TransportThermalRisk("UNKNOWN").IsValid() {
		t.Errorf("expected unknown TransportThermalRisk to be invalid")
	}

	tests := []struct {
		name   string
		mutate func(*domain.TransportRelayMission)
	}{
		{
			name:   "empty missionId",
			mutate: func(m *domain.TransportRelayMission) { m.MissionID = "" },
		},
		{
			name:   "empty petId",
			mutate: func(m *domain.TransportRelayMission) { m.PetID = "" },
		},
		{
			name:   "empty petName",
			mutate: func(m *domain.TransportRelayMission) { m.PetName = "" },
		},
		{
			name:   "invalid species",
			mutate: func(m *domain.TransportRelayMission) { m.Species = "BIRD" },
		},
		{
			name:   "empty originHub",
			mutate: func(m *domain.TransportRelayMission) { m.OriginHub = "" },
		},
		{
			name:   "empty destinationHub",
			mutate: func(m *domain.TransportRelayMission) { m.DestinationHub = "" },
		},
		{
			name:   "invalid transportMode",
			mutate: func(m *domain.TransportRelayMission) { m.TransportMode = "CARGO_SHIP" },
		},
		{
			name:   "totalDistanceMiles too low",
			mutate: func(m *domain.TransportRelayMission) { m.TotalDistanceMiles = 5 },
		},
		{
			name:   "totalDistanceMiles too high",
			mutate: func(m *domain.TransportRelayMission) { m.TotalDistanceMiles = 4000 },
		},
		{
			name:   "cruiseAltitudeFeet negative",
			mutate: func(m *domain.TransportRelayMission) { m.CruiseAltitudeFeet = -100 },
		},
		{
			name:   "cruiseAltitudeFeet too high",
			mutate: func(m *domain.TransportRelayMission) { m.CruiseAltitudeFeet = 15000 },
		},
		{
			name:   "transportStressScore out of range",
			mutate: func(m *domain.TransportRelayMission) { m.TransportStressScore = 105 },
		},
		{
			name:   "invalid thermalRisk",
			mutate: func(m *domain.TransportRelayMission) { m.ThermalRisk = "BAD_RISK" },
		},
		{
			name:   "invalid relayStatus",
			mutate: func(m *domain.TransportRelayMission) { m.RelayStatus = "BAD_STATUS" },
		},
		{
			name:   "flightReadinessScore out of range",
			mutate: func(m *domain.TransportRelayMission) { m.FlightReadinessScore = -1 },
		},
		{
			name:   "empty relayLegChecks",
			mutate: func(m *domain.TransportRelayMission) { m.RelayLegChecks = nil },
		},
		{
			name: "invalid leg check category",
			mutate: func(m *domain.TransportRelayMission) {
				m.RelayLegChecks[0].Category = "BAD_CAT"
			},
		},
		{
			name: "delivered destination without manifestCompletionCode",
			mutate: func(m *domain.TransportRelayMission) {
				m.RelayStatus = domain.RelayStatusDeliveredDestination
				m.ManifestCompletionCode = ""
			},
		},
		{
			name:   "empty createdAt",
			mutate: func(m *domain.TransportRelayMission) { m.CreatedAt = "" },
		},
		{
			name:   "empty updatedAt",
			mutate: func(m *domain.TransportRelayMission) { m.UpdatedAt = "" },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			candidate := validSampleTransportRelayMission()
			tc.mutate(&candidate)
			if err := candidate.Validate(); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}
