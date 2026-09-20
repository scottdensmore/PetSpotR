package sighting_test

import (
	"math"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sighting"
)

func TestCalculateTrajectory(t *testing.T) {
	t.Parallel()

	originTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	originPoint := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}

	sightings := []domain.PetSightingRecord{
		{
			SightingID: "s-1",
			LostPetID:  "lost-1",
			SightedAt:  originTime.Add(30 * time.Minute),
			Coordinates: &domain.LocationPoint{
				Latitude:  47.610,
				Longitude: -122.330,
			},
			Status: domain.SightingStatusActive,
		},
		{
			SightingID: "s-2",
			LostPetID:  "lost-1",
			SightedAt:  originTime.Add(60 * time.Minute),
			Coordinates: &domain.LocationPoint{
				Latitude:  47.620,
				Longitude: -122.330,
			},
			Status: domain.SightingStatusActive,
		},
	}

	traj := sighting.CalculateTrajectory("lost-1", originPoint, originTime, sightings)

	if traj.SightingsCount != 2 {
		t.Fatalf("expected 2 sightings, got %d", traj.SightingsCount)
	}
	if len(traj.Legs) != 2 { // Origin -> s-1, and s-1 -> s-2
		t.Fatalf("expected 2 legs, got %d", len(traj.Legs))
	}
	if traj.Legs[0].SpeedMph <= 0 {
		t.Errorf("expected positive speed in mph, got %f", traj.Legs[0].SpeedMph)
	}
	if traj.EstimatedPerimeter == nil {
		t.Fatal("expected non-nil EstimatedPerimeter")
	}

	// Verify Leg 0 details: from origin to s-1
	leg0 := traj.Legs[0]
	if leg0.FromSightingID != "origin" {
		t.Errorf("expected FromSightingID 'origin', got %q", leg0.FromSightingID)
	}
	if leg0.ToSightingID != "s-1" {
		t.Errorf("expected ToSightingID 's-1', got %q", leg0.ToSightingID)
	}
	if leg0.CardinalHeading != "N" {
		t.Errorf("expected heading 'N', got %q", leg0.CardinalHeading)
	}
	if leg0.BearingDegrees < 0 || leg0.BearingDegrees > 1.0 {
		t.Errorf("expected bearing ~0 deg, got %f", leg0.BearingDegrees)
	}
	if leg0.ElapsedDuration != 1800 { // 30 min = 1800 s
		t.Errorf("expected elapsed 1800s, got %f", leg0.ElapsedDuration)
	}

	// Verify Leg 1 details: s-1 to s-2
	leg1 := traj.Legs[1]
	if leg1.FromSightingID != "s-1" {
		t.Errorf("expected FromSightingID 's-1', got %q", leg1.FromSightingID)
	}
	if leg1.ToSightingID != "s-2" {
		t.Errorf("expected ToSightingID 's-2', got %q", leg1.ToSightingID)
	}
	if leg1.CardinalHeading != "N" {
		t.Errorf("expected heading 'N', got %q", leg1.CardinalHeading)
	}

	// Verify Total distance is sum of legs
	expectedTotal := leg0.DistanceMiles + leg1.DistanceMiles
	if math.Abs(traj.TotalDistanceMiles-expectedTotal) > 0.001 {
		t.Errorf("expected total distance %f, got %f", expectedTotal, traj.TotalDistanceMiles)
	}

	// Verify perimeter center is s-2 coordinates
	if traj.EstimatedPerimeter.CenterCoordinates.Latitude != 47.620 ||
		traj.EstimatedPerimeter.CenterCoordinates.Longitude != -122.330 {
		t.Errorf("expected perimeter center at s-2, got %+v", traj.EstimatedPerimeter.CenterCoordinates)
	}
}

func TestCalculateTrajectory_ChronologicalSorting(t *testing.T) {
	t.Parallel()

	originTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	// Sightings intentionally out of chronological order
	sightings := []domain.PetSightingRecord{
		{
			SightingID: "s-late",
			LostPetID:  "lost-sort",
			SightedAt:  originTime.Add(2 * time.Hour),
			Coordinates: &domain.LocationPoint{
				Latitude:  47.620,
				Longitude: -122.330,
			},
			Status: domain.SightingStatusActive,
		},
		{
			SightingID: "s-early",
			LostPetID:  "lost-sort",
			SightedAt:  originTime.Add(30 * time.Minute),
			Coordinates: &domain.LocationPoint{
				Latitude:  47.610,
				Longitude: -122.330,
			},
			Status: domain.SightingStatusActive,
		},
	}

	traj := sighting.CalculateTrajectory("lost-sort", nil, time.Time{}, sightings)

	if len(traj.OrderedSightings) != 2 {
		t.Fatalf("expected 2 ordered sightings, got %d", len(traj.OrderedSightings))
	}
	if traj.OrderedSightings[0].SightingID != "s-early" {
		t.Errorf("expected first sighting to be 's-early', got %s", traj.OrderedSightings[0].SightingID)
	}
	if traj.OrderedSightings[1].SightingID != "s-late" {
		t.Errorf("expected second sighting to be 's-late', got %s", traj.OrderedSightings[1].SightingID)
	}
	// Without origin, 2 sightings yield 1 leg: s-early -> s-late
	if len(traj.Legs) != 1 {
		t.Fatalf("expected 1 leg without origin, got %d", len(traj.Legs))
	}
	if traj.Legs[0].FromSightingID != "s-early" || traj.Legs[0].ToSightingID != "s-late" {
		t.Errorf("unexpected leg endpoints: %+v", traj.Legs[0])
	}
}

func TestCalculateTrajectory_ZeroAndSingleSightings(t *testing.T) {
	t.Parallel()

	originTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	originPoint := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}

	t.Run("no origin and no sightings", func(t *testing.T) {
		traj := sighting.CalculateTrajectory("lost-empty", nil, time.Time{}, nil)
		if traj.SightingsCount != 0 {
			t.Errorf("expected 0 sightings, got %d", traj.SightingsCount)
		}
		if len(traj.Legs) != 0 {
			t.Errorf("expected 0 legs, got %d", len(traj.Legs))
		}
		if traj.EstimatedPerimeter != nil {
			t.Errorf("expected nil perimeter when no points exist, got %+v", traj.EstimatedPerimeter)
		}
	})

	t.Run("origin provided but no sightings", func(t *testing.T) {
		traj := sighting.CalculateTrajectory("lost-origin-only", originPoint, originTime, nil)
		if traj.SightingsCount != 0 {
			t.Errorf("expected 0 sightings, got %d", traj.SightingsCount)
		}
		if len(traj.Legs) != 0 {
			t.Errorf("expected 0 legs, got %d", len(traj.Legs))
		}
		if traj.EstimatedPerimeter == nil {
			t.Fatal("expected non-nil perimeter centered at origin")
		}
		if traj.EstimatedPerimeter.CenterCoordinates != *originPoint {
			t.Errorf("expected perimeter center at origin, got %+v", traj.EstimatedPerimeter.CenterCoordinates)
		}
	})

	t.Run("origin provided and single sighting", func(t *testing.T) {
		s := []domain.PetSightingRecord{
			{
				SightingID: "s-1",
				LostPetID:  "lost-single",
				SightedAt:  originTime.Add(15 * time.Minute),
				Coordinates: &domain.LocationPoint{
					Latitude:  47.605,
					Longitude: -122.330,
				},
				Status: domain.SightingStatusActive,
			},
		}
		traj := sighting.CalculateTrajectory("lost-single", originPoint, originTime, s)
		if traj.SightingsCount != 1 {
			t.Errorf("expected 1 sighting, got %d", traj.SightingsCount)
		}
		if len(traj.Legs) != 1 {
			t.Fatalf("expected 1 leg from origin to s-1, got %d", len(traj.Legs))
		}
		if traj.Legs[0].FromSightingID != "origin" || traj.Legs[0].ToSightingID != "s-1" {
			t.Errorf("unexpected leg endpoints: %+v", traj.Legs[0])
		}
	})

	t.Run("no origin and single sighting", func(t *testing.T) {
		s := []domain.PetSightingRecord{
			{
				SightingID: "s-1",
				LostPetID:  "lost-single-no-origin",
				SightedAt:  originTime.Add(15 * time.Minute),
				Coordinates: &domain.LocationPoint{
					Latitude:  47.605,
					Longitude: -122.330,
				},
				Status: domain.SightingStatusActive,
			},
		}
		traj := sighting.CalculateTrajectory("lost-single-no-origin", nil, time.Time{}, s)
		if traj.SightingsCount != 1 {
			t.Errorf("expected 1 sighting, got %d", traj.SightingsCount)
		}
		if len(traj.Legs) != 0 {
			t.Errorf("expected 0 legs when only 1 point and no origin, got %d", len(traj.Legs))
		}
		if traj.EstimatedPerimeter == nil {
			t.Fatal("expected non-nil perimeter")
		}
		if traj.EstimatedPerimeter.CenterCoordinates != *s[0].Coordinates {
			t.Errorf("expected perimeter center at s-1, got %+v", traj.EstimatedPerimeter.CenterCoordinates)
		}
	})
}

func TestCalculateTrajectory_ElapsedZeroSpeed(t *testing.T) {
	t.Parallel()

	sameTime := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	sightings := []domain.PetSightingRecord{
		{
			SightingID:  "s-1",
			LostPetID:   "lost-same",
			SightedAt:   sameTime,
			Coordinates: &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330},
			Status:      domain.SightingStatusActive,
		},
		{
			SightingID:  "s-2",
			LostPetID:   "lost-same",
			SightedAt:   sameTime, // identical timestamp
			Coordinates: &domain.LocationPoint{Latitude: 47.610, Longitude: -122.330},
			Status:      domain.SightingStatusActive,
		},
	}

	traj := sighting.CalculateTrajectory("lost-same", nil, time.Time{}, sightings)
	if len(traj.Legs) != 1 {
		t.Fatalf("expected 1 leg, got %d", len(traj.Legs))
	}
	if traj.Legs[0].SpeedMph != 0.0 {
		t.Errorf("expected 0.0 speed for zero elapsed time, got %f", traj.Legs[0].SpeedMph)
	}
}

func TestCalculateTrajectory_PerimeterExpansion(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()

	t.Run("sighting within 2 hours yields 0.5 mile High confidence perimeter", func(t *testing.T) {
		sightings := []domain.PetSightingRecord{
			{
				SightingID:  "s-recent",
				LostPetID:   "lost-recent",
				SightedAt:   now.Add(-30 * time.Minute),
				Coordinates: &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330},
				Status:      domain.SightingStatusActive,
			},
		}
		traj := sighting.CalculateTrajectory("lost-recent", nil, time.Time{}, sightings)
		if traj.EstimatedPerimeter == nil {
			t.Fatal("expected non-nil perimeter")
		}
		if math.Abs(traj.EstimatedPerimeter.RadiusMiles-0.5) > 0.001 {
			t.Errorf("expected radius 0.5 miles within 2h, got %f", traj.EstimatedPerimeter.RadiusMiles)
		}
		if traj.EstimatedPerimeter.ConfidenceLevel != "High" {
			t.Errorf("expected confidence 'High', got %q", traj.EstimatedPerimeter.ConfidenceLevel)
		}
		expectedMeters := 0.5 * 1609.344
		if math.Abs(traj.EstimatedPerimeter.RadiusMeters-expectedMeters) > 0.1 {
			t.Errorf("expected radius meters %f, got %f", expectedMeters, traj.EstimatedPerimeter.RadiusMeters)
		}
	})

	t.Run("sighting 4 hours ago yields 0.75 mile Expanding perimeter", func(t *testing.T) {
		sightings := []domain.PetSightingRecord{
			{
				SightingID:  "s-4h",
				LostPetID:   "lost-4h",
				SightedAt:   now.Add(-4 * time.Hour),
				Coordinates: &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330},
				Status:      domain.SightingStatusActive,
			},
		}
		traj := sighting.CalculateTrajectory("lost-4h", nil, time.Time{}, sightings)
		if traj.EstimatedPerimeter == nil {
			t.Fatal("expected non-nil perimeter")
		}
		if math.Abs(traj.EstimatedPerimeter.RadiusMiles-0.75) > 0.05 {
			t.Errorf("expected radius ~0.75 miles at 4h, got %f", traj.EstimatedPerimeter.RadiusMiles)
		}
		if traj.EstimatedPerimeter.ConfidenceLevel != "Expanding" {
			t.Errorf("expected confidence 'Expanding', got %q", traj.EstimatedPerimeter.ConfidenceLevel)
		}
	})

	t.Run("sighting 30 hours ago caps at 3.0 miles Wide perimeter", func(t *testing.T) {
		sightings := []domain.PetSightingRecord{
			{
				SightingID:  "s-old",
				LostPetID:   "lost-old",
				SightedAt:   now.Add(-30 * time.Hour),
				Coordinates: &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330},
				Status:      domain.SightingStatusActive,
			},
		}
		traj := sighting.CalculateTrajectory("lost-old", nil, time.Time{}, sightings)
		if traj.EstimatedPerimeter == nil {
			t.Fatal("expected non-nil perimeter")
		}
		if math.Abs(traj.EstimatedPerimeter.RadiusMiles-3.0) > 0.001 {
			t.Errorf("expected capped radius 3.0 miles, got %f", traj.EstimatedPerimeter.RadiusMiles)
		}
		if traj.EstimatedPerimeter.ConfidenceLevel != "Wide" {
			t.Errorf("expected confidence 'Wide', got %q", traj.EstimatedPerimeter.ConfidenceLevel)
		}
	})
}

func TestBearingAndCardinalHeadings(t *testing.T) {
	t.Parallel()

	originTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	center := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}

	testCases := []struct {
		name            string
		dest            *domain.LocationPoint
		expectedHeading string
	}{
		{name: "North", dest: &domain.LocationPoint{Latitude: 47.650, Longitude: -122.330}, expectedHeading: "N"},
		{name: "South", dest: &domain.LocationPoint{Latitude: 47.550, Longitude: -122.330}, expectedHeading: "S"},
		{name: "East", dest: &domain.LocationPoint{Latitude: 47.600, Longitude: -122.250}, expectedHeading: "E"},
		{name: "West", dest: &domain.LocationPoint{Latitude: 47.600, Longitude: -122.410}, expectedHeading: "W"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sightings := []domain.PetSightingRecord{
				{
					SightingID:  "s-dest",
					LostPetID:   "lost-test",
					SightedAt:   originTime.Add(10 * time.Minute),
					Coordinates: tc.dest,
					Status:      domain.SightingStatusActive,
				},
			}
			traj := sighting.CalculateTrajectory("lost-test", center, originTime, sightings)
			if len(traj.Legs) != 1 {
				t.Fatalf("expected 1 leg, got %d", len(traj.Legs))
			}
			if traj.Legs[0].CardinalHeading != tc.expectedHeading {
				t.Errorf("expected heading %s, got %s (bearing: %f)",
					tc.expectedHeading, traj.Legs[0].CardinalHeading, traj.Legs[0].BearingDegrees)
			}
		})
	}
}
