package domain

import (
	"fmt"
	"strings"
)

// AvianClassType classifies the avian, parrot, or falconry recovery taxonomy and flight mechanics.
type AvianClassType string

const (
	AvianClassPsittacineParrotMacaw   AvianClassType = "PSITTACINE_PARROT_MACAW"
	AvianClassCockatielParakeetSmall  AvianClassType = "COCKATIEL_PARAKEET_SMALL"
	AvianClassFalconryRaptorTelemetry AvianClassType = "FALCONRY_RAPTOR_TELEMETRY"
	AvianClassWaterfowlDomesticGoose  AvianClassType = "WATERFOWL_DOMESTIC_GOOSE"
	AvianClassColumbiformHomingPigeon AvianClassType = "COLUMBIFORM_HOMING_PIGEON"
)

// IsValid returns true when AvianClassType is a supported enum value.
func (c AvianClassType) IsValid() bool {
	switch c {
	case AvianClassPsittacineParrotMacaw,
		AvianClassCockatielParakeetSmall,
		AvianClassFalconryRaptorTelemetry,
		AvianClassWaterfowlDomesticGoose,
		AvianClassColumbiformHomingPigeon:
		return true
	default:
		return false
	}
}

// AvianRecoveryStatus represents the lifecycle state of an avian recovery operation.
type AvianRecoveryStatus string

const (
	AvianStatusThermalDriftPredationCritical AvianRecoveryStatus = "THERMAL_DRIFT_PREDATION_CRITICAL"
	AvianStatusAcousticLurePerchStaged       AvianRecoveryStatus = "ACOUSTIC_LURE_PERCH_STAGED"
	AvianStatusRecoveredAviarySafe           AvianRecoveryStatus = "RECOVERED_AVIARY_SAFE"
)

// IsValid returns true when AvianRecoveryStatus is a supported lifecycle state.
func (s AvianRecoveryStatus) IsValid() bool {
	switch s {
	case AvianStatusThermalDriftPredationCritical,
		AvianStatusAcousticLurePerchStaged,
		AvianStatusRecoveredAviarySafe:
		return true
	default:
		return false
	}
}

// AvianProtocolStep represents an actionable avian telemetry, acoustic lure, or capture checklist step.
type AvianProtocolStep struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Category    string `json:"category"` // "VHF_BAND_TRIANGULATION", "FLOCK_ACOUSTIC_LURE", "KEVLAR_TOWEL_CAGE_TRANSFER"
	Required    bool   `json:"required"`
	Completed   bool   `json:"completed"`
	CompletedAt string `json:"completedAt,omitempty"`
}

// AvianRecoveryOperation represents an avian, exotic parrot, or falconry flight-corridor recovery operation.
type AvianRecoveryOperation struct {
	OperationID              string              `json:"operationId"`
	PetID                    string              `json:"petId"`
	PetName                  string              `json:"petName"`
	Species                  string              `json:"species"`
	WeightGrams              float64             `json:"weightGrams"`
	AvianClass               AvianClassType      `json:"avianClass"`
	Status                   AvianRecoveryStatus `json:"status"`
	PerchAltitudeMeters      float64             `json:"perchAltitudeMeters"`
	ThermalUpdraftKnots      float64             `json:"thermalUpdraftKnots"`
	AmbientTempCelsius       float64             `json:"ambientTempCelsius"`
	HoursAloft               float64             `json:"hoursAloft"`
	TelemetrySignalDbm       int                 `json:"telemetrySignalDbm"`
	HazardScore              int                 `json:"hazardScore"`
	LureReadinessScore       int                 `json:"lureReadinessScore"`
	ThermalDriftAlert        bool                `json:"thermalDriftAlert"`
	CropStarvationColdAlert  bool                `json:"cropStarvationColdAlert"`
	RecommendedLureAudio     string              `json:"recommendedLureAudio"`
	RecommendedCaptureRig    string              `json:"recommendedCaptureRig"`
	FlightCorridorBearing    string              `json:"flightCorridorBearing"`
	RiskFlags                []string            `json:"riskFlags"`
	ProtocolSteps            []AvianProtocolStep `json:"protocolSteps"`
	ClearanceCertificateHash string              `json:"clearanceCertificateHash,omitempty"`
	LeadAvianTechCallsign    string              `json:"leadAvianTechCallsign"`
	Notes                    string              `json:"notes"`
	UpdatedAt                string              `json:"updatedAt"`
}

// AvianEvaluationRequest represents input parameters for evaluating an avian recovery operation.
type AvianEvaluationRequest struct {
	OperationID           string         `json:"operationId,omitempty"`
	PetID                 string         `json:"petId"`
	PetName               string         `json:"petName"`
	Species               string         `json:"species"`
	WeightGrams           float64        `json:"weightGrams"`
	AvianClass            AvianClassType `json:"avianClass"`
	PerchAltitudeMeters   float64        `json:"perchAltitudeMeters"`
	ThermalUpdraftKnots   float64        `json:"thermalUpdraftKnots"`
	AmbientTempCelsius    float64        `json:"ambientTempCelsius"`
	HoursAloft            float64        `json:"hoursAloft"`
	TelemetrySignalDbm    int            `json:"telemetrySignalDbm"`
	FlightCorridorBearing string         `json:"flightCorridorBearing"`
	LeadAvianTechCallsign string         `json:"leadAvianTechCallsign"`
	Notes                 string         `json:"notes"`
}

// AvianStepUpdateRequest updates a protocol step on an existing avian recovery operation.
type AvianStepUpdateRequest struct {
	StepID    string `json:"stepId"`
	Completed bool   `json:"completed"`
	Notes     string `json:"notes,omitempty"`
}

func validAvianStepCategory(cat string) bool {
	switch cat {
	case "VHF_BAND_TRIANGULATION", "FLOCK_ACOUSTIC_LURE", "KEVLAR_TOWEL_CAGE_TRANSFER":
		return true
	default:
		return false
	}
}

// Validate checks domain invariants on AvianRecoveryOperation.
func (op AvianRecoveryOperation) Validate() error {
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
	if op.WeightGrams <= 0 || op.WeightGrams > 25000 {
		return fmt.Errorf("weightGrams must be in (0, 25000], got %v", op.WeightGrams)
	}
	if !op.AvianClass.IsValid() {
		return fmt.Errorf("invalid avianClass: %q", op.AvianClass)
	}
	if !op.Status.IsValid() {
		return fmt.Errorf("invalid status: %q", op.Status)
	}
	if op.PerchAltitudeMeters < 0 || op.PerchAltitudeMeters > 500 {
		return fmt.Errorf("perchAltitudeMeters must be in [0, 500], got %v", op.PerchAltitudeMeters)
	}
	if op.ThermalUpdraftKnots < 0 || op.ThermalUpdraftKnots > 120 {
		return fmt.Errorf("thermalUpdraftKnots must be in [0, 120], got %v", op.ThermalUpdraftKnots)
	}
	if op.AmbientTempCelsius < -40 || op.AmbientTempCelsius > 60 {
		return fmt.Errorf("ambientTempCelsius must be in [-40, 60], got %v", op.AmbientTempCelsius)
	}
	if op.HoursAloft < 0 || op.HoursAloft > 720 {
		return fmt.Errorf("hoursAloft must be in [0, 720], got %v", op.HoursAloft)
	}
	if op.TelemetrySignalDbm < -140 || op.TelemetrySignalDbm > 0 {
		return fmt.Errorf("telemetrySignalDbm must be in [-140, 0], got %d", op.TelemetrySignalDbm)
	}
	if op.HazardScore < 0 || op.HazardScore > 100 {
		return fmt.Errorf("hazardScore must be in [0, 100], got %d", op.HazardScore)
	}
	if op.LureReadinessScore < 0 || op.LureReadinessScore > 100 {
		return fmt.Errorf("lureReadinessScore must be in [0, 100], got %d", op.LureReadinessScore)
	}
	if len(op.ProtocolSteps) == 0 {
		return fmt.Errorf("at least one protocolStep is required")
	}
	for i, step := range op.ProtocolSteps {
		if strings.TrimSpace(step.ID) == "" ||
			strings.TrimSpace(step.Label) == "" ||
			!validAvianStepCategory(strings.TrimSpace(step.Category)) {
			return fmt.Errorf("protocolSteps[%d] must have non-empty id, label, and valid category", i)
		}
	}
	if op.Status == AvianStatusRecoveredAviarySafe && strings.TrimSpace(op.ClearanceCertificateHash) == "" {
		return fmt.Errorf("clearanceCertificateHash is required when status is RECOVERED_AVIARY_SAFE")
	}
	if strings.TrimSpace(op.UpdatedAt) == "" {
		return fmt.Errorf("updatedAt is required")
	}
	return nil
}
