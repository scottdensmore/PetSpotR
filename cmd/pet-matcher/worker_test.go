package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type failOncePublisher struct {
	broker *pubsub.MemoryPubSub
	calls  atomic.Int32
}

func (p *failOncePublisher) Publish(ctx context.Context, topic string, data []byte) error {
	if p.calls.Add(1) == 1 {
		return errors.New("temporary publish failure")
	}
	return p.broker.Publish(ctx, topic, data)
}

type failCompleteOnceStore struct {
	matcherStore
	calls atomic.Int32
}

func (s *failCompleteOnceStore) CompleteDeliveryOperation(
	ctx context.Context,
	id string,
	attempt int,
	completedAt time.Time,
) error {
	if s.calls.Add(1) == 1 {
		return errors.New("temporary completion failure")
	}
	return s.matcherStore.CompleteDeliveryOperation(ctx, id, attempt, completedAt)
}

type getStateFailingStore struct {
	matcherStore
	err error
}

func (s *getStateFailingStore) GetState(context.Context, string, string) ([]byte, error) {
	return nil, s.err
}

func TestMatcherWorker_ProcessFoundPet(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	var ollamaCalls atomic.Int32
	var matchPublications atomic.Int32

	// Mock Ollama server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ollamaCalls.Add(1)
		resp := ollama.GenerateResponse{
			Model: "gemma2:2b",
			Response: "{\n" +
				"  \"breed\": \"Golden Retriever\",\n" +
				"  \"primaryColor\": \"Golden\",\n" +
				"  \"secondaryColor\": \"Cream\",\n" +
				"  \"distinctiveMarkings\": [\"White chest patch\"],\n" +
				"  \"eyeColor\": \"Brown\"\n" +
				"}",
			Done: true,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	ollamaClient := ollama.NewClient(ollama.WithBaseURL(ts.URL))
	worker := NewWorker(st, ps, ollamaClient)

	// Save lost pet in store
	lostEvt := domain.LostPetEvent{
		PetID:         "lost-101",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Seattle, WA",
	}
	lostData, _ := lostEvt.ToJSON()
	_ = st.SaveState(context.Background(), store.LostPetsCollection, "lost-101", lostData)

	var matchFoundEvent domain.MatchResult
	var matchFoundEnvelope *domain.EventEnvelope
	var matchPublished bool
	_ = ps.Subscribe("matchFound", func(ctx context.Context, data []byte) error {
		matchPublications.Add(1)
		matchPublished = true
		var err error
		matchFoundEnvelope, err = domain.DecodeEventPayload(data, domain.EventTypeMatchFound, &matchFoundEvent)
		return err
	})

	t.Run("found pet matching lost pet publishes matchFound event", func(t *testing.T) {
		foundEvt := domain.FoundPetEvent{
			PetID:    "found-202",
			ImageURL: "https://storage.petspotr.io/found-202.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Seattle, WA",
		}
		foundData, _ := foundEvt.ToJSON()

		err := worker.ProcessFoundPet(context.Background(), foundData)
		if err != nil {
			t.Fatalf("ProcessFoundPet failed: %v", err)
		}

		if !matchPublished {
			t.Fatal("expected matchFound event to be published")
		}

		if matchFoundEvent.FoundPetID != "found-202" || matchFoundEvent.MatchedPetID != "lost-101" {
			t.Errorf("match event IDs mismatch: got found %s, matched %s", matchFoundEvent.FoundPetID, matchFoundEvent.MatchedPetID)
		}

		if !matchFoundEvent.IsMatch {
			t.Errorf("expected IsMatch true, got false")
		}
		if matchFoundEnvelope == nil || matchFoundEnvelope.AggregateID != "found-202:lost-101" {
			t.Fatalf("match envelope = %#v", matchFoundEnvelope)
		}
		firstEventID := matchFoundEnvelope.ID
		if err := worker.ProcessFoundPet(context.Background(), foundData); err != nil {
			t.Fatalf("duplicate ProcessFoundPet failed: %v", err)
		}
		if matchFoundEnvelope.ID != firstEventID {
			t.Fatalf("duplicate match event ID = %q, want stable %q", matchFoundEnvelope.ID, firstEventID)
		}
		if got := ollamaCalls.Load(); got != 1 {
			t.Fatalf("Ollama calls after duplicate = %d, want 1", got)
		}
		if got := matchPublications.Load(); got != 1 {
			t.Fatalf("matchFound publications after duplicate = %d, want 1", got)
		}
	})

	t.Run("versioned found pet envelope remains consumable", func(t *testing.T) {
		foundEvt := domain.FoundPetEvent{
			PetID:    "found-envelope-202",
			ImageURL: "https://storage.petspotr.io/found-envelope-202.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Seattle, WA",
		}
		payload, _ := foundEvt.ToJSON()
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeFoundPetReported,
			OccurredAt:       foundEvt.FoundAt,
			AggregateID:      foundEvt.PetID,
			AggregateVersion: 1,
			PayloadVersion:   1,
			Payload:          payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(envelope)
		if err := worker.ProcessFoundPet(context.Background(), data); err != nil {
			t.Fatalf("ProcessFoundPet(envelope) error = %v", err)
		}
		if matchFoundEvent.FoundPetID != foundEvt.PetID {
			t.Fatalf("match found pet ID = %q, want %q", matchFoundEvent.FoundPetID, foundEvt.PetID)
		}
	})

	t.Run("invalid json event returns error", func(t *testing.T) {
		err := worker.ProcessFoundPet(context.Background(), []byte("{invalid-json"))
		if err == nil {
			t.Error("expected error for invalid json, got nil")
		}
	})

	t.Run("invalid event validation returns error", func(t *testing.T) {
		invalidEvt := domain.FoundPetEvent{PetID: ""} // missing required fields
		data, _ := invalidEvt.ToJSON()
		err := worker.ProcessFoundPet(context.Background(), data)
		if err == nil {
			t.Error("expected error for unvalidated event, got nil")
		}
	})

	t.Run("no candidate in lost pet store returns nil without error", func(t *testing.T) {
		emptyStore := store.NewMemoryStore()
		if err := emptyStore.SaveState(context.Background(), store.LostPetsCollection, "placeholder", []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		if err := emptyStore.DeleteState(context.Background(), store.LostPetsCollection, "placeholder"); err != nil {
			t.Fatal(err)
		}
		emptyWorker := NewWorker(emptyStore, ps, ollamaClient)

		foundEvt := domain.FoundPetEvent{
			PetID:    "found-203",
			ImageURL: "https://storage.petspotr.io/found-203.jpg",
		}
		data, _ := foundEvt.ToJSON()
		err := emptyWorker.ProcessFoundPet(context.Background(), data)
		if err != nil {
			t.Errorf("expected nil error when no candidate available, got %v", err)
		}
	})

	t.Run("transient candidate store failure requests redelivery", func(t *testing.T) {
		transientErr := errors.New("firestore unavailable")
		failingStore := &getStateFailingStore{matcherStore: st, err: transientErr}
		failingWorker := NewWorker(failingStore, ps, ollamaClient)
		foundEvt := domain.FoundPetEvent{
			PetID:    "found-store-error",
			ImageURL: "https://storage.petspotr.io/found-store-error.jpg",
			FoundAt:  time.Now().UTC(),
		}
		data, _ := foundEvt.ToJSON()
		if err := failingWorker.ProcessFoundPet(context.Background(), data); !errors.Is(err, transientErr) {
			t.Fatalf("ProcessFoundPet() error = %v, want transient store error", err)
		}
	})

	t.Run("invalid candidate state requests redelivery", func(t *testing.T) {
		invalidStore := store.NewMemoryStore()
		if err := invalidStore.SaveState(context.Background(), store.LostPetsCollection, "lost-101", []byte(`{invalid`)); err != nil {
			t.Fatal(err)
		}
		invalidWorker := NewWorker(invalidStore, ps, ollamaClient)
		foundEvt := domain.FoundPetEvent{
			PetID:    "found-invalid-candidate",
			ImageURL: "https://storage.petspotr.io/found-invalid-candidate.jpg",
			FoundAt:  time.Now().UTC(),
		}
		data, _ := foundEvt.ToJSON()
		if err := invalidWorker.ProcessFoundPet(context.Background(), data); err == nil {
			t.Fatal("ProcessFoundPet() error = nil, want invalid stored candidate error")
		}
	})

	t.Run("ollama error returned when generate fails", func(t *testing.T) {
		badOllama := ollama.NewClient(ollama.WithBaseURL("http://invalid-host-12345"))
		badWorker := NewWorker(st, ps, badOllama)

		foundEvt := domain.FoundPetEvent{
			PetID:    "found-204",
			ImageURL: "https://storage.petspotr.io/found-204.jpg",
		}
		data, _ := foundEvt.ToJSON()
		err := badWorker.ProcessFoundPet(context.Background(), data)
		if err == nil {
			t.Error("expected error when Ollama client fails, got nil")
		}
	})
}

func TestMatcherWorker_Start(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	oc := ollama.NewClient()
	worker := NewWorker(st, ps, oc)

	t.Run("Start registers foundPet subscription successfully", func(t *testing.T) {
		ctx := context.Background()
		if err := worker.Start(ctx); err != nil {
			t.Fatalf("worker.Start failed: %v", err)
		}
	})

	t.Run("Start returns error on cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := worker.Start(ctx); err == nil {
			t.Error("expected error on cancelled context, got nil")
		}
	})
}

func TestMatcherWorker_RetryPublishesPersistedResultWithoutRerunningOllama(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	publisher := &failOncePublisher{broker: ps}
	var ollamaCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ollamaCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"model":"gemma4:e2b","response":"{\"breed\":\"Golden Retriever\",\"primaryColor\":\"Golden\",\"secondaryColor\":\"Cream\",\"distinctiveMarkings\":[\"White chest patch\"],\"eyeColor\":\"Brown\"}","done":true}`)
	}))
	t.Cleanup(server.Close)

	lostEvent := domain.LostPetEvent{
		PetID:         "lost-101",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Seattle, WA",
	}
	lostData, err := lostEvent.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveState(context.Background(), store.LostPetsCollection, lostEvent.PetID, lostData); err != nil {
		t.Fatal(err)
	}

	var published atomic.Int32
	if err := ps.Subscribe("matchFound", func(context.Context, []byte) error {
		published.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(st, publisher, ollama.NewClient(ollama.WithBaseURL(server.URL)))
	foundEvent := domain.FoundPetEvent{
		PetID:    "found-publish-retry",
		ImageURL: "https://storage.petspotr.io/found-publish-retry.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	foundData, err := foundEvent.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	if err := worker.ProcessFoundPet(context.Background(), foundData); err == nil {
		t.Fatal("first ProcessFoundPet() error = nil, want publish failure")
	}
	if err := worker.ProcessFoundPet(context.Background(), foundData); err != nil {
		t.Fatalf("second ProcessFoundPet() error = %v", err)
	}
	if got := ollamaCalls.Load(); got != 1 {
		t.Fatalf("Ollama calls = %d, want 1", got)
	}
	if got := published.Load(); got != 1 {
		t.Fatalf("matchFound publications = %d, want 1", got)
	}
	assertCompletedMatcherOperation(t, st, foundData, foundEvent.PetID)
	assertPublishedMatcherResult(t, st)
}

func TestMatcherWorker_CompletionRetryDoesNotRepeatPublishedMatch(t *testing.T) {
	baseStore := store.NewMemoryStore()
	st := &failCompleteOnceStore{matcherStore: baseStore}
	ps := pubsub.NewMemoryPubSub()
	var ollamaCalls atomic.Int32
	server := newMatcherOllamaServer(t, &ollamaCalls, nil, nil)
	seedMatcherLostPet(t, baseStore)

	var published atomic.Int32
	if err := ps.Subscribe("matchFound", func(context.Context, []byte) error {
		published.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(st, ps, ollama.NewClient(ollama.WithBaseURL(server.URL)))
	foundEvent := domain.FoundPetEvent{
		PetID:    "found-completion-retry",
		ImageURL: "https://storage.petspotr.io/found-completion-retry.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	foundData, err := foundEvent.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	if err := worker.ProcessFoundPet(context.Background(), foundData); err == nil {
		t.Fatal("first ProcessFoundPet() error = nil, want completion failure")
	}
	if err := worker.ProcessFoundPet(context.Background(), foundData); err != nil {
		t.Fatalf("second ProcessFoundPet() error = %v", err)
	}
	if got := ollamaCalls.Load(); got != 1 {
		t.Fatalf("Ollama calls = %d, want 1", got)
	}
	if got := published.Load(); got != 1 {
		t.Fatalf("matchFound publications = %d, want 1", got)
	}
	assertCompletedMatcherOperation(t, baseStore, foundData, foundEvent.PetID)
	assertPublishedMatcherResult(t, baseStore)
}

func TestMatcherWorker_ConcurrentDuplicateHasOneModelCallAndWinner(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	var ollamaCalls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	server := newMatcherOllamaServer(t, &ollamaCalls, entered, release)
	seedMatcherLostPet(t, st)

	var published atomic.Int32
	if err := ps.Subscribe("matchFound", func(context.Context, []byte) error {
		published.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(st, ps, ollama.NewClient(ollama.WithBaseURL(server.URL)))
	foundEvent := domain.FoundPetEvent{
		PetID:    "found-concurrent",
		ImageURL: "https://storage.petspotr.io/found-concurrent.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	foundData, err := foundEvent.ToJSON()
	if err != nil {
		t.Fatal(err)
	}

	firstResult := make(chan error, 1)
	go func() {
		firstResult <- worker.ProcessFoundPet(context.Background(), foundData)
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first matcher call did not reach Ollama")
	}
	if err := worker.ProcessFoundPet(context.Background(), foundData); !errors.Is(err, delivery.ErrOperationInProgress) {
		t.Fatalf("concurrent ProcessFoundPet() error = %v, want operation in progress", err)
	}
	close(release)
	if err := <-firstResult; err != nil {
		t.Fatalf("first ProcessFoundPet() error = %v", err)
	}
	if got := ollamaCalls.Load(); got != 1 {
		t.Fatalf("Ollama calls = %d, want 1", got)
	}
	if got := published.Load(); got != 1 {
		t.Fatalf("matchFound publications = %d, want 1", got)
	}
}

func TestMatcherWorker_DistinctInputsWithSameScoreHaveDistinctResults(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	var ollamaCalls atomic.Int32
	server := newMatcherOllamaServer(t, &ollamaCalls, nil, nil)
	seedMatcherLostPet(t, st)

	var publishedIDs []string
	if err := ps.Subscribe("matchFound", func(_ context.Context, data []byte) error {
		var result domain.MatchResult
		envelope, err := domain.DecodeEventPayload(data, domain.EventTypeMatchFound, &result)
		if err != nil {
			return err
		}
		publishedIDs = append(publishedIDs, envelope.ID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(st, ps, ollama.NewClient(ollama.WithBaseURL(server.URL)))
	baseTime := time.Now().UTC()
	for index := range 2 {
		foundEvent := domain.FoundPetEvent{
			PetID:    "found-same-score",
			ImageURL: "https://storage.petspotr.io/found-same-score.jpg",
			FoundAt:  baseTime.Add(time.Duration(index) * time.Second),
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
			AggregateVersion: int64(index + 1),
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
		if err := worker.ProcessFoundPet(context.Background(), data); err != nil {
			t.Fatalf("ProcessFoundPet(input %d) error = %v", index+1, err)
		}
	}
	if got := ollamaCalls.Load(); got != 2 {
		t.Fatalf("Ollama calls = %d, want 2 distinct inputs", got)
	}
	if len(publishedIDs) != 2 || publishedIDs[0] == publishedIDs[1] {
		t.Fatalf("published event IDs = %v, want two distinct results", publishedIDs)
	}
	results, err := st.ListState(context.Background(), store.MatcherResultsCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("matcher result count = %d, want 2", len(results))
	}
}

func newMatcherOllamaServer(
	t *testing.T,
	calls *atomic.Int32,
	entered chan<- struct{},
	release <-chan struct{},
) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		if entered != nil {
			entered <- struct{}{}
		}
		if release != nil {
			<-release
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"model":"gemma4:e2b","response":"{\"breed\":\"Golden Retriever\",\"primaryColor\":\"Golden\",\"secondaryColor\":\"Cream\",\"distinctiveMarkings\":[\"White chest patch\"],\"eyeColor\":\"Brown\"}","done":true}`)
	}))
	t.Cleanup(server.Close)
	return server
}

func seedMatcherLostPet(t *testing.T, st store.StateStore) {
	t.Helper()
	lostEvent := domain.LostPetEvent{
		PetID:         "lost-101",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Now().UTC(),
		Location:      "Seattle, WA",
	}
	data, err := lostEvent.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveState(context.Background(), store.LostPetsCollection, lostEvent.PetID, data); err != nil {
		t.Fatal(err)
	}
}

func assertCompletedMatcherOperation(t *testing.T, st store.DeliveryOperationStore, input []byte, petID string) {
	t.Helper()
	eventID, err := delivery.ResolveEventID("", domain.EventTypeFoundPetReported, input)
	if err != nil {
		t.Fatal(err)
	}
	operation, err := delivery.NewOperation(eventID, petID, matcherDeliveryChannel, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	stored, err := st.GetDeliveryOperation(context.Background(), operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != delivery.StatusCompleted {
		t.Fatalf("matcher operation status = %q, want completed", stored.Status)
	}
}

func assertPublishedMatcherResult(t *testing.T, st store.StateStore) {
	t.Helper()
	results, err := st.ListState(context.Background(), store.MatcherResultsCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("matcher result count = %d, want 1", len(results))
	}
	for _, data := range results {
		var result matcherResultRecord
		if err := json.Unmarshal(data, &result); err != nil {
			t.Fatal(err)
		}
		record, err := outbox.GetRecord(context.Background(), st, result.OutboxID)
		if err != nil {
			t.Fatal(err)
		}
		if record.Status != outbox.StatusPublished {
			t.Fatalf("matcher outbox status = %q, want published", record.Status)
		}
	}
}
