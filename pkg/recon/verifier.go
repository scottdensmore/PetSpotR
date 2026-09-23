package recon

import (
	"context"
	"encoding/json"
	"image"
	"math"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
)

const (
	verificationTimeout   = 1500 * time.Millisecond
	defaultClassification = "Thermal Heat Anomaly"
	verificationPrompt    = `Analyze this aerial thermal image crop. Is there a warm-blooded domestic animal (dog, cat, pet) visible? Return JSON: {"isPet": boolean, "confidence": float, "category": string}`
)

type verificationResult struct {
	IsPet      bool    `json:"isPet"`
	Confidence float64 `json:"confidence"`
	Category   string  `json:"category"`
}

// VerifyHotspotWithOllama performs optional multimodal AI verification using the local Ollama instance (gemma4:e2b).
// If Ollama is unreachable, times out after 1.5s, or returns invalid JSON, it gracefully falls back to deterministic
// radiometric scoring with classification "Thermal Heat Anomaly".
func VerifyHotspotWithOllama(
	ctx context.Context,
	ollamaClient *ollama.Client,
	hotspot domain.ThermalHotspot,
	fullImage image.Image,
) (domain.ThermalHotspot, error) {
	if hotspot.Classification == "" {
		hotspot.Classification = defaultClassification
	}

	if ollamaClient == nil {
		return hotspot, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	callCtx, cancel := context.WithTimeout(ctx, verificationTimeout)
	defer cancel()

	// Extract or generate base64 image crop
	rawB64 := ""
	if hotspot.ThumbnailBase64 != "" {
		rawB64 = hotspot.ThumbnailBase64
		if idx := strings.Index(rawB64, ","); idx != -1 && strings.HasPrefix(rawB64, "data:") {
			rawB64 = rawB64[idx+1:]
		}
	} else if fullImage != nil {
		bounds := fullImage.Bounds()
		x := int(hotspot.BoundingBox.X*float64(bounds.Dx())) + bounds.Min.X
		y := int(hotspot.BoundingBox.Y*float64(bounds.Dy())) + bounds.Min.Y
		w := int(hotspot.BoundingBox.Width * float64(bounds.Dx()))
		h := int(hotspot.BoundingBox.Height * float64(bounds.Dy()))
		if w <= 0 {
			w = 1
		}
		if h <= 0 {
			h = 1
		}
		crop := cropImage(fullImage, image.Rect(x, y, x+w, y+h))
		dataURI, err := encodeJPEG(crop)
		if err == nil {
			hotspot.ThumbnailBase64 = dataURI
			if idx := strings.Index(dataURI, ","); idx != -1 {
				rawB64 = dataURI[idx+1:]
			}
		}
	}

	if rawB64 == "" {
		// No image data available to verify
		return hotspot, nil
	}

	req := &ollama.GenerateRequest{
		Model:  ollama.Gemma4Model,
		Prompt: verificationPrompt,
		Images: []string{rawB64},
		Format: "json",
	}

	resp, err := ollamaClient.Generate(callCtx, req)
	if err != nil {
		// Non-fatal graceful fallback on timeout or error
		return hotspot, nil
	}

	raw := strings.TrimSpace(resp.Response)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || start >= end {
		return hotspot, nil
	}

	var res verificationResult
	if err := json.Unmarshal([]byte(raw[start:end+1]), &res); err != nil {
		return hotspot, nil
	}

	if res.IsPet {
		if res.Category != "" {
			hotspot.Classification = res.Category
		} else {
			hotspot.Classification = "Domestic Animal Signature"
		}
		if res.Confidence > 0 {
			hotspot.ConfidenceScore = (hotspot.ConfidenceScore + res.Confidence) / 2.0
			if hotspot.ConfidenceScore > 1.0 {
				hotspot.ConfidenceScore = 1.0
			}
			hotspot.ConfidenceScore = math.Round(hotspot.ConfidenceScore*1000.0) / 1000.0
		}
	} else {
		if res.Category != "" {
			hotspot.Classification = res.Category
		} else {
			hotspot.Classification = defaultClassification
		}
		if res.Confidence > 0 {
			hotspot.ConfidenceScore = math.Max(0.0, math.Min(1.0, hotspot.ConfidenceScore*(1.0-res.Confidence*0.5)))
			hotspot.ConfidenceScore = math.Round(hotspot.ConfidenceScore*1000.0) / 1000.0
		}
	}

	return hotspot, nil
}
