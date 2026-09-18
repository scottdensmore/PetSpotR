package webfrontend

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	defaultReunionPingInterval = 15 * time.Second
	maxPresenceBodyBytes       = 65536
)

// ReunionPresenceRequest defines the incoming payload for ephemeral presence updates.
type ReunionPresenceRequest struct {
	MatchID    string                      `json:"matchId"`
	SenderRole domain.MatchParticipantRole `json:"senderRole,omitempty"`
	Status     string                      `json:"status"`
}

// handleApiReunionEvents serves the real-time SSE stream for a verified match participant.
func (s *Server) handleApiReunionEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principal := s.authenticatedPrincipal(r)
	if principal == nil {
		http.NotFound(w, r)
		return
	}

	matchID := strings.TrimSpace(r.URL.Query().Get("matchId"))
	if matchID == "" {
		http.Error(w, "matchId is required", http.StatusBadRequest)
		return
	}

	_, err := s.loadAuthorizedMediatedThread(r.Context(), matchID, *principal)
	if errors.Is(err, errMediatedThreadHidden) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load mediated thread", http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	if s.reunionHub == nil {
		return
	}

	subCh, unsubscribe := s.reunionHub.Subscribe(matchID)
	defer unsubscribe()

	pingInterval := s.reunionPingInterval
	if pingInterval <= 0 {
		pingInterval = defaultReunionPingInterval
	}
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-subCh:
			if !ok {
				return
			}
			if err := formatSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// handleApiReunionPresence handles ephemeral presence updates (typing, online, idle)
// and broadcasts them via the ReunionHub without exposing any PII.
func (s *Server) handleApiReunionPresence(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	principal := s.authenticatedPrincipal(r)
	if principal == nil {
		http.NotFound(w, r)
		return
	}

	if !s.validCSRF(r) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxPresenceBodyBytes)
	var req ReunionPresenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	req.MatchID = strings.TrimSpace(req.MatchID)
	req.Status = strings.TrimSpace(req.Status)
	if req.MatchID == "" || req.Status == "" {
		http.Error(w, "matchId and status are required", http.StatusBadRequest)
		return
	}

	switch req.Status {
	case "typing", "online", "idle":
	default:
		http.Error(w, "Invalid presence status", http.StatusBadRequest)
		return
	}

	participants, err := s.loadAuthorizedMediatedThread(r.Context(), req.MatchID, *principal)
	if errors.Is(err, errMediatedThreadHidden) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to load mediated thread", http.StatusInternalServerError)
		return
	}

	var role domain.MatchParticipantRole
	if principalMatchesRef(*principal, participants.Reporter) {
		role = domain.MatchParticipantRoleReporter
	} else if principalMatchesRef(*principal, participants.Finder) {
		role = domain.MatchParticipantRoleFinder
	} else {
		http.NotFound(w, r)
		return
	}

	if req.SenderRole != "" && req.SenderRole != role {
		http.Error(w, "Mismatched participant role", http.StatusBadRequest)
		return
	}

	if s.reunionHub != nil {
		s.reunionHub.BroadcastLocal(domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_presence_%d", time.Now().UnixNano()),
			Type:      domain.ReunionEventPresence,
			MatchID:   req.MatchID,
			Timestamp: time.Now().UTC(),
			Payload: domain.ReunionPresencePayload{
				SenderRole: role,
				Status:     req.Status,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func formatSSEEvent(w io.Writer, event domain.ReunionStreamEvent) error {
	payloadBytes, err := json.Marshal(event.Payload)
	if err != nil {
		return err
	}
	eventID := event.EventID
	if eventID == "" {
		eventID = fmt.Sprintf("evt_%d", event.Timestamp.UnixNano())
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", eventID, event.Type, payloadBytes)
	return err
}
