package searchparty

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	earthRadiusMeters = 6371000.0
	metersPerMile     = 1609.344
)

// SectorStatus represents the current state of a geo-fenced search sector.
type SectorStatus string

const (
	SectorStatusUnassigned       SectorStatus = "unassigned"
	SectorStatusActiveSearch     SectorStatus = "active_search"
	SectorStatusCleared          SectorStatus = "cleared"
	SectorStatusSightingReported SectorStatus = "sighting_reported"
)

// Validate checks that the SectorStatus is one of the valid enum values.
func (s SectorStatus) Validate() error {
	switch s {
	case SectorStatusUnassigned, SectorStatusActiveSearch, SectorStatusCleared, SectorStatusSightingReported:
		return nil
	default:
		return fmt.Errorf("searchparty: invalid sector status %q", s)
	}
}

// SearchSector defines a specific spatial polygon assigned to volunteers.
type SearchSector struct {
	SectorID      string                 `json:"sectorId"`
	Name          string                 `json:"name"`
	PolygonPoints []domain.LocationPoint `json:"polygonPoints"`
	Status        SectorStatus           `json:"status"`
	PriorityScore float64                `json:"priorityScore"`
	TotalAreaSqM  float64                `json:"totalAreaSqM"`
}

// Validate checks required fields for SearchSector.
func (s SearchSector) Validate() error {
	if strings.TrimSpace(s.SectorID) == "" {
		return errors.New("searchparty: sector ID is required")
	}
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("searchparty: sector name is required")
	}
	if len(s.PolygonPoints) < 3 {
		return fmt.Errorf("searchparty: polygon must have at least 3 points, got %d", len(s.PolygonPoints))
	}
	for i, pt := range s.PolygonPoints {
		if err := pt.Validate(); err != nil {
			return fmt.Errorf("searchparty: invalid polygon point %d: %w", i, err)
		}
	}
	if err := s.Status.Validate(); err != nil {
		return err
	}
	if s.TotalAreaSqM <= 0 {
		return errors.New("searchparty: total area must be positive")
	}
	return nil
}

// SectorAssignment tracks a volunteer's claim on a sector.
type SectorAssignment struct {
	AssignmentID   string       `json:"assignmentId"`
	SectorID       string       `json:"sectorId"`
	VolunteerAlias string       `json:"volunteerAlias"`
	ClaimedAt      time.Time    `json:"claimedAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
	Status         SectorStatus `json:"status"`
	ClearanceNotes string       `json:"clearanceNotes,omitempty"`
}

// Validate checks required fields for SectorAssignment.
func (a SectorAssignment) Validate() error {
	if strings.TrimSpace(a.AssignmentID) == "" {
		return errors.New("searchparty: assignment ID is required")
	}
	if strings.TrimSpace(a.SectorID) == "" {
		return errors.New("searchparty: sector ID is required")
	}
	if strings.TrimSpace(a.VolunteerAlias) == "" {
		return errors.New("searchparty: volunteer alias is required")
	}
	if a.ClaimedAt.IsZero() {
		return errors.New("searchparty: claimedAt timestamp is required")
	}
	if err := a.Status.Validate(); err != nil {
		return err
	}
	return nil
}

// SearchParty encapsulates the entire search operation for a lost pet.
type SearchParty struct {
	PartyID               string               `json:"partyId"`
	LostPetID             string               `json:"lostPetId"`
	CenterCoordinates     domain.LocationPoint `json:"centerCoordinates"`
	RadiusMeters          float64              `json:"radiusMeters"`
	CreatedAt             time.Time            `json:"createdAt"`
	Sectors               []SearchSector       `json:"sectors"`
	ActiveAssignments     []SectorAssignment   `json:"activeAssignments"`
	CoveragePercentage    float64              `json:"coveragePercentage"`
	ActiveVolunteersCount int                  `json:"activeVolunteersCount"`
}

// Validate checks required fields for SearchParty.
func (p SearchParty) Validate() error {
	if strings.TrimSpace(p.PartyID) == "" {
		return errors.New("searchparty: party ID is required")
	}
	if strings.TrimSpace(p.LostPetID) == "" {
		return errors.New("searchparty: lost pet ID is required")
	}
	if err := p.CenterCoordinates.Validate(); err != nil {
		return fmt.Errorf("searchparty: invalid center coordinates: %w", err)
	}
	if p.RadiusMeters <= 0 {
		return errors.New("searchparty: radius meters must be positive")
	}
	for i, s := range p.Sectors {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("searchparty: invalid sector at %d: %w", i, err)
		}
	}
	return nil
}

// CalculateCoverage computes the percentage of total perimeter area that has been cleared.
func (p SearchParty) CalculateCoverage() float64 {
	if len(p.Sectors) == 0 {
		return 0.0
	}

	var totalArea float64
	var clearedArea float64
	var clearedCount int

	for _, s := range p.Sectors {
		totalArea += s.TotalAreaSqM
		if s.Status == SectorStatusCleared {
			clearedArea += s.TotalAreaSqM
			clearedCount++
		}
	}

	if totalArea <= 0 {
		return float64(clearedCount) / float64(len(p.Sectors)) * 100.0
	}

	coverage := (clearedArea / totalArea) * 100.0
	return math.Round(coverage*100.0) / 100.0
}

var cardinalPoints = [16]string{
	"N", "NNE", "NE", "ENE",
	"E", "ESE", "SE", "SSE",
	"S", "SSW", "SW", "WSW",
	"W", "WNW", "NW", "NNW",
}

var natoAlphabet = []string{
	"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel",
	"India", "Juliet", "Kilo", "Lima", "Mike", "November", "Oscar", "Papa",
}

func cardinalHeading(bearingDeg float64) string {
	bearingDeg = math.Mod(bearingDeg, 360.0)
	if bearingDeg < 0 {
		bearingDeg += 360.0
	}
	index := int(math.Floor((bearingDeg+11.25)/22.5)) % 16
	return cardinalPoints[index]
}

func sectorDesignation(index int) string {
	if index >= 0 && index < len(natoAlphabet) {
		return natoAlphabet[index]
	}
	return fmt.Sprintf("%d", index+1)
}

// destinationPoint computes the destination coordinates along a great circle arc.
func destinationPoint(start domain.LocationPoint, distanceMeters float64, bearingDegrees float64) domain.LocationPoint {
	lat1Rad := start.Latitude * math.Pi / 180.0
	lon1Rad := start.Longitude * math.Pi / 180.0
	bearingRad := bearingDegrees * math.Pi / 180.0
	distRatio := distanceMeters / earthRadiusMeters

	asinArg := math.Sin(lat1Rad)*math.Cos(distRatio) + math.Cos(lat1Rad)*math.Sin(distRatio)*math.Cos(bearingRad)
	if asinArg > 1.0 {
		asinArg = 1.0
	} else if asinArg < -1.0 {
		asinArg = -1.0
	}
	lat2Rad := math.Asin(asinArg)
	lon2Rad := lon1Rad + math.Atan2(math.Sin(bearingRad)*math.Sin(distRatio)*math.Cos(lat1Rad), math.Cos(distRatio)-math.Sin(lat1Rad)*math.Sin(lat2Rad))

	lat2 := lat2Rad * 180.0 / math.Pi
	lon2 := lon2Rad * 180.0 / math.Pi

	lon2 = math.Mod(lon2+540.0, 360.0) - 180.0

	return domain.LocationPoint{
		Latitude:  lat2,
		Longitude: lon2,
	}
}

// DecomposePerimeter decomposes a radial perimeter into distinct geo-fenced sectors (wedges).
func DecomposePerimeter(center *domain.LocationPoint, radiusMeters float64, sectorCount int) []SearchSector {
	if center == nil || radiusMeters <= 0 || sectorCount <= 0 {
		return []SearchSector{}
	}

	anglePerSector := 360.0 / float64(sectorCount)
	areaPerSector := (math.Pi * radiusMeters * radiusMeters) / float64(sectorCount)

	sectors := make([]SearchSector, sectorCount)
	for i := 0; i < sectorCount; i++ {
		startAngle := float64(i) * anglePerSector
		endAngle := float64(i+1) * anglePerSector
		midAngle := startAngle + (anglePerSector / 2.0)

		heading := cardinalHeading(midAngle)
		sectorName := sectorDesignation(i)
		name := fmt.Sprintf("Sector %s (%s)", sectorName, heading)

		const arcSegments = 8
		polygonPoints := make([]domain.LocationPoint, 0, arcSegments+3)
		polygonPoints = append(polygonPoints, *center)

		for s := 0; s <= arcSegments; s++ {
			bearing := startAngle + (float64(s)/float64(arcSegments))*(endAngle-startAngle)
			pt := destinationPoint(*center, radiusMeters, bearing)
			polygonPoints = append(polygonPoints, pt)
		}
		// Close polygon ring back to center
		polygonPoints = append(polygonPoints, *center)

		sectors[i] = SearchSector{
			SectorID:      fmt.Sprintf("sector-%d", i+1),
			Name:          name,
			PolygonPoints: polygonPoints,
			Status:        SectorStatusUnassigned,
			PriorityScore: 1.0,
			TotalAreaSqM:  areaPerSector,
		}
	}
	return sectors
}

// DecomposePerimeterWithBearing decomposes a radial perimeter and prioritizes sectors aligned with the given bearing.
func DecomposePerimeterWithBearing(center *domain.LocationPoint, radiusMeters float64, sectorCount int, bearingDegrees float64) []SearchSector {
	sectors := DecomposePerimeter(center, radiusMeters, sectorCount)
	if len(sectors) == 0 {
		return sectors
	}

	bearingDegrees = math.Mod(bearingDegrees, 360.0)
	if bearingDegrees < 0 {
		bearingDegrees += 360.0
	}

	anglePerSector := 360.0 / float64(len(sectors))
	for i := range sectors {
		midAngle := (float64(i) * anglePerSector) + (anglePerSector / 2.0)
		diff := math.Abs(midAngle - bearingDegrees)
		if diff > 180.0 {
			diff = 360.0 - diff
		}
		// Higher score for sectors closer to movement bearing (range 1.0 to 2.0)
		score := 1.0 + ((180.0 - diff) / 180.0)
		sectors[i].PriorityScore = math.Round(score*100.0) / 100.0
	}
	return sectors
}

// DecomposePerimeterMiles decomposes a search perimeter given a radius in miles.
func DecomposePerimeterMiles(center *domain.LocationPoint, radiusMiles float64, sectorCount int) []SearchSector {
	return DecomposePerimeter(center, radiusMiles*metersPerMile, sectorCount)
}
