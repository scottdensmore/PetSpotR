package domain

import (
	"fmt"
	"strings"
)

// MaritimeZoneType classifies the maritime, coastal, or tidal mudflat rescue zone.
type MaritimeZoneType string

const (
	MaritimeZoneTidalMudflatEstuary      MaritimeZoneType = "TIDAL_MUDFLAT_ESTUARY"
	MaritimeZoneRipCurrentSurfBreak      MaritimeZoneType = "RIP_CURRENT_SURF_BREAK"
	MaritimeZoneJettyTetrapodBreakwater  MaritimeZoneType = "JETTY_TETRAPOD_BREAKWATER"
	MaritimeZoneOffshoreVesselOverboard  MaritimeZoneType = "OFFSHORE_VESSEL_OVERBOARD"
	MaritimeZoneSaltMarshCordgrassSlough MaritimeZoneType = "SALT_MARSH_CORDGRASS_SLOUGH"
)

// IsValid returns true when MaritimeZoneType is a supported enum value.
func (z MaritimeZoneType) IsValid() bool {
	switch z {
	case MaritimeZoneTidalMudflatEstuary,
		MaritimeZoneRipCurrentSurfBreak,
		MaritimeZoneJettyTetrapodBreakwater,
		MaritimeZoneOffshoreVesselOverboard,
		MaritimeZoneSaltMarshCordgrassSlough:
		return true
	default:
		return false
	}
}

// MaritimeRescueStatus represents the lifecycle state of a maritime or coastal pet rescue operation.
type MaritimeRescueStatus string

const (
	MaritimeStatusTidalFloodAspirationCritical MaritimeRescueStatus = "TIDAL_FLOOD_ASPIRATION_CRITICAL"
	MaritimeStatusHovercraftMudSledStaged      MaritimeRescueStatus = "HOVERCRAFT_MUD_SLED_STAGED"
	MaritimeStatusExtractedHarborDeckSafe      MaritimeRescueStatus = "EXTRACTED_HARBOR_DECK_SAFE"
)

// IsValid returns true when MaritimeRescueStatus is a supported lifecycle state.
func (s MaritimeRescueStatus) IsValid() bool {
	switch s {
	case MaritimeStatusTidalFloodAspirationCritical,
		MaritimeStatusHovercraftMudSledStaged,
		MaritimeStatusExtractedHarborDeckSafe:
		return true
	default:
		return false
	}
}

// MaritimeProtocolStep represents an actionable tidal mudflat suction relief, vessel PFD rigging,
// saltwater aspiration oxygenation, or harbor deck decontamination/rewarming checklist step.
type MaritimeProtocolStep struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// MaritimeRescueOperation represents a maritime, coastal, or tidal mudflat pet rescue operation.
type MaritimeRescueOperation struct {
	ID                           string                 `json:"id"`
	PetID                        string                 `json:"pet_id"`
	PetName                      string                 `json:"pet_name"`
	Species                      string                 `json:"species"`
	WeightKg                     float64                `json:"weight_kg"`
	MaritimeZone                 MaritimeZoneType       `json:"maritime_zone"`
	Status                       MaritimeRescueStatus   `json:"status"`
	TideSurgeRateCmHr            float64                `json:"tide_surge_rate_cm_hr"`
	SwellHeightMeters            float64                `json:"swell_height_meters"`
	WaterTempCelsius             float64                `json:"water_temp_celsius"`
	SaltwaterAspirationSuspected bool                   `json:"saltwater_aspiration_suspected"`
	MudSuctionDepthCm            float64                `json:"mud_suction_depth_cm"`
	MudWalkersAirLanceDeployed   bool                   `json:"mud_walkers_air_lance_deployed"`
	CaninePFDHarnessSecured      bool                   `json:"canine_pfd_harness_secured"`
	RequiredVessel               string                 `json:"required_vessel"`
	TidalRiskAdvisory            string                 `json:"tidal_risk_advisory"`
	ProtocolSteps                []MaritimeProtocolStep `json:"protocol_steps"`
	ClearanceToken               string                 `json:"clearance_token,omitempty"`
	CreatedAt                    string                 `json:"created_at"`
	UpdatedAt                    string                 `json:"updated_at"`
}

// MaritimeEvaluationRequest represents input parameters for evaluating a maritime or coastal rescue operation.
type MaritimeEvaluationRequest struct {
	ID                           string           `json:"id,omitempty"`
	PetID                        string           `json:"pet_id,omitempty"`
	PetName                      string           `json:"pet_name"`
	Species                      string           `json:"species"`
	WeightKg                     float64          `json:"weight_kg"`
	MaritimeZone                 MaritimeZoneType `json:"maritime_zone"`
	TideSurgeRateCmHr            float64          `json:"tide_surge_rate_cm_hr"`
	SwellHeightMeters            float64          `json:"swell_height_meters"`
	WaterTempCelsius             float64          `json:"water_temp_celsius"`
	SaltwaterAspirationSuspected bool             `json:"saltwater_aspiration_suspected"`
	MudSuctionDepthCm            float64          `json:"mud_suction_depth_cm"`
	MudWalkersAirLanceDeployed   bool             `json:"mud_walkers_air_lance_deployed"`
	CaninePFDHarnessSecured      bool             `json:"canine_pfd_harness_secured"`
}

// MaritimeStepUpdateRequest updates a protocol step on an existing maritime rescue operation.
type MaritimeStepUpdateRequest struct {
	StepID    string `json:"step_id"`
	Completed bool   `json:"completed"`
}

// Validate checks domain invariants on MaritimeRescueOperation.
func (op MaritimeRescueOperation) Validate() error {
	if strings.TrimSpace(op.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(op.PetID) == "" {
		return fmt.Errorf("pet_id is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("pet_name is required")
	}
	if strings.TrimSpace(op.Species) == "" {
		return fmt.Errorf("species is required")
	}
	if op.WeightKg <= 0 {
		return fmt.Errorf("weight_kg must be > 0, got %v", op.WeightKg)
	}
	if !op.MaritimeZone.IsValid() {
		return fmt.Errorf("invalid maritime_zone: %q", op.MaritimeZone)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.TideSurgeRateCmHr < 0 {
		return fmt.Errorf("tide_surge_rate_cm_hr must be >= 0, got %v", op.TideSurgeRateCmHr)
	}
	if op.SwellHeightMeters < 0 {
		return fmt.Errorf("swell_height_meters must be >= 0, got %v", op.SwellHeightMeters)
	}
	if op.WaterTempCelsius < -2.0 || op.WaterTempCelsius > 40.0 {
		return fmt.Errorf("water_temp_celsius must be in [-2.0, 40.0], got %v", op.WaterTempCelsius)
	}
	if op.MudSuctionDepthCm < 0 {
		return fmt.Errorf("mud_suction_depth_cm must be >= 0, got %v", op.MudSuctionDepthCm)
	}
	if len(op.ProtocolSteps) == 0 {
		return fmt.Errorf("at least one protocol step is required")
	}
	if op.Status == MaritimeStatusExtractedHarborDeckSafe && strings.TrimSpace(op.ClearanceToken) == "" {
		return fmt.Errorf("clearance_token is required when status is EXTRACTED_HARBOR_DECK_SAFE")
	}
	return nil
}
