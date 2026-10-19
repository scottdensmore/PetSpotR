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

	"github.com/scottdensmore/petspotr/pkg/airwaytracheostomy"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastAirwayTracheostomyNano atomic.Int64

func nextAirwayTracheostomyNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastAirwayTracheostomyNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastAirwayTracheostomyNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// AirwayTracheostomyPageData represents template data passed to templates/airway_tracheostomy_triage.html.
type AirwayTracheostomyPageData struct {
	Operations      []domain.AirwayTracheostomyOperation
	ActiveOperation *domain.AirwayTracheostomyOperation
	Metrics         airwaytracheostomy.AirwayCockpitMetrics
	Locale          string
}

type createAirwayOperationRequest struct {
	IncidentID           string                       `json:"incident_id"`
	PetName              string                       `json:"pet_name"`
	PatientProfile       *domain.AirwayPatientProfile `json:"patient_profile,omitempty"`
	Species              string                       `json:"species"`
	Breed                string                       `json:"breed"`
	WeightKg             float64                      `json:"weight_kg"`
	RectalTempFahrenheit float64                      `json:"rectal_temp_fahrenheit"`
	SpO2Percent          float64                      `json:"spo2_percent"`
	InspiratoryEffort    string                       `json:"inspiratory_effort"`
	LaryngealEdemaGrade  string                       `json:"laryngeal_edema_grade"`
	GagReflexPresent     bool                         `json:"gag_reflex_present"`
}

func (r createAirwayOperationRequest) resolveProfile() domain.AirwayPatientProfile {
	if r.PatientProfile != nil {
		return *r.PatientProfile
	}
	return domain.AirwayPatientProfile{
		Species:              r.Species,
		Breed:                r.Breed,
		WeightKg:             r.WeightKg,
		RectalTempFahrenheit: r.RectalTempFahrenheit,
		SpO2Percent:          r.SpO2Percent,
		InspiratoryEffort:    r.InspiratoryEffort,
		LaryngealEdemaGrade:  r.LaryngealEdemaGrade,
		GagReflexPresent:     r.GagReflexPresent,
	}
}

type stageStabilizationRequest struct {
	PatientProfile       *domain.AirwayPatientProfile `json:"patient_profile,omitempty"`
	Species              string                       `json:"species"`
	Breed                string                       `json:"breed"`
	WeightKg             float64                      `json:"weight_kg"`
	RectalTempFahrenheit float64                      `json:"rectal_temp_fahrenheit"`
	SpO2Percent          float64                      `json:"spo2_percent"`
	InspiratoryEffort    string                       `json:"inspiratory_effort"`
	LaryngealEdemaGrade  string                       `json:"laryngeal_edema_grade"`
	GagReflexPresent     *bool                        `json:"gag_reflex_present,omitempty"`
}

func (s *Server) ensureAirwayService() *airwaytracheostomy.AirwayService {
	if s.airwayService == nil {
		s.airwayService = airwaytracheostomy.NewAirwayService()
	}
	return s.airwayService
}

func (s *Server) registerAirwayTracheostomyRoutes() {
	s.mux.HandleFunc("GET /airway-tracheostomy-triage", s.handleAirwayTracheostomyTriagePage)

	s.mux.HandleFunc("GET /api/airway-tracheostomy/operations", s.handleListAirwayTracheostomyOperations)
	s.mux.HandleFunc("POST /api/airway-tracheostomy/operations", s.handleCreateAirwayTracheostomyOperation)
	s.mux.HandleFunc("GET /api/airway-tracheostomy/operations/{id}", s.handleGetAirwayTracheostomyOperation)
	s.mux.HandleFunc("POST /api/airway-tracheostomy/operations/{id}/stage-stabilization", s.handleStageAirwayStabilization)
	s.mux.HandleFunc("POST /api/airway-tracheostomy/operations/{id}/trach-care-log", s.handleRecordTracheostomyCareLog)
	s.mux.HandleFunc("POST /api/airway-tracheostomy/operations/{id}/extubation-clearance", s.handleIssueAirwayExtubationClearance)
	s.mux.HandleFunc("GET /api/airway-tracheostomy/stream", s.handleAirwayTracheostomyStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/airway-tracheostomy/operations", s.handleListAirwayTracheostomyOperations)
	s.mux.HandleFunc("POST /api/v1/airway-tracheostomy/operations", s.handleCreateAirwayTracheostomyOperation)
	s.mux.HandleFunc("GET /api/v1/airway-tracheostomy/operations/{id}", s.handleGetAirwayTracheostomyOperation)
	s.mux.HandleFunc("POST /api/v1/airway-tracheostomy/operations/{id}/stage-stabilization", s.handleStageAirwayStabilization)
	s.mux.HandleFunc("POST /api/v1/airway-tracheostomy/operations/{id}/trach-care-log", s.handleRecordTracheostomyCareLog)
	s.mux.HandleFunc("POST /api/v1/airway-tracheostomy/operations/{id}/extubation-clearance", s.handleIssueAirwayExtubationClearance)
	s.mux.HandleFunc("GET /api/v1/airway-tracheostomy/stream", s.handleAirwayTracheostomyStream)
}

func writeAirwayTracheostomyJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleAirwayTracheostomyTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAirwayService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.AirwayTracheostomyOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("airway_tracheostomy_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/airway_tracheostomy_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis & Temporary Tracheostomy Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := AirwayTracheostomyPageData{
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

func (s *Server) handleListAirwayTracheostomyOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAirwayService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.AirwayTracheostomyOperation, 0, len(ops))
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

func (s *Server) handleGetAirwayTracheostomyOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAirwayService()
	op, err := svc.GetOperation(opID)
	if err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateAirwayTracheostomyOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createAirwayOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureAirwayService()
	created, err := svc.CreateOperation(req.IncidentID, req.PetName, req.resolveProfile())
	if err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAirwayTracheostomyNano()).UTC()
	s.broadcastAirwayTracheostomyEvent("airway_tracheostomy_updated", map[string]any{
		"type":      "airway_tracheostomy_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageAirwayStabilization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAirwayService()
	existing, err := svc.GetOperation(opID)
	if err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req stageStabilizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	profile := existing.PatientProfile
	if req.PatientProfile != nil {
		profile = *req.PatientProfile
	} else {
		if strings.TrimSpace(req.Species) != "" {
			profile.Species = req.Species
		}
		if strings.TrimSpace(req.Breed) != "" {
			profile.Breed = req.Breed
		}
		if req.WeightKg != 0 {
			profile.WeightKg = req.WeightKg
		}
		if req.RectalTempFahrenheit != 0 {
			profile.RectalTempFahrenheit = req.RectalTempFahrenheit
		}
		if req.SpO2Percent != 0 {
			profile.SpO2Percent = req.SpO2Percent
		}
		if strings.TrimSpace(req.InspiratoryEffort) != "" {
			profile.InspiratoryEffort = req.InspiratoryEffort
		}
		if strings.TrimSpace(req.LaryngealEdemaGrade) != "" {
			profile.LaryngealEdemaGrade = req.LaryngealEdemaGrade
		}
		if req.GagReflexPresent != nil {
			profile.GagReflexPresent = *req.GagReflexPresent
		}
	}

	updated, err := svc.StageStabilizationPlan(opID, profile)
	if err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAirwayTracheostomyNano()).UTC()
	s.broadcastAirwayTracheostomyEvent("airway_tracheostomy_updated", map[string]any{
		"type":      "airway_tracheostomy_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordTracheostomyCareLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAirwayService()
	if _, err := svc.GetOperation(opID); err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var logEntry domain.TracheostomyCareLog
	if err := json.NewDecoder(r.Body).Decode(&logEntry); err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordTrachCareLog(opID, logEntry)
	if err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAirwayTracheostomyNano()).UTC()
	s.broadcastAirwayTracheostomyEvent("airway_tracheostomy_updated", map[string]any{
		"type":      "airway_tracheostomy_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueAirwayExtubationClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAirwayService()
	if _, err := svc.GetOperation(opID); err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.AirwayExtubationClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueExtubationClearance(opID, clearance)
	if err != nil {
		writeAirwayTracheostomyJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAirwayTracheostomyNano()).UTC()
	s.broadcastAirwayTracheostomyEvent("airway_tracheostomy_updated", map[string]any{
		"type":      "airway_tracheostomy_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleAirwayTracheostomyStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("airway_tracheostomy")
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

func (s *Server) broadcastAirwayTracheostomyEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextAirwayTracheostomyNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "airway_tracheostomy",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "airway_tracheostomy",
			SenderNodeID:  "airway-tracheostomy-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
