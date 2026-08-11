package main

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type idempotentTestSender struct {
	mu            sync.Mutex
	channel       Channel
	failRemaining int
	calls         int
	effects       map[string]int
}

func newIdempotentTestSender(channel Channel, failRemaining int) *idempotentTestSender {
	return &idempotentTestSender{channel: channel, failRemaining: failRemaining, effects: make(map[string]int)}
}

func (s *idempotentTestSender) Channel() Channel { return s.channel }

func (s *idempotentTestSender) Send(_ context.Context, message *NotificationMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if message.IdempotencyKey == "" {
		return errors.New("missing provider idempotency key")
	}
	if s.failRemaining > 0 {
		s.failRemaining--
		return errors.New("provider unavailable")
	}
	if s.effects[message.IdempotencyKey] == 0 {
		s.effects[message.IdempotencyKey]++
	}
	return nil
}

func (s *idempotentTestSender) snapshot() (calls, effects int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, count := range s.effects {
		effects += count
	}
	return s.calls, effects
}

type completionFailingDeliveryStore struct {
	store.DeliveryOperationStore
	mu        sync.Mutex
	remaining int
}

func (s *completionFailingDeliveryStore) CompleteDeliveryOperation(
	ctx context.Context,
	id string,
	attempt int,
	completedAt time.Time,
) error {
	s.mu.Lock()
	if s.remaining > 0 {
		s.remaining--
		s.mu.Unlock()
		return errors.New("completion store unavailable")
	}
	s.mu.Unlock()
	return s.DeliveryOperationStore.CompleteDeliveryOperation(ctx, id, attempt, completedAt)
}

func TestMatchFoundRedeliverySkipsCompletedChannels(t *testing.T) {
	stateStore := store.NewMemoryStore()
	email := newIdempotentTestSender(ChannelEmail, 0)
	sms := newIdempotentTestSender(ChannelSMS, 1)
	push := newIdempotentTestSender(ChannelPush, 0)
	dispatcher := NewMultiChannelDispatcher(email, sms, push)
	worker := NewWorkerWithStoreAndDispatcher(stateStore, pubsub.NewMemoryPubSub(), dispatcher)
	data := matchFoundEnvelope(t, "found-partial", "lost-partial")

	if _, err := worker.ProcessMatchFound(context.Background(), data); err == nil {
		t.Fatal("first ProcessMatchFound() error = nil, want SMS provider failure")
	}
	if _, err := worker.ProcessMatchFound(context.Background(), data); err != nil {
		t.Fatalf("redelivered ProcessMatchFound() error = %v", err)
	}
	if _, err := worker.ProcessMatchFound(context.Background(), data); err != nil {
		t.Fatalf("completed replay ProcessMatchFound() error = %v", err)
	}

	if calls, effects := email.snapshot(); calls != 1 || effects != 1 {
		t.Fatalf("email calls/effects = %d/%d, want 1/1", calls, effects)
	}
	if calls, effects := sms.snapshot(); calls != 2 || effects != 1 {
		t.Fatalf("SMS calls/effects = %d/%d, want 2/1", calls, effects)
	}
	if calls, effects := push.snapshot(); calls != 1 || effects != 1 {
		t.Fatalf("push calls/effects = %d/%d, want 1/1", calls, effects)
	}
}

func TestMatchFoundCompletionCrashRetriesWithSameProviderKey(t *testing.T) {
	baseStore := store.NewMemoryStore()
	stateStore := &completionFailingDeliveryStore{DeliveryOperationStore: baseStore, remaining: 1}
	email := newIdempotentTestSender(ChannelEmail, 0)
	sms := newIdempotentTestSender(ChannelSMS, 0)
	push := newIdempotentTestSender(ChannelPush, 0)
	worker := NewWorkerWithStoreAndDispatcher(
		stateStore,
		pubsub.NewMemoryPubSub(),
		NewMultiChannelDispatcher(email, sms, push),
	)
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	worker.now = func() time.Time { return now }
	worker.deliveryLease = time.Second
	data := matchFoundEnvelope(t, "found-crash", "lost-crash")

	if _, err := worker.ProcessMatchFound(context.Background(), data); err == nil {
		t.Fatal("first ProcessMatchFound() error = nil, want completion persistence failure")
	}
	now = now.Add(2 * time.Second)
	if _, err := worker.ProcessMatchFound(context.Background(), data); err != nil {
		t.Fatalf("redelivered ProcessMatchFound() error = %v", err)
	}

	if calls, effects := email.snapshot(); calls != 2 || effects != 1 {
		t.Fatalf("email calls/effects = %d/%d, want provider retry 2 and one side effect", calls, effects)
	}
	if _, effects := sms.snapshot(); effects != 1 {
		t.Fatalf("SMS effects = %d, want 1", effects)
	}
	if _, effects := push.snapshot(); effects != 1 {
		t.Fatalf("push effects = %d, want 1", effects)
	}
}

func matchFoundEnvelope(t *testing.T, foundPetID, lostPetID string) []byte {
	t.Helper()
	result := domain.MatchResult{
		FoundPetID:   foundPetID,
		MatchedPetID: lostPetID,
		Score:        0.95,
		IsMatch:      true,
	}
	payload, err := result.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeMatchFound,
		OccurredAt:       time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC),
		AggregateID:      foundPetID + ":" + lostPetID,
		AggregateVersion: 1,
		PayloadVersion:   1,
		Payload:          payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
