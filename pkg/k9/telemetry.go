package k9

import (
	"errors"
	"fmt"
	"html"
	"math"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// IsWaypointInScentPlume determines whether a GPS coordinate falls within the active downwind scent cone.
func IsWaypointInScentPlume(lat, lng float64, plume domain.ScentPlumePolygon) bool {
	distM, bearingDeg := distanceAndBearing(plume.OriginLat, plume.OriginLng, lat, lng)
	if distM <= 25.0 {
		return true
	}
	if distM > plume.EffectiveLengthM*1.05 {
		return false
	}

	diffDeg := math.Abs(bearingDeg - plume.DownwindAzimuthDeg)
	if diffDeg > 180.0 {
		diffDeg = 360.0 - diffDeg
	}

	return diffDeg <= (plume.SpreadAngleDeg/2.0)+3.0
}

// RecalculateMissionMetrics re-evaluates atmospheric scent viability, downwind plume polygon,
// waypoint plume containment, and Probability of Detection (POD) for a K9 mission.
func RecalculateMissionMetrics(mission domain.K9ScentMission) domain.K9ScentMission {
	mission.ViabilityScore, mission.ViabilityRating, mission.PawThermalHazard, mission.SafetyAdvisory = EvaluateScentViability(mission.Environment)
	mission.PlumeCone = ComputeGaussianScentPlume(mission.LKPOriginLat, mission.LKPOriginLng, mission.Environment, mission.ViabilityScore)

	if len(mission.Waypoints) == 0 {
		mission.PlumeIntersectionPct = 0
		mission.EstimatedPOD = math.Round(float64(mission.ViabilityScore)*0.35*10) / 10
		return mission
	}

	inPlumeCount := 0
	positiveAlerts := 0

	for i := range mission.Waypoints {
		w := mission.Waypoints[i]
		inPlume := IsWaypointInScentPlume(w.Lat, w.Lng, mission.PlumeCone)
		mission.Waypoints[i].InScentPlume = inPlume
		if inPlume {
			inPlumeCount++
		}
		switch w.Behavior {
		case domain.K9BehaviorHeadSnap, domain.K9BehaviorStrongPull, domain.K9BehaviorScentPooling:
			positiveAlerts++
		}
	}

	mission.PlumeIntersectionPct = math.Round((float64(inPlumeCount)/float64(len(mission.Waypoints)))*1000) / 10

	pod := float64(mission.ViabilityScore)*0.40 +
		mission.PlumeIntersectionPct*0.35 +
		float64(positiveAlerts)*8.5 +
		float64(len(mission.Waypoints))*2.0

	if pod < 5.0 {
		pod = 5.0
	} else if pod > 99.0 {
		pod = 99.0
	}
	mission.EstimatedPOD = math.Round(pod*10) / 10

	return mission
}

// ExportK9MissionGPX formats a K9ScentMission and its GPS track waypoints as a GPX 1.1 XML document.
func ExportK9MissionGPX(mission domain.K9ScentMission) (string, error) {
	if strings.TrimSpace(mission.MissionID) == "" {
		return "", errors.New("missionId is required for GPX export")
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<gpx version="1.1" creator="PetSpotR K9 SAR Telemetry" xmlns="http://www.topografix.com/GPX/1/1">` + "\n")

	metaName := fmt.Sprintf("K9 Mission %s (%s)", mission.MissionID, mission.CanineCallsign)
	metaDesc := fmt.Sprintf("Canine: %s (%s) | Handler: %s | Viability: %s (%d/100) | POD: %.1f%%",
		mission.CanineName,
		mission.Discipline,
		mission.HandlerName,
		mission.ViabilityRating,
		mission.ViabilityScore,
		mission.EstimatedPOD,
	)

	b.WriteString("  <metadata>\n")
	b.WriteString(fmt.Sprintf("    <name>%s</name>\n", html.EscapeString(metaName)))
	b.WriteString(fmt.Sprintf("    <desc>%s</desc>\n", html.EscapeString(metaDesc)))
	b.WriteString("  </metadata>\n")

	lkpName := fmt.Sprintf("LKP Origin: %s", mission.ScentArticleDesc)
	b.WriteString(fmt.Sprintf("  <wpt lat=\"%.6f\" lon=\"%.6f\">\n", mission.LKPOriginLat, mission.LKPOriginLng))
	b.WriteString(fmt.Sprintf("    <name>%s</name>\n", html.EscapeString(lkpName)))
	b.WriteString("  </wpt>\n")

	trkName := fmt.Sprintf("K9 Track: %s (%s)", mission.CanineName, mission.MissionID)
	b.WriteString("  <trk>\n")
	b.WriteString(fmt.Sprintf("    <name>%s</name>\n", html.EscapeString(trkName)))
	b.WriteString("    <trkseg>\n")

	for _, w := range mission.Waypoints {
		ts := w.RecordedAt
		if ts.IsZero() {
			ts = time.Now().UTC()
		}
		desc := fmt.Sprintf("InPlume=%t | %s", w.InScentPlume, w.Notes)
		b.WriteString(fmt.Sprintf("      <trkpt lat=\"%.6f\" lon=\"%.6f\">\n", w.Lat, w.Lng))
		b.WriteString(fmt.Sprintf("        <name>%s</name>\n", html.EscapeString(string(w.Behavior))))
		b.WriteString(fmt.Sprintf("        <desc>%s</desc>\n", html.EscapeString(desc)))
		b.WriteString(fmt.Sprintf("        <time>%s</time>\n", ts.UTC().Format(time.RFC3339)))
		b.WriteString("      </trkpt>\n")
	}

	b.WriteString("    </trkseg>\n")
	b.WriteString("  </trk>\n")
	b.WriteString("</gpx>\n")

	return b.String(), nil
}

func distanceAndBearing(lat1Deg, lng1Deg, lat2Deg, lng2Deg float64) (float64, float64) {
	lat1 := lat1Deg * math.Pi / 180.0
	lat2 := lat2Deg * math.Pi / 180.0
	dLat := (lat2Deg - lat1Deg) * math.Pi / 180.0
	dLng := (lng2Deg - lng1Deg) * math.Pi / 180.0

	sinHalfLat := math.Sin(dLat / 2.0)
	sinHalfLng := math.Sin(dLng / 2.0)
	a := sinHalfLat*sinHalfLat + math.Cos(lat1)*math.Cos(lat2)*sinHalfLng*sinHalfLng
	c := 2.0 * math.Atan2(math.Sqrt(a), math.Sqrt(math.Max(0.0, 1.0-a)))
	distM := earthRadiusM * c

	y := math.Sin(dLng) * math.Cos(lat2)
	x := math.Cos(lat1)*math.Sin(lat2) - math.Sin(lat1)*math.Cos(lat2)*math.Cos(dLng)
	brngDeg := math.Atan2(y, x) * 180.0 / math.Pi
	brngDeg = math.Mod(brngDeg+360.0, 360.0)

	return distM, brngDeg
}
