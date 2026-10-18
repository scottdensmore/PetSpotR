package domain

import (
	"fmt"
	"strings"
)

type TransportRelayStatus string

const (
	RelayStatusPreFlightStaging     TransportRelayStatus = "PRE_FLIGHT_STAGING"
	RelayStatusInTransitActiveRelay TransportRelayStatus = "IN_TRANSIT_ACTIVE_RELAY"
	RelayStatusDeliveredDestination TransportRelayStatus = "DELIVERED_DESTINATION_VERIFIED"
)

func (s TransportRelayStatus) IsValid() bool {
	switch s {
	case RelayStatusPreFlightStaging,
		RelayStatusInTransitActiveRelay,
		RelayStatusDeliveredDestination:
		return true
	default:
		return false
	}
}

type TransportThermalRisk string

const (
	ThermalRiskBrachycephalicHypoxia   TransportThermalRisk = "BRACHYCEPHALIC_CABIN_RISK"
	ThermalRiskTarmacExcursionWatch    TransportThermalRisk = "TARMAC_THERMAL_WATCH"
	ThermalRiskPressurizedSafeEnvelope TransportThermalRisk = "PRESSURIZED_SAFE_ENVELOPE"
)

func (r TransportThermalRisk) IsValid() bool {
	switch r {
	case ThermalRiskBrachycephalicHypoxia,
		ThermalRiskTarmacExcursionWatch,
		ThermalRiskPressurizedSafeEnvelope:
		return true
	default:
		return false
	}
}

type TransportRelayLegCheck struct {
	LegID       string `json:"legId"`
	Title       string `json:"title"`
	Category    string `json:"category"` // "USDA_APHIS_CVI_CLEARANCE", "CABIN_PRESSURE_THERMAL", "RELAY_HANDOFF_CHAIN"
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

type TransportRelayMission struct {
	MissionID              string                   `json:"missionId"`
	PetID                  string                   `json:"petId"`
	PetName                string                   `json:"petName"`
	Species                string                   `json:"species"` // "DOG" or "CAT"
	OriginHub              string                   `json:"originHub"`
	DestinationHub         string                   `json:"destinationHub"`
	TransportMode          string                   `json:"transportMode"`      // "GENERAL_AVIATION_AIRLIFT", "CLIMATE_CONTROLLED_VAN_RELAY", "HYBRID_AIR_GROUND_CORRIDOR"
	TotalDistanceMiles     int                      `json:"totalDistanceMiles"` // 10 to 3500
	CruiseAltitudeFeet     int                      `json:"cruiseAltitudeFeet"` // 0 to 12500
	BrachycephalicFlag     bool                     `json:"brachycephalicFlag"`
	TransportStressScore   int                      `json:"transportStressScore"` // 0 to 100
	ThermalRisk            TransportThermalRisk     `json:"thermalRisk"`
	RelayStatus            TransportRelayStatus     `json:"relayStatus"`
	HypoxiaCabinAlert      bool                     `json:"hypoxiaCabinAlert"`
	USDACVICertified       bool                     `json:"usdaCviCertified"`
	IATACrateVerified      bool                     `json:"iataCrateVerified"`
	FlightReadinessScore   int                      `json:"flightReadinessScore"` // 0 to 100
	RelayLegChecks         []TransportRelayLegCheck `json:"relayLegChecks"`
	ManifestCompletionCode string                   `json:"manifestCompletionCode,omitempty"`
	CreatedAt              string                   `json:"createdAt"`
	UpdatedAt              string                   `json:"updatedAt"`
}

func validTransportRelayMode(mode string) bool {
	switch mode {
	case "GENERAL_AVIATION_AIRLIFT", "CLIMATE_CONTROLLED_VAN_RELAY", "HYBRID_AIR_GROUND_CORRIDOR":
		return true
	default:
		return false
	}
}

func validTransportLegCheckCategory(cat string) bool {
	switch cat {
	case "USDA_APHIS_CVI_CLEARANCE", "CABIN_PRESSURE_THERMAL", "RELAY_HANDOFF_CHAIN":
		return true
	default:
		return false
	}
}

// Validate checks invariants for TransportRelayMission.
func (m TransportRelayMission) Validate() error {
	if strings.TrimSpace(m.MissionID) == "" {
		return fmt.Errorf("missionId is required")
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
	if strings.TrimSpace(m.OriginHub) == "" {
		return fmt.Errorf("originHub is required")
	}
	if strings.TrimSpace(m.DestinationHub) == "" {
		return fmt.Errorf("destinationHub is required")
	}
	if !validTransportRelayMode(strings.TrimSpace(m.TransportMode)) {
		return fmt.Errorf("invalid transportMode: %q", m.TransportMode)
	}
	if m.TotalDistanceMiles < 10 || m.TotalDistanceMiles > 3500 {
		return fmt.Errorf("totalDistanceMiles must be in [10, 3500], got %d", m.TotalDistanceMiles)
	}
	if m.CruiseAltitudeFeet < 0 || m.CruiseAltitudeFeet > 12500 {
		return fmt.Errorf("cruiseAltitudeFeet must be in [0, 12500], got %d", m.CruiseAltitudeFeet)
	}
	if m.TransportStressScore < 0 || m.TransportStressScore > 100 {
		return fmt.Errorf("transportStressScore must be in [0, 100], got %d", m.TransportStressScore)
	}
	if !m.ThermalRisk.IsValid() {
		return fmt.Errorf("invalid thermalRisk: %q", m.ThermalRisk)
	}
	if !m.RelayStatus.IsValid() {
		return fmt.Errorf("invalid relayStatus: %q", m.RelayStatus)
	}
	if m.FlightReadinessScore < 0 || m.FlightReadinessScore > 100 {
		return fmt.Errorf("flightReadinessScore must be in [0, 100], got %d", m.FlightReadinessScore)
	}
	if len(m.RelayLegChecks) < 1 {
		return fmt.Errorf("at least one relayLegCheck is required")
	}
	for i, leg := range m.RelayLegChecks {
		if strings.TrimSpace(leg.LegID) == "" ||
			strings.TrimSpace(leg.Title) == "" ||
			!validTransportLegCheckCategory(strings.TrimSpace(leg.Category)) {
			return fmt.Errorf("relayLegChecks[%d] must have non-empty legId, title, and valid category", i)
		}
	}
	if m.RelayStatus == RelayStatusDeliveredDestination && strings.TrimSpace(m.ManifestCompletionCode) == "" {
		return fmt.Errorf("manifestCompletionCode is required when relayStatus is DELIVERED_DESTINATION_VERIFIED")
	}
	if strings.TrimSpace(m.CreatedAt) == "" {
		return fmt.Errorf("createdAt is required")
	}
	if strings.TrimSpace(m.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
