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
	"github.com/scottdensmore/petspotr/pkg/imhathrombosis"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastIMHAThrombosisNano atomic.Int64

func nextIMHAThrombosisNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastIMHAThrombosisNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastIMHAThrombosisNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// IMHAThrombosisPageData represents template data passed to templates/imha_thrombosis_triage.html.
type IMHAThrombosisPageData struct {
	Operations             []domain.IMHAThrombosisOperation
	ActiveOperation        *domain.IMHAThrombosisOperation
	TotalOperations        int
	CriticalHemolyticCount int
	StagedRegimenCount     int
	ClearedStableCount     int
	PTEAlertCount          int
	Locale                 string
}

type createIMHAOperationPayload struct {
	IncidentID                      string                     `json:"incident_id"`
	PetName                         string                     `json:"pet_name"`
	PatientProfile                  *domain.IMHAPatientProfile `json:"patient_profile"`
	Species                         string                     `json:"species"`
	Breed                           string                     `json:"breed"`
	WeightKg                        float64                    `json:"weight_kg"`
	PackedCellVolumePercent         float64                    `json:"packed_cell_volume_percent"`
	TotalBilirubinMgDl              float64                    `json:"total_bilirubin_mg_dl"`
	PlateletCountKUl                float64                    `json:"platelet_count_k_ul"`
	SpherocytesPerHPF               int                        `json:"spherocytes_per_hpf"`
	SalineAgglutination4To1Positive bool                       `json:"saline_agglutination_4_to_1_positive"`
	IntravascularHemoglobinuria     bool                       `json:"intravascular_hemoglobinuria"`
}

type stageIMHARegimenPayload struct {
	PatientProfile                  *domain.IMHAPatientProfile `json:"patient_profile"`
	Species                         string                     `json:"species"`
	Breed                           string                     `json:"breed"`
	WeightKg                        float64                    `json:"weight_kg"`
	PackedCellVolumePercent         float64                    `json:"packed_cell_volume_percent"`
	TotalBilirubinMgDl              *float64                   `json:"total_bilirubin_mg_dl"`
	PlateletCountKUl                *float64                   `json:"platelet_count_k_ul"`
	SpherocytesPerHPF               *int                       `json:"spherocytes_per_hpf"`
	SalineAgglutination4To1Positive *bool                      `json:"saline_agglutination_4_to_1_positive"`
	IntravascularHemoglobinuria     *bool                      `json:"intravascular_hemoglobinuria"`
}

func (s *Server) ensureIMHAService() *imhathrombosis.IMHAService {
	if s.imhaService == nil {
		s.imhaService = imhathrombosis.NewIMHAService()
	}
	return s.imhaService
}

func (s *Server) registerIMHAThrombosisRoutes() {
	s.mux.HandleFunc("GET /imha-thrombosis-triage", s.handleIMHAThrombosisTriagePage)

	s.mux.HandleFunc("GET /api/imha-thrombosis/operations", s.handleListIMHAThrombosisOperations)
	s.mux.HandleFunc("POST /api/imha-thrombosis/operations", s.handleCreateIMHAThrombosisOperation)
	s.mux.HandleFunc("GET /api/imha-thrombosis/operations/{id}", s.handleGetIMHAThrombosisOperation)
	s.mux.HandleFunc("POST /api/imha-thrombosis/operations/{id}/stage-regimen", s.handleStageIMHAThrombosisRegimen)
	s.mux.HandleFunc("POST /api/imha-thrombosis/operations/{id}/hemolysis-check", s.handleRecordIMHAThrombosisHemolysisCheck)
	s.mux.HandleFunc("POST /api/imha-thrombosis/operations/{id}/discharge-clearance", s.handleIssueIMHAThrombosisDischargeClearance)
	s.mux.HandleFunc("GET /api/imha-thrombosis/stream", s.handleIMHAThrombosisStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/imha-thrombosis/operations", s.handleListIMHAThrombosisOperations)
	s.mux.HandleFunc("POST /api/v1/imha-thrombosis/operations", s.handleCreateIMHAThrombosisOperation)
	s.mux.HandleFunc("GET /api/v1/imha-thrombosis/operations/{id}", s.handleGetIMHAThrombosisOperation)
	s.mux.HandleFunc("POST /api/v1/imha-thrombosis/operations/{id}/stage-regimen", s.handleStageIMHAThrombosisRegimen)
	s.mux.HandleFunc("POST /api/v1/imha-thrombosis/operations/{id}/hemolysis-check", s.handleRecordIMHAThrombosisHemolysisCheck)
	s.mux.HandleFunc("POST /api/v1/imha-thrombosis/operations/{id}/discharge-clearance", s.handleIssueIMHAThrombosisDischargeClearance)
	s.mux.HandleFunc("GET /api/v1/imha-thrombosis/stream", s.handleIMHAThrombosisStream)
}

func writeIMHAThrombosisJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleIMHAThrombosisTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureIMHAService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.IMHAThrombosisOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("imha_thrombosis_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/imha_thrombosis_triage.html")
	if err != nil {
		http.Error(w, "Failed to load IMHA Thrombosis Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := IMHAThrombosisPageData{
		Operations:             ops,
		ActiveOperation:        activeOp,
		TotalOperations:        metrics.TotalOperations,
		CriticalHemolyticCount: metrics.CriticalHemolyticCount,
		StagedRegimenCount:     metrics.StagedRegimenCount,
		ClearedStableCount:     metrics.ClearedStableCount,
		PTEAlertCount:          metrics.PTEAlertCount,
		Locale:                 locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListIMHAThrombosisOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureIMHAService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.IMHAThrombosisOperation, 0, len(ops))
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

func (s *Server) handleGetIMHAThrombosisOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureIMHAService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateIMHAThrombosisOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createIMHAOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.IMHAPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.IMHAPatientProfile{
			Species:                         payload.Species,
			Breed:                           payload.Breed,
			WeightKg:                        payload.WeightKg,
			PackedCellVolumePercent:         payload.PackedCellVolumePercent,
			TotalBilirubinMgDl:              payload.TotalBilirubinMgDl,
			PlateletCountKUl:                payload.PlateletCountKUl,
			SpherocytesPerHPF:               payload.SpherocytesPerHPF,
			SalineAgglutination4To1Positive: payload.SalineAgglutination4To1Positive,
			IntravascularHemoglobinuria:     payload.IntravascularHemoglobinuria,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-IM-%d", 9700+len(s.ensureIMHAService().ListOperations(""))+1)
	}

	svc := s.ensureIMHAService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextIMHAThrombosisNano()).UTC()
	s.broadcastIMHAThrombosisEvent("imha_thrombosis_updated", map[string]any{
		"type":      "imha_thrombosis_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageIMHAThrombosisRegimen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureIMHAService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageIMHARegimenPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if payload.PackedCellVolumePercent != 0 {
			profile.PackedCellVolumePercent = payload.PackedCellVolumePercent
		}
		if payload.TotalBilirubinMgDl != nil {
			profile.TotalBilirubinMgDl = *payload.TotalBilirubinMgDl
		}
		if payload.PlateletCountKUl != nil {
			profile.PlateletCountKUl = *payload.PlateletCountKUl
		}
		if payload.SpherocytesPerHPF != nil {
			profile.SpherocytesPerHPF = *payload.SpherocytesPerHPF
		}
		if payload.SalineAgglutination4To1Positive != nil {
			profile.SalineAgglutination4To1Positive = *payload.SalineAgglutination4To1Positive
		}
		if payload.IntravascularHemoglobinuria != nil {
			profile.IntravascularHemoglobinuria = *payload.IntravascularHemoglobinuria
		}
	}

	updated, err := svc.StageImmunosuppressiveRegimen(id, profile)
	if err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextIMHAThrombosisNano()).UTC()
	s.broadcastIMHAThrombosisEvent("imha_thrombosis_updated", map[string]any{
		"type":      "imha_thrombosis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordIMHAThrombosisHemolysisCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureIMHAService()
	if _, err := svc.GetOperation(id); err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.IMHAHemolysisSerialCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordHemolysisCheck(id, check)
	if err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextIMHAThrombosisNano()).UTC()
	s.broadcastIMHAThrombosisEvent("imha_thrombosis_updated", map[string]any{
		"type":      "imha_thrombosis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueIMHAThrombosisDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureIMHAService()
	if _, err := svc.GetOperation(id); err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.IMHADischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writeIMHAThrombosisJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextIMHAThrombosisNano()).UTC()
	s.broadcastIMHAThrombosisEvent("imha_thrombosis_updated", map[string]any{
		"type":      "imha_thrombosis_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIMHAThrombosisStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("imha_thrombosis")
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

func (s *Server) broadcastIMHAThrombosisEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextIMHAThrombosisNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "imha_thrombosis",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "imha_thrombosis",
			SenderNodeID:  "imha-thrombosis-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
