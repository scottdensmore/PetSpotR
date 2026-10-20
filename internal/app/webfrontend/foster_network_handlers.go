package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/fosternetwork"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastFosterNano atomic.Int64
	fosterSeedMu   sync.Mutex
)

func nextFosterNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastFosterNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastFosterNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// FosterNetworkPageData represents template data passed to templates/foster_network.html.
type FosterNetworkPageData struct {
	Placements []domain.FosterPlacementMatch
	Locale     string
}

type createFosterPlacementRequest struct {
	PlacementID        string `json:"placementId"`
	PetID              string `json:"petId"`
	PetName            string `json:"petName"`
	Species            string `json:"species"`
	FosterHostName     string `json:"fosterHostName"`
	ContagionFlag      string `json:"contagionFlag"`
	MedicalNeedLevel   string `json:"medicalNeedLevel"`
	ResidentPetsCount  int    `json:"residentPetsCount"`
	HEPAIsolationReady bool   `json:"hepaIsolationReady"`
	MedCertVerified    bool   `json:"medCertVerified"`
}

type transitionFosterPlacementRequest struct {
	VerifyBiosecurityRig    bool   `json:"verifyBiosecurityRig"`
	ActivateFosterPlacement bool   `json:"activateFosterPlacement"`
	CompleteCheckID         string `json:"completeCheckId"`
	HEPAIsolationReady      *bool  `json:"hepaIsolationReady"`
	MedCertVerified         *bool  `json:"medCertVerified"`
}

func (s *Server) registerFosterNetworkRoutes() {
	s.mux.HandleFunc("GET /foster-network", s.handleFosterNetworkPage)
	s.mux.HandleFunc("GET /api/v1/foster-network/placements", s.handleListFosterPlacements)
	s.mux.HandleFunc("POST /api/v1/foster-network/placements", s.handleCreateFosterPlacement)
	s.mux.HandleFunc("POST /api/v1/foster-network/placements/{id}/transition", s.handleTransitionFosterPlacement)
	s.mux.HandleFunc("GET /api/v1/foster-network/stream", s.handleFosterNetworkStream)
}

func defaultFosterPlacements(now time.Time) []domain.FosterPlacementMatch {
	placements := make([]domain.FosterPlacementMatch, 0, 3)

	if p1, err := fosternetwork.EvaluateFosterPlacement(fosternetwork.CreateFosterPlacementInput{
		PlacementID:        "foster-match-101",
		PetID:              "pet-2026-002",
		PetName:            "Barnaby",
		Species:            "DOG",
		FosterHostName:     "Elena Vance (RN / Vet Tech)",
		ContagionFlag:      "PARVO_SUSPECTED",
		MedicalNeedLevel:   "INJECTABLE_SUBQ_FLUIDS",
		ResidentPetsCount:  2,
		HEPAIsolationReady: false,
		MedCertVerified:    false,
	}, now.Add(-1*time.Minute)); err == nil && p1 != nil {
		placements = append(placements, *p1)
	}

	if p2, err := fosternetwork.EvaluateFosterPlacement(fosternetwork.CreateFosterPlacementInput{
		PlacementID:        "foster-match-102",
		PetID:              "pet-2026-001",
		PetName:            "Willow",
		Species:            "CAT",
		FosterHostName:     "Marcus Sterling",
		ContagionFlag:      "RINGWORM_DERMATOPHYTE",
		MedicalNeedLevel:   "WOUND_DRESSING_BID",
		ResidentPetsCount:  0,
		HEPAIsolationReady: false,
		MedCertVerified:    false,
	}, now.Add(-3*time.Minute)); err == nil && p2 != nil {
		_ = fosternetwork.ApplyFosterPlacementTransition(p2, fosternetwork.FosterPlacementUpdateInput{
			VerifyBiosecurityRig: true,
		}, now.Add(-2*time.Minute))
		placements = append(placements, *p2)
	}

	if p3, err := fosternetwork.EvaluateFosterPlacement(fosternetwork.CreateFosterPlacementInput{
		PlacementID:        "foster-match-103",
		PetID:              "pet-2026-003",
		PetName:            "Otis",
		Species:            "DOG",
		FosterHostName:     "Dr. Clara Chen",
		ContagionFlag:      "NONE_CLEARED",
		MedicalNeedLevel:   "STANDARD_DECOMPRESSION",
		ResidentPetsCount:  0,
		HEPAIsolationReady: true,
		MedCertVerified:    true,
	}, now.Add(-5*time.Minute)); err == nil && p3 != nil {
		_ = fosternetwork.ApplyFosterPlacementTransition(p3, fosternetwork.FosterPlacementUpdateInput{
			ActivateFosterPlacement: true,
		}, now.Add(-4*time.Minute))
		placements = append(placements, *p3)
	}

	return placements
}

func (s *Server) ensureDefaultFosterPlacements(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	fosterSeedMu.Lock()
	defer fosterSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionFosterPlacementMatches)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultFosterPlacements(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionFosterPlacementMatches, item.PlacementID, data)
		}
	}
}

func (s *Server) loadFosterPlacements(ctx context.Context, petIDFilter, statusFilter string) []domain.FosterPlacementMatch {
	placements := make([]domain.FosterPlacementMatch, 0)
	if s.stateStore == nil {
		return placements
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionFosterPlacementMatches)
	if err != nil {
		return placements
	}
	for _, b := range raw {
		var item domain.FosterPlacementMatch
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.PlacementStatus), statusFilter) {
				continue
			}
			placements = append(placements, item)
		}
	}
	sort.Slice(placements, func(i, j int) bool {
		if placements[i].UpdatedAt != placements[j].UpdatedAt {
			return placements[i].UpdatedAt > placements[j].UpdatedAt
		}
		return placements[i].PlacementID < placements[j].PlacementID
	})
	return placements
}

func (s *Server) handleFosterNetworkPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultFosterPlacements(r.Context())
	placements := s.loadFosterPlacements(r.Context(), "", "")

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := FosterNetworkPageData{
		Placements: placements,
		Locale:     locale,
	}

	s.renderCockpitTemplate(w, r, "foster_network.html", "Foster Network", data)
}

func (s *Server) handleListFosterPlacements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultFosterPlacements(r.Context())
	q := r.URL.Query()
	placements := s.loadFosterPlacements(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"placements": placements,
	})
}

func (s *Server) handleCreateFosterPlacement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createFosterPlacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultFosterPlacements(r.Context())

	seqNano := nextFosterNano()
	now := time.Unix(0, seqNano).UTC()

	placementID := strings.TrimSpace(req.PlacementID)
	if placementID == "" {
		placementID = "foster-match-" + randomHex8()
	}

	placement, err := fosternetwork.EvaluateFosterPlacement(fosternetwork.CreateFosterPlacementInput{
		PlacementID:        placementID,
		PetID:              req.PetID,
		PetName:            req.PetName,
		Species:            req.Species,
		FosterHostName:     req.FosterHostName,
		ContagionFlag:      req.ContagionFlag,
		MedicalNeedLevel:   req.MedicalNeedLevel,
		ResidentPetsCount:  req.ResidentPetsCount,
		HEPAIsolationReady: req.HEPAIsolationReady,
		MedCertVerified:    req.MedCertVerified,
	}, now)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	placementBytes, err := json.Marshal(placement)
	if err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to encode foster placement")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionFosterPlacementMatches, placement.PlacementID, placementBytes); err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to save foster placement")
		return
	}

	s.broadcastFosterNetworkEvent("foster_placement_updated", map[string]any{
		"type":      "foster_placement_updated",
		"placement": placement,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, placement)
}

func (s *Server) handleTransitionFosterPlacement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	placementID := strings.TrimSpace(r.PathValue("id"))
	if placementID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "placement id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req transitionFosterPlacementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultFosterPlacements(r.Context())

	seqNano := nextFosterNano()
	now := time.Unix(0, seqNano).UTC()

	var (
		updatedPlacement domain.FosterPlacementMatch
		transErr         error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionFosterPlacementMatches, placementID, func(current []byte) ([]byte, error) {
		var existing domain.FosterPlacementMatch
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt foster placement: %w", err)
		}

		if tErr := fosternetwork.ApplyFosterPlacementTransition(&existing, fosternetwork.FosterPlacementUpdateInput{
			VerifyBiosecurityRig:    req.VerifyBiosecurityRig,
			ActivateFosterPlacement: req.ActivateFosterPlacement,
			CompleteCheckID:         req.CompleteCheckID,
			HEPAIsolationReady:      req.HEPAIsolationReady,
			MedCertVerified:         req.MedCertVerified,
		}, now); tErr != nil {
			transErr = tErr
			return nil, tErr
		}
		updatedPlacement = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "Foster placement not found")
			return
		}
		if transErr != nil {
			writeCockpitJSONError(w, http.StatusBadRequest, transErr.Error())
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to transition foster placement: %v", err))
		return
	}

	s.broadcastFosterNetworkEvent("foster_placement_updated", map[string]any{
		"type":      "foster_placement_updated",
		"placement": updatedPlacement,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updatedPlacement)
}

func (s *Server) handleFosterNetworkStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "foster_network")
}

func (s *Server) broadcastFosterNetworkEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("foster_network", "foster-network-hub", eventType, payload, now)
}
