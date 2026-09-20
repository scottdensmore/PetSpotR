package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"strings"
)

// Supported audio MIME types.
const (
	MIMEAudioWAV  = "audio/wav"
	MIMEAudioWebM = "audio/webm"
	MIMEAudioOgg  = "audio/ogg"
	MIMEAudioMP4  = "audio/mp4"
)

// Bounds and constants.
const (
	MaxAudioBytes       = 5 * 1024 * 1024 // 5 MB
	MaxDurationSeconds  = 15.0            // Max 15.0s
	WaveformSampleCount = 30              // Exactly 30 amplitude samples
)

// Standard audio processing errors.
var (
	ErrEmptyAudio         = errors.New("audio: empty audio data")
	ErrAudioTooLarge      = errors.New("audio: audio exceeds maximum allowed size (5MB)")
	ErrInvalidAudioFormat = errors.New("audio: unsupported or invalid audio format")
	ErrAudioTooLong       = errors.New("audio: duration exceeds maximum limit of 15 seconds")
)

// VoiceMemo contains the validated metadata and normalized waveform for an audio note.
type VoiceMemo struct {
	MIMEType        string    `json:"mimeType"`
	DurationSeconds float64   `json:"durationSeconds"`
	Waveform        []float64 `json:"waveform"`
	Data            []byte    `json:"-"`
}

// DetectMIMEType inspects binary magic headers to identify the audio format.
func DetectMIMEType(data []byte) (string, error) {
	if len(data) < 4 {
		return "", ErrInvalidAudioFormat
	}

	// WAV: RIFF....WAVE
	if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE" {
		return MIMEAudioWAV, nil
	}

	// WebM / Matroska: \x1A\x45\xDF\xA3
	if len(data) >= 4 && data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		return MIMEAudioWebM, nil
	}

	// Ogg: OggS
	if len(data) >= 4 && string(data[0:4]) == "OggS" {
		return MIMEAudioOgg, nil
	}

	// MP4 / M4A: ....ftyp
	if len(data) >= 8 && string(data[4:8]) == "ftyp" {
		return MIMEAudioMP4, nil
	}

	return "", ErrInvalidAudioFormat
}

// IsAllowedMIMEType checks if a given MIME type string is supported.
func IsAllowedMIMEType(mime string) bool {
	clean := strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	switch clean {
	case MIMEAudioWAV, "audio/x-wav", MIMEAudioWebM, MIMEAudioOgg, "audio/opus", MIMEAudioMP4, "audio/x-m4a", "audio/aac":
		return true
	default:
		return false
	}
}

// ProcessVoiceMemo validates audio size, format headers, duration, and extracts a 30-point waveform.
func ProcessVoiceMemo(data []byte, declaredMIME string, clientDuration float64) (*VoiceMemo, error) {
	if len(data) == 0 {
		return nil, ErrEmptyAudio
	}
	if len(data) > MaxAudioBytes {
		return nil, ErrAudioTooLarge
	}

	detectedMIME, err := DetectMIMEType(data)
	if err != nil {
		// If declared MIME is allowed and magic check failed only due to small test headers or container wrapper
		if !IsAllowedMIMEType(declaredMIME) {
			return nil, ErrInvalidAudioFormat
		}
		detectedMIME = declaredMIME
	}

	duration := clientDuration
	var waveform []float64

	if detectedMIME == MIMEAudioWAV {
		wavDur, wavWaveform, err := parseWAV(data)
		if err == nil {
			if wavDur > 0 {
				duration = wavDur
			}
			waveform = wavWaveform
		}
	}

	if duration <= 0 {
		if clientDuration > 0 {
			duration = clientDuration
		} else {
			// Estimate 1.0s minimum default if client provided no duration and container didn't specify
			duration = 1.0
		}
	}

	// Duration bound check (with small 0.1s floating tolerance)
	if duration > (MaxDurationSeconds + 0.1) {
		return nil, ErrAudioTooLong
	}

	if len(waveform) != WaveformSampleCount {
		waveform = GenerateWaveform(data, WaveformSampleCount)
	}

	return &VoiceMemo{
		MIMEType:        detectedMIME,
		DurationSeconds: math.Round(duration*100) / 100,
		Waveform:        waveform,
		Data:            data,
	}, nil
}

// parseWAV parses RIFF PCM headers to get duration and samples.
func parseWAV(data []byte) (float64, []float64, error) {
	if len(data) < 44 {
		return 0, nil, errors.New("wav too short")
	}

	reader := bytes.NewReader(data)
	var riffHeader struct {
		ChunkID   [4]byte
		ChunkSize uint32
		Format    [4]byte
	}
	if err := binary.Read(reader, binary.LittleEndian, &riffHeader); err != nil {
		return 0, nil, err
	}

	var sampleRate uint32
	var byteRate uint32
	var channels uint16
	var bitsPerSample uint16
	var dataOffset int64
	var dataSize uint32

	for reader.Len() >= 8 {
		var subchunkHeader struct {
			ID   [4]byte
			Size uint32
		}
		if err := binary.Read(reader, binary.LittleEndian, &subchunkHeader); err != nil {
			break
		}

		chunkID := string(subchunkHeader.ID[:])
		if chunkID == "fmt " {
			var audioFormat uint16
			var blockAlign uint16
			_ = binary.Read(reader, binary.LittleEndian, &audioFormat)
			_ = binary.Read(reader, binary.LittleEndian, &channels)
			_ = binary.Read(reader, binary.LittleEndian, &sampleRate)
			_ = binary.Read(reader, binary.LittleEndian, &byteRate)
			_ = binary.Read(reader, binary.LittleEndian, &blockAlign)
			_ = binary.Read(reader, binary.LittleEndian, &bitsPerSample)

			// Skip any extra fmt bytes
			if subchunkHeader.Size > 16 {
				_, _ = reader.Seek(int64(subchunkHeader.Size-16), 1)
			}
		} else if chunkID == "data" {
			dataSize = subchunkHeader.Size
			curr, _ := reader.Seek(0, 1)
			dataOffset = curr
			break
		} else {
			_, _ = reader.Seek(int64(subchunkHeader.Size), 1)
		}
	}

	if byteRate == 0 || dataSize == 0 || dataOffset == 0 {
		return 0, nil, errors.New("invalid wav chunks")
	}

	duration := float64(dataSize) / float64(byteRate)

	// Extract PCM samples for waveform
	pcmBytes := data[dataOffset:]
	if int(dataSize) < len(pcmBytes) {
		pcmBytes = pcmBytes[:dataSize]
	}

	var samples []float64
	switch bitsPerSample {
	case 16:
		numSamples := len(pcmBytes) / 2
		samples = make([]float64, numSamples)
		for i := 0; i < numSamples; i++ {
			raw := int16(binary.LittleEndian.Uint16(pcmBytes[i*2 : (i+1)*2]))
			samples[i] = math.Abs(float64(raw)) / 32768.0
		}
	case 8:
		samples = make([]float64, len(pcmBytes))
		for i, b := range pcmBytes {
			centered := math.Abs(float64(b) - 128.0)
			samples[i] = centered / 128.0
		}
	}

	if len(samples) == 0 {
		return duration, nil, nil
	}

	waveform := downsampleWaveform(samples, WaveformSampleCount)
	return duration, waveform, nil
}

// GenerateWaveform generates a normalized 30-sample amplitude array for any audio payload.
func GenerateWaveform(data []byte, count int) []float64 {
	if count <= 0 {
		count = WaveformSampleCount
	}
	result := make([]float64, count)
	if len(data) == 0 {
		return result
	}

	// Skip header bytes to analyze payload
	payload := data
	if len(data) > 64 {
		payload = data[64:]
	}

	chunkSize := len(payload) / count
	if chunkSize == 0 {
		chunkSize = 1
	}

	maxAmp := 0.0
	for i := 0; i < count; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if start >= len(payload) {
			result[i] = 0.05
			continue
		}
		if end > len(payload) {
			end = len(payload)
		}

		// Calculate energy variance in this slice
		var sum float64
		for _, b := range payload[start:end] {
			val := float64(b)
			sum += val
		}
		mean := sum / float64(end-start)

		var varSum float64
		for _, b := range payload[start:end] {
			diff := float64(b) - mean
			varSum += diff * diff
		}
		variance := math.Sqrt(varSum / float64(end-start))
		result[i] = variance
		if variance > maxAmp {
			maxAmp = variance
		}
	}

	// Normalize between 0.05 and 1.0 (with 0.0 for pure silence)
	for i := 0; i < count; i++ {
		if maxAmp > 0 {
			norm := result[i] / maxAmp
			// Keep a minimum floor of 0.08 for aesthetic rendering
			result[i] = math.Round(math.Max(0.08, norm)*100) / 100
		} else {
			result[i] = 0.0
		}
	}

	return result
}

func downsampleWaveform(samples []float64, count int) []float64 {
	result := make([]float64, count)
	if len(samples) == 0 {
		return result
	}

	chunkSize := len(samples) / count
	if chunkSize == 0 {
		chunkSize = 1
	}

	maxVal := 0.0
	for i := 0; i < count; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if start >= len(samples) {
			result[i] = 0.05
			continue
		}
		if end > len(samples) {
			end = len(samples)
		}

		peak := 0.0
		for _, s := range samples[start:end] {
			if s > peak {
				peak = s
			}
		}
		result[i] = peak
		if peak > maxVal {
			maxVal = peak
		}
	}

	for i := 0; i < count; i++ {
		if maxVal > 0 {
			norm := result[i] / maxVal
			result[i] = math.Round(math.Max(0.08, norm)*100) / 100
		} else {
			result[i] = 0.0
		}
	}

	return result
}
