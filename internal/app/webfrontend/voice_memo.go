package webfrontend

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/audio"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// VoiceMemoRecord stores persistence metadata for a sighting voice memo.
type VoiceMemoRecord struct {
	SightingID      string    `json:"sightingId"`
	LostPetID       string    `json:"lostPetId"`
	MIMEType        string    `json:"mimeType"`
	DurationSeconds float64   `json:"durationSeconds"`
	Waveform        []float64 `json:"waveform"`
	VoiceMemoURL    string    `json:"voiceMemoUrl"`
	CreatedAt       time.Time `json:"createdAt"`
}

// VoiceMemoUploadResponse is returned upon successful voice note upload.
type VoiceMemoUploadResponse struct {
	VoiceMemoURL    string    `json:"voiceMemoUrl"`
	DurationSeconds float64   `json:"durationSeconds"`
	Waveform        []float64 `json:"waveform"`
}

func (s *Server) handleApiVoiceMemo(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleApiPostVoiceMemo(w, r)
	case http.MethodGet:
		s.handleApiGetVoiceMemo(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleApiPostVoiceMemo(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	sightingID := strings.TrimSpace(r.PathValue("sightingID"))
	if sightingID == "" {
		sightingID = strings.TrimSpace(r.PathValue("id"))
	}
	if petID == "" || sightingID == "" {
		http.NotFound(w, r)
		return
	}

	// Verify pet exists
	_, err := s.stateStore.GetState(r.Context(), store.LostPetsCollection, petID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to verify lost pet", http.StatusInternalServerError)
		return
	}

	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	r.Body = http.MaxBytesReader(w, r.Body, audio.MaxAudioBytes+1024)
	file, fileHeader, err := r.FormFile("audio")
	if err != nil {
		// Fallback to "file" field name
		file, fileHeader, err = r.FormFile("file")
	}
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, audio.ErrAudioTooLarge.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "Invalid multipart form or missing 'audio' field", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	audioData, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Failed to read audio file", http.StatusBadRequest)
		return
	}

	var clientDuration float64
	if durStr := r.FormValue("duration"); durStr != "" {
		if d, err := strconv.ParseFloat(durStr, 64); err == nil {
			clientDuration = d
		}
	}

	contentType := fileHeader.Header.Get("Content-Type")
	memo, err := audio.ProcessVoiceMemo(audioData, contentType, clientDuration)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	memoURL := fmt.Sprintf("/api/v1/lost-pets/%s/sightings/%s/voice-memo", petID, sightingID)
	record := VoiceMemoRecord{
		SightingID:      sightingID,
		LostPetID:       petID,
		MIMEType:        memo.MIMEType,
		DurationSeconds: memo.DurationSeconds,
		Waveform:        memo.Waveform,
		VoiceMemoURL:    memoURL,
		CreatedAt:       time.Now().UTC(),
	}

	recordBytes, err := json.Marshal(record)
	if err != nil {
		http.Error(w, "Failed to encode voice memo record", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.VoiceMemosCollection, sightingID, recordBytes); err != nil {
		http.Error(w, "Failed to persist voice memo metadata", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.VoiceMemosAudioCollection, sightingID, memo.Data); err != nil {
		http.Error(w, "Failed to persist voice memo audio data", http.StatusInternalServerError)
		return
	}

	// Update sighting record if already present in store
	if sBytes, err := s.stateStore.GetState(r.Context(), store.SightingsCollection, sightingID); err == nil {
		var sRec domain.PetSightingRecord
		if err := json.Unmarshal(sBytes, &sRec); err == nil {
			sRec.VoiceMemoURL = memoURL
			sRec.VoiceMemoDuration = memo.DurationSeconds
			sRec.VoiceMemoWaveform = memo.Waveform
			if updatedBytes, err := json.Marshal(sRec); err == nil {
				_ = s.stateStore.SaveState(r.Context(), store.SightingsCollection, sightingID, updatedBytes)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(VoiceMemoUploadResponse{
		VoiceMemoURL:    memoURL,
		DurationSeconds: memo.DurationSeconds,
		Waveform:        memo.Waveform,
	})
}

func (s *Server) handleApiGetVoiceMemo(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	petID := strings.TrimSpace(r.PathValue("petID"))
	sightingID := strings.TrimSpace(r.PathValue("sightingID"))
	if sightingID == "" {
		sightingID = strings.TrimSpace(r.PathValue("id"))
	}
	if petID == "" || sightingID == "" {
		http.NotFound(w, r)
		return
	}

	metaBytes, err := s.stateStore.GetState(r.Context(), store.VoiceMemosCollection, sightingID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve voice memo", http.StatusInternalServerError)
		return
	}

	var record VoiceMemoRecord
	if err := json.Unmarshal(metaBytes, &record); err != nil {
		http.Error(w, "Invalid voice memo metadata", http.StatusInternalServerError)
		return
	}

	if record.LostPetID != "" && record.LostPetID != petID {
		http.NotFound(w, r)
		return
	}

	audioBytes, err := s.stateStore.GetState(r.Context(), store.VoiceMemosAudioCollection, sightingID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve audio data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", record.MIMEType)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, "voice-memo", record.CreatedAt, bytes.NewReader(audioBytes))
}
