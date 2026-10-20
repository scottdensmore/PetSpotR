package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/gdvbloat"
)

// GDVBloatPageData represents template data passed to templates/gdv_bloat_triage.html.
type GDVBloatPageData struct {
	Operations        []domain.GDVBloatOperation
	ActiveOperation   *domain.GDVBloatOperation
	TotalOperations   int
	CriticalGDVCount  int
	StagedTrocarCount int
	ClearedSafeCount  int
	Locale            string
}

type gdvStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureGDVBloatStore() *gdvbloat.Store {
	if s.gdvBloatStore == nil {
		s.gdvBloatStore = gdvbloat.NewStore()
	}
	return s.gdvBloatStore
}

func (s *Server) registerGDVBloatRoutes() {
	s.mux.HandleFunc("GET /gdv-bloat-triage", s.handleGDVBloatTriagePage)
	s.mux.HandleFunc("GET /api/v1/gdv-bloat/operations", s.handleListGDVBloatOperations)
	s.mux.HandleFunc("POST /api/v1/gdv-bloat/evaluate", s.handleEvaluateGDVBloatOperation)
	s.mux.HandleFunc("POST /api/v1/gdv-bloat/operations/{id}/steps", s.handleUpdateGDVBloatStep)
	s.mux.HandleFunc("POST /api/v1/gdv-bloat/operations/{id}/step", s.handleUpdateGDVBloatStep)
	s.mux.HandleFunc("POST /api/v1/gdv-bloat/operations/{id}/clearance", s.handleIssueGDVBloatClearance)
	s.mux.HandleFunc("GET /api/v1/gdv-bloat/stream", s.handleGDVBloatStream)
}

func computeGDVBloatSummary(ops []domain.GDVBloatOperation) (total, criticalGDV, stagedTrocar, clearedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.GDVStatusCriticalNecrosisVPCShock:
			criticalGDV++
		case domain.GDVStatusStagedTrocarDualIV:
			stagedTrocar++
		case domain.GDVStatusClearedGastropexySinus:
			clearedSafe++
		}
	}
	return total, criticalGDV, stagedTrocar, clearedSafe
}

func (s *Server) handleGDVBloatTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gdvStore := s.ensureGDVBloatStore()
	ops := gdvStore.List()
	total, criticalGDV, stagedTrocar, clearedSafe := computeGDVBloatSummary(ops)

	var activeOp *domain.GDVBloatOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := GDVBloatPageData{
		Operations:        ops,
		ActiveOperation:   activeOp,
		TotalOperations:   total,
		CriticalGDVCount:  criticalGDV,
		StagedTrocarCount: stagedTrocar,
		ClearedSafeCount:  clearedSafe,
		Locale:            locale,
	}

	s.renderCockpitTemplate(w, r, "gdv_bloat_triage.html", "GDV / Bloat, Splenic Torsion & Trocar Decompression Triage", data)
}

func (s *Server) handleListGDVBloatOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	gdvStore := s.ensureGDVBloatStore()
	allOps := gdvStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	presFilter := strings.TrimSpace(r.URL.Query().Get("presentation_type"))
	if presFilter == "" {
		presFilter = strings.TrimSpace(r.URL.Query().Get("presentation"))
	}

	filtered := make([]domain.GDVBloatOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if presFilter != "" && !strings.EqualFold(string(op.PresentationType), presFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalGDV, stagedTrocar, clearedSafe := computeGDVBloatSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":   total,
			"critical_gdv_count": criticalGDV,
			"staged_trocar":      stagedTrocar,
			"cleared_safe":       clearedSafe,
		},
	})
}

func (s *Server) handleEvaluateGDVBloatOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.GDVEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	gdvStore := s.ensureGDVBloatStore()
	created, err := gdvStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastGDVBloatEvent("gdv_bloat_updated", map[string]any{
		"type":      "gdv_bloat_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateGDVBloatStep(w http.ResponseWriter, r *http.Request) {
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
	var payload gdvStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	gdvStore := s.ensureGDVBloatStore()
	if _, exists := gdvStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "GDV bloat triage operation not found")
		return
	}

	updated, err := gdvStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastGDVBloatEvent("gdv_bloat_updated", map[string]any{
		"type":      "gdv_bloat_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueGDVBloatClearance(w http.ResponseWriter, r *http.Request) {
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

	gdvStore := s.ensureGDVBloatStore()
	if _, exists := gdvStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "GDV bloat triage operation not found")
		return
	}

	updated, err := gdvStore.IssueClearance(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastGDVBloatEvent("gdv_bloat_updated", map[string]any{
		"type":      "gdv_bloat_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleGDVBloatStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "gdv_bloat")
}

func (s *Server) broadcastGDVBloatEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("gdv_bloat", "gdv-bloat-hub", eventType, payload, now)
}
