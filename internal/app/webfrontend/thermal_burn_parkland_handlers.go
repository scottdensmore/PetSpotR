package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/thermalburnparkland"
)

// ThermalBurnParklandPageData represents template data passed to templates/thermal_burn_parkland_triage.html.
type ThermalBurnParklandPageData struct {
	Operations                 []domain.ThermalBurnOperation
	ActiveOperation            *domain.ThermalBurnOperation
	Metrics                    domain.ThermalBurnOverviewMetrics
	TotalOperations            int
	AcuteBurnTriageCount       int
	ParklandStagedCount        int
	ClearedGranulationCount    int
	SevereTBSAOver20PctCount   int
	CircumferentialEscharCount int
	Locale                     string
}

type createThermalBurnOperationPayload struct {
	CaseID                        string                         `json:"case_id"`
	PetName                       string                         `json:"pet_name"`
	Species                       string                         `json:"species"`
	Breed                         string                         `json:"breed"`
	WeightKg                      float64                        `json:"weight_kg"`
	DepthClass                    domain.BurnDepthClassification `json:"depth_class"`
	TBSAPercent                   float64                        `json:"tbsa_percent"`
	CurrentFluidRateMLHr          float64                        `json:"current_fluid_rate_ml_hr"`
	UrineOutputMLKgHr             float64                        `json:"urine_output_ml_kg_hr"`
	SerumAlbuminGDL               float64                        `json:"serum_albumin_g_dl"`
	PCVPercent                    float64                        `json:"pcv_percent"`
	CircumferentialThoracicEschar bool                           `json:"circumferential_thoracic_eschar"`
	SingedVibrissaeSootAirway     bool                           `json:"singed_vibrissae_soot_airway"`
}

type stageThermalBurnParklandPayload struct {
	ParklandCrystalloidSchedule      string `json:"parkland_crystalloid_schedule"`
	ParklandSchedule                 string `json:"parkland_schedule"`
	SilverSulfadiazineHoneyWoundPlan string `json:"silver_sulfadiazine_honey_wound_plan"`
	SSDHoneyPlan                     string `json:"ssd_honey_plan"`
	EscharotomyWetDebridementPlan    string `json:"escharotomy_wet_debridement_plan"`
	EscharotomyPlan                  string `json:"escharotomy_plan"`
	MultimodalBurnAnalgesiaCRI       string `json:"multimodal_burn_analgesia_cri"`
	AnalgesiaCRI                     string `json:"analgesia_cri"`
	AlbuminOncotic24hTransition      string `json:"albumin_oncotic_24h_transition"`
	AlbuminTransition                string `json:"albumin_transition"`
	CornealThermalAirwayProtection   string `json:"corneal_thermal_airway_protection"`
	AirwayCornealPlan                string `json:"airway_corneal_plan"`
}

type recordThermalBurnCheckPayload struct {
	TBSAPercent          float64 `json:"tbsa_percent"`
	CurrentFluidRateMLHr float64 `json:"current_fluid_rate_ml_hr"`
	UrineOutputMLKgHr    float64 `json:"urine_output_ml_kg_hr"`
	SerumAlbuminGDL      float64 `json:"serum_albumin_g_dl"`
	PCVPercent           float64 `json:"pcv_percent"`
	RecordedBy           string  `json:"recorded_by"`
	ClinicalNote         string  `json:"clinical_note"`
}

type clearThermalBurnDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureBurnService() *thermalburnparkland.BurnService {
	if s.burnService == nil {
		s.burnService = thermalburnparkland.NewBurnService()
	}
	return s.burnService
}

func (s *Server) registerThermalBurnParklandRoutes() {
	s.mux.HandleFunc("GET /thermal-burn-parkland-triage", s.handleThermalBurnParklandTriagePage)

	s.registerDualAPIRoute("GET", "thermal-burn-parkland/operations", s.handleListThermalBurnOperations)
	s.registerDualAPIRoute("POST", "thermal-burn-parkland/operations", s.handleCreateThermalBurnOperation)
	s.registerDualAPIRoute("GET", "thermal-burn-parkland/operations/{id}", s.handleGetThermalBurnOperation)
	s.registerDualAPIRoute("POST", "thermal-burn-parkland/operations/{id}/stage-parkland", s.handleStageThermalBurnParkland)
	s.registerDualAPIRoute("POST", "thermal-burn-parkland/operations/{id}/record-check", s.handleRecordThermalBurnCheck)
	s.registerDualAPIRoute("POST", "thermal-burn-parkland/operations/{id}/clear-discharge", s.handleClearThermalBurnDischarge)
	s.registerDualAPIRoute("GET", "thermal-burn-parkland/stream", s.handleThermalBurnParklandStream)
}

func (s *Server) handleThermalBurnParklandTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureBurnService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.ThermalBurnOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := ThermalBurnParklandPageData{
		Operations:                 ops,
		ActiveOperation:            activeOp,
		Metrics:                    metrics,
		TotalOperations:            metrics.TotalOperations,
		AcuteBurnTriageCount:       metrics.AcuteBurnTriageCount,
		ParklandStagedCount:        metrics.ParklandStagedCount,
		ClearedGranulationCount:    metrics.ClearedGranulationCount,
		SevereTBSAOver20PctCount:   metrics.SevereTBSAOver20PctCount,
		CircumferentialEscharCount: metrics.CircumferentialEscharCount,
		Locale:                     locale,
	}

	s.renderCockpitTemplate(w, r, "thermal_burn_parkland_triage.html", "Thermal Burn Parkland Triage", data)
}

func (s *Server) handleListThermalBurnOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureBurnService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	depthFilter := strings.TrimSpace(r.URL.Query().Get("depth_class"))

	ops := svc.ListOperations(statusFilter)
	if depthFilter != "" {
		filtered := make([]domain.ThermalBurnOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.DepthClass), depthFilter) {
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

func (s *Server) handleGetThermalBurnOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureBurnService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateThermalBurnOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createThermalBurnOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureBurnService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-BURN-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.DepthClass,
		payload.TBSAPercent,
		payload.CurrentFluidRateMLHr,
		payload.UrineOutputMLKgHr,
		payload.SerumAlbuminGDL,
		payload.PCVPercent,
		payload.CircumferentialThoracicEschar,
		payload.SingedVibrissaeSootAirway,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastThermalBurnParklandEvent("thermal_burn_parkland_updated", map[string]any{
		"type":      "thermal_burn_parkland_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageThermalBurnParkland(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureBurnService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageThermalBurnParklandPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	parkland := firstNonEmpty(payload.ParklandCrystalloidSchedule, payload.ParklandSchedule, existing.WoundPlan.ParklandCrystalloidSchedule)
	ssd := firstNonEmpty(payload.SilverSulfadiazineHoneyWoundPlan, payload.SSDHoneyPlan, existing.WoundPlan.SilverSulfadiazineHoneyWoundPlan)
	eschar := firstNonEmpty(payload.EscharotomyWetDebridementPlan, payload.EscharotomyPlan, existing.WoundPlan.EscharotomyWetDebridementPlan)
	analgesia := firstNonEmpty(payload.MultimodalBurnAnalgesiaCRI, payload.AnalgesiaCRI, existing.WoundPlan.MultimodalBurnAnalgesiaCRI)
	albumin := firstNonEmpty(payload.AlbuminOncotic24hTransition, payload.AlbuminTransition, existing.WoundPlan.AlbuminOncotic24hTransition)
	airway := firstNonEmpty(payload.CornealThermalAirwayProtection, payload.AirwayCornealPlan, existing.WoundPlan.CornealThermalAirwayProtection)

	updated, err := svc.StageParklandWoundPlan(id, parkland, ssd, eschar, analgesia, albumin, airway)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastThermalBurnParklandEvent("thermal_burn_parkland_updated", map[string]any{
		"type":      "thermal_burn_parkland_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordThermalBurnCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureBurnService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordThermalBurnCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordBurnCheck(
		id,
		payload.TBSAPercent,
		payload.CurrentFluidRateMLHr,
		payload.UrineOutputMLKgHr,
		payload.SerumAlbuminGDL,
		payload.PCVPercent,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastThermalBurnParklandEvent("thermal_burn_parkland_updated", map[string]any{
		"type":      "thermal_burn_parkland_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearThermalBurnDischarge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureBurnService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearThermalBurnDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearBurnDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastThermalBurnParklandEvent("thermal_burn_parkland_updated", map[string]any{
		"type":      "thermal_burn_parkland_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleThermalBurnParklandStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "thermal_burn_parkland")
}

func (s *Server) broadcastThermalBurnParklandEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("thermal_burn_parkland", "thermal-burn-parkland-hub", eventType, payload, now)
}
