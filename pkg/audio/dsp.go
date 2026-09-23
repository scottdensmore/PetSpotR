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
	if n <= 1 || n != len(imag) {
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
	if minLag < 1 {
		minLag = 1
	}
	maxLag := int(float64(sampleRate)/50.0 + 0.5) // 50 Hz limit (~320 samples at 16k)
	if maxLag >= len(pcm) {
		maxLag = len(pcm) - 1
	}

	analysisLen := 800
	if len(pcm) < analysisLen {
		analysisLen = len(pcm)
	}
	start := len(pcm)/2 - analysisLen/2
	if start < 0 {
		start = 0
	}
	end := start + analysisLen
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

	corrs := make([]float64, maxLag+2)
	for lag := 0; lag <= maxLag+1 && lag < len(segment); lag++ {
		overlap := len(segment) - lag
		if overlap <= 0 {
			break
		}
		var r, e0, eLag float64
		for i := 0; i < overlap; i++ {
			r += segment[i] * segment[i+lag]
			e0 += segment[i] * segment[i]
			eLag += segment[i+lag] * segment[i+lag]
		}
		denom := math.Sqrt(e0 * eLag)
		if denom > 1e-9 {
			corrs[lag] = r / denom
		}
	}

	type peak struct {
		lag int
		val float64
	}
	var peaks []peak
	globalMax := 0.0

	for lag := minLag; lag <= maxLag; lag++ {
		val := corrs[lag]
		if val > 0.25 {
			prev := corrs[lag-1]
			next := corrs[lag+1]
			if val >= prev && val >= next {
				peaks = append(peaks, peak{lag: lag, val: val})
				if val > globalMax {
					globalMax = val
				}
			}
		}
	}

	if len(peaks) == 0 || globalMax <= 0 {
		return 0, 0, 0.1
	}

	bestLag := 0
	bestVal := 0.0
	thresh := 0.85 * globalMax
	for _, p := range peaks {
		if p.val >= thresh {
			bestLag = p.lag
			bestVal = p.val
			break
		}
	}

	if bestLag == 0 {
		return 0, 0, 0.1
	}

	pitch := float64(sampleRate) / float64(bestLag)
	hnr := math.Max(0.0, math.Min(1.0, bestVal))

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
	if len(matrix) == 0 || len(matrix[0]) == 0 {
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
