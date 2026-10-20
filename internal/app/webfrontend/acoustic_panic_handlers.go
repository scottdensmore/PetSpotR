package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/acousticpanic"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// AcousticPanicPageData represents template data passed to templates/acoustic_panic_trajectory.html.
type AcousticPanicPageData struct {
	Operations            []domain.AcousticPanicOperation
	ActiveOperation       *domain.AcousticPanicOperation
	TotalOperations       int
	CriticalBoltCount     int
	StagedLureCount       int
	DecompressedSafeCount int
	Locale                string
}

type acousticStepPayload struct {
	StepID      string `json:"step_id"`
	StepIDCamel string `json:"stepId"`
	Completed   bool   `json:"completed"`
}

func (s *Server) ensureAcousticPanicStore() *acousticpanic.Store {
	if s.acousticPanicStore == nil {
		s.acousticPanicStore = acousticpanic.NewStore()
	}
	return s.acousticPanicStore
}

func (s *Server) registerAcousticPanicRoutes() {
	s.mux.HandleFunc("GET /acoustic-panic-trajectory", s.handleAcousticPanicTrajectoryPage)
	s.mux.HandleFunc("GET /api/v1/acoustic-panic/operations", s.handleListAcousticPanicOperations)
	s.mux.HandleFunc("POST /api/v1/acoustic-panic/evaluate", s.handleEvaluateAcousticPanicOperation)
	s.mux.HandleFunc("POST /api/v1/acoustic-panic/operations/{id}/steps", s.handleUpdateAcousticPanicStep)
	s.mux.HandleFunc("POST /api/v1/acoustic-panic/operations/{id}/step", s.handleUpdateAcousticPanicStep)
	s.mux.HandleFunc("POST /api/v1/acoustic-panic/operations/{id}/clearance", s.handleIssueAcousticPanicClearance)
	s.mux.HandleFunc("GET /api/v1/acoustic-panic/stream", s.handleAcousticPanicStream)
}

func computeAcousticPanicSummary(ops []domain.AcousticPanicOperation) (total, criticalBolt, stagedLure, decompressedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.AcousticStatusCriticalBolt:
			criticalBolt++
		case domain.AcousticStatusStagedSniff:
			stagedLure++
		case domain.AcousticStatusDecompressedSafe:
			decompressedSafe++
		}
	}
	return total, criticalBolt, stagedLure, decompressedSafe
}

func (s *Server) handleAcousticPanicTrajectoryPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apStore := s.ensureAcousticPanicStore()
	ops := apStore.List()
	total, criticalBolt, stagedLure, decompressedSafe := computeAcousticPanicSummary(ops)

	var activeOp *domain.AcousticPanicOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := AcousticPanicPageData{
		Operations:            ops,
		ActiveOperation:       activeOp,
		TotalOperations:       total,
		CriticalBoltCount:     criticalBolt,
		StagedLureCount:       stagedLure,
		DecompressedSafeCount: decompressedSafe,
		Locale:                locale,
	}

	s.renderCockpitTemplate(w, r, "acoustic_panic_trajectory.html", "Acoustic Panic & Bolt-Trajectory SAR", data)
}

func (s *Server) handleListAcousticPanicOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	apStore := s.ensureAcousticPanicStore()
	allOps := apStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	triggerFilter := strings.TrimSpace(r.URL.Query().Get("trigger_type"))
	if triggerFilter == "" {
		triggerFilter = strings.TrimSpace(r.URL.Query().Get("trigger"))
	}

	filtered := make([]domain.AcousticPanicOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if triggerFilter != "" && !strings.EqualFold(string(op.TriggerType), triggerFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalBolt, stagedLure, decompressedSafe := computeAcousticPanicSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"total_operations":    total,
			"critical_bolt_count": criticalBolt,
			"staged_lure_count":   stagedLure,
			"decompressed_safe":   decompressedSafe,
		},
	})
}

func (s *Server) handleEvaluateAcousticPanicOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.AcousticEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	apStore := s.ensureAcousticPanicStore()
	created, err := apStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAcousticPanicEvent("acoustic_panic_updated", map[string]any{
		"type":      "acoustic_panic_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateAcousticPanicStep(w http.ResponseWriter, r *http.Request) {
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
	var payload acousticStepPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	stepID := strings.TrimSpace(payload.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(payload.StepIDCamel)
	}

	apStore := s.ensureAcousticPanicStore()
	if _, exists := apStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Acoustic panic operation not found")
		return
	}

	updated, err := apStore.UpdateStep(opID, stepID, payload.Completed)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAcousticPanicEvent("acoustic_panic_updated", map[string]any{
		"type":      "acoustic_panic_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueAcousticPanicClearance(w http.ResponseWriter, r *http.Request) {
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

	apStore := s.ensureAcousticPanicStore()
	if _, exists := apStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Acoustic panic operation not found")
		return
	}

	updated, err := apStore.IssueClearance(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAcousticPanicEvent("acoustic_panic_updated", map[string]any{
		"type":      "acoustic_panic_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAcousticPanicStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "acoustic_panic")
}

func (s *Server) broadcastAcousticPanicEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("acoustic_panic", "acoustic-panic-hub", eventType, payload, now)
}
