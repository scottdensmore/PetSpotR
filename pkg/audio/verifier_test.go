package audio_test

import (
	"context"
	"testing"

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

func TestVerifyAcousticWithOllama_NonPet(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `{"isPet": false, "category": "CAR_ALARM", "confidence": 0.90}`,
		Done:     true,
	}, nil)

	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 800.0,
		HarmonicRatio:   0.60,
	}
	cat, conf, err := audio.VerifyAcousticWithOllama(context.Background(), client, domain.VocalizationCanineBark, 0.70, vp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != domain.VocalizationAmbientNoise {
		t.Errorf("expected AMBIENT_NOISE for non-pet, got %s", cat)
	}
	// Expected confidence = 0.70 * 0.5 = 0.35
	if conf != 0.35 {
		t.Errorf("expected confidence 0.35, got %f", conf)
	}
}

func TestVerifyAcousticWithOllama_Meow(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `{"isPet": true, "category": "FELINE_MEOW", "confidence": 0.92}`,
		Done:     true,
	}, nil)

	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 500.0,
		HarmonicRatio:   0.75,
	}
	cat, conf, err := audio.VerifyAcousticWithOllama(context.Background(), client, domain.VocalizationCanineBark, 0.70, vp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != domain.VocalizationFelineMeow {
		t.Errorf("expected FELINE_MEOW, got %s", cat)
	}
	// Expected confidence = round((0.70 + 0.92) / 2.0 * 100) / 100 = 0.81
	if conf != 0.81 {
		t.Errorf("expected confidence 0.81, got %f", conf)
	}
}

func TestVerifyAcousticWithOllama_Distress(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `{"isPet": true, "category": "ANIMAL_DISTRESS", "confidence": 0.88}`,
		Done:     true,
	}, nil)

	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 1100.0,
		HarmonicRatio:   0.65,
	}
	cat, conf, err := audio.VerifyAcousticWithOllama(context.Background(), client, domain.VocalizationCanineBark, 0.60, vp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != domain.VocalizationDistressYowl {
		t.Errorf("expected ANIMAL_DISTRESS, got %s", cat)
	}
	// Expected confidence = round((0.60 + 0.88) / 2.0 * 100) / 100 = 0.74
	if conf != 0.74 {
		t.Errorf("expected confidence 0.74, got %f", conf)
	}
}

func TestVerifyAcousticWithOllama_MalformedJSON(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `not valid json at all`,
		Done:     true,
	}, nil)

	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 400.0,
		HarmonicRatio:   0.70,
	}
	cat, conf, err := audio.VerifyAcousticWithOllama(context.Background(), client, domain.VocalizationCanineBark, 0.85, vp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != domain.VocalizationCanineBark || conf != 0.85 {
		t.Errorf("expected fallback on malformed response, got %s, %f", cat, conf)
	}
}

func TestVerifyAcousticWithOllama_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `{"isPet": true, "category": "CANINE_BARK", "confidence": 0.95}`,
		Done:     true,
	}, nil)

	vp := domain.AcousticVoiceprint{
		DominantPitchHz: 400.0,
		HarmonicRatio:   0.70,
	}
	cat, conf, err := audio.VerifyAcousticWithOllama(ctx, client, domain.VocalizationCanineBark, 0.85, vp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cat != domain.VocalizationCanineBark || conf != 0.85 {
		t.Errorf("expected original classification on fallback, got %s, %f", cat, conf)
	}
}
