package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/diccoagulopathy"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// DICCoagulopathyPageData represents template data passed to templates/dic_coagulopathy_triage.html.
type DICCoagulopathyPageData struct {
	Operations                      []domain.DICCoagulopathyOperation
	ActiveOperation                 *domain.DICCoagulopathyOperation
	Metrics                         domain.DICOverviewMetrics
	TotalOperations                 int
	OvertDICTriageCount             int
	FFPTXAStagedCount               int
	ClearedNormocoagulableCount     int
	ConsumptiveHemorrhagicCount     int
	HyperfibrinolysisGreyhoundCount int
	Locale                          string
}

type createDICOperationPayload struct {
	CaseID                     string                   `json:"case_id"`
	PetName                    string                   `json:"pet_name"`
	Species                    string                   `json:"species"`
	Breed                      string                   `json:"breed"`
	WeightKg                   float64                  `json:"weight_kg"`
	PhasePhenotype             domain.DICPhasePhenotype `json:"phase_phenotype"`
	PlateletCountKUl           float64                  `json:"platelet_count_k_ul"`
	PTSeconds                  float64                  `json:"pt_seconds"`
	APTTSeconds                float64                  `json:"aptt_seconds"`
	DDimerNgML                 float64                  `json:"d_dimer_ng_ml"`
	FibrinogenMgDL             float64                  `json:"fibrinogen_mg_dl"`
	AntithrombinPercent        float64                  `json:"antithrombin_percent"`
	SchistocytesPresent        bool                     `json:"schistocytes_present"`
	SpontaneousMucosalBleeding bool                     `json:"spontaneous_mucosal_bleeding"`
}

type stageDICComponentTherapyPayload struct {
	FreshFrozenPlasmaProtocol      string `json:"fresh_frozen_plasma_protocol"`
	FFPProtocol                    string `json:"ffp_protocol"`
	CryoprecipitatevWFProtocol     string `json:"cryoprecipitate_vwf_protocol"`
	CryoprecipitateProtocol        string `json:"cryoprecipitate_protocol"`
	TranexamicAcidAminocaproicPlan string `json:"tranexamic_acid_aminocaproic_plan"`
	TXAPlan                        string `json:"txa_plan"`
	AntithrombinHeparinPhasePlan   string `json:"antithrombin_heparin_phase_plan"`
	HeparinPhasePlan               string `json:"heparin_phase_plan"`
	UnderlyingTriggerControlPlan   string `json:"underlying_trigger_control_plan"`
	TriggerControlPlan             string `json:"trigger_control_plan"`
	SerialCoagPanelSchedule        string `json:"serial_coag_panel_schedule"`
	CoagSchedule                   string `json:"coag_schedule"`
}

type recordDICCoagCheckPayload struct {
	PlateletCountKUl float64 `json:"platelet_count_k_ul"`
	PTSeconds        float64 `json:"pt_seconds"`
	APTTSeconds      float64 `json:"aptt_seconds"`
	DDimerNgML       float64 `json:"d_dimer_ng_ml"`
	FibrinogenMgDL   float64 `json:"fibrinogen_mg_dl"`
	RecordedBy       string  `json:"recorded_by"`
	ClinicalNote     string  `json:"clinical_note"`
}

type clearDICDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureDICService() *diccoagulopathy.DICService {
	if s.dicService == nil {
		s.dicService = diccoagulopathy.NewDICService()
	}
	return s.dicService
}

func (s *Server) registerDICCoagulopathyRoutes() {
	s.mux.HandleFunc("GET /dic-coagulopathy-triage", s.handleDICCoagulopathyTriagePage)

	s.registerDualAPIRoute("GET", "dic-coagulopathy/operations", s.handleListDICCoagulopathyOperations)
	s.registerDualAPIRoute("POST", "dic-coagulopathy/operations", s.handleCreateDICCoagulopathyOperation)
	s.registerDualAPIRoute("GET", "dic-coagulopathy/operations/{id}", s.handleGetDICCoagulopathyOperation)
	s.registerDualAPIRoute("POST", "dic-coagulopathy/operations/{id}/stage-therapy", s.handleStageDICComponentTherapy)
	s.registerDualAPIRoute("POST", "dic-coagulopathy/operations/{id}/record-check", s.handleRecordDICCoagCheck)
	s.registerDualAPIRoute("POST", "dic-coagulopathy/operations/{id}/clear-discharge", s.handleClearDICDischarge)
	s.registerDualAPIRoute("GET", "dic-coagulopathy/stream", s.handleDICCoagulopathyStream)
}

func (s *Server) handleDICCoagulopathyTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureDICService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.DICCoagulopathyOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := DICCoagulopathyPageData{
		Operations:                      ops,
		ActiveOperation:                 activeOp,
		Metrics:                         metrics,
		TotalOperations:                 metrics.TotalOperations,
		OvertDICTriageCount:             metrics.OvertDICTriageCount,
		FFPTXAStagedCount:               metrics.FFPTXAStagedCount,
		ClearedNormocoagulableCount:     metrics.ClearedNormocoagulableCount,
		ConsumptiveHemorrhagicCount:     metrics.ConsumptiveHemorrhagicCount,
		HyperfibrinolysisGreyhoundCount: metrics.HyperfibrinolysisGreyhoundCount,
		Locale:                          locale,
	}

	s.renderCockpitTemplate(w, r, "dic_coagulopathy_triage.html", "DIC Coagulopathy Triage", data)
}

func (s *Server) handleListDICCoagulopathyOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureDICService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	phenotypeFilter := strings.TrimSpace(r.URL.Query().Get("phenotype"))

	ops := svc.ListOperations(statusFilter)
	if phenotypeFilter != "" {
		filtered := make([]domain.DICCoagulopathyOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.PhasePhenotype), phenotypeFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	metrics := svc.GetOverviewMetrics()

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
	})
}

func (s *Server) handleGetDICCoagulopathyOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureDICService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateDICCoagulopathyOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createDICOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureDICService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-DIC-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.PhasePhenotype,
		payload.PlateletCountKUl,
		payload.PTSeconds,
		payload.APTTSeconds,
		payload.DDimerNgML,
		payload.FibrinogenMgDL,
		payload.AntithrombinPercent,
		payload.SchistocytesPresent,
		payload.SpontaneousMucosalBleeding,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastDICCoagulopathyEvent("dic_coagulopathy_updated", map[string]any{
		"type":      "dic_coagulopathy_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageDICComponentTherapy(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureDICService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageDICComponentTherapyPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	ffp := firstNonEmpty(payload.FreshFrozenPlasmaProtocol, payload.FFPProtocol, existing.ComponentPlan.FreshFrozenPlasmaProtocol)
	cryo := firstNonEmpty(payload.CryoprecipitatevWFProtocol, payload.CryoprecipitateProtocol, existing.ComponentPlan.CryoprecipitatevWFProtocol)
	txa := firstNonEmpty(payload.TranexamicAcidAminocaproicPlan, payload.TXAPlan, existing.ComponentPlan.TranexamicAcidAminocaproicPlan)
	heparin := firstNonEmpty(payload.AntithrombinHeparinPhasePlan, payload.HeparinPhasePlan, existing.ComponentPlan.AntithrombinHeparinPhasePlan)
	trigger := firstNonEmpty(payload.UnderlyingTriggerControlPlan, payload.TriggerControlPlan, existing.ComponentPlan.UnderlyingTriggerControlPlan)
	schedule := firstNonEmpty(payload.SerialCoagPanelSchedule, payload.CoagSchedule, existing.ComponentPlan.SerialCoagPanelSchedule)

	updated, err := svc.StageComponentTherapy(id, ffp, cryo, txa, heparin, trigger, schedule)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastDICCoagulopathyEvent("dic_coagulopathy_updated", map[string]any{
		"type":      "dic_coagulopathy_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordDICCoagCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureDICService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordDICCoagCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordCoagCheck(
		id,
		payload.PlateletCountKUl,
		payload.PTSeconds,
		payload.APTTSeconds,
		payload.DDimerNgML,
		payload.FibrinogenMgDL,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastDICCoagulopathyEvent("dic_coagulopathy_updated", map[string]any{
		"type":      "dic_coagulopathy_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearDICDischarge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureDICService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearDICDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearDICDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastDICCoagulopathyEvent("dic_coagulopathy_updated", map[string]any{
		"type":      "dic_coagulopathy_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDICCoagulopathyStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "dic_coagulopathy")
}

func (s *Server) broadcastDICCoagulopathyEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("dic_coagulopathy", "dic-coagulopathy-hub", eventType, payload, now)
}
