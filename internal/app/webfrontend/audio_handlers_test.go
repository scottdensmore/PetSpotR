package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func generateTestWAVBytes(freq float64, durationSec float64) []byte {
	sampleRate := 16000
	numSamples := int(float64(sampleRate) * durationSec)
	dataSize := numSamples * 2
	totalSize := 36 + dataSize

	buf := new(bytes.Buffer)
	buf.WriteString("RIFF")
	buf.Write([]byte{byte(totalSize), byte(totalSize >> 8), byte(totalSize >> 16), byte(totalSize >> 24)})
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	buf.Write([]byte{16, 0, 0, 0}) // Subchunk1Size
	buf.Write([]byte{1, 0})        // AudioFormat (PCM)
	buf.Write([]byte{1, 0})        // NumChannels (1)
	buf.Write([]byte{byte(sampleRate), byte(sampleRate >> 8), 0, 0})
	byteRate := sampleRate * 2
	buf.Write([]byte{byte(byteRate), byte(byteRate >> 8), 0, 0})
	buf.Write([]byte{2, 0})  // BlockAlign
	buf.Write([]byte{16, 0}) // BitsPerSample
	buf.WriteString("data")
	buf.Write([]byte{byte(dataSize), byte(dataSize >> 8), byte(dataSize >> 16), byte(dataSize >> 24)})

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		val := int16(0.7 * 32767.0 * math.Sin(2*math.Pi*freq*t))
		buf.Write([]byte{byte(val), byte(val >> 8)})
	}

	return buf.Bytes()
}

func generateTestWAVBase64(freq float64, durationSec float64) string {
	raw := generateTestWAVBytes(freq, durationSec)
	return "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(raw)
}

func TestAudioAnalyzeEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Test 1: JSON payload with Base64 audioDataUri
	wavURI := generateTestWAVBase64(350.0, 0.4) // Canine bark range
	payload := map[string]interface{}{
		"audioDataUri": wavURI,
	}
	bodyBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audio/analyze", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", w.Code, w.Body.String())
	}

	var profile domain.AudioProfile
	if err := json.Unmarshal(w.Body.Bytes(), &profile); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if profile.AudioID == "" {
		t.Error("expected non-empty AudioID")
	}
	if profile.Vocalization != domain.VocalizationCanineBark {
		t.Errorf("expected CANINE_BARK, got %s", profile.Vocalization)
	}
	if len(profile.Voiceprint.Features) != 32 {
		t.Errorf("expected 32 features, got %d", len(profile.Voiceprint.Features))
	}
	if len(profile.SpectrogramBins) != 32 {
		t.Errorf("expected 32 spectrogram slices, got %d", len(profile.SpectrogramBins))
	}

	// Test 2: multipart/form-data upload
	body := new(bytes.Buffer)
	mpWriter := multipart.NewWriter(body)
	filePart, err := mpWriter.CreateFormFile("audio", "sample.wav")
	if err != nil {
		t.Fatalf("failed to create multipart form file: %v", err)
	}
	rawWAV := generateTestWAVBytes(350.0, 0.4)
	if _, err := filePart.Write(rawWAV); err != nil {
		t.Fatalf("failed to write multipart bytes: %v", err)
	}
	_ = mpWriter.Close()

	mpReq := httptest.NewRequest(http.MethodPost, "/api/v1/audio/analyze", body)
	mpReq.Header.Set("Content-Type", mpWriter.FormDataContentType())
	mpW := httptest.NewRecorder()
	server.ServeHTTP(mpW, mpReq)

	if mpW.Code != http.StatusOK {
		t.Fatalf("expected 200 on multipart upload, got %d (body: %s)", mpW.Code, mpW.Body.String())
	}
	var mpProfile domain.AudioProfile
	if err := json.Unmarshal(mpW.Body.Bytes(), &mpProfile); err != nil {
		t.Fatalf("failed to decode multipart profile: %v", err)
	}
	if mpProfile.AudioID == "" {
		t.Error("expected non-empty AudioID for multipart profile")
	}
}

func TestLostPetAudioProfileAndSightingMatch(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	pet := domain.LostPetReport{
		ID:        "pet-audio-101",
		PetName:   "Buster",
		Species:   "Dog",
		Status:    "lost",
		CreatedAt: time.Now().UTC(),
	}
	pBytes, _ := json.Marshal(pet)
	_ = memStore.SaveState(context.Background(), store.CollectionLostPets, pet.ID, pBytes)

	// Subscribe to mesh signaling to verify mesh:audio-match broadcast
	sigHub := server.SignalingHub()
	msgCh, unsubscribe := sigHub.Subscribe(pet.ID, "node-acoustic-listener")
	defer unsubscribe()

	// 1. Attach reference audio profile
	refWAV := generateTestWAVBase64(380.0, 0.35)
	refPayload, _ := json.Marshal(map[string]interface{}{
		"audioDataUri": refWAV,
	})

	reqAttach := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/lost-pets/%s/audio-profile", pet.ID), bytes.NewReader(refPayload))
	reqAttach.Header.Set("Content-Type", "application/json")
	wAttach := httptest.NewRecorder()
	server.ServeHTTP(wAttach, reqAttach)
	if wAttach.Code != http.StatusOK {
		t.Fatalf("expected 200 on attach, got %d (body: %s)", wAttach.Code, wAttach.Body.String())
	}

	// 2. Retrieve attached audio profile via GET
	reqGetProfile := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/lost-pets/%s/audio-profile", pet.ID), nil)
	wGetProfile := httptest.NewRecorder()
	server.ServeHTTP(wGetProfile, reqGetProfile)
	if wGetProfile.Code != http.StatusOK {
		t.Fatalf("expected 200 on get audio-profile, got %d", wGetProfile.Code)
	}
	var fetchedProfile domain.AudioProfile
	if err := json.Unmarshal(wGetProfile.Body.Bytes(), &fetchedProfile); err != nil {
		t.Fatalf("failed to decode profile: %v", err)
	}
	if fetchedProfile.PetID != pet.ID {
		t.Errorf("expected PetID %s, got %s", pet.ID, fetchedProfile.PetID)
	}

	// 3. Submit sighting with matching audio
	sightingWAV := generateTestWAVBase64(380.0, 0.35)
	sightingPayload, _ := json.Marshal(map[string]interface{}{
		"petId":        pet.ID,
		"latitude":     37.7749,
		"longitude":    -122.4194,
		"notes":        "Heard barking in the drainage ditch",
		"audioDataUri": sightingWAV,
	})

	reqSighting := httptest.NewRequest(http.MethodPost, "/api/v1/sightings", bytes.NewReader(sightingPayload))
	reqSighting.Header.Set("Content-Type", "application/json")
	wSighting := httptest.NewRecorder()
	server.ServeHTTP(wSighting, reqSighting)
	if wSighting.Code != http.StatusCreated && wSighting.Code != http.StatusOK {
		t.Fatalf("expected 200/201 on sighting submission, got %d (body: %s)", wSighting.Code, wSighting.Body.String())
	}

	var createdSighting domain.Sighting
	_ = json.Unmarshal(wSighting.Body.Bytes(), &createdSighting)
	if createdSighting.AcousticMatch == nil {
		t.Fatal("expected non-nil AcousticMatch on created sighting")
	}
	if createdSighting.AcousticMatch.SimilarityScore < 0.75 {
		t.Errorf("expected similarity >= 0.75, got %f", createdSighting.AcousticMatch.SimilarityScore)
	}
	if !createdSighting.AcousticMatch.IsProbableMatch {
		t.Error("expected IsProbableMatch to be true")
	}

	// 4. Verify mesh:audio-match broadcast arrived on channel
	select {
	case env := <-msgCh:
		if env.Type != mesh.SignalingType("mesh:audio-match") {
			t.Errorf("expected mesh:audio-match event, got %s", env.Type)
		}
		if env.SearchPartyID != pet.ID {
			t.Errorf("expected search party ID %s, got %s", pet.ID, env.SearchPartyID)
		}
	case <-time.After(1 * time.Second):
		t.Error("timed out waiting for mesh:audio-match broadcast")
	}
}

func TestAudioMatchEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Analyze reference profile
	refWAV := generateTestWAVBase64(400.0, 0.35)
	refPayload, _ := json.Marshal(map[string]interface{}{"audioDataUri": refWAV})
	refReq := httptest.NewRequest(http.MethodPost, "/api/v1/audio/analyze", bytes.NewReader(refPayload))
	refReq.Header.Set("Content-Type", "application/json")
	refW := httptest.NewRecorder()
	server.ServeHTTP(refW, refReq)
	var refProf domain.AudioProfile
	_ = json.Unmarshal(refW.Body.Bytes(), &refProf)

	// Analyze identical candidate profile
	candWAV := generateTestWAVBase64(400.0, 0.35)
	candPayload, _ := json.Marshal(map[string]interface{}{"audioDataUri": candWAV})
	candReq := httptest.NewRequest(http.MethodPost, "/api/v1/audio/analyze", bytes.NewReader(candPayload))
	candReq.Header.Set("Content-Type", "application/json")
	candW := httptest.NewRecorder()
	server.ServeHTTP(candW, candReq)
	var candProf domain.AudioProfile
	_ = json.Unmarshal(candW.Body.Bytes(), &candProf)

	// Call POST /api/v1/audio/match
	matchPayload, _ := json.Marshal(map[string]interface{}{
		"referenceAudioId": refProf.AudioID,
		"candidateAudioId": candProf.AudioID,
	})
	matchReq := httptest.NewRequest(http.MethodPost, "/api/v1/audio/match", bytes.NewReader(matchPayload))
	matchReq.Header.Set("Content-Type", "application/json")
	matchW := httptest.NewRecorder()
	server.ServeHTTP(matchW, matchReq)

	if matchW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", matchW.Code, matchW.Body.String())
	}
	var matchRes domain.AcousticMatchResult
	if err := json.Unmarshal(matchW.Body.Bytes(), &matchRes); err != nil {
		t.Fatalf("failed to decode match result: %v", err)
	}
	if matchRes.SimilarityScore < 0.95 {
		t.Errorf("expected similarity score >= 0.95 for identical frequencies, got %f", matchRes.SimilarityScore)
	}
	if !matchRes.IsProbableMatch {
		t.Error("expected IsProbableMatch to be true")
	}
}

func TestAudioSpectrogramEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Analyze audio
	wavURI := generateTestWAVBase64(350.0, 0.4)
	payload, _ := json.Marshal(map[string]interface{}{"audioDataUri": wavURI})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/audio/analyze", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	var profile domain.AudioProfile
	_ = json.Unmarshal(w.Body.Bytes(), &profile)

	// GET /api/v1/audio/{id}/spectrogram
	specReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/audio/%s/spectrogram", profile.AudioID), nil)
	specW := httptest.NewRecorder()
	server.ServeHTTP(specW, specReq)

	if specW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body: %s)", specW.Code, specW.Body.String())
	}
	var res struct {
		AudioID         string      `json:"audioId"`
		SpectrogramBins [][]float64 `json:"spectrogramBins"`
	}
	if err := json.Unmarshal(specW.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode spectrogram response: %v", err)
	}
	if res.AudioID != profile.AudioID {
		t.Errorf("expected audioId %s, got %s", profile.AudioID, res.AudioID)
	}
	if len(res.SpectrogramBins) != 32 {
		t.Errorf("expected 32 spectrogram bins, got %d", len(res.SpectrogramBins))
	}
}

func TestAudioErrorCases(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

	// 1. Analyze with invalid data URI
	badPayload, _ := json.Marshal(map[string]interface{}{"audioDataUri": "invalid-no-comma"})
	badReq := httptest.NewRequest(http.MethodPost, "/api/v1/audio/analyze", bytes.NewReader(badPayload))
	badReq.Header.Set("Content-Type", "application/json")
	badW := httptest.NewRecorder()
	server.ServeHTTP(badW, badReq)
	if badW.Code != http.StatusBadRequest {
		t.Errorf("expected 400 on invalid data URI, got %d", badW.Code)
	}

	// 2. Spectrogram for nonexistent audio ID
	specReq := httptest.NewRequest(http.MethodGet, "/api/v1/audio/nonexistent/spectrogram", nil)
	specW := httptest.NewRecorder()
	server.ServeHTTP(specW, specReq)
	if specW.Code != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent audio, got %d", specW.Code)
	}

	// 3. Audio match for nonexistent audio ID
	matchPayload, _ := json.Marshal(map[string]interface{}{
		"referenceAudioId": "missing-1",
		"candidateAudioId": "missing-2",
	})
	matchReq := httptest.NewRequest(http.MethodPost, "/api/v1/audio/match", bytes.NewReader(matchPayload))
	matchReq.Header.Set("Content-Type", "application/json")
	matchW := httptest.NewRecorder()
	server.ServeHTTP(matchW, matchReq)
	if matchW.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing profiles in match, got %d", matchW.Code)
	}

	// 4. Lost pet audio profile for nonexistent pet
	petReq := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/missing-pet/audio-profile", nil)
	petW := httptest.NewRecorder()
	server.ServeHTTP(petW, petReq)
	if petW.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing pet, got %d", petW.Code)
	}
}
