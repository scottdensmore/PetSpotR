package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pancreatitiscpl"
)

// PancreatitisCPLPageData represents template data passed to templates/pancreatitis_cpl_triage.html.
type PancreatitisCPLPageData struct {
	Operations      []domain.PancreatitisOperation
	ActiveOperation *domain.PancreatitisOperation
	Metrics         domain.PancreatitisOverviewMetrics
	Locale          string
}

type createPancreatitisOperationPayload struct {
	CaseID                       string                           `json:"case_id"`
	PetName                      string                           `json:"pet_name"`
	Species                      string                           `json:"species"`
	Breed                        string                           `json:"breed"`
	WeightKg                     float64                          `json:"weight_kg"`
	SeverityClass                domain.PancreatitisSeverityClass `json:"severity_class"`
	SpecPLUgL                    int                              `json:"spec_pl_ug_l"`
	ModifiedGlasgowPainScore     int                              `json:"modified_glasgow_pain_score"`
	EnteralRERPercent            int                              `json:"enteral_rer_percent"`
	POCUSHypoechoicPancreas      bool                             `json:"pocus_hypoechoic_pancreas"`
	HyperechoicPeripancreaticFat bool                             `json:"hyperechoic_peripancreatic_fat"`
	PrayerPositionPresent        bool                             `json:"prayer_position_present"`
	FelineTriaditisSuspected     bool                             `json:"feline_triaditis_suspected"`
}

type stagePancreatitisNutritionPayload struct {
	FuzapladibProtocol     string `json:"fuzapladib_protocol"`
	AntiemeticDualProtocol string `json:"antiemetic_dual_protocol"`
	VisceralAnalgesiaCRI   string `json:"visceral_analgesia_cri"`
	EarlyEnteralTubePlan   string `json:"early_enteral_tube_plan"`
	RERCaloricRampSchedule string `json:"rer_caloric_ramp_schedule"`
	FluidCrystalloidPlan   string `json:"fluid_crystalloid_plan"`
}

type recordPancreatitisCheckPayload struct {
	SpecPLUgL                int    `json:"spec_pl_ug_l"`
	ModifiedGlasgowPainScore int    `json:"modified_glasgow_pain_score"`
	EnteralRERPercent        int    `json:"enteral_rer_percent"`
	VomitingRegurgStatus     string `json:"vomiting_regurg_status"`
	RecordedBy               string `json:"recorded_by"`
	ClinicalNote             string `json:"clinical_note"`
}

type clearPancreatitisDischargePayload struct {
	DischargedBy string `json:"discharged_by"`
	Summary      string `json:"summary"`
}

func (s *Server) ensurePancreatitisService() *pancreatitiscpl.PancreatitisService {
	if s.pancreatitisService == nil {
		s.pancreatitisService = pancreatitiscpl.NewPancreatitisService()
	}
	return s.pancreatitisService
}

func (s *Server) registerPancreatitisCPLRoutes() {
	s.mux.HandleFunc("GET /pancreatitis-cpl-triage", s.handlePancreatitisCPLTriagePage)

	s.registerDualAPIRoute("GET", "pancreatitis-cpl/operations", s.handleListPancreatitisCPLOperations)
	s.registerDualAPIRoute("POST", "pancreatitis-cpl/operations", s.handleCreatePancreatitisCPLOperation)
	s.registerDualAPIRoute("GET", "pancreatitis-cpl/operations/{id}", s.handleGetPancreatitisCPLOperation)
	s.registerDualAPIRoute("POST", "pancreatitis-cpl/operations/{id}/stage-nutrition", s.handleStagePancreatitisCPLNutrition)
	s.registerDualAPIRoute("POST", "pancreatitis-cpl/operations/{id}/record-check", s.handleRecordPancreatitisCPLCheck)
	s.registerDualAPIRoute("POST", "pancreatitis-cpl/operations/{id}/clear-discharge", s.handleClearPancreatitisCPLDischarge)
	s.registerDualAPIRoute("GET", "pancreatitis-cpl/stream", s.handlePancreatitisCPLStream)
}

func (s *Server) handlePancreatitisCPLTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePancreatitisService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.PancreatitisOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := PancreatitisCPLPageData{
		Operations:      ops,
		ActiveOperation: activeOp,
		Metrics:         metrics,
		Locale:          locale,
	}

	s.renderCockpitTemplate(w, r, "pancreatitis_cpl_triage.html", "Pancreatitis cPL/fPL Triage", data)
}

func (s *Server) handleListPancreatitisCPLOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePancreatitisService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	ops := svc.ListOperations(statusFilter)
	metrics := svc.GetOverviewMetrics()

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
	})
}

func (s *Server) handleGetPancreatitisCPLOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePancreatitisService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreatePancreatitisCPLOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createPancreatitisOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensurePancreatitisService()
	created, err := svc.CreateOperation(
		payload.CaseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.SeverityClass,
		payload.SpecPLUgL,
		payload.ModifiedGlasgowPainScore,
		payload.EnteralRERPercent,
		payload.POCUSHypoechoicPancreas,
		payload.HyperechoicPeripancreaticFat,
		payload.PrayerPositionPresent,
		payload.FelineTriaditisSuspected,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPancreatitisCPLEvent("pancreatitis_cpl_updated", map[string]any{
		"type":      "pancreatitis_cpl_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStagePancreatitisCPLNutrition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePancreatitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stagePancreatitisNutritionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.StageEnteralNutrition(
		id,
		payload.FuzapladibProtocol,
		payload.AntiemeticDualProtocol,
		payload.VisceralAnalgesiaCRI,
		payload.EarlyEnteralTubePlan,
		payload.RERCaloricRampSchedule,
		payload.FluidCrystalloidPlan,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPancreatitisCPLEvent("pancreatitis_cpl_updated", map[string]any{
		"type":      "pancreatitis_cpl_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordPancreatitisCPLCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePancreatitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordPancreatitisCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordEnteralCheck(
		id,
		payload.SpecPLUgL,
		payload.ModifiedGlasgowPainScore,
		payload.EnteralRERPercent,
		payload.VomitingRegurgStatus,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPancreatitisCPLEvent("pancreatitis_cpl_updated", map[string]any{
		"type":      "pancreatitis_cpl_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearPancreatitisCPLDischarge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePancreatitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearPancreatitisDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.ClearPancreatitisDischarge(
		id,
		payload.DischargedBy,
		payload.Summary,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPancreatitisCPLEvent("pancreatitis_cpl_updated", map[string]any{
		"type":      "pancreatitis_cpl_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handlePancreatitisCPLStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "pancreatitis_cpl")
}

func (s *Server) broadcastPancreatitisCPLEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("pancreatitis_cpl", "pancreatitis-cpl-hub", eventType, payload, now)
}
