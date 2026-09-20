package audio_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/audio"
)

// Helper to generate a minimal valid PCM WAV byte slice
func createTestWAV(numSamples int, sampleRate int, channels int, bitsPerSample int) []byte {
	var buf bytes.Buffer
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)
	dataSize := numSamples * blockAlign

	// RIFF header
	buf.WriteString("RIFF")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(36+dataSize))
	buf.WriteString("WAVE")

	// fmt subchunk
	buf.WriteString("fmt ")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(16)) // Subchunk1Size (16 for PCM)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(1))  // AudioFormat (1 for PCM)
	_ = binary.Write(&buf, binary.LittleEndian, uint16(channels))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(&buf, binary.LittleEndian, uint16(bitsPerSample))

	// data subchunk
	buf.WriteString("data")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(dataSize))

	// sample data (sine-like alternation)
	for i := 0; i < numSamples*channels; i++ {
		sample := int16((i % 100) * 300)
		_ = binary.Write(&buf, binary.LittleEndian, sample)
	}

	return buf.Bytes()
}

func TestValidateAudioHeader(t *testing.T) {
	// Valid WAV
	wavData := createTestWAV(8000, 8000, 1, 16) // 1 second WAV
	mime, err := audio.DetectMIMEType(wavData)
	if err != nil {
		t.Fatalf("unexpected error detecting WAV MIME: %v", err)
	}
	if mime != audio.MIMEAudioWAV {
		t.Errorf("DetectMIMEType(wav) = %s, want %s", mime, audio.MIMEAudioWAV)
	}

	// Valid WebM
	webmHeader := []byte{0x1A, 0x45, 0xDF, 0xA3, 0x93, 0x42, 0x82, 0x88, 0x6D, 0x61, 0x74, 0x72, 0x6F, 0x73, 0x6B, 0x61}
	mime, err = audio.DetectMIMEType(webmHeader)
	if err != nil {
		t.Fatalf("unexpected error detecting WebM MIME: %v", err)
	}
	if mime != audio.MIMEAudioWebM {
		t.Errorf("DetectMIMEType(webm) = %s, want %s", mime, audio.MIMEAudioWebM)
	}

	// Valid Ogg
	oggHeader := []byte("OggS\x00\x02\x00\x00\x00\x00\x00\x00\x00\x00")
	mime, err = audio.DetectMIMEType(oggHeader)
	if err != nil {
		t.Fatalf("unexpected error detecting Ogg MIME: %v", err)
	}
	if mime != audio.MIMEAudioOgg {
		t.Errorf("DetectMIMEType(ogg) = %s, want %s", mime, audio.MIMEAudioOgg)
	}

	// Valid MP4
	mp4Header := []byte("\x00\x00\x00\x20ftypmp42\x00\x00\x00\x00isommp42")
	mime, err = audio.DetectMIMEType(mp4Header)
	if err != nil {
		t.Fatalf("unexpected error detecting MP4 MIME: %v", err)
	}
	if mime != audio.MIMEAudioMP4 {
		t.Errorf("DetectMIMEType(mp4) = %s, want %s", mime, audio.MIMEAudioMP4)
	}

	// Invalid header
	_, err = audio.DetectMIMEType([]byte("NOT_AN_AUDIO_FILE"))
	if !errors.Is(err, audio.ErrInvalidAudioFormat) {
		t.Errorf("expected ErrInvalidAudioFormat, got %v", err)
	}
}

func TestAudioSizeBounds(t *testing.T) {
	// Empty audio
	_, err := audio.ProcessVoiceMemo([]byte{}, "", 0)
	if !errors.Is(err, audio.ErrEmptyAudio) {
		t.Errorf("expected ErrEmptyAudio, got %v", err)
	}

	// Too large audio (> 5MB)
	tooLarge := make([]byte, audio.MaxAudioBytes+1)
	copy(tooLarge, []byte("RIFF"))
	_, err = audio.ProcessVoiceMemo(tooLarge, "audio/wav", 0)
	if !errors.Is(err, audio.ErrAudioTooLarge) {
		t.Errorf("expected ErrAudioTooLarge, got %v", err)
	}
}

func TestDurationBounds(t *testing.T) {
	// WAV of 16 seconds (16 * 8000 samples at 8000 Hz)
	longWAV := createTestWAV(16*8000, 8000, 1, 16)
	_, err := audio.ProcessVoiceMemo(longWAV, "audio/wav", 0)
	if !errors.Is(err, audio.ErrAudioTooLong) {
		t.Errorf("expected ErrAudioTooLong for 16s WAV, got %v", err)
	}

	// WebM with declared duration > 15s
	webmHeader := append([]byte{0x1A, 0x45, 0xDF, 0xA3}, make([]byte, 100)...)
	_, err = audio.ProcessVoiceMemo(webmHeader, "audio/webm", 16.5)
	if !errors.Is(err, audio.ErrAudioTooLong) {
		t.Errorf("expected ErrAudioTooLong for 16.5s client duration, got %v", err)
	}

	// Valid WAV of 5 seconds
	validWAV := createTestWAV(5*8000, 8000, 1, 16)
	memo, err := audio.ProcessVoiceMemo(validWAV, "audio/wav", 0)
	if err != nil {
		t.Fatalf("unexpected error processing valid WAV: %v", err)
	}
	if memo.DurationSeconds < 4.9 || memo.DurationSeconds > 5.1 {
		t.Errorf("expected ~5.0 duration, got %f", memo.DurationSeconds)
	}
}

func TestWaveformGeneration(t *testing.T) {
	validWAV := createTestWAV(5*8000, 8000, 1, 16)
	memo, err := audio.ProcessVoiceMemo(validWAV, "audio/wav", 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(memo.Waveform) != audio.WaveformSampleCount {
		t.Fatalf("expected %d waveform samples, got %d", audio.WaveformSampleCount, len(memo.Waveform))
	}

	for i, sample := range memo.Waveform {
		if sample < 0.0 || sample > 1.0 {
			t.Errorf("sample %d out of normalized range [0.0, 1.0]: %f", i, sample)
		}
	}
}

func TestIsAllowedMIMEType(t *testing.T) {
	allowed := []string{"audio/wav", "audio/x-wav", "audio/webm", "audio/ogg", "audio/opus", "audio/mp4", "audio/x-m4a", "audio/aac", "audio/webm;codecs=opus"}
	for _, m := range allowed {
		if !audio.IsAllowedMIMEType(m) {
			t.Errorf("expected %s to be allowed", m)
		}
	}

	disallowed := []string{"video/mp4", "image/png", "application/json", "text/plain", ""}
	for _, m := range disallowed {
		if audio.IsAllowedMIMEType(m) {
			t.Errorf("expected %s to be disallowed", m)
		}
	}
}

func TestCompressedAudioWaveform(t *testing.T) {
	data := make([]byte, 1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	wf := audio.GenerateWaveform(data, 30)
	if len(wf) != 30 {
		t.Fatalf("expected 30 points, got %d", len(wf))
	}
	for _, v := range wf {
		if v < 0.0 || v > 1.0 {
			t.Errorf("waveform value out of range: %f", v)
		}
	}

	// Zero/silence payload
	silent := make([]byte, 100)
	wfSilent := audio.GenerateWaveform(silent, 30)
	for _, v := range wfSilent {
		if v != 0.0 {
			t.Errorf("expected 0.0 for silence, got %f", v)
		}
	}

	// Empty data
	wfEmpty := audio.GenerateWaveform(nil, 30)
	if len(wfEmpty) != 30 {
		t.Errorf("expected 30 points for nil, got %d", len(wfEmpty))
	}
}

func Test8BitWAV(t *testing.T) {
	wav8 := createTestWAV(8000, 8000, 1, 8)
	memo, err := audio.ProcessVoiceMemo(wav8, "audio/wav", 0)
	if err != nil {
		t.Fatalf("unexpected error parsing 8-bit WAV: %v", err)
	}
	if len(memo.Waveform) != audio.WaveformSampleCount {
		t.Fatalf("expected %d waveform samples, got %d", audio.WaveformSampleCount, len(memo.Waveform))
	}
}

func TestMalformedAudio(t *testing.T) {
	// Malformed WAV header (< 44 bytes)
	malformedWav := []byte("RIFF1234WAVEfmt ")
	memo, err := audio.ProcessVoiceMemo(malformedWav, "audio/wav", 2.0)
	if err != nil {
		t.Fatalf("expected fallback to client duration, got error: %v", err)
	}
	if memo.DurationSeconds != 2.0 {
		t.Errorf("expected 2.0 duration, got %f", memo.DurationSeconds)
	}
}
