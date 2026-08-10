package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreCreateStateAndOutboxTransaction(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stateStore, err := store.NewFirestoreEmulatorStore(ctx, "petspotr-outbox-contract", host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore() error = %v", err)
	}
	t.Cleanup(func() { _ = stateStore.Close() })

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	state := store.StateWrite{
		StoreName: store.LostPetsCollection,
		Key:       "lost-atomic-" + suffix,
		Data:      []byte(`{"petId":"lost-atomic"}`),
	}
	outbox := store.StateWrite{
		StoreName: store.OutboxCollection,
		Key:       "evt-atomic-" + suffix,
		Data:      []byte(`{"id":"evt-atomic","status":"pending"}`),
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = stateStore.DeleteState(cleanupCtx, state.StoreName, state.Key)
		_ = stateStore.DeleteState(cleanupCtx, outbox.StoreName, outbox.Key)
	})

	created, err := stateStore.CreateStateAndOutbox(ctx, state, outbox)
	if err != nil || !created {
		t.Fatalf("CreateStateAndOutbox() = %t, %v; want true, nil", created, err)
	}

	// A retry may carry different request metadata in the envelope, but the
	// stable event ID must preserve the first outbox record rather than reset it.
	retryOutbox := outbox
	retryOutbox.Data = []byte(`{"id":"evt-atomic","status":"pending","correlationId":"retry"}`)
	created, err = stateStore.CreateStateAndOutbox(ctx, state, retryOutbox)
	if err != nil || created {
		t.Fatalf("retry CreateStateAndOutbox() = %t, %v; want false, nil", created, err)
	}
	storedOutbox, err := stateStore.GetState(ctx, outbox.StoreName, outbox.Key)
	if err != nil {
		t.Fatal(err)
	}
	if string(storedOutbox) != string(outbox.Data) {
		t.Fatalf("retry replaced outbox = %s, want %s", storedOutbox, outbox.Data)
	}

	competing := outbox
	competing.Key = "evt-competing-" + suffix
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = stateStore.DeleteState(cleanupCtx, competing.StoreName, competing.Key)
	})
	created, err = stateStore.CreateStateAndOutbox(ctx, state, competing)
	if !errors.Is(err, store.ErrConflict) || created {
		t.Fatalf("competing CreateStateAndOutbox() = %t, %v; want false, ErrConflict", created, err)
	}
	if _, err := stateStore.GetState(ctx, competing.StoreName, competing.Key); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("competing outbox error = %v, want ErrNotFound", err)
	}
}

func TestFirestoreCreateStateAndOutboxConcurrentCreators(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	stateStore, err := store.NewFirestoreEmulatorStore(ctx, "petspotr-outbox-contention", host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore() error = %v", err)
	}
	t.Cleanup(func() { _ = stateStore.Close() })

	t.Run("identical creators report exactly one create", func(t *testing.T) {
		for round := 0; round < 8; round++ {
			suffix := fmt.Sprintf("%d-%d", time.Now().UnixNano(), round)
			state := store.StateWrite{StoreName: store.LostPetsCollection, Key: "lost-identical-" + suffix, Data: []byte(`{"petId":"lost-identical"}`)}
			outbox := store.StateWrite{StoreName: store.OutboxCollection, Key: "evt-identical-" + suffix, Data: []byte(`{"id":"evt-identical"}`)}
			registerAtomicCleanup(t, stateStore, state, outbox)

			results := runConcurrentCreates(ctx, stateStore,
				createCall{state: state, outbox: outbox},
				createCall{state: state, outbox: outbox},
			)
			createdCount := 0
			for _, result := range results {
				if result.err != nil {
					t.Fatalf("round %d concurrent identical error = %v", round, result.err)
				}
				if result.created {
					createdCount++
				}
			}
			if createdCount != 1 {
				t.Fatalf("round %d created count = %d, want 1", round, createdCount)
			}
		}
	})

	t.Run("competing creators leave no orphan outbox", func(t *testing.T) {
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		firstState := store.StateWrite{StoreName: store.LostPetsCollection, Key: "lost-competing-" + suffix, Data: []byte(`{"petId":"lost-competing","location":"Seattle"}`)}
		secondState := firstState
		secondState.Data = []byte(`{"petId":"lost-competing","location":"Portland"}`)
		firstOutbox := store.StateWrite{StoreName: store.OutboxCollection, Key: "evt-first-" + suffix, Data: []byte(`{"id":"evt-first"}`)}
		secondOutbox := store.StateWrite{StoreName: store.OutboxCollection, Key: "evt-second-" + suffix, Data: []byte(`{"id":"evt-second"}`)}
		registerAtomicCleanup(t, stateStore, firstState, firstOutbox, secondOutbox)

		results := runConcurrentCreates(ctx, stateStore,
			createCall{state: firstState, outbox: firstOutbox},
			createCall{state: secondState, outbox: secondOutbox},
		)
		createdCount := 0
		conflictCount := 0
		for _, result := range results {
			if result.created {
				createdCount++
			}
			if errors.Is(result.err, store.ErrConflict) {
				conflictCount++
			} else if result.err != nil {
				t.Fatalf("unexpected competing error = %v", result.err)
			}
		}
		if createdCount != 1 || conflictCount != 1 {
			t.Fatalf("competing results: created = %d, conflicts = %d; want 1, 1", createdCount, conflictCount)
		}
		outboxCount := 0
		for _, candidate := range []store.StateWrite{firstOutbox, secondOutbox} {
			if _, err := stateStore.GetState(ctx, candidate.StoreName, candidate.Key); err == nil {
				outboxCount++
			} else if !errors.Is(err, store.ErrNotFound) {
				t.Fatal(err)
			}
		}
		if outboxCount != 1 {
			t.Fatalf("persisted competing outboxes = %d, want 1", outboxCount)
		}
	})
}

type createCall struct {
	state  store.StateWrite
	outbox store.StateWrite
}

type createResult struct {
	created bool
	err     error
}

func runConcurrentCreates(ctx context.Context, stateStore store.StateStore, calls ...createCall) []createResult {
	start := make(chan struct{})
	results := make([]createResult, len(calls))
	var wait sync.WaitGroup
	for index, call := range calls {
		index, call := index, call
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			results[index].created, results[index].err = stateStore.CreateStateAndOutbox(ctx, call.state, call.outbox)
		}()
	}
	close(start)
	wait.Wait()
	return results
}

func registerAtomicCleanup(t *testing.T, stateStore store.StateStore, state store.StateWrite, outboxes ...store.StateWrite) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = stateStore.DeleteState(ctx, state.StoreName, state.Key)
		for _, outbox := range outboxes {
			_ = stateStore.DeleteState(ctx, outbox.StoreName, outbox.Key)
		}
	})
}
