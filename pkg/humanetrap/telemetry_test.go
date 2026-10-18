package humanetrap

import (
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func baseTestTrap(now time.Time) *domain.HumaneTrapDeployment {
	return &domain.HumaneTrapDeployment{
		TrapID:                   "trap-404",
		PetID:                    "pet-whiskers-01",
		PetName:                  "Whiskers",
		TargetSpecies:            "CAT",
		TargetWeightKg:           4.5,
		TrapLabel:                "Tomahawk Tru-Catch #404 - Creek Culvert",
		TrapType:                 domain.TrapTypeBoxGravity,
		Status:                   domain.TrapStatusArmedActive,
		Location:                 domain.GeoPoint{Latitude: 37.7749, Longitude: -122.4194},
		SectorID:                 "SEC-NORTH-02",
		BaitType:                 "SARDINES_IN_OIL",
		BaitPlacedAt:             now.Add(-1 * time.Hour),
		BaitFreshnessPct:         88,
		BaitState:                domain.BaitStateFreshPeak,
		DoorClosed:               false,
		InternalTempC:            20.0,
		BatteryPct:               92,
		RssiDbm:                  -68,
		AccelerometerPeakG:       0.1,
		EstimatedCaptureWeightKg: 0,
		TelemetrySummary:         "Armed and active",
		CreatedAt:                now.Add(-2 * time.Hour),
		UpdatedAt:                now.Add(-1 * time.Hour),
	}
}

func TestComputeBaitFreshness(t *testing.T) {
	baseTime := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		baitType      string
		placedAt      time.Time
		tempC         float64
		now           time.Time
		wantPct       int
		wantBaitState domain.BaitFreshnessState
	}{
		{
			name:          "Freshly placed sardines at 20C",
			baitType:      "SARDINES_IN_OIL",
			placedAt:      baseTime,
			tempC:         20.0,
			now:           baseTime,
			wantPct:       100,
			wantBaitState: domain.BaitStateFreshPeak,
		},
		{
			name:          "Sardines after 2h at 20C (base 8h -> 75%)",
			baitType:      "SARDINES_IN_OIL",
			placedAt:      baseTime.Add(-2 * time.Hour),
			tempC:         20.0,
			now:           baseTime,
			wantPct:       75,
			wantBaitState: domain.BaitStateFreshPeak,
		},
		{
			name:          "Rotisserie chicken after 5h at 20C (base 10h -> 50% degrading)",
			baitType:      "ROTISSERIE_CHICKEN",
			placedAt:      baseTime.Add(-5 * time.Hour),
			tempC:         20.0,
			now:           baseTime,
			wantPct:       50,
			wantBaitState: domain.BaitStateDegradingRefresh,
		},
		{
			name:          "Wet recovery pate after 3h at 30C (base 6h, mult 1.8 -> eff 5.4h -> 10% spoiled)",
			baitType:      "WET_RECOVERY_PATE",
			placedAt:      baseTime.Add(-3 * time.Hour),
			tempC:         30.0,
			now:           baseTime,
			wantPct:       10,
			wantBaitState: domain.BaitStateSpoiledExpired,
		},
		{
			name:          "Catnip mackerel blend after 4.5h at 10C (base 9h, mult 0.8 -> eff 3.6h -> 60% fresh)",
			baitType:      "CATNIP_MACKEREL_BLEND",
			placedAt:      baseTime.Add(-270 * time.Minute),
			tempC:         10.0,
			now:           baseTime,
			wantPct:       60,
			wantBaitState: domain.BaitStateFreshPeak,
		},
		{
			name:          "Cold clamp multiplier at -20C (min mult 0.5, unknown bait base 8h, 8h elapsed -> eff 4h -> 50%)",
			baitType:      "CUSTOM_TUNA",
			placedAt:      baseTime.Add(-8 * time.Hour),
			tempC:         -20.0,
			now:           baseTime,
			wantPct:       50,
			wantBaitState: domain.BaitStateDegradingRefresh,
		},
		{
			name:          "Clamped to 0% when long expired",
			baitType:      "SARDINES_IN_OIL",
			placedAt:      baseTime.Add(-24 * time.Hour),
			tempC:         25.0,
			now:           baseTime,
			wantPct:       0,
			wantBaitState: domain.BaitStateSpoiledExpired,
		},
		{
			name:          "Future placedAt clamped to 100%",
			baitType:      "SARDINES_IN_OIL",
			placedAt:      baseTime.Add(1 * time.Hour),
			tempC:         20.0,
			now:           baseTime,
			wantPct:       100,
			wantBaitState: domain.BaitStateFreshPeak,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pct, state := ComputeBaitFreshness(tc.baitType, tc.placedAt, tc.tempC, tc.now)
			if pct != tc.wantPct {
				t.Errorf("ComputeBaitFreshness pct = %d, want %d", pct, tc.wantPct)
			}
			if state != tc.wantBaitState {
				t.Errorf("ComputeBaitFreshness state = %q, want %q", state, tc.wantBaitState)
			}
		})
	}
}

func TestApplyTelemetryUpdate_DoorTripAndBycatch(t *testing.T) {
	now := time.Date(2026, 10, 18, 14, 0, 0, 0, time.UTC)

	if err := ApplyTelemetryUpdate(nil, TelemetryUpdateInput{}, now); err == nil {
		t.Fatalf("expected error for nil trap")
	}

	trap := baseTestTrap(now)
	err := ApplyTelemetryUpdate(trap, TelemetryUpdateInput{
		DoorClosed:               true,
		InternalTempC:            21.5,
		BatteryPct:               84,
		RssiDbm:                  -70,
		AccelerometerPeakG:       2.4,
		EstimatedCaptureWeightKg: 8.2, // >45% diff from 4.5kg target (raccoon/opossum bycatch)
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trap.Status != domain.TrapStatusDoorTrippedUrgent {
		t.Errorf("expected status %q, got %q", domain.TrapStatusDoorTrippedUrgent, trap.Status)
	}
	if trap.TrippedAt == nil || !trap.TrippedAt.Equal(now) {
		t.Errorf("expected TrippedAt=%v, got %v", now, trap.TrippedAt)
	}
	wantDeadline := now.Add(HumaneCheckSLAWindow)
	if trap.CheckSLADeadline == nil || !trap.CheckSLADeadline.Equal(wantDeadline) {
		t.Errorf("expected CheckSLADeadline=%v, got %v", wantDeadline, trap.CheckSLADeadline)
	}

	hasSLA := false
	hasBycatch := false
	for _, f := range trap.AlertFlags {
		if f == domain.TrapAlertUrgentCheckSLA {
			hasSLA = true
		}
		if f == domain.TrapAlertBycatchWeight {
			hasBycatch = true
		}
	}
	if !hasSLA || !hasBycatch {
		t.Errorf("expected URGENT_15MIN_CHECK_SLA and WEIGHT_MISMATCH_POSSIBLE_BYCATCH flags, got %v", trap.AlertFlags)
	}
	if err := trap.Validate(); err != nil {
		t.Errorf("expected valid trap after update, got: %v", err)
	}
}

func TestApplyTelemetryUpdate_ThermalLockoutAndAlerts(t *testing.T) {
	now := time.Date(2026, 10, 18, 15, 0, 0, 0, time.UTC)
	trap := baseTestTrap(now)
	trap.BaitPlacedAt = now.Add(-12 * time.Hour) // spoiled

	// Hyperthermia + Low Battery + Bait Spoiled while door is open
	err := ApplyTelemetryUpdate(trap, TelemetryUpdateInput{
		DoorClosed:         false,
		InternalTempC:      34.5,
		BatteryPct:         12,
		RssiDbm:            -75,
		AccelerometerPeakG: 0.1,
	}, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trap.Status != domain.TrapStatusThermalLockout {
		t.Errorf("expected status %q, got %q", domain.TrapStatusThermalLockout, trap.Status)
	}

	wantFlags := map[domain.HumaneTrapAlertFlag]bool{
		domain.TrapAlertHyperthermiaHazard: false,
		domain.TrapAlertBaitSpoiled:        false,
		domain.TrapAlertLowBattery:         false,
	}
	for _, f := range trap.AlertFlags {
		if _, ok := wantFlags[f]; ok {
			wantFlags[f] = true
		}
	}
	for flag, seen := range wantFlags {
		if !seen {
			t.Errorf("missing expected alert flag %q in %v", flag, trap.AlertFlags)
		}
	}

	// Hypothermia check + BaitReset
	err = ApplyTelemetryUpdate(trap, TelemetryUpdateInput{
		DoorClosed:         false,
		InternalTempC:      1.0,
		BatteryPct:         90,
		RssiDbm:            -65,
		AccelerometerPeakG: 0.1,
		BaitReset:          true,
		BaitType:           "ROTISSERIE_CHICKEN",
	}, now.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trap.Status != domain.TrapStatusThermalLockout {
		t.Errorf("expected status %q on hypothermia, got %q", domain.TrapStatusThermalLockout, trap.Status)
	}
	if trap.BaitType != "ROTISSERIE_CHICKEN" || trap.BaitFreshnessPct != 100 {
		t.Errorf("expected bait reset to ROTISSERIE_CHICKEN at 100%%, got %s %d%%", trap.BaitType, trap.BaitFreshnessPct)
	}
	hasHypo := false
	for _, f := range trap.AlertFlags {
		if f == domain.TrapAlertHypothermiaHazard {
			hasHypo = true
		}
	}
	if !hasHypo {
		t.Errorf("expected HYPOTHERMIA_FREEZE_HAZARD in %v", trap.AlertFlags)
	}
}

func TestDispatchAndResolveTrapInspection(t *testing.T) {
	now := time.Date(2026, 10, 18, 16, 0, 0, 0, time.UTC)
	trap := baseTestTrap(now)

	// Trip the trap first
	_ = ApplyTelemetryUpdate(trap, TelemetryUpdateInput{
		DoorClosed:               true,
		InternalTempC:            20.0,
		BatteryPct:               85,
		RssiDbm:                  -64,
		AccelerometerPeakG:       1.9,
		EstimatedCaptureWeightKg: 4.4,
	}, now)

	// Invalid dispatch inputs
	if err := DispatchTrapResponder(nil, DispatchInput{ResponderID: "r1", ResponderName: "Maya", ETAMinutes: 5}, now); err == nil {
		t.Errorf("expected error for nil trap")
	}
	if err := DispatchTrapResponder(trap, DispatchInput{ResponderID: "", ResponderName: "Maya", ETAMinutes: 5}, now); err == nil {
		t.Errorf("expected error for empty ResponderID")
	}
	if err := DispatchTrapResponder(trap, DispatchInput{ResponderID: "r1", ResponderName: "Maya", ETAMinutes: 0}, now); err == nil {
		t.Errorf("expected error for non-positive ETAMinutes")
	}

	// Valid dispatch
	if err := DispatchTrapResponder(trap, DispatchInput{
		ResponderID:   "resp-maya-07",
		ResponderName: "Maya Lin",
		ETAMinutes:    6,
	}, now.Add(1*time.Minute)); err != nil {
		t.Fatalf("unexpected dispatch error: %v", err)
	}
	if trap.Status != domain.TrapStatusResponderDispatched {
		t.Errorf("expected status %q, got %q", domain.TrapStatusResponderDispatched, trap.Status)
	}
	if trap.AssignedResponderID != "resp-maya-07" || trap.ResponderETAMinutes != 6 {
		t.Errorf("unexpected responder fields: %+v", trap)
	}

	// Resolve as bycatch released & re-armed
	if err := ResolveTrapInspection(trap, OutcomeBycatchReleased, "Adult opossum unharmed; released and re-baited", now.Add(8*time.Minute)); err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if trap.Status != domain.TrapStatusArmedActive || trap.DoorClosed || trap.TrippedAt != nil || trap.CheckSLADeadline != nil {
		t.Errorf("expected trap reset to ARMED_ACTIVE with open door, got %+v", trap)
	}
	if trap.AssignedResponderID != "" || trap.ResponderETAMinutes != 0 {
		t.Errorf("expected responder cleared after bycatch release, got %+v", trap)
	}

	// Resolve as thermal shaded re-armed
	trap.Status = domain.TrapStatusThermalLockout
	trap.InternalTempC = 35.0
	if err := ResolveTrapInspection(trap, OutcomeThermalShadedRearmed, "Installed reflective tarp canopy over culvert cage", now.Add(15*time.Minute)); err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if trap.Status != domain.TrapStatusArmedActive || trap.InternalTempC != 22.0 || trap.BaitFreshnessPct != 100 {
		t.Errorf("expected thermal shaded re-arm to reset temp to 22.0C and 100%% bait, got %+v", trap)
	}

	// Resolve as target pet captured
	if err := ResolveTrapInspection(trap, OutcomeTargetPetCaptured, "Microchip 985-112 verified on site; Whiskers calm in carrier", now.Add(30*time.Minute)); err != nil {
		t.Fatalf("unexpected resolve error: %v", err)
	}
	if trap.Status != domain.TrapStatusTargetCaptured || !trap.DoorClosed || len(trap.AlertFlags) != 0 {
		t.Errorf("expected TARGET_CAPTURED_SECURED with closed door and cleared alerts, got %+v", trap)
	}
	if !strings.Contains(trap.TelemetrySummary, "TARGET PET SECURED (Whiskers)") {
		t.Errorf("unexpected summary: %q", trap.TelemetrySummary)
	}

	// Invalid resolution calls
	if err := ResolveTrapInspection(nil, OutcomeTargetPetCaptured, "notes", now); err == nil {
		t.Errorf("expected error for nil trap")
	}
	if err := ResolveTrapInspection(trap, OutcomeTargetPetCaptured, "   ", now); err == nil {
		t.Errorf("expected error for blank notes")
	}
	if err := ResolveTrapInspection(trap, ResolutionOutcome("UNKNOWN"), "notes", now); err == nil {
		t.Errorf("expected error for unsupported outcome")
	}
}
