package petmatcher

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/delivery"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestMatcherWorker_ReadsPrivateFoundPetImage(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	if err := ps.Subscribe("matchFound", func(context.Context, []byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	images := blob.NewMemoryBlobStore("https://storage.petspotr.io")
	grant, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeFoundPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	imageBytes := encodedMatcherImage(t)
	if _, err := images.UploadImage(ctx, grant.ObjectName, imageBytes); err != nil {
		t.Fatal(err)
	}
	finalized, err := images.FinalizeImage(ctx, grant.ReportID, grant.ObjectName, grant.FinalizeToken)
	if err != nil {
		t.Fatal(err)
	}
	foundAt := time.Now().UTC()
	foundEvent := domain.FoundPetReportedV2{
		PetID: grant.ReportID, ImageObject: finalized.ObjectName, FoundAt: foundAt,
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: matcherTestPoint(), CustodyStatus: domain.CustodyUnknown,
		Status: domain.FoundPetStatusFound,
	}
	seedMatcherFoundPet(t, st, foundEvent)

	var received ollama.GenerateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode Ollama request: %v", err)
		}
		_ = json.NewEncoder(w).Encode(ollama.GenerateResponse{
			Model: "gemma4:e2b", Done: true,
			Response: `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"Cream","distinctiveMarkings":[],"eyeColor":"Brown"}`,
		})
	}))
	defer server.Close()

	seedMatcherLostPet(t, st)
	worker := NewWorkerWithImageStore(
		st,
		ps,
		ollama.NewClient(ollama.WithBaseURL(server.URL)),
		images,
	)
	foundData := verifiedFoundEventData(t, foundEvent)
	if err := worker.ProcessFoundPet(ctx, foundData); err != nil {
		t.Fatalf("ProcessFoundPet() error = %v", err)
	}
	if len(received.Images) != 1 || received.Images[0] != base64.StdEncoding.EncodeToString(imageBytes) {
		t.Fatalf("Ollama images = %#v, want base64-encoded private object", received.Images)
	}
	foundState, err := st.GetState(ctx, store.FoundPetsCollection, grant.ReportID)
	if err != nil {
		t.Fatal(err)
	}
	var persisted domain.FoundPetRecord
	if err := json.Unmarshal(foundState, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.ImageAnalysis == nil || persisted.ImageAnalysis.SourceImageObject != finalized.ObjectName ||
		persisted.ImageAnalysis.Status != domain.ImageTraitsVerified {
		t.Fatalf("persisted found image analysis = %#v", persisted.ImageAnalysis)
	}
}

func TestMatcherWorkerPrivateFoundAnalysisSurvivesCompletionRetryWithoutMatch(t *testing.T) {
	ctx := context.Background()
	baseStore := store.NewMemoryStore()
	failingStore := &failCompleteOnceStore{Store: baseStore}
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
	seedMatcherFoundPet(t, baseStore, foundEvent)
	seedMatcherLostPet(t, baseStore)

	var ollamaCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		ollamaCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"model":"gemma4:e2b","response":"{\"breed\":\"Poodle\",\"primaryColor\":\"Black\",\"secondaryColor\":\"Gray\",\"distinctiveMarkings\":[\"Black ear\"],\"eyeColor\":\"Blue\"}","done":true}`)
	}))
	t.Cleanup(server.Close)
	client := ollama.NewClient(ollama.WithBaseURL(server.URL))
	foundData := verifiedFoundEventData(t, foundEvent)

	if err := NewWorkerWithImageStore(failingStore, pubsub.NewMemoryPubSub(), client, images).
		ProcessFoundPet(ctx, foundData); err == nil {
		t.Fatal("first ProcessFoundPet() error = nil, want completion failure")
	}
	if err := NewWorkerWithImageStore(baseStore, pubsub.NewMemoryPubSub(), client, images).
		ProcessFoundPet(ctx, foundData); err != nil {
		t.Fatalf("second ProcessFoundPet() error = %v", err)
	}
	if got := ollamaCalls.Load(); got != 1 {
		t.Fatalf("Ollama calls across completion retry = %d, want 1", got)
	}
	assertCompletedMatcherOperation(t, baseStore, foundData, foundEvent.PetID)
	foundState, err := baseStore.GetState(ctx, store.FoundPetsCollection, foundEvent.PetID)
	if err != nil {
		t.Fatal(err)
	}
	var persisted domain.FoundPetRecord
	if err := json.Unmarshal(foundState, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.ImageAnalysis == nil || persisted.ImageAnalysis.SourceImageObject != finalized.ObjectName {
		t.Fatalf("persisted found image analysis = %#v", persisted.ImageAnalysis)
	}
}

func TestMatcherWorkerReclaimedAttemptScoresCommittedFoundAnalysis(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
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
	start := time.Date(2026, time.August, 17, 19, 0, 0, 0, time.UTC)
	foundEvent := domain.FoundPetReportedV2{
		PetID: grant.ReportID, ImageObject: finalized.ObjectName, FoundAt: start,
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: matcherTestPoint(), Species: "Dog", CustodyStatus: domain.CustodyFinderHome,
		Status: domain.FoundPetStatusFound,
	}
	seedMatcherFoundPet(t, stateStore, foundEvent)
	seedMatcherLostPet(t, stateStore)
	foundData := verifiedFoundEventData(t, foundEvent)

	firstEntered := make(chan struct{}, 1)
	releaseFirst := make(chan struct{})
	firstServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		firstEntered <- struct{}{}
		<-releaseFirst
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"model":"first-model","response":"{\"breed\":\"Golden Retriever\",\"primaryColor\":\"Golden\",\"distinctiveMarkings\":[\"White chest patch\"]}","done":true}`)
	}))
	t.Cleanup(firstServer.Close)
	secondServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"model":"second-model","response":"{\"breed\":\"Poodle\",\"primaryColor\":\"Black\",\"distinctiveMarkings\":[\"Black ear\"]}","done":true}`)
	}))
	t.Cleanup(secondServer.Close)

	var currentTime atomic.Int64
	currentTime.Store(start.UnixNano())
	now := func() time.Time { return time.Unix(0, currentTime.Load()).UTC() }
	broker := pubsub.NewMemoryPubSub()
	var publications atomic.Int32
	if err := broker.Subscribe("matchFound", func(context.Context, []byte) error {
		publications.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	firstWorker := NewWorkerWithImageStore(
		stateStore, broker, ollama.NewClient(ollama.WithBaseURL(firstServer.URL)), images,
	)
	firstWorker.now = now
	secondWorker := NewWorkerWithImageStore(
		stateStore, broker, ollama.NewClient(ollama.WithBaseURL(secondServer.URL)), images,
	)
	secondWorker.now = now

	firstResult := make(chan error, 1)
	go func() { firstResult <- firstWorker.ProcessFoundPet(ctx, foundData) }()
	select {
	case <-firstEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("first attempt did not reach the model")
	}
	currentTime.Store(start.Add(defaultMatcherLease + time.Minute).UnixNano())
	if err := secondWorker.ProcessFoundPet(ctx, foundData); err != nil {
		t.Fatalf("reclaimed ProcessFoundPet() error = %v", err)
	}
	close(releaseFirst)
	if err := <-firstResult; err != nil {
		t.Fatalf("stale ProcessFoundPet() error = %v", err)
	}

	if got := publications.Load(); got != 0 {
		t.Fatalf("matchFound publications = %d, want 0 from committed non-match analysis", got)
	}
	matches, err := stateStore.ListState(ctx, store.MatchesCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("persisted matches = %d, want 0 from committed non-match analysis", len(matches))
	}
	foundState, err := stateStore.GetState(ctx, store.FoundPetsCollection, foundEvent.PetID)
	if err != nil {
		t.Fatal(err)
	}
	var persisted domain.FoundPetRecord
	if err := json.Unmarshal(foundState, &persisted); err != nil {
		t.Fatal(err)
	}
	if persisted.ImageAnalysis == nil || persisted.ImageAnalysis.Model != "second-model" ||
		persisted.ImageAnalysis.Traits.Breed != "Poodle" {
		t.Fatalf("committed found analysis = %#v", persisted.ImageAnalysis)
	}
}

func TestMatcherWorkerRejectsInvalidPrivateFoundImageProvenance(t *testing.T) {
	now := time.Now().UTC()
	tests := []struct {
		name        string
		foundEvent  domain.FoundPetReportedV2
		seedDurable bool
	}{
		{
			name: "cross-purpose object",
			foundEvent: domain.FoundPetReportedV2{
				PetID: "found-cross-purpose", ImageObject: "images/lost-pets/found-cross-purpose/image.jpg",
			},
		},
		{
			name: "durable object mismatch",
			foundEvent: domain.FoundPetReportedV2{
				PetID: "found-object-mismatch", ImageObject: "images/found-pets/found-object-mismatch/image.jpg",
			},
			seedDurable: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateStore := store.NewMemoryStore()
			seedMatcherLostPet(t, stateStore)
			test.foundEvent.FoundAt = now
			test.foundEvent.Location = "Seattle, WA"
			test.foundEvent.GeocodingStatus = domain.GeocodingVerified
			test.foundEvent.Coordinates = matcherTestPoint()
			test.foundEvent.CustodyStatus = domain.CustodyFinderHome
			test.foundEvent.Status = domain.FoundPetStatusFound
			if test.seedDurable {
				durable := test.foundEvent
				durable.ImageObject = "images/found-pets/found-object-mismatch/other.jpg"
				seedMatcherFoundPet(t, stateStore, durable)
			}
			worker := NewWorker(stateStore, pubsub.NewMemoryPubSub(), nil)
			if err := worker.ProcessFoundPet(context.Background(), verifiedFoundEventData(t, test.foundEvent)); err == nil {
				t.Fatal("ProcessFoundPet() error = nil, want invalid private provenance")
			}
		})
	}
}

func encodedMatcherImage(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := jpeg.Encode(&output, image.NewRGBA(image.Rect(0, 0, 2, 3)), nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return output.Bytes()
}

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
	Store
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
	return s.Store.CompleteDeliveryOperation(ctx, id, attempt, completedAt)
}

type getStateFailingStore struct {
	Store
	err error
}

func (s *getStateFailingStore) GetState(context.Context, string, string) ([]byte, error) {
	return nil, s.err
}

func (s *getStateFailingStore) ListState(context.Context, string) (map[string][]byte, error) {
	return nil, s.err
}

func (s *getStateFailingStore) QueryLostPetCandidates(
	context.Context,
	store.LostPetCandidateQuery,
) (map[string][]byte, error) {
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

	seedMatcherLostPet(t, st)

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
		foundEvt := domain.FoundPetReportedV2{
			PetID:    "found-202",
			ImageURL: "https://storage.petspotr.io/found-202.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Seattle, WA",
		}
		foundData := verifiedFoundEventData(t, foundEvt)

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
		matches, err := st.ListState(context.Background(), store.MatchesCollection)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("persisted match count = %d, want 1", len(matches))
		}
		participants, err := st.ListState(context.Background(), store.MatchParticipantsCollection)
		if err != nil {
			t.Fatal(err)
		}
		if len(participants) != 0 {
			t.Fatalf("legacy ownerless match participants = %d, want 0", len(participants))
		}
		var persisted domain.MatchRecord
		for _, data := range matches {
			if err := json.Unmarshal(data, &persisted); err != nil {
				t.Fatalf("decode persisted match: %v", err)
			}
		}
		if err := persisted.Validate(); err != nil {
			t.Fatalf("persisted match validation: %v", err)
		}
		if persisted.MatchID == "" || persisted.MatchID != matchFoundEvent.MatchID ||
			persisted.FoundPetID != "found-202" || persisted.MatchedPetID != "lost-101" ||
			persisted.Status != domain.MatchStatusPendingReview || persisted.SourceEventID == "" ||
			persisted.Model != "gemma2:2b" || persisted.Model != matchFoundEvent.Model ||
			persisted.ThresholdVersion == "" || persisted.ThresholdVersion != matchFoundEvent.ThresholdVersion ||
			persisted.MatchedAt.IsZero() ||
			persisted.Scores.Visual <= 0 || persisted.Scores.Spatial <= 0 || persisted.Explanation == "" {
			t.Fatalf("persisted match = %#v; event = %#v", persisted, matchFoundEvent)
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
		matches, err = st.ListState(context.Background(), store.MatchesCollection)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("persisted match count after duplicate = %d, want 1", len(matches))
		}
	})

	t.Run("versioned found pet envelope remains consumable", func(t *testing.T) {
		publicationsBefore := matchPublications.Load()
		ollamaCallsBefore := ollamaCalls.Load()
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
		if got := matchPublications.Load(); got != publicationsBefore {
			t.Fatalf("matchFound publications = %d, want unchanged %d for unverified legacy event", got, publicationsBefore)
		}
		if got := ollamaCalls.Load(); got != ollamaCallsBefore {
			t.Fatalf("Ollama calls = %d, want unchanged %d for unverified legacy event", got, ollamaCallsBefore)
		}
	})

	t.Run("canonical payload-v2 rejects invalid report state", func(t *testing.T) {
		foundEvt := domain.FoundPetReportedV2{
			PetID:           "found-invalid-state",
			ImageURL:        "https://storage.petspotr.io/found-invalid-state.jpg",
			FoundAt:         time.Now().UTC(),
			Location:        "Seattle, WA",
			GeocodingStatus: domain.GeocodingPending,
			CustodyStatus:   domain.CustodyUnknown,
			Status:          domain.FoundPetStatus("reunited"),
		}
		payload, err := json.Marshal(foundEvt)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeFoundPetReported,
			OccurredAt:       foundEvt.FoundAt,
			AggregateID:      foundEvt.PetID,
			AggregateVersion: 1,
			PayloadVersion:   domain.FoundPetReportedPayloadVersion,
			Payload:          payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		if err := worker.ProcessFoundPet(context.Background(), data); err == nil {
			t.Fatal("ProcessFoundPet(payload-v2) error = nil, want invalid state error")
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

		foundEvt := domain.FoundPetReportedV2{
			PetID:    "found-203",
			ImageURL: "https://storage.petspotr.io/found-203.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Seattle, WA",
		}
		data := verifiedFoundEventData(t, foundEvt)
		err := emptyWorker.ProcessFoundPet(context.Background(), data)
		if err != nil {
			t.Errorf("expected nil error when no candidate available, got %v", err)
		}
	})

	t.Run("transient candidate store failure requests redelivery", func(t *testing.T) {
		transientErr := errors.New("firestore unavailable")
		failingStore := &getStateFailingStore{Store: st, err: transientErr}
		failingWorker := NewWorker(failingStore, ps, ollamaClient)
		foundEvt := domain.FoundPetReportedV2{
			PetID:    "found-store-error",
			ImageURL: "https://storage.petspotr.io/found-store-error.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Seattle, WA",
		}
		data := verifiedFoundEventData(t, foundEvt)
		if err := failingWorker.ProcessFoundPet(context.Background(), data); !errors.Is(err, transientErr) {
			t.Fatalf("ProcessFoundPet() error = %v, want transient store error", err)
		}
	})

	t.Run("invalid candidate state is isolated from other reports", func(t *testing.T) {
		invalidStore := store.NewMemoryStore()
		if err := invalidStore.SaveState(context.Background(), store.LostPetsCollection, "lost-101", []byte(`{invalid`)); err != nil {
			t.Fatal(err)
		}
		invalidWorker := NewWorker(invalidStore, ps, ollamaClient)
		foundEvt := domain.FoundPetReportedV2{
			PetID:    "found-invalid-candidate",
			ImageURL: "https://storage.petspotr.io/found-invalid-candidate.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Seattle, WA",
		}
		data := verifiedFoundEventData(t, foundEvt)
		if err := invalidWorker.ProcessFoundPet(context.Background(), data); err != nil {
			t.Fatalf("ProcessFoundPet() error = %v, want malformed candidate to be skipped", err)
		}
	})

	t.Run("ollama error returned when generate fails", func(t *testing.T) {
		badOllama := ollama.NewClient(ollama.WithBaseURL("http://invalid-host-12345"))
		badWorker := NewWorker(st, ps, badOllama)

		foundEvt := domain.FoundPetReportedV2{
			PetID:    "found-204",
			ImageURL: "https://storage.petspotr.io/found-204.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Seattle, WA",
		}
		data := verifiedFoundEventData(t, foundEvt)
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

	t.Run("Start registers foundPet and lostPet subscriptions successfully", func(t *testing.T) {
		ctx := context.Background()
		if err := worker.Start(ctx); err != nil {
			t.Fatalf("worker.Start failed: %v", err)
		}
		data, eventID := encodeLostAnalysisEvent(t, domain.LostPetReportedV4{
			PetID: "lost-in-process-no-image", ReportedAt: time.Now().UTC(),
			Location: "Seattle, WA", GeocodingStatus: domain.GeocodingPending,
			Status: domain.LostPetStatusLost,
		})
		if err := ps.Publish(ctx, "lostPet", data); err != nil {
			t.Fatalf("publish lostPet after Start: %v", err)
		}
		assertCompletedLostAnalysisOperation(t, st, eventID, "lost-in-process-no-image")
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

	seedMatcherLostPet(t, st)

	var published atomic.Int32
	if err := ps.Subscribe("matchFound", func(context.Context, []byte) error {
		published.Add(1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	worker := NewWorker(st, publisher, ollama.NewClient(ollama.WithBaseURL(server.URL)))
	foundEvent := domain.FoundPetReportedV2{
		PetID:    "found-publish-retry",
		ImageURL: "https://storage.petspotr.io/found-publish-retry.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	foundData := verifiedFoundEventData(t, foundEvent)

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
	st := &failCompleteOnceStore{Store: baseStore}
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
	foundEvent := domain.FoundPetReportedV2{
		PetID:    "found-completion-retry",
		ImageURL: "https://storage.petspotr.io/found-completion-retry.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	foundData := verifiedFoundEventData(t, foundEvent)

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
	foundEvent := domain.FoundPetReportedV2{
		PetID:    "found-concurrent",
		ImageURL: "https://storage.petspotr.io/found-concurrent.jpg",
		FoundAt:  time.Now().UTC(),
		Location: "Seattle, WA",
	}
	foundData := verifiedFoundEventData(t, foundEvent)

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
		foundEvent := domain.FoundPetReportedV2{
			PetID:           "found-same-score",
			ImageURL:        "https://storage.petspotr.io/found-same-score.jpg",
			FoundAt:         baseTime.Add(time.Duration(index) * time.Second),
			Location:        "Seattle, WA",
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     matcherTestPoint(),
			CustodyStatus:   domain.CustodyFinderHome,
			Status:          domain.FoundPetStatusFound,
		}
		payload, err := json.Marshal(foundEvent)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeFoundPetReported,
			OccurredAt:       foundEvent.FoundAt,
			AggregateID:      foundEvent.PetID,
			AggregateVersion: int64(index + 1),
			PayloadVersion:   domain.FoundPetReportedPayloadVersion,
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
	seedMatcherLostPetWithOwner(t, st, nil)
}

func seedMatcherLostPetWithOwner(t *testing.T, st store.StateStore, ownedBy *domain.PrincipalRef) {
	t.Helper()
	record := domain.LostPetRecord{
		PetID:            "lost-101",
		PetName:          "Buddy",
		Species:          "Dog",
		Breed:            "Golden Retriever",
		PrimaryColor:     "Golden",
		Description:      "White chest patch",
		OwnerIdentityRef: "identity-lost-101",
		ReportedAt:       time.Now().UTC(),
		Location:         "Seattle, WA",
		GeocodingStatus:  domain.GeocodingVerified,
		Coordinates:      matcherTestPoint(),
		Status:           domain.LostPetStatusLost,
		OwnedBy:          ownedBy,
	}
	record.ImageObject = "images/lost-pets/" + record.PetID + "/image.jpg"
	record.ImageAnalysis = verifiedCandidateAnalysis(record.PetID, record.ImageObject, domain.PetImageTraits{
		Breed: record.Breed, PrimaryColor: record.PrimaryColor,
		DistinctiveMarkings: []string{record.Description},
	})
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveState(context.Background(), store.LostPetsCollection, record.PetID, data); err != nil {
		t.Fatal(err)
	}
}

func seedMatcherFoundPet(t *testing.T, st store.StateStore, event domain.FoundPetReportedV2) {
	seedMatcherFoundPetWithOwner(t, st, event, nil)
}

func seedMatcherFoundPetWithOwner(
	t *testing.T,
	st store.StateStore,
	event domain.FoundPetReportedV2,
	ownedBy *domain.PrincipalRef,
) {
	t.Helper()
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID: event.PetID, ImageURL: event.ImageURL, ImageObject: event.ImageObject,
		FoundAt: event.FoundAt, Location: event.Location,
		GeocodingStatus: event.GeocodingStatus, Coordinates: event.Coordinates,
		Species: event.Species, Breed: event.Breed, PrimaryColor: event.PrimaryColor,
		SecondaryColor: event.SecondaryColor, DistinctiveMarkings: event.DistinctiveMarkings,
		CustodyStatus: event.CustodyStatus, Status: event.Status, OwnedBy: ownedBy,
	})
	record, _ := report.Persisted()
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveState(context.Background(), store.FoundPetsCollection, record.PetID, data); err != nil {
		t.Fatal(err)
	}
}

func verifiedFoundEventData(t *testing.T, event domain.FoundPetReportedV2) []byte {
	t.Helper()
	if event.FoundAt.IsZero() {
		event.FoundAt = time.Now().UTC()
	}
	if event.Location == "" {
		event.Location = "Seattle, WA"
	}
	event.GeocodingStatus = domain.GeocodingVerified
	event.Coordinates = matcherTestPoint()
	if event.CustodyStatus == "" {
		event.CustodyStatus = domain.CustodyUnknown
	}
	if event.Status == "" {
		event.Status = domain.FoundPetStatusFound
	}
	return encodeFoundCandidateEvent(t, event)
}

func matcherTestPoint() *domain.LocationPoint {
	return &domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200}
}

func assertCompletedMatcherOperation(t *testing.T, st store.DeliveryOperationStore, input []byte, petID string) {
	t.Helper()
	_, envelope, err := domain.DecodeFoundPetReported(input)
	if err != nil {
		t.Fatal(err)
	}
	envelopeID := ""
	if envelope != nil {
		envelopeID = envelope.ID
	}
	eventID, err := delivery.ResolveEventID(envelopeID, domain.EventTypeFoundPetReported, input)
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
