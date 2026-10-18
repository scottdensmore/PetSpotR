package webfrontend

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/verticalrescue"
)

// VerticalRescuePageData represents template data passed to templates/vertical_rope_rescue.html.
type VerticalRescuePageData struct {
	Operations        []domain.VerticalRescueOperation
	TotalOps          int
	CriticalFallCount int
	RiggedTwinCount   int
	LoweredSafeCount  int
	Locale            string
}

func (s *Server) ensureVerticalRescueStore() *verticalrescue.Store {
	if s.verticalRescueStore == nil {
		s.verticalRescueStore = verticalrescue.NewStore()
	}
	return s.verticalRescueStore
}

func (s *Server) registerVerticalRescueRoutes() {
	s.mux.HandleFunc("GET /vertical-rope-rescue", s.handleVerticalRescuePage)
	s.mux.HandleFunc("GET /api/v1/vertical-rescue/operations", s.handleListVerticalRescueOperations)
	s.mux.HandleFunc("POST /api/v1/vertical-rescue/evaluate", s.handleEvaluateVerticalRescue)
	s.mux.HandleFunc("POST /api/v1/vertical-rescue/operations/{id}/step", s.handleUpdateVerticalRescueStep)
	s.mux.HandleFunc("POST /api/v1/vertical-rescue/operations/{id}/clearance", s.handleIssueVerticalRescueClearance)
}

func writeVerticalRescueJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func computeVerticalRescueStats(ops []domain.VerticalRescueOperation) (total, critical, rigged, lowered int) {
	total = len(ops)
	for _, op := range ops {
		if op.HighRiseFallShockAlert || op.CanopyFailureAlert || op.Status == domain.VerticalStatusFallShockCritical {
			critical++
		}
		if op.Status == domain.VerticalStatusTwinTensionRigged {
			rigged++
		}
		if op.Status == domain.VerticalStatusLoweredGroundSafe {
			lowered++
		}
	}
	return total, critical, rigged, lowered
}

func (s *Server) handleVerticalRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vrStore := s.ensureVerticalRescueStore()
	ops := vrStore.List()
	total, critical, rigged, lowered := computeVerticalRescueStats(ops)

	tmpl, err := template.New("vertical_rope_rescue.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/vertical_rope_rescue.html")
	if err != nil {
		http.Error(w, "Failed to load Vertical Rope Rescue template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := VerticalRescuePageData{
		Operations:        ops,
		TotalOps:          total,
		CriticalFallCount: critical,
		RiggedTwinCount:   rigged,
		LoweredSafeCount:  lowered,
		Locale:            locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListVerticalRescueOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vrStore := s.ensureVerticalRescueStore()
	allOps := vrStore.List()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	envFilter := strings.TrimSpace(r.URL.Query().Get("environment"))

	ops := make([]domain.VerticalRescueOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if envFilter != "" && !strings.EqualFold(string(op.EnvironmentType), envFilter) {
			continue
		}
		ops = append(ops, op)
	}

	total, critical, rigged, lowered := computeVerticalRescueStats(ops)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations":        ops,
		"ops":               ops,
		"total":             total,
		"criticalFallCount": critical,
		"riggedTwinCount":   rigged,
		"loweredSafeCount":  lowered,
	})
}

func (s *Server) handleEvaluateVerticalRescue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.VerticalRescueEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVerticalRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	vrStore := s.ensureVerticalRescueStore()
	op, err := vrStore.Evaluate(req)
	if err != nil {
		writeVerticalRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broadcastVerticalRescueEvent("vertical_rescue_updated", map[string]any{
		"type":      "vertical_rescue_updated",
		"operation": op,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleUpdateVerticalRescueStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeVerticalRescueJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.VerticalStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeVerticalRescueJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	vrStore := s.ensureVerticalRescueStore()
	if _, exists := vrStore.Get(opID); !exists {
		writeVerticalRescueJSONError(w, http.StatusNotFound, "Vertical rescue operation not found")
		return
	}

	updated, err := vrStore.UpdateStep(opID, req)
	if err != nil {
		writeVerticalRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broadcastVerticalRescueEvent("vertical_rescue_updated", map[string]any{
		"type":      "vertical_rescue_updated",
		"operation": updated,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueVerticalRescueClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeVerticalRescueJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	vrStore := s.ensureVerticalRescueStore()
	if _, exists := vrStore.Get(opID); !exists {
		writeVerticalRescueJSONError(w, http.StatusNotFound, "Vertical rescue operation not found")
		return
	}

	cleared, err := vrStore.IssueClearance(opID)
	if err != nil {
		writeVerticalRescueJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	s.broadcastVerticalRescueEvent("vertical_rescue_updated", map[string]any{
		"type":      "vertical_rescue_updated",
		"operation": cleared,
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(cleared)
}

func (s *Server) broadcastVerticalRescueEvent(eventType string, payload map[string]any) {
	now := time.Now().UTC()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   "evt_" + eventType + "_" + randomHex8(),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "vertical_rescue",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}
	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "vertical_rescue",
			SenderNodeID:  "vertical-rescue-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
