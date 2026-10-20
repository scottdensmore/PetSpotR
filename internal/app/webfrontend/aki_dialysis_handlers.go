package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/akidialysis"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// AKIDialysisPageData represents template data passed to templates/aki_dialysis_triage.html.
type AKIDialysisPageData struct {
	Operations                    []domain.AKIDialysisOperation
	ActiveOperation               *domain.AKIDialysisOperation
	TotalOperations               int
	OliguricAzotemiaCriticalCount int
	FomepizoleDiuresisStagedCount int
	PolyuricClearedCount          int
	AnuricOverloadAlertCount      int
	Locale                        string
}

type createAKIOperationPayload struct {
	IncidentID                        string                    `json:"incident_id"`
	PetName                           string                    `json:"pet_name"`
	PatientProfile                    *domain.AKIPatientProfile `json:"patient_profile"`
	Species                           string                    `json:"species"`
	Breed                             string                    `json:"breed"`
	WeightKg                          float64                   `json:"weight_kg"`
	IRISGrade                         int                       `json:"iris_grade"`
	SerumCreatinineMgDl               float64                   `json:"serum_creatinine_mg_dl"`
	SerumPotassiumMeqL                float64                   `json:"serum_potassium_meq_l"`
	UrineOutputMlKgHr                 float64                   `json:"urine_output_ml_kg_hr"`
	BodyWeightGainPercent             float64                   `json:"body_weight_gain_percent"`
	CalciumOxalateMonohydrateCrystals bool                      `json:"calcium_oxalate_monohydrate_crystals"`
}

type stageAKIPlanPayload struct {
	PatientProfile                    *domain.AKIPatientProfile `json:"patient_profile"`
	Species                           string                    `json:"species"`
	Breed                             string                    `json:"breed"`
	WeightKg                          float64                   `json:"weight_kg"`
	IRISGrade                         int                       `json:"iris_grade"`
	SerumCreatinineMgDl               float64                   `json:"serum_creatinine_mg_dl"`
	SerumPotassiumMeqL                float64                   `json:"serum_potassium_meq_l"`
	UrineOutputMlKgHr                 *float64                  `json:"urine_output_ml_kg_hr"`
	BodyWeightGainPercent             *float64                  `json:"body_weight_gain_percent"`
	CalciumOxalateMonohydrateCrystals *bool                     `json:"calcium_oxalate_monohydrate_crystals"`
}

func (s *Server) ensureAKIService() *akidialysis.AKIService {
	if s.akiService == nil {
		s.akiService = akidialysis.NewAKIService()
	}
	return s.akiService
}

func (s *Server) registerAKIDialysisRoutes() {
	s.mux.HandleFunc("GET /aki-dialysis-triage", s.handleAKIDialysisTriagePage)

	s.registerDualAPIRoute("GET", "aki-dialysis/operations", s.handleListAKIDialysisOperations)
	s.registerDualAPIRoute("POST", "aki-dialysis/operations", s.handleCreateAKIDialysisOperation)
	s.registerDualAPIRoute("GET", "aki-dialysis/operations/{id}", s.handleGetAKIDialysisOperation)
	s.registerDualAPIRoute("POST", "aki-dialysis/operations/{id}/stage-plan", s.handleStageAKIDialysisPlan)
	s.registerDualAPIRoute("POST", "aki-dialysis/operations/{id}/renal-check", s.handleRecordAKIDialysisRenalCheck)
	s.registerDualAPIRoute("POST", "aki-dialysis/operations/{id}/discharge-clearance", s.handleIssueAKIDialysisDischargeClearance)
	s.registerDualAPIRoute("GET", "aki-dialysis/stream", s.handleAKIDialysisStream)
}

func (s *Server) handleAKIDialysisTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAKIService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.AKIDialysisOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := AKIDialysisPageData{
		Operations:                    ops,
		ActiveOperation:               activeOp,
		TotalOperations:               metrics.TotalOperations,
		OliguricAzotemiaCriticalCount: metrics.OliguricAzotemiaCriticalCount,
		FomepizoleDiuresisStagedCount: metrics.FomepizoleDiuresisStagedCount,
		PolyuricClearedCount:          metrics.PolyuricClearedCount,
		AnuricOverloadAlertCount:      metrics.AnuricOverloadAlertCount,
		Locale:                        locale,
	}

	s.renderCockpitTemplate(w, r, "aki_dialysis_triage.html", "AKI Dialysis Triage", data)
}

func (s *Server) handleListAKIDialysisOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAKIService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.AKIDialysisOperation, 0, len(ops))
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

func (s *Server) handleGetAKIDialysisOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAKIService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateAKIDialysisOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createAKIOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.AKIPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.AKIPatientProfile{
			Species:                           payload.Species,
			Breed:                             payload.Breed,
			WeightKg:                          payload.WeightKg,
			IRISGrade:                         payload.IRISGrade,
			SerumCreatinineMgDl:               payload.SerumCreatinineMgDl,
			SerumPotassiumMeqL:                payload.SerumPotassiumMeqL,
			UrineOutputMlKgHr:                 payload.UrineOutputMlKgHr,
			BodyWeightGainPercent:             payload.BodyWeightGainPercent,
			CalciumOxalateMonohydrateCrystals: payload.CalciumOxalateMonohydrateCrystals,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-AKI-%d", 9930+len(s.ensureAKIService().ListOperations(""))+1)
	}

	svc := s.ensureAKIService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAKIDialysisEvent("aki_dialysis_updated", map[string]any{
		"type":      "aki_dialysis_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageAKIDialysisPlan(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAKIService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageAKIPlanPayload
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
		if payload.IRISGrade != 0 {
			profile.IRISGrade = payload.IRISGrade
		}
		if payload.SerumCreatinineMgDl != 0 {
			profile.SerumCreatinineMgDl = payload.SerumCreatinineMgDl
		}
		if payload.SerumPotassiumMeqL != 0 {
			profile.SerumPotassiumMeqL = payload.SerumPotassiumMeqL
		}
		if payload.UrineOutputMlKgHr != nil {
			profile.UrineOutputMlKgHr = *payload.UrineOutputMlKgHr
		}
		if payload.BodyWeightGainPercent != nil {
			profile.BodyWeightGainPercent = *payload.BodyWeightGainPercent
		}
		if payload.CalciumOxalateMonohydrateCrystals != nil {
			profile.CalciumOxalateMonohydrateCrystals = *payload.CalciumOxalateMonohydrateCrystals
		}
	}

	updated, err := svc.StageDiuresisPlan(id, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAKIDialysisEvent("aki_dialysis_updated", map[string]any{
		"type":      "aki_dialysis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordAKIDialysisRenalCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAKIService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.AKIRenalTelemetryCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordRenalCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAKIDialysisEvent("aki_dialysis_updated", map[string]any{
		"type":      "aki_dialysis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueAKIDialysisDischargeClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAKIService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.AKIDischargeClearance
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
	s.broadcastAKIDialysisEvent("aki_dialysis_updated", map[string]any{
		"type":      "aki_dialysis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAKIDialysisStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "aki_dialysis")
}

func (s *Server) broadcastAKIDialysisEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("aki_dialysis", "aki-dialysis-hub", eventType, payload, now)
}
