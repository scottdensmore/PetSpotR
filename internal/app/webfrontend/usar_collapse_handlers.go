package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/usarcollapse"
)

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

func (s *Server) handleUSARStructuralCollapsePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	ops := usarStore.List()
	total, criticalCollapse, stagedShoring, extractedSafe := computeUSARCollapseSummary(ops)

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

	s.renderCockpitTemplate(w, r, "usar_structural_collapse.html", "USAR Structural Collapse", data)
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

	writeCockpitJSON(w, http.StatusOK, map[string]any{
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
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	created, err := usarStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastUSARCollapseEvent("usar_collapse_updated", map[string]any{
		"type":      "usar_collapse_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateUSARCollapseStep(w http.ResponseWriter, r *http.Request) {
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
	var req domain.USARStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	if _, exists := usarStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "USAR collapse operation not found")
		return
	}

	updated, err := usarStore.UpdateStep(opID, req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastUSARCollapseEvent("usar_collapse_updated", map[string]any{
		"type":      "usar_collapse_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueUSARCollapseClearance(w http.ResponseWriter, r *http.Request) {
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
	var req usarCollapseClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	usarStore := s.ensureUSARCollapseStore()
	if _, exists := usarStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "USAR collapse operation not found")
		return
	}

	updated, err := usarStore.IssueClearance(opID, req.LeadUSARSpecCallsign, req.Notes)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastUSARCollapseEvent("usar_collapse_updated", map[string]any{
		"type":      "usar_collapse_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleUSARCollapseStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "usar_collapse")
}

func (s *Server) broadcastUSARCollapseEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("usar_collapse", "usar-collapse-hub", eventType, payload, now)
}
