package domain

import (
	"fmt"
	"strings"
)

// WinterTerrainType classifies the cold-weather or avalanche rescue terrain.
type WinterTerrainType string

const (
	WinterTerrainAvalancheDebrisChute    WinterTerrainType = "AVALANCHE_DEBRIS_CHUTE"
	WinterTerrainTreeWellPowderPit       WinterTerrainType = "TREE_WELL_POWDER_PIT"
	WinterTerrainBlizzardSnowdriftTrench WinterTerrainType = "BLIZZARD_SNOWDRIFT_TRENCH"
	WinterTerrainGlacialCrevasseLedge    WinterTerrainType = "GLACIAL_CREVASSE_LEDGE"
	WinterTerrainSubzeroFrozenTrail      WinterTerrainType = "SUBZERO_FROZEN_TRAIL"
)

// IsValid returns true when WinterTerrainType is a supported enum value.
func (t WinterTerrainType) IsValid() bool {
	switch t {
	case WinterTerrainAvalancheDebrisChute,
		WinterTerrainTreeWellPowderPit,
		WinterTerrainBlizzardSnowdriftTrench,
		WinterTerrainGlacialCrevasseLedge,
		WinterTerrainSubzeroFrozenTrail:
		return true
	default:
		return false
	}
}

// WinterRescueStatus represents the lifecycle state of a cold-weather / avalanche rescue operation.
type WinterRescueStatus string

const (
	WinterRescueStatusBurialHypothermiaCritical  WinterRescueStatus = "BURIAL_HYPOTHERMIA_CRITICAL"
	WinterRescueStatusProbeTrenchRewarmingStaged WinterRescueStatus = "PROBE_TRENCH_REWARMING_STAGED"
	WinterRescueStatusExtractedNormothermicSafe  WinterRescueStatus = "EXTRACTED_NORMOTHERMIC_SAFE"
)

// IsValid returns true when WinterRescueStatus is a supported lifecycle state.
func (s WinterRescueStatus) IsValid() bool {
	switch s {
	case WinterRescueStatusBurialHypothermiaCritical,
		WinterRescueStatusProbeTrenchRewarmingStaged,
		WinterRescueStatusExtractedNormothermicSafe:
		return true
	default:
		return false
	}
}

// WinterProtocolStep represents an actionable snowpack probing, airway excavation, or core rewarming checklist step.
type WinterProtocolStep struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Category    string `json:"category"` // "BEACON_PROBE_PINPOINT", "AIRWAY_SNOW_EXCAVATION", "ACTIVE_CORE_REWARMING"
	Required    bool   `json:"required"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// WinterRescueOperation represents a cold-weather avalanche, deep-snow drift, or hypothermia rewarming SAR operation.
type WinterRescueOperation struct {
	OperationID                   string               `json:"operationId"`
	PetID                         string               `json:"petId"`
	PetName                       string               `json:"petName"`
	Species                       string               `json:"species"`
	WeightKg                      float64              `json:"weightKg"`
	TerrainType                   WinterTerrainType    `json:"terrainType"`
	Status                        WinterRescueStatus   `json:"status"`
	BurialDepthCm                 float64              `json:"burialDepthCm"`
	WindChillCelsius              float64              `json:"windChillCelsius"`
	CoreTempCelsius               float64              `json:"coreTempCelsius"`
	MinutesBuried                 int                  `json:"minutesBuried"`
	AvalancheDangerScale          int                  `json:"avalancheDangerScale"`
	HazardScore                   int                  `json:"hazardScore"`
	RewarmingReadinessScore       int                  `json:"rewarmingReadinessScore"`
	AsphyxiaAirPocketAlert        bool                 `json:"asphyxiaAirPocketAlert"`
	SevereFrostbiteAfterdropAlert bool                 `json:"severeFrostbiteAfterdropAlert"`
	RecommendedLocatorGear        string               `json:"recommendedLocatorGear"`
	RecommendedRewarmingBivy      string               `json:"recommendedRewarmingBivy"`
	SnowpackSectorGrid            string               `json:"snowpackSectorGrid"`
	RiskFlags                     []string             `json:"riskFlags"`
	ProtocolSteps                 []WinterProtocolStep `json:"protocolSteps"`
	ClearanceCertificateHash      string               `json:"clearanceCertificateHash,omitempty"`
	LeadSkiPatrolCallsign         string               `json:"leadSkiPatrolCallsign"`
	Notes                         string               `json:"notes"`
	UpdatedAt                     string               `json:"updatedAt"`
}

// WinterEvaluationRequest represents input parameters for evaluating a winter / avalanche rescue operation.
type WinterEvaluationRequest struct {
	OperationID           string            `json:"operationId,omitempty"`
	PetID                 string            `json:"petId"`
	PetName               string            `json:"petName"`
	Species               string            `json:"species"`
	WeightKg              float64           `json:"weightKg"`
	TerrainType           WinterTerrainType `json:"terrainType"`
	BurialDepthCm         float64           `json:"burialDepthCm"`
	WindChillCelsius      float64           `json:"windChillCelsius"`
	CoreTempCelsius       float64           `json:"coreTempCelsius"`
	MinutesBuried         int               `json:"minutesBuried"`
	AvalancheDangerScale  int               `json:"avalancheDangerScale"`
	SnowpackSectorGrid    string            `json:"snowpackSectorGrid"`
	LeadSkiPatrolCallsign string            `json:"leadSkiPatrolCallsign"`
	Notes                 string            `json:"notes"`
}

// WinterStepUpdateRequest updates a protocol step on an existing winter rescue operation.
type WinterStepUpdateRequest struct {
	StepID    string `json:"stepId"`
	Completed bool   `json:"completed"`
	Notes     string `json:"notes,omitempty"`
}

func validWinterStepCategory(cat string) bool {
	switch cat {
	case "BEACON_PROBE_PINPOINT", "AIRWAY_SNOW_EXCAVATION", "ACTIVE_CORE_REWARMING":
		return true
	default:
		return false
	}
}

// Validate checks domain invariants on WinterRescueOperation.
func (op WinterRescueOperation) Validate() error {
	if strings.TrimSpace(op.OperationID) == "" {
		return fmt.Errorf("operationId is required")
	}
	if strings.TrimSpace(op.PetID) == "" {
		return fmt.Errorf("petId is required")
	}
	if strings.TrimSpace(op.PetName) == "" {
		return fmt.Errorf("petName is required")
	}
	if strings.TrimSpace(op.Species) == "" {
		return fmt.Errorf("species is required")
	}
	if op.WeightKg <= 0 || op.WeightKg > 250 {
		return fmt.Errorf("weightKg must be in (0, 250], got %v", op.WeightKg)
	}
	if !op.TerrainType.IsValid() {
		return fmt.Errorf("invalid terrainType: %q", op.TerrainType)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.BurialDepthCm < 0 || op.BurialDepthCm > 1000 {
		return fmt.Errorf("burialDepthCm must be in [0, 1000], got %v", op.BurialDepthCm)
	}
	if op.WindChillCelsius < -70 || op.WindChillCelsius > 20 {
		return fmt.Errorf("windChillCelsius must be in [-70, 20], got %v", op.WindChillCelsius)
	}
	if op.CoreTempCelsius < 20 || op.CoreTempCelsius > 43 {
		return fmt.Errorf("coreTempCelsius must be in [20, 43], got %v", op.CoreTempCelsius)
	}
	if op.MinutesBuried < 0 || op.MinutesBuried > 1440 {
		return fmt.Errorf("minutesBuried must be in [0, 1440], got %d", op.MinutesBuried)
	}
	if op.AvalancheDangerScale < 1 || op.AvalancheDangerScale > 5 {
		return fmt.Errorf("avalancheDangerScale must be in [1, 5], got %d", op.AvalancheDangerScale)
	}
	if op.HazardScore < 0 || op.HazardScore > 100 {
		return fmt.Errorf("hazardScore must be in [0, 100], got %d", op.HazardScore)
	}
	if op.RewarmingReadinessScore < 0 || op.RewarmingReadinessScore > 100 {
		return fmt.Errorf("rewarmingReadinessScore must be in [0, 100], got %d", op.RewarmingReadinessScore)
	}
	if len(op.ProtocolSteps) == 0 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.ID) == "" ||
			strings.TrimSpace(step.Label) == "" ||
			!validWinterStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty id, label, and valid category", i)
		}
	}
	if op.Status == WinterRescueStatusExtractedNormothermicSafe && strings.TrimSpace(op.ClearanceCertificateHash) == "" {
		return fmt.Errorf("clearanceCertificateHash is required when status is EXTRACTED_NORMOTHERMIC_SAFE")
	}
	if strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
