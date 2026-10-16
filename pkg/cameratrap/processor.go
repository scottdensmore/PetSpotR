package cameratrap

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"math"
	"strings"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/imaging"
)

const maxFrameBytes = 5 * 1024 * 1024 // 5MB

// FrameAnalysisResult encapsulates the edge-frame analysis pipeline output.
type FrameAnalysisResult struct {
	FrameDHash       uint64
	FrameDHashHex    string
	HammingDistance  int
	IsDuplicate      bool
	IsNightVisionIR  bool
	IREnhanced       bool
	ROI              domain.ROIBoundingBox
	SaliencyScore    float64
	SnapshotDataURI  string
	CropThumbnailURI string
}

// DetectNightVisionIR inspects up to a 16x16 pixel grid for monochromatic IR signature
// (low chrominance spread) or low-light nocturnal luminance.
func DetectNightVisionIR(img image.Image) bool {
	if img == nil {
		return false
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return false
	}

	gridX := 16
	if w < gridX {
		gridX = w
	}
	gridY := 16
	if h < gridY {
		gridY = h
	}

	var totalChroma float64
	var totalLum float64
	var count int

	for gy := 0; gy < gridY; gy++ {
		sy := bounds.Min.Y + (gy*h+h/2)/gridY
		if sy >= bounds.Max.Y {
			sy = bounds.Max.Y - 1
		}
		for gx := 0; gx < gridX; gx++ {
			sx := bounds.Min.X + (gx*w+w/2)/gridX
			if sx >= bounds.Max.X {
				sx = bounds.Max.X - 1
			}
			r, g, b, _ := img.At(sx, sy).RGBA()
			rf := float64(r >> 8)
			gf := float64(g >> 8)
			bf := float64(b >> 8)

			chroma := (math.Abs(rf-gf) + math.Abs(gf-bf) + math.Abs(rf-bf)) / 3.0
			lum := 0.299*rf + 0.587*gf + 0.114*bf

			totalChroma += chroma
			totalLum += lum
			count++
		}
	}

	if count == 0 {
		return false
	}

	meanChroma := totalChroma / float64(count)
	meanLum := totalLum / float64(count)

	return meanChroma < 12.0 || meanLum < 55.0
}

// DetectMotionROI divides an image into an 8x8 saliency grid, compares each cell against
// the outer perimeter baseline luminance and internal contrast, and returns a normalized
// bounding box plus saliency score.
func DetectMotionROI(img image.Image) (domain.ROIBoundingBox, float64) {
	defaultBox := domain.ROIBoundingBox{X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5}
	if img == nil {
		return defaultBox, 0.0
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return defaultBox, 0.0
	}

	var cellLum [8][8]float64
	var cellContrast [8][8]float64

	for gy := 0; gy < 8; gy++ {
		y0 := bounds.Min.Y + (gy*h)/8
		y1 := bounds.Min.Y + ((gy+1)*h)/8
		if y1 <= y0 {
			y1 = y0 + 1
		}
		if y1 > bounds.Max.Y {
			y1 = bounds.Max.Y
		}

		for gx := 0; gx < 8; gx++ {
			x0 := bounds.Min.X + (gx*w)/8
			x1 := bounds.Min.X + ((gx+1)*w)/8
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if x1 > bounds.Max.X {
				x1 = bounds.Max.X
			}

			var sum float64
			var sumSq float64
			var n int
			stepY := (y1 - y0) / 4
			if stepY < 1 {
				stepY = 1
			}
			stepX := (x1 - x0) / 4
			if stepX < 1 {
				stepX = 1
			}

			for py := y0; py < y1; py += stepY {
				for px := x0; px < x1; px += stepX {
					r, g, b, _ := img.At(px, py).RGBA()
					lum := 0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8)
					sum += lum
					sumSq += lum * lum
					n++
				}
			}
			if n > 0 {
				mean := sum / float64(n)
				variance := (sumSq / float64(n)) - (mean * mean)
				if variance < 0 {
					variance = 0
				}
				cellLum[gy][gx] = mean
				cellContrast[gy][gx] = math.Sqrt(variance)
			}
		}
	}

	var bgSum float64
	var bgCount int
	for gy := 0; gy < 8; gy++ {
		for gx := 0; gx < 8; gx++ {
			if gx == 0 || gx == 7 || gy == 0 || gy == 7 {
				bgSum += cellLum[gy][gx]
				bgCount++
			}
		}
	}
	bgLum := bgSum / float64(bgCount)

	var energy [8][8]float64
	maxEnergy := 0.0
	for gy := 0; gy < 8; gy++ {
		for gx := 0; gx < 8; gx++ {
			e := math.Abs(cellLum[gy][gx]-bgLum)*1.5 + cellContrast[gy][gx]
			energy[gy][gx] = e
			if e > maxEnergy {
				maxEnergy = e
			}
		}
	}

	if maxEnergy < 5.0 {
		return defaultBox, 0.15
	}

	threshold := maxEnergy * 0.45
	minGX, maxGX := 7, 0
	minGY, maxGY := 7, 0
	found := false

	for gy := 0; gy < 8; gy++ {
		for gx := 0; gx < 8; gx++ {
			if energy[gy][gx] >= threshold {
				if gx < minGX {
					minGX = gx
				}
				if gx > maxGX {
					maxGX = gx
				}
				if gy < minGY {
					minGY = gy
				}
				if gy > maxGY {
					maxGY = gy
				}
				found = true
			}
		}
	}

	if !found {
		return defaultBox, 0.2
	}

	const margin = 0.04
	x := float64(minGX)/8.0 - margin
	y := float64(minGY)/8.0 - margin
	width := float64(maxGX-minGX+1)/8.0 + 2*margin
	height := float64(maxGY-minGY+1)/8.0 + 2*margin

	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if width < 0.15 {
		width = 0.15
	}
	if height < 0.15 {
		height = 0.15
	}
	if x+width > 1.0 {
		if width > 1.0 {
			width = 1.0
			x = 0
		} else {
			x = 1.0 - width
		}
	}
	if y+height > 1.0 {
		if height > 1.0 {
			height = 1.0
			y = 0
		} else {
			y = 1.0 - height
		}
	}

	roi := domain.ROIBoundingBox{
		X:      round3(x),
		Y:      round3(y),
		Width:  round3(width),
		Height: round3(height),
	}
	saliency := round3(math.Min(1.0, math.Max(0.2, maxEnergy/120.0)))
	return roi, saliency
}

func round3(v float64) float64 {
	return math.Round(v*1000.0) / 1000.0
}

// DecodeDataURIImage decodes a base64 data URI (or raw base64 payload) into an image.Image
// and raw image bytes, enforcing a 5MB size cap.
func DecodeDataURIImage(dataURI string) (image.Image, []byte, error) {
	trimmed := strings.TrimSpace(dataURI)
	if trimmed == "" {
		return nil, nil, errors.New("cameratrap: empty image data URI")
	}

	b64Payload := trimmed
	if strings.HasPrefix(trimmed, "data:") {
		idx := strings.Index(trimmed, ",")
		if idx < 0 || idx+1 >= len(trimmed) {
			return nil, nil, errors.New("cameratrap: malformed data URI")
		}
		b64Payload = trimmed[idx+1:]
	}

	if len(b64Payload) > maxFrameBytes*4/3+1024 {
		return nil, nil, errors.New("cameratrap: base64 frame exceeds 5MB limit")
	}

	rawBytes, err := base64.StdEncoding.DecodeString(strings.TrimSpace(b64Payload))
	if err != nil {
		rawBytes, err = base64.RawStdEncoding.DecodeString(strings.TrimSpace(b64Payload))
		if err != nil {
			return nil, nil, errors.New("cameratrap: invalid base64 image payload")
		}
	}

	if len(rawBytes) == 0 || len(rawBytes) > maxFrameBytes {
		return nil, nil, errors.New("cameratrap: decoded image bytes out of bounds")
	}

	img, _, err := image.Decode(bytes.NewReader(rawBytes))
	if err != nil {
		return nil, nil, err
	}
	return img, rawBytes, nil
}

// ProcessEdgeFrame runs the full pure-Go edge frame pipeline:
// 1. Decode data URI
// 2. Compute 64-bit perceptual dHash & Hamming deduplication against prevHash
// 3. Detect Night-Vision IR and enhance contrast via pkg/imaging
// 4. Localize motion saliency ROI and crop JPEG thumbnail
func ProcessEdgeFrame(dataURI string, prevHash uint64, hasPrev bool) (*FrameAnalysisResult, error) {
	img, rawBytes, err := DecodeDataURIImage(dataURI)
	if err != nil {
		return nil, err
	}

	dHash := ComputeDHash(img)
	isDup, hamming := IsDuplicateFrame(prevHash, dHash, hasPrev, DefaultDuplicateMaxHamming)
	isIR := DetectNightVisionIR(img)

	workingImg := img
	irEnhanced := false
	snapshotURI := strings.TrimSpace(dataURI)
	if !strings.HasPrefix(snapshotURI, "data:image/") {
		snapshotURI = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(rawBytes)
	}

	if isIR {
		if enhancedBytes, enhErr := imaging.EnhanceImage(rawBytes); enhErr == nil && len(enhancedBytes) > 0 {
			if enhImg, _, decErr := image.Decode(bytes.NewReader(enhancedBytes)); decErr == nil && enhImg != nil {
				workingImg = enhImg
				irEnhanced = true
				snapshotURI = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(enhancedBytes)
			}
		}
	}

	roi, saliency := DetectMotionROI(workingImg)
	cropURI := encodeCropThumbnail(workingImg, roi)

	return &FrameAnalysisResult{
		FrameDHash:       dHash,
		FrameDHashHex:    FormatDHashHex(dHash),
		HammingDistance:  hamming,
		IsDuplicate:      isDup,
		IsNightVisionIR:  isIR,
		IREnhanced:       irEnhanced,
		ROI:              roi,
		SaliencyScore:    saliency,
		SnapshotDataURI:  snapshotURI,
		CropThumbnailURI: cropURI,
	}, nil
}

func encodeCropThumbnail(img image.Image, roi domain.ROIBoundingBox) string {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w <= 0 || h <= 0 {
		return ""
	}

	x0 := bounds.Min.X + int(math.Floor(roi.X*float64(w)))
	y0 := bounds.Min.Y + int(math.Floor(roi.Y*float64(h)))
	x1 := bounds.Min.X + int(math.Ceil((roi.X+roi.Width)*float64(w)))
	y1 := bounds.Min.Y + int(math.Ceil((roi.Y+roi.Height)*float64(h)))

	if x0 < bounds.Min.X {
		x0 = bounds.Min.X
	}
	if y0 < bounds.Min.Y {
		y0 = bounds.Min.Y
	}
	if x1 > bounds.Max.X {
		x1 = bounds.Max.X
	}
	if y1 > bounds.Max.Y {
		y1 = bounds.Max.Y
	}
	if x1 <= x0 {
		x1 = bounds.Max.X
		x0 = bounds.Min.X
	}
	if y1 <= y0 {
		y1 = bounds.Max.Y
		y0 = bounds.Min.Y
	}

	cropRect := image.Rect(0, 0, x1-x0, y1-y0)
	dst := image.NewRGBA(cropRect)
	draw.Draw(dst, cropRect, img, image.Point{X: x0, Y: y0}, draw.Src)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}
