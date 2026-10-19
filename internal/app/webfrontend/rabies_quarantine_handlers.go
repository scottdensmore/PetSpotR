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
	"github.com/scottdensmore/petspotr/pkg/rabiesquarantine"
)

var lastRabiesQuarantineNano atomic.Int64

func nextRabiesQuarantineNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastRabiesQuarantineNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastRabiesQuarantineNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// RabiesQuarantinePageData represents template data passed to templates/rabies_quarantine_triage.html.
type RabiesQuarantinePageData struct {
	Operations                              []domain.RabiesQuarantineOperation
	ActiveOperation                         *domain.RabiesQuarantineOperation
	Metrics                                 domain.RabiesQuarantineOverview
	TotalOperations                         int
	BiosecurityIsolationRCOHoldStagedCount  int
	SerialNeuroSalivaryMonitoredCount       int
	StatutoryQuarantineSurvivedClearedCount int
	TenDayDomesticBiteHoldCount             int
	FourMonthStrictUnvaccinatedCount        int
	ProtectiveRFFITTiterCount               int
	Locale                                  string
}

type createRabiesQuarantineOperationPayload struct {
	CaseID                               string                           `json:"case_id"`
	PetName                              string                           `json:"pet_name"`
	Species                              string                           `json:"species"`
	Breed                                string                           `json:"breed"`
	WeightKg                             float64                          `json:"weight_kg"`
	StatutoryCategory                    domain.RabiesStatutoryCategory   `json:"statutory_category"`
	Protocol                             domain.RabiesBiosecurityProtocol `json:"protocol"`
	RCOCaseFilingNumber                  string                           `json:"rco_case_filing_number"`
	QuarantineDayElapsed                 int                              `json:"quarantine_day_elapsed"`
	RequiredQuarantineDays               int                              `json:"required_quarantine_days"`
	RFFITTiterIUML                       float64                          `json:"rffit_titer_iu_ml"`
	NoVaccineDuring10DayBiteHoldVerified *bool                            `json:"no_vaccine_during_10day_bite_hold_verified"`
	DoubleBarrierLockedEnclosureVerified *bool                            `json:"double_barrier_locked_enclosure_verified"`
}

type stageRabiesQuarantinePayload struct {
	Protocol                             domain.RabiesBiosecurityProtocol `json:"protocol"`
	RCOCaseFilingNumber                  string                           `json:"rco_case_filing_number"`
	RequiredQuarantineDays               int                              `json:"required_quarantine_days"`
	NoVaccineDuring10DayBiteHoldVerified *bool                            `json:"no_vaccine_during_10day_bite_hold_verified"`
	DoubleBarrierLockedEnclosureVerified *bool                            `json:"double_barrier_locked_enclosure_verified"`
	DoubleBarrierBiosecurityPPEPlan      string                           `json:"double_barrier_biosecurity_ppe_plan"`
	BiosecurityPPEPlan                   string                           `json:"biosecurity_ppe_plan"`
	VirucidalWoundLavageBoosterPEPPlan   string                           `json:"virucidal_wound_lavage_booster_pep_plan"`
	WoundLavagePEPPlan                   string                           `json:"wound_lavage_pep_plan"`
	ProspectiveSerologicRFFITPlan        string                           `json:"prospective_serologic_rffit_plan"`
	RFFITPlan                            string                           `json:"rffit_plan"`
	BrainStemHippocampusPreservedGuard   *bool                            `json:"brain_stem_hippocampus_preserved_guard"`
	DFANeuropathologyPreservationNote    string                           `json:"dfa_neuropathology_preservation_note"`
	DFAPreservationNote                  string                           `json:"dfa_preservation_note"`
	StagedBy                             string                           `json:"staged_by"`
}

type recordRabiesNeuroCheckPayload struct {
	QuarantineDayElapsed                 int     `json:"quarantine_day_elapsed"`
	RFFITTiterIUML                       float64 `json:"rffit_titer_iu_ml"`
	CranialNerveBulbarPalsyAbsent        *bool   `json:"cranial_nerve_bulbar_palsy_absent"`
	HypersalivationDysphagiaAbsent       *bool   `json:"hypersalivation_dysphagia_absent"`
	DoubleBarrierLockedEnclosureVerified *bool   `json:"double_barrier_locked_enclosure_verified"`
	NoVaccineDuring10DayBiteHoldVerified *bool   `json:"no_vaccine_during_10day_bite_hold_verified"`
	RecordedBy                           string  `json:"recorded_by"`
	ClinicalNote                         string  `json:"clinical_note"`
}

type evaluateRabiesClearancePayload struct {
	QuarantineDayElapsed          *int     `json:"quarantine_day_elapsed"`
	CranialNerveBulbarPalsyAbsent *bool    `json:"cranial_nerve_bulbar_palsy_absent"`
	RCOWrittenReleaseSigned       *bool    `json:"rco_written_release_signed"`
	ExitRabiesVaccinationVerified *bool    `json:"exit_rabies_vaccination_verified"`
	RFFITTiterIUML                *float64 `json:"rffit_titer_iu_ml"`
	ClearedBy                     string   `json:"cleared_by"`
	DischargedBy                  string   `json:"discharged_by"`
	ClearanceSummary              string   `json:"clearance_summary"`
	DischargeSummary              string   `json:"discharge_summary"`
	Summary                       string   `json:"summary"`
}

func (s *Server) ensureRabiesQuarantineService() *rabiesquarantine.RabiesQuarantineService {
	if s.rabiesQuarantineService == nil {
		s.rabiesQuarantineService = rabiesquarantine.NewRabiesQuarantineService()
	}
	return s.rabiesQuarantineService
}

func (s *Server) registerRabiesQuarantineRoutes() {
	s.mux.HandleFunc("GET /rabies-quarantine-pep-triage", s.handleRabiesQuarantineTriagePage)

	s.mux.HandleFunc("GET /api/rabies-quarantine/overview", s.handleRabiesQuarantineOverview)
	s.mux.HandleFunc("GET /api/rabies-quarantine/operations", s.handleListRabiesQuarantineOperations)
	s.mux.HandleFunc("POST /api/rabies-quarantine/operations", s.handleCreateRabiesQuarantineOperation)
	s.mux.HandleFunc("GET /api/rabies-quarantine/operations/{id}", s.handleGetRabiesQuarantineOperation)
	s.mux.HandleFunc("POST /api/rabies-quarantine/operations/{id}/staging", s.handleStageRabiesQuarantine)
	s.mux.HandleFunc("POST /api/rabies-quarantine/operations/{id}/neuro-check", s.handleRecordRabiesNeuroCheck)
	s.mux.HandleFunc("POST /api/rabies-quarantine/operations/{id}/clearance", s.handleEvaluateRabiesClearance)
	s.mux.HandleFunc("GET /api/rabies-quarantine/stream", s.handleRabiesQuarantineStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/rabies-quarantine/overview", s.handleRabiesQuarantineOverview)
	s.mux.HandleFunc("GET /api/v1/rabies-quarantine/operations", s.handleListRabiesQuarantineOperations)
	s.mux.HandleFunc("POST /api/v1/rabies-quarantine/operations", s.handleCreateRabiesQuarantineOperation)
	s.mux.HandleFunc("GET /api/v1/rabies-quarantine/operations/{id}", s.handleGetRabiesQuarantineOperation)
	s.mux.HandleFunc("POST /api/v1/rabies-quarantine/operations/{id}/staging", s.handleStageRabiesQuarantine)
	s.mux.HandleFunc("POST /api/v1/rabies-quarantine/operations/{id}/neuro-check", s.handleRecordRabiesNeuroCheck)
	s.mux.HandleFunc("POST /api/v1/rabies-quarantine/operations/{id}/clearance", s.handleEvaluateRabiesClearance)
	s.mux.HandleFunc("GET /api/v1/rabies-quarantine/stream", s.handleRabiesQuarantineStream)
}

func writeRabiesQuarantineJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleRabiesQuarantineTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureRabiesQuarantineService()
	ops := svc.ListOperations("")
	metrics := svc.CalculateOverview()

	var activeOp *domain.RabiesQuarantineOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("rabies_quarantine_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/rabies_quarantine_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Rabies Quarantine Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := RabiesQuarantinePageData{
		Operations:                              ops,
		ActiveOperation:                         activeOp,
		Metrics:                                 metrics,
		TotalOperations:                         metrics.TotalOperations,
		BiosecurityIsolationRCOHoldStagedCount:  metrics.BiosecurityIsolationRCOHoldStagedCount,
		SerialNeuroSalivaryMonitoredCount:       metrics.SerialNeuroSalivaryMonitoredCount,
		StatutoryQuarantineSurvivedClearedCount: metrics.StatutoryQuarantineSurvivedClearedCount,
		TenDayDomesticBiteHoldCount:             metrics.TenDayDomesticBiteHoldCount,
		FourMonthStrictUnvaccinatedCount:        metrics.FourMonthStrictUnvaccinatedCount,
		ProtectiveRFFITTiterCount:               metrics.ProtectiveRFFITTiterCount,
		Locale:                                  locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleRabiesQuarantineOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureRabiesQuarantineService()
	overview := svc.CalculateOverview()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(overview)
}

func (s *Server) handleListRabiesQuarantineOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureRabiesQuarantineService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	categoryFilter := strings.TrimSpace(r.URL.Query().Get("statutory_category"))

	ops := svc.ListOperations(statusFilter)
	if categoryFilter != "" {
		filtered := make([]domain.RabiesQuarantineOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.StatutoryCategory), categoryFilter) {
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

func (s *Server) handleGetRabiesQuarantineOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureRabiesQuarantineService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateRabiesQuarantineOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createRabiesQuarantineOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureRabiesQuarantineService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-RBQ-%d", 100+len(svc.ListOperations(""))+1)
	}
	rcoFiling := strings.TrimSpace(payload.RCOCaseFilingNumber)
	if rcoFiling == "" {
		rcoFiling = fmt.Sprintf("RCO-STAT-2026-%d", 100+len(svc.ListOperations(""))+1)
	}
	noVaccineHold := true
	if payload.NoVaccineDuring10DayBiteHoldVerified != nil {
		noVaccineHold = *payload.NoVaccineDuring10DayBiteHoldVerified
	}
	doubleBarrier := true
	if payload.DoubleBarrierLockedEnclosureVerified != nil {
		doubleBarrier = *payload.DoubleBarrierLockedEnclosureVerified
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.StatutoryCategory,
		payload.Protocol,
		rcoFiling,
		payload.QuarantineDayElapsed,
		payload.RequiredQuarantineDays,
		payload.RFFITTiterIUML,
		noVaccineHold,
		doubleBarrier,
	)
	if err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextRabiesQuarantineNano()).UTC()
	s.broadcastRabiesQuarantineEvent("rabies_quarantine_updated", map[string]any{
		"type":      "rabies_quarantine_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageRabiesQuarantine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureRabiesQuarantineService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageRabiesQuarantinePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	protocol := payload.Protocol
	if protocol == "" {
		protocol = existing.Protocol
	}
	rcoFiling := firstNonEmpty(payload.RCOCaseFilingNumber, existing.RCOCaseFilingNumber)
	reqDays := payload.RequiredQuarantineDays
	if reqDays == 0 {
		reqDays = existing.RequiredQuarantineDays
	}
	noVaccineHold := existing.NoVaccineDuring10DayBiteHoldVerified
	if payload.NoVaccineDuring10DayBiteHoldVerified != nil {
		noVaccineHold = *payload.NoVaccineDuring10DayBiteHoldVerified
	}
	doubleBarrier := existing.DoubleBarrierLockedEnclosureVerified
	if payload.DoubleBarrierLockedEnclosureVerified != nil {
		doubleBarrier = *payload.DoubleBarrierLockedEnclosureVerified
	}
	ppePlan := firstNonEmpty(
		payload.DoubleBarrierBiosecurityPPEPlan,
		payload.BiosecurityPPEPlan,
		existing.QuarantineStaging.DoubleBarrierBiosecurityPPEPlan,
	)
	lavagePlan := firstNonEmpty(
		payload.VirucidalWoundLavageBoosterPEPPlan,
		payload.WoundLavagePEPPlan,
		existing.QuarantineStaging.VirucidalWoundLavageBoosterPEPPlan,
	)
	rffitPlan := firstNonEmpty(
		payload.ProspectiveSerologicRFFITPlan,
		payload.RFFITPlan,
		existing.QuarantineStaging.ProspectiveSerologicRFFITPlan,
	)
	brainStemGuard := existing.BrainStemHippocampusPreservedGuard
	if payload.BrainStemHippocampusPreservedGuard != nil {
		brainStemGuard = *payload.BrainStemHippocampusPreservedGuard
	}
	dfaNote := firstNonEmpty(
		payload.DFANeuropathologyPreservationNote,
		payload.DFAPreservationNote,
		existing.QuarantineStaging.DFANeuropathologyPreservationNote,
	)

	updated, err := svc.StageQuarantine(
		id,
		protocol,
		rcoFiling,
		reqDays,
		noVaccineHold,
		doubleBarrier,
		ppePlan,
		lavagePlan,
		rffitPlan,
		brainStemGuard,
		dfaNote,
		payload.StagedBy,
	)
	if err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextRabiesQuarantineNano()).UTC()
	s.broadcastRabiesQuarantineEvent("rabies_quarantine_updated", map[string]any{
		"type":      "rabies_quarantine_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordRabiesNeuroCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureRabiesQuarantineService()
	if _, err := svc.GetOperation(id); err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordRabiesNeuroCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	bulbarAbsent := true
	if payload.CranialNerveBulbarPalsyAbsent != nil {
		bulbarAbsent = *payload.CranialNerveBulbarPalsyAbsent
	}
	dysphagiaAbsent := true
	if payload.HypersalivationDysphagiaAbsent != nil {
		dysphagiaAbsent = *payload.HypersalivationDysphagiaAbsent
	}
	doubleBarrier := true
	if payload.DoubleBarrierLockedEnclosureVerified != nil {
		doubleBarrier = *payload.DoubleBarrierLockedEnclosureVerified
	}
	noVaccineHold := true
	if payload.NoVaccineDuring10DayBiteHoldVerified != nil {
		noVaccineHold = *payload.NoVaccineDuring10DayBiteHoldVerified
	}

	updated, err := svc.RecordNeuroObservationCheck(
		id,
		payload.QuarantineDayElapsed,
		payload.RFFITTiterIUML,
		bulbarAbsent,
		dysphagiaAbsent,
		doubleBarrier,
		noVaccineHold,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextRabiesQuarantineNano()).UTC()
	s.broadcastRabiesQuarantineEvent("rabies_quarantine_updated", map[string]any{
		"type":      "rabies_quarantine_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleEvaluateRabiesClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureRabiesQuarantineService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload evaluateRabiesClearancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	dayElapsed := existing.RequiredQuarantineDays
	if payload.QuarantineDayElapsed != nil {
		dayElapsed = *payload.QuarantineDayElapsed
	}
	bulbarAbsent := true
	if payload.CranialNerveBulbarPalsyAbsent != nil {
		bulbarAbsent = *payload.CranialNerveBulbarPalsyAbsent
	}
	rcoReleaseSigned := true
	if payload.RCOWrittenReleaseSigned != nil {
		rcoReleaseSigned = *payload.RCOWrittenReleaseSigned
	}
	exitVaccineVerified := true
	if payload.ExitRabiesVaccinationVerified != nil {
		exitVaccineVerified = *payload.ExitRabiesVaccinationVerified
	}
	rffitTiter := existing.RFFITTiterIUML
	if payload.RFFITTiterIUML != nil {
		rffitTiter = *payload.RFFITTiterIUML
	}
	clearedBy := firstNonEmpty(payload.ClearedBy, payload.DischargedBy)
	summary := firstNonEmpty(payload.ClearanceSummary, payload.DischargeSummary, payload.Summary)

	updated, err := svc.EvaluateClearance(
		id,
		dayElapsed,
		bulbarAbsent,
		rcoReleaseSigned,
		exitVaccineVerified,
		rffitTiter,
		clearedBy,
		summary,
	)
	if err != nil {
		writeRabiesQuarantineJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextRabiesQuarantineNano()).UTC()
	s.broadcastRabiesQuarantineEvent("rabies_quarantine_updated", map[string]any{
		"type":      "rabies_quarantine_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRabiesQuarantineStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("rabies_quarantine")
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

func (s *Server) broadcastRabiesQuarantineEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextRabiesQuarantineNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "rabies_quarantine",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "rabies_quarantine",
			SenderNodeID:  "rabies-quarantine-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
