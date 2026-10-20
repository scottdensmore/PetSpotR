package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pneumothorax"
)

// PneumothoraxPageData represents template data passed to templates/pneumothorax_thoracocentesis_triage.html.
type PneumothoraxPageData struct {
	Operations                     []domain.PneumothoraxOperation
	ActiveOperation                *domain.PneumothoraxOperation
	TotalOperations                int
	CriticalTensionCount           int
	ThoracocentesisTubeStagedCount int
	PleuralSealRemovedCount        int
	TensionRadiographAlertCount    int
	Locale                         string
}

type createPneumothoraxOperationPayload struct {
	IncidentID                 string                             `json:"incident_id"`
	PetName                    string                             `json:"pet_name"`
	PatientProfile             *domain.PneumothoraxPatientProfile `json:"patient_profile"`
	Species                    string                             `json:"species"`
	Breed                      string                             `json:"breed"`
	WeightKg                   float64                            `json:"weight_kg"`
	RespiratoryRateBpm         int                                `json:"respiratory_rate_bpm"`
	SpO2Percent                float64                            `json:"spo2_percent"`
	TfastGlideSignPresent      bool                               `json:"tfast_glide_sign_present"`
	CumulativeAirAspiratedMlKg float64                            `json:"cumulative_air_aspirated_ml_kg"`
	TapCountLast24Hr           int                                `json:"tap_count_last_24hr"`
	OpenSuckingChestWound      bool                               `json:"open_sucking_chest_wound"`
}

type stagePneumothoraxPlanPayload struct {
	PatientProfile             *domain.PneumothoraxPatientProfile `json:"patient_profile"`
	Species                    string                             `json:"species"`
	Breed                      string                             `json:"breed"`
	WeightKg                   float64                            `json:"weight_kg"`
	RespiratoryRateBpm         int                                `json:"respiratory_rate_bpm"`
	SpO2Percent                float64                            `json:"spo2_percent"`
	TfastGlideSignPresent      *bool                              `json:"tfast_glide_sign_present"`
	CumulativeAirAspiratedMlKg *float64                           `json:"cumulative_air_aspirated_ml_kg"`
	TapCountLast24Hr           *int                               `json:"tap_count_last_24hr"`
	OpenSuckingChestWound      *bool                              `json:"open_sucking_chest_wound"`
}

func (s *Server) ensurePneumothoraxService() *pneumothorax.PneumothoraxService {
	if s.pneumothoraxService == nil {
		s.pneumothoraxService = pneumothorax.NewPneumothoraxService()
	}
	return s.pneumothoraxService
}

func (s *Server) registerPneumothoraxRoutes() {
	s.mux.HandleFunc("GET /pneumothorax-thoracocentesis-triage", s.handlePneumothoraxTriagePage)

	s.registerDualAPIRoute("GET", "pneumothorax-thoracocentesis/operations", s.handleListPneumothoraxOperations)
	s.registerDualAPIRoute("POST", "pneumothorax-thoracocentesis/operations", s.handleCreatePneumothoraxOperation)
	s.registerDualAPIRoute("GET", "pneumothorax-thoracocentesis/operations/{id}", s.handleGetPneumothoraxOperation)
	s.registerDualAPIRoute("POST", "pneumothorax-thoracocentesis/operations/{id}/stage-plan", s.handleStagePneumothoraxPlan)
	s.registerDualAPIRoute("POST", "pneumothorax-thoracocentesis/operations/{id}/drainage-check", s.handleRecordPneumothoraxDrainageCheck)
	s.registerDualAPIRoute("POST", "pneumothorax-thoracocentesis/operations/{id}/removal-clearance", s.handleIssuePneumothoraxRemovalClearance)
	s.registerDualAPIRoute("GET", "pneumothorax-thoracocentesis/stream", s.handlePneumothoraxStream)
}

func (s *Server) handlePneumothoraxTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePneumothoraxService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.PneumothoraxOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := PneumothoraxPageData{
		Operations:                     ops,
		ActiveOperation:                activeOp,
		TotalOperations:                metrics.TotalOperations,
		CriticalTensionCount:           metrics.CriticalTensionCount,
		ThoracocentesisTubeStagedCount: metrics.ThoracocentesisTubeStagedCount,
		PleuralSealRemovedCount:        metrics.PleuralSealRemovedCount,
		TensionRadiographAlertCount:    metrics.TensionRadiographAlertCount,
		Locale:                         locale,
	}

	s.renderCockpitTemplate(w, r, "pneumothorax_thoracocentesis_triage.html", "Pneumothorax & Thoracocentesis Triage", data)
}

func (s *Server) handleListPneumothoraxOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensurePneumothoraxService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.PneumothoraxOperation, 0, len(ops))
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

func (s *Server) handleGetPneumothoraxOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensurePneumothoraxService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreatePneumothoraxOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createPneumothoraxOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.PneumothoraxPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.PneumothoraxPatientProfile{
			Species:                    payload.Species,
			Breed:                      payload.Breed,
			WeightKg:                   payload.WeightKg,
			RespiratoryRateBpm:         payload.RespiratoryRateBpm,
			SpO2Percent:                payload.SpO2Percent,
			TfastGlideSignPresent:      payload.TfastGlideSignPresent,
			CumulativeAirAspiratedMlKg: payload.CumulativeAirAspiratedMlKg,
			TapCountLast24Hr:           payload.TapCountLast24Hr,
			OpenSuckingChestWound:      payload.OpenSuckingChestWound,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-PTX-%d", 9910+len(s.ensurePneumothoraxService().ListOperations(""))+1)
	}

	svc := s.ensurePneumothoraxService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPneumothoraxEvent("pneumothorax_updated", map[string]any{
		"type":      "pneumothorax_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStagePneumothoraxPlan(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensurePneumothoraxService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stagePneumothoraxPlanPayload
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
		if payload.RespiratoryRateBpm != 0 {
			profile.RespiratoryRateBpm = payload.RespiratoryRateBpm
		}
		if payload.SpO2Percent != 0 {
			profile.SpO2Percent = payload.SpO2Percent
		}
		if payload.TfastGlideSignPresent != nil {
			profile.TfastGlideSignPresent = *payload.TfastGlideSignPresent
		}
		if payload.CumulativeAirAspiratedMlKg != nil {
			profile.CumulativeAirAspiratedMlKg = *payload.CumulativeAirAspiratedMlKg
		}
		if payload.TapCountLast24Hr != nil {
			profile.TapCountLast24Hr = *payload.TapCountLast24Hr
		}
		if payload.OpenSuckingChestWound != nil {
			profile.OpenSuckingChestWound = *payload.OpenSuckingChestWound
		}
	}

	updated, err := svc.StageDecompressionPlan(id, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPneumothoraxEvent("pneumothorax_updated", map[string]any{
		"type":      "pneumothorax_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordPneumothoraxDrainageCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensurePneumothoraxService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.PleuralDrainageSerialCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordDrainageCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPneumothoraxEvent("pneumothorax_updated", map[string]any{
		"type":      "pneumothorax_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssuePneumothoraxRemovalClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensurePneumothoraxService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.PneumothoraxRemovalClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueRemovalClearance(id, clearance)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPneumothoraxEvent("pneumothorax_updated", map[string]any{
		"type":      "pneumothorax_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handlePneumothoraxStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "pneumothorax")
}

func (s *Server) broadcastPneumothoraxEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("pneumothorax", "pneumothorax-hub", eventType, payload, now)
}
