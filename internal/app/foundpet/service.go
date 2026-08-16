// Package foundpet implements the found-pet reporting application service.
package foundpet

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

// ReportMetadata carries transport metadata into the durable event envelope.
type ReportMetadata struct {
	CorrelationID string
	TraceID       string
}

// ReportResult identifies the accepted report and its durable event.
type ReportResult struct {
	PetID   string
	EventID string
}

// ErrInvalidReport identifies domain validation failures.
var ErrInvalidReport = errors.New("foundpet: invalid report")

type invalidReportError struct {
	cause error
}

func (e *invalidReportError) Error() string {
	return fmt.Sprintf("%s: %v", ErrInvalidReport, e.cause)
}

func (e *invalidReportError) Unwrap() error {
	return e.cause
}

func (e *invalidReportError) Is(target error) bool {
	return target == ErrInvalidReport || errors.Is(e.cause, target)
}

// InvalidReportCause returns the domain validation cause when err identifies
// an invalid report.
func InvalidReportCause(err error) error {
	var validationErr *invalidReportError
	if errors.As(err, &validationErr) {
		return validationErr.cause
	}
	return nil
}

const defaultReportOperationTimeout = 2 * time.Minute

// NewService constructs a FoundPet Service instance.
func NewService(st store.StateStore, br pubsub.Publisher, images blob.ImageStore) *Service {
	return NewServiceWithOptions(st, br, images, ServiceOptions{})
}

// NewReportService constructs a Service for an adapter that accepts existing
// image references and does not own the secure upload lifecycle.
func NewReportService(st store.StateStore, br pubsub.Publisher) *Service {
	return NewService(st, br, nil)
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

// RecoverOutbox publishes one bounded batch of durable foundPet events.
func (s *Service) RecoverOutbox(ctx context.Context) (int, error) {
	return s.relay.PublishPending(ctx, "foundPet")
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

	result, err := s.ReportFoundPet(ctx, evt, ReportMetadata{
		CorrelationID: r.Header.Get("X-Correlation-ID"),
		TraceID:       r.Header.Get("X-Trace-ID"),
	})
	switch {
	case errors.Is(err, ErrInvalidReport):
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Event validation failed: %v", InvalidReportCause(err)))
		return
	case errors.Is(err, store.ErrConflict):
		respondWithError(w, http.StatusConflict, "A different report already exists for this pet ID")
		return
	case err != nil:
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "success",
		"petId":   result.PetID,
		"eventId": result.EventID,
	})
}

// ReportFoundPet validates and durably accepts one found-pet report before
// attempting best-effort publication from its transactional outbox. Image
// finalization remains the responsibility of the direct HTTP adapter because
// browser-compatible reports may carry an existing image URL.
func (s *Service) ReportFoundPet(
	ctx context.Context,
	evt domain.FoundPetEvent,
	metadata ReportMetadata,
) (ReportResult, error) {
	if err := evt.Validate(); err != nil {
		return ReportResult{}, &invalidReportError{cause: err}
	}
	if strings.TrimSpace(evt.Location) == "" {
		return ReportResult{}, &invalidReportError{cause: errors.New("foundpet: location is required")}
	}

	data, err := evt.ToJSON()
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal event: %w", err)
	}

	occurredAt := evt.FoundAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       occurredAt,
		CorrelationID:    metadata.CorrelationID,
		TraceID:          metadata.TraceID,
		AggregateID:      evt.PetID,
		AggregateVersion: 1,
		PayloadVersion:   1,
		Payload:          data,
	})
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to create event envelope: %w", err)
	}
	envelopeData, err := json.Marshal(envelope)
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal event envelope: %w", err)
	}
	recordData, err := outbox.MarshalRecord(outbox.NewRecord(envelope.ID, "foundPet", envelopeData, time.Now().UTC()))
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to create outbox record: %w", err)
	}

	_, err = s.store.CreateStateAndOutbox(ctx,
		store.StateWrite{StoreName: store.FoundPetsCollection, Key: evt.PetID, Data: data},
		store.StateWrite{StoreName: store.OutboxCollection, Key: envelope.ID, Data: recordData},
	)
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to save state and outbox: %w", err)
	}

	// Publication is best effort on the request path. A failure leaves the
	// atomic outbox record pending for a later request or the managed relay,
	// so the durable report can still be acknowledged safely.
	if s.relay.CanPublish("foundPet") {
		if _, err := s.relay.PublishRecords(ctx, envelope.ID); err != nil {
			log.Printf("FoundPet outbox publication deferred: %v", err)
		}
	}

	return ReportResult{PetID: evt.PetID, EventID: envelope.ID}, nil
}
