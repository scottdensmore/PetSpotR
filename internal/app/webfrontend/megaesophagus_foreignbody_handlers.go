package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/megaesophagusfb"
)

// MegaesophagusPageData represents template data passed to templates/megaesophagus_foreignbody_triage.html.
type MegaesophagusPageData struct {
	Operations                         []domain.MegaesophagusOperation
	ActiveOperation                    *domain.MegaesophagusOperation
	Metrics                            domain.MegaesophagusOverviewMetrics
	TotalOperations                    int
	AcuteObstructionTriageCount        int
	EndoscopyPyridostigmineStagedCount int
	ClearedBaileyChairCount            int
	ForeignBodyObstructionCount        int
	AspirationPneumoniaActiveCount     int
	Locale                             string
}

type createMegaesophagusOperationPayload struct {
	CaseID                       string                         `json:"case_id"`
	PetName                      string                         `json:"pet_name"`
	Species                      string                         `json:"species"`
	Breed                        string                         `json:"breed"`
	WeightKg                     float64                        `json:"weight_kg"`
	PathologyType                domain.EsophagealPathologyType `json:"pathology_type"`
	SpO2Percent                  float64                        `json:"spo2_percent"`
	AlveolarArterialGradientMmHg float64                        `json:"alveolar_arterial_gradient_mmhg"`
	RegurgitationEpisodes24h     int                            `json:"regurgitation_episodes_24h"`
	UprightHoldMinutes           int                            `json:"upright_hold_minutes"`
	MucosalEsophagitisGrade      int                            `json:"mucosal_esophagitis_grade"`
	AChRAntibodyTiterNmolL       float64                        `json:"achr_antibody_titer_nmol_l"`
	AspirationPneumoniaPresent   bool                           `json:"aspiration_pneumonia_present"`
}

type stageMegaesophagusBaileyPayload struct {
	EndoscopicRetrievalOrGastricPush  string `json:"endoscopic_retrieval_or_gastric_push"`
	RetrievalPlan                     string `json:"retrieval_plan"`
	PostRemovalSucralfateOmeprazole   string `json:"post_removal_sucralfate_omeprazole"`
	SucralfateOmeprazole              string `json:"sucralfate_omeprazole"`
	PyridostigmineAnticholinesterase  string `json:"pyridostigmine_anticholinesterase"`
	PyridostigminePlan                string `json:"pyridostigmine_plan"`
	AspirationPneumoniaAntibioticPlan string `json:"aspiration_pneumonia_antibiotic_plan"`
	PneumoniaAbxPlan                  string `json:"pneumonia_abx_plan"`
	UprightBaileyChairFeedingProtocol string `json:"upright_bailey_chair_feeding_protocol"`
	BaileyChairProtocol               string `json:"bailey_chair_protocol"`
	SildenafilLESRelaxationNote       string `json:"sildenafil_les_relaxation_note"`
	SildenafilNote                    string `json:"sildenafil_note"`
}

type recordMegaesophagusCheckPayload struct {
	SpO2Percent                  float64 `json:"spo2_percent"`
	AlveolarArterialGradientMmHg float64 `json:"alveolar_arterial_gradient_mmhg"`
	RegurgitationEpisodes24h     int     `json:"regurgitation_episodes_24h"`
	UprightHoldMinutes           int     `json:"upright_hold_minutes"`
	MucosalEsophagitisGrade      int     `json:"mucosal_esophagitis_grade"`
	RecordedBy                   string  `json:"recorded_by"`
	ClinicalNote                 string  `json:"clinical_note"`
}

type clearMegaesophagusDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureMegaesophagusService() *megaesophagusfb.MegaesophagusService {
	if s.megaService == nil {
		s.megaService = megaesophagusfb.NewMegaesophagusService()
	}
	return s.megaService
}

func (s *Server) registerMegaesophagusForeignBodyRoutes() {
	s.mux.HandleFunc("GET /megaesophagus-foreignbody-triage", s.handleMegaesophagusForeignBodyTriagePage)

	s.registerDualAPIRoute("GET", "megaesophagus-foreignbody/operations", s.handleListMegaesophagusOperations)
	s.registerDualAPIRoute("POST", "megaesophagus-foreignbody/operations", s.handleCreateMegaesophagusOperation)
	s.registerDualAPIRoute("GET", "megaesophagus-foreignbody/operations/{id}", s.handleGetMegaesophagusOperation)
	s.registerDualAPIRoute("POST", "megaesophagus-foreignbody/operations/{id}/stage-bailey", s.handleStageMegaesophagusBaileyPlan)
	s.registerDualAPIRoute("POST", "megaesophagus-foreignbody/operations/{id}/record-check", s.handleRecordMegaesophagusCheck)
	s.registerDualAPIRoute("POST", "megaesophagus-foreignbody/operations/{id}/clear-discharge", s.handleClearMegaesophagusDischarge)
	s.registerDualAPIRoute("GET", "megaesophagus-foreignbody/stream", s.handleMegaesophagusStream)
}

func (s *Server) handleMegaesophagusForeignBodyTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureMegaesophagusService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.MegaesophagusOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := MegaesophagusPageData{
		Operations:                         ops,
		ActiveOperation:                    activeOp,
		Metrics:                            metrics,
		TotalOperations:                    metrics.TotalOperations,
		AcuteObstructionTriageCount:        metrics.AcuteObstructionTriageCount,
		EndoscopyPyridostigmineStagedCount: metrics.EndoscopyPyridostigmineStagedCount,
		ClearedBaileyChairCount:            metrics.ClearedBaileyChairCount,
		ForeignBodyObstructionCount:        metrics.ForeignBodyObstructionCount,
		AspirationPneumoniaActiveCount:     metrics.AspirationPneumoniaActiveCount,
		Locale:                             locale,
	}

	s.renderCockpitTemplate(w, r, "megaesophagus_foreignbody_triage.html", "Megaesophagus & Foreign Body Triage", data)
}

func (s *Server) handleListMegaesophagusOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureMegaesophagusService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	pathologyFilter := strings.TrimSpace(r.URL.Query().Get("pathology"))

	ops := svc.ListOperations(statusFilter)
	if pathologyFilter != "" {
		filtered := make([]domain.MegaesophagusOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.PathologyType), pathologyFilter) {
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

func (s *Server) handleGetMegaesophagusOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureMegaesophagusService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateMegaesophagusOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createMegaesophagusOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureMegaesophagusService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-MEGA-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.PathologyType,
		payload.SpO2Percent,
		payload.AlveolarArterialGradientMmHg,
		payload.RegurgitationEpisodes24h,
		payload.UprightHoldMinutes,
		payload.MucosalEsophagitisGrade,
		payload.AChRAntibodyTiterNmolL,
		payload.AspirationPneumoniaPresent,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMegaesophagusEvent("megaesophagus_foreignbody_updated", map[string]any{
		"type":      "megaesophagus_foreignbody_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageMegaesophagusBaileyPlan(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureMegaesophagusService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageMegaesophagusBaileyPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	retrieval := firstNonEmpty(payload.EndoscopicRetrievalOrGastricPush, payload.RetrievalPlan, existing.BaileyPlan.EndoscopicRetrievalOrGastricPush)
	sucralfate := firstNonEmpty(payload.PostRemovalSucralfateOmeprazole, payload.SucralfateOmeprazole, existing.BaileyPlan.PostRemovalSucralfateOmeprazole)
	pyrido := firstNonEmpty(payload.PyridostigmineAnticholinesterase, payload.PyridostigminePlan, existing.BaileyPlan.PyridostigmineAnticholinesterase)
	abx := firstNonEmpty(payload.AspirationPneumoniaAntibioticPlan, payload.PneumoniaAbxPlan, existing.BaileyPlan.AspirationPneumoniaAntibioticPlan)
	bailey := firstNonEmpty(payload.UprightBaileyChairFeedingProtocol, payload.BaileyChairProtocol, existing.BaileyPlan.UprightBaileyChairFeedingProtocol)
	sildenafil := firstNonEmpty(payload.SildenafilLESRelaxationNote, payload.SildenafilNote, existing.BaileyPlan.SildenafilLESRelaxationNote)

	updated, err := svc.StageEndoscopyBaileyPlan(id, retrieval, sucralfate, pyrido, abx, bailey, sildenafil)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMegaesophagusEvent("megaesophagus_foreignbody_updated", map[string]any{
		"type":      "megaesophagus_foreignbody_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordMegaesophagusCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureMegaesophagusService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordMegaesophagusCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordEsophagealCheck(
		id,
		payload.SpO2Percent,
		payload.AlveolarArterialGradientMmHg,
		payload.RegurgitationEpisodes24h,
		payload.UprightHoldMinutes,
		payload.MucosalEsophagitisGrade,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMegaesophagusEvent("megaesophagus_foreignbody_updated", map[string]any{
		"type":      "megaesophagus_foreignbody_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearMegaesophagusDischarge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureMegaesophagusService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearMegaesophagusDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearMegaesophagusDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMegaesophagusEvent("megaesophagus_foreignbody_updated", map[string]any{
		"type":      "megaesophagus_foreignbody_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleMegaesophagusStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "megaesophagus_foreignbody")
}

func (s *Server) broadcastMegaesophagusEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("megaesophagus_foreignbody", "megaesophagus-foreignbody-hub", eventType, payload, now)
}
