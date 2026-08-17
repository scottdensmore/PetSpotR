package petmatcher

import (
	"context"
	"errors"
	"testing"
)

type candidateBackfillResult struct {
	migrated int
	complete bool
	err      error
}

type recordingCandidateBackfiller struct {
	results []candidateBackfillResult
	limits  []int
}

func (b *recordingCandidateBackfiller) BackfillLostPetCandidateIndexes(
	_ context.Context,
	limit int,
) (int, bool, error) {
	b.limits = append(b.limits, limit)
	result := b.results[0]
	b.results = b.results[1:]
	return result.migrated, result.complete, result.err
}

func TestPrepareLostPetCandidateIndexCompletesEveryBatch(t *testing.T) {
	backfiller := &recordingCandidateBackfiller{results: []candidateBackfillResult{
		{migrated: 400},
		{migrated: 2, complete: true},
	}}
	if err := PrepareLostPetCandidateIndex(context.Background(), backfiller, nil); err != nil {
		t.Fatalf("PrepareLostPetCandidateIndex() error = %v", err)
	}
	if len(backfiller.limits) != 2 {
		t.Fatalf("backfill calls = %d, want 2", len(backfiller.limits))
	}
	for _, limit := range backfiller.limits {
		if limit != 400 {
			t.Fatalf("backfill limit = %d, want 400", limit)
		}
	}
}

func TestPrepareLostPetCandidateIndexStopsOnFailure(t *testing.T) {
	wantErr := errors.New("firestore unavailable")
	backfiller := &recordingCandidateBackfiller{results: []candidateBackfillResult{{err: wantErr}}}
	if err := PrepareLostPetCandidateIndex(context.Background(), backfiller, nil); !errors.Is(err, wantErr) {
		t.Fatalf("PrepareLostPetCandidateIndex() error = %v, want %v", err, wantErr)
	}
}
