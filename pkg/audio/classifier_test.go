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

func TestClassifyVocalization_FelineMeow(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 450.0,
		HarmonicRatio:   0.75,
		DurationSeconds: 1.2,
		CreatedAt:       time.Now(),
	}
	pcm := make([]float64, 19200)
	// Peak is late (>100ms)
	pcm[3000] = 0.9

	category, conf := audio.ClassifyVocalization(vp, pcm, 16000)
	if category != domain.VocalizationFelineMeow {
		t.Errorf("expected FELINE_MEOW, got %s", category)
	}
	if conf < 0.70 {
		t.Errorf("expected confidence >= 0.70, got %f", conf)
	}
}

func TestClassifyVocalization_AnimalDistress(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 1200.0,
		PitchVarianceHz: 150.0,
		HarmonicRatio:   0.60,
		DurationSeconds: 0.8,
		CreatedAt:       time.Now(),
	}
	pcm := make([]float64, 12800)

	category, conf := audio.ClassifyVocalization(vp, pcm, 16000)
	if category != domain.VocalizationDistressYowl {
		t.Errorf("expected ANIMAL_DISTRESS, got %s", category)
	}
	if conf < 0.70 {
		t.Errorf("expected confidence >= 0.70, got %f", conf)
	}
}

func TestClassifyVocalization_FallbackHarmonic(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 2100.0, // outside bark / meow / distress pitch
		HarmonicRatio:   0.50,
		DurationSeconds: 0.8,
	}
	pcm := make([]float64, 12800)

	category, conf := audio.ClassifyVocalization(vp, pcm, 16000)
	if category != domain.VocalizationCanineBark {
		t.Errorf("expected CANINE_BARK fallback, got %s", category)
	}
	if conf != 0.60 {
		t.Errorf("expected confidence 0.60, got %f", conf)
	}
}

func TestClassifyVocalization_FallbackAmbient(t *testing.T) {
	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 2100.0,
		HarmonicRatio:   0.25, // between 0.20 and 0.40, not matching other rules
		DurationSeconds: 0.8,
	}
	pcm := make([]float64, 12800)

	category, conf := audio.ClassifyVocalization(vp, pcm, 16000)
	if category != domain.VocalizationAmbientNoise {
		t.Errorf("expected AMBIENT_NOISE fallback, got %s", category)
	}
	if conf != 0.70 {
		t.Errorf("expected confidence 0.70, got %f", conf)
	}
}
