package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/imhathrombosis"
)

// IMHAThrombosisPageData represents template data passed to templates/imha_thrombosis_triage.html.
type IMHAThrombosisPageData struct {
	Operations             []domain.IMHAThrombosisOperation
	ActiveOperation        *domain.IMHAThrombosisOperation
	TotalOperations        int
	CriticalHemolyticCount int
	StagedRegimenCount     int
	ClearedStableCount     int
	PTEAlertCount          int
	Locale                 string
}

type createIMHAOperationPayload struct {
	IncidentID                      string                     `json:"incident_id"`
	PetName                         string                     `json:"pet_name"`
	PatientProfile                  *domain.IMHAPatientProfile `json:"patient_profile"`
	Species                         string                     `json:"species"`
	Breed                           string                     `json:"breed"`
	WeightKg                        float64                    `json:"weight_kg"`
	PackedCellVolumePercent         float64                    `json:"packed_cell_volume_percent"`
	TotalBilirubinMgDl              float64                    `json:"total_bilirubin_mg_dl"`
	PlateletCountKUl                float64                    `json:"platelet_count_k_ul"`
	SpherocytesPerHPF               int                        `json:"spherocytes_per_hpf"`
	SalineAgglutination4To1Positive bool                       `json:"saline_agglutination_4_to_1_positive"`
	IntravascularHemoglobinuria     bool                       `json:"intravascular_hemoglobinuria"`
}

type stageIMHARegimenPayload struct {
	PatientProfile                  *domain.IMHAPatientProfile `json:"patient_profile"`
	Species                         string                     `json:"species"`
	Breed                           string                     `json:"breed"`
	WeightKg                        float64                    `json:"weight_kg"`
	PackedCellVolumePercent         float64                    `json:"packed_cell_volume_percent"`
	TotalBilirubinMgDl              *float64                   `json:"total_bilirubin_mg_dl"`
	PlateletCountKUl                *float64                   `json:"platelet_count_k_ul"`
	SpherocytesPerHPF               *int                       `json:"spherocytes_per_hpf"`
	SalineAgglutination4To1Positive *bool                      `json:"saline_agglutination_4_to_1_positive"`
	IntravascularHemoglobinuria     *bool                      `json:"intravascular_hemoglobinuria"`
}

func (s *Server) ensureIMHAService() *imhathrombosis.IMHAService {
	if s.imhaService == nil {
		s.imhaService = imhathrombosis.NewIMHAService()
	}
	return s.imhaService
}

func (s *Server) registerIMHAThrombosisRoutes() {
	s.mux.HandleFunc("GET /imha-thrombosis-triage", s.handleIMHAThrombosisTriagePage)

	s.registerDualAPIRoute("GET", "imha-thrombosis/operations", s.handleListIMHAThrombosisOperations)
	s.registerDualAPIRoute("POST", "imha-thrombosis/operations", s.handleCreateIMHAThrombosisOperation)
	s.registerDualAPIRoute("GET", "imha-thrombosis/operations/{id}", s.handleGetIMHAThrombosisOperation)
	s.registerDualAPIRoute("POST", "imha-thrombosis/operations/{id}/stage-regimen", s.handleStageIMHAThrombosisRegimen)
	s.registerDualAPIRoute("POST", "imha-thrombosis/operations/{id}/hemolysis-check", s.handleRecordIMHAThrombosisHemolysisCheck)
	s.registerDualAPIRoute("POST", "imha-thrombosis/operations/{id}/discharge-clearance", s.handleIssueIMHAThrombosisDischargeClearance)
	s.registerDualAPIRoute("GET", "imha-thrombosis/stream", s.handleIMHAThrombosisStream)
}

func (s *Server) handleIMHAThrombosisTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureIMHAService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.IMHAThrombosisOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := IMHAThrombosisPageData{
		Operations:             ops,
		ActiveOperation:        activeOp,
		TotalOperations:        metrics.TotalOperations,
		CriticalHemolyticCount: metrics.CriticalHemolyticCount,
		StagedRegimenCount:     metrics.StagedRegimenCount,
		ClearedStableCount:     metrics.ClearedStableCount,
		PTEAlertCount:          metrics.PTEAlertCount,
		Locale:                 locale,
	}

	s.renderCockpitTemplate(w, r, "imha_thrombosis_triage.html", "IMHA Thrombosis Triage", data)
}

func (s *Server) handleListIMHAThrombosisOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureIMHAService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.IMHAThrombosisOperation, 0, len(ops))
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

func (s *Server) handleGetIMHAThrombosisOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureIMHAService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateIMHAThrombosisOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createIMHAOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.IMHAPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.IMHAPatientProfile{
			Species:                         payload.Species,
			Breed:                           payload.Breed,
			WeightKg:                        payload.WeightKg,
			PackedCellVolumePercent:         payload.PackedCellVolumePercent,
			TotalBilirubinMgDl:              payload.TotalBilirubinMgDl,
			PlateletCountKUl:                payload.PlateletCountKUl,
			SpherocytesPerHPF:               payload.SpherocytesPerHPF,
			SalineAgglutination4To1Positive: payload.SalineAgglutination4To1Positive,
			IntravascularHemoglobinuria:     payload.IntravascularHemoglobinuria,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-IM-%d", 9700+len(s.ensureIMHAService().ListOperations(""))+1)
	}

	svc := s.ensureIMHAService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastIMHAThrombosisEvent("imha_thrombosis_updated", map[string]any{
		"type":      "imha_thrombosis_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageIMHAThrombosisRegimen(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureIMHAService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageIMHARegimenPayload
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
		if payload.PackedCellVolumePercent != 0 {
			profile.PackedCellVolumePercent = payload.PackedCellVolumePercent
		}
		if payload.TotalBilirubinMgDl != nil {
			profile.TotalBilirubinMgDl = *payload.TotalBilirubinMgDl
		}
		if payload.PlateletCountKUl != nil {
			profile.PlateletCountKUl = *payload.PlateletCountKUl
		}
		if payload.SpherocytesPerHPF != nil {
			profile.SpherocytesPerHPF = *payload.SpherocytesPerHPF
		}
		if payload.SalineAgglutination4To1Positive != nil {
			profile.SalineAgglutination4To1Positive = *payload.SalineAgglutination4To1Positive
		}
		if payload.IntravascularHemoglobinuria != nil {
			profile.IntravascularHemoglobinuria = *payload.IntravascularHemoglobinuria
		}
	}

	updated, err := svc.StageImmunosuppressiveRegimen(id, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastIMHAThrombosisEvent("imha_thrombosis_updated", map[string]any{
		"type":      "imha_thrombosis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordIMHAThrombosisHemolysisCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureIMHAService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.IMHAHemolysisSerialCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordHemolysisCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastIMHAThrombosisEvent("imha_thrombosis_updated", map[string]any{
		"type":      "imha_thrombosis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueIMHAThrombosisDischargeClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureIMHAService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.IMHADischargeClearance
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
	s.broadcastIMHAThrombosisEvent("imha_thrombosis_updated", map[string]any{
		"type":      "imha_thrombosis_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIMHAThrombosisStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "imha_thrombosis")
}

func (s *Server) broadcastIMHAThrombosisEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("imha_thrombosis", "imha-thrombosis-hub", eventType, payload, now)
}
