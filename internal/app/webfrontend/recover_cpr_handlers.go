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
	"github.com/scottdensmore/petspotr/pkg/recovercpr"
)

var lastRecoverCPRNano atomic.Int64

func nextRecoverCPRNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastRecoverCPRNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastRecoverCPRNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// RecoverCPRPageData represents template data passed to templates/recover_cpr_triage.html.
type RecoverCPRPageData struct {
	Operations                         []domain.RecoverCPROperation
	ActiveOperation                    *domain.RecoverCPROperation
	TotalOperations                    int
	ArrestInProgressCriticalCount      int
	EpiDefibCycleStagedCount           int
	ROSCPostArrestClearedCount         int
	HyperventilationHighDoseAlertCount int
	Locale                             string
}

type createRecoverCPROperationPayload struct {
	IncidentID                    string                    `json:"incident_id"`
	PetName                       string                    `json:"pet_name"`
	PatientProfile                *domain.CPRPatientProfile `json:"patient_profile"`
	Species                       string                    `json:"species"`
	Breed                         string                    `json:"breed"`
	WeightKg                      float64                   `json:"weight_kg"`
	ArrestRhythm                  string                    `json:"arrest_rhythm"`
	EndTidalCO2MmHg               float64                   `json:"end_tidal_co2_mmhg"`
	CompressionCyclesCompleted    int                       `json:"compression_cycles_completed"`
	ReversibleOpioidAlpha2Present bool                      `json:"reversible_opioid_alpha2_present"`
}

type stageRecoverCPRPlanPayload struct {
	PatientProfile                *domain.CPRPatientProfile `json:"patient_profile"`
	Species                       string                    `json:"species"`
	Breed                         string                    `json:"breed"`
	WeightKg                      float64                   `json:"weight_kg"`
	ArrestRhythm                  string                    `json:"arrest_rhythm"`
	EndTidalCO2MmHg               float64                   `json:"end_tidal_co2_mmhg"`
	CompressionCyclesCompleted    *int                      `json:"compression_cycles_completed"`
	ReversibleOpioidAlpha2Present *bool                     `json:"reversible_opioid_alpha2_present"`
}

func (s *Server) ensureRecoverService() *recovercpr.RecoverService {
	if s.recoverService == nil {
		s.recoverService = recovercpr.NewRecoverService()
	}
	return s.recoverService
}

func (s *Server) registerRecoverCPRRoutes() {
	s.mux.HandleFunc("GET /recover-cpr-triage", s.handleRecoverCPRTriagePage)

	s.mux.HandleFunc("GET /api/recover-cpr/operations", s.handleListRecoverCPROperations)
	s.mux.HandleFunc("POST /api/recover-cpr/operations", s.handleCreateRecoverCPROperation)
	s.mux.HandleFunc("GET /api/recover-cpr/operations/{id}", s.handleGetRecoverCPROperation)
	s.mux.HandleFunc("POST /api/recover-cpr/operations/{id}/stage-plan", s.handleStageRecoverCPRPlan)
	s.mux.HandleFunc("POST /api/recover-cpr/operations/{id}/cycle-check", s.handleRecordRecoverCPRCycleCheck)
	s.mux.HandleFunc("POST /api/recover-cpr/operations/{id}/rosc-clearance", s.handleIssueRecoverCPRROSCClearance)
	s.mux.HandleFunc("GET /api/recover-cpr/stream", s.handleRecoverCPRStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/recover-cpr/operations", s.handleListRecoverCPROperations)
	s.mux.HandleFunc("POST /api/v1/recover-cpr/operations", s.handleCreateRecoverCPROperation)
	s.mux.HandleFunc("GET /api/v1/recover-cpr/operations/{id}", s.handleGetRecoverCPROperation)
	s.mux.HandleFunc("POST /api/v1/recover-cpr/operations/{id}/stage-plan", s.handleStageRecoverCPRPlan)
	s.mux.HandleFunc("POST /api/v1/recover-cpr/operations/{id}/cycle-check", s.handleRecordRecoverCPRCycleCheck)
	s.mux.HandleFunc("POST /api/v1/recover-cpr/operations/{id}/rosc-clearance", s.handleIssueRecoverCPRROSCClearance)
	s.mux.HandleFunc("GET /api/v1/recover-cpr/stream", s.handleRecoverCPRStream)
}

func writeRecoverCPRJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleRecoverCPRTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureRecoverService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.RecoverCPROperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("recover_cpr_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/recover_cpr_triage.html")
	if err != nil {
		http.Error(w, "Failed to load RECOVER CPR Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := RecoverCPRPageData{
		Operations:                         ops,
		ActiveOperation:                    activeOp,
		TotalOperations:                    metrics.TotalOperations,
		ArrestInProgressCriticalCount:      metrics.ArrestInProgressCriticalCount,
		EpiDefibCycleStagedCount:           metrics.EpiDefibCycleStagedCount,
		ROSCPostArrestClearedCount:         metrics.ROSCPostArrestClearedCount,
		HyperventilationHighDoseAlertCount: metrics.HyperventilationHighDoseAlertCount,
		Locale:                             locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListRecoverCPROperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureRecoverService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.RecoverCPROperation, 0, len(ops))
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

func (s *Server) handleGetRecoverCPROperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureRecoverService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeRecoverCPRJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateRecoverCPROperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createRecoverCPROperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.CPRPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.CPRPatientProfile{
			Species:                       payload.Species,
			Breed:                         payload.Breed,
			WeightKg:                      payload.WeightKg,
			ArrestRhythm:                  payload.ArrestRhythm,
			EndTidalCO2MmHg:               payload.EndTidalCO2MmHg,
			CompressionCyclesCompleted:    payload.CompressionCyclesCompleted,
			ReversibleOpioidAlpha2Present: payload.ReversibleOpioidAlpha2Present,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-CPR-%d", 9940+len(s.ensureRecoverService().ListOperations(""))+1)
	}

	svc := s.ensureRecoverService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextRecoverCPRNano()).UTC()
	s.broadcastRecoverCPREvent("recover_cpr_updated", map[string]any{
		"type":      "recover_cpr_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageRecoverCPRPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureRecoverService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeRecoverCPRJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageRecoverCPRPlanPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if strings.TrimSpace(payload.ArrestRhythm) != "" {
			profile.ArrestRhythm = payload.ArrestRhythm
		}
		if payload.EndTidalCO2MmHg != 0 {
			profile.EndTidalCO2MmHg = payload.EndTidalCO2MmHg
		}
		if payload.CompressionCyclesCompleted != nil {
			profile.CompressionCyclesCompleted = *payload.CompressionCyclesCompleted
		}
		if payload.ReversibleOpioidAlpha2Present != nil {
			profile.ReversibleOpioidAlpha2Present = *payload.ReversibleOpioidAlpha2Present
		}
	}

	updated, err := svc.StageALSProtocolPlan(id, profile)
	if err != nil {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextRecoverCPRNano()).UTC()
	s.broadcastRecoverCPREvent("recover_cpr_updated", map[string]any{
		"type":      "recover_cpr_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordRecoverCPRCycleCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureRecoverService()
	if _, err := svc.GetOperation(id); err != nil {
		writeRecoverCPRJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.CPRTwoMinuteCycleCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordCycleCheck(id, check)
	if err != nil {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextRecoverCPRNano()).UTC()
	s.broadcastRecoverCPREvent("recover_cpr_updated", map[string]any{
		"type":      "recover_cpr_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueRecoverCPRROSCClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureRecoverService()
	if _, err := svc.GetOperation(id); err != nil {
		writeRecoverCPRJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.ROSCDischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueROSCClearance(id, clearance)
	if err != nil {
		writeRecoverCPRJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextRecoverCPRNano()).UTC()
	s.broadcastRecoverCPREvent("recover_cpr_updated", map[string]any{
		"type":      "recover_cpr_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecoverCPRStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("recover_cpr")
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

func (s *Server) broadcastRecoverCPREvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextRecoverCPRNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "recover_cpr",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "recover_cpr",
			SenderNodeID:  "recover-cpr-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
