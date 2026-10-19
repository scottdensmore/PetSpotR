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
	"github.com/scottdensmore/petspotr/pkg/gdvbloat"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastGDVBloatNano atomic.Int64

func nextGDVBloatNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastGDVBloatNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastGDVBloatNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// GDVBloatPageData represents template data passed to templates/gdv_bloat_triage.html.
type GDVBloatPageData struct {
	Operations        []domain.GDVBloatOperation
	ActiveOperation   *domain.GDVBloatOperation
	TotalOperations   int
	CriticalGDVCount  int
	StagedTrocarCount int
	ClearedSafeCount  int
	Locale            string
}

type gdvStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureGDVBloatStore() *gdvbloat.Store {
	if s.gdvBloatStore == nil {
		s.gdvBloatStore = gdvbloat.NewStore()
	}
	return s.gdvBloatStore
}

func (s *Server) registerGDVBloatRoutes() {
	s.mux.HandleFunc("GET /gdv-bloat-triage", s.handleGDVBloatTriagePage)
	s.mux.HandleFunc("GET /api/v1/gdv-bloat/operations", s.handleListGDVBloatOperations)
	s.mux.HandleFunc("POST /api/v1/gdv-bloat/evaluate", s.handleEvaluateGDVBloatOperation)
	s.mux.HandleFunc("POST /api/v1/gdv-bloat/operations/{id}/steps", s.handleUpdateGDVBloatStep)
	s.mux.HandleFunc("POST /api/v1/gdv-bloat/operations/{id}/step", s.handleUpdateGDVBloatStep)
	s.mux.HandleFunc("POST /api/v1/gdv-bloat/operations/{id}/clearance", s.handleIssueGDVBloatClearance)
	s.mux.HandleFunc("GET /api/v1/gdv-bloat/stream", s.handleGDVBloatStream)
}

func computeGDVBloatSummary(ops []domain.GDVBloatOperation) (total, criticalGDV, stagedTrocar, clearedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.GDVStatusCriticalNecrosisVPCShock:
			criticalGDV++
		case domain.GDVStatusStagedTrocarDualIV:
			stagedTrocar++
		case domain.GDVStatusClearedGastropexySinus:
			clearedSafe++
		}
	}
	return total, criticalGDV, stagedTrocar, clearedSafe
}

func writeGDVBloatJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleGDVBloatTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gdvStore := s.ensureGDVBloatStore()
	ops := gdvStore.List()
	total, criticalGDV, stagedTrocar, clearedSafe := computeGDVBloatSummary(ops)

	var activeOp *domain.GDVBloatOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("gdv_bloat_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/gdv_bloat_triage.html")
	if err != nil {
		http.Error(w, "Failed to load GDV / Bloat, Splenic Torsion & Trocar Decompression Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := GDVBloatPageData{
		Operations:        ops,
		ActiveOperation:   activeOp,
		TotalOperations:   total,
		CriticalGDVCount:  criticalGDV,
		StagedTrocarCount: stagedTrocar,
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

func (s *Server) handleListGDVBloatOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gdvStore := s.ensureGDVBloatStore()
	allOps := gdvStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	presFilter := strings.TrimSpace(r.URL.Query().Get("presentation_type"))
	if presFilter == "" {
		presFilter = strings.TrimSpace(r.URL.Query().Get("presentation"))
	}

	filtered := make([]domain.GDVBloatOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if presFilter != "" && !strings.EqualFold(string(op.PresentationType), presFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalGDV, stagedTrocar, clearedSafe := computeGDVBloatSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":   total,
			"critical_gdv_count": criticalGDV,
			"staged_trocar":      stagedTrocar,
			"cleared_safe":       clearedSafe,
		},
	})
}

func (s *Server) handleEvaluateGDVBloatOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.GDVEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeGDVBloatJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	gdvStore := s.ensureGDVBloatStore()
	created, err := gdvStore.EvaluateAndSave(req)
	if err != nil {
		writeGDVBloatJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextGDVBloatNano()).UTC()
	s.broadcastGDVBloatEvent("gdv_bloat_updated", map[string]any{
		"type":      "gdv_bloat_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateGDVBloatStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeGDVBloatJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload gdvStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeGDVBloatJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	gdvStore := s.ensureGDVBloatStore()
	if _, exists := gdvStore.Get(opID); !exists {
		writeGDVBloatJSONError(w, http.StatusNotFound, "GDV bloat triage operation not found")
		return
	}

	updated, err := gdvStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeGDVBloatJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextGDVBloatNano()).UTC()
	s.broadcastGDVBloatEvent("gdv_bloat_updated", map[string]any{
		"type":      "gdv_bloat_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueGDVBloatClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeGDVBloatJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeGDVBloatJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	gdvStore := s.ensureGDVBloatStore()
	if _, exists := gdvStore.Get(opID); !exists {
		writeGDVBloatJSONError(w, http.StatusNotFound, "GDV bloat triage operation not found")
		return
	}

	updated, err := gdvStore.IssueClearance(opID)
	if err != nil {
		writeGDVBloatJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextGDVBloatNano()).UTC()
	s.broadcastGDVBloatEvent("gdv_bloat_updated", map[string]any{
		"type":      "gdv_bloat_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleGDVBloatStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("gdv_bloat")
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

func (s *Server) broadcastGDVBloatEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextGDVBloatNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "gdv_bloat",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "gdv_bloat",
			SenderNodeID:  "gdv-bloat-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
