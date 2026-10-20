package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/septicperitonitis"
)

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

	s.registerDualAPIRoute("GET", "septic-peritonitis/operations", s.handleListSepticPeritonitisOperations)
	s.registerDualAPIRoute("POST", "septic-peritonitis/operations", s.handleCreateSepticPeritonitisOperation)
	s.registerDualAPIRoute("GET", "septic-peritonitis/operations/{id}", s.handleGetSepticPeritonitisOperation)
	s.registerDualAPIRoute("POST", "septic-peritonitis/operations/{id}/stage-plan", s.handleStageSepticPeritonitisPlan)
	s.registerDualAPIRoute("POST", "septic-peritonitis/operations/{id}/drain-check", s.handleRecordSepticPeritonitisDrainCheck)
	s.registerDualAPIRoute("POST", "septic-peritonitis/operations/{id}/drain-clearance", s.handleIssueSepticPeritonitisDrainClearance)
	s.registerDualAPIRoute("GET", "septic-peritonitis/stream", s.handleSepticPeritonitisStream)
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

	s.renderCockpitTemplate(w, r, "septic_peritonitis_triage.html", "Septic Peritonitis Triage", data)
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

	writeCockpitJSON(w, http.StatusOK, map[string]any{
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
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensurePeritonitisService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
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
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastSepticPeritonitisEvent("septic_peritonitis_updated", map[string]any{
		"type":      "septic_peritonitis_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageSepticPeritonitisPlan(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensurePeritonitisService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stagePeritonitisPlanPayload
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
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastSepticPeritonitisEvent("septic_peritonitis_updated", map[string]any{
		"type":      "septic_peritonitis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordSepticPeritonitisDrainCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensurePeritonitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.JacksonPrattDrainCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordDrainCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastSepticPeritonitisEvent("septic_peritonitis_updated", map[string]any{
		"type":      "septic_peritonitis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueSepticPeritonitisDrainClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensurePeritonitisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.PeritonitisDrainRemovalClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueDrainClearance(id, clearance)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastSepticPeritonitisEvent("septic_peritonitis_updated", map[string]any{
		"type":      "septic_peritonitis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleSepticPeritonitisStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "septic_peritonitis")
}

func (s *Server) broadcastSepticPeritonitisEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("septic_peritonitis", "septic-peritonitis-hub", eventType, payload, now)
}
