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

	"github.com/scottdensmore/petspotr/pkg/beacon"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

const (
	maxBeaconPingBodyBytes   = 65536
	defaultBeaconTxPower1m   = -59
	beaconPingWindowDuration = 15 * time.Minute
)

// BeaconPingSubmissionRequest represents the client payload for reporting a BLE beacon ping.
type BeaconPingSubmissionRequest struct {
	VolunteerAlias string               `json:"volunteerAlias"`
	ObserverCoords domain.LocationPoint `json:"observerCoords"`
	RSSI           int                  `json:"rssi"`
	TxPower1m      int                  `json:"txPower1m"`
	RecordedAt     *time.Time           `json:"recordedAt,omitempty"`
}

// BeaconPingSubmissionResponse represents the server response after ingesting a beacon ping.
type BeaconPingSubmissionResponse struct {
	Ping          beacon.BeaconPing          `json:"ping"`
	Triangulation beacon.TriangulationResult `json:"triangulation"`
}

// BeaconTriangulationResponse represents the response when querying beacon triangulation for a pet.
type BeaconTriangulationResponse struct {
	PetID         string                     `json:"petId"`
	Triangulation beacon.TriangulationResult `json:"triangulation"`
	RecentPings   []beacon.BeaconPing        `json:"recentPings"`
}

// handleBeaconPingSubmission ingests volunteer BLE RSSI pings and broadcasts triangulation updates.
func (s *Server) handleBeaconPingSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petId"))
	if petID == "" {
		petID = strings.TrimSpace(r.PathValue("petID"))
	}
	if petID == "" {
		http.Error(w, "petId is required", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBeaconPingBodyBytes)
	var req BeaconPingSubmissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if err := req.ObserverCoords.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("invalid observer coordinates: %v", err), http.StatusBadRequest)
		return
	}

	if req.RSSI < -120 || req.RSSI > 0 {
		http.Error(w, "rssi must be between -120 and 0 dBm", http.StatusBadRequest)
		return
	}

	txPower := req.TxPower1m
	if txPower == 0 {
		txPower = defaultBeaconTxPower1m
	}

	now := time.Now().UTC()
	recordedAt := now
	if req.RecordedAt != nil && !req.RecordedAt.IsZero() {
		recordedAt = req.RecordedAt.UTC()
	}

	volunteerAlias := strings.TrimSpace(req.VolunteerAlias)
	if volunteerAlias != "" {
		volunteerAlias = sanitizeVolunteerAlias(volunteerAlias, 1)
	} else {
		volunteerAlias = "Volunteer Alpha"
	}

	dist := beacon.EstimateDistance(req.RSSI, txPower, beacon.DefaultPathLossExponent)
	pingID := generateBeaconPingID(recordedAt)

	ping := beacon.BeaconPing{
		PingID:         pingID,
		PetID:          petID,
		VolunteerAlias: volunteerAlias,
		ObserverCoords: req.ObserverCoords,
		RSSI:           req.RSSI,
		TxPower1m:      txPower,
		DistanceMeters: dist,
		RecordedAt:     recordedAt,
	}

	pingBytes, err := json.Marshal(ping)
	if err != nil {
		http.Error(w, "Failed to serialize beacon ping", http.StatusInternalServerError)
		return
	}

	storeKey := petID + ":" + pingID
	if err := s.stateStore.SaveState(r.Context(), store.BeaconPingsCollection, storeKey, pingBytes); err != nil {
		http.Error(w, "Failed to save beacon ping", http.StatusInternalServerError)
		return
	}

	// Retrieve recent pings for pet within last 15 minutes
	pings, err := s.getRecentBeaconPings(r.Context(), petID, now)
	if err != nil {
		http.Error(w, "Failed to load recent beacon pings", http.StatusInternalServerError)
		return
	}

	triangulation := beacon.TriangulateBeacon(pings)

	// SSE Broadcast if hub is available
	if s.reunionHub != nil {
		s.broadcastBeaconPing(r.Context(), petID, ping, triangulation)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(BeaconPingSubmissionResponse{
		Ping:          ping,
		Triangulation: triangulation,
	})
}

// handleGetBeaconTriangulation returns the current triangulation result and recent pings for a pet.
func (s *Server) handleGetBeaconTriangulation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petId"))
	if petID == "" {
		petID = strings.TrimSpace(r.PathValue("petID"))
	}
	if petID == "" {
		http.Error(w, "petId is required", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	pings, err := s.getRecentBeaconPings(r.Context(), petID, now)
	if err != nil {
		http.Error(w, "Failed to load recent beacon pings", http.StatusInternalServerError)
		return
	}

	triangulation := beacon.TriangulateBeacon(pings)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(BeaconTriangulationResponse{
		PetID:         petID,
		Triangulation: triangulation,
		RecentPings:   pings,
	})
}

// getRecentBeaconPings fetches pings from the state store for petID within the last 15 minutes.
func (s *Server) getRecentBeaconPings(ctx context.Context, petID string, now time.Time) ([]beacon.BeaconPing, error) {
	if s.stateStore == nil {
		return []beacon.BeaconPing{}, nil
	}

	rawItems, err := s.stateStore.ListState(ctx, store.BeaconPingsCollection)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			return []beacon.BeaconPing{}, nil
		}
		return nil, err
	}

	cutoff := now.Add(-beaconPingWindowDuration)
	var pings []beacon.BeaconPing
	for _, raw := range rawItems {
		var p beacon.BeaconPing
		if err := json.Unmarshal(raw, &p); err == nil {
			if p.PetID == petID && (p.RecordedAt.After(cutoff) || p.RecordedAt.Equal(cutoff)) {
				pings = append(pings, p)
			}
		}
	}

	sort.Slice(pings, func(i, j int) bool {
		return pings[i].RecordedAt.After(pings[j].RecordedAt)
	})

	if pings == nil {
		pings = []beacon.BeaconPing{}
	}
	return pings, nil
}

// broadcastBeaconPing emits ReunionEventBeaconPing over the reunionHub.
func (s *Server) broadcastBeaconPing(ctx context.Context, petID string, ping beacon.BeaconPing, triangulation beacon.TriangulationResult) {
	if s.reunionHub == nil {
		return
	}

	payload := domain.BeaconPingEventPayload{
		Type:          string(domain.ReunionEventBeaconPing),
		PetID:         petID,
		Ping:          ping,
		Triangulation: triangulation,
	}

	now := time.Now().UTC()
	event := domain.ReunionStreamEvent{
		EventID:   fmt.Sprintf("evt_beacon_%s_%d", petID, now.UnixNano()),
		Type:      domain.ReunionEventBeaconPing,
		MatchID:   petID,
		Timestamp: now,
		Payload:   payload,
	}

	// 1. Broadcast to petID subscribers
	s.reunionHub.Broadcast(event)

	// 2. Broadcast to search party subscribers
	partyID := fmt.Sprintf("party-%s", petID)
	partyEvent := event
	partyEvent.MatchID = partyID
	s.reunionHub.Broadcast(partyEvent)

	// 3. Broadcast to any active matches associated with this lost pet
	if s.stateStore != nil {
		if matchBytes, err := s.stateStore.ListState(ctx, store.MatchesCollection); err == nil {
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
}

func generateBeaconPingID(t time.Time) string {
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("ping_%d", t.UnixNano())
	}
	return fmt.Sprintf("ping_%d_%s", t.UnixNano(), hex.EncodeToString(random[:]))
}
