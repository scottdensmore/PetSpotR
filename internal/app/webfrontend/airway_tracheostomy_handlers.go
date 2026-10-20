package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/airwaytracheostomy"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// AirwayTracheostomyPageData represents template data passed to templates/airway_tracheostomy_triage.html.
type AirwayTracheostomyPageData struct {
	Operations      []domain.AirwayTracheostomyOperation
	ActiveOperation *domain.AirwayTracheostomyOperation
	Metrics         airwaytracheostomy.AirwayCockpitMetrics
	Locale          string
}

type createAirwayOperationRequest struct {
	IncidentID           string                       `json:"incident_id"`
	PetName              string                       `json:"pet_name"`
	PatientProfile       *domain.AirwayPatientProfile `json:"patient_profile,omitempty"`
	Species              string                       `json:"species"`
	Breed                string                       `json:"breed"`
	WeightKg             float64                      `json:"weight_kg"`
	RectalTempFahrenheit float64                      `json:"rectal_temp_fahrenheit"`
	SpO2Percent          float64                      `json:"spo2_percent"`
	InspiratoryEffort    string                       `json:"inspiratory_effort"`
	LaryngealEdemaGrade  string                       `json:"laryngeal_edema_grade"`
	GagReflexPresent     bool                         `json:"gag_reflex_present"`
}

func (r createAirwayOperationRequest) resolveProfile() domain.AirwayPatientProfile {
	if r.PatientProfile != nil {
		return *r.PatientProfile
	}
	return domain.AirwayPatientProfile{
		Species:              r.Species,
		Breed:                r.Breed,
		WeightKg:             r.WeightKg,
		RectalTempFahrenheit: r.RectalTempFahrenheit,
		SpO2Percent:          r.SpO2Percent,
		InspiratoryEffort:    r.InspiratoryEffort,
		LaryngealEdemaGrade:  r.LaryngealEdemaGrade,
		GagReflexPresent:     r.GagReflexPresent,
	}
}

type stageStabilizationRequest struct {
	PatientProfile       *domain.AirwayPatientProfile `json:"patient_profile,omitempty"`
	Species              string                       `json:"species"`
	Breed                string                       `json:"breed"`
	WeightKg             float64                      `json:"weight_kg"`
	RectalTempFahrenheit float64                      `json:"rectal_temp_fahrenheit"`
	SpO2Percent          float64                      `json:"spo2_percent"`
	InspiratoryEffort    string                       `json:"inspiratory_effort"`
	LaryngealEdemaGrade  string                       `json:"laryngeal_edema_grade"`
	GagReflexPresent     *bool                        `json:"gag_reflex_present,omitempty"`
}

func (s *Server) ensureAirwayService() *airwaytracheostomy.AirwayService {
	if s.airwayService == nil {
		s.airwayService = airwaytracheostomy.NewAirwayService()
	}
	return s.airwayService
}

func (s *Server) registerAirwayTracheostomyRoutes() {
	s.mux.HandleFunc("GET /airway-tracheostomy-triage", s.handleAirwayTracheostomyTriagePage)

	s.registerDualAPIRoute("GET", "airway-tracheostomy/operations", s.handleListAirwayTracheostomyOperations)
	s.registerDualAPIRoute("POST", "airway-tracheostomy/operations", s.handleCreateAirwayTracheostomyOperation)
	s.registerDualAPIRoute("GET", "airway-tracheostomy/operations/{id}", s.handleGetAirwayTracheostomyOperation)
	s.registerDualAPIRoute("POST", "airway-tracheostomy/operations/{id}/stage-stabilization", s.handleStageAirwayStabilization)
	s.registerDualAPIRoute("POST", "airway-tracheostomy/operations/{id}/trach-care-log", s.handleRecordTracheostomyCareLog)
	s.registerDualAPIRoute("POST", "airway-tracheostomy/operations/{id}/extubation-clearance", s.handleIssueAirwayExtubationClearance)
	s.registerDualAPIRoute("GET", "airway-tracheostomy/stream", s.handleAirwayTracheostomyStream)
}

func (s *Server) handleAirwayTracheostomyTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAirwayService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.AirwayTracheostomyOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := AirwayTracheostomyPageData{
		Operations:      ops,
		ActiveOperation: activeOp,
		Metrics:         metrics,
		Locale:          locale,
	}

	s.renderCockpitTemplate(w, r, "airway_tracheostomy_triage.html", "Brachycephalic Obstructive Airway Syndrome (BOAS), Laryngeal Paralysis & Temporary Tracheostomy Triage", data)
}

func (s *Server) handleListAirwayTracheostomyOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAirwayService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.AirwayTracheostomyOperation, 0, len(ops))
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

func (s *Server) handleGetAirwayTracheostomyOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAirwayService()
	op, err := svc.GetOperation(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateAirwayTracheostomyOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createAirwayOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureAirwayService()
	created, err := svc.CreateOperation(req.IncidentID, req.PetName, req.resolveProfile())
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAirwayTracheostomyEvent("airway_tracheostomy_updated", map[string]any{
		"type":      "airway_tracheostomy_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageAirwayStabilization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAirwayService()
	existing, err := svc.GetOperation(opID)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req stageStabilizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	profile := existing.PatientProfile
	if req.PatientProfile != nil {
		profile = *req.PatientProfile
	} else {
		if strings.TrimSpace(req.Species) != "" {
			profile.Species = req.Species
		}
		if strings.TrimSpace(req.Breed) != "" {
			profile.Breed = req.Breed
		}
		if req.WeightKg != 0 {
			profile.WeightKg = req.WeightKg
		}
		if req.RectalTempFahrenheit != 0 {
			profile.RectalTempFahrenheit = req.RectalTempFahrenheit
		}
		if req.SpO2Percent != 0 {
			profile.SpO2Percent = req.SpO2Percent
		}
		if strings.TrimSpace(req.InspiratoryEffort) != "" {
			profile.InspiratoryEffort = req.InspiratoryEffort
		}
		if strings.TrimSpace(req.LaryngealEdemaGrade) != "" {
			profile.LaryngealEdemaGrade = req.LaryngealEdemaGrade
		}
		if req.GagReflexPresent != nil {
			profile.GagReflexPresent = *req.GagReflexPresent
		}
	}

	updated, err := svc.StageStabilizationPlan(opID, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAirwayTracheostomyEvent("airway_tracheostomy_updated", map[string]any{
		"type":      "airway_tracheostomy_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordTracheostomyCareLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAirwayService()
	if _, err := svc.GetOperation(opID); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var logEntry domain.TracheostomyCareLog
	if err := json.NewDecoder(r.Body).Decode(&logEntry); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordTrachCareLog(opID, logEntry)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAirwayTracheostomyEvent("airway_tracheostomy_updated", map[string]any{
		"type":      "airway_tracheostomy_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueAirwayExtubationClearance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opID := strings.TrimSpace(r.PathValue("id"))
	if opID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "operation id is required")
		return
	}

	svc := s.ensureAirwayService()
	if _, err := svc.GetOperation(opID); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.AirwayExtubationClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueExtubationClearance(opID, clearance)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAirwayTracheostomyEvent("airway_tracheostomy_updated", map[string]any{
		"type":      "airway_tracheostomy_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAirwayTracheostomyStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "airway_tracheostomy")
}

func (s *Server) broadcastAirwayTracheostomyEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("airway_tracheostomy", "airway-tracheostomy-hub", eventType, payload, now)
}
