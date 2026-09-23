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
