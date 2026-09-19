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

func TestComparePetsGeo(t *testing.T) {
	t.Run("calculates distance-weighted combined match score", func(t *testing.T) {
		lost := &scoring.PetTraits{Breed: "Golden Retriever", PrimaryColor: "Golden", SecondaryColor: "White", DistinctiveMarkings: []string{"White patch on chest"}, EyeColor: "Brown"}
		found := &scoring.PetTraits{Breed: "Golden Retriever", PrimaryColor: "Golden", SecondaryColor: "White", DistinctiveMarkings: []string{"White patch on chest"}, EyeColor: "Brown"}

		// Capitol Hill to Green Lake (~4.5 miles)
		res := scoring.ComparePetsGeo("lost-101", "found-202", "Capitol Hill, Seattle, WA", "Green Lake Park, Seattle, WA", lost, found)

		if res == nil {
			t.Fatal("expected non-nil MatchResult")
		}

		if !res.IsMatch {
			t.Errorf("expected IsMatch to be true, got false")
		}

		if res.Score < 0.70 {
			t.Errorf("expected combined score >= 0.70, got %f", res.Score)
		}
		if res.Scores == nil || res.Scores.Visual != 1 || res.Scores.Color != 1 ||
			res.Scores.Spatial <= 0 || res.Scores.DistanceMiles <= 0 ||
			res.Scores.Threshold != scoring.MatchThreshold || res.ThresholdVersion != scoring.MatchThresholdVersion {
			t.Fatalf("score provenance = %#v; threshold version = %q", res.Scores, res.ThresholdVersion)
		}
	})
}

func TestComparePetsAtDistanceUsesExplicitVerifiedDistance(t *testing.T) {
	traits := &scoring.PetTraits{
		Breed: "Golden Retriever", PrimaryColor: "Golden", SecondaryColor: "Cream",
		DistinctiveMarkings: []string{"White chest patch"}, EyeColor: "Brown",
	}
	result := scoring.ComparePetsAtDistance("lost-verified", "found-verified", 4.25, traits, traits)
	if result == nil || !result.IsMatch || result.Scores == nil {
		t.Fatalf("ComparePetsAtDistance() = %#v, want a scored match", result)
	}
	if result.Scores.DistanceMiles != 4.25 {
		t.Fatalf("distance = %f, want exact verified distance 4.25", result.Scores.DistanceMiles)
	}
	if result := scoring.ComparePetsAtDistance("lost-invalid", "found-invalid", -1, traits, traits); result != nil {
		t.Fatalf("negative-distance result = %#v, want nil", result)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name      string
		u         []float32
		v         []float32
		wantScore float64
		tolerance float64
	}{
		{
			name:      "identical vectors",
			u:         []float32{1, 0, 0},
			v:         []float32{1, 0, 0},
			wantScore: 1.0,
			tolerance: 1e-4,
		},
		{
			name:      "orthogonal vectors",
			u:         []float32{1, 0, 0},
			v:         []float32{0, 1, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "opposite vectors clamped to 0",
			u:         []float32{1, 0, 0},
			v:         []float32{-1, 0, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "empty vectors return 0",
			u:         nil,
			v:         []float32{1, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "both vectors nil return 0",
			u:         nil,
			v:         nil,
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "mismatched lengths return 0",
			u:         []float32{1, 0},
			v:         []float32{1, 0, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "zero vectors return 0",
			u:         []float32{0, 0, 0},
			v:         []float32{0, 0, 0},
			wantScore: 0.0,
			tolerance: 1e-4,
		},
		{
			name:      "45-degree angle vectors",
			u:         []float32{1, 1},
			v:         []float32{1, 0},
			wantScore: 1.0 / math.Sqrt(2),
			tolerance: 1e-4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := scoring.CosineSimilarity(tt.u, tt.v)
			if math.Abs(got-tt.wantScore) > tt.tolerance {
				t.Errorf("CosineSimilarity() = %v, want %v", got, tt.wantScore)
			}
		})
	}
}

func TestHybridTriFactorScoring(t *testing.T) {
	t.Run("linear combination weights", func(t *testing.T) {
		// 0.40 * 1.0 + 0.35 * 0.8 + 0.25 * 0.6 = 0.40 + 0.28 + 0.15 = 0.83
		score := scoring.CalculateHybridMatchScore(1.0, 0.8, 0.6)
		expected := 0.83
		if math.Abs(score-expected) > 1e-4 {
			t.Errorf("CalculateHybridMatchScore = %f, want %f", score, expected)
		}
	})

	t.Run("clamping at upper and lower bounds", func(t *testing.T) {
		if s := scoring.CalculateHybridMatchScore(2.0, 2.0, 2.0); s > 1.0 {
			t.Errorf("expected clamp to 1.0, got %f", s)
		}
		if s := scoring.CalculateHybridMatchScore(-1.0, -1.0, -1.0); s < 0.0 {
			t.Errorf("expected clamp to 0.0, got %f", s)
		}
	})

	t.Run("species mismatch hard veto", func(t *testing.T) {
		traits1 := &scoring.PetTraits{Breed: "Golden Retriever"}
		traits2 := &scoring.PetTraits{Breed: "Golden Retriever"}
		u := []float32{1, 0}
		v := []float32{1, 0}
		result := scoring.ComparePetsHybrid("lost-1", "found-1", "Dog", "Cat", 0.5, traits1, traits2, u, v)
		if result == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		if result.IsMatch || result.Score != 0.0 {
			t.Fatalf("expected species veto score 0.0 and IsMatch=false, got score=%f, isMatch=%t", result.Score, result.IsMatch)
		}
		if result.Scores == nil || result.Scores.Vector != 0.0 {
			t.Fatalf("expected vector score 0.0 in breakdown on veto, got %+v", result.Scores)
		}
		if result.ThresholdVersion != scoring.HybridThresholdVersion {
			t.Errorf("expected threshold version %s, got %s", scoring.HybridThresholdVersion, result.ThresholdVersion)
		}
	})

	t.Run("species matching is case-insensitive", func(t *testing.T) {
		traits1 := &scoring.PetTraits{Breed: "Golden Retriever"}
		traits2 := &scoring.PetTraits{Breed: "Golden Retriever"}
		u := []float32{1, 0}
		v := []float32{1, 0}
		result := scoring.ComparePetsHybrid("lost-1", "found-1", "Dog", "dog", 0.0, traits1, traits2, u, v)
		if result == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		if result.Score == 0.0 {
			t.Fatalf("expected non-zero score for case-insensitive matching species, got 0.0")
		}
	})

	t.Run("tri-factor scoring with both embeddings", func(t *testing.T) {
		traits1 := &scoring.PetTraits{Breed: "Golden Retriever", PrimaryColor: "Golden"}
		traits2 := &scoring.PetTraits{Breed: "Golden Retriever", PrimaryColor: "Golden"}
		// traits: Breed 0.40 + PrimaryColor 0.20 + Markings 0.20 = 0.80
		// spatial: 0 miles = 1.0
		// vector: identical = 1.0
		// hybrid: 0.40 * 1.0 + 0.35 * 0.80 + 0.25 * 1.0 = 0.40 + 0.28 + 0.25 = 0.93
		u := []float32{1, 0, 0}
		v := []float32{1, 0, 0}
		result := scoring.ComparePetsHybrid("lost-1", "found-1", "Dog", "Dog", 0.0, traits1, traits2, u, v)
		if result == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		expected := 0.93
		if math.Abs(result.Score-expected) > 1e-3 {
			t.Fatalf("expected hybrid score ~%f, got %f", expected, result.Score)
		}
		if !result.IsMatch {
			t.Errorf("expected IsMatch=true, got false")
		}
		if result.Scores == nil {
			t.Fatal("expected non-nil Scores breakdown")
		}
		if math.Abs(result.Scores.Vector-1.0) > 1e-4 {
			t.Errorf("expected breakdown Vector 1.0, got %f", result.Scores.Vector)
		}
		if math.Abs(result.Scores.Trait-0.80) > 1e-4 {
			t.Errorf("expected breakdown Trait 0.80, got %f", result.Scores.Trait)
		}
		if math.Abs(result.Scores.Visual-0.80) > 1e-4 {
			t.Errorf("expected breakdown Visual 0.80, got %f", result.Scores.Visual)
		}
		if math.Abs(result.Scores.Spatial-1.0) > 1e-4 {
			t.Errorf("expected breakdown Spatial 1.0, got %f", result.Scores.Spatial)
		}
		if result.Scores.DistanceMiles != 0.0 {
			t.Errorf("expected DistanceMiles 0.0, got %f", result.Scores.DistanceMiles)
		}
		if result.Scores.Threshold != scoring.MatchThreshold {
			t.Errorf("expected Threshold %f, got %f", scoring.MatchThreshold, result.Scores.Threshold)
		}
		if result.ThresholdVersion != scoring.HybridThresholdVersion {
			t.Errorf("expected threshold version %s, got %s", scoring.HybridThresholdVersion, result.ThresholdVersion)
		}
	})

	t.Run("fallback when embeddings missing", func(t *testing.T) {
		traits1 := &scoring.PetTraits{Breed: "Golden Retriever", DistinctiveMarkings: []string{"scar"}}
		traits2 := &scoring.PetTraits{Breed: "Golden Retriever", DistinctiveMarkings: []string{"spot"}}
		// traits match = 0.40 breed (different markings = 0.0), spatial = 1.0 (0 miles)
		// legacy: 0.70 * 0.40 + 0.30 * 1.0 = 0.28 + 0.30 = 0.58
		result := scoring.ComparePetsHybrid("lost-1", "found-1", "Dog", "Dog", 0.0, traits1, traits2, nil, nil)
		if result == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		expected := 0.58
		if math.Abs(result.Score-expected) > 1e-3 {
			t.Fatalf("expected fallback score ~%f, got %f", expected, result.Score)
		}
		if result.Scores.Vector != 0.0 {
			t.Fatalf("expected vector score 0.0 when fallback, got %f", result.Scores.Vector)
		}
		if math.Abs(result.Scores.Trait-0.40) > 1e-4 {
			t.Fatalf("expected trait score 0.40, got %f", result.Scores.Trait)
		}
	})

	t.Run("fallback when one embedding is missing", func(t *testing.T) {
		traits1 := &scoring.PetTraits{Breed: "Golden Retriever", DistinctiveMarkings: []string{"scar"}}
		traits2 := &scoring.PetTraits{Breed: "Golden Retriever", DistinctiveMarkings: []string{"spot"}}
		u := []float32{1, 0, 0}

		// lost has embedding, found does not
		r1 := scoring.ComparePetsHybrid("lost-1", "found-1", "Dog", "Dog", 0.0, traits1, traits2, u, nil)
		if r1 == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		expected := 0.58
		if math.Abs(r1.Score-expected) > 1e-3 {
			t.Fatalf("expected fallback score ~%f, got %f", expected, r1.Score)
		}
		if r1.Scores.Vector != 0.0 {
			t.Fatalf("expected vector score 0.0 when fallback, got %f", r1.Scores.Vector)
		}

		// lost does not have embedding, found does
		r2 := scoring.ComparePetsHybrid("lost-1", "found-1", "Dog", "Dog", 0.0, traits1, traits2, nil, u)
		if r2 == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		if math.Abs(r2.Score-expected) > 1e-3 {
			t.Fatalf("expected fallback score ~%f, got %f", expected, r2.Score)
		}
		if r2.Scores.Vector != 0.0 {
			t.Fatalf("expected vector score 0.0 when fallback, got %f", r2.Scores.Vector)
		}
	})

	t.Run("invalid distance returns nil", func(t *testing.T) {
		traits := &scoring.PetTraits{Breed: "Beagle"}
		u := []float32{1, 0}
		if r := scoring.ComparePetsHybrid("l1", "f1", "Dog", "Dog", -1.0, traits, traits, u, u); r != nil {
			t.Fatalf("expected nil for negative distance, got %+v", r)
		}
		if r := scoring.ComparePetsHybrid("l1", "f1", "Dog", "Dog", math.NaN(), traits, traits, u, u); r != nil {
			t.Fatalf("expected nil for NaN distance, got %+v", r)
		}
		if r := scoring.ComparePetsHybrid("l1", "f1", "Dog", "Dog", math.Inf(1), traits, traits, u, u); r != nil {
			t.Fatalf("expected nil for Inf distance, got %+v", r)
		}
	})

	t.Run("invalid pet IDs return nil", func(t *testing.T) {
		traits := &scoring.PetTraits{Breed: "Beagle"}
		u := []float32{1, 0}
		if r := scoring.ComparePetsHybrid("", "f1", "Dog", "Dog", 1.0, traits, traits, u, u); r != nil {
			t.Fatalf("expected nil for empty lost pet ID, got %+v", r)
		}
		if r := scoring.ComparePetsHybrid("l1", "", "Dog", "Dog", 1.0, traits, traits, u, u); r != nil {
			t.Fatalf("expected nil for empty found pet ID, got %+v", r)
		}
		if r := scoring.ComparePetsHybrid("", "f1", "Dog", "Cat", 1.0, traits, traits, u, u); r != nil {
			t.Fatalf("expected nil for empty lost pet ID even on species mismatch, got %+v", r)
		}
	})
}

func TestCalculateDeterministicMatchScore(t *testing.T) {
	t.Parallel()

	t.Run("Identical microchips return score 1.0 and deterministic true", func(t *testing.T) {
		res := scoring.EvaluateMicrochipMatch("985141000123456", "985141000123456")
		if !res.IsMatch || res.Score != 1.0 || !res.Deterministic || res.Mismatch {
			t.Fatalf("expected deterministic 1.0 match, got %+v", res)
		}
	})

	t.Run("Identical microchips with formatting differences match", func(t *testing.T) {
		res := scoring.EvaluateMicrochipMatch("985-141-000 123 456", "985141000123456")
		if !res.IsMatch || res.Score != 1.0 || !res.Deterministic || res.Mismatch {
			t.Fatalf("expected deterministic 1.0 match with formatting, got %+v", res)
		}
	})

	t.Run("Conflicting microchips return score 0.0 and mismatch true", func(t *testing.T) {
		res := scoring.EvaluateMicrochipMatch("985141000123456", "981010000999999")
		if res.IsMatch || res.Score != 0.0 || res.Deterministic || !res.Mismatch {
			t.Fatalf("expected mismatch 0.0 penalty, got %+v", res)
		}
	})

	t.Run("Missing microchip in one or both delegates to probabilistic scoring", func(t *testing.T) {
		tests := []struct {
			name  string
			lost  string
			found string
		}{
			{"lost only", "985141000123456", ""},
			{"found only", "", "985141000123456"},
			{"both empty", "", ""},
			{"invalid format lost", "bad-chip", "985141000123456"},
			{"invalid format both", "bad-chip", "bad-chip"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				res := scoring.EvaluateMicrochipMatch(tc.lost, tc.found)
				if res.IsMatch || res.Score != 0.0 || res.Deterministic || res.Mismatch {
					t.Fatalf("expected non-deterministic non-mismatch result, got %+v", res)
				}
			})
		}
	})
}

func TestComparePetsHybridWithMicrochip(t *testing.T) {
	t.Parallel()

	traits := &scoring.PetTraits{
		Breed:               "Golden Retriever",
		PrimaryColor:        "Golden",
		SecondaryColor:      "Cream",
		DistinctiveMarkings: []string{"White patch on chest"},
		EyeColor:            "Brown",
	}
	vec := []float32{1.0, 0.0}

	t.Run("identical microchips return 1.0 deterministic match", func(t *testing.T) {
		res := scoring.ComparePetsHybridWithMicrochip(
			"lost-1", "found-1",
			"Dog", "Dog",
			"985141000123456", "985141000123456",
			5.0,
			traits, traits,
			vec, vec,
		)
		if res == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		if res.Score != 1.0 || !res.IsMatch || !res.DeterministicMatch {
			t.Fatalf("expected 1.0 deterministic match, got Score=%f, IsMatch=%v, Deterministic=%v",
				res.Score, res.IsMatch, res.DeterministicMatch)
		}
		if res.MatchType != "deterministic_microchip" {
			t.Errorf("expected matchType deterministic_microchip, got %s", res.MatchType)
		}
		if res.MatchedMicrochip != "HomeAgain ••••3456" {
			t.Errorf("expected masked microchip HomeAgain ••••3456, got %s", res.MatchedMicrochip)
		}
		if res.Scores.MicrochipMatch != 1.0 {
			t.Errorf("expected MicrochipMatch score 1.0, got %f", res.Scores.MicrochipMatch)
		}
		if res.Scores.Visual <= 0 || res.Scores.Spatial <= 0 || res.Scores.Vector <= 0 {
			t.Errorf("expected preserved subscores, got %+v", res.Scores)
		}
	})

	t.Run("conflicting microchips return 0.0 mismatch veto", func(t *testing.T) {
		res := scoring.ComparePetsHybridWithMicrochip(
			"lost-1", "found-1",
			"Dog", "Dog",
			"985141000123456", "981010000999999",
			0.0,
			traits, traits,
			vec, vec,
		)
		if res == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		if res.Score != 0.0 || res.IsMatch || res.DeterministicMatch {
			t.Fatalf("expected 0.0 non-match veto, got Score=%f, IsMatch=%v", res.Score, res.IsMatch)
		}
		if res.Details != "Microchip mismatch veto (score forced to 0.0)" {
			t.Errorf("expected veto details, got %s", res.Details)
		}
		if res.Scores.MicrochipMatch != 0.0 {
			t.Errorf("expected MicrochipMatch 0.0, got %f", res.Scores.MicrochipMatch)
		}
	})

	t.Run("single or unchipped falls back to hybrid scoring", func(t *testing.T) {
		res := scoring.ComparePetsHybridWithMicrochip(
			"lost-1", "found-1",
			"Dog", "Dog",
			"985141000123456", "",
			0.0,
			traits, traits,
			vec, vec,
		)
		if res == nil {
			t.Fatal("expected non-nil MatchResult")
		}
		if res.DeterministicMatch {
			t.Errorf("expected non-deterministic match when one chip missing")
		}
		if res.Score < 0.70 || !res.IsMatch {
			t.Errorf("expected hybrid match >= 0.70, got %f", res.Score)
		}
	})

	t.Run("invalid distance returns nil", func(t *testing.T) {
		res := scoring.ComparePetsHybridWithMicrochip(
			"lost-1", "found-1", "Dog", "Dog",
			"985141000123456", "985141000123456",
			-1.0, traits, traits, vec, vec,
		)
		if res != nil {
			t.Fatalf("expected nil for negative distance, got %+v", res)
		}
	})
}
