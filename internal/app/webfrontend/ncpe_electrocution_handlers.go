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
	"github.com/scottdensmore/petspotr/pkg/ncpeelectrocution"
)

var lastNCPEElectrocutionNano atomic.Int64

func nextNCPEElectrocutionNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastNCPEElectrocutionNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastNCPEElectrocutionNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

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

	s.mux.HandleFunc("GET /api/ncpe-electrocution/operations", s.handleListNCPEElectrocutionOperations)
	s.mux.HandleFunc("POST /api/ncpe-electrocution/operations", s.handleCreateNCPEElectrocutionOperation)
	s.mux.HandleFunc("GET /api/ncpe-electrocution/operations/{id}", s.handleGetNCPEElectrocutionOperation)
	s.mux.HandleFunc("POST /api/ncpe-electrocution/operations/{id}/stage-cpap", s.handleStageNCPECPAPVentilation)
	s.mux.HandleFunc("POST /api/ncpe-electrocution/operations/{id}/record-check", s.handleRecordNCPERespiratoryCheck)
	s.mux.HandleFunc("POST /api/ncpe-electrocution/operations/{id}/clear-discharge", s.handleClearNCPEElectrocutionDischarge)
	s.mux.HandleFunc("GET /api/ncpe-electrocution/stream", s.handleNCPEElectrocutionStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/ncpe-electrocution/operations", s.handleListNCPEElectrocutionOperations)
	s.mux.HandleFunc("POST /api/v1/ncpe-electrocution/operations", s.handleCreateNCPEElectrocutionOperation)
	s.mux.HandleFunc("GET /api/v1/ncpe-electrocution/operations/{id}", s.handleGetNCPEElectrocutionOperation)
	s.mux.HandleFunc("POST /api/v1/ncpe-electrocution/operations/{id}/stage-cpap", s.handleStageNCPECPAPVentilation)
	s.mux.HandleFunc("POST /api/v1/ncpe-electrocution/operations/{id}/record-check", s.handleRecordNCPERespiratoryCheck)
	s.mux.HandleFunc("POST /api/v1/ncpe-electrocution/operations/{id}/clear-discharge", s.handleClearNCPEElectrocutionDischarge)
	s.mux.HandleFunc("GET /api/v1/ncpe-electrocution/stream", s.handleNCPEElectrocutionStream)
}

func writeNCPEElectrocutionJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
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

	tmpl, err := template.New("ncpe_electrocution_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/ncpe_electrocution_triage.html")
	if err != nil {
		http.Error(w, "Failed to load NCPE Electrocution Triage template: "+err.Error(), http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
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
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureNCPEService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeNCPEElectrocutionJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
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
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextNCPEElectrocutionNano()).UTC()
	s.broadcastNCPEElectrocutionEvent("ncpe_electrocution_updated", map[string]any{
		"type":      "ncpe_electrocution_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageNCPECPAPVentilation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureNCPEService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeNCPEElectrocutionJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageNCPECPAPVentilationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextNCPEElectrocutionNano()).UTC()
	s.broadcastNCPEElectrocutionEvent("ncpe_electrocution_updated", map[string]any{
		"type":      "ncpe_electrocution_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordNCPERespiratoryCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureNCPEService()
	if _, err := svc.GetOperation(id); err != nil {
		writeNCPEElectrocutionJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordNCPERespiratoryCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextNCPEElectrocutionNano()).UTC()
	s.broadcastNCPEElectrocutionEvent("ncpe_electrocution_updated", map[string]any{
		"type":      "ncpe_electrocution_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleClearNCPEElectrocutionDischarge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureNCPEService()
	if _, err := svc.GetOperation(id); err != nil {
		writeNCPEElectrocutionJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearNCPEElectrocutionDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearNCPEDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeNCPEElectrocutionJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextNCPEElectrocutionNano()).UTC()
	s.broadcastNCPEElectrocutionEvent("ncpe_electrocution_updated", map[string]any{
		"type":      "ncpe_electrocution_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleNCPEElectrocutionStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("ncpe_electrocution")
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

func (s *Server) broadcastNCPEElectrocutionEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextNCPEElectrocutionNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "ncpe_electrocution",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "ncpe_electrocution",
			SenderNodeID:  "ncpe-electrocution-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
