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
	"github.com/scottdensmore/petspotr/pkg/usarcollapse"
)

var lastUSARCollapseNano atomic.Int64

func nextUSARCollapseNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastUSARCollapseNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastUSARCollapseNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// USARCollapsePageData represents template data passed to templates/usar_structural_collapse.html.
type USARCollapsePageData struct {
	Operations             []domain.USARRescueOperation
	TotalOperations        int
	CriticalCollapseAlerts int
	StagedShoringRigs      int
	ExtractedUSARCount     int
	Locale                 string
}

type usarCollapseClearanceRequest struct {
	LeadUSARSpecCallsign string `json:"leadUsarSpecCallsign"`
	Notes                string `json:"notes"`
}

func (s *Server) ensureUSARCollapseStore() *usarcollapse.Store {
	if s.usarCollapseStore == nil {
		s.usarCollapseStore = usarcollapse.NewStore()
	}
	return s.usarCollapseStore
}

func (s *Server) registerUSARCollapseRoutes() {
	s.mux.HandleFunc("GET /usar-structural-collapse", s.handleUSARStructuralCollapsePage)
	s.mux.HandleFunc("GET /api/v1/usar-collapse/operations", s.handleListUSARCollapseOperations)
	s.mux.HandleFunc("POST /api/v1/usar-collapse/evaluate", s.handleEvaluateUSARCollapseOperation)
	s.mux.HandleFunc("POST /api/v1/usar-collapse/operations/{id}/step", s.handleUpdateUSARCollapseStep)
	s.mux.HandleFunc("POST /api/v1/usar-collapse/operations/{id}/clearance", s.handleIssueUSARCollapseClearance)
	s.mux.HandleFunc("GET /api/v1/usar-collapse/stream", s.handleUSARCollapseStream)
}

func computeUSARCollapseSummary(ops []domain.USARRescueOperation) (total, criticalCollapse, stagedShoring, extractedSafe int) {
	total = len(ops)
	for _, op := range ops {
		if op.SecondaryCollapseAlert || op.CrushRhabdoAlert || op.Status == domain.USARRescueStatusSecondaryCollapseCrushCritical {
			criticalCollapse++
		}
		if op.Status == domain.USARRescueStatusShoringCribbingDelsarStaged {
			stagedShoring++
		}
		if op.Status == domain.USARRescueStatusExtractedUSARDeconSafe {
			extractedSafe++
		}
	}
	return total, criticalCollapse, stagedShoring, extractedSafe
}

func writeUSARCollapseJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleUSARStructuralCollapsePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	ops := usarStore.List()
	total, criticalCollapse, stagedShoring, extractedSafe := computeUSARCollapseSummary(ops)

	tmpl, err := template.New("usar_structural_collapse.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/usar_structural_collapse.html")
	if err != nil {
		http.Error(w, "Failed to load USAR Structural Collapse template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := USARCollapsePageData{
		Operations:             ops,
		TotalOperations:        total,
		CriticalCollapseAlerts: criticalCollapse,
		StagedShoringRigs:      stagedShoring,
		ExtractedUSARCount:     extractedSafe,
		Locale:                 locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListUSARCollapseOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	allOps := usarStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	voidFilter := strings.TrimSpace(r.URL.Query().Get("voidType"))

	filtered := make([]domain.USARRescueOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if voidFilter != "" && !strings.EqualFold(string(op.VoidType), voidFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalCollapse, stagedShoring, extractedSafe := computeUSARCollapseSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"totalOperations":        total,
			"criticalCollapseAlerts": criticalCollapse,
			"stagedShoringRigs":      stagedShoring,
			"extractedUsarCount":     extractedSafe,
		},
	})
}

func (s *Server) handleEvaluateUSARCollapseOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.USAREvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeUSARCollapseJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	created, err := usarStore.EvaluateAndSave(req)
	if err != nil {
		writeUSARCollapseJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextUSARCollapseNano()).UTC()
	s.broadcastUSARCollapseEvent("usar_collapse_updated", map[string]any{
		"type":      "usar_collapse_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateUSARCollapseStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeUSARCollapseJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.USARStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeUSARCollapseJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	if _, exists := usarStore.Get(opID); !exists {
		writeUSARCollapseJSONError(w, http.StatusNotFound, "USAR collapse operation not found")
		return
	}

	updated, err := usarStore.UpdateStep(opID, req)
	if err != nil {
		writeUSARCollapseJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextUSARCollapseNano()).UTC()
	s.broadcastUSARCollapseEvent("usar_collapse_updated", map[string]any{
		"type":      "usar_collapse_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueUSARCollapseClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeUSARCollapseJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req usarCollapseClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeUSARCollapseJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	if _, exists := usarStore.Get(opID); !exists {
		writeUSARCollapseJSONError(w, http.StatusNotFound, "USAR collapse operation not found")
		return
	}

	updated, err := usarStore.IssueClearance(opID, req.LeadUSARSpecCallsign, req.Notes)
	if err != nil {
		writeUSARCollapseJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextUSARCollapseNano()).UTC()
	s.broadcastUSARCollapseEvent("usar_collapse_updated", map[string]any{
		"type":      "usar_collapse_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleUSARCollapseStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("usar_collapse")
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

func (s *Server) broadcastUSARCollapseEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextUSARCollapseNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "usar_collapse",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "usar_collapse",
			SenderNodeID:  "usar-collapse-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
