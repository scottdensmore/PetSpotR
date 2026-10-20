package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/toxicologypoison"
)

// ToxicologyPoisonPageData represents template data passed to templates/toxicology_poison_triage.html.
type ToxicologyPoisonPageData struct {
	Operations          []domain.ToxicologyPoisonOperation
	ActiveOperation     *domain.ToxicologyPoisonOperation
	TotalOperations     int
	CriticalPoisonCount int
	StagedDeconCount    int
	ClearedSafeCount    int
	Locale              string
}

type toxicologyStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureToxicologyPoisonStore() *toxicologypoison.Store {
	if s.toxicologyPoisonStore == nil {
		s.toxicologyPoisonStore = toxicologypoison.NewStore()
	}
	return s.toxicologyPoisonStore
}

func (s *Server) registerToxicologyPoisonRoutes() {
	s.mux.HandleFunc("GET /toxicology-poison-triage", s.handleToxicologyPoisonTriagePage)
	s.mux.HandleFunc("GET /api/v1/toxicology-poison/operations", s.handleListToxicologyPoisonOperations)
	s.mux.HandleFunc("POST /api/v1/toxicology-poison/evaluate", s.handleEvaluateToxicologyPoisonOperation)
	s.mux.HandleFunc("POST /api/v1/toxicology-poison/operations/{id}/steps", s.handleUpdateToxicologyPoisonStep)
	s.mux.HandleFunc("POST /api/v1/toxicology-poison/operations/{id}/step", s.handleUpdateToxicologyPoisonStep)
	s.mux.HandleFunc("POST /api/v1/toxicology-poison/operations/{id}/clearance", s.handleIssueToxicologyPoisonClearance)
	s.mux.HandleFunc("GET /api/v1/toxicology-poison/stream", s.handleToxicologyPoisonStream)
}

func computeToxicologyPoisonSummary(ops []domain.ToxicologyPoisonOperation) (total, criticalPoison, stagedDecon, clearedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.ToxicologyStatusCriticalCoagulopathyHepaticNeuro:
			criticalPoison++
		case domain.ToxicologyStatusStagedVitaminK1ILECharcoalDecon:
			stagedDecon++
		case domain.ToxicologyStatusClearedDecontaminatedPTGlucose:
			clearedSafe++
		}
	}
	return total, criticalPoison, stagedDecon, clearedSafe
}

func (s *Server) handleToxicologyPoisonTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tpStore := s.ensureToxicologyPoisonStore()
	ops := tpStore.List()
	total, criticalPoison, stagedDecon, clearedSafe := computeToxicologyPoisonSummary(ops)

	var activeOp *domain.ToxicologyPoisonOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := ToxicologyPoisonPageData{
		Operations:          ops,
		ActiveOperation:     activeOp,
		TotalOperations:     total,
		CriticalPoisonCount: criticalPoison,
		StagedDeconCount:    stagedDecon,
		ClearedSafeCount:    clearedSafe,
		Locale:              locale,
	}

	s.renderCockpitTemplate(w, r, "toxicology_poison_triage.html", "Anticoagulant Rodenticide, Xylitol & Lipid Emulsion (ILE) Poison Control Triage", data)
}

func (s *Server) handleListToxicologyPoisonOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tpStore := s.ensureToxicologyPoisonStore()
	allOps := tpStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	agentFilter := strings.TrimSpace(r.URL.Query().Get("agent_type"))
	if agentFilter == "" {
		agentFilter = strings.TrimSpace(r.URL.Query().Get("agent"))
	}

	filtered := make([]domain.ToxicologyPoisonOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if agentFilter != "" && !strings.EqualFold(string(op.AgentType), agentFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalPoison, stagedDecon, clearedSafe := computeToxicologyPoisonSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":      total,
			"critical_poison_count": criticalPoison,
			"staged_decon":          stagedDecon,
			"cleared_safe":          clearedSafe,
		},
	})
}

func (s *Server) handleEvaluateToxicologyPoisonOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.ToxicologyEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	tpStore := s.ensureToxicologyPoisonStore()
	created, err := tpStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastToxicologyPoisonEvent("toxicology_poison_updated", map[string]any{
		"type":      "toxicology_poison_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateToxicologyPoisonStep(w http.ResponseWriter, r *http.Request) {
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
	var payload toxicologyStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	tpStore := s.ensureToxicologyPoisonStore()
	if _, exists := tpStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Toxicology poison triage operation not found")
		return
	}

	updated, err := tpStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastToxicologyPoisonEvent("toxicology_poison_updated", map[string]any{
		"type":      "toxicology_poison_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueToxicologyPoisonClearance(w http.ResponseWriter, r *http.Request) {
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

	tpStore := s.ensureToxicologyPoisonStore()
	if _, exists := tpStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Toxicology poison triage operation not found")
		return
	}

	updated, err := tpStore.IssueClearance(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastToxicologyPoisonEvent("toxicology_poison_updated", map[string]any{
		"type":      "toxicology_poison_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleToxicologyPoisonStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "toxicology_poison")
}

func (s *Server) broadcastToxicologyPoisonEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("toxicology_poison", "toxicology-poison-hub", eventType, payload, now)
}
