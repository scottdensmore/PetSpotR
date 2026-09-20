package webfrontend

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sighting"
	"github.com/scottdensmore/petspotr/pkg/store"
)

type reportSightingRequest struct {
	LocationDescription string                `json:"locationDescription"`
	SightedAt           time.Time             `json:"sightedAt"`
	Coordinates         *domain.LocationPoint `json:"coordinates"`
	MovementDirection   string                `json:"movementDirection,omitempty"`
	ImageURL            string                `json:"imageUrl,omitempty"`
	ImageURLAlt         string                `json:"imageURL,omitempty"`
	ImageObject         string                `json:"imageObject,omitempty"`
	Notes               string                `json:"notes,omitempty"`
	ReporterContact     any                   `json:"reporterContact,omitempty"`
}

func generateSightingID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("sight-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("sight-%s", hex.EncodeToString(random[:]))
}

func (s *Server) handleApiSightings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleApiReportSighting(w, r)
	case http.MethodGet:
		s.handleApiListSightings(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleApiReportSighting(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	petBytes, err := s.stateStore.GetState(r.Context(), store.LostPetsCollection, petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to verify lost pet", http.StatusInternalServerError)
		return
	}

	var lostPet domain.LostPetRecord
	if err := json.Unmarshal(petBytes, &lostPet); err != nil {
		lostPet.PetID = petID
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req reportSightingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.Coordinates == nil {
		http.Error(w, "coordinates are required", http.StatusBadRequest)
		return
	}
	if math.IsNaN(req.Coordinates.Latitude) || math.IsNaN(req.Coordinates.Longitude) ||
		math.IsInf(req.Coordinates.Latitude, 0) || math.IsInf(req.Coordinates.Longitude, 0) {
		http.Error(w, "coordinates must be valid finite numbers", http.StatusBadRequest)
		return
	}
	if err := req.Coordinates.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.SightedAt.IsZero() {
		http.Error(w, "sightedAt is required", http.StatusBadRequest)
		return
	}
	if req.SightedAt.After(time.Now().UTC().Add(10 * time.Second)) {
		http.Error(w, "sightedAt cannot be in the future", http.StatusBadRequest)
		return
	}

	imageURL := req.ImageURL
	if imageURL == "" && req.ImageURLAlt != "" {
		imageURL = req.ImageURLAlt
	}

	record := domain.PetSightingRecord{
		SightingID:          generateSightingID(),
		LostPetID:           petID,
		ReportedAt:          time.Now().UTC(),
		SightedAt:           req.SightedAt.UTC(),
		LocationDescription: strings.TrimSpace(req.LocationDescription),
		Coordinates:         req.Coordinates,
		MovementDirection:   strings.TrimSpace(req.MovementDirection),
		ImageURL:            imageURL,
		ImageObject:         strings.TrimSpace(req.ImageObject),
		Notes:               strings.TrimSpace(req.Notes),
		Status:              domain.SightingStatusActive,
	}

	if err := record.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	recordBytes, err := json.Marshal(record)
	if err != nil {
		http.Error(w, "Failed to encode sighting record", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.SightingsCollection, record.SightingID, recordBytes); err != nil {
		http.Error(w, "Failed to persist sighting", http.StatusInternalServerError)
		return
	}

	if s.reunionHub != nil {
		event := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_sighting_%s", record.SightingID),
			Type:      domain.ReunionEventSighting,
			MatchID:   petID,
			Timestamp: time.Now().UTC(),
			Payload: domain.SightingEventPayload{
				Type:                "sighting",
				PetID:               petID,
				SightingID:          record.SightingID,
				SightedAt:           record.SightedAt,
				LocationDescription: record.LocationDescription,
				Coordinates:         record.Coordinates,
				MovementDirection:   record.MovementDirection,
			},
		}
		s.reunionHub.Broadcast(event)

		// Broadcast to any active matches associated with this pet
		if matchBytes, err := s.stateStore.ListState(r.Context(), store.MatchesCollection); err == nil {
			for _, mb := range matchBytes {
				var m domain.MatchRecord
				if err := json.Unmarshal(mb, &m); err == nil {
					if m.LostPetID == petID || m.MatchedPetID == petID || m.LostPet.PetID == petID {
						matchEvent := event
						matchEvent.MatchID = m.MatchID
						s.reunionHub.Broadcast(matchEvent)
					}
				}
			}
		}
	}

	petName := strings.TrimSpace(lostPet.PetName)
	if petName == "" {
		petName = "Lost Pet"
	}

	var title string
	if record.LocationDescription != "" {
		title = fmt.Sprintf("Pet Sighting: %s spotted near %s!", petName, record.LocationDescription)
	} else {
		title = fmt.Sprintf("Pet Sighting: %s spotted!", petName)
	}

	var message string
	if record.MovementDirection != "" {
		message = fmt.Sprintf("A community member spotted %s heading %s.", petName, record.MovementDirection)
	} else if record.LocationDescription != "" {
		message = fmt.Sprintf("A community member spotted %s near %s.", petName, record.LocationDescription)
	} else {
		message = fmt.Sprintf("A community member reported a new sighting of %s.", petName)
	}

	notif := domain.NotificationItem{
		ID:        fmt.Sprintf("notif-%s", record.SightingID),
		PetID:     petID,
		Type:      "sighting_reported",
		Title:     title,
		Message:   message,
		CreatedAt: time.Now().UTC(),
		Read:      false,
	}

	if notifBytes, err := json.Marshal(notif); err == nil {
		_ = s.stateStore.SaveState(r.Context(), store.NotificationsCollection, notif.ID, notifBytes)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(record)
}

func (s *Server) handleApiListSightings(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	rawItems, err := s.stateStore.ListState(r.Context(), store.SightingsCollection)
	if err != nil && !errors.Is(err, store.ErrStoreNotFound) && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "Failed to list sightings", http.StatusInternalServerError)
		return
	}

	sightings := make([]domain.PetSightingRecord, 0)
	for _, b := range rawItems {
		var sRec domain.PetSightingRecord
		if err := json.Unmarshal(b, &sRec); err != nil {
			continue
		}
		if sRec.LostPetID == petID && sRec.Status == domain.SightingStatusActive {
			sightings = append(sightings, sRec)
		}
	}

	sort.Slice(sightings, func(i, j int) bool {
		if !sightings[i].SightedAt.Equal(sightings[j].SightedAt) {
			return sightings[i].SightedAt.Before(sightings[j].SightedAt)
		}
		return sightings[i].SightingID < sightings[j].SightingID
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(sightings)
}

func (s *Server) handleApiTrajectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleApiGetTrajectory(w, r)
}

func (s *Server) handleApiGetTrajectory(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	petBytes, err := s.stateStore.GetState(r.Context(), store.LostPetsCollection, petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve lost pet", http.StatusInternalServerError)
		return
	}

	var pet domain.LostPetRecord
	if err := json.Unmarshal(petBytes, &pet); err != nil {
		http.Error(w, "Failed to decode lost pet", http.StatusInternalServerError)
		return
	}
	pet = domain.NormalizeLostPetRecord(pet)

	rawItems, err := s.stateStore.ListState(r.Context(), store.SightingsCollection)
	if err != nil && !errors.Is(err, store.ErrStoreNotFound) && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "Failed to list sightings", http.StatusInternalServerError)
		return
	}

	sightings := make([]domain.PetSightingRecord, 0)
	for _, b := range rawItems {
		var sRec domain.PetSightingRecord
		if err := json.Unmarshal(b, &sRec); err != nil {
			continue
		}
		if sRec.LostPetID == petID && sRec.Status == domain.SightingStatusActive {
			sightings = append(sightings, sRec)
		}
	}

	analysis := sighting.CalculateTrajectory(petID, pet.Coordinates, pet.ReportedAt, sightings)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(analysis)
}
