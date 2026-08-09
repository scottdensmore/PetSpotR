package domain_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestLostPetEvent_Validation(t *testing.T) {
	t.Run("valid lost pet event", func(t *testing.T) {
		evt := domain.LostPetEvent{
			PetID:         "pet-123",
			ReporterEmail: "owner@example.com",
			ReportedAt:    time.Now(),
			Location:      "Seattle, WA",
		}

		if err := evt.Validate(); err != nil {
			t.Fatalf("expected valid lost pet event, got error: %v", err)
		}
	})

	t.Run("missing pet ID", func(t *testing.T) {
		evt := domain.LostPetEvent{
			ReporterEmail: "owner@example.com",
		}

		if err := evt.Validate(); err == nil {
			t.Fatal("expected error for missing pet ID, got nil")
		}
	})

	t.Run("invalid reporter email format", func(t *testing.T) {
		evt := domain.LostPetEvent{
			PetID:         "pet-123",
			ReporterEmail: "invalid-email-format",
		}

		if err := evt.Validate(); err == nil {
			t.Fatal("expected error for invalid reporterEmail format, got nil")
		}
	})
}

func TestFoundPetEvent_Validation(t *testing.T) {
	t.Run("valid found pet event", func(t *testing.T) {
		evt := domain.FoundPetEvent{
			PetID:    "pet-found-999",
			ImageURL: "https://storage.googleapis.com/petspotr/found.jpg",
			FoundAt:  time.Now(),
			Location: "Bellevue, WA",
		}

		if err := evt.Validate(); err != nil {
			t.Fatalf("expected valid found pet event, got error: %v", err)
		}
	})

	t.Run("missing image URL", func(t *testing.T) {
		evt := domain.FoundPetEvent{
			PetID: "pet-found-999",
		}

		if err := evt.Validate(); err == nil {
			t.Fatal("expected error for missing image URL, got nil")
		}
	})
}

func TestMatchResult_Validation(t *testing.T) {
	t.Run("valid match result", func(t *testing.T) {
		result := domain.MatchResult{
			FoundPetID:   "found-1",
			MatchedPetID: "lost-1",
			Score:        0.85,
			IsMatch:      true,
			Details:      "High similarity on breed and fur pattern",
		}

		if err := result.Validate(); err != nil {
			t.Fatalf("expected valid match result, got error: %v", err)
		}
	})

	t.Run("missing found pet ID", func(t *testing.T) {
		result := domain.MatchResult{
			MatchedPetID: "lost-1",
			Score:        0.85,
		}

		if err := result.Validate(); err == nil {
			t.Fatal("expected error for missing foundPetId, got nil")
		}
	})

	t.Run("missing matched pet ID", func(t *testing.T) {
		result := domain.MatchResult{
			FoundPetID: "found-1",
			Score:      0.85,
		}

		if err := result.Validate(); err == nil {
			t.Fatal("expected error for missing matchedPetId, got nil")
		}
	})
}

func TestEvents_JSONSerialization(t *testing.T) {
	t.Run("LostPetEvent roundtrip", func(t *testing.T) {
		evt := domain.LostPetEvent{
			PetID:         "pet-123",
			ReporterEmail: "owner@example.com",
			Location:      "Seattle, WA",
		}
		data, err := evt.ToJSON()
		if err != nil {
			t.Fatalf("failed to serialize: %v", err)
		}
		var out domain.LostPetEvent
		if err := out.FromJSON(data); err != nil {
			t.Fatalf("failed to deserialize: %v", err)
		}
		if out.PetID != evt.PetID || out.ReporterEmail != evt.ReporterEmail {
			t.Errorf("mismatch: got %+v, want %+v", out, evt)
		}
	})

	t.Run("MatchResult roundtrip", func(t *testing.T) {
		res := domain.MatchResult{
			FoundPetID:   "found-1",
			MatchedPetID: "lost-1",
			Score:        0.91,
			IsMatch:      true,
		}
		data, err := res.ToJSON()
		if err != nil {
			t.Fatalf("failed to serialize: %v", err)
		}
		var out domain.MatchResult
		if err := out.FromJSON(data); err != nil {
			t.Fatalf("failed to deserialize: %v", err)
		}
		if out.FoundPetID != res.FoundPetID || out.Score != res.Score {
			t.Errorf("mismatch: got %+v, want %+v", out, res)
		}
	})
}
