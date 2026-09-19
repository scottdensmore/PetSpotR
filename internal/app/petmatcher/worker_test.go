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
	"github.com/scottdensmore/petspotr/pkg/embedding"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/scoring"
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
	start := time.Now().UTC().Add(-2 * time.Hour)
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

func TestMatcherWorker_RecordsGemma4ModelProvenance(t *testing.T) {
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

	deterministicClient := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:      ollama.Gemma4Model,
		Done:       true,
		Response:   `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"Cream","distinctiveMarkings":[],"eyeColor":"Brown"}`,
		Provenance: ollama.DefaultGemma4Provenance(),
	}, nil)

	seedMatcherLostPet(t, st)
	worker := NewWorkerWithImageStore(st, ps, deterministicClient, images)
	foundData := verifiedFoundEventData(t, foundEvent)
	if err := worker.ProcessFoundPet(ctx, foundData); err != nil {
		t.Fatalf("ProcessFoundPet() error = %v", err)
	}

	matches, err := st.ListState(ctx, store.MatchesCollection)
	if err != nil {
		t.Fatalf("ListState matches error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	var record domain.MatchRecord
	for _, data := range matches {
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatalf("decode match record: %v", err)
		}
	}
	if record.Model != ollama.Gemma4Model {
		t.Errorf("record.Model = %q, want %q", record.Model, ollama.Gemma4Model)
	}
	if err := record.Validate(); err != nil {
		t.Errorf("record.Validate() error: %v", err)
	}
}

func TestWorker_HybridMultimodalMatching(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	broker := pubsub.NewMemoryPubSub()
	embedder := embedding.NewMockEmbedder()

	worker := NewWorkerWithImageStore(st, broker, nil, nil)
	worker.SetEmbedder(embedder)

	// Verify Worker computes hybrid scores with Scores.Vector populated
	now := time.Now().UTC()
	emb, _ := embedder.EmbedText(ctx, "Golden Retriever Dog")

	lostRecord := domain.LostPetRecord{
		PetID:           "lost-1",
		Species:         "Dog",
		Status:          domain.LostPetStatusLost,
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		ReportedAt:      now,
		Embedding:       emb,
		ImageObject:     "images/lost-pets/lost-1/image.jpg",
		ImageAnalysis: verifiedCandidateAnalysis("lost-1", "images/lost-pets/lost-1/image.jpg", domain.PetImageTraits{
			Breed: "Golden Retriever",
		}),
	}
	data, _ := json.Marshal(lostRecord)
	_ = st.SaveState(ctx, store.LostPetsCollection, "lost-1", data)

	foundEvt := domain.FoundPetReportedV2{
		PetID:           "found-1",
		Species:         "Dog",
		Breed:           "Golden Retriever",
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		FoundAt:         now,
		Images: []domain.PetImage{
			{Object: "images/found/1/face.jpg", Tag: domain.PetImageTagPrimary, Embedding: emb},
		},
	}

	// Verify candidate evaluation uses ComparePetsHybrid and preserves vector score
	candidates, err := worker.EligibleLostPetCandidates(ctx, foundEvt)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}
	if len(candidates[0].Embedding) != embedder.Dimension() {
		t.Fatalf("expected %d-dim candidate embedding, got %d", embedder.Dimension(), len(candidates[0].Embedding))
	}

	// Verify ComparePetsHybrid produces vector score
	res := scoring.ComparePetsHybrid(
		candidates[0].Record.PetID,
		foundEvt.PetID,
		candidates[0].Record.Species,
		foundEvt.Species,
		candidates[0].DistanceMiles,
		candidates[0].Traits,
		&scoring.PetTraits{Breed: foundEvt.Breed},
		candidates[0].Record.Embedding,
		emb,
	)
	if res == nil || !res.IsMatch {
		t.Fatalf("expected match, got %v", res)
	}
	if res.Scores.Vector <= 0 {
		t.Errorf("expected positive vector score, got %f", res.Scores.Vector)
	}
	if res.ThresholdVersion != scoring.HybridThresholdVersion {
		t.Errorf("expected threshold version %s, got %s", scoring.HybridThresholdVersion, res.ThresholdVersion)
	}
}

func TestWorker_ProcessFoundPet_HybridVectorMatching(t *testing.T) {
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
	embedder := embedding.NewMockEmbedder()
	emb, err := embedder.EmbedImage(ctx, imageBytes, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}

	foundAt := time.Now().UTC()
	foundEvent := domain.FoundPetReportedV2{
		PetID: grant.ReportID, ImageObject: finalized.ObjectName, FoundAt: foundAt,
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingVerified,
		Coordinates: matcherTestPoint(), CustodyStatus: domain.CustodyUnknown,
		Species: "Dog", Breed: "Golden Retriever",
		Status:    domain.FoundPetStatusFound,
		Embedding: emb,
		Images: []domain.PetImage{
			{Object: finalized.ObjectName, Tag: domain.PetImageTagPrimary, Embedding: emb},
		},
	}
	seedMatcherFoundPet(t, st, foundEvent)

	// Seed lost pet with embedding and images
	lostRecord := domain.LostPetRecord{
		PetID:           "lost-101",
		Species:         "Dog",
		Breed:           "Golden Retriever",
		PrimaryColor:    "Golden",
		Status:          domain.LostPetStatusLost,
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     matcherTestPoint(),
		ReportedAt:      foundAt.Add(-time.Hour),
		ImageObject:     "images/lost-pets/lost-101/image.jpg",
		Embedding:       emb,
		Images: []domain.PetImage{
			{Object: "images/lost-pets/lost-101/image.jpg", Tag: domain.PetImageTagPrimary, Embedding: emb},
		},
		ImageAnalysis: verifiedCandidateAnalysis("lost-101", "images/lost-pets/lost-101/image.jpg", domain.PetImageTraits{
			Breed: "Golden Retriever", PrimaryColor: "Golden",
		}),
	}
	data, _ := json.Marshal(lostRecord)
	_ = st.SaveState(ctx, store.LostPetsCollection, "lost-101", data)

	deterministicClient := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:      ollama.Gemma4Model,
		Done:       true,
		Response:   `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"Cream","distinctiveMarkings":[],"eyeColor":"Brown"}`,
		Provenance: ollama.DefaultGemma4Provenance(),
	}, nil)

	worker := NewWorkerWithImageStore(st, ps, deterministicClient, images)
	worker.SetEmbedder(embedder)
	foundData := verifiedFoundEventData(t, foundEvent)
	if err := worker.ProcessFoundPet(ctx, foundData); err != nil {
		t.Fatalf("ProcessFoundPet() error = %v", err)
	}

	matches, err := st.ListState(ctx, store.MatchesCollection)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	var record domain.MatchRecord
	for _, data := range matches {
		if err := json.Unmarshal(data, &record); err != nil {
			t.Fatal(err)
		}
	}
	if record.Scores.Vector <= 0 {
		t.Errorf("record.Scores.Vector = %f, want > 0", record.Scores.Vector)
	}
	if record.ThresholdVersion != scoring.HybridThresholdVersion {
		t.Errorf("record.ThresholdVersion = %q, want %q", record.ThresholdVersion, scoring.HybridThresholdVersion)
	}
	if len(record.LostPet.Images) != 1 {
		t.Errorf("record.LostPet.Images length = %d, want 1", len(record.LostPet.Images))
	}
	if len(record.FoundPet.Images) != 1 {
		t.Errorf("record.FoundPet.Images length = %d, want 1", len(record.FoundPet.Images))
	}
}

func TestWorker_ProcessLostPet_GeneratesCompositeEmbedding(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	images := blob.NewMemoryBlobStore("https://storage.petspotr.io")

	grant1, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeLostPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	imageBytes1 := encodedMatcherImage(t)
	if _, err := images.UploadImage(ctx, grant1.ObjectName, imageBytes1); err != nil {
		t.Fatal(err)
	}
	finalized1, err := images.FinalizeImage(ctx, grant1.ReportID, grant1.ObjectName, grant1.FinalizeToken)
	if err != nil {
		t.Fatal(err)
	}

	grant2, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeLostPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	imageBytes2 := encodedMatcherImage(t)
	if _, err := images.UploadImage(ctx, grant2.ObjectName, imageBytes2); err != nil {
		t.Fatal(err)
	}
	finalized2, err := images.FinalizeImage(ctx, grant2.ReportID, grant2.ObjectName, grant2.FinalizeToken)
	if err != nil {
		t.Fatal(err)
	}

	reportedAt := time.Date(2026, time.August, 17, 17, 0, 0, 0, time.UTC)
	petID := grant1.ReportID
	record := domain.LostPetRecord{
		PetID:            petID,
		Species:          "Dog",
		Breed:            "Golden Retriever",
		OwnerIdentityRef: "identity-" + petID,
		ImageObject:      finalized1.ObjectName,
		ReportedAt:       reportedAt,
		Location:         "Seattle, WA",
		GeocodingStatus:  domain.GeocodingVerified,
		Coordinates:      matcherTestPoint(),
		Status:           domain.LostPetStatusLost,
		Images: []domain.PetImage{
			{Object: finalized1.ObjectName, Tag: domain.PetImageTagPrimary},
			{Object: finalized2.ObjectName, Tag: domain.PetImageTagFace},
		},
	}
	stateData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.LostPetsCollection, record.PetID, stateData); err != nil {
		t.Fatal(err)
	}

	deterministicClient := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:      ollama.Gemma4Model,
		Done:       true,
		Response:   `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"Cream","distinctiveMarkings":["White patch"],"eyeColor":"Brown"}`,
		Provenance: ollama.DefaultGemma4Provenance(),
	}, nil)

	embedder := embedding.NewMockEmbedder()
	worker := NewWorkerWithImageStore(
		stateStore,
		pubsub.NewMemoryPubSub(),
		deterministicClient,
		images,
	)
	worker.SetEmbedder(embedder)
	worker.now = func() time.Time { return reportedAt.Add(time.Minute) }

	eventData, _ := encodeLostAnalysisEvent(t, domain.LostPetReportedV4{
		PetID:           record.PetID,
		Species:         record.Species,
		Breed:           record.Breed,
		ImageObject:     record.ImageObject,
		Images:          record.Images,
		ReportedAt:      record.ReportedAt,
		Location:        record.Location,
		GeocodingStatus: record.GeocodingStatus,
		Coordinates:     record.Coordinates,
		Status:          record.Status,
	})

	if err := worker.ProcessLostPet(ctx, eventData); err != nil {
		t.Fatalf("ProcessLostPet() error = %v", err)
	}

	updatedData, err := stateStore.GetState(ctx, store.LostPetsCollection, record.PetID)
	if err != nil {
		t.Fatal(err)
	}
	var updated domain.LostPetRecord
	if err := json.Unmarshal(updatedData, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.ImageAnalysis == nil || updated.ImageAnalysis.Status != domain.ImageTraitsVerified {
		t.Fatalf("expected verified image analysis, got %#v", updated.ImageAnalysis)
	}
	if len(updated.Embedding) != embedder.Dimension() {
		t.Fatalf("expected %d-dim composite embedding on record, got %d", embedder.Dimension(), len(updated.Embedding))
	}
	if err := domain.ValidateEmbedding(updated.Embedding); err != nil {
		t.Fatalf("composite embedding validation failed: %v", err)
	}
	if len(updated.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(updated.Images))
	}
	for i, img := range updated.Images {
		if len(img.Embedding) != embedder.Dimension() {
			t.Fatalf("image[%d] (%s) embedding missing: got %d, want %d", i, img.Tag, len(img.Embedding), embedder.Dimension())
		}
		if err := domain.ValidateEmbedding(img.Embedding); err != nil {
			t.Fatalf("image[%d] embedding validation failed: %v", i, err)
		}
	}
}

func TestWorker_ProcessFoundPet_GeneratesCompositeEmbedding(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	var matchPublished atomic.Bool
	if err := ps.Subscribe("matchFound", func(context.Context, []byte) error {
		matchPublished.Store(true)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	images := blob.NewMemoryBlobStore("https://storage.petspotr.io")
	grant1, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeFoundPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	imageBytes1 := encodedMatcherImage(t)
	if _, err := images.UploadImage(ctx, grant1.ObjectName, imageBytes1); err != nil {
		t.Fatal(err)
	}
	finalized1, err := images.FinalizeImage(ctx, grant1.ReportID, grant1.ObjectName, grant1.FinalizeToken)
	if err != nil {
		t.Fatal(err)
	}

	grant2, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeFoundPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	imageBytes2 := encodedMatcherImage(t)
	if _, err := images.UploadImage(ctx, grant2.ObjectName, imageBytes2); err != nil {
		t.Fatal(err)
	}
	finalized2, err := images.FinalizeImage(ctx, grant2.ReportID, grant2.ObjectName, grant2.FinalizeToken)
	if err != nil {
		t.Fatal(err)
	}

	foundAt := time.Now().UTC()
	petID := grant1.ReportID
	embedder := embedding.NewMockEmbedder()

	// Seed found pet record in durable state before processing
	foundRecord := domain.FoundPetRecord{
		PetID:           petID,
		ImageObject:     finalized1.ObjectName,
		FoundAt:         foundAt,
		Location:        "Seattle, WA",
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     matcherTestPoint(),
		Species:         "Dog",
		Breed:           "Golden Retriever",
		Status:          domain.FoundPetStatusFound,
		CustodyStatus:   domain.CustodyFinderHome,
		Images: []domain.PetImage{
			{Object: finalized1.ObjectName, Tag: domain.PetImageTagPrimary},
			{Object: finalized2.ObjectName, Tag: domain.PetImageTagCoat},
		},
	}
	foundData, err := json.Marshal(foundRecord)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.FoundPetsCollection, petID, foundData); err != nil {
		t.Fatal(err)
	}

	// Also seed matching lost pet candidate so matching completes
	lostEmb, _ := embedder.EmbedMultimodal(ctx, imageBytes1, "image/jpeg", "Dog Golden Retriever")
	lostRecord := domain.LostPetRecord{
		PetID:           "lost-match-target",
		Species:         "Dog",
		Breed:           "Golden Retriever",
		PrimaryColor:    "Golden",
		Status:          domain.LostPetStatusLost,
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     matcherTestPoint(),
		ReportedAt:      foundAt.Add(-time.Hour),
		ImageObject:     "images/lost-pets/lost-match-target/image.jpg",
		Embedding:       lostEmb,
		Images: []domain.PetImage{
			{Object: "images/lost-pets/lost-match-target/image.jpg", Tag: domain.PetImageTagPrimary, Embedding: lostEmb},
		},
		ImageAnalysis: verifiedCandidateAnalysis("lost-match-target", "images/lost-pets/lost-match-target/image.jpg", domain.PetImageTraits{
			Breed: "Golden Retriever", PrimaryColor: "Golden",
		}),
	}
	seedCandidateRecord(t, stateStore, lostRecord)

	deterministicClient := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:      ollama.Gemma4Model,
		Done:       true,
		Response:   `{"breed":"Golden Retriever","primaryColor":"Golden","secondaryColor":"Cream","distinctiveMarkings":[],"eyeColor":"Brown"}`,
		Provenance: ollama.DefaultGemma4Provenance(),
	}, nil)

	worker := NewWorkerWithImageStore(stateStore, ps, deterministicClient, images)
	worker.SetEmbedder(embedder)
	worker.now = func() time.Time { return foundAt.Add(time.Minute) }

	foundEvent := domain.FoundPetReportedV2{
		PetID:           petID,
		ImageObject:     finalized1.ObjectName,
		FoundAt:         foundAt,
		Location:        "Seattle, WA",
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     matcherTestPoint(),
		Species:         "Dog",
		Breed:           "Golden Retriever",
		Status:          domain.FoundPetStatusFound,
		CustodyStatus:   domain.CustodyFinderHome,
		Images: []domain.PetImage{
			{Object: finalized1.ObjectName, Tag: domain.PetImageTagPrimary},
			{Object: finalized2.ObjectName, Tag: domain.PetImageTagCoat},
		},
	}
	eventBytes := verifiedFoundEventData(t, foundEvent)
	if err := worker.ProcessFoundPet(ctx, eventBytes); err != nil {
		t.Fatalf("ProcessFoundPet() error = %v", err)
	}

	updatedFoundData, err := stateStore.GetState(ctx, store.FoundPetsCollection, petID)
	if err != nil {
		t.Fatal(err)
	}
	var updated domain.FoundPetRecord
	if err := json.Unmarshal(updatedFoundData, &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Embedding) != embedder.Dimension() {
		t.Fatalf("expected %d-dim composite embedding on found record, got %d", embedder.Dimension(), len(updated.Embedding))
	}
	if err := domain.ValidateEmbedding(updated.Embedding); err != nil {
		t.Fatalf("found composite embedding validation failed: %v", err)
	}
	if len(updated.Images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(updated.Images))
	}
	for i, img := range updated.Images {
		if len(img.Embedding) != embedder.Dimension() {
			t.Fatalf("image[%d] (%s) embedding missing: got %d, want %d", i, img.Tag, len(img.Embedding), embedder.Dimension())
		}
		if err := domain.ValidateEmbedding(img.Embedding); err != nil {
			t.Fatalf("image[%d] embedding validation failed: %v", i, err)
		}
	}
	if !matchPublished.Load() {
		t.Fatal("expected matchFound to be published")
	}
}

func TestWorker_ResolveFoundEmbedding_PersistsComputedEmbedding(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
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

	petID := grant.ReportID
	initialRecord := domain.FoundPetRecord{
		PetID:           petID,
		ImageObject:     finalized.ObjectName,
		FoundAt:         time.Now().UTC(),
		Location:        "Seattle, WA",
		GeocodingStatus: domain.GeocodingVerified,
		Coordinates:     matcherTestPoint(),
		Species:         "Dog",
		Breed:           "Golden Retriever",
		Status:          domain.FoundPetStatusFound,
		CustodyStatus:   domain.CustodyFinderHome,
		Images: []domain.PetImage{
			{Object: finalized.ObjectName, Tag: domain.PetImageTagPrimary},
		},
	}
	data, err := json.Marshal(initialRecord)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.FoundPetsCollection, petID, data); err != nil {
		t.Fatal(err)
	}

	embedder := embedding.NewMockEmbedder()
	worker := NewWorkerWithImageStore(stateStore, pubsub.NewMemoryPubSub(), nil, images)
	worker.SetEmbedder(embedder)

	foundEvt := domain.FoundPetReportedV2{
		PetID:       petID,
		ImageObject: finalized.ObjectName,
		Species:     "Dog",
		Breed:       "Golden Retriever",
	}

	emb := worker.resolveFoundEmbedding(ctx, foundEvt)
	if len(emb) != embedder.Dimension() {
		t.Fatalf("resolveFoundEmbedding() length = %d, want %d", len(emb), embedder.Dimension())
	}

	storedBytes, err := stateStore.GetState(ctx, store.FoundPetsCollection, petID)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	var storedRecord domain.FoundPetRecord
	if err := json.Unmarshal(storedBytes, &storedRecord); err != nil {
		t.Fatalf("unmarshal stored record: %v", err)
	}
	if len(storedRecord.Embedding) != embedder.Dimension() {
		t.Fatalf("stored record Embedding length = %d, want %d", len(storedRecord.Embedding), embedder.Dimension())
	}
	if len(storedRecord.Images) == 0 || len(storedRecord.Images[0].Embedding) != embedder.Dimension() {
		t.Fatalf("stored record Images[0] Embedding missing or wrong length")
	}

	workerWithoutEmbedder := NewWorkerWithImageStore(stateStore, pubsub.NewMemoryPubSub(), nil, nil)
	secondEmb := workerWithoutEmbedder.resolveFoundEmbedding(ctx, domain.FoundPetReportedV2{PetID: petID})
	if len(secondEmb) != embedder.Dimension() {
		t.Fatalf("second resolveFoundEmbedding length = %d, want %d", len(secondEmb), embedder.Dimension())
	}
}
