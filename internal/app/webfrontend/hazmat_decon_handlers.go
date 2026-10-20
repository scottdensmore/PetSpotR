package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/hazmatdecon"
)

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

func (s *Server) handleHazmatDeconPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	ops := hzStore.List()
	total, criticalSmoke, staged, cleared := computeHazmatDeconSummary(ops)

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

	s.renderCockpitTemplate(w, r, "hazmat_decon.html", "Hazmat Decon", data)
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

	writeCockpitJSON(w, http.StatusOK, map[string]any{
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
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	created, err := hzStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHazmatDeconEvent("hazmat_decon_updated", map[string]any{
		"type":      "hazmat_decon_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateHazmatDeconStep(w http.ResponseWriter, r *http.Request) {
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
	var req domain.HazmatStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	if _, exists := hzStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Hazmat decon operation not found")
		return
	}

	updated, err := hzStore.UpdateStep(opID, req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHazmatDeconEvent("hazmat_decon_updated", map[string]any{
		"type":      "hazmat_decon_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueHazmatDeconClearance(w http.ResponseWriter, r *http.Request) {
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
	var req hazmatDeconClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	hzStore := s.ensureHazmatDeconStore()
	if _, exists := hzStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Hazmat decon operation not found")
		return
	}

	updated, err := hzStore.IssueClearance(opID, req.LeadToxTechCallsign, req.Notes)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHazmatDeconEvent("hazmat_decon_updated", map[string]any{
		"type":      "hazmat_decon_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleHazmatDeconStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "hazmat_decon")
}

func (s *Server) broadcastHazmatDeconEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("hazmat_decon", "hazmat-decon-hub", eventType, payload, now)
}
