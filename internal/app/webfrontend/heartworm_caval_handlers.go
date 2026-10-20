package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/heartwormcaval"
)

// HeartwormCavalPageData represents template data passed to templates/heartworm_caval_triage.html.
type HeartwormCavalPageData struct {
	Operations                             []domain.HeartwormCavalOperation
	ActiveOperation                        *domain.HeartwormCavalOperation
	Metrics                                domain.HeartwormOverview
	TotalOperations                        int
	JugularVenotomyWolbachiaStagedCount    int
	PulmonaryHypertensionPTEMonitoredCount int
	CavalPatentClearedCount                int
	ClassIVCavalSyndromeCount              int
	SeverePulmonaryHypertensionCount       int
	ActiveHemoglobinuriaCount              int
	Locale                                 string
}

type createHeartwormCavalOperationPayload struct {
	CaseID                        string                   `json:"case_id"`
	PetName                       string                   `json:"pet_name"`
	Species                       string                   `json:"species"`
	Breed                         string                   `json:"breed"`
	WeightKg                      float64                  `json:"weight_kg"`
	SeverityClass                 domain.HeartwormClass    `json:"severity_class"`
	Protocol                      domain.HeartwormProtocol `json:"protocol"`
	TRJetVelocityMS               float64                  `json:"tr_jet_velocity_ms"`
	WormsExtractedCount           int                      `json:"worms_extracted_count"`
	PCVPercent                    float64                  `json:"pcv_percent"`
	FreeHemoglobinuriaPresent     bool                     `json:"free_hemoglobinuria_present"`
	ParallelHyperechoicCavalLines bool                     `json:"parallel_hyperechoic_caval_lines"`
}

type stageHeartwormInterventionPayload struct {
	Protocol                            domain.HeartwormProtocol `json:"protocol"`
	RightJugularVenotomyExtractionPlan  string                   `json:"right_jugular_venotomy_extraction_plan"`
	JugularExtractionPlan               string                   `json:"jugular_extraction_plan"`
	ImmediateMelarsomineContraindicated *bool                    `json:"immediate_melarsomine_contraindicated"`
	WolbachiaDoxycyclineMgKgDay         float64                  `json:"wolbachia_doxycycline_mg_kg_day"`
	WolbachiaDoxycycline28DayPlan       string                   `json:"wolbachia_doxycycline_28day_plan"`
	Doxycycline28DayPlan                string                   `json:"doxycycline_28day_plan"`
	MacrocyclicLactonePretreatPlan      string                   `json:"macrocyclic_lactone_pretreat_plan"`
	LactonePretreatPlan                 string                   `json:"lactone_pretreat_plan"`
	PrednisoneTaperMgKgDay              float64                  `json:"prednisone_taper_mg_kg_day"`
	PrednisoneAntiInflammatoryTaperPlan string                   `json:"prednisone_anti_inflammatory_taper_plan"`
	PrednisoneTaperPlan                 string                   `json:"prednisone_taper_plan"`
	SildenafilPulmonaryHypertensionPlan string                   `json:"sildenafil_pulmonary_hypertension_plan"`
	SildenafilPlan                      string                   `json:"sildenafil_plan"`
	SplitDoseMelarsomineScheduleNote    string                   `json:"split_dose_melarsomine_schedule_note"`
	SplitMelarsomineNote                string                   `json:"split_melarsomine_note"`
	StrictCageRestWeeks                 int                      `json:"strict_cage_rest_weeks"`
	StagedBy                            string                   `json:"staged_by"`
}

type recordHeartwormHemodynamicCheckPayload struct {
	TRJetVelocityMS               float64 `json:"tr_jet_velocity_ms"`
	WormsExtractedCount           int     `json:"worms_extracted_count"`
	PCVPercent                    float64 `json:"pcv_percent"`
	FreeHemoglobinuriaPresent     bool    `json:"free_hemoglobinuria_present"`
	ParallelHyperechoicCavalLines bool    `json:"parallel_hyperechoic_caval_lines"`
	SildenafilMgKgDose            float64 `json:"sildenafil_mg_kg_dose"`
	RecordedBy                    string  `json:"recorded_by"`
	ClinicalNote                  string  `json:"clinical_note"`
}

type evaluateHeartwormClearancePayload struct {
	ParallelHyperechoicCavalLines *bool    `json:"parallel_hyperechoic_caval_lines"`
	FreeHemoglobinuriaPresent     *bool    `json:"free_hemoglobinuria_present"`
	TRJetVelocityMS               *float64 `json:"tr_jet_velocity_ms"`
	StrictCageRestVerified        *bool    `json:"strict_cage_rest_verified"`
	ClearedBy                     string   `json:"cleared_by"`
	DischargedBy                  string   `json:"discharged_by"`
	ClearanceSummary              string   `json:"clearance_summary"`
	DischargeSummary              string   `json:"discharge_summary"`
	Summary                       string   `json:"summary"`
}

func (s *Server) ensureHeartwormService() *heartwormcaval.HeartwormService {
	if s.heartwormService == nil {
		s.heartwormService = heartwormcaval.NewHeartwormService()
	}
	return s.heartwormService
}

func (s *Server) registerHeartwormCavalRoutes() {
	s.mux.HandleFunc("GET /heartworm-caval-triage", s.handleHeartwormCavalTriagePage)

	s.registerDualAPIRoute("GET", "heartworm-caval/overview", s.handleHeartwormCavalOverview)
	s.registerDualAPIRoute("GET", "heartworm-caval/operations", s.handleListHeartwormCavalOperations)
	s.registerDualAPIRoute("POST", "heartworm-caval/operations", s.handleCreateHeartwormCavalOperation)
	s.registerDualAPIRoute("GET", "heartworm-caval/operations/{id}", s.handleGetHeartwormCavalOperation)
	s.registerDualAPIRoute("POST", "heartworm-caval/operations/{id}/intervention", s.handleStageHeartwormIntervention)
	s.registerDualAPIRoute("POST", "heartworm-caval/operations/{id}/hemodynamic-check", s.handleRecordHeartwormHemodynamicCheck)
	s.registerDualAPIRoute("POST", "heartworm-caval/operations/{id}/clearance", s.handleEvaluateHeartwormClearance)
	s.registerDualAPIRoute("GET", "heartworm-caval/stream", s.handleHeartwormCavalStream)
}

func (s *Server) handleHeartwormCavalTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHeartwormService()
	ops := svc.ListOperations("")
	metrics := svc.CalculateOverview()

	var activeOp *domain.HeartwormCavalOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := HeartwormCavalPageData{
		Operations:                             ops,
		ActiveOperation:                        activeOp,
		Metrics:                                metrics,
		TotalOperations:                        metrics.TotalOperations,
		JugularVenotomyWolbachiaStagedCount:    metrics.JugularVenotomyWolbachiaStagedCount,
		PulmonaryHypertensionPTEMonitoredCount: metrics.PulmonaryHypertensionPTEMonitoredCount,
		CavalPatentClearedCount:                metrics.CavalPatentClearedCount,
		ClassIVCavalSyndromeCount:              metrics.ClassIVCavalSyndromeCount,
		SeverePulmonaryHypertensionCount:       metrics.SeverePulmonaryHypertensionCount,
		ActiveHemoglobinuriaCount:              metrics.ActiveHemoglobinuriaCount,
		Locale:                                 locale,
	}

	s.renderCockpitTemplate(w, r, "heartworm_caval_triage.html", "Heartworm Caval Triage", data)
}

func (s *Server) handleHeartwormCavalOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHeartwormService()
	overview := svc.CalculateOverview()

	writeCockpitJSON(w, http.StatusOK, overview)
}

func (s *Server) handleListHeartwormCavalOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHeartwormService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	classFilter := strings.TrimSpace(r.URL.Query().Get("severity_class"))

	ops := svc.ListOperations(statusFilter)
	if classFilter != "" {
		filtered := make([]domain.HeartwormCavalOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.SeverityClass), classFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	metrics := svc.CalculateOverview()

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
		"overview":   metrics,
	})
}

func (s *Server) handleGetHeartwormCavalOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureHeartwormService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateHeartwormCavalOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createHeartwormCavalOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureHeartwormService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-HWC-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.SeverityClass,
		payload.Protocol,
		payload.TRJetVelocityMS,
		payload.WormsExtractedCount,
		payload.PCVPercent,
		payload.FreeHemoglobinuriaPresent,
		payload.ParallelHyperechoicCavalLines,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{
		"type":      "heartworm_caval_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageHeartwormIntervention(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureHeartwormService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageHeartwormInterventionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	protocol := payload.Protocol
	if protocol == "" {
		protocol = existing.Protocol
	}
	jugularPlan := firstNonEmpty(
		payload.RightJugularVenotomyExtractionPlan,
		payload.JugularExtractionPlan,
		existing.InterventionStaging.RightJugularVenotomyExtractionPlan,
	)
	immediateContraindicated := existing.ImmediateMelarsomineContraindicated
	if payload.ImmediateMelarsomineContraindicated != nil {
		immediateContraindicated = *payload.ImmediateMelarsomineContraindicated
	}
	doxyPlan := firstNonEmpty(
		payload.WolbachiaDoxycycline28DayPlan,
		payload.Doxycycline28DayPlan,
		existing.InterventionStaging.WolbachiaDoxycycline28DayPlan,
	)
	lactonePlan := firstNonEmpty(
		payload.MacrocyclicLactonePretreatPlan,
		payload.LactonePretreatPlan,
		existing.InterventionStaging.MacrocyclicLactonePretreatPlan,
	)
	predPlan := firstNonEmpty(
		payload.PrednisoneAntiInflammatoryTaperPlan,
		payload.PrednisoneTaperPlan,
		existing.InterventionStaging.PrednisoneAntiInflammatoryTaperPlan,
	)
	sildenafilPlan := firstNonEmpty(
		payload.SildenafilPulmonaryHypertensionPlan,
		payload.SildenafilPlan,
		existing.InterventionStaging.SildenafilPulmonaryHypertensionPlan,
	)
	melarsomineNote := firstNonEmpty(
		payload.SplitDoseMelarsomineScheduleNote,
		payload.SplitMelarsomineNote,
		existing.InterventionStaging.SplitDoseMelarsomineScheduleNote,
	)

	updated, err := svc.StageIntervention(
		id,
		protocol,
		jugularPlan,
		immediateContraindicated,
		payload.WolbachiaDoxycyclineMgKgDay,
		doxyPlan,
		lactonePlan,
		payload.PrednisoneTaperMgKgDay,
		predPlan,
		sildenafilPlan,
		melarsomineNote,
		payload.StrictCageRestWeeks,
		payload.StagedBy,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{
		"type":      "heartworm_caval_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordHeartwormHemodynamicCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureHeartwormService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordHeartwormHemodynamicCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordHemodynamicCheck(
		id,
		payload.TRJetVelocityMS,
		payload.WormsExtractedCount,
		payload.PCVPercent,
		payload.FreeHemoglobinuriaPresent,
		payload.ParallelHyperechoicCavalLines,
		payload.SildenafilMgKgDose,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{
		"type":      "heartworm_caval_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleEvaluateHeartwormClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureHeartwormService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload evaluateHeartwormClearancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	parallelLines := false
	if payload.ParallelHyperechoicCavalLines != nil {
		parallelLines = *payload.ParallelHyperechoicCavalLines
	}
	hemoglobinuria := false
	if payload.FreeHemoglobinuriaPresent != nil {
		hemoglobinuria = *payload.FreeHemoglobinuriaPresent
	}
	trJet := 2.2
	if payload.TRJetVelocityMS != nil {
		trJet = *payload.TRJetVelocityMS
	}
	cageRestVerified := true
	if payload.StrictCageRestVerified != nil {
		cageRestVerified = *payload.StrictCageRestVerified
	}
	clearedBy := firstNonEmpty(payload.ClearedBy, payload.DischargedBy)
	summary := firstNonEmpty(payload.ClearanceSummary, payload.DischargeSummary, payload.Summary)

	updated, err := svc.EvaluateClearance(
		id,
		parallelLines,
		hemoglobinuria,
		trJet,
		cageRestVerified,
		clearedBy,
		summary,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{
		"type":      "heartworm_caval_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleHeartwormCavalStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "heartworm_caval")
}

func (s *Server) broadcastHeartwormCavalEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("heartworm_caval", "heartworm-caval-hub", eventType, payload, now)
}
