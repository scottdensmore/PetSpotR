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
	"github.com/scottdensmore/petspotr/pkg/neonatalincubator"
)

var lastNeonatalIncubatorNano atomic.Int64

func nextNeonatalIncubatorNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastNeonatalIncubatorNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastNeonatalIncubatorNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// NeonatalIncubatorPageData represents template data passed to templates/neonatal_incubator_triage.html.
type NeonatalIncubatorPageData struct {
	Operations           []domain.NeonatalTriageOperation
	ActiveOperation      *domain.NeonatalTriageOperation
	TotalOperations      int
	CriticalFadingCount  int
	StagedIncubatorCount int
	StabilizedSafeCount  int
	Locale               string
}

type neonatalStepUpdatePayload struct {
	StepID    string `json:"step_id"`
	StepIDAlt string `json:"stepId"`
	Completed bool   `json:"completed"`
}

func (s *Server) ensureNeonatalIncubatorStore() *neonatalincubator.Store {
	if s.neonatalIncubatorStore == nil {
		s.neonatalIncubatorStore = neonatalincubator.NewStore()
	}
	return s.neonatalIncubatorStore
}

func (s *Server) registerNeonatalIncubatorRoutes() {
	s.mux.HandleFunc("GET /neonatal-incubator-triage", s.handleNeonatalIncubatorTriagePage)
	s.mux.HandleFunc("GET /api/v1/neonatal-triage/operations", s.handleListNeonatalTriageOperations)
	s.mux.HandleFunc("POST /api/v1/neonatal-triage/evaluate", s.handleEvaluateNeonatalTriageOperation)
	s.mux.HandleFunc("POST /api/v1/neonatal-triage/operations/{id}/steps", s.handleUpdateNeonatalTriageStep)
	s.mux.HandleFunc("POST /api/v1/neonatal-triage/operations/{id}/step", s.handleUpdateNeonatalTriageStep)
	s.mux.HandleFunc("POST /api/v1/neonatal-triage/operations/{id}/clearance", s.handleIssueNeonatalTriageClearance)
	s.mux.HandleFunc("GET /api/v1/neonatal-triage/stream", s.handleNeonatalTriageStream)
}

func computeNeonatalTriageSummary(ops []domain.NeonatalTriageOperation) (total, criticalFading, stagedIncubator, stabilizedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.NeonatalTriageStatusFadingTriadHypothermicCritical:
			criticalFading++
		case domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged:
			stagedIncubator++
		case domain.NeonatalTriageStatusStabilizedNursingGainingSafe:
			stabilizedSafe++
		}
	}
	return total, criticalFading, stagedIncubator, stabilizedSafe
}

func writeNeonatalTriageJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleNeonatalIncubatorTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	niStore := s.ensureNeonatalIncubatorStore()
	ops := niStore.List()
	total, criticalFading, stagedIncubator, stabilizedSafe := computeNeonatalTriageSummary(ops)

	var activeOp *domain.NeonatalTriageOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("neonatal_incubator_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/neonatal_incubator_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Neonatal Incubator Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := NeonatalIncubatorPageData{
		Operations:           ops,
		ActiveOperation:      activeOp,
		TotalOperations:      total,
		CriticalFadingCount:  criticalFading,
		StagedIncubatorCount: stagedIncubator,
		StabilizedSafeCount:  stabilizedSafe,
		Locale:               locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListNeonatalTriageOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	niStore := s.ensureNeonatalIncubatorStore()
	allOps := niStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	litterFilter := strings.TrimSpace(r.URL.Query().Get("litter_type"))
	if litterFilter == "" {
		litterFilter = strings.TrimSpace(r.URL.Query().Get("litterType"))
	}

	filtered := make([]domain.NeonatalTriageOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if litterFilter != "" && !strings.EqualFold(string(op.LitterType), litterFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalFading, stagedIncubator, stabilizedSafe := computeNeonatalTriageSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":       total,
			"critical_fading_count":  criticalFading,
			"staged_incubator_count": stagedIncubator,
			"stabilized_safe_count":  stabilizedSafe,
			"totalOperations":        total,
			"criticalFadingCount":    criticalFading,
			"stagedIncubatorCount":   stagedIncubator,
			"stabilizedSafeCount":    stabilizedSafe,
		},
	})
}

func (s *Server) handleEvaluateNeonatalTriageOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.NeonatalEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeNeonatalTriageJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	niStore := s.ensureNeonatalIncubatorStore()
	created, err := niStore.EvaluateAndSave(req)
	if err != nil {
		writeNeonatalTriageJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextNeonatalIncubatorNano()).UTC()
	s.broadcastNeonatalTriageEvent("neonatal_triage_updated", map[string]any{
		"type":      "neonatal_triage_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateNeonatalTriageStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeNeonatalTriageJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload neonatalStepUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeNeonatalTriageJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDAlt)
	}

	niStore := s.ensureNeonatalIncubatorStore()
	if _, exists := niStore.Get(opID); !exists {
		writeNeonatalTriageJSONError(w, http.StatusNotFound, "Neonatal triage operation not found")
		return
	}

	updated, err := niStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeNeonatalTriageJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextNeonatalIncubatorNano()).UTC()
	s.broadcastNeonatalTriageEvent("neonatal_triage_updated", map[string]any{
		"type":      "neonatal_triage_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueNeonatalTriageClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeNeonatalTriageJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeNeonatalTriageJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	niStore := s.ensureNeonatalIncubatorStore()
	if _, exists := niStore.Get(opID); !exists {
		writeNeonatalTriageJSONError(w, http.StatusNotFound, "Neonatal triage operation not found")
		return
	}

	updated, err := niStore.IssueClearance(opID)
	if err != nil {
		writeNeonatalTriageJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextNeonatalIncubatorNano()).UTC()
	s.broadcastNeonatalTriageEvent("neonatal_triage_updated", map[string]any{
		"type":      "neonatal_triage_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleNeonatalTriageStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("neonatal_triage")
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

func (s *Server) broadcastNeonatalTriageEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextNeonatalIncubatorNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "neonatal_triage",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "neonatal_triage",
			SenderNodeID:  "neonatal-triage-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
