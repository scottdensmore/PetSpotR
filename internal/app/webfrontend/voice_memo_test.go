package webfrontend

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// Helper to generate minimal PCM WAV bytes for tests
func makeTestWAV(seconds int) []byte {
	var buf bytes.Buffer
	sampleRate := 8000
	channels := 1
	bitsPerSample := 16
	numSamples := seconds * sampleRate
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)
	dataSize := numSamples * blockAlign

	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))

	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	for i := 0; i < numSamples; i++ {
		sample := int16((i % 100) * 200)
		_ = binary.Write(&buf, binary.LittleEndian, sample)
	}

	return buf.Bytes()
}

func createMultipartAudioRequest(url string, fieldName string, fileName string, contentType string, fileBytes []byte) (*http.Request, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	partHeader := make(map[string][]string)
	partHeader["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName)}
	partHeader["Content-Type"] = []string{contentType}

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, bytes.NewReader(fileBytes)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, url, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

func TestVoiceMemoEndpoints(t *testing.T) {
	srv := NewDemoServer()

	// Seed test lost pet and sighting
	petID := "demo-lost-1"
	sightingID := "sight-test-101"

	sightingRecord := domain.PetSightingRecord{
		SightingID:          sightingID,
		LostPetID:           petID,
		ReportedAt:          time.Now().UTC(),
		SightedAt:           time.Now().UTC().Add(-5 * time.Minute),
		LocationDescription: "Near Pike Place",
		Coordinates: &domain.LocationPoint{
			Latitude:  47.6097,
			Longitude: -122.3421,
		},
		Status: domain.SightingStatusActive,
	}
	sBytes, _ := json.Marshal(sightingRecord)
	_ = srv.stateStore.SaveState(context.Background(), store.SightingsCollection, sightingID, sBytes)

	t.Run("POST voice memo success with valid WAV audio", func(t *testing.T) {
		wavData := makeTestWAV(5) // 5 seconds
		url := fmt.Sprintf("/api/v1/lost-pets/%s/sightings/%s/voice-memo", petID, sightingID)
		req, err := createMultipartAudioRequest(url, "audio", "memo.wav", "audio/wav", wavData)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			VoiceMemoURL    string    `json:"voiceMemoUrl"`
			DurationSeconds float64   `json:"durationSeconds"`
			Waveform        []float64 `json:"waveform"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal JSON response: %v", err)
		}

		if resp.VoiceMemoURL != url {
			t.Errorf("voiceMemoUrl = %q, want %q", resp.VoiceMemoURL, url)
		}
		if resp.DurationSeconds < 4.8 || resp.DurationSeconds > 5.2 {
			t.Errorf("durationSeconds = %f, want ~5.0", resp.DurationSeconds)
		}
		if len(resp.Waveform) != 30 {
			t.Errorf("waveform length = %d, want 30", len(resp.Waveform))
		}

		// Verify GET returns the audio stream
		getReq := httptest.NewRequest(http.MethodGet, url, nil)
		getRec := httptest.NewRecorder()
		srv.ServeHTTP(getRec, getReq)

		if getRec.Code != http.StatusOK {
			t.Fatalf("GET voice memo expected 200 OK, got %d", getRec.Code)
		}
		if contentType := getRec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "audio/wav") {
			t.Errorf("GET Content-Type = %q, want audio/wav", contentType)
		}
		if acceptRanges := getRec.Header().Get("Accept-Ranges"); acceptRanges != "bytes" {
			t.Errorf("GET Accept-Ranges = %q, want bytes", acceptRanges)
		}
		if len(getRec.Body.Bytes()) != len(wavData) {
			t.Errorf("GET body length = %d, want %d", len(getRec.Body.Bytes()), len(wavData))
		}
	})

	t.Run("POST rejects audio exceeding 15 seconds", func(t *testing.T) {
		wavData := makeTestWAV(18) // 18 seconds
		url := fmt.Sprintf("/api/v1/lost-pets/%s/sightings/%s/voice-memo", petID, sightingID)
		req, err := createMultipartAudioRequest(url, "audio", "memo.wav", "audio/wav", wavData)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request for >15s audio, got %d", rec.Code)
		}
	})

	t.Run("POST rejects invalid MIME type", func(t *testing.T) {
		url := fmt.Sprintf("/api/v1/lost-pets/%s/sightings/%s/voice-memo", petID, sightingID)
		req, err := createMultipartAudioRequest(url, "audio", "notes.txt", "text/plain", []byte("hello world"))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request for text/plain, got %d", rec.Code)
		}
	})

	t.Run("POST returns 404 for unknown pet", func(t *testing.T) {
		wavData := makeTestWAV(2)
		url := "/api/v1/lost-pets/non-existent-pet/sightings/sight-1/voice-memo"
		req, err := createMultipartAudioRequest(url, "audio", "memo.wav", "audio/wav", wavData)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}

		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for unknown pet, got %d", rec.Code)
		}
	})

	t.Run("GET returns 404 for missing voice memo", func(t *testing.T) {
		url := fmt.Sprintf("/api/v1/lost-pets/%s/sightings/missing-sighting-id/voice-memo", petID)
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for missing voice memo, got %d", rec.Code)
		}
	})
}
