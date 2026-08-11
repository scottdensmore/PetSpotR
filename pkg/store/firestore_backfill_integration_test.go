package store

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type backfillRaceRecord struct {
	ID        string    `json:"id"`
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"payload"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

func TestFirestoreBackfillRejectsStaleSnapshotAfterConcurrentPublish(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	stateStore, err := NewFirestoreEmulatorStore(ctx, "petspotr-outbox-backfill-race", host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore() error = %v", err)
	}
	t.Cleanup(func() { _ = stateStore.Close() })

	record := backfillRaceRecord{
		ID:        "evt-stale-backfill",
		Topic:     "foundPet",
		Payload:   []byte(`{"id":"stale-backfill"}`),
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}
	recordData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	doc := stateStore.client.Collection(OutboxCollection).Doc(firestoreDocumentID(record.ID))
	if _, err := doc.Set(ctx, map[string]any{"key": record.ID, "data": recordData}); err != nil {
		t.Fatalf("write legacy outbox: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = doc.Delete(cleanupCtx)
	})

	staleSnapshot, err := doc.Get(ctx)
	if err != nil {
		t.Fatalf("read stale snapshot: %v", err)
	}
	var staleRecord firestoreRecord
	if err := staleSnapshot.DataTo(&staleRecord); err != nil {
		t.Fatalf("decode stale snapshot: %v", err)
	}
	staleIndex, err := newFirestoreRecord(OutboxCollection, staleRecord.Key, staleRecord.Data)
	if err != nil {
		t.Fatalf("build stale index: %v", err)
	}

	record.Status = "published"
	publishedData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, OutboxCollection, record.ID, publishedData); err != nil {
		t.Fatalf("persist concurrent published status: %v", err)
	}

	bulkWriter := stateStore.client.BulkWriter(ctx)
	job, err := stateStore.queueOutboxIndexBackfill(bulkWriter, staleSnapshot, staleIndex)
	if err != nil {
		bulkWriter.End()
		t.Fatalf("queue stale backfill: %v", err)
	}
	bulkWriter.End()
	if _, err := job.Results(); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("stale backfill error = %v, want FailedPrecondition", err)
	}

	after, err := doc.Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var stored firestoreRecord
	if err := after.DataTo(&stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status != "published" {
		t.Fatalf("top-level status = %q, want published", stored.Status)
	}
	var embedded backfillRaceRecord
	if err := json.Unmarshal(stored.Data, &embedded); err != nil {
		t.Fatal(err)
	}
	if embedded.Status != "published" {
		t.Fatalf("embedded status = %q, want published", embedded.Status)
	}
	pending, err := stateStore.ListPendingOutbox(ctx, "foundPet", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending IDs after concurrent publish = %#v, want empty", pending)
	}
}
