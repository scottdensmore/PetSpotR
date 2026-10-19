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
	"github.com/scottdensmore/petspotr/pkg/fatethrombus"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastFATEThrombusNano atomic.Int64

func nextFATEThrombusNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastFATEThrombusNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastFATEThrombusNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

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

	s.mux.HandleFunc("GET /api/fate-thrombus/operations", s.handleListFATEThrombusOperations)
	s.mux.HandleFunc("POST /api/fate-thrombus/operations", s.handleCreateFATEThrombusOperation)
	s.mux.HandleFunc("GET /api/fate-thrombus/operations/{id}", s.handleGetFATEThrombusOperation)
	s.mux.HandleFunc("POST /api/fate-thrombus/operations/{id}/stage-antithrombotic", s.handleStageFATEAntithrombotic)
	s.mux.HandleFunc("POST /api/fate-thrombus/operations/{id}/record-check", s.handleRecordFATEPerfusionCheck)
	s.mux.HandleFunc("POST /api/fate-thrombus/operations/{id}/clear-discharge", s.handleClearFATEDischarge)
	s.mux.HandleFunc("GET /api/fate-thrombus/stream", s.handleFATEThrombusStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/fate-thrombus/operations", s.handleListFATEThrombusOperations)
	s.mux.HandleFunc("POST /api/v1/fate-thrombus/operations", s.handleCreateFATEThrombusOperation)
	s.mux.HandleFunc("GET /api/v1/fate-thrombus/operations/{id}", s.handleGetFATEThrombusOperation)
	s.mux.HandleFunc("POST /api/v1/fate-thrombus/operations/{id}/stage-antithrombotic", s.handleStageFATEAntithrombotic)
	s.mux.HandleFunc("POST /api/v1/fate-thrombus/operations/{id}/record-check", s.handleRecordFATEPerfusionCheck)
	s.mux.HandleFunc("POST /api/v1/fate-thrombus/operations/{id}/clear-discharge", s.handleClearFATEDischarge)
	s.mux.HandleFunc("GET /api/v1/fate-thrombus/stream", s.handleFATEThrombusStream)
}

func writeFATEThrombusJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
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

	tmpl, err := template.New("fate_thrombus_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/fate_thrombus_triage.html")
	if err != nil {
		http.Error(w, "Failed to load FATE Thrombus Triage template: "+err.Error(), http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
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

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
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
		writeFATEThrombusJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureFATEService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeFATEThrombusJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
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
		writeFATEThrombusJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeFATEThrombusJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextFATEThrombusNano()).UTC()
	s.broadcastFATEThrombusEvent("fate_thrombus_updated", map[string]any{
		"type":      "fate_thrombus_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageFATEAntithrombotic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeFATEThrombusJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureFATEService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeFATEThrombusJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageFATEAntithromboticPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeFATEThrombusJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeFATEThrombusJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextFATEThrombusNano()).UTC()
	s.broadcastFATEThrombusEvent("fate_thrombus_updated", map[string]any{
		"type":      "fate_thrombus_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordFATEPerfusionCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeFATEThrombusJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureFATEService()
	if _, err := svc.GetOperation(id); err != nil {
		writeFATEThrombusJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordFATEPerfusionCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeFATEThrombusJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeFATEThrombusJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextFATEThrombusNano()).UTC()
	s.broadcastFATEThrombusEvent("fate_thrombus_updated", map[string]any{
		"type":      "fate_thrombus_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleClearFATEDischarge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeFATEThrombusJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureFATEService()
	if _, err := svc.GetOperation(id); err != nil {
		writeFATEThrombusJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearFATEDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeFATEThrombusJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearFATEDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeFATEThrombusJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextFATEThrombusNano()).UTC()
	s.broadcastFATEThrombusEvent("fate_thrombus_updated", map[string]any{
		"type":      "fate_thrombus_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleFATEThrombusStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("fate_thrombus")
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

func (s *Server) broadcastFATEThrombusEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextFATEThrombusNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "fate_thrombus",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "fate_thrombus",
			SenderNodeID:  "fate-thrombus-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
