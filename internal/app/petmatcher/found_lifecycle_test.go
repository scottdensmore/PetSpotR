package petmatcher

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMatcherWorkerCompletesResolvedFoundReportWithoutNewInferenceOrMatch(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	now := time.Now().UTC()
	finder := domain.PrincipalRef{Issuer: "issuer", Subject: "finder-terminal"}
	event := domain.FoundPetReportedV2{
		PetID: "found-terminal", ImageURL: "https://storage.petspotr.io/found-terminal.jpg",
		FoundAt: now, Location: "Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: matcherTestPoint(), Species: "Dog", CustodyStatus: domain.CustodyFinderHome,
		Status: domain.FoundPetStatusFound,
	}
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: event.PetID, ImageURL: event.ImageURL, FoundAt: event.FoundAt,
		Location: event.Location, GeocodingStatus: event.GeocodingStatus,
		Coordinates: event.Coordinates, Species: event.Species, CustodyStatus: event.CustodyStatus,
		Status: event.Status, OwnedBy: &finder,
	})
	record, _ := report.Persisted()
	resolved, err := domain.ApplyFinderFoundPetResolution(record, finder, "resolve-before-delivery", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resolvedData, err := json.Marshal(resolved.Record)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.FoundPetsCollection, event.PetID, resolvedData); err != nil {
		t.Fatal(err)
	}
	seedMatcherLostPet(t, state)

	var ollamaCalls atomic.Int32
	server := newMatcherOllamaServer(t, &ollamaCalls, nil, nil)
	worker := NewWorker(state, pubsub.NewMemoryPubSub(), ollama.NewClient(ollama.WithBaseURL(server.URL)))
	input := verifiedFoundEventData(t, event)
	if err := worker.ProcessFoundPet(ctx, input); err != nil {
		t.Fatalf("ProcessFoundPet() error = %v", err)
	}
	if ollamaCalls.Load() != 0 {
		t.Fatalf("resolved found report Ollama calls = %d, want 0", ollamaCalls.Load())
	}
	if matches, err := state.ListState(ctx, store.MatchesCollection); err != nil || len(matches) != 0 {
		t.Fatalf("resolved found report matches = %#v, %v", matches, err)
	}
	if records, err := state.ListPendingOutbox(ctx, matcherDeliveryChannel, 10); err != nil || len(records) != 0 {
		t.Fatalf("resolved found report match outbox = %#v, %v", records, err)
	}
	assertCompletedMatcherOperation(t, state, input, event.PetID)
}

func TestMatcherWorkerPublishesExistingResultBeforeResolvedFoundGuard(t *testing.T) {
	ctx := context.Background()
	state := store.NewMemoryStore()
	now := time.Now().UTC()
	finder := domain.PrincipalRef{Issuer: "issuer", Subject: "finder-result-recovery"}
	event := domain.FoundPetReportedV2{
		PetID: "found-terminal-recovery", ImageURL: "https://storage.petspotr.io/found-terminal-recovery.jpg",
		FoundAt: now, Location: "Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: matcherTestPoint(), Species: "Dog", CustodyStatus: domain.CustodyFinderHome,
		Status: domain.FoundPetStatusFound,
	}
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: event.PetID, ImageURL: event.ImageURL, FoundAt: event.FoundAt,
		Location: event.Location, GeocodingStatus: event.GeocodingStatus,
		Coordinates: event.Coordinates, Species: event.Species, CustodyStatus: event.CustodyStatus,
		Status: event.Status, OwnedBy: &finder,
	})
	record, _ := report.Persisted()
	resolved, err := domain.ApplyFinderFoundPetResolution(record, finder, "resolve-after-result", now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	resolvedData, err := json.Marshal(resolved.Record)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.FoundPetsCollection, event.PetID, resolvedData); err != nil {
		t.Fatal(err)
	}

	input := verifiedFoundEventData(t, event)
	_, inputEnvelope, err := domain.DecodeFoundPetReported(input)
	if err != nil {
		t.Fatal(err)
	}
	match := domain.MatchResult{
		FoundPetID: event.PetID, MatchedPetID: "lost-terminal-recovery",
		Score: 0.9, IsMatch: true, SourceEventID: inputEnvelope.ID,
	}
	matchPayload, err := match.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	matchEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type: domain.EventTypeMatchFound, OccurredAt: now,
		AggregateID:      event.PetID + ":" + match.MatchedPetID,
		AggregateVersion: 1, PayloadVersion: 1, Payload: matchPayload,
	})
	if err != nil {
		t.Fatal(err)
	}
	matchData, err := json.Marshal(matchEnvelope)
	if err != nil {
		t.Fatal(err)
	}
	resultData, err := json.Marshal(matcherResultRecord{
		InputEventID: inputEnvelope.ID, OutboxID: matchEnvelope.ID, MatchID: "match-terminal-recovery",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := state.SaveState(ctx, store.MatcherResultsCollection, inputEnvelope.ID, resultData); err != nil {
		t.Fatal(err)
	}
	if err := outbox.SaveRecord(ctx, state, outbox.NewRecord(
		matchEnvelope.ID, matcherDeliveryChannel, matchData, now,
	)); err != nil {
		t.Fatal(err)
	}

	broker := pubsub.NewMemoryPubSub()
	var publications atomic.Int32
	if err := broker.Subscribe(matcherDeliveryChannel, func(_ context.Context, got []byte) error {
		publications.Add(1)
		if string(got) != string(matchData) {
			t.Errorf("published payload = %s, want %s", got, matchData)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := NewWorker(state, broker, nil).ProcessFoundPet(ctx, input); err != nil {
		t.Fatalf("ProcessFoundPet() error = %v", err)
	}
	if publications.Load() != 1 {
		t.Fatalf("existing match publications = %d, want 1", publications.Load())
	}
	assertCompletedMatcherOperation(t, state, input, event.PetID)
}
