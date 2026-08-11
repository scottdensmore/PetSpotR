package outbox_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type firstPublishBlockingBroker struct {
	mu      sync.Mutex
	calls   int
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newFirstPublishBlockingBroker() *firstPublishBlockingBroker {
	return &firstPublishBlockingBroker{entered: make(chan struct{}), release: make(chan struct{})}
}

func (b *firstPublishBlockingBroker) Publish(context.Context, string, []byte) error {
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

func (b *firstPublishBlockingBroker) Calls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.calls
}

func (b *firstPublishBlockingBroker) Release() { b.once.Do(func() { close(b.release) }) }

type failingBroker struct {
	err error
}

type completionFailingStore struct {
	*store.MemoryStore
	failCompletion bool
}

func (s *completionFailingStore) SaveState(ctx context.Context, storeName, key string, data []byte) error {
	if s.failCompletion && storeName == store.OutboxCollection {
		return errors.New("completion write interrupted")
	}
	return s.MemoryStore.SaveState(ctx, storeName, key, data)
}

func (s *completionFailingStore) UpdateState(
	ctx context.Context,
	storeName string,
	key string,
	update store.StateUpdater,
) error {
	if !s.failCompletion || storeName != store.OutboxCollection {
		return s.MemoryStore.UpdateState(ctx, storeName, key, update)
	}
	current, err := s.GetState(ctx, storeName, key)
	if err != nil {
		return err
	}
	next, err := update(current)
	if err != nil {
		return err
	}
	var record outbox.Record
	if err := json.Unmarshal(next, &record); err != nil {
		return err
	}
	if record.Status == outbox.StatusPublished {
		return errors.New("completion write interrupted")
	}
	return s.MemoryStore.SaveState(ctx, storeName, key, next)
}

func (b *failingBroker) Publish(context.Context, string, []byte) error { return b.err }
func (b *failingBroker) Subscribe(string, pubsub.Handler) error        { return nil }

func TestRelayRetainsPendingRecordAcrossPublishFailureAndRecovery(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	record := outbox.Record{
		ID:        "evt-101",
		Topic:     "lostPet",
		Payload:   []byte(`{"envelopeVersion":1,"id":"evt-101"}`),
		Status:    outbox.StatusPending,
		CreatedAt: time.Date(2026, time.August, 10, 18, 30, 0, 0, time.UTC),
	}
	if err := outbox.SaveRecord(ctx, stateStore, record); err != nil {
		t.Fatal(err)
	}

	failedRelay := outbox.NewRelay(stateStore, &failingBroker{err: errors.New("broker unavailable")})
	if _, err := failedRelay.PublishRecords(ctx, record.ID); err == nil {
		t.Fatal("PublishRecords() error = nil, want publish failure")
	}
	pending, err := outbox.GetRecord(ctx, stateStore, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != outbox.StatusPending || pending.Attempts != 1 {
		t.Fatalf("record after failure = %#v", pending)
	}

	broker := pubsub.NewMemoryPubSub()
	var published int
	if err := broker.Subscribe("lostPet", func(context.Context, []byte) error {
		published++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	restartedRelay := outbox.NewRelay(stateStore, broker)
	count, err := restartedRelay.PublishRecords(ctx, record.ID)
	if err != nil {
		t.Fatalf("restarted PublishRecords() error = %v", err)
	}
	if count != 1 || published != 1 {
		t.Fatalf("restarted PublishPending() count = %d, published = %d", count, published)
	}
	completed, err := outbox.GetRecord(ctx, stateStore, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != outbox.StatusPublished || completed.PublishedAt == nil {
		t.Fatalf("completed record = %#v", completed)
	}
}

func TestRelayRecoversWhenProcessCrashesAfterDispatchBeforeCompletion(t *testing.T) {
	ctx := context.Background()
	stateStore := &completionFailingStore{MemoryStore: store.NewMemoryStore()}
	now := time.Date(2026, time.August, 10, 18, 30, 0, 0, time.UTC)
	record := outbox.NewRecord("evt-crash", "lostPet", []byte(`{"id":"evt-crash"}`), now)
	if err := outbox.SaveRecord(ctx, stateStore, record); err != nil {
		t.Fatal(err)
	}
	broker := pubsub.NewMemoryPubSub()
	dispatches := 0
	if err := broker.Subscribe("lostPet", func(context.Context, []byte) error {
		dispatches++
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	newRelay := func() *outbox.Relay {
		return outbox.NewRelay(
			stateStore,
			broker,
			outbox.WithClock(func() time.Time { return now }),
			outbox.WithPublishLease(time.Second),
		)
	}
	stateStore.failCompletion = true
	if _, err := newRelay().PublishRecords(ctx, record.ID); err == nil {
		t.Fatal("PublishRecords() error = nil, want completion write failure")
	}
	if dispatches != 1 {
		t.Fatalf("dispatches before restart = %d, want 1", dispatches)
	}
	pending, err := outbox.GetRecord(ctx, stateStore, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Status != outbox.StatusPending || pending.LeaseUntil == nil {
		t.Fatalf("record after interrupted completion = %#v", pending)
	}

	stateStore.failCompletion = false
	now = now.Add(2 * time.Second)
	if _, err := newRelay().PublishRecords(ctx, record.ID); err != nil {
		t.Fatalf("PublishRecords() after restart error = %v", err)
	}
	if dispatches != 2 {
		t.Fatalf("at-least-once dispatches after restart = %d, want 2", dispatches)
	}
}

func TestRelayIsolatesPoisonRecordsAndPublishesValidIDs(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	if err := stateStore.SaveState(ctx, store.OutboxCollection, "invalid-json", []byte(`{invalid`)); err != nil {
		t.Fatal(err)
	}
	mismatched := outbox.NewRecord("copied-id", "lostPet", []byte(`{"id":"copied-id"}`), time.Now().UTC())
	mismatchedData, err := outbox.MarshalRecord(mismatched)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.OutboxCollection, "mismatched-key", mismatchedData); err != nil {
		t.Fatal(err)
	}
	valid := outbox.NewRecord("valid-id", "lostPet", []byte(`{"id":"valid-id"}`), time.Now().UTC())
	if err := outbox.SaveRecord(ctx, stateStore, valid); err != nil {
		t.Fatal(err)
	}

	broker := pubsub.NewMemoryPubSub()
	published := 0
	if err := broker.Subscribe("lostPet", func(context.Context, []byte) error {
		published++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	count, err := outbox.NewRelay(stateStore, broker).PublishRecords(
		ctx,
		"invalid-json",
		"mismatched-key",
		valid.ID,
	)
	if err == nil {
		t.Fatal("PublishRecords() error = nil, want poison-record errors")
	}
	if count != 1 || published != 1 {
		t.Fatalf("PublishRecords() count = %d, published = %d; want 1, 1", count, published)
	}
	if _, err := stateStore.GetState(ctx, store.OutboxCollection, "copied-id"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("mismatched record was copied under embedded ID: %v", err)
	}
	completed, err := outbox.GetRecord(ctx, stateStore, valid.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != outbox.StatusPublished {
		t.Fatalf("valid record status = %q, want published", completed.Status)
	}
}

func TestRelayPublishesBoundedPendingTopicBatch(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	for _, record := range []outbox.Record{
		outbox.NewRecord("found-pending", "foundPet", []byte(`{"id":"found-pending"}`), time.Now().UTC()),
		outbox.NewRecord("lost-pending", "lostPet", []byte(`{"id":"lost-pending"}`), time.Now().UTC()),
	} {
		if err := outbox.SaveRecord(ctx, stateStore, record); err != nil {
			t.Fatal(err)
		}
	}
	broker := pubsub.NewMemoryPubSub()
	published := 0
	if err := broker.Subscribe("foundPet", func(context.Context, []byte) error {
		published++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	relay := outbox.NewRelay(stateStore, broker)
	count, err := relay.PublishPending(ctx, "foundPet")
	if err != nil {
		t.Fatalf("PublishPending() error = %v", err)
	}
	if count != 1 || published != 1 {
		t.Fatalf("PublishPending() count = %d, published = %d; want 1, 1", count, published)
	}
	lost, err := outbox.GetRecord(ctx, stateStore, "lost-pending")
	if err != nil {
		t.Fatal(err)
	}
	if lost.Status != outbox.StatusPending {
		t.Fatalf("unrelated record status = %q, want pending", lost.Status)
	}
}

func TestConcurrentRelaysClaimOneOutboxPublication(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	record := outbox.NewRecord("evt-concurrent-relay", "foundPet", []byte(`{"id":"evt-concurrent-relay"}`), time.Now().UTC())
	if err := outbox.SaveRecord(ctx, stateStore, record); err != nil {
		t.Fatal(err)
	}
	broker := newFirstPublishBlockingBroker()
	firstResult := make(chan error, 1)
	go func() {
		_, err := outbox.NewRelay(stateStore, broker).PublishRecords(ctx, record.ID)
		firstResult <- err
	}()
	<-broker.entered
	defer broker.Release()

	count, err := outbox.NewRelay(stateStore, broker).PublishRecords(ctx, record.ID)
	if err != nil {
		t.Fatalf("second PublishRecords() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("second PublishRecords() count = %d, want 0 while claimed", count)
	}
	broker.Release()
	if err := <-firstResult; err != nil {
		t.Fatalf("first PublishRecords() error = %v", err)
	}
	if broker.Calls() != 1 {
		t.Fatalf("broker calls = %d, want 1", broker.Calls())
	}
}
