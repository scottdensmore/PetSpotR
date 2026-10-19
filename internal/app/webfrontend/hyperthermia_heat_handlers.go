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
	"github.com/scottdensmore/petspotr/pkg/hyperthermiaheat"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastHeatTriageNano atomic.Int64

func nextHeatTriageNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastHeatTriageNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastHeatTriageNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// HyperthermiaHeatPageData represents template data passed to templates/hyperthermia_heat_triage.html.
type HyperthermiaHeatPageData struct {
	Operations          []domain.HeatTriageOperation
	ActiveOperation     *domain.HeatTriageOperation
	TotalOperations     int
	CriticalDICCount    int
	StagedCoolingCount  int
	StabilizedSafeCount int
	Locale              string
}

type heatStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureHeatTriageStore() *hyperthermiaheat.Store {
	if s.heatTriageStore == nil {
		s.heatTriageStore = hyperthermiaheat.NewStore()
	}
	return s.heatTriageStore
}

func (s *Server) registerHyperthermiaHeatRoutes() {
	s.mux.HandleFunc("GET /hyperthermia-heat-triage", s.handleHyperthermiaHeatTriagePage)
	s.mux.HandleFunc("GET /api/v1/heat-triage/operations", s.handleListHeatTriageOperations)
	s.mux.HandleFunc("POST /api/v1/heat-triage/evaluate", s.handleEvaluateHeatTriageOperation)
	s.mux.HandleFunc("POST /api/v1/heat-triage/operations/{id}/steps", s.handleUpdateHeatTriageStep)
	s.mux.HandleFunc("POST /api/v1/heat-triage/operations/{id}/step", s.handleUpdateHeatTriageStep)
	s.mux.HandleFunc("POST /api/v1/heat-triage/operations/{id}/clearance", s.handleIssueHeatTriageClearance)
	s.mux.HandleFunc("GET /api/v1/heat-triage/stream", s.handleHeatTriageStream)
}

func computeHeatTriageSummary(ops []domain.HeatTriageOperation) (total, criticalDIC, stagedCooling, stabilizedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.HeatStatusCriticalDIC:
			criticalDIC++
		case domain.HeatStatusStagedCooling:
			stagedCooling++
		case domain.HeatStatusStabilizedCooled:
			stabilizedSafe++
		}
	}
	return total, criticalDIC, stagedCooling, stabilizedSafe
}

func writeHeatTriageJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleHyperthermiaHeatTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	htStore := s.ensureHeatTriageStore()
	ops := htStore.List()
	total, criticalDIC, stagedCooling, stabilizedSafe := computeHeatTriageSummary(ops)

	var activeOp *domain.HeatTriageOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("hyperthermia_heat_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/hyperthermia_heat_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Hyperthermia & Heatstroke Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := HyperthermiaHeatPageData{
		Operations:          ops,
		ActiveOperation:     activeOp,
		TotalOperations:     total,
		CriticalDICCount:    criticalDIC,
		StagedCoolingCount:  stagedCooling,
		StabilizedSafeCount: stabilizedSafe,
		Locale:              locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListHeatTriageOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	htStore := s.ensureHeatTriageStore()
	allOps := htStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	exposureFilter := strings.TrimSpace(r.URL.Query().Get("exposure"))
	if exposureFilter == "" {
		exposureFilter = strings.TrimSpace(r.URL.Query().Get("exposure_type"))
	}

	filtered := make([]domain.HeatTriageOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if exposureFilter != "" && !strings.EqualFold(string(op.ExposureType), exposureFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalDIC, stagedCooling, stabilizedSafe := computeHeatTriageSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":   total,
			"critical_dic_count": criticalDIC,
			"staged_cooling":     stagedCooling,
			"stabilized_safe":    stabilizedSafe,
		},
	})
}

func (s *Server) handleEvaluateHeatTriageOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.HeatEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHeatTriageJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	htStore := s.ensureHeatTriageStore()
	created, err := htStore.EvaluateAndSave(req)
	if err != nil {
		writeHeatTriageJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHeatTriageNano()).UTC()
	s.broadcastHeatTriageEvent("heat_triage_updated", map[string]any{
		"type":      "heat_triage_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateHeatTriageStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeHeatTriageJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload heatStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeHeatTriageJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	htStore := s.ensureHeatTriageStore()
	if _, exists := htStore.Get(opID); !exists {
		writeHeatTriageJSONError(w, http.StatusNotFound, "Heat triage operation not found")
		return
	}

	updated, err := htStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeHeatTriageJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHeatTriageNano()).UTC()
	s.broadcastHeatTriageEvent("heat_triage_updated", map[string]any{
		"type":      "heat_triage_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueHeatTriageClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeHeatTriageJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeHeatTriageJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	htStore := s.ensureHeatTriageStore()
	if _, exists := htStore.Get(opID); !exists {
		writeHeatTriageJSONError(w, http.StatusNotFound, "Heat triage operation not found")
		return
	}

	updated, err := htStore.IssueClearance(opID)
	if err != nil {
		writeHeatTriageJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHeatTriageNano()).UTC()
	s.broadcastHeatTriageEvent("heat_triage_updated", map[string]any{
		"type":      "heat_triage_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleHeatTriageStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("heat_triage")
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

func (s *Server) broadcastHeatTriageEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextHeatTriageNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "heat_triage",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "heat_triage",
			SenderNodeID:  "heat-triage-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
