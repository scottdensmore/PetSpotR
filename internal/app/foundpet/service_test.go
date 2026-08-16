package foundpet

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestFoundPetService_HandleFoundPet(t *testing.T) {
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	bs := blob.NewMemoryBlobStore("https://storage.petspotr.io/images")
	svc := NewService(st, ps, bs)

	var publishedEvent domain.FoundPetEvent
	var publishedEnvelope *domain.EventEnvelope
	var published bool
	_ = ps.Subscribe("foundPet", func(ctx context.Context, data []byte) error {
		published = true
		var err error
		publishedEnvelope, err = domain.DecodeEventPayload(data, domain.EventTypeFoundPetReported, &publishedEvent)
		return err
	})

	t.Run("valid found pet submission saves state and publishes event", func(t *testing.T) {
		evt := domain.FoundPetEvent{
			PetID:    "pet-found-555",
			ImageURL: "https://storage.petspotr.io/images/found-555.jpg",
			FoundAt:  time.Now().UTC(),
			Location: "Portland, OR",
		}

		body, _ := json.Marshal(evt)
		req := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		svc.HandleFoundPet(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 status code, got %d (body: %s)", rec.Code, rec.Body.String())
		}

		// Verify state persistence
		data, err := st.GetState(context.Background(), store.FoundPetsCollection, "pet-found-555")
		if err != nil {
			t.Fatalf("failed to retrieve saved state: %v", err)
		}

		var savedEvt domain.FoundPetEvent
		_ = json.Unmarshal(data, &savedEvt)
		if savedEvt.PetID != "pet-found-555" {
			t.Errorf("saved pet ID mismatch: got %s, want pet-found-555", savedEvt.PetID)
		}

		// Verify pubsub event publication
		if !published {
			t.Fatal("expected foundPet event to be published")
		}
		if publishedEvent.PetID != "pet-found-555" {
			t.Errorf("published pet ID mismatch: got %s, want pet-found-555", publishedEvent.PetID)
		}
		if publishedEnvelope == nil || publishedEnvelope.AggregateVersion != 1 ||
			publishedEnvelope.PayloadVersion != domain.FoundPetReportedPayloadVersion {
			t.Fatalf("published envelope = %#v", publishedEnvelope)
		}

		published = false
		retry := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(body))
		retryRecorder := httptest.NewRecorder()
		svc.HandleFoundPet(retryRecorder, retry)
		if retryRecorder.Code != http.StatusCreated {
			t.Fatalf("retry status = %d, want 201", retryRecorder.Code)
		}
		if published {
			t.Fatal("exact retry republished an already completed event")
		}

		competing := evt
		competing.Location = "Seattle, WA"
		competingBody, _ := json.Marshal(competing)
		competingRequest := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(competingBody))
		competingRecorder := httptest.NewRecorder()
		svc.HandleFoundPet(competingRecorder, competingRequest)
		if competingRecorder.Code != http.StatusConflict {
			t.Fatalf("competing create status = %d, want 409", competingRecorder.Code)
		}
	})

	t.Run("non-POST method returns 405 Method Not Allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/foundPet", nil)
		rec := httptest.NewRecorder()

		svc.HandleFoundPet(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 status code, got %d", rec.Code)
		}
	})

	t.Run("invalid payload returns 400 Bad Request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader([]byte(`{"petId":""}`)))
		rec := httptest.NewRecorder()

		svc.HandleFoundPet(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 status code, got %d", rec.Code)
		}
	})
}

func TestFoundPetServicePersistsPrivateContactSeparately(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	service := NewReportService(stateStore, pubsub.NewMemoryPubSub())
	command := ReportCommand{
		PetID:       "found-private-contact",
		ImageURL:    "https://storage.petspotr.io/found-private-contact.jpg",
		FoundAt:     time.Date(2026, time.August, 16, 13, 0, 0, 0, time.UTC),
		Location:    "Seattle, WA",
		FinderEmail: "FINDER@EXAMPLE.COM",
	}

	if _, err := service.ReportFoundPet(ctx, command, ReportMetadata{}); err != nil {
		t.Fatalf("ReportFoundPet() error = %v", err)
	}
	reportData, err := stateStore.GetState(ctx, store.FoundPetsCollection, command.PetID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(reportData), "finder@example.com") || strings.Contains(string(reportData), "finderEmail") {
		t.Fatalf("persisted report exposed private contact: %s", reportData)
	}
	var report domain.FoundPetRecord
	if err := json.Unmarshal(reportData, &report); err != nil {
		t.Fatal(err)
	}
	contactData, err := stateStore.GetState(ctx, store.ReportContactsCollection, report.FinderIdentityRef)
	if err != nil {
		t.Fatal(err)
	}
	var contact domain.ReportContact
	if err := json.Unmarshal(contactData, &contact); err != nil {
		t.Fatal(err)
	}
	if contact.IdentityRef != report.FinderIdentityRef || contact.Email != "finder@example.com" {
		t.Fatalf("private contact = %#v; report = %#v", contact, report)
	}

	changed := command
	changed.FinderEmail = "other@example.com"
	if _, err := service.ReportFoundPet(ctx, changed, ReportMetadata{}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("changed contact retry error = %v, want ErrConflict", err)
	}
}

func TestFoundPetService_PayloadV1RetryRemainsIdempotentAfterSchemaUpgrade(t *testing.T) {
	petIDs := []string{"found-before-v2", " found-before-v2-spaced "}
	for _, petID := range petIDs {
		t.Run(strings.ReplaceAll(petID, " ", "_"), func(t *testing.T) {
			ctx := context.Background()
			stateStore := store.NewMemoryStore()
			legacy := domain.FoundPetEvent{
				PetID:    petID,
				ImageURL: "https://storage.petspotr.io/images/found-before-v2.jpg",
				FoundAt:  time.Date(2026, time.August, 15, 13, 0, 0, 0, time.UTC),
				Location: "Seattle, WA",
			}
			legacyData, err := legacy.ToJSON()
			if err != nil {
				t.Fatal(err)
			}
			legacyEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
				Type:             domain.EventTypeFoundPetReported,
				OccurredAt:       legacy.FoundAt,
				AggregateID:      legacy.PetID,
				AggregateVersion: 1,
				PayloadVersion:   domain.FoundPetReportedLegacyPayloadVersion,
				Payload:          legacyData,
			})
			if err != nil {
				t.Fatal(err)
			}
			legacyEnvelopeData, err := json.Marshal(legacyEnvelope)
			if err != nil {
				t.Fatal(err)
			}
			if err := stateStore.SaveState(ctx, store.FoundPetsCollection, legacy.PetID, legacyData); err != nil {
				t.Fatal(err)
			}
			if err := outbox.SaveRecord(ctx, stateStore, outbox.NewRecord(
				legacyEnvelope.ID,
				"foundPet",
				legacyEnvelopeData,
				legacy.FoundAt,
			)); err != nil {
				t.Fatal(err)
			}

			svc := NewService(stateStore, pubsub.NewMemoryPubSub(), blob.NewMemoryBlobStore("https://storage.petspotr.io/images"))
			submit := func(location string) *httptest.ResponseRecorder {
				body, marshalErr := json.Marshal(map[string]any{
					"petId":               legacy.PetID,
					"imageUrl":            legacy.ImageURL,
					"foundAt":             legacy.FoundAt,
					"location":            location,
					"finderEmail":         "finder@example.com",
					"species":             "Dog",
					"breed":               "Golden Retriever",
					"primaryColor":        "Golden",
					"secondaryColor":      "Cream",
					"distinctiveMarkings": []string{"White chest patch"},
					"custodyStatus":       "Finder Home",
				})
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				request := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(body))
				recorder := httptest.NewRecorder()
				svc.HandleFoundPet(recorder, request)
				return recorder
			}

			retry := submit(legacy.Location)
			if retry.Code != http.StatusCreated {
				t.Fatalf("payload-v1 retry status = %d, want 201; body = %s", retry.Code, retry.Body.String())
			}
			var response map[string]string
			if err := json.Unmarshal(retry.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response["petId"] != legacy.PetID || response["eventId"] != legacyEnvelope.ID {
				t.Fatalf("payload-v1 retry response = %#v", response)
			}
			preserved, err := stateStore.GetState(ctx, store.FoundPetsCollection, legacy.PetID)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(preserved, legacyData) {
				t.Fatalf("payload-v1 state was rewritten: %s", preserved)
			}
			if normalizedID := strings.TrimSpace(legacy.PetID); normalizedID != legacy.PetID {
				if _, err := stateStore.GetState(ctx, store.FoundPetsCollection, normalizedID); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("normalized duplicate state lookup error = %v, want %v", err, store.ErrNotFound)
				}
			}

			conflict := submit("Portland, OR")
			if conflict.Code != http.StatusConflict {
				t.Fatalf("conflicting payload-v1 retry status = %d, want 409; body = %s", conflict.Code, conflict.Body.String())
			}
			outboxRecords, err := stateStore.ListState(ctx, store.OutboxCollection)
			if err != nil {
				t.Fatal(err)
			}
			if len(outboxRecords) != 1 {
				t.Fatalf("outbox record count = %d, want 1", len(outboxRecords))
			}
		})
	}
}

func TestFoundPetServiceContactBearingPayloadV2StateRemainsRetryable(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	previous := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID:         "found-before-private-contact",
		ImageURL:      "https://storage.petspotr.io/found-before-private-contact.jpg",
		FoundAt:       time.Date(2026, time.August, 15, 13, 0, 0, 0, time.UTC),
		Location:      "Seattle, WA",
		FinderEmail:   "finder@example.com",
		Species:       "Dog",
		CustodyStatus: domain.CustodyFinderHome,
	})
	stateData, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	eventData, err := json.Marshal(previous.ReportedEvent())
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       previous.FoundAt,
		AggregateID:      previous.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.FoundPetReportedPayloadVersion,
		Payload:          eventData,
	})
	if err != nil {
		t.Fatal(err)
	}
	envelopeData, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.FoundPetsCollection, previous.PetID, stateData); err != nil {
		t.Fatal(err)
	}
	if err := outbox.SaveRecord(ctx, stateStore, outbox.NewRecord(envelope.ID, "foundPet", envelopeData, previous.FoundAt)); err != nil {
		t.Fatal(err)
	}

	service := NewReportService(stateStore, pubsub.NewMemoryPubSub())
	command := ReportCommand{
		PetID:         previous.PetID,
		ImageURL:      previous.ImageURL,
		FoundAt:       previous.FoundAt,
		Location:      previous.Location,
		FinderEmail:   previous.FinderEmail,
		Species:       previous.Species,
		CustodyStatus: previous.CustodyStatus,
	}
	result, err := service.ReportFoundPet(ctx, command, ReportMetadata{})
	if err != nil || result.EventID != envelope.ID {
		t.Fatalf("ReportFoundPet() = %#v, %v; want legacy event %s", result, err, envelope.ID)
	}
	preserved, err := stateStore.GetState(ctx, store.FoundPetsCollection, previous.PetID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(preserved, stateData) {
		t.Fatalf("contact-bearing state was rewritten: %s", preserved)
	}
	_, contact := previous.Persisted()
	if _, err := stateStore.GetState(ctx, store.ReportContactsCollection, contact.IdentityRef); !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		t.Fatalf("compatibility retry contact lookup error = %v, want not found", err)
	}

	command.FinderEmail = "other@example.com"
	if _, err := service.ReportFoundPet(ctx, command, ReportMetadata{}); !errors.Is(err, store.ErrConflict) {
		t.Fatalf("changed legacy contact error = %v, want ErrConflict", err)
	}
}

func TestFoundPetServiceRejectsSeparatedStateMissingPrivateContact(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	previous := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID:    "found-missing-private-contact",
		ImageURL: "https://storage.petspotr.io/found-missing-private-contact.jpg",
		FoundAt:  time.Date(2026, time.August, 16, 13, 0, 0, 0, time.UTC),
		Location: "Seattle, WA",
		Species:  "Dog",
	})
	record, _ := previous.Persisted()
	stateData, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	eventData, err := json.Marshal(previous.ReportedEvent())
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       previous.FoundAt,
		AggregateID:      previous.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.FoundPetReportedPayloadVersion,
		Payload:          eventData,
	})
	if err != nil {
		t.Fatal(err)
	}
	envelopeData, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err := stateStore.SaveState(ctx, store.FoundPetsCollection, previous.PetID, stateData); err != nil {
		t.Fatal(err)
	}
	if err := outbox.SaveRecord(ctx, stateStore, outbox.NewRecord(envelope.ID, "foundPet", envelopeData, previous.FoundAt)); err != nil {
		t.Fatal(err)
	}

	service := NewReportService(stateStore, pubsub.NewMemoryPubSub())
	_, err = service.ReportFoundPet(ctx, ReportCommand{
		PetID:    previous.PetID,
		ImageURL: previous.ImageURL,
		FoundAt:  previous.FoundAt,
		Location: previous.Location,
		Species:  previous.Species,
	}, ReportMetadata{})
	if !errors.Is(err, store.ErrConflict) {
		t.Fatalf("missing private contact error = %v, want ErrConflict", err)
	}
}

func TestFoundPetService_SecureImageLifecycle(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	ps := pubsub.NewMemoryPubSub()
	images := blob.NewMemoryBlobStore("https://storage.petspotr.io")
	svc := NewServiceWithOptions(st, ps, images, ServiceOptions{RequireFinalizedImage: true})

	beginBody := bytes.NewBufferString(`{"purpose":"found-pet","fileName":"../../untrusted.jpg","contentType":"image/jpeg"}`)
	beginRequest := httptest.NewRequest(http.MethodPost, "/foundPet/uploads", beginBody)
	beginRecorder := httptest.NewRecorder()
	svc.HandleBeginImageUpload(beginRecorder, beginRequest)
	if beginRecorder.Code != http.StatusCreated {
		t.Fatalf("begin upload status = %d, want 201: %s", beginRecorder.Code, beginRecorder.Body.String())
	}
	var grant blob.ImageUploadGrant
	if err := json.Unmarshal(beginRecorder.Body.Bytes(), &grant); err != nil {
		t.Fatalf("decode upload grant: %v", err)
	}
	if grant.ReportID == "" || !strings.HasPrefix(grant.ObjectName, "uploads/found-pets/"+grant.ReportID+"/") {
		t.Fatalf("upload grant = %#v", grant)
	}
	if _, err := images.UploadImage(ctx, grant.ObjectName, encodedServiceImage(t)); err != nil {
		t.Fatalf("upload image: %v", err)
	}

	event := domain.FoundPetEvent{
		PetID: grant.ReportID, ImageObject: grant.ObjectName,
		FoundAt: time.Now().UTC(), Location: "Portland, OR",
	}
	eventBody, _ := json.Marshal(event)
	reportRequest := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(eventBody))
	reportRequest.Header.Set("X-PetSpotR-Upload-Token", grant.FinalizeToken)
	reportRecorder := httptest.NewRecorder()
	svc.HandleFoundPet(reportRecorder, reportRequest)
	if reportRecorder.Code != http.StatusCreated {
		t.Fatalf("report status = %d, want 201: %s", reportRecorder.Code, reportRecorder.Body.String())
	}

	stateData, err := st.GetState(ctx, store.FoundPetsCollection, grant.ReportID)
	if err != nil {
		t.Fatalf("get found-pet state: %v", err)
	}
	var saved domain.FoundPetReport
	if err := json.Unmarshal(stateData, &saved); err != nil {
		t.Fatalf("decode saved report: %v", err)
	}
	wantObject := "images/found-pets/" + grant.ReportID + "/image.jpg"
	if saved.ImageObject != wantObject || saved.ImageURL != "" ||
		saved.GeocodingStatus != domain.GeocodingPending || saved.CustodyStatus != domain.CustodyUnknown ||
		saved.Status != domain.FoundPetStatusFound {
		t.Fatalf("saved image reference = %#v, want private object %q", saved, wantObject)
	}
	if _, err := images.GetImage(ctx, grant.ObjectName); !errors.Is(err, blob.ErrNotFound) {
		t.Fatalf("temporary image remains after report creation: %v", err)
	}

	retryRequest := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(eventBody))
	retryRequest.Header.Set("X-PetSpotR-Upload-Token", grant.FinalizeToken)
	retryRecorder := httptest.NewRecorder()
	svc.HandleFoundPet(retryRecorder, retryRequest)
	if retryRecorder.Code != http.StatusCreated {
		t.Fatalf("idempotent report retry status = %d, want 201: %s", retryRecorder.Code, retryRecorder.Body.String())
	}
}

func TestFoundPetService_SecureModeRejectsExternalImageURL(t *testing.T) {
	st := store.NewMemoryStore()
	svc := NewServiceWithOptions(
		st,
		pubsub.NewMemoryPubSub(),
		blob.NewMemoryBlobStore("https://storage.petspotr.io"),
		ServiceOptions{RequireFinalizedImage: true},
	)
	event := domain.FoundPetEvent{
		PetID: "caller-id", ImageURL: "https://attacker.example/pet.jpg", Location: "Seattle, WA",
	}
	body, _ := json.Marshal(event)
	recorder := httptest.NewRecorder()
	svc.HandleFoundPet(recorder, httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(body)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("external image status = %d, want 400: %s", recorder.Code, recorder.Body.String())
	}
	if _, err := st.GetState(context.Background(), store.FoundPetsCollection, event.PetID); !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		t.Fatalf("rejected report mutated state: %v", err)
	}
}

func encodedServiceImage(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := jpeg.Encode(&output, image.NewRGBA(image.Rect(0, 0, 2, 3)), nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	return output.Bytes()
}

type cleanupCheckingImageStore struct {
	*blob.MemoryBlobStore
	checked bool
}

type deadlineBlockingStore struct {
	*store.MemoryStore
	entered  chan struct{}
	timedOut chan struct{}
	release  chan struct{}
}

func (s *deadlineBlockingStore) CreateStateAndOutbox(
	ctx context.Context,
	stateWrite store.StateWrite,
	outboxWrite store.StateWrite,
) (bool, error) {
	close(s.entered)
	<-ctx.Done()
	close(s.timedOut)
	<-s.release
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return s.MemoryStore.CreateStateAndOutbox(ctx, stateWrite, outboxWrite)
}

func (s *deadlineBlockingStore) CreateStatesAndOutbox(
	ctx context.Context,
	stateWrites []store.StateWrite,
	outboxWrite store.StateWrite,
) (bool, error) {
	close(s.entered)
	<-ctx.Done()
	close(s.timedOut)
	<-s.release
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return s.MemoryStore.CreateStatesAndOutbox(ctx, stateWrites, outboxWrite)
}

type deadlineCleanupImageStore struct {
	*blob.MemoryBlobStore
	reportID   string
	objectName string
	cleaned    bool
}

func (s *deadlineCleanupImageStore) CleanupOrphanedFinalizedImages(
	ctx context.Context,
	_ time.Time,
	_ int,
	referenced blob.FinalizedImageReferenceChecker,
) (int, error) {
	for range 2 {
		isReferenced, err := referenced(ctx, s.reportID, s.objectName)
		if err != nil {
			return 0, err
		}
		if isReferenced {
			return 0, errors.New("paused report was treated as durably referenced")
		}
	}
	s.cleaned = true
	return 1, nil
}

func TestFoundPetService_ReportDeadlinePreventsCleanupCommitRace(t *testing.T) {
	ctx := context.Background()
	st := &deadlineBlockingStore{
		MemoryStore: store.NewMemoryStore(),
		entered:     make(chan struct{}),
		timedOut:    make(chan struct{}),
		release:     make(chan struct{}),
	}
	images := &deadlineCleanupImageStore{MemoryBlobStore: blob.NewMemoryBlobStore("https://storage.petspotr.io")}
	svc := NewServiceWithOptions(st, pubsub.NewMemoryPubSub(), images, ServiceOptions{
		RequireFinalizedImage:  true,
		ReportOperationTimeout: 25 * time.Millisecond,
	})

	beginRecorder := httptest.NewRecorder()
	svc.HandleBeginImageUpload(beginRecorder, httptest.NewRequest(
		http.MethodPost,
		"/foundPet/uploads",
		bytes.NewBufferString(`{"purpose":"found-pet","contentType":"image/jpeg"}`),
	))
	if beginRecorder.Code != http.StatusCreated {
		t.Fatalf("begin upload status = %d: %s", beginRecorder.Code, beginRecorder.Body.String())
	}
	var grant blob.ImageUploadGrant
	if err := json.Unmarshal(beginRecorder.Body.Bytes(), &grant); err != nil {
		t.Fatal(err)
	}
	if _, err := images.UploadImage(ctx, grant.ObjectName, encodedServiceImage(t)); err != nil {
		t.Fatal(err)
	}
	images.reportID = grant.ReportID
	images.objectName = "images/found-pets/" + grant.ReportID + "/image.jpg"

	event := domain.FoundPetEvent{
		PetID: grant.ReportID, ImageObject: grant.ObjectName,
		FoundAt: time.Now().UTC(), Location: "Portland, OR",
	}
	body, _ := json.Marshal(event)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/foundPet", bytes.NewReader(body))
	request.Header.Set("X-PetSpotR-Upload-Token", grant.FinalizeToken)
	done := make(chan struct{})
	go func() {
		svc.HandleFoundPet(recorder, request)
		close(done)
	}()

	<-st.entered
	<-st.timedOut
	deleted, err := svc.CleanupOrphanedImages(ctx, time.Now().UTC().Add(blob.DefaultOrphanGracePeriod))
	if err != nil || deleted != 1 || !images.cleaned {
		t.Fatalf("cleanup = (%d, %v), cleaned=%t", deleted, err, images.cleaned)
	}
	close(st.release)
	<-done

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("timed-out report status = %d, want 500: %s", recorder.Code, recorder.Body.String())
	}
	if _, err := st.GetState(ctx, store.FoundPetsCollection, grant.ReportID); !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		t.Fatalf("timed-out report committed after cleanup: %v", err)
	}
}

func (s *cleanupCheckingImageStore) CleanupOrphanedFinalizedImages(
	ctx context.Context,
	_ time.Time,
	_ int,
	referenced blob.FinalizedImageReferenceChecker,
) (int, error) {
	s.checked = true
	isReferenced, err := referenced(ctx, "found-referenced", "images/found-pets/found-referenced/image.jpg")
	if err != nil {
		return 0, err
	}
	if !isReferenced {
		return 0, errors.New("durable report reference was not recognized")
	}
	isReferenced, err = referenced(ctx, "found-orphan", "images/found-pets/found-orphan/image.jpg")
	if err != nil {
		return 0, err
	}
	if isReferenced {
		return 0, errors.New("missing report was treated as referenced")
	}
	return 1, nil
}

func TestFoundPetService_ReconcilesFinalizedImageReferences(t *testing.T) {
	ctx := context.Background()
	st := store.NewMemoryStore()
	stored, _ := (&domain.FoundPetEvent{
		PetID: "found-referenced", ImageObject: "images/found-pets/found-referenced/image.jpg",
	}).ToJSON()
	if err := st.SaveState(ctx, store.FoundPetsCollection, "found-referenced", stored); err != nil {
		t.Fatal(err)
	}
	images := &cleanupCheckingImageStore{MemoryBlobStore: blob.NewMemoryBlobStore("https://storage.petspotr.io")}
	svc := NewService(st, pubsub.NewMemoryPubSub(), images)
	deleted, err := svc.CleanupOrphanedImages(ctx, time.Now().UTC())
	if err != nil || deleted != 1 || !images.checked {
		t.Fatalf("CleanupOrphanedImages() = (%d, %v), checked=%t", deleted, err, images.checked)
	}
}

func TestFoundPetService_RecoversPendingOutbox(t *testing.T) {
	ctx := context.Background()
	stateStore := store.NewMemoryStore()
	broker := pubsub.NewMemoryPubSub()
	published := make(chan []byte, 1)
	if err := broker.Subscribe("foundPet", func(_ context.Context, data []byte) error {
		published <- append([]byte(nil), data...)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	record := outbox.NewRecord(
		"evt-found-recovery",
		"foundPet",
		[]byte(`{"petId":"found-recovery"}`),
		time.Now().UTC(),
	)
	if err := outbox.SaveRecord(ctx, stateStore, record); err != nil {
		t.Fatal(err)
	}
	svc := NewService(
		stateStore,
		broker,
		blob.NewMemoryBlobStore("https://storage.petspotr.io/images"),
	)

	count, err := svc.RecoverOutbox(ctx)
	if err != nil || count != 1 {
		t.Fatalf("RecoverOutbox() = %d, %v; want 1, nil", count, err)
	}
	select {
	case got := <-published:
		if string(got) != string(record.Payload) {
			t.Fatalf("published payload = %s, want %s", got, record.Payload)
		}
	default:
		t.Fatal("RecoverOutbox() did not publish the pending foundPet event")
	}
	got, err := outbox.GetRecord(ctx, stateStore, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != outbox.StatusPublished {
		t.Fatalf("recovered status = %q, want %q", got.Status, outbox.StatusPublished)
	}
}
