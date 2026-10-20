package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/cardiogenicedema"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// CardiogenicEdemaPageData represents template data passed to templates/cardiogenic_edema_triage.html.
type CardiogenicEdemaPageData struct {
	Operations        []domain.CardiogenicEdemaOperation
	ActiveOperation   *domain.CardiogenicEdemaOperation
	Metrics           cardiogenicedema.CardiogenicMetrics
	TotalOperations   int
	FulminantCritical int
	FurosemideStaged  int
	EupneicCleared    int
	FluidGuardLocks   int
	Locale            string
}

type createCardiogenicOperationRequest struct {
	IncidentID             string                            `json:"incident_id"`
	PetName                string                            `json:"pet_name"`
	PatientProfile         *domain.CardiogenicPatientProfile `json:"patient_profile,omitempty"`
	Species                string                            `json:"species"`
	WeightKg               float64                           `json:"weight_kg"`
	RespiratoryRateBpm     int                               `json:"respiratory_rate_bpm"`
	HeartRateBpm           int                               `json:"heart_rate_bpm"`
	MurmurGrade            string                            `json:"murmur_grade"`
	TfastBLinesPerSite     int                               `json:"tfast_b_lines_per_site"`
	LaAoRatio              float64                           `json:"la_ao_ratio"`
	PleuralEffusionPresent bool                              `json:"pleural_effusion_present"`
}

func (s *Server) ensureCardiogenicService() *cardiogenicedema.CardiogenicService {
	if s.cardiogenicService == nil {
		s.cardiogenicService = cardiogenicedema.NewCardiogenicService()
	}
	return s.cardiogenicService
}

func (s *Server) registerCardiogenicEdemaRoutes() {
	s.mux.HandleFunc("GET /cardiogenic-edema-triage", s.handleCardiogenicEdemaTriagePage)
	s.mux.HandleFunc("GET /api/cardiogenic-edema/operations", s.handleListCardiogenicEdemaOperations)
	s.mux.HandleFunc("POST /api/cardiogenic-edema/operations", s.handleCreateCardiogenicEdemaOperation)
	s.mux.HandleFunc("GET /api/cardiogenic-edema/operations/{id}", s.handleGetCardiogenicEdemaOperation)
	s.mux.HandleFunc("POST /api/cardiogenic-edema/operations/{id}/stage-regimen", s.handleStageCardiogenicRegimen)
	s.mux.HandleFunc("POST /api/cardiogenic-edema/operations/{id}/diuresis-check", s.handleRecordCardiogenicDiuresisCheck)
	s.mux.HandleFunc("POST /api/cardiogenic-edema/operations/{id}/discharge-clearance", s.handleIssueCardiogenicDischargeClearance)
	s.mux.HandleFunc("GET /api/cardiogenic-edema/stream", s.handleCardiogenicEdemaStream)
}

func (s *Server) handleCardiogenicEdemaTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureCardiogenicService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.CardiogenicEdemaOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := CardiogenicEdemaPageData{
		Operations:        ops,
		ActiveOperation:   activeOp,
		Metrics:           metrics,
		TotalOperations:   metrics.TotalOperations,
		FulminantCritical: metrics.FulminantCritical,
		FurosemideStaged:  metrics.FurosemideO2Staged,
		EupneicCleared:    metrics.EupneicCleared,
		FluidGuardLocks:   metrics.ActiveFluidGuardLock,
		Locale:            locale,
	}

	s.renderCockpitTemplate(w, r, "cardiogenic_edema_triage.html", "Acute Cardiogenic Pulmonary Edema, Mitral Valve Chordae Rupture & Oxygen Cage Triage", data)
}

func (s *Server) handleListCardiogenicEdemaOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureCardiogenicService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	ops := svc.ListOperations(statusFilter)
	metrics := svc.ComputeMetrics()

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
	})
}

func (s *Server) handleGetCardiogenicEdemaOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureCardiogenicService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateCardiogenicEdemaOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createCardiogenicOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.CardiogenicPatientProfile
	if req.PatientProfile != nil {
		profile = *req.PatientProfile
	} else {
		profile = domain.CardiogenicPatientProfile{
			Species:                req.Species,
			WeightKg:               req.WeightKg,
			RespiratoryRateBpm:     req.RespiratoryRateBpm,
			HeartRateBpm:           req.HeartRateBpm,
			MurmurGrade:            req.MurmurGrade,
			TfastBLinesPerSite:     req.TfastBLinesPerSite,
			LaAoRatio:              req.LaAoRatio,
			PleuralEffusionPresent: req.PleuralEffusionPresent,
		}
	}

	svc := s.ensureCardiogenicService()
	created, err := svc.CreateOperation(req.IncidentID, req.PetName, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastCardiogenicEdemaEvent("cardiogenic_edema_updated", map[string]any{
		"type":      "cardiogenic_edema_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageCardiogenicRegimen(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureCardiogenicService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var body struct {
		PatientProfile *domain.CardiogenicPatientProfile `json:"patient_profile,omitempty"`
		domain.CardiogenicPatientProfile
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	profile := existing.PatientProfile
	if body.PatientProfile != nil {
		profile = *body.PatientProfile
	} else if strings.TrimSpace(body.Species) != "" || body.WeightKg != 0 || body.RespiratoryRateBpm != 0 {
		profile = body.CardiogenicPatientProfile
	}

	updated, err := svc.StageDiureticRegimen(id, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastCardiogenicEdemaEvent("cardiogenic_edema_updated", map[string]any{
		"type":      "cardiogenic_edema_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordCardiogenicDiuresisCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureCardiogenicService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.DiuresisTelemetryCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordDiuresisCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastCardiogenicEdemaEvent("cardiogenic_edema_updated", map[string]any{
		"type":      "cardiogenic_edema_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueCardiogenicDischargeClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureCardiogenicService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.CardiogenicDischargeClearance
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
	s.broadcastCardiogenicEdemaEvent("cardiogenic_edema_updated", map[string]any{
		"type":      "cardiogenic_edema_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleCardiogenicEdemaStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "cardiogenic_edema")
}

func (s *Server) broadcastCardiogenicEdemaEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("cardiogenic_edema", "cardiogenic-edema-hub", eventType, payload, now)
}
