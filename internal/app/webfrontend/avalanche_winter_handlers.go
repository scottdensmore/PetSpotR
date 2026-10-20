package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/avalanchewinter"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// WinterRescuePageData represents template data passed to templates/avalanche_winter_rescue.html.
type WinterRescuePageData struct {
	Operations            []domain.WinterRescueOperation
	TotalOperations       int
	CriticalBurialAlerts  int
	StagedRewarmBivies    int
	NormothermicSafeCount int
	Locale                string
}

type winterRescueClearanceRequest struct {
	LeadSkiPatrolCallsign string `json:"leadSkiPatrolCallsign"`
	Notes                 string `json:"notes"`
}

func (s *Server) ensureWinterRescueStore() *avalanchewinter.Store {
	if s.winterRescueStore == nil {
		s.winterRescueStore = avalanchewinter.NewStore()
	}
	return s.winterRescueStore
}

func (s *Server) registerWinterRescueRoutes() {
	s.mux.HandleFunc("GET /avalanche-winter-rescue", s.handleAvalancheWinterRescuePage)
	s.mux.HandleFunc("GET /api/v1/winter-rescue/operations", s.handleListWinterRescueOperations)
	s.mux.HandleFunc("POST /api/v1/winter-rescue/evaluate", s.handleEvaluateWinterRescueOperation)
	s.mux.HandleFunc("POST /api/v1/winter-rescue/operations/{id}/step", s.handleUpdateWinterRescueStep)
	s.mux.HandleFunc("POST /api/v1/winter-rescue/operations/{id}/clearance", s.handleIssueWinterRescueClearance)
	s.mux.HandleFunc("GET /api/v1/winter-rescue/stream", s.handleWinterRescueStream)
}

func computeWinterRescueSummary(ops []domain.WinterRescueOperation) (total, criticalBurial, stagedBivies, normothermicSafe int) {
	total = len(ops)
	for _, op := range ops {
		if op.AsphyxiaAirPocketAlert || op.SevereFrostbiteAfterdropAlert || op.Status == domain.WinterRescueStatusBurialHypothermiaCritical {
			criticalBurial++
		}
		if op.Status == domain.WinterRescueStatusProbeTrenchRewarmingStaged {
			stagedBivies++
		}
		if op.Status == domain.WinterRescueStatusExtractedNormothermicSafe {
			normothermicSafe++
		}
	}
	return total, criticalBurial, stagedBivies, normothermicSafe
}

func (s *Server) handleAvalancheWinterRescuePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	awStore := s.ensureWinterRescueStore()
	ops := awStore.List()
	total, criticalBurial, stagedBivies, normothermicSafe := computeWinterRescueSummary(ops)

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := WinterRescuePageData{
		Operations:            ops,
		TotalOperations:       total,
		CriticalBurialAlerts:  criticalBurial,
		StagedRewarmBivies:    stagedBivies,
		NormothermicSafeCount: normothermicSafe,
		Locale:                locale,
	}

	s.renderCockpitTemplate(w, r, "avalanche_winter_rescue.html", "Avalanche & Winter Rescue", data)
}

func (s *Server) handleListWinterRescueOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	awStore := s.ensureWinterRescueStore()
	allOps := awStore.List()

	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	terrainFilter := strings.TrimSpace(r.URL.Query().Get("terrainType"))

	filtered := make([]domain.WinterRescueOperation, 0, len(allOps))
	for _, op := range allOps {
		if statusFilter != "" && !strings.EqualFold(string(op.Status), statusFilter) {
			continue
		}
		if terrainFilter != "" && !strings.EqualFold(string(op.TerrainType), terrainFilter) {
			continue
		}
		filtered = append(filtered, op)
	}

	total, criticalBurial, stagedBivies, normothermicSafe := computeWinterRescueSummary(filtered)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": filtered,
		"ops":        filtered,
		"summary": map[string]int{
			"totalOperations":       total,
			"criticalBurialAlerts":  criticalBurial,
			"stagedRewarmBivies":    stagedBivies,
			"normothermicSafeCount": normothermicSafe,
		},
	})
}

func (s *Server) handleEvaluateWinterRescueOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req domain.WinterEvaluationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	awStore := s.ensureWinterRescueStore()
	created, err := awStore.EvaluateAndSave(req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastWinterRescueEvent("winter_rescue_updated", map[string]any{
		"type":      "winter_rescue_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleUpdateWinterRescueStep(w http.ResponseWriter, r *http.Request) {
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
	var req domain.WinterStepUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	awStore := s.ensureWinterRescueStore()
	if _, exists := awStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Winter rescue operation not found")
		return
	}

	updated, err := awStore.UpdateStep(opID, req)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastWinterRescueEvent("winter_rescue_updated", map[string]any{
		"type":      "winter_rescue_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueWinterRescueClearance(w http.ResponseWriter, r *http.Request) {
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
	var req winterRescueClearanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	awStore := s.ensureWinterRescueStore()
	if _, exists := awStore.Get(opID); !exists {
		writeCockpitJSONError(w, http.StatusNotFound, "Winter rescue operation not found")
		return
	}

	updated, err := awStore.IssueClearance(opID, req.LeadSkiPatrolCallsign, req.Notes)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastWinterRescueEvent("winter_rescue_updated", map[string]any{
		"type":      "winter_rescue_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleWinterRescueStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "winter_rescue")
}

func (s *Server) broadcastWinterRescueEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("winter_rescue", "winter-rescue-hub", eventType, payload, now)
}
