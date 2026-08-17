package petmatcher

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/scottdensmore/petspotr/pkg/store"
)

// PrepareLostPetCandidateIndex completes the bounded compatibility migration
// before the matcher accepts found-pet deliveries. This prevents legacy lost
// reports from disappearing during the indexed-query rollout.
func PrepareLostPetCandidateIndex(
	ctx context.Context,
	backfiller store.LostPetCandidateIndexBackfiller,
	logf func(string, ...any),
) error {
	if backfiller == nil {
		return errors.New("pet-matcher: lost-pet candidate index backfiller is required")
	}
	if logf == nil {
		logf = log.Printf
	}
	for {
		migrated, complete, err := backfiller.BackfillLostPetCandidateIndexes(
			ctx,
			store.MaxLostPetCandidateBackfillBatch,
		)
		if err != nil {
			return fmt.Errorf("pet-matcher: backfill lost-pet candidate index: %w", err)
		}
		if migrated > 0 {
			logf("Pet Matcher legacy candidate index backfill migrated %d records (complete=%t)", migrated, complete)
		}
		if complete {
			return nil
		}
	}
}
