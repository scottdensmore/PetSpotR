package searchparty

import (
	"math"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// HaversineDistanceMeters calculates the great-circle distance between two points in meters.
func HaversineDistanceMeters(p1, p2 domain.LocationPoint) float64 {
	lat1Rad := p1.Latitude * math.Pi / 180.0
	lat2Rad := p2.Latitude * math.Pi / 180.0
	deltaLatRad := (p2.Latitude - p1.Latitude) * math.Pi / 180.0
	deltaLonRad := (p2.Longitude - p1.Longitude) * math.Pi / 180.0

	a := math.Sin(deltaLatRad/2.0)*math.Sin(deltaLatRad/2.0) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*
			math.Sin(deltaLonRad/2.0)*math.Sin(deltaLonRad/2.0)

	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(1.0-a))
	return earthRadiusMeters * c
}

// CalculateTrailDistance computes the total walked distance in meters along a sequence of breadcrumb points.
func CalculateTrailDistance(points []BreadcrumbPoint) float64 {
	if len(points) < 2 {
		return 0.0
	}
	var total float64
	for i := 0; i < len(points)-1; i++ {
		p1 := domain.LocationPoint{Latitude: points[i].Latitude, Longitude: points[i].Longitude}
		p2 := domain.LocationPoint{Latitude: points[i+1].Latitude, Longitude: points[i+1].Longitude}
		total += HaversineDistanceMeters(p1, p2)
	}
	return math.Round(total*100.0) / 100.0
}

// PointInPolygon checks whether a geographical coordinate lies inside a closed polygon using ray casting.
func PointInPolygon(point domain.LocationPoint, polygon []domain.LocationPoint) bool {
	if len(polygon) < 3 {
		return false
	}
	inside := false
	x := point.Longitude
	y := point.Latitude
	n := len(polygon)
	for i, j := 0, n-1; i < n; i++ {
		xi, yi := polygon[i].Longitude, polygon[i].Latitude
		xj, yj := polygon[j].Longitude, polygon[j].Latitude

		intersect := ((yi > y) != (yj > y)) && (x < (xj-xi)*(y-yi)/(yj-yi)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}
	return inside
}

// PointInSectorPolygon checks whether a breadcrumb point lies inside a sector polygon.
func PointInSectorPolygon(pt BreadcrumbPoint, polygon []domain.LocationPoint) bool {
	return PointInPolygon(domain.LocationPoint{Latitude: pt.Latitude, Longitude: pt.Longitude}, polygon)
}

// AppendPoints adds new breadcrumb points to the trail and recalculates cumulative distance and duration.
func (t *VolunteerBreadcrumbTrail) AppendPoints(newPoints ...BreadcrumbPoint) {
	t.Points = append(t.Points, newPoints...)
	t.TotalDistanceM = CalculateTrailDistance(t.Points)
	if len(t.Points) >= 2 {
		diff := t.Points[len(t.Points)-1].Timestamp.Sub(t.Points[0].Timestamp).Seconds()
		if diff > 0 {
			t.DurationSeconds = int(diff)
		}
	}
}
