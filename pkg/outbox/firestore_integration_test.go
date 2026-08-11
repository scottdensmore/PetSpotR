package outbox_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type firestoreBlockingBroker struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newFirestoreBlockingBroker() *firestoreBlockingBroker {
	return &firestoreBlockingBroker{entered: make(chan struct{}), release: make(chan struct{})}
}

func (b *firestoreBlockingBroker) Publish(context.Context, string, []byte) error {
	b.mu.Lock()
	b.calls++
	call := b.calls
	b.mu.Unlock()
	if call == 1 {
		close(b.entered)
		<-b.release
	}
	return nil
}

func (b *firestoreBlockingBroker) Release() { b.once.Do(func() { close(b.release) }) }

func (b *firestoreBlockingBroker) Calls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func TestFirestoreConcurrentRelaysClaimOnePublication(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx := context.Background()
	firstStore, err := store.NewFirestoreEmulatorStore(ctx, "petspotr-outbox-relay-claim", host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore(first) error = %v", err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	secondStore, err := store.NewFirestoreEmulatorStore(ctx, "petspotr-outbox-relay-claim", host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore(second) error = %v", err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })

	now := time.Now().UTC()
	record := outbox.NewRecord(
		fmt.Sprintf("evt-firestore-relay-%d", now.UnixNano()),
		"foundPet",
		[]byte(`{"id":"evt-firestore-relay"}`),
		now,
	)
	if err := outbox.SaveRecord(ctx, firstStore, record); err != nil {
		t.Fatalf("SaveRecord() error = %v", err)
	}
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.OutboxCollection, record.ID)
	})

	broker := newFirestoreBlockingBroker()
	defer broker.Release()
	firstResult := make(chan error, 1)
	go func() {
		_, publishErr := outbox.NewRelay(firstStore, broker).PublishRecords(ctx, record.ID)
		firstResult <- publishErr
	}()
	<-broker.entered

	count, err := outbox.NewRelay(secondStore, broker).PublishRecords(ctx, record.ID)
	if err != nil {
		t.Fatalf("second PublishRecords() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("second PublishRecords() count = %d, want 0", count)
	}
	broker.Release()
	if err := <-firstResult; err != nil {
		t.Fatalf("first PublishRecords() error = %v", err)
	}
	if broker.Calls() != 1 {
		t.Fatalf("broker calls = %d, want 1", broker.Calls())
	}
	completed, err := outbox.GetRecord(ctx, secondStore, record.ID)
	if err != nil {
		t.Fatalf("GetRecord() error = %v", err)
	}
	if completed.Status != outbox.StatusPublished || completed.Attempts != 1 || completed.LeaseUntil != nil {
		t.Fatalf("completed record = %#v, want published attempt 1 without lease", completed)
	}
}
