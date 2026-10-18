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

	"github.com/scottdensmore/petspotr/pkg/avianrecovery"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastAvianRecoveryNano atomic.Int64

func nextAvianRecoveryNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastAvianRecoveryNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastAvianRecoveryNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// AvianRecoveryPageData represents template data passed to templates/avian_recovery.html.
type AvianRecoveryPageData struct {
	Operations            []domain.AvianRecoveryOperation
	TotalOperations       int
	CriticalThermalAlerts int
	StagedAcousticLures   int
	RecoveredAviaryCount  int
	Locale                string
}

type avianClearanceRequest struct {
	LeadAvianTechCallsign string `json:"leadAvianTechCallsign"`
	Notes                 string `json:"notes"`
}

func (s *Server) ensureAvianRecoveryStore() *avianrecovery.Store {
	if s.avianRecoveryStore == nil {
		s.avianRecoveryStore = avianrecovery.NewStore()
	}
	return s.avianRecoveryStore
}

func (s *Server) registerAvianRecoveryRoutes() {
	s.mux.HandleFunc("GET /avian-recovery", s.handleAvianRecoveryPage)
	s.mux.HandleFunc("GET /api/v1/avian-recovery/operations", s.handleListAvianRecoveryOperations)
	s.mux.HandleFunc("POST /api/v1/avian-recovery/evaluate", s.handleEvaluateAvianRecoveryOperation)
	s.mux.HandleFunc("POST /api/v1/avian-recovery/operations/{id}/step", s.handleUpdateAvianRecoveryStep)
	s.mux.HandleFunc("POST /api/v1/avian-recovery/operations/{id}/clearance", s.handleIssueAvianRecoveryClearance)
	s.mux.HandleFunc("GET /api/v1/avian-recovery/stream", s.handleAvianRecoveryStream)
}

func computeAvianRecoverySummary(ops []domain.AvianRecoveryOperation) (total, criticalThermal, staged, recovered int) {
	total = len(ops)
	for _, op := range ops {
		if op.ThermalDriftAlert || op.CropStarvationColdAlert || op.Status == domain.AvianStatusThermalDriftPredationCritical {
			criticalThermal++
		}
		if op.Status == domain.AvianStatusAcousticLurePerchStaged {
			staged++
		}
		if op.Status == domain.AvianStatusRecoveredAviarySafe {
			recovered++
		}
	}
	return total, criticalThermal, staged, recovered
}

func writeAvianRecoveryJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleAvianRecoveryPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	ops := avStore.List()
	total, criticalThermal, staged, recovered := computeAvianRecoverySummary(ops)

	tmpl, err := template.New("avian_recovery.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/avian_recovery.html")
	if err != nil {
		http.Error(w, "Failed to load Avian Recovery template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := AvianRecoveryPageData{
		Operations:            ops,
		TotalOperations:       total,
		CriticalThermalAlerts: criticalThermal,
		StagedAcousticLures:   staged,
		RecoveredAviaryCount:  recovered,
		Locale:                locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListAvianRecoveryOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	allOps := avStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	classFilter := strings.TrimSpace(r.URL.Query().Get("avianClass"))

	filtered := make([]domain.AvianRecoveryOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if classFilter != "" && !strings.EqualFold(string(op.AvianClass), classFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalThermal, staged, recovered := computeAvianRecoverySummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"totalOperations":       total,
			"criticalThermalAlerts": criticalThermal,
			"stagedAcousticLures":   staged,
			"recoveredAviaryCount":  recovered,
		},
	})
}

func (s *Server) handleEvaluateAvianRecoveryOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.AvianEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAvianRecoveryJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	created, err := avStore.EvaluateAndSave(req)
	if err != nil {
		writeAvianRecoveryJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAvianRecoveryNano()).UTC()
	s.broadcastAvianRecoveryEvent("avian_recovery_updated", map[string]any{
		"type":      "avian_recovery_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateAvianRecoveryStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeAvianRecoveryJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.AvianStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAvianRecoveryJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	if _, exists := avStore.Get(opID); !exists {
		writeAvianRecoveryJSONError(w, http.StatusNotFound, "Avian recovery operation not found")
		return
	}

	updated, err := avStore.UpdateStep(opID, req)
	if err != nil {
		writeAvianRecoveryJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAvianRecoveryNano()).UTC()
	s.broadcastAvianRecoveryEvent("avian_recovery_updated", map[string]any{
		"type":      "avian_recovery_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueAvianRecoveryClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeAvianRecoveryJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req avianClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeAvianRecoveryJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	if _, exists := avStore.Get(opID); !exists {
		writeAvianRecoveryJSONError(w, http.StatusNotFound, "Avian recovery operation not found")
		return
	}

	updated, err := avStore.IssueClearance(opID, req.LeadAvianTechCallsign, req.Notes)
	if err != nil {
		writeAvianRecoveryJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAvianRecoveryNano()).UTC()
	s.broadcastAvianRecoveryEvent("avian_recovery_updated", map[string]any{
		"type":      "avian_recovery_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleAvianRecoveryStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("avian_recovery")
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

func (s *Server) broadcastAvianRecoveryEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextAvianRecoveryNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "avian_recovery",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "avian_recovery",
			SenderNodeID:  "avian-recovery-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
