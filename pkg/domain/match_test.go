package domain_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestMatchRecordUsesStableIdentityAndValidatedProvenance(t *testing.T) {
	first, err := domain.StableMatchID("evt-found-1", "found-1", "lost-1")
	if err != nil {
		t.Fatal(err)
	}
	retry, err := domain.StableMatchID("evt-found-1", "found-1", "lost-1")
	if err != nil {
		t.Fatal(err)
	}
	distinct, err := domain.StableMatchID("evt-found-2", "found-1", "lost-1")
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || retry != first || distinct == first {
		t.Fatalf("stable match IDs = %q, %q, %q", first, retry, distinct)
	}

	record := domain.MatchRecord{
		MatchID:      first,
		FoundPetID:   "found-1",
		MatchedPetID: "lost-1",
		Score:        0.91,
		Status:       domain.MatchStatusPendingReview,
		MatchedAt:    time.Date(2026, time.August, 16, 13, 0, 0, 0, time.UTC),
		Scores: domain.MatchScoreBreakdown{
			Visual:        0.95,
			Color:         1,
			Spatial:       0.82,
			DistanceMiles: 2.7,
			Threshold:     0.7,
		},
		LostPet:          domain.MatchPetDetail{PetID: "lost-1", Breed: "Golden Retriever"},
		FoundPet:         domain.MatchPetDetail{PetID: "found-1", Breed: "Golden Retriever"},
		SourceEventID:    "evt-found-1",
		Model:            "gemma4:e2b",
		ThresholdVersion: "visual-spatial-v1",
		Explanation:      "High visual and geographic similarity",
	}
	if err := record.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	record.FoundPet.PetID = "found-other"
	if err := record.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want mismatched snapshot rejection")
	}
	record.FoundPet.PetID = record.FoundPetID
	record.Scores.Threshold = 0
	if err := record.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want missing threshold rejection")
	}
}
