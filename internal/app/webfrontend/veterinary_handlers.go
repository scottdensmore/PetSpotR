package webfrontend

import (
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
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
	keyOnce       sync.Once
)

func getMasterKeys() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	var err error
	keyOnce.Do(func() {
		masterPrivKey, masterPubKey, err = veterinary.GeneratePassportKey()
	})
	return masterPrivKey, masterPubKey, err
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
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
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

	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	data, err := s.stateStore.GetState(r.Context(), store.CollectionTriageAssessments, assessmentID)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	var assessment domain.TriageAssessment
	if err := json.Unmarshal(data, &assessment); err != nil {
		http.Error(w, "Data corrupt", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	treatment := domain.ClinicalTreatment{
		TreatmentID:    fmt.Sprintf("treat-%d", now.UnixNano()),
		MedicationName: req.MedicationName,
		Dosage:         req.Dosage,
		Route:          req.Route,
		AdministeredBy: req.AdministeredBy,
		AdministeredAt: now,
		Notes:          req.Notes,
	}

	assessment.AdministeredTreatments = append(assessment.AdministeredTreatments, treatment)
	assessment.ReassessedAt = &now

	updatedBytes, err := json.Marshal(assessment)
	if err != nil {
		http.Error(w, "Failed to encode updated assessment", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.CollectionTriageAssessments, assessmentID, updatedBytes); err != nil {
		http.Error(w, "Failed to save treatment", http.StatusInternalServerError)
		return
	}

	if s.reunionHub != nil {
		s.reunionHub.Broadcast(domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_treatment_%s_%d", treatment.TreatmentID, now.UnixNano()),
			Type:      domain.ReunionEventType("triage_treatment_administered"),
			MatchID:   assessment.PetID,
			Timestamp: now,
			Payload: map[string]any{
				"type":         "triage_treatment_administered",
				"assessmentId": assessment.AssessmentID,
				"petId":        assessment.PetID,
				"treatment":    treatment,
			},
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assessment)
}
