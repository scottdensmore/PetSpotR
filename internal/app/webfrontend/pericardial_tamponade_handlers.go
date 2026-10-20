package webfrontend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pericardialtamponade"
)

// PericardialTamponadePageData represents template data passed to templates/pericardial_tamponade_triage.html.
type PericardialTamponadePageData struct {
	Operations                         []domain.PericardialTamponadeOperation
	ActiveOperation                    *domain.PericardialTamponadeOperation
	TotalOperations                    int
	CriticalTamponadeCount             int
	RightICS45TapStagedCount           int
	DecompressedSinusClearedCount      int
	FurosemideLeftCoronaryHazardAlerts int
	Locale                             string
}

type createTamponadeOperationPayload struct {
	IncidentID                   string                          `json:"incident_id"`
	PetName                      string                          `json:"pet_name"`
	PatientProfile               *domain.TamponadePatientProfile `json:"patient_profile"`
	Species                      string                          `json:"species"`
	Breed                        string                          `json:"breed"`
	WeightKg                     float64                         `json:"weight_kg"`
	HeartRateBpm                 int                             `json:"heart_rate_bpm"`
	MeanArterialPressureMmHg     int                             `json:"mean_arterial_pressure_mmhg"`
	RightAtrialDiastolicCollapse bool                            `json:"right_atrial_diastolic_collapse"`
	ElectricalAlternansPresent   bool                            `json:"electrical_alternans_present"`
	PulsusParadoxusMmHg          int                             `json:"pulsus_paradoxus_mmhg"`
	EffusionPCVPercent           float64                         `json:"effusion_pcv_percent"`
}

type stageTamponadePlanPayload struct {
	PatientProfile               *domain.TamponadePatientProfile `json:"patient_profile"`
	Species                      string                          `json:"species"`
	Breed                        string                          `json:"breed"`
	WeightKg                     float64                         `json:"weight_kg"`
	HeartRateBpm                 int                             `json:"heart_rate_bpm"`
	MeanArterialPressureMmHg     int                             `json:"mean_arterial_pressure_mmhg"`
	RightAtrialDiastolicCollapse *bool                           `json:"right_atrial_diastolic_collapse"`
	ElectricalAlternansPresent   *bool                           `json:"electrical_alternans_present"`
	PulsusParadoxusMmHg          *int                            `json:"pulsus_paradoxus_mmhg"`
	EffusionPCVPercent           *float64                        `json:"effusion_pcv_percent"`
}

func (s *Server) ensureTamponadeService() *pericardialtamponade.TamponadeService {
	if s.tamponadeService == nil {
		s.tamponadeService = pericardialtamponade.NewTamponadeService()
	}
	return s.tamponadeService
}

func (s *Server) registerPericardialTamponadeRoutes() {
	s.mux.HandleFunc("GET /pericardial-tamponade-triage", s.handlePericardialTamponadeTriagePage)

	s.registerDualAPIRoute("GET", "pericardial-tamponade/operations", s.handleListPericardialTamponadeOperations)
	s.registerDualAPIRoute("POST", "pericardial-tamponade/operations", s.handleCreatePericardialTamponadeOperation)
	s.registerDualAPIRoute("GET", "pericardial-tamponade/operations/{id}", s.handleGetPericardialTamponadeOperation)
	s.registerDualAPIRoute("POST", "pericardial-tamponade/operations/{id}/stage-plan", s.handleStagePericardialTamponadePlan)
	s.registerDualAPIRoute("POST", "pericardial-tamponade/operations/{id}/tap-check", s.handleRecordPericardialTamponadeTapCheck)
	s.registerDualAPIRoute("POST", "pericardial-tamponade/operations/{id}/discharge-clearance", s.handleIssuePericardialTamponadeDischargeClearance)
	s.registerDualAPIRoute("GET", "pericardial-tamponade/stream", s.handlePericardialTamponadeStream)
}

func (s *Server) handlePericardialTamponadeTriagePage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTamponadeService()
	ops := svc.ListOperations("")
	metrics := svc.ComputeMetrics()

	var activeOp *domain.PericardialTamponadeOperation
	if len(ops) > 0 {
		first := ops[0]
		activeOp = &first
	}

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := PericardialTamponadePageData{
		Operations:                         ops,
		ActiveOperation:                    activeOp,
		TotalOperations:                    metrics.TotalOperations,
		CriticalTamponadeCount:             metrics.CriticalTamponadeCount,
		RightICS45TapStagedCount:           metrics.RightICS45TapStagedCount,
		DecompressedSinusClearedCount:      metrics.DecompressedSinusClearedCount,
		FurosemideLeftCoronaryHazardAlerts: metrics.FurosemideLeftCoronaryHazardAlerts,
		Locale:                             locale,
	}

	s.renderCockpitTemplate(w, r, "pericardial_tamponade_triage.html", "Pericardial Tamponade Triage", data)
}

func (s *Server) handleListPericardialTamponadeOperations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	svc := s.ensureTamponadeService()
	statusFilter := strings.TrimSpace(r.URL.Query().Get("status"))
	speciesFilter := strings.TrimSpace(r.URL.Query().Get("species"))

	ops := svc.ListOperations(statusFilter)
	if speciesFilter != "" {
		filtered := make([]domain.PericardialTamponadeOperation, 0, len(ops))
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

func (s *Server) handleGetPericardialTamponadeOperation(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureTamponadeService()
	op, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	writeCockpitJSON(w, http.StatusOK, op)
}

func (s *Server) handleCreatePericardialTamponadeOperation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload createTamponadeOperationPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	var profile domain.TamponadePatientProfile
	if payload.PatientProfile != nil {
		profile = *payload.PatientProfile
	} else {
		profile = domain.TamponadePatientProfile{
			Species:                      payload.Species,
			Breed:                        payload.Breed,
			WeightKg:                     payload.WeightKg,
			HeartRateBpm:                 payload.HeartRateBpm,
			MeanArterialPressureMmHg:     payload.MeanArterialPressureMmHg,
			RightAtrialDiastolicCollapse: payload.RightAtrialDiastolicCollapse,
			ElectricalAlternansPresent:   payload.ElectricalAlternansPresent,
			PulsusParadoxusMmHg:          payload.PulsusParadoxusMmHg,
			EffusionPCVPercent:           payload.EffusionPCVPercent,
		}
	}

	incidentID := strings.TrimSpace(payload.IncidentID)
	if incidentID == "" {
		incidentID = fmt.Sprintf("INC-PT-%d", 9920+len(s.ensureTamponadeService().ListOperations(""))+1)
	}

	svc := s.ensureTamponadeService()
	created, err := svc.CreateOperation(incidentID, payload.PetName, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPericardialTamponadeEvent("pericardial_tamponade_updated", map[string]any{
		"type":      "pericardial_tamponade_updated",
		"operation": created,
	}, now)

	writeCockpitJSON(w, http.StatusCreated, created)
}

func (s *Server) handleStagePericardialTamponadePlan(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureTamponadeService()
	existing, err := svc.GetOperation(id)
	if err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var payload stageTamponadePlanPayload
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
		if payload.HeartRateBpm != 0 {
			profile.HeartRateBpm = payload.HeartRateBpm
		}
		if payload.MeanArterialPressureMmHg != 0 {
			profile.MeanArterialPressureMmHg = payload.MeanArterialPressureMmHg
		}
		if payload.RightAtrialDiastolicCollapse != nil {
			profile.RightAtrialDiastolicCollapse = *payload.RightAtrialDiastolicCollapse
		}
		if payload.ElectricalAlternansPresent != nil {
			profile.ElectricalAlternansPresent = *payload.ElectricalAlternansPresent
		}
		if payload.PulsusParadoxusMmHg != nil {
			profile.PulsusParadoxusMmHg = *payload.PulsusParadoxusMmHg
		}
		if payload.EffusionPCVPercent != nil {
			profile.EffusionPCVPercent = *payload.EffusionPCVPercent
		}
	}

	updated, err := svc.StagePericardiocentesisPlan(id, profile)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPericardialTamponadeEvent("pericardial_tamponade_updated", map[string]any{
		"type":      "pericardial_tamponade_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleRecordPericardialTamponadeTapCheck(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureTamponadeService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var check domain.PericardiocentesisTapCheck
	if err := json.NewDecoder(r.Body).Decode(&check); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	updated, err := svc.RecordTapCheck(id, check)
	if err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Unix(0, nextCockpitNano()).UTC()
	s.broadcastPericardialTamponadeEvent("pericardial_tamponade_updated", map[string]any{
		"type":      "pericardial_tamponade_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handleIssuePericardialTamponadeDischargeClearance(w http.ResponseWriter, r *http.Request) {
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

	svc := s.ensureTamponadeService()
	if _, err := svc.GetOperation(id); err != nil {
		writeCockpitJSONError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clearance domain.TamponadeDischargeClearance
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
	s.broadcastPericardialTamponadeEvent("pericardial_tamponade_updated", map[string]any{
		"type":      "pericardial_tamponade_updated",
		"operation": updated,
	}, now)

	writeCockpitJSON(w, http.StatusOK, updated)
}

func (s *Server) handlePericardialTamponadeStream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "pericardial_tamponade")
}

func (s *Server) broadcastPericardialTamponadeEvent(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("pericardial_tamponade", "pericardial-tamponade-hub", eventType, payload, now)
}
