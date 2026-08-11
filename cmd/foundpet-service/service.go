package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/scottdensmore/petspotr/pkg/blob"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/outbox"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Service coordinates FoundPet event persistence, image storage, and event publishing.
type Service struct {
	store     store.StateStore
	blobStore blob.BlobStore // Reserved for future direct binary image upload handling
	relay     *outbox.Relay
}

// NewService constructs a FoundPet Service instance.
func NewService(st store.StateStore, br pubsub.Publisher, bs blob.BlobStore) *Service {
	return &Service{
		store:     st,
		blobStore: bs,
		relay:     outbox.NewRelay(st, br),
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

// HandleFoundPet handles POST /foundPet HTTP requests.
func (s *Service) HandleFoundPet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	var evt domain.FoundPetEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
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

	ctx := r.Context()
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
