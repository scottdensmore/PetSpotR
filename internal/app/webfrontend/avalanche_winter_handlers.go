package webfrontend

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/avalanchewinter"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastWinterRescueNano atomic.Int64

func nextWinterRescueNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastWinterRescueNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastWinterRescueNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// WinterRescuePageData represents template data passed to templates/avalanche_winter_rescue.html.
type WinterRescuePageData struct {
	Operations            []domain.WinterRescueOperation
	TotalOperations       int
	CriticalBurialAlerts  int
	StagedRewarmBivies    int
	NormothermicSafeCount int
	Locale                string
}

type winterRescueClearanceRequest struct {
	LeadSkiPatrolCallsign string `json:"leadSkiPatrolCallsign"`
	Notes                 string `json:"notes"`
}

func (s *Server) ensureWinterRescueStore() *avalanchewinter.Store {
	if s.winterRescueStore == nil {
		s.winterRescueStore = avalanchewinter.NewStore()
	}
	return s.winterRescueStore
}

func (s *Server) registerWinterRescueRoutes() {
	s.mux.HandleFunc("GET /avalanche-winter-rescue", s.handleAvalancheWinterRescuePage)
	s.mux.HandleFunc("GET /api/v1/winter-rescue/operations", s.handleListWinterRescueOperations)
	s.mux.HandleFunc("POST /api/v1/winter-rescue/evaluate", s.handleEvaluateWinterRescueOperation)
	s.mux.HandleFunc("POST /api/v1/winter-rescue/operations/{id}/step", s.handleUpdateWinterRescueStep)
	s.mux.HandleFunc("POST /api/v1/winter-rescue/operations/{id}/clearance", s.handleIssueWinterRescueClearance)
	s.mux.HandleFunc("GET /api/v1/winter-rescue/stream", s.handleWinterRescueStream)
}

func computeWinterRescueSummary(ops []domain.WinterRescueOperation) (total, criticalBurial, stagedBivies, normothermicSafe int) {
	total = len(ops)
	for _, op := range ops {
		if op.AsphyxiaAirPocketAlert || op.SevereFrostbiteAfterdropAlert || op.Status == domain.WinterRescueStatusBurialHypothermiaCritical {
			criticalBurial++
		}
		if op.Status == domain.WinterRescueStatusProbeTrenchRewarmingStaged {
			stagedBivies++
		}
		if op.Status == domain.WinterRescueStatusExtractedNormothermicSafe {
			normothermicSafe++
		}
	}
	return total, criticalBurial, stagedBivies, normothermicSafe
}

func writeWinterRescueJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleAvalancheWinterRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	awStore := s.ensureWinterRescueStore()
	ops := awStore.List()
	total, criticalBurial, stagedBivies, normothermicSafe := computeWinterRescueSummary(ops)

	tmpl, err := template.New("avalanche_winter_rescue.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/avalanche_winter_rescue.html")
	if err != nil {
		http.Error(w, "Failed to load Avalanche & Winter Rescue template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := WinterRescuePageData{
		Operations:            ops,
		TotalOperations:       total,
		CriticalBurialAlerts:  criticalBurial,
		StagedRewarmBivies:    stagedBivies,
		NormothermicSafeCount: normothermicSafe,
		Locale:                locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListWinterRescueOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	awStore := s.ensureWinterRescueStore()
	allOps := awStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	terrainFilter := strings.TrimSpace(r.URL.Query().Get("terrainType"))

	filtered := make([]domain.WinterRescueOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if terrainFilter != "" && !strings.EqualFold(string(op.TerrainType), terrainFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalBurial, stagedBivies, normothermicSafe := computeWinterRescueSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"totalOperations":       total,
			"criticalBurialAlerts":  criticalBurial,
			"stagedRewarmBivies":    stagedBivies,
			"normothermicSafeCount": normothermicSafe,
		},
	})
}

func (s *Server) handleEvaluateWinterRescueOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.WinterEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWinterRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	awStore := s.ensureWinterRescueStore()
	created, err := awStore.EvaluateAndSave(req)
	if err != nil {
		writeWinterRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextWinterRescueNano()).UTC()
	s.broadcastWinterRescueEvent("winter_rescue_updated", map[string]any{
		"type":      "winter_rescue_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateWinterRescueStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeWinterRescueJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.WinterStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWinterRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	awStore := s.ensureWinterRescueStore()
	if _, exists := awStore.Get(opID); !exists {
		writeWinterRescueJSONError(w, http.StatusNotFound, "Winter rescue operation not found")
		return
	}

	updated, err := awStore.UpdateStep(opID, req)
	if err != nil {
		writeWinterRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextWinterRescueNano()).UTC()
	s.broadcastWinterRescueEvent("winter_rescue_updated", map[string]any{
		"type":      "winter_rescue_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueWinterRescueClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeWinterRescueJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req winterRescueClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeWinterRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	awStore := s.ensureWinterRescueStore()
	if _, exists := awStore.Get(opID); !exists {
		writeWinterRescueJSONError(w, http.StatusNotFound, "Winter rescue operation not found")
		return
	}

	updated, err := awStore.IssueClearance(opID, req.LeadSkiPatrolCallsign, req.Notes)
	if err != nil {
		writeWinterRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextWinterRescueNano()).UTC()
	s.broadcastWinterRescueEvent("winter_rescue_updated", map[string]any{
		"type":      "winter_rescue_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleWinterRescueStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

	subCh, unsubscribe := s.reunionHub.Subscribe("winter_rescue")
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

func (s *Server) broadcastWinterRescueEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextWinterRescueNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "winter_rescue",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "winter_rescue",
			SenderNodeID:  "winter-rescue-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
