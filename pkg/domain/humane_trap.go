package domain

import (
	"fmt"
	"strings"
	"time"
)

// GeoPoint aliases LocationPoint for WGS-84 coordinates with Validate().
type GeoPoint = LocationPoint

type HumaneTrapType string

const (
	TrapTypeBoxGravity        HumaneTrapType = "BOX_TRAP_GRAVITY"
	TrapTypeDropRemote        HumaneTrapType = "DROP_TRAP_REMOTE"
	TrapTypeEnclosureCellular HumaneTrapType = "ENCLOSURE_PEN_CELLULAR"
)

func ValidHumaneTrapType(t HumaneTrapType) bool {
	switch t {
	case TrapTypeBoxGravity, TrapTypeDropRemote, TrapTypeEnclosureCellular:
		return true
	default:
		return false
	}
}

func (t HumaneTrapType) IsValid() bool {
	return ValidHumaneTrapType(t)
}

type HumaneTrapStatus string

const (
	TrapStatusArmedActive         HumaneTrapStatus = "ARMED_ACTIVE"
	TrapStatusDoorTrippedUrgent   HumaneTrapStatus = "DOOR_TRIPPED_URGENT"
	TrapStatusResponderDispatched HumaneTrapStatus = "RESPONDER_DISPATCHED"
	TrapStatusTargetCaptured      HumaneTrapStatus = "TARGET_CAPTURED_SECURED"
	TrapStatusThermalLockout      HumaneTrapStatus = "THERMAL_LOCKOUT_DISARMED"
)

func ValidHumaneTrapStatus(s HumaneTrapStatus) bool {
	switch s {
	case TrapStatusArmedActive,
		TrapStatusDoorTrippedUrgent,
		TrapStatusResponderDispatched,
		TrapStatusTargetCaptured,
		TrapStatusThermalLockout:
		return true
	default:
		return false
	}
}

func (s HumaneTrapStatus) IsValid() bool {
	return ValidHumaneTrapStatus(s)
}

type BaitFreshnessState string

const (
	BaitStateFreshPeak        BaitFreshnessState = "FRESH_PEAK_SCENT"
	BaitStateDegradingRefresh BaitFreshnessState = "DEGRADING_REFRESH_SOON"
	BaitStateSpoiledExpired   BaitFreshnessState = "SPOILED_EXPIRED"
)

func ValidBaitFreshnessState(b BaitFreshnessState) bool {
	switch b {
	case BaitStateFreshPeak, BaitStateDegradingRefresh, BaitStateSpoiledExpired:
		return true
	default:
		return false
	}
}

func (b BaitFreshnessState) IsValid() bool {
	return ValidBaitFreshnessState(b)
}

type HumaneTrapAlertFlag string

const (
	TrapAlertUrgentCheckSLA     HumaneTrapAlertFlag = "URGENT_15MIN_CHECK_SLA"
	TrapAlertHyperthermiaHazard HumaneTrapAlertFlag = "HYPERTHERMIA_HEAT_HAZARD"
	TrapAlertHypothermiaHazard  HumaneTrapAlertFlag = "HYPOTHERMIA_FREEZE_HAZARD"
	TrapAlertBaitSpoiled        HumaneTrapAlertFlag = "BAIT_SPOILED_REFRESH_REQUIRED"
	TrapAlertLowBattery         HumaneTrapAlertFlag = "LOW_BATTERY_CRITICAL"
	TrapAlertBycatchWeight      HumaneTrapAlertFlag = "WEIGHT_MISMATCH_POSSIBLE_BYCATCH"
)

func ValidHumaneTrapAlertFlag(f HumaneTrapAlertFlag) bool {
	switch f {
	case TrapAlertUrgentCheckSLA,
		TrapAlertHyperthermiaHazard,
		TrapAlertHypothermiaHazard,
		TrapAlertBaitSpoiled,
		TrapAlertLowBattery,
		TrapAlertBycatchWeight:
		return true
	default:
		return false
	}
}

func (f HumaneTrapAlertFlag) IsValid() bool {
	return ValidHumaneTrapAlertFlag(f)
}

type HumaneTrapDeployment struct {
	TrapID                   string                `json:"trapId"`
	PetID                    string                `json:"petId"`
	PetName                  string                `json:"petName"`
	TargetSpecies            string                `json:"targetSpecies"`  // e.g. "CAT", "DOG"
	TargetWeightKg           float64               `json:"targetWeightKg"` // > 0
	TrapLabel                string                `json:"trapLabel"`      // e.g. "Tomahawk Tru-Catch #404 - Creek Culvert"
	TrapType                 HumaneTrapType        `json:"trapType"`
	Status                   HumaneTrapStatus      `json:"status"`
	Location                 GeoPoint              `json:"location"`
	SectorID                 string                `json:"sectorId,omitempty"`
	BaitType                 string                `json:"baitType"` // e.g. "SARDINES_IN_OIL", "ROTISSERIE_CHICKEN", "WET_RECOVERY_PATE", "CATNIP_MACKEREL_BLEND"
	BaitPlacedAt             time.Time             `json:"baitPlacedAt"`
	BaitFreshnessPct         int                   `json:"baitFreshnessPct"` // 0..100
	BaitState                BaitFreshnessState    `json:"baitState"`
	DoorClosed               bool                  `json:"doorClosed"`
	InternalTempC            float64               `json:"internalTempC"`      // -40..70
	BatteryPct               int                   `json:"batteryPct"`         // 0..100
	RssiDbm                  int                   `json:"rssiDbm"`            // -130..-20
	AccelerometerPeakG       float64               `json:"accelerometerPeakG"` // >= 0
	EstimatedCaptureWeightKg float64               `json:"estimatedCaptureWeightKg,omitempty"`
	TrippedAt                *time.Time            `json:"trippedAt,omitempty"`
	CheckSLADeadline         *time.Time            `json:"checkSlaDeadline,omitempty"`
	AssignedResponderID      string                `json:"assignedResponderId,omitempty"`
	AssignedResponderName    string                `json:"assignedResponderName,omitempty"`
	ResponderETAMinutes      int                   `json:"responderEtaMinutes,omitempty"`
	AlertFlags               []HumaneTrapAlertFlag `json:"alertFlags,omitempty"`
	TelemetrySummary         string                `json:"telemetrySummary"`
	PromotedSightingID       string                `json:"promotedSightingId,omitempty"`
	CreatedAt                time.Time             `json:"createdAt"`
	UpdatedAt                time.Time             `json:"updatedAt"`
}

func (d HumaneTrapDeployment) Validate() error {
	if strings.TrimSpace(d.TrapID) == "" {
		return fmt.Errorf("domain: trapId is required")
	}
	if strings.TrimSpace(d.PetID) == "" {
		return fmt.Errorf("domain: petId is required")
	}
	if strings.TrimSpace(d.PetName) == "" {
		return fmt.Errorf("domain: petName is required")
	}
	if strings.TrimSpace(d.TrapLabel) == "" {
		return fmt.Errorf("domain: trapLabel is required")
	}
	if strings.TrimSpace(d.BaitType) == "" {
		return fmt.Errorf("domain: baitType is required")
	}
	if strings.TrimSpace(d.TelemetrySummary) == "" {
		return fmt.Errorf("domain: telemetrySummary is required")
	}
	species := strings.TrimSpace(d.TargetSpecies)
	if species == "" || species != strings.ToUpper(species) {
		return fmt.Errorf("domain: targetSpecies must be non-empty uppercase (e.g. CAT or DOG)")
	}
	if d.TargetWeightKg <= 0 || d.TargetWeightKg > 120 {
		return fmt.Errorf("domain: targetWeightKg must be in (0, 120], got %f", d.TargetWeightKg)
	}
	if !ValidHumaneTrapType(d.TrapType) {
		return fmt.Errorf("domain: invalid trapType %q", d.TrapType)
	}
	if !ValidHumaneTrapStatus(d.Status) {
		return fmt.Errorf("domain: invalid status %q", d.Status)
	}
	if err := d.Location.Validate(); err != nil {
		return err
	}
	if d.BaitFreshnessPct < 0 || d.BaitFreshnessPct > 100 {
		return fmt.Errorf("domain: baitFreshnessPct must be in [0, 100], got %d", d.BaitFreshnessPct)
	}
	if !ValidBaitFreshnessState(d.BaitState) {
		return fmt.Errorf("domain: invalid baitState %q", d.BaitState)
	}
	if d.InternalTempC < -40.0 || d.InternalTempC > 70.0 {
		return fmt.Errorf("domain: internalTempC must be in [-40, 70], got %f", d.InternalTempC)
	}
	if d.BatteryPct < 0 || d.BatteryPct > 100 {
		return fmt.Errorf("domain: batteryPct must be in [0, 100], got %d", d.BatteryPct)
	}
	if d.RssiDbm < -130 || d.RssiDbm > -20 {
		return fmt.Errorf("domain: rssiDbm must be in [-130, -20], got %d", d.RssiDbm)
	}
	if d.AccelerometerPeakG < 0 {
		return fmt.Errorf("domain: accelerometerPeakG must be >= 0, got %f", d.AccelerometerPeakG)
	}
	for _, flag := range d.AlertFlags {
		if !ValidHumaneTrapAlertFlag(flag) {
			return fmt.Errorf("domain: invalid alertFlag %q", flag)
		}
	}
	if d.Status == TrapStatusResponderDispatched {
		if strings.TrimSpace(d.AssignedResponderID) == "" || strings.TrimSpace(d.AssignedResponderName) == "" {
			return fmt.Errorf("domain: assignedResponderId and assignedResponderName are required when status is RESPONDER_DISPATCHED")
		}
		if d.ResponderETAMinutes <= 0 {
			return fmt.Errorf("domain: responderEtaMinutes must be > 0 when status is RESPONDER_DISPATCHED")
		}
	}
	if d.BaitPlacedAt.IsZero() {
		return fmt.Errorf("domain: baitPlacedAt is required")
	}
	if d.CreatedAt.IsZero() {
		return fmt.Errorf("domain: createdAt is required")
	}
	if d.UpdatedAt.IsZero() {
		return fmt.Errorf("domain: updatedAt is required")
	}
	return nil
}
