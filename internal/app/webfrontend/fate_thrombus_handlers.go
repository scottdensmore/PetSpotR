package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/fatethrombus"
)

// FATEThrombusPageData represents template data passed to templates/fate_thrombus_triage.html.
type FATEThrombusPageData struct {
	Operations             []domain.FATEThrombusOperation
	ActiveOperation        *domain.FATEThrombusOperation
	Metrics                domain.FATEOverviewMetrics
	TotalOperations        int
	AcuteSaddleTriageCount int
	AnticoagStagedCount    int
	ClearedReperfusedCount int
	BilateralSaddleCount   int
	ConcurrentCHFCount     int
	Locale                 string
}

type createFATEOperationPayload struct {
	CaseID                   string                          `json:"case_id"`
	PetName                  string                          `json:"pet_name"`
	Species                  string                          `json:"species"`
	Breed                    string                          `json:"breed"`
	WeightKg                 float64                         `json:"weight_kg"`
	DistributionType         domain.ThrombusDistributionType `json:"distribution_type"`
	SystemicGlucoseMgDL      float64                         `json:"systemic_glucose_mg_dl"`
	AffectedLimbGlucoseMgDL  float64                         `json:"affected_limb_glucose_mg_dl"`
	SystemicLactateMmolL     float64                         `json:"systemic_lactate_mmol_l"`
	AffectedLimbLactateMmolL float64                         `json:"affected_limb_lactate_mmol_l"`
	SerumPotassiumMmolL      float64                         `json:"serum_potassium_mmol_l"`
	DistalLimbTempC          float64                         `json:"distal_limb_temp_c"`
	LA2AoRatio               float64                         `json:"la2ao_ratio"`
	ConcurrentCardiogenicCHF bool                            `json:"concurrent_cardiogenic_chf"`
}

type stageFATEAntithromboticPayload struct {
	FullMuOpioidCRIProtocol     string `json:"full_mu_opioid_cri_protocol"`
	OpioidProtocol              string `json:"opioid_protocol"`
	ClopidogrelLoadingProtocol  string `json:"clopidogrel_loading_protocol"`
	ClopidogrelLoading          string `json:"clopidogrel_loading"`
	FactorXaOrHeparinProtocol   string `json:"factor_xa_or_heparin_protocol"`
	FactorXaProtocol            string `json:"factor_xa_protocol"`
	ConcurrentCHFFurosemidePlan string `json:"concurrent_chf_furosemide_plan"`
	CHFFurosemidePlan           string `json:"chf_furosemide_plan"`
	SerialPotassiumMonitoring   string `json:"serial_potassium_monitoring"`
	PotassiumMonitoring         string `json:"potassium_monitoring"`
	PimobendanLVOTNote          string `json:"pimobendan_lvot_note"`
	PimobendanNote              string `json:"pimobendan_note"`
}

type recordFATEPerfusionCheckPayload struct {
	GlucoseDeltaMgDL    float64 `json:"glucose_delta_mg_dl"`
	LactateDeltaMmolL   float64 `json:"lactate_delta_mmol_l"`
	SerumPotassiumMmolL float64 `json:"serum_potassium_mmol_l"`
	DistalLimbTempC     float64 `json:"distal_limb_temp_c"`
	FemoralPulseStatus  string  `json:"femoral_pulse_status"`
	RecordedBy          string  `json:"recorded_by"`
	ClinicalNote        string  `json:"clinical_note"`
}

type clearFATEDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureFATEService() *fatethrombus.FATEService {
	if s.fateService == nil {
		s.fateService = fatethrombus.NewFATEService()
	}
	return s.fateService
}

func (s *Server) registerFATEThrombusRoutes() {
	s.mux.HandleFunc("GET /fate-thrombus-triage", s.handleFATEThrombusTriagePage)

	s.registerDualAPIRoute("GET", "fate-thrombus/operations", s.handleListFATEThrombusOperations)
	s.registerDualAPIRoute("POST", "fate-thrombus/operations", s.handleCreateFATEThrombusOperation)
	s.registerDualAPIRoute("GET", "fate-thrombus/operations/{id}", s.handleGetFATEThrombusOperation)
	s.registerDualAPIRoute("POST", "fate-thrombus/operations/{id}/stage-antithrombotic", s.handleStageFATEAntithrombotic)
	s.registerDualAPIRoute("POST", "fate-thrombus/operations/{id}/record-check", s.handleRecordFATEPerfusionCheck)
	s.registerDualAPIRoute("POST", "fate-thrombus/operations/{id}/clear-discharge", s.handleClearFATEDischarge)
	s.registerDualAPIRoute("GET", "fate-thrombus/stream", s.handleFATEThrombusStream)
}

func (s *Server) handleFATEThrombusTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureFATEService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.FATEThrombusOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := FATEThrombusPageData{
		Operations:             ops,
		ActiveOperation:        activeOp,
		Metrics:                metrics,
		TotalOperations:        metrics.TotalOperations,
		AcuteSaddleTriageCount: metrics.AcuteSaddleTriageCount,
		AnticoagStagedCount:    metrics.AnticoagStagedCount,
		ClearedReperfusedCount: metrics.ClearedReperfusedCount,
		BilateralSaddleCount:   metrics.BilateralSaddleCount,
		ConcurrentCHFCount:     metrics.ConcurrentCHFCount,
		Locale:                 locale,
	}

	s.renderCockpitTemplate(w, r, "fate_thrombus_triage.html", "FATE Thrombus Triage", data)
}

func (s *Server) handleListFATEThrombusOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureFATEService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	distFilter := strings.TrimSpace(r.URL.Query().Get("distribution"))

	ops := svc.ListOperations(statusFilter)
	if distFilter != "" {
		filtered := make([]domain.FATEThrombusOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.DistributionType), distFilter) {
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

func (s *Server) handleGetFATEThrombusOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureFATEService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateFATEThrombusOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createFATEOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureFATEService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-FATE-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.DistributionType,
		payload.SystemicGlucoseMgDL,
		payload.AffectedLimbGlucoseMgDL,
		payload.SystemicLactateMmolL,
		payload.AffectedLimbLactateMmolL,
		payload.SerumPotassiumMmolL,
		payload.DistalLimbTempC,
		payload.LA2AoRatio,
		payload.ConcurrentCardiogenicCHF,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastFATEThrombusEvent("fate_thrombus_updated", map[string]any{
		"type":      "fate_thrombus_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageFATEAntithrombotic(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureFATEService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageFATEAntithromboticPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	opioid := firstNonEmpty(payload.FullMuOpioidCRIProtocol, payload.OpioidProtocol, existing.AntithromboticPlan.FullMuOpioidCRIProtocol)
	clopidogrel := firstNonEmpty(payload.ClopidogrelLoadingProtocol, payload.ClopidogrelLoading, existing.AntithromboticPlan.ClopidogrelLoadingProtocol)
	factorXa := firstNonEmpty(payload.FactorXaOrHeparinProtocol, payload.FactorXaProtocol, existing.AntithromboticPlan.FactorXaOrHeparinProtocol)
	chfPlan := firstNonEmpty(payload.ConcurrentCHFFurosemidePlan, payload.CHFFurosemidePlan, existing.AntithromboticPlan.ConcurrentCHFFurosemidePlan)
	kMonitor := firstNonEmpty(payload.SerialPotassiumMonitoring, payload.PotassiumMonitoring, existing.AntithromboticPlan.SerialPotassiumMonitoring)
	pimoNote := firstNonEmpty(payload.PimobendanLVOTNote, payload.PimobendanNote, existing.AntithromboticPlan.PimobendanLVOTNote)

	updated, err := svc.StageAntithrombotic(id, opioid, clopidogrel, factorXa, chfPlan, kMonitor, pimoNote)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastFATEThrombusEvent("fate_thrombus_updated", map[string]any{
		"type":      "fate_thrombus_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordFATEPerfusionCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureFATEService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordFATEPerfusionCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordPerfusionCheck(
		id,
		payload.GlucoseDeltaMgDL,
		payload.LactateDeltaMmolL,
		payload.SerumPotassiumMmolL,
		payload.DistalLimbTempC,
		payload.FemoralPulseStatus,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastFATEThrombusEvent("fate_thrombus_updated", map[string]any{
		"type":      "fate_thrombus_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearFATEDischarge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureFATEService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearFATEDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearFATEDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastFATEThrombusEvent("fate_thrombus_updated", map[string]any{
		"type":      "fate_thrombus_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleFATEThrombusStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "fate_thrombus")
}

func (s *Server) broadcastFATEThrombusEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("fate_thrombus", "fate-thrombus-hub", eventType, payload, now)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
