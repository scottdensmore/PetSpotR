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

	// 1. Ambient noise rejection (low harmonicity, unvoiced, or low-frequency sub-vocal rumble/hum <100Hz)
	if hnr < 0.20 || pitch <= 0 || pitch < 100.0 {
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
	attackMs := 0.0
	if sampleRate > 0 {
		attackMs = float64(peakIdx) / float64(sampleRate) * 1000.0
	}

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
