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
	"github.com/scottdensmore/petspotr/pkg/heartwormcaval"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastHeartwormCavalNano atomic.Int64

func nextHeartwormCavalNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastHeartwormCavalNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastHeartwormCavalNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

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

	s.mux.HandleFunc("GET /api/heartworm-caval/overview", s.handleHeartwormCavalOverview)
	s.mux.HandleFunc("GET /api/heartworm-caval/operations", s.handleListHeartwormCavalOperations)
	s.mux.HandleFunc("POST /api/heartworm-caval/operations", s.handleCreateHeartwormCavalOperation)
	s.mux.HandleFunc("GET /api/heartworm-caval/operations/{id}", s.handleGetHeartwormCavalOperation)
	s.mux.HandleFunc("POST /api/heartworm-caval/operations/{id}/intervention", s.handleStageHeartwormIntervention)
	s.mux.HandleFunc("POST /api/heartworm-caval/operations/{id}/hemodynamic-check", s.handleRecordHeartwormHemodynamicCheck)
	s.mux.HandleFunc("POST /api/heartworm-caval/operations/{id}/clearance", s.handleEvaluateHeartwormClearance)
	s.mux.HandleFunc("GET /api/heartworm-caval/stream", s.handleHeartwormCavalStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/heartworm-caval/overview", s.handleHeartwormCavalOverview)
	s.mux.HandleFunc("GET /api/v1/heartworm-caval/operations", s.handleListHeartwormCavalOperations)
	s.mux.HandleFunc("POST /api/v1/heartworm-caval/operations", s.handleCreateHeartwormCavalOperation)
	s.mux.HandleFunc("GET /api/v1/heartworm-caval/operations/{id}", s.handleGetHeartwormCavalOperation)
	s.mux.HandleFunc("POST /api/v1/heartworm-caval/operations/{id}/intervention", s.handleStageHeartwormIntervention)
	s.mux.HandleFunc("POST /api/v1/heartworm-caval/operations/{id}/hemodynamic-check", s.handleRecordHeartwormHemodynamicCheck)
	s.mux.HandleFunc("POST /api/v1/heartworm-caval/operations/{id}/clearance", s.handleEvaluateHeartwormClearance)
	s.mux.HandleFunc("GET /api/v1/heartworm-caval/stream", s.handleHeartwormCavalStream)
}

func writeHeartwormCavalJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
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

	tmpl, err := template.New("heartworm_caval_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/heartworm_caval_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Heartworm Caval Triage template: "+err.Error(), http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleHeartwormCavalOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHeartwormService()
	overview := svc.CalculateOverview()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(overview)
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
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
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHeartwormService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeHeartwormCavalJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
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
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHeartwormCavalNano()).UTC()
	s.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{
		"type":      "heartworm_caval_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageHeartwormIntervention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHeartwormService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeHeartwormCavalJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageHeartwormInterventionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHeartwormCavalNano()).UTC()
	s.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{
		"type":      "heartworm_caval_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordHeartwormHemodynamicCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHeartwormService()
	if _, err := svc.GetOperation(id); err != nil {
		writeHeartwormCavalJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordHeartwormHemodynamicCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHeartwormCavalNano()).UTC()
	s.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{
		"type":      "heartworm_caval_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleEvaluateHeartwormClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHeartwormService()
	if _, err := svc.GetOperation(id); err != nil {
		writeHeartwormCavalJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload evaluateHeartwormClearancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeHeartwormCavalJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHeartwormCavalNano()).UTC()
	s.broadcastHeartwormCavalEvent("heartworm_caval_updated", map[string]any{
		"type":      "heartworm_caval_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleHeartwormCavalStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("heartworm_caval")
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

func (s *Server) broadcastHeartwormCavalEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextHeartwormCavalNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "heartworm_caval",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "heartworm_caval",
			SenderNodeID:  "heartworm-caval-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
