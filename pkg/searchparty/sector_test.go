package searchparty_test

import (
	"math"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
)

func TestSearchParty_DecomposePerimeter(t *testing.T) {
	t.Parallel()

	center := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}
	radius := 1000.0 // 1km
	sectorCount := 4

	sectors := searchparty.DecomposePerimeter(center, radius, sectorCount)

	if len(sectors) != sectorCount {
		t.Fatalf("expected %d sectors, got %d", sectorCount, len(sectors))
	}

	for _, s := range sectors {
		if len(s.PolygonPoints) < 3 {
			t.Errorf("sector %s has invalid polygon with %d points", s.SectorID, len(s.PolygonPoints))
		}
		if s.Status != searchparty.SectorStatusUnassigned {
			t.Errorf("expected new sector to be unassigned, got %s", s.Status)
		}
		if s.TotalAreaSqM <= 0 {
			t.Errorf("expected positive area, got %f", s.TotalAreaSqM)
		}
		for idx, pt := range s.PolygonPoints {
			if err := pt.Validate(); err != nil {
				t.Errorf("sector %s polygon point %d invalid: %v", s.SectorID, idx, err)
			}
		}
		if s.PolygonPoints[0] != s.PolygonPoints[len(s.PolygonPoints)-1] {
			t.Errorf("sector %s polygon should be closed (first point == last point)", s.SectorID)
		}
		if s.Name == "" {
			t.Errorf("sector %s has empty name", s.SectorID)
		}
	}
}

func TestSearchParty_DecomposePerimeter_EightSectors(t *testing.T) {
	t.Parallel()

	center := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}
	radius := 1609.34 // ~1 mile in meters
	sectorCount := 8

	sectors := searchparty.DecomposePerimeter(center, radius, sectorCount)

	if len(sectors) != 8 {
		t.Fatalf("expected 8 sectors, got %d", len(sectors))
	}

	expectedAreaPerSector := (math.Pi * radius * radius) / 8.0
	for _, s := range sectors {
		if len(s.PolygonPoints) < 3 {
			t.Errorf("sector %s has invalid polygon with %d points", s.SectorID, len(s.PolygonPoints))
		}
		if math.Abs(s.TotalAreaSqM-expectedAreaPerSector) > 1.0 {
			t.Errorf("sector %s expected area ~%f, got %f", s.SectorID, expectedAreaPerSector, s.TotalAreaSqM)
		}
	}
}

func TestSearchParty_DecomposePerimeter_EdgeCases(t *testing.T) {
	t.Parallel()

	validCenter := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}

	t.Run("nil center", func(t *testing.T) {
		sectors := searchparty.DecomposePerimeter(nil, 1000, 4)
		if len(sectors) != 0 {
			t.Fatalf("expected 0 sectors for nil center, got %d", len(sectors))
		}
	})

	t.Run("zero or negative radius", func(t *testing.T) {
		sectors := searchparty.DecomposePerimeter(validCenter, 0, 4)
		if len(sectors) != 0 {
			t.Fatalf("expected 0 sectors for 0 radius, got %d", len(sectors))
		}

		sectors = searchparty.DecomposePerimeter(validCenter, -500, 4)
		if len(sectors) != 0 {
			t.Fatalf("expected 0 sectors for negative radius, got %d", len(sectors))
		}
	})

	t.Run("zero or negative sector count", func(t *testing.T) {
		sectors := searchparty.DecomposePerimeter(validCenter, 1000, 0)
		if len(sectors) != 0 {
			t.Fatalf("expected 0 sectors for 0 sectorCount, got %d", len(sectors))
		}

		sectors = searchparty.DecomposePerimeter(validCenter, 1000, -2)
		if len(sectors) != 0 {
			t.Fatalf("expected 0 sectors for negative sectorCount, got %d", len(sectors))
		}
	})
}

func TestSearchParty_Coverage(t *testing.T) {
	t.Parallel()

	party := searchparty.SearchParty{
		PartyID: "party-1",
		Sectors: []searchparty.SearchSector{
			{SectorID: "s-1", Status: searchparty.SectorStatusCleared, TotalAreaSqM: 500},
			{SectorID: "s-2", Status: searchparty.SectorStatusUnassigned, TotalAreaSqM: 500},
		},
	}

	coverage := party.CalculateCoverage()
	if coverage != 50.0 {
		t.Errorf("expected 50%% coverage, got %f", coverage)
	}

	t.Run("empty sectors", func(t *testing.T) {
		emptyParty := searchparty.SearchParty{PartyID: "empty"}
		if cov := emptyParty.CalculateCoverage(); cov != 0.0 {
			t.Errorf("expected 0%% coverage for empty sectors, got %f", cov)
		}
	})

	t.Run("fully cleared", func(t *testing.T) {
		fullParty := searchparty.SearchParty{
			PartyID: "full",
			Sectors: []searchparty.SearchSector{
				{SectorID: "s-1", Status: searchparty.SectorStatusCleared, TotalAreaSqM: 300},
				{SectorID: "s-2", Status: searchparty.SectorStatusCleared, TotalAreaSqM: 700},
			},
		}
		if cov := fullParty.CalculateCoverage(); cov != 100.0 {
			t.Errorf("expected 100%% coverage, got %f", cov)
		}
	})

	t.Run("none cleared", func(t *testing.T) {
		noneParty := searchparty.SearchParty{
			PartyID: "none",
			Sectors: []searchparty.SearchSector{
				{SectorID: "s-1", Status: searchparty.SectorStatusActiveSearch, TotalAreaSqM: 300},
				{SectorID: "s-2", Status: searchparty.SectorStatusSightingReported, TotalAreaSqM: 700},
			},
		}
		if cov := noneParty.CalculateCoverage(); cov != 0.0 {
			t.Errorf("expected 0%% coverage, got %f", cov)
		}
	})
}

func TestSectorAssignment_Model(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	assignment := searchparty.SectorAssignment{
		AssignmentID:   "assign-123",
		SectorID:       "sector-1",
		VolunteerAlias: "Volunteer #1",
		ClaimedAt:      now,
		UpdatedAt:      now,
		Status:         searchparty.SectorStatusActiveSearch,
		ClearanceNotes: "Checking block",
	}

	if assignment.AssignmentID != "assign-123" {
		t.Errorf("unexpected AssignmentID: %s", assignment.AssignmentID)
	}
	if assignment.VolunteerAlias != "Volunteer #1" {
		t.Errorf("unexpected VolunteerAlias: %s", assignment.VolunteerAlias)
	}
	if assignment.Status != searchparty.SectorStatusActiveSearch {
		t.Errorf("unexpected Status: %s", assignment.Status)
	}
}

func TestSectorStatus_Constants(t *testing.T) {
	t.Parallel()

	if searchparty.SectorStatusUnassigned != "unassigned" {
		t.Errorf("SectorStatusUnassigned = %q, want unassigned", searchparty.SectorStatusUnassigned)
	}
	if searchparty.SectorStatusActiveSearch != "active_search" {
		t.Errorf("SectorStatusActiveSearch = %q, want active_search", searchparty.SectorStatusActiveSearch)
	}
	if searchparty.SectorStatusCleared != "cleared" {
		t.Errorf("SectorStatusCleared = %q, want cleared", searchparty.SectorStatusCleared)
	}
	if searchparty.SectorStatusSightingReported != "sighting_reported" {
		t.Errorf("SectorStatusSightingReported = %q, want sighting_reported", searchparty.SectorStatusSightingReported)
	}
}

func TestSearchParty_DecomposePerimeterMiles(t *testing.T) {
	t.Parallel()

	center := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}
	sectors := searchparty.DecomposePerimeterMiles(center, 1.0, 4)
	if len(sectors) != 4 {
		t.Fatalf("expected 4 sectors, got %d", len(sectors))
	}
	expectedArea := (math.Pi * 1609.344 * 1609.344) / 4.0
	if math.Abs(sectors[0].TotalAreaSqM-expectedArea) > 1.0 {
		t.Errorf("expected sector area ~%f, got %f", expectedArea, sectors[0].TotalAreaSqM)
	}
}

func TestSearchParty_DecomposePerimeterWithBearing(t *testing.T) {
	t.Parallel()

	center := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}
	// Bearing 45 degrees corresponds to Sector Alpha (0 to 90 degrees)
	sectors := searchparty.DecomposePerimeterWithBearing(center, 1000.0, 4, 45.0)
	if len(sectors) != 4 {
		t.Fatalf("expected 4 sectors, got %d", len(sectors))
	}
	// Sector Alpha should have the highest priority score
	if sectors[0].PriorityScore <= sectors[2].PriorityScore {
		t.Errorf("expected sector 0 priority (%f) > sector 2 priority (%f)", sectors[0].PriorityScore, sectors[2].PriorityScore)
	}
}

func TestSearchParty_Validation(t *testing.T) {
	t.Parallel()

	center := domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}
	validParty := searchparty.SearchParty{
		PartyID:           "party-1",
		LostPetID:         "pet-1",
		CenterCoordinates: center,
		RadiusMeters:      1000.0,
		CreatedAt:         time.Now().UTC(),
		Sectors: []searchparty.SearchSector{
			{
				SectorID: "s-1",
				Name:     "Sector Alpha (NE)",
				PolygonPoints: []domain.LocationPoint{
					center,
					{Latitude: 47.605, Longitude: -122.330},
					{Latitude: 47.605, Longitude: -122.325},
					center,
				},
				Status:        searchparty.SectorStatusUnassigned,
				PriorityScore: 1.0,
				TotalAreaSqM:  100.0,
			},
		},
	}

	if err := validParty.Validate(); err != nil {
		t.Errorf("expected valid party, got %v", err)
	}

	invalidParty := validParty
	invalidParty.PartyID = ""
	if err := invalidParty.Validate(); err == nil {
		t.Errorf("expected error for empty party ID")
	}

	invalidSector := validParty.Sectors[0]
	invalidSector.SectorID = ""
	if err := invalidSector.Validate(); err == nil {
		t.Errorf("expected error for empty sector ID")
	}

	assignment := searchparty.SectorAssignment{
		AssignmentID:   "a-1",
		SectorID:       "s-1",
		VolunteerAlias: "Volunteer #1",
		ClaimedAt:      time.Now().UTC(),
		Status:         searchparty.SectorStatusActiveSearch,
	}
	if err := assignment.Validate(); err != nil {
		t.Errorf("expected valid assignment, got %v", err)
	}

	invalidAssignment := assignment
	invalidAssignment.VolunteerAlias = ""
	if err := invalidAssignment.Validate(); err == nil {
		t.Errorf("expected error for empty volunteer alias")
	}
}
