package cameratrap

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
)

type ollamaVerificationResponse struct {
	Species    string   `json:"species"`
	Confidence float64  `json:"confidence"`
	Traits     []string `json:"traits"`
}

// VerifyCaptureWithOllama establishes deterministic baseline species, traits, and confidence
// for a camera-trap capture and optionally refines them using local Ollama Gemma 4 multimodal
// inference when available and non-duplicate.
func VerifyCaptureWithOllama(
	ctx context.Context,
	client *ollama.Client,
	capture domain.CameraCapture,
	vendorHintSpecies string,
	vendorHintTraits []string,
) domain.CameraCapture {
	species := strings.ToUpper(strings.TrimSpace(vendorHintSpecies))
	if species == "" {
		species = strings.ToUpper(strings.TrimSpace(capture.DetectedSpecies))
	}
	if species == "" {
		species = "DOG"
	}
	capture.DetectedSpecies = species

	if len(vendorHintTraits) > 0 {
		traits := make([]string, 0, len(vendorHintTraits))
		for _, t := range vendorHintTraits {
			if clean := strings.TrimSpace(t); clean != "" {
				traits = append(traits, clean)
			}
		}
		if len(traits) > 0 {
			capture.ExtractedTraits = traits
		}
	}
	if len(capture.ExtractedTraits) == 0 {
		if capture.IsNightVisionIR {
			capture.ExtractedTraits = []string{"Night-vision IR reflection", "Motion silhouette"}
		} else {
			capture.ExtractedTraits = []string{"Daylight motion subject"}
		}
	}

	if capture.IsDuplicate {
		if capture.ConfidenceScore <= 0 {
			capture.ConfidenceScore = round3(math.Min(0.50, math.Max(0.20, capture.SaliencyScore*0.50)))
		}
		return capture
	}

	baselineConf := round3(math.Min(0.95, math.Max(0.55, capture.SaliencyScore*0.85+0.25)))
	capture.ConfidenceScore = baselineConf

	if client == nil {
		return capture
	}

	imgURI := capture.CropThumbnailURI
	if strings.TrimSpace(imgURI) == "" {
		imgURI = capture.SnapshotDataURI
	}
	b64Img := extractBase64Payload(imgURI)
	if b64Img == "" {
		return capture
	}

	if ctx == nil {
		ctx = context.Background()
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	req := &ollama.GenerateRequest{
		Model:  ollama.Gemma4Model,
		Prompt: `Inspect this smart camera-trap ROI crop and return JSON: {"species":"DOG|CAT|OTHER","confidence":0.0-1.0,"traits":["visual trait 1","visual trait 2"]}`,
		Images: []string{b64Img},
		Format: "json",
	}

	resp, err := client.Generate(verifyCtx, req)
	if err != nil || resp == nil || strings.TrimSpace(resp.Response) == "" {
		return capture
	}

	var parsed ollamaVerificationResponse
	if err := json.Unmarshal([]byte(strings.TrimSpace(resp.Response)), &parsed); err != nil {
		return capture
	}

	if cleanSpecies := strings.ToUpper(strings.TrimSpace(parsed.Species)); cleanSpecies != "" {
		capture.DetectedSpecies = cleanSpecies
	}
	if len(parsed.Traits) > 0 {
		cleanTraits := make([]string, 0, len(parsed.Traits))
		for _, t := range parsed.Traits {
			if ct := strings.TrimSpace(t); ct != "" {
				cleanTraits = append(cleanTraits, ct)
			}
		}
		if len(cleanTraits) > 0 {
			capture.ExtractedTraits = cleanTraits
		}
	}
	if parsed.Confidence > 0 && parsed.Confidence <= 1.0 {
		blended := baselineConf*0.35 + parsed.Confidence*0.65
		capture.ConfidenceScore = round3(math.Min(0.99, math.Max(0.35, blended)))
	}

	return capture
}

func extractBase64Payload(dataURI string) string {
	trimmed := strings.TrimSpace(dataURI)
	if trimmed == "" {
		return ""
	}
	if idx := strings.Index(trimmed, ","); idx >= 0 && idx+1 < len(trimmed) {
		return strings.TrimSpace(trimmed[idx+1:])
	}
	return trimmed
}
