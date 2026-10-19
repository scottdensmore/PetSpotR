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
	"github.com/scottdensmore/petspotr/pkg/toxicologypoison"
)

var lastToxicologyPoisonNano atomic.Int64

func nextToxicologyPoisonNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastToxicologyPoisonNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastToxicologyPoisonNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// ToxicologyPoisonPageData represents template data passed to templates/toxicology_poison_triage.html.
type ToxicologyPoisonPageData struct {
	Operations          []domain.ToxicologyPoisonOperation
	ActiveOperation     *domain.ToxicologyPoisonOperation
	TotalOperations     int
	CriticalPoisonCount int
	StagedDeconCount    int
	ClearedSafeCount    int
	Locale              string
}

type toxicologyStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureToxicologyPoisonStore() *toxicologypoison.Store {
	if s.toxicologyPoisonStore == nil {
		s.toxicologyPoisonStore = toxicologypoison.NewStore()
	}
	return s.toxicologyPoisonStore
}

func (s *Server) registerToxicologyPoisonRoutes() {
	s.mux.HandleFunc("GET /toxicology-poison-triage", s.handleToxicologyPoisonTriagePage)
	s.mux.HandleFunc("GET /api/v1/toxicology-poison/operations", s.handleListToxicologyPoisonOperations)
	s.mux.HandleFunc("POST /api/v1/toxicology-poison/evaluate", s.handleEvaluateToxicologyPoisonOperation)
	s.mux.HandleFunc("POST /api/v1/toxicology-poison/operations/{id}/steps", s.handleUpdateToxicologyPoisonStep)
	s.mux.HandleFunc("POST /api/v1/toxicology-poison/operations/{id}/step", s.handleUpdateToxicologyPoisonStep)
	s.mux.HandleFunc("POST /api/v1/toxicology-poison/operations/{id}/clearance", s.handleIssueToxicologyPoisonClearance)
	s.mux.HandleFunc("GET /api/v1/toxicology-poison/stream", s.handleToxicologyPoisonStream)
}

func computeToxicologyPoisonSummary(ops []domain.ToxicologyPoisonOperation) (total, criticalPoison, stagedDecon, clearedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro:
			criticalPoison++
		case domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon:
			stagedDecon++
		case domain.ToxicologyStatusClearedDecontaminatedPTGlucose:
			clearedSafe++
		}
	}
	return total, criticalPoison, stagedDecon, clearedSafe
}

func writeToxicologyPoisonJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleToxicologyPoisonTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tpStore := s.ensureToxicologyPoisonStore()
	ops := tpStore.List()
	total, criticalPoison, stagedDecon, clearedSafe := computeToxicologyPoisonSummary(ops)

	var activeOp *domain.ToxicologyPoisonOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("toxicology_poison_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/toxicology_poison_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Anticoagulant Rodenticide, Xylitol & Lipid Emulsion (ILE) Poison Control Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := ToxicologyPoisonPageData{
		Operations:          ops,
		ActiveOperation:     activeOp,
		TotalOperations:     total,
		CriticalPoisonCount: criticalPoison,
		StagedDeconCount:    stagedDecon,
		ClearedSafeCount:    clearedSafe,
		Locale:              locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListToxicologyPoisonOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tpStore := s.ensureToxicologyPoisonStore()
	allOps := tpStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	agentFilter := strings.TrimSpace(r.URL.Query().Get("agent_type"))
	if agentFilter == "" {
		agentFilter = strings.TrimSpace(r.URL.Query().Get("agent"))
	}

	filtered := make([]domain.ToxicologyPoisonOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if agentFilter != "" && !strings.EqualFold(string(op.AgentType), agentFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalPoison, stagedDecon, clearedSafe := computeToxicologyPoisonSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":      total,
			"critical_poison_count": criticalPoison,
			"staged_decon":          stagedDecon,
			"cleared_safe":          clearedSafe,
		},
	})
}

func (s *Server) handleEvaluateToxicologyPoisonOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.ToxicologyEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeToxicologyPoisonJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	tpStore := s.ensureToxicologyPoisonStore()
	created, err := tpStore.EvaluateAndSave(req)
	if err != nil {
		writeToxicologyPoisonJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextToxicologyPoisonNano()).UTC()
	s.broadcastToxicologyPoisonEvent("toxicology_poison_updated", map[string]any{
		"type":      "toxicology_poison_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateToxicologyPoisonStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeToxicologyPoisonJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload toxicologyStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeToxicologyPoisonJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	tpStore := s.ensureToxicologyPoisonStore()
	if _, exists := tpStore.Get(opID); !exists {
		writeToxicologyPoisonJSONError(w, http.StatusNotFound, "Toxicology poison triage operation not found")
		return
	}

	updated, err := tpStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeToxicologyPoisonJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextToxicologyPoisonNano()).UTC()
	s.broadcastToxicologyPoisonEvent("toxicology_poison_updated", map[string]any{
		"type":      "toxicology_poison_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueToxicologyPoisonClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeToxicologyPoisonJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeToxicologyPoisonJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	tpStore := s.ensureToxicologyPoisonStore()
	if _, exists := tpStore.Get(opID); !exists {
		writeToxicologyPoisonJSONError(w, http.StatusNotFound, "Toxicology poison triage operation not found")
		return
	}

	updated, err := tpStore.IssueClearance(opID)
	if err != nil {
		writeToxicologyPoisonJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextToxicologyPoisonNano()).UTC()
	s.broadcastToxicologyPoisonEvent("toxicology_poison_updated", map[string]any{
		"type":      "toxicology_poison_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleToxicologyPoisonStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("toxicology_poison")
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

func (s *Server) broadcastToxicologyPoisonEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextToxicologyPoisonNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "toxicology_poison",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "toxicology_poison",
			SenderNodeID:  "toxicology-poison-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
