package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/polytraumaortho"
)

// PolytraumaOrthoPageData represents template data passed to templates/polytrauma_orthopedic_triage.html.
type PolytraumaOrthoPageData struct {
	Operations                   []domain.PolytraumaOrthoOperation
	ActiveOperation              *domain.PolytraumaOrthoOperation
	Metrics                      domain.PolytraumaOrthoOverviewMetrics
	TotalOperations              int
	AcuteOpenFractureTriageCount int
	NPWTSplintStagedCount        int
	ClearedFixationCount         int
	GustiloGradeIIIShearingCount int
	ThoracicBitePenetrationCount int
	Locale                       string
}

type createPolytraumaOrthoOperationPayload struct {
	CaseID                           string                             `json:"case_id"`
	PetName                          string                             `json:"pet_name"`
	Species                          string                             `json:"species"`
	Breed                            string                             `json:"breed"`
	WeightKg                         float64                            `json:"weight_kg"`
	GustiloClass                     domain.GustiloInjuryClassification `json:"gustilo_class"`
	AnimalTraumaTriageScore          int                                `json:"animal_trauma_triage_score"`
	LavageVolumeML                   int                                `json:"lavage_volume_ml"`
	NPWTVacuumMmHg                   int                                `json:"npwt_vacuum_mmhg"`
	BloodLactateMmolL                float64                            `json:"blood_lactate_mmol_l"`
	DistalToeTempDeltaC              float64                            `json:"distal_toe_temp_delta_c"`
	ExposedCorticalBoneTendon        bool                               `json:"exposed_cortical_bone_tendon"`
	ThoracicWallPenetrationSuspected bool                               `json:"thoracic_wall_penetration_suspected"`
}

type stagePolytraumaOrthoFixationPayload struct {
	SterileWaterSolubleGelClipPlan    string `json:"sterile_water_soluble_gel_clip_plan"`
	GelClipPlan                       string `json:"gel_clip_plan"`
	CopiousLavage35mLSyringe18GPlan   string `json:"copious_lavage_35_ml_syringe_18g_plan"`
	LavagePlan                        string `json:"lavage_plan"`
	NPWTVacuumOrTieOverDressingPlan   string `json:"npwt_vacuum_or_tie_over_dressing_plan"`
	NPWTOrTieOverPlan                 string `json:"npwt_or_tie_over_plan"`
	EmpiricAbxCultureBeforeLavage     string `json:"empiric_abx_culture_before_lavage"`
	AbxCulturePlan                    string `json:"abx_culture_plan"`
	RobertJonesOrSpicaSplintPlan      string `json:"robert_jones_or_spica_splint_plan"`
	SplintPlan                        string `json:"splint_plan"`
	DefinitiveExternalSkeletalFixator string `json:"definitive_external_skeletal_fixator"`
	ESFFixationPlan                   string `json:"esf_fixation_plan"`
}

type recordPolytraumaOrthoWoundCheckPayload struct {
	AnimalTraumaTriageScore int     `json:"animal_trauma_triage_score"`
	LavageVolumeML          int     `json:"lavage_volume_ml"`
	NPWTVacuumMmHg          int     `json:"npwt_vacuum_mmhg"`
	BloodLactateMmolL       float64 `json:"blood_lactate_mmol_l"`
	DistalToeTempDeltaC     float64 `json:"distal_toe_temp_delta_c"`
	RecordedBy              string  `json:"recorded_by"`
	ClinicalNote            string  `json:"clinical_note"`
}

type clearPolytraumaOrthoDischargePayload struct {
	DischargedBy     string `json:"discharged_by"`
	DischargeSummary string `json:"discharge_summary"`
	Summary          string `json:"summary"`
}

func (s *Server) ensureOrthoService() *polytraumaortho.OrthoService {
	if s.orthoService == nil {
		s.orthoService = polytraumaortho.NewOrthoService()
	}
	return s.orthoService
}

func (s *Server) registerPolytraumaOrthopedicRoutes() {
	s.mux.HandleFunc("GET /polytrauma-orthopedic-triage", s.handlePolytraumaOrthoTriagePage)

	s.registerDualAPIRoute("GET", "polytrauma-orthopedic/operations", s.handleListPolytraumaOrthoOperations)
	s.registerDualAPIRoute("POST", "polytrauma-orthopedic/operations", s.handleCreatePolytraumaOrthoOperation)
	s.registerDualAPIRoute("GET", "polytrauma-orthopedic/operations/{id}", s.handleGetPolytraumaOrthoOperation)
	s.registerDualAPIRoute("POST", "polytrauma-orthopedic/operations/{id}/stage-fixation", s.handleStagePolytraumaOrthoFixation)
	s.registerDualAPIRoute("POST", "polytrauma-orthopedic/operations/{id}/record-check", s.handleRecordPolytraumaOrthoWoundCheck)
	s.registerDualAPIRoute("POST", "polytrauma-orthopedic/operations/{id}/clear-discharge", s.handleClearPolytraumaOrthoDischarge)
	s.registerDualAPIRoute("GET", "polytrauma-orthopedic/stream", s.handlePolytraumaOrthoStream)
}

func (s *Server) handlePolytraumaOrthoTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureOrthoService()
	ops := svc.ListOperations("")
	metrics := svc.GetOverviewMetrics()

	var activeOp *domain.PolytraumaOrthoOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := PolytraumaOrthoPageData{
		Operations:                   ops,
		ActiveOperation:              activeOp,
		Metrics:                      metrics,
		TotalOperations:              metrics.TotalOperations,
		AcuteOpenFractureTriageCount: metrics.AcuteOpenFractureTriageCount,
		NPWTSplintStagedCount:        metrics.NPWTSplintStagedCount,
		ClearedFixationCount:         metrics.ClearedFixationCount,
		GustiloGradeIIIShearingCount: metrics.GustiloGradeIIIShearingCount,
		ThoracicBitePenetrationCount: metrics.ThoracicBitePenetrationCount,
		Locale:                       locale,
	}

	s.renderCockpitTemplate(w, r, "polytrauma_orthopedic_triage.html", "Polytrauma Orthopedic Triage", data)
}

func (s *Server) handleListPolytraumaOrthoOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureOrthoService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	gustiloFilter := strings.TrimSpace(r.URL.Query().Get("gustilo_class"))

	ops := svc.ListOperations(statusFilter)
	if gustiloFilter != "" {
		filtered := make([]domain.PolytraumaOrthoOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.GustiloClass), gustiloFilter) {
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

func (s *Server) handleGetPolytraumaOrthoOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureOrthoService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreatePolytraumaOrthoOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createPolytraumaOrthoOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureOrthoService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-ORTHO-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.GustiloClass,
		payload.AnimalTraumaTriageScore,
		payload.LavageVolumeML,
		payload.NPWTVacuumMmHg,
		payload.BloodLactateMmolL,
		payload.DistalToeTempDeltaC,
		payload.ExposedCorticalBoneTendon,
		payload.ThoracicWallPenetrationSuspected,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPolytraumaOrthoEvent("polytrauma_ortho_updated", map[string]any{
		"type":      "polytrauma_ortho_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStagePolytraumaOrthoFixation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureOrthoService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stagePolytraumaOrthoFixationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	gelClip := firstNonEmpty(payload.SterileWaterSolubleGelClipPlan, payload.GelClipPlan, existing.FixationPlan.SterileWaterSolubleGelClipPlan)
	lavage := firstNonEmpty(payload.CopiousLavage35mLSyringe18GPlan, payload.LavagePlan, existing.FixationPlan.CopiousLavage35mLSyringe18GPlan)
	npwt := firstNonEmpty(payload.NPWTVacuumOrTieOverDressingPlan, payload.NPWTOrTieOverPlan, existing.FixationPlan.NPWTVacuumOrTieOverDressingPlan)
	abx := firstNonEmpty(payload.EmpiricAbxCultureBeforeLavage, payload.AbxCulturePlan, existing.FixationPlan.EmpiricAbxCultureBeforeLavage)
	splint := firstNonEmpty(payload.RobertJonesOrSpicaSplintPlan, payload.SplintPlan, existing.FixationPlan.RobertJonesOrSpicaSplintPlan)
	esf := firstNonEmpty(payload.DefinitiveExternalSkeletalFixator, payload.ESFFixationPlan, existing.FixationPlan.DefinitiveExternalSkeletalFixator)

	updated, err := svc.StageDebridementFixation(id, gelClip, lavage, npwt, abx, splint, esf)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPolytraumaOrthoEvent("polytrauma_ortho_updated", map[string]any{
		"type":      "polytrauma_ortho_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordPolytraumaOrthoWoundCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureOrthoService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordPolytraumaOrthoWoundCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordWoundCheck(
		id,
		payload.AnimalTraumaTriageScore,
		payload.LavageVolumeML,
		payload.NPWTVacuumMmHg,
		payload.BloodLactateMmolL,
		payload.DistalToeTempDeltaC,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPolytraumaOrthoEvent("polytrauma_ortho_updated", map[string]any{
		"type":      "polytrauma_ortho_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearPolytraumaOrthoDischarge(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureOrthoService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearPolytraumaOrthoDischargePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	summary := firstNonEmpty(payload.DischargeSummary, payload.Summary)
	updated, err := svc.ClearOrthoDischarge(id, payload.DischargedBy, summary)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPolytraumaOrthoEvent("polytrauma_ortho_updated", map[string]any{
		"type":      "polytrauma_ortho_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handlePolytraumaOrthoStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "polytrauma_ortho")
}

func (s *Server) broadcastPolytraumaOrthoEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("polytrauma_ortho", "polytrauma-ortho-hub", eventType, payload, now)
}
