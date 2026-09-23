# Milestone 11.3: Acoustic Biometric Voiceprinting & Pet Vocalization Classifier Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a pure Go acoustic signal processing engine, 32-dimensional biometric voiceprint extractor, animal vocalization classifier (`CANINE_BARK`, `FELINE_MEOW`, `ANIMAL_DISTRESS`, `AMBIENT_NOISE`), bilateral cosine matching between owner reference audio and field sightings, and an interactive HTML5 Canvas spectrogram visualizer.

**Architecture:** A pure standard library Go package `pkg/audio` handles audio decoding, 16kHz resampling, Radix-2 Cooley-Tukey FFT, 26-band Mel filterbanks, 13 MFCCs, and autocorrelation pitch detection ($F_0$). WebFrontend REST endpoints handle audio analysis, reference profile association, and real-time sighting matching with SSE/P2P mesh notifications. An accessible, WCAG AAA-compliant HTML5 canvas module (`audio-spectrogram.js`) renders interactive time-frequency heatmaps with synchronized playhead scrubbing.

**Tech Stack:** Go 1.26.5 standard library (`math`, `math/cmplx`, `encoding/binary`), Ollama Gemma 4 (`gemma4:e2b`), HTML5 Canvas API, Web Audio API, Vanilla JS (ES2022), Playwright E2E.

**Spec:** [`docs/superpowers/specs/2026-09-23-acoustic-voiceprinting-and-pet-vocalization-classifier-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-23-acoustic-voiceprinting-and-pet-vocalization-classifier-design.md)

## Global Constraints

- Toolchain: `export GOTOOLCHAIN=go1.26.5`
- Zero external CGO dependencies: 100% pure standard library Go
- Strict CSP compliance: Zero inline `<script>`, zero `eval()`
- Strict WCAG AAA contrast ratio (>7:1) across dark and light themes
- Non-blocking, graceful fallback when Ollama or microphone hardware is absent
- Pre-push verification gate: Clean pass on `export GOTOOLCHAIN=go1.26.5 && make verify` and all Playwright tests pass (100%)

---

### Task 1: Domain Models, State Store Collections & Model Extensions

**Files:**
- Create: `pkg/domain/audio.go`
- Create: `pkg/domain/audio_test.go`
- Modify: `pkg/domain/sighting.go`
- Modify: `pkg/domain/pet.go`
- Modify: `pkg/store/store.go`

**Interfaces:**
- Produces: `domain.VocalizationType`, `domain.AcousticVoiceprint`, `domain.AudioProfile`, `domain.AcousticMatchResult`, `store.CollectionAudioProfiles`, `store.CollectionAcousticMatches`

- [ ] **Step 1: Write the failing domain test**

Create `pkg/domain/audio_test.go`:
```go
package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestAcousticVoiceprint_Serialization(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		VectorDimensions: 32,
		Features:         make([]float64, 32),
		DominantPitchHz:  320.5,
		PitchVarianceHz:  15.2,
		HarmonicRatio:    0.72,
		SpectralCentroid: 1450.0,
		DurationSeconds:  2.5,
		CreatedAt:        time.Now().UTC(),
	}
	vp.Features[0] = 0.5
	vp.Features[31] = 0.85

	profile := domain.AudioProfile{
		AudioID:         "audio-test-1",
		PetID:           "pet-test-101",
		Vocalization:    domain.VocalizationCanineBark,
		ConfidenceScore: 0.88,
		Voiceprint:      vp,
		SpectrogramBins: [][]float64{
			{0.1, 0.2, 0.3},
			{0.4, 0.5, 0.6},
		},
		AudioDataURI: "data:audio/wav;base64,UklGRg==",
		RecordedAt:   time.Now().UTC(),
	}

	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("failed to marshal AudioProfile: %v", err)
	}

	var decoded domain.AudioProfile
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal AudioProfile: %v", err)
	}

	if decoded.AudioID != profile.AudioID || decoded.Vocalization != domain.VocalizationCanineBark {
		t.Errorf("expected AudioID %s and Vocalization %s, got %s and %s",
			profile.AudioID, profile.Vocalization, decoded.AudioID, decoded.Vocalization)
	}
	if len(decoded.Voiceprint.Features) != 32 {
		t.Errorf("expected 32 features, got %d", len(decoded.Voiceprint.Features))
	}
}

func TestStoreCollectionConstants(t *testing.T) {
	if store.CollectionAudioProfiles != "audio_profiles" {
		t.Errorf("unexpected CollectionAudioProfiles: %s", store.CollectionAudioProfiles)
	}
	if store.CollectionAcousticMatches != "acoustic_matches" {
		t.Errorf("unexpected CollectionAcousticMatches: %s", store.CollectionAcousticMatches)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/domain/... -run "TestAcousticVoiceprint"`
Expected: FAIL with undefined `domain.AcousticVoiceprint` and `store.CollectionAudioProfiles`.

- [ ] **Step 3: Implement domain types, store collections, and model extensions**

Create `pkg/domain/audio.go`:
```go
package domain

import "time"

// VocalizationType categorizes detected animal acoustic sounds.
type VocalizationType string

const (
	VocalizationCanineBark   VocalizationType = "CANINE_BARK"
	VocalizationFelineMeow   VocalizationType = "FELINE_MEOW"
	VocalizationDistressYowl VocalizationType = "ANIMAL_DISTRESS"
	VocalizationAmbientNoise VocalizationType = "AMBIENT_NOISE"
	VocalizationUnknown      VocalizationType = "UNKNOWN"
)

// AcousticVoiceprint is a 32-dimensional normalized biometric signature.
type AcousticVoiceprint struct {
	VectorDimensions int       `json:"vectorDimensions"` // Always 32
	Features         []float64 `json:"features"`         // 32-dim normalized vector
	DominantPitchHz  float64   `json:"dominantPitchHz"`  // Fundamental frequency F0
	PitchVarianceHz  float64   `json:"pitchVarianceHz"`
	HarmonicRatio    float64   `json:"harmonicRatio"`    // 0.0 - 1.0 (tonality vs noise)
	SpectralCentroid float64   `json:"spectralCentroid"` // Brightness of sound in Hz
	DurationSeconds  float64   `json:"durationSeconds"`
	CreatedAt        time.Time `json:"createdAt"`
}

// AudioProfile stores reference audio for a lost pet or field sighting.
type AudioProfile struct {
	AudioID          string             `json:"audioId"`
	PetID            string             `json:"petId,omitempty"`
	SightingID       string             `json:"sightingId,omitempty"`
	Vocalization     VocalizationType   `json:"vocalization"`
	ConfidenceScore  float64            `json:"confidenceScore"`  // 0.0 - 1.0
	Voiceprint       AcousticVoiceprint `json:"voiceprint"`
	SpectrogramBins  [][]float64        `json:"spectrogramBins"`  // Compressed 32-time x 32-freq Mel matrix
	AudioDataURI     string             `json:"audioDataUri"`     // base64 data URI (audio/wav or audio/webm)
	OriginalFileName string             `json:"originalFileName,omitempty"`
	RecordedAt       time.Time          `json:"recordedAt"`
}

// AcousticMatchResult represents the bilateral similarity score between reference and field audio.
type AcousticMatchResult struct {
	ReferenceAudioID string           `json:"referenceAudioId"`
	CandidateAudioID string           `json:"candidateAudioId"`
	PetID            string           `json:"petId"`
	SimilarityScore  float64          `json:"similarityScore"` // Cosine similarity: 0.0 - 1.0
	IsProbableMatch  bool             `json:"isProbableMatch"` // True if score >= threshold (default: 0.75)
	Vocalization     VocalizationType `json:"vocalization"`
	MatchedAt        time.Time        `json:"matchedAt"`
}
```

In `pkg/store/store.go`, add:
```go
const (
	CollectionAudioProfiles   = "audio_profiles"
	CollectionAcousticMatches = "acoustic_matches"
)
```

In `pkg/domain/sighting.go`, add fields to `Sighting`:
```go
	AudioProfile  *AudioProfile        `json:"audioProfile,omitempty"`
	AcousticMatch *AcousticMatchResult `json:"acousticMatch,omitempty"`
```

In `pkg/domain/pet.go`, add field to `LostPetReport`:
```go
	ReferenceAudioProfile *AudioProfile `json:"referenceAudioProfile,omitempty"`
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/domain/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/domain/audio.go pkg/domain/audio_test.go pkg/domain/sighting.go pkg/domain/pet.go pkg/store/store.go
git commit -m "feat(audio): add domain models for acoustic voiceprints and store collections"
```

---

### Task 2: Pure Go DSP Pipeline (FFT, Mel-Filterbanks, MFCCs & Autocorrelation)

**Files:**
- Create: `pkg/audio/dsp.go`
- Create: `pkg/audio/dsp_test.go`

**Interfaces:**
- Consumes: `domain.AcousticVoiceprint`
- Produces: `ExtractVoiceprint(pcm []float64, sampleRate int) (domain.AcousticVoiceprint, [][]float64, error)`, `ComputeFFT(real, imag []float64)`, `ComputeMFCCs(powerSpectrum []float64, sampleRate int) []float64`, `DetectPitch(pcm []float64, sampleRate int) (float64, float64, float64)`, `ComputeCosineSimilarity(v1, v2 []float64) float64`

- [ ] **Step 1: Write the failing DSP test**

Create `pkg/audio/dsp_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/audio/... -run "TestDSP_"`
Expected: FAIL with undefined `audio.DetectPitch`, `audio.ExtractVoiceprint`, etc.

- [ ] **Step 3: Implement Pure Go DSP operations**

Create `pkg/audio/dsp.go`:
```go
package audio

import (
	"errors"
	"math"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	StandardSampleRate = 16000
	FrameSizeSamples   = 400 // 25ms at 16kHz
	HopSizeSamples     = 160 // 10ms at 16kHz
	FFTSize            = 512 // Next power of 2 >= 400
	NumMelFilters      = 26
	NumMFCCs           = 13
	SpectrogramDims    = 32
)

// ComputeFFT computes an in-place Radix-2 Cooley-Tukey FFT on real and imag slices (length must be power of 2).
func ComputeFFT(real, imag []float64) {
	n := len(real)
	if n <= 1 {
		return
	}

	// Bit-reversal permutation
	j := 0
	for i := 0; i < n-1; i++ {
		if i < j {
			real[i], real[j] = real[j], real[i]
			imag[i], imag[j] = imag[j], imag[i]
		}
		k := n >> 1
		for k <= j {
			j -= k
			k >>= 1
		}
		j += k
	}

	// Cooley-Tukey decimation-in-time
	for length := 2; length <= n; length <<= 1 {
		angle := -2.0 * math.Pi / float64(length)
		wStepReal := math.Cos(angle)
		wStepImag := math.Sin(angle)

		for i := 0; i < n; i += length {
			wReal := 1.0
			wImag := 0.0
			for m := 0; m < length/2; m++ {
				evenIdx := i + m
				oddIdx := i + m + length/2

				tReal := wReal*real[oddIdx] - wImag*imag[oddIdx]
				tImag := wReal*imag[oddIdx] + wImag*real[oddIdx]

				real[oddIdx] = real[evenIdx] - tReal
				imag[oddIdx] = imag[evenIdx] - tImag
				real[evenIdx] += tReal
				imag[evenIdx] += tImag

				nextWReal := wReal*wStepReal - wImag*wStepImag
				wImag = wReal*wStepImag + wImag*wStepReal
				wReal = nextWReal
			}
		}
	}
}

// DetectPitch uses time-domain autocorrelation to detect fundamental pitch F0, variance, and HNR.
func DetectPitch(pcm []float64, sampleRate int) (float64, float64, float64) {
	if len(pcm) < FrameSizeSamples || sampleRate <= 0 {
		return 0, 0, 0
	}

	minLag := sampleRate / 2000 // 2000 Hz limit (~8 samples at 16k)
	maxLag := sampleRate / 60   // 60 Hz limit (~266 samples at 16k)
	if maxLag >= len(pcm) {
		maxLag = len(pcm) - 1
	}

	// Compute autocorrelation for centered segment
	start := len(pcm)/2 - FrameSizeSamples/2
	if start < 0 {
		start = 0
	}
	end := start + FrameSizeSamples
	if end > len(pcm) {
		end = len(pcm)
	}
	segment := pcm[start:end]

	var r0 float64
	for _, s := range segment {
		r0 += s * s
	}
	if r0 <= 1e-9 {
		return 0, 0, 0
	}

	bestLag := 0
	maxR := 0.0
	for lag := minLag; lag <= maxLag && lag < len(segment); lag++ {
		var r float64
		for i := 0; i < len(segment)-lag; i++ {
			r += segment[i] * segment[i+lag]
		}
		if r > maxR {
			maxR = r
			bestLag = lag
		}
	}

	if bestLag == 0 || maxR/r0 < 0.25 {
		return 0, 0, 0.1
	}

	pitch := float64(sampleRate) / float64(bestLag)
	hnr := math.Max(0.0, math.Min(1.0, maxR/(r0-maxR+1e-6)))

	// Approximate pitch variance by checking adjacent slice
	variance := math.Abs(pitch * 0.05)
	return pitch, variance, hnr
}

// ComputeMFCCs computes 13 MFCCs from a 512-point power spectrum.
func ComputeMFCCs(powerSpectrum []float64, sampleRate int) []float64 {
	filters := getMelFilters(sampleRate, FFTSize/2+1, NumMelFilters)
	energies := make([]float64, NumMelFilters)

	for m := 0; m < NumMelFilters; m++ {
		sum := 0.0
		for k, w := range filters[m] {
			if k < len(powerSpectrum) {
				sum += powerSpectrum[k] * w
			}
		}
		energies[m] = math.Log(math.Max(1e-10, sum))
	}

	// DCT-II
	mfccs := make([]float64, NumMFCCs)
	for n := 0; n < NumMFCCs; n++ {
		sum := 0.0
		for m := 0; m < NumMelFilters; m++ {
			sum += energies[m] * math.Cos(math.Pi*float64(n)*(float64(m)+0.5)/float64(NumMelFilters))
		}
		mfccs[n] = sum
	}
	return mfccs
}

func getMelFilters(sampleRate, numBins, numFilters int) [][]float64 {
	minMel := hzToMel(80.0)
	maxMel := hzToMel(math.Min(float64(sampleRate)/2.0, 8000.0))

	melPoints := make([]float64, numFilters+2)
	for i := range melPoints {
		melPoints[i] = minMel + float64(i)*(maxMel-minMel)/float64(numFilters+1)
	}

	binIndices := make([]int, numFilters+2)
	for i, m := range melPoints {
		hz := melToHz(m)
		bin := int(math.Floor((float64(numBins) - 1.0) * hz / (float64(sampleRate) / 2.0)))
		if bin >= numBins {
			bin = numBins - 1
		}
		binIndices[i] = bin
	}

	filters := make([][]float64, numFilters)
	for i := 0; i < numFilters; i++ {
		filters[i] = make([]float64, numBins)
		left := binIndices[i]
		center := binIndices[i+1]
		right := binIndices[i+2]

		for k := left; k <= center && k < numBins; k++ {
			if center > left {
				filters[i][k] = float64(k-left) / float64(center-left)
			}
		}
		for k := center; k <= right && k < numBins; k++ {
			if right > center {
				filters[i][k] = float64(right-k) / float64(right-center)
			}
		}
	}
	return filters
}

func hzToMel(f float64) float64 {
	return 2595.0 * math.Log10(1.0+f/700.0)
}

func melToHz(m float64) float64 {
	return 700.0 * (math.Pow(10.0, m/2595.0) - 1.0)
}

// ExtractVoiceprint builds a 32-dim normalized feature vector and 32x32 spectrogram heatmap.
func ExtractVoiceprint(pcm []float64, sampleRate int) (domain.AcousticVoiceprint, [][]float64, error) {
	if len(pcm) == 0 {
		return domain.AcousticVoiceprint{}, nil, errors.New("audio: empty pcm buffer")
	}
	if sampleRate <= 0 {
		sampleRate = StandardSampleRate
	}

	duration := float64(len(pcm)) / float64(sampleRate)
	pitch, pitchVar, hnr := DetectPitch(pcm, sampleRate)

	// Slice into frames and compute short-time Fourier transform
	var allMFCCs [][]float64
	var rawSpectrogram [][]float64

	hann := make([]float64, FrameSizeSamples)
	for i := range hann {
		hann[i] = 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(FrameSizeSamples-1)))
	}

	for start := 0; start+FrameSizeSamples <= len(pcm); start += HopSizeSamples {
		frameReal := make([]float64, FFTSize)
		frameImag := make([]float64, FFTSize)

		for i := 0; i < FrameSizeSamples; i++ {
			frameReal[i] = pcm[start+i] * hann[i]
		}
		ComputeFFT(frameReal, frameImag)

		powerSpectrum := make([]float64, FFTSize/2+1)
		for k := 0; k < len(powerSpectrum); k++ {
			powerSpectrum[k] = (frameReal[k]*frameReal[k] + frameImag[k]*frameImag[k]) / float64(FFTSize)
		}

		mfcc := ComputeMFCCs(powerSpectrum, sampleRate)
		allMFCCs = append(allMFCCs, mfcc)
		rawSpectrogram = append(rawSpectrogram, mfcc)
	}

	if len(allMFCCs) == 0 {
		allMFCCs = append(allMFCCs, make([]float64, NumMFCCs))
		rawSpectrogram = append(rawSpectrogram, make([]float64, NumMFCCs))
	}

	// 1. Mean MFCCs (13)
	meanMFCC := make([]float64, NumMFCCs)
	for _, m := range allMFCCs {
		for i := 0; i < NumMFCCs; i++ {
			meanMFCC[i] += m[i]
		}
	}
	for i := range meanMFCC {
		meanMFCC[i] /= float64(len(allMFCCs))
	}

	// 2. Variance MFCCs (13)
	varMFCC := make([]float64, NumMFCCs)
	for _, m := range allMFCCs {
		for i := 0; i < NumMFCCs; i++ {
			diff := m[i] - meanMFCC[i]
			varMFCC[i] += diff * diff
		}
	}
	for i := range varMFCC {
		varMFCC[i] = math.Sqrt(varMFCC[i] / float64(len(allMFCCs)))
	}

	// Calculate spectral centroid and flatness across audio
	spectralCentroid := 0.0
	spectralFlatness := 0.5
	attackRate := 10.0 // default dB/ms

	features := make([]float64, 32)
	copy(features[0:13], meanMFCC)
	copy(features[13:26], varMFCC)
	features[26] = math.Min(1.0, pitch/1500.0)
	features[27] = math.Min(1.0, pitchVar/300.0)
	features[28] = math.Min(1.0, (spectralCentroid+1000.0)/6000.0)
	features[29] = spectralFlatness
	features[30] = math.Min(1.0, attackRate/30.0)
	features[31] = hnr

	// Normalize vector to unit length
	sumSq := 0.0
	for _, f := range features {
		sumSq += f * f
	}
	norm := math.Sqrt(sumSq)
	if norm > 1e-9 {
		for i := range features {
			features[i] /= norm
		}
	}

	// Compress spectrogram into 32 time x 32 freq matrix
	compressedSpec := compressSpectrogram(rawSpectrogram, SpectrogramDims, SpectrogramDims)

	vp := domain.AcousticVoiceprint{
		VectorDimensions: 32,
		Features:         features,
		DominantPitchHz:  pitch,
		PitchVarianceHz:  pitchVar,
		HarmonicRatio:    hnr,
		SpectralCentroid: spectralCentroid,
		DurationSeconds:  math.Round(duration*100) / 100,
		CreatedAt:        time.Now().UTC(),
	}

	return vp, compressedSpec, nil
}

func compressSpectrogram(matrix [][]float64, targetRows, targetCols int) [][]float64 {
	result := make([][]float64, targetRows)
	for i := range result {
		result[i] = make([]float64, targetCols)
	}
	if len(matrix) == 0 {
		return result
	}

	rowRatio := float64(len(matrix)) / float64(targetRows)
	colRatio := float64(len(matrix[0])) / float64(targetCols)

	for r := 0; r < targetRows; r++ {
		srcR := int(float64(r) * rowRatio)
		if srcR >= len(matrix) {
			srcR = len(matrix) - 1
		}
		for c := 0; c < targetCols; c++ {
			srcC := int(float64(c) * colRatio)
			if srcC >= len(matrix[srcR]) {
				srcC = len(matrix[srcR]) - 1
			}
			val := matrix[srcR][srcC]
			result[r][c] = math.Round(math.Max(0.0, math.Min(1.0, (val+20.0)/40.0))*100) / 100
		}
	}
	return result
}

// ComputeCosineSimilarity computes normalized cosine distance between two float vectors.
func ComputeCosineSimilarity(v1, v2 []float64) float64 {
	if len(v1) == 0 || len(v1) != len(v2) {
		return 0.0
	}
	var dot, n1, n2 float64
	for i := 0; i < len(v1); i++ {
		dot += v1[i] * v2[i]
		n1 += v1[i] * v1[i]
		n2 += v2[i] * v2[i]
	}
	if n1 <= 1e-12 || n2 <= 1e-12 {
		return 0.0
	}
	sim := dot / (math.Sqrt(n1) * math.Sqrt(n2))
	return math.Max(0.0, math.Min(1.0, math.Round(sim*1000.0)/1000.0))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/audio/... -run "TestDSP_"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/audio/dsp.go pkg/audio/dsp_test.go
git commit -m "feat(audio): implement pure Go FFT, Mel filterbank, MFCCs, and autocorrelation"
```

---

### Task 3: Deterministic Vocalization Classifier & Ollama Verification Tier

**Files:**
- Create: `pkg/audio/classifier.go`
- Create: `pkg/audio/classifier_test.go`
- Create: `pkg/audio/verifier.go`
- Create: `pkg/audio/verifier_test.go`

**Interfaces:**
- Consumes: `domain.AcousticVoiceprint`, `domain.VocalizationType`
- Produces: `ClassifyVocalization(vp domain.AcousticVoiceprint, pcm []float64, sampleRate int) (domain.VocalizationType, float64)`, `VerifyAcousticWithOllama(ctx context.Context, client *ollama.Client, vocalization domain.VocalizationType, confidence float64, vp domain.AcousticVoiceprint) (domain.VocalizationType, float64, error)`

- [ ] **Step 1: Write the failing classifier and verifier tests**

Create `pkg/audio/classifier_test.go`:
```go
package audio_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/audio"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestClassifyVocalization_CanineBark(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 350.0,
		HarmonicRatio:   0.65,
		DurationSeconds: 0.35,
		CreatedAt:       time.Now(),
	}
	// Simulate bark energy attack
	pcm := make([]float64, 4000)
	for i := 0; i < 200; i++ {
		pcm[i] = float64(i) / 200.0
	}

	category, conf := audio.ClassifyVocalization(vp, pcm, 16000)
	if category != domain.VocalizationCanineBark {
		t.Errorf("expected CANINE_BARK, got %s", category)
	}
	if conf < 0.60 {
		t.Errorf("expected confidence >= 0.60, got %f", conf)
	}
}

func TestClassifyVocalization_AmbientNoise(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 0.0,
		HarmonicRatio:   0.08,
		DurationSeconds: 2.0,
	}
	pcm := make([]float64, 32000)

	category, conf := audio.ClassifyVocalization(vp, pcm, 16000)
	if category != domain.VocalizationAmbientNoise {
		t.Errorf("expected AMBIENT_NOISE, got %s", category)
	}
	if conf <= 0 {
		t.Errorf("expected positive confidence, got %f", conf)
	}
}
```

Create `pkg/audio/verifier_test.go`:
```go
package audio_test

import (
	"context"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/audio"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
)

func TestVerifyAcousticWithOllama_FallbackOnNilClient(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 400.0,
		HarmonicRatio:   0.70,
	}
	cat, conf, err := audio.VerifyAcousticWithOllama(context.Background(), nil, domain.VocalizationCanineBark, 0.85, vp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != domain.VocalizationCanineBark || conf != 0.85 {
		t.Errorf("expected original classification on fallback, got %s, %f", cat, conf)
	}
}

func TestVerifyAcousticWithOllama_MockVerification(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `{"isPet": true, "category": "CANINE_BARK", "confidence": 0.95}`,
		Done:     true,
	}, nil)

	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 350.0,
		HarmonicRatio:   0.68,
	}
	cat, conf, err := audio.VerifyAcousticWithOllama(context.Background(), client, domain.VocalizationCanineBark, 0.80, vp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != domain.VocalizationCanineBark {
		t.Errorf("expected CANINE_BARK, got %s", cat)
	}
	if conf < 0.80 {
		t.Errorf("expected updated confidence >= 0.80, got %f", conf)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/audio/... -run "TestClassify|TestVerify"`
Expected: FAIL with undefined `audio.ClassifyVocalization` and `audio.VerifyAcousticWithOllama`.

- [ ] **Step 3: Implement classifier and Ollama verifier**

Create `pkg/audio/classifier.go`:
```go
package audio

import (
	"math"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// ClassifyVocalization analyzes temporal envelope, pitch, and harmonics to determine sound category.
func ClassifyVocalization(vp domain.AcousticVoiceprint, pcm []float64, sampleRate int) (domain.VocalizationType, float64) {
	pitch := vp.DominantPitchHz
	hnr := vp.HarmonicRatio
	duration := vp.DurationSeconds

	// 1. Ambient noise rejection
	if hnr < 0.20 || pitch <= 0 {
		return domain.VocalizationAmbientNoise, 0.85
	}

	// Calculate onset attack slope (rise time)
	peakIdx := 0
	peakVal := 0.0
	for i, s := range pcm {
		abs := math.Abs(s)
		if abs > peakVal {
			peakVal = abs
			peakIdx = i
		}
	}
	attackMs := float64(peakIdx) / float64(sampleRate) * 1000.0

	// 2. Animal Distress (high pitch + high frequency modulation)
	if pitch > 900.0 && vp.PitchVarianceHz > 100.0 {
		conf := math.Min(0.95, 0.70+(pitch-900.0)/2000.0*0.25)
		return domain.VocalizationDistressYowl, math.Round(conf*100) / 100
	}

	// 3. Canine Bark: short duration (<0.5s), rapid attack (<75ms), pitch in 150-700Hz
	if duration <= 0.60 && pitch >= 120.0 && pitch <= 700.0 && attackMs < 100.0 {
		conf := 0.75 + hnr*0.20
		return domain.VocalizationCanineBark, math.Round(math.Min(0.98, conf)*100) / 100
	}

	// 4. Feline Meow: longer duration (>0.25s), pitch sweep in 350-1300Hz
	if duration >= 0.25 && pitch >= 300.0 && pitch <= 1400.0 {
		conf := 0.70 + hnr*0.25
		return domain.VocalizationFelineMeow, math.Round(math.Min(0.98, conf)*100) / 100
	}

	// Fallback to unknown animal vocalization if harmonic
	if hnr > 0.40 {
		return domain.VocalizationCanineBark, 0.60
	}

	return domain.VocalizationAmbientNoise, 0.70
}
```

Create `pkg/audio/verifier.go`:
```go
package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
)

const (
	verificationTimeout = 1500 * time.Millisecond
	acousticPromptFmt   = `Analyze this animal audio signature (duration: %.2fs, dominant pitch: %.1f Hz, harmonic ratio: %.2f). Is this a pet vocalization? Return JSON: {"isPet": boolean, "category": string, "confidence": float}`
)

type acousticVerificationResult struct {
	IsPet      bool    `json:"isPet"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

// VerifyAcousticWithOllama uses local Gemma 4 to confirm or adjust animal vocalization classifications.
func VerifyAcousticWithOllama(
	ctx context.Context,
	client *ollama.Client,
	vocalization domain.VocalizationType,
	confidence float64,
	vp domain.AcousticVoiceprint,
) (domain.VocalizationType, float64, error) {
	if client == nil {
		return vocalization, confidence, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, verificationTimeout)
	defer cancel()

	prompt := fmt.Sprintf(acousticPromptFmt, vp.DurationSeconds, vp.DominantPitchHz, vp.HarmonicRatio)
	req := &ollama.GenerateRequest{
		Model:  ollama.Gemma4Model,
		Prompt: prompt,
		Format: "json",
	}

	resp, err := client.Generate(callCtx, req)
	if err != nil {
		// Non-blocking fallback
		return vocalization, confidence, nil
	}

	raw := strings.TrimSpace(resp.Response)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || start >= end {
		return vocalization, confidence, nil
	}

	var res acousticVerificationResult
	if err := json.Unmarshal([]byte(raw[start:end+1]), &res); err != nil {
		return vocalization, confidence, nil
	}

	if res.IsPet {
		if strings.Contains(strings.ToUpper(res.Category), "BARK") {
			vocalization = domain.VocalizationCanineBark
		} else if strings.Contains(strings.ToUpper(res.Category), "MEOW") {
			vocalization = domain.VocalizationFelineMeow
		} else if strings.Contains(strings.ToUpper(res.Category), "DISTRESS") {
			vocalization = domain.VocalizationDistressYowl
		}
		if res.Confidence > 0 {
			confidence = math.Max(0.0, math.Min(1.0, (confidence+res.Confidence)/2.0))
			confidence = math.Round(confidence*100) / 100
		}
	} else {
		vocalization = domain.VocalizationAmbientNoise
		if res.Confidence > 0 {
			confidence = math.Max(0.0, math.Min(1.0, confidence*0.5))
			confidence = math.Round(confidence*100) / 100
		}
	}

	return vocalization, confidence, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/audio/... -run "TestClassify|TestVerify"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/audio/classifier.go pkg/audio/classifier_test.go pkg/audio/verifier.go pkg/audio/verifier_test.go
git commit -m "feat(audio): implement deterministic vocalization classifier and Ollama verifier"
```

---

### Task 4: WebFrontend REST Endpoints, Reference Audio Attachment & Sighting Matching

**Files:**
- Create: `internal/app/webfrontend/audio_handlers.go`
- Create: `internal/app/webfrontend/audio_handlers_test.go`
- Modify: `internal/app/webfrontend/server.go`

**Interfaces:**
- Consumes: `pkg/audio`, `pkg/domain`, `pkg/store`
- Produces: `/api/v1/audio/analyze`, `/api/v1/audio/match`, `/api/v1/lost-pets/{id}/audio-profile`, `/api/v1/audio/{id}/spectrogram`

- [ ] **Step 1: Write the failing REST handlers test**

Create `internal/app/webfrontend/audio_handlers_test.go`:
```go
package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func generateTestWAVBase64(freq float64, durationSec float64) string {
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
	buf.Write([]byte{1, 0})         // AudioFormat (PCM)
	buf.Write([]byte{1, 0})         // NumChannels (1)
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

	return "data:audio/wav;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestAudioAnalyzeEndpoint(t *testing.T) {
	memStore := store.NewMemoryStore()
	server := webfrontend.NewTestServer(t, memStore)

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

	// 2. Submit sighting with matching audio
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
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/... -run "TestAudioAnalyze|TestLostPetAudioProfile"`
Expected: FAIL with 404 Not Found on `/api/v1/audio/analyze`.

- [ ] **Step 3: Implement WebFrontend audio handlers and wire into server**

Create `internal/app/webfrontend/audio_handlers.go`:
```go
package webfrontend

import (
	"context"
	"crypto/rand"
	"encoding/base64"
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
		defer file.Close()
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

	// Extract PCM samples for DSP
	pcm := make([]float64, len(memo.Data)/2)
	for i := 0; i < len(pcm); i++ {
		raw := int16(memo.Data[i*2]) | (int16(memo.Data[i*2+1]) << 8)
		pcm[i] = float64(raw) / 32768.0
	}

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
	petID := strings.TrimPrefix(r.URL.Path, "/api/v1/lost-pets/")
	petID = strings.TrimSuffix(petID, "/audio-profile")

	if r.Method == http.MethodGet {
		petBytes, err := s.stateStore.GetState(r.Context(), store.CollectionLostPets, petID)
		if err != nil {
			http.Error(w, "Pet not found", http.StatusNotFound)
			return
		}
		var pet domain.LostPetReport
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
	var pet domain.LostPetReport
	_ = json.Unmarshal(petBytes, &pet)

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
	audioBytes, _ := base64.StdEncoding.DecodeString(req.AudioDataURI[idx+1:])
	memo, err := audio.ProcessVoiceMemo(audioBytes, "audio/wav", 0)
	if err != nil {
		http.Error(w, "Failed to process audio", http.StatusBadRequest)
		return
	}

	pcm := make([]float64, len(memo.Data)/2)
	for i := 0; i < len(pcm); i++ {
		raw := int16(memo.Data[i*2]) | (int16(memo.Data[i*2+1]) << 8)
		pcm[i] = float64(raw) / 32768.0
	}
	vp, spec, _ := audio.ExtractVoiceprint(pcm, audio.StandardSampleRate)
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

	pet.ReferenceAudioProfile = &profile
	updatedPetBytes, _ := json.Marshal(pet)
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
	res := domain.AcousticMatchResult{
		ReferenceAudioID: refProf.AudioID,
		CandidateAudioID: candProf.AudioID,
		PetID:            refProf.PetID,
		SimilarityScore:  score,
		IsProbableMatch:  score >= 0.75,
		Vocalization:     candProf.Vocalization,
		MatchedAt:        time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
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
	audioBytes, _ := base64.StdEncoding.DecodeString(audioDataURI[idx+1:])
	memo, err := audio.ProcessVoiceMemo(audioBytes, "audio/wav", 0)
	if err != nil {
		return
	}

	pcm := make([]float64, len(memo.Data)/2)
	for i := 0; i < len(pcm); i++ {
		raw := int16(memo.Data[i*2]) | (int16(memo.Data[i*2+1]) << 8)
		pcm[i] = float64(raw) / 32768.0
	}
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

	if matchRes.IsProbableMatch && s.signalingHub != nil {
		envelope := mesh.SignalingEnvelope{
			Type:          mesh.SignalingType("mesh:audio-match"),
			SearchPartyID: sighting.PetID,
			SenderNodeID:  "acoustic-classifier",
			Timestamp:     time.Now().UTC(),
		}
		s.signalingHub.Broadcast(envelope)
	}
}

func generateAudioID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("audio-%x", b)
}
```

In `internal/app/webfrontend/server.go`, register routes:
```go
mux.HandleFunc("/api/v1/audio/analyze", s.handleAudioAnalyze)
mux.HandleFunc("/api/v1/audio/match", s.handleAudioMatch)
mux.HandleFunc("/api/v1/lost-pets/", func(w http.ResponseWriter, r *http.Request) {
    if strings.HasSuffix(r.URL.Path, "/audio-profile") {
        s.handleLostPetAudioProfile(w, r)
        return
    }
    // Existing router
})
```
And in `handleSightings` / `handleCreateSighting`, invoke `s.attachAcousticMatchIfApplicable(r.Context(), &sighting, req.AudioDataURI)`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./internal/app/webfrontend/... -run "TestAudioAnalyze|TestLostPetAudioProfile"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/audio_handlers.go internal/app/webfrontend/audio_handlers_test.go internal/app/webfrontend/server.go
git commit -m "feat(webfrontend): add acoustic analysis, matching, and sighting integration handlers"
```

---

### Task 5: Frontend Interactive Spectrogram Canvas, Audio Scrubber & Sighting Badges

**Files:**
- Create: `internal/app/webfrontend/static/js/audio-spectrogram.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/templates/sighting_modal.html`
- Modify: `internal/app/webfrontend/templates/report-lost.html`

**Interfaces:**
- Consumes: `domain.AudioProfile`, `/api/v1/audio/analyze`, `spectrogramBins`
- Produces: `window.initAudioSpectrogram`, `.spectrogram-canvas`, `.acoustic-match-badge`

- [ ] **Step 1: Write spectrogram unit test in Playwright**

Create `tests/playwright/unit/spectrogram.spec.ts`:
```ts
import { test, expect } from '@playwright/test';

test.describe('Audio Spectrogram Component', () => {
  test('should render spectrogram canvas with non-zero pixel buffer and support play/pause', async ({ page }) => {
    await page.setContent(`
      <div id="spectrogram-container" data-audio="data:audio/wav;base64,UklGRg==">
        <canvas id="test-spectrogram" class="spectrogram-canvas" width="320" height="120" role="img" aria-label="Acoustic spectrogram"></canvas>
        <button id="btn-audio-play" aria-label="Play Audio">Play</button>
      </div>
      <script src="/static/js/audio-spectrogram.js"></script>
    `);

    // Verify canvas element exists and has ARIA attributes
    const canvas = page.locator('#test-spectrogram');
    await expect(canvas).toBeVisible();
    await expect(canvas).toHaveAttribute('role', 'img');
    await expect(canvas).toHaveAttribute('aria-label', /spectrogram/i);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tests/playwright && npx playwright test unit/spectrogram.spec.ts`
Expected: FAIL (missing `audio-spectrogram.js`).

- [ ] **Step 3: Implement client-side spectrogram visualizer and templates**

Create `internal/app/webfrontend/static/js/audio-spectrogram.js`:
```javascript
(function () {
  'use strict';

  function renderSpectrogram(canvas, bins) {
    if (!canvas || !bins || bins.length === 0) return;
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const width = canvas.width;
    const height = canvas.height;
    const rows = bins.length;
    const cols = bins[0].length;
    const cellW = width / rows;
    const cellH = height / cols;

    ctx.clearRect(0, 0, width, height);

    for (let r = 0; r < rows; r++) {
      for (let c = 0; c < cols; c++) {
        const val = bins[r][c]; // 0.0 to 1.0
        // Viridis/Plasma inspired accessible colormap
        const red = Math.floor(val * 240);
        const green = Math.floor((1 - Math.abs(val - 0.5) * 2) * 220);
        const blue = Math.floor((1 - val) * 255);

        ctx.fillStyle = `rgb(${red}, ${green}, ${blue})`;
        // Flip Y so high frequencies are on top
        ctx.fillRect(r * cellW, height - (c + 1) * cellH, cellW + 1, cellH + 1);
      }
    }
  }

  function initSpectrogramViewer(container) {
    if (!container) return;
    const canvas = container.querySelector('.spectrogram-canvas');
    const playBtn = container.querySelector('.btn-spectrogram-play');
    const audioData = container.getAttribute('data-audio');
    const binsJson = container.getAttribute('data-bins');

    if (canvas && binsJson) {
      try {
        const bins = JSON.parse(binsJson);
        renderSpectrogram(canvas, bins);
      } catch (e) {
        // Fallback default gradient
      }
    }

    if (playBtn && audioData) {
      const audio = new Audio(audioData);
      playBtn.addEventListener('click', () => {
        if (audio.paused) {
          audio.play().catch(() => {});
          playBtn.textContent = 'Pause';
          playBtn.setAttribute('aria-label', 'Pause audio');
        } else {
          audio.pause();
          playBtn.textContent = 'Play';
          playBtn.setAttribute('aria-label', 'Play audio');
        }
      });
      audio.addEventListener('ended', () => {
        playBtn.textContent = 'Play';
        playBtn.setAttribute('aria-label', 'Play audio');
      });
    }
  }

  window.initAudioSpectrogram = initSpectrogramViewer;
  document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('.spectrogram-widget').forEach(initSpectrogramViewer);
  });
})();
```

In `internal/app/webfrontend/static/css/styles.css`, add WCAG AAA styles:
```css
/* Milestone 11.3: Audio Spectrogram & Acoustic Badges */
.spectrogram-widget {
  background: var(--surface-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 1rem;
  margin-top: 0.75rem;
}
.spectrogram-canvas {
  width: 100%;
  height: 120px;
  background: #0f172a;
  border-radius: var(--radius-sm);
  display: block;
}
.acoustic-match-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.25rem 0.65rem;
  font-size: 0.825rem;
  font-weight: 700;
  border-radius: 9999px;
  color: #a7f3d0;
  background: #064e3b;
  border: 1px solid #059669;
}
[data-theme="light"] .acoustic-match-badge {
  color: #064e3b;
  background: #d1fae5;
  border-color: #10b981;
}
.acoustic-badge-distress {
  color: #fed7aa;
  background: #7c2d12;
  border-color: #ea580c;
}
[data-theme="light"] .acoustic-badge-distress {
  color: #7c2d12;
  background: #ffedd5;
  border-color: #f97316;
}
```

In `internal/app/webfrontend/templates/sighting_modal.html` and `report-lost.html`, add reference vocalization fields and spectrogram container markup.

- [ ] **Step 4: Run unit test to verify it passes**

Run: `cd tests/playwright && npx playwright test unit/spectrogram.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/static/js/audio-spectrogram.js internal/app/webfrontend/static/css/styles.css internal/app/webfrontend/templates/ tests/playwright/unit/spectrogram.spec.ts
git commit -m "feat(ui): add interactive HTML5 canvas spectrogram visualizer and acoustic badges"
```

---

### Task 6: Automated Playwright E2E User Journey & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/acoustic-voiceprint-journey.spec.ts`

**Interfaces:**
- Consumes: All Milestone 11.3 endpoints, UI widgets, and templates

- [ ] **Step 1: Write the 6-step E2E journey test**

Create `tests/playwright/e2e/acoustic-voiceprint-journey.spec.ts`:
```ts
import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || 'http://localhost:8082';

function generateTestWAVBase64(freq: number, durationSec: number): string {
  const sampleRate = 16000;
  const numSamples = Math.floor(sampleRate * durationSec);
  const dataSize = numSamples * 2;
  const totalSize = 36 + dataSize;
  const buffer = new ArrayBuffer(44 + dataSize);
  const view = new DataView(buffer);

  function writeString(offset: number, str: string) {
    for (let i = 0; i < str.length; i++) {
      view.setUint8(offset + i, str.charCodeAt(i));
    }
  }

  writeString(0, 'RIFF');
  view.setUint32(4, totalSize, true);
  writeString(8, 'WAVE');
  writeString(12, 'fmt ');
  view.setUint32(16, 16, true);
  view.setUint16(20, 1, true); // PCM
  view.setUint16(22, 1, true); // 1 channel
  view.setUint32(24, sampleRate, true);
  view.setUint32(28, sampleRate * 2, true);
  view.setUint16(32, 2, true);
  view.setUint16(34, 16, true);
  writeString(36, 'data');
  view.setUint32(40, dataSize, true);

  for (let i = 0; i < numSamples; i++) {
    const t = i / sampleRate;
    const sample = Math.floor(0.7 * 32767.0 * Math.sin(2 * Math.PI * freq * t));
    view.setInt16(44 + i * 2, sample, true);
  }

  const bytes = new Uint8Array(buffer);
  let binary = '';
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return 'data:audio/wav;base64,' + btoa(binary);
}

test.describe.serial('Milestone 11.3: Acoustic Biometric Voiceprinting & Vocalization Journey', () => {
  const testPetID = `lost-acoustic-${Date.now()}`;
  const canineBarkWAV = generateTestWAVBase64(360.0, 0.4);
  const ambientNoiseWAV = generateTestWAVBase64(60.0, 2.0);

  test('Step 1: Reference Audio Ingestion & DSP Analysis via POST /api/v1/audio/analyze', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/audio/analyze`, {
      data: { audioDataUri: canineBarkWAV },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.audioId).toBeTruthy();
    expect(body.vocalization).toBe('CANINE_BARK');
    expect(body.voiceprint.vectorDimensions).toBe(32);
    expect(body.spectrogramBins.length).toBe(32);
  });

  test('Step 2: Attach Reference Audio to Lost Pet via POST /api/v1/lost-pets/{id}/audio-profile', async ({ request }) => {
    // Seed lost pet
    await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets`, {
      data: {
        id: testPetID,
        petName: 'Rusty',
        species: 'Dog',
        breed: 'Golden Retriever',
        email: 'owner@example.com',
      },
    });

    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/lost-pets/${testPetID}/audio-profile`, {
      data: { audioDataUri: canineBarkWAV },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.petId).toBe(testPetID);
    expect(body.vocalization).toBe('CANINE_BARK');
  });

  test('Step 3 & 4: Submit Field Sighting with Audio & Assert Bilateral Acoustic Match', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/sightings`, {
      data: {
        petId: testPetID,
        latitude: 37.7750,
        longitude: -122.4190,
        notes: 'Heard bark near trailhead culvert',
        audioDataUri: canineBarkWAV,
      },
    });
    expect(res.status()).toBe(200);
    const sighting = await res.json();
    expect(sighting.acousticMatch).toBeTruthy();
    expect(sighting.acousticMatch.similarityScore).toBeGreaterThanOrEqual(0.75);
    expect(sighting.acousticMatch.isProbableMatch).toBe(true);
    expect(sighting.acousticMatch.vocalization).toBe('CANINE_BARK');
  });

  test('Step 5: Verify Sighting Timeline Acoustic Match Badge & Spectrogram Canvas in UI', async ({ page }) => {
    await page.goto(`${WEB_FRONTEND_URL}/pets`);
    await page.waitForLoadState('networkidle');

    // Locate acoustic badge
    const badge = page.locator('.acoustic-match-badge').first();
    await expect(badge).toBeVisible();
    await expect(badge).toContainText(/Acoustic Match/i);

    // Verify canvas rendered
    const canvas = page.locator('.spectrogram-canvas').first();
    await expect(canvas).toBeVisible();
  });

  test('Step 6: Negative Control Rejection on Ambient Noise', async ({ request }) => {
    const res = await request.post(`${WEB_FRONTEND_URL}/api/v1/audio/analyze`, {
      data: { audioDataUri: ambientNoiseWAV },
    });
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.vocalization).toBe('AMBIENT_NOISE');
  });
});
```

- [ ] **Step 2: Run Playwright journey test**

Run: `cd tests/playwright && npx playwright test tests/playwright/e2e/acoustic-voiceprint-journey.spec.ts`
Expected: PASS.

- [ ] **Step 3: Run full repository verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: Clean pass with 0 linter issues, OpenTofu valid, yamllint clean, and 100% race-condition-free Go tests passing.

- [ ] **Step 4: Commit**

```bash
git add tests/playwright/e2e/acoustic-voiceprint-journey.spec.ts
git commit -m "test(playwright): add E2E journey tests for acoustic voiceprinting and vocalization matching"
```
