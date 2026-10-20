package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/urethralobstruction"
)

// UrethralObstructionPageData represents template data passed to templates/urethral_obstruction_triage.html.
type UrethralObstructionPageData struct {
	Operations          []domain.UrethralObstructionOperation
	ActiveOperation     *domain.UrethralObstructionOperation
	TotalOperations     int
	CriticalHyperKCount int
	StagedUnblockCount  int
	ClearedSafeCount    int
	Locale              string
}

type urethralStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureUrethralObstructionStore() *urethralobstruction.Store {
	if s.urethralObstructionStore == nil {
		s.urethralObstructionStore = urethralobstruction.NewStore()
	}
	return s.urethralObstructionStore
}

func (s *Server) registerUrethralObstructionRoutes() {
	s.mux.HandleFunc("GET /urethral-obstruction-triage", s.handleUrethralObstructionTriagePage)
	s.mux.HandleFunc("GET /api/v1/urethral-obstruction/operations", s.handleListUrethralObstructionOperations)
	s.mux.HandleFunc("POST /api/v1/urethral-obstruction/evaluate", s.handleEvaluateUrethralObstructionOperation)
	s.mux.HandleFunc("POST /api/v1/urethral-obstruction/operations/{id}/steps", s.handleUpdateUrethralObstructionStep)
	s.mux.HandleFunc("POST /api/v1/urethral-obstruction/operations/{id}/step", s.handleUpdateUrethralObstructionStep)
	s.mux.HandleFunc("POST /api/v1/urethral-obstruction/operations/{id}/clearance", s.handleIssueUrethralObstructionClearance)
	s.mux.HandleFunc("GET /api/v1/urethral-obstruction/stream", s.handleUrethralObstructionStream)
}

func computeUrethralObstructionSummary(ops []domain.UrethralObstructionOperation) (total, criticalHyperK, stagedUnblock, clearedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.UrethralStatusCriticalHyperkalemicBradycardia:
			criticalHyperK++
		case domain.UrethralStatusStagedCalciumRetrohydropropulsion:
			stagedUnblock++
		case domain.UrethralStatusClearedIndwellingNormokalemic:
			clearedSafe++
		}
	}
	return total, criticalHyperK, stagedUnblock, clearedSafe
}

func (s *Server) handleUrethralObstructionTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uoStore := s.ensureUrethralObstructionStore()
	ops := uoStore.List()
	total, criticalHyperK, stagedUnblock, clearedSafe := computeUrethralObstructionSummary(ops)

	var activeOp *domain.UrethralObstructionOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := UrethralObstructionPageData{
		Operations:          ops,
		ActiveOperation:     activeOp,
		TotalOperations:     total,
		CriticalHyperKCount: criticalHyperK,
		StagedUnblockCount:  stagedUnblock,
		ClearedSafeCount:    clearedSafe,
		Locale:              locale,
	}

	s.renderCockpitTemplate(w, r, "urethral_obstruction_triage.html", "Feline Urethral Obstruction, Hyperkalemic Bradycardia & Calcium Gluconate Cardioprotection Triage", data)
}

func (s *Server) handleListUrethralObstructionOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uoStore := s.ensureUrethralObstructionStore()
	allOps := uoStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	causeFilter := strings.TrimSpace(r.URL.Query().Get("obstruction_cause"))
	if causeFilter == "" {
		causeFilter = strings.TrimSpace(r.URL.Query().Get("cause"))
	}

	filtered := make([]domain.UrethralObstructionOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if causeFilter != "" && !strings.EqualFold(string(op.ObstructionCause), causeFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalHyperK, stagedUnblock, clearedSafe := computeUrethralObstructionSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":      total,
			"critical_hyperk_count": criticalHyperK,
			"staged_unblock":        stagedUnblock,
			"cleared_safe":          clearedSafe,
		},
	})
}

func (s *Server) handleEvaluateUrethralObstructionOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.UrethralEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	uoStore := s.ensureUrethralObstructionStore()
	created, err := uoStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastUrethralObstructionEvent("urethral_obstruction_updated", map[string]any{
		"type":      "urethral_obstruction_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateUrethralObstructionStep(w http.ResponseWriter, r *http.Request) {
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
	var payload urethralStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	uoStore := s.ensureUrethralObstructionStore()
	if _, exists := uoStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Urethral obstruction triage operation not found")
		return
	}

	updated, err := uoStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastUrethralObstructionEvent("urethral_obstruction_updated", map[string]any{
		"type":      "urethral_obstruction_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueUrethralObstructionClearance(w http.ResponseWriter, r *http.Request) {
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

	uoStore := s.ensureUrethralObstructionStore()
	if _, exists := uoStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Urethral obstruction triage operation not found")
		return
	}

	updated, err := uoStore.IssueClearance(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastUrethralObstructionEvent("urethral_obstruction_updated", map[string]any{
		"type":      "urethral_obstruction_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleUrethralObstructionStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "urethral_obstruction")
}

func (s *Server) broadcastUrethralObstructionEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("urethral_obstruction", "urethral-obstruction-hub", eventType, payload, now)
}
