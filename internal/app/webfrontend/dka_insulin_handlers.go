package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/dkainsulin"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

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

	s.registerDualAPIRoute("GET", "dka-insulin/operations", s.handleListDKAInsulinOperations)
	s.registerDualAPIRoute("POST", "dka-insulin/operations", s.handleCreateDKAInsulinOperation)
	s.registerDualAPIRoute("GET", "dka-insulin/operations/{id}", s.handleGetDKAInsulinOperation)
	s.registerDualAPIRoute("POST", "dka-insulin/operations/{id}/stage-regimen", s.handleStageDKAInsulinRegimen)
	s.registerDualAPIRoute("POST", "dka-insulin/operations/{id}/serial-check", s.handleRecordDKAInsulinSerialCheck)
	s.registerDualAPIRoute("POST", "dka-insulin/operations/{id}/discharge-clearance", s.handleIssueDKAInsulinDischargeClearance)
	s.registerDualAPIRoute("GET", "dka-insulin/stream", s.handleDKAInsulinStream)
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

	s.renderCockpitTemplate(w, r, "dka_insulin_triage.html", "DKA Insulin Triage", data)
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

	writeCockpitJSON(w, http.StatusOK, map[string]any{
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
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureDKAService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
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
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
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
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastDKAInsulinEvent("dka_insulin_updated", map[string]any{
		"type":      "dka_insulin_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageDKAInsulinRegimen(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureDKAService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageDKARegimenPayload
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
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastDKAInsulinEvent("dka_insulin_updated", map[string]any{
		"type":      "dka_insulin_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordDKAInsulinSerialCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureDKAService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.DKAGlucoseElectrolyteCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordSerialCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastDKAInsulinEvent("dka_insulin_updated", map[string]any{
		"type":      "dka_insulin_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueDKAInsulinDischargeClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureDKAService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.DKADischargeClearance
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
	s.broadcastDKAInsulinEvent("dka_insulin_updated", map[string]any{
		"type":      "dka_insulin_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDKAInsulinStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "dka_insulin")
}

func (s *Server) broadcastDKAInsulinEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("dka_insulin", "dka-insulin-hub", eventType, payload, now)
}
