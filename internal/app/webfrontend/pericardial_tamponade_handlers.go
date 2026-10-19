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
	"github.com/scottdensmore/petspotr/pkg/pericardialtamponade"
)

var lastPericardialTamponadeNano atomic.Int64

func nextPericardialTamponadeNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastPericardialTamponadeNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastPericardialTamponadeNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// PericardialTamponadePageData represents template data passed to templates/pericardial_tamponade_triage.html.
type PericardialTamponadePageData struct {
	Operations                         []domain.PericardialTamponadeOperation
	ActiveOperation                    *domain.PericardialTamponadeOperation
	TotalOperations                    int
	CriticalTamponadeCount             int
	RightICS45TapStagedCount           int
	DecompressedSinusClearedCount      int
	FurosemideLeftCoronaryHazardAlerts int
	Locale                             string
}

type createTamponadeOperationPayload struct {
	IncidentID                   string                          `json:"incident_id"`
	PetName                      string                          `json:"pet_name"`
	PatientProfile               *domain.TamponadePatientProfile `json:"patient_profile"`
	Species                      string                          `json:"species"`
	Breed                        string                          `json:"breed"`
	WeightKg                     float64                         `json:"weight_kg"`
	HeartRateBpm                 int                             `json:"heart_rate_bpm"`
	MeanArterialPressureMmHg     int                             `json:"mean_arterial_pressure_mmhg"`
	RightAtrialDiastolicCollapse bool                            `json:"right_atrial_diastolic_collapse"`
	ElectricalAlternansPresent   bool                            `json:"electrical_alternans_present"`
	PulsusParadoxusMmHg          int                             `json:"pulsus_paradoxus_mmhg"`
	EffusionPCVPercent           float64                         `json:"effusion_pcv_percent"`
}

type stageTamponadePlanPayload struct {
	PatientProfile               *domain.TamponadePatientProfile `json:"patient_profile"`
	Species                      string                          `json:"species"`
	Breed                        string                          `json:"breed"`
	WeightKg                     float64                         `json:"weight_kg"`
	HeartRateBpm                 int                             `json:"heart_rate_bpm"`
	MeanArterialPressureMmHg     int                             `json:"mean_arterial_pressure_mmhg"`
	RightAtrialDiastolicCollapse *bool                           `json:"right_atrial_diastolic_collapse"`
	ElectricalAlternansPresent   *bool                           `json:"electrical_alternans_present"`
	PulsusParadoxusMmHg          *int                            `json:"pulsus_paradoxus_mmhg"`
	EffusionPCVPercent           *float64                        `json:"effusion_pcv_percent"`
}

func (s *Server) ensureTamponadeService() *pericardialtamponade.TamponadeService {
	if s.tamponadeService == nil {
		s.tamponadeService = pericardialtamponade.NewTamponadeService()
	}
	return s.tamponadeService
}

func (s *Server) registerPericardialTamponadeRoutes() {
	s.mux.HandleFunc("GET /pericardial-tamponade-triage", s.handlePericardialTamponadeTriagePage)

	s.mux.HandleFunc("GET /api/pericardial-tamponade/operations", s.handleListPericardialTamponadeOperations)
	s.mux.HandleFunc("POST /api/pericardial-tamponade/operations", s.handleCreatePericardialTamponadeOperation)
	s.mux.HandleFunc("GET /api/pericardial-tamponade/operations/{id}", s.handleGetPericardialTamponadeOperation)
	s.mux.HandleFunc("POST /api/pericardial-tamponade/operations/{id}/stage-plan", s.handleStagePericardialTamponadePlan)
	s.mux.HandleFunc("POST /api/pericardial-tamponade/operations/{id}/tap-check", s.handleRecordPericardialTamponadeTapCheck)
	s.mux.HandleFunc("POST /api/pericardial-tamponade/operations/{id}/discharge-clearance", s.handleIssuePericardialTamponadeDischargeClearance)
	s.mux.HandleFunc("GET /api/pericardial-tamponade/stream", s.handlePericardialTamponadeStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/pericardial-tamponade/operations", s.handleListPericardialTamponadeOperations)
	s.mux.HandleFunc("POST /api/v1/pericardial-tamponade/operations", s.handleCreatePericardialTamponadeOperation)
	s.mux.HandleFunc("GET /api/v1/pericardial-tamponade/operations/{id}", s.handleGetPericardialTamponadeOperation)
	s.mux.HandleFunc("POST /api/v1/pericardial-tamponade/operations/{id}/stage-plan", s.handleStagePericardialTamponadePlan)
	s.mux.HandleFunc("POST /api/v1/pericardial-tamponade/operations/{id}/tap-check", s.handleRecordPericardialTamponadeTapCheck)
	s.mux.HandleFunc("POST /api/v1/pericardial-tamponade/operations/{id}/discharge-clearance", s.handleIssuePericardialTamponadeDischargeClearance)
	s.mux.HandleFunc("GET /api/v1/pericardial-tamponade/stream", s.handlePericardialTamponadeStream)
}

func writePericardialTamponadeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handlePericardialTamponadeTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTamponadeService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.PericardialTamponadeOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("pericardial_tamponade_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/pericardial_tamponade_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Pericardial Tamponade Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := PericardialTamponadePageData{
		Operations:                         ops,
		ActiveOperation:                    activeOp,
		TotalOperations:                    metrics.TotalOperations,
		CriticalTamponadeCount:             metrics.CriticalTamponadeCount,
		RightICS45TapStagedCount:           metrics.RightICS45TapStagedCount,
		DecompressedSinusClearedCount:      metrics.DecompressedSinusClearedCount,
		FurosemideLeftCoronaryHazardAlerts: metrics.FurosemideLeftCoronaryHazardAlerts,
		Locale:                             locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListPericardialTamponadeOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTamponadeService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.PericardialTamponadeOperation, 0, len(ops))
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

func (s *Server) handleGetPericardialTamponadeOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTamponadeService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writePericardialTamponadeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreatePericardialTamponadeOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createTamponadeOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.TamponadePatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.TamponadePatientProfile{
			Species:                      payload.Species,
			Breed:                        payload.Breed,
			WeightKg:                     payload.WeightKg,
			HeartRateBpm:                 payload.HeartRateBpm,
			MeanArterialPressureMmHg:     payload.MeanArterialPressureMmHg,
			RightAtrialDiastolicCollapse: payload.RightAtrialDiastolicCollapse,
			ElectricalAlternansPresent:   payload.ElectricalAlternansPresent,
			PulsusParadoxusMmHg:          payload.PulsusParadoxusMmHg,
			EffusionPCVPercent:           payload.EffusionPCVPercent,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-PT-%d", 9920+len(s.ensureTamponadeService().ListOperations(""))+1)
	}

	svc := s.ensureTamponadeService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPericardialTamponadeNano()).UTC()
	s.broadcastPericardialTamponadeEvent("pericardial_tamponade_updated", map[string]any{
		"type":      "pericardial_tamponade_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStagePericardialTamponadePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTamponadeService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writePericardialTamponadeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageTamponadePlanPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if payload.HeartRateBpm != 0 {
			profile.HeartRateBpm = payload.HeartRateBpm
		}
		if payload.MeanArterialPressureMmHg != 0 {
			profile.MeanArterialPressureMmHg = payload.MeanArterialPressureMmHg
		}
		if payload.RightAtrialDiastolicCollapse != nil {
			profile.RightAtrialDiastolicCollapse = *payload.RightAtrialDiastolicCollapse
		}
		if payload.ElectricalAlternansPresent != nil {
			profile.ElectricalAlternansPresent = *payload.ElectricalAlternansPresent
		}
		if payload.PulsusParadoxusMmHg != nil {
			profile.PulsusParadoxusMmHg = *payload.PulsusParadoxusMmHg
		}
		if payload.EffusionPCVPercent != nil {
			profile.EffusionPCVPercent = *payload.EffusionPCVPercent
		}
	}

	updated, err := svc.StagePericardiocentesisPlan(id, profile)
	if err != nil {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPericardialTamponadeNano()).UTC()
	s.broadcastPericardialTamponadeEvent("pericardial_tamponade_updated", map[string]any{
		"type":      "pericardial_tamponade_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordPericardialTamponadeTapCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTamponadeService()
	if _, err := svc.GetOperation(id); err != nil {
		writePericardialTamponadeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.PericardiocentesisTapCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordTapCheck(id, check)
	if err != nil {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPericardialTamponadeNano()).UTC()
	s.broadcastPericardialTamponadeEvent("pericardial_tamponade_updated", map[string]any{
		"type":      "pericardial_tamponade_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssuePericardialTamponadeDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTamponadeService()
	if _, err := svc.GetOperation(id); err != nil {
		writePericardialTamponadeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.TamponadeDischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writePericardialTamponadeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextPericardialTamponadeNano()).UTC()
	s.broadcastPericardialTamponadeEvent("pericardial_tamponade_updated", map[string]any{
		"type":      "pericardial_tamponade_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handlePericardialTamponadeStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("pericardial_tamponade")
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

func (s *Server) broadcastPericardialTamponadeEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextPericardialTamponadeNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "pericardial_tamponade",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "pericardial_tamponade",
			SenderNodeID:  "pericardial-tamponade-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
