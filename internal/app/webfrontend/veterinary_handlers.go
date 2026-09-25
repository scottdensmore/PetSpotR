package webfrontend

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
	"github.com/scottdensmore/petspotr/pkg/veterinary"
)

var (
	masterPrivKey *ecdsa.PrivateKey
	masterPubKey  *ecdsa.PublicKey
	masterKeyErr  error
	keyOnce       sync.Once
)

func getMasterKeys() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	keyOnce.Do(func() {
		masterPrivKey, masterPubKey, masterKeyErr = veterinary.GeneratePassportKey()
	})
	return masterPrivKey, masterPubKey, masterKeyErr
}

func (s *Server) handleCreateVeterinaryPassport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PetID             string                     `json:"petId"`
		PetName           string                     `json:"petName"`
		Species           string                     `json:"species"`
		Breed             string                     `json:"breed"`
		MicrochipID       string                     `json:"microchipId"`
		RabiesTagID       string                     `json:"rabiesTagId"`
		BloodType         string                     `json:"bloodType"`
		WeightKg          float64                    `json:"weightKg"`
		Vaccinations      []domain.VaccinationRecord `json:"vaccinations"`
		Allergies         []domain.ClinicalAllergy   `json:"allergies"`
		ChronicConditions []domain.ChronicCondition  `json:"chronicConditions"`
		EmergencyContact  string                     `json:"emergencyContact"`
		PrimaryClinic     string                     `json:"primaryClinic"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	petID := strings.TrimSpace(req.PetID)
	petName := strings.TrimSpace(req.PetName)
	if petID == "" || petName == "" {
		http.Error(w, "petId and petName are required", http.StatusBadRequest)
		return
	}

	priv, _, err := getMasterKeys()
	if err != nil {
		http.Error(w, "Failed to initialize crypto keys", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	passport := domain.VeterinaryPassport{
		PassportID:        fmt.Sprintf("vp-%d", now.UnixNano()),
		PetID:             petID,
		PetName:           petName,
		Species:           req.Species,
		Breed:             req.Breed,
		MicrochipID:       req.MicrochipID,
		RabiesTagID:       req.RabiesTagID,
		BloodType:         req.BloodType,
		WeightKg:          req.WeightKg,
		Vaccinations:      req.Vaccinations,
		Allergies:         req.Allergies,
		ChronicConditions: req.ChronicConditions,
		EmergencyContact:  req.EmergencyContact,
		PrimaryClinic:     req.PrimaryClinic,
		IssuedAt:          now,
		ExpiresAt:         now.AddDate(1, 0, 0),
	}

	qrPayload, err := veterinary.EncodeOfflinePassportPayload(&passport, priv)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode passport: %v", err), http.StatusInternalServerError)
		return
	}

	qrDataURI, err := veterinary.GeneratePassportQRCode(qrPayload)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate QR code: %v", err), http.StatusInternalServerError)
		return
	}

	if s.stateStore != nil {
		passBytes, err := json.Marshal(passport)
		if err != nil {
			http.Error(w, "Failed to serialize passport", http.StatusInternalServerError)
			return
		}
		if err := s.stateStore.SaveState(r.Context(), store.CollectionVeterinaryPassports, passport.PassportID, passBytes); err != nil {
			http.Error(w, "Failed to save passport", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"passportId": passport.PassportID,
		"passport":   passport,
		"qrPayload":  qrPayload,
		"qrDataUri":  qrDataURI,
		"verified":   true,
	})
}

func (s *Server) handleVerifyVeterinaryPassport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		QRPayload string `json:"qrPayload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.QRPayload) == "" {
		http.Error(w, "qrPayload is required", http.StatusBadRequest)
		return
	}

	_, pub, err := getMasterKeys()
	if err != nil {
		http.Error(w, "Crypto error", http.StatusInternalServerError)
		return
	}

	passport, valid, err := veterinary.DecodeOfflinePassportPayload(req.QRPayload, pub)
	if err != nil || !valid {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"verified": false,
			"error":    "Signature invalid or payload corrupt",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"verified": true,
		"passport": passport,
	})
}

func (s *Server) handleGetVeterinaryPassport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, "Passport ID is required", http.StatusBadRequest)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	data, err := s.stateStore.GetState(r.Context(), store.CollectionVeterinaryPassports, id)
	if err == nil {
		var passport domain.VeterinaryPassport
		if err := json.Unmarshal(data, &passport); err == nil {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(passport)
			return
		}
	}

	// Fallback lookup by PetID
	rawPassports, listErr := s.stateStore.ListState(r.Context(), store.CollectionVeterinaryPassports)
	if listErr == nil {
		for _, b := range rawPassports {
			var p domain.VeterinaryPassport
			if err := json.Unmarshal(b, &p); err == nil {
				if p.PetID == id || p.PassportID == id {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(p)
					return
				}
			}
		}
	}

	http.Error(w, "Passport not found", http.StatusNotFound)
}

func (s *Server) handleCreateTriageAssessment(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.handleListAllTriageAssessments(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PetID       string                      `json:"petId"`
		HubID       string                      `json:"hubId"`
		MedicID     string                      `json:"medicId"`
		MedicName   string                      `json:"medicName"`
		Species     string                      `json:"species"`
		WeightKg    float64                     `json:"weightKg"`
		Vitals      domain.VitalSigns           `json:"vitals"`
		Trauma      veterinary.TraumaIndicators `json:"trauma"`
		TraumaNotes string                      `json:"traumaNotes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		http.Error(w, "petId is required", http.StatusBadRequest)
		return
	}

	category, reasons := veterinary.EvaluateTriage(req.Species, req.Vitals, req.Trauma)
	dosages := veterinary.CalculateEmergencyDosages(req.Species, req.WeightKg)

	now := time.Now().UTC()
	assessment := domain.TriageAssessment{
		AssessmentID:           fmt.Sprintf("triage-%d", now.UnixNano()),
		PetID:                  petID,
		HubID:                  strings.TrimSpace(req.HubID),
		MedicID:                strings.TrimSpace(req.MedicID),
		MedicName:              strings.TrimSpace(req.MedicName),
		Species:                strings.TrimSpace(req.Species),
		WeightKg:               req.WeightKg,
		Category:               category,
		Vitals:                 req.Vitals,
		TraumaNotes:            strings.TrimSpace(req.TraumaNotes),
		AdministeredTreatments: []domain.ClinicalTreatment{},
		AssessedAt:             now,
	}

	if s.stateStore != nil {
		assessBytes, err := json.Marshal(assessment)
		if err != nil {
			http.Error(w, "Failed to serialize assessment", http.StatusInternalServerError)
			return
		}
		if err := s.stateStore.SaveState(r.Context(), store.CollectionTriageAssessments, assessment.AssessmentID, assessBytes); err != nil {
			http.Error(w, "Failed to save assessment", http.StatusInternalServerError)
			return
		}
	}

	eventPayload := map[string]any{
		"type":         "triage_assessment_created",
		"assessmentId": assessment.AssessmentID,
		"petId":        assessment.PetID,
		"hubId":        assessment.HubID,
		"category":     assessment.Category,
		"species":      assessment.Species,
		"reasons":      reasons,
		"dosages":      dosages,
		"assessedAt":   assessment.AssessedAt,
	}

	// SSE distribution via s.reunionHub
	if s.reunionHub != nil {
		event := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_triage_%s_%d", assessment.AssessmentID, now.UnixNano()),
			Type:      domain.ReunionEventType("triage_assessment_created"),
			MatchID:   assessment.PetID,
			Timestamp: now,
			Payload:   eventPayload,
		}
		s.reunionHub.Broadcast(event)

		if assessment.HubID != "" && assessment.HubID != assessment.PetID {
			hubEvent := event
			hubEvent.MatchID = assessment.HubID
			s.reunionHub.Broadcast(hubEvent)
		}

		triageEvent := event
		triageEvent.MatchID = "triage"
		s.reunionHub.Broadcast(triageEvent)
	}

	// Mesh distribution via s.signalingHub
	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(eventPayload)
		medicNodeID := assessment.MedicID
		if medicNodeID == "" {
			medicNodeID = "medic-node"
		}
		envelope := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType("triage_assessment_created"),
			SearchPartyID: assessment.PetID,
			SenderNodeID:  medicNodeID,
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(envelope)

		if assessment.HubID != "" && assessment.HubID != assessment.PetID {
			hubEnv := envelope
			hubEnv.SearchPartyID = assessment.HubID
			s.signalingHub.Broadcast(hubEnv)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assessment)
}

func (s *Server) handleGetTriageAssessmentsByPetID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petId"))
	if petID == "" {
		petID = strings.TrimSpace(r.PathValue("petID"))
	}
	if petID == "" {
		http.Error(w, "petId is required", http.StatusBadRequest)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	rawAssessments, err := s.stateStore.ListState(r.Context(), store.CollectionTriageAssessments)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrStoreNotFound) {
		http.Error(w, "Failed to query triage assessments", http.StatusInternalServerError)
		return
	}

	assessments := make([]domain.TriageAssessment, 0)
	for _, b := range rawAssessments {
		var a domain.TriageAssessment
		if err := json.Unmarshal(b, &a); err == nil {
			if a.PetID == petID {
				assessments = append(assessments, a)
			}
		}
	}

	sort.Slice(assessments, func(i, j int) bool {
		return assessments[i].AssessedAt.After(assessments[j].AssessedAt)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assessments)
}

func (s *Server) handleAppendTriageTreatment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	assessmentID := strings.TrimSpace(r.PathValue("assessmentId"))
	if assessmentID == "" {
		assessmentID = strings.TrimSpace(r.PathValue("assessmentID"))
	}
	if assessmentID == "" {
		http.Error(w, "assessmentId required", http.StatusBadRequest)
		return
	}

	var req struct {
		MedicationName string                `json:"medicationName"`
		Dosage         string                `json:"dosage"`
		Route          domain.TreatmentRoute `json:"route"`
		AdministeredBy string                `json:"administeredBy"`
		Notes          string                `json:"notes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	medicationName := strings.TrimSpace(req.MedicationName)
	if medicationName == "" {
		http.Error(w, "medicationName is required", http.StatusBadRequest)
		return
	}

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	treatment := domain.ClinicalTreatment{
		TreatmentID:    fmt.Sprintf("treat-%d", now.UnixNano()),
		MedicationName: medicationName,
		Dosage:         req.Dosage,
		Route:          req.Route,
		AdministeredBy: req.AdministeredBy,
		AdministeredAt: now,
		Notes:          req.Notes,
	}

	var updatedAssessment domain.TriageAssessment
	err := s.stateStore.UpdateState(r.Context(), store.CollectionTriageAssessments, assessmentID, func(current []byte) ([]byte, error) {
		var a domain.TriageAssessment
		if err := json.Unmarshal(current, &a); err != nil {
			return nil, fmt.Errorf("corrupt triage assessment: %w", err)
		}
		a.AdministeredTreatments = append(a.AdministeredTreatments, treatment)
		a.ReassessedAt = &now
		updatedAssessment = a
		return json.Marshal(a)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			http.Error(w, "Assessment not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to record treatment: %v", err), http.StatusInternalServerError)
		return
	}

	treatmentPayload := map[string]any{
		"type":         "triage_treatment_administered",
		"assessmentId": updatedAssessment.AssessmentID,
		"petId":        updatedAssessment.PetID,
		"hubId":        updatedAssessment.HubID,
		"treatment":    treatment,
	}

	if s.reunionHub != nil {
		event := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_treatment_%s_%d", treatment.TreatmentID, now.UnixNano()),
			Type:      domain.ReunionEventType("triage_treatment_administered"),
			MatchID:   updatedAssessment.PetID,
			Timestamp: now,
			Payload:   treatmentPayload,
		}
		s.reunionHub.Broadcast(event)

		if updatedAssessment.HubID != "" && updatedAssessment.HubID != updatedAssessment.PetID {
			hubEvent := event
			hubEvent.MatchID = updatedAssessment.HubID
			s.reunionHub.Broadcast(hubEvent)
		}

		triageEvent := event
		triageEvent.MatchID = "triage"
		s.reunionHub.Broadcast(triageEvent)
	}

	if s.signalingHub != nil {
		payloadBytes, _ := json.Marshal(treatmentPayload)
		adminNodeID := treatment.AdministeredBy
		if adminNodeID == "" {
			adminNodeID = "medic-node"
		}
		envelope := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType("triage_treatment_administered"),
			SearchPartyID: updatedAssessment.PetID,
			SenderNodeID:  adminNodeID,
			Payload:       payloadBytes,
			Timestamp:     now,
		}
		s.signalingHub.Broadcast(envelope)

		if updatedAssessment.HubID != "" && updatedAssessment.HubID != updatedAssessment.PetID {
			hubEnv := envelope
			hubEnv.SearchPartyID = updatedAssessment.HubID
			s.signalingHub.Broadcast(hubEnv)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(updatedAssessment)
}

func (s *Server) handleListAllTriageAssessments(w http.ResponseWriter, r *http.Request) {
	assessments := make([]domain.TriageAssessment, 0)
	if s.stateStore != nil {
		rawAssessments, err := s.stateStore.ListState(r.Context(), store.CollectionTriageAssessments)
		if err == nil {
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
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assessments)
}

// handleApiVeterinaryTriageStream serves the real-time SSE stream for veterinary triage assessments and treatments.
func (s *Server) handleApiVeterinaryTriageStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	if _, err := fmt.Fprint(w, ": connected\n\n"); err != nil {
		return
	}
	flusher.Flush()

	if s.reunionHub == nil {
		return
	}

	subCh, unsubscribe := s.reunionHub.Subscribe("triage")
	defer unsubscribe()

	pingInterval := s.reunionPingInterval
	if pingInterval <= 0 {
		pingInterval = defaultReunionPingInterval
	}
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case event, ok := <-subCh:
			if !ok {
				return
			}
			if err := formatSSEEvent(w, event); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// TriagePageData represents the data passed to templates/triage.html.
type TriagePageData struct {
	Assessments []domain.TriageAssessment
	Locale      string
}

// handleRenderTriage renders the interactive Crisis Medical Triage Cockpit.
func (s *Server) handleRenderTriage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var assessments []domain.TriageAssessment
	if s.stateStore != nil {
		rawAssessments, err := s.stateStore.ListState(r.Context(), store.CollectionTriageAssessments)
		if err == nil {
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

	tmpl, err := template.New("triage.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/triage.html")
	if err != nil {
		http.Error(w, "Failed to load triage template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := TriagePageData{
		Assessments: assessments,
		Locale:      LocaleFromContext(r.Context()),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=(self)")
	w.WriteHeader(http.StatusOK)
	_ = tmpl.Execute(w, data)
}

// PassportViewModel represents the data passed to templates/passport.html.
type PassportViewModel struct {
	Passport           domain.VeterinaryPassport
	QRDataURI          string
	QRPayload          string
	HasCriticalAllergy bool
	CriticalAllergies  []domain.ClinicalAllergy
	FormattedIssuedAt  string
	FormattedExpiresAt string
	Locale             string
}

// handleRenderPassport renders the printable emergency passport card.
func (s *Server) handleRenderPassport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "p" {
			petID = parts[1]
		}
	}
	if petID == "" {
		http.NotFound(w, r)
		return
	}

	var passport domain.VeterinaryPassport
	found := false

	if s.stateStore != nil {
		// 1. Direct state lookup by petID
		if data, err := s.stateStore.GetState(r.Context(), store.CollectionVeterinaryPassports, petID); err == nil {
			if err := json.Unmarshal(data, &passport); err == nil {
				found = true
			}
		}

		// 2. Scan collection for matching petID or passportID
		if !found {
			if rawPassports, err := s.stateStore.ListState(r.Context(), store.CollectionVeterinaryPassports); err == nil {
				for _, b := range rawPassports {
					var p domain.VeterinaryPassport
					if err := json.Unmarshal(b, &p); err == nil {
						if p.PetID == petID || p.PassportID == petID {
							passport = p
							found = true
							break
						}
					}
				}
			}
		}
	}

	// 3. Fallback sample passport for testing or demo preview
	if !found {
		if petID != "sample-passport" {
			http.NotFound(w, r)
			return
		}
		now := time.Now().UTC()
		petName := "Rusty"
		species := "Dog"
		breed := "Golden Retriever"

		if s.stateStore != nil {
			if petRecord, err := s.getLostPetRecord(r.Context(), petID); err == nil && petRecord != nil {
				if strings.TrimSpace(petRecord.PetName) != "" {
					petName = petRecord.PetName
				}
				if strings.TrimSpace(petRecord.Species) != "" {
					species = petRecord.Species
				}
				if strings.TrimSpace(petRecord.Breed) != "" {
					breed = petRecord.Breed
				}
			}
		}

		passport = domain.VeterinaryPassport{
			PassportID:  fmt.Sprintf("vp-%s", petID),
			PetID:       petID,
			PetName:     petName,
			Species:     species,
			Breed:       breed,
			MicrochipID: "985141000998811",
			RabiesTagID: "RAB-2026-X1",
			BloodType:   "DEA 1.1 Negative",
			WeightKg:    30.0,
			Vaccinations: []domain.VaccinationRecord{
				{
					VaccineName:      "Rabies 3-Yr",
					AdministeredDate: now.AddDate(-1, 0, 0),
					ExpirationDate:   now.AddDate(2, 0, 0),
					ClinicName:       "Metro Emergency Veterinary Center",
					Verified:         true,
				},
				{
					VaccineName:      "DHPP (Distemper, Hepatitis, Parvo)",
					AdministeredDate: now.AddDate(-1, 0, 0),
					ExpirationDate:   now.AddDate(2, 0, 0),
					ClinicName:       "Metro Emergency Veterinary Center",
					Verified:         true,
				},
			},
			Allergies: []domain.ClinicalAllergy{
				{
					Allergen:            "Penicillin",
					Severity:            domain.AllergySeverityAnaphylactic,
					ReactionDescription: "Anaphylaxis / acute collapse upon administration",
				},
			},
			ChronicConditions: []domain.ChronicCondition{
				{
					ConditionName: "Mild Canine Hip Dysplasia",
					DiagnosedDate: now.AddDate(-2, 0, 0),
					Medications:   []string{"Omega-3 fatty acids"},
					CriticalFlag:  false,
				},
			},
			EmergencyContact: "Sarah Jenkins: (555) 234-5678",
			PrimaryClinic:    "Metro Emergency Veterinary Center",
			IssuedAt:         now.AddDate(0, -1, 0),
			ExpiresAt:        now.AddDate(0, 11, 0),
		}
	}

	priv, _, _ := getMasterKeys()
	var qrDataURI, qrPayload string
	if priv != nil {
		if payload, err := veterinary.EncodeOfflinePassportPayload(&passport, priv); err == nil {
			qrPayload = payload
			if dataURI, err := veterinary.GeneratePassportQRCode(payload); err == nil {
				qrDataURI = dataURI
			}
		}
	}

	var criticalAllergies []domain.ClinicalAllergy
	for _, a := range passport.Allergies {
		if a.Severity == domain.AllergySeverityAnaphylactic || strings.EqualFold(string(a.Severity), "ANAPHYLACTIC") {
			criticalAllergies = append(criticalAllergies, a)
		}
	}

	tmpl, err := template.New("passport.html").Funcs(templateFuncMap).ParseFS(embeddedFiles, "templates/passport.html")
	if err != nil {
		http.Error(w, "Failed to load passport template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	vm := PassportViewModel{
		Passport:           passport,
		QRDataURI:          qrDataURI,
		QRPayload:          qrPayload,
		HasCriticalAllergy: len(criticalAllergies) > 0,
		CriticalAllergies:  criticalAllergies,
		FormattedIssuedAt:  passport.IssuedAt.Format("Jan 02, 2006"),
		FormattedExpiresAt: passport.ExpiresAt.Format("Jan 02, 2006"),
		Locale:             LocaleFromContext(r.Context()),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Permissions-Policy", "camera=(), geolocation=(self), microphone=(self)")
	w.WriteHeader(http.StatusOK)
	_ = tmpl.Execute(w, vm)
}
