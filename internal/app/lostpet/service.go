// Package lostpet implements the lost-pet reporting application service.
package lostpet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Service coordinates LostPet event persistence and publishing.
type Service struct {
	store store.StateStore
	relay *outbox.Relay
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
	ReportedAt      time.Time
	Location        string
	GeocodingStatus domain.GeocodingStatus
	Coordinates     *domain.LocationPoint
}

// ReportResult identifies the accepted report and its durable event.
type ReportResult struct {
	PetID   string
	EventID string
}

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

// NewService constructs a LostPet Service instance.
func NewService(st store.StateStore, br pubsub.Publisher) *Service {
	return &Service{
		store: st,
		relay: outbox.NewRelay(st, br),
	}
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// RecoverOutbox publishes one bounded batch of durable lostPet events.
func (s *Service) RecoverOutbox(ctx context.Context) (int, error) {
	return s.relay.PublishPending(ctx, "lostPet")
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

// HandleLostPet handles POST /lostPet HTTP requests.
func (s *Service) HandleLostPet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

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
		ReportedAt      time.Time              `json:"reportedAt"`
		Location        string                 `json:"location"`
		GeocodingStatus domain.GeocodingStatus `json:"geocodingStatus"`
		Coordinates     *domain.LocationPoint  `json:"coordinates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
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
		ReportedAt:      command.ReportedAt,
		Location:        command.Location,
		GeocodingStatus: command.GeocodingStatus,
		Coordinates:     command.Coordinates,
	})
	if err := report.Validate(); err != nil {
		return ReportResult{}, &invalidReportError{cause: err}
	}

	// Payload-v1 persisted state used the caller-provided key even though the
	// envelope normalized its aggregate ID. Check that exact legacy key before
	// creating canonical state so an old report such as " lost-123 " cannot be
	// duplicated under the normalized "lost-123" key after an upgrade.
	if command.PetID != report.PetID {
		legacyResult, legacyExists, matches, err := s.matchPayloadV1Retry(ctx, command.PetID, report)
		if err != nil {
			return ReportResult{}, fmt.Errorf("failed to check payload-v1 retry: %w", err)
		}
		if matches {
			return legacyResult, nil
		}
		if legacyExists {
			return ReportResult{}, fmt.Errorf("failed to save state and outbox: %w", store.ErrConflict)
		}
	}

	stateData, err := json.Marshal(report)
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal lost-pet report: %w", err)
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

	_, err = s.store.CreateStateAndOutbox(ctx,
		store.StateWrite{StoreName: store.LostPetsCollection, Key: report.PetID, Data: stateData},
		store.StateWrite{StoreName: store.OutboxCollection, Key: envelope.ID, Data: recordData},
	)
	if errors.Is(err, store.ErrConflict) {
		legacyResult, _, matches, compatibilityErr := s.matchPayloadV1Retry(ctx, report.PetID, report)
		if compatibilityErr != nil {
			return ReportResult{}, fmt.Errorf("failed to check payload-v1 retry: %w", compatibilityErr)
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

func (s *Service) matchPayloadV1Retry(
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
	if _, isCurrent := shape["geocodingStatus"]; isCurrent {
		return ReportResult{}, true, false, nil
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
	record, err := outbox.GetRecord(ctx, s.store, legacyEnvelope.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return ReportResult{}, true, false, nil
		}
		return ReportResult{}, true, false, err
	}
	if record.Topic != "lostPet" {
		return ReportResult{}, true, false, nil
	}
	return ReportResult{PetID: legacy.PetID, EventID: legacyEnvelope.ID}, true, true, nil
}

func sameTimestamp(first, second time.Time) bool {
	if first.IsZero() || second.IsZero() {
		return first.IsZero() && second.IsZero()
	}
	return first.Equal(second)
}
