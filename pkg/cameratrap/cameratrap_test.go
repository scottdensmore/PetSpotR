package cameratrap

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/ollama"
)

func createSyntheticImage(w, h int, fn func(x, y int) color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, fn(x, y))
		}
	}
	return img
}

func encodePNGDataURI(t *testing.T, img image.Image) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode PNG: %v", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestComputeDHashAndDuplicateDetection(t *testing.T) {
	if h := ComputeDHash(nil); h != 0 {
		t.Fatalf("expected 0 hash for nil image, got %d", h)
	}

	// Horizontal gradient: left bright -> right dark so lum[row][col] > lum[row][col+1]
	imgA := createSyntheticImage(64, 64, func(x, y int) color.RGBA {
		v := uint8(255 - (x * 255 / 63))
		return color.RGBA{R: v, G: v, B: v, A: 255}
	})
	imgAClone := createSyntheticImage(64, 64, func(x, y int) color.RGBA {
		v := uint8(255 - (x * 255 / 63))
		return color.RGBA{R: v, G: v, B: v, A: 255}
	})
	// Opposite gradient: left dark -> right bright
	imgB := createSyntheticImage(64, 64, func(x, y int) color.RGBA {
		v := uint8(x * 255 / 63)
		return color.RGBA{R: v, G: v, B: v, A: 255}
	})

	hashA := ComputeDHash(imgA)
	hashAClone := ComputeDHash(imgAClone)
	hashB := ComputeDHash(imgB)

	if hashA == 0 {
		t.Fatalf("expected non-zero dHash for descending gradient image")
	}
	if len(FormatDHashHex(hashA)) != 16 {
		t.Fatalf("expected 16-char hex string, got %q", FormatDHashHex(hashA))
	}

	if dist := HammingDistance(hashA, hashAClone); dist != 0 {
		t.Fatalf("expected distance 0 for identical images, got %d", dist)
	}

	isDup, dist := IsDuplicateFrame(hashA, hashAClone, true, DefaultDuplicateMaxHamming)
	if !isDup || dist != 0 {
		t.Fatalf("expected duplicate=true, dist=0, got duplicate=%v, dist=%d", isDup, dist)
	}

	isDupNoPrev, distNoPrev := IsDuplicateFrame(hashA, hashAClone, false, DefaultDuplicateMaxHamming)
	if isDupNoPrev || distNoPrev != 64 {
		t.Fatalf("expected duplicate=false, dist=64 when hasPrev=false, got duplicate=%v, dist=%d", isDupNoPrev, distNoPrev)
	}

	isDupDistinct, distDistinct := IsDuplicateFrame(hashA, hashB, true, DefaultDuplicateMaxHamming)
	if isDupDistinct || distDistinct <= DefaultDuplicateMaxHamming {
		t.Fatalf("expected distinct images to have distance > %d and duplicate=false, got duplicate=%v, dist=%d",
			DefaultDuplicateMaxHamming, isDupDistinct, distDistinct)
	}
}

func TestDetectNightVisionIR(t *testing.T) {
	if DetectNightVisionIR(nil) {
		t.Fatalf("expected false for nil image")
	}

	// Monochromatic dark IR image
	irImg := createSyntheticImage(64, 64, func(x, y int) color.RGBA {
		v := uint8(25 + (x+y)%20)
		return color.RGBA{R: v, G: v, B: v, A: 255}
	})
	if !DetectNightVisionIR(irImg) {
		t.Fatalf("expected true for monochromatic dark IR image")
	}

	// Bright colorful daylight RGB image
	rgbImg := createSyntheticImage(64, 64, func(x, y int) color.RGBA {
		return color.RGBA{R: 230, G: 140, B: 45, A: 255}
	})
	if DetectNightVisionIR(rgbImg) {
		t.Fatalf("expected false for bright colorful daylight RGB image")
	}
}

func TestDetectMotionROI(t *testing.T) {
	nilROI, nilSal := DetectMotionROI(nil)
	if nilSal != 0.0 || nilROI.Width != 0.5 {
		t.Fatalf("expected default ROI and 0 saliency for nil image, got %+v, %f", nilROI, nilSal)
	}

	// Dark background with bright textured patch in lower-right quadrant (x: 40..58, y: 40..58)
	motionImg := createSyntheticImage(64, 64, func(x, y int) color.RGBA {
		if x >= 40 && x <= 58 && y >= 40 && y <= 58 {
			if (x+y)%2 == 0 {
				return color.RGBA{R: 245, G: 245, B: 245, A: 255}
			}
			return color.RGBA{R: 180, G: 180, B: 180, A: 255}
		}
		return color.RGBA{R: 18, G: 18, B: 18, A: 255}
	})

	roi, saliency := DetectMotionROI(motionImg)
	if roi.X < 0.45 || roi.Y < 0.45 {
		t.Fatalf("expected ROI to localize in lower-right quadrant (X>=0.45, Y>=0.45), got %+v", roi)
	}
	if roi.X+roi.Width > 1.0 || roi.Y+roi.Height > 1.0 {
		t.Fatalf("expected ROI bounds within [0, 1], got %+v", roi)
	}
	if saliency < 0.5 {
		t.Fatalf("expected high saliency (>= 0.5) for bright motion patch, got %f", saliency)
	}
}

func TestProcessEdgeFrame(t *testing.T) {
	// Dark IR image with a bright subject in lower-right quadrant
	irMotionImg := createSyntheticImage(64, 64, func(x, y int) color.RGBA {
		if x >= 36 && x <= 56 && y >= 36 && y <= 56 {
			v := uint8(190 + (x*7+y*3)%50)
			return color.RGBA{R: v, G: v, B: v, A: 255}
		}
		return color.RGBA{R: 20, G: 22, B: 20, A: 255}
	})
	dataURI := encodePNGDataURI(t, irMotionImg)

	res, err := ProcessEdgeFrame(dataURI, 0, false)
	if err != nil {
		t.Fatalf("ProcessEdgeFrame failed: %v", err)
	}
	if len(res.FrameDHashHex) != 16 {
		t.Fatalf("expected 16-char FrameDHashHex, got %q", res.FrameDHashHex)
	}
	if res.IsDuplicate {
		t.Fatalf("expected first frame not to be duplicate")
	}
	if !res.IsNightVisionIR {
		t.Fatalf("expected IsNightVisionIR=true")
	}
	if !res.IREnhanced {
		t.Fatalf("expected IREnhanced=true")
	}
	if !strings.HasPrefix(res.CropThumbnailURI, "data:image/jpeg;base64,") {
		t.Fatalf("expected JPEG data URI for CropThumbnailURI, got prefix %q", res.CropThumbnailURI[:30])
	}
	if res.ROI.Width < 0.15 || res.ROI.Height < 0.15 {
		t.Fatalf("expected valid ROI dimensions, got %+v", res.ROI)
	}

	// Second call with identical frame hash should detect duplicate
	resDup, err := ProcessEdgeFrame(dataURI, res.FrameDHash, true)
	if err != nil {
		t.Fatalf("second ProcessEdgeFrame failed: %v", err)
	}
	if !resDup.IsDuplicate || resDup.HammingDistance != 0 {
		t.Fatalf("expected duplicate frame with hamming=0, got isDup=%v, hamming=%d", resDup.IsDuplicate, resDup.HammingDistance)
	}
}

func TestVerifyCaptureWithOllama(t *testing.T) {
	baseCapture := domain.CameraCapture{
		ID:               "cap-1",
		IsNightVisionIR:  true,
		SaliencyScore:    0.72,
		CropThumbnailURI: "data:image/jpeg;base64,dGVzdA==",
	}

	// 1. Nil client fallback
	fallback := VerifyCaptureWithOllama(context.Background(), nil, baseCapture, "cat", nil)
	if fallback.DetectedSpecies != "CAT" {
		t.Fatalf("expected normalized species CAT, got %q", fallback.DetectedSpecies)
	}
	if len(fallback.ExtractedTraits) == 0 {
		t.Fatalf("expected default IR traits when vendorHintTraits is empty")
	}
	if fallback.ConfidenceScore < 0.55 || fallback.ConfidenceScore > 0.95 {
		t.Fatalf("expected deterministic confidence in [0.55, 0.95], got %f", fallback.ConfidenceScore)
	}

	// 2. Mock httptest.Server Ollama endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"gemma4:e2b","response":"{\"species\":\"DOG\",\"confidence\":0.92,\"traits\":[\"Reflective collar\",\"Pointed ears\"]}","done":true}`))
	}))
	defer srv.Close()

	client := ollama.NewClient(
		ollama.WithBaseURL(srv.URL),
		ollama.WithMaxRetries(0),
	)

	verified := VerifyCaptureWithOllama(context.Background(), client, baseCapture, "", []string{"Motion blur"})
	if verified.DetectedSpecies != "DOG" {
		t.Fatalf("expected Ollama verified species DOG, got %q", verified.DetectedSpecies)
	}
	if len(verified.ExtractedTraits) < 2 {
		t.Fatalf("expected traits from Ollama verification, got %+v", verified.ExtractedTraits)
	}
	if verified.ConfidenceScore <= fallback.ConfidenceScore {
		t.Fatalf("expected blended confidence (%f) to reflect high Ollama confidence (0.92) vs baseline (%f)",
			verified.ConfidenceScore, fallback.ConfidenceScore)
	}
}
