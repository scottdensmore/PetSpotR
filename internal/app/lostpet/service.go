// Package lostpet implements the lost-pet reporting application service.
package lostpet

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

// Service coordinates LostPet event persistence and publishing.
type Service struct {
	store                  store.StateStore
	imageStore             blob.ImageStore
	relay                  *outbox.Relay
	reportOperationTimeout time.Duration
}

// ReportMetadata carries transport metadata into the durable event envelope.
type ReportMetadata struct {
	CorrelationID string
	TraceID       string
}

// ReportCommand carries one lost-pet report from an HTTP adapter into the
// canonical application service.
type ReportCommand struct {
	PetID           string
	PetName         string
	Species         string
	Breed           string
	PrimaryColor    string
	Description     string
	ReporterEmail   string
	Phone           string
	ImageObject     string
	ReportedAt      time.Time
	Location        string
	GeocodingStatus domain.GeocodingStatus
	Coordinates     *domain.LocationPoint
	// OwnedBy is trusted transport identity, never a caller-supplied JSON field.
	OwnedBy *domain.PrincipalRef
}

// ReportResult identifies the accepted report and its durable event.
type ReportResult struct {
	PetID   string
	EventID string
}

// LifecycleCommand carries trusted owner identity and one idempotent terminal
// transition into the canonical application service.
type LifecycleCommand struct {
	PetID       string
	Status      domain.LostPetStatus
	OperationID string
	Actor       domain.PrincipalRef
}

// LifecycleResult identifies the durable lifecycle event.
type LifecycleResult struct {
	PetID   string
	Status  domain.LostPetStatus
	EventID string
}

var (
	ErrInvalidLifecycleCommand = errors.New("lostpet: invalid lifecycle command")
	ErrLifecycleUnavailable    = errors.New("lostpet: lifecycle persistence is unavailable")
	ErrLifecycleHidden         = errors.New("lostpet: lifecycle target is unavailable")
)

var errLifecycleAlreadyApplied = errors.New("lostpet: lifecycle operation already applied")

// ErrInvalidReport identifies domain validation failures.
var ErrInvalidReport = errors.New("lostpet: invalid report")

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

// NewService constructs a LostPet Service instance without an upload adapter.
func NewService(st store.StateStore, br pubsub.Publisher) *Service {
	return NewServiceWithImageStore(st, br, nil)
}

// NewServiceWithImageStore constructs a LostPet Service that owns the secure
// private-image upload and finalization lifecycle.
func NewServiceWithImageStore(st store.StateStore, br pubsub.Publisher, images blob.ImageStore) *Service {
	return &Service{
		store:                  st,
		imageStore:             images,
		relay:                  outbox.NewRelay(st, br),
		reportOperationTimeout: defaultReportOperationTimeout,
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// RecoverOutbox publishes one bounded batch of lost-report creation events.
// It is safe for runtimes that only have permission to publish lostPet.
func (s *Service) RecoverOutbox(ctx context.Context) (int, error) {
	return s.relay.PublishPending(ctx, "lostPet")
}

// RecoverLifecycleOutbox publishes one bounded batch of lost-report lifecycle
// events. Callers must have permission to publish petStatusChanged.
func (s *Service) RecoverLifecycleOutbox(ctx context.Context) (int, error) {
	return s.relay.PublishPending(ctx, "petStatusChanged")
}

// ReuniteLostPet atomically persists an owner- or global-operator-authorized
// terminal transition, its private audit receipt, and one redacted status-change
// outbox event. Contextual ownership is attempted before durable role authority.
func (s *Service) ReuniteLostPet(ctx context.Context, command LifecycleCommand) (LifecycleResult, error) {
	atomicStore, ok := s.store.(store.StateAndOutboxStore)
	if !ok {
		return LifecycleResult{}, ErrLifecycleUnavailable
	}
	command.PetID = strings.TrimSpace(command.PetID)
	command.OperationID = strings.TrimSpace(command.OperationID)
	if command.PetID == "" || command.Status != domain.LostPetStatusReunited || command.OperationID == "" {
		return LifecycleResult{}, fmt.Errorf("%w: pet ID, reunited status, and operation ID are required", ErrInvalidLifecycleCommand)
	}
	changedAt := time.Now().UTC()
	var applied domain.LostPetLifecycleResult
	err := atomicStore.UpdateStateAndCreateOutbox(
		ctx,
		store.LostPetsCollection,
		command.PetID,
		func(current []byte) (store.StateWrite, store.StateWrite, error) {
			return lostPetLifecycleWrites(current, command, changedAt, nil, &applied)
		},
	)
	if errors.Is(err, domain.ErrLostPetNotOwned) {
		roleStore, ok := s.store.(store.RoleAuthorizedStateAndOutboxStore)
		if !ok {
			return LifecycleResult{}, ErrLifecycleHidden
		}
		err = roleStore.UpdateStateAndCreateOutboxAsRole(
			ctx,
			command.Actor,
			domain.RoleOperator,
			domain.RoleScope{Kind: domain.RoleScopeGlobal},
			store.LostPetsCollection,
			command.PetID,
			func(authorization domain.RoleAssignment, current []byte) (store.StateWrite, store.StateWrite, error) {
				return lostPetLifecycleWrites(current, command, changedAt, &authorization, &applied)
			},
		)
	}
	if errors.Is(err, errLifecycleAlreadyApplied) {
		record, outboxErr := outbox.GetRecord(ctx, s.store, applied.EventID)
		if outboxErr != nil {
			return LifecycleResult{}, fmt.Errorf("lostpet: verify lifecycle retry: %w", outboxErr)
		}
		expectedEnvelope, envelopeErr := domain.NewPetStatusChangedEnvelope(applied.Event)
		expectedPayload, marshalErr := json.Marshal(expectedEnvelope)
		if record.ID != applied.EventID || record.Topic != "petStatusChanged" ||
			envelopeErr != nil || marshalErr != nil || !bytes.Equal(record.Payload, expectedPayload) {
			return LifecycleResult{}, errors.New("lostpet: lifecycle retry does not match its durable outbox")
		}
		err = nil
	}
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) ||
		errors.Is(err, store.ErrRoleDenied) || errors.Is(err, domain.ErrLostPetNotOwned) ||
		errors.Is(err, domain.ErrInvalidLostPetLifecycle) {
		return LifecycleResult{}, ErrLifecycleHidden
	}
	if err != nil {
		return LifecycleResult{}, err
	}
	if s.relay.CanPublish("petStatusChanged") {
		if _, publishErr := s.relay.PublishRecords(ctx, applied.EventID); publishErr != nil {
			log.Printf("LostPet lifecycle outbox publication deferred: %v", publishErr)
		}
	}
	return LifecycleResult{PetID: applied.Record.PetID, Status: applied.Record.Status, EventID: applied.EventID}, nil
}

func lostPetLifecycleWrites(
	current []byte,
	command LifecycleCommand,
	changedAt time.Time,
	authorization *domain.RoleAssignment,
	applied *domain.LostPetLifecycleResult,
) (store.StateWrite, store.StateWrite, error) {
	*applied = domain.LostPetLifecycleResult{}
	var record domain.LostPetRecord
	if err := json.Unmarshal(current, &record); err != nil || record.PetID != command.PetID {
		return store.StateWrite{}, store.StateWrite{}, ErrLifecycleHidden
	}
	var (
		result domain.LostPetLifecycleResult
		err    error
	)
	if authorization == nil {
		result, err = domain.ApplyOwnerLostPetReunion(record, command.Actor, command.OperationID, changedAt)
	} else {
		result, err = domain.ApplyGlobalOperatorLostPetReunion(
			record, command.Actor, *authorization, command.OperationID, changedAt,
		)
	}
	if err != nil {
		return store.StateWrite{}, store.StateWrite{}, err
	}
	*applied = result
	if !result.Changed {
		return store.StateWrite{}, store.StateWrite{}, errLifecycleAlreadyApplied
	}
	nextData, err := json.Marshal(result.Record)
	if err != nil {
		return store.StateWrite{}, store.StateWrite{}, err
	}
	envelope, err := domain.NewPetStatusChangedEnvelope(result.Event)
	if err != nil {
		return store.StateWrite{}, store.StateWrite{}, fmt.Errorf("lostpet: create lifecycle event: %w", err)
	}
	if envelope.ID != result.EventID {
		return store.StateWrite{}, store.StateWrite{}, errors.New("lostpet: lifecycle event identity changed")
	}
	envelopeData, err := json.Marshal(envelope)
	if err != nil {
		return store.StateWrite{}, store.StateWrite{}, err
	}
	outboxData, err := outbox.MarshalRecord(outbox.NewRecord(
		envelope.ID, "petStatusChanged", envelopeData, result.Event.ChangedAt,
	))
	if err != nil {
		return store.StateWrite{}, store.StateWrite{}, err
	}
	return store.StateWrite{StoreName: store.LostPetsCollection, Key: command.PetID, Data: nextData},
		store.StateWrite{StoreName: store.OutboxCollection, Key: envelope.ID, Data: outboxData}, nil
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// HandleBeginImageUpload handles POST /lostPet/uploads requests.
func (s *Service) HandleBeginImageUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}
	if s.imageStore == nil {
		respondWithError(w, http.StatusServiceUnavailable, "Private image uploads are unavailable")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	var intent blob.ImageUploadIntent
	if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
	}
	if intent.Purpose != blob.ImagePurposeLostPet {
		respondWithError(w, http.StatusBadRequest, "Upload purpose must be lost-pet")
		return
	}
	grant, err := s.imageStore.BeginImageUpload(r.Context(), intent)
	if errors.Is(err, blob.ErrInvalidImage) {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Printf("LostPet image upload grant failed: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to create image upload")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(grant)
}

// CleanupOrphanedImages removes old finalized lost-pet images that remain
// unreferenced after the report transaction's recovery window.
func (s *Service) CleanupOrphanedImages(ctx context.Context, now time.Time) (int, error) {
	cleaner, ok := s.imageStore.(blob.ScopedOrphanCleaner)
	if !ok {
		return 0, nil
	}
	referenced := func(checkCtx context.Context, reportID, objectName string) (bool, error) {
		data, err := s.store.GetState(checkCtx, store.LostPetsCollection, reportID)
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		var record domain.LostPetRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return false, fmt.Errorf("decode lost-pet state %q: %w", reportID, err)
		}
		return record.PetID == reportID && record.ImageObject == objectName, nil
	}
	return cleaner.CleanupOrphanedFinalizedImagesForPurpose(
		ctx,
		blob.ImagePurposeLostPet,
		now.UTC().Add(-blob.DefaultOrphanGracePeriod),
		blob.MaxOrphanCleanupBatch,
		referenced,
	)
}

// HandleLostPet handles POST /lostPet HTTP requests.
func (s *Service) HandleLostPet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.reportOperationTimeout)
	defer cancel()
	r = r.WithContext(ctx)
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var request struct {
		PetID           string                 `json:"petId"`
		PetName         string                 `json:"petName"`
		Species         string                 `json:"species"`
		Breed           string                 `json:"breed"`
		PrimaryColor    string                 `json:"primaryColor"`
		Description     string                 `json:"description"`
		ReporterEmail   string                 `json:"reporterEmail"`
		Phone           string                 `json:"phone"`
		ImageObject     string                 `json:"imageObject"`
		ReportedAt      time.Time              `json:"reportedAt"`
		Location        string                 `json:"location"`
		GeocodingStatus domain.GeocodingStatus `json:"geocodingStatus"`
		Coordinates     *domain.LocationPoint  `json:"coordinates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
	}
	if strings.TrimSpace(request.ImageObject) != "" {
		if s.imageStore == nil {
			respondWithError(w, http.StatusBadRequest, "A generated private image upload is required")
			return
		}
		finalized, err := s.imageStore.FinalizeImageForPurpose(
			r.Context(), blob.ImagePurposeLostPet, request.PetID, request.ImageObject,
			r.Header.Get("X-PetSpotR-Upload-Token"),
		)
		if errors.Is(err, blob.ErrInvalidImage) || errors.Is(err, blob.ErrUploadMismatch) ||
			errors.Is(err, blob.ErrUploadExpired) || errors.Is(err, blob.ErrNotFound) ||
			errors.Is(err, blob.ErrNotFinalized) {
			respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Image finalization failed: %v", err))
			return
		}
		if err != nil {
			log.Printf("LostPet image finalization failed: %v", err)
			respondWithError(w, http.StatusInternalServerError, "Image finalization failed")
			return
		}
		request.ImageObject = finalized.ObjectName
	}

	result, err := s.ReportLostPet(r.Context(), ReportCommand{
		PetID:           request.PetID,
		PetName:         request.PetName,
		Species:         request.Species,
		Breed:           request.Breed,
		PrimaryColor:    request.PrimaryColor,
		Description:     request.Description,
		ReporterEmail:   request.ReporterEmail,
		Phone:           request.Phone,
		ImageObject:     request.ImageObject,
		ReportedAt:      request.ReportedAt,
		Location:        request.Location,
		GeocodingStatus: request.GeocodingStatus,
		Coordinates:     request.Coordinates,
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

// ReportLostPet validates and durably accepts one lost-pet report before
// attempting best-effort publication from its transactional outbox.
func (s *Service) ReportLostPet(
	ctx context.Context,
	command ReportCommand,
	metadata ReportMetadata,
) (ReportResult, error) {
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID:           command.PetID,
		PetName:         command.PetName,
		Species:         command.Species,
		Breed:           command.Breed,
		PrimaryColor:    command.PrimaryColor,
		Description:     command.Description,
		ReporterEmail:   command.ReporterEmail,
		Phone:           command.Phone,
		ImageObject:     command.ImageObject,
		ReportedAt:      command.ReportedAt,
		Location:        command.Location,
		GeocodingStatus: command.GeocodingStatus,
		Coordinates:     command.Coordinates,
		OwnedBy:         command.OwnedBy,
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
	// creating canonical state so an old report such as " lost-123 " cannot be
	// duplicated under the normalized "lost-123" key after an upgrade.
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
		return ReportResult{}, fmt.Errorf("failed to marshal lost-pet report: %w", err)
	}
	contactData, err := json.Marshal(contact)
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal lost-pet contact: %w", err)
	}
	eventData, err := json.Marshal(report.ReportedEvent())
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal lost-pet event: %w", err)
	}
	occurredAt := report.ReportedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeLostPetReported,
		OccurredAt:       occurredAt,
		CorrelationID:    metadata.CorrelationID,
		TraceID:          metadata.TraceID,
		AggregateID:      report.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.LostPetReportedPayloadVersion,
		Payload:          eventData,
	})
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to create event envelope: %w", err)
	}
	envelopeData, err := json.Marshal(envelope)
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal event envelope: %w", err)
	}
	recordData, err := outbox.MarshalRecord(outbox.NewRecord(envelope.ID, "lostPet", envelopeData, time.Now().UTC()))
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to create outbox record: %w", err)
	}

	_, err = s.store.CreateStatesAndOutbox(ctx, []store.StateWrite{
		{StoreName: store.LostPetsCollection, Key: report.PetID, Data: stateData},
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
	if s.relay.CanPublish("lostPet") {
		if _, err := s.relay.PublishRecords(ctx, envelope.ID); err != nil {
			log.Printf("LostPet outbox publication deferred: %v", err)
		}
	}

	return ReportResult{PetID: report.PetID, EventID: envelope.ID}, nil
}

func (s *Service) matchPersistedRetry(
	ctx context.Context,
	lookupPetID string,
	report domain.LostPetReport,
) (ReportResult, bool, bool, error) {
	legacyData, err := s.store.GetState(ctx, store.LostPetsCollection, lookupPetID)
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
	if _, isSeparated := shape["ownerIdentityRef"]; isSeparated {
		result, matches, err := s.matchSeparatedRetry(ctx, legacyData, report)
		return result, true, matches, err
	}
	// Pre-ownership records remain readable and anonymously retryable, but an
	// authenticated caller cannot acquire them merely by replaying their input.
	if report.OwnedBy != nil {
		return ReportResult{}, true, false, nil
	}
	if _, isCurrent := shape["geocodingStatus"]; isCurrent {
		var previous domain.LostPetReport
		if err := json.Unmarshal(legacyData, &previous); err != nil {
			return ReportResult{}, true, false, nil
		}
		previous = domain.NormalizeLostPetReport(previous)
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
		payload, err := json.Marshal(previous.ReportedEventV2())
		if err != nil {
			return ReportResult{}, true, false, err
		}
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeLostPetReported,
			OccurredAt:       previous.ReportedAt,
			AggregateID:      previous.PetID,
			AggregateVersion: 1,
			PayloadVersion:   domain.LostPetReportedContactPayloadVersion,
			Payload:          payload,
		})
		if err != nil {
			return ReportResult{}, true, false, nil
		}
		result, matches, err := s.matchPersistedOutbox(ctx, previous.PetID, envelope)
		return result, true, matches, err
	}

	var legacy domain.LostPetEvent
	if err := json.Unmarshal(legacyData, &legacy); err != nil {
		return ReportResult{}, true, false, nil
	}
	if strings.TrimSpace(legacy.PetID) != report.PetID ||
		strings.ToLower(strings.TrimSpace(legacy.ReporterEmail)) != report.ReporterEmail ||
		strings.TrimSpace(legacy.Location) != report.Location ||
		!sameTimestamp(legacy.ReportedAt, report.ReportedAt) {
		return ReportResult{}, true, false, nil
	}

	occurredAt := legacy.ReportedAt
	if occurredAt.IsZero() {
		occurredAt = time.Unix(0, 0).UTC()
	}
	legacyEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeLostPetReported,
		OccurredAt:       occurredAt,
		AggregateID:      report.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.LostPetReportedLegacyPayloadVersion,
		Payload:          legacyData,
	})
	if err != nil {
		return ReportResult{}, true, false, nil
	}
	result, matches, err := s.matchPersistedOutbox(ctx, legacy.PetID, legacyEnvelope)
	return result, true, matches, err
}

func (s *Service) matchSeparatedRetry(
	ctx context.Context,
	stateData []byte,
	report domain.LostPetReport,
) (ReportResult, bool, error) {
	var previous domain.LostPetRecord
	if err := json.Unmarshal(stateData, &previous); err != nil {
		return ReportResult{}, false, nil
	}
	previous = domain.NormalizeLostPetRecord(previous)
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

	contactData, err := s.store.GetState(ctx, store.ReportContactsCollection, previous.OwnerIdentityRef)
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

	variants := []struct {
		version int
		payload any
	}{
		{version: domain.LostPetReportedPayloadVersion, payload: report.ReportedEvent()},
		{version: domain.LostPetReportedRedactedPayloadVersion, payload: report.ReportedEventV3()},
		{version: domain.LostPetReportedContactPayloadVersion, payload: report.ReportedEventV2()},
	}
	for _, variant := range variants {
		payload, err := json.Marshal(variant.payload)
		if err != nil {
			return ReportResult{}, false, err
		}
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeLostPetReported,
			OccurredAt:       report.ReportedAt,
			AggregateID:      report.PetID,
			AggregateVersion: 1,
			PayloadVersion:   variant.version,
			Payload:          payload,
		})
		if err != nil {
			continue
		}
		result, matches, err := s.matchPersistedOutbox(ctx, previous.PetID, envelope)
		if err != nil || matches {
			return result, matches, err
		}
	}
	return ReportResult{}, false, nil
}

func (s *Service) matchPersistedOutbox(
	ctx context.Context,
	petID string,
	envelope domain.EventEnvelope,
) (ReportResult, bool, error) {
	record, err := outbox.GetRecord(ctx, s.store, envelope.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return ReportResult{}, false, nil
		}
		return ReportResult{}, false, err
	}
	if record.Topic != "lostPet" {
		return ReportResult{}, false, nil
	}
	return ReportResult{PetID: petID, EventID: envelope.ID}, true, nil
}

func sameTimestamp(first, second time.Time) bool {
	if first.IsZero() || second.IsZero() {
		return first.IsZero() && second.IsZero()
	}
	return first.Equal(second)
}
