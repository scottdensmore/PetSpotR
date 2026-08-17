package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const MaxLostPetCandidateBackfillBatch = 400

// LostPetCandidateQuery bounds matcher retrieval before exact distance and
// visual scoring. Species is normalized to lowercase; an empty stored species
// remains eligible for a known-species query.
type LostPetCandidateQuery struct {
	Status          string
	GeocodingStatus string
	Species         string
	ReportedAfter   time.Time
	ReportedBefore  time.Time
	MinLatitude     float64
	MaxLatitude     float64
	MinLongitude    float64
	MaxLongitude    float64
}

// LostPetCandidateStore exposes the production-bounded lookup used by the
// matcher instead of a full lost-pet collection scan.
type LostPetCandidateStore interface {
	QueryLostPetCandidates(ctx context.Context, query LostPetCandidateQuery) (map[string][]byte, error)
}

// LostPetCandidateIndexBackfiller upgrades opaque legacy lost-pet documents
// with the indexed metadata required by LostPetCandidateStore.
type LostPetCandidateIndexBackfiller interface {
	BackfillLostPetCandidateIndexes(ctx context.Context, limit int) (migrated int, complete bool, err error)
}

type lostPetCandidateState struct {
	PetID           string    `json:"petId"`
	Species         string    `json:"species"`
	ReportedAt      time.Time `json:"reportedAt"`
	GeocodingStatus string    `json:"geocodingStatus"`
	Status          string    `json:"status"`
	Coordinates     *struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"coordinates"`
}

func (q LostPetCandidateQuery) validate() error {
	if strings.TrimSpace(q.Status) == "" || strings.TrimSpace(q.GeocodingStatus) == "" ||
		q.ReportedAfter.IsZero() || q.ReportedBefore.IsZero() || q.ReportedAfter.After(q.ReportedBefore) ||
		q.MinLatitude < -90 || q.MaxLatitude > 90 || q.MinLatitude > q.MaxLatitude ||
		q.MinLongitude < -180 || q.MaxLongitude > 180 || q.MinLongitude > q.MaxLongitude {
		return errors.New("store: invalid lost-pet candidate query")
	}
	return nil
}

func decodeLostPetCandidateState(key string, data []byte) (lostPetCandidateState, bool) {
	var state lostPetCandidateState
	if err := json.Unmarshal(data, &state); err != nil || state.PetID != key || state.Coordinates == nil ||
		state.ReportedAt.IsZero() || state.Coordinates.Latitude < -90 || state.Coordinates.Latitude > 90 ||
		state.Coordinates.Longitude < -180 || state.Coordinates.Longitude > 180 {
		return lostPetCandidateState{}, false
	}
	state.Status = strings.TrimSpace(state.Status)
	state.GeocodingStatus = strings.TrimSpace(state.GeocodingStatus)
	state.Species = strings.ToLower(strings.TrimSpace(state.Species))
	return state, true
}

// QueryLostPetCandidates applies the same bounded contract in local/test
// memory runtimes. Managed Firestore performs these filters in its index.
func (m *MemoryStore) QueryLostPetCandidates(
	ctx context.Context,
	query LostPetCandidateQuery,
) (map[string][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := query.validate(); err != nil {
		return nil, err
	}
	query.Species = strings.ToLower(strings.TrimSpace(query.Species))

	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string][]byte)
	for key, data := range m.items[LostPetsCollection] {
		state, ok := decodeLostPetCandidateState(key, data)
		if !ok || state.Status != query.Status || state.GeocodingStatus != query.GeocodingStatus ||
			state.ReportedAt.Before(query.ReportedAfter) || state.ReportedAt.After(query.ReportedBefore) ||
			(query.Species != "" && state.Species != "" && state.Species != query.Species) ||
			state.Coordinates.Latitude < query.MinLatitude || state.Coordinates.Latitude > query.MaxLatitude ||
			state.Coordinates.Longitude < query.MinLongitude || state.Coordinates.Longitude > query.MaxLongitude {
			continue
		}
		result[key] = bytes.Clone(data)
	}
	return result, nil
}

// BackfillLostPetCandidateIndexes is a no-op for memory state because queries
// decode the in-memory payloads directly.
func (m *MemoryStore) BackfillLostPetCandidateIndexes(ctx context.Context, limit int) (int, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	if limit < 1 || limit > MaxLostPetCandidateBackfillBatch {
		return 0, false, errors.New("store: lost-pet candidate index backfill limit must be between 1 and 400")
	}
	return 0, true, nil
}
