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
	"github.com/scottdensmore/petspotr/pkg/hemoabdomen"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastHemoabdomenNano atomic.Int64

func nextHemoabdomenNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastHemoabdomenNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastHemoabdomenNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// HemoabdomenPageData represents template data passed to templates/hemoabdomen_autotransfusion_triage.html.
type HemoabdomenPageData struct {
	Operations                     []domain.HemoabdomenOperation
	ActiveOperation                *domain.HemoabdomenOperation
	TotalOperations                int
	HemorrhagicShockCriticalCount  int
	AutotransfusionTXAStagedCount  int
	HemostasisClearedCount         int
	ClotPoppingNeoplasiaAlertCount int
	Locale                         string
}

type createHemoabdomenOperationPayload struct {
	IncidentID                  string                            `json:"incident_id"`
	PetName                     string                            `json:"pet_name"`
	PatientProfile              *domain.HemoabdomenPatientProfile `json:"patient_profile"`
	Species                     string                            `json:"species"`
	Breed                       string                            `json:"breed"`
	WeightKg                    float64                           `json:"weight_kg"`
	AfastAFS4Score              int                               `json:"afast_afs4_score"`
	EffusionPCVPercent          float64                           `json:"effusion_pcv_percent"`
	PeripheralPCVPercent        float64                           `json:"peripheral_pcv_percent"`
	MeanArterialPressureMmHg    int                               `json:"mean_arterial_pressure_mmhg"`
	PlasmaLactateMmolL          float64                           `json:"plasma_lactate_mmol_l"`
	EffusionNonClottingVerified bool                              `json:"effusion_non_clotting_verified"`
	SuspectedNeoplasia          bool                              `json:"suspected_neoplasia"`
}

type stageHemoabdomenPlanPayload struct {
	PatientProfile              *domain.HemoabdomenPatientProfile `json:"patient_profile"`
	Species                     string                            `json:"species"`
	Breed                       string                            `json:"breed"`
	WeightKg                    float64                           `json:"weight_kg"`
	AfastAFS4Score              *int                              `json:"afast_afs4_score"`
	EffusionPCVPercent          *float64                          `json:"effusion_pcv_percent"`
	PeripheralPCVPercent        float64                           `json:"peripheral_pcv_percent"`
	MeanArterialPressureMmHg    int                               `json:"mean_arterial_pressure_mmhg"`
	PlasmaLactateMmolL          float64                           `json:"plasma_lactate_mmol_l"`
	EffusionNonClottingVerified *bool                             `json:"effusion_non_clotting_verified"`
	SuspectedNeoplasia          *bool                             `json:"suspected_neoplasia"`
}

func (s *Server) ensureHemoabdomenService() *hemoabdomen.HemoabdomenService {
	if s.hemoabdomenService == nil {
		s.hemoabdomenService = hemoabdomen.NewHemoabdomenService()
	}
	return s.hemoabdomenService
}

func (s *Server) registerHemoabdomenRoutes() {
	s.mux.HandleFunc("GET /hemoabdomen-autotransfusion-triage", s.handleHemoabdomenTriagePage)

	s.mux.HandleFunc("GET /api/hemoabdomen-autotransfusion/operations", s.handleListHemoabdomenOperations)
	s.mux.HandleFunc("POST /api/hemoabdomen-autotransfusion/operations", s.handleCreateHemoabdomenOperation)
	s.mux.HandleFunc("GET /api/hemoabdomen-autotransfusion/operations/{id}", s.handleGetHemoabdomenOperation)
	s.mux.HandleFunc("POST /api/hemoabdomen-autotransfusion/operations/{id}/stage-plan", s.handleStageHemoabdomenPlan)
	s.mux.HandleFunc("POST /api/hemoabdomen-autotransfusion/operations/{id}/hemodynamic-check", s.handleRecordHemoabdomenHemodynamicCheck)
	s.mux.HandleFunc("POST /api/hemoabdomen-autotransfusion/operations/{id}/hemostasis-clearance", s.handleIssueHemoabdomenHemostasisClearance)
	s.mux.HandleFunc("GET /api/hemoabdomen-autotransfusion/stream", s.handleHemoabdomenStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/hemoabdomen-autotransfusion/operations", s.handleListHemoabdomenOperations)
	s.mux.HandleFunc("POST /api/v1/hemoabdomen-autotransfusion/operations", s.handleCreateHemoabdomenOperation)
	s.mux.HandleFunc("GET /api/v1/hemoabdomen-autotransfusion/operations/{id}", s.handleGetHemoabdomenOperation)
	s.mux.HandleFunc("POST /api/v1/hemoabdomen-autotransfusion/operations/{id}/stage-plan", s.handleStageHemoabdomenPlan)
	s.mux.HandleFunc("POST /api/v1/hemoabdomen-autotransfusion/operations/{id}/hemodynamic-check", s.handleRecordHemoabdomenHemodynamicCheck)
	s.mux.HandleFunc("POST /api/v1/hemoabdomen-autotransfusion/operations/{id}/hemostasis-clearance", s.handleIssueHemoabdomenHemostasisClearance)
	s.mux.HandleFunc("GET /api/v1/hemoabdomen-autotransfusion/stream", s.handleHemoabdomenStream)
}

func writeHemoabdomenJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleHemoabdomenTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHemoabdomenService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.HemoabdomenOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("hemoabdomen_autotransfusion_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/hemoabdomen_autotransfusion_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Hemoabdomen Autotransfusion Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := HemoabdomenPageData{
		Operations:                     ops,
		ActiveOperation:                activeOp,
		TotalOperations:                metrics.TotalOperations,
		HemorrhagicShockCriticalCount:  metrics.HemorrhagicShockCriticalCount,
		AutotransfusionTXAStagedCount:  metrics.AutotransfusionTXAStagedCount,
		HemostasisClearedCount:         metrics.HemostasisClearedCount,
		ClotPoppingNeoplasiaAlertCount: metrics.ClotPoppingNeoplasiaAlertCount,
		Locale:                         locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListHemoabdomenOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHemoabdomenService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.HemoabdomenOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(op.PatientProfile.Species, speciesFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	metrics := svc.ComputeMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
	})
}

func (s *Server) handleGetHemoabdomenOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHemoabdomenService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeHemoabdomenJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateHemoabdomenOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createHemoabdomenOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.HemoabdomenPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.HemoabdomenPatientProfile{
			Species:                     payload.Species,
			Breed:                       payload.Breed,
			WeightKg:                    payload.WeightKg,
			AfastAFS4Score:              payload.AfastAFS4Score,
			EffusionPCVPercent:          payload.EffusionPCVPercent,
			PeripheralPCVPercent:        payload.PeripheralPCVPercent,
			MeanArterialPressureMmHg:    payload.MeanArterialPressureMmHg,
			PlasmaLactateMmolL:          payload.PlasmaLactateMmolL,
			EffusionNonClottingVerified: payload.EffusionNonClottingVerified,
			SuspectedNeoplasia:          payload.SuspectedNeoplasia,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-HA-%d", 9400+len(s.ensureHemoabdomenService().ListOperations(""))+1)
	}

	svc := s.ensureHemoabdomenService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHemoabdomenNano()).UTC()
	s.broadcastHemoabdomenEvent("hemoabdomen_updated", map[string]any{
		"type":      "hemoabdomen_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageHemoabdomenPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHemoabdomenService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeHemoabdomenJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageHemoabdomenPlanPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	profile := existing.PatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		if strings.TrimSpace(payload.Species) != "" {
			profile.Species = payload.Species
		}
		if strings.TrimSpace(payload.Breed) != "" {
			profile.Breed = payload.Breed
		}
		if payload.WeightKg != 0 {
			profile.WeightKg = payload.WeightKg
		}
		if payload.AfastAFS4Score != nil {
			profile.AfastAFS4Score = *payload.AfastAFS4Score
		}
		if payload.EffusionPCVPercent != nil {
			profile.EffusionPCVPercent = *payload.EffusionPCVPercent
		}
		if payload.PeripheralPCVPercent != 0 {
			profile.PeripheralPCVPercent = payload.PeripheralPCVPercent
		}
		if payload.MeanArterialPressureMmHg != 0 {
			profile.MeanArterialPressureMmHg = payload.MeanArterialPressureMmHg
		}
		if payload.PlasmaLactateMmolL != 0 {
			profile.PlasmaLactateMmolL = payload.PlasmaLactateMmolL
		}
		if payload.EffusionNonClottingVerified != nil {
			profile.EffusionNonClottingVerified = *payload.EffusionNonClottingVerified
		}
		if payload.SuspectedNeoplasia != nil {
			profile.SuspectedNeoplasia = *payload.SuspectedNeoplasia
		}
	}

	updated, err := svc.StageResuscitationPlan(id, profile)
	if err != nil {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHemoabdomenNano()).UTC()
	s.broadcastHemoabdomenEvent("hemoabdomen_updated", map[string]any{
		"type":      "hemoabdomen_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordHemoabdomenHemodynamicCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHemoabdomenService()
	if _, err := svc.GetOperation(id); err != nil {
		writeHemoabdomenJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.HemoabdomenHemodynamicCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordHemodynamicCheck(id, check)
	if err != nil {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHemoabdomenNano()).UTC()
	s.broadcastHemoabdomenEvent("hemoabdomen_updated", map[string]any{
		"type":      "hemoabdomen_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueHemoabdomenHemostasisClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureHemoabdomenService()
	if _, err := svc.GetOperation(id); err != nil {
		writeHemoabdomenJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.HemoabdomenHemostasisClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueHemostasisClearance(id, clearance)
	if err != nil {
		writeHemoabdomenJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextHemoabdomenNano()).UTC()
	s.broadcastHemoabdomenEvent("hemoabdomen_updated", map[string]any{
		"type":      "hemoabdomen_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleHemoabdomenStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("hemoabdomen")
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

func (s *Server) broadcastHemoabdomenEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextHemoabdomenNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "hemoabdomen",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "hemoabdomen",
			SenderNodeID:  "hemoabdomen-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
