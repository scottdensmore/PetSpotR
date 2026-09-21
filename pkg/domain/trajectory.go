package domain

import "time"

// BarrierType represents the physical classification of a geographic obstacle or corridor.
type BarrierType string

const (
	// BarrierTypeWaterway represents rivers, lakes, and marine coastal boundaries.
	BarrierTypeWaterway BarrierType = "waterway"
	// BarrierTypeFreeway represents multi-lane interstates and high-speed divided highways.
	BarrierTypeFreeway BarrierType = "freeway"
	// BarrierTypeGreenway represents parks, greenways, and wooded nature preserves.
	BarrierTypeGreenway BarrierType = "greenway"
	// BarrierTypeSteepSlope represents steep topographical grades and bluffs.
	BarrierTypeSteepSlope BarrierType = "steep_slope"
)

// TerrainFeature describes a physical obstacle or attraction corridor on the friction surface.
type TerrainFeature struct {
	ID           string          `json:"id"`
	Type         BarrierType     `json:"type"`
	Name         string          `json:"name"`
	FrictionCost float64         `json:"frictionCost"`
	Geometry     []LocationPoint `json:"geometry"`
}

// IsochroneContour represents a closed cost-distance probability boundary.
type IsochroneContour struct {
	ProbabilityLevel   float64           `json:"probabilityLevel"` // 0.50, 0.75, 0.90
	Label              string            `json:"label"`
	ColorHex           string            `json:"colorHex"`
	PolygonCoordinates [][]LocationPoint `json:"polygonCoordinates"`
}

// HidingCluster represents a concentrated refuge or shelter zone inside attraction corridors.
type HidingCluster struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Centroid        LocationPoint `json:"centroid"`
	RadiusMeters    float64       `json:"radiusMeters"`
	AttractionScore float64       `json:"attractionScore"`
	Description     string        `json:"description"`
}

// PredictiveTrajectoryResult encapsulates anisotropic time-decayed isochrones, encountered barriers,
// and shelter clusters for a lost animal.
type PredictiveTrajectoryResult struct {
	LostPetID           string             `json:"lostPetId"`
	Species             string             `json:"species"`
	GeneratedAt         time.Time          `json:"generatedAt"`
	ElapsedHours        float64            `json:"elapsedHours"`
	LastSightingPoint   LocationPoint      `json:"lastSightingPoint"`
	HeadingDegrees      float64            `json:"headingDegrees"`
	Isochrones          []IsochroneContour `json:"isochrones"`
	BarriersEncountered []TerrainFeature   `json:"barriersEncountered"`
	HidingClusters      []HidingCluster    `json:"hidingClusters"`
}
