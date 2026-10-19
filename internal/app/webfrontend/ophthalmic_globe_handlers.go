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
	"github.com/scottdensmore/petspotr/pkg/ophthalmicglobe"
)

var lastOphthalmicGlobeNano atomic.Int64

func nextOphthalmicGlobeNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastOphthalmicGlobeNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastOphthalmicGlobeNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// OphthalmicGlobePageData represents template data passed to templates/ophthalmic_globe_triage.html.
type OphthalmicGlobePageData struct {
	Operations                         []domain.OphthalmicGlobeOperation
	ActiveOperation                    *domain.OphthalmicGlobeOperation
	TotalOperations                    int
	VisionThreateningCriticalCount     int
	LatanoprostTarsorrhaphyStagedCount int
	IOPNormalizedClearedCount          int
	LatanoprostSteroidAlertCount       int
	Locale                             string
}

type createOphthalmicOperationPayload struct {
	IncidentID                     string                           `json:"incident_id"`
	PetName                        string                           `json:"pet_name"`
	PatientProfile                 *domain.OphthalmicPatientProfile `json:"patient_profile"`
	Species                        string                           `json:"species"`
	Breed                          string                           `json:"breed"`
	WeightKg                       float64                          `json:"weight_kg"`
	IntraocularPressureMmHg        int                              `json:"intraocular_pressure_mmhg"`
	FluoresceinStainPattern        string                           `json:"fluorescein_stain_pattern"`
	AnteriorLensLuxationPresent    bool                             `json:"anterior_lens_luxation_present"`
	ExtraocularMusclesAvulsedCount int                              `json:"extraocular_muscles_avulsed_count"`
	ConsensualPLRPresent           *bool                            `json:"consensual_plr_present"`
}

type stageOphthalmicPlanPayload struct {
	PatientProfile                 *domain.OphthalmicPatientProfile `json:"patient_profile"`
	Species                        string                           `json:"species"`
	Breed                          string                           `json:"breed"`
	WeightKg                       float64                          `json:"weight_kg"`
	IntraocularPressureMmHg        int                              `json:"intraocular_pressure_mmhg"`
	FluoresceinStainPattern        string                           `json:"fluorescein_stain_pattern"`
	AnteriorLensLuxationPresent    *bool                            `json:"anterior_lens_luxation_present"`
	ExtraocularMusclesAvulsedCount *int                             `json:"extraocular_muscles_avulsed_count"`
	ConsensualPLRPresent           *bool                            `json:"consensual_plr_present"`
}

func (s *Server) ensureOphthalmicService() *ophthalmicglobe.OphthalmicService {
	if s.ophthalmicService == nil {
		s.ophthalmicService = ophthalmicglobe.NewOphthalmicService()
	}
	return s.ophthalmicService
}

func (s *Server) registerOphthalmicGlobeRoutes() {
	s.mux.HandleFunc("GET /ophthalmic-globe-triage", s.handleOphthalmicGlobeTriagePage)

	s.mux.HandleFunc("GET /api/ophthalmic-globe/operations", s.handleListOphthalmicGlobeOperations)
	s.mux.HandleFunc("POST /api/ophthalmic-globe/operations", s.handleCreateOphthalmicGlobeOperation)
	s.mux.HandleFunc("GET /api/ophthalmic-globe/operations/{id}", s.handleGetOphthalmicGlobeOperation)
	s.mux.HandleFunc("POST /api/ophthalmic-globe/operations/{id}/stage-plan", s.handleStageOphthalmicGlobePlan)
	s.mux.HandleFunc("POST /api/ophthalmic-globe/operations/{id}/tonometry-check", s.handleRecordOphthalmicGlobeTonometryCheck)
	s.mux.HandleFunc("POST /api/ophthalmic-globe/operations/{id}/discharge-clearance", s.handleIssueOphthalmicGlobeDischargeClearance)
	s.mux.HandleFunc("GET /api/ophthalmic-globe/stream", s.handleOphthalmicGlobeStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/ophthalmic-globe/operations", s.handleListOphthalmicGlobeOperations)
	s.mux.HandleFunc("POST /api/v1/ophthalmic-globe/operations", s.handleCreateOphthalmicGlobeOperation)
	s.mux.HandleFunc("GET /api/v1/ophthalmic-globe/operations/{id}", s.handleGetOphthalmicGlobeOperation)
	s.mux.HandleFunc("POST /api/v1/ophthalmic-globe/operations/{id}/stage-plan", s.handleStageOphthalmicGlobePlan)
	s.mux.HandleFunc("POST /api/v1/ophthalmic-globe/operations/{id}/tonometry-check", s.handleRecordOphthalmicGlobeTonometryCheck)
	s.mux.HandleFunc("POST /api/v1/ophthalmic-globe/operations/{id}/discharge-clearance", s.handleIssueOphthalmicGlobeDischargeClearance)
	s.mux.HandleFunc("GET /api/v1/ophthalmic-globe/stream", s.handleOphthalmicGlobeStream)
}

func writeOphthalmicGlobeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleOphthalmicGlobeTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureOphthalmicService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.OphthalmicGlobeOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("ophthalmic_globe_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/ophthalmic_globe_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Ophthalmic Globe Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := OphthalmicGlobePageData{
		Operations:                         ops,
		ActiveOperation:                    activeOp,
		TotalOperations:                    metrics.TotalOperations,
		VisionThreateningCriticalCount:     metrics.VisionThreateningCriticalCount,
		LatanoprostTarsorrhaphyStagedCount: metrics.LatanoprostTarsorrhaphyStagedCount,
		IOPNormalizedClearedCount:          metrics.IOPNormalizedClearedCount,
		LatanoprostSteroidAlertCount:       metrics.LatanoprostSteroidAlertCount,
		Locale:                             locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListOphthalmicGlobeOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureOphthalmicService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.OphthalmicGlobeOperation, 0, len(ops))
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

func (s *Server) handleGetOphthalmicGlobeOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureOphthalmicService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateOphthalmicGlobeOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createOphthalmicOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.OphthalmicPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		plrPresent := true
		if payload.ConsensualPLRPresent != nil {
			plrPresent = *payload.ConsensualPLRPresent
		}
		profile = domain.OphthalmicPatientProfile{
			Species:                        payload.Species,
			Breed:                          payload.Breed,
			WeightKg:                       payload.WeightKg,
			IntraocularPressureMmHg:        payload.IntraocularPressureMmHg,
			FluoresceinStainPattern:        payload.FluoresceinStainPattern,
			AnteriorLensLuxationPresent:    payload.AnteriorLensLuxationPresent,
			ExtraocularMusclesAvulsedCount: payload.ExtraocularMusclesAvulsedCount,
			ConsensualPLRPresent:           plrPresent,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-OG-%d", 9950+len(s.ensureOphthalmicService().ListOperations(""))+1)
	}

	svc := s.ensureOphthalmicService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextOphthalmicGlobeNano()).UTC()
	s.broadcastOphthalmicGlobeEvent("ophthalmic_globe_updated", map[string]any{
		"type":      "ophthalmic_globe_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageOphthalmicGlobePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureOphthalmicService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageOphthalmicPlanPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if payload.IntraocularPressureMmHg != 0 {
			profile.IntraocularPressureMmHg = payload.IntraocularPressureMmHg
		}
		if strings.TrimSpace(payload.FluoresceinStainPattern) != "" {
			profile.FluoresceinStainPattern = payload.FluoresceinStainPattern
		}
		if payload.AnteriorLensLuxationPresent != nil {
			profile.AnteriorLensLuxationPresent = *payload.AnteriorLensLuxationPresent
		}
		if payload.ExtraocularMusclesAvulsedCount != nil {
			profile.ExtraocularMusclesAvulsedCount = *payload.ExtraocularMusclesAvulsedCount
		}
		if payload.ConsensualPLRPresent != nil {
			profile.ConsensualPLRPresent = *payload.ConsensualPLRPresent
		}
	}

	updated, err := svc.StagePreservationPlan(id, profile)
	if err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextOphthalmicGlobeNano()).UTC()
	s.broadcastOphthalmicGlobeEvent("ophthalmic_globe_updated", map[string]any{
		"type":      "ophthalmic_globe_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordOphthalmicGlobeTonometryCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureOphthalmicService()
	if _, err := svc.GetOperation(id); err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.OphthalmicTonometryCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordTonometryCheck(id, check)
	if err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextOphthalmicGlobeNano()).UTC()
	s.broadcastOphthalmicGlobeEvent("ophthalmic_globe_updated", map[string]any{
		"type":      "ophthalmic_globe_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueOphthalmicGlobeDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureOphthalmicService()
	if _, err := svc.GetOperation(id); err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.OphthalmicDischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writeOphthalmicGlobeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextOphthalmicGlobeNano()).UTC()
	s.broadcastOphthalmicGlobeEvent("ophthalmic_globe_updated", map[string]any{
		"type":      "ophthalmic_globe_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleOphthalmicGlobeStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("ophthalmic_globe")
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

func (s *Server) broadcastOphthalmicGlobeEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextOphthalmicGlobeNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "ophthalmic_globe",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "ophthalmic_globe",
			SenderNodeID:  "ophthalmic-globe-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
