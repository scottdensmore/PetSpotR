package domain_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestPetSightingRecord_Validation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	valid := domain.PetSightingRecord{
		SightingID:          "sight-1",
		LostPetID:           "lost-1",
		ReportedAt:          now,
		SightedAt:           now.Add(-10 * time.Minute),
		LocationDescription: "Near 4th & Pine",
		Coordinates: &domain.LocationPoint{
			Latitude:  47.611,
			Longitude: -122.338,
		},
		MovementDirection: "Northeast",
		Status:            domain.SightingStatusActive,
	}

	t.Run("valid sighting passes", func(t *testing.T) {
		if err := valid.Validate(); err != nil {
			t.Fatalf("expected valid sighting, got error: %v", err)
		}
	})

	t.Run("empty sighting ID fails", func(t *testing.T) {
		s := valid
		s.SightingID = ""
		if err := s.Validate(); err == nil {
			t.Error("expected error for empty sighting ID, got nil")
		}
	})

	t.Run("empty lost pet ID fails", func(t *testing.T) {
		s := valid
		s.LostPetID = "   "
		if err := s.Validate(); err == nil {
			t.Error("expected error for empty lost pet ID, got nil")
		}
	})

	t.Run("zero sightedAt fails", func(t *testing.T) {
		s := valid
		s.SightedAt = time.Time{}
		if err := s.Validate(); err == nil {
			t.Error("expected error for zero sightedAt, got nil")
		}
	})

	t.Run("nil coordinates fails", func(t *testing.T) {
		s := valid
		s.Coordinates = nil
		if err := s.Validate(); err == nil {
			t.Error("expected error for nil coordinates, got nil")
		}
	})

	t.Run("invalid latitude > 90 fails", func(t *testing.T) {
		s := valid
		s.Coordinates = &domain.LocationPoint{Latitude: 95.0, Longitude: 0.0}
		if err := s.Validate(); err == nil {
			t.Error("expected error for latitude > 90, got nil")
		}
	})

	t.Run("invalid longitude > 180 fails", func(t *testing.T) {
		s := valid
		s.Coordinates = &domain.LocationPoint{Latitude: 45.0, Longitude: 185.0}
		if err := s.Validate(); err == nil {
			t.Error("expected error for longitude > 180, got nil")
		}
	})

	t.Run("invalid status fails", func(t *testing.T) {
		s := valid
		s.Status = "unknown_status"
		if err := s.Validate(); err == nil {
			t.Error("expected error for invalid status, got nil")
		}
	})

	t.Run("valid flagged and dismissed statuses pass", func(t *testing.T) {
		sFlagged := valid
		sFlagged.Status = domain.SightingStatusFlagged
		if err := sFlagged.Validate(); err != nil {
			t.Errorf("expected flagged sighting to be valid, got: %v", err)
		}

		sDismissed := valid
		sDismissed.Status = domain.SightingStatusDismissed
		if err := sDismissed.Validate(); err != nil {
			t.Errorf("expected dismissed sighting to be valid, got: %v", err)
		}
	})
}
