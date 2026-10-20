package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/confinedspace"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

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

func (s *Server) handleConfinedSpaceRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	ops := csStore.List()
	total, criticalGas, rigged, extracted := computeConfinedSpaceSummary(ops)

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

	s.renderCockpitTemplate(w, r, "confined_space_rescue.html", "Confined Space Rescue", data)
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

	writeCockpitJSON(w, http.StatusOK, map[string]any{
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
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	created, err := csStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastConfinedSpaceEvent("confined_space_updated", map[string]any{
		"type":      "confined_space_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateConfinedSpaceStep(w http.ResponseWriter, r *http.Request) {
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
	var req domain.ConfinedSpaceStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	if _, exists := csStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Confined space operation not found")
		return
	}

	updated, err := csStore.UpdateStep(opID, req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastConfinedSpaceEvent("confined_space_updated", map[string]any{
		"type":      "confined_space_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueConfinedSpaceClearance(w http.ResponseWriter, r *http.Request) {
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
	var req confinedSpaceClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	csStore := s.ensureConfinedSpaceStore()
	if _, exists := csStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Confined space operation not found")
		return
	}

	updated, err := csStore.IssueClearance(opID, req.LeadTechCallsign, req.Notes)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastConfinedSpaceEvent("confined_space_updated", map[string]any{
		"type":      "confined_space_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleConfinedSpaceStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "confined_space_rescue")
}

func (s *Server) broadcastConfinedSpaceEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("confined_space_rescue", "confined-space-rescue-hub", eventType, payload, now)
}
