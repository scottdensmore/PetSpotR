package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/traumaticbrain"
)

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

	s.registerDualAPIRoute("GET", "traumatic-brain/operations", s.handleListTraumaticBrainOperations)
	s.registerDualAPIRoute("POST", "traumatic-brain/operations", s.handleCreateTraumaticBrainOperation)
	s.registerDualAPIRoute("GET", "traumatic-brain/operations/{id}", s.handleGetTraumaticBrainOperation)
	s.registerDualAPIRoute("POST", "traumatic-brain/operations/{id}/stage-plan", s.handleStageTraumaticBrainPlan)
	s.registerDualAPIRoute("POST", "traumatic-brain/operations/{id}/neuro-check", s.handleRecordTraumaticBrainCheck)
	s.registerDualAPIRoute("POST", "traumatic-brain/operations/{id}/discharge-clearance", s.handleIssueTraumaticBrainDischargeClearance)
	s.registerDualAPIRoute("GET", "traumatic-brain/stream", s.handleTraumaticBrainStream)
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

	s.renderCockpitTemplate(w, r, "traumatic_brain_triage.html", "Traumatic Brain Triage", data)
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

	writeCockpitJSON(w, http.StatusOK, map[string]any{
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
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTBIService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
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
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastTraumaticBrainEvent("traumatic_brain_updated", map[string]any{
		"type":      "traumatic_brain_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageTraumaticBrainPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTBIService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageTBIPlanPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastTraumaticBrainEvent("traumatic_brain_updated", map[string]any{
		"type":      "traumatic_brain_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordTraumaticBrainCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTBIService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.TBINeuroSerialCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordNeuroCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastTraumaticBrainEvent("traumatic_brain_updated", map[string]any{
		"type":      "traumatic_brain_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueTraumaticBrainDischargeClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureTBIService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.TBIDischargeClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDischargeClearance(id, clearance)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastTraumaticBrainEvent("traumatic_brain_updated", map[string]any{
		"type":      "traumatic_brain_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleTraumaticBrainStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "traumatic_brain")
}

func (s *Server) broadcastTraumaticBrainEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("traumatic_brain", "traumatic-brain-hub", eventType, payload, now)
}
