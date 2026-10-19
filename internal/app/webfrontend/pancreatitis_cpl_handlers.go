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
	"github.com/scottdensmore/petspotr/pkg/pancreatitiscpl"
)

var lastPancreatitisCPLNano atomic.Int64

func nextPancreatitisCPLNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastPancreatitisCPLNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastPancreatitisCPLNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// PancreatitisCPLPageData represents template data passed to templates/pancreatitis_cpl_triage.html.
type PancreatitisCPLPageData struct {
	Operations      []domain.PancreatitisOperation
	ActiveOperation *domain.PancreatitisOperation
	Metrics         domain.PancreatitisOverviewMetrics
	Locale          string
}

type createPancreatitisOperationPayload struct {
	CaseID                       string                           `json:"case_id"`
	PetName                      string                           `json:"pet_name"`
	Species                      string                           `json:"species"`
	Breed                        string                           `json:"breed"`
	WeightKg                     float64                          `json:"weight_kg"`
	SeverityClass                domain.PancreatitisSeverityClass `json:"severity_class"`
	SpecPLUgL                    int                              `json:"spec_pl_ug_l"`
	ModifiedGlasgowPainScore     int                              `json:"modified_glasgow_pain_score"`
	EnteralRERPercent            int                              `json:"enteral_rer_percent"`
	POCUSHypoechoicPancreas      bool                             `json:"pocus_hypoechoic_pancreas"`
	HyperechoicPeripancreaticFat bool                             `json:"hyperechoic_peripancreatic_fat"`
	PrayerPositionPresent        bool                             `json:"prayer_position_present"`
	FelineTriaditisSuspected     bool                             `json:"feline_triaditis_suspected"`
}

type stagePancreatitisNutritionPayload struct {
	FuzapladibProtocol     string `json:"fuzapladib_protocol"`
	AntiemeticDualProtocol string `json:"antiemetic_dual_protocol"`
	VisceralAnalgesiaCRI   string `json:"visceral_analgesia_cri"`
	EarlyEnteralTubePlan   string `json:"early_enteral_tube_plan"`
	RERCaloricRampSchedule string `json:"rer_caloric_ramp_schedule"`
	FluidCrystalloidPlan   string `json:"fluid_crystalloid_plan"`
}

type recordPancreatitisCheckPayload struct {
	SpecPLUgL                int    `json:"spec_pl_ug_l"`
	ModifiedGlasgowPainScore int    `json:"modified_glasgow_pain_score"`
	EnteralRERPercent        int    `json:"enteral_rer_percent"`
	VomitingRegurgStatus     string `json:"vomiting_regurg_status"`
	RecordedBy               string `json:"recorded_by"`
	ClinicalNote             string `json:"clinical_note"`
}

type clearPancreatitisDischargePayload struct {
	DischargedBy string `json:"discharged_by"`
	Summary      string `json:"summary"`
}

func (s *Server) ensurePancreatitisService() *pancreatitiscpl.PancreatitisService {
	if s.pancreatitisService == nil {
		s.pancreatitisService = pancreatitiscpl.NewPancreatitisService()
	}
	return s.pancreatitisService
}

func (s *Server) registerPancreatitisCPLRoutes() {
	s.mux.HandleFunc("GET /pancreatitis-cpl-triage", s.handlePancreatitisCPLTriagePage)

	s.mux.HandleFunc("GET /api/pancreatitis-cpl/operations", s.handleListPancreatitisCPLOperations)
	s.mux.HandleFunc("POST /api/pancreatitis-cpl/operations", s.handleCreatePancreatitisCPLOperation)
	s.mux.HandleFunc("GET /api/pancreatitis-cpl/operations/{id}", s.handleGetPancreatitisCPLOperation)
	s.mux.HandleFunc("POST /api/pancreatitis-cpl/operations/{id}/stage-nutrition", s.handleStagePancreatitisCPLNutrition)
	s.mux.HandleFunc("POST /api/pancreatitis-cpl/operations/{id}/record-check", s.handleRecordPancreatitisCPLCheck)
	s.mux.HandleFunc("POST /api/pancreatitis-cpl/operations/{id}/clear-discharge", s.handleClearPancreatitisCPLDischarge)
	s.mux.HandleFunc("GET /api/pancreatitis-cpl/stream", s.handlePancreatitisCPLStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/pancreatitis-cpl/operations", s.handleListPancreatitisCPLOperations)
	s.mux.HandleFunc("POST /api/v1/pancreatitis-cpl/operations", s.handleCreatePancreatitisCPLOperation)
	s.mux.HandleFunc("GET /api/v1/pancreatitis-cpl/operations/{id}", s.handleGetPancreatitisCPLOperation)
	s.mux.HandleFunc("POST /api/v1/pancreatitis-cpl/operations/{id}/stage-nutrition", s.handleStagePancreatitisCPLNutrition)
	s.mux.HandleFunc("POST /api/v1/pancreatitis-cpl/operations/{id}/record-check", s.handleRecordPancreatitisCPLCheck)
	s.mux.HandleFunc("POST /api/v1/pancreatitis-cpl/operations/{id}/clear-discharge", s.handleClearPancreatitisCPLDischarge)
	s.mux.HandleFunc("GET /api/v1/pancreatitis-cpl/stream", s.handlePancreatitisCPLStream)
}

func writePancreatitisCPLJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handlePancreatitisCPLTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePancreatitisService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.PancreatitisOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("pancreatitis_cpl_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/pancreatitis_cpl_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Pancreatitis cPL/fPL Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := PancreatitisCPLPageData{
		Operations:      ops,
		ActiveOperation: activeOp,
		Metrics:         metrics,
		Locale:          locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListPancreatitisCPLOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePancreatitisService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))

	ops := svc.ListOperations(statusFilter)
	metrics := svc.GetOverviewMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
	})
}

func (s *Server) handleGetPancreatitisCPLOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePancreatitisService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writePancreatitisCPLJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreatePancreatitisCPLOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createPancreatitisOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensurePancreatitisService()
	created, err := svc.CreateOperation(
		payload.CaseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.SeverityClass,
		payload.SpecPLUgL,
		payload.ModifiedGlasgowPainScore,
		payload.EnteralRERPercent,
		payload.POCUSHypoechoicPancreas,
		payload.HyperechoicPeripancreaticFat,
		payload.PrayerPositionPresent,
		payload.FelineTriaditisSuspected,
	)
	if err != nil {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPancreatitisCPLNano()).UTC()
	s.broadcastPancreatitisCPLEvent("pancreatitis_cpl_updated", map[string]any{
		"type":      "pancreatitis_cpl_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStagePancreatitisCPLNutrition(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePancreatitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writePancreatitisCPLJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stagePancreatitisNutritionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.StageEnteralNutrition(
		id,
		payload.FuzapladibProtocol,
		payload.AntiemeticDualProtocol,
		payload.VisceralAnalgesiaCRI,
		payload.EarlyEnteralTubePlan,
		payload.RERCaloricRampSchedule,
		payload.FluidCrystalloidPlan,
	)
	if err != nil {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPancreatitisCPLNano()).UTC()
	s.broadcastPancreatitisCPLEvent("pancreatitis_cpl_updated", map[string]any{
		"type":      "pancreatitis_cpl_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordPancreatitisCPLCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePancreatitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writePancreatitisCPLJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordPancreatitisCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordEnteralCheck(
		id,
		payload.SpecPLUgL,
		payload.ModifiedGlasgowPainScore,
		payload.EnteralRERPercent,
		payload.VomitingRegurgStatus,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPancreatitisCPLNano()).UTC()
	s.broadcastPancreatitisCPLEvent("pancreatitis_cpl_updated", map[string]any{
		"type":      "pancreatitis_cpl_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleClearPancreatitisCPLDischarge(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePancreatitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writePancreatitisCPLJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearPancreatitisDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.ClearPancreatitisDischarge(
		id,
		payload.DischargedBy,
		payload.Summary,
	)
	if err != nil {
		writePancreatitisCPLJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPancreatitisCPLNano()).UTC()
	s.broadcastPancreatitisCPLEvent("pancreatitis_cpl_updated", map[string]any{
		"type":      "pancreatitis_cpl_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handlePancreatitisCPLStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("pancreatitis_cpl")
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

func (s *Server) broadcastPancreatitisCPLEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextPancreatitisCPLNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "pancreatitis_cpl",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "pancreatitis_cpl",
			SenderNodeID:  "pancreatitis-cpl-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
