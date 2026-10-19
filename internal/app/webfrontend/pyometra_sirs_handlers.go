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
	"github.com/scottdensmore/petspotr/pkg/pyometrasirs"
)

var lastPyometraSIRSNano atomic.Int64

func nextPyometraSIRSNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastPyometraSIRSNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastPyometraSIRSNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// PyometraSIRSPageData represents template data passed to templates/pyometra_sirs_triage.html.
type PyometraSIRSPageData struct {
	Operations                   []domain.PyometraOperation
	ActiveOperation              *domain.PyometraOperation
	Metrics                      domain.PyometraOverviewMetrics
	TotalOperations              int
	ClosedCervixSIRSTriageCount  int
	OHEStagedCount               int
	ClearedPostopOHECount        int
	ClosedCervixRuptureRiskCount int
	PeritonealEffusionAlertCount int
	Locale                       string
}

type createPyometraOperationPayload struct {
	CaseID                    string                   `json:"case_id"`
	PetName                   string                   `json:"pet_name"`
	Species                   string                   `json:"species"`
	Breed                     string                   `json:"breed"`
	WeightKg                  float64                  `json:"weight_kg"`
	CervixType                domain.CervixPatencyType `json:"cervix_type"`
	UterineHornDiameterCm     float64                  `json:"uterine_horn_diameter_cm"`
	BloodLactateMmolL         float64                  `json:"blood_lactate_mmol_l"`
	WBCCountKUl               float64                  `json:"wbc_count_k_ul"`
	BandNeutrophilPercent     float64                  `json:"band_neutrophil_percent"`
	MAPMmHg                   int                      `json:"map_mm_hg"`
	POCUSFreePeritonealFluid  bool                     `json:"pocus_free_peritoneal_fluid"`
	PolyuriaPolydipsiaPresent bool                     `json:"polyuria_polydipsia_present"`
}

type stagePyometraOHEPayload struct {
	CrystalloidResuscitationPlan string `json:"crystalloid_resuscitation_plan"`
	CrystalloidPlan              string `json:"crystalloid_plan"`
	EmpiricBactericidalAbxPlan   string `json:"empiric_bactericidal_abx_plan"`
	EmpiricAbxPlan               string `json:"empiric_abx_plan"`
	SurgicalOHEHandlingProtocol  string `json:"surgical_ohe_handling_protocol"`
	OHEHandlingProtocol          string `json:"ohe_handling_protocol"`
	AbdominalLavagePlan          string `json:"abdominal_lavage_plan"`
	LavagePlan                   string `json:"lavage_plan"`
	PerioperativeAnalgesiaPlan   string `json:"perioperative_analgesia_plan"`
	AnalgesiaPlan                string `json:"analgesia_plan"`
	PostopSepsisMonitoring       string `json:"postop_sepsis_monitoring"`
	SepsisMonitoring             string `json:"sepsis_monitoring"`
}

type recordPyometraHemodynamicCheckPayload struct {
	UterineHornDiameterCm float64 `json:"uterine_horn_diameter_cm"`
	BloodLactateMmolL     float64 `json:"blood_lactate_mmol_l"`
	WBCCountKUl           float64 `json:"wbc_count_k_ul"`
	BandNeutrophilPercent float64 `json:"band_neutrophil_percent"`
	MAPMmHg               int     `json:"map_mm_hg"`
	RecordedBy            string  `json:"recorded_by"`
	ClinicalNote          string  `json:"clinical_note"`
}

type clearPyometraDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensurePyometraService() *pyometrasirs.PyometraService {
	if s.pyometraService == nil {
		s.pyometraService = pyometrasirs.NewPyometraService()
	}
	return s.pyometraService
}

func (s *Server) registerPyometraSIRSRoutes() {
	s.mux.HandleFunc("GET /pyometra-sirs-triage", s.handlePyometraSIRSTriagePage)

	s.mux.HandleFunc("GET /api/pyometra-sirs/operations", s.handleListPyometraSIRSOperations)
	s.mux.HandleFunc("POST /api/pyometra-sirs/operations", s.handleCreatePyometraSIRSOperation)
	s.mux.HandleFunc("GET /api/pyometra-sirs/operations/{id}", s.handleGetPyometraSIRSOperation)
	s.mux.HandleFunc("POST /api/pyometra-sirs/operations/{id}/stage-ohe", s.handleStagePyometraOHE)
	s.mux.HandleFunc("POST /api/pyometra-sirs/operations/{id}/record-check", s.handleRecordPyometraHemodynamicCheck)
	s.mux.HandleFunc("POST /api/pyometra-sirs/operations/{id}/clear-discharge", s.handleClearPyometraDischarge)
	s.mux.HandleFunc("GET /api/pyometra-sirs/stream", s.handlePyometraSIRSStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/pyometra-sirs/operations", s.handleListPyometraSIRSOperations)
	s.mux.HandleFunc("POST /api/v1/pyometra-sirs/operations", s.handleCreatePyometraSIRSOperation)
	s.mux.HandleFunc("GET /api/v1/pyometra-sirs/operations/{id}", s.handleGetPyometraSIRSOperation)
	s.mux.HandleFunc("POST /api/v1/pyometra-sirs/operations/{id}/stage-ohe", s.handleStagePyometraOHE)
	s.mux.HandleFunc("POST /api/v1/pyometra-sirs/operations/{id}/record-check", s.handleRecordPyometraHemodynamicCheck)
	s.mux.HandleFunc("POST /api/v1/pyometra-sirs/operations/{id}/clear-discharge", s.handleClearPyometraDischarge)
	s.mux.HandleFunc("GET /api/v1/pyometra-sirs/stream", s.handlePyometraSIRSStream)
}

func writePyometraSIRSJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handlePyometraSIRSTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePyometraService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.PyometraOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("pyometra_sirs_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/pyometra_sirs_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Pyometra SIRS Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := PyometraSIRSPageData{
		Operations:                   ops,
		ActiveOperation:              activeOp,
		Metrics:                      metrics,
		TotalOperations:              metrics.TotalOperations,
		ClosedCervixSIRSTriageCount:  metrics.ClosedCervixSIRSTriageCount,
		OHEStagedCount:               metrics.OHEStagedCount,
		ClearedPostopOHECount:        metrics.ClearedPostopOHECount,
		ClosedCervixRuptureRiskCount: metrics.ClosedCervixRuptureRiskCount,
		PeritonealEffusionAlertCount: metrics.PeritonealEffusionAlertCount,
		Locale:                       locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListPyometraSIRSOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePyometraService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	cervixFilter := strings.TrimSpace(r.URL.Query().Get("cervix_type"))

	ops := svc.ListOperations(statusFilter)
	if cervixFilter != "" {
		filtered := make([]domain.PyometraOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.CervixType), cervixFilter) {
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

func (s *Server) handleGetPyometraSIRSOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePyometraService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writePyometraSIRSJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreatePyometraSIRSOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createPyometraOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensurePyometraService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-PYO-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.CervixType,
		payload.UterineHornDiameterCm,
		payload.BloodLactateMmolL,
		payload.WBCCountKUl,
		payload.BandNeutrophilPercent,
		payload.MAPMmHg,
		payload.POCUSFreePeritonealFluid,
		payload.PolyuriaPolydipsiaPresent,
	)
	if err != nil {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPyometraSIRSNano()).UTC()
	s.broadcastPyometraSIRSEvent("pyometra_sirs_updated", map[string]any{
		"type":      "pyometra_sirs_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStagePyometraOHE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePyometraService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writePyometraSIRSJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stagePyometraOHEPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	crystalloid := firstNonEmpty(payload.CrystalloidResuscitationPlan, payload.CrystalloidPlan, existing.OHEPlan.CrystalloidResuscitationPlan)
	empiricAbx := firstNonEmpty(payload.EmpiricBactericidalAbxPlan, payload.EmpiricAbxPlan, existing.OHEPlan.EmpiricBactericidalAbxPlan)
	oheHandling := firstNonEmpty(payload.SurgicalOHEHandlingProtocol, payload.OHEHandlingProtocol, existing.OHEPlan.SurgicalOHEHandlingProtocol)
	lavage := firstNonEmpty(payload.AbdominalLavagePlan, payload.LavagePlan, existing.OHEPlan.AbdominalLavagePlan)
	analgesia := firstNonEmpty(payload.PerioperativeAnalgesiaPlan, payload.AnalgesiaPlan, existing.OHEPlan.PerioperativeAnalgesiaPlan)
	sepsisMonitor := firstNonEmpty(payload.PostopSepsisMonitoring, payload.SepsisMonitoring, existing.OHEPlan.PostopSepsisMonitoring)

	updated, err := svc.StageOHEStabilization(id, crystalloid, empiricAbx, oheHandling, lavage, analgesia, sepsisMonitor)
	if err != nil {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPyometraSIRSNano()).UTC()
	s.broadcastPyometraSIRSEvent("pyometra_sirs_updated", map[string]any{
		"type":      "pyometra_sirs_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordPyometraHemodynamicCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePyometraService()
	if _, err := svc.GetOperation(id); err != nil {
		writePyometraSIRSJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordPyometraHemodynamicCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordHemodynamicCheck(
		id,
		payload.UterineHornDiameterCm,
		payload.BloodLactateMmolL,
		payload.WBCCountKUl,
		payload.BandNeutrophilPercent,
		payload.MAPMmHg,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPyometraSIRSNano()).UTC()
	s.broadcastPyometraSIRSEvent("pyometra_sirs_updated", map[string]any{
		"type":      "pyometra_sirs_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleClearPyometraDischarge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePyometraService()
	if _, err := svc.GetOperation(id); err != nil {
		writePyometraSIRSJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearPyometraDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearPyometraDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writePyometraSIRSJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPyometraSIRSNano()).UTC()
	s.broadcastPyometraSIRSEvent("pyometra_sirs_updated", map[string]any{
		"type":      "pyometra_sirs_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handlePyometraSIRSStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("pyometra_sirs")
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

func (s *Server) broadcastPyometraSIRSEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextPyometraSIRSNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "pyometra_sirs",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "pyometra_sirs",
			SenderNodeID:  "pyometra-sirs-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
