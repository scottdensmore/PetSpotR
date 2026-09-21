package beacon

import "math"

const (
	// DefaultPathLossExponent represents standard suburban / neighborhood attenuation.
	DefaultPathLossExponent = 2.5

	// MinDistanceMeters is the lower clamp boundary for estimated distance.
	MinDistanceMeters = 0.1

	// MaxDistanceMeters is the upper clamp boundary for estimated distance.
	MaxDistanceMeters = 100.0
)

// EstimateDistance computes estimated distance in meters using the log-distance path loss model:
// d = 10^((txPower1m - rssi) / (10 * n))
// Distance is clamped between 0.1m and 100.0m.
// If pathLossExponent <= 0, DefaultPathLossExponent (2.5) is used.
func EstimateDistance(rssi int, txPower1m int, pathLossExponent float64) float64 {
	if pathLossExponent <= 0 {
		pathLossExponent = DefaultPathLossExponent
	}
	ratio := float64(txPower1m-rssi) / (10.0 * pathLossExponent)
	d := math.Pow(10.0, ratio)
	if d < MinDistanceMeters {
		return MinDistanceMeters
	}
	if d > MaxDistanceMeters {
		return MaxDistanceMeters
	}
	return d
}

// DetermineProximity categorizes the current signal proximity zone based on distance and RSSI.
// Immediate: < 1.0m or rssi >= -60
// Near: 1.0m - 5.0m or -75 <= rssi < -60
// Far: 5.0m - 30.0m or -90 <= rssi < -75
// OutOfRange: >= 30.0m or rssi < -90
func DetermineProximity(distance float64, rssi int) ProximityZone {
	if distance < 1.0 || rssi >= -60 {
		return ProximityImmediate
	}
	if distance < 5.0 || rssi >= -75 {
		return ProximityNear
	}
	if distance < 30.0 || rssi >= -90 {
		return ProximityFar
	}
	return ProximityOutOfRange
}
