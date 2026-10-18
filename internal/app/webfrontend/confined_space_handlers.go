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

	"github.com/scottdensmore/petspotr/pkg/confinedspace"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastConfinedSpaceNano atomic.Int64

func nextConfinedSpaceNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastConfinedSpaceNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastConfinedSpaceNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// ConfinedSpacePageData represents template data passed to templates/confined_space_rescue.html.
type ConfinedSpacePageData struct {
	Operations         []domain.ConfinedSpaceOperation
	TotalOperations    int
	CriticalGasAlerts  int
	RiggedBorescopes   int
	ExtractedSafeCount int
	Locale             string
}

type confinedSpaceClearanceRequest struct {
	LeadTechCallsign string `json:"leadTechCallsign"`
	Notes            string `json:"notes"`
}

func (s *Server) ensureConfinedSpaceStore() *confinedspace.Store {
	if s.confinedSpaceStore == nil {
		s.confinedSpaceStore = confinedspace.NewStore()
	}
	return s.confinedSpaceStore
}

func (s *Server) registerConfinedSpaceRoutes() {
	s.mux.HandleFunc("GET /confined-space-rescue", s.handleConfinedSpaceRescuePage)
	s.mux.HandleFunc("GET /api/v1/confined-space/operations", s.handleListConfinedSpaceOperations)
	s.mux.HandleFunc("POST /api/v1/confined-space/evaluate", s.handleEvaluateConfinedSpaceOperation)
	s.mux.HandleFunc("POST /api/v1/confined-space/operations/{id}/step", s.handleUpdateConfinedSpaceStep)
	s.mux.HandleFunc("POST /api/v1/confined-space/operations/{id}/clearance", s.handleIssueConfinedSpaceClearance)
	s.mux.HandleFunc("GET /api/v1/confined-space/stream", s.handleConfinedSpaceStream)
}

func computeConfinedSpaceSummary(ops []domain.ConfinedSpaceOperation) (total, criticalGas, rigged, extracted int) {
	total = len(ops)
	for _, op := range ops {
		if op.ToxicGasAlert || op.Status == domain.ConfinedSpaceStatusAtmosphericHazardCritical {
			criticalGas++
		}
		if op.Status == domain.ConfinedSpaceStatusBorescopePipeGateRigged {
			rigged++
		}
		if op.Status == domain.ConfinedSpaceStatusExtractedSubterraneanSafe {
			extracted++
		}
	}
	return total, criticalGas, rigged, extracted
}

func writeConfinedSpaceJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleConfinedSpaceRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	ops := csStore.List()
	total, criticalGas, rigged, extracted := computeConfinedSpaceSummary(ops)

	tmpl, err := template.New("confined_space_rescue.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/confined_space_rescue.html")
	if err != nil {
		http.Error(w, "Failed to load Confined Space Rescue template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := ConfinedSpacePageData{
		Operations:         ops,
		TotalOperations:    total,
		CriticalGasAlerts:  criticalGas,
		RiggedBorescopes:   rigged,
		ExtractedSafeCount: extracted,
		Locale:             locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListConfinedSpaceOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	allOps := csStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	spaceFilter := strings.TrimSpace(r.URL.Query().Get("spaceType"))

	filtered := make([]domain.ConfinedSpaceOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if spaceFilter != "" && !strings.EqualFold(string(op.SpaceType), spaceFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalGas, rigged, extracted := computeConfinedSpaceSummary(filtered)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"totalOperations":    total,
			"criticalGasAlerts":  criticalGas,
			"riggedBorescopes":   rigged,
			"extractedSafeCount": extracted,
		},
	})
}

func (s *Server) handleEvaluateConfinedSpaceOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.ConfinedSpaceEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeConfinedSpaceJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	created, err := csStore.EvaluateAndSave(req)
	if err != nil {
		writeConfinedSpaceJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextConfinedSpaceNano()).UTC()
	s.broadcastConfinedSpaceEvent("confined_space_updated", map[string]any{
		"type":      "confined_space_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleUpdateConfinedSpaceStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeConfinedSpaceJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.ConfinedSpaceStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeConfinedSpaceJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	if _, exists := csStore.Get(opID); !exists {
		writeConfinedSpaceJSONError(w, http.StatusNotFound, "Confined space operation not found")
		return
	}

	updated, err := csStore.UpdateStep(opID, req)
	if err != nil {
		writeConfinedSpaceJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextConfinedSpaceNano()).UTC()
	s.broadcastConfinedSpaceEvent("confined_space_updated", map[string]any{
		"type":      "confined_space_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueConfinedSpaceClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeConfinedSpaceJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req confinedSpaceClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeConfinedSpaceJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	if _, exists := csStore.Get(opID); !exists {
		writeConfinedSpaceJSONError(w, http.StatusNotFound, "Confined space operation not found")
		return
	}

	updated, err := csStore.IssueClearance(opID, req.LeadTechCallsign, req.Notes)
	if err != nil {
		writeConfinedSpaceJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextConfinedSpaceNano()).UTC()
	s.broadcastConfinedSpaceEvent("confined_space_updated", map[string]any{
		"type":      "confined_space_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleConfinedSpaceStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("confined_space_rescue")
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

func (s *Server) broadcastConfinedSpaceEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextConfinedSpaceNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "confined_space_rescue",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "confined_space_rescue",
			SenderNodeID:  "confined-space-rescue-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
