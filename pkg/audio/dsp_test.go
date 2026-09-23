package audio_test

import (
	"math"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/audio"
)

func generateSineWave(freqHz float64, durationSec float64, sampleRate int) []float64 {
	numSamples := int(float64(sampleRate) * durationSec)
	samples := make([]float64, numSamples)
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		samples[i] = 0.8 * math.Sin(2*math.Pi*freqHz*t)
	}
	return samples
}

func TestDSP_PitchAutocorrelation(t *testing.T) {
	sampleRate := 16000
	freq := 440.0 // Concert A
	sine := generateSineWave(freq, 0.5, sampleRate)

	detectedPitch, variance, hnr := audio.DetectPitch(sine, sampleRate)
	if math.Abs(detectedPitch-freq) > 15.0 {
		t.Errorf("expected pitch ~440Hz, got %f", detectedPitch)
	}
	if hnr < 0.5 {
		t.Errorf("expected high HNR for pure sine wave, got %f", hnr)
	}
	if variance < 0 {
		t.Errorf("expected non-negative variance, got %f", variance)
	}
}

func TestDSP_VoiceprintExtraction(t *testing.T) {
	sampleRate := 16000
	sine := generateSineWave(350.0, 1.0, sampleRate)

	vp, spec, err := audio.ExtractVoiceprint(sine, sampleRate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vp.VectorDimensions != 32 || len(vp.Features) != 32 {
		t.Fatalf("expected 32 dimensions, got %d", len(vp.Features))
	}
	if len(spec) != 32 {
		t.Errorf("expected 32 time slices in compressed spectrogram, got %d", len(spec))
	}
	if len(spec[0]) != 32 {
		t.Errorf("expected 32 frequency bins, got %d", len(spec[0]))
	}

	// Verify L2 normalization
	sumSq := 0.0
	for _, f := range vp.Features {
		sumSq += f * f
	}
	norm := math.Sqrt(sumSq)
	if math.Abs(norm-1.0) > 0.01 {
		t.Errorf("expected unit L2 norm, got %f", norm)
	}
}

func TestDSP_CosineSimilarity(t *testing.T) {
	v1 := []float64{1.0, 0.0, 0.0}
	v2 := []float64{1.0, 0.0, 0.0}
	v3 := []float64{0.0, 1.0, 0.0}

	simSame := audio.ComputeCosineSimilarity(v1, v2)
	if math.Abs(simSame-1.0) > 1e-6 {
		t.Errorf("expected similarity 1.0, got %f", simSame)
	}

	simOrtho := audio.ComputeCosineSimilarity(v1, v3)
	if math.Abs(simOrtho) > 1e-6 {
		t.Errorf("expected similarity 0.0, got %f", simOrtho)
	}
}

func TestDSP_ComputeFFT(t *testing.T) {
	// Delta impulse at 0: FFT should have real=1.0, imag=0.0 across all bins
	n := 8
	real := make([]float64, n)
	imag := make([]float64, n)
	real[0] = 1.0

	audio.ComputeFFT(real, imag)
	for i := 0; i < n; i++ {
		if math.Abs(real[i]-1.0) > 1e-9 || math.Abs(imag[i]) > 1e-9 {
			t.Errorf("bin %d: expected (1.0, 0.0), got (%f, %f)", i, real[i], imag[i])
		}
	}

	// Edge case: n <= 1
	singleReal := []float64{5.0}
	singleImag := []float64{0.0}
	audio.ComputeFFT(singleReal, singleImag)
	if singleReal[0] != 5.0 || singleImag[0] != 0.0 {
		t.Errorf("expected unchanged single-element FFT")
	}
}

func TestDSP_ComputeMFCCs(t *testing.T) {
	numBins := audio.FFTSize/2 + 1
	powerSpectrum := make([]float64, numBins)
	for i := range powerSpectrum {
		powerSpectrum[i] = 1.0
	}

	mfccs := audio.ComputeMFCCs(powerSpectrum, 16000)
	if len(mfccs) != audio.NumMFCCs {
		t.Fatalf("expected %d MFCCs, got %d", audio.NumMFCCs, len(mfccs))
	}
}

func TestDSP_EdgeCases(t *testing.T) {
	// Empty PCM in ExtractVoiceprint
	_, _, err := audio.ExtractVoiceprint(nil, 16000)
	if err == nil {
		t.Errorf("expected error for nil PCM")
	}

	_, _, err = audio.ExtractVoiceprint([]float64{}, 16000)
	if err == nil {
		t.Errorf("expected error for empty PCM")
	}

	// DetectPitch with insufficient samples
	p, v, h := audio.DetectPitch([]float64{0.1, 0.2}, 16000)
	if p != 0 || v != 0 || h != 0 {
		t.Errorf("expected (0, 0, 0) for short PCM, got (%f, %f, %f)", p, v, h)
	}

	// DetectPitch with negative sampleRate
	p, v, h = audio.DetectPitch(generateSineWave(440, 0.1, 16000), -1)
	if p != 0 || v != 0 || h != 0 {
		t.Errorf("expected (0, 0, 0) for invalid sample rate, got (%f, %f, %f)", p, v, h)
	}

	// DetectPitch with silence (all zeros)
	silence := make([]float64, 1600)
	p, v, h = audio.DetectPitch(silence, 16000)
	if p != 0 || v != 0 || h != 0 {
		t.Errorf("expected (0, 0, 0) for silence, got (%f, %f, %f)", p, v, h)
	}

	// ComputeCosineSimilarity with empty or mismatched vectors
	if sim := audio.ComputeCosineSimilarity(nil, nil); sim != 0.0 {
		t.Errorf("expected 0.0 for nil vectors, got %f", sim)
	}
	if sim := audio.ComputeCosineSimilarity([]float64{1.0}, []float64{1.0, 2.0}); sim != 0.0 {
		t.Errorf("expected 0.0 for mismatched lengths, got %f", sim)
	}
	if sim := audio.ComputeCosineSimilarity([]float64{0.0, 0.0}, []float64{0.0, 0.0}); sim != 0.0 {
		t.Errorf("expected 0.0 for zero vectors, got %f", sim)
	}
}
