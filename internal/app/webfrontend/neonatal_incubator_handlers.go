package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/neonatalincubator"
)

// NeonatalIncubatorPageData represents template data passed to templates/neonatal_incubator_triage.html.
type NeonatalIncubatorPageData struct {
	Operations           []domain.NeonatalTriageOperation
	ActiveOperation      *domain.NeonatalTriageOperation
	TotalOperations      int
	CriticalFadingCount  int
	StagedIncubatorCount int
	StabilizedSafeCount  int
	Locale               string
}

type neonatalStepUpdatePayload struct {
	StepID    string `json:"step_id"`
	StepIDAlt string `json:"stepId"`
	Completed bool   `json:"completed"`
}

func (s *Server) ensureNeonatalIncubatorStore() *neonatalincubator.Store {
	if s.neonatalIncubatorStore == nil {
		s.neonatalIncubatorStore = neonatalincubator.NewStore()
	}
	return s.neonatalIncubatorStore
}

func (s *Server) registerNeonatalIncubatorRoutes() {
	s.mux.HandleFunc("GET /neonatal-incubator-triage", s.handleNeonatalIncubatorTriagePage)
	s.mux.HandleFunc("GET /api/v1/neonatal-triage/operations", s.handleListNeonatalTriageOperations)
	s.mux.HandleFunc("POST /api/v1/neonatal-triage/evaluate", s.handleEvaluateNeonatalTriageOperation)
	s.mux.HandleFunc("POST /api/v1/neonatal-triage/operations/{id}/steps", s.handleUpdateNeonatalTriageStep)
	s.mux.HandleFunc("POST /api/v1/neonatal-triage/operations/{id}/step", s.handleUpdateNeonatalTriageStep)
	s.mux.HandleFunc("POST /api/v1/neonatal-triage/operations/{id}/clearance", s.handleIssueNeonatalTriageClearance)
	s.mux.HandleFunc("GET /api/v1/neonatal-triage/stream", s.handleNeonatalTriageStream)
}

func computeNeonatalTriageSummary(ops []domain.NeonatalTriageOperation) (total, criticalFading, stagedIncubator, stabilizedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.NeonatalTriageStatusFadingTriadHypothermicCritical:
			criticalFading++
		case domain.NeonatalTriageStatusIncubatorWarmedDextroseStaged:
			stagedIncubator++
		case domain.NeonatalTriageStatusStabilizedNursingGainingSafe:
			stabilizedSafe++
		}
	}
	return total, criticalFading, stagedIncubator, stabilizedSafe
}

func (s *Server) handleNeonatalIncubatorTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	niStore := s.ensureNeonatalIncubatorStore()
	ops := niStore.List()
	total, criticalFading, stagedIncubator, stabilizedSafe := computeNeonatalTriageSummary(ops)

	var activeOp *domain.NeonatalTriageOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := NeonatalIncubatorPageData{
		Operations:           ops,
		ActiveOperation:      activeOp,
		TotalOperations:      total,
		CriticalFadingCount:  criticalFading,
		StagedIncubatorCount: stagedIncubator,
		StabilizedSafeCount:  stabilizedSafe,
		Locale:               locale,
	}

	s.renderCockpitTemplate(w, r, "neonatal_incubator_triage.html", "Neonatal Incubator Triage", data)
}

func (s *Server) handleListNeonatalTriageOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	niStore := s.ensureNeonatalIncubatorStore()
	allOps := niStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	litterFilter := strings.TrimSpace(r.URL.Query().Get("litter_type"))
	if litterFilter == "" {
		litterFilter = strings.TrimSpace(r.URL.Query().Get("litterType"))
	}

	filtered := make([]domain.NeonatalTriageOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if litterFilter != "" && !strings.EqualFold(string(op.LitterType), litterFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalFading, stagedIncubator, stabilizedSafe := computeNeonatalTriageSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":       total,
			"critical_fading_count":  criticalFading,
			"staged_incubator_count": stagedIncubator,
			"stabilized_safe_count":  stabilizedSafe,
			"totalOperations":        total,
			"criticalFadingCount":    criticalFading,
			"stagedIncubatorCount":   stagedIncubator,
			"stabilizedSafeCount":    stabilizedSafe,
		},
	})
}

func (s *Server) handleEvaluateNeonatalTriageOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.NeonatalEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	niStore := s.ensureNeonatalIncubatorStore()
	created, err := niStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastNeonatalTriageEvent("neonatal_triage_updated", map[string]any{
		"type":      "neonatal_triage_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateNeonatalTriageStep(w http.ResponseWriter, r *http.Request) {
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
	var payload neonatalStepUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDAlt)
	}

	niStore := s.ensureNeonatalIncubatorStore()
	if _, exists := niStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Neonatal triage operation not found")
		return
	}

	updated, err := niStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastNeonatalTriageEvent("neonatal_triage_updated", map[string]any{
		"type":      "neonatal_triage_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueNeonatalTriageClearance(w http.ResponseWriter, r *http.Request) {
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

	niStore := s.ensureNeonatalIncubatorStore()
	if _, exists := niStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Neonatal triage operation not found")
		return
	}

	updated, err := niStore.IssueClearance(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastNeonatalTriageEvent("neonatal_triage_updated", map[string]any{
		"type":      "neonatal_triage_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleNeonatalTriageStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "neonatal_triage")
}

func (s *Server) broadcastNeonatalTriageEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("neonatal_triage", "neonatal-triage-hub", eventType, payload, now)
}
