package transportrelay_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/transportrelay"
)

func TestPlanTransportRelayMission_BrachycephalicAirlift(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	mission, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID:          "relay-msn-101",
		PetID:              "pet-2026-001",
		PetName:            "Winston",
		Species:            "DOG",
		OriginHub:          "KBOI Boise Air Terminal FBO",
		DestinationHub:     "KBFI Boeing Field Seattle",
		TransportMode:      "GENERAL_AVIATION_AIRLIFT",
		TotalDistanceMiles: 410,
		CruiseAltitudeFeet: 8500,
		BrachycephalicFlag: true,
		USDACVICertified:   false,
		IATACrateVerified:  false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error planning transport relay mission: %v", err)
	}

	if mission.RelayStatus != domain.RelayStatusPreFlightStaging {
		t.Errorf("expected PRE_FLIGHT_STAGING, got %s", mission.RelayStatus)
	}
	if mission.ThermalRisk != domain.ThermalRiskBrachycephalicHypoxia {
		t.Errorf("expected BRACHYCEPHALIC_CABIN_RISK, got %s", mission.ThermalRisk)
	}
	if !mission.HypoxiaCabinAlert {
		t.Errorf("expected HypoxiaCabinAlert=true for BrachycephalicFlag=true and altitude=8500")
	}
	if mission.TransportStressScore != 100 {
		t.Errorf("expected TransportStressScore=100, got %d", mission.TransportStressScore)
	}
	if len(mission.RelayLegChecks) != 4 {
		t.Errorf("expected 4 relay leg checks, got %d", len(mission.RelayLegChecks))
	}
}

func TestApplyRelayMissionUpdate_LaunchAndDeliver(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	mission, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID:          "relay-msn-102",
		PetID:              "pet-2026-002",
		PetName:            "Saffron",
		Species:            "CAT",
		OriginHub:          "KPDX Portland Hillsboro Relay",
		DestinationHub:     "KPAE Paine Field Snohomish",
		TransportMode:      "HYBRID_AIR_GROUND_CORRIDOR",
		TotalDistanceMiles: 195,
		CruiseAltitudeFeet: 5500,
		BrachycephalicFlag: false,
		USDACVICertified:   false,
		IATACrateVerified:  false,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error planning mission: %v", err)
	}

	// 1. ClearPreFlightAndLaunch -> IN_TRANSIT_ACTIVE_RELAY
	if err := transportrelay.ApplyRelayMissionUpdate(mission, transportrelay.RelayMissionUpdateInput{
		ClearPreFlightAndLaunch: true,
	}, now.Add(time.Minute)); err != nil {
		t.Fatalf("unexpected error launching relay mission: %v", err)
	}

	if mission.RelayStatus != domain.RelayStatusInTransitActiveRelay {
		t.Errorf("expected IN_TRANSIT_ACTIVE_RELAY, got %s", mission.RelayStatus)
	}
	if mission.ThermalRisk != domain.ThermalRiskTarmacExcursionWatch {
		t.Errorf("expected TARMAC_THERMAL_WATCH, got %s", mission.ThermalRisk)
	}
	if mission.FlightReadinessScore != 100 {
		t.Errorf("expected FlightReadinessScore=100, got %d", mission.FlightReadinessScore)
	}

	// 2. Complete individual leg check
	if err := transportrelay.ApplyRelayMissionUpdate(mission, transportrelay.RelayMissionUpdateInput{
		CompleteLegID: "leg-cabin-oximetry",
	}, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing leg check: %v", err)
	}

	// 3. Unknown leg check returns error
	if err := transportrelay.ApplyRelayMissionUpdate(mission, transportrelay.RelayMissionUpdateInput{
		CompleteLegID: "unknown-leg",
	}, now.Add(3*time.Minute)); err == nil {
		t.Errorf("expected error for unknown leg ID")
	}

	// 4. CompleteRelayDelivery -> DELIVERED_DESTINATION_VERIFIED
	if err := transportrelay.ApplyRelayMissionUpdate(mission, transportrelay.RelayMissionUpdateInput{
		CompleteRelayDelivery: true,
	}, now.Add(4*time.Minute)); err != nil {
		t.Fatalf("unexpected error completing relay delivery: %v", err)
	}

	if mission.RelayStatus != domain.RelayStatusDeliveredDestination {
		t.Errorf("expected DELIVERED_DESTINATION_VERIFIED, got %s", mission.RelayStatus)
	}
	if mission.ThermalRisk != domain.ThermalRiskPressurizedSafeEnvelope {
		t.Errorf("expected PRESSURIZED_SAFE_ENVELOPE, got %s", mission.ThermalRisk)
	}
	if mission.TransportStressScore != 5 {
		t.Errorf("expected TransportStressScore=5, got %d", mission.TransportStressScore)
	}
	if mission.ManifestCompletionCode != "RELAY-MANIFEST-RELAY-MSN-102" {
		t.Errorf("expected RELAY-MANIFEST-RELAY-MSN-102, got %s", mission.ManifestCompletionCode)
	}
}

func TestPlanTransportRelayMission_ValidationErrors(t *testing.T) {
	now := time.Now().UTC()

	if _, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID: "",
		PetID:     "pet-1",
		PetName:   "Buddy",
	}, now); err == nil {
		t.Errorf("expected error for missing missionId")
	}

	if _, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID:          "relay-1",
		PetID:              "pet-1",
		PetName:            "Buddy",
		OriginHub:          "",
		DestinationHub:     "KBFI",
		TotalDistanceMiles: 200,
	}, now); err == nil {
		t.Errorf("expected error for missing originHub")
	}

	if _, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID:          "relay-1",
		PetID:              "pet-1",
		PetName:            "Buddy",
		OriginHub:          "KBOI",
		DestinationHub:     "KBFI",
		TotalDistanceMiles: 2,
	}, now); err == nil {
		t.Errorf("expected error for totalDistanceMiles out of range")
	}

	if err := transportrelay.ApplyRelayMissionUpdate(nil, transportrelay.RelayMissionUpdateInput{}, now); err == nil {
		t.Errorf("expected error for nil mission")
	}
}
