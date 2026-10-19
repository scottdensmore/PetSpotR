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

	"github.com/scottdensmore/petspotr/pkg/addisoniancrisis"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastAddisonianCrisisNano atomic.Int64

func nextAddisonianCrisisNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastAddisonianCrisisNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastAddisonianCrisisNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// AddisonianCrisisPageData represents template data passed to templates/addisonian_crisis_triage.html.
type AddisonianCrisisPageData struct {
	Operations                  []domain.AddisonianCrisisOperation
	ActiveOperation             *domain.AddisonianCrisisOperation
	TotalOperations             int
	CriticalShockCount          int
	DexSPACTHStagedCount        int
	DOCPEukalemicClearedCount   int
	MyelinolysisAssayAlertCount int
	Locale                      string
}

type createAddisonianOperationPayload struct {
	IncidentID                string                           `json:"incident_id"`
	PetName                   string                           `json:"pet_name"`
	PatientProfile            *domain.AddisonianPatientProfile `json:"patient_profile"`
	Species                   string                           `json:"species"`
	Breed                     string                           `json:"breed"`
	WeightKg                  float64                          `json:"weight_kg"`
	SerumSodiumMeqL           float64                          `json:"serum_sodium_meq_l"`
	SerumPotassiumMeqL        float64                          `json:"serum_potassium_meq_l"`
	BloodGlucoseMgDl          float64                          `json:"blood_glucose_mg_dl"`
	HeartRateBpm              int                              `json:"heart_rate_bpm"`
	SystolicBloodPressureMmHg int                              `json:"systolic_blood_pressure_mmhg"`
	RestingCortisolMcgDl      float64                          `json:"resting_cortisol_mcg_dl"`
}

type stageAddisonianRegimenPayload struct {
	PatientProfile            *domain.AddisonianPatientProfile `json:"patient_profile"`
	Species                   string                           `json:"species"`
	Breed                     string                           `json:"breed"`
	WeightKg                  float64                          `json:"weight_kg"`
	SerumSodiumMeqL           float64                          `json:"serum_sodium_meq_l"`
	SerumPotassiumMeqL        float64                          `json:"serum_potassium_meq_l"`
	BloodGlucoseMgDl          float64                          `json:"blood_glucose_mg_dl"`
	HeartRateBpm              int                              `json:"heart_rate_bpm"`
	SystolicBloodPressureMmHg int                              `json:"systolic_blood_pressure_mmhg"`
	RestingCortisolMcgDl      *float64                         `json:"resting_cortisol_mcg_dl"`
}

func (s *Server) ensureAddisonianService() *addisoniancrisis.AddisonianService {
	if s.addisonianService == nil {
		s.addisonianService = addisoniancrisis.NewAddisonianService()
	}
	return s.addisonianService
}

func (s *Server) registerAddisonianCrisisRoutes() {
	s.mux.HandleFunc("GET /addisonian-crisis-triage", s.handleAddisonianCrisisTriagePage)

	s.mux.HandleFunc("GET /api/addisonian-crisis/operations", s.handleListAddisonianCrisisOperations)
	s.mux.HandleFunc("POST /api/addisonian-crisis/operations", s.handleCreateAddisonianCrisisOperation)
	s.mux.HandleFunc("GET /api/addisonian-crisis/operations/{id}", s.handleGetAddisonianCrisisOperation)
	s.mux.HandleFunc("POST /api/addisonian-crisis/operations/{id}/stage-regimen", s.handleStageAddisonianCrisisRegimen)
	s.mux.HandleFunc("POST /api/addisonian-crisis/operations/{id}/electrolyte-check", s.handleRecordAddisonianCrisisElectrolyteCheck)
	s.mux.HandleFunc("POST /api/addisonian-crisis/operations/{id}/discharge-clearance", s.handleIssueAddisonianCrisisDischargeClearance)
	s.mux.HandleFunc("GET /api/addisonian-crisis/stream", s.handleAddisonianCrisisStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/addisonian-crisis/operations", s.handleListAddisonianCrisisOperations)
	s.mux.HandleFunc("POST /api/v1/addisonian-crisis/operations", s.handleCreateAddisonianCrisisOperation)
	s.mux.HandleFunc("GET /api/v1/addisonian-crisis/operations/{id}", s.handleGetAddisonianCrisisOperation)
	s.mux.HandleFunc("POST /api/v1/addisonian-crisis/operations/{id}/stage-regimen", s.handleStageAddisonianCrisisRegimen)
	s.mux.HandleFunc("POST /api/v1/addisonian-crisis/operations/{id}/electrolyte-check", s.handleRecordAddisonianCrisisElectrolyteCheck)
	s.mux.HandleFunc("POST /api/v1/addisonian-crisis/operations/{id}/discharge-clearance", s.handleIssueAddisonianCrisisDischargeClearance)
	s.mux.HandleFunc("GET /api/v1/addisonian-crisis/stream", s.handleAddisonianCrisisStream)
}

func writeAddisonianCrisisJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleAddisonianCrisisTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAddisonianService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.AddisonianCrisisOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("addisonian_crisis_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/addisonian_crisis_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Addisonian Crisis Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := AddisonianCrisisPageData{
		Operations:                  ops,
		ActiveOperation:             activeOp,
		TotalOperations:             metrics.TotalOperations,
		CriticalShockCount:          metrics.CriticalShockCount,
		DexSPACTHStagedCount:        metrics.DexSPACTHStagedCount,
		DOCPEukalemicClearedCount:   metrics.DOCPEukalemicClearedCount,
		MyelinolysisAssayAlertCount: metrics.MyelinolysisAssayAlertCount,
		Locale:                      locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListAddisonianCrisisOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAddisonianService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.AddisonianCrisisOperation, 0, len(ops))
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

func (s *Server) handleGetAddisonianCrisisOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAddisonianService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateAddisonianCrisisOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createAddisonianOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.AddisonianPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.AddisonianPatientProfile{
			Species:                   payload.Species,
			Breed:                     payload.Breed,
			WeightKg:                  payload.WeightKg,
			SerumSodiumMeqL:           payload.SerumSodiumMeqL,
			SerumPotassiumMeqL:        payload.SerumPotassiumMeqL,
			BloodGlucoseMgDl:          payload.BloodGlucoseMgDl,
			HeartRateBpm:              payload.HeartRateBpm,
			SystolicBloodPressureMmHg: payload.SystolicBloodPressureMmHg,
			RestingCortisolMcgDl:      payload.RestingCortisolMcgDl,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-AC-%d", 9600+len(s.ensureAddisonianService().ListOperations(""))+1)
	}

	svc := s.ensureAddisonianService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAddisonianCrisisNano()).UTC()
	s.broadcastAddisonianCrisisEvent("addisonian_crisis_updated", map[string]any{
		"type":      "addisonian_crisis_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageAddisonianCrisisRegimen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAddisonianService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageAddisonianRegimenPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if payload.SerumSodiumMeqL != 0 {
			profile.SerumSodiumMeqL = payload.SerumSodiumMeqL
		}
		if payload.SerumPotassiumMeqL != 0 {
			profile.SerumPotassiumMeqL = payload.SerumPotassiumMeqL
		}
		if payload.BloodGlucoseMgDl != 0 {
			profile.BloodGlucoseMgDl = payload.BloodGlucoseMgDl
		}
		if payload.HeartRateBpm != 0 {
			profile.HeartRateBpm = payload.HeartRateBpm
		}
		if payload.SystolicBloodPressureMmHg != 0 {
			profile.SystolicBloodPressureMmHg = payload.SystolicBloodPressureMmHg
		}
		if payload.RestingCortisolMcgDl != nil {
			profile.RestingCortisolMcgDl = *payload.RestingCortisolMcgDl
		}
	}

	updated, err := svc.StageResuscitationRegimen(id, profile)
	if err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAddisonianCrisisNano()).UTC()
	s.broadcastAddisonianCrisisEvent("addisonian_crisis_updated", map[string]any{
		"type":      "addisonian_crisis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordAddisonianCrisisElectrolyteCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAddisonianService()
	if _, err := svc.GetOperation(id); err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.AddisonianElectrolyteCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordElectrolyteCheck(id, check)
	if err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAddisonianCrisisNano()).UTC()
	s.broadcastAddisonianCrisisEvent("addisonian_crisis_updated", map[string]any{
		"type":      "addisonian_crisis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueAddisonianCrisisDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAddisonianService()
	if _, err := svc.GetOperation(id); err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.AddisonianDischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writeAddisonianCrisisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextAddisonianCrisisNano()).UTC()
	s.broadcastAddisonianCrisisEvent("addisonian_crisis_updated", map[string]any{
		"type":      "addisonian_crisis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleAddisonianCrisisStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("addisonian_crisis")
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

func (s *Server) broadcastAddisonianCrisisEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextAddisonianCrisisNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "addisonian_crisis",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "addisonian_crisis",
			SenderNodeID:  "addisonian-crisis-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
