package beacon_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/beacon"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEstimateDistance(t *testing.T) {
	// At calibrated txPower (-59 dBm), distance at 1m should equal 1.0m
	dist1m := beacon.EstimateDistance(-59, -59, 2.5)
	if math.Abs(dist1m-1.0) > 0.05 {
		t.Errorf("expected ~1.0m, got %.2fm", dist1m)
	}

	// At -74 dBm (15 dB drop, ~2.5 exponent), distance should be approx 3.98m
	distNear := beacon.EstimateDistance(-74, -59, 2.5)
	if distNear < 3.0 || distNear > 5.0 {
		t.Errorf("expected between 3m and 5m, got %.2fm", distNear)
	}

	// Clamping: very strong RSSI clamped to 0.1m
	distStrong := beacon.EstimateDistance(-20, -59, 2.5)
	if distStrong != 0.1 {
		t.Errorf("expected clamped 0.1m for strong signal, got %.2fm", distStrong)
	}

	// Clamping: very weak RSSI clamped to 100.0m
	distWeak := beacon.EstimateDistance(-120, -59, 2.5)
	if distWeak != 100.0 {
		t.Errorf("expected clamped 100.0m for weak signal, got %.2fm", distWeak)
	}

	// Path loss exponent <= 0 defaults to 2.5
	distDefaultExp := beacon.EstimateDistance(-59, -59, 0)
	if math.Abs(distDefaultExp-1.0) > 0.05 {
		t.Errorf("expected ~1.0m with default exponent, got %.2fm", distDefaultExp)
	}

	distNegExp := beacon.EstimateDistance(-59, -59, -1.5)
	if math.Abs(distNegExp-1.0) > 0.05 {
		t.Errorf("expected ~1.0m with negative exponent falling back to default, got %.2fm", distNegExp)
	}

	// Proximity classifications
	if beacon.DetermineProximity(0.8, -55) != beacon.ProximityImmediate {
		t.Errorf("expected immediate proximity for 0.8m")
	}
	if beacon.DetermineProximity(3.2, -68) != beacon.ProximityNear {
		t.Errorf("expected near proximity for 3.2m")
	}
	if beacon.DetermineProximity(14.0, -82) != beacon.ProximityFar {
		t.Errorf("expected far proximity for 14.0m")
	}
	if beacon.DetermineProximity(45.0, -95) != beacon.ProximityOutOfRange {
		t.Errorf("expected out of range for 45.0m")
	}
}

func TestTriangulateBeacon_Empty(t *testing.T) {
	res := beacon.TriangulateBeacon(nil)
	if res.ObservationCount != 0 {
		t.Errorf("expected 0 observations for nil pings, got %d", res.ObservationCount)
	}

	res2 := beacon.TriangulateBeacon([]beacon.BeaconPing{})
	if res2.ObservationCount != 0 {
		t.Errorf("expected 0 observations for empty pings, got %d", res2.ObservationCount)
	}
}

func TestTriangulateBeacon_SinglePing(t *testing.T) {
	now := time.Now().UTC()
	pings := []beacon.BeaconPing{
		{
			PingID:         "p1",
			PetID:          "pet-123",
			VolunteerAlias: "Volunteer Alpha",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
			RSSI:           -68,
			TxPower1m:      -59,
			DistanceMeters: 3.2,
			RecordedAt:     now,
		},
	}

	res := beacon.TriangulateBeacon(pings)
	if res.ObservationCount != 1 {
		t.Fatalf("expected 1 observation, got %d", res.ObservationCount)
	}
	if math.Abs(res.EstimatedCoordinates.Latitude-47.6062) > 1e-5 {
		t.Errorf("expected latitude 47.6062, got %f", res.EstimatedCoordinates.Latitude)
	}
	if math.Abs(res.EstimatedCoordinates.Longitude-(-122.3321)) > 1e-5 {
		t.Errorf("expected longitude -122.3321, got %f", res.EstimatedCoordinates.Longitude)
	}
	if res.AccuracyRadiusMeters != 3.2 {
		t.Errorf("expected accuracy radius 3.2, got %f", res.AccuracyRadiusMeters)
	}
	if res.ConfidenceScore != 0.5 {
		t.Errorf("expected confidence 0.5 for single ping, got %f", res.ConfidenceScore)
	}
	if res.LastObservedAt != now {
		t.Errorf("expected LastObservedAt %v, got %v", now, res.LastObservedAt)
	}
	if res.Proximity != beacon.ProximityNear {
		t.Errorf("expected proximity near, got %v", res.Proximity)
	}
}

func TestTriangulateBeacon_TwoPings(t *testing.T) {
	now := time.Now().UTC()
	pings := []beacon.BeaconPing{
		{
			PingID:         "p1",
			PetID:          "pet-123",
			VolunteerAlias: "Volunteer Alpha",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6000, Longitude: -122.3300},
			RSSI:           -65,
			TxPower1m:      -59,
			DistanceMeters: 4.0,
			RecordedAt:     now,
		},
		{
			PingID:         "p2",
			PetID:          "pet-123",
			VolunteerAlias: "Volunteer Beta",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6002, Longitude: -122.3300},
			RSSI:           -60,
			TxPower1m:      -59,
			DistanceMeters: 2.0,
			RecordedAt:     now.Add(5 * time.Second),
		},
	}

	res := beacon.TriangulateBeacon(pings)
	if res.ObservationCount != 2 {
		t.Fatalf("expected 2 observations, got %d", res.ObservationCount)
	}
	// Inverse-variance weights: w1 = 1/(4^2) = 1/16 = 0.0625; w2 = 1/(2^2) = 1/4 = 0.25
	expectedLat := ((1.0/16.0)*47.6000 + (1.0/4.0)*47.6002) / ((1.0 / 16.0) + (1.0 / 4.0))
	if math.Abs(res.EstimatedCoordinates.Latitude-expectedLat) > 1e-5 {
		t.Errorf("expected latitude %f, got %f", expectedLat, res.EstimatedCoordinates.Latitude)
	}
	expectedAccuracy := 2.0 * 0.85
	if math.Abs(res.AccuracyRadiusMeters-expectedAccuracy) > 1e-4 {
		t.Errorf("expected accuracy %f, got %f", expectedAccuracy, res.AccuracyRadiusMeters)
	}
	if res.ConfidenceScore != 0.75 {
		t.Errorf("expected confidence 0.75, got %f", res.ConfidenceScore)
	}
	if res.LastObservedAt != now.Add(5*time.Second) {
		t.Errorf("expected latest timestamp %v, got %v", now.Add(5*time.Second), res.LastObservedAt)
	}
}

func TestTriangulateBeacon_MultiPing_LeastSquares(t *testing.T) {
	now := time.Now().UTC()
	pings := []beacon.BeaconPing{
		{
			PingID:         "p1",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6000, Longitude: -122.3300},
			RSSI:           -65,
			TxPower1m:      -59,
			DistanceMeters: 5.0,
			RecordedAt:     now,
		},
		{
			PingID:         "p2",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6001, Longitude: -122.3300},
			RSSI:           -62,
			TxPower1m:      -59,
			DistanceMeters: 3.0,
			RecordedAt:     now.Add(10 * time.Second),
		},
		{
			PingID:         "p3",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.60005, Longitude: -122.3301},
			RSSI:           -60,
			TxPower1m:      -59,
			DistanceMeters: 2.0,
			RecordedAt:     now.Add(20 * time.Second),
		},
	}

	res := beacon.TriangulateBeacon(pings)
	if res.ObservationCount != 3 {
		t.Fatalf("expected 3 observations, got %d", res.ObservationCount)
	}
	if res.ConfidenceScore < 0.7 {
		t.Errorf("expected high confidence score for 3 pings, got %f", res.ConfidenceScore)
	}
	if res.EstimatedCoordinates.Latitude < 47.599 || res.EstimatedCoordinates.Latitude > 47.601 {
		t.Errorf("estimated latitude out of bounds: %f", res.EstimatedCoordinates.Latitude)
	}
	if res.EstimatedCoordinates.Longitude < -122.331 || res.EstimatedCoordinates.Longitude > -122.329 {
		t.Errorf("estimated longitude out of bounds: %f", res.EstimatedCoordinates.Longitude)
	}
	if res.AccuracyRadiusMeters <= 0 || res.AccuracyRadiusMeters > 10.0 {
		t.Errorf("unexpected accuracy radius: %f", res.AccuracyRadiusMeters)
	}
}

func TestTriangulateBeacon_CappedAt10RecentPings(t *testing.T) {
	baseTime := time.Now().UTC()
	var pings []beacon.BeaconPing
	for i := 0; i < 15; i++ {
		pings = append(pings, beacon.BeaconPing{
			PingID:         string(rune('a' + i)),
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6000 + float64(i)*0.0001, Longitude: -122.3300},
			RSSI:           -65,
			TxPower1m:      -59,
			DistanceMeters: 4.0,
			RecordedAt:     baseTime.Add(time.Duration(i) * time.Minute),
		})
	}

	res := beacon.TriangulateBeacon(pings)
	if res.ObservationCount != 10 {
		t.Fatalf("expected capped 10 observations, got %d", res.ObservationCount)
	}
	expectedLatest := baseTime.Add(14 * time.Minute)
	if res.LastObservedAt != expectedLatest {
		t.Errorf("expected latest timestamp %v, got %v", expectedLatest, res.LastObservedAt)
	}
}

func TestTriangulateBeacon_CollinearOrIdentical(t *testing.T) {
	now := time.Now().UTC()
	pings := []beacon.BeaconPing{
		{
			PingID:         "p1",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6000, Longitude: -122.3300},
			RSSI:           -65,
			TxPower1m:      -59,
			DistanceMeters: 3.0,
			RecordedAt:     now,
		},
		{
			PingID:         "p2",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6000, Longitude: -122.3300},
			RSSI:           -65,
			TxPower1m:      -59,
			DistanceMeters: 3.0,
			RecordedAt:     now.Add(1 * time.Second),
		},
		{
			PingID:         "p3",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6000, Longitude: -122.3300},
			RSSI:           -65,
			TxPower1m:      -59,
			DistanceMeters: 3.0,
			RecordedAt:     now.Add(2 * time.Second),
		},
	}

	res := beacon.TriangulateBeacon(pings)
	if res.ObservationCount != 3 {
		t.Fatalf("expected 3 observations, got %d", res.ObservationCount)
	}
	if math.Abs(res.EstimatedCoordinates.Latitude-47.6000) > 1e-5 {
		t.Errorf("expected latitude 47.6000, got %f", res.EstimatedCoordinates.Latitude)
	}
	if math.Abs(res.EstimatedCoordinates.Longitude-(-122.3300)) > 1e-5 {
		t.Errorf("expected longitude -122.3300, got %f", res.EstimatedCoordinates.Longitude)
	}
}

func TestTriangulateBeacon_RaceSafety(t *testing.T) {
	now := time.Now().UTC()
	pings := []beacon.BeaconPing{
		{
			PingID:         "p1",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6000, Longitude: -122.3300},
			RSSI:           -65,
			TxPower1m:      -59,
			DistanceMeters: 5.0,
			RecordedAt:     now,
		},
		{
			PingID:         "p2",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6001, Longitude: -122.3300},
			RSSI:           -60,
			TxPower1m:      -59,
			DistanceMeters: 2.0,
			RecordedAt:     now.Add(5 * time.Second),
		},
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = beacon.EstimateDistance(-70, -59, 2.5)
			_ = beacon.DetermineProximity(4.5, -72)
			_ = beacon.TriangulateBeacon(pings)
		}()
	}
	wg.Wait()
}
