package petmatcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestMatcherWorkerAnalyzesPrivateLostPetImageAndPersistsVerifiedTraits(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	images := blob.NewMemoryBlobStore("https://storage.petspotr.invalid")
	grant, err := images.BeginImageUpload(ctx, blob.ImageUploadIntent{
		Purpose: blob.ImagePurposeLostPet, ContentType: "image/jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	imageBytes := encodedMatcherImage(t)
	if _, err := images.UploadImage(ctx, grant.ObjectName, imageBytes); err != nil {
		t.Fatal(err)
	}
	finalized, err := images.FinalizeImageForPurpose(
		ctx,
		blob.ImagePurposeLostPet,
		grant.ReportID,
		grant.ObjectName,
		grant.FinalizeToken,
	)
	if err != nil {
		t.Fatal(err)
	}
	reportedAt := time.Date(2026, time.August, 17, 17, 0, 0, 0, time.UTC)
	record := domain.LostPetRecord{
		PetID: grant.ReportID, Species: "Dog", Breed: "Retriever",
		OwnerIdentityRef: "identity-" + grant.ReportID,
		ImageObject:      finalized.ObjectName, ReportedAt: reportedAt,
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingPending,
		Status: domain.LostPetStatusLost,
	}
	stateData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.LostPetsCollection, record.PetID, stateData); err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	var received ollama.GenerateRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode Ollama request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"model":"gemma4:e2b","response":"{\"breed\":\"Golden Retriever\",\"primaryColor\":\"Golden\",\"secondaryColor\":\"Cream\",\"distinctiveMarkings\":[\"White chest patch\"],\"eyeColor\":\"Brown\"}","done":true}`)
	}))
	t.Cleanup(server.Close)

	verifiedAt := reportedAt.Add(2 * time.Minute)
	worker := NewWorkerWithImageStore(
		stateStore,
		pubsub.NewMemoryPubSub(),
		ollama.NewClient(ollama.WithBaseURL(server.URL)),
		images,
	)
	worker.now = func() time.Time { return verifiedAt }
	eventData, eventID := encodeLostAnalysisEvent(t, domain.LostPetReportedV4{
		PetID: record.PetID, Species: record.Species, Breed: record.Breed,
		ImageObject: record.ImageObject, ReportedAt: record.ReportedAt,
		Location: record.Location, GeocodingStatus: record.GeocodingStatus,
		Status: record.Status,
	})

	if err := worker.ProcessLostPet(ctx, eventData); err != nil {
		t.Fatalf("ProcessLostPet() error = %v", err)
	}
	if len(received.Images) != 1 || received.Images[0] != base64.StdEncoding.EncodeToString(imageBytes) {
		t.Fatalf("Ollama images = %#v, want base64 private image", received.Images)
	}
	if received.Model != "gemma4:e2b" {
		t.Fatalf("Ollama model = %q, want gemma4:e2b", received.Model)
	}

	updatedData, err := stateStore.GetState(ctx, store.LostPetsCollection, record.PetID)
	if err != nil {
		t.Fatal(err)
	}
	var updated domain.LostPetRecord
	if err := json.Unmarshal(updatedData, &updated); err != nil {
		t.Fatal(err)
	}
	analysis := updated.ImageAnalysis
	if analysis == nil || analysis.Status != domain.ImageTraitsVerified {
		t.Fatalf("image analysis = %#v, want verified", analysis)
	}
	if analysis.Traits.Breed != "Golden Retriever" || analysis.Traits.PrimaryColor != "Golden" ||
		analysis.Traits.SecondaryColor != "Cream" || analysis.Traits.EyeColor != "Brown" ||
		len(analysis.Traits.DistinctiveMarkings) != 1 || analysis.Traits.DistinctiveMarkings[0] != "White chest patch" {
		t.Fatalf("verified traits = %#v", analysis.Traits)
	}
	if analysis.Model != "gemma4:e2b" || analysis.AnalysisVersion != imageTraitAnalysisVersion ||
		analysis.SourceEventID != eventID || analysis.SourceImageObject != finalized.ObjectName ||
		!analysis.VerifiedAt.Equal(verifiedAt) {
		t.Fatalf("analysis provenance = %#v", analysis)
	}

	if err := worker.ProcessLostPet(ctx, eventData); err != nil {
		t.Fatalf("duplicate ProcessLostPet() error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("Ollama calls = %d, want 1 for duplicate delivery", calls.Load())
	}
	assertCompletedLostAnalysisOperation(t, stateStore, eventID, record.PetID)
}

func TestMatcherWorkerLostEventsWithoutImagesCompleteWithoutModel(t *testing.T) {
	baseTime := time.Date(2026, time.August, 17, 18, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		petID string
		data  func(*testing.T) []byte
	}{
		{
			name: "raw payload v1", petID: "lost-v1-no-image",
			data: func(t *testing.T) []byte {
				event := domain.LostPetEvent{
					PetID: "lost-v1-no-image", ReporterEmail: "owner@example.com",
					ReportedAt: baseTime, Location: "Seattle, WA",
				}
				data, err := event.ToJSON()
				if err != nil {
					t.Fatal(err)
				}
				return data
			},
		},
		{
			name: "payload v2", petID: "lost-v2-no-image",
			data: func(t *testing.T) []byte {
				return encodeLostPayloadVersion(t, "lost-v2-no-image", baseTime, domain.LostPetReportedContactPayloadVersion, domain.LostPetReportedV2{
					PetID: "lost-v2-no-image", ReportedAt: baseTime, Location: "Seattle, WA",
					GeocodingStatus: domain.GeocodingPending, Status: domain.LostPetStatusLost,
				})
			},
		},
		{
			name: "payload v3", petID: "lost-v3-no-image",
			data: func(t *testing.T) []byte {
				return encodeLostPayloadVersion(t, "lost-v3-no-image", baseTime, domain.LostPetReportedRedactedPayloadVersion, domain.LostPetReportedV3{
					PetID: "lost-v3-no-image", ReportedAt: baseTime, Location: "Seattle, WA",
					GeocodingStatus: domain.GeocodingPending, Status: domain.LostPetStatusLost,
				})
			},
		},
		{
			name: "payload v4", petID: "lost-v4-no-image",
			data: func(t *testing.T) []byte {
				return encodeLostPayloadVersion(t, "lost-v4-no-image", baseTime, domain.LostPetReportedPayloadVersion, domain.LostPetReportedV4{
					PetID: "lost-v4-no-image", ReportedAt: baseTime, Location: "Seattle, WA",
					GeocodingStatus: domain.GeocodingPending, Status: domain.LostPetStatusLost,
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateStore := store.NewMemoryStore()
			worker := NewWorker(stateStore, pubsub.NewMemoryPubSub(), nil)
			data := test.data(t)
			if err := worker.ProcessLostPet(context.Background(), data); err != nil {
				t.Fatalf("ProcessLostPet() error = %v", err)
			}
			_, envelope, err := domain.DecodeLostPetReported(data)
			if err != nil {
				t.Fatal(err)
			}
			envelopeID := ""
			if envelope != nil {
				envelopeID = envelope.ID
			}
			eventID, err := delivery.ResolveEventID(envelopeID, domain.EventTypeLostPetReported, data)
			if err != nil {
				t.Fatal(err)
			}
			assertCompletedLostAnalysisOperation(t, stateStore, eventID, test.petID)
		})
	}
}

func TestMatcherWorkerLostImageCompletionRetryDoesNotRepeatModel(t *testing.T) {
	ctx := context.Background()
	baseStore := store.NewMemoryStore()
	stateStore := &failCompleteOnceStore{Store: baseStore}
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
	reportedAt := time.Date(2026, time.August, 17, 19, 0, 0, 0, time.UTC)
	record := domain.LostPetRecord{
		PetID: grant.ReportID, OwnerIdentityRef: "identity-" + grant.ReportID,
		ImageObject: finalized.ObjectName, ReportedAt: reportedAt,
		Location: "Seattle, WA", GeocodingStatus: domain.GeocodingPending,
		Status: domain.LostPetStatusLost,
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := baseStore.SaveState(ctx, store.LostPetsCollection, record.PetID, data); err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	server := newMatcherOllamaServer(t, &calls, nil, nil)
	worker := NewWorkerWithImageStore(stateStore, pubsub.NewMemoryPubSub(), ollama.NewClient(ollama.WithBaseURL(server.URL)), images)
	eventData, eventID := encodeLostAnalysisEvent(t, domain.LostPetReportedV4{
		PetID: record.PetID, ImageObject: record.ImageObject, ReportedAt: record.ReportedAt,
		Location: record.Location, GeocodingStatus: record.GeocodingStatus, Status: record.Status,
	})

	if err := worker.ProcessLostPet(ctx, eventData); err == nil {
		t.Fatal("first ProcessLostPet() error = nil, want completion failure")
	}
	if err := worker.ProcessLostPet(ctx, eventData); err != nil {
		t.Fatalf("second ProcessLostPet() error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("Ollama calls = %d, want 1 after completion retry", calls.Load())
	}
	assertCompletedLostAnalysisOperation(t, baseStore, eventID, record.PetID)
}

func TestMatcherWorkerRejectsLostEventBoundToFoundImageNamespace(t *testing.T) {
	reportedAt := time.Date(2026, time.August, 17, 19, 30, 0, 0, time.UTC)
	stateStore := store.NewMemoryStore()
	record := domain.LostPetRecord{
		PetID: "lost-cross-purpose", OwnerIdentityRef: "identity-lost-cross-purpose",
		ImageObject: "images/found-pets/found-cross-purpose/image.jpg",
		ReportedAt:  reportedAt, Location: "Seattle, WA",
		GeocodingStatus: domain.GeocodingPending, Status: domain.LostPetStatusLost,
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(context.Background(), store.LostPetsCollection, record.PetID, data); err != nil {
		t.Fatal(err)
	}
	eventData, _ := encodeLostAnalysisEvent(t, domain.LostPetReportedV4{
		PetID: record.PetID, ImageObject: record.ImageObject, ReportedAt: record.ReportedAt,
		Location: record.Location, GeocodingStatus: record.GeocodingStatus, Status: record.Status,
	})
	worker := NewWorker(stateStore, pubsub.NewMemoryPubSub(), nil)
	if err := worker.ProcessLostPet(context.Background(), eventData); err == nil {
		t.Fatal("ProcessLostPet() error = nil, want cross-purpose rejection")
	}
	updated, err := stateStore.GetState(context.Background(), store.LostPetsCollection, record.PetID)
	if err != nil {
		t.Fatal(err)
	}
	var preserved domain.LostPetRecord
	if err := json.Unmarshal(updated, &preserved); err != nil {
		t.Fatal(err)
	}
	if preserved.ImageAnalysis != nil {
		t.Fatalf("cross-purpose event persisted analysis = %#v", preserved.ImageAnalysis)
	}
}

func encodeLostAnalysisEvent(t *testing.T, event domain.LostPetReportedV4) ([]byte, string) {
	t.Helper()
	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type: domain.EventTypeLostPetReported, OccurredAt: event.ReportedAt,
		AggregateID: event.PetID, AggregateVersion: 1,
		PayloadVersion: domain.LostPetReportedPayloadVersion, Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data, envelope.ID
}

func encodeLostPayloadVersion(t *testing.T, petID string, occurredAt time.Time, payloadVersion int, payload any) []byte {
	t.Helper()
	payloadData, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type: domain.EventTypeLostPetReported, OccurredAt: occurredAt,
		AggregateID: petID, AggregateVersion: 1,
		PayloadVersion: payloadVersion, Payload: payloadData,
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

func assertCompletedLostAnalysisOperation(
	t *testing.T,
	stateStore store.DeliveryOperationStore,
	eventID string,
	petID string,
) {
	t.Helper()
	operation, err := delivery.NewOperation(eventID, petID, lostImageAnalysisChannel, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	stored, err := stateStore.GetDeliveryOperation(context.Background(), operation.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != delivery.StatusCompleted {
		t.Fatalf("lost analysis operation status = %q, want completed", stored.Status)
	}
}
