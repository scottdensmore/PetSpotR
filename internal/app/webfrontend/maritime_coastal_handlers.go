package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/maritimecoastal"
)

// MaritimeRescuePageData represents template data passed to templates/maritime_coastal_rescue.html.
type MaritimeRescuePageData struct {
	Operations         []domain.MaritimeRescueOperation
	TotalOperations    int
	CriticalTideCount  int
	StagedVesselCount  int
	ExtractedSafeCount int
	Locale             string
}

func (s *Server) ensureMaritimeRescueStore() *maritimecoastal.Store {
	if s.maritimeRescueStore == nil {
		s.maritimeRescueStore = maritimecoastal.NewStore()
	}
	return s.maritimeRescueStore
}

func (s *Server) registerMaritimeCoastalRoutes() {
	s.mux.HandleFunc("GET /maritime-coastal-rescue", s.handleMaritimeCoastalRescuePage)
	s.mux.HandleFunc("GET /api/v1/maritime-rescue/operations", s.handleListMaritimeRescueOperations)
	s.mux.HandleFunc("POST /api/v1/maritime-rescue/evaluate", s.handleEvaluateMaritimeRescueOperation)
	s.mux.HandleFunc("POST /api/v1/maritime-rescue/operations/{id}/steps", s.handleUpdateMaritimeRescueStep)
	s.mux.HandleFunc("POST /api/v1/maritime-rescue/operations/{id}/step", s.handleUpdateMaritimeRescueStep)
	s.mux.HandleFunc("POST /api/v1/maritime-rescue/operations/{id}/clearance", s.handleIssueMaritimeRescueClearance)
	s.mux.HandleFunc("GET /api/v1/maritime-rescue/stream", s.handleMaritimeRescueStream)
}

func computeMaritimeRescueSummary(ops []domain.MaritimeRescueOperation) (total, criticalTide, stagedVessel, extractedSafe int) {
	total = len(ops)
	for _, op := range ops {
		switch op.Status {
		case domain.MaritimeStatusTidalFloodAspirationCritical:
			criticalTide++
		case domain.MaritimeStatusHovercraftMudSledStaged:
			stagedVessel++
		case domain.MaritimeStatusExtractedHarborDeckSafe:
			extractedSafe++
		}
	}
	return total, criticalTide, stagedVessel, extractedSafe
}

func (s *Server) handleMaritimeCoastalRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mcStore := s.ensureMaritimeRescueStore()
	ops := mcStore.List()
	total, criticalTide, stagedVessel, extractedSafe := computeMaritimeRescueSummary(ops)

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := MaritimeRescuePageData{
		Operations:         ops,
		TotalOperations:    total,
		CriticalTideCount:  criticalTide,
		StagedVesselCount:  stagedVessel,
		ExtractedSafeCount: extractedSafe,
		Locale:             locale,
	}

	s.renderCockpitTemplate(w, r, "maritime_coastal_rescue.html", "Maritime & Coastal Rescue", data)
}

func (s *Server) handleListMaritimeRescueOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mcStore := s.ensureMaritimeRescueStore()
	allOps := mcStore.List()

	zoneFilter := strings.TrimSpace(r.URL.Query().Get("zone"))
	if zoneFilter == "" {
		zoneFilter = strings.TrimSpace(r.URL.Query().Get("maritime_zone"))
	}
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	filtered := make([]domain.MaritimeRescueOperation, 0, len(allOps))
	for _, op := range allOps {
		if zoneFilter != "" && !strings.EqualFold(string(op.MaritimeZone), zoneFilter) {
			continue
		}
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalTide, stagedVessel, extractedSafe := computeMaritimeRescueSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations":     filtered,
		"total":          total,
		"critical_count": criticalTide,
		"staged_count":   stagedVessel,
		"safe_count":     extractedSafe,
		"summary": map[string]int{
			"total_operations": total,
			"critical_tide":    criticalTide,
			"staged_vessel":    stagedVessel,
			"extracted_safe":   extractedSafe,
		},
	})
}

func (s *Server) handleEvaluateMaritimeRescueOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.MaritimeEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	mcStore := s.ensureMaritimeRescueStore()
	created, err := mcStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMaritimeRescueEvent("maritime_rescue_updated", map[string]any{
		"type":      "maritime_rescue_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateMaritimeRescueStep(w http.ResponseWriter, r *http.Request) {
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
	var raw struct {
		StepID    string `json:"step_id"`
		StepIDAlt string `json:"stepId"`
		Completed bool   `json:"completed"`
	}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	stepID := strings.TrimSpace(raw.StepID)
	if stepID == "" {
		stepID = strings.TrimSpace(raw.StepIDAlt)
	}

	mcStore := s.ensureMaritimeRescueStore()
	if _, exists := mcStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Maritime rescue operation not found")
		return
	}

	updated, err := mcStore.UpdateStep(opID, domain.MaritimeStepUpdateRequest{
		StepID:    stepID,
		Completed: raw.Completed,
	})
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMaritimeRescueEvent("maritime_rescue_updated", map[string]any{
		"type":      "maritime_rescue_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueMaritimeRescueClearance(w http.ResponseWriter, r *http.Request) {
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

	mcStore := s.ensureMaritimeRescueStore()
	if _, exists := mcStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Maritime rescue operation not found")
		return
	}

	updated, err := mcStore.IssueClearance(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMaritimeRescueEvent("maritime_rescue_updated", map[string]any{
		"type":      "maritime_rescue_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleMaritimeRescueStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "maritime_rescue")
}

func (s *Server) broadcastMaritimeRescueEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("maritime_rescue", "maritime-rescue-hub", eventType, payload, now)
}
