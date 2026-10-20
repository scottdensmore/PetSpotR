package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/oncologictls"
)

// OncologicTLSPageData represents template data passed to templates/oncologic_tls_triage.html.
type OncologicTLSPageData struct {
	Operations                    []domain.OncologicTLSOperation
	ActiveOperation               *domain.OncologicTLSOperation
	Metrics                       domain.OncologicTLSOverviewMetrics
	TotalOperations               int
	AcuteMetabolicTriageCount     int
	DiuresisStagedCount           int
	ClearedNormoelectrolyticCount int
	TumorLysisSyndromeCount       int
	HypercalcemiaMalignancyCount  int
	Locale                        string
}

type createOncologicTLSOperationPayload struct {
	CaseID                      string                        `json:"case_id"`
	PetName                     string                        `json:"pet_name"`
	Species                     string                        `json:"species"`
	Breed                       string                        `json:"breed"`
	WeightKg                    float64                       `json:"weight_kg"`
	EmergencyType               domain.OncologicEmergencyType `json:"emergency_type"`
	SerumPotassiumMmolL         float64                       `json:"serum_potassium_mmol_l"`
	SerumPhosphorusMgDL         float64                       `json:"serum_phosphorus_mg_dl"`
	IonizedCalciumMmolL         float64                       `json:"ionized_calcium_mmol_l"`
	UricAcidMgDL                float64                       `json:"uric_acid_mg_dl"`
	SerumCreatinineMgDL         float64                       `json:"serum_creatinine_mg_dl"`
	CaXPProduct                 float64                       `json:"ca_x_p_product"`
	DefinitiveCytologyConfirmed bool                          `json:"definitive_cytology_confirmed"`
}

type stageOncologicTLSDiuresisPayload struct {
	ZeroCalciumSalineDiuresisPlan    string `json:"zero_calcium_saline_diuresis_plan"`
	SalineDiuresisPlan               string `json:"saline_diuresis_plan"`
	RasburicaseAllopurinolProtocol   string `json:"rasburicase_allopurinol_protocol"`
	RasburicaseProtocol              string `json:"rasburicase_protocol"`
	ZoledronateCalcitoninProtocol    string `json:"zoledronate_calcitonin_protocol"`
	ZoledronateProtocol              string `json:"zoledronate_protocol"`
	HyperkalemiaCardioprotectionPlan string `json:"hyperkalemia_cardioprotection_plan"`
	CardioprotectionPlan             string `json:"cardioprotection_plan"`
	PhosphateBinderOrLoopDiuretic    string `json:"phosphate_binder_or_loop_diuretic"`
	BinderOrDiuretic                 string `json:"binder_or_diuretic"`
	SerialElectrolyteECGSchedule     string `json:"serial_electrolyte_ecg_schedule"`
	ElectrolyteSchedule              string `json:"electrolyte_schedule"`
}

type recordOncologicTLSElectrolyteCheckPayload struct {
	SerumPotassiumMmolL float64 `json:"serum_potassium_mmol_l"`
	SerumPhosphorusMgDL float64 `json:"serum_phosphorus_mg_dl"`
	IonizedCalciumMmolL float64 `json:"ionized_calcium_mmol_l"`
	UricAcidMgDL        float64 `json:"uric_acid_mg_dl"`
	SerumCreatinineMgDL float64 `json:"serum_creatinine_mg_dl"`
	RecordedBy          string  `json:"recorded_by"`
	ClinicalNote        string  `json:"clinical_note"`
}

type clearOncologicTLSDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureTLSService() *oncologictls.TLSService {
	if s.tlsService == nil {
		s.tlsService = oncologictls.NewTLSService()
	}
	return s.tlsService
}

func (s *Server) registerOncologicTLSRoutes() {
	s.mux.HandleFunc("GET /oncologic-tls-triage", s.handleOncologicTLSTriagePage)

	s.registerDualAPIRoute("GET", "oncologic-tls/operations", s.handleListOncologicTLSOperations)
	s.registerDualAPIRoute("POST", "oncologic-tls/operations", s.handleCreateOncologicTLSOperation)
	s.registerDualAPIRoute("GET", "oncologic-tls/operations/{id}", s.handleGetOncologicTLSOperation)
	s.registerDualAPIRoute("POST", "oncologic-tls/operations/{id}/stage-diuresis", s.handleStageOncologicTLSDiuresis)
	s.registerDualAPIRoute("POST", "oncologic-tls/operations/{id}/record-check", s.handleRecordOncologicTLSElectrolyteCheck)
	s.registerDualAPIRoute("POST", "oncologic-tls/operations/{id}/clear-discharge", s.handleClearOncologicTLSDischarge)
	s.registerDualAPIRoute("GET", "oncologic-tls/stream", s.handleOncologicTLSStream)
}

func (s *Server) handleOncologicTLSTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTLSService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.OncologicTLSOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := OncologicTLSPageData{
		Operations:                    ops,
		ActiveOperation:               activeOp,
		Metrics:                       metrics,
		TotalOperations:               metrics.TotalOperations,
		AcuteMetabolicTriageCount:     metrics.AcuteMetabolicTriageCount,
		DiuresisStagedCount:           metrics.DiuresisStagedCount,
		ClearedNormoelectrolyticCount: metrics.ClearedNormoelectrolyticCount,
		TumorLysisSyndromeCount:       metrics.TumorLysisSyndromeCount,
		HypercalcemiaMalignancyCount:  metrics.HypercalcemiaMalignancyCount,
		Locale:                        locale,
	}

	s.renderCockpitTemplate(w, r, "oncologic_tls_triage.html", "Oncologic TLS Triage", data)
}

func (s *Server) handleListOncologicTLSOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTLSService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	emergencyFilter := strings.TrimSpace(r.URL.Query().Get("emergency_type"))

	ops := svc.ListOperations(statusFilter)
	if emergencyFilter != "" {
		filtered := make([]domain.OncologicTLSOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.EmergencyType), emergencyFilter) {
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

func (s *Server) handleGetOncologicTLSOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureTLSService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateOncologicTLSOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createOncologicTLSOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureTLSService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-TLS-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.EmergencyType,
		payload.SerumPotassiumMmolL,
		payload.SerumPhosphorusMgDL,
		payload.IonizedCalciumMmolL,
		payload.UricAcidMgDL,
		payload.SerumCreatinineMgDL,
		payload.CaXPProduct,
		payload.DefinitiveCytologyConfirmed,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastOncologicTLSEvent("oncologic_tls_updated", map[string]any{
		"type":      "oncologic_tls_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageOncologicTLSDiuresis(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureTLSService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageOncologicTLSDiuresisPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	saline := firstNonEmpty(payload.ZeroCalciumSalineDiuresisPlan, payload.SalineDiuresisPlan, existing.DiuresisPlan.ZeroCalciumSalineDiuresisPlan)
	rasburicase := firstNonEmpty(payload.RasburicaseAllopurinolProtocol, payload.RasburicaseProtocol, existing.DiuresisPlan.RasburicaseAllopurinolProtocol)
	zoledronate := firstNonEmpty(payload.ZoledronateCalcitoninProtocol, payload.ZoledronateProtocol, existing.DiuresisPlan.ZoledronateCalcitoninProtocol)
	cardio := firstNonEmpty(payload.HyperkalemiaCardioprotectionPlan, payload.CardioprotectionPlan, existing.DiuresisPlan.HyperkalemiaCardioprotectionPlan)
	binder := firstNonEmpty(payload.PhosphateBinderOrLoopDiuretic, payload.BinderOrDiuretic, existing.DiuresisPlan.PhosphateBinderOrLoopDiuretic)
	schedule := firstNonEmpty(payload.SerialElectrolyteECGSchedule, payload.ElectrolyteSchedule, existing.DiuresisPlan.SerialElectrolyteECGSchedule)

	updated, err := svc.StageDiuresisTherapy(id, saline, rasburicase, zoledronate, cardio, binder, schedule)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastOncologicTLSEvent("oncologic_tls_updated", map[string]any{
		"type":      "oncologic_tls_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordOncologicTLSElectrolyteCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureTLSService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordOncologicTLSElectrolyteCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordElectrolyteCheck(
		id,
		payload.SerumPotassiumMmolL,
		payload.SerumPhosphorusMgDL,
		payload.IonizedCalciumMmolL,
		payload.UricAcidMgDL,
		payload.SerumCreatinineMgDL,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastOncologicTLSEvent("oncologic_tls_updated", map[string]any{
		"type":      "oncologic_tls_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearOncologicTLSDischarge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureTLSService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearOncologicTLSDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearTLSDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastOncologicTLSEvent("oncologic_tls_updated", map[string]any{
		"type":      "oncologic_tls_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleOncologicTLSStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "oncologic_tls")
}

func (s *Server) broadcastOncologicTLSEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("oncologic_tls", "oncologic-tls-hub", eventType, payload, now)
}
