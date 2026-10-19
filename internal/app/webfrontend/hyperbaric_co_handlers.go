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
	"github.com/scottdensmore/petspotr/pkg/hyperbaricco"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastHyperbaricCONano atomic.Int64

func nextHyperbaricCONano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastHyperbaricCONano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastHyperbaricCONano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// HyperbaricCOPageData represents template data passed to templates/hyperbaric_co_triage.html.
type HyperbaricCOPageData struct {
	Operations        []domain.HyperbaricCOOperation
	ActiveOperation   *domain.HyperbaricCOOperation
	TotalOperations   int
	CriticalCOHbCount int
	StagedHBOTCount   int
	ClearedSafeCount  int
	Locale            string
}

type hyperbaricStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureHyperbaricCOStore() *hyperbaricco.Store {
	if s.hyperbaricCOStore == nil {
		s.hyperbaricCOStore = hyperbaricco.NewStore()
	}
	return s.hyperbaricCOStore
}

func (s *Server) registerHyperbaricCORoutes() {
	s.mux.HandleFunc("GET /hyperbaric-co-triage", s.handleHyperbaricCOTriagePage)
	s.mux.HandleFunc("GET /api/v1/hyperbaric-co/operations", s.handleListHyperbaricCOOperations)
	s.mux.HandleFunc("POST /api/v1/hyperbaric-co/evaluate", s.handleEvaluateHyperbaricCOOperation)
	s.mux.HandleFunc("POST /api/v1/hyperbaric-co/operations/{id}/steps", s.handleUpdateHyperbaricCOStep)
	s.mux.HandleFunc("POST /api/v1/hyperbaric-co/operations/{id}/step", s.handleUpdateHyperbaricCOStep)
	s.mux.HandleFunc("POST /api/v1/hyperbaric-co/operations/{id}/clearance", s.handleIssueHyperbaricCOClearance)
	s.mux.HandleFunc("GET /api/v1/hyperbaric-co/stream", s.handleHyperbaricCOStream)
}

func computeHyperbaricCOSummary(ops []domain.HyperbaricCOOperation) (total, criticalCOHb, stagedHBOT, clearedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.HyperbaricStatusCriticalCOHbCyanide:
			criticalCOHb++
		case domain.HyperbaricStatusStagedHBOTHydroxocobalamin:
			stagedHBOT++
		case domain.HyperbaricStatusClearedSafeNormoxic:
			clearedSafe++
		}
	}
	return total, criticalCOHb, stagedHBOT, clearedSafe
}

func writeHyperbaricCOJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleHyperbaricCOTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hbStore := s.ensureHyperbaricCOStore()
	ops := hbStore.List()
	total, criticalCOHb, stagedHBOT, clearedSafe := computeHyperbaricCOSummary(ops)

	var activeOp *domain.HyperbaricCOOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("hyperbaric_co_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/hyperbaric_co_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Hyperbaric Oxygen & CO/Cyanide Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := HyperbaricCOPageData{
		Operations:        ops,
		ActiveOperation:   activeOp,
		TotalOperations:   total,
		CriticalCOHbCount: criticalCOHb,
		StagedHBOTCount:   stagedHBOT,
		ClearedSafeCount:  clearedSafe,
		Locale:            locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListHyperbaricCOOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hbStore := s.ensureHyperbaricCOStore()
	allOps := hbStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	sourceFilter := strings.TrimSpace(r.URL.Query().Get("exposure_source"))
	if sourceFilter == "" {
		sourceFilter = strings.TrimSpace(r.URL.Query().Get("exposure"))
	}

	filtered := make([]domain.HyperbaricCOOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if sourceFilter != "" && !strings.EqualFold(string(op.ExposureSource), sourceFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalCOHb, stagedHBOT, clearedSafe := computeHyperbaricCOSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":    total,
			"critical_cohb_count": criticalCOHb,
			"staged_hbot":         stagedHBOT,
			"cleared_safe":        clearedSafe,
		},
	})
}

func (s *Server) handleEvaluateHyperbaricCOOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.HyperbaricEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHyperbaricCOJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	hbStore := s.ensureHyperbaricCOStore()
	created, err := hbStore.EvaluateAndSave(req)
	if err != nil {
		writeHyperbaricCOJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHyperbaricCONano()).UTC()
	s.broadcastHyperbaricCOEvent("hyperbaric_co_updated", map[string]any{
		"type":      "hyperbaric_co_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateHyperbaricCOStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeHyperbaricCOJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload hyperbaricStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeHyperbaricCOJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	hbStore := s.ensureHyperbaricCOStore()
	if _, exists := hbStore.Get(opID); !exists {
		writeHyperbaricCOJSONError(w, http.StatusNotFound, "Hyperbaric CO triage operation not found")
		return
	}

	updated, err := hbStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeHyperbaricCOJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHyperbaricCONano()).UTC()
	s.broadcastHyperbaricCOEvent("hyperbaric_co_updated", map[string]any{
		"type":      "hyperbaric_co_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueHyperbaricCOClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeHyperbaricCOJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeHyperbaricCOJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	hbStore := s.ensureHyperbaricCOStore()
	if _, exists := hbStore.Get(opID); !exists {
		writeHyperbaricCOJSONError(w, http.StatusNotFound, "Hyperbaric CO triage operation not found")
		return
	}

	updated, err := hbStore.IssueClearance(opID)
	if err != nil {
		writeHyperbaricCOJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHyperbaricCONano()).UTC()
	s.broadcastHyperbaricCOEvent("hyperbaric_co_updated", map[string]any{
		"type":      "hyperbaric_co_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleHyperbaricCOStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("hyperbaric_co")
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

func (s *Server) broadcastHyperbaricCOEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextHyperbaricCONano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "hyperbaric_co",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "hyperbaric_co",
			SenderNodeID:  "hyperbaric-co-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
