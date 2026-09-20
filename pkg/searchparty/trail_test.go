package searchparty_test

import (
	"math"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
)

func TestVolunteerBreadcrumbTrail_Validation(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	validTrail := searchparty.VolunteerBreadcrumbTrail{
		TrailID:        "trail-101",
		SearchPartyID:  "party-lost-123",
		SectorID:       "sector-1",
		VolunteerAlias: "Trail Scout #42",
		Points: []searchparty.BreadcrumbPoint{
			{
				Latitude:       47.6150,
				Longitude:      -122.3200,
				Timestamp:      now,
				AccuracyMeters: 4.5,
			},
			{
				Latitude:       47.6160,
				Longitude:      -122.3210,
				Timestamp:      now.Add(10 * time.Second),
				AccuracyMeters: 5.0,
			},
		},
		TotalDistanceM:  135.2,
		DurationSeconds: 10,
		CreatedAt:       now,
		UpdatedAt:       now.Add(10 * time.Second),
	}

	if err := validTrail.Validate(); err != nil {
		t.Fatalf("expected valid trail, got %v", err)
	}

	t.Run("empty trail ID", func(t *testing.T) {
		tr := validTrail
		tr.TrailID = ""
		if err := tr.Validate(); err == nil {
			t.Errorf("expected error for empty trail ID")
		}
	})

	t.Run("empty search party ID", func(t *testing.T) {
		tr := validTrail
		tr.SearchPartyID = ""
		if err := tr.Validate(); err == nil {
			t.Errorf("expected error for empty search party ID")
		}
	})

	t.Run("empty sector ID", func(t *testing.T) {
		tr := validTrail
		tr.SectorID = ""
		if err := tr.Validate(); err == nil {
			t.Errorf("expected error for empty sector ID")
		}
	})

	t.Run("empty volunteer alias", func(t *testing.T) {
		tr := validTrail
		tr.VolunteerAlias = "   "
		if err := tr.Validate(); err == nil {
			t.Errorf("expected error for empty volunteer alias")
		}
	})

	t.Run("invalid point latitude", func(t *testing.T) {
		tr := validTrail
		tr.Points = []searchparty.BreadcrumbPoint{
			{
				Latitude:       95.0,
				Longitude:      -122.3200,
				Timestamp:      now,
				AccuracyMeters: 5.0,
			},
		}
		if err := tr.Validate(); err == nil {
			t.Errorf("expected error for invalid latitude")
		}
	})

	t.Run("invalid point longitude", func(t *testing.T) {
		tr := validTrail
		tr.Points = []searchparty.BreadcrumbPoint{
			{
				Latitude:       47.6150,
				Longitude:      -190.0,
				Timestamp:      now,
				AccuracyMeters: 5.0,
			},
		}
		if err := tr.Validate(); err == nil {
			t.Errorf("expected error for invalid longitude")
		}
	})

	t.Run("zero point timestamp", func(t *testing.T) {
		tr := validTrail
		tr.Points = []searchparty.BreadcrumbPoint{
			{
				Latitude:       47.6150,
				Longitude:      -122.3200,
				AccuracyMeters: 5.0,
			},
		}
		if err := tr.Validate(); err == nil {
			t.Errorf("expected error for zero timestamp")
		}
	})

	t.Run("negative accuracy", func(t *testing.T) {
		tr := validTrail
		tr.Points = []searchparty.BreadcrumbPoint{
			{
				Latitude:       47.6150,
				Longitude:      -122.3200,
				Timestamp:      now,
				AccuracyMeters: -1.0,
			},
		}
		if err := tr.Validate(); err == nil {
			t.Errorf("expected error for negative accuracy")
		}
	})
}

func TestCalculateTrailDistance(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	t.Run("empty or single point returns 0", func(t *testing.T) {
		if dist := searchparty.CalculateTrailDistance(nil); dist != 0.0 {
			t.Errorf("expected 0 for nil, got %f", dist)
		}
		if dist := searchparty.CalculateTrailDistance([]searchparty.BreadcrumbPoint{}); dist != 0.0 {
			t.Errorf("expected 0 for empty slice, got %f", dist)
		}
		if dist := searchparty.CalculateTrailDistance([]searchparty.BreadcrumbPoint{
			{Latitude: 47.6, Longitude: -122.3, Timestamp: now},
		}); dist != 0.0 {
			t.Errorf("expected 0 for single point, got %f", dist)
		}
	})

	t.Run("calculates 1 degree latitude difference", func(t *testing.T) {
		// 1 degree latitude is approximately 111,195 meters (111.195 km)
		pts := []searchparty.BreadcrumbPoint{
			{Latitude: 0.0, Longitude: 0.0, Timestamp: now},
			{Latitude: 1.0, Longitude: 0.0, Timestamp: now.Add(time.Minute)},
		}
		dist := searchparty.CalculateTrailDistance(pts)
		// Earth radius 6,371,000 m * pi / 180 = ~111,194.9 m
		expected := 111194.9
		if math.Abs(dist-expected) > 50.0 {
			t.Errorf("expected ~%f, got %f", expected, dist)
		}
	})

	t.Run("multi-segment cumulative distance", func(t *testing.T) {
		p1 := searchparty.BreadcrumbPoint{Latitude: 47.6000, Longitude: -122.3300, Timestamp: now}
		p2 := searchparty.BreadcrumbPoint{Latitude: 47.6010, Longitude: -122.3300, Timestamp: now.Add(time.Minute)}
		p3 := searchparty.BreadcrumbPoint{Latitude: 47.6020, Longitude: -122.3300, Timestamp: now.Add(2 * time.Minute)}

		d1 := searchparty.CalculateTrailDistance([]searchparty.BreadcrumbPoint{p1, p2})
		d2 := searchparty.CalculateTrailDistance([]searchparty.BreadcrumbPoint{p2, p3})
		dTotal := searchparty.CalculateTrailDistance([]searchparty.BreadcrumbPoint{p1, p2, p3})

		if math.Abs(dTotal-(d1+d2)) > 0.1 {
			t.Errorf("expected sum of segments %f, got %f", d1+d2, dTotal)
		}
	})
}

func TestPointInSectorPolygon(t *testing.T) {
	t.Parallel()

	// Square polygon from lat 47.60 to 47.61, lon -122.34 to -122.33
	square := []domain.LocationPoint{
		{Latitude: 47.60, Longitude: -122.34},
		{Latitude: 47.61, Longitude: -122.34},
		{Latitude: 47.61, Longitude: -122.33},
		{Latitude: 47.60, Longitude: -122.33},
		{Latitude: 47.60, Longitude: -122.34}, // closed ring
	}

	t.Run("point inside polygon returns true", func(t *testing.T) {
		insidePt := searchparty.BreadcrumbPoint{Latitude: 47.605, Longitude: -122.335}
		if !searchparty.PointInSectorPolygon(insidePt, square) {
			t.Errorf("expected point to be inside polygon")
		}
	})

	t.Run("point outside polygon returns false", func(t *testing.T) {
		outsidePt := searchparty.BreadcrumbPoint{Latitude: 47.620, Longitude: -122.335}
		if searchparty.PointInSectorPolygon(outsidePt, square) {
			t.Errorf("expected point to be outside polygon")
		}
	})

	t.Run("polygon with fewer than 3 points returns false", func(t *testing.T) {
		pt := searchparty.BreadcrumbPoint{Latitude: 47.605, Longitude: -122.335}
		if searchparty.PointInSectorPolygon(pt, square[:2]) {
			t.Errorf("expected false for degenerate polygon")
		}
	})

	t.Run("wedge sector generated by DecomposePerimeter", func(t *testing.T) {
		center := domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}
		sectors := searchparty.DecomposePerimeter(&center, 1000.0, 4)
		if len(sectors) != 4 {
			t.Fatalf("expected 4 sectors, got %d", len(sectors))
		}

		// Sector 0 (NE: 0 to 90 degrees bearing).
		// A point slightly north-east of center should be in Sector 0.
		nePoint := searchparty.BreadcrumbPoint{Latitude: 47.604, Longitude: -122.325}
		inSec0 := searchparty.PointInSectorPolygon(nePoint, sectors[0].PolygonPoints)
		if !inSec0 {
			t.Errorf("expected NE point to be inside Sector 0")
		}

		// A point south-west of center should NOT be in Sector 0
		swPoint := searchparty.BreadcrumbPoint{Latitude: 47.596, Longitude: -122.335}
		inSec0ForSW := searchparty.PointInSectorPolygon(swPoint, sectors[0].PolygonPoints)
		if inSec0ForSW {
			t.Errorf("expected SW point to NOT be in Sector 0")
		}
	})
}
