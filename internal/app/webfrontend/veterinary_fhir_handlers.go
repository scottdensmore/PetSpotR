package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/veterinary"
)

var (
	lastFHIRNano atomic.Int64
	clinicSeedMu sync.Mutex
)

func nextFHIRNano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastFHIRNano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastFHIRNano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

type createClinicalHandoffRequest struct {
	AssessmentID  string `json:"assessmentId"`
	PetID         string `json:"petId"`
	PassportID    string `json:"passportId"`
	ClinicID      string `json:"clinicId"`
	TransportUnit string `json:"transportUnit"`
	ETAMinutes    int    `json:"etaMinutes"`
}

type inboundPIMSWebhookRequest struct {
	Status              domain.HandoffStatus `json:"status"`
	VerifiedMicrochipID string               `json:"verifiedMicrochipId"`
	AttendingVet        string               `json:"attendingVet"`
	DischargeSummary    string               `json:"dischargeSummary"`
}

type clinicalHandoffEnvelope struct {
	domain.ClinicalHandoff
	Handoff domain.ClinicalHandoff `json:"handoff"`
}

type veterinaryClinicEnvelope struct {
	domain.VeterinaryClinic
	Clinic domain.VeterinaryClinic `json:"clinic"`
}

// VeterinaryFHIRPageData represents the template data passed to templates/veterinary_fhir.html.
type VeterinaryFHIRPageData struct {
	Clinics     []domain.VeterinaryClinic
	Assessments []domain.TriageAssessment
	Handoffs    []domain.ClinicalHandoff
	Locale      string
}

func defaultVeterinaryClinics(now time.Time) []domain.VeterinaryClinic {
	return []domain.VeterinaryClinic{
		{
			ClinicID:          "clinic-er-seattle-1",
			Name:              "Northwest Veterinary Critical Care & ER",
			PIMS:              domain.PIMSCornerstone,
			Emergency24Hr:     true,
			Phone:             "(206) 555-0191",
			Address:           "1400 Elliott Ave W, Seattle, WA",
			Latitude:          47.6318,
			Longitude:         -122.3752,
			AcceptingPatients: true,
			CreatedAt:         now,
		},
		{
			ClinicID:          "clinic-er-bellevue-2",
			Name:              "Eastside Specialty Animal Hospital",
			PIMS:              domain.PIMSEzyVet,
			Emergency24Hr:     true,
			Phone:             "(425) 555-0144",
			Address:           "11800 NE 8th St, Bellevue, WA",
			Latitude:          47.6174,
			Longitude:         -122.1825,
			AcceptingPatients: true,
			CreatedAt:         now.Add(time.Millisecond),
		},
		{
			ClinicID:          "clinic-shelter-3",
			Name:              "King County Regional Shelter Medical Intake",
			PIMS:              domain.PIMSShelterluv,
			Emergency24Hr:     true,
			Phone:             "(206) 555-0177",
			Address:           "21615 64th Ave S, Kent, WA",
			Latitude:          47.4082,
			Longitude:         -122.2541,
			AcceptingPatients: true,
			CreatedAt:         now.Add(2 * time.Millisecond),
		},
	}
}

func (s *Server) ensureDefaultVeterinaryClinics(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	clinicSeedMu.Lock()
	defer clinicSeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionVeterinaryClinics)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC()
	for _, clinic := range defaultVeterinaryClinics(now) {
		if data, err := json.Marshal(clinic); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionVeterinaryClinics, clinic.ClinicID, data)
		}
	}
}

func (s *Server) loadVeterinaryClinics(ctx context.Context) []domain.VeterinaryClinic {
	clinics := make([]domain.VeterinaryClinic, 0)
	if s.stateStore == nil {
		return clinics
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionVeterinaryClinics)
	if err != nil {
		return clinics
	}
	for _, b := range raw {
		var c domain.VeterinaryClinic
		if err := json.Unmarshal(b, &c); err == nil {
			clinics = append(clinics, c)
		}
	}
	sort.Slice(clinics, func(i, j int) bool {
		if !clinics[i].CreatedAt.Equal(clinics[j].CreatedAt) {
			return clinics[i].CreatedAt.Before(clinics[j].CreatedAt)
		}
		return clinics[i].ClinicID < clinics[j].ClinicID
	})
	return clinics
}

func (s *Server) loadClinicalHandoffs(ctx context.Context, petIDFilter, clinicIDFilter, statusFilter string) []domain.ClinicalHandoff {
	handoffs := make([]domain.ClinicalHandoff, 0)
	if s.stateStore == nil {
		return handoffs
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionClinicalHandoffs)
	if err != nil {
		return handoffs
	}
	for _, b := range raw {
		var h domain.ClinicalHandoff
		if err := json.Unmarshal(b, &h); err == nil {
			if petIDFilter != "" && !strings.EqualFold(h.PetID, petIDFilter) {
				continue
			}
			if clinicIDFilter != "" && !strings.EqualFold(h.ClinicID, clinicIDFilter) {
				continue
			}
			if statusFilter != "" && !strings.EqualFold(string(h.Status), statusFilter) {
				continue
			}
			handoffs = append(handoffs, h)
		}
	}
	sort.Slice(handoffs, func(i, j int) bool {
		if !handoffs[i].DispatchedAt.Equal(handoffs[j].DispatchedAt) {
			return handoffs[i].DispatchedAt.After(handoffs[j].DispatchedAt)
		}
		return handoffs[i].HandoffID > handoffs[j].HandoffID
	})
	return handoffs
}

func (s *Server) handleVeterinaryFHIRPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultVeterinaryClinics(r.Context())
	clinics := s.loadVeterinaryClinics(r.Context())
	handoffs := s.loadClinicalHandoffs(r.Context(), "", "", "")

	assessments := make([]domain.TriageAssessment, 0)
	if s.stateStore != nil {
		if rawAssessments, err := s.stateStore.ListState(r.Context(), store.CollectionTriageAssessments); err == nil {
			for _, b := range rawAssessments {
				var a domain.TriageAssessment
				if err := json.Unmarshal(b, &a); err == nil {
					assessments = append(assessments, a)
				}
			}
			sort.Slice(assessments, func(i, j int) bool {
				return assessments[i].AssessedAt.After(assessments[j].AssessedAt)
			})
		}
	}

	tmpl, err := template.New("veterinary_fhir.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/veterinary_fhir.html")
	if err != nil {
		http.Error(w, "Failed to load veterinary FHIR template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := VeterinaryFHIRPageData{
		Clinics:     clinics,
		Assessments: assessments,
		Handoffs:    handoffs,
		Locale:      LocaleFromContext(r.Context()),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=()")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_ = tmpl.Execute(w, data)
	}
}

func (s *Server) handleListVeterinaryClinics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	s.ensureDefaultVeterinaryClinics(r.Context())
	clinics := s.loadVeterinaryClinics(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"clinics": clinics,
	})
}

func (s *Server) handleCreateVeterinaryClinic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var clinic domain.VeterinaryClinic
	if err := json.NewDecoder(r.Body).Decode(&clinic); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	clinic.Name = strings.TrimSpace(clinic.Name)
	clinic.Phone = strings.TrimSpace(clinic.Phone)
	clinic.Address = strings.TrimSpace(clinic.Address)
	clinic.PIMS = domain.PIMSProvider(strings.ToUpper(strings.TrimSpace(string(clinic.PIMS))))
	if clinic.PIMS == "" {
		clinic.PIMS = domain.PIMSCornerstone
	}
	seqNano := nextFHIRNano()
	clinic.ClinicID = fmt.Sprintf("clinic-%d", seqNano)
	clinic.AcceptingPatients = true
	clinic.CreatedAt = time.Unix(0, seqNano).UTC()

	if err := clinic.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	clinicBytes, err := json.Marshal(clinic)
	if err != nil {
		http.Error(w, "Failed to encode clinic", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.CollectionVeterinaryClinics, clinic.ClinicID, clinicBytes); err != nil {
		http.Error(w, "Failed to save clinic", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(veterinaryClinicEnvelope{
		VeterinaryClinic: clinic,
		Clinic:           clinic,
	})
}

func (s *Server) handleCreateClinicalHandoff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createClinicalHandoffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	s.ensureDefaultVeterinaryClinics(r.Context())

	clinicID := strings.TrimSpace(req.ClinicID)
	if clinicID == "" {
		http.Error(w, "clinicId is required", http.StatusBadRequest)
		return
	}
	clinicBytes, err := s.stateStore.GetState(r.Context(), store.CollectionVeterinaryClinics, clinicID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			http.Error(w, "Veterinary clinic not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to load clinic", http.StatusInternalServerError)
		return
	}
	var clinic domain.VeterinaryClinic
	if err := json.Unmarshal(clinicBytes, &clinic); err != nil {
		http.Error(w, "Corrupt clinic record", http.StatusInternalServerError)
		return
	}

	// Resolve TriageAssessment
	var assessment domain.TriageAssessment
	foundAssessment := false
	assessmentID := strings.TrimSpace(req.AssessmentID)
	petID := strings.TrimSpace(req.PetID)

	if assessmentID != "" {
		if aBytes, err := s.stateStore.GetState(r.Context(), store.CollectionTriageAssessments, assessmentID); err == nil {
			if err := json.Unmarshal(aBytes, &assessment); err == nil {
				foundAssessment = true
			}
		}
	} else if petID != "" {
		if rawAssessments, err := s.stateStore.ListState(r.Context(), store.CollectionTriageAssessments); err == nil {
			var matches []domain.TriageAssessment
			for _, b := range rawAssessments {
				var a domain.TriageAssessment
				if err := json.Unmarshal(b, &a); err == nil && a.PetID == petID {
					matches = append(matches, a)
				}
			}
			if len(matches) > 0 {
				sort.Slice(matches, func(i, j int) bool {
					return matches[i].AssessedAt.After(matches[j].AssessedAt)
				})
				assessment = matches[0]
				foundAssessment = true
			}
		}
	}

	if !foundAssessment {
		http.Error(w, "Triage assessment not found", http.StatusNotFound)
		return
	}

	// Resolve optional *domain.VeterinaryPassport
	var passport *domain.VeterinaryPassport
	passportID := strings.TrimSpace(req.PassportID)
	if passportID != "" {
		if pBytes, err := s.stateStore.GetState(r.Context(), store.CollectionVeterinaryPassports, passportID); err == nil {
			var p domain.VeterinaryPassport
			if err := json.Unmarshal(pBytes, &p); err == nil {
				passport = &p
			}
		}
	}
	if passport == nil {
		if rawPassports, err := s.stateStore.ListState(r.Context(), store.CollectionVeterinaryPassports); err == nil {
			var candidates []domain.VeterinaryPassport
			for _, b := range rawPassports {
				var p domain.VeterinaryPassport
				if err := json.Unmarshal(b, &p); err == nil {
					if (passportID != "" && p.PassportID == passportID) || p.PetID == assessment.PetID {
						candidates = append(candidates, p)
					}
				}
			}
			if len(candidates) > 0 {
				sort.Slice(candidates, func(i, j int) bool {
					return candidates[i].IssuedAt.After(candidates[j].IssuedAt)
				})
				p := candidates[0]
				passport = &p
			}
		}
	}

	transportUnit := strings.TrimSpace(req.TransportUnit)
	if transportUnit == "" {
		transportUnit = "SAR MedEvac Unit #1"
	}
	etaMinutes := req.ETAMinutes
	if etaMinutes <= 0 {
		etaMinutes = 15
	}

	seqNano := nextFHIRNano()
	handoffID := fmt.Sprintf("handoff-%d", seqNano)
	now := time.Unix(0, seqNano).UTC()

	fhirBundle, err := veterinary.BuildFHIRVeterinaryBundle(assessment, passport, clinic)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to build FHIR bundle: %v", err), http.StatusInternalServerError)
		return
	}
	pimsRecord := veterinary.BuildPIMSIntakeRecord(handoffID, assessment, passport, clinic)

	resolvedPassportID := passportID
	if passport != nil && passport.PassportID != "" {
		resolvedPassportID = passport.PassportID
	}

	handoff := domain.ClinicalHandoff{
		HandoffID:      handoffID,
		AssessmentID:   assessment.AssessmentID,
		PetID:          assessment.PetID,
		PassportID:     resolvedPassportID,
		ClinicID:       clinic.ClinicID,
		ClinicName:     clinic.Name,
		PIMS:           clinic.PIMS,
		TriageCategory: assessment.Category,
		TransportUnit:  transportUnit,
		ETAMinutes:     etaMinutes,
		Status:         domain.HandoffStatusEnRoute,
		FHIRBundle:     fhirBundle,
		PIMSRecord:     pimsRecord,
		DispatchedAt:   now,
		UpdatedAt:      now,
	}

	handoffBytes, err := json.Marshal(handoff)
	if err != nil {
		http.Error(w, "Failed to encode clinical handoff", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionClinicalHandoffs, handoff.HandoffID, handoffBytes); err != nil {
		http.Error(w, "Failed to persist clinical handoff", http.StatusInternalServerError)
		return
	}

	s.broadcastClinicalHandoffEvent("clinical_handoff_dispatched", handoff, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(clinicalHandoffEnvelope{
		ClinicalHandoff: handoff,
		Handoff:         handoff,
	})
}

func (s *Server) handleListClinicalHandoffs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	petIDFilter := strings.TrimSpace(q.Get("petId"))
	clinicIDFilter := strings.TrimSpace(q.Get("clinicId"))
	statusFilter := strings.TrimSpace(q.Get("status"))

	handoffs := s.loadClinicalHandoffs(r.Context(), petIDFilter, clinicIDFilter, statusFilter)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"handoffs": handoffs,
	})
}

func (s *Server) handleExportClinicalHandoffFHIR(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	handoffID := strings.TrimSpace(r.PathValue("id"))
	if handoffID == "" {
		http.Error(w, "handoff id is required", http.StatusBadRequest)
		return
	}

	data, err := s.stateStore.GetState(r.Context(), store.CollectionClinicalHandoffs, handoffID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			http.Error(w, "Clinical handoff not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to load clinical handoff", http.StatusInternalServerError)
		return
	}

	var handoff domain.ClinicalHandoff
	if err := json.Unmarshal(data, &handoff); err != nil {
		http.Error(w, "Corrupt clinical handoff record", http.StatusInternalServerError)
		return
	}

	if strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("format")), "csv") {
		csvContent, err := veterinary.FormatPIMSRecordCSV([]domain.PIMSIntakeRecord{handoff.PIMSRecord})
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to format PIMS CSV: %v", err), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"pims-intake-%s.csv\"", handoff.HandoffID))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(csvContent))
		return
	}

	if err := veterinary.ValidateFHIRVeterinaryBundle(handoff.FHIRBundle); err != nil {
		http.Error(w, fmt.Sprintf("FHIR bundle validation failed: %v", err), http.StatusUnprocessableEntity)
		return
	}

	w.Header().Set("Content-Type", "application/fhir+json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(handoff.FHIRBundle)
}

func (s *Server) handleInboundPIMSWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	handoffID := strings.TrimSpace(r.PathValue("id"))
	if handoffID == "" {
		http.Error(w, "handoff id is required", http.StatusBadRequest)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req inboundPIMSWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	req.Status = domain.HandoffStatus(strings.ToUpper(strings.TrimSpace(string(req.Status))))
	if !req.Status.IsValid() {
		http.Error(w, "Invalid handoff status", http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	var updatedHandoff domain.ClinicalHandoff

	err := s.stateStore.UpdateState(r.Context(), store.CollectionClinicalHandoffs, handoffID, func(current []byte) ([]byte, error) {
		var h domain.ClinicalHandoff
		if err := json.Unmarshal(current, &h); err != nil {
			return nil, fmt.Errorf("corrupt clinical handoff: %w", err)
		}

		h.Status = req.Status
		if v := strings.TrimSpace(req.VerifiedMicrochipID); v != "" {
			h.VerifiedMicrochipID = v
			h.PIMSRecord.MicrochipID = v
		}
		if v := strings.TrimSpace(req.AttendingVet); v != "" {
			h.AttendingVet = v
			h.PIMSRecord.AttendingVet = v
		}
		if v := strings.TrimSpace(req.DischargeSummary); v != "" {
			h.DischargeSummary = v
			h.PIMSRecord.DischargeSummary = v
		}
		h.ReadyForReunion = (req.Status == domain.HandoffStatusClearedForReunification)
		h.UpdatedAt = now

		updatedHandoff = h
		return json.Marshal(h)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			http.Error(w, "Clinical handoff not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to update clinical handoff: %v", err), http.StatusInternalServerError)
		return
	}

	s.broadcastClinicalHandoffEvent("clinical_handoff_updated", updatedHandoff, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(clinicalHandoffEnvelope{
		ClinicalHandoff: updatedHandoff,
		Handoff:         updatedHandoff,
	})
}

func (s *Server) broadcastClinicalHandoffEvent(eventType string, handoff domain.ClinicalHandoff, now time.Time) {
	payload := map[string]any{
		"type":                eventType,
		"handoffId":           handoff.HandoffID,
		"assessmentId":        handoff.AssessmentID,
		"petId":               handoff.PetID,
		"clinicId":            handoff.ClinicID,
		"clinicName":          handoff.ClinicName,
		"pims":                handoff.PIMS,
		"triageCategory":      handoff.TriageCategory,
		"transportUnit":       handoff.TransportUnit,
		"etaMinutes":          handoff.ETAMinutes,
		"status":              handoff.Status,
		"verifiedMicrochipId": handoff.VerifiedMicrochipID,
		"attendingVet":        handoff.AttendingVet,
		"dischargeSummary":    handoff.DischargeSummary,
		"readyForReunion":     handoff.ReadyForReunion,
		"bundleDigestHex":     handoff.FHIRBundle.BundleDigestHex,
		"handoff":             handoff,
		"updatedAt":           handoff.UpdatedAt,
	}

	seqNano := nextFHIRNano()
	if s.reunionHub != nil {
		triageEvent := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_%s_%s_%d", eventType, handoff.HandoffID, seqNano),
			Type:      domain.ReunionEventType(eventType),
			MatchID:   "triage",
			Timestamp: now,
			Payload:   payload,
		}
		s.reunionHub.Broadcast(triageEvent)

		if handoff.PetID != "" && handoff.PetID != "triage" {
			petEvent := triageEvent
			petEvent.MatchID = handoff.PetID
			s.reunionHub.Broadcast(petEvent)
		}
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(payload)
		senderNodeID := handoff.ClinicID
		if senderNodeID == "" {
			senderNodeID = "vet-fhir-gateway"
		}
		triageEnv := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType(eventType),
			SearchPartyID: "triage",
			SenderNodeID:  senderNodeID,
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(triageEnv)

		if handoff.PetID != "" && handoff.PetID != "triage" {
			petEnv := triageEnv
			petEnv.SearchPartyID = handoff.PetID
			s.signalingHub.Broadcast(petEnv)
		}
	}
}
