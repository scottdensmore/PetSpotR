package petmatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/scoring"
	"github.com/scottdensmore/petspotr/pkg/store"
)

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
		traits := &scoring.PetTraits{
			Breed:        record.Breed,
			PrimaryColor: record.PrimaryColor,
		}
		if description := strings.TrimSpace(record.Description); description != "" {
			traits.DistinctiveMarkings = []string{description}
		}
		candidates = append(candidates, lostPetCandidate{
			record: record, distanceMiles: distanceMiles, traits: traits,
		})
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
