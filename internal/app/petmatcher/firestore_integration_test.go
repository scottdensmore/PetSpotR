package petmatcher

import (
	"context"
	"encoding/json"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreMatcherRecoversPersistedResultAcrossWorkers(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx := context.Background()
	firstStore, err := store.NewFirestoreEmulatorStore(ctx, "petspotr-matcher-recovery", host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore(first) error = %v", err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	secondStore, err := store.NewFirestoreEmulatorStore(ctx, "petspotr-matcher-recovery", host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore(second) error = %v", err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })

	var ollamaCalls atomic.Int32
	server := newMatcherOllamaServer(t, &ollamaCalls, nil, nil)
	seedMatcherLostPet(t, firstStore)
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.LostPetsCollection, "lost-101")
	})

	foundEvent := domain.FoundPetEvent{
		PetID:    "found-firestore-recovery",
		ImageURL: "https://storage.petspotr.io/found-firestore-recovery.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	payload, err := foundEvent.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       foundEvent.FoundAt,
		AggregateID:      foundEvent.PetID,
		AggregateVersion: 1,
		PayloadVersion:   1,
		Payload:          payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	foundData, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := delivery.NewOperation(envelope.ID, foundEvent.PetID, matcherDeliveryChannel, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.NotificationDeliveriesCollection, operation.ID)
		_ = firstStore.DeleteState(context.Background(), store.MatcherResultsCollection, envelope.ID)
	})

	broker := pubsub.NewMemoryPubSub()
	publisher := &failOncePublisher{broker: broker}
	var published atomic.Int32
	if err := broker.Subscribe("matchFound", func(context.Context, []byte) error {
		published.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	client := ollama.NewClient(ollama.WithBaseURL(server.URL))
	if err := NewWorker(firstStore, publisher, client).ProcessFoundPet(ctx, foundData); err == nil {
		t.Fatal("first ProcessFoundPet() error = nil, want publish failure")
	}
	storedResult, err := firstStore.GetState(ctx, store.MatcherResultsCollection, envelope.ID)
	if err != nil {
		t.Fatalf("GetState(matcher result) error = %v", err)
	}
	var result matcherResultRecord
	if err := json.Unmarshal(storedResult, &result); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.OutboxCollection, result.OutboxID)
	})

	if err := NewWorker(secondStore, publisher, client).ProcessFoundPet(ctx, foundData); err != nil {
		t.Fatalf("second ProcessFoundPet() error = %v", err)
	}
	if got := ollamaCalls.Load(); got != 1 {
		t.Fatalf("Ollama calls across workers = %d, want 1", got)
	}
	if got := published.Load(); got != 1 {
		t.Fatalf("matchFound publications across workers = %d, want 1", got)
	}
	completed, err := secondStore.GetDeliveryOperation(ctx, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != delivery.StatusCompleted {
		t.Fatalf("matcher operation status = %q, want completed", completed.Status)
	}
}
