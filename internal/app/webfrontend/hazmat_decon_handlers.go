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
	"github.com/scottdensmore/petspotr/pkg/hazmatdecon"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastHazmatDeconNano atomic.Int64

func nextHazmatDeconNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastHazmatDeconNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastHazmatDeconNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// HazmatDeconPageData represents template data passed to templates/hazmat_decon.html.
type HazmatDeconPageData struct {
	Operations          []domain.HazmatDeconOperation
	TotalOperations     int
	CriticalSmokeAlerts int
	StagedDeconWashes   int
	ClearedSafeCount    int
	Locale              string
}

type hazmatDeconClearanceRequest struct {
	LeadToxTechCallsign string `json:"leadToxTechCallsign"`
	Notes               string `json:"notes"`
}

func (s *Server) ensureHazmatDeconStore() *hazmatdecon.Store {
	if s.hazmatDeconStore == nil {
		s.hazmatDeconStore = hazmatdecon.NewStore()
	}
	return s.hazmatDeconStore
}

func (s *Server) registerHazmatDeconRoutes() {
	s.mux.HandleFunc("GET /hazmat-decon", s.handleHazmatDeconPage)
	s.mux.HandleFunc("GET /api/v1/hazmat-decon/operations", s.handleListHazmatDeconOperations)
	s.mux.HandleFunc("POST /api/v1/hazmat-decon/evaluate", s.handleEvaluateHazmatDeconOperation)
	s.mux.HandleFunc("POST /api/v1/hazmat-decon/operations/{id}/step", s.handleUpdateHazmatDeconStep)
	s.mux.HandleFunc("POST /api/v1/hazmat-decon/operations/{id}/clearance", s.handleIssueHazmatDeconClearance)
	s.mux.HandleFunc("GET /api/v1/hazmat-decon/stream", s.handleHazmatDeconStream)
}

func computeHazmatDeconSummary(ops []domain.HazmatDeconOperation) (total, criticalSmoke, staged, cleared int) {
	total = len(ops)
	for _, op := range ops {
		if op.SmokeInhalationAlert || op.Status == domain.HazmatStatusRespiratoryDermalCritical {
			criticalSmoke++
		}
		if op.Status == domain.HazmatStatusDeconWashNebulizerStaged {
			staged++
		}
		if op.Status == domain.HazmatStatusDecontaminatedClearedSafe {
			cleared++
		}
	}
	return total, criticalSmoke, staged, cleared
}

func writeHazmatDeconJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleHazmatDeconPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	ops := hzStore.List()
	total, criticalSmoke, staged, cleared := computeHazmatDeconSummary(ops)

	tmpl, err := template.New("hazmat_decon.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/hazmat_decon.html")
	if err != nil {
		http.Error(w, "Failed to load Hazmat Decon template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := HazmatDeconPageData{
		Operations:          ops,
		TotalOperations:     total,
		CriticalSmokeAlerts: criticalSmoke,
		StagedDeconWashes:   staged,
		ClearedSafeCount:    cleared,
		Locale:              locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListHazmatDeconOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	allOps := hzStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	exposureFilter := strings.TrimSpace(r.URL.Query().Get("exposureType"))

	filtered := make([]domain.HazmatDeconOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if exposureFilter != "" && !strings.EqualFold(string(op.ExposureType), exposureFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalSmoke, staged, cleared := computeHazmatDeconSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"totalOperations":     total,
			"criticalSmokeAlerts": criticalSmoke,
			"stagedDeconWashes":   staged,
			"clearedSafeCount":    cleared,
		},
	})
}

func (s *Server) handleEvaluateHazmatDeconOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.HazmatEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHazmatDeconJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	created, err := hzStore.EvaluateAndSave(req)
	if err != nil {
		writeHazmatDeconJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHazmatDeconNano()).UTC()
	s.broadcastHazmatDeconEvent("hazmat_decon_updated", map[string]any{
		"type":      "hazmat_decon_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateHazmatDeconStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeHazmatDeconJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.HazmatStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHazmatDeconJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	if _, exists := hzStore.Get(opID); !exists {
		writeHazmatDeconJSONError(w, http.StatusNotFound, "Hazmat decon operation not found")
		return
	}

	updated, err := hzStore.UpdateStep(opID, req)
	if err != nil {
		writeHazmatDeconJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHazmatDeconNano()).UTC()
	s.broadcastHazmatDeconEvent("hazmat_decon_updated", map[string]any{
		"type":      "hazmat_decon_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueHazmatDeconClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeHazmatDeconJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req hazmatDeconClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeHazmatDeconJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	if _, exists := hzStore.Get(opID); !exists {
		writeHazmatDeconJSONError(w, http.StatusNotFound, "Hazmat decon operation not found")
		return
	}

	updated, err := hzStore.IssueClearance(opID, req.LeadToxTechCallsign, req.Notes)
	if err != nil {
		writeHazmatDeconJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHazmatDeconNano()).UTC()
	s.broadcastHazmatDeconEvent("hazmat_decon_updated", map[string]any{
		"type":      "hazmat_decon_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleHazmatDeconStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("hazmat_decon")
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

func (s *Server) broadcastHazmatDeconEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextHazmatDeconNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "hazmat_decon",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "hazmat_decon",
			SenderNodeID:  "hazmat-decon-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
