package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Service coordinates FoundPet event persistence, image storage, and event publishing.
type Service struct {
	store                  store.StateStore
	imageStore             blob.ImageStore
	requireFinalizedImage  bool
	relay                  *outbox.Relay
	reportOperationTimeout time.Duration
}

// ServiceOptions controls compatibility boundaries for found-pet reports.
type ServiceOptions struct {
	RequireFinalizedImage  bool
	ReportOperationTimeout time.Duration
}

const defaultReportOperationTimeout = 2 * time.Minute

// NewService constructs a FoundPet Service instance.
func NewService(st store.StateStore, br pubsub.Publisher, images blob.ImageStore) *Service {
	return NewServiceWithOptions(st, br, images, ServiceOptions{})
}

// NewServiceWithOptions constructs a FoundPet Service with explicit image policy.
func NewServiceWithOptions(st store.StateStore, br pubsub.Publisher, images blob.ImageStore, options ServiceOptions) *Service {
	if options.ReportOperationTimeout <= 0 {
		options.ReportOperationTimeout = defaultReportOperationTimeout
	}
	return &Service{
		store:                  st,
		imageStore:             images,
		requireFinalizedImage:  options.RequireFinalizedImage,
		relay:                  outbox.NewRelay(st, br),
		reportOperationTimeout: options.ReportOperationTimeout,
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// HandleBeginImageUpload handles POST /foundPet/uploads requests.
func (s *Service) HandleBeginImageUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var intent blob.ImageUploadIntent
	if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
	}
	grant, err := s.imageStore.BeginImageUpload(r.Context(), intent)
	if errors.Is(err, blob.ErrInvalidImage) {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Printf("FoundPet image upload grant failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create image upload")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(grant)
}

// CleanupOrphanedImages removes old finalized images that are not referenced
// by durable found-pet state. The storage adapter performs a second reference
// check and generation recheck immediately before deletion.
func (s *Service) CleanupOrphanedImages(ctx context.Context, now time.Time) (int, error) {
	cleaner, ok := s.imageStore.(blob.OrphanCleaner)
	if !ok {
		return 0, nil
	}
	referenced := func(checkCtx context.Context, reportID, objectName string) (bool, error) {
		data, err := s.store.GetState(checkCtx, store.FoundPetsCollection, reportID)
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		var event domain.FoundPetEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return false, fmt.Errorf("decode found-pet state %q: %w", reportID, err)
		}
		return event.PetID == reportID && event.ImageObject == objectName, nil
	}
	return cleaner.CleanupOrphanedFinalizedImages(
		ctx,
		now.UTC().Add(-blob.DefaultOrphanGracePeriod),
		blob.MaxOrphanCleanupBatch,
		referenced,
	)
}

// HandleFoundPet handles POST /foundPet HTTP requests.
func (s *Service) HandleFoundPet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), s.reportOperationTimeout)
	defer cancel()
	r = r.WithContext(ctx)

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var evt domain.FoundPetEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
	}
	if s.requireFinalizedImage {
		if strings.TrimSpace(evt.ImageURL) != "" || strings.TrimSpace(evt.ImageObject) == "" {
			respondWithError(w, http.StatusBadRequest, "A generated private image upload is required")
			return
		}
		finalized, err := s.imageStore.FinalizeImage(
			r.Context(), evt.PetID, evt.ImageObject, r.Header.Get("X-PetSpotR-Upload-Token"),
		)
		if errors.Is(err, blob.ErrInvalidImage) || errors.Is(err, blob.ErrUploadMismatch) ||
			errors.Is(err, blob.ErrUploadExpired) ||
			errors.Is(err, blob.ErrNotFound) || errors.Is(err, blob.ErrNotFinalized) {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Image finalization failed: %v", err))
			return
		}
		if err != nil {
			log.Printf("FoundPet image finalization failed: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Image finalization failed")
			return
		}
		evt.ImageObject = finalized.ObjectName
	}

	if err := evt.Validate(); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Event validation failed: %v", err))
		return
	}

	data, err := evt.ToJSON()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to marshal event: %v", err))
		return
	}

	occurredAt := evt.FoundAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       occurredAt,
		CorrelationID:    r.Header.Get("X-Correlation-ID"),
		TraceID:          r.Header.Get("X-Trace-ID"),
		AggregateID:      evt.PetID,
		AggregateVersion: 1,
		PayloadVersion:   1,
		Payload:          data,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create event envelope: %v", err))
		return
	}
	envelopeData, err := json.Marshal(envelope)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to marshal event envelope: %v", err))
		return
	}
	recordData, err := outbox.MarshalRecord(outbox.NewRecord(envelope.ID, "foundPet", envelopeData, time.Now().UTC()))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create outbox record: %v", err))
		return
	}

	_, err = s.store.CreateStateAndOutbox(ctx,
		store.StateWrite{StoreName: store.FoundPetsCollection, Key: evt.PetID, Data: data},
		store.StateWrite{StoreName: store.OutboxCollection, Key: envelope.ID, Data: recordData},
	)
	if errors.Is(err, store.ErrConflict) {
		respondWithError(w, http.StatusConflict, "A different report already exists for this pet ID")
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save state and outbox: %v", err))
		return
	}
	if s.relay.CanPublish("foundPet") {
		if _, err := s.relay.PublishRecords(ctx, envelope.ID); err != nil {
			log.Printf("FoundPet outbox publication deferred: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"petId":   evt.PetID,
		"eventId": envelope.ID,
	})
}
