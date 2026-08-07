package domain

import (
	"math"
	"strconv"
	"strings"
)

// LocationPoint represents WGS-84 geographical coordinates.
type LocationPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// EarthRadiusMiles is the mean radius of Earth in miles.
const EarthRadiusMiles = 3958.8

// HaversineDistanceMiles calculates the great-circle distance between two points in miles using the Haversine formula.
func HaversineDistanceMiles(p1, p2 LocationPoint) float64 {
	lat1Rad := p1.Latitude * math.Pi / 180.0
	lat2Rad := p2.Latitude * math.Pi / 180.0
	deltaLatRad := (p2.Latitude - p1.Latitude) * math.Pi / 180.0
	deltaLonRad := (p2.Longitude - p1.Longitude) * math.Pi / 180.0

	a := math.Sin(deltaLatRad/2.0)*math.Sin(deltaLatRad/2.0) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLonRad/2.0)*math.Sin(deltaLonRad/2.0)

	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
	return EarthRadiusMiles * c
}

// ParseLocationCoordinates attempts to extract coordinates or maps known neighborhood names to coordinates.
func ParseLocationCoordinates(locationStr string) LocationPoint {
	loc := strings.ToLower(strings.TrimSpace(locationStr))

	// Known Seattle neighborhood coordinates map
	if strings.Contains(loc, "capitol hill") {
		return LocationPoint{Latitude: 47.6150, Longitude: -122.3200}
	}
	if strings.Contains(loc, "green lake") {
		return LocationPoint{Latitude: 47.6800, Longitude: -122.3290}
	}
	if strings.Contains(loc, "ballard") {
		return LocationPoint{Latitude: 47.6684, Longitude: -122.3847}
	}
	if strings.Contains(loc, "fremont") {
		return LocationPoint{Latitude: 47.6512, Longitude: -122.3500}
	}

	// Try parsing "lat,lon" string
	parts := strings.Split(loc, ",")
	if len(parts) == 2 {
		lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		lon, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 == nil && err2 == nil {
			return LocationPoint{Latitude: lat, Longitude: lon}
		}
	}

	// Default Seattle center coordinates fallback
	return LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
}
