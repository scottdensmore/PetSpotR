package humanetrap

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	HumaneCheckSLAWindow   = 15 * time.Minute
	HyperthermiaThresholdC = 32.0
	HypothermiaThresholdC  = 2.0
	LowBatteryThresholdPct = 15
	BycatchWeightDiffRatio = 0.45 // > 45% deviation from TargetWeightKg flags possible wildlife bycatch
)

type TelemetryUpdateInput struct {
	DoorClosed               bool
	InternalTempC            float64
	BatteryPct               int
	RssiDbm                  int
	AccelerometerPeakG       float64
	EstimatedCaptureWeightKg float64
	BaitReset                bool
	BaitType                 string
}

type DispatchInput struct {
	ResponderID   string
	ResponderName string
	ETAMinutes    int
}

type ResolutionOutcome string

const (
	OutcomeTargetPetCaptured    ResolutionOutcome = "TARGET_PET_CAPTURED"
	OutcomeBycatchReleased      ResolutionOutcome = "BYCATCH_RELEASED_REARMED"
	OutcomeThermalShadedRearmed ResolutionOutcome = "THERMAL_SHADED_REARMED"
)

// ComputeBaitFreshness computes the olfactory bait freshness percentage (0..100) and state
// based on bait type shelf life and internal cage temperature.
func ComputeBaitFreshness(baitType string, baitPlacedAt time.Time, internalTempC float64, now time.Time) (int, domain.BaitFreshnessState) {
	baseHours := 8.0
	switch strings.ToUpper(strings.TrimSpace(baitType)) {
	case "SARDINES_IN_OIL":
		baseHours = 8.0
	case "ROTISSERIE_CHICKEN":
		baseHours = 10.0
	case "WET_RECOVERY_PATE":
		baseHours = 6.0
	case "CATNIP_MACKEREL_BLEND":
		baseHours = 9.0
	}

	var thermalMultiplier float64
	if internalTempC > 20.0 {
		thermalMultiplier = 1.0 + (internalTempC-20.0)*0.08
	} else {
		thermalMultiplier = math.Max(0.5, 1.0-(20.0-internalTempC)*0.02)
	}

	elapsedHours := math.Max(0, now.Sub(baitPlacedAt).Hours())
	effectiveHours := elapsedHours * thermalMultiplier
	freshnessPct := int(math.Round(100.0 * (1.0 - effectiveHours/baseHours)))
	if freshnessPct < 0 {
		freshnessPct = 0
	} else if freshnessPct > 100 {
		freshnessPct = 100
	}

	var state domain.BaitFreshnessState
	switch {
	case freshnessPct >= 60:
		state = domain.BaitStateFreshPeak
	case freshnessPct >= 25:
		state = domain.BaitStateDegradingRefresh
	default:
		state = domain.BaitStateSpoiledExpired
	}

	return freshnessPct, state
}

// ApplyTelemetryUpdate applies sensor telemetry, recalculates bait freshness, evaluates
// door-trip SLA, bycatch weight discrepancy, and thermal safety lockouts.
func ApplyTelemetryUpdate(trap *domain.HumaneTrapDeployment, input TelemetryUpdateInput, now time.Time) error {
	if trap == nil {
		return fmt.Errorf("humanetrap: trap deployment is nil")
	}
	now = now.UTC()

	if input.BaitReset {
		trap.BaitPlacedAt = now
		if trimmedBait := strings.TrimSpace(input.BaitType); trimmedBait != "" {
			trap.BaitType = trimmedBait
		}
	}

	trap.InternalTempC = input.InternalTempC
	trap.BatteryPct = input.BatteryPct
	trap.RssiDbm = input.RssiDbm
	trap.AccelerometerPeakG = input.AccelerometerPeakG
	trap.EstimatedCaptureWeightKg = input.EstimatedCaptureWeightKg
	trap.DoorClosed = input.DoorClosed

	trap.BaitFreshnessPct, trap.BaitState = ComputeBaitFreshness(trap.BaitType, trap.BaitPlacedAt, trap.InternalTempC, now)

	var alerts []domain.HumaneTrapAlertFlag

	// 1. Door Trip Detection
	if input.DoorClosed {
		if trap.TrippedAt == nil {
			tripped := now
			trap.TrippedAt = &tripped
		}
		if trap.CheckSLADeadline == nil {
			deadline := trap.TrippedAt.Add(HumaneCheckSLAWindow)
			trap.CheckSLADeadline = &deadline
		}
		alerts = append(alerts, domain.TrapAlertUrgentCheckSLA)

		if input.EstimatedCaptureWeightKg > 0 && trap.TargetWeightKg > 0 {
			diffRatio := math.Abs(input.EstimatedCaptureWeightKg-trap.TargetWeightKg) / trap.TargetWeightKg
			if diffRatio > BycatchWeightDiffRatio {
				alerts = append(alerts, domain.TrapAlertBycatchWeight)
			}
		}

		if trap.Status != domain.TrapStatusResponderDispatched && trap.Status != domain.TrapStatusTargetCaptured {
			trap.Status = domain.TrapStatusDoorTrippedUrgent
		}
	}

	// 2. Thermal Safety Evaluation
	if input.InternalTempC >= HyperthermiaThresholdC {
		alerts = append(alerts, domain.TrapAlertHyperthermiaHazard)
		if !input.DoorClosed && trap.Status != domain.TrapStatusTargetCaptured {
			trap.Status = domain.TrapStatusThermalLockout
		}
	} else if input.InternalTempC <= HypothermiaThresholdC {
		alerts = append(alerts, domain.TrapAlertHypothermiaHazard)
		if !input.DoorClosed && trap.Status != domain.TrapStatusTargetCaptured {
			trap.Status = domain.TrapStatusThermalLockout
		}
	} else if !input.DoorClosed && trap.Status != domain.TrapStatusTargetCaptured && trap.Status != domain.TrapStatusResponderDispatched {
		trap.Status = domain.TrapStatusArmedActive
		trap.TrippedAt = nil
		trap.CheckSLADeadline = nil
	}

	// 3. Bait & Battery Alerts
	if trap.BaitState == domain.BaitStateSpoiledExpired {
		alerts = append(alerts, domain.TrapAlertBaitSpoiled)
	}
	if trap.BatteryPct <= LowBatteryThresholdPct {
		alerts = append(alerts, domain.TrapAlertLowBattery)
	}

	trap.AlertFlags = alerts

	flagStrings := make([]string, 0, len(alerts))
	for _, f := range alerts {
		flagStrings = append(flagStrings, string(f))
	}
	alertDesc := "no active alerts"
	if len(flagStrings) > 0 {
		alertDesc = "alerts: " + strings.Join(flagStrings, ", ")
	}
	trap.TelemetrySummary = fmt.Sprintf(
		"Status %s | Temp %.1f°C | Bait %d%% (%s) | %s",
		trap.Status,
		trap.InternalTempC,
		trap.BaitFreshnessPct,
		trap.BaitState,
		alertDesc,
	)
	trap.UpdatedAt = now

	return nil
}

// DispatchTrapResponder assigns a field responder to inspect a tripped or hazardous trap.
func DispatchTrapResponder(trap *domain.HumaneTrapDeployment, input DispatchInput, now time.Time) error {
	if trap == nil {
		return fmt.Errorf("humanetrap: trap deployment is nil")
	}
	responderID := strings.TrimSpace(input.ResponderID)
	responderName := strings.TrimSpace(input.ResponderName)
	if responderID == "" {
		return fmt.Errorf("humanetrap: responderId is required")
	}
	if responderName == "" {
		return fmt.Errorf("humanetrap: responderName is required")
	}
	if input.ETAMinutes <= 0 {
		return fmt.Errorf("humanetrap: etaMinutes must be > 0")
	}

	trap.AssignedResponderID = responderID
	trap.AssignedResponderName = responderName
	trap.ResponderETAMinutes = input.ETAMinutes
	trap.Status = domain.TrapStatusResponderDispatched
	trap.TelemetrySummary = fmt.Sprintf(
		"Responder %s (%s) dispatched — ETA %d min for urgent trap verification.",
		trap.AssignedResponderName,
		trap.AssignedResponderID,
		trap.ResponderETAMinutes,
	)
	trap.UpdatedAt = now.UTC()
	return nil
}

// ResolveTrapInspection records the field verification outcome (target pet secured, bycatch released, or thermal shade installed).
func ResolveTrapInspection(trap *domain.HumaneTrapDeployment, outcome ResolutionOutcome, notes string, now time.Time) error {
	if trap == nil {
		return fmt.Errorf("humanetrap: trap deployment is nil")
	}
	trimmedNotes := strings.TrimSpace(notes)
	if trimmedNotes == "" {
		return fmt.Errorf("humanetrap: resolution notes are required")
	}
	nowUTC := now.UTC()

	switch outcome {
	case OutcomeTargetPetCaptured:
		trap.Status = domain.TrapStatusTargetCaptured
		trap.DoorClosed = true
		trap.AlertFlags = nil
		trap.TelemetrySummary = fmt.Sprintf("TARGET PET SECURED (%s): %s", trap.PetName, trimmedNotes)
	case OutcomeBycatchReleased:
		trap.Status = domain.TrapStatusArmedActive
		trap.DoorClosed = false
		trap.TrippedAt = nil
		trap.CheckSLADeadline = nil
		trap.EstimatedCaptureWeightKg = 0
		trap.AccelerometerPeakG = 0.1
		trap.AssignedResponderID = ""
		trap.AssignedResponderName = ""
		trap.ResponderETAMinutes = 0
		trap.BaitPlacedAt = nowUTC
		trap.BaitFreshnessPct = 100
		trap.BaitState = domain.BaitStateFreshPeak
		trap.AlertFlags = nil
		trap.TelemetrySummary = fmt.Sprintf("Bycatch safely released; trap re-baited & re-armed: %s", trimmedNotes)
	case OutcomeThermalShadedRearmed:
		trap.Status = domain.TrapStatusArmedActive
		trap.DoorClosed = false
		trap.InternalTempC = 22.0
		trap.TrippedAt = nil
		trap.CheckSLADeadline = nil
		trap.BaitPlacedAt = nowUTC
		trap.BaitFreshnessPct = 100
		trap.BaitState = domain.BaitStateFreshPeak
		trap.AlertFlags = nil
		trap.TelemetrySummary = fmt.Sprintf("Thermal shade canopy installed & bait refreshed; trap re-armed: %s", trimmedNotes)
	default:
		return fmt.Errorf("humanetrap: unsupported resolution outcome %q", outcome)
	}

	trap.UpdatedAt = nowUTC
	return nil
}
