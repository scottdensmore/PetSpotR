package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/vectorbornerickettsial"
)

// VectorBorneRickettsialPageData represents template data passed to templates/vector_borne_rickettsial_triage.html.
type VectorBorneRickettsialPageData struct {
	Operations                      []domain.VectorBorneOperation
	ActiveOperation                 *domain.VectorBorneOperation
	Overview                        domain.VectorBorneOverview
	TotalOperations                 int
	AntimicrobialStagedCount        int
	ThrombocytopeniaMonitoredCount  int
	PCRNegativeClearedCount         int
	LymeNephritisProteinuriaCount   int
	SevereThrombocytopeniaCount     int
	BabesiaCytauxzoonProtozoalCount int
	Locale                          string
}

type createVectorBorneOperationPayload struct {
	CaseID                       string                          `json:"case_id"`
	PetName                      string                          `json:"pet_name"`
	Species                      string                          `json:"species"`
	Breed                        string                          `json:"breed"`
	WeightKg                     float64                         `json:"weight_kg"`
	Syndrome                     domain.VectorBorneSyndrome      `json:"syndrome"`
	Assay                        domain.VectorBorneAssay         `json:"assay"`
	AntimicrobialProtocol        domain.VectorBorneAntimicrobial `json:"antimicrobial_protocol"`
	PlateletCountKUL             float64                         `json:"platelet_count_kul"`
	PCVPercent                   float64                         `json:"pcv_percent"`
	UrinePCRatio                 float64                         `json:"urine_pc_ratio"`
	SerumAlbuminGDL              float64                         `json:"serum_albumin_g_dl"`
	DoxycyclineMgKgDay           float64                         `json:"doxycycline_mg_kg_day"`
	AtovaquoneMgKgTID            float64                         `json:"atovaquone_mg_kg_tid"`
	ImidocarbMgKgIM              float64                         `json:"imidocarb_mg_kg_im"`
	EsophagealWaterChaseVerified bool                            `json:"esophageal_water_chase_verified"`
	AtropinePremedVerified       bool                            `json:"atropine_premed_verified"`
}

type stageVectorBorneAntimicrobialPayload struct {
	Protocol                          domain.VectorBorneAntimicrobial `json:"protocol"`
	AntimicrobialProtocol             domain.VectorBorneAntimicrobial `json:"antimicrobial_protocol"`
	DoxycyclineMgKgDay                *float64                        `json:"doxycycline_mg_kg_day"`
	AtovaquoneMgKgTID                 *float64                        `json:"atovaquone_mg_kg_tid"`
	ImidocarbMgKgIM                   *float64                        `json:"imidocarb_mg_kg_im"`
	EsophagealWaterChaseVerified      *bool                           `json:"esophageal_water_chase_verified"`
	AtropinePremedVerified            *bool                           `json:"atropine_premed_verified"`
	FattyMealBioavailabilityNote      string                          `json:"fatty_meal_bioavailability_note"`
	LymeNephritisImmunomodulationNote string                          `json:"lyme_nephritis_immunomodulation_note"`
	StagedBy                          string                          `json:"staged_by"`
	ClinicalRationale                 string                          `json:"clinical_rationale"`
}

type recordVectorBorneHematologyCheckPayload struct {
	PlateletCountKUL           float64 `json:"platelet_count_kul"`
	PCVPercent                 float64 `json:"pcv_percent"`
	UrinePCRatio               float64 `json:"urine_pc_ratio"`
	SerumAlbuminGDL            float64 `json:"serum_albumin_g_dl"`
	MorulaeOrPiroplasmsCleared bool    `json:"morulae_or_piroplasms_cleared"`
	RecordedBy                 string  `json:"recorded_by"`
	ClinicalNote               string  `json:"clinical_note"`
}

type clearVectorBorneOperationPayload struct {
	PlateletCountKUL           *float64 `json:"platelet_count_kul"`
	PCVPercent                 *float64 `json:"pcv_percent"`
	UrinePCRatio               *float64 `json:"urine_pc_ratio"`
	PCRNegativeVerified        *bool    `json:"pcr_negative_verified"`
	NormothermicEatingVerified *bool    `json:"normothermic_eating_verified"`
	ClearedBy                  string   `json:"cleared_by"`
	DischargedBy               string   `json:"discharged_by"`
	ClearanceSummary           string   `json:"clearance_summary"`
	DischargeSummary           string   `json:"discharge_summary"`
	Summary                    string   `json:"summary"`
}

func (s *Server) ensureVectorBorneService() *vectorbornerickettsial.VectorBorneService {
	if s.vectorBorneService == nil {
		s.vectorBorneService = vectorbornerickettsial.NewVectorBorneService()
	}
	return s.vectorBorneService
}

func (s *Server) registerVectorBorneRickettsialRoutes() {
	s.mux.HandleFunc("GET /vector-borne-rickettsial-triage", s.handleVectorBorneRickettsialTriagePage)

	s.registerDualAPIRoute("GET", "vector-borne-rickettsial/overview", s.handleGetVectorBorneRickettsialOverview)
	s.registerDualAPIRoute("GET", "vector-borne-rickettsial/operations", s.handleListVectorBorneRickettsialOperations)
	s.registerDualAPIRoute("POST", "vector-borne-rickettsial/operations", s.handleCreateVectorBorneRickettsialOperation)
	s.registerDualAPIRoute("GET", "vector-borne-rickettsial/operations/{id}", s.handleGetVectorBorneRickettsialOperation)
	s.registerDualAPIRoute("POST", "vector-borne-rickettsial/operations/{id}/antimicrobial", s.handleStageVectorBorneRickettsialAntimicrobial)
	s.registerDualAPIRoute("POST", "vector-borne-rickettsial/operations/{id}/hematology-check", s.handleRecordVectorBorneRickettsialHematologyCheck)
	s.registerDualAPIRoute("POST", "vector-borne-rickettsial/operations/{id}/clearance", s.handleClearVectorBorneRickettsialOperation)
	s.registerDualAPIRoute("GET", "vector-borne-rickettsial/stream", s.handleVectorBorneRickettsialStream)
}

func (s *Server) handleVectorBorneRickettsialTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureVectorBorneService()
	ops := svc.ListOperations("")
	overview := svc.CalculateOverview()

	var activeOp *domain.VectorBorneOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := VectorBorneRickettsialPageData{
		Operations:                      ops,
		ActiveOperation:                 activeOp,
		Overview:                        overview,
		TotalOperations:                 overview.TotalOperations,
		AntimicrobialStagedCount:        overview.AntimicrobialStagedCount,
		ThrombocytopeniaMonitoredCount:  overview.ThrombocytopeniaMonitoredCount,
		PCRNegativeClearedCount:         overview.PCRNegativeClearedCount,
		LymeNephritisProteinuriaCount:   overview.LymeNephritisProteinuriaCount,
		SevereThrombocytopeniaCount:     overview.SevereThrombocytopeniaCount,
		BabesiaCytauxzoonProtozoalCount: overview.BabesiaCytauxzoonProtozoalCount,
		Locale:                          locale,
	}

	s.renderCockpitTemplate(w, r, "vector_borne_rickettsial_triage.html", "Vector-Borne & Rickettsial Triage", data)
}

func (s *Server) handleGetVectorBorneRickettsialOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureVectorBorneService()
	overview := svc.CalculateOverview()

	writeCockpitJSON(w, http.StatusOK, overview)
}

func (s *Server) handleListVectorBorneRickettsialOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureVectorBorneService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	syndromeFilter := strings.TrimSpace(r.URL.Query().Get("syndrome"))

	ops := svc.ListOperations(statusFilter)
	if syndromeFilter != "" {
		filtered := make([]domain.VectorBorneOperation, 0, len(ops))
		for _, op := range ops {
			if strings.EqualFold(string(op.Syndrome), syndromeFilter) {
				filtered = append(filtered, op)
			}
		}
		ops = filtered
	}

	overview := svc.CalculateOverview()

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"operations": ops,
		"ops":        ops,
		"overview":   overview,
		"metrics":    overview,
	})
}

func (s *Server) handleGetVectorBorneRickettsialOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureVectorBorneService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateVectorBorneRickettsialOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createVectorBorneOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	svc := s.ensureVectorBorneService()
	caseID := strings.TrimSpace(payload.CaseID)
	if caseID == "" {
		caseID = fmt.Sprintf("CASE-VBR-%d", 100+len(svc.ListOperations(""))+1)
	}

	created, err := svc.CreateOperation(
		caseID,
		payload.PetName,
		payload.Species,
		payload.Breed,
		payload.WeightKg,
		payload.Syndrome,
		payload.Assay,
		payload.AntimicrobialProtocol,
		payload.PlateletCountKUL,
		payload.PCVPercent,
		payload.UrinePCRatio,
		payload.SerumAlbuminGDL,
		payload.DoxycyclineMgKgDay,
		payload.AtovaquoneMgKgTID,
		payload.ImidocarbMgKgIM,
		payload.EsophagealWaterChaseVerified,
		payload.AtropinePremedVerified,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastVectorBorneRickettsialEvent("vector_borne_rickettsial_updated", map[string]any{
		"type":      "vector_borne_rickettsial_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageVectorBorneRickettsialAntimicrobial(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureVectorBorneService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageVectorBorneAntimicrobialPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && err != io.EOF {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	protocol := payload.Protocol
	if protocol == "" {
		protocol = payload.AntimicrobialProtocol
	}
	if protocol == "" {
		protocol = existing.AntimicrobialProtocol
	}

	doxy := existing.DoxycyclineMgKgDay
	if payload.DoxycyclineMgKgDay != nil {
		doxy = *payload.DoxycyclineMgKgDay
	}
	atovaquone := existing.AtovaquoneMgKgTID
	if payload.AtovaquoneMgKgTID != nil {
		atovaquone = *payload.AtovaquoneMgKgTID
	}
	imidocarb := existing.ImidocarbMgKgIM
	if payload.ImidocarbMgKgIM != nil {
		imidocarb = *payload.ImidocarbMgKgIM
	}
	waterChase := existing.EsophagealWaterChaseVerified
	if payload.EsophagealWaterChaseVerified != nil {
		waterChase = *payload.EsophagealWaterChaseVerified
	}
	atropinePremed := existing.AtropinePremedVerified
	if payload.AtropinePremedVerified != nil {
		atropinePremed = *payload.AtropinePremedVerified
	}

	updated, err := svc.StageAntimicrobial(
		id,
		protocol,
		doxy, atovaquone, imidocarb,
		waterChase, atropinePremed,
		payload.FattyMealBioavailabilityNote,
		payload.LymeNephritisImmunomodulationNote,
		payload.StagedBy,
		payload.ClinicalRationale,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastVectorBorneRickettsialEvent("vector_borne_rickettsial_updated", map[string]any{
		"type":      "vector_borne_rickettsial_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordVectorBorneRickettsialHematologyCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureVectorBorneService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload recordVectorBorneHematologyCheckPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordHematologyCheck(
		id,
		payload.PlateletCountKUL,
		payload.PCVPercent,
		payload.UrinePCRatio,
		payload.SerumAlbuminGDL,
		payload.MorulaeOrPiroplasmsCleared,
		payload.RecordedBy,
		payload.ClinicalNote,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastVectorBorneRickettsialEvent("vector_borne_rickettsial_updated", map[string]any{
		"type":      "vector_borne_rickettsial_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleClearVectorBorneRickettsialOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureVectorBorneService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload clearVectorBorneOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	platelets := existing.PlateletCountKUL
	if payload.PlateletCountKUL != nil {
		platelets = *payload.PlateletCountKUL
	}
	pcv := existing.PCVPercent
	if payload.PCVPercent != nil {
		pcv = *payload.PCVPercent
	}
	upc := existing.UrinePCRatio
	if payload.UrinePCRatio != nil {
		upc = *payload.UrinePCRatio
	}
	pcrNeg := true
	if payload.PCRNegativeVerified != nil {
		pcrNeg = *payload.PCRNegativeVerified
	}
	normothermicEating := true
	if payload.NormothermicEatingVerified != nil {
		normothermicEating = *payload.NormothermicEatingVerified
	}

	clearedBy := firstNonEmpty(payload.ClearedBy, payload.DischargedBy)
	summary := firstNonEmpty(payload.ClearanceSummary, payload.DischargeSummary, payload.Summary)

	updated, err := svc.EvaluateClearance(
		id,
		platelets, pcv, upc,
		pcrNeg, normothermicEating,
		clearedBy, summary,
	)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastVectorBorneRickettsialEvent("vector_borne_rickettsial_updated", map[string]any{
		"type":      "vector_borne_rickettsial_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleVectorBorneRickettsialStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "vector_borne_rickettsial")
}

func (s *Server) broadcastVectorBorneRickettsialEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("vector_borne_rickettsial", "vector-borne-rickettsial-hub", eventType, payload, now)
}
