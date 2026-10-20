package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/envenomationtoxin"
)

// EnvenomationToxinPageData represents template data passed to templates/envenomation_toxin_triage.html.
type EnvenomationToxinPageData struct {
	Operations           []domain.EnvenomationTriageOperation
	ActiveOperation      *domain.EnvenomationTriageOperation
	TotalOperations      int
	CriticalToxinCount   int
	StagedAntivenomCount int
	StabilizedSafeCount  int
	Locale               string
}

type envenomationStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureEnvenomationToxinStore() *envenomationtoxin.Store {
	if s.envenomationToxinStore == nil {
		s.envenomationToxinStore = envenomationtoxin.NewStore()
	}
	return s.envenomationToxinStore
}

func (s *Server) registerEnvenomationToxinRoutes() {
	s.mux.HandleFunc("GET /envenomation-toxin-triage", s.handleEnvenomationToxinTriagePage)
	s.mux.HandleFunc("GET /api/v1/envenomation-toxin/operations", s.handleListEnvenomationToxinOperations)
	s.mux.HandleFunc("POST /api/v1/envenomation-toxin/evaluate", s.handleEvaluateEnvenomationToxinOperation)
	s.mux.HandleFunc("POST /api/v1/envenomation-toxin/operations/{id}/steps", s.handleUpdateEnvenomationToxinStep)
	s.mux.HandleFunc("POST /api/v1/envenomation-toxin/operations/{id}/step", s.handleUpdateEnvenomationToxinStep)
	s.mux.HandleFunc("POST /api/v1/envenomation-toxin/operations/{id}/clearance", s.handleIssueEnvenomationToxinClearance)
	s.mux.HandleFunc("GET /api/v1/envenomation-toxin/stream", s.handleEnvenomationToxinStream)
}

func computeEnvenomationToxinSummary(ops []domain.EnvenomationTriageOperation) (total, criticalToxin, stagedAntivenom, stabilizedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.EnvenomationStatusCritical:
			criticalToxin++
		case domain.EnvenomationStatusStaged:
			stagedAntivenom++
		case domain.EnvenomationStatusStabilized:
			stabilizedSafe++
		}
	}
	return total, criticalToxin, stagedAntivenom, stabilizedSafe
}

func (s *Server) handleEnvenomationToxinTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	etStore := s.ensureEnvenomationToxinStore()
	ops := etStore.List()
	total, criticalToxin, stagedAntivenom, stabilizedSafe := computeEnvenomationToxinSummary(ops)

	var activeOp *domain.EnvenomationTriageOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := EnvenomationToxinPageData{
		Operations:           ops,
		ActiveOperation:      activeOp,
		TotalOperations:      total,
		CriticalToxinCount:   criticalToxin,
		StagedAntivenomCount: stagedAntivenom,
		StabilizedSafeCount:  stabilizedSafe,
		Locale:               locale,
	}

	s.renderCockpitTemplate(w, r, "envenomation_toxin_triage.html", "Envenomation & Toxin Triage", data)
}

func (s *Server) handleListEnvenomationToxinOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	etStore := s.ensureEnvenomationToxinStore()
	allOps := etStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	toxinFilter := strings.TrimSpace(r.URL.Query().Get("toxin_type"))
	if toxinFilter == "" {
		toxinFilter = strings.TrimSpace(r.URL.Query().Get("toxin"))
	}

	filtered := make([]domain.EnvenomationTriageOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if toxinFilter != "" && !strings.EqualFold(string(op.ToxinType), toxinFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalToxin, stagedAntivenom, stabilizedSafe := computeEnvenomationToxinSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":     total,
			"critical_toxin_count": criticalToxin,
			"staged_antivenom":     stagedAntivenom,
			"stabilized_safe":      stabilizedSafe,
		},
	})
}

func (s *Server) handleEvaluateEnvenomationToxinOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.EnvenomationEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	etStore := s.ensureEnvenomationToxinStore()
	created, err := etStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastEnvenomationToxinEvent("envenomation_toxin_updated", map[string]any{
		"type":      "envenomation_toxin_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateEnvenomationToxinStep(w http.ResponseWriter, r *http.Request) {
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
	var payload envenomationStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	etStore := s.ensureEnvenomationToxinStore()
	if _, exists := etStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Envenomation toxin triage operation not found")
		return
	}

	updated, err := etStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastEnvenomationToxinEvent("envenomation_toxin_updated", map[string]any{
		"type":      "envenomation_toxin_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueEnvenomationToxinClearance(w http.ResponseWriter, r *http.Request) {
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
	var discard map[string]any
	if err := json.NewDecoder(r.Body).Decode(&discard); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	etStore := s.ensureEnvenomationToxinStore()
	if _, exists := etStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Envenomation toxin triage operation not found")
		return
	}

	updated, err := etStore.IssueClearance(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastEnvenomationToxinEvent("envenomation_toxin_updated", map[string]any{
		"type":      "envenomation_toxin_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleEnvenomationToxinStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "envenomation_toxin")
}

func (s *Server) broadcastEnvenomationToxinEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("envenomation_toxin", "envenomation-toxin-hub", eventType, payload, now)
}
