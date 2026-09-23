package recon_test

import (
	"context"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
	"github.com/scottdensmore/petspotr/pkg/recon"
)

func TestVerifyHotspotWithOllama_Success(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `{"isPet": true, "confidence": 0.95, "category": "Canine Signature"}`,
		Done:     true,
	}, nil)

	hotspot := domain.ThermalHotspot{
		ID:              "hotspot-1",
		ConfidenceScore: 0.75,
		ThumbnailBase64: "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
		Status:          domain.HotspotUnverified,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	verified, err := recon.VerifyHotspotWithOllama(ctx, client, hotspot, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verified.Classification != "Canine Signature" {
		t.Errorf("expected classification 'Canine Signature', got %q", verified.Classification)
	}
	if verified.ConfidenceScore < 0.75 {
		t.Errorf("expected updated confidence score >= 0.75, got %f", verified.ConfidenceScore)
	}
}

func TestVerifyHotspotWithOllama_NilClientFallback(t *testing.T) {
	hotspot := domain.ThermalHotspot{
		ID:              "hotspot-2",
		ConfidenceScore: 0.78,
		Status:          domain.HotspotUnverified,
	}

	verified, err := recon.VerifyHotspotWithOllama(context.Background(), nil, hotspot, nil)
	if err != nil {
		t.Fatalf("unexpected error with nil client: %v", err)
	}
	if verified.ConfidenceScore != 0.78 {
		t.Errorf("expected deterministic score 0.78 retained, got %f", verified.ConfidenceScore)
	}
	if verified.Classification != "Thermal Heat Anomaly" {
		t.Errorf("expected classification 'Thermal Heat Anomaly', got %q", verified.Classification)
	}
}

func TestVerifyHotspotWithOllama_OfflineFallback(t *testing.T) {
	// Point client to a failing HTTP test server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := ollama.NewClient(
		ollama.WithBaseURL(ts.URL),
		ollama.WithMaxRetries(0),
		ollama.WithTimeout(500*time.Millisecond),
	)

	hotspot := domain.ThermalHotspot{
		ID:              "hotspot-3",
		ConfidenceScore: 0.82,
		ThumbnailBase64: "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
	}

	verified, err := recon.VerifyHotspotWithOllama(context.Background(), client, hotspot, nil)
	if err != nil {
		t.Fatalf("expected non-fatal graceful fallback on error, got %v", err)
	}
	if verified.ConfidenceScore != 0.82 {
		t.Errorf("expected confidence score 0.82 preserved, got %f", verified.ConfidenceScore)
	}
	if verified.Classification != "Thermal Heat Anomaly" {
		t.Errorf("expected fallback classification 'Thermal Heat Anomaly', got %q", verified.Classification)
	}
}

func TestVerifyHotspotWithOllama_InvalidJSON(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `I observed a glowing thermal signature but cannot determine the object.`,
		Done:     true,
	}, nil)

	hotspot := domain.ThermalHotspot{
		ID:              "hotspot-4",
		ConfidenceScore: 0.80,
		ThumbnailBase64: "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
	}

	verified, err := recon.VerifyHotspotWithOllama(context.Background(), client, hotspot, nil)
	if err != nil {
		t.Fatalf("expected non-fatal graceful fallback on invalid json, got %v", err)
	}
	if verified.ConfidenceScore != 0.80 {
		t.Errorf("expected confidence score 0.80 preserved, got %f", verified.ConfidenceScore)
	}
	if verified.Classification != "Thermal Heat Anomaly" {
		t.Errorf("expected fallback classification 'Thermal Heat Anomaly', got %q", verified.Classification)
	}
}

func TestVerifyHotspotWithOllama_NotPet(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `{"isPet": false, "confidence": 0.88, "category": "HVAC Exhaust Unit"}`,
		Done:     true,
	}, nil)

	hotspot := domain.ThermalHotspot{
		ID:              "hotspot-5",
		ConfidenceScore: 0.70,
		ThumbnailBase64: "data:image/jpeg;base64,/9j/4AAQSkZJRg==",
	}

	verified, err := recon.VerifyHotspotWithOllama(context.Background(), client, hotspot, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verified.Classification != "HVAC Exhaust Unit" {
		t.Errorf("expected classification 'HVAC Exhaust Unit', got %q", verified.Classification)
	}
}

func TestVerifyHotspotWithOllama_CropFromFullImage(t *testing.T) {
	client := ollama.NewDeterministicClient(&ollama.GenerateResponse{
		Model:    ollama.Gemma4Model,
		Response: `{"isPet": true, "confidence": 0.90, "category": "Feline Signature"}`,
		Done:     true,
	}, nil)

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
		}
	}

	hotspot := domain.ThermalHotspot{
		ID:              "hotspot-6",
		ConfidenceScore: 0.72,
		BoundingBox: domain.NormalizedBox{
			X:      0.4,
			Y:      0.4,
			Width:  0.2,
			Height: 0.2,
		},
		// ThumbnailBase64 is deliberately empty to test cropping from fullImage
	}

	verified, err := recon.VerifyHotspotWithOllama(context.Background(), client, hotspot, img)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verified.Classification != "Feline Signature" {
		t.Errorf("expected classification 'Feline Signature', got %q", verified.Classification)
	}
}
