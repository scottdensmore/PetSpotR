package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/statusepilepticus"
)

// StatusEpilepticusPageData represents template data passed to templates/status_epilepticus_triage.html.
type StatusEpilepticusPageData struct {
	Operations                   []domain.StatusEpilepticusOperation
	ActiveOperation              *domain.StatusEpilepticusOperation
	TotalOperations              int
	ActiveIctalCriticalCount     int
	AnticonvulsantCRIStagedCount int
	SeizureFreeClearedCount      int
	HyperthermiaAlertCount       int
	Locale                       string
}

type createEpilepticusOperationPayload struct {
	IncidentID               string                            `json:"incident_id"`
	PetName                  string                            `json:"pet_name"`
	PatientProfile           *domain.EpilepticusPatientProfile `json:"patient_profile"`
	Species                  string                            `json:"species"`
	Breed                    string                            `json:"breed"`
	WeightKg                 float64                           `json:"weight_kg"`
	NeuroEtiology            string                            `json:"neuro_etiology"`
	ContinuousIctalMinutes   int                               `json:"continuous_ictal_minutes"`
	ClusterCount24Hr         int                               `json:"cluster_count_24hr"`
	RectalTempFahrenheit     float64                           `json:"rectal_temp_fahrenheit"`
	BloodGlucoseMgDl         float64                           `json:"blood_glucose_mg_dl"`
	ModifiedGlasgowComaScore int                               `json:"modified_glasgow_coma_score"`
}

type stageEpilepticusRegimenPayload struct {
	PatientProfile           *domain.EpilepticusPatientProfile `json:"patient_profile"`
	Species                  string                            `json:"species"`
	Breed                    string                            `json:"breed"`
	WeightKg                 float64                           `json:"weight_kg"`
	NeuroEtiology            string                            `json:"neuro_etiology"`
	ContinuousIctalMinutes   *int                              `json:"continuous_ictal_minutes"`
	ClusterCount24Hr         *int                              `json:"cluster_count_24hr"`
	RectalTempFahrenheit     float64                           `json:"rectal_temp_fahrenheit"`
	BloodGlucoseMgDl         float64                           `json:"blood_glucose_mg_dl"`
	ModifiedGlasgowComaScore int                               `json:"modified_glasgow_coma_score"`
}

func (s *Server) ensureEpilepticusService() *statusepilepticus.EpilepticusService {
	if s.epilepticusService == nil {
		s.epilepticusService = statusepilepticus.NewEpilepticusService()
	}
	return s.epilepticusService
}

func (s *Server) registerStatusEpilepticusRoutes() {
	s.mux.HandleFunc("GET /status-epilepticus-triage", s.handleStatusEpilepticusTriagePage)

	s.registerDualAPIRoute("GET", "status-epilepticus/operations", s.handleListStatusEpilepticusOperations)
	s.registerDualAPIRoute("POST", "status-epilepticus/operations", s.handleCreateStatusEpilepticusOperation)
	s.registerDualAPIRoute("GET", "status-epilepticus/operations/{id}", s.handleGetStatusEpilepticusOperation)
	s.registerDualAPIRoute("POST", "status-epilepticus/operations/{id}/stage-regimen", s.handleStageStatusEpilepticusRegimen)
	s.registerDualAPIRoute("POST", "status-epilepticus/operations/{id}/neuro-check", s.handleRecordStatusEpilepticusNeuroCheck)
	s.registerDualAPIRoute("POST", "status-epilepticus/operations/{id}/discharge-clearance", s.handleIssueStatusEpilepticusDischargeClearance)
	s.registerDualAPIRoute("GET", "status-epilepticus/stream", s.handleStatusEpilepticusStream)
}

func (s *Server) handleStatusEpilepticusTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureEpilepticusService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.StatusEpilepticusOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := StatusEpilepticusPageData{
		Operations:                   ops,
		ActiveOperation:              activeOp,
		TotalOperations:              metrics.TotalOperations,
		ActiveIctalCriticalCount:     metrics.ActiveIctalCriticalCount,
		AnticonvulsantCRIStagedCount: metrics.AnticonvulsantCRIStagedCount,
		SeizureFreeClearedCount:      metrics.SeizureFreeClearedCount,
		HyperthermiaAlertCount:       metrics.HyperthermiaAlertCount,
		Locale:                       locale,
	}

	s.renderCockpitTemplate(w, r, "status_epilepticus_triage.html", "Status Epilepticus Triage", data)
}

func (s *Server) handleListStatusEpilepticusOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureEpilepticusService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	etiologyFilter := strings.TrimSpace(r.URL.Query().Get("neuro_etiology"))
	if etiologyFilter == "" {
		etiologyFilter = strings.TrimSpace(r.URL.Query().Get("etiology"))
	}

	ops := svc.ListOperations(statusFilter)
	if etiologyFilter != "" {
		filtered := make([]domain.StatusEpilepticusOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(op.PatientProfile.NeuroEtiology, etiologyFilter) {
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

func (s *Server) handleGetStatusEpilepticusOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureEpilepticusService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateStatusEpilepticusOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createEpilepticusOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.EpilepticusPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.EpilepticusPatientProfile{
			Species:                  payload.Species,
			Breed:                    payload.Breed,
			WeightKg:                 payload.WeightKg,
			NeuroEtiology:            payload.NeuroEtiology,
			ContinuousIctalMinutes:   payload.ContinuousIctalMinutes,
			ClusterCount24Hr:         payload.ClusterCount24Hr,
			RectalTempFahrenheit:     payload.RectalTempFahrenheit,
			BloodGlucoseMgDl:         payload.BloodGlucoseMgDl,
			ModifiedGlasgowComaScore: payload.ModifiedGlasgowComaScore,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-SE-%d", 9200+len(s.ensureEpilepticusService().ListOperations(""))+1)
	}

	svc := s.ensureEpilepticusService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastStatusEpilepticusEvent("status_epilepticus_updated", map[string]any{
		"type":      "status_epilepticus_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageStatusEpilepticusRegimen(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureEpilepticusService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageEpilepticusRegimenPayload
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
		if strings.TrimSpace(payload.NeuroEtiology) != "" {
			profile.NeuroEtiology = payload.NeuroEtiology
		}
		if payload.ContinuousIctalMinutes != nil {
			profile.ContinuousIctalMinutes = *payload.ContinuousIctalMinutes
		}
		if payload.ClusterCount24Hr != nil {
			profile.ClusterCount24Hr = *payload.ClusterCount24Hr
		}
		if payload.RectalTempFahrenheit != 0 {
			profile.RectalTempFahrenheit = payload.RectalTempFahrenheit
		}
		if payload.BloodGlucoseMgDl != 0 {
			profile.BloodGlucoseMgDl = payload.BloodGlucoseMgDl
		}
		if payload.ModifiedGlasgowComaScore != 0 {
			profile.ModifiedGlasgowComaScore = payload.ModifiedGlasgowComaScore
		}
	}

	updated, err := svc.StageTitrationRegimen(id, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastStatusEpilepticusEvent("status_epilepticus_updated", map[string]any{
		"type":      "status_epilepticus_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordStatusEpilepticusNeuroCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureEpilepticusService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.NeuroIcuSerialCheck
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
	s.broadcastStatusEpilepticusEvent("status_epilepticus_updated", map[string]any{
		"type":      "status_epilepticus_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueStatusEpilepticusDischargeClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureEpilepticusService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.EpilepticusDischargeClearance
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
	s.broadcastStatusEpilepticusEvent("status_epilepticus_updated", map[string]any{
		"type":      "status_epilepticus_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleStatusEpilepticusStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "status_epilepticus")
}

func (s *Server) broadcastStatusEpilepticusEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("status_epilepticus", "status-epilepticus-hub", eventType, payload, now)
}
