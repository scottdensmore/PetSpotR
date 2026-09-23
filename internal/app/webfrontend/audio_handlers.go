package webfrontend

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/audio"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func (s *Server) handleAudioAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var audioBytes []byte
	var dataURI string

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		_ = r.ParseMultipartForm(10 * 1024 * 1024)
		file, _, err := r.FormFile("audio")
		if err != nil {
			http.Error(w, "Missing audio file in form", http.StatusBadRequest)
			return
		}
		defer func() {
			_ = file.Close()
		}()
		audioBytes, _ = io.ReadAll(file)
		dataURI = "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(audioBytes)
	} else {
		var req struct {
			AudioDataURI string `json:"audioDataUri"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AudioDataURI == "" {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}
		dataURI = req.AudioDataURI
		idx := strings.Index(dataURI, ",")
		if idx == -1 {
			http.Error(w, "Invalid data URI", http.StatusBadRequest)
			return
		}
		raw, err := base64.StdEncoding.DecodeString(dataURI[idx+1:])
		if err != nil {
			http.Error(w, "Failed to decode base64 audio", http.StatusBadRequest)
			return
		}
		audioBytes = raw
	}

	memo, err := audio.ProcessVoiceMemo(audioBytes, "audio/wav", 0)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to process audio: %v", err), http.StatusBadRequest)
		return
	}

	pcm := extractPCMSamples(memo.Data)
	voiceprint, spec, err := audio.ExtractVoiceprint(pcm, audio.StandardSampleRate)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract voiceprint: %v", err), http.StatusInternalServerError)
		return
	}

	vocalization, conf := audio.ClassifyVocalization(voiceprint, pcm, audio.StandardSampleRate)
	vocalization, conf, _ = audio.VerifyAcousticWithOllama(r.Context(), s.ollamaClient, vocalization, conf, voiceprint)

	audioID := generateAudioID()
	profile := domain.AudioProfile{
		AudioID:         audioID,
		Vocalization:    vocalization,
		ConfidenceScore: conf,
		Voiceprint:      voiceprint,
		SpectrogramBins: spec,
		AudioDataURI:    dataURI,
		RecordedAt:      time.Now().UTC(),
	}

	pBytes, _ := json.Marshal(profile)
	_ = s.stateStore.SaveState(r.Context(), store.CollectionAudioProfiles, audioID, pBytes)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profile)
}

func (s *Server) handleLostPetAudioProfile(w http.ResponseWriter, r *http.Request) {
	petID := strings.TrimSpace(r.PathValue("petID"))
	if petID == "" {
		petID = strings.TrimPrefix(r.URL.Path, "/api/v1/lost-pets/")
		petID = strings.TrimSuffix(petID, "/audio-profile")
		petID = strings.TrimSpace(petID)
	}

	if r.Method == http.MethodGet {
		petBytes, err := s.stateStore.GetState(r.Context(), store.CollectionLostPets, petID)
		if err != nil {
			http.Error(w, "Pet not found", http.StatusNotFound)
			return
		}
		var pet domain.LostPetRecord
		_ = json.Unmarshal(petBytes, &pet)
		if pet.ReferenceAudioProfile == nil {
			http.Error(w, "No reference audio profile for pet", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(pet.ReferenceAudioProfile)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petBytes, err := s.stateStore.GetState(r.Context(), store.CollectionLostPets, petID)
	if err != nil {
		http.Error(w, "Pet not found", http.StatusNotFound)
		return
	}

	var req struct {
		AudioDataURI string `json:"audioDataUri"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AudioDataURI == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	idx := strings.Index(req.AudioDataURI, ",")
	if idx == -1 {
		http.Error(w, "Invalid data URI", http.StatusBadRequest)
		return
	}
	audioBytes, err := base64.StdEncoding.DecodeString(req.AudioDataURI[idx+1:])
	if err != nil {
		http.Error(w, "Failed to decode base64 audio", http.StatusBadRequest)
		return
	}
	memo, err := audio.ProcessVoiceMemo(audioBytes, "audio/wav", 0)
	if err != nil {
		http.Error(w, "Failed to process audio", http.StatusBadRequest)
		return
	}

	pcm := extractPCMSamples(memo.Data)
	vp, spec, err := audio.ExtractVoiceprint(pcm, audio.StandardSampleRate)
	if err != nil {
		http.Error(w, "Failed to extract voiceprint", http.StatusInternalServerError)
		return
	}
	vocal, conf := audio.ClassifyVocalization(vp, pcm, audio.StandardSampleRate)
	vocal, conf, _ = audio.VerifyAcousticWithOllama(r.Context(), s.ollamaClient, vocal, conf, vp)

	profile := domain.AudioProfile{
		AudioID:         generateAudioID(),
		PetID:           petID,
		Vocalization:    vocal,
		ConfidenceScore: conf,
		Voiceprint:      vp,
		SpectrogramBins: spec,
		AudioDataURI:    req.AudioDataURI,
		RecordedAt:      time.Now().UTC(),
	}

	var petMap map[string]any
	if err := json.Unmarshal(petBytes, &petMap); err != nil {
		http.Error(w, "Failed to parse pet state", http.StatusInternalServerError)
		return
	}
	petMap["referenceAudioProfile"] = profile
	updatedPetBytes, err := json.Marshal(petMap)
	if err != nil {
		http.Error(w, "Failed to serialize updated pet state", http.StatusInternalServerError)
		return
	}
	_ = s.stateStore.SaveState(r.Context(), store.CollectionLostPets, petID, updatedPetBytes)

	profBytes, _ := json.Marshal(profile)
	_ = s.stateStore.SaveState(r.Context(), store.CollectionAudioProfiles, profile.AudioID, profBytes)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(profile)
}

func (s *Server) handleAudioMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ReferenceAudioID string `json:"referenceAudioId"`
		CandidateAudioID string `json:"candidateAudioId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	refBytes, err1 := s.stateStore.GetState(r.Context(), store.CollectionAudioProfiles, req.ReferenceAudioID)
	candBytes, err2 := s.stateStore.GetState(r.Context(), store.CollectionAudioProfiles, req.CandidateAudioID)
	if err1 != nil || err2 != nil {
		http.Error(w, "Audio profiles not found", http.StatusNotFound)
		return
	}

	var refProf, candProf domain.AudioProfile
	_ = json.Unmarshal(refBytes, &refProf)
	_ = json.Unmarshal(candBytes, &candProf)

	score := audio.ComputeCosineSimilarity(refProf.Voiceprint.Features, candProf.Voiceprint.Features)
	petID := refProf.PetID
	if petID == "" {
		petID = candProf.PetID
	}

	res := domain.AcousticMatchResult{
		ReferenceAudioID: refProf.AudioID,
		CandidateAudioID: candProf.AudioID,
		PetID:            petID,
		SimilarityScore:  score,
		IsProbableMatch:  score >= 0.75,
		Vocalization:     candProf.Vocalization,
		MatchedAt:        time.Now().UTC(),
	}

	matchKey := fmt.Sprintf("%s_%s", refProf.AudioID, candProf.AudioID)
	if resBytes, err := json.Marshal(res); err == nil {
		_ = s.stateStore.SaveState(r.Context(), store.CollectionAcousticMatches, matchKey, resBytes)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleAudioSpectrogram(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	audioID := strings.TrimSpace(r.PathValue("id"))
	if audioID == "" {
		audioID = strings.TrimPrefix(r.URL.Path, "/api/v1/audio/")
		audioID = strings.TrimSuffix(audioID, "/spectrogram")
		audioID = strings.TrimSpace(audioID)
	}

	profileBytes, err := s.stateStore.GetState(r.Context(), store.CollectionAudioProfiles, audioID)
	if err != nil {
		http.Error(w, "Audio profile not found", http.StatusNotFound)
		return
	}

	var profile domain.AudioProfile
	if err := json.Unmarshal(profileBytes, &profile); err != nil {
		http.Error(w, "Failed to parse audio profile", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"audioId":         profile.AudioID,
		"spectrogramBins": profile.SpectrogramBins,
	})
}

func (s *Server) handleApiSightingsGeneric(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateSighting(w, r)
	case http.MethodGet:
		s.handleListAllSightings(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type genericSightingRequest struct {
	PetID               string                `json:"petId"`
	LostPetID           string                `json:"lostPetId"`
	Latitude            *float64              `json:"latitude"`
	Longitude           *float64              `json:"longitude"`
	Coordinates         *domain.LocationPoint `json:"coordinates"`
	LocationDescription string                `json:"locationDescription"`
	MovementDirection   string                `json:"movementDirection"`
	ImageURL            string                `json:"imageUrl"`
	ImageURLAlt         string                `json:"imageURL"`
	ImageObject         string                `json:"imageObject"`
	Notes               string                `json:"notes"`
	AudioDataURI        string                `json:"audioDataUri"`
	SightedAt           time.Time             `json:"sightedAt"`
}

func (s *Server) handleCreateSighting(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	var req genericSightingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	petID := strings.TrimSpace(req.PetID)
	if petID == "" {
		petID = strings.TrimSpace(req.LostPetID)
	}

	coords := req.Coordinates
	if coords == nil && req.Latitude != nil && req.Longitude != nil {
		coords = &domain.LocationPoint{
			Latitude:  *req.Latitude,
			Longitude: *req.Longitude,
		}
	}

	sightedAt := req.SightedAt
	if sightedAt.IsZero() {
		sightedAt = time.Now().UTC()
	}

	imageURL := req.ImageURL
	if imageURL == "" && req.ImageURLAlt != "" {
		imageURL = req.ImageURLAlt
	}

	sightingID := generateSightingID()
	sighting := domain.Sighting{
		ID:                  sightingID,
		SightingID:          sightingID,
		PetID:               petID,
		LostPetID:           petID,
		ReportedAt:          time.Now().UTC(),
		SightedAt:           sightedAt.UTC(),
		LocationDescription: strings.TrimSpace(req.LocationDescription),
		Coordinates:         coords,
		MovementDirection:   strings.TrimSpace(req.MovementDirection),
		ImageURL:            imageURL,
		ImageObject:         strings.TrimSpace(req.ImageObject),
		Notes:               strings.TrimSpace(req.Notes),
		Status:              domain.SightingStatusActive,
	}

	s.attachAcousticMatchIfApplicable(r.Context(), &sighting, req.AudioDataURI)

	sBytes, err := json.Marshal(sighting)
	if err != nil {
		http.Error(w, "Failed to encode sighting", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.SightingsCollection, sighting.ID, sBytes); err != nil {
		http.Error(w, "Failed to persist sighting", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(sBytes)
}

func (s *Server) handleListAllSightings(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	rawItems, err := s.stateStore.ListState(r.Context(), store.SightingsCollection)
	if err != nil {
		http.Error(w, "Failed to list sightings", http.StatusInternalServerError)
		return
	}

	sightings := make([]domain.Sighting, 0, len(rawItems))
	for _, b := range rawItems {
		var sRec domain.Sighting
		if err := json.Unmarshal(b, &sRec); err == nil {
			sightings = append(sightings, sRec)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(sightings)
}

func (s *Server) attachAcousticMatchIfApplicable(ctx context.Context, sighting *domain.Sighting, audioDataURI string) {
	if sighting == nil || audioDataURI == "" || sighting.PetID == "" {
		return
	}

	petBytes, err := s.stateStore.GetState(ctx, store.CollectionLostPets, sighting.PetID)
	if err != nil {
		return
	}
	var pet domain.LostPetReport
	if json.Unmarshal(petBytes, &pet) != nil || pet.ReferenceAudioProfile == nil {
		return
	}

	idx := strings.Index(audioDataURI, ",")
	if idx == -1 {
		return
	}
	audioBytes, err := base64.StdEncoding.DecodeString(audioDataURI[idx+1:])
	if err != nil {
		return
	}

	memo, err := audio.ProcessVoiceMemo(audioBytes, "audio/wav", 0)
	if err != nil {
		return
	}

	pcm := extractPCMSamples(memo.Data)
	vp, spec, _ := audio.ExtractVoiceprint(pcm, audio.StandardSampleRate)
	vocal, conf := audio.ClassifyVocalization(vp, pcm, audio.StandardSampleRate)

	candProfile := domain.AudioProfile{
		AudioID:         generateAudioID(),
		PetID:           sighting.PetID,
		SightingID:      sighting.ID,
		Vocalization:    vocal,
		ConfidenceScore: conf,
		Voiceprint:      vp,
		SpectrogramBins: spec,
		AudioDataURI:    audioDataURI,
		RecordedAt:      time.Now().UTC(),
	}
	sighting.AudioProfile = &candProfile

	if profBytes, err := json.Marshal(candProfile); err == nil {
		_ = s.stateStore.SaveState(ctx, store.CollectionAudioProfiles, candProfile.AudioID, profBytes)
	}

	score := audio.ComputeCosineSimilarity(pet.ReferenceAudioProfile.Voiceprint.Features, vp.Features)
	matchRes := domain.AcousticMatchResult{
		ReferenceAudioID: pet.ReferenceAudioProfile.AudioID,
		CandidateAudioID: candProfile.AudioID,
		PetID:            sighting.PetID,
		SimilarityScore:  score,
		IsProbableMatch:  score >= 0.75,
		Vocalization:     vocal,
		MatchedAt:        time.Now().UTC(),
	}
	sighting.AcousticMatch = &matchRes

	if matchBytes, err := json.Marshal(matchRes); err == nil {
		matchKey := fmt.Sprintf("%s_%s", pet.ReferenceAudioProfile.AudioID, candProfile.AudioID)
		_ = s.stateStore.SaveState(ctx, store.CollectionAcousticMatches, matchKey, matchBytes)
	}

	if matchRes.IsProbableMatch && s.signalingHub != nil {
		envelope := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType("mesh:audio-match"),
			SearchPartyID: sighting.PetID,
			SenderNodeID:  "acoustic-classifier",
			Timestamp:     time.Now().UTC(),
		}
		s.signalingHub.Broadcast(envelope)
	}

	if matchRes.IsProbableMatch && s.reunionHub != nil {
		event := domain.ReunionStreamEvent{
			EventID:   fmt.Sprintf("evt_audio_match_%s", candProfile.AudioID),
			Type:      domain.ReunionEventType("sighting_acoustic_matched"),
			MatchID:   sighting.PetID,
			Timestamp: time.Now().UTC(),
			Payload: map[string]interface{}{
				"type":          "sighting_acoustic_matched",
				"petId":         sighting.PetID,
				"sightingId":    sighting.ID,
				"acousticMatch": matchRes,
			},
		}
		s.reunionHub.Broadcast(event)
	}
}

func extractPCMSamples(data []byte) []float64 {
	raw := data
	if len(raw) >= 44 && string(raw[:4]) == "RIFF" {
		idx := bytes.Index(raw, []byte("data"))
		if idx != -1 && len(raw) >= idx+8 {
			raw = raw[idx+8:]
		}
	}
	pcm := make([]float64, len(raw)/2)
	for i := 0; i < len(pcm); i++ {
		s := int16(raw[i*2]) | (int16(raw[i*2+1]) << 8)
		pcm[i] = float64(s) / 32768.0
	}
	return pcm
}

func generateAudioID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("audio-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("audio-%s", hex.EncodeToString(b))
}
