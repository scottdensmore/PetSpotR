package webfrontend

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/systemicmycoses"
)

var lastSystemicMycosesNano atomic.Int64

func nextSystemicMycosesNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastSystemicMycosesNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastSystemicMycosesNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// SystemicMycosesPageData represents template data passed to templates/systemic_mycoses_triage.html.
type SystemicMycosesPageData struct {
	Operations                               []domain.SystemicMycosesOperation
	ActiveOperation                          *domain.SystemicMycosesOperation
	Metrics                                  domain.SystemicMycosesOverview
	TotalOperations                          int
	AzoleAmphotericinTrephinationStagedCount int
	AntigenEIACreaOphthalmicMonitoredCount   int
	TwoNegativeAntigenEIAClearedCount        int
	BlastomycosisHistoplasmosisCount         int
	CribriformBreachContraindicatedCount     int
	AzotemiaNephrotoxicityAlertCount         int
	Locale                                   string
}

type createSystemicMycosesOperationPayload struct {
	CaseID                                  string                    `json:"case_id"`
	PetName                                 string                    `json:"pet_name"`
	Species                                 string                    `json:"species"`
	Breed                                   string                    `json:"breed"`
	WeightKg                                float64                   `json:"weight_kg"`
	Syndrome                                domain.FungalSyndrome     `json:"syndrome"`
	Protocol                                domain.AntifungalProtocol `json:"protocol"`
	MiraVistaAntigenNgML                    float64                   `json:"miravista_antigen_ng_ml"`
	SerumCreatinineMgDL                     float64                   `json:"serum_creatinine_mg_dl"`
	ALTActivityUL                           float64                   `json:"alt_activity_u_l"`
	SpO2Percent                             float64                   `json:"spo2_percent"`
	CribriformPlateIntactOnCT               *bool                     `json:"cribriform_plate_intact_on_ct"`
	CompoundedBulkItraconazoleBlocked       *bool                     `json:"compounded_bulk_itraconazole_blocked"`
	AmphotericinLipidSalineDiuresisVerified *bool                     `json:"amphotericin_lipid_saline_diuresis_verified"`
}

type stageSystemicMycosesTherapyPayload struct {
	Protocol                                  domain.AntifungalProtocol `json:"protocol"`
	ItraconazoleMgKgDay                       float64                   `json:"itraconazole_mg_kg_day"`
	FluconazoleMgKgDay                        float64                   `json:"fluconazole_mg_kg_day"`
	AmphotericinBLipidMgKgDose                float64                   `json:"amphotericin_b_lipid_mg_kg_dose"`
	CompoundedBulkItraconazoleBlocked         *bool                     `json:"compounded_bulk_itraconazole_blocked"`
	CribriformPlateIntactOnCT                 *bool                     `json:"cribriform_plate_intact_on_ct"`
	AmphotericinLipidSalineDiuresisVerified   *bool                     `json:"amphotericin_lipid_saline_diuresis_verified"`
	DieOffDexamethasoneAntiInflammatoryStaged *bool                     `json:"die_off_dexamethasone_anti_inflammatory_staged"`
	SystemicAzoleAndPolyeneRegimenPlan        string                    `json:"systemic_azole_and_polyene_regimen_plan"`
	AzoleRegimenPlan                          string                    `json:"azole_regimen_plan"`
	PulmonaryDieOffARDSPlan                   string                    `json:"pulmonary_die_off_ards_plan"`
	DieOffARDSPlan                            string                    `json:"die_off_ards_plan"`
	OphthalmicChorioretinitisNeuroPlan        string                    `json:"ophthalmic_chorioretinitis_neuro_plan"`
	OphthalmicNeuroPlan                       string                    `json:"ophthalmic_neuro_plan"`
	MinimumAzoleDurationMonths                int                       `json:"minimum_azole_duration_months"`
	StagedBy                                  string                    `json:"staged_by"`
}

type recordMycosesAntigenRenalCheckPayload struct {
	MiraVistaAntigenNgML              float64 `json:"miravista_antigen_ng_ml"`
	SerumCreatinineMgDL               float64 `json:"serum_creatinine_mg_dl"`
	ALTActivityUL                     float64 `json:"alt_activity_u_l"`
	SpO2Percent                       float64 `json:"spo2_percent"`
	CribriformPlateIntactOnCT         *bool   `json:"cribriform_plate_intact_on_ct"`
	AmphotericinDoseHeldForAzotemia   bool    `json:"amphotericin_dose_held_for_azotemia"`
	OphthalmicChorioretinitisResolved bool    `json:"ophthalmic_chorioretinitis_resolved"`
	RecordedBy                        string  `json:"recorded_by"`
	ClinicalNote                      string  `json:"clinical_note"`
}

type evaluateSystemicMycosesClearancePayload struct {
	MiraVistaAntigenNgML              *float64 `json:"miravista_antigen_ng_ml"`
	SerumCreatinineMgDL               *float64 `json:"serum_creatinine_mg_dl"`
	TwoConsecutiveNegativeEIAVerified *bool    `json:"two_consecutive_negative_eia_verified"`
	Minimum6MonthAzoleCourseVerified  *bool    `json:"minimum_6month_azole_course_verified"`
	ClearedBy                         string   `json:"cleared_by"`
	DischargedBy                      string   `json:"discharged_by"`
	ClearanceSummary                  string   `json:"clearance_summary"`
	DischargeSummary                  string   `json:"discharge_summary"`
	Summary                           string   `json:"summary"`
}

func (s *Server) ensureSystemicMycosesService() *systemicmycoses.SystemicMycosesService {
	if s.systemicMycosesService == nil {
		s.systemicMycosesService = systemicmycoses.NewSystemicMycosesService()
	}
	return s.systemicMycosesService
}

func (s *Server) registerSystemicMycosesRoutes() {
	s.mux.HandleFunc("GET /systemic-mycoses-fungal-triage", s.handleSystemicMycosesTriagePage)

	s.mux.HandleFunc("GET /api/systemic-mycoses/overview", s.handleSystemicMycosesOverview)
	s.mux.HandleFunc("GET /api/systemic-mycoses/operations", s.handleListSystemicMycosesOperations)
	s.mux.HandleFunc("POST /api/systemic-mycoses/operations", s.handleCreateSystemicMycosesOperation)
	s.mux.HandleFunc("GET /api/systemic-mycoses/operations/{id}", s.handleGetSystemicMycosesOperation)
	s.mux.HandleFunc("POST /api/systemic-mycoses/operations/{id}/therapy", s.handleStageSystemicMycosesTherapy)
	s.mux.HandleFunc("POST /api/systemic-mycoses/operations/{id}/antigen-check", s.handleRecordMycosesAntigenRenalCheck)
	s.mux.HandleFunc("POST /api/systemic-mycoses/operations/{id}/clearance", s.handleEvaluateSystemicMycosesClearance)
	s.mux.HandleFunc("GET /api/systemic-mycoses/stream", s.handleSystemicMycosesStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/systemic-mycoses/overview", s.handleSystemicMycosesOverview)
	s.mux.HandleFunc("GET /api/v1/systemic-mycoses/operations", s.handleListSystemicMycosesOperations)
	s.mux.HandleFunc("POST /api/v1/systemic-mycoses/operations", s.handleCreateSystemicMycosesOperation)
	s.mux.HandleFunc("GET /api/v1/systemic-mycoses/operations/{id}", s.handleGetSystemicMycosesOperation)
	s.mux.HandleFunc("POST /api/v1/systemic-mycoses/operations/{id}/therapy", s.handleStageSystemicMycosesTherapy)
	s.mux.HandleFunc("POST /api/v1/systemic-mycoses/operations/{id}/antigen-check", s.handleRecordMycosesAntigenRenalCheck)
	s.mux.HandleFunc("POST /api/v1/systemic-mycoses/operations/{id}/clearance", s.handleEvaluateSystemicMycosesClearance)
	s.mux.HandleFunc("GET /api/v1/systemic-mycoses/stream", s.handleSystemicMycosesStream)
}

func writeSystemicMycosesJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleSystemicMycosesTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureSystemicMycosesService()
	ops := svc.ListOperations("")
	metrics := svc.CalculateOverview()

	var activeOp *domain.SystemicMycosesOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("systemic_mycoses_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/systemic_mycoses_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Systemic Mycoses Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := SystemicMycosesPageData{
		Operations:                               ops,
		ActiveOperation:                          activeOp,
		Metrics:                                  metrics,
		TotalOperations:                          metrics.TotalOperations,
		AzoleAmphotericinTrephinationStagedCount: metrics.AzoleAmphotericinTrephinationStagedCount,
		AntigenEIACreaOphthalmicMonitoredCount:   metrics.AntigenEIACreaOphthalmicMonitoredCount,
		TwoNegativeAntigenEIAClearedCount:        metrics.TwoNegativeAntigenEIAClearedCount,
		BlastomycosisHistoplasmosisCount:         metrics.BlastomycosisHistoplasmosisCount,
		CribriformBreachContraindicatedCount:     metrics.CribriformBreachContraindicatedCount,
		AzotemiaNephrotoxicityAlertCount:         metrics.AzotemiaNephrotoxicityAlertCount,
		Locale:                                   locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleSystemicMycosesOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureSystemicMycosesService()
	overview := svc.CalculateOverview()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(overview)
}

func (s *Server) handleListSystemicMycosesOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureSystemicMycosesService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	syndromeFilter := strings.TrimSpace(r.URL.Query().Get("syndrome"))

	ops := svc.ListOperations(statusFilter)
	if syndromeFilter != "" {
		filtered := make([]domain.SystemicMycosesOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.Syndrome), syndromeFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	metrics := svc.CalculateOverview()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
		"overview":   metrics,
	})
}

func (s *Server) handleGetSystemicMycosesOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureSystemicMycosesService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeSystemicMycosesJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateSystemicMycosesOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createSystemicMycosesOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureSystemicMycosesService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-MYC-%d", 100+len(svc.ListOperations(""))+1)
	}

	cribriformIntact := true
	if payload.CribriformPlateIntactOnCT != nil {
		cribriformIntact = *payload.CribriformPlateIntactOnCT
	}
	compoundedBlocked := true
	if payload.CompoundedBulkItraconazoleBlocked != nil {
		compoundedBlocked = *payload.CompoundedBulkItraconazoleBlocked
	}
	salineDiuresis := true
	if payload.AmphotericinLipidSalineDiuresisVerified != nil {
		salineDiuresis = *payload.AmphotericinLipidSalineDiuresisVerified
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.Syndrome,
		payload.Protocol,
		payload.MiraVistaAntigenNgML,
		payload.SerumCreatinineMgDL,
		payload.ALTActivityUL,
		payload.SpO2Percent,
		cribriformIntact,
		compoundedBlocked,
		salineDiuresis,
	)
	if err != nil {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextSystemicMycosesNano()).UTC()
	s.broadcastSystemicMycosesEvent("systemic_mycoses_updated", map[string]any{
		"type":      "systemic_mycoses_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageSystemicMycosesTherapy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureSystemicMycosesService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeSystemicMycosesJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageSystemicMycosesTherapyPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	protocol := payload.Protocol
	if protocol == "" {
		protocol = existing.Protocol
	}
	compoundedBlocked := existing.CompoundedBulkItraconazoleBlocked
	if payload.CompoundedBulkItraconazoleBlocked != nil {
		compoundedBlocked = *payload.CompoundedBulkItraconazoleBlocked
	}
	cribriformIntact := existing.CribriformPlateIntactOnCT
	if payload.CribriformPlateIntactOnCT != nil {
		cribriformIntact = *payload.CribriformPlateIntactOnCT
	}
	salineDiuresis := existing.AmphotericinLipidSalineDiuresisVerified
	if payload.AmphotericinLipidSalineDiuresisVerified != nil {
		salineDiuresis = *payload.AmphotericinLipidSalineDiuresisVerified
	}
	dieOffStaged := existing.DieOffDexamethasoneAntiInflammatoryStaged
	if payload.DieOffDexamethasoneAntiInflammatoryStaged != nil {
		dieOffStaged = *payload.DieOffDexamethasoneAntiInflammatoryStaged
	}

	azolePlan := firstNonEmpty(
		payload.SystemicAzoleAndPolyeneRegimenPlan,
		payload.AzoleRegimenPlan,
		existing.TherapyStaging.SystemicAzoleAndPolyeneRegimenPlan,
	)
	dieOffPlan := firstNonEmpty(
		payload.PulmonaryDieOffARDSPlan,
		payload.DieOffARDSPlan,
		existing.TherapyStaging.PulmonaryDieOffARDSPlan,
	)
	ophthalmicPlan := firstNonEmpty(
		payload.OphthalmicChorioretinitisNeuroPlan,
		payload.OphthalmicNeuroPlan,
		existing.TherapyStaging.OphthalmicChorioretinitisNeuroPlan,
	)

	updated, err := svc.StageTherapy(
		id,
		protocol,
		payload.ItraconazoleMgKgDay,
		payload.FluconazoleMgKgDay,
		payload.AmphotericinBLipidMgKgDose,
		compoundedBlocked,
		cribriformIntact,
		salineDiuresis,
		dieOffStaged,
		azolePlan,
		dieOffPlan,
		ophthalmicPlan,
		payload.MinimumAzoleDurationMonths,
		payload.StagedBy,
	)
	if err != nil {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextSystemicMycosesNano()).UTC()
	s.broadcastSystemicMycosesEvent("systemic_mycoses_updated", map[string]any{
		"type":      "systemic_mycoses_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordMycosesAntigenRenalCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureSystemicMycosesService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeSystemicMycosesJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordMycosesAntigenRenalCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	cribriformIntact := existing.CribriformPlateIntactOnCT
	if payload.CribriformPlateIntactOnCT != nil {
		cribriformIntact = *payload.CribriformPlateIntactOnCT
	}

	updated, err := svc.RecordAntigenRenalCheck(
		id,
		payload.MiraVistaAntigenNgML,
		payload.SerumCreatinineMgDL,
		payload.ALTActivityUL,
		payload.SpO2Percent,
		cribriformIntact,
		payload.AmphotericinDoseHeldForAzotemia,
		payload.OphthalmicChorioretinitisResolved,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextSystemicMycosesNano()).UTC()
	s.broadcastSystemicMycosesEvent("systemic_mycoses_updated", map[string]any{
		"type":      "systemic_mycoses_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleEvaluateSystemicMycosesClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureSystemicMycosesService()
	if _, err := svc.GetOperation(id); err != nil {
		writeSystemicMycosesJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload evaluateSystemicMycosesClearancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	antigenNgML := 0.0
	if payload.MiraVistaAntigenNgML != nil {
		antigenNgML = *payload.MiraVistaAntigenNgML
	}
	creatinineMgDL := 1.15
	if payload.SerumCreatinineMgDL != nil {
		creatinineMgDL = *payload.SerumCreatinineMgDL
	}
	twoNegativeVerified := true
	if payload.TwoConsecutiveNegativeEIAVerified != nil {
		twoNegativeVerified = *payload.TwoConsecutiveNegativeEIAVerified
	}
	sixMonthVerified := true
	if payload.Minimum6MonthAzoleCourseVerified != nil {
		sixMonthVerified = *payload.Minimum6MonthAzoleCourseVerified
	}
	clearedBy := firstNonEmpty(payload.ClearedBy, payload.DischargedBy)
	summary := firstNonEmpty(payload.ClearanceSummary, payload.DischargeSummary, payload.Summary)

	updated, err := svc.EvaluateClearance(
		id,
		antigenNgML,
		creatinineMgDL,
		twoNegativeVerified,
		sixMonthVerified,
		clearedBy,
		summary,
	)
	if err != nil {
		writeSystemicMycosesJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextSystemicMycosesNano()).UTC()
	s.broadcastSystemicMycosesEvent("systemic_mycoses_updated", map[string]any{
		"type":      "systemic_mycoses_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleSystemicMycosesStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	if s.reunionHub == nil {
		return
	}

	subCh, unsubscribe := s.reunionHub.Subscribe("systemic_mycoses")
	defer unsubscribe()

	pingInterval := s.reunionPingInterval
	if pingInterval <= 0 {
		pingInterval = defaultReunionPingInterval
	}
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-subCh:
			if !ok {
				return
			}
			if err := formatSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastSystemicMycosesEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextSystemicMycosesNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "systemic_mycoses",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "systemic_mycoses",
			SenderNodeID:  "systemic-mycoses-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
