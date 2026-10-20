package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/biosecurityiso"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// BiosecurityIsolationPageData represents template data passed to templates/biosecurity_isolation_triage.html.
type BiosecurityIsolationPageData struct {
	Operations          []domain.BiosecurityIsolationOperation
	ActiveOperation     *domain.BiosecurityIsolationOperation
	TotalOperations     int
	CriticalSepsisCount int
	StagedIsoCount      int
	ClearedSafeCount    int
	Locale              string
}

type biosecurityStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureBiosecurityIsolationStore() *biosecurityiso.Store {
	if s.biosecurityIsolationStore == nil {
		s.biosecurityIsolationStore = biosecurityiso.NewStore()
	}
	return s.biosecurityIsolationStore
}

func (s *Server) registerBiosecurityIsolationRoutes() {
	s.mux.HandleFunc("GET /biosecurity-isolation-triage", s.handleBiosecurityIsolationTriagePage)
	s.mux.HandleFunc("GET /api/v1/biosecurity-isolation/operations", s.handleListBiosecurityIsolationOperations)
	s.mux.HandleFunc("POST /api/v1/biosecurity-isolation/evaluate", s.handleEvaluateBiosecurityIsolationOperation)
	s.mux.HandleFunc("POST /api/v1/biosecurity-isolation/operations/{id}/steps", s.handleUpdateBiosecurityIsolationStep)
	s.mux.HandleFunc("POST /api/v1/biosecurity-isolation/operations/{id}/step", s.handleUpdateBiosecurityIsolationStep)
	s.mux.HandleFunc("POST /api/v1/biosecurity-isolation/operations/{id}/clearance", s.handleIssueBiosecurityIsolationClearance)
	s.mux.HandleFunc("GET /api/v1/biosecurity-isolation/stream", s.handleBiosecurityIsolationStream)
}

func computeBiosecurityIsolationSummary(ops []domain.BiosecurityIsolationOperation) (total, criticalSepsis, stagedIso, clearedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.BiosecurityStatusCriticalNeutropenicSepsisBreach:
			criticalSepsis++
		case domain.BiosecurityStatusStagedNegativePressureIsoCohort:
			stagedIso++
		case domain.BiosecurityStatusClearedSafePCRNegativeDecon:
			clearedSafe++
		}
	}
	return total, criticalSepsis, stagedIso, clearedSafe
}

func (s *Server) handleBiosecurityIsolationTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	biStore := s.ensureBiosecurityIsolationStore()
	ops := biStore.List()
	total, criticalSepsis, stagedIso, clearedSafe := computeBiosecurityIsolationSummary(ops)

	var activeOp *domain.BiosecurityIsolationOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := BiosecurityIsolationPageData{
		Operations:          ops,
		ActiveOperation:     activeOp,
		TotalOperations:     total,
		CriticalSepsisCount: criticalSepsis,
		StagedIsoCount:      stagedIso,
		ClearedSafeCount:    clearedSafe,
		Locale:              locale,
	}

	s.renderCockpitTemplate(w, r, "biosecurity_isolation_triage.html", "Biosecurity Isolation Cohort Triage", data)
}

func (s *Server) handleListBiosecurityIsolationOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	biStore := s.ensureBiosecurityIsolationStore()
	allOps := biStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	pathogenFilter := strings.TrimSpace(r.URL.Query().Get("pathogen_type"))
	if pathogenFilter == "" {
		pathogenFilter = strings.TrimSpace(r.URL.Query().Get("pathogen"))
	}

	filtered := make([]domain.BiosecurityIsolationOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if pathogenFilter != "" && !strings.EqualFold(string(op.PathogenType), pathogenFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalSepsis, stagedIso, clearedSafe := computeBiosecurityIsolationSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":      total,
			"critical_sepsis_count": criticalSepsis,
			"staged_iso":            stagedIso,
			"cleared_safe":          clearedSafe,
		},
	})
}

func (s *Server) handleEvaluateBiosecurityIsolationOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.BiosecurityEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	biStore := s.ensureBiosecurityIsolationStore()
	created, err := biStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastBiosecurityIsoEvent("biosecurity_isolation_updated", map[string]any{
		"type":      "biosecurity_isolation_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateBiosecurityIsolationStep(w http.ResponseWriter, r *http.Request) {
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
	var payload biosecurityStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	biStore := s.ensureBiosecurityIsolationStore()
	if _, exists := biStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Biosecurity isolation operation not found")
		return
	}

	updated, err := biStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastBiosecurityIsoEvent("biosecurity_isolation_updated", map[string]any{
		"type":      "biosecurity_isolation_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueBiosecurityIsolationClearance(w http.ResponseWriter, r *http.Request) {
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

	biStore := s.ensureBiosecurityIsolationStore()
	if _, exists := biStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Biosecurity isolation operation not found")
		return
	}

	updated, err := biStore.IssueClearance(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastBiosecurityIsoEvent("biosecurity_isolation_updated", map[string]any{
		"type":      "biosecurity_isolation_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleBiosecurityIsolationStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "biosecurity_isolation")
}

func (s *Server) broadcastBiosecurityIsoEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("biosecurity_isolation", "biosecurity-iso-hub", eventType, payload, now)
}
