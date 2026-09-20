package webfrontend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const maxBreadcrumbsBodyBytes = 1048576 // 1MB

// handleApiSectorBreadcrumbs routes GET and POST requests for a sector's volunteer GPS breadcrumbs.
func (s *Server) handleApiSectorBreadcrumbs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleApiRecordBreadcrumbs(w, r)
	case http.MethodGet:
		s.handleApiGetBreadcrumbs(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleApiRecordBreadcrumbs ingests GPS breadcrumb points, persists them to the state store,
// and broadcasts an SSE breadcrumb_updated event via ReunionHub.
func (s *Server) handleApiRecordBreadcrumbs(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	sectorID := strings.TrimSpace(r.PathValue("sectorID"))
	if petID == "" || sectorID == "" {
		http.NotFound(w, r)
		return
	}

	// 1. Locate search party by petID or partyID
	party, found, err := s.resolveSearchParty(r.Context(), petID)
	if err != nil {
		http.Error(w, "Failed to load search party", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	// 2. Validate sector exists in party
	sectorExists := false
	for _, sec := range party.Sectors {
		if sec.SectorID == sectorID {
			sectorExists = true
			break
		}
	}
	if !sectorExists {
		http.NotFound(w, r)
		return
	}

	// 3. Decode incoming batch
	type recordRequest struct {
		TrailID        string                        `json:"trailId"`
		VolunteerAlias string                        `json:"volunteerAlias"`
		Points         []searchparty.BreadcrumbPoint `json:"points"`
	}

	var req recordRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBreadcrumbsBodyBytes)).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if len(req.Points) == 0 {
		http.Error(w, "at least one breadcrumb point is required", http.StatusBadRequest)
		return
	}

	for i, pt := range req.Points {
		if err := pt.Validate(); err != nil {
			http.Error(w, fmt.Sprintf("invalid breadcrumb point at index %d: %v", i, err), http.StatusBadRequest)
			return
		}
	}

	// 4. Create or update trail
	trailID := strings.TrimSpace(req.TrailID)
	now := time.Now().UTC()
	var trail searchparty.VolunteerBreadcrumbTrail

	if trailID != "" {
		existingBytes, err := s.stateStore.GetState(r.Context(), store.BreadcrumbsCollection, trailID)
		if err == nil {
			var existing searchparty.VolunteerBreadcrumbTrail
			if err := json.Unmarshal(existingBytes, &existing); err == nil && existing.TrailID == trailID {
				trail = existing
				trail.AppendPoints(req.Points...)
				if req.VolunteerAlias != "" {
					trail.VolunteerAlias = sanitizeVolunteerAlias(req.VolunteerAlias, len(party.ActiveAssignments)+1)
				}
				trail.UpdatedAt = now
			}
		}
	}

	if trail.TrailID == "" {
		if trailID == "" {
			trailID = generateTrailID(sectorID)
		}
		alias := sanitizeVolunteerAlias(req.VolunteerAlias, len(party.ActiveAssignments)+1)
		trail = searchparty.VolunteerBreadcrumbTrail{
			TrailID:         trailID,
			SearchPartyID:   party.PartyID,
			SectorID:        sectorID,
			VolunteerAlias:  alias,
			Points:          req.Points,
			TotalDistanceM:  searchparty.CalculateTrailDistance(req.Points),
			DurationSeconds: 0,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if len(req.Points) >= 2 {
			diff := req.Points[len(req.Points)-1].Timestamp.Sub(req.Points[0].Timestamp).Seconds()
			if diff > 0 {
				trail.DurationSeconds = int(diff)
			}
		}
	}

	// 5. Save trail to state store
	trailBytes, err := json.Marshal(trail)
	if err != nil {
		http.Error(w, "Failed to serialize breadcrumbs trail", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.BreadcrumbsCollection, trail.TrailID, trailBytes); err != nil {
		http.Error(w, "Failed to persist breadcrumbs trail", http.StatusInternalServerError)
		return
	}

	// 6. Broadcast SSE event via ReunionHub
	s.broadcastBreadcrumbUpdate(r.Context(), party, trail)

	// 7. Return 201 Created
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(trail)
}

// handleApiGetBreadcrumbs retrieves all breadcrumb trails for a specific sector.
func (s *Server) handleApiGetBreadcrumbs(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	sectorID := strings.TrimSpace(r.PathValue("sectorID"))
	if petID == "" || sectorID == "" {
		http.NotFound(w, r)
		return
	}

	party, found, err := s.resolveSearchParty(r.Context(), petID)
	if err != nil {
		http.Error(w, "Failed to load search party", http.StatusInternalServerError)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}

	sectorExists := false
	for _, sec := range party.Sectors {
		if sec.SectorID == sectorID {
			sectorExists = true
			break
		}
	}
	if !sectorExists {
		http.NotFound(w, r)
		return
	}

	rawTrails, err := s.stateStore.ListState(r.Context(), store.BreadcrumbsCollection)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		http.Error(w, "Failed to list breadcrumbs", http.StatusInternalServerError)
		return
	}

	trails := make([]searchparty.VolunteerBreadcrumbTrail, 0)
	for _, b := range rawTrails {
		var tr searchparty.VolunteerBreadcrumbTrail
		if err := json.Unmarshal(b, &tr); err != nil {
			continue
		}
		if (tr.SearchPartyID == party.PartyID || tr.SearchPartyID == party.LostPetID || tr.SearchPartyID == petID) && tr.SectorID == sectorID {
			trails = append(trails, tr)
		}
	}

	sort.Slice(trails, func(i, j int) bool {
		return trails[i].CreatedAt.Before(trails[j].CreatedAt)
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(trails)
}

// resolveSearchParty looks up a SearchParty by petID, and falls back to partyID lookup.
func (s *Server) resolveSearchParty(ctx context.Context, petOrPartyID string) (searchparty.SearchParty, bool, error) {
	party, found, err := s.findSearchPartyByPetID(ctx, petOrPartyID)
	if err == nil && found {
		return party, true, nil
	}
	return s.getSearchPartyByID(ctx, petOrPartyID)
}

// broadcastBreadcrumbUpdate dispatches an SSE event across all channels subscribed to this pet or party.
func (s *Server) broadcastBreadcrumbUpdate(ctx context.Context, party searchparty.SearchParty, trail searchparty.VolunteerBreadcrumbTrail) {
	if s.reunionHub == nil {
		return
	}

	payload := struct {
		Type            string                               `json:"type"`
		PartyID         string                               `json:"partyId"`
		SectorID        string                               `json:"sectorId"`
		TrailID         string                               `json:"trailId"`
		VolunteerAlias  string                               `json:"volunteerAlias"`
		Points          []searchparty.BreadcrumbPoint        `json:"points"`
		TotalDistanceM  float64                              `json:"totalDistanceM"`
		DurationSeconds int                                  `json:"durationSeconds"`
		Trail           searchparty.VolunteerBreadcrumbTrail `json:"trail"`
	}{
		Type:            "breadcrumb_updated",
		PartyID:         party.PartyID,
		SectorID:        trail.SectorID,
		TrailID:         trail.TrailID,
		VolunteerAlias:  trail.VolunteerAlias,
		Points:          trail.Points,
		TotalDistanceM:  trail.TotalDistanceM,
		DurationSeconds: trail.DurationSeconds,
		Trail:           trail,
	}

	now := time.Now().UTC()
	event := domain.ReunionStreamEvent{
		EventID:   fmt.Sprintf("evt_breadcrumb_%s_%d", trail.TrailID, now.UnixNano()),
		Type:      domain.ReunionEventBreadcrumbUpdated,
		MatchID:   party.LostPetID,
		Timestamp: now,
		Payload:   payload,
	}

	// Broadcast to pet subscribers
	s.reunionHub.Broadcast(event)

	// Broadcast to party subscribers (if distinct)
	if party.PartyID != party.LostPetID {
		partyEvent := event
		partyEvent.MatchID = party.PartyID
		s.reunionHub.Broadcast(partyEvent)
	}

	// Broadcast to active matches associated with this pet
	if party.LostPetID != "" && s.stateStore != nil {
		if matchBytes, err := s.stateStore.ListState(ctx, store.MatchesCollection); err == nil {
			for _, mb := range matchBytes {
				var m domain.MatchRecord
				if err := json.Unmarshal(mb, &m); err == nil {
					if m.LostPetID == party.LostPetID || m.MatchedPetID == party.LostPetID || m.LostPet.PetID == party.LostPetID {
						matchEvent := event
						matchEvent.MatchID = m.MatchID
						s.reunionHub.Broadcast(matchEvent)
					}
				}
			}
		}
	}
}

func generateTrailID(sectorID string) string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("trail-%s-%d", sectorID, time.Now().UnixNano())
	}
	return fmt.Sprintf("trail-%s-%s", sectorID, hex.EncodeToString(random[:]))
}
