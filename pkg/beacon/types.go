package beacon

import (
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// BeaconPing represents a single Bluetooth Low Energy collar beacon observation.
type BeaconPing struct {
	PingID         string               `json:"pingId"`
	PetID          string               `json:"petId"`
	VolunteerAlias string               `json:"volunteerAlias"`
	ObserverCoords domain.LocationPoint `json:"observerCoords"`
	RSSI           int                  `json:"rssi"`           // dBm
	TxPower1m      int                  `json:"txPower1m"`      // Measured power at 1 meter
	DistanceMeters float64              `json:"distanceMeters"` // Computed distance
	RecordedAt     time.Time            `json:"recordedAt"`
}

// ProximityZone categorizes distance and signal strength into proximity bands.
type ProximityZone string

const (
	ProximityImmediate  ProximityZone = "immediate"    // < 1.0m
	ProximityNear       ProximityZone = "near"         // 1.0m - 5.0m
	ProximityFar        ProximityZone = "far"          // 5.0m - 30.0m
	ProximityOutOfRange ProximityZone = "out_of_range" // >= 30.0m
)

// TriangulationResult represents the estimated position and confidence of a beacon.
type TriangulationResult struct {
	EstimatedCoordinates domain.LocationPoint `json:"estimatedCoordinates"`
	AccuracyRadiusMeters float64              `json:"accuracyRadiusMeters"`
	ObservationCount     int                  `json:"observationCount"`
	LastObservedAt       time.Time            `json:"lastObservedAt"`
	ConfidenceScore      float64              `json:"confidenceScore"` // 0.0 to 1.0
	Proximity            ProximityZone        `json:"proximity"`
}
