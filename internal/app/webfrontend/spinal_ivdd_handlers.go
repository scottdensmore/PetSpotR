package webfrontend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/spinalivdd"
)

// SpinalIVDDPageData represents template data passed to templates/spinal_ivdd_triage.html.
type SpinalIVDDPageData struct {
	Operations                   []domain.SpinalIVDDOperation
	ActiveOperation              *domain.SpinalIVDDOperation
	Metrics                      domain.SpinalIVDDOverviewMetrics
	TotalOperations              int
	AcuteParaplegiaTriageCount   int
	HemilaminectomyStagedCount   int
	ClearedPostopCount           int
	DeepPainAbsentEmergencyCount int
	SchiffSherringtonCount       int
	Locale                       string
}

type createSpinalIVDDOperationPayload struct {
	CaseID                   string                       `json:"case_id"`
	PetName                  string                       `json:"pet_name"`
	Species                  string                       `json:"species"`
	Breed                    string                       `json:"breed"`
	WeightKg                 float64                      `json:"weight_kg"`
	FrankelGrade             domain.ModifiedFrankelGrade  `json:"frankel_grade"`
	LocalizationZone         domain.NeurolocalizationZone `json:"localization_zone"`
	OnsetHours               float64                      `json:"onset_hours"`
	CutaneousTrunciVertebra  string                       `json:"cutaneous_trunci_vertebra"`
	DeepPainPresent          bool                         `json:"deep_pain_present"`
	SchiffSherringtonPresent bool                         `json:"schiff_sherrington_present"`
	PriorSteroidOrNSAID      string                       `json:"prior_steroid_or_nsaid"`
	SurgicalWindowHours      float64                      `json:"surgical_window_hours"`
	TargetMAPMmHg            int                          `json:"target_map_mmhg"`
}

type stageSpinalIVDDDecompressionPayload struct {
	SurgicalWindowTarget      string `json:"surgical_window_target"`
	ImagingModality           string `json:"imaging_modality"`
	HemilaminectomySite       string `json:"hemilaminectomy_site"`
	BladderManagementProtocol string `json:"bladder_management_protocol"`
	BladderProtocol           string `json:"bladder_protocol"`
	AnalgesiaCRIProtocol      string `json:"analgesia_cri_protocol"`
	AnalgesiaProtocol         string `json:"analgesia_protocol"`
	StrictCrateRestProtocol   string `json:"strict_crate_rest_protocol"`
	CrateRestProtocol         string `json:"crate_rest_protocol"`
}

type recordSpinalIVDDCheckPayload struct {
	FrankelGrade         domain.ModifiedFrankelGrade `json:"frankel_grade"`
	DeepPainStatus       string                      `json:"deep_pain_status"`
	CutaneousTrunciLevel string                      `json:"cutaneous_trunci_level"`
	BladderToneStatus    string                      `json:"bladder_tone_status"`
	RecordedBy           string                      `json:"recorded_by"`
	ClinicalNote         string                      `json:"clinical_note"`
}

type clearSpinalIVDDDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureSpinalService() *spinalivdd.IVDDService {
	if s.spinalService == nil {
		s.spinalService = spinalivdd.NewIVDDService()
	}
	return s.spinalService
}

func (s *Server) registerSpinalIVDDRoutes() {
	s.mux.HandleFunc("GET /spinal-ivdd-triage", s.handleSpinalIVDDTriagePage)

	s.registerDualAPIRoute("GET", "spinal-ivdd/operations", s.handleListSpinalIVDDOperations)
	s.registerDualAPIRoute("POST", "spinal-ivdd/operations", s.handleCreateSpinalIVDDOperation)
	s.registerDualAPIRoute("GET", "spinal-ivdd/operations/{id}", s.handleGetSpinalIVDDOperation)
	s.registerDualAPIRoute("POST", "spinal-ivdd/operations/{id}/stage-decompression", s.handleStageSpinalIVDDDecompression)
	s.registerDualAPIRoute("POST", "spinal-ivdd/operations/{id}/record-check", s.handleRecordSpinalIVDDCheck)
	s.registerDualAPIRoute("POST", "spinal-ivdd/operations/{id}/clear-discharge", s.handleClearSpinalIVDDDischarge)
	s.registerDualAPIRoute("GET", "spinal-ivdd/stream", s.handleSpinalIVDDStream)
}

func (s *Server) handleSpinalIVDDTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureSpinalService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.SpinalIVDDOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := SpinalIVDDPageData{
		Operations:                   ops,
		ActiveOperation:              activeOp,
		Metrics:                      metrics,
		TotalOperations:              metrics.TotalOperations,
		AcuteParaplegiaTriageCount:   metrics.AcuteParaplegiaTriageCount,
		HemilaminectomyStagedCount:   metrics.HemilaminectomyStagedCount,
		ClearedPostopCount:           metrics.ClearedPostopCount,
		DeepPainAbsentEmergencyCount: metrics.DeepPainAbsentEmergencyCount,
		SchiffSherringtonCount:       metrics.SchiffSherringtonCount,
		Locale:                       locale,
	}

	s.renderCockpitTemplate(w, r, "spinal_ivdd_triage.html", "Spinal IVDD Triage", data)
}

func (s *Server) handleListSpinalIVDDOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureSpinalService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	zoneFilter := strings.TrimSpace(r.URL.Query().Get("localization_zone"))

	ops := svc.ListOperations(statusFilter)
	if zoneFilter != "" {
		filtered := make([]domain.SpinalIVDDOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.LocalizationZone), zoneFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	metrics := svc.GetOverviewMetrics()

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
	})
}

func (s *Server) handleGetSpinalIVDDOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureSpinalService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateSpinalIVDDOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createSpinalIVDDOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureSpinalService()
	created, err := svc.CreateOperation(spinalivdd.CreateOperationInput{
		CaseID:                   payload.CaseID,
		PetName:                  payload.PetName,
		Species:                  payload.Species,
		Breed:                    payload.Breed,
		WeightKg:                 payload.WeightKg,
		FrankelGrade:             payload.FrankelGrade,
		LocalizationZone:         payload.LocalizationZone,
		OnsetHours:               payload.OnsetHours,
		CutaneousTrunciVertebra:  payload.CutaneousTrunciVertebra,
		DeepPainPresent:          payload.DeepPainPresent,
		SchiffSherringtonPresent: payload.SchiffSherringtonPresent,
		PriorSteroidOrNSAID:      payload.PriorSteroidOrNSAID,
		SurgicalWindowHours:      payload.SurgicalWindowHours,
		TargetMAPMmHg:            payload.TargetMAPMmHg,
	})
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastSpinalIVDDEvent("spinal_ivdd_updated", map[string]any{
		"type":      "spinal_ivdd_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageSpinalIVDDDecompression(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureSpinalService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageSpinalIVDDDecompressionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	bladder := payload.BladderManagementProtocol
	if strings.TrimSpace(bladder) == "" {
		bladder = payload.BladderProtocol
	}
	analgesia := payload.AnalgesiaCRIProtocol
	if strings.TrimSpace(analgesia) == "" {
		analgesia = payload.AnalgesiaProtocol
	}
	crateRest := payload.StrictCrateRestProtocol
	if strings.TrimSpace(crateRest) == "" {
		crateRest = payload.CrateRestProtocol
	}

	updated, err := svc.StageDecompression(
		id,
		payload.SurgicalWindowTarget,
		payload.ImagingModality,
		payload.HemilaminectomySite,
		bladder,
		analgesia,
		crateRest,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastSpinalIVDDEvent("spinal_ivdd_updated", map[string]any{
		"type":      "spinal_ivdd_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordSpinalIVDDCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureSpinalService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordSpinalIVDDCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordNeuroCheck(
		id,
		payload.FrankelGrade,
		payload.DeepPainStatus,
		payload.CutaneousTrunciLevel,
		payload.BladderToneStatus,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastSpinalIVDDEvent("spinal_ivdd_updated", map[string]any{
		"type":      "spinal_ivdd_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearSpinalIVDDDischarge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureSpinalService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearSpinalIVDDDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := payload.DischargeSummary
	if strings.TrimSpace(summary) == "" {
		summary = payload.Summary
	}

	updated, err := svc.ClearNeurologyDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastSpinalIVDDEvent("spinal_ivdd_updated", map[string]any{
		"type":      "spinal_ivdd_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleSpinalIVDDStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "spinal_ivdd")
}

func (s *Server) broadcastSpinalIVDDEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("spinal_ivdd", "spinal-ivdd-hub", eventType, payload, now)
}
