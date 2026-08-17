package petmatcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var errLostImageTraitsPending = errors.New("pet-matcher: eligible lost-pet image traits are pending")

const (
	matcherCandidateWindow      = 30 * 24 * time.Hour
	matcherCandidateRadiusMiles = scoring.MatchRadiusMiles
)

type lostPetCandidate struct {
	record        domain.LostPetRecord
	distanceMiles float64
	traits        *scoring.PetTraits
}

type rankedCandidate struct {
	candidate lostPetCandidate
	result    *domain.MatchResult
}

func outranks(challenger, current rankedCandidate) bool {
	if challenger.result.Score != current.result.Score {
		return challenger.result.Score > current.result.Score
	}
	if challenger.candidate.distanceMiles != current.candidate.distanceMiles {
		return challenger.candidate.distanceMiles < current.candidate.distanceMiles
	}
	return challenger.candidate.record.PetID < current.candidate.record.PetID
}

func (w *Worker) eligibleLostPetCandidates(
	ctx context.Context,
	found domain.FoundPetReportedV2,
) ([]lostPetCandidate, error) {
	if found.GeocodingStatus != domain.GeocodingVerified || found.Coordinates == nil {
		return nil, nil
	}
	if err := found.Coordinates.Validate(); err != nil {
		return nil, fmt.Errorf("pet-matcher: validate found-pet coordinates: %w", err)
	}
	rawCandidates := make(map[string][]byte)
	for _, query := range boundedCandidateQueries(found) {
		queryResults, err := w.store.QueryLostPetCandidates(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("pet-matcher: query lost-pet candidates: %w", err)
		}
		for key, data := range queryResults {
			rawCandidates[key] = data
		}
	}

	candidates := make([]lostPetCandidate, 0, len(rawCandidates))
	pendingImageTraits := false
	for key, data := range rawCandidates {
		var record domain.LostPetRecord
		if err := json.Unmarshal(data, &record); err != nil {
			log.Printf("[Pet Matcher] Skipping malformed lost-pet candidate %q", key)
			continue
		}
		record = domain.NormalizeLostPetRecord(record)
		if record.PetID == "" || record.PetID != key {
			log.Printf("[Pet Matcher] Skipping lost-pet candidate %q with mismatched identity", key)
			continue
		}
		if record.Status != domain.LostPetStatusLost || record.GeocodingStatus != domain.GeocodingVerified ||
			record.Coordinates == nil || record.ReportedAt.IsZero() {
			continue
		}
		if err := record.Coordinates.Validate(); err != nil {
			log.Printf("[Pet Matcher] Skipping lost-pet candidate %q with invalid coordinates", key)
			continue
		}
		age := found.FoundAt.Sub(record.ReportedAt)
		if age < 0 {
			age = -age
		}
		if age > matcherCandidateWindow {
			continue
		}
		if found.Species != "" && record.Species != "" && !strings.EqualFold(found.Species, record.Species) {
			continue
		}
		distanceMiles := domain.HaversineDistanceMiles(*found.Coordinates, *record.Coordinates)
		if distanceMiles > matcherCandidateRadiusMiles {
			continue
		}
		if record.ImageObject == "" {
			continue
		}
		if !blob.IsFinalizedImageForPurpose(blob.ImagePurposeLostPet, record.PetID, record.ImageObject) {
			log.Printf("[Pet Matcher] Skipping lost-pet candidate %q with invalid image namespace", key)
			continue
		}
		analysis := record.ImageAnalysis
		if analysis == nil {
			pendingImageTraits = true
			continue
		}
		if analysis.SourceImageObject != record.ImageObject || analysis.Status != domain.ImageTraitsVerified {
			log.Printf("[Pet Matcher] Skipping lost-pet candidate %q with invalid image provenance", key)
			continue
		}
		if err := analysis.Validate(); err != nil {
			log.Printf("[Pet Matcher] Skipping lost-pet candidate %q with invalid image analysis", key)
			continue
		}
		traits := &scoring.PetTraits{
			Breed:               analysis.Traits.Breed,
			PrimaryColor:        analysis.Traits.PrimaryColor,
			SecondaryColor:      analysis.Traits.SecondaryColor,
			DistinctiveMarkings: append([]string(nil), analysis.Traits.DistinctiveMarkings...),
			EyeColor:            analysis.Traits.EyeColor,
		}
		candidates = append(candidates, lostPetCandidate{
			record: record, distanceMiles: distanceMiles, traits: traits,
		})
	}
	if pendingImageTraits {
		return nil, errLostImageTraitsPending
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].record.PetID < candidates[j].record.PetID
	})
	return candidates, nil
}

func boundedCandidateQueries(found domain.FoundPetReportedV2) []store.LostPetCandidateQuery {
	latitudeDelta := matcherCandidateRadiusMiles / 69.0
	longitudeMilesPerDegree := 69.0 * math.Cos(found.Coordinates.Latitude*math.Pi/180)
	longitudeDelta := 180.0
	if math.Abs(longitudeMilesPerDegree) > 0.001 {
		longitudeDelta = math.Min(180, matcherCandidateRadiusMiles/math.Abs(longitudeMilesPerDegree))
	}
	base := store.LostPetCandidateQuery{
		Status:          string(domain.LostPetStatusLost),
		GeocodingStatus: string(domain.GeocodingVerified),
		Species:         found.Species,
		ReportedAfter:   found.FoundAt.Add(-matcherCandidateWindow),
		ReportedBefore:  found.FoundAt.Add(matcherCandidateWindow),
		MinLatitude:     math.Max(-90, found.Coordinates.Latitude-latitudeDelta),
		MaxLatitude:     math.Min(90, found.Coordinates.Latitude+latitudeDelta),
	}
	minimumLongitude := found.Coordinates.Longitude - longitudeDelta
	maximumLongitude := found.Coordinates.Longitude + longitudeDelta
	switch {
	case minimumLongitude < -180:
		west := base
		west.MinLongitude = -180
		west.MaxLongitude = maximumLongitude
		east := base
		east.MinLongitude = minimumLongitude + 360
		east.MaxLongitude = 180
		return []store.LostPetCandidateQuery{west, east}
	case maximumLongitude > 180:
		west := base
		west.MinLongitude = minimumLongitude
		west.MaxLongitude = 180
		east := base
		east.MinLongitude = -180
		east.MaxLongitude = maximumLongitude - 360
		return []store.LostPetCandidateQuery{west, east}
	default:
		base.MinLongitude = minimumLongitude
		base.MaxLongitude = maximumLongitude
		return []store.LostPetCandidateQuery{base}
	}
}
