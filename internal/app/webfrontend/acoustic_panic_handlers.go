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

	"github.com/scottdensmore/petspotr/pkg/acousticpanic"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastAcousticPanicNano atomic.Int64

func nextAcousticPanicNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastAcousticPanicNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastAcousticPanicNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// AcousticPanicPageData represents template data passed to templates/acoustic_panic_trajectory.html.
type AcousticPanicPageData struct {
	Operations            []domain.AcousticPanicOperation
	ActiveOperation       *domain.AcousticPanicOperation
	TotalOperations       int
	CriticalBoltCount     int
	StagedLureCount       int
	DecompressedSafeCount int
	Locale                string
}

type acousticStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureAcousticPanicStore() *acousticpanic.Store {
	if s.acousticPanicStore == nil {
		s.acousticPanicStore = acousticpanic.NewStore()
	}
	return s.acousticPanicStore
}

func (s *Server) registerAcousticPanicRoutes() {
	s.mux.HandleFunc("GET /acoustic-panic-trajectory", s.handleAcousticPanicTrajectoryPage)
	s.mux.HandleFunc("GET /api/v1/acoustic-panic/operations", s.handleListAcousticPanicOperations)
	s.mux.HandleFunc("POST /api/v1/acoustic-panic/evaluate", s.handleEvaluateAcousticPanicOperation)
	s.mux.HandleFunc("POST /api/v1/acoustic-panic/operations/{id}/steps", s.handleUpdateAcousticPanicStep)
	s.mux.HandleFunc("POST /api/v1/acoustic-panic/operations/{id}/step", s.handleUpdateAcousticPanicStep)
	s.mux.HandleFunc("POST /api/v1/acoustic-panic/operations/{id}/clearance", s.handleIssueAcousticPanicClearance)
	s.mux.HandleFunc("GET /api/v1/acoustic-panic/stream", s.handleAcousticPanicStream)
}

func computeAcousticPanicSummary(ops []domain.AcousticPanicOperation) (total, criticalBolt, stagedLure, decompressedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.AcousticStatusCriticalBolt:
			criticalBolt++
		case domain.AcousticStatusStagedSniff:
			stagedLure++
		case domain.AcousticStatusDecompressedSafe:
			decompressedSafe++
		}
	}
	return total, criticalBolt, stagedLure, decompressedSafe
}

func writeAcousticPanicJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleAcousticPanicTrajectoryPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apStore := s.ensureAcousticPanicStore()
	ops := apStore.List()
	total, criticalBolt, stagedLure, decompressedSafe := computeAcousticPanicSummary(ops)

	var activeOp *domain.AcousticPanicOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("acoustic_panic_trajectory.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/acoustic_panic_trajectory.html")
	if err != nil {
		http.Error(w, "Failed to load Acoustic Panic & Bolt-Trajectory SAR template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := AcousticPanicPageData{
		Operations:            ops,
		ActiveOperation:       activeOp,
		TotalOperations:       total,
		CriticalBoltCount:     criticalBolt,
		StagedLureCount:       stagedLure,
		DecompressedSafeCount: decompressedSafe,
		Locale:                locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListAcousticPanicOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apStore := s.ensureAcousticPanicStore()
	allOps := apStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	triggerFilter := strings.TrimSpace(r.URL.Query().Get("trigger_type"))
	if triggerFilter == "" {
		triggerFilter = strings.TrimSpace(r.URL.Query().Get("trigger"))
	}

	filtered := make([]domain.AcousticPanicOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if triggerFilter != "" && !strings.EqualFold(string(op.TriggerType), triggerFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalBolt, stagedLure, decompressedSafe := computeAcousticPanicSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":    total,
			"critical_bolt_count": criticalBolt,
			"staged_lure_count":   stagedLure,
			"decompressed_safe":   decompressedSafe,
		},
	})
}

func (s *Server) handleEvaluateAcousticPanicOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.AcousticEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAcousticPanicJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	apStore := s.ensureAcousticPanicStore()
	created, err := apStore.EvaluateAndSave(req)
	if err != nil {
		writeAcousticPanicJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAcousticPanicNano()).UTC()
	s.broadcastAcousticPanicEvent("acoustic_panic_updated", map[string]any{
		"type":      "acoustic_panic_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateAcousticPanicStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeAcousticPanicJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload acousticStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAcousticPanicJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	apStore := s.ensureAcousticPanicStore()
	if _, exists := apStore.Get(opID); !exists {
		writeAcousticPanicJSONError(w, http.StatusNotFound, "Acoustic panic operation not found")
		return
	}

	updated, err := apStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeAcousticPanicJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAcousticPanicNano()).UTC()
	s.broadcastAcousticPanicEvent("acoustic_panic_updated", map[string]any{
		"type":      "acoustic_panic_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueAcousticPanicClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeAcousticPanicJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeAcousticPanicJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	apStore := s.ensureAcousticPanicStore()
	if _, exists := apStore.Get(opID); !exists {
		writeAcousticPanicJSONError(w, http.StatusNotFound, "Acoustic panic operation not found")
		return
	}

	updated, err := apStore.IssueClearance(opID)
	if err != nil {
		writeAcousticPanicJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAcousticPanicNano()).UTC()
	s.broadcastAcousticPanicEvent("acoustic_panic_updated", map[string]any{
		"type":      "acoustic_panic_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleAcousticPanicStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("acoustic_panic")
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

func (s *Server) broadcastAcousticPanicEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextAcousticPanicNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "acoustic_panic",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "acoustic_panic",
			SenderNodeID:  "acoustic-panic-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
