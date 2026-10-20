package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/fipgs441524"
)

// FIPGS441524PageData represents template data passed to templates/fip_gs441524_triage.html.
type FIPGS441524PageData struct {
	Operations               []domain.FIPGS441524Operation
	ActiveOperation          *domain.FIPGS441524Operation
	Metrics                  domain.FIPOverviewMetrics
	TotalOperations          int
	AcuteFIPTriageCount      int
	GS441524StagedCount      int
	Cleared84DayCuredCount   int
	NeuroOcularHighDoseCount int
	AGRatioUnder04Count      int
	Locale                   string
}

type createFIPGS441524OperationPayload struct {
	CaseID                        string                      `json:"case_id"`
	PetName                       string                      `json:"pet_name"`
	Species                       string                      `json:"species"`
	Breed                         string                      `json:"breed"`
	WeightKg                      float64                     `json:"weight_kg"`
	Phenotype                     domain.FIPClinicalPhenotype `json:"phenotype"`
	TreatmentDay                  int                         `json:"treatment_day"`
	AlbuminGlobulinRatio          float64                     `json:"albumin_globulin_ratio"`
	TotalBilirubinMgDL            float64                     `json:"total_bilirubin_mg_dl"`
	HematocritPercent             float64                     `json:"hematocrit_percent"`
	DailyGS441524MgKg             float64                     `json:"daily_gs441524_mg_kg"`
	RivaltaTestPositive           bool                        `json:"rivalta_test_positive"`
	PleuralEffusionDyspneaPresent bool                        `json:"pleural_effusion_dyspnea_present"`
}

type stageFIPGS441524AntiviralPayload struct {
	GS441524OrRemdesivirDosing      string `json:"gs441524_or_remdesivir_dosing"`
	GS441524Dosing                  string `json:"gs441524_dosing"`
	WeightAdjustedDoseRecalcNote    string `json:"weight_adjusted_dose_recalc_note"`
	WeightRecalcNote                string `json:"weight_recalc_note"`
	TherapeuticThoracocentesisLimit string `json:"therapeutic_thoracocentesis_limit"`
	CentesisLimit                   string `json:"centesis_limit"`
	OcularTopicalAtropinePredNote   string `json:"ocular_topical_atropine_pred_note"`
	OcularSupportNote               string `json:"ocular_support_note"`
	HepatoprotectantB12SupportPlan  string `json:"hepatoprotectant_b12_support_plan"`
	HepatoprotectantPlan            string `json:"hepatoprotectant_plan"`
	Day28Day56Day84MonitoringPlan   string `json:"day28_day56_day84_monitoring_plan"`
	MonitoringPlan                  string `json:"monitoring_plan"`
}

type recordFIPGS441524ProgressCheckPayload struct {
	TreatmentDay         int     `json:"treatment_day"`
	WeightKg             float64 `json:"weight_kg"`
	AlbuminGlobulinRatio float64 `json:"albumin_globulin_ratio"`
	TotalBilirubinMgDL   float64 `json:"total_bilirubin_mg_dl"`
	HematocritPercent    float64 `json:"hematocrit_percent"`
	RecordedBy           string  `json:"recorded_by"`
	ClinicalNote         string  `json:"clinical_note"`
}

type clearFIPGS441524DischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureFIPService() *fipgs441524.FIPService {
	if s.fipService == nil {
		s.fipService = fipgs441524.NewFIPService()
	}
	return s.fipService
}

func (s *Server) registerFIPGS441524Routes() {
	s.mux.HandleFunc("GET /fip-gs441524-triage", s.handleFIPGS441524TriagePage)

	s.registerDualAPIRoute("GET", "fip-gs441524/operations", s.handleListFIPGS441524Operations)
	s.registerDualAPIRoute("POST", "fip-gs441524/operations", s.handleCreateFIPGS441524Operation)
	s.registerDualAPIRoute("GET", "fip-gs441524/operations/{id}", s.handleGetFIPGS441524Operation)
	s.registerDualAPIRoute("POST", "fip-gs441524/operations/{id}/stage-antiviral", s.handleStageFIPGS441524Antiviral)
	s.registerDualAPIRoute("POST", "fip-gs441524/operations/{id}/record-check", s.handleRecordFIPGS441524ProgressCheck)
	s.registerDualAPIRoute("POST", "fip-gs441524/operations/{id}/clear-discharge", s.handleClearFIPGS441524Discharge)
	s.registerDualAPIRoute("GET", "fip-gs441524/stream", s.handleFIPGS441524Stream)
}

func (s *Server) handleFIPGS441524TriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureFIPService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.FIPGS441524Operation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := FIPGS441524PageData{
		Operations:               ops,
		ActiveOperation:          activeOp,
		Metrics:                  metrics,
		TotalOperations:          metrics.TotalOperations,
		AcuteFIPTriageCount:      metrics.AcuteFIPTriageCount,
		GS441524StagedCount:      metrics.GS441524StagedCount,
		Cleared84DayCuredCount:   metrics.Cleared84DayCuredCount,
		NeuroOcularHighDoseCount: metrics.NeuroOcularHighDoseCount,
		AGRatioUnder04Count:      metrics.AGRatioUnder04Count,
		Locale:                   locale,
	}

	s.renderCockpitTemplate(w, r, "fip_gs441524_triage.html", "FIP & GS-441524 Triage", data)
}

func (s *Server) handleListFIPGS441524Operations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureFIPService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	phenotypeFilter := strings.TrimSpace(r.URL.Query().Get("phenotype"))

	ops := svc.ListOperations(statusFilter)
	if phenotypeFilter != "" {
		filtered := make([]domain.FIPGS441524Operation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.Phenotype), phenotypeFilter) {
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

func (s *Server) handleGetFIPGS441524Operation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureFIPService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateFIPGS441524Operation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createFIPGS441524OperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureFIPService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-FIP-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.Phenotype,
		payload.TreatmentDay,
		payload.AlbuminGlobulinRatio,
		payload.TotalBilirubinMgDL,
		payload.HematocritPercent,
		payload.DailyGS441524MgKg,
		payload.RivaltaTestPositive,
		payload.PleuralEffusionDyspneaPresent,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastFIPGS441524Event("fip_gs441524_updated", map[string]any{
		"type":      "fip_gs441524_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageFIPGS441524Antiviral(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureFIPService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageFIPGS441524AntiviralPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	dosing := firstNonEmpty(payload.GS441524OrRemdesivirDosing, payload.GS441524Dosing, existing.AntiviralPlan.GS441524OrRemdesivirDosing)
	recalc := firstNonEmpty(payload.WeightAdjustedDoseRecalcNote, payload.WeightRecalcNote, existing.AntiviralPlan.WeightAdjustedDoseRecalcNote)
	centesis := firstNonEmpty(payload.TherapeuticThoracocentesisLimit, payload.CentesisLimit, existing.AntiviralPlan.TherapeuticThoracocentesisLimit)
	ocular := firstNonEmpty(payload.OcularTopicalAtropinePredNote, payload.OcularSupportNote, existing.AntiviralPlan.OcularTopicalAtropinePredNote)
	hepato := firstNonEmpty(payload.HepatoprotectantB12SupportPlan, payload.HepatoprotectantPlan, existing.AntiviralPlan.HepatoprotectantB12SupportPlan)
	monitoring := firstNonEmpty(payload.Day28Day56Day84MonitoringPlan, payload.MonitoringPlan, existing.AntiviralPlan.Day28Day56Day84MonitoringPlan)

	updated, err := svc.StageAntiviralProtocol(id, dosing, recalc, centesis, ocular, hepato, monitoring)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastFIPGS441524Event("fip_gs441524_updated", map[string]any{
		"type":      "fip_gs441524_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordFIPGS441524ProgressCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureFIPService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordFIPGS441524ProgressCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordProgressCheck(
		id,
		payload.TreatmentDay,
		payload.WeightKg,
		payload.AlbuminGlobulinRatio,
		payload.TotalBilirubinMgDL,
		payload.HematocritPercent,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastFIPGS441524Event("fip_gs441524_updated", map[string]any{
		"type":      "fip_gs441524_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearFIPGS441524Discharge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureFIPService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearFIPGS441524DischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearFIPDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastFIPGS441524Event("fip_gs441524_updated", map[string]any{
		"type":      "fip_gs441524_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleFIPGS441524Stream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "fip_gs441524")
}

func (s *Server) broadcastFIPGS441524Event(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("fip_gs441524", "fip-gs441524-hub", eventType, payload, now)
}
