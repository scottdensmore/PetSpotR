package imaging

import (
	"bytes"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
)

// Maximum limits to defend against decompression bombs and memory exhaustion.
const (
	MaxEnhanceDimension = 8192
	MaxImageBytes       = 32 * 1024 * 1024 // 32MB
	MaxPixelCount       = 8192 * 8192      // 67.1M pixels
)

// QualityReport encapsulates statistical metrics of an image's brightness,
// contrast distribution, and exposure diagnosis.
type QualityReport struct {
	Width          int         `json:"width"`
	Height         int         `json:"height"`
	MinBrightness  uint8       `json:"minBrightness"`
	MaxBrightness  uint8       `json:"maxBrightness"`
	MeanBrightness float64     `json:"meanBrightness"`
	Contrast       float64     `json:"contrast"`
	Histogram      [256]uint32 `json:"histogram"`
	IsLowContrast  bool        `json:"isLowContrast"`
	IsUnderexposed bool        `json:"isUnderexposed"`
	IsOverexposed  bool        `json:"isOverexposed"`
}

// EnhanceImage decodes raw image bytes (JPEG, PNG, GIF), inspects EXIF orientation,
// rotates the image to upright if needed, stretches the dynamic range via histogram
// normalization and low-light compensation, and encodes the optimized result to JPEG.
func EnhanceImage(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrInvalidImage
	}
	if len(data) > MaxImageBytes {
		return nil, ErrDimensionLimitExceeded
	}

	// 1. If format is JPEG, inspect markers directly for SOF dimensions to guard against bombs early
	if isJPEG(data) {
		_, sofW, sofH, _, err := parseJPEG(data)
		if err == nil && (sofW > MaxEnhanceDimension || sofH > MaxEnhanceDimension || (sofW > 0 && sofH > 0 && int64(sofW)*int64(sofH) > MaxPixelCount)) {
			return nil, ErrDimensionLimitExceeded
		}
	}

	// 2. Decode header config to verify safe dimensions before allocating full raster
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidImage
	}

	if cfg.Width > MaxEnhanceDimension || cfg.Height > MaxEnhanceDimension || cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, ErrDimensionLimitExceeded
	}
	if int64(cfg.Width)*int64(cfg.Height) > MaxPixelCount {
		return nil, ErrDimensionLimitExceeded
	}

	// 2. Extract EXIF orientation if available
	orientation := 1
	if meta, metaErr := ExtractMetadata(data); metaErr == nil && meta != nil {
		orientation = meta.Orientation
	}
	if orientation < 1 || orientation > 8 {
		orientation = 1
	}

	// 3. Decode full image
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalidImage
	}

	if src.Bounds().Dx() > MaxEnhanceDimension || src.Bounds().Dy() > MaxEnhanceDimension {
		return nil, ErrDimensionLimitExceeded
	}

	// 4. Normalize orientation
	oriented := ApplyOrientation(src, orientation)

	// 5. Apply auto-contrast and dynamic range stretching
	enhanced := NormalizeContrast(oriented)

	// 6. Encode to JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, enhanced, &jpeg.Options{Quality: 90}); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// AnalyzeQuality computes the luminance histogram, dynamic range boundaries,
// standard deviation contrast, and exposure diagnosis for an image.
func AnalyzeQuality(img image.Image) QualityReport {
	if img == nil {
		return QualityReport{}
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	totalPixels := w * h
	if totalPixels == 0 {
		return QualityReport{}
	}

	var hist [256]uint32
	var sum float64
	minBrightness := uint8(255)
	maxBrightness := uint8(0)

	minX := bounds.Min.X
	minY := bounds.Min.Y

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(minX+x, minY+y).RGBA()
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			lum, _, _ := color.RGBToYCbCr(r8, g8, b8)

			hist[lum]++
			sum += float64(lum)
			if lum < minBrightness {
				minBrightness = lum
			}
			if lum > maxBrightness {
				maxBrightness = lum
			}
		}
	}

	mean := sum / float64(totalPixels)

	// Compute variance / standard deviation contrast
	var varSum float64
	for i := 0; i < 256; i++ {
		if hist[i] > 0 {
			diff := float64(i) - mean
			varSum += diff * diff * float64(hist[i])
		}
	}
	contrast := math.Sqrt(varSum / float64(totalPixels))

	// Robust 0.5% percentiles for contrast stretching
	clipCount := uint32(float64(totalPixels) * 0.005)
	pLow := uint8(0)
	pHigh := uint8(255)

	var cum uint32
	for i := 0; i < 256; i++ {
		cum += hist[i]
		if cum >= clipCount {
			pLow = uint8(i)
			break
		}
	}

	cum = 0
	for i := 255; i >= 0; i-- {
		cum += hist[i]
		if cum >= clipCount {
			pHigh = uint8(i)
			break
		}
	}

	isLowContrast := (int(pHigh)-int(pLow) < 128) || contrast < 35.0
	isUnderexposed := mean < 95.0
	isOverexposed := mean > 175.0 && pLow > 40

	return QualityReport{
		Width:          w,
		Height:         h,
		MinBrightness:  minBrightness,
		MaxBrightness:  maxBrightness,
		MeanBrightness: mean,
		Contrast:       contrast,
		Histogram:      hist,
		IsLowContrast:  isLowContrast,
		IsUnderexposed: isUnderexposed,
		IsOverexposed:  isOverexposed,
	}
}

// ApplyOrientation transforms an image according to standard EXIF orientation tags (1-8):
// 1: Normal (0 deg)
// 2: Flip Horizontal
// 3: Rotate 180 deg
// 4: Flip Vertical
// 5: Transpose
// 6: Rotate 90 deg CW
// 7: Transverse
// 8: Rotate 270 deg CW (90 deg CCW)
func ApplyOrientation(img image.Image, orientation int) image.Image {
	if img == nil || orientation <= 1 || orientation > 8 {
		return img
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	minX, minY := bounds.Min.X, bounds.Min.Y

	dstW, dstH := w, h
	if orientation >= 5 && orientation <= 8 {
		dstW, dstH = h, w
	}

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	// Fast path for *image.RGBA to eliminate interface boxing in loop
	if rgba, ok := img.(*image.RGBA); ok {
		for dy := 0; dy < dstH; dy++ {
			for dx := 0; dx < dstW; dx++ {
				var sx, sy int
				switch orientation {
				case 2: // Flip Horizontal
					sx = w - 1 - dx
					sy = dy
				case 3: // Rotate 180
					sx = w - 1 - dx
					sy = h - 1 - dy
				case 4: // Flip Vertical
					sx = dx
					sy = h - 1 - dy
				case 5: // Transpose
					sx = dy
					sy = dx
				case 6: // Rotate 90 CW
					sx = dy
					sy = h - 1 - dx
				case 7: // Transverse
					sx = w - 1 - dy
					sy = h - 1 - dx
				case 8: // Rotate 270 CW (90 CCW)
					sx = w - 1 - dy
					sy = dx
				}
				dst.SetRGBA(dx, dy, rgba.RGBAAt(minX+sx, minY+sy))
			}
		}
		return dst
	}

	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			var sx, sy int
			switch orientation {
			case 2: // Flip Horizontal
				sx = w - 1 - dx
				sy = dy
			case 3: // Rotate 180
				sx = w - 1 - dx
				sy = h - 1 - dy
			case 4: // Flip Vertical
				sx = dx
				sy = h - 1 - dy
			case 5: // Transpose
				sx = dy
				sy = dx
			case 6: // Rotate 90 CW
				sx = dy
				sy = h - 1 - dx
			case 7: // Transverse
				sx = w - 1 - dy
				sy = h - 1 - dx
			case 8: // Rotate 270 CW (90 CCW)
				sx = w - 1 - dy
				sy = dx
			}
			dst.Set(dx, dy, img.At(minX+sx, minY+sy))
		}
	}

	return dst
}

// NormalizeContrast applies dynamic range stretching and low-light compensation
// using an optimized 256-value lookup table constructed from the luminance histogram.
func NormalizeContrast(img image.Image) *image.RGBA {
	if img == nil {
		return nil
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	totalPixels := w * h
	if totalPixels == 0 {
		return image.NewRGBA(bounds)
	}

	report := AnalyzeQuality(img)

	// Determine low and high bounds with 0.5% percentile cutoff
	clipCount := uint32(float64(totalPixels) * 0.005)
	pLow := uint8(0)
	pHigh := uint8(255)

	var cum uint32
	for i := 0; i < 256; i++ {
		cum += report.Histogram[i]
		if cum >= clipCount {
			pLow = uint8(i)
			break
		}
	}

	cum = 0
	for i := 255; i >= 0; i-- {
		cum += report.Histogram[i]
		if cum >= clipCount {
			pHigh = uint8(i)
			break
		}
	}

	// Avoid degenerate or inverted ranges
	if pHigh <= pLow {
		pLow = 0
		pHigh = 255
	}

	// Gamma compensation for underexposed shadows or overexposed highlights
	gamma := 1.0
	if report.IsUnderexposed {
		if report.MeanBrightness < 60.0 {
			gamma = 0.70
		} else if report.MeanBrightness < 80.0 {
			gamma = 0.80
		} else {
			gamma = 0.88
		}
	} else if report.IsOverexposed {
		gamma = 1.15
	}

	// Pre-build 256-entry lookup table (LUT)
	var lut [256]uint8
	rangeSpan := float64(pHigh - pLow)
	for v := 0; v < 256; v++ {
		var val float64
		if v <= int(pLow) {
			val = 0.0
		} else if v >= int(pHigh) {
			val = 1.0
		} else {
			val = float64(v-int(pLow)) / rangeSpan
		}
		if gamma != 1.0 && val > 0.0 && val < 1.0 {
			val = math.Pow(val, gamma)
		}
		clamped := math.Round(val * 255.0)
		if clamped < 0 {
			clamped = 0
		} else if clamped > 255 {
			clamped = 255
		}
		lut[v] = uint8(clamped)
	}

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	minX := bounds.Min.X
	minY := bounds.Min.Y

	// Fast path for *image.RGBA
	if rgba, ok := img.(*image.RGBA); ok {
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := rgba.RGBAAt(minX+x, minY+y)
				dst.SetRGBA(x, y, color.RGBA{
					R: lut[c.R],
					G: lut[c.G],
					B: lut[c.B],
					A: c.A,
				})
			}
		}
		return dst
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(minX+x, minY+y).RGBA()
			dst.SetRGBA(x, y, color.RGBA{
				R: lut[uint8(r>>8)],
				G: lut[uint8(g>>8)],
				B: lut[uint8(b>>8)],
				A: uint8(a >> 8),
			})
		}
	}

	return dst
}
