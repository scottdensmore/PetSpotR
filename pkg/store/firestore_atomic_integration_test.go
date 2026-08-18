package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreBackfillsLegacyOutboxIndexes(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const projectID = "petspotr-outbox-backfill"
	stateStore, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore() error = %v", err)
	}
	t.Cleanup(func() { _ = stateStore.Close() })
	rawClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		t.Fatalf("firestore.NewClient() error = %v", err)
	}
	t.Cleanup(func() { _ = rawClient.Close() })

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	legacy := outbox.NewRecord("evt-legacy-index-"+suffix, "foundPet", []byte(`{"id":"legacy"}`), time.Now().UTC())
	legacyData, err := outbox.MarshalRecord(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyDoc := rawClient.Collection(store.OutboxCollection).Doc(firestoreDocumentIDForTest(legacy.ID))
	lateLegacy := outbox.NewRecord("aaa-late-legacy-index-"+suffix, "foundPet", []byte(`{"id":"late-legacy"}`), time.Now().UTC())
	lateLegacyData, err := outbox.MarshalRecord(lateLegacy)
	if err != nil {
		t.Fatal(err)
	}
	lateLegacyDoc := rawClient.Collection(store.OutboxCollection).Doc(firestoreDocumentIDForTest(lateLegacy.ID))
	migrationDoc := rawClient.Collection("runtimeMigrations").Doc(firestoreDocumentIDForTest("outbox-index-v1"))
	if _, err := legacyDoc.Set(ctx, map[string]any{"key": legacy.ID, "data": legacyData}); err != nil {
		t.Fatalf("write legacy outbox: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = legacyDoc.Delete(cleanupCtx)
		_, _ = lateLegacyDoc.Delete(cleanupCtx)
		_, _ = migrationDoc.Delete(cleanupCtx)
	})

	before, err := stateStore.ListPendingOutbox(ctx, "foundPet", 10)
	if err != nil {
		t.Fatalf("ListPendingOutbox() before backfill error = %v", err)
	}
	if len(before) != 0 {
		t.Fatalf("ListPendingOutbox() before backfill = %#v, want empty", before)
	}
	migrated, _, err := stateStore.BackfillOutboxIndexes(ctx, 100)
	if err != nil {
		t.Fatalf("BackfillOutboxIndexes() error = %v", err)
	}
	if migrated != 1 {
		t.Fatalf("BackfillOutboxIndexes() migrated = %d, want 1", migrated)
	}
	after, err := stateStore.ListPendingOutbox(ctx, "foundPet", 10)
	if err != nil {
		t.Fatalf("ListPendingOutbox() after backfill error = %v", err)
	}
	if len(after) != 1 || after[0] != legacy.ID {
		t.Fatalf("ListPendingOutbox() after backfill = %#v, want [%s]", after, legacy.ID)
	}

	// Simulate an old Cloud Run revision completing a request after the new
	// revision finished migration. Its key sorts behind the durable cursor.
	if _, err := lateLegacyDoc.Set(ctx, map[string]any{"key": lateLegacy.ID, "data": lateLegacyData}); err != nil {
		t.Fatalf("write late legacy outbox: %v", err)
	}
	migrated, _, err = stateStore.BackfillOutboxIndexes(ctx, 100)
	if err != nil {
		t.Fatalf("BackfillOutboxIndexes() late record error = %v", err)
	}
	if migrated != 1 {
		t.Fatalf("BackfillOutboxIndexes() late migrated = %d, want 1", migrated)
	}
	afterLateWrite, err := stateStore.ListPendingOutbox(ctx, "foundPet", 10)
	if err != nil {
		t.Fatalf("ListPendingOutbox() after late write error = %v", err)
	}
	gotIDs := make(map[string]bool, len(afterLateWrite))
	for _, id := range afterLateWrite {
		gotIDs[id] = true
	}
	if len(afterLateWrite) != 2 || !gotIDs[lateLegacy.ID] || !gotIDs[legacy.ID] {
		t.Fatalf("ListPendingOutbox() after late write = %#v, want both %s and %s", afterLateWrite, lateLegacy.ID, legacy.ID)
	}
}

func firestoreDocumentIDForTest(key string) string {
	digest := sha256.Sum256([]byte(key))
	return hex.EncodeToString(digest[:])
}

func TestFirestoreUpdatesMatchAndParticipantsAtomicallyAcrossRuntimes(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const projectID = "petspotr-match-decision-atomic"
	writer, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	reader, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	matchID := fmt.Sprintf("match-decision-%d", time.Now().UnixNano())
	if err := writer.SaveState(ctx, store.MatchesCollection, matchID, []byte("match-before")); err != nil {
		t.Fatal(err)
	}
	if err := writer.SaveState(ctx, store.MatchParticipantsCollection, matchID, []byte("participants-before")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = writer.DeleteState(cleanupCtx, store.MatchesCollection, matchID)
		_ = writer.DeleteState(cleanupCtx, store.MatchParticipantsCollection, matchID)
	})

	if err := writer.UpdateMatchAndParticipants(ctx, matchID, func(match, participants []byte) ([]byte, []byte, error) {
		if string(match) != "match-before" || string(participants) != "participants-before" {
			t.Fatalf("transaction inputs = %q/%q", match, participants)
		}
		return []byte("match-after"), []byte("participants-after"), nil
	}); err != nil {
		t.Fatalf("UpdateMatchAndParticipants() error = %v", err)
	}
	assertStoredValue(t, reader, store.MatchesCollection, matchID, "match-after")
	assertStoredValue(t, reader, store.MatchParticipantsCollection, matchID, "participants-after")

	rejected := errors.New("decision rejected")
	if err := reader.UpdateMatchAndParticipants(ctx, matchID, func([]byte, []byte) ([]byte, []byte, error) {
		return []byte("discarded-match"), []byte("discarded-participants"), rejected
	}); !errors.Is(err, rejected) {
		t.Fatalf("rejected update error = %v, want %v", err, rejected)
	}
	assertStoredValue(t, writer, store.MatchesCollection, matchID, "match-after")
	assertStoredValue(t, writer, store.MatchParticipantsCollection, matchID, "participants-after")
}

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
	contact := store.StateWrite{
		StoreName: store.ReportContactsCollection,
		Key:       "lost/lost-atomic/owner-" + suffix,
		Data:      []byte(`{"email":"owner@example.com"}`),
	}
	outboxRecord := outbox.NewRecord("evt-atomic-"+suffix, "lostPet", []byte(`{"id":"evt-atomic"}`), time.Now().UTC())
	outboxData, err := outbox.MarshalRecord(outboxRecord)
	if err != nil {
		t.Fatal(err)
	}
	outboxWrite := store.StateWrite{
		StoreName: store.OutboxCollection,
		Key:       outboxRecord.ID,
		Data:      outboxData,
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = stateStore.DeleteState(cleanupCtx, state.StoreName, state.Key)
		_ = stateStore.DeleteState(cleanupCtx, contact.StoreName, contact.Key)
		_ = stateStore.DeleteState(cleanupCtx, outboxWrite.StoreName, outboxWrite.Key)
	})

	created, err := stateStore.CreateStatesAndOutbox(ctx, []store.StateWrite{state, contact}, outboxWrite)
	if err != nil || !created {
		t.Fatalf("CreateStatesAndOutbox() = %t, %v; want true, nil", created, err)
	}

	// A retry may carry different request metadata in the envelope, but the
	// stable event ID must preserve the first outbox record rather than reset it.
	retryRecord := outboxRecord
	retryRecord.LastError = "retry metadata"
	retryData, err := outbox.MarshalRecord(retryRecord)
	if err != nil {
		t.Fatal(err)
	}
	retryOutbox := outboxWrite
	retryOutbox.Data = retryData
	created, err = stateStore.CreateStatesAndOutbox(ctx, []store.StateWrite{state, contact}, retryOutbox)
	if err != nil || created {
		t.Fatalf("retry CreateStatesAndOutbox() = %t, %v; want false, nil", created, err)
	}
	storedOutbox, err := stateStore.GetState(ctx, outboxWrite.StoreName, outboxWrite.Key)
	if err != nil {
		t.Fatal(err)
	}
	if string(storedOutbox) != string(outboxWrite.Data) {
		t.Fatalf("retry replaced outbox = %s, want %s", storedOutbox, outboxWrite.Data)
	}
	pendingIDs, err := stateStore.ListPendingOutbox(ctx, "lostPet", 10)
	if err != nil {
		t.Fatalf("ListPendingOutbox() error = %v", err)
	}
	if len(pendingIDs) != 1 || pendingIDs[0] != outboxRecord.ID {
		t.Fatalf("ListPendingOutbox() = %#v, want [%s]", pendingIDs, outboxRecord.ID)
	}

	competing := outboxWrite
	competing.Key = "evt-competing-" + suffix
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_ = stateStore.DeleteState(cleanupCtx, competing.StoreName, competing.Key)
	})
	changedContact := contact
	changedContact.Data = []byte(`{"email":"other@example.com"}`)
	created, err = stateStore.CreateStatesAndOutbox(ctx, []store.StateWrite{state, changedContact}, competing)
	if !errors.Is(err, store.ErrConflict) || created {
		t.Fatalf("competing CreateStatesAndOutbox() = %t, %v; want false, ErrConflict", created, err)
	}
	if _, err := stateStore.GetState(ctx, competing.StoreName, competing.Key); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("competing outbox error = %v, want ErrNotFound", err)
	}
	storedContact, err := stateStore.GetState(ctx, contact.StoreName, contact.Key)
	if err != nil {
		t.Fatal(err)
	}
	if string(storedContact) != string(contact.Data) {
		t.Fatalf("competing create replaced contact = %s, want %s", storedContact, contact.Data)
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
			outboxWrite := newAtomicOutboxWrite(t, "evt-identical-"+suffix)
			registerAtomicCleanup(t, stateStore, state, outboxWrite)

			results := runConcurrentCreates(ctx, stateStore,
				createCall{state: state, outbox: outboxWrite},
				createCall{state: state, outbox: outboxWrite},
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
		firstOutbox := newAtomicOutboxWrite(t, "evt-first-"+suffix)
		secondOutbox := newAtomicOutboxWrite(t, "evt-second-"+suffix)
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

func newAtomicOutboxWrite(t *testing.T, id string) store.StateWrite {
	t.Helper()
	record := outbox.NewRecord(id, "lostPet", []byte(`{"id":"atomic-event"}`), time.Now().UTC())
	data, err := outbox.MarshalRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	return store.StateWrite{StoreName: store.OutboxCollection, Key: id, Data: data}
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
