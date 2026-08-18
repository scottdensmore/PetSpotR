package petmatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFirestoreFoundImageAnalysisSurvivesCompletionRetryAcrossWorkers(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx := context.Background()
	projectID := "petspotr-found-analysis-recovery"
	firstStore, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore(first) error = %v", err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	secondStore, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore(second) error = %v", err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })

	images := blob.NewMemoryBlobStore("https://storage.petspotr.invalid")
	grant, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeFoundPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := images.UploadImage(ctx, grant.ObjectName, encodedMatcherImage(t)); err != nil {
		t.Fatal(err)
	}
	finalized, err := images.FinalizeImageForPurpose(
		ctx, blob.ImagePurposeFoundPet, grant.ReportID, grant.ObjectName, grant.FinalizeToken,
	)
	if err != nil {
		t.Fatal(err)
	}
	foundEvent := domain.FoundPetReportedV2{
		PetID: grant.ReportID, ImageObject: finalized.ObjectName, FoundAt: time.Now().UTC(),
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: matcherTestPoint(), Species: "Dog", CustodyStatus: domain.CustodyFinderHome,
		Status: domain.FoundPetStatusFound,
	}
	seedMatcherFoundPet(t, firstStore, foundEvent)
	seedMatcherLostPet(t, firstStore)
	foundData := verifiedFoundEventData(t, foundEvent)
	_, envelope, err := domain.DecodeFoundPetReported(foundData)
	if err != nil {
		t.Fatal(err)
	}
	if envelope == nil {
		t.Fatal("verified found-pet fixture omitted event envelope")
	}
	operation, err := delivery.NewOperation(envelope.ID, foundEvent.PetID, matcherDeliveryChannel, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.FoundPetsCollection, foundEvent.PetID)
		_ = firstStore.DeleteState(context.Background(), store.LostPetsCollection, "lost-101")
		_ = firstStore.DeleteState(context.Background(), store.NotificationDeliveriesCollection, operation.ID)
	})

	var ollamaCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ollamaCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"model":"gemma4:e2b","response":"{\"breed\":\"Poodle\",\"primaryColor\":\"Black\",\"secondaryColor\":\"Gray\",\"distinctiveMarkings\":[\"Black ear\"],\"eyeColor\":\"Blue\"}","done":true}`)
	}))
	t.Cleanup(server.Close)
	client := ollama.NewClient(ollama.WithBaseURL(server.URL))
	failingStore := &failCompleteOnceStore{Store: firstStore}
	if err := NewWorkerWithImageStore(failingStore, pubsub.NewMemoryPubSub(), client, images).
		ProcessFoundPet(ctx, foundData); err == nil {
		t.Fatal("first ProcessFoundPet() error = nil, want completion failure")
	}
	if err := NewWorkerWithImageStore(secondStore, pubsub.NewMemoryPubSub(), client, images).
		ProcessFoundPet(ctx, foundData); err != nil {
		t.Fatalf("second ProcessFoundPet() error = %v", err)
	}
	if got := ollamaCalls.Load(); got != 1 {
		t.Fatalf("Ollama calls across workers = %d, want 1", got)
	}
	foundState, err := secondStore.GetState(ctx, store.FoundPetsCollection, foundEvent.PetID)
	if err != nil {
		t.Fatal(err)
	}
	var persisted domain.FoundPetRecord
	if err := json.Unmarshal(foundState, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.ImageAnalysis == nil || persisted.ImageAnalysis.SourceEventID != envelope.ID {
		t.Fatalf("persisted found image analysis = %#v", persisted.ImageAnalysis)
	}
	completed, err := secondStore.GetDeliveryOperation(ctx, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != delivery.StatusCompleted {
		t.Fatalf("matcher operation status = %q, want completed", completed.Status)
	}
}

func TestFirestoreLostImageAnalysisSurvivesCompletionRetryAcrossWorkers(t *testing.T) {
	host := os.Getenv("FIRESTORE_EMULATOR_HOST")
	if host == "" {
		t.Skip("FIRESTORE_EMULATOR_HOST is not set")
	}
	ctx := context.Background()
	projectID := "petspotr-lost-analysis-recovery"
	firstStore, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore(first) error = %v", err)
	}
	t.Cleanup(func() { _ = firstStore.Close() })
	secondStore, err := store.NewFirestoreEmulatorStore(ctx, projectID, host)
	if err != nil {
		t.Fatalf("NewFirestoreEmulatorStore(second) error = %v", err)
	}
	t.Cleanup(func() { _ = secondStore.Close() })

	images := blob.NewMemoryBlobStore("https://storage.petspotr.invalid")
	grant, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeLostPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := images.UploadImage(ctx, grant.ObjectName, encodedMatcherImage(t)); err != nil {
		t.Fatal(err)
	}
	finalized, err := images.FinalizeImageForPurpose(ctx, blob.ImagePurposeLostPet, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil {
		t.Fatal(err)
	}
	reportedAt := time.Now().UTC()
	record := domain.LostPetRecord{
		PetID: grant.ReportID, OwnerIdentityRef: "identity-" + grant.ReportID,
		ImageObject: finalized.ObjectName, ReportedAt: reportedAt,
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingPending,
		Status: domain.LostPetStatusLost,
	}
	stateData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := firstStore.SaveState(ctx, store.LostPetsCollection, record.PetID, stateData); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.LostPetsCollection, record.PetID)
	})
	eventData, eventID := encodeLostAnalysisEvent(t, domain.LostPetReportedV4{
		PetID: record.PetID, ImageObject: record.ImageObject, ReportedAt: record.ReportedAt,
		Location: record.Location, GeocodingStatus: record.GeocodingStatus, Status: record.Status,
	})
	operation, err := delivery.NewOperation(eventID, record.PetID, lostImageAnalysisChannel, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.NotificationDeliveriesCollection, operation.ID)
	})

	var ollamaCalls atomic.Int32
	server := newMatcherOllamaServer(t, &ollamaCalls, nil, nil)
	client := ollama.NewClient(ollama.WithBaseURL(server.URL))
	failingStore := &failCompleteOnceStore{Store: firstStore}
	if err := NewWorkerWithImageStore(failingStore, pubsub.NewMemoryPubSub(), client, images).ProcessLostPet(ctx, eventData); err == nil {
		t.Fatal("first ProcessLostPet() error = nil, want completion failure")
	}
	if err := NewWorkerWithImageStore(secondStore, pubsub.NewMemoryPubSub(), client, images).ProcessLostPet(ctx, eventData); err != nil {
		t.Fatalf("second ProcessLostPet() error = %v", err)
	}
	if ollamaCalls.Load() != 1 {
		t.Fatalf("Ollama calls across workers = %d, want 1", ollamaCalls.Load())
	}
	updatedData, err := secondStore.GetState(ctx, store.LostPetsCollection, record.PetID)
	if err != nil {
		t.Fatal(err)
	}
	var updated domain.LostPetRecord
	if err := json.Unmarshal(updatedData, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.ImageAnalysis == nil || updated.ImageAnalysis.SourceEventID != eventID {
		t.Fatalf("persisted image analysis = %#v", updated.ImageAnalysis)
	}
	completed, err := secondStore.GetDeliveryOperation(ctx, operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != delivery.StatusCompleted {
		t.Fatalf("lost image operation status = %q, want completed", completed.Status)
	}
}

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
	reporter := &domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "reporter-firestore-recovery",
	}
	finder := &domain.PrincipalRef{
		Issuer: "https://securetoken.google.com/petspotr-test", Subject: "finder-firestore-recovery",
	}
	seedMatcherLostPetWithOwner(t, firstStore, reporter)
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.LostPetsCollection, "lost-101")
	})

	foundEvent := domain.FoundPetReportedV2{
		PetID:    "found-firestore-recovery",
		ImageURL: "https://storage.petspotr.io/found-firestore-recovery.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	seedMatcherFoundPetWithOwner(t, firstStore, foundEvent, finder)
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.FoundPetsCollection, foundEvent.PetID)
	})
	foundData := verifiedFoundEventData(t, foundEvent)
	_, envelope, err := domain.DecodeFoundPetReported(foundData)
	if err != nil {
		t.Fatal(err)
	}
	if envelope == nil {
		t.Fatal("verified found-pet fixture omitted event envelope")
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
	if result.MatchID == "" {
		t.Fatal("persisted matcher result omitted match ID")
	}
	t.Cleanup(func() {
		_ = firstStore.DeleteState(context.Background(), store.OutboxCollection, result.OutboxID)
		_ = firstStore.DeleteState(context.Background(), store.MatchesCollection, result.MatchID)
		_ = firstStore.DeleteState(context.Background(), store.MatchParticipantsCollection, result.MatchID)
	})
	matchData, err := firstStore.GetState(ctx, store.MatchesCollection, result.MatchID)
	if err != nil {
		t.Fatalf("GetState(match record) error = %v", err)
	}
	var match domain.MatchRecord
	if err := json.Unmarshal(matchData, &match); err != nil {
		t.Fatal(err)
	}
	if err := match.Validate(); err != nil {
		t.Fatalf("persisted match validation: %v", err)
	}
	participantsData, err := secondStore.GetState(ctx, store.MatchParticipantsCollection, result.MatchID)
	if err != nil {
		t.Fatalf("GetState(match participants) error = %v", err)
	}
	var participants domain.MatchParticipantRecord
	if err := json.Unmarshal(participantsData, &participants); err != nil {
		t.Fatal(err)
	}
	if err := participants.Validate(); err != nil {
		t.Fatalf("persisted match participants validation: %v", err)
	}
	if participants.Reporter == nil || *participants.Reporter != *reporter ||
		participants.Finder == nil || *participants.Finder != *finder {
		t.Fatalf("persisted match participants = %#v", participants)
	}
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
