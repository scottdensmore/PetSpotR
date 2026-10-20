package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ncpeelectrocution"
)

// NCPEElectrocutionPageData represents template data passed to templates/ncpe_electrocution_triage.html.
type NCPEElectrocutionPageData struct {
	Operations                  []domain.NCPEElectrocutionOperation
	ActiveOperation             *domain.NCPEElectrocutionOperation
	Metrics                     domain.NCPEOverviewMetrics
	TotalOperations             int
	AcuteNCPETriageCount        int
	CPAPOxygenStagedCount       int
	ClearedRoomAirCount         int
	OralCordElectrocutionCount  int
	SeverePaO2FiO2Under200Count int
	Locale                      string
}

type createNCPEElectrocutionOperationPayload struct {
	CaseID                      string                            `json:"case_id"`
	PetName                     string                            `json:"pet_name"`
	Species                     string                            `json:"species"`
	Breed                       string                            `json:"breed"`
	WeightKg                    float64                           `json:"weight_kg"`
	EtiologyClass               domain.NCPEEtiologyClassification `json:"etiology_class"`
	PaO2FiO2Ratio               float64                           `json:"pao2_fio2_ratio"`
	SpO2Percent                 float64                           `json:"spo2_percent"`
	RespiratoryRateBpm          int                               `json:"respiratory_rate_bpm"`
	CaudodorsalBLinesCount      int                               `json:"caudodorsal_blines_count"`
	LA2AoRatio                  float64                           `json:"la2ao_ratio"`
	OralElectricalEscharPresent bool                              `json:"oral_electrical_eschar_present"`
	PinkFrothyAirwayFluid       bool                              `json:"pink_frothy_airway_fluid"`
}

type stageNCPECPAPVentilationPayload struct {
	OxygenFiO2OrHFNCProtocol            string `json:"oxygen_fio2_or_hfnc_protocol"`
	OxygenProtocol                      string `json:"oxygen_protocol"`
	AnxiolyticButorphanolAcepromazine   string `json:"anxiolytic_butorphanol_acepromazine"`
	AnxiolyticPlan                      string `json:"anxiolytic_plan"`
	LowDoseFurosemideBronchodilatorNote string `json:"low_dose_furosemide_bronchodilator_note"`
	LowDoseFurosemideNote               string `json:"low_dose_furosemide_note"`
	CPAPOrPEEPMechanicalVentilation     string `json:"cpap_or_peep_mechanical_ventilation"`
	CPAPPeepPlan                        string `json:"cpap_peep_plan"`
	ConservativeFluidMaintenancePlan    string `json:"conservative_fluid_maintenance_plan"`
	ConservativeFluidPlan               string `json:"conservative_fluid_plan"`
	OralCommissureBurnAnalgesiaPlan     string `json:"oral_commissure_burn_analgesia_plan"`
	OralBurnAnalgesia                   string `json:"oral_burn_analgesia"`
}

type recordNCPERespiratoryCheckPayload struct {
	PaO2FiO2Ratio          float64 `json:"pao2_fio2_ratio"`
	SpO2Percent            float64 `json:"spo2_percent"`
	RespiratoryRateBpm     int     `json:"respiratory_rate_bpm"`
	CaudodorsalBLinesCount int     `json:"caudodorsal_blines_count"`
	LA2AoRatio             float64 `json:"la2ao_ratio"`
	RecordedBy             string  `json:"recorded_by"`
	ClinicalNote           string  `json:"clinical_note"`
}

type clearNCPEElectrocutionDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureNCPEService() *ncpeelectrocution.NCPEService {
	if s.ncpeService == nil {
		s.ncpeService = ncpeelectrocution.NewNCPEService()
	}
	return s.ncpeService
}

func (s *Server) registerNCPEElectrocutionRoutes() {
	s.mux.HandleFunc("GET /ncpe-electrocution-triage", s.handleNCPEElectrocutionTriagePage)

	s.registerDualAPIRoute("GET", "ncpe-electrocution/operations", s.handleListNCPEElectrocutionOperations)
	s.registerDualAPIRoute("POST", "ncpe-electrocution/operations", s.handleCreateNCPEElectrocutionOperation)
	s.registerDualAPIRoute("GET", "ncpe-electrocution/operations/{id}", s.handleGetNCPEElectrocutionOperation)
	s.registerDualAPIRoute("POST", "ncpe-electrocution/operations/{id}/stage-cpap", s.handleStageNCPECPAPVentilation)
	s.registerDualAPIRoute("POST", "ncpe-electrocution/operations/{id}/record-check", s.handleRecordNCPERespiratoryCheck)
	s.registerDualAPIRoute("POST", "ncpe-electrocution/operations/{id}/clear-discharge", s.handleClearNCPEElectrocutionDischarge)
	s.registerDualAPIRoute("GET", "ncpe-electrocution/stream", s.handleNCPEElectrocutionStream)
}

func (s *Server) handleNCPEElectrocutionTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureNCPEService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.NCPEElectrocutionOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := NCPEElectrocutionPageData{
		Operations:                  ops,
		ActiveOperation:             activeOp,
		Metrics:                     metrics,
		TotalOperations:             metrics.TotalOperations,
		AcuteNCPETriageCount:        metrics.AcuteNCPETriageCount,
		CPAPOxygenStagedCount:       metrics.CPAPOxygenStagedCount,
		ClearedRoomAirCount:         metrics.ClearedRoomAirCount,
		OralCordElectrocutionCount:  metrics.OralCordElectrocutionCount,
		SeverePaO2FiO2Under200Count: metrics.SeverePaO2FiO2Under200Count,
		Locale:                      locale,
	}

	s.renderCockpitTemplate(w, r, "ncpe_electrocution_triage.html", "NCPE Electrocution Triage", data)
}

func (s *Server) handleListNCPEElectrocutionOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureNCPEService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	etiologyFilter := strings.TrimSpace(r.URL.Query().Get("etiology_class"))

	ops := svc.ListOperations(statusFilter)
	if etiologyFilter != "" {
		filtered := make([]domain.NCPEElectrocutionOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.EtiologyClass), etiologyFilter) {
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

func (s *Server) handleGetNCPEElectrocutionOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureNCPEService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateNCPEElectrocutionOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createNCPEElectrocutionOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureNCPEService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-NCPE-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.EtiologyClass,
		payload.PaO2FiO2Ratio,
		payload.SpO2Percent,
		payload.RespiratoryRateBpm,
		payload.CaudodorsalBLinesCount,
		payload.LA2AoRatio,
		payload.OralElectricalEscharPresent,
		payload.PinkFrothyAirwayFluid,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastNCPEElectrocutionEvent("ncpe_electrocution_updated", map[string]any{
		"type":      "ncpe_electrocution_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageNCPECPAPVentilation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureNCPEService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageNCPECPAPVentilationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	oxygen := firstNonEmpty(payload.OxygenFiO2OrHFNCProtocol, payload.OxygenProtocol, existing.VentilationPlan.OxygenFiO2OrHFNCProtocol)
	anxiolytic := firstNonEmpty(payload.AnxiolyticButorphanolAcepromazine, payload.AnxiolyticPlan, existing.VentilationPlan.AnxiolyticButorphanolAcepromazine)
	furosemide := firstNonEmpty(payload.LowDoseFurosemideBronchodilatorNote, payload.LowDoseFurosemideNote, existing.VentilationPlan.LowDoseFurosemideBronchodilatorNote)
	cpap := firstNonEmpty(payload.CPAPOrPEEPMechanicalVentilation, payload.CPAPPeepPlan, existing.VentilationPlan.CPAPOrPEEPMechanicalVentilation)
	fluid := firstNonEmpty(payload.ConservativeFluidMaintenancePlan, payload.ConservativeFluidPlan, existing.VentilationPlan.ConservativeFluidMaintenancePlan)
	oral := firstNonEmpty(payload.OralCommissureBurnAnalgesiaPlan, payload.OralBurnAnalgesia, existing.VentilationPlan.OralCommissureBurnAnalgesiaPlan)

	updated, err := svc.StageCPAPVentilation(id, oxygen, anxiolytic, furosemide, cpap, fluid, oral)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastNCPEElectrocutionEvent("ncpe_electrocution_updated", map[string]any{
		"type":      "ncpe_electrocution_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordNCPERespiratoryCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureNCPEService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordNCPERespiratoryCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordRespiratoryCheck(
		id,
		payload.PaO2FiO2Ratio,
		payload.SpO2Percent,
		payload.RespiratoryRateBpm,
		payload.CaudodorsalBLinesCount,
		payload.LA2AoRatio,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastNCPEElectrocutionEvent("ncpe_electrocution_updated", map[string]any{
		"type":      "ncpe_electrocution_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearNCPEElectrocutionDischarge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureNCPEService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearNCPEElectrocutionDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearNCPEDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastNCPEElectrocutionEvent("ncpe_electrocution_updated", map[string]any{
		"type":      "ncpe_electrocution_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleNCPEElectrocutionStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "ncpe_electrocution")
}

func (s *Server) broadcastNCPEElectrocutionEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("ncpe_electrocution", "ncpe-electrocution-hub", eventType, payload, now)
}
