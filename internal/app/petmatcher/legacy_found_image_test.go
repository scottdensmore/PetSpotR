package petmatcher

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMatcherWorkerSafelyCompletesLegacyURLOnlyFoundEvents(t *testing.T) {
	now := time.Now().UTC()
	var untrustedImageRequests atomic.Int32
	untrustedImageServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		untrustedImageRequests.Add(1)
	}))
	t.Cleanup(untrustedImageServer.Close)

	legacy := domain.FoundPetEvent{
		PetID: "found-legacy-v1", ImageURL: untrustedImageServer.URL + "/caller-controlled.jpg",
		FoundAt: now, Location: "Seattle, WA",
	}
	rawLegacy, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyPayload, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	legacyEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type: domain.EventTypeFoundPetReported, OccurredAt: now, AggregateID: legacy.PetID,
		AggregateVersion: 1, PayloadVersion: domain.FoundPetReportedLegacyPayloadVersion, Payload: legacyPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	envelopedLegacy, err := json.Marshal(legacyEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		data []byte
	}{
		{name: "enveloped payload v1", data: envelopedLegacy},
		{name: "raw pre-envelope payload", data: rawLegacy},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateStore := store.NewMemoryStore()
			seedMatcherLostPet(t, stateStore)
			broker := pubsub.NewMemoryPubSub()
			var publications atomic.Int32
			if err := broker.Subscribe("matchFound", func(context.Context, []byte) error {
				publications.Add(1)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			var ollamaCalls atomic.Int32
			ollamaServer := newMatcherOllamaServer(t, &ollamaCalls, nil, nil)
			worker := NewWorker(stateStore, broker, ollama.NewClient(ollama.WithBaseURL(ollamaServer.URL)))

			for attempt := range 2 {
				if err := worker.ProcessFoundPet(context.Background(), test.data); err != nil {
					t.Fatalf("ProcessFoundPet() attempt %d error = %v", attempt+1, err)
				}
			}
			if got := ollamaCalls.Load(); got != 0 {
				t.Fatalf("Ollama calls = %d, want 0 for caller-controlled imageUrl", got)
			}
			if got := untrustedImageRequests.Load(); got != 0 {
				t.Fatalf("untrusted image requests = %d, want 0", got)
			}
			if got := publications.Load(); got != 0 {
				t.Fatalf("matchFound publications = %d, want 0", got)
			}
			for _, collection := range []string{
				store.MatcherResultsCollection,
				store.MatchesCollection,
				store.MatchParticipantsCollection,
				store.OutboxCollection,
			} {
				records, listErr := stateStore.ListState(context.Background(), collection)
				if listErr != nil {
					t.Fatal(listErr)
				}
				if len(records) != 0 {
					t.Fatalf("%s records = %d, want 0", collection, len(records))
				}
			}

			decoded, envelope, decodeErr := domain.DecodeFoundPetReported(test.data)
			if decodeErr != nil {
				t.Fatalf("DecodeFoundPetReported() error = %v", decodeErr)
			}
			envelopeID := ""
			if envelope != nil {
				envelopeID = envelope.ID
			}
			eventID, resolveErr := delivery.ResolveEventID(envelopeID, domain.EventTypeFoundPetReported, test.data)
			if resolveErr != nil {
				t.Fatal(resolveErr)
			}
			operation, operationErr := delivery.NewOperation(eventID, decoded.PetID, matcherDeliveryChannel, now)
			if operationErr != nil {
				t.Fatal(operationErr)
			}
			completed, getErr := stateStore.GetDeliveryOperation(context.Background(), operation.ID)
			if getErr != nil {
				t.Fatal(getErr)
			}
			if completed.Status != delivery.StatusCompleted || completed.Attempt != 1 {
				t.Fatalf("delivery status/attempt = %s/%d, want completed/1", completed.Status, completed.Attempt)
			}
		})
	}
}

func TestMatcherWorkerLegacyV1RetryPublishesExistingDurableResult(t *testing.T) {
	now := time.Now().UTC()
	legacy := domain.FoundPetEvent{
		PetID: "found-legacy-recovery", ImageURL: "https://caller.invalid/found.jpg",
		FoundAt: now, Location: "Seattle, WA",
	}
	payload, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	inputEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type: domain.EventTypeFoundPetReported, OccurredAt: now, AggregateID: legacy.PetID,
		AggregateVersion: 1, PayloadVersion: domain.FoundPetReportedLegacyPayloadVersion, Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	input, err := json.Marshal(inputEnvelope)
	if err != nil {
		t.Fatal(err)
	}

	match := domain.MatchResult{
		FoundPetID: legacy.PetID, MatchedPetID: "lost-legacy-recovery",
		Score: 0.9, IsMatch: true, SourceEventID: inputEnvelope.ID,
	}
	matchPayload, err := match.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	matchEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type: domain.EventTypeMatchFound, OccurredAt: now,
		AggregateID:      legacy.PetID + ":" + match.MatchedPetID,
		AggregateVersion: 1, PayloadVersion: 1, Payload: matchPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	matchData, err := json.Marshal(matchEnvelope)
	if err != nil {
		t.Fatal(err)
	}

	stateStore := store.NewMemoryStore()
	resultData, err := json.Marshal(matcherResultRecord{
		InputEventID: inputEnvelope.ID, OutboxID: matchEnvelope.ID, MatchID: "match-legacy-recovery",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(
		context.Background(), store.MatcherResultsCollection, inputEnvelope.ID, resultData,
	); err != nil {
		t.Fatal(err)
	}
	if err := outbox.SaveRecord(
		context.Background(), stateStore,
		outbox.NewRecord(matchEnvelope.ID, matcherDeliveryChannel, matchData, now),
	); err != nil {
		t.Fatal(err)
	}

	broker := pubsub.NewMemoryPubSub()
	var publications atomic.Int32
	if err := broker.Subscribe("matchFound", func(_ context.Context, got []byte) error {
		publications.Add(1)
		if string(got) != string(matchData) {
			t.Errorf("published payload = %s, want %s", got, matchData)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(stateStore, broker, nil)
	for attempt := range 2 {
		if err := worker.ProcessFoundPet(context.Background(), input); err != nil {
			t.Fatalf("ProcessFoundPet() attempt %d error = %v", attempt+1, err)
		}
	}
	if got := publications.Load(); got != 1 {
		t.Fatalf("matchFound publications = %d, want 1 durable recovery publication", got)
	}
	assertCompletedMatcherOperation(t, stateStore, input, legacy.PetID)
	storedOutbox, err := outbox.GetRecord(context.Background(), stateStore, matchEnvelope.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedOutbox.Status != outbox.StatusPublished {
		t.Fatalf("recovered outbox status = %q, want %q", storedOutbox.Status, outbox.StatusPublished)
	}
}
