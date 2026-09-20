package imaging_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/scottdensmore/petspotr/pkg/imaging"
)

// createSyntheticImage creates an image with width, height and a generator function for colors.
func createSyntheticImage(w, h int, fn func(x, y int) color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, fn(x, y))
		}
	}
	return img
}

// encodeToJPEG encodes an image to JPEG bytes.
func encodeToJPEG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("failed to encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// encodeToPNG encodes an image to PNG bytes.
func encodeToPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}
	return buf.Bytes()
}

func TestEnhanceImage_InvalidInputs(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		data []byte
	}{
		{"dummy text bytes", []byte("dummy raw image bytes")},
		{"empty slice", []byte{}},
		{"nil slice", nil},
		{"corrupt header", []byte{0xFF, 0xD8, 0xFF, 0x00, 0x12}},
		{"short slice", []byte{0xFF, 0xD8}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := imaging.EnhanceImage(tc.data)
			if err == nil {
				t.Fatalf("expected error for %s, got nil", tc.name)
			}
			if !errors.Is(err, imaging.ErrInvalidImage) {
				t.Errorf("expected ErrInvalidImage, got %v", err)
			}
		})
	}
}

func TestEnhanceImage_DimensionSafetyGuards(t *testing.T) {
	t.Parallel()

	t.Run("excessive buffer size", func(t *testing.T) {
		t.Parallel()
		oversized := make([]byte, imaging.MaxImageBytes+1)
		_, err := imaging.EnhanceImage(oversized)
		if err == nil {
			t.Fatalf("expected error for oversized buffer, got nil")
		}
		if !errors.Is(err, imaging.ErrDimensionLimitExceeded) {
			t.Errorf("expected ErrDimensionLimitExceeded, got %v", err)
		}
	})

	t.Run("excessive image dimensions in SOF", func(t *testing.T) {
		t.Parallel()
		// Synthetic JPEG with SOF0 stating width=10000, height=10000 (> 8192)
		rawJPEG := buildJPEGWithEXIF(nil, 10000, 10000)
		_, err := imaging.EnhanceImage(rawJPEG)
		if err == nil {
			t.Fatalf("expected error for excessive image dimensions, got nil")
		}
		if !errors.Is(err, imaging.ErrDimensionLimitExceeded) {
			t.Errorf("expected ErrDimensionLimitExceeded, got %v", err)
		}
	})
}

func TestAnalyzeQuality(t *testing.T) {
	t.Parallel()

	t.Run("dark low-contrast image", func(t *testing.T) {
		t.Parallel()
		// Pixel values in narrow range 20..40
		img := createSyntheticImage(50, 50, func(x, y int) color.RGBA {
			v := uint8(20 + (x+y)%20)
			return color.RGBA{R: v, G: v, B: v, A: 255}
		})

		report := imaging.AnalyzeQuality(img)
		if !report.IsUnderexposed {
			t.Errorf("expected IsUnderexposed to be true, got false (mean: %f)", report.MeanBrightness)
		}
		if !report.IsLowContrast {
			t.Errorf("expected IsLowContrast to be true, got false (contrast: %f)", report.Contrast)
		}
		if report.MinBrightness < 20 || report.MaxBrightness > 40 {
			t.Errorf("expected brightness range [20, 40], got [%d, %d]", report.MinBrightness, report.MaxBrightness)
		}
	})

	t.Run("bright overexposed image", func(t *testing.T) {
		t.Parallel()
		// Pixel values in high range 220..250
		img := createSyntheticImage(50, 50, func(x, y int) color.RGBA {
			v := uint8(220 + (x+y)%30)
			return color.RGBA{R: v, G: v, B: v, A: 255}
		})

		report := imaging.AnalyzeQuality(img)
		if !report.IsOverexposed {
			t.Errorf("expected IsOverexposed to be true, got false (mean: %f)", report.MeanBrightness)
		}
		if report.IsUnderexposed {
			t.Errorf("expected IsUnderexposed to be false, got true")
		}
	})

	t.Run("high contrast checkerboard image", func(t *testing.T) {
		t.Parallel()
		// Alternating 0 and 255
		img := createSyntheticImage(50, 50, func(x, y int) color.RGBA {
			if (x+y)%2 == 0 {
				return color.RGBA{R: 0, G: 0, B: 0, A: 255}
			}
			return color.RGBA{R: 255, G: 255, B: 255, A: 255}
		})

		report := imaging.AnalyzeQuality(img)
		if report.IsLowContrast {
			t.Errorf("expected IsLowContrast to be false for checkerboard, got true")
		}
		if report.MinBrightness != 0 || report.MaxBrightness != 255 {
			t.Errorf("expected min=0 max=255, got [%d, %d]", report.MinBrightness, report.MaxBrightness)
		}
		if report.PercentileLow != 0 || report.PercentileHigh != 255 {
			t.Errorf("expected PercentileLow=0 PercentileHigh=255, got [%d, %d]", report.PercentileLow, report.PercentileHigh)
		}
	})
}

func TestNormalizeContrast(t *testing.T) {
	t.Parallel()

	// Low-contrast dark image: all pixels between 30 and 80
	src := createSyntheticImage(100, 100, func(x, y int) color.RGBA {
		v := uint8(30 + (x*50)/100)
		return color.RGBA{R: v, G: v, B: v, A: 255}
	})

	initialReport := imaging.AnalyzeQuality(src)
	normalized := imaging.NormalizeContrast(src)
	enhancedReport := imaging.AnalyzeQuality(normalized)

	if enhancedReport.Contrast <= initialReport.Contrast {
		t.Errorf("expected enhanced contrast (%f) to exceed initial (%f)",
			enhancedReport.Contrast, initialReport.Contrast)
	}

	if enhancedReport.MinBrightness > 10 {
		t.Errorf("expected enhanced min brightness <= 10 after stretching, got %d", enhancedReport.MinBrightness)
	}

	if enhancedReport.MaxBrightness < 240 {
		t.Errorf("expected enhanced max brightness >= 240 after stretching, got %d", enhancedReport.MaxBrightness)
	}
}

func TestApplyOrientation_All8Orientations(t *testing.T) {
	t.Parallel()

	// Create an asymmetric 4x2 test pattern:
	// Row 0: Red, Green, Blue, White
	// Row 1: Black, Cyan, Magenta, Yellow
	red := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	green := color.RGBA{R: 0, G: 255, B: 0, A: 255}
	blue := color.RGBA{R: 0, G: 0, B: 255, A: 255}
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	black := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	cyan := color.RGBA{R: 0, G: 255, B: 255, A: 255}
	magenta := color.RGBA{R: 255, G: 0, B: 255, A: 255}
	yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}

	grid := [][]color.RGBA{
		{red, green, blue, white},
		{black, cyan, magenta, yellow},
	}

	src := createSyntheticImage(4, 2, func(x, y int) color.RGBA {
		return grid[y][x]
	})

	testCases := []struct {
		orientation int
		wantW       int
		wantH       int
		testPixels  map[image.Point]color.RGBA
	}{
		{
			orientation: 1, // Normal
			wantW:       4,
			wantH:       2,
			testPixels: map[image.Point]color.RGBA{
				{0, 0}: red,
				{3, 0}: white,
				{0, 1}: black,
				{3, 1}: yellow,
			},
		},
		{
			orientation: 2, // Flip Horizontal
			wantW:       4,
			wantH:       2,
			testPixels: map[image.Point]color.RGBA{
				{0, 0}: white,
				{3, 0}: red,
				{0, 1}: yellow,
				{3, 1}: black,
			},
		},
		{
			orientation: 3, // Rotate 180
			wantW:       4,
			wantH:       2,
			testPixels: map[image.Point]color.RGBA{
				{0, 0}: yellow,
				{3, 0}: black,
				{0, 1}: white,
				{3, 1}: red,
			},
		},
		{
			orientation: 4, // Flip Vertical
			wantW:       4,
			wantH:       2,
			testPixels: map[image.Point]color.RGBA{
				{0, 0}: black,
				{3, 0}: yellow,
				{0, 1}: red,
				{3, 1}: white,
			},
		},
		{
			orientation: 5, // Transpose (swap X and Y)
			wantW:       2,
			wantH:       4,
			testPixels: map[image.Point]color.RGBA{
				{0, 0}: red,
				{1, 0}: black,
				{0, 3}: white,
				{1, 3}: yellow,
			},
		},
		{
			orientation: 6, // Rotate 90 CW
			wantW:       2,
			wantH:       4,
			testPixels: map[image.Point]color.RGBA{
				{0, 0}: black,
				{1, 0}: red,
				{0, 3}: yellow,
				{1, 3}: white,
			},
		},
		{
			orientation: 7, // Transverse (Flip Horizontal + Rotate 90 CW)
			wantW:       2,
			wantH:       4,
			testPixels: map[image.Point]color.RGBA{
				{0, 0}: yellow,
				{1, 0}: white,
				{0, 3}: black,
				{1, 3}: red,
			},
		},
		{
			orientation: 8, // Rotate 270 CW (90 CCW)
			wantW:       2,
			wantH:       4,
			testPixels: map[image.Point]color.RGBA{
				{0, 0}: white,
				{1, 0}: yellow,
				{0, 3}: red,
				{1, 3}: black,
			},
		},
	}

	for _, tc := range testCases {
		t.Run("orientation_"+string(rune('0'+tc.orientation)), func(t *testing.T) {
			t.Parallel()
			dst := imaging.ApplyOrientation(src, tc.orientation)
			if dst.Bounds().Dx() != tc.wantW || dst.Bounds().Dy() != tc.wantH {
				t.Fatalf("expected dimensions %dx%d, got %dx%d",
					tc.wantW, tc.wantH, dst.Bounds().Dx(), dst.Bounds().Dy())
			}
			for pt, wantCol := range tc.testPixels {
				got := dst.At(pt.X, pt.Y)
				r, g, b, a := got.RGBA()
				wantR, wantG, wantB, wantA := wantCol.RGBA()
				if r != wantR || g != wantG || b != wantB || a != wantA {
					t.Errorf("orientation %d at (%d,%d): want %+v, got %+v",
						tc.orientation, pt.X, pt.Y, wantCol, got)
				}
			}
		})
	}
}

func TestEnhanceImage_WithOrientationAndLowLight(t *testing.T) {
	t.Parallel()

	// 1. Build a synthetic TIFF with orientation = 6 (Rotate 90 CW)
	order := binary.LittleEndian
	rawTIFF := buildSyntheticTIFF(order, true, func(tw *tiffWriter) {
		_ = binary.Write(tw.buf, order, uint16(1)) // 1 entry: Orientation
		_ = binary.Write(tw.buf, order, uint16(0x0112))
		_ = binary.Write(tw.buf, order, uint16(3)) // SHORT
		_ = binary.Write(tw.buf, order, uint32(1))
		_ = binary.Write(tw.buf, order, uint16(6)) // Orientation = 6
		_ = binary.Write(tw.buf, order, uint16(0))
		_ = binary.Write(tw.buf, order, uint32(0))
	})

	// 2. Create a low-contrast dark image of size 60x40
	darkImg := createSyntheticImage(60, 40, func(x, y int) color.RGBA {
		v := uint8(25 + (x*30)/60) // 25..55
		return color.RGBA{R: v, G: v, B: v, A: 255}
	})

	darkJPEG := encodeToJPEG(t, darkImg)

	// Combine EXIF APP1 with dark JPEG
	var fullJPEG bytes.Buffer
	fullJPEG.Write([]byte{0xFF, 0xD8}) // SOI
	// Write APP1 with rawTIFF
	app1Len := uint16(2 + 6 + len(rawTIFF))
	fullJPEG.Write([]byte{0xFF, 0xE1})
	_ = binary.Write(&fullJPEG, binary.BigEndian, app1Len)
	fullJPEG.Write([]byte("Exif\x00\x00"))
	fullJPEG.Write(rawTIFF)
	// Append darkJPEG payload skipping SOI
	fullJPEG.Write(darkJPEG[2:])

	// 3. Call EnhanceImage
	enhancedBytes, err := imaging.EnhanceImage(fullJPEG.Bytes())
	if err != nil {
		t.Fatalf("unexpected error enhancing image: %v", err)
	}

	// 4. Decode enhanced JPEG
	enhancedImg, err := jpeg.Decode(bytes.NewReader(enhancedBytes))
	if err != nil {
		t.Fatalf("failed to decode enhanced output JPEG: %v", err)
	}

	// 5. Verify orientation rotation: original was 60x40, rotated 90 CW should be 40x60!
	if enhancedImg.Bounds().Dx() != 40 || enhancedImg.Bounds().Dy() != 60 {
		t.Errorf("expected rotated dimensions 40x60, got %dx%d",
			enhancedImg.Bounds().Dx(), enhancedImg.Bounds().Dy())
	}

	// 6. Verify contrast and dynamic range expansion
	report := imaging.AnalyzeQuality(enhancedImg)
	if report.MinBrightness > 15 {
		t.Errorf("expected enhanced MinBrightness <= 15, got %d", report.MinBrightness)
	}
	if report.MaxBrightness < 235 {
		t.Errorf("expected enhanced MaxBrightness >= 235, got %d", report.MaxBrightness)
	}
}

func TestEnhanceImage_PNGInput(t *testing.T) {
	t.Parallel()

	// PNG image with low contrast
	img := createSyntheticImage(30, 20, func(x, y int) color.RGBA {
		v := uint8(40 + (y*20)/20) // 40..60
		return color.RGBA{R: v, G: v, B: v, A: 255}
	})
	pngBytes := encodeToPNG(t, img)

	enhancedBytes, err := imaging.EnhanceImage(pngBytes)
	if err != nil {
		t.Fatalf("unexpected error for PNG input: %v", err)
	}

	// Output MUST be valid JPEG
	decoded, err := jpeg.Decode(bytes.NewReader(enhancedBytes))
	if err != nil {
		t.Fatalf("expected enhanced output to be valid JPEG: %v", err)
	}

	if decoded.Bounds().Dx() != 30 || decoded.Bounds().Dy() != 20 {
		t.Errorf("expected 30x20 dimensions, got %dx%d", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}
