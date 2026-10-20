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
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/transportrelay"
)

var (
	lastTransportRelayNano atomic.Int64
	transportRelaySeedMu   sync.Mutex
)

func nextTransportRelayNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastTransportRelayNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastTransportRelayNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// TransportRelayPageData represents template data passed to templates/transport_relay.html.
type TransportRelayPageData struct {
	Missions []domain.TransportRelayMission
	Locale   string
}

type createTransportRelayMissionRequest struct {
	MissionID          string `json:"missionId"`
	PetID              string `json:"petId"`
	PetName            string `json:"petName"`
	Species            string `json:"species"`
	OriginHub          string `json:"originHub"`
	DestinationHub     string `json:"destinationHub"`
	TransportMode      string `json:"transportMode"`
	TotalDistanceMiles int    `json:"totalDistanceMiles"`
	CruiseAltitudeFeet int    `json:"cruiseAltitudeFeet"`
	BrachycephalicFlag bool   `json:"brachycephalicFlag"`
	USDACVICertified   bool   `json:"usdaCviCertified"`
	IATACrateVerified  bool   `json:"iataCrateVerified"`
}

type advanceTransportRelayMissionRequest struct {
	ClearPreFlightAndLaunch bool   `json:"clearPreFlightAndLaunch"`
	CompleteRelayDelivery   bool   `json:"completeRelayDelivery"`
	CompleteLegID           string `json:"completeLegId"`
	USDACVICertified        *bool  `json:"usdaCviCertified"`
	IATACrateVerified       *bool  `json:"iataCrateVerified"`
}

func (s *Server) registerTransportRelayRoutes() {
	s.mux.HandleFunc("GET /transport-relay", s.handleTransportRelayPage)
	s.mux.HandleFunc("GET /api/v1/transport-relay/missions", s.handleListTransportRelayMissions)
	s.mux.HandleFunc("POST /api/v1/transport-relay/missions", s.handleCreateTransportRelayMission)
	s.mux.HandleFunc("POST /api/v1/transport-relay/missions/{id}/advance", s.handleAdvanceTransportRelayMission)
	s.mux.HandleFunc("GET /api/v1/transport-relay/stream", s.handleTransportRelayStream)
}

func defaultTransportRelayMissions(now time.Time) []domain.TransportRelayMission {
	missions := make([]domain.TransportRelayMission, 0, 3)

	if m1, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID:          "relay-msn-101",
		PetID:              "pet-2026-001",
		PetName:            "Winston",
		Species:            "DOG",
		OriginHub:          "KBOI Boise Air Terminal FBO",
		DestinationHub:     "KBFI Boeing Field Seattle",
		TransportMode:      "GENERAL_AVIATION_AIRLIFT",
		TotalDistanceMiles: 410,
		CruiseAltitudeFeet: 8500,
		BrachycephalicFlag: true,
		USDACVICertified:   false,
		IATACrateVerified:  false,
	}, now.Add(-1*time.Minute)); err == nil && m1 != nil {
		missions = append(missions, *m1)
	}

	if m2, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID:          "relay-msn-102",
		PetID:              "pet-2026-002",
		PetName:            "Saffron",
		Species:            "CAT",
		OriginHub:          "KPDX Portland Hillsboro Relay",
		DestinationHub:     "KPAE Paine Field Snohomish",
		TransportMode:      "HYBRID_AIR_GROUND_CORRIDOR",
		TotalDistanceMiles: 195,
		CruiseAltitudeFeet: 5500,
		BrachycephalicFlag: false,
		USDACVICertified:   false,
		IATACrateVerified:  false,
	}, now.Add(-3*time.Minute)); err == nil && m2 != nil {
		_ = transportrelay.ApplyRelayMissionUpdate(m2, transportrelay.RelayMissionUpdateInput{
			ClearPreFlightAndLaunch: true,
		}, now.Add(-2*time.Minute))
		missions = append(missions, *m2)
	}

	if m3, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID:          "relay-msn-103",
		PetID:              "pet-2026-003",
		PetName:            "Kodiak",
		Species:            "DOG",
		OriginHub:          "KGEG Spokane Felts Field",
		DestinationHub:     "KRNT Renton Municipal",
		TransportMode:      "CLIMATE_CONTROLLED_VAN_RELAY",
		TotalDistanceMiles: 280,
		CruiseAltitudeFeet: 2500,
		BrachycephalicFlag: false,
		USDACVICertified:   true,
		IATACrateVerified:  true,
	}, now.Add(-5*time.Minute)); err == nil && m3 != nil {
		_ = transportrelay.ApplyRelayMissionUpdate(m3, transportrelay.RelayMissionUpdateInput{
			CompleteRelayDelivery: true,
		}, now.Add(-4*time.Minute))
		missions = append(missions, *m3)
	}

	return missions
}

func (s *Server) ensureDefaultTransportRelayMissions(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	transportRelaySeedMu.Lock()
	defer transportRelaySeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionTransportRelayMissions)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC().Truncate(time.Second)
	for _, item := range defaultTransportRelayMissions(now) {
		if data, err := json.Marshal(item); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionTransportRelayMissions, item.MissionID, data)
		}
	}
}

func (s *Server) loadTransportRelayMissions(ctx context.Context, petIDFilter, statusFilter string) []domain.TransportRelayMission {
	missions := make([]domain.TransportRelayMission, 0)
	if s.stateStore == nil {
		return missions
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionTransportRelayMissions)
	if err != nil {
		return missions
	}
	for _, b := range raw {
		var item domain.TransportRelayMission
		if err := json.Unmarshal(b, &item); err == nil {
			if petIDFilter != "" && !strings.EqualFold(item.PetID, petIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(item.RelayStatus), statusFilter) {
				continue
			}
			missions = append(missions, item)
		}
	}
	sort.Slice(missions, func(i, j int) bool {
		if missions[i].UpdatedAt != missions[j].UpdatedAt {
			return missions[i].UpdatedAt > missions[j].UpdatedAt
		}
		return missions[i].MissionID < missions[j].MissionID
	})
	return missions
}

func (s *Server) handleTransportRelayPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultTransportRelayMissions(r.Context())
	missions := s.loadTransportRelayMissions(r.Context(), "", "")

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := TransportRelayPageData{
		Missions: missions,
		Locale:   locale,
	}

	s.renderCockpitTemplate(w, r, "transport_relay.html", "Transport Relay", data)
}

func (s *Server) handleListTransportRelayMissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultTransportRelayMissions(r.Context())
	q := r.URL.Query()
	missions := s.loadTransportRelayMissions(
		r.Context(),
		strings.TrimSpace(q.Get("petId")),
		strings.TrimSpace(q.Get("status")),
	)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"missions": missions,
	})
}

func (s *Server) handleCreateTransportRelayMission(w http.ResponseWriter, r *http.Request) {
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
	var req createTransportRelayMissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultTransportRelayMissions(r.Context())

	seqNano := nextTransportRelayNano()
	now := time.Unix(0, seqNano).UTC()

	missionID := strings.TrimSpace(req.MissionID)
	if missionID == "" {
		missionID = "relay-msn-" + randomHex8()
	}

	mission, err := transportrelay.PlanTransportRelayMission(transportrelay.CreateRelayMissionInput{
		MissionID:          missionID,
		PetID:              req.PetID,
		PetName:            req.PetName,
		Species:            req.Species,
		OriginHub:          req.OriginHub,
		DestinationHub:     req.DestinationHub,
		TransportMode:      req.TransportMode,
		TotalDistanceMiles: req.TotalDistanceMiles,
		CruiseAltitudeFeet: req.CruiseAltitudeFeet,
		BrachycephalicFlag: req.BrachycephalicFlag,
		USDACVICertified:   req.USDACVICertified,
		IATACrateVerified:  req.IATACrateVerified,
	}, now)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	missionBytes, err := json.Marshal(mission)
	if err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to encode transport relay mission")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionTransportRelayMissions, mission.MissionID, missionBytes); err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to save transport relay mission")
		return
	}

	s.broadcastTransportRelayEvent("transport_relay_updated", map[string]any{
		"type":    "transport_relay_updated",
		"mission": mission,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, mission)
}

func (s *Server) handleAdvanceTransportRelayMission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	missionID := strings.TrimSpace(r.PathValue("id"))
	if missionID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req advanceTransportRelayMissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultTransportRelayMissions(r.Context())

	seqNano := nextTransportRelayNano()
	now := time.Unix(0, seqNano).UTC()

	var (
		updatedMission domain.TransportRelayMission
		advErr         error
	)

	err := s.stateStore.UpdateState(r.Context(), store.CollectionTransportRelayMissions, missionID, func(current []byte) ([]byte, error) {
		var existing domain.TransportRelayMission
		if err := json.Unmarshal(current, &existing); err != nil {
			return nil, fmt.Errorf("corrupt transport relay mission: %w", err)
		}

		if aErr := transportrelay.ApplyRelayMissionUpdate(&existing, transportrelay.RelayMissionUpdateInput{
			ClearPreFlightAndLaunch: req.ClearPreFlightAndLaunch,
			CompleteRelayDelivery:   req.CompleteRelayDelivery,
			CompleteLegID:           req.CompleteLegID,
			USDACVICertified:        req.USDACVICertified,
			IATACrateVerified:       req.IATACrateVerified,
		}, now); aErr != nil {
			advErr = aErr
			return nil, aErr
		}
		updatedMission = existing
		return json.Marshal(existing)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "Transport relay mission not found")
			return
		}
		if advErr != nil {
			writeCockpitJSONError(w, http.StatusBadRequest, advErr.Error())
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to advance transport relay mission: %v", err))
		return
	}

	s.broadcastTransportRelayEvent("transport_relay_updated", map[string]any{
		"type":    "transport_relay_updated",
		"mission": updatedMission,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updatedMission)
}

func (s *Server) handleTransportRelayStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "transport_relay")
}

func (s *Server) broadcastTransportRelayEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("transport_relay", "transport-relay-hub", eventType, payload, now)
}
