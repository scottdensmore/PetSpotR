package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/masscasualtyvmat"
)

// MassCasualtyVMATPageData represents template data passed to templates/mass_casualty_vmat_triage.html.
type MassCasualtyVMATPageData struct {
	Operations                       []domain.MassCasualtyVMATOperation
	ActiveOperation                  *domain.MassCasualtyVMATOperation
	Metrics                          domain.MassCasualtyVMATOverview
	TotalOperations                  int
	StartSaltTaggedDeconStagedCount  int
	DamageControlSurgeMonitoredCount int
	ReunifiedEvacClearedCount        int
	RedImmediatePriority1Count       int
	ReverseTriageSurgeTriggeredCount int
	ColdZoneDeconVerifiedCount       int
	Locale                           string
}

type createMassCasualtyVMATOperationPayload struct {
	CaseID                                      string                    `json:"case_id"`
	PetName                                     string                    `json:"pet_name"`
	Species                                     string                    `json:"species"`
	Breed                                       string                    `json:"breed"`
	WeightKg                                    float64                   `json:"weight_kg"`
	TriageTag                                   domain.VeterinarySTARTTag `json:"triage_tag"`
	Protocol                                    domain.VMATSurgeProtocol  `json:"protocol"`
	AnimalTriageScoreATS                        int                       `json:"animal_triage_score_ats"`
	LactateMmolL                                float64                   `json:"lactate_mmol_l"`
	SpO2Percent                                 float64                   `json:"spo2_percent"`
	SurgeBedOccupancyPercent                    float64                   `json:"surge_bed_occupancy_percent"`
	GrossDecontaminationPriorToColdZoneVerified *bool                     `json:"gross_decontamination_prior_to_cold_zone_verified"`
	WaterproofRFIDTriageCollarNumber            string                    `json:"waterproof_rfid_triage_collar_number"`
	ICS214IncidentNumber                        string                    `json:"ics214_incident_number"`
	ContagiousCohortSegregated                  *bool                     `json:"contagious_cohort_segregated"`
}

type stageMassCasualtyVMATPayload struct {
	Protocol                                    domain.VMATSurgeProtocol  `json:"protocol"`
	TriageTag                                   domain.VeterinarySTARTTag `json:"triage_tag"`
	GrossDecontaminationPriorToColdZoneVerified *bool                     `json:"gross_decontamination_prior_to_cold_zone_verified"`
	WarmZoneDecontaminationCorridorPlan         string                    `json:"warm_zone_decontamination_corridor_plan"`
	DamageControlResuscitationPlan              string                    `json:"damage_control_resuscitation_plan"`
	WaterproofRFIDTriageCollarNumber            string                    `json:"waterproof_rfid_triage_collar_number"`
	ICS214IncidentNumber                        string                    `json:"ics214_incident_number"`
	ICS214ChainOfCustodyPlan                    string                    `json:"ics214_chain_of_custody_plan"`
	ContagiousCohortSegregated                  *bool                     `json:"contagious_cohort_segregated"`
	BiosecurityCohortIsolationPlan              string                    `json:"biosecurity_cohort_isolation_plan"`
	SurgeBedOccupancyPercent                    float64                   `json:"surge_bed_occupancy_percent"`
	ReverseTriageSurgeAllocationPlan            string                    `json:"reverse_triage_surge_allocation_plan"`
	StagedBy                                    string                    `json:"staged_by"`
}

type recordMassCasualtyVMATReassessmentPayload struct {
	TriageTag                                   domain.VeterinarySTARTTag `json:"triage_tag"`
	AnimalTriageScoreATS                        int                       `json:"animal_triage_score_ats"`
	LactateMmolL                                float64                   `json:"lactate_mmol_l"`
	SpO2Percent                                 float64                   `json:"spo2_percent"`
	SurgeBedOccupancyPercent                    float64                   `json:"surge_bed_occupancy_percent"`
	GrossDecontaminationPriorToColdZoneVerified *bool                     `json:"gross_decontamination_prior_to_cold_zone_verified"`
	ContagiousCohortSegregated                  *bool                     `json:"contagious_cohort_segregated"`
	RecordedBy                                  string                    `json:"recorded_by"`
	ClinicalNote                                string                    `json:"clinical_note"`
}

type evaluateMassCasualtyVMATClearancePayload struct {
	LactateMmolL                  *float64 `json:"lactate_mmol_l"`
	SpO2Percent                   *float64 `json:"spo2_percent"`
	WaterproofRFIDMatchedVerified *bool    `json:"waterproof_rfid_matched_verified"`
	ICS214ChainOfCustodySigned    *bool    `json:"ics214_chain_of_custody_signed"`
	ClearedBy                     string   `json:"cleared_by"`
	DischargedBy                  string   `json:"discharged_by"`
	ClearanceSummary              string   `json:"clearance_summary"`
	DischargeSummary              string   `json:"discharge_summary"`
	Summary                       string   `json:"summary"`
}

func (s *Server) ensureMassCasualtyVMATService() *masscasualtyvmat.MassCasualtyVMATService {
	if s.massCasualtyVMATService == nil {
		s.massCasualtyVMATService = masscasualtyvmat.NewMassCasualtyVMATService()
	}
	return s.massCasualtyVMATService
}

func (s *Server) registerMassCasualtyVMATRoutes() {
	s.mux.HandleFunc("GET /mass-casualty-vmat-triage", s.handleMassCasualtyVMATTriagePage)

	s.registerDualAPIRoute("GET", "mass-casualty-vmat/overview", s.handleMassCasualtyVMATOverview)
	s.registerDualAPIRoute("GET", "mass-casualty-vmat/operations", s.handleListMassCasualtyVMATOperations)
	s.registerDualAPIRoute("POST", "mass-casualty-vmat/operations", s.handleCreateMassCasualtyVMATOperation)
	s.registerDualAPIRoute("GET", "mass-casualty-vmat/operations/{id}", s.handleGetMassCasualtyVMATOperation)
	s.registerDualAPIRoute("POST", "mass-casualty-vmat/operations/{id}/staging", s.handleStageMassCasualtyVMAT)
	s.registerDualAPIRoute("POST", "mass-casualty-vmat/operations/{id}/reassessment", s.handleRecordMassCasualtyVMATReassessment)
	s.registerDualAPIRoute("POST", "mass-casualty-vmat/operations/{id}/clearance", s.handleEvaluateMassCasualtyVMATClearance)
	s.registerDualAPIRoute("GET", "mass-casualty-vmat/stream", s.handleMassCasualtyVMATStream)
}

func (s *Server) handleMassCasualtyVMATTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureMassCasualtyVMATService()
	ops := svc.ListOperations("")
	metrics := svc.CalculateOverview()

	var activeOp *domain.MassCasualtyVMATOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := MassCasualtyVMATPageData{
		Operations:                       ops,
		ActiveOperation:                  activeOp,
		Metrics:                          metrics,
		TotalOperations:                  metrics.TotalOperations,
		StartSaltTaggedDeconStagedCount:  metrics.StartSaltTaggedDeconStagedCount,
		DamageControlSurgeMonitoredCount: metrics.DamageControlSurgeMonitoredCount,
		ReunifiedEvacClearedCount:        metrics.ReunifiedEvacClearedCount,
		RedImmediatePriority1Count:       metrics.RedImmediatePriority1Count,
		ReverseTriageSurgeTriggeredCount: metrics.ReverseTriageSurgeTriggeredCount,
		ColdZoneDeconVerifiedCount:       metrics.ColdZoneDeconVerifiedCount,
		Locale:                           locale,
	}

	s.renderCockpitTemplate(w, r, "mass_casualty_vmat_triage.html", "Mass-Casualty VMAT Triage", data)
}

func (s *Server) handleMassCasualtyVMATOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureMassCasualtyVMATService()
	overview := svc.CalculateOverview()

	writeCockpitJSON(w, http.StatusOK, overview)
}

func (s *Server) handleListMassCasualtyVMATOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureMassCasualtyVMATService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	tagFilter := strings.TrimSpace(r.URL.Query().Get("triage_tag"))

	ops := svc.ListOperations(statusFilter)
	if tagFilter != "" {
		filtered := make([]domain.MassCasualtyVMATOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.TriageTag), tagFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	metrics := svc.CalculateOverview()

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": ops,
		"ops":        ops,
		"metrics":    metrics,
		"overview":   metrics,
	})
}

func (s *Server) handleGetMassCasualtyVMATOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureMassCasualtyVMATService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateMassCasualtyVMATOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createMassCasualtyVMATOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureMassCasualtyVMATService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-MCI-%d", 100+len(svc.ListOperations(""))+1)
	}

	grossDecon := true
	if payload.GrossDecontaminationPriorToColdZoneVerified != nil {
		grossDecon = *payload.GrossDecontaminationPriorToColdZoneVerified
	}
	contagiousSegregated := true
	if payload.ContagiousCohortSegregated != nil {
		contagiousSegregated = *payload.ContagiousCohortSegregated
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.TriageTag,
		payload.Protocol,
		payload.AnimalTriageScoreATS,
		payload.LactateMmolL,
		payload.SpO2Percent,
		payload.SurgeBedOccupancyPercent,
		grossDecon,
		payload.WaterproofRFIDTriageCollarNumber,
		payload.ICS214IncidentNumber,
		contagiousSegregated,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMassCasualtyVMATEvent("mass_casualty_vmat_updated", map[string]any{
		"type":      "mass_casualty_vmat_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageMassCasualtyVMAT(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureMassCasualtyVMATService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageMassCasualtyVMATPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	protocol := payload.Protocol
	if protocol == "" {
		protocol = existing.Protocol
	}
	triageTag := payload.TriageTag
	if triageTag == "" {
		triageTag = existing.TriageTag
	}
	grossDecon := existing.GrossDecontaminationPriorToColdZoneVerified
	if payload.GrossDecontaminationPriorToColdZoneVerified != nil {
		grossDecon = *payload.GrossDecontaminationPriorToColdZoneVerified
	}
	contagiousSegregated := existing.ContagiousCohortSegregated
	if payload.ContagiousCohortSegregated != nil {
		contagiousSegregated = *payload.ContagiousCohortSegregated
	}

	deconPlan := firstNonEmpty(
		payload.WarmZoneDecontaminationCorridorPlan,
		existing.FieldTriageStaging.WarmZoneDecontaminationCorridorPlan,
	)
	dcrPlan := firstNonEmpty(
		payload.DamageControlResuscitationPlan,
		existing.FieldTriageStaging.DamageControlResuscitationPlan,
	)
	rfidCollar := firstNonEmpty(
		payload.WaterproofRFIDTriageCollarNumber,
		existing.WaterproofRFIDTriageCollarNumber,
	)
	icsNumber := firstNonEmpty(
		payload.ICS214IncidentNumber,
		existing.ICS214IncidentNumber,
	)
	icsPlan := firstNonEmpty(
		payload.ICS214ChainOfCustodyPlan,
		existing.FieldTriageStaging.ICS214ChainOfCustodyPlan,
	)
	bioPlan := firstNonEmpty(
		payload.BiosecurityCohortIsolationPlan,
		existing.FieldTriageStaging.BiosecurityCohortIsolationPlan,
	)
	reversePlan := firstNonEmpty(
		payload.ReverseTriageSurgeAllocationPlan,
		existing.FieldTriageStaging.ReverseTriageSurgeAllocationPlan,
	)

	updated, err := svc.StageFieldTriage(
		id,
		protocol,
		triageTag,
		grossDecon,
		deconPlan,
		dcrPlan,
		rfidCollar,
		icsNumber,
		icsPlan,
		contagiousSegregated,
		bioPlan,
		payload.SurgeBedOccupancyPercent,
		reversePlan,
		payload.StagedBy,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMassCasualtyVMATEvent("mass_casualty_vmat_updated", map[string]any{
		"type":      "mass_casualty_vmat_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordMassCasualtyVMATReassessment(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureMassCasualtyVMATService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordMassCasualtyVMATReassessmentPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	grossDecon := existing.GrossDecontaminationPriorToColdZoneVerified
	if payload.GrossDecontaminationPriorToColdZoneVerified != nil {
		grossDecon = *payload.GrossDecontaminationPriorToColdZoneVerified
	}
	contagiousSegregated := existing.ContagiousCohortSegregated
	if payload.ContagiousCohortSegregated != nil {
		contagiousSegregated = *payload.ContagiousCohortSegregated
	}

	updated, err := svc.RecordReassessmentCheck(
		id,
		payload.TriageTag,
		payload.AnimalTriageScoreATS,
		payload.LactateMmolL,
		payload.SpO2Percent,
		payload.SurgeBedOccupancyPercent,
		grossDecon,
		contagiousSegregated,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMassCasualtyVMATEvent("mass_casualty_vmat_updated", map[string]any{
		"type":      "mass_casualty_vmat_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleEvaluateMassCasualtyVMATClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureMassCasualtyVMATService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload evaluateMassCasualtyVMATClearancePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	lactate := 1.85
	if payload.LactateMmolL != nil {
		lactate = *payload.LactateMmolL
	}
	spo2 := 97.5
	if payload.SpO2Percent != nil {
		spo2 = *payload.SpO2Percent
	}
	rfidVerified := true
	if payload.WaterproofRFIDMatchedVerified != nil {
		rfidVerified = *payload.WaterproofRFIDMatchedVerified
	}
	icsSigned := true
	if payload.ICS214ChainOfCustodySigned != nil {
		icsSigned = *payload.ICS214ChainOfCustodySigned
	}
	clearedBy := firstNonEmpty(payload.ClearedBy, payload.DischargedBy)
	summary := firstNonEmpty(payload.ClearanceSummary, payload.DischargeSummary, payload.Summary)

	updated, err := svc.EvaluateClearance(
		id,
		lactate,
		spo2,
		rfidVerified,
		icsSigned,
		clearedBy,
		summary,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastMassCasualtyVMATEvent("mass_casualty_vmat_updated", map[string]any{
		"type":      "mass_casualty_vmat_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleMassCasualtyVMATStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "mass_casualty_vmat")
}

func (s *Server) broadcastMassCasualtyVMATEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("mass_casualty_vmat", "mass-casualty-vmat-hub", eventType, payload, now)
}
