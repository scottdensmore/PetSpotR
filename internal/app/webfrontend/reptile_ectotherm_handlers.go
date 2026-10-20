package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/reptileectotherm"
)

// EctothermRescuePageData represents template data passed to templates/reptile_ectotherm_rescue.html.
type EctothermRescuePageData struct {
	Operations           []domain.EctothermRescueOperation
	TotalOperations      int
	CriticalTorporAlerts int
	StagedBaskingLures   int
	ThermalSafeCount     int
	Locale               string
}

type ectothermRescueClearanceRequest struct {
	LeadHerpTechCallsign string `json:"leadHerpTechCallsign"`
	Notes                string `json:"notes"`
}

func (s *Server) ensureEctothermRescueStore() *reptileectotherm.Store {
	if s.ectothermRescueStore == nil {
		s.ectothermRescueStore = reptileectotherm.NewStore()
	}
	return s.ectothermRescueStore
}

func (s *Server) registerEctothermRescueRoutes() {
	s.mux.HandleFunc("GET /reptile-ectotherm-rescue", s.handleReptileEctothermRescuePage)
	s.mux.HandleFunc("GET /api/v1/ectotherm-rescue/operations", s.handleListEctothermRescueOperations)
	s.mux.HandleFunc("POST /api/v1/ectotherm-rescue/evaluate", s.handleEvaluateEctothermRescueOperation)
	s.mux.HandleFunc("POST /api/v1/ectotherm-rescue/operations/{id}/step", s.handleUpdateEctothermRescueStep)
	s.mux.HandleFunc("POST /api/v1/ectotherm-rescue/operations/{id}/clearance", s.handleIssueEctothermRescueClearance)
	s.mux.HandleFunc("GET /api/v1/ectotherm-rescue/stream", s.handleEctothermRescueStream)
}

func computeEctothermRescueSummary(ops []domain.EctothermRescueOperation) (total, criticalTorpor, stagedLures, thermalSafe int) {
	total = len(ops)
	for _, op := range ops {
		if op.CriticalTorporAlert || op.DesiccationDysecdysisAlert || op.Status == domain.EctothermRescueStatusColdTorporBrumationCritical {
			criticalTorpor++
		}
		if op.Status == domain.EctothermRescueStatusBaskingLureCreviceStaged {
			stagedLures++
		}
		if op.Status == domain.EctothermRescueStatusRecoveredThermalGradientSafe {
			thermalSafe++
		}
	}
	return total, criticalTorpor, stagedLures, thermalSafe
}

func (s *Server) handleReptileEctothermRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reStore := s.ensureEctothermRescueStore()
	ops := reStore.List()
	total, criticalTorpor, stagedLures, thermalSafe := computeEctothermRescueSummary(ops)

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := EctothermRescuePageData{
		Operations:           ops,
		TotalOperations:      total,
		CriticalTorporAlerts: criticalTorpor,
		StagedBaskingLures:   stagedLures,
		ThermalSafeCount:     thermalSafe,
		Locale:               locale,
	}

	s.renderCockpitTemplate(w, r, "reptile_ectotherm_rescue.html", "Reptile & Ectotherm Rescue", data)
}

func (s *Server) handleListEctothermRescueOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reStore := s.ensureEctothermRescueStore()
	allOps := reStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	taxonFilter := strings.TrimSpace(r.URL.Query().Get("taxonType"))

	filtered := make([]domain.EctothermRescueOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if taxonFilter != "" && !strings.EqualFold(string(op.TaxonType), taxonFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalTorpor, stagedLures, thermalSafe := computeEctothermRescueSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"totalOperations":      total,
			"criticalTorporAlerts": criticalTorpor,
			"stagedBaskingLures":   stagedLures,
			"thermalSafeCount":     thermalSafe,
		},
	})
}

func (s *Server) handleEvaluateEctothermRescueOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.EctothermEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	reStore := s.ensureEctothermRescueStore()
	created, err := reStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastEctothermRescueEvent("ectotherm_rescue_updated", map[string]any{
		"type":      "ectotherm_rescue_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateEctothermRescueStep(w http.ResponseWriter, r *http.Request) {
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
	var req domain.EctothermStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	reStore := s.ensureEctothermRescueStore()
	if _, exists := reStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Ectotherm rescue operation not found")
		return
	}

	updated, err := reStore.UpdateStep(opID, req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastEctothermRescueEvent("ectotherm_rescue_updated", map[string]any{
		"type":      "ectotherm_rescue_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueEctothermRescueClearance(w http.ResponseWriter, r *http.Request) {
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
	var req ectothermRescueClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	reStore := s.ensureEctothermRescueStore()
	if _, exists := reStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Ectotherm rescue operation not found")
		return
	}

	updated, err := reStore.IssueClearance(opID, req.LeadHerpTechCallsign, req.Notes)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastEctothermRescueEvent("ectotherm_rescue_updated", map[string]any{
		"type":      "ectotherm_rescue_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleEctothermRescueStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "ectotherm_rescue")
}

func (s *Server) broadcastEctothermRescueEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("ectotherm_rescue", "ectotherm-rescue-hub", eventType, payload, now)
}
