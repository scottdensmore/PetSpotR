package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/hemoabdomen"
)

// HemoabdomenPageData represents template data passed to templates/hemoabdomen_autotransfusion_triage.html.
type HemoabdomenPageData struct {
	Operations                     []domain.HemoabdomenOperation
	ActiveOperation                *domain.HemoabdomenOperation
	TotalOperations                int
	HemorrhagicShockCriticalCount  int
	AutotransfusionTXAStagedCount  int
	HemostasisClearedCount         int
	ClotPoppingNeoplasiaAlertCount int
	Locale                         string
}

type createHemoabdomenOperationPayload struct {
	IncidentID                  string                            `json:"incident_id"`
	PetName                     string                            `json:"pet_name"`
	PatientProfile              *domain.HemoabdomenPatientProfile `json:"patient_profile"`
	Species                     string                            `json:"species"`
	Breed                       string                            `json:"breed"`
	WeightKg                    float64                           `json:"weight_kg"`
	AfastAFS4Score              int                               `json:"afast_afs4_score"`
	EffusionPCVPercent          float64                           `json:"effusion_pcv_percent"`
	PeripheralPCVPercent        float64                           `json:"peripheral_pcv_percent"`
	MeanArterialPressureMmHg    int                               `json:"mean_arterial_pressure_mmhg"`
	PlasmaLactateMmolL          float64                           `json:"plasma_lactate_mmol_l"`
	EffusionNonClottingVerified bool                              `json:"effusion_non_clotting_verified"`
	SuspectedNeoplasia          bool                              `json:"suspected_neoplasia"`
}

type stageHemoabdomenPlanPayload struct {
	PatientProfile              *domain.HemoabdomenPatientProfile `json:"patient_profile"`
	Species                     string                            `json:"species"`
	Breed                       string                            `json:"breed"`
	WeightKg                    float64                           `json:"weight_kg"`
	AfastAFS4Score              *int                              `json:"afast_afs4_score"`
	EffusionPCVPercent          *float64                          `json:"effusion_pcv_percent"`
	PeripheralPCVPercent        float64                           `json:"peripheral_pcv_percent"`
	MeanArterialPressureMmHg    int                               `json:"mean_arterial_pressure_mmhg"`
	PlasmaLactateMmolL          float64                           `json:"plasma_lactate_mmol_l"`
	EffusionNonClottingVerified *bool                             `json:"effusion_non_clotting_verified"`
	SuspectedNeoplasia          *bool                             `json:"suspected_neoplasia"`
}

func (s *Server) ensureHemoabdomenService() *hemoabdomen.HemoabdomenService {
	if s.hemoabdomenService == nil {
		s.hemoabdomenService = hemoabdomen.NewHemoabdomenService()
	}
	return s.hemoabdomenService
}

func (s *Server) registerHemoabdomenRoutes() {
	s.mux.HandleFunc("GET /hemoabdomen-autotransfusion-triage", s.handleHemoabdomenTriagePage)

	s.registerDualAPIRoute("GET", "hemoabdomen-autotransfusion/operations", s.handleListHemoabdomenOperations)
	s.registerDualAPIRoute("POST", "hemoabdomen-autotransfusion/operations", s.handleCreateHemoabdomenOperation)
	s.registerDualAPIRoute("GET", "hemoabdomen-autotransfusion/operations/{id}", s.handleGetHemoabdomenOperation)
	s.registerDualAPIRoute("POST", "hemoabdomen-autotransfusion/operations/{id}/stage-plan", s.handleStageHemoabdomenPlan)
	s.registerDualAPIRoute("POST", "hemoabdomen-autotransfusion/operations/{id}/hemodynamic-check", s.handleRecordHemoabdomenHemodynamicCheck)
	s.registerDualAPIRoute("POST", "hemoabdomen-autotransfusion/operations/{id}/hemostasis-clearance", s.handleIssueHemoabdomenHemostasisClearance)
	s.registerDualAPIRoute("GET", "hemoabdomen-autotransfusion/stream", s.handleHemoabdomenStream)
}

func (s *Server) handleHemoabdomenTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHemoabdomenService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.HemoabdomenOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := HemoabdomenPageData{
		Operations:                     ops,
		ActiveOperation:                activeOp,
		TotalOperations:                metrics.TotalOperations,
		HemorrhagicShockCriticalCount:  metrics.HemorrhagicShockCriticalCount,
		AutotransfusionTXAStagedCount:  metrics.AutotransfusionTXAStagedCount,
		HemostasisClearedCount:         metrics.HemostasisClearedCount,
		ClotPoppingNeoplasiaAlertCount: metrics.ClotPoppingNeoplasiaAlertCount,
		Locale:                         locale,
	}

	s.renderCockpitTemplate(w, r, "hemoabdomen_autotransfusion_triage.html", "Hemoabdomen Autotransfusion Triage", data)
}

func (s *Server) handleListHemoabdomenOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureHemoabdomenService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.HemoabdomenOperation, 0, len(ops))
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

func (s *Server) handleGetHemoabdomenOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureHemoabdomenService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateHemoabdomenOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createHemoabdomenOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.HemoabdomenPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.HemoabdomenPatientProfile{
			Species:                     payload.Species,
			Breed:                       payload.Breed,
			WeightKg:                    payload.WeightKg,
			AfastAFS4Score:              payload.AfastAFS4Score,
			EffusionPCVPercent:          payload.EffusionPCVPercent,
			PeripheralPCVPercent:        payload.PeripheralPCVPercent,
			MeanArterialPressureMmHg:    payload.MeanArterialPressureMmHg,
			PlasmaLactateMmolL:          payload.PlasmaLactateMmolL,
			EffusionNonClottingVerified: payload.EffusionNonClottingVerified,
			SuspectedNeoplasia:          payload.SuspectedNeoplasia,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-HA-%d", 9400+len(s.ensureHemoabdomenService().ListOperations(""))+1)
	}

	svc := s.ensureHemoabdomenService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHemoabdomenEvent("hemoabdomen_updated", map[string]any{
		"type":      "hemoabdomen_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageHemoabdomenPlan(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureHemoabdomenService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageHemoabdomenPlanPayload
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
		if payload.AfastAFS4Score != nil {
			profile.AfastAFS4Score = *payload.AfastAFS4Score
		}
		if payload.EffusionPCVPercent != nil {
			profile.EffusionPCVPercent = *payload.EffusionPCVPercent
		}
		if payload.PeripheralPCVPercent != 0 {
			profile.PeripheralPCVPercent = payload.PeripheralPCVPercent
		}
		if payload.MeanArterialPressureMmHg != 0 {
			profile.MeanArterialPressureMmHg = payload.MeanArterialPressureMmHg
		}
		if payload.PlasmaLactateMmolL != 0 {
			profile.PlasmaLactateMmolL = payload.PlasmaLactateMmolL
		}
		if payload.EffusionNonClottingVerified != nil {
			profile.EffusionNonClottingVerified = *payload.EffusionNonClottingVerified
		}
		if payload.SuspectedNeoplasia != nil {
			profile.SuspectedNeoplasia = *payload.SuspectedNeoplasia
		}
	}

	updated, err := svc.StageResuscitationPlan(id, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHemoabdomenEvent("hemoabdomen_updated", map[string]any{
		"type":      "hemoabdomen_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordHemoabdomenHemodynamicCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureHemoabdomenService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.HemoabdomenHemodynamicCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordHemodynamicCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHemoabdomenEvent("hemoabdomen_updated", map[string]any{
		"type":      "hemoabdomen_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueHemoabdomenHemostasisClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureHemoabdomenService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.HemoabdomenHemostasisClearance
	if err := json.NewDecoder(r.Body).Decode(&clearance); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.IssueHemostasisClearance(id, clearance)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastHemoabdomenEvent("hemoabdomen_updated", map[string]any{
		"type":      "hemoabdomen_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleHemoabdomenStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "hemoabdomen")
}

func (s *Server) broadcastHemoabdomenEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("hemoabdomen", "hemoabdomen-hub", eventType, payload, now)
}
