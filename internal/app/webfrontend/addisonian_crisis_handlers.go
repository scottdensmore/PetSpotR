package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/addisoniancrisis"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

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

	s.registerDualAPIRoute("GET", "addisonian-crisis/operations", s.handleListAddisonianCrisisOperations)
	s.registerDualAPIRoute("POST", "addisonian-crisis/operations", s.handleCreateAddisonianCrisisOperation)
	s.registerDualAPIRoute("GET", "addisonian-crisis/operations/{id}", s.handleGetAddisonianCrisisOperation)
	s.registerDualAPIRoute("POST", "addisonian-crisis/operations/{id}/stage-regimen", s.handleStageAddisonianCrisisRegimen)
	s.registerDualAPIRoute("POST", "addisonian-crisis/operations/{id}/electrolyte-check", s.handleRecordAddisonianCrisisElectrolyteCheck)
	s.registerDualAPIRoute("POST", "addisonian-crisis/operations/{id}/discharge-clearance", s.handleIssueAddisonianCrisisDischargeClearance)
	s.registerDualAPIRoute("GET", "addisonian-crisis/stream", s.handleAddisonianCrisisStream)
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

	s.renderCockpitTemplate(w, r, "addisonian_crisis_triage.html", "Addisonian Crisis Triage", data)
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

	writeCockpitJSON(w, http.StatusOK, map[string]any{
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
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAddisonianService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
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
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAddisonianCrisisEvent("addisonian_crisis_updated", map[string]any{
		"type":      "addisonian_crisis_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageAddisonianCrisisRegimen(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAddisonianService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageAddisonianRegimenPayload
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
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAddisonianCrisisEvent("addisonian_crisis_updated", map[string]any{
		"type":      "addisonian_crisis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordAddisonianCrisisElectrolyteCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAddisonianService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.AddisonianElectrolyteCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordElectrolyteCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAddisonianCrisisEvent("addisonian_crisis_updated", map[string]any{
		"type":      "addisonian_crisis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueAddisonianCrisisDischargeClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAddisonianService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.AddisonianDischargeClearance
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
	s.broadcastAddisonianCrisisEvent("addisonian_crisis_updated", map[string]any{
		"type":      "addisonian_crisis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAddisonianCrisisStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "addisonian_crisis")
}

func (s *Server) broadcastAddisonianCrisisEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("addisonian_crisis", "addisonian-crisis-hub", eventType, payload, now)
}
