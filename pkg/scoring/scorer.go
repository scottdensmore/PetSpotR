package scoring

import (
	"fmt"
	"math"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	WeightBreed          = 0.40
	WeightPrimaryColor   = 0.20
	WeightSecondaryColor = 0.10
	WeightMarkings       = 0.20
	WeightEyeColor       = 0.10
	MatchThreshold       = 0.70
)

// CalculateMatchScore computes a weighted similarity score between 0.0 and 1.0.
func CalculateMatchScore(t1, t2 *PetTraits) float64 {
	if t1 == nil || t2 == nil {
		return 0.0
	}

	var score float64

	// 1. Breed match (0.40)
	if compareStrings(t1.Breed, t2.Breed) {
		score += WeightBreed
	}

	// 2. Primary color match (0.20)
	if compareStrings(t1.PrimaryColor, t2.PrimaryColor) {
		score += WeightPrimaryColor
	}

	// 3. Secondary color match (0.10)
	if compareStrings(t1.SecondaryColor, t2.SecondaryColor) {
		score += WeightSecondaryColor
	}

	// 4. Distinctive markings match (0.20)
	score += WeightMarkings * compareMarkings(t1.DistinctiveMarkings, t2.DistinctiveMarkings)

	// 5. Eye color match (0.10)
	if compareStrings(t1.EyeColor, t2.EyeColor) {
		score += WeightEyeColor
	}

	return math.Min(1.0, math.Max(0.0, score))
}

// ComparePets generates a validated domain.MatchResult from two pet trait sets.
func ComparePets(lostPetID, foundPetID string, lostTraits, foundTraits *PetTraits) *domain.MatchResult {
	score := CalculateMatchScore(lostTraits, foundTraits)
	isMatch := score >= MatchThreshold

	details := fmt.Sprintf("Calculated visual match score: %.2f (Threshold: %.2f)", score, MatchThreshold)

	res := &domain.MatchResult{
		FoundPetID:   foundPetID,
		MatchedPetID: lostPetID,
		Score:        score,
		IsMatch:      isMatch,
		Details:      details,
	}

	if err := res.Validate(); err != nil {
		return nil
	}

	return res
}

func compareStrings(s1, s2 string) bool {
	s1Clean := strings.TrimSpace(strings.ToLower(s1))
	s2Clean := strings.TrimSpace(strings.ToLower(s2))
	if s1Clean == "" || s2Clean == "" {
		return false
	}
	return s1Clean == s2Clean
}

func compareMarkings(m1, m2 []string) float64 {
	set1 := make(map[string]bool)
	for _, item := range m1 {
		clean := strings.TrimSpace(strings.ToLower(item))
		if clean != "" {
			set1[clean] = true
		}
	}

	set2 := make(map[string]bool)
	for _, item := range m2 {
		clean := strings.TrimSpace(strings.ToLower(item))
		if clean != "" {
			set2[clean] = true
		}
	}

	if len(set1) == 0 && len(set2) == 0 {
		return 1.0
	}

	matches := 0
	for k := range set2 {
		if set1[k] {
			matches++
		}
	}

	union := len(set1)
	for k := range set2 {
		if !set1[k] {
			union++
		}
	}

	if union == 0 {
		return 0.0
	}

	return float64(matches) / float64(union)
}
