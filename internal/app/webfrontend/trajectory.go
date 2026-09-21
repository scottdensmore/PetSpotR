package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sighting"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// handleApiTrajectory routes GET requests for a lost pet's trajectory analysis.
func (s *Server) handleApiTrajectory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleGetTrajectory(w, r)
}

// handleGetTrajectory computes trajectory analysis augmented with predictive modeling.
func (s *Server) handleGetTrajectory(w http.ResponseWriter, r *http.Request) {
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
		petID = strings.TrimSpace(r.PathValue("id"))
	}
	if petID == "" {
		petID = strings.TrimSpace(r.PathValue("petId"))
	}
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

	sightings, err := s.getActiveSightingsForPet(r.Context(), petID)
	if err != nil {
		http.Error(w, "Failed to retrieve pet sightings", http.StatusInternalServerError)
		return
	}

	originCoords := pet.Coordinates
	if originCoords == nil && pet.Location != "" {
		if pt, ok := extractCoordinates(nil, pet.Location); ok {
			originCoords = &pt
		}
	}

	analysis := sighting.CalculateTrajectory(petID, originCoords, pet.ReportedAt, sightings)

	// Determine species (defaulting to "dog" or "general" if unspecified)
	species := strings.ToLower(strings.TrimSpace(pet.Species))
	if species == "" {
		species = "dog"
	}

	// Determine elapsedHours
	var elapsedHours float64
	if qElapsed := r.URL.Query().Get("elapsedHours"); qElapsed != "" {
		if v, err := strconv.ParseFloat(qElapsed, 64); err == nil && v > 0 {
			elapsedHours = v
		}
	}
	if elapsedHours <= 0 {
		var lastTime time.Time
		if len(analysis.OrderedSightings) > 0 {
			lastTime = analysis.OrderedSightings[len(analysis.OrderedSightings)-1].SightedAt
		} else if !pet.ReportedAt.IsZero() {
			lastTime = pet.ReportedAt
		}
		if !lastTime.IsZero() {
			diff := time.Since(lastTime)
			if diff > 0 {
				elapsedHours = diff.Hours()
			}
		}
	}
	if elapsedHours <= 0 {
		elapsedHours = 1.0
	}

	predModel := sighting.GeneratePredictiveTrajectory(petID, species, originCoords, sightings, elapsedHours)
	analysis.PredictiveModel = &predModel

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(analysis)
}

// handleGetPredictiveTrajectoryScenario handles GET /api/v1/lost-pets/{id}/predictive-trajectory,
// computing a hypothetical scenario parameterized by elapsedHours and species.
func (s *Server) handleGetPredictiveTrajectoryScenario(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		petID = strings.TrimSpace(r.PathValue("id"))
	}
	if petID == "" {
		petID = strings.TrimSpace(r.PathValue("petId"))
	}
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

	species := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("species")))
	if species == "" {
		species = strings.ToLower(strings.TrimSpace(pet.Species))
	}
	if species == "" {
		species = "dog"
	}

	sightings, err := s.getActiveSightingsForPet(r.Context(), petID)
	if err != nil {
		http.Error(w, "Failed to retrieve pet sightings", http.StatusInternalServerError)
		return
	}

	var elapsedHours float64
	if qElapsed := r.URL.Query().Get("elapsedHours"); qElapsed != "" {
		if v, err := strconv.ParseFloat(qElapsed, 64); err == nil && v > 0 {
			elapsedHours = v
		}
	}
	if elapsedHours <= 0 {
		var lastTime time.Time
		if len(sightings) > 0 {
			for _, sRec := range sightings {
				if sRec.SightedAt.After(lastTime) {
					lastTime = sRec.SightedAt
				}
			}
		} else if !pet.ReportedAt.IsZero() {
			lastTime = pet.ReportedAt
		}
		if !lastTime.IsZero() {
			diff := time.Since(lastTime)
			if diff > 0 {
				elapsedHours = diff.Hours()
			}
		}
	}
	if elapsedHours <= 0 {
		elapsedHours = 1.0
	}

	originCoords := pet.Coordinates
	if originCoords == nil && pet.Location != "" {
		if pt, ok := extractCoordinates(nil, pet.Location); ok {
			originCoords = &pt
		}
	}

	predResult := sighting.GeneratePredictiveTrajectory(petID, species, originCoords, sightings, elapsedHours)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(predResult)
}

// getActiveSightingsForPet retrieves all active sightings for petID from stateStore.
func (s *Server) getActiveSightingsForPet(ctx context.Context, petID string) ([]domain.PetSightingRecord, error) {
	if s.stateStore == nil {
		return nil, nil
	}
	rawItems, err := s.stateStore.ListState(ctx, store.SightingsCollection)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return nil, nil
		}
		return nil, err
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
	return sightings, nil
}
