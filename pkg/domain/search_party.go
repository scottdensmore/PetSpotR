package domain

// SectorUrgencyLevel represents the triage priority of a search sector based on isochrone overlap.
type SectorUrgencyLevel string

const (
	// SectorUrgencyCritical indicates a sector overlapping the 50% core isochrone.
	SectorUrgencyCritical SectorUrgencyLevel = "CRITICAL"
	// SectorUrgencyHigh indicates a sector overlapping the 75% search envelope.
	SectorUrgencyHigh SectorUrgencyLevel = "HIGH"
	// SectorUrgencyStandard indicates a standard priority search sector.
	SectorUrgencyStandard SectorUrgencyLevel = "STANDARD"
)

// SearchSector defines a specific spatial polygon assigned to search party volunteers.
type SearchSector struct {
	SectorID         string             `json:"sectorId"`
	Name             string             `json:"name,omitempty"`
	BoundingPolygon  []LocationPoint    `json:"boundingPolygon,omitempty"`
	PolygonPoints    []LocationPoint    `json:"polygonPoints,omitempty"`
	Status           string             `json:"status,omitempty"`
	TotalAreaSqM     float64            `json:"totalAreaSqM,omitempty"`
	PriorityScore    float64            `json:"priorityScore,omitempty"`
	UrgencyLevel     SectorUrgencyLevel `json:"urgencyLevel,omitempty"`
	HidingClusterIDs []string           `json:"hidingClusterIds,omitempty"`
}
