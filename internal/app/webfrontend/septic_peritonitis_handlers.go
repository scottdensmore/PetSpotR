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
	"github.com/scottdensmore/petspotr/pkg/septicperitonitis"
)

var lastSepticPeritonitisNano atomic.Int64

func nextSepticPeritonitisNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastSepticPeritonitisNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastSepticPeritonitisNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// SepticPeritonitisPageData represents template data passed to templates/septic_peritonitis_triage.html.
type SepticPeritonitisPageData struct {
	Operations                    []domain.SepticPeritonitisOperation
	ActiveOperation               *domain.SepticPeritonitisOperation
	TotalOperations               int
	SepticEffusionCriticalCount   int
	AntibioticLavageJPStagedCount int
	NonSepticClearedCount         int
	SepticGradientAlertCount      int
	Locale                        string
}

type createPeritonitisOperationPayload struct {
	IncidentID                   string                            `json:"incident_id"`
	PetName                      string                            `json:"pet_name"`
	PatientProfile               *domain.PeritonitisPatientProfile `json:"patient_profile"`
	Species                      string                            `json:"species"`
	Breed                        string                            `json:"breed"`
	WeightKg                     float64                           `json:"weight_kg"`
	BloodGlucoseMgDl             float64                           `json:"blood_glucose_mg_dl"`
	EffusionGlucoseMgDl          float64                           `json:"effusion_glucose_mg_dl"`
	BloodLactateMmolL            float64                           `json:"blood_lactate_mmol_l"`
	EffusionLactateMmolL         float64                           `json:"effusion_lactate_mmol_l"`
	DegenerateNeutrophilsCountUl int                               `json:"degenerate_neutrophils_count_ul"`
	IntracellularBacteriaSeen    bool                              `json:"intracellular_bacteria_seen"`
}

type stagePeritonitisPlanPayload struct {
	PatientProfile               *domain.PeritonitisPatientProfile `json:"patient_profile"`
	Species                      string                            `json:"species"`
	Breed                        string                            `json:"breed"`
	WeightKg                     float64                           `json:"weight_kg"`
	BloodGlucoseMgDl             float64                           `json:"blood_glucose_mg_dl"`
	EffusionGlucoseMgDl          float64                           `json:"effusion_glucose_mg_dl"`
	BloodLactateMmolL            float64                           `json:"blood_lactate_mmol_l"`
	EffusionLactateMmolL         float64                           `json:"effusion_lactate_mmol_l"`
	DegenerateNeutrophilsCountUl *int                              `json:"degenerate_neutrophils_count_ul"`
	IntracellularBacteriaSeen    *bool                             `json:"intracellular_bacteria_seen"`
}

func (s *Server) ensurePeritonitisService() *septicperitonitis.PeritonitisService {
	if s.peritonitisService == nil {
		s.peritonitisService = septicperitonitis.NewPeritonitisService()
	}
	return s.peritonitisService
}

func (s *Server) registerSepticPeritonitisRoutes() {
	s.mux.HandleFunc("GET /septic-peritonitis-triage", s.handleSepticPeritonitisTriagePage)

	s.mux.HandleFunc("GET /api/septic-peritonitis/operations", s.handleListSepticPeritonitisOperations)
	s.mux.HandleFunc("POST /api/septic-peritonitis/operations", s.handleCreateSepticPeritonitisOperation)
	s.mux.HandleFunc("GET /api/septic-peritonitis/operations/{id}", s.handleGetSepticPeritonitisOperation)
	s.mux.HandleFunc("POST /api/septic-peritonitis/operations/{id}/stage-plan", s.handleStageSepticPeritonitisPlan)
	s.mux.HandleFunc("POST /api/septic-peritonitis/operations/{id}/drain-check", s.handleRecordSepticPeritonitisDrainCheck)
	s.mux.HandleFunc("POST /api/septic-peritonitis/operations/{id}/drain-clearance", s.handleIssueSepticPeritonitisDrainClearance)
	s.mux.HandleFunc("GET /api/septic-peritonitis/stream", s.handleSepticPeritonitisStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/septic-peritonitis/operations", s.handleListSepticPeritonitisOperations)
	s.mux.HandleFunc("POST /api/v1/septic-peritonitis/operations", s.handleCreateSepticPeritonitisOperation)
	s.mux.HandleFunc("GET /api/v1/septic-peritonitis/operations/{id}", s.handleGetSepticPeritonitisOperation)
	s.mux.HandleFunc("POST /api/v1/septic-peritonitis/operations/{id}/stage-plan", s.handleStageSepticPeritonitisPlan)
	s.mux.HandleFunc("POST /api/v1/septic-peritonitis/operations/{id}/drain-check", s.handleRecordSepticPeritonitisDrainCheck)
	s.mux.HandleFunc("POST /api/v1/septic-peritonitis/operations/{id}/drain-clearance", s.handleIssueSepticPeritonitisDrainClearance)
	s.mux.HandleFunc("GET /api/v1/septic-peritonitis/stream", s.handleSepticPeritonitisStream)
}

func writeSepticPeritonitisJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleSepticPeritonitisTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePeritonitisService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.SepticPeritonitisOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("septic_peritonitis_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/septic_peritonitis_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Septic Peritonitis Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := SepticPeritonitisPageData{
		Operations:                    ops,
		ActiveOperation:               activeOp,
		TotalOperations:               metrics.TotalOperations,
		SepticEffusionCriticalCount:   metrics.SepticEffusionCriticalCount,
		AntibioticLavageJPStagedCount: metrics.AntibioticLavageJPStagedCount,
		NonSepticClearedCount:         metrics.NonSepticClearedCount,
		SepticGradientAlertCount:      metrics.SepticGradientAlertCount,
		Locale:                        locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListSepticPeritonitisOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePeritonitisService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.SepticPeritonitisOperation, 0, len(ops))
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

func (s *Server) handleGetSepticPeritonitisOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePeritonitisService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateSepticPeritonitisOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createPeritonitisOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.PeritonitisPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.PeritonitisPatientProfile{
			Species:                      payload.Species,
			Breed:                        payload.Breed,
			WeightKg:                     payload.WeightKg,
			BloodGlucoseMgDl:             payload.BloodGlucoseMgDl,
			EffusionGlucoseMgDl:          payload.EffusionGlucoseMgDl,
			BloodLactateMmolL:            payload.BloodLactateMmolL,
			EffusionLactateMmolL:         payload.EffusionLactateMmolL,
			DegenerateNeutrophilsCountUl: payload.DegenerateNeutrophilsCountUl,
			IntracellularBacteriaSeen:    payload.IntracellularBacteriaSeen,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-SP-%d", 9500+len(s.ensurePeritonitisService().ListOperations(""))+1)
	}

	svc := s.ensurePeritonitisService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextSepticPeritonitisNano()).UTC()
	s.broadcastSepticPeritonitisEvent("septic_peritonitis_updated", map[string]any{
		"type":      "septic_peritonitis_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageSepticPeritonitisPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePeritonitisService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stagePeritonitisPlanPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if payload.BloodGlucoseMgDl != 0 {
			profile.BloodGlucoseMgDl = payload.BloodGlucoseMgDl
		}
		if payload.EffusionGlucoseMgDl != 0 {
			profile.EffusionGlucoseMgDl = payload.EffusionGlucoseMgDl
		}
		if payload.BloodLactateMmolL != 0 {
			profile.BloodLactateMmolL = payload.BloodLactateMmolL
		}
		if payload.EffusionLactateMmolL != 0 {
			profile.EffusionLactateMmolL = payload.EffusionLactateMmolL
		}
		if payload.DegenerateNeutrophilsCountUl != nil {
			profile.DegenerateNeutrophilsCountUl = *payload.DegenerateNeutrophilsCountUl
		}
		if payload.IntracellularBacteriaSeen != nil {
			profile.IntracellularBacteriaSeen = *payload.IntracellularBacteriaSeen
		}
	}

	updated, err := svc.StageSurgicalPlan(id, profile)
	if err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextSepticPeritonitisNano()).UTC()
	s.broadcastSepticPeritonitisEvent("septic_peritonitis_updated", map[string]any{
		"type":      "septic_peritonitis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordSepticPeritonitisDrainCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePeritonitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.JacksonPrattDrainCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordDrainCheck(id, check)
	if err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextSepticPeritonitisNano()).UTC()
	s.broadcastSepticPeritonitisEvent("septic_peritonitis_updated", map[string]any{
		"type":      "septic_peritonitis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueSepticPeritonitisDrainClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePeritonitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.PeritonitisDrainRemovalClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDrainClearance(id, clearance)
	if err != nil {
		writeSepticPeritonitisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextSepticPeritonitisNano()).UTC()
	s.broadcastSepticPeritonitisEvent("septic_peritonitis_updated", map[string]any{
		"type":      "septic_peritonitis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleSepticPeritonitisStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("septic_peritonitis")
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

func (s *Server) broadcastSepticPeritonitisEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextSepticPeritonitisNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "septic_peritonitis",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "septic_peritonitis",
			SenderNodeID:  "septic-peritonitis-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
