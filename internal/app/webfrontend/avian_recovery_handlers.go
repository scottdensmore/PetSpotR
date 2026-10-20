package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/avianrecovery"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

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

func (s *Server) handleAvianRecoveryPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	ops := avStore.List()
	total, criticalThermal, staged, recovered := computeAvianRecoverySummary(ops)

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

	s.renderCockpitTemplate(w, r, "avian_recovery.html", "Avian Recovery", data)
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

	writeCockpitJSON(w, http.StatusOK, map[string]any{
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
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	created, err := avStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAvianRecoveryEvent("avian_recovery_updated", map[string]any{
		"type":      "avian_recovery_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateAvianRecoveryStep(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.AvianStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	if _, exists := avStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Avian recovery operation not found")
		return
	}

	updated, err := avStore.UpdateStep(opID, req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAvianRecoveryEvent("avian_recovery_updated", map[string]any{
		"type":      "avian_recovery_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueAvianRecoveryClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req avianClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	avStore := s.ensureAvianRecoveryStore()
	if _, exists := avStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Avian recovery operation not found")
		return
	}

	updated, err := avStore.IssueClearance(opID, req.LeadAvianTechCallsign, req.Notes)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAvianRecoveryEvent("avian_recovery_updated", map[string]any{
		"type":      "avian_recovery_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAvianRecoveryStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "avian_recovery")
}

func (s *Server) broadcastAvianRecoveryEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("avian_recovery", "avian-recovery-hub", eventType, payload, now)
}
