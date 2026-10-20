package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/anaphylaxishalo"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

// AnaphylaxisHaloPageData represents template data passed to templates/anaphylaxis_halo_triage.html.
type AnaphylaxisHaloPageData struct {
	Operations                    []domain.AnaphylaxisHaloOperation
	ActiveOperation               *domain.AnaphylaxisHaloOperation
	TotalOperations               int
	CriticalShockHaloCount        int
	EpinephrineFFPStagedCount     int
	Biphasic24HrClearedCount      int
	HepaticCoagulopathyAlertCount int
	Locale                        string
}

type createAnaphylaxisOperationPayload struct {
	IncidentID                 string                            `json:"incident_id"`
	PetName                    string                            `json:"pet_name"`
	PatientProfile             *domain.AnaphylaxisPatientProfile `json:"patient_profile"`
	Species                    string                            `json:"species"`
	Breed                      string                            `json:"breed"`
	WeightKg                   float64                           `json:"weight_kg"`
	GallbladderWallThicknessMm float64                           `json:"gallbladder_wall_thickness_mm"`
	GallbladderHaloSignPresent bool                              `json:"gallbladder_halo_sign_present"`
	MeanArterialPressureMmHg   int                               `json:"mean_arterial_pressure_mmhg"`
	AlanineAminotransferaseUL  int                               `json:"alanine_aminotransferase_u_l"`
	PackedCellVolumePercent    float64                           `json:"packed_cell_volume_percent"`
	ProthrombinTimeSeconds     float64                           `json:"prothrombin_time_seconds"`
}

type stageAnaphylaxisPlanPayload struct {
	PatientProfile             *domain.AnaphylaxisPatientProfile `json:"patient_profile"`
	Species                    string                            `json:"species"`
	Breed                      string                            `json:"breed"`
	WeightKg                   float64                           `json:"weight_kg"`
	GallbladderWallThicknessMm float64                           `json:"gallbladder_wall_thickness_mm"`
	GallbladderHaloSignPresent *bool                             `json:"gallbladder_halo_sign_present"`
	MeanArterialPressureMmHg   int                               `json:"mean_arterial_pressure_mmhg"`
	AlanineAminotransferaseUL  int                               `json:"alanine_aminotransferase_u_l"`
	PackedCellVolumePercent    float64                           `json:"packed_cell_volume_percent"`
	ProthrombinTimeSeconds     float64                           `json:"prothrombin_time_seconds"`
}

func (s *Server) ensureAnaphylaxisService() *anaphylaxishalo.AnaphylaxisService {
	if s.anaphylaxisService == nil {
		s.anaphylaxisService = anaphylaxishalo.NewAnaphylaxisService()
	}
	return s.anaphylaxisService
}

func (s *Server) registerAnaphylaxisHaloRoutes() {
	s.mux.HandleFunc("GET /anaphylaxis-halo-triage", s.handleAnaphylaxisHaloTriagePage)

	s.registerDualAPIRoute("GET", "anaphylaxis-halo/operations", s.handleListAnaphylaxisHaloOperations)
	s.registerDualAPIRoute("POST", "anaphylaxis-halo/operations", s.handleCreateAnaphylaxisHaloOperation)
	s.registerDualAPIRoute("GET", "anaphylaxis-halo/operations/{id}", s.handleGetAnaphylaxisHaloOperation)
	s.registerDualAPIRoute("POST", "anaphylaxis-halo/operations/{id}/stage-plan", s.handleStageAnaphylaxisHaloPlan)
	s.registerDualAPIRoute("POST", "anaphylaxis-halo/operations/{id}/biphasic-check", s.handleRecordAnaphylaxisHaloBiphasicCheck)
	s.registerDualAPIRoute("POST", "anaphylaxis-halo/operations/{id}/discharge-clearance", s.handleIssueAnaphylaxisHaloDischargeClearance)
	s.registerDualAPIRoute("GET", "anaphylaxis-halo/stream", s.handleAnaphylaxisHaloStream)
}

func (s *Server) handleAnaphylaxisHaloTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAnaphylaxisService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.AnaphylaxisHaloOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := AnaphylaxisHaloPageData{
		Operations:                    ops,
		ActiveOperation:               activeOp,
		TotalOperations:               metrics.TotalOperations,
		CriticalShockHaloCount:        metrics.CriticalShockHaloCount,
		EpinephrineFFPStagedCount:     metrics.EpinephrineFFPStagedCount,
		Biphasic24HrClearedCount:      metrics.Biphasic24HrClearedCount,
		HepaticCoagulopathyAlertCount: metrics.HepaticCoagulopathyAlertCount,
		Locale:                        locale,
	}

	s.renderCockpitTemplate(w, r, "anaphylaxis_halo_triage.html", "Anaphylaxis Halo Triage", data)
}

func (s *Server) handleListAnaphylaxisHaloOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureAnaphylaxisService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.AnaphylaxisHaloOperation, 0, len(ops))
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

func (s *Server) handleGetAnaphylaxisHaloOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAnaphylaxisService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreateAnaphylaxisHaloOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createAnaphylaxisOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.AnaphylaxisPatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.AnaphylaxisPatientProfile{
			Species:                    payload.Species,
			Breed:                      payload.Breed,
			WeightKg:                   payload.WeightKg,
			GallbladderWallThicknessMm: payload.GallbladderWallThicknessMm,
			GallbladderHaloSignPresent: payload.GallbladderHaloSignPresent,
			MeanArterialPressureMmHg:   payload.MeanArterialPressureMmHg,
			AlanineAminotransferaseUL:  payload.AlanineAminotransferaseUL,
			PackedCellVolumePercent:    payload.PackedCellVolumePercent,
			ProthrombinTimeSeconds:     payload.ProthrombinTimeSeconds,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-AH-%d", 9900+len(s.ensureAnaphylaxisService().ListOperations(""))+1)
	}

	svc := s.ensureAnaphylaxisService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAnaphylaxisHaloEvent("anaphylaxis_halo_updated", map[string]any{
		"type":      "anaphylaxis_halo_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStageAnaphylaxisHaloPlan(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAnaphylaxisService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageAnaphylaxisPlanPayload
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
		if payload.GallbladderWallThicknessMm != 0 {
			profile.GallbladderWallThicknessMm = payload.GallbladderWallThicknessMm
		}
		if payload.GallbladderHaloSignPresent != nil {
			profile.GallbladderHaloSignPresent = *payload.GallbladderHaloSignPresent
		}
		if payload.MeanArterialPressureMmHg != 0 {
			profile.MeanArterialPressureMmHg = payload.MeanArterialPressureMmHg
		}
		if payload.AlanineAminotransferaseUL != 0 {
			profile.AlanineAminotransferaseUL = payload.AlanineAminotransferaseUL
		}
		if payload.PackedCellVolumePercent != 0 {
			profile.PackedCellVolumePercent = payload.PackedCellVolumePercent
		}
		if payload.ProthrombinTimeSeconds != 0 {
			profile.ProthrombinTimeSeconds = payload.ProthrombinTimeSeconds
		}
	}

	updated, err := svc.StageResuscitationPlan(id, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAnaphylaxisHaloEvent("anaphylaxis_halo_updated", map[string]any{
		"type":      "anaphylaxis_halo_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordAnaphylaxisHaloBiphasicCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAnaphylaxisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.AnaphylaxisBiphasicCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordBiphasicCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastAnaphylaxisHaloEvent("anaphylaxis_halo_updated", map[string]any{
		"type":      "anaphylaxis_halo_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssueAnaphylaxisHaloDischargeClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureAnaphylaxisService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.AnaphylaxisDischargeClearance
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
	s.broadcastAnaphylaxisHaloEvent("anaphylaxis_halo_updated", map[string]any{
		"type":      "anaphylaxis_halo_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleAnaphylaxisHaloStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "anaphylaxis_halo")
}

func (s *Server) broadcastAnaphylaxisHaloEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("anaphylaxis_halo", "anaphylaxis-halo-hub", eventType, payload, now)
}
