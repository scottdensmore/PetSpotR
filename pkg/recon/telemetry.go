package recon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

var (
	reTimeRange = regexp.MustCompile(`(\d{1,2}):(\d{2}):(\d{2})[,.](\d{3})\s*-->\s*(\d{1,2}):(\d{2}):(\d{2})[,.](\d{3})`)
	reDate      = regexp.MustCompile(`(\d{4})[./-](\d{2})[./-](\d{2})[ T](\d{2}):(\d{2}):(\d{2})(?:[.,](\d{1,6}))?`)

	reDLat      = regexp.MustCompile(`(?i)\bdlatitude\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	reLat       = regexp.MustCompile(`(?i)\blatitude\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	reShortLat  = regexp.MustCompile(`(?i)\blat\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)

	reDLon      = regexp.MustCompile(`(?i)\bdlongitude\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	reLon       = regexp.MustCompile(`(?i)\blongitude\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	reShortLon  = regexp.MustCompile(`(?i)\b(?:long|lng)\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)

	reGPS       = regexp.MustCompile(`(?i)\bGPS\s*\(\s*([+-]?\d+(?:\.\d+)?)\s*,\s*([+-]?\d+(?:\.\d+)?)(?:\s*,\s*([+-]?\d+(?:\.\d+)?))?\s*\)`)

	reRelAlt    = regexp.MustCompile(`(?i)\b(?:rel_alt|relative_altitude|rel_altitude)\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	reAlt       = regexp.MustCompile(`(?i)\b(?:altitude|alt|height)\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)

	reHeading   = regexp.MustCompile(`(?i)\b(?:heading|compass)\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	rePitch     = regexp.MustCompile(`(?i)\b(?:gimbal_pitch|pitch)\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	reRoll      = regexp.MustCompile(`(?i)\b(?:gimbal_roll|roll)\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	reYaw       = regexp.MustCompile(`(?i)\b(?:gimbal_yaw|yaw)\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)

	reSpeed     = regexp.MustCompile(`(?i)\b(?:ground_speed|groundspeed|hspeed|speed)\s*[:=]\s*([+-]?\d+(?:\.\d+)?)`)
	reBat       = regexp.MustCompile(`(?i)\b(?:battery|bat)\s*[:=]\s*(\d+)`)

	reCoordinates = regexp.MustCompile(`(?s)<coordinates>(.*?)</coordinates>`)
	reWhen        = regexp.MustCompile(`<when>(.*?)</when>`)
)

// ParseDJISRT scans an SRT stream from DJI/Autel UAV flight logs and extracts structured waypoints.
func ParseDJISRT(r io.Reader) ([]domain.DroneWaypoint, error) {
	if r == nil {
		return nil, errors.New("reader cannot be nil")
	}

	scanner := bufio.NewScanner(r)
	var blocks [][]string
	var currentBlock []string

	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			if len(currentBlock) > 0 {
				blocks = append(blocks, currentBlock)
				currentBlock = nil
			}
		} else {
			currentBlock = append(currentBlock, line)
		}
	}
	if len(currentBlock) > 0 {
		blocks = append(blocks, currentBlock)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SRT stream: %w", err)
	}

	var waypoints []domain.DroneWaypoint
	var baseHomeDate time.Time

	for _, block := range blocks {
		wp, parsedTime, hasTime, hasCoords := parseSRTBlock(block, baseHomeDate)
		if hasTime && !parsedTime.IsZero() && baseHomeDate.IsZero() {
			baseHomeDate = parsedTime
		}
		if hasCoords {
			waypoints = append(waypoints, wp)
		}
	}

	if len(waypoints) == 0 {
		return nil, errors.New("no valid telemetry waypoints found in SRT stream")
	}

	return waypoints, nil
}

// parseSRTBlock processes a single SRT subtitle block into a DroneWaypoint.
func parseSRTBlock(lines []string, fallbackBaseDate time.Time) (wp domain.DroneWaypoint, parsedDate time.Time, hasDate bool, hasCoords bool) {
	var startDuration time.Duration
	var payloadLines []string

	for _, line := range lines {
		if m := reTimeRange.FindStringSubmatch(line); m != nil {
			h, _ := strconv.Atoi(m[1])
			min, _ := strconv.Atoi(m[2])
			sec, _ := strconv.Atoi(m[3])
			ms, _ := strconv.Atoi(m[4])
			startDuration = time.Duration(h)*time.Hour +
				time.Duration(min)*time.Minute +
				time.Duration(sec)*time.Second +
				time.Duration(ms)*time.Millisecond
			continue
		}
		// If line is just the integer subtitle sequence number, ignore it
		if _, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && len(payloadLines) == 0 {
			continue
		}
		payloadLines = append(payloadLines, line)
	}

	payload := strings.Join(payloadLines, " ")

	// 1. Date / Timestamp parsing
	if dm := reDate.FindStringSubmatch(payload); dm != nil {
		y, _ := strconv.Atoi(dm[1])
		mo, _ := strconv.Atoi(dm[2])
		d, _ := strconv.Atoi(dm[3])
		h, _ := strconv.Atoi(dm[4])
		min, _ := strconv.Atoi(dm[5])
		s, _ := strconv.Atoi(dm[6])
		ns := 0
		if len(dm) > 7 && dm[7] != "" {
			fracStr := dm[7]
			if len(fracStr) > 9 {
				fracStr = fracStr[:9]
			}
			for len(fracStr) < 9 {
				fracStr += "0"
			}
			ns, _ = strconv.Atoi(fracStr)
		}
		parsedDate = time.Date(y, time.Month(mo), d, h, min, s, ns, time.UTC)
		hasDate = true

		// If line mentions HOME, treat parsedDate as sortie takeoff time and add startDuration
		if strings.Contains(strings.ToUpper(payload), "HOME") {
			wp.Timestamp = parsedDate.Add(startDuration)
		} else if ns == 0 && startDuration > 0 {
			// If subsecond precision is omitted in text timestamp, add subsecond portion of startDuration
			subSecOffset := startDuration % time.Second
			wp.Timestamp = parsedDate.Add(subSecOffset)
		} else {
			wp.Timestamp = parsedDate
		}
	} else if !fallbackBaseDate.IsZero() {
		wp.Timestamp = fallbackBaseDate.Add(startDuration)
	} else {
		wp.Timestamp = time.Unix(0, 0).Add(startDuration).UTC()
	}

	// 2. Latitude & Longitude
	var hasLat, hasLon bool

	if m := reDLat.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.Latitude = val
			hasLat = true
		}
	} else if m := reLat.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.Latitude = val
			hasLat = true
		}
	} else if m := reShortLat.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.Latitude = val
			hasLat = true
		}
	}

	if m := reDLon.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.Longitude = val
			hasLon = true
		}
	} else if m := reLon.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.Longitude = val
			hasLon = true
		}
	} else if m := reShortLon.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.Longitude = val
			hasLon = true
		}
	}

	// GPS(lng, lat[, sats]) fallback
	if (!hasLat || !hasLon) {
		if m := reGPS.FindStringSubmatch(payload); m != nil {
			if !hasLon {
				if val, err := strconv.ParseFloat(m[1], 64); err == nil {
					wp.Longitude = val
					hasLon = true
				}
			}
			if !hasLat {
				if val, err := strconv.ParseFloat(m[2], 64); err == nil {
					wp.Latitude = val
					hasLat = true
				}
			}
		}
	}

	hasCoords = hasLat && hasLon

	// 3. Altitude (Prefer relative altitude / AGL)
	if m := reRelAlt.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.AltitudeAGL = val
		}
	} else if m := reAlt.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.AltitudeAGL = val
		}
	}

	// 4. Heading & Gimbal Angles
	if m := reHeading.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.HeadingDeg = val
		}
	}

	if m := rePitch.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.GimbalPitchDeg = val
		}
	}

	if m := reRoll.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.GimbalRollDeg = val
		}
	}

	if m := reYaw.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.GimbalYawDeg = val
		}
	}

	// 5. Ground Speed & Battery
	if m := reSpeed.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil {
			wp.GroundSpeedMps = val
		}
	}

	if m := reBat.FindStringSubmatch(payload); m != nil {
		if val, err := strconv.Atoi(m[1]); err == nil {
			wp.BatteryPercent = val
		}
	}

	return wp, parsedDate, hasDate, hasCoords
}

// ParseFlightLog parses drone flight logs supporting SRT, KML, and GeoJSON formats.
func ParseFlightLog(content []byte, filename string) ([]domain.DroneWaypoint, error) {
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 {
		return nil, errors.New("flight log content is empty")
	}

	lowerName := strings.ToLower(filename)
	if strings.HasSuffix(lowerName, ".srt") {
		return ParseDJISRT(bytes.NewReader(content))
	}
	if strings.HasSuffix(lowerName, ".kml") {
		return parseKML(content)
	}
	if strings.HasSuffix(lowerName, ".json") || strings.HasSuffix(lowerName, ".geojson") {
		return parseGeoJSON(content)
	}

	// Content sniffing
	if bytes.HasPrefix(trimmed, []byte("{")) {
		if wps, err := parseGeoJSON(content); err == nil && len(wps) > 0 {
			return wps, nil
		}
	}

	if bytes.Contains(trimmed, []byte("<kml")) || bytes.Contains(trimmed, []byte("<coordinates")) {
		if wps, err := parseKML(content); err == nil && len(wps) > 0 {
			return wps, nil
		}
	}

	return ParseDJISRT(bytes.NewReader(content))
}

func parseKML(content []byte) ([]domain.DroneWaypoint, error) {
	coordMatches := reCoordinates.FindAllStringSubmatch(string(content), -1)
	if len(coordMatches) == 0 {
		return nil, errors.New("no <coordinates> found in KML content")
	}

	var waypoints []domain.DroneWaypoint
	var timestamps []time.Time
	whenMatches := reWhen.FindAllStringSubmatch(string(content), -1)
	for _, wm := range whenMatches {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(wm[1])); err == nil {
			timestamps = append(timestamps, t)
		}
	}

	for _, cm := range coordMatches {
		rawCoords := cm[1]
		tokens := strings.Fields(rawCoords)
		for _, token := range tokens {
			parts := strings.Split(strings.TrimSpace(token), ",")
			if len(parts) >= 2 {
				lng, errLng := strconv.ParseFloat(parts[0], 64)
				lat, errLat := strconv.ParseFloat(parts[1], 64)
				if errLng != nil || errLat != nil {
					continue
				}
				alt := 0.0
				if len(parts) >= 3 {
					alt, _ = strconv.ParseFloat(parts[2], 64)
				}

				wp := domain.DroneWaypoint{
					Latitude:       lat,
					Longitude:      lng,
					AltitudeAGL:    alt,
					GimbalPitchDeg: -90.0, // Standard nadir default for aerial KML surveys
				}

				if len(waypoints) < len(timestamps) {
					wp.Timestamp = timestamps[len(waypoints)]
				} else {
					wp.Timestamp = time.Now().UTC()
				}

				waypoints = append(waypoints, wp)
			}
		}
	}

	if len(waypoints) == 0 {
		return nil, errors.New("no valid coordinates parsed from KML")
	}

	// Compute headings between successive waypoints if not set
	for i := range waypoints {
		if waypoints[i].HeadingDeg == 0 {
			if i < len(waypoints)-1 {
				waypoints[i].HeadingDeg = computeBearing(
					waypoints[i].Latitude, waypoints[i].Longitude,
					waypoints[i+1].Latitude, waypoints[i+1].Longitude,
				)
			} else if i > 0 {
				waypoints[i].HeadingDeg = waypoints[i-1].HeadingDeg
			}
		}
	}

	return waypoints, nil
}

type geoJSONDoc struct {
	Type        string           `json:"type"`
	Coordinates json.RawMessage  `json:"coordinates"`
	Features    []geoJSONFeature `json:"features"`
	Properties  map[string]any   `json:"properties"`
}

type geoJSONFeature struct {
	Type       string          `json:"type"`
	Geometry   geoJSONGeometry `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}

type geoJSONGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

func parseGeoJSON(content []byte) ([]domain.DroneWaypoint, error) {
	var doc geoJSONDoc
	if err := json.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("invalid GeoJSON: %w", err)
	}

	var waypoints []domain.DroneWaypoint

	if strings.EqualFold(doc.Type, "FeatureCollection") {
		for _, feat := range doc.Features {
			wps := parseFeature(feat)
			waypoints = append(waypoints, wps...)
		}
	} else if strings.EqualFold(doc.Type, "Feature") {
		feat := geoJSONFeature{
			Type: doc.Type,
			Geometry: geoJSONGeometry{
				Coordinates: doc.Coordinates,
			},
			Properties: doc.Properties,
		}
		waypoints = append(waypoints, parseFeature(feat)...)
	}

	if len(waypoints) == 0 {
		return nil, errors.New("no valid coordinates parsed from GeoJSON")
	}

	return waypoints, nil
}

func parseFeature(feat geoJSONFeature) []domain.DroneWaypoint {
	var waypoints []domain.DroneWaypoint

	heading := extractFloat(feat.Properties, "heading", "headingDeg")
	pitch := extractFloat(feat.Properties, "pitch", "gimbalPitch", "gimbalPitchDeg")
	roll := extractFloat(feat.Properties, "roll", "gimbalRoll", "gimbalRollDeg")
	yaw := extractFloat(feat.Properties, "yaw", "gimbalYaw", "gimbalYawDeg")

	// Try LineString ([][2+]float64)
	var lineCoords [][]float64
	if err := json.Unmarshal(feat.Geometry.Coordinates, &lineCoords); err == nil && len(lineCoords) > 0 {
		for _, pt := range lineCoords {
			if len(pt) < 2 {
				continue
			}
			wp := domain.DroneWaypoint{
				Longitude:      pt[0],
				Latitude:       pt[1],
				HeadingDeg:     heading,
				GimbalPitchDeg: pitch,
				GimbalRollDeg:  roll,
				GimbalYawDeg:   yaw,
				Timestamp:      time.Now().UTC(),
			}
			if len(pt) >= 3 {
				wp.AltitudeAGL = pt[2]
			}
			waypoints = append(waypoints, wp)
		}
		return waypoints
	}

	// Try Point ([2+]float64)
	var ptCoords []float64
	if err := json.Unmarshal(feat.Geometry.Coordinates, &ptCoords); err == nil && len(ptCoords) >= 2 {
		wp := domain.DroneWaypoint{
			Longitude:      ptCoords[0],
			Latitude:       ptCoords[1],
			HeadingDeg:     heading,
			GimbalPitchDeg: pitch,
			GimbalRollDeg:  roll,
			GimbalYawDeg:   yaw,
			Timestamp:      time.Now().UTC(),
		}
		if len(ptCoords) >= 3 {
			wp.AltitudeAGL = ptCoords[2]
		}
		waypoints = append(waypoints, wp)
	}

	return waypoints
}

func extractFloat(props map[string]any, keys ...string) float64 {
	if props == nil {
		return 0.0
	}
	for _, k := range keys {
		if val, ok := props[k]; ok {
			switch v := val.(type) {
			case float64:
				return v
			case int:
				return float64(v)
			case string:
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					return f
				}
			}
		}
	}
	return 0.0
}

func computeBearing(lat1, lon1, lat2, lon2 float64) float64 {
	dLon := (lon2 - lon1) * math.Pi / 180.0
	lat1Rad := lat1 * math.Pi / 180.0
	lat2Rad := lat2 * math.Pi / 180.0

	y := math.Sin(dLon) * math.Cos(lat2Rad)
	x := math.Cos(lat1Rad)*math.Sin(lat2Rad) - math.Sin(lat1Rad)*math.Cos(lat2Rad)*math.Cos(dLon)
	brng := math.Atan2(y, x) * 180.0 / math.Pi
	if brng < 0 {
		brng += 360.0
	}
	return brng
}
