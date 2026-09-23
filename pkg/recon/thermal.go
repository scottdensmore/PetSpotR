package recon

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"sort"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	minHotspotPixels          = 3
	minAnimalGroundDimMeters  = 0.2 // Ground sample size of smallest pet (e.g. small cat)
	maxAnimalGroundDimMeters  = 5.0 // Ground sample size upper threshold (accommodating thermal bloom)
	maxAnimalGroundAreaSqM    = 25.0
	minLocalRingContrastRatio = 0.15 // Local contrast between candidate blob and ambient ring buffer (>15%)
)

// AnalyzeThermalImage performs deterministic pure Go radiometric thermal analysis on aerial frames.
// It extracts candidate heat anomalies using adaptive standard deviation thresholding,
// 8-connected component contouring, GSD scale filtering, and local ring contrast rejection.
func AnalyzeThermalImage(
	img image.Image,
	palette domain.ThermalPalette,
	wp domain.DroneWaypoint,
	cam domain.CameraIntrinsics,
) ([]domain.ThermalHotspot, error) {
	if img == nil {
		return nil, errors.New("recon: thermal image cannot be nil")
	}
	if wp.AltitudeAGL <= 0 {
		return nil, errors.New("recon: drone altitude AGL must be positive")
	}

	cam = normalizeIntrinsics(cam)
	if err := validateIntrinsics(cam); err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, errors.New("recon: image dimensions must be positive")
	}

	if palette == "" {
		palette = domain.PaletteWhiteHot
	}

	totalPixels := width * height
	grid := make([]float64, totalPixels)

	var sumVal float64
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := img.At(bounds.Min.X+x, bounds.Min.Y+y)
			val := pixelThermalIntensity(c, palette)
			idx := y*width + x
			grid[idx] = val
			sumVal += val
		}
	}

	mean := sumVal / float64(totalPixels)
	var varianceSum float64
	for _, v := range grid {
		diff := v - mean
		varianceSum += diff * diff
	}
	stdDev := math.Sqrt(varianceSum / float64(totalPixels))
	if stdDev < 1e-4 {
		return []domain.ThermalHotspot{}, nil
	}

	threshold := mean + 2.2*stdDev

	mask := make([]bool, totalPixels)
	for i, v := range grid {
		mask[i] = v > threshold
	}

	// GSD calculation in meters per pixel
	hRad := (cam.HFOV * math.Pi / 180.0) / 2.0
	vRad := (cam.VFOV * math.Pi / 180.0) / 2.0
	gsdX := (2.0 * wp.AltitudeAGL * math.Tan(hRad)) / float64(width)
	gsdY := (2.0 * wp.AltitudeAGL * math.Tan(vRad)) / float64(height)

	visited := make([]bool, totalPixels)
	var hotspots []domain.ThermalHotspot

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			if !mask[idx] || visited[idx] {
				continue
			}

			// 8-connected flood fill
			queue := [][2]int{{x, y}}
			visited[idx] = true

			minX, maxX := x, x
			minY, maxY := y, y
			sumX, sumY := 0, 0
			var blobSumVal float64
			points := make([][2]int, 0, 16)

			for len(queue) > 0 {
				curr := queue[0]
				queue = queue[1:]
				cx, cy := curr[0], curr[1]
				cidx := cy*width + cx
				val := grid[cidx]

				points = append(points, curr)
				if cx < minX {
					minX = cx
				}
				if cx > maxX {
					maxX = cx
				}
				if cy < minY {
					minY = cy
				}
				if cy > maxY {
					maxY = cy
				}
				sumX += cx
				sumY += cy
				blobSumVal += val

				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if dx == 0 && dy == 0 {
							continue
						}
						nx, ny := cx+dx, cy+dy
						if nx >= 0 && nx < width && ny >= 0 && ny < height {
							nidx := ny*width + nx
							if mask[nidx] && !visited[nidx] {
								visited[nidx] = true
								queue = append(queue, [2]int{nx, ny})
							}
						}
					}
				}
			}

			// Minimum pixel count filter
			if len(points) < minHotspotPixels {
				continue
			}

			// GSD physical dimension filter
			blobWidthMeters := float64(maxX-minX+1) * gsdX
			blobHeightMeters := float64(maxY-minY+1) * gsdY
			maxDimMeters := math.Max(blobWidthMeters, blobHeightMeters)
			minDimMeters := math.Min(blobWidthMeters, blobHeightMeters)
			blobAreaSqM := blobWidthMeters * blobHeightMeters

			if maxDimMeters < minAnimalGroundDimMeters || maxDimMeters > maxAnimalGroundDimMeters {
				continue
			}
			if blobAreaSqM > maxAnimalGroundAreaSqM {
				continue
			}

			// Ambient ring buffer evaluation
			inBlob := make(map[int]bool, len(points))
			for _, p := range points {
				inBlob[p[1]*width+p[0]] = true
			}

			const ringMargin = 2
			ringMinX := max(0, minX-ringMargin)
			ringMaxX := min(width-1, maxX+ringMargin)
			ringMinY := max(0, minY-ringMargin)
			ringMaxY := min(height-1, maxY+ringMargin)

			var ringSum float64
			var ringCount int
			for ry := ringMinY; ry <= ringMaxY; ry++ {
				for rx := ringMinX; rx <= ringMaxX; rx++ {
					ridx := ry*width + rx
					if !inBlob[ridx] {
						ringSum += grid[ridx]
						ringCount++
					}
				}
			}

			if ringCount == 0 {
				continue
			}

			blobMean := blobSumVal / float64(len(points))
			ringMean := ringSum / float64(ringCount)
			contrast := (blobMean - ringMean) / math.Max(blobMean, 1.0)
			if contrast < minLocalRingContrastRatio {
				continue
			}

			// Normalized bounding box
			bbox := domain.NormalizedBox{
				X:      float64(minX) / float64(width),
				Y:      float64(minY) / float64(height),
				Width:  float64(maxX-minX+1) / float64(width),
				Height: float64(maxY-minY+1) / float64(height),
			}

			// Projected ground coordinate via raycasting from centroid
			centroidU := float64(sumX) / (float64(len(points)) * float64(width))
			centroidV := float64(sumY) / (float64(len(points)) * float64(height))
			lat, lng, err := RaycastPixelToGround(centroidU, centroidV, wp, cam)
			if err != nil {
				lat = wp.Latitude
				lng = wp.Longitude
			}

			// Estimated temperature mapping
			estTemp := 15.0 + (blobMean/255.0)*25.0

			// Confidence calculation
			intensityFactor := blobMean / 255.0
			contrastFactor := math.Min(contrast/0.5, 1.0)
			aspectFactor := minDimMeters / math.Max(maxDimMeters, 1e-6)
			confidence := 0.35*intensityFactor + 0.45*contrastFactor + 0.20*aspectFactor
			confidence = math.Max(0.0, math.Min(1.0, confidence))

			// Thumbnail generation with margin
			const cropPad = 4
			cropRect := image.Rect(
				bounds.Min.X+max(0, minX-cropPad),
				bounds.Min.Y+max(0, minY-cropPad),
				bounds.Min.X+min(width, maxX+1+cropPad),
				bounds.Min.Y+min(height, maxY+1+cropPad),
			)
			subImg := cropImage(img, cropRect)
			thumbBase64, _ := encodeJPEG(subImg)

			ts := wp.Timestamp
			if ts.IsZero() {
				ts = time.Now().UTC()
			}

			hotspotID := generateHotspotID()

			hotspots = append(hotspots, domain.ThermalHotspot{
				ID:              hotspotID,
				Timestamp:       ts,
				Latitude:        lat,
				Longitude:       lng,
				EstimatedTempC:  math.Round(estTemp*10.0) / 10.0,
				ConfidenceScore: math.Round(confidence*1000.0) / 1000.0,
				Palette:         palette,
				BoundingBox:     bbox,
				ThumbnailBase64: thumbBase64,
				Status:          domain.HotspotUnverified,
				Classification:  "Thermal Heat Anomaly",
			})
		}
	}

	sort.Slice(hotspots, func(i, j int) bool {
		return hotspots[i].ConfidenceScore > hotspots[j].ConfidenceScore
	})

	return hotspots, nil
}

func pixelThermalIntensity(c color.Color, palette domain.ThermalPalette) float64 {
	r, g, b, _ := c.RGBA()
	R := float64(r >> 8)
	G := float64(g >> 8)
	B := float64(b >> 8)

	switch palette {
	case domain.PaletteBlackHot:
		return 255.0 - (0.299*R + 0.587*G + 0.114*B)

	case domain.PaletteIronbow:
		if R > 230 && G > 230 && B > 230 {
			return 230.0 + (R+G+B-690.0)/3.0
		}
		if R > 200 && G > 160 && B < 80 {
			return 175.0 + ((G-160.0)/95.0)*50.0
		}
		if R > 150 && B < 100 {
			return 110.0 + ((R-150.0)/105.0)*60.0
		}
		if R > 80 && B > 40 {
			return 50.0 + (R/255.0)*50.0
		}
		return (0.299*R + 0.587*G + 0.114*B) * 0.35

	case domain.PaletteWhiteHot:
		fallthrough
	default:
		return 0.299*R + 0.587*G + 0.114*B
	}
}

func cropImage(img image.Image, rect image.Rectangle) image.Image {
	rect = rect.Intersect(img.Bounds())
	if rect.Empty() {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	if si, ok := img.(subImager); ok {
		return si.SubImage(rect)
	}
	dst := image.NewRGBA(image.Rect(0, 0, rect.Dx(), rect.Dy()))
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dst.Set(x-rect.Min.X, y-rect.Min.Y, img.At(x, y))
		}
	}
	return dst
}

func encodeJPEG(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func generateHotspotID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("hotspot-%x", b)
}
