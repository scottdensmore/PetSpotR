package sighting

import (
	"math"
	"sort"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	earthRadiusMeters = 6371000.0
	metersPerMile     = 1609.344
)

var cardinalPoints = [16]string{
	"N", "NNE", "NE", "ENE",
	"E", "ESE", "SE", "SSE",
	"S", "SSW", "SW", "WSW",
	"W", "WNW", "NW", "NNW",
}

// TrajectoryLeg represents the movement vector between two consecutive sightings.
type TrajectoryLeg struct {
	FromSightingID  string  `json:"fromSightingId"`
	ToSightingID    string  `json:"toSightingId"`
	DistanceMeters  float64 `json:"distanceMeters"`
	DistanceMiles   float64 `json:"distanceMiles"`
	ElapsedDuration float64 `json:"elapsedSeconds"`
	SpeedMph        float64 `json:"speedMph"`
	BearingDegrees  float64 `json:"bearingDegrees"`  // 0 - 360 degrees
	CardinalHeading string  `json:"cardinalHeading"` // e.g. "NE", "SSE"
}

// SearchPerimeter defines the predictive radial boundary centered on the latest sighting.
type SearchPerimeter struct {
	CenterCoordinates domain.LocationPoint `json:"centerCoordinates"`
	RadiusMeters      float64              `json:"radiusMeters"`
	RadiusMiles       float64              `json:"radiusMiles"`
	ConfidenceLevel   string               `json:"confidenceLevel"` // "High", "Expanding", "Wide"
}

// TrajectoryAnalysis encapsulates the chronological path and motion metrics of a lost pet.
type TrajectoryAnalysis struct {
	LostPetID          string                     `json:"lostPetId"`
	GeneratedAt        time.Time                  `json:"generatedAt"`
	OriginLocation     *domain.LocationPoint      `json:"originLocation,omitempty"` // Initial last seen spot from LostPetRecord
	SightingsCount     int                        `json:"sightingsCount"`
	OrderedSightings   []domain.PetSightingRecord `json:"orderedSightings"`
	Legs               []TrajectoryLeg            `json:"legs"`
	TotalDistanceMiles float64                    `json:"totalDistanceMiles"`
	EstimatedPerimeter *SearchPerimeter           `json:"estimatedPerimeter,omitempty"`
}

// CalculateTrajectory processes chronological sightings and computes movement velocity,
// compass bearing, and predictive search perimeter.
func CalculateTrajectory(
	lostPetID string,
	origin *domain.LocationPoint,
	originTime time.Time,
	sightings []domain.PetSightingRecord,
) TrajectoryAnalysis {
	return calculateTrajectoryWithTime(lostPetID, origin, originTime, sightings, time.Now().UTC())
}

func calculateTrajectoryWithTime(
	lostPetID string,
	origin *domain.LocationPoint,
	originTime time.Time,
	sightings []domain.PetSightingRecord,
	now time.Time,
) TrajectoryAnalysis {
	orderedSightings := make([]domain.PetSightingRecord, 0, len(sightings))
	for _, s := range sightings {
		if s.Coordinates != nil {
			orderedSightings = append(orderedSightings, s)
		}
	}

	sort.SliceStable(orderedSightings, func(i, j int) bool {
		return orderedSightings[i].SightedAt.Before(orderedSightings[j].SightedAt)
	})

	legs := make([]TrajectoryLeg, 0)
	var totalDistanceMiles float64

	var prevPoint *domain.LocationPoint
	var prevTime time.Time
	var prevID string

	if origin != nil {
		prevPoint = origin
		prevTime = originTime
		prevID = "origin"
	}

	for _, s := range orderedSightings {
		if prevPoint != nil {
			distMeters := haversineDistanceMeters(*prevPoint, *s.Coordinates)
			distMiles := distMeters / metersPerMile
			var elapsedSec float64
			if !prevTime.IsZero() && !s.SightedAt.IsZero() {
				elapsedSec = s.SightedAt.Sub(prevTime).Seconds()
			}
			var speedMph float64
			if elapsedSec > 0 {
				speedMph = distMiles / (elapsedSec / 3600.0)
			}
			bearingDeg := initialBearingDegrees(*prevPoint, *s.Coordinates)
			heading := cardinalHeading(bearingDeg)

			leg := TrajectoryLeg{
				FromSightingID:  prevID,
				ToSightingID:    s.SightingID,
				DistanceMeters:  distMeters,
				DistanceMiles:   distMiles,
				ElapsedDuration: elapsedSec,
				SpeedMph:        speedMph,
				BearingDegrees:  bearingDeg,
				CardinalHeading: heading,
			}
			legs = append(legs, leg)
			totalDistanceMiles += distMiles
		}

		p := *s.Coordinates
		prevPoint = &p
		prevTime = s.SightedAt
		prevID = s.SightingID
	}

	var perimeter *SearchPerimeter
	if len(orderedSightings) > 0 {
		latest := orderedSightings[len(orderedSightings)-1]
		perimeter = computePerimeter(*latest.Coordinates, latest.SightedAt, now)
	} else if origin != nil {
		perimeter = computePerimeter(*origin, originTime, now)
	}

	return TrajectoryAnalysis{
		LostPetID:          lostPetID,
		GeneratedAt:        now,
		OriginLocation:     origin,
		SightingsCount:     len(orderedSightings),
		OrderedSightings:   orderedSightings,
		Legs:               legs,
		TotalDistanceMiles: totalDistanceMiles,
		EstimatedPerimeter: perimeter,
	}
}

func computePerimeter(center domain.LocationPoint, lastTime, now time.Time) *SearchPerimeter {
	var elapsedHours float64
	if !lastTime.IsZero() {
		diff := now.Sub(lastTime)
		if diff > 0 {
			elapsedHours = diff.Hours()
		}
	}

	var radiusMiles float64
	var confidence string

	if elapsedHours <= 2.0 {
		radiusMiles = 0.5
		confidence = "High"
	} else {
		radiusMiles = 0.5 + ((elapsedHours-2.0)/2.0)*0.25
		if radiusMiles >= 3.0 {
			radiusMiles = 3.0
			confidence = "Wide"
		} else {
			confidence = "Expanding"
		}
	}

	return &SearchPerimeter{
		CenterCoordinates: center,
		RadiusMeters:      radiusMiles * metersPerMile,
		RadiusMiles:       radiusMiles,
		ConfidenceLevel:   confidence,
	}
}

func haversineDistanceMeters(p1, p2 domain.LocationPoint) float64 {
	lat1Rad := p1.Latitude * math.Pi / 180.0
	lat2Rad := p2.Latitude * math.Pi / 180.0
	deltaLatRad := (p2.Latitude - p1.Latitude) * math.Pi / 180.0
	deltaLonRad := (p2.Longitude - p1.Longitude) * math.Pi / 180.0

	a := math.Sin(deltaLatRad/2.0)*math.Sin(deltaLatRad/2.0) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLonRad/2.0)*math.Sin(deltaLonRad/2.0)

	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(math.Max(0.0, 1.0-a)))
	return earthRadiusMeters * c
}

func initialBearingDegrees(p1, p2 domain.LocationPoint) float64 {
	lat1Rad := p1.Latitude * math.Pi / 180.0
	lat2Rad := p2.Latitude * math.Pi / 180.0
	deltaLonRad := (p2.Longitude - p1.Longitude) * math.Pi / 180.0

	y := math.Sin(deltaLonRad) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) - math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(deltaLonRad)

	bearingRad := math.Atan2(y, x)
	bearingDeg := bearingRad * 180.0 / math.Pi
	bearingDeg = math.Mod(bearingDeg+360.0, 360.0)
	return bearingDeg
}

func cardinalHeading(bearingDeg float64) string {
	index := int(math.Floor((bearingDeg+11.25)/22.5)) % 16
	if index < 0 {
		index = (index + 16) % 16
	}
	return cardinalPoints[index]
}
