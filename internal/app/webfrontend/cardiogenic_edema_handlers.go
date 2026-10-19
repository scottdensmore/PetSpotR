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

	"github.com/scottdensmore/petspotr/pkg/cardiogenicedema"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastCardiogenicEdemaNano atomic.Int64

func nextCardiogenicEdemaNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastCardiogenicEdemaNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastCardiogenicEdemaNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// CardiogenicEdemaPageData represents template data passed to templates/cardiogenic_edema_triage.html.
type CardiogenicEdemaPageData struct {
	Operations        []domain.CardiogenicEdemaOperation
	ActiveOperation   *domain.CardiogenicEdemaOperation
	Metrics           cardiogenicedema.CardiogenicMetrics
	TotalOperations   int
	FulminantCritical int
	FurosemideStaged  int
	EupneicCleared    int
	FluidGuardLocks   int
	Locale            string
}

type createCardiogenicOperationRequest struct {
	IncidentID             string                            `json:"incident_id"`
	PetName                string                            `json:"pet_name"`
	PatientProfile         *domain.CardiogenicPatientProfile `json:"patient_profile,omitempty"`
	Species                string                            `json:"species"`
	WeightKg               float64                           `json:"weight_kg"`
	RespiratoryRateBpm     int                               `json:"respiratory_rate_bpm"`
	HeartRateBpm           int                               `json:"heart_rate_bpm"`
	MurmurGrade            string                            `json:"murmur_grade"`
	TfastBLinesPerSite     int                               `json:"tfast_b_lines_per_site"`
	LaAoRatio              float64                           `json:"la_ao_ratio"`
	PleuralEffusionPresent bool                              `json:"pleural_effusion_present"`
}

func (s *Server) ensureCardiogenicService() *cardiogenicedema.CardiogenicService {
	if s.cardiogenicService == nil {
		s.cardiogenicService = cardiogenicedema.NewCardiogenicService()
	}
	return s.cardiogenicService
}

func (s *Server) registerCardiogenicEdemaRoutes() {
	s.mux.HandleFunc("GET /cardiogenic-edema-triage", s.handleCardiogenicEdemaTriagePage)
	s.mux.HandleFunc("GET /api/cardiogenic-edema/operations", s.handleListCardiogenicEdemaOperations)
	s.mux.HandleFunc("POST /api/cardiogenic-edema/operations", s.handleCreateCardiogenicEdemaOperation)
	s.mux.HandleFunc("GET /api/cardiogenic-edema/operations/{id}", s.handleGetCardiogenicEdemaOperation)
	s.mux.HandleFunc("POST /api/cardiogenic-edema/operations/{id}/stage-regimen", s.handleStageCardiogenicRegimen)
	s.mux.HandleFunc("POST /api/cardiogenic-edema/operations/{id}/diuresis-check", s.handleRecordCardiogenicDiuresisCheck)
	s.mux.HandleFunc("POST /api/cardiogenic-edema/operations/{id}/discharge-clearance", s.handleIssueCardiogenicDischargeClearance)
	s.mux.HandleFunc("GET /api/cardiogenic-edema/stream", s.handleCardiogenicEdemaStream)
}

func writeCardiogenicJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleCardiogenicEdemaTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureCardiogenicService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.CardiogenicEdemaOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("cardiogenic_edema_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/cardiogenic_edema_triage.html")
	if err != nil {
		http.Error(w, "Failed to load Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture & Oxygen Cage Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := CardiogenicEdemaPageData{
		Operations:        ops,
		ActiveOperation:   activeOp,
		Metrics:           metrics,
		TotalOperations:   metrics.TotalOperations,
		FulminantCritical: metrics.FulminantCritical,
		FurosemideStaged:  metrics.FurosemideO2Staged,
		EupneicCleared:    metrics.EupneicCleared,
		FluidGuardLocks:   metrics.ActiveFluidGuardLock,
		Locale:            locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListCardiogenicEdemaOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureCardiogenicService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	ops := svc.ListOperations(statusFilter)
	metrics := svc.ComputeMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
	})
}

func (s *Server) handleGetCardiogenicEdemaOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCardiogenicJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureCardiogenicService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCardiogenicJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateCardiogenicEdemaOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createCardiogenicOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCardiogenicJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.CardiogenicPatientProfile
	if req.PatientProfile != nil {
		profile = *req.PatientProfile
	} else {
		profile = domain.CardiogenicPatientProfile{
			Species:                req.Species,
			WeightKg:               req.WeightKg,
			RespiratoryRateBpm:     req.RespiratoryRateBpm,
			HeartRateBpm:           req.HeartRateBpm,
			MurmurGrade:            req.MurmurGrade,
			TfastBLinesPerSite:     req.TfastBLinesPerSite,
			LaAoRatio:              req.LaAoRatio,
			PleuralEffusionPresent: req.PleuralEffusionPresent,
		}
	}

	svc := s.ensureCardiogenicService()
	created, err := svc.CreateOperation(req.IncidentID, req.PetName, profile)
	if err != nil {
		writeCardiogenicJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCardiogenicEdemaNano()).UTC()
	s.broadcastCardiogenicEdemaEvent("cardiogenic_edema_updated", map[string]any{
		"type":      "cardiogenic_edema_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageCardiogenicRegimen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCardiogenicJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureCardiogenicService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCardiogenicJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var body struct {
		PatientProfile *domain.CardiogenicPatientProfile `json:"patient_profile,omitempty"`
		domain.CardiogenicPatientProfile
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeCardiogenicJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	profile := existing.PatientProfile
	if body.PatientProfile != nil {
		profile = *body.PatientProfile
	} else if strings.TrimSpace(body.Species) != "" || body.WeightKg != 0 || body.RespiratoryRateBpm != 0 {
		profile = body.CardiogenicPatientProfile
	}

	updated, err := svc.StageDiureticRegimen(id, profile)
	if err != nil {
		writeCardiogenicJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCardiogenicEdemaNano()).UTC()
	s.broadcastCardiogenicEdemaEvent("cardiogenic_edema_updated", map[string]any{
		"type":      "cardiogenic_edema_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordCardiogenicDiuresisCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCardiogenicJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureCardiogenicService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCardiogenicJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.DiuresisTelemetryCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCardiogenicJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordDiuresisCheck(id, check)
	if err != nil {
		writeCardiogenicJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCardiogenicEdemaNano()).UTC()
	s.broadcastCardiogenicEdemaEvent("cardiogenic_edema_updated", map[string]any{
		"type":      "cardiogenic_edema_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueCardiogenicDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCardiogenicJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureCardiogenicService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCardiogenicJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.CardiogenicDischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeCardiogenicJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writeCardiogenicJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCardiogenicEdemaNano()).UTC()
	s.broadcastCardiogenicEdemaEvent("cardiogenic_edema_updated", map[string]any{
		"type":      "cardiogenic_edema_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleCardiogenicEdemaStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("cardiogenic_edema")
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

func (s *Server) broadcastCardiogenicEdemaEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextCardiogenicEdemaNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "cardiogenic_edema",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "cardiogenic_edema",
			SenderNodeID:  "cardiogenic-edema-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
