package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Service coordinates LostPet event persistence and publishing.
type Service struct {
	store  store.StateStore
	broker pubsub.Broker
}

// NewService constructs a LostPet Service instance.
func NewService(st store.StateStore, br pubsub.Broker) *Service {
	return &Service{
		store:  st,
		broker: br,
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

	if err := evt.Validate(); err != nil {
		respondWithError(w, http.StatusBadRequest, fmt.Sprintf("Event validation failed: %v", err))
		return
	}

	data, err := evt.ToJSON()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to marshal event: %v", err))
		return
	}

	ctx := r.Context()

	// 1. Save state
	if err := s.store.SaveState(ctx, "lostPets", evt.PetID, data); err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save state: %v", err))
		return
	}

	// 2. Publish lostPet event
	if err := s.broker.Publish(ctx, "lostPet", data); err != nil {
		respondWithError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to publish event: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
		"petId":  evt.PetID,
	})
}
