// Package lostpet implements the lost-pet reporting application service.
package lostpet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
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

	var evt domain.LostPetEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
	}

	result, err := s.ReportLostPet(r.Context(), evt, ReportMetadata{
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
	evt domain.LostPetEvent,
	metadata ReportMetadata,
) (ReportResult, error) {
	if err := evt.Validate(); err != nil {
		return ReportResult{}, &invalidReportError{cause: err}
	}

	data, err := evt.ToJSON()
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to marshal event: %w", err)
	}

	occurredAt := evt.ReportedAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeLostPetReported,
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
	recordData, err := outbox.MarshalRecord(outbox.NewRecord(envelope.ID, "lostPet", envelopeData, time.Now().UTC()))
	if err != nil {
		return ReportResult{}, fmt.Errorf("failed to create outbox record: %w", err)
	}

	_, err = s.store.CreateStateAndOutbox(ctx,
		store.StateWrite{StoreName: store.LostPetsCollection, Key: evt.PetID, Data: data},
		store.StateWrite{StoreName: store.OutboxCollection, Key: envelope.ID, Data: recordData},
	)
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

	return ReportResult{PetID: evt.PetID, EventID: envelope.ID}, nil
}
