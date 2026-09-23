package recon_test

import (
	"image"
	"image/color"
	"math"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/recon"
)

func TestAnalyzeThermalImage_WhiteHot(t *testing.T) {
	// Create 100x100 dark grayscale background (cold terrain)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
		}
	}
	// Inject a 6x6 bright hot cluster at (50, 50) simulating a pet body
	for y := 47; y <= 53; y++ {
		for x := 47; x <= 53; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}

	wp := domain.DroneWaypoint{
		Timestamp:      time.Now(),
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    30.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: -90.0,
	}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	hotspots, err := recon.AnalyzeThermalImage(img, domain.PaletteWhiteHot, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hotspots) != 1 {
		t.Fatalf("expected 1 detected hotspot, got %d", len(hotspots))
	}
	h := hotspots[0]
	if h.ConfidenceScore < 0.70 {
		t.Errorf("expected high confidence score, got %f", h.ConfidenceScore)
	}
	if h.BoundingBox.X < 0.40 || h.BoundingBox.X > 0.55 {
		t.Errorf("expected bounding box X centered around 0.5, got %f", h.BoundingBox.X)
	}
	if h.ThumbnailBase64 == "" {
		t.Errorf("expected non-empty thumbnail base64")
	}
	if math.Abs(h.Latitude-37.7749) > 0.001 {
		t.Errorf("expected projected latitude near 37.7749, got %f", h.Latitude)
	}
	if math.Abs(h.Longitude-(-122.4194)) > 0.001 {
		t.Errorf("expected projected longitude near -122.4194, got %f", h.Longitude)
	}
	if h.EstimatedTempC < 30.0 || h.EstimatedTempC > 45.0 {
		t.Errorf("expected mammalian body temperature estimate ~35-40C, got %f", h.EstimatedTempC)
	}
}

func TestAnalyzeThermalImage_SuppressUniformSolarRoad(t *testing.T) {
	// Create a large 80x20 hot horizontal bar (e.g. solar-heated road)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if y >= 40 && y <= 60 && x >= 10 && x <= 90 {
				img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
			}
		}
	}

	wp := domain.DroneWaypoint{AltitudeAGL: 30.0, GimbalPitchDeg: -90.0}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	hotspots, err := recon.AnalyzeThermalImage(img, domain.PaletteWhiteHot, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Massive continuous road should be rejected by GSD maximum ground dimension filter
	if len(hotspots) != 0 {
		t.Fatalf("expected road to be filtered out as false-positive, got %d hotspots", len(hotspots))
	}
}

func TestAnalyzeThermalImage_BlackHot(t *testing.T) {
	// In Black-Hot: cold terrain is bright (high RGB), hot target is dark (low RGB)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 220, G: 220, B: 220, A: 255})
		}
	}
	// Hot pet body appears dark
	for y := 48; y <= 52; y++ {
		for x := 48; x <= 52; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 20, B: 20, A: 255})
		}
	}

	wp := domain.DroneWaypoint{
		AltitudeAGL:    25.0,
		GimbalPitchDeg: -90.0,
		Latitude:       37.7749,
		Longitude:      -122.4194,
	}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	hotspots, err := recon.AnalyzeThermalImage(img, domain.PaletteBlackHot, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hotspots) != 1 {
		t.Fatalf("expected 1 detected hotspot in black-hot mode, got %d", len(hotspots))
	}
	if hotspots[0].ConfidenceScore < 0.65 {
		t.Errorf("expected high confidence score for black-hot target, got %f", hotspots[0].ConfidenceScore)
	}
}

func TestAnalyzeThermalImage_Ironbow(t *testing.T) {
	// In Ironbow: cold terrain is dark purple/blue; hot target is bright yellow/orange/white
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 25, G: 15, B: 70, A: 255}) // dark purple/blue
		}
	}
	// Hot pet body appears yellow/orange (R > 200, G > 160, B < 80)
	for y := 48; y <= 52; y++ {
		for x := 48; x <= 52; x++ {
			img.Set(x, y, color.RGBA{R: 245, G: 190, B: 30, A: 255})
		}
	}

	wp := domain.DroneWaypoint{
		AltitudeAGL:    20.0,
		GimbalPitchDeg: -90.0,
		Latitude:       37.7749,
		Longitude:      -122.4194,
	}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	hotspots, err := recon.AnalyzeThermalImage(img, domain.PaletteIronbow, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hotspots) != 1 {
		t.Fatalf("expected 1 detected hotspot in ironbow mode, got %d", len(hotspots))
	}
	if hotspots[0].ConfidenceScore < 0.65 {
		t.Errorf("expected high confidence score for ironbow target, got %f", hotspots[0].ConfidenceScore)
	}
}

func TestAnalyzeThermalImage_SmallNoiseSuppressed(t *testing.T) {
	// Create dark background with a 1-pixel hot noise spike (smaller than 3 pixels)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
		}
	}
	img.Set(50, 50, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	wp := domain.DroneWaypoint{AltitudeAGL: 30.0, GimbalPitchDeg: -90.0}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	hotspots, err := recon.AnalyzeThermalImage(img, domain.PaletteWhiteHot, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hotspots) != 0 {
		t.Fatalf("expected 1-pixel noise spike to be suppressed, got %d", len(hotspots))
	}
}

func TestAnalyzeThermalImage_LowContrastSuppressed(t *testing.T) {
	// Candidate blob with less than 15% local ring contrast
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 100, G: 100, B: 100, A: 255})
		}
	}
	// Blob only ~6% brighter (106 vs 100)
	for y := 48; y <= 52; y++ {
		for x := 48; x <= 52; x++ {
			img.Set(x, y, color.RGBA{R: 106, G: 106, B: 106, A: 255})
		}
	}

	wp := domain.DroneWaypoint{AltitudeAGL: 30.0, GimbalPitchDeg: -90.0}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	hotspots, err := recon.AnalyzeThermalImage(img, domain.PaletteWhiteHot, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hotspots) != 0 {
		t.Fatalf("expected low-contrast blob (<15%%) to be rejected, got %d", len(hotspots))
	}
}

func TestAnalyzeThermalImage_ValidationErrors(t *testing.T) {
	wp := domain.DroneWaypoint{AltitudeAGL: 30.0, GimbalPitchDeg: -90.0}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	// Nil image
	if _, err := recon.AnalyzeThermalImage(nil, domain.PaletteWhiteHot, wp, cam); err == nil {
		t.Errorf("expected error for nil image")
	}

	// Zero or negative altitude
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	badWP := wp
	badWP.AltitudeAGL = 0
	if _, err := recon.AnalyzeThermalImage(img, domain.PaletteWhiteHot, badWP, cam); err == nil {
		t.Errorf("expected error for zero altitude")
	}

	// Invalid HFOV
	badCam := cam
	badCam.HFOV = -10
	if _, err := recon.AnalyzeThermalImage(img, domain.PaletteWhiteHot, wp, badCam); err == nil {
		t.Errorf("expected error for negative HFOV")
	}
}
