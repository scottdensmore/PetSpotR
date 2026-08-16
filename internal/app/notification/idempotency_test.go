package notification

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
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

type blockingTestSender struct {
	delegate *idempotentTestSender
	entered  chan struct{}
	release  chan struct{}
	once     sync.Once
}

func newBlockingTestSender(channel Channel) *blockingTestSender {
	return &blockingTestSender{
		delegate: newIdempotentTestSender(channel, 0),
		entered:  make(chan struct{}),
		release:  make(chan struct{}),
	}
}

func (s *blockingTestSender) Channel() Channel { return s.delegate.Channel() }

func (s *blockingTestSender) Send(ctx context.Context, message *NotificationMessage) error {
	s.once.Do(func() { close(s.entered) })
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return s.delegate.Send(ctx, message)
	}
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

func TestLostPetBroadcastRedeliverySkipsCompletedSubscriberChannels(t *testing.T) {
	stateStore := store.NewMemoryStore()
	email := newIdempotentTestSender(ChannelEmail, 0)
	sms := newIdempotentTestSender(ChannelSMS, 1)
	push := newIdempotentTestSender(ChannelPush, 0)
	dispatcher := NewMultiChannelDispatcher(email, sms, push)
	worker := NewWorkerWithStoreAndDispatcher(stateStore, pubsub.NewMemoryPubSub(), dispatcher)
	worker.geoEngine = NewGeoBroadcastEngine([]CommunitySubscriber{
		{
			ID:          "subscriber-nearby",
			Email:       "neighbor@example.com",
			Phone:       "+12065550123",
			Coordinates: domain.LocationPoint{Latitude: 47.6800, Longitude: -122.3290},
			RadiusMiles: 5,
			Channels:    []Channel{ChannelEmail, ChannelSMS, ChannelPush},
		},
	}, dispatcher)
	data := lostPetEnvelope(t, "lost-community")

	if _, err := worker.ProcessLostPetBroadcast(context.Background(), data); err == nil {
		t.Fatal("first ProcessLostPetBroadcast() error = nil, want SMS provider failure")
	}
	if _, err := worker.ProcessLostPetBroadcast(context.Background(), data); err != nil {
		t.Fatalf("redelivered ProcessLostPetBroadcast() error = %v", err)
	}
	if _, err := worker.ProcessLostPetBroadcast(context.Background(), data); err != nil {
		t.Fatalf("completed replay ProcessLostPetBroadcast() error = %v", err)
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

func TestLegacyLostPetBroadcastIsAcceptedWithoutInventedCoordinates(t *testing.T) {
	stateStore := store.NewMemoryStore()
	email := newIdempotentTestSender(ChannelEmail, 0)
	dispatcher := NewMultiChannelDispatcher(email)
	worker := NewWorkerWithStoreAndDispatcher(stateStore, pubsub.NewMemoryPubSub(), dispatcher)
	worker.geoEngine = NewGeoBroadcastEngine([]CommunitySubscriber{
		{
			ID:          "subscriber-legacy",
			Email:       "legacy-neighbor@example.com",
			Coordinates: domain.LocationPoint{Latitude: 47.6800, Longitude: -122.3290},
			RadiusMiles: 5,
			Channels:    []Channel{ChannelEmail},
		},
	}, dispatcher)
	event := domain.LostPetEvent{
		PetID:         "lost-legacy-community",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC),
		Location:      "Green Lake Park, Seattle, WA",
	}
	data, err := event.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := worker.ProcessLostPetBroadcast(context.Background(), data); err != nil {
			t.Fatalf("ProcessLostPetBroadcast(legacy) error = %v", err)
		}
	}
	if calls, effects := email.snapshot(); calls != 0 || effects != 0 {
		t.Fatalf("legacy email calls/effects = %d/%d, want 0/0", calls, effects)
	}
}

func TestLostPetBroadcastDefaultsEmptyChannelsToIdempotentEmail(t *testing.T) {
	stateStore := store.NewMemoryStore()
	email := newIdempotentTestSender(ChannelEmail, 0)
	dispatcher := NewMultiChannelDispatcher(email)
	worker := NewWorkerWithStoreAndDispatcher(stateStore, pubsub.NewMemoryPubSub(), dispatcher)
	worker.geoEngine = NewGeoBroadcastEngine([]CommunitySubscriber{
		{
			ID:          "subscriber-default-email",
			Email:       "default-neighbor@example.com",
			Coordinates: domain.LocationPoint{Latitude: 47.6800, Longitude: -122.3290},
			RadiusMiles: 5,
			Channels:    nil,
		},
	}, dispatcher)
	data := lostPetEnvelope(t, "lost-default-email")

	for range 2 {
		results, err := worker.ProcessLostPetBroadcast(context.Background(), data)
		if err != nil {
			t.Fatalf("ProcessLostPetBroadcast(default email) error = %v", err)
		}
		if len(results) != 1 || results[0].Channel != ChannelEmail || !results[0].Success {
			t.Fatalf("default email results = %#v, want one successful email", results)
		}
	}
	if calls, effects := email.snapshot(); calls != 1 || effects != 1 {
		t.Fatalf("default email calls/effects = %d/%d, want 1/1", calls, effects)
	}
}

func TestConcurrentLostPetBroadcastRequestsRedeliveryForActiveLease(t *testing.T) {
	stateStore := store.NewMemoryStore()
	email := newBlockingTestSender(ChannelEmail)
	dispatcher := NewMultiChannelDispatcher(email)
	worker := NewWorkerWithStoreAndDispatcher(stateStore, pubsub.NewMemoryPubSub(), dispatcher)
	worker.geoEngine = NewGeoBroadcastEngine([]CommunitySubscriber{
		{
			ID:          "subscriber-concurrent",
			Email:       "concurrent-neighbor@example.com",
			Coordinates: domain.LocationPoint{Latitude: 47.6800, Longitude: -122.3290},
			RadiusMiles: 5,
			Channels:    []Channel{ChannelEmail},
		},
	}, dispatcher)
	data := lostPetEnvelope(t, "lost-concurrent-community")
	firstResult := make(chan error, 1)
	go func() {
		_, err := worker.ProcessLostPetBroadcast(context.Background(), data)
		firstResult <- err
	}()
	<-email.entered

	if _, err := worker.ProcessLostPetBroadcast(context.Background(), data); !errors.Is(err, delivery.ErrOperationInProgress) {
		t.Fatalf("concurrent ProcessLostPetBroadcast() error = %v, want ErrOperationInProgress", err)
	}
	close(email.release)
	if err := <-firstResult; err != nil {
		t.Fatalf("first ProcessLostPetBroadcast() error = %v", err)
	}
	if calls, effects := email.delegate.snapshot(); calls != 1 || effects != 1 {
		t.Fatalf("concurrent email calls/effects = %d/%d, want 1/1", calls, effects)
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

func lostPetEnvelope(t *testing.T, petID string) []byte {
	t.Helper()
	event := domain.LostPetReportedV2{
		PetID:           petID,
		ReportedAt:      time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC),
		Location:        "Green Lake Park, Seattle, WA",
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     &domain.LocationPoint{Latitude: 47.68, Longitude: -122.329},
		Status:          domain.LostPetStatusLost,
	}
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeLostPetReported,
		OccurredAt:       event.ReportedAt,
		AggregateID:      event.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.LostPetReportedPayloadVersion,
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
