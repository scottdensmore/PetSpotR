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
	"github.com/scottdensmore/petspotr/pkg/envenomationtoxin"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastEnvenomationToxinNano atomic.Int64

func nextEnvenomationToxinNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastEnvenomationToxinNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastEnvenomationToxinNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// EnvenomationToxinPageData represents template data passed to templates/envenomation_toxin_triage.html.
type EnvenomationToxinPageData struct {
	Operations           []domain.EnvenomationTriageOperation
	ActiveOperation      *domain.EnvenomationTriageOperation
	TotalOperations      int
	CriticalToxinCount   int
	StagedAntivenomCount int
	StabilizedSafeCount  int
	Locale               string
}

type envenomationStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureEnvenomationToxinStore() *envenomationtoxin.Store {
	if s.envenomationToxinStore == nil {
		s.envenomationToxinStore = envenomationtoxin.NewStore()
	}
	return s.envenomationToxinStore
}

func (s *Server) registerEnvenomationToxinRoutes() {
	s.mux.HandleFunc("GET /envenomation-toxin-triage", s.handleEnvenomationToxinTriagePage)
	s.mux.HandleFunc("GET /api/v1/envenomation-toxin/operations", s.handleListEnvenomationToxinOperations)
	s.mux.HandleFunc("POST /api/v1/envenomation-toxin/evaluate", s.handleEvaluateEnvenomationToxinOperation)
	s.mux.HandleFunc("POST /api/v1/envenomation-toxin/operations/{id}/steps", s.handleUpdateEnvenomationToxinStep)
	s.mux.HandleFunc("POST /api/v1/envenomation-toxin/operations/{id}/step", s.handleUpdateEnvenomationToxinStep)
	s.mux.HandleFunc("POST /api/v1/envenomation-toxin/operations/{id}/clearance", s.handleIssueEnvenomationToxinClearance)
	s.mux.HandleFunc("GET /api/v1/envenomation-toxin/stream", s.handleEnvenomationToxinStream)
}

func computeEnvenomationToxinSummary(ops []domain.EnvenomationTriageOperation) (total, criticalToxin, stagedAntivenom, stabilizedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.EnvenomationStatusCritical:
			criticalToxin++
		case domain.EnvenomationStatusStaged:
			stagedAntivenom++
		case domain.EnvenomationStatusStabilized:
			stabilizedSafe++
		}
	}
	return total, criticalToxin, stagedAntivenom, stabilizedSafe
}

func writeEnvenomationToxinJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleEnvenomationToxinTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	etStore := s.ensureEnvenomationToxinStore()
	ops := etStore.List()
	total, criticalToxin, stagedAntivenom, stabilizedSafe := computeEnvenomationToxinSummary(ops)

	var activeOp *domain.EnvenomationTriageOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("envenomation_toxin_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/envenomation_toxin_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Envenomation & Toxin Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := EnvenomationToxinPageData{
		Operations:           ops,
		ActiveOperation:      activeOp,
		TotalOperations:      total,
		CriticalToxinCount:   criticalToxin,
		StagedAntivenomCount: stagedAntivenom,
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

func (s *Server) handleListEnvenomationToxinOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	etStore := s.ensureEnvenomationToxinStore()
	allOps := etStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	toxinFilter := strings.TrimSpace(r.URL.Query().Get("toxin_type"))
	if toxinFilter == "" {
		toxinFilter = strings.TrimSpace(r.URL.Query().Get("toxin"))
	}

	filtered := make([]domain.EnvenomationTriageOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if toxinFilter != "" && !strings.EqualFold(string(op.ToxinType), toxinFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalToxin, stagedAntivenom, stabilizedSafe := computeEnvenomationToxinSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":     total,
			"critical_toxin_count": criticalToxin,
			"staged_antivenom":     stagedAntivenom,
			"stabilized_safe":      stabilizedSafe,
		},
	})
}

func (s *Server) handleEvaluateEnvenomationToxinOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.EnvenomationEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeEnvenomationToxinJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	etStore := s.ensureEnvenomationToxinStore()
	created, err := etStore.EvaluateAndSave(req)
	if err != nil {
		writeEnvenomationToxinJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextEnvenomationToxinNano()).UTC()
	s.broadcastEnvenomationToxinEvent("envenomation_toxin_updated", map[string]any{
		"type":      "envenomation_toxin_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateEnvenomationToxinStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeEnvenomationToxinJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload envenomationStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeEnvenomationToxinJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	etStore := s.ensureEnvenomationToxinStore()
	if _, exists := etStore.Get(opID); !exists {
		writeEnvenomationToxinJSONError(w, http.StatusNotFound, "Envenomation toxin triage operation not found")
		return
	}

	updated, err := etStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeEnvenomationToxinJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextEnvenomationToxinNano()).UTC()
	s.broadcastEnvenomationToxinEvent("envenomation_toxin_updated", map[string]any{
		"type":      "envenomation_toxin_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueEnvenomationToxinClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeEnvenomationToxinJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeEnvenomationToxinJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	etStore := s.ensureEnvenomationToxinStore()
	if _, exists := etStore.Get(opID); !exists {
		writeEnvenomationToxinJSONError(w, http.StatusNotFound, "Envenomation toxin triage operation not found")
		return
	}

	updated, err := etStore.IssueClearance(opID)
	if err != nil {
		writeEnvenomationToxinJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextEnvenomationToxinNano()).UTC()
	s.broadcastEnvenomationToxinEvent("envenomation_toxin_updated", map[string]any{
		"type":      "envenomation_toxin_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleEnvenomationToxinStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("envenomation_toxin")
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

func (s *Server) broadcastEnvenomationToxinEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextEnvenomationToxinNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "envenomation_toxin",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "envenomation_toxin",
			SenderNodeID:  "envenomation-toxin-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
