// Package foundpet implements the found-pet reporting application service.
package foundpet

import (
	"bytes"
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

// ReportCommand carries one found-pet report from an HTTP adapter into the
// canonical application service.
type ReportCommand struct {
	PetID               string
	ImageURL            string
	ImageObject         string
	FoundAt             time.Time
	Location            string
	GeocodingStatus     domain.GeocodingStatus
	Coordinates         *domain.LocationPoint
	FinderEmail         string
	Species             string
	Breed               string
	PrimaryColor        string
	SecondaryColor      string
	DistinctiveMarkings []string
	CustodyStatus       domain.CustodyStatus
	// OwnedBy is trusted transport identity, never a caller-supplied JSON field.
	OwnedBy *domain.PrincipalRef
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
	if intent.Purpose != blob.ImagePurposeFoundPet {
		respondWithError(w, http.StatusBadRequest, "Upload purpose must be found-pet")
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

	var request struct {
		PetID               string                 `json:"petId"`
		ImageURL            string                 `json:"imageUrl"`
		ImageObject         string                 `json:"imageObject"`
		FoundAt             time.Time              `json:"foundAt"`
		Location            string                 `json:"location"`
		GeocodingStatus     domain.GeocodingStatus `json:"geocodingStatus"`
		Coordinates         *domain.LocationPoint  `json:"coordinates"`
		FinderEmail         string                 `json:"finderEmail"`
		Species             string                 `json:"species"`
		Breed               string                 `json:"breed"`
		PrimaryColor        string                 `json:"primaryColor"`
		SecondaryColor      string                 `json:"secondaryColor"`
		DistinctiveMarkings []string               `json:"distinctiveMarkings"`
		CustodyStatus       domain.CustodyStatus   `json:"custodyStatus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
	}
	if s.requireFinalizedImage {
		if strings.TrimSpace(request.ImageURL) != "" || strings.TrimSpace(request.ImageObject) == "" {
			respondWithError(w, http.StatusBadRequest, "A generated private image upload is required")
			return
		}
		finalized, err := s.imageStore.FinalizeImageForPurpose(
			r.Context(), blob.ImagePurposeFoundPet, request.PetID, request.ImageObject,
			r.Header.Get("X-PetSpotR-Upload-Token"),
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
		request.ImageObject = finalized.ObjectName
	}

	result, err := s.ReportFoundPet(ctx, ReportCommand{
		PetID:               request.PetID,
		ImageURL:            request.ImageURL,
		ImageObject:         request.ImageObject,
		FoundAt:             request.FoundAt,
		Location:            request.Location,
		GeocodingStatus:     request.GeocodingStatus,
		Coordinates:         request.Coordinates,
		FinderEmail:         request.FinderEmail,
		Species:             request.Species,
		Breed:               request.Breed,
		PrimaryColor:        request.PrimaryColor,
		SecondaryColor:      request.SecondaryColor,
		DistinctiveMarkings: request.DistinctiveMarkings,
		CustodyStatus:       request.CustodyStatus,
	}, ReportMetadata{
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
	command ReportCommand,
	metadata ReportMetadata,
) (ReportResult, error) {
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID:               command.PetID,
		ImageURL:            command.ImageURL,
		ImageObject:         command.ImageObject,
		FoundAt:             command.FoundAt,
		Location:            command.Location,
		GeocodingStatus:     command.GeocodingStatus,
		Coordinates:         command.Coordinates,
		FinderEmail:         command.FinderEmail,
		Species:             command.Species,
		Breed:               command.Breed,
		PrimaryColor:        command.PrimaryColor,
		SecondaryColor:      command.SecondaryColor,
		DistinctiveMarkings: command.DistinctiveMarkings,
		CustodyStatus:       command.CustodyStatus,
		OwnedBy:             command.OwnedBy,
	})
	if err := report.Validate(); err != nil {
		return ReportResult{}, &invalidReportError{cause: err}
	}
	persistedReport, contact := report.Persisted()
	if err := contact.Validate(); err != nil {
		return ReportResult{}, &invalidReportError{cause: err}
	}

	// Payload-v1 persisted state used the caller-provided key even though the
	// envelope normalized its aggregate ID. Check that exact legacy key before
	// creating canonical state so an old whitespace-bearing key cannot be
	// duplicated under its normalized form after an upgrade.
	if command.PetID != report.PetID {
		legacyResult, legacyExists, matches, err := s.matchPersistedRetry(ctx, command.PetID, report)
		if err != nil {
			return ReportResult{}, fmt.Errorf("failed to check persisted report retry: %w", err)
		}
		if matches {
			return legacyResult, nil
		}
		if legacyExists {
			return ReportResult{}, fmt.Errorf("failed to save state and outbox: %w", store.ErrConflict)
		}
	}

	stateData, err := json.Marshal(persistedReport)
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal found-pet report: %w", err)
	}
	contactData, err := json.Marshal(contact)
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal found-pet contact: %w", err)
	}
	eventData, err := json.Marshal(report.ReportedEvent())
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal found-pet event: %w", err)
	}

	occurredAt := report.FoundAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       occurredAt,
		CorrelationID:    metadata.CorrelationID,
		TraceID:          metadata.TraceID,
		AggregateID:      report.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.FoundPetReportedPayloadVersion,
		Payload:          eventData,
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

	_, err = s.store.CreateStatesAndOutbox(ctx, []store.StateWrite{
		{StoreName: store.FoundPetsCollection, Key: report.PetID, Data: stateData},
		{StoreName: store.ReportContactsCollection, Key: contact.IdentityRef, Data: contactData},
	},
		store.StateWrite{StoreName: store.OutboxCollection, Key: envelope.ID, Data: recordData},
	)
	if errors.Is(err, store.ErrConflict) {
		legacyResult, _, matches, compatibilityErr := s.matchPersistedRetry(ctx, report.PetID, report)
		if compatibilityErr != nil {
			return ReportResult{}, fmt.Errorf("failed to check persisted report retry: %w", compatibilityErr)
		}
		if matches {
			return legacyResult, nil
		}
	}
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

	return ReportResult{PetID: report.PetID, EventID: envelope.ID}, nil
}

func (s *Service) matchPersistedRetry(
	ctx context.Context,
	lookupPetID string,
	report domain.FoundPetReport,
) (ReportResult, bool, bool, error) {
	legacyData, err := s.store.GetState(ctx, store.FoundPetsCollection, lookupPetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return ReportResult{}, false, false, nil
		}
		return ReportResult{}, false, false, err
	}
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(legacyData, &shape); err != nil {
		return ReportResult{}, true, false, nil
	}
	if _, isSeparated := shape["finderIdentityRef"]; isSeparated {
		result, matches, err := s.matchSeparatedRetry(ctx, legacyData, report)
		return result, true, matches, err
	}
	// Pre-ownership records remain readable and anonymously retryable, but an
	// authenticated caller cannot acquire them merely by replaying their input.
	if report.OwnedBy != nil {
		return ReportResult{}, true, false, nil
	}
	if _, isCurrent := shape["geocodingStatus"]; isCurrent {
		var previous domain.FoundPetReport
		if err := json.Unmarshal(legacyData, &previous); err != nil {
			return ReportResult{}, true, false, nil
		}
		previous = domain.NormalizeFoundPetReport(previous)
		previousData, err := json.Marshal(previous)
		if err != nil {
			return ReportResult{}, true, false, err
		}
		reportData, err := json.Marshal(report)
		if err != nil {
			return ReportResult{}, true, false, err
		}
		if !bytes.Equal(previousData, reportData) {
			return ReportResult{}, true, false, nil
		}
		payload, err := json.Marshal(previous.ReportedEvent())
		if err != nil {
			return ReportResult{}, true, false, err
		}
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeFoundPetReported,
			OccurredAt:       previous.FoundAt,
			AggregateID:      previous.PetID,
			AggregateVersion: 1,
			PayloadVersion:   domain.FoundPetReportedPayloadVersion,
			Payload:          payload,
		})
		if err != nil {
			return ReportResult{}, true, false, nil
		}
		record, err := outbox.GetRecord(ctx, s.store, envelope.ID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
				return ReportResult{}, true, false, nil
			}
			return ReportResult{}, true, false, err
		}
		if record.Topic != "foundPet" {
			return ReportResult{}, true, false, nil
		}
		return ReportResult{PetID: previous.PetID, EventID: envelope.ID}, true, true, nil
	}

	var legacy domain.FoundPetEvent
	if err := json.Unmarshal(legacyData, &legacy); err != nil {
		return ReportResult{}, true, false, nil
	}
	if strings.TrimSpace(legacy.PetID) != report.PetID ||
		strings.TrimSpace(legacy.ImageURL) != report.ImageURL ||
		strings.TrimSpace(legacy.ImageObject) != report.ImageObject ||
		strings.TrimSpace(legacy.Location) != report.Location ||
		!sameTimestamp(legacy.FoundAt, report.FoundAt) {
		return ReportResult{}, true, false, nil
	}

	occurredAt := legacy.FoundAt
	if occurredAt.IsZero() {
		occurredAt = time.Unix(0, 0).UTC()
	}
	legacyEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       occurredAt,
		AggregateID:      report.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.FoundPetReportedLegacyPayloadVersion,
		Payload:          legacyData,
	})
	if err != nil {
		return ReportResult{}, true, false, nil
	}
	record, err := outbox.GetRecord(ctx, s.store, legacyEnvelope.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return ReportResult{}, true, false, nil
		}
		return ReportResult{}, true, false, err
	}
	if record.Topic != "foundPet" {
		return ReportResult{}, true, false, nil
	}
	return ReportResult{PetID: legacy.PetID, EventID: legacyEnvelope.ID}, true, true, nil
}

func (s *Service) matchSeparatedRetry(
	ctx context.Context,
	stateData []byte,
	report domain.FoundPetReport,
) (ReportResult, bool, error) {
	var previous domain.FoundPetRecord
	if err := json.Unmarshal(stateData, &previous); err != nil {
		return ReportResult{}, false, nil
	}
	previous = domain.NormalizeFoundPetRecord(previous)
	// Image analysis is matcher-owned enrichment added after report creation.
	// It must not turn an exact reporter retry into a competing create.
	previous.ImageAnalysis = nil
	expectedRecord, expectedContact := report.Persisted()
	previousData, err := json.Marshal(previous)
	if err != nil {
		return ReportResult{}, false, err
	}
	expectedData, err := json.Marshal(expectedRecord)
	if err != nil {
		return ReportResult{}, false, err
	}
	if !bytes.Equal(previousData, expectedData) {
		return ReportResult{}, false, nil
	}

	contactData, err := s.store.GetState(ctx, store.ReportContactsCollection, previous.FinderIdentityRef)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return ReportResult{}, false, nil
		}
		return ReportResult{}, false, err
	}
	var previousContact domain.ReportContact
	if err := json.Unmarshal(contactData, &previousContact); err != nil {
		return ReportResult{}, false, nil
	}
	previousContact = domain.NormalizeReportContact(previousContact)
	previousContactData, err := json.Marshal(previousContact)
	if err != nil {
		return ReportResult{}, false, err
	}
	expectedContactData, err := json.Marshal(expectedContact)
	if err != nil {
		return ReportResult{}, false, err
	}
	if !bytes.Equal(previousContactData, expectedContactData) {
		return ReportResult{}, false, nil
	}

	payload, err := json.Marshal(report.ReportedEvent())
	if err != nil {
		return ReportResult{}, false, err
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       report.FoundAt,
		AggregateID:      report.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.FoundPetReportedPayloadVersion,
		Payload:          payload,
	})
	if err != nil {
		return ReportResult{}, false, nil
	}
	record, err := outbox.GetRecord(ctx, s.store, envelope.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return ReportResult{}, false, nil
		}
		return ReportResult{}, false, err
	}
	if record.Topic != "foundPet" {
		return ReportResult{}, false, nil
	}
	return ReportResult{PetID: previous.PetID, EventID: envelope.ID}, true, nil
}

func sameTimestamp(first, second time.Time) bool {
	if first.IsZero() || second.IsZero() {
		return first.IsZero() && second.IsZero()
	}
	return first.Equal(second)
}
