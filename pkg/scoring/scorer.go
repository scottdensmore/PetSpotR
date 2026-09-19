package scoring

import (
	"fmt"
	"math"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/microchip"
)

const (
	WeightBreed            = 0.40
	WeightPrimaryColor     = 0.20
	WeightSecondaryColor   = 0.10
	WeightMarkings         = 0.20
	WeightEyeColor         = 0.10
	MatchThreshold         = 0.70
	MatchRadiusMiles       = 15.0
	MatchThresholdVersion  = "visual-spatial-v1"
	WeightVectorSimilarity = 0.40
	WeightTraitSimilarity  = 0.35
	WeightSpatialProximity = 0.25
	HybridThresholdVersion = "hybrid-multimodal-v1"
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

// CalculateDistanceScore computes a spatial score between 0.0 and 1.0 based on physical distance miles.
func CalculateDistanceScore(distanceMiles, maxRadiusMiles float64) float64 {
	if maxRadiusMiles <= 0 {
		maxRadiusMiles = 15.0
	}
	if distanceMiles <= 0 {
		return 1.0
	}
	if distanceMiles >= maxRadiusMiles {
		return 0.0
	}
	return math.Max(0.0, 1.0-(distanceMiles/maxRadiusMiles))
}

// CalculateCombinedMatchScore combines visual similarity (70% weight) and spatial proximity (30% weight).
func CalculateCombinedMatchScore(visualScore, spatialScore float64) float64 {
	combined := (0.70 * visualScore) + (0.30 * spatialScore)
	return math.Min(1.0, math.Max(0.0, combined))
}

// CalculateLegacyMatchScore combines visual similarity (70% weight) and spatial proximity (30% weight).
func CalculateLegacyMatchScore(traitScore, spatialScore float64) float64 {
	return CalculateCombinedMatchScore(traitScore, spatialScore)
}

// ComparePets generates a validated domain.MatchResult from two pet trait sets.
func ComparePets(lostPetID, foundPetID string, lostTraits, foundTraits *PetTraits) *domain.MatchResult {
	return ComparePetsGeo(lostPetID, foundPetID, "", "", lostTraits, foundTraits)
}

// ComparePetsGeo generates a validated domain.MatchResult combining visual similarity and spatial distance.
func ComparePetsGeo(lostPetID, foundPetID, lostLocation, foundLocation string, lostTraits, foundTraits *PetTraits) *domain.MatchResult {
	p1 := domain.ParseLocationCoordinates(lostLocation)
	p2 := domain.ParseLocationCoordinates(foundLocation)
	distMiles := domain.HaversineDistanceMiles(p1, p2)
	return ComparePetsAtDistance(lostPetID, foundPetID, distMiles, lostTraits, foundTraits)
}

// ComparePetsAtDistance generates a validated MatchResult from an explicit,
// verified distance without parsing or inventing location coordinates.
func ComparePetsAtDistance(lostPetID, foundPetID string, distMiles float64, lostTraits, foundTraits *PetTraits) *domain.MatchResult {
	if math.IsNaN(distMiles) || math.IsInf(distMiles, 0) || distMiles < 0 {
		return nil
	}
	visualScore := CalculateMatchScore(lostTraits, foundTraits)
	spatialScore := CalculateDistanceScore(distMiles, MatchRadiusMiles)
	colorScore := calculateColorScore(lostTraits, foundTraits)

	combinedScore := CalculateCombinedMatchScore(visualScore, spatialScore)
	isMatch := combinedScore >= MatchThreshold

	details := fmt.Sprintf("Match score: %.2f (Visual: %.2f, Spatial: %.2f, Distance: %.1f mi, Threshold: %.2f)",
		combinedScore, visualScore, spatialScore, distMiles, MatchThreshold)

	res := &domain.MatchResult{
		FoundPetID:   foundPetID,
		MatchedPetID: lostPetID,
		Score:        combinedScore,
		IsMatch:      isMatch,
		Details:      details,
		Scores: &domain.MatchScoreBreakdown{
			Visual:        visualScore,
			Color:         colorScore,
			Spatial:       spatialScore,
			DistanceMiles: distMiles,
			Threshold:     MatchThreshold,
		},
		ThresholdVersion: MatchThresholdVersion,
	}

	if err := res.Validate(); err != nil {
		return nil
	}

	return res
}

// CosineSimilarity computes cosine similarity between two float32 vectors, clamped to [0.0, 1.0].
func CosineSimilarity(u, v []float32) float64 {
	if len(u) == 0 || len(v) == 0 || len(u) != len(v) {
		return 0.0
	}
	var dot, normU, normV float64
	for i := range u {
		dot += float64(u[i]) * float64(v[i])
		normU += float64(u[i]) * float64(u[i])
		normV += float64(v[i]) * float64(v[i])
	}
	if normU <= 0 || normV <= 0 {
		return 0.0
	}
	denom := math.Sqrt(normU) * math.Sqrt(normV)
	if denom <= 0 || math.IsNaN(denom) || math.IsInf(denom, 0) {
		return 0.0
	}
	sim := dot / denom
	if math.IsNaN(sim) || math.IsInf(sim, 0) {
		return 0.0
	}
	return math.Min(1.0, math.Max(0.0, sim))
}

// CalculateHybridMatchScore combines semantic vector (40%), discrete traits (35%), and spatial proximity (25%).
func CalculateHybridMatchScore(vectorScore, traitScore, spatialScore float64) float64 {
	combined := (WeightVectorSimilarity * vectorScore) +
		(WeightTraitSimilarity * traitScore) +
		(WeightSpatialProximity * spatialScore)
	return math.Min(1.0, math.Max(0.0, combined))
}

// MicrochipMatchResult defines the deterministic microchip matching evaluation outcome.
type MicrochipMatchResult struct {
	IsMatch       bool
	Score         float64
	Deterministic bool
	Mismatch      bool
}

// EvaluateMicrochipMatch compares two microchips and returns deterministic match or mismatch outcomes.
func EvaluateMicrochipMatch(lostChip, foundChip string) MicrochipMatchResult {
	normLost := microchip.Normalize(lostChip)
	normFound := microchip.Normalize(foundChip)

	if normLost != "" && normFound != "" {
		if normLost == normFound {
			return MicrochipMatchResult{
				IsMatch:       true,
				Score:         1.0,
				Deterministic: true,
				Mismatch:      false,
			}
		}
		return MicrochipMatchResult{
			IsMatch:       false,
			Score:         0.0,
			Deterministic: false,
			Mismatch:      true,
		}
	}
	return MicrochipMatchResult{
		IsMatch:       false,
		Score:         0.0,
		Deterministic: false,
		Mismatch:      false,
	}
}

// ComparePetsHybridWithMicrochip scores two reports using microchip deterministic matching,
// falling back to tri-factor hybrid scoring and enforcing hard species and microchip mismatch vetoes.
func ComparePetsHybridWithMicrochip(
	lostPetID, foundPetID string,
	lostSpecies, foundSpecies string,
	lostMicrochip, foundMicrochip string,
	distMiles float64,
	lostTraits, foundTraits *PetTraits,
	lostEmbedding, foundEmbedding []float32,
) *domain.MatchResult {
	if math.IsNaN(distMiles) || math.IsInf(distMiles, 0) || distMiles < 0 {
		return nil
	}

	traitScore := CalculateMatchScore(lostTraits, foundTraits)
	spatialScore := CalculateDistanceScore(distMiles, MatchRadiusMiles)
	colorScore := calculateColorScore(lostTraits, foundTraits)

	var vectorScore float64
	hasVector := len(lostEmbedding) > 0 && len(foundEmbedding) > 0
	if hasVector {
		vectorScore = CosineSimilarity(lostEmbedding, foundEmbedding)
	}

	chipResult := EvaluateMicrochipMatch(lostMicrochip, foundMicrochip)
	if chipResult.Mismatch {
		res := &domain.MatchResult{
			FoundPetID:   foundPetID,
			MatchedPetID: lostPetID,
			Score:        0.0,
			IsMatch:      false,
			Details:      "Microchip mismatch veto (score forced to 0.0)",
			Scores: &domain.MatchScoreBreakdown{
				Visual:         traitScore,
				Trait:          traitScore,
				Color:          colorScore,
				Spatial:        spatialScore,
				DistanceMiles:  distMiles,
				Threshold:      MatchThreshold,
				Vector:         vectorScore,
				MicrochipMatch: 0.0,
			},
			ThresholdVersion: HybridThresholdVersion,
		}
		if err := res.Validate(); err != nil {
			return nil
		}
		return res
	}

	if chipResult.IsMatch && chipResult.Deterministic {
		maskedChip := microchip.MaskMicrochip(lostMicrochip)
		res := &domain.MatchResult{
			FoundPetID:         foundPetID,
			MatchedPetID:       lostPetID,
			Score:              1.0,
			IsMatch:            true,
			Details:            fmt.Sprintf("Deterministic microchip match: %s (Score: 1.00)", maskedChip),
			DeterministicMatch: true,
			MatchType:          "deterministic_microchip",
			MatchedMicrochip:   maskedChip,
			Scores: &domain.MatchScoreBreakdown{
				Visual:         traitScore,
				Trait:          traitScore,
				Color:          colorScore,
				Spatial:        spatialScore,
				DistanceMiles:  distMiles,
				Threshold:      MatchThreshold,
				Vector:         vectorScore,
				MicrochipMatch: 1.0,
			},
			ThresholdVersion: HybridThresholdVersion,
		}
		if err := res.Validate(); err != nil {
			return nil
		}
		return res
	}

	// Hard species veto
	cleanLostSpecies := strings.TrimSpace(lostSpecies)
	cleanFoundSpecies := strings.TrimSpace(foundSpecies)
	if cleanLostSpecies != "" && cleanFoundSpecies != "" && !strings.EqualFold(cleanLostSpecies, cleanFoundSpecies) {
		res := &domain.MatchResult{
			FoundPetID:   foundPetID,
			MatchedPetID: lostPetID,
			Score:        0.0,
			IsMatch:      false,
			Details:      "Species mismatch veto (score forced to 0.0)",
			Scores: &domain.MatchScoreBreakdown{
				Visual:        traitScore,
				Trait:         traitScore,
				Color:         colorScore,
				Spatial:       spatialScore,
				DistanceMiles: distMiles,
				Threshold:     MatchThreshold,
				Vector:        0.0,
			},
			ThresholdVersion: HybridThresholdVersion,
		}
		if err := res.Validate(); err != nil {
			return nil
		}
		return res
	}

	var combinedScore float64
	if hasVector {
		combinedScore = CalculateHybridMatchScore(vectorScore, traitScore, spatialScore)
	} else {
		combinedScore = CalculateCombinedMatchScore(traitScore, spatialScore)
	}

	isMatch := combinedScore >= MatchThreshold
	details := fmt.Sprintf("Hybrid match score: %.2f (Vector: %.2f, Trait: %.2f, Spatial: %.2f, Distance: %.1f mi, Threshold: %.2f)",
		combinedScore, vectorScore, traitScore, spatialScore, distMiles, MatchThreshold)

	res := &domain.MatchResult{
		FoundPetID:   foundPetID,
		MatchedPetID: lostPetID,
		Score:        combinedScore,
		IsMatch:      isMatch,
		Details:      details,
		Scores: &domain.MatchScoreBreakdown{
			Visual:        traitScore,
			Trait:         traitScore,
			Color:         colorScore,
			Spatial:       spatialScore,
			DistanceMiles: distMiles,
			Threshold:     MatchThreshold,
			Vector:        vectorScore,
		},
		ThresholdVersion: HybridThresholdVersion,
	}

	if err := res.Validate(); err != nil {
		return nil
	}
	return res
}

// ComparePetsHybrid scores two reports using tri-factor weights and enforces the hard species veto.
func ComparePetsHybrid(
	lostPetID, foundPetID string,
	lostSpecies, foundSpecies string,
	distMiles float64,
	lostTraits, foundTraits *PetTraits,
	lostEmbedding, foundEmbedding []float32,
) *domain.MatchResult {
	return ComparePetsHybridWithMicrochip(
		lostPetID, foundPetID,
		lostSpecies, foundSpecies,
		"", "",
		distMiles,
		lostTraits, foundTraits,
		lostEmbedding, foundEmbedding,
	)
}

func calculateColorScore(first, second *PetTraits) float64 {
	if first == nil || second == nil {
		return 0
	}
	compared := 0
	matched := 0
	for _, colors := range [][2]string{
		{first.PrimaryColor, second.PrimaryColor},
		{first.SecondaryColor, second.SecondaryColor},
	} {
		if strings.TrimSpace(colors[0]) == "" || strings.TrimSpace(colors[1]) == "" {
			continue
		}
		compared++
		if compareStrings(colors[0], colors[1]) {
			matched++
		}
	}
	if compared == 0 {
		return 0
	}
	return float64(matched) / float64(compared)
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
