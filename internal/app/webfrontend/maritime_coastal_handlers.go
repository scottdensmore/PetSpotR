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

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/maritimecoastal"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastMaritimeRescueNano atomic.Int64

func nextMaritimeRescueNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastMaritimeRescueNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastMaritimeRescueNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// MaritimeRescuePageData represents template data passed to templates/maritime_coastal_rescue.html.
type MaritimeRescuePageData struct {
	Operations         []domain.MaritimeRescueOperation
	TotalOperations    int
	CriticalTideCount  int
	StagedVesselCount  int
	ExtractedSafeCount int
	Locale             string
}

func (s *Server) ensureMaritimeRescueStore() *maritimecoastal.Store {
	if s.maritimeRescueStore == nil {
		s.maritimeRescueStore = maritimecoastal.NewStore()
	}
	return s.maritimeRescueStore
}

func (s *Server) registerMaritimeCoastalRoutes() {
	s.mux.HandleFunc("GET /maritime-coastal-rescue", s.handleMaritimeCoastalRescuePage)
	s.mux.HandleFunc("GET /api/v1/maritime-rescue/operations", s.handleListMaritimeRescueOperations)
	s.mux.HandleFunc("POST /api/v1/maritime-rescue/evaluate", s.handleEvaluateMaritimeRescueOperation)
	s.mux.HandleFunc("POST /api/v1/maritime-rescue/operations/{id}/steps", s.handleUpdateMaritimeRescueStep)
	s.mux.HandleFunc("POST /api/v1/maritime-rescue/operations/{id}/step", s.handleUpdateMaritimeRescueStep)
	s.mux.HandleFunc("POST /api/v1/maritime-rescue/operations/{id}/clearance", s.handleIssueMaritimeRescueClearance)
	s.mux.HandleFunc("GET /api/v1/maritime-rescue/stream", s.handleMaritimeRescueStream)
}

func computeMaritimeRescueSummary(ops []domain.MaritimeRescueOperation) (total, criticalTide, stagedVessel, extractedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.MaritimeStatusTidalFloodAspirationCritical:
			criticalTide++
		case domain.MaritimeStatusHovercraftMudSledStaged:
			stagedVessel++
		case domain.MaritimeStatusExtractedHarborDeckSafe:
			extractedSafe++
		}
	}
	return total, criticalTide, stagedVessel, extractedSafe
}

func writeMaritimeRescueJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleMaritimeCoastalRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mcStore := s.ensureMaritimeRescueStore()
	ops := mcStore.List()
	total, criticalTide, stagedVessel, extractedSafe := computeMaritimeRescueSummary(ops)

	tmpl, err := template.New("maritime_coastal_rescue.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/maritime_coastal_rescue.html")
	if err != nil {
		http.Error(w, "Failed to load Maritime & Coastal Rescue template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := MaritimeRescuePageData{
		Operations:         ops,
		TotalOperations:    total,
		CriticalTideCount:  criticalTide,
		StagedVesselCount:  stagedVessel,
		ExtractedSafeCount: extractedSafe,
		Locale:             locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListMaritimeRescueOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mcStore := s.ensureMaritimeRescueStore()
	allOps := mcStore.List()

	zoneFilter := strings.TrimSpace(r.URL.Query().Get("zone"))
	if zoneFilter == "" {
		zoneFilter = strings.TrimSpace(r.URL.Query().Get("maritime_zone"))
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	filtered := make([]domain.MaritimeRescueOperation, 0, len(allOps))
	for _, op := range allOps {
		if zoneFilter != "" && !strings.EqualFold(string(op.MaritimeZone), zoneFilter) {
			continue
		}
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalTide, stagedVessel, extractedSafe := computeMaritimeRescueSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations":     filtered,
		"total":          total,
		"critical_count": criticalTide,
		"staged_count":   stagedVessel,
		"safe_count":     extractedSafe,
		"summary": map[string]int{
			"total_operations": total,
			"critical_tide":    criticalTide,
			"staged_vessel":    stagedVessel,
			"extracted_safe":   extractedSafe,
		},
	})
}

func (s *Server) handleEvaluateMaritimeRescueOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.MaritimeEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMaritimeRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	mcStore := s.ensureMaritimeRescueStore()
	created, err := mcStore.EvaluateAndSave(req)
	if err != nil {
		writeMaritimeRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextMaritimeRescueNano()).UTC()
	s.broadcastMaritimeRescueEvent("maritime_rescue_updated", map[string]any{
		"type":      "maritime_rescue_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateMaritimeRescueStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeMaritimeRescueJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var raw struct {
		StepID    string `json:"step_id"`
		StepIDAlt string `json:"stepId"`
		Completed bool   `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeMaritimeRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	stepID := strings.TrimSpace(raw.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(raw.StepIDAlt)
	}

	mcStore := s.ensureMaritimeRescueStore()
	if _, exists := mcStore.Get(opID); !exists {
		writeMaritimeRescueJSONError(w, http.StatusNotFound, "Maritime rescue operation not found")
		return
	}

	updated, err := mcStore.UpdateStep(opID, domain.MaritimeStepUpdateRequest{
		StepID:    stepID,
		Completed: raw.Completed,
	})
	if err != nil {
		writeMaritimeRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextMaritimeRescueNano()).UTC()
	s.broadcastMaritimeRescueEvent("maritime_rescue_updated", map[string]any{
		"type":      "maritime_rescue_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueMaritimeRescueClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeMaritimeRescueJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeMaritimeRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	mcStore := s.ensureMaritimeRescueStore()
	if _, exists := mcStore.Get(opID); !exists {
		writeMaritimeRescueJSONError(w, http.StatusNotFound, "Maritime rescue operation not found")
		return
	}

	updated, err := mcStore.IssueClearance(opID)
	if err != nil {
		writeMaritimeRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextMaritimeRescueNano()).UTC()
	s.broadcastMaritimeRescueEvent("maritime_rescue_updated", map[string]any{
		"type":      "maritime_rescue_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleMaritimeRescueStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("maritime_rescue")
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

func (s *Server) broadcastMaritimeRescueEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextMaritimeRescueNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "maritime_rescue",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "maritime_rescue",
			SenderNodeID:  "maritime-rescue-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
