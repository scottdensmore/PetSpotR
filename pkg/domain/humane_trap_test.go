package domain

import (
	"testing"
	"time"
)

func validHumaneTrapDeployment() HumaneTrapDeployment {
	now := time.Date(2026, 10, 18, 4, 0, 0, 0, time.UTC)
	return HumaneTrapDeployment{
		TrapID:             "trap-ht-101",
		PetID:              "pet-2026-002",
		PetName:            "Luna",
		TargetSpecies:      "CAT",
		TargetWeightKg:     4.2,
		TrapLabel:          "Tomahawk Tru-Catch #404 - Creek Culvert",
		TrapType:           TrapTypeBoxGravity,
		Status:             TrapStatusArmedActive,
		Location:           GeoPoint{Latitude: 47.6612, Longitude: -122.4135},
		SectorID:           "SEC-NW-04",
		BaitType:           "SARDINES_IN_OIL",
		BaitPlacedAt:       now.Add(-1 * time.Hour),
		BaitFreshnessPct:   88,
		BaitState:          BaitStateFreshPeak,
		DoorClosed:         false,
		InternalTempC:      16.4,
		BatteryPct:         92,
		RssiDbm:            -68,
		AccelerometerPeakG: 0.1,
		AlertFlags:         []HumaneTrapAlertFlag{TrapAlertUrgentCheckSLA},
		TelemetrySummary:   "Armed & monitoring (16.4°C, bait freshness 88%).",
		CreatedAt:          now.Add(-2 * time.Hour),
		UpdatedAt:          now,
	}
}

func TestHumaneTrapEnums(t *testing.T) {
	t.Parallel()

	for _, tt := range []HumaneTrapType{TrapTypeBoxGravity, TrapTypeDropRemote, TrapTypeEnclosureCellular} {
		if !ValidHumaneTrapType(tt) {
			t.Errorf("expected ValidHumaneTrapType(%q) = true", tt)
		}
	}
	if ValidHumaneTrapType("INVALID_TRAP") {
		t.Error("expected ValidHumaneTrapType(INVALID_TRAP) = false")
	}

	for _, st := range []HumaneTrapStatus{
		TrapStatusArmedActive,
		TrapStatusDoorTrippedUrgent,
		TrapStatusResponderDispatched,
		TrapStatusTargetCaptured,
		TrapStatusThermalLockout,
	} {
		if !ValidHumaneTrapStatus(st) {
			t.Errorf("expected ValidHumaneTrapStatus(%q) = true", st)
		}
	}
	if ValidHumaneTrapStatus("INVALID_STATUS") {
		t.Error("expected ValidHumaneTrapStatus(INVALID_STATUS) = false")
	}

	for _, bs := range []BaitFreshnessState{BaitStateFreshPeak, BaitStateDegradingRefresh, BaitStateSpoiledExpired} {
		if !ValidBaitFreshnessState(bs) {
			t.Errorf("expected ValidBaitFreshnessState(%q) = true", bs)
		}
	}
	if ValidBaitFreshnessState("INVALID_BAIT") {
		t.Error("expected ValidBaitFreshnessState(INVALID_BAIT) = false")
	}

	for _, af := range []HumaneTrapAlertFlag{
		TrapAlertUrgentCheckSLA,
		TrapAlertHyperthermiaHazard,
		TrapAlertHypothermiaHazard,
		TrapAlertBaitSpoiled,
		TrapAlertLowBattery,
		TrapAlertBycatchWeight,
	} {
		if !ValidHumaneTrapAlertFlag(af) {
			t.Errorf("expected ValidHumaneTrapAlertFlag(%q) = true", af)
		}
	}
	if ValidHumaneTrapAlertFlag("INVALID_ALERT") {
		t.Error("expected ValidHumaneTrapAlertFlag(INVALID_ALERT) = false")
	}
}

func TestHumaneTrapDeploymentValidation(t *testing.T) {
	t.Parallel()

	base := validHumaneTrapDeployment()
	if err := base.Validate(); err != nil {
		t.Fatalf("expected valid deployment, got error: %v", err)
	}

	dispatched := validHumaneTrapDeployment()
	dispatched.Status = TrapStatusResponderDispatched
	dispatched.AssignedResponderID = "vol-sar-104"
	dispatched.AssignedResponderName = "Maya Lin (SAR Tech)"
	dispatched.ResponderETAMinutes = 6
	if err := dispatched.Validate(); err != nil {
		t.Fatalf("expected valid dispatched deployment, got error: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(d *HumaneTrapDeployment)
	}{
		{"empty TrapID", func(d *HumaneTrapDeployment) { d.TrapID = "   " }},
		{"empty PetID", func(d *HumaneTrapDeployment) { d.PetID = "" }},
		{"empty PetName", func(d *HumaneTrapDeployment) { d.PetName = " " }},
		{"empty TrapLabel", func(d *HumaneTrapDeployment) { d.TrapLabel = "" }},
		{"empty BaitType", func(d *HumaneTrapDeployment) { d.BaitType = "" }},
		{"empty TelemetrySummary", func(d *HumaneTrapDeployment) { d.TelemetrySummary = "" }},
		{"empty TargetSpecies", func(d *HumaneTrapDeployment) { d.TargetSpecies = "" }},
		{"lowercase TargetSpecies", func(d *HumaneTrapDeployment) { d.TargetSpecies = "cat" }},
		{"zero TargetWeightKg", func(d *HumaneTrapDeployment) { d.TargetWeightKg = 0 }},
		{"excessive TargetWeightKg", func(d *HumaneTrapDeployment) { d.TargetWeightKg = 120.1 }},
		{"invalid TrapType", func(d *HumaneTrapDeployment) { d.TrapType = "UNKNOWN" }},
		{"invalid Status", func(d *HumaneTrapDeployment) { d.Status = "UNKNOWN" }},
		{"invalid Location lat", func(d *HumaneTrapDeployment) { d.Location.Latitude = 95.0 }},
		{"invalid Location lng", func(d *HumaneTrapDeployment) { d.Location.Longitude = -185.0 }},
		{"negative BaitFreshnessPct", func(d *HumaneTrapDeployment) { d.BaitFreshnessPct = -1 }},
		{"over 100 BaitFreshnessPct", func(d *HumaneTrapDeployment) { d.BaitFreshnessPct = 101 }},
		{"invalid BaitState", func(d *HumaneTrapDeployment) { d.BaitState = "ROTTEN" }},
		{"too cold InternalTempC", func(d *HumaneTrapDeployment) { d.InternalTempC = -40.1 }},
		{"too hot InternalTempC", func(d *HumaneTrapDeployment) { d.InternalTempC = 70.1 }},
		{"negative BatteryPct", func(d *HumaneTrapDeployment) { d.BatteryPct = -1 }},
		{"over 100 BatteryPct", func(d *HumaneTrapDeployment) { d.BatteryPct = 101 }},
		{"too low RssiDbm", func(d *HumaneTrapDeployment) { d.RssiDbm = -131 }},
		{"too high RssiDbm", func(d *HumaneTrapDeployment) { d.RssiDbm = -19 }},
		{"negative AccelerometerPeakG", func(d *HumaneTrapDeployment) { d.AccelerometerPeakG = -0.1 }},
		{"invalid AlertFlag", func(d *HumaneTrapDeployment) {
			d.AlertFlags = []HumaneTrapAlertFlag{"BAD_FLAG"}
		}},
		{"dispatched missing responder ID", func(d *HumaneTrapDeployment) {
			d.Status = TrapStatusResponderDispatched
			d.AssignedResponderID = ""
			d.AssignedResponderName = "Maya Lin"
			d.ResponderETAMinutes = 6
		}},
		{"dispatched missing responder name", func(d *HumaneTrapDeployment) {
			d.Status = TrapStatusResponderDispatched
			d.AssignedResponderID = "vol-1"
			d.AssignedResponderName = " "
			d.ResponderETAMinutes = 6
		}},
		{"dispatched non-positive ETA", func(d *HumaneTrapDeployment) {
			d.Status = TrapStatusResponderDispatched
			d.AssignedResponderID = "vol-1"
			d.AssignedResponderName = "Maya Lin"
			d.ResponderETAMinutes = 0
		}},
		{"zero BaitPlacedAt", func(d *HumaneTrapDeployment) { d.BaitPlacedAt = time.Time{} }},
		{"zero CreatedAt", func(d *HumaneTrapDeployment) { d.CreatedAt = time.Time{} }},
		{"zero UpdatedAt", func(d *HumaneTrapDeployment) { d.UpdatedAt = time.Time{} }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validHumaneTrapDeployment()
			tc.mutate(&d)
			if err := d.Validate(); err == nil {
				t.Errorf("expected validation error for %s, got nil", tc.name)
			}
		})
	}
}
