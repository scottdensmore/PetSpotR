package domain_test

import (
	"math"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestHaversineDistanceMiles(t *testing.T) {
	// Seattle Capitol Hill (47.6150, -122.3200) to Green Lake (47.6800, -122.3290) is ~4.5 miles
	capitolHill := domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200}
	greenLake := domain.LocationPoint{Latitude: 47.6800, Longitude: -122.3290}

	dist := domain.HaversineDistanceMiles(capitolHill, greenLake)

	if dist < 4.0 || dist > 5.5 {
		t.Errorf("expected distance between 4.0 and 5.5 miles, got %.2f", dist)
	}

	// Same point distance should be 0
	if zeroDist := domain.HaversineDistanceMiles(capitolHill, capitolHill); math.Abs(zeroDist) > 0.001 {
		t.Errorf("expected 0 distance for same point, got %.4f", zeroDist)
	}
}

func TestLocationPoint_Validation(t *testing.T) {
	t.Run("valid location point", func(t *testing.T) {
		pt := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
		if err := pt.Validate(); err != nil {
			t.Fatalf("expected valid location point, got %v", err)
		}
	})

	t.Run("invalid latitude out of range", func(t *testing.T) {
		pt := domain.LocationPoint{Latitude: 95.5, Longitude: -122.3321}
		if err := pt.Validate(); err == nil {
			t.Fatal("expected error for latitude > 90, got nil")
		}
	})

	t.Run("invalid longitude out of range", func(t *testing.T) {
		pt := domain.LocationPoint{Latitude: 47.6062, Longitude: -190.0}
		if err := pt.Validate(); err == nil {
			t.Fatal("expected error for longitude < -180, got nil")
		}
	})
}
