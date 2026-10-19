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
	"github.com/scottdensmore/petspotr/pkg/statusepilepticus"
)

var lastStatusEpilepticusNano atomic.Int64

func nextStatusEpilepticusNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastStatusEpilepticusNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastStatusEpilepticusNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// StatusEpilepticusPageData represents template data passed to templates/status_epilepticus_triage.html.
type StatusEpilepticusPageData struct {
	Operations                   []domain.StatusEpilepticusOperation
	ActiveOperation              *domain.StatusEpilepticusOperation
	TotalOperations              int
	ActiveIctalCriticalCount     int
	AnticonvulsantCRIStagedCount int
	SeizureFreeClearedCount      int
	HyperthermiaAlertCount       int
	Locale                       string
}

type createEpilepticusOperationPayload struct {
	IncidentID               string                            `json:"incident_id"`
	PetName                  string                            `json:"pet_name"`
	PatientProfile           *domain.EpilepticusPatientProfile `json:"patient_profile"`
	Species                  string                            `json:"species"`
	Breed                    string                            `json:"breed"`
	WeightKg                 float64                           `json:"weight_kg"`
	NeuroEtiology            string                            `json:"neuro_etiology"`
	ContinuousIctalMinutes   int                               `json:"continuous_ictal_minutes"`
	ClusterCount24Hr         int                               `json:"cluster_count_24hr"`
	RectalTempFahrenheit     float64                           `json:"rectal_temp_fahrenheit"`
	BloodGlucoseMgDl         float64                           `json:"blood_glucose_mg_dl"`
	ModifiedGlasgowComaScore int                               `json:"modified_glasgow_coma_score"`
}

type stageEpilepticusRegimenPayload struct {
	PatientProfile           *domain.EpilepticusPatientProfile `json:"patient_profile"`
	Species                  string                            `json:"species"`
	Breed                    string                            `json:"breed"`
	WeightKg                 float64                           `json:"weight_kg"`
	NeuroEtiology            string                            `json:"neuro_etiology"`
	ContinuousIctalMinutes   *int                              `json:"continuous_ictal_minutes"`
	ClusterCount24Hr         *int                              `json:"cluster_count_24hr"`
	RectalTempFahrenheit     float64                           `json:"rectal_temp_fahrenheit"`
	BloodGlucoseMgDl         float64                           `json:"blood_glucose_mg_dl"`
	ModifiedGlasgowComaScore int                               `json:"modified_glasgow_coma_score"`
}

func (s *Server) ensureEpilepticusService() *statusepilepticus.EpilepticusService {
	if s.epilepticusService == nil {
		s.epilepticusService = statusepilepticus.NewEpilepticusService()
	}
	return s.epilepticusService
}

func (s *Server) registerStatusEpilepticusRoutes() {
	s.mux.HandleFunc("GET /status-epilepticus-triage", s.handleStatusEpilepticusTriagePage)

	s.mux.HandleFunc("GET /api/status-epilepticus/operations", s.handleListStatusEpilepticusOperations)
	s.mux.HandleFunc("POST /api/status-epilepticus/operations", s.handleCreateStatusEpilepticusOperation)
	s.mux.HandleFunc("GET /api/status-epilepticus/operations/{id}", s.handleGetStatusEpilepticusOperation)
	s.mux.HandleFunc("POST /api/status-epilepticus/operations/{id}/stage-regimen", s.handleStageStatusEpilepticusRegimen)
	s.mux.HandleFunc("POST /api/status-epilepticus/operations/{id}/neuro-check", s.handleRecordStatusEpilepticusNeuroCheck)
	s.mux.HandleFunc("POST /api/status-epilepticus/operations/{id}/discharge-clearance", s.handleIssueStatusEpilepticusDischargeClearance)
	s.mux.HandleFunc("GET /api/status-epilepticus/stream", s.handleStatusEpilepticusStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/status-epilepticus/operations", s.handleListStatusEpilepticusOperations)
	s.mux.HandleFunc("POST /api/v1/status-epilepticus/operations", s.handleCreateStatusEpilepticusOperation)
	s.mux.HandleFunc("GET /api/v1/status-epilepticus/operations/{id}", s.handleGetStatusEpilepticusOperation)
	s.mux.HandleFunc("POST /api/v1/status-epilepticus/operations/{id}/stage-regimen", s.handleStageStatusEpilepticusRegimen)
	s.mux.HandleFunc("POST /api/v1/status-epilepticus/operations/{id}/neuro-check", s.handleRecordStatusEpilepticusNeuroCheck)
	s.mux.HandleFunc("POST /api/v1/status-epilepticus/operations/{id}/discharge-clearance", s.handleIssueStatusEpilepticusDischargeClearance)
	s.mux.HandleFunc("GET /api/v1/status-epilepticus/stream", s.handleStatusEpilepticusStream)
}

func writeStatusEpilepticusJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleStatusEpilepticusTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureEpilepticusService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.StatusEpilepticusOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("status_epilepticus_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/status_epilepticus_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Status Epilepticus Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := StatusEpilepticusPageData{
		Operations:                   ops,
		ActiveOperation:              activeOp,
		TotalOperations:              metrics.TotalOperations,
		ActiveIctalCriticalCount:     metrics.ActiveIctalCriticalCount,
		AnticonvulsantCRIStagedCount: metrics.AnticonvulsantCRIStagedCount,
		SeizureFreeClearedCount:      metrics.SeizureFreeClearedCount,
		HyperthermiaAlertCount:       metrics.HyperthermiaAlertCount,
		Locale:                       locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListStatusEpilepticusOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureEpilepticusService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	etiologyFilter := strings.TrimSpace(r.URL.Query().Get("neuro_etiology"))
	if etiologyFilter == "" {
		etiologyFilter = strings.TrimSpace(r.URL.Query().Get("etiology"))
	}

	ops := svc.ListOperations(statusFilter)
	if etiologyFilter != "" {
		filtered := make([]domain.StatusEpilepticusOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(op.PatientProfile.NeuroEtiology, etiologyFilter) {
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

func (s *Server) handleGetStatusEpilepticusOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureEpilepticusService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateStatusEpilepticusOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createEpilepticusOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.EpilepticusPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.EpilepticusPatientProfile{
			Species:                  payload.Species,
			Breed:                    payload.Breed,
			WeightKg:                 payload.WeightKg,
			NeuroEtiology:            payload.NeuroEtiology,
			ContinuousIctalMinutes:   payload.ContinuousIctalMinutes,
			ClusterCount24Hr:         payload.ClusterCount24Hr,
			RectalTempFahrenheit:     payload.RectalTempFahrenheit,
			BloodGlucoseMgDl:         payload.BloodGlucoseMgDl,
			ModifiedGlasgowComaScore: payload.ModifiedGlasgowComaScore,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-SE-%d", 9200+len(s.ensureEpilepticusService().ListOperations(""))+1)
	}

	svc := s.ensureEpilepticusService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextStatusEpilepticusNano()).UTC()
	s.broadcastStatusEpilepticusEvent("status_epilepticus_updated", map[string]any{
		"type":      "status_epilepticus_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageStatusEpilepticusRegimen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureEpilepticusService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageEpilepticusRegimenPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if strings.TrimSpace(payload.NeuroEtiology) != "" {
			profile.NeuroEtiology = payload.NeuroEtiology
		}
		if payload.ContinuousIctalMinutes != nil {
			profile.ContinuousIctalMinutes = *payload.ContinuousIctalMinutes
		}
		if payload.ClusterCount24Hr != nil {
			profile.ClusterCount24Hr = *payload.ClusterCount24Hr
		}
		if payload.RectalTempFahrenheit != 0 {
			profile.RectalTempFahrenheit = payload.RectalTempFahrenheit
		}
		if payload.BloodGlucoseMgDl != 0 {
			profile.BloodGlucoseMgDl = payload.BloodGlucoseMgDl
		}
		if payload.ModifiedGlasgowComaScore != 0 {
			profile.ModifiedGlasgowComaScore = payload.ModifiedGlasgowComaScore
		}
	}

	updated, err := svc.StageTitrationRegimen(id, profile)
	if err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextStatusEpilepticusNano()).UTC()
	s.broadcastStatusEpilepticusEvent("status_epilepticus_updated", map[string]any{
		"type":      "status_epilepticus_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordStatusEpilepticusNeuroCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureEpilepticusService()
	if _, err := svc.GetOperation(id); err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.NeuroIcuSerialCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordNeuroCheck(id, check)
	if err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextStatusEpilepticusNano()).UTC()
	s.broadcastStatusEpilepticusEvent("status_epilepticus_updated", map[string]any{
		"type":      "status_epilepticus_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueStatusEpilepticusDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureEpilepticusService()
	if _, err := svc.GetOperation(id); err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.EpilepticusDischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writeStatusEpilepticusJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextStatusEpilepticusNano()).UTC()
	s.broadcastStatusEpilepticusEvent("status_epilepticus_updated", map[string]any{
		"type":      "status_epilepticus_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleStatusEpilepticusStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("status_epilepticus")
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

func (s *Server) broadcastStatusEpilepticusEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextStatusEpilepticusNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "status_epilepticus",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "status_epilepticus",
			SenderNodeID:  "status-epilepticus-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
