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

	"github.com/scottdensmore/petspotr/pkg/dkainsulin"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
)

var lastDKAInsulinNano atomic.Int64

func nextDKAInsulinNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastDKAInsulinNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastDKAInsulinNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// DKAInsulinPageData represents template data passed to templates/dka_insulin_triage.html.
type DKAInsulinPageData struct {
	Operations                 []domain.DKAInsulinOperation
	ActiveOperation            *domain.DKAInsulinOperation
	TotalOperations            int
	CriticalAcidosisCount      int
	InsulinCRIStagedCount      int
	KetoneClearedCount         int
	HypokalemiaEdemaAlertCount int
	Locale                     string
}

type createDKAOperationPayload struct {
	IncidentID                string                    `json:"incident_id"`
	PetName                   string                    `json:"pet_name"`
	PatientProfile            *domain.DKAPatientProfile `json:"patient_profile"`
	Species                   string                    `json:"species"`
	Breed                     string                    `json:"breed"`
	WeightKg                  float64                   `json:"weight_kg"`
	BloodGlucoseMgDl          float64                   `json:"blood_glucose_mg_dl"`
	BetaHydroxybutyrateMmolL  float64                   `json:"beta_hydroxybutyrate_mmol_l"`
	VenousPH                  float64                   `json:"venous_ph"`
	SerumPotassiumMeqL        float64                   `json:"serum_potassium_meq_l"`
	SerumPhosphorusMgDl       float64                   `json:"serum_phosphorus_mg_dl"`
	EffectiveOsmolalityMosmKg float64                   `json:"effective_osmolality_mosm_kg"`
	DehydrationPercent        float64                   `json:"dehydration_percent"`
}

type stageDKARegimenPayload struct {
	PatientProfile            *domain.DKAPatientProfile `json:"patient_profile"`
	Species                   string                    `json:"species"`
	Breed                     string                    `json:"breed"`
	WeightKg                  float64                   `json:"weight_kg"`
	BloodGlucoseMgDl          float64                   `json:"blood_glucose_mg_dl"`
	BetaHydroxybutyrateMmolL  *float64                  `json:"beta_hydroxybutyrate_mmol_l"`
	VenousPH                  float64                   `json:"venous_ph"`
	SerumPotassiumMeqL        float64                   `json:"serum_potassium_meq_l"`
	SerumPhosphorusMgDl       float64                   `json:"serum_phosphorus_mg_dl"`
	EffectiveOsmolalityMosmKg float64                   `json:"effective_osmolality_mosm_kg"`
	DehydrationPercent        *float64                  `json:"dehydration_percent"`
}

func (s *Server) ensureDKAService() *dkainsulin.DKAService {
	if s.dkaService == nil {
		s.dkaService = dkainsulin.NewDKAService()
	}
	return s.dkaService
}

func (s *Server) registerDKAInsulinRoutes() {
	s.mux.HandleFunc("GET /dka-insulin-triage", s.handleDKAInsulinTriagePage)

	s.mux.HandleFunc("GET /api/dka-insulin/operations", s.handleListDKAInsulinOperations)
	s.mux.HandleFunc("POST /api/dka-insulin/operations", s.handleCreateDKAInsulinOperation)
	s.mux.HandleFunc("GET /api/dka-insulin/operations/{id}", s.handleGetDKAInsulinOperation)
	s.mux.HandleFunc("POST /api/dka-insulin/operations/{id}/stage-regimen", s.handleStageDKAInsulinRegimen)
	s.mux.HandleFunc("POST /api/dka-insulin/operations/{id}/serial-check", s.handleRecordDKAInsulinSerialCheck)
	s.mux.HandleFunc("POST /api/dka-insulin/operations/{id}/discharge-clearance", s.handleIssueDKAInsulinDischargeClearance)
	s.mux.HandleFunc("GET /api/dka-insulin/stream", s.handleDKAInsulinStream)

	// v1 aliases for consistency
	s.mux.HandleFunc("GET /api/v1/dka-insulin/operations", s.handleListDKAInsulinOperations)
	s.mux.HandleFunc("POST /api/v1/dka-insulin/operations", s.handleCreateDKAInsulinOperation)
	s.mux.HandleFunc("GET /api/v1/dka-insulin/operations/{id}", s.handleGetDKAInsulinOperation)
	s.mux.HandleFunc("POST /api/v1/dka-insulin/operations/{id}/stage-regimen", s.handleStageDKAInsulinRegimen)
	s.mux.HandleFunc("POST /api/v1/dka-insulin/operations/{id}/serial-check", s.handleRecordDKAInsulinSerialCheck)
	s.mux.HandleFunc("POST /api/v1/dka-insulin/operations/{id}/discharge-clearance", s.handleIssueDKAInsulinDischargeClearance)
	s.mux.HandleFunc("GET /api/v1/dka-insulin/stream", s.handleDKAInsulinStream)
}

func writeDKAInsulinJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func (s *Server) handleDKAInsulinTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureDKAService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.DKAInsulinOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	tmpl, err := template.New("dka_insulin_triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/dka_insulin_triage.html")
	if err != nil {
		http.Error(w, "Failed to load DKA Insulin Triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := DKAInsulinPageData{
		Operations:                 ops,
		ActiveOperation:            activeOp,
		TotalOperations:            metrics.TotalOperations,
		CriticalAcidosisCount:      metrics.CriticalAcidosisCount,
		InsulinCRIStagedCount:      metrics.InsulinCRIStagedCount,
		KetoneClearedCount:         metrics.KetoneClearedCount,
		HypokalemiaEdemaAlertCount: metrics.HypokalemiaEdemaAlertCount,
		Locale:                     locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListDKAInsulinOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureDKAService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.DKAInsulinOperation, 0, len(ops))
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

func (s *Server) handleGetDKAInsulinOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureDKAService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeDKAInsulinJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(op)
}

func (s *Server) handleCreateDKAInsulinOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createDKAOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.DKAPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.DKAPatientProfile{
			Species:                   payload.Species,
			Breed:                     payload.Breed,
			WeightKg:                  payload.WeightKg,
			BloodGlucoseMgDl:          payload.BloodGlucoseMgDl,
			BetaHydroxybutyrateMmolL:  payload.BetaHydroxybutyrateMmolL,
			VenousPH:                  payload.VenousPH,
			SerumPotassiumMeqL:        payload.SerumPotassiumMeqL,
			SerumPhosphorusMgDl:       payload.SerumPhosphorusMgDl,
			EffectiveOsmolalityMosmKg: payload.EffectiveOsmolalityMosmKg,
			DehydrationPercent:        payload.DehydrationPercent,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-DKA-%d", 9300+len(s.ensureDKAService().ListOperations(""))+1)
	}

	svc := s.ensureDKAService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextDKAInsulinNano()).UTC()
	s.broadcastDKAInsulinEvent("dka_insulin_updated", map[string]any{
		"type":      "dka_insulin_updated",
		"operation": created,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleStageDKAInsulinRegimen(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureDKAService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeDKAInsulinJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageDKARegimenPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		if payload.BetaHydroxybutyrateMmolL != nil {
			profile.BetaHydroxybutyrateMmolL = *payload.BetaHydroxybutyrateMmolL
		}
		if payload.VenousPH != 0 {
			profile.VenousPH = payload.VenousPH
		}
		if payload.SerumPotassiumMeqL != 0 {
			profile.SerumPotassiumMeqL = payload.SerumPotassiumMeqL
		}
		if payload.SerumPhosphorusMgDl != 0 {
			profile.SerumPhosphorusMgDl = payload.SerumPhosphorusMgDl
		}
		if payload.EffectiveOsmolalityMosmKg != 0 {
			profile.EffectiveOsmolalityMosmKg = payload.EffectiveOsmolalityMosmKg
		}
		if payload.DehydrationPercent != nil {
			profile.DehydrationPercent = *payload.DehydrationPercent
		}
	}

	updated, err := svc.StageInsulinRegimen(id, profile)
	if err != nil {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextDKAInsulinNano()).UTC()
	s.broadcastDKAInsulinEvent("dka_insulin_updated", map[string]any{
		"type":      "dka_insulin_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleRecordDKAInsulinSerialCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureDKAService()
	if _, err := svc.GetOperation(id); err != nil {
		writeDKAInsulinJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.DKAGlucoseElectrolyteCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordSerialCheck(id, check)
	if err != nil {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextDKAInsulinNano()).UTC()
	s.broadcastDKAInsulinEvent("dka_insulin_updated", map[string]any{
		"type":      "dka_insulin_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleIssueDKAInsulinDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureDKAService()
	if _, err := svc.GetOperation(id); err != nil {
		writeDKAInsulinJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.DKADischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writeDKAInsulinJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextDKAInsulinNano()).UTC()
	s.broadcastDKAInsulinEvent("dka_insulin_updated", map[string]any{
		"type":      "dka_insulin_updated",
		"operation": updated,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

func (s *Server) handleDKAInsulinStream(w http.ResponseWriter, r *http.Request) {
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

	subCh, unsubscribe := s.reunionHub.Subscribe("dka_insulin")
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

func (s *Server) broadcastDKAInsulinEvent(eventType string, payload map[string]any, now time.Time) {
	seqNano := nextDKAInsulinNano()
	if s.reunionHub != nil {
		evt := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%d", eventType, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "dka_insulin",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(evt)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		env := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "dka_insulin",
			SenderNodeID:  "dka-insulin-hub",
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(env)
	}
}
