package transportrelay

import (
	"fmt"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

type CreateRelayMissionInput struct {
	MissionID          string
	PetID              string
	PetName            string
	Species            string
	OriginHub          string
	DestinationHub     string
	TransportMode      string
	TotalDistanceMiles int
	CruiseAltitudeFeet int
	BrachycephalicFlag bool
	USDACVICertified   bool
	IATACrateVerified  bool
}

type RelayMissionUpdateInput struct {
	ClearPreFlightAndLaunch bool
	CompleteRelayDelivery   bool
	CompleteLegID           string
	USDACVICertified        *bool
	IATACrateVerified       *bool
}

func PlanTransportRelayMission(input CreateRelayMissionInput, now time.Time) (*domain.TransportRelayMission, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	missionID := strings.TrimSpace(input.MissionID)
	petID := strings.TrimSpace(input.PetID)
	petName := strings.TrimSpace(input.PetName)
	originHub := strings.TrimSpace(input.OriginHub)
	destinationHub := strings.TrimSpace(input.DestinationHub)

	if missionID == "" {
		return nil, fmt.Errorf("missionId is required")
	}
	if petID == "" {
		return nil, fmt.Errorf("petId is required")
	}
	if petName == "" {
		return nil, fmt.Errorf("petName is required")
	}
	if originHub == "" {
		return nil, fmt.Errorf("originHub is required")
	}
	if destinationHub == "" {
		return nil, fmt.Errorf("destinationHub is required")
	}
	if input.TotalDistanceMiles < 10 || input.TotalDistanceMiles > 3500 {
		return nil, fmt.Errorf("totalDistanceMiles must be in [10, 3500], got %d", input.TotalDistanceMiles)
	}
	if input.CruiseAltitudeFeet < 0 || input.CruiseAltitudeFeet > 12500 {
		return nil, fmt.Errorf("cruiseAltitudeFeet must be in [0, 12500], got %d", input.CruiseAltitudeFeet)
	}

	species := strings.ToUpper(strings.TrimSpace(input.Species))
	if species == "" {
		species = "DOG"
	}

	transportMode := strings.ToUpper(strings.TrimSpace(input.TransportMode))
	if transportMode == "" {
		transportMode = "GENERAL_AVIATION_AIRLIFT"
	}

	legs := []domain.TransportRelayLegCheck{
		{
			LegID:     "leg-cvi-interstate",
			Title:     "Verify USDA APHIS Form 7001 Certificate of Veterinary Inspection (CVI) & Rabies titer",
			Category:  "USDA_APHIS_CVI_CLEARANCE",
			Completed: false,
		},
		{
			LegID:     "leg-iata-crate",
			Title:     "Confirm IATA LAR Live-Animal rigid crate tie-down & dual spill-proof hydration troughs",
			Category:  "CABIN_PRESSURE_THERMAL",
			Completed: false,
		},
		{
			LegID:     "leg-cabin-oximetry",
			Title:     "Enforce <= 6,500 ft pressurized cabin ceiling & continuous Bluetooth ambient temp telemetry",
			Category:  "CABIN_PRESSURE_THERMAL",
			Completed: false,
		},
		{
			LegID:     "leg-fbo-handoff",
			Title:     "Execute tarmac FBO / ground-van dual-leash transfer inside enclosed airlock zone",
			Category:  "RELAY_HANDOFF_CHAIN",
			Completed: false,
		},
	}

	mission := &domain.TransportRelayMission{
		MissionID:          missionID,
		PetID:              petID,
		PetName:            petName,
		Species:            species,
		OriginHub:          originHub,
		DestinationHub:     destinationHub,
		TransportMode:      transportMode,
		TotalDistanceMiles: input.TotalDistanceMiles,
		CruiseAltitudeFeet: input.CruiseAltitudeFeet,
		BrachycephalicFlag: input.BrachycephalicFlag,
		USDACVICertified:   input.USDACVICertified,
		IATACrateVerified:  input.IATACrateVerified,
		RelayLegChecks:     legs,
		CreatedAt:          nowStr,
		UpdatedAt:          nowStr,
	}

	RecomputeRelayMetrics(mission, now)

	if err := mission.Validate(); err != nil {
		return nil, err
	}

	return mission, nil
}

func RecomputeRelayMetrics(mission *domain.TransportRelayMission, now time.Time) {
	if mission == nil {
		return
	}
	_ = now

	mission.HypoxiaCabinAlert = mission.BrachycephalicFlag || mission.CruiseAltitudeFeet > 7000

	readiness := 0
	if mission.USDACVICertified {
		readiness += 50
	}
	if mission.IATACrateVerified {
		readiness += 50
	}
	mission.FlightReadinessScore = readiness

	totalLegs := len(mission.RelayLegChecks)
	completedLegs := 0
	for _, leg := range mission.RelayLegChecks {
		if leg.Completed {
			completedLegs++
		}
	}

	baseStress := 25
	switch mission.TransportMode {
	case "GENERAL_AVIATION_AIRLIFT":
		baseStress = 45
	case "HYBRID_AIR_GROUND_CORRIDOR":
		baseStress = 40
	case "CLIMATE_CONTROLLED_VAN_RELAY":
		baseStress = 25
	}

	brachyPenalty := 0
	if mission.BrachycephalicFlag {
		brachyPenalty = 25
	}

	altPenalty := 0
	if mission.CruiseAltitudeFeet > 7000 {
		altPenalty = 20
	} else if mission.CruiseAltitudeFeet > 4000 {
		altPenalty = 10
	}

	distPenalty := 0
	if mission.TotalDistanceMiles >= 300 {
		distPenalty = 10
	}

	stress := baseStress + brachyPenalty + altPenalty + distPenalty - (completedLegs * 15) - (readiness / 4)
	if stress < 5 {
		stress = 5
	}
	if stress > 100 {
		stress = 100
	}
	mission.TransportStressScore = stress

	if totalLegs > 0 && completedLegs == totalLegs && mission.USDACVICertified && mission.IATACrateVerified {
		mission.RelayStatus = domain.RelayStatusDeliveredDestination
		mission.ThermalRisk = domain.ThermalRiskPressurizedSafeEnvelope
		mission.TransportStressScore = 5
		if mission.ManifestCompletionCode == "" {
			mission.ManifestCompletionCode = "RELAY-MANIFEST-" + strings.ToUpper(mission.MissionID)
		}
	} else if completedLegs >= 1 || (mission.USDACVICertified && mission.IATACrateVerified) {
		mission.RelayStatus = domain.RelayStatusInTransitActiveRelay
		if mission.TransportStressScore >= 70 {
			mission.ThermalRisk = domain.ThermalRiskBrachycephalicHypoxia
		} else {
			mission.ThermalRisk = domain.ThermalRiskTarmacExcursionWatch
		}
	} else {
		mission.RelayStatus = domain.RelayStatusPreFlightStaging
		if mission.HypoxiaCabinAlert || mission.TransportStressScore >= 65 {
			mission.ThermalRisk = domain.ThermalRiskBrachycephalicHypoxia
		} else {
			mission.ThermalRisk = domain.ThermalRiskTarmacExcursionWatch
		}
	}
}

func ApplyRelayMissionUpdate(mission *domain.TransportRelayMission, input RelayMissionUpdateInput, now time.Time) error {
	if mission == nil {
		return fmt.Errorf("mission is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	nowStr := now.Format(time.RFC3339)

	if input.USDACVICertified != nil {
		mission.USDACVICertified = *input.USDACVICertified
	}
	if input.IATACrateVerified != nil {
		mission.IATACrateVerified = *input.IATACrateVerified
	}

	if legID := strings.TrimSpace(input.CompleteLegID); legID != "" {
		found := false
		for i := range mission.RelayLegChecks {
			if mission.RelayLegChecks[i].LegID == legID {
				mission.RelayLegChecks[i].Completed = true
				if mission.RelayLegChecks[i].CompletedAt == "" {
					mission.RelayLegChecks[i].CompletedAt = nowStr
				}
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("relay leg check %q not found", legID)
		}
	}

	if input.ClearPreFlightAndLaunch {
		for i := range mission.RelayLegChecks {
			if i < 2 {
				mission.RelayLegChecks[i].Completed = true
				if mission.RelayLegChecks[i].CompletedAt == "" {
					mission.RelayLegChecks[i].CompletedAt = nowStr
				}
			}
		}
		mission.USDACVICertified = true
		mission.IATACrateVerified = true
	}

	if input.CompleteRelayDelivery {
		for i := range mission.RelayLegChecks {
			mission.RelayLegChecks[i].Completed = true
			if mission.RelayLegChecks[i].CompletedAt == "" {
				mission.RelayLegChecks[i].CompletedAt = nowStr
			}
		}
		mission.USDACVICertified = true
		mission.IATACrateVerified = true
	}

	RecomputeRelayMetrics(mission, now)
	mission.UpdatedAt = nowStr

	return mission.Validate()
}
