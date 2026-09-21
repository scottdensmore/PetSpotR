package sighting_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sighting"
)

func TestPredictiveTrajectory_DogMomentum(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	now := time.Now().UTC()

	// Dog profile with directional momentum heading North
	dogSightings := []domain.PetSightingRecord{
		{
			SightingID:  "sight-1",
			LostPetID:   "dog-1",
			SightedAt:   now.Add(-2 * time.Hour),
			Coordinates: &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
			Status:      domain.SightingStatusActive,
		},
		{
			SightingID:  "sight-2",
			LostPetID:   "dog-1",
			SightedAt:   now.Add(-1 * time.Hour),
			Coordinates: &domain.LocationPoint{Latitude: 47.6100, Longitude: -122.3321}, // Heading North
			Status:      domain.SightingStatusActive,
		},
	}

	resultDog := sighting.GeneratePredictiveTrajectory("dog-1", "dog", &origin, dogSightings, 1.0)
	if resultDog.LostPetID != "dog-1" {
		t.Errorf("expected LostPetID 'dog-1', got %q", resultDog.LostPetID)
	}
	if resultDog.Species != "dog" {
		t.Errorf("expected Species 'dog', got %q", resultDog.Species)
	}
	if len(resultDog.Isochrones) != 3 {
		t.Fatalf("expected 3 isochrone levels, got %d", len(resultDog.Isochrones))
	}
	if resultDog.HeadingDegrees < 350 && resultDog.HeadingDegrees > 10 {
		t.Errorf("expected heading roughly North (~0/360 deg), got %.1f", resultDog.HeadingDegrees)
	}

	// Verify the 3 contours are closed polygons
	for _, iso := range resultDog.Isochrones {
		if len(iso.PolygonCoordinates) == 0 {
			t.Fatalf("expected polygon coordinates for isochrone %s", iso.Label)
		}
		ring := iso.PolygonCoordinates[0]
		if len(ring) < 4 {
			t.Fatalf("expected at least 4 points for closed ring in %s, got %d", iso.Label, len(ring))
		}
		first := ring[0]
		last := ring[len(ring)-1]
		if math.Abs(first.Latitude-last.Latitude) > 1e-6 || math.Abs(first.Longitude-last.Longitude) > 1e-6 {
			t.Errorf("isochrone ring %s is not closed: first=(%f,%f), last=(%f,%f)",
				iso.Label, first.Latitude, first.Longitude, last.Latitude, last.Longitude)
		}
	}

	// Verify directional momentum: North extent should be greater than South extent from last sighting
	lastPt := resultDog.LastSightingPoint
	var maxNorthLat, minSouthLat float64
	for _, pt := range resultDog.Isochrones[2].PolygonCoordinates[0] {
		if pt.Latitude > maxNorthLat || maxNorthLat == 0 {
			maxNorthLat = pt.Latitude
		}
		if pt.Latitude < minSouthLat || minSouthLat == 0 {
			minSouthLat = pt.Latitude
		}
	}
	northDist := maxNorthLat - lastPt.Latitude
	southDist := lastPt.Latitude - minSouthLat
	if northDist <= southDist {
		t.Errorf("expected north extent (%f) to exceed south extent (%f) due to dog momentum", northDist, southDist)
	}
}

func TestPredictiveTrajectory_CatContainment(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	now := time.Now().UTC()

	catSightings := []domain.PetSightingRecord{
		{
			SightingID:  "sight-cat-1",
			LostPetID:   "cat-1",
			SightedAt:   now.Add(-30 * time.Minute),
			Coordinates: &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
			Status:      domain.SightingStatusActive,
		},
	}

	resultCat := sighting.GeneratePredictiveTrajectory("cat-1", "cat", &origin, catSightings, 0.5)
	if len(resultCat.Isochrones) != 3 {
		t.Fatalf("expected 3 isochrone levels for cat, got %d", len(resultCat.Isochrones))
	}

	// 90% containment boundary must be strictly within feline max roaming radius (< 550m ~ 0.35 miles)
	const maxCatRadiusMiles = 0.40
	for _, pt := range resultCat.Isochrones[2].PolygonCoordinates[0] {
		dist := domain.HaversineDistanceMiles(origin, pt)
		if dist > maxCatRadiusMiles {
			t.Errorf("cat isochrone point (%f, %f) exceeded max containment radius: %f miles > %f miles",
				pt.Latitude, pt.Longitude, dist, maxCatRadiusMiles)
		}
	}
}

func TestPredictiveTrajectory_BarrierDeflection(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}

	result := sighting.GeneratePredictiveTrajectory("pet-1", "general", &origin, nil, 1.0)
	if len(result.BarriersEncountered) == 0 {
		t.Fatalf("expected barriers to be encountered near Seattle downtown origin, got 0")
	}

	hasFreeway := false
	hasWaterway := false
	for _, b := range result.BarriersEncountered {
		if b.Type == domain.BarrierTypeFreeway {
			hasFreeway = true
		}
		if b.Type == domain.BarrierTypeWaterway {
			hasWaterway = true
		}
	}
	if !hasFreeway {
		t.Errorf("expected Freeway barrier (I-5) to be encountered")
	}
	if !hasWaterway {
		t.Errorf("expected Waterway barrier (Elliott Bay) to be encountered")
	}
}

func TestScoreSectorUrgency(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	result := sighting.GeneratePredictiveTrajectory("dog-1", "dog", &origin, nil, 1.0)

	// Sector overlapping center point / 50% core isochrone
	sectorCore := domain.SearchSector{
		SectorID: "sector-core",
		BoundingPolygon: []domain.LocationPoint{
			{Latitude: 47.6050, Longitude: -122.3330},
			{Latitude: 47.6070, Longitude: -122.3330},
			{Latitude: 47.6070, Longitude: -122.3310},
			{Latitude: 47.6050, Longitude: -122.3310},
		},
	}

	scoreCore, urgencyCore := sighting.ScoreSectorUrgency(sectorCore, result.Isochrones)
	if urgencyCore != domain.SectorUrgencyCritical {
		t.Errorf("expected SectorUrgencyCritical for core sector, got %s (score %.2f)", urgencyCore, scoreCore)
	}
	if scoreCore < 80.0 {
		t.Errorf("expected score >= 80 for critical sector, got %.2f", scoreCore)
	}

	// Distant sector far outside any isochrone (15 km north)
	sectorFar := domain.SearchSector{
		SectorID: "sector-far",
		BoundingPolygon: []domain.LocationPoint{
			{Latitude: 47.7500, Longitude: -122.3330},
			{Latitude: 47.7520, Longitude: -122.3330},
			{Latitude: 47.7520, Longitude: -122.3310},
			{Latitude: 47.7500, Longitude: -122.3310},
		},
	}

	scoreFar, urgencyFar := sighting.ScoreSectorUrgency(sectorFar, result.Isochrones)
	if urgencyFar != domain.SectorUrgencyStandard {
		t.Errorf("expected SectorUrgencyStandard for distant sector, got %s (score %.2f)", urgencyFar, scoreFar)
	}
	if scoreFar >= 50.0 {
		t.Errorf("expected score < 50 for distant sector, got %.2f", scoreFar)
	}
}

func TestHidingClusters_Extraction(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	result := sighting.GeneratePredictiveTrajectory("cat-1", "cat", &origin, nil, 1.0)

	if len(result.HidingClusters) == 0 {
		t.Fatalf("expected hiding clusters to be detected near greenways/parks, got 0")
	}

	for _, c := range result.HidingClusters {
		if c.ID == "" {
			t.Errorf("expected non-empty cluster ID")
		}
		if c.Name == "" {
			t.Errorf("expected non-empty cluster name")
		}
		if c.RadiusMeters <= 0 {
			t.Errorf("expected positive cluster radius, got %f", c.RadiusMeters)
		}
		if c.AttractionScore <= 0 || c.AttractionScore > 1.0 {
			t.Errorf("expected attraction score between 0 and 1.0, got %f", c.AttractionScore)
		}
		if c.Description == "" {
			t.Errorf("expected non-empty description for cluster %s", c.ID)
		}
	}
}

func TestPredictiveTrajectory_Concurrency(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	var wg sync.WaitGroup

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			species := "dog"
			if idx%2 == 1 {
				species = "cat"
			}
			res := sighting.GeneratePredictiveTrajectory("pet-concurrent", species, &origin, nil, 0.8)
			if len(res.Isochrones) != 3 {
				t.Errorf("concurrent run expected 3 isochrones, got %d", len(res.Isochrones))
			}
		}(i)
	}
	wg.Wait()
}

func TestScoreSectorUrgency_HighAndFallback(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	result := sighting.GeneratePredictiveTrajectory("pet-1", "dog", &origin, nil, 1.0)

	// Sector that overlaps the 75% envelope ring point, using PolygonPoints field
	envelopeRing := result.Isochrones[1].PolygonCoordinates[0]
	pt := envelopeRing[len(envelopeRing)/2]

	sectorHigh := domain.SearchSector{
		SectorID: "sector-high",
		PolygonPoints: []domain.LocationPoint{
			{Latitude: pt.Latitude - 0.0005, Longitude: pt.Longitude - 0.0005},
			{Latitude: pt.Latitude + 0.0005, Longitude: pt.Longitude - 0.0005},
			{Latitude: pt.Latitude + 0.0005, Longitude: pt.Longitude + 0.0005},
			{Latitude: pt.Latitude - 0.0005, Longitude: pt.Longitude + 0.0005},
		},
	}

	score, urgency := sighting.ScoreSectorUrgency(sectorHigh, result.Isochrones)
	if urgency != domain.SectorUrgencyHigh && urgency != domain.SectorUrgencyCritical {
		t.Errorf("expected SectorUrgencyHigh or Critical, got %s (score %.2f)", urgency, score)
	}
	if score < 50.0 {
		t.Errorf("expected score >= 50.0, got %.2f", score)
	}

	// Invalid sector (< 3 points)
	sectorInvalid := domain.SearchSector{
		SectorID: "sector-invalid",
		BoundingPolygon: []domain.LocationPoint{
			{Latitude: 47.6050, Longitude: -122.3330},
		},
	}
	scoreInv, urgencyInv := sighting.ScoreSectorUrgency(sectorInvalid, result.Isochrones)
	if urgencyInv != domain.SectorUrgencyStandard {
		t.Errorf("expected SectorUrgencyStandard for invalid sector, got %s", urgencyInv)
	}
	if scoreInv > 20.0 {
		t.Errorf("expected score <= 20.0 for invalid sector, got %.2f", scoreInv)
	}
}

func TestPredictiveTrajectory_SpeciesWrapperAndNilOrigin(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	resSpecies := sighting.GeneratePredictiveTrajectoryForSpecies("cat", origin, nil, 0.5)
	if resSpecies.Species != "cat" {
		t.Errorf("expected species 'cat', got %s", resSpecies.Species)
	}

	// Nil origin with sightings
	now := time.Now().UTC()
	sighting1 := domain.PetSightingRecord{
		SightingID:  "s-1",
		SightedAt:   now,
		Coordinates: &domain.LocationPoint{Latitude: 47.6100, Longitude: -122.3300},
	}
	resNilOrigin := sighting.GeneratePredictiveTrajectory("pet-nil", "dog", nil, []domain.PetSightingRecord{sighting1}, 0.5)
	if resNilOrigin.LastSightingPoint.Latitude != 47.6100 {
		t.Errorf("expected LastSightingPoint lat 47.6100, got %f", resNilOrigin.LastSightingPoint.Latitude)
	}

	// Nil origin and empty sightings falls back to Seattle default
	resDefault := sighting.GeneratePredictiveTrajectory("pet-def", "general", nil, nil, 0.5)
	if resDefault.LastSightingPoint.Latitude != 47.6062 {
		t.Errorf("expected default lat 47.6062, got %f", resDefault.LastSightingPoint.Latitude)
	}
}
