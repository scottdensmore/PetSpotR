package scoring_test

import (
	"math"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/scoring"
)

func TestCalculateMatchScore(t *testing.T) {
	t.Run("identical traits returns score 1.0", func(t *testing.T) {
		t1 := &scoring.PetTraits{
			Breed:               "Golden Retriever",
			PrimaryColor:        "Golden",
			SecondaryColor:      "White",
			DistinctiveMarkings: []string{"White patch on chest"},
			EyeColor:            "Brown",
		}
		t2 := &scoring.PetTraits{
			Breed:               "golden retriever",
			PrimaryColor:        "golden",
			SecondaryColor:      "white",
			DistinctiveMarkings: []string{"white patch on chest"},
			EyeColor:            "brown",
		}

		score := scoring.CalculateMatchScore(t1, t2)
		if math.Abs(score-1.0) > 0.001 {
			t.Errorf("expected score 1.0, got %f", score)
		}
	})

	t.Run("Jaccard similarity deduplicates markings and caps at 1.0", func(t *testing.T) {
		t1 := &scoring.PetTraits{
			Breed:               "Labrador",
			DistinctiveMarkings: []string{"white patch", "white patch"},
		}
		t2 := &scoring.PetTraits{
			Breed:               "Labrador",
			DistinctiveMarkings: []string{"white patch"},
		}

		score := scoring.CalculateMatchScore(t1, t2)
		// Breed (0.40) + Markings Jaccard (1.0 * 0.20) = 0.60
		expected := 0.60
		if math.Abs(score-expected) > 0.001 {
			t.Errorf("expected score %f, got %f", expected, score)
		}
	})

	t.Run("whitespace-only markings return 1.0 Jaccard factor", func(t *testing.T) {
		t1 := &scoring.PetTraits{Breed: "Beagle", DistinctiveMarkings: []string{"   "}}
		t2 := &scoring.PetTraits{Breed: "Beagle", DistinctiveMarkings: []string{"   "}}

		score := scoring.CalculateMatchScore(t1, t2)
		// Breed (0.40) + Empty Markings Jaccard (1.0 * 0.20) = 0.60
		expected := 0.60
		if math.Abs(score-expected) > 0.001 {
			t.Errorf("expected score %f, got %f", expected, score)
		}
	})

	t.Run("partial match breed and color", func(t *testing.T) {
		t1 := &scoring.PetTraits{
			Breed:        "Labrador",
			PrimaryColor: "Black",
			EyeColor:     "Brown",
		}
		t2 := &scoring.PetTraits{
			Breed:        "Labrador",
			PrimaryColor: "Black",
			EyeColor:     "Green", // Mismatch
		}

		score := scoring.CalculateMatchScore(t1, t2)
		// Breed (0.40) + Primary Color (0.20) + Empty Markings (0.20) = 0.80
		if score < 0.70 {
			t.Errorf("expected score >= 0.70 for matching breed & primary color, got %f", score)
		}
	})

	t.Run("exact breed name check prevents false positive contains match", func(t *testing.T) {
		t1 := &scoring.PetTraits{Breed: "Cat"}
		t2 := &scoring.PetTraits{Breed: "Cattle Dog"}

		score := scoring.CalculateMatchScore(t1, t2)
		if score >= 0.40 {
			t.Errorf("expected score < 0.40 for Cat vs Cattle Dog, got %f", score)
		}
	})

	t.Run("nil traits input returns 0.0", func(t *testing.T) {
		score := scoring.CalculateMatchScore(nil, nil)
		if score != 0.0 {
			t.Errorf("expected score 0.0 for nil inputs, got %f", score)
		}
	})
}

func TestComparePets(t *testing.T) {
	t.Run("creates validated MatchResult domain model", func(t *testing.T) {
		lost := &scoring.PetTraits{Breed: "Beagle", PrimaryColor: "Tricolor"}
		found := &scoring.PetTraits{Breed: "Beagle", PrimaryColor: "Tricolor"}

		res := scoring.ComparePets("pet-lost-1", "pet-found-2", lost, found)
		if res == nil {
			t.Fatal("expected non-nil MatchResult")
		}

		if err := res.Validate(); err != nil {
			t.Fatalf("MatchResult validation failed: %v", err)
		}

		if res.FoundPetID != "pet-found-2" || res.MatchedPetID != "pet-lost-1" {
			t.Errorf("ID mismatch: got found %s, matched %s", res.FoundPetID, res.MatchedPetID)
		}

		if !res.IsMatch {
			t.Errorf("expected IsMatch true, got false")
		}
	})

	t.Run("invalid IDs return nil MatchResult", func(t *testing.T) {
		lost := &scoring.PetTraits{Breed: "Beagle"}
		found := &scoring.PetTraits{Breed: "Beagle"}

		res := scoring.ComparePets("", "", lost, found)
		if res != nil {
			t.Errorf("expected nil for empty pet IDs, got %+v", res)
		}
	})
}
