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
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/reptileectotherm"
)

var lastEctothermRescueNano atomic.Int64

func nextEctothermRescueNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastEctothermRescueNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastEctothermRescueNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// EctothermRescuePageData represents template data passed to templates/reptile_ectotherm_rescue.html.
type EctothermRescuePageData struct {
	Operations           []domain.EctothermRescueOperation
	TotalOperations      int
	CriticalTorporAlerts int
	StagedBaskingLures   int
	ThermalSafeCount     int
	Locale               string
}

type ectothermRescueClearanceRequest struct {
	LeadHerpTechCallsign string `json:"leadHerpTechCallsign"`
	Notes                string `json:"notes"`
}

func (s *Server) ensureEctothermRescueStore() *reptileectotherm.Store {
	if s.ectothermRescueStore == nil {
		s.ectothermRescueStore = reptileectotherm.NewStore()
	}
	return s.ectothermRescueStore
}

func (s *Server) registerEctothermRescueRoutes() {
	s.mux.HandleFunc("GET /reptile-ectotherm-rescue", s.handleReptileEctothermRescuePage)
	s.mux.HandleFunc("GET /api/v1/ectotherm-rescue/operations", s.handleListEctothermRescueOperations)
	s.mux.HandleFunc("POST /api/v1/ectotherm-rescue/evaluate", s.handleEvaluateEctothermRescueOperation)
	s.mux.HandleFunc("POST /api/v1/ectotherm-rescue/operations/{id}/step", s.handleUpdateEctothermRescueStep)
	s.mux.HandleFunc("POST /api/v1/ectotherm-rescue/operations/{id}/clearance", s.handleIssueEctothermRescueClearance)
	s.mux.HandleFunc("GET /api/v1/ectotherm-rescue/stream", s.handleEctothermRescueStream)
}

func computeEctothermRescueSummary(ops []domain.EctothermRescueOperation) (total, criticalTorpor, stagedLures, thermalSafe int) {
	total = len(ops)
	for _, op := range ops {
		if op.CriticalTorporAlert || op.DesiccationDysecdysisAlert || op.Status == domain.EctothermRescueStatusColdTorporBrumationCritical {
			criticalTorpor++
		}
		if op.Status == domain.EctothermRescueStatusBaskingLureCreviceStaged {
			stagedLures++
		}
		if op.Status == domain.EctothermRescueStatusRecoveredThermalGradientSafe {
			thermalSafe++
		}
	}
	return total, criticalTorpor, stagedLures, thermalSafe
}

func writeEctothermRescueJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleReptileEctothermRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reStore := s.ensureEctothermRescueStore()
	ops := reStore.List()
	total, criticalTorpor, stagedLures, thermalSafe := computeEctothermRescueSummary(ops)

	tmpl, err := template.New("reptile_ectotherm_rescue.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/reptile_ectotherm_rescue.html")
	if err != nil {
		http.Error(w, "Failed to load Reptile & Ectotherm Rescue template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := EctothermRescuePageData{
		Operations:           ops,
		TotalOperations:      total,
		CriticalTorporAlerts: criticalTorpor,
		StagedBaskingLures:   stagedLures,
		ThermalSafeCount:     thermalSafe,
		Locale:               locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListEctothermRescueOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reStore := s.ensureEctothermRescueStore()
	allOps := reStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	taxonFilter := strings.TrimSpace(r.URL.Query().Get("taxonType"))

	filtered := make([]domain.EctothermRescueOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if taxonFilter != "" && !strings.EqualFold(string(op.TaxonType), taxonFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalTorpor, stagedLures, thermalSafe := computeEctothermRescueSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"totalOperations":      total,
			"criticalTorporAlerts": criticalTorpor,
			"stagedBaskingLures":   stagedLures,
			"thermalSafeCount":     thermalSafe,
		},
	})
}

func (s *Server) handleEvaluateEctothermRescueOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.EctothermEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeEctothermRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	reStore := s.ensureEctothermRescueStore()
	created, err := reStore.EvaluateAndSave(req)
	if err != nil {
		writeEctothermRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextEctothermRescueNano()).UTC()
	s.broadcastEctothermRescueEvent("ectotherm_rescue_updated", map[string]any{
		"type":      "ectotherm_rescue_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateEctothermRescueStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeEctothermRescueJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.EctothermStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeEctothermRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	reStore := s.ensureEctothermRescueStore()
	if _, exists := reStore.Get(opID); !exists {
		writeEctothermRescueJSONError(w, http.StatusNotFound, "Ectotherm rescue operation not found")
		return
	}

	updated, err := reStore.UpdateStep(opID, req)
	if err != nil {
		writeEctothermRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextEctothermRescueNano()).UTC()
	s.broadcastEctothermRescueEvent("ectotherm_rescue_updated", map[string]any{
		"type":      "ectotherm_rescue_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueEctothermRescueClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeEctothermRescueJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req ectothermRescueClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeEctothermRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	reStore := s.ensureEctothermRescueStore()
	if _, exists := reStore.Get(opID); !exists {
		writeEctothermRescueJSONError(w, http.StatusNotFound, "Ectotherm rescue operation not found")
		return
	}

	updated, err := reStore.IssueClearance(opID, req.LeadHerpTechCallsign, req.Notes)
	if err != nil {
		writeEctothermRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextEctothermRescueNano()).UTC()
	s.broadcastEctothermRescueEvent("ectotherm_rescue_updated", map[string]any{
		"type":      "ectotherm_rescue_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleEctothermRescueStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("ectotherm_rescue")
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

func (s *Server) broadcastEctothermRescueEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextEctothermRescueNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "ectotherm_rescue",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "ectotherm_rescue",
			SenderNodeID:  "ectotherm-rescue-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
