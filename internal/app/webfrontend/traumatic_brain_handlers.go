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
	"github.com/scottdensmore/petspotr/pkg/traumaticbrain"
)

var lastTraumaticBrainNano atomic.Int64

func nextTraumaticBrainNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastTraumaticBrainNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastTraumaticBrainNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// TraumaticBrainPageData represents template data passed to templates/traumatic_brain_triage.html.
type TraumaticBrainPageData struct {
	Operations                []domain.TraumaticBrainOperation
	ActiveOperation           *domain.TraumaticBrainOperation
	TotalOperations           int
	CushingHerniationCount    int
	HypertonicMannitolStaged  int
	MGCSRecoveredClearedCount int
	CushingSteroidAlertCount  int
	Locale                    string
}

type createTBIOperationPayload struct {
	IncidentID               string                    `json:"incident_id"`
	PetName                  string                    `json:"pet_name"`
	PatientProfile           *domain.TBIPatientProfile `json:"patient_profile"`
	Species                  string                    `json:"species"`
	Breed                    string                    `json:"breed"`
	WeightKg                 float64                   `json:"weight_kg"`
	ModifiedGlasgowComaScore int                       `json:"modified_glasgow_coma_score"`
	MeanArterialPressureMmHg int                       `json:"mean_arterial_pressure_mmhg"`
	HeartRateBpm             int                       `json:"heart_rate_bpm"`
	EndTidalCO2MmHg          float64                   `json:"end_tidal_co2_mmhg"`
	PupillaryStatus          string                    `json:"pupillary_status"`
	CushingReflexPresent     bool                      `json:"cushing_reflex_present"`
}

type stageTBIPlanPayload struct {
	PatientProfile           *domain.TBIPatientProfile `json:"patient_profile"`
	Species                  string                    `json:"species"`
	Breed                    string                    `json:"breed"`
	WeightKg                 float64                   `json:"weight_kg"`
	ModifiedGlasgowComaScore int                       `json:"modified_glasgow_coma_score"`
	MeanArterialPressureMmHg int                       `json:"mean_arterial_pressure_mmhg"`
	HeartRateBpm             int                       `json:"heart_rate_bpm"`
	EndTidalCO2MmHg          float64                   `json:"end_tidal_co2_mmhg"`
	PupillaryStatus          string                    `json:"pupillary_status"`
	CushingReflexPresent     *bool                     `json:"cushing_reflex_present"`
}

func (s *Server) ensureTBIService() *traumaticbrain.TBIService {
	if s.tbiService == nil {
		s.tbiService = traumaticbrain.NewTBIService()
	}
	return s.tbiService
}

func (s *Server) registerTraumaticBrainRoutes() {
	s.mux.HandleFunc("GET /traumatic-brain-triage", s.handleTraumaticBrainTriagePage)

	s.mux.HandleFunc("GET /api/traumatic-brain/operations", s.handleListTraumaticBrainOperations)
	s.mux.HandleFunc("POST /api/traumatic-brain/operations", s.handleCreateTraumaticBrainOperation)
	s.mux.HandleFunc("GET /api/traumatic-brain/operations/{id}", s.handleGetTraumaticBrainOperation)
	s.mux.HandleFunc("POST /api/traumatic-brain/operations/{id}/stage-plan", s.handleStageTraumaticBrainPlan)
	s.mux.HandleFunc("POST /api/traumatic-brain/operations/{id}/neuro-check", s.handleRecordTraumaticBrainCheck)
	s.mux.HandleFunc("POST /api/traumatic-brain/operations/{id}/discharge-clearance", s.handleIssueTraumaticBrainDischargeClearance)
	s.mux.HandleFunc("GET /api/traumatic-brain/stream", s.handleTraumaticBrainStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/traumatic-brain/operations", s.handleListTraumaticBrainOperations)
	s.mux.HandleFunc("POST /api/v1/traumatic-brain/operations", s.handleCreateTraumaticBrainOperation)
	s.mux.HandleFunc("GET /api/v1/traumatic-brain/operations/{id}", s.handleGetTraumaticBrainOperation)
	s.mux.HandleFunc("POST /api/v1/traumatic-brain/operations/{id}/stage-plan", s.handleStageTraumaticBrainPlan)
	s.mux.HandleFunc("POST /api/v1/traumatic-brain/operations/{id}/neuro-check", s.handleRecordTraumaticBrainCheck)
	s.mux.HandleFunc("POST /api/v1/traumatic-brain/operations/{id}/discharge-clearance", s.handleIssueTraumaticBrainDischargeClearance)
	s.mux.HandleFunc("GET /api/v1/traumatic-brain/stream", s.handleTraumaticBrainStream)
}

func writeTraumaticBrainJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleTraumaticBrainTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTBIService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.TraumaticBrainOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("traumatic_brain_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/traumatic_brain_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Traumatic Brain Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := TraumaticBrainPageData{
		Operations:                ops,
		ActiveOperation:           activeOp,
		TotalOperations:           metrics.TotalOperations,
		CushingHerniationCount:    metrics.CushingHerniationCount,
		HypertonicMannitolStaged:  metrics.HypertonicMannitolStaged,
		MGCSRecoveredClearedCount: metrics.MGCSRecoveredClearedCount,
		CushingSteroidAlertCount:  metrics.CushingSteroidAlertCount,
		Locale:                    locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListTraumaticBrainOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTBIService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.TraumaticBrainOperation, 0, len(ops))
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

func (s *Server) handleGetTraumaticBrainOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTBIService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeTraumaticBrainJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateTraumaticBrainOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createTBIOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.TBIPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.TBIPatientProfile{
			Species:                  payload.Species,
			Breed:                    payload.Breed,
			WeightKg:                 payload.WeightKg,
			ModifiedGlasgowComaScore: payload.ModifiedGlasgowComaScore,
			MeanArterialPressureMmHg: payload.MeanArterialPressureMmHg,
			HeartRateBpm:             payload.HeartRateBpm,
			EndTidalCO2MmHg:          payload.EndTidalCO2MmHg,
			PupillaryStatus:          payload.PupillaryStatus,
			CushingReflexPresent:     payload.CushingReflexPresent,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-TBI-%d", 9800+len(s.ensureTBIService().ListOperations(""))+1)
	}

	svc := s.ensureTBIService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTraumaticBrainNano()).UTC()
	s.broadcastTraumaticBrainEvent("traumatic_brain_updated", map[string]any{
		"type":      "traumatic_brain_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageTraumaticBrainPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTBIService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeTraumaticBrainJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageTBIPlanPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if payload.ModifiedGlasgowComaScore != 0 {
			profile.ModifiedGlasgowComaScore = payload.ModifiedGlasgowComaScore
		}
		if payload.MeanArterialPressureMmHg != 0 {
			profile.MeanArterialPressureMmHg = payload.MeanArterialPressureMmHg
		}
		if payload.HeartRateBpm != 0 {
			profile.HeartRateBpm = payload.HeartRateBpm
		}
		if payload.EndTidalCO2MmHg != 0 {
			profile.EndTidalCO2MmHg = payload.EndTidalCO2MmHg
		}
		if strings.TrimSpace(payload.PupillaryStatus) != "" {
			profile.PupillaryStatus = payload.PupillaryStatus
		}
		if payload.CushingReflexPresent != nil {
			profile.CushingReflexPresent = *payload.CushingReflexPresent
		}
	}

	updated, err := svc.StageNeuroprotectivePlan(id, profile)
	if err != nil {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTraumaticBrainNano()).UTC()
	s.broadcastTraumaticBrainEvent("traumatic_brain_updated", map[string]any{
		"type":      "traumatic_brain_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordTraumaticBrainCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTBIService()
	if _, err := svc.GetOperation(id); err != nil {
		writeTraumaticBrainJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.TBINeuroSerialCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordNeuroCheck(id, check)
	if err != nil {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTraumaticBrainNano()).UTC()
	s.broadcastTraumaticBrainEvent("traumatic_brain_updated", map[string]any{
		"type":      "traumatic_brain_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueTraumaticBrainDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTBIService()
	if _, err := svc.GetOperation(id); err != nil {
		writeTraumaticBrainJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.TBIDischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writeTraumaticBrainJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextTraumaticBrainNano()).UTC()
	s.broadcastTraumaticBrainEvent("traumatic_brain_updated", map[string]any{
		"type":      "traumatic_brain_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleTraumaticBrainStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("traumatic_brain")
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

func (s *Server) broadcastTraumaticBrainEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextTraumaticBrainNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "traumatic_brain",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "traumatic_brain",
			SenderNodeID:  "traumatic-brain-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
