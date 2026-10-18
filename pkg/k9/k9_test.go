package k9

import (
	"encoding/xml"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEvaluateScentViability(t *testing.T) {
	tests := []struct {
		name           string
		env            domain.K9EnvironmentConditions
		wantScore      int
		wantRating     domain.ScentViabilityRating
		wantPawHazard  bool
		wantAdvisoryIn string
	}{
		{
			name: "optimal cool humid forest floor",
			env: domain.K9EnvironmentConditions{
				WindSpeedKmh:        12.0,
				WindFromDeg:         270.0,
				TemperatureC:        16.0,
				RelativeHumidityPct: 75.0,
				GroundSurface:       domain.SurfaceForestFloor,
				TrailAgeHours:       2.0,
			},
			// 80 + 8 (forest) + 10 (humidity>=60) + 6 (temp 10..22) - 3 (age 2*1.5) + 5 (wind 5..20) = 106 -> clamped 100
			wantScore:      100,
			wantRating:     domain.ScentViabilityHigh,
			wantPawHazard:  false,
			wantAdvisoryIn: "Optimal scent retention & laminar transport",
		},
		{
			name: "lush vegetation clamped to 100",
			env: domain.K9EnvironmentConditions{
				WindSpeedKmh:        10.0,
				WindFromDeg:         180.0,
				TemperatureC:        15.0,
				RelativeHumidityPct: 80.0,
				GroundSurface:       domain.SurfaceLushVegetation,
				TrailAgeHours:       1.0,
			},
			// 80 + 12 + 10 + 6 - 2 + 5 = 111 -> 100
			wantScore:      100,
			wantRating:     domain.ScentViabilityHigh,
			wantPawHazard:  false,
			wantAdvisoryIn: "Optimal scent retention",
		},
		{
			name: "moderate dry grass conditions",
			env: domain.K9EnvironmentConditions{
				WindSpeedKmh:        25.0,
				WindFromDeg:         90.0,
				TemperatureC:        24.0,
				RelativeHumidityPct: 45.0,
				GroundSurface:       domain.SurfaceDryGrass,
				TrailAgeHours:       10.0,
			},
			// 80 - 10 (dry grass) + 0 (hum 45) + 0 (temp 24) - 15 (age 10*1.5) + 0 (wind 25) = 55
			wantScore:      55,
			wantRating:     domain.ScentViabilityModerate,
			wantPawHazard:  false,
			wantAdvisoryIn: "Moderate scent retention",
		},
		{
			name: "hot urban asphalt triggers paw thermal hazard at 28C",
			env: domain.K9EnvironmentConditions{
				WindSpeedKmh:        10.0,
				WindFromDeg:         180.0,
				TemperatureC:        28.0,
				RelativeHumidityPct: 40.0,
				GroundSurface:       domain.SurfaceUrbanAsphalt,
				TrailAgeHours:       4.0,
			},
			// 80 - 22 (asphalt) + 0 (hum 40) - 15 (temp>=27) - 6 (age 4*1.5) + 5 (wind 10) = 42
			wantScore:      42,
			wantRating:     domain.ScentViabilityDegraded,
			wantPawHazard:  true,
			wantAdvisoryIn: "THERMAL PAW HAZARD",
		},
		{
			name: "degraded non-thermal conditions (score 20..44 without paw hazard)",
			env: domain.K9EnvironmentConditions{
				WindSpeedKmh:        38.0,
				WindFromDeg:         45.0,
				TemperatureC:        25.0,
				RelativeHumidityPct: 25.0,
				GroundSurface:       domain.SurfaceDryGrass,
				TrailAgeHours:       6.0,
			},
			// 80 - 10 (dry grass) - 18 (hum<30) + 0 (temp 25) - 9 (age 6*1.5) - 20 (wind>35) = 23
			wantScore:      23,
			wantRating:     domain.ScentViabilityDegraded,
			wantPawHazard:  false,
			wantAdvisoryIn: "Degraded scent conditions",
		},
		{
			name: "old extreme conditions expired scent trail",
			env: domain.K9EnvironmentConditions{
				WindSpeedKmh:        45.0,
				WindFromDeg:         0.0,
				TemperatureC:        35.0,
				RelativeHumidityPct: 15.0,
				GroundSurface:       domain.SurfaceUrbanAsphalt,
				TrailAgeHours:       48.0,
			},
			// clamped to 0
			wantScore:      0,
			wantRating:     domain.ScentViabilityExpired,
			wantPawHazard:  true,
			wantAdvisoryIn: "Scent trail severely degraded (<20 viability)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score, rating, pawHazard, advisory := EvaluateScentViability(tc.env)
			if score != tc.wantScore {
				t.Errorf("score = %d, want %d", score, tc.wantScore)
			}
			if rating != tc.wantRating {
				t.Errorf("rating = %q, want %q", rating, tc.wantRating)
			}
			if pawHazard != tc.wantPawHazard {
				t.Errorf("pawHazard = %v, want %v", pawHazard, tc.wantPawHazard)
			}
			if !strings.Contains(advisory, tc.wantAdvisoryIn) {
				t.Errorf("advisory = %q, want substring %q", advisory, tc.wantAdvisoryIn)
			}
		})
	}
}

func TestComputeGaussianScentPlume(t *testing.T) {
	originLat := 47.6062
	originLng := -122.3321

	env := domain.K9EnvironmentConditions{
		WindSpeedKmh:        15.0,
		WindFromDeg:         270.0, // Wind from West -> blows East (90 deg)
		TemperatureC:        16.0,
		RelativeHumidityPct: 70.0,
		GroundSurface:       domain.SurfaceForestFloor,
		TrailAgeHours:       2.0,
	}

	plume := ComputeGaussianScentPlume(originLat, originLng, env, 90)
	if math.Abs(plume.DownwindAzimuthDeg-90.0) > 0.01 {
		t.Errorf("DownwindAzimuthDeg = %v, want 90.0", plume.DownwindAzimuthDeg)
	}
	if math.Abs(plume.SpreadAngleDeg-36.0) > 0.01 {
		t.Errorf("SpreadAngleDeg = %v, want 36.0", plume.SpreadAngleDeg)
	}
	if len(plume.Vertices) != 9 {
		t.Fatalf("len(Vertices) = %d, want 9", len(plume.Vertices))
	}
	if plume.Vertices[0].Lat != originLat || plume.Vertices[0].Lng != originLng {
		t.Errorf("first vertex = %+v, want origin", plume.Vertices[0])
	}
	if plume.Vertices[8].Lat != originLat || plume.Vertices[8].Lng != originLng {
		t.Errorf("closing vertex = %+v, want origin", plume.Vertices[8])
	}
	// Center of arc (vertex index 4) should be directly East (higher Lng, ~same Lat)
	if plume.Vertices[4].Lng <= originLng {
		t.Errorf("center downwind vertex Lng = %v, expected > %v (East)", plume.Vertices[4].Lng, originLng)
	}

	// Low wind speed (<8 km/h) -> wide spread angle 60 deg
	lowWindEnv := env
	lowWindEnv.WindSpeedKmh = 5.0
	lowPlume := ComputeGaussianScentPlume(originLat, originLng, lowWindEnv, 80)
	if lowPlume.SpreadAngleDeg != 60.0 {
		t.Errorf("low wind SpreadAngleDeg = %v, want 60.0", lowPlume.SpreadAngleDeg)
	}

	// High wind speed (>22 km/h) -> narrow spread angle 24 deg
	highWindEnv := env
	highWindEnv.WindSpeedKmh = 30.0
	highPlume := ComputeGaussianScentPlume(originLat, originLng, highWindEnv, 80)
	if highPlume.SpreadAngleDeg != 24.0 {
		t.Errorf("high wind SpreadAngleDeg = %v, want 24.0", highPlume.SpreadAngleDeg)
	}
}

func TestIsWaypointInScentPlume(t *testing.T) {
	originLat := 47.6062
	originLng := -122.3321

	env := domain.K9EnvironmentConditions{
		WindSpeedKmh:        15.0,
		WindFromDeg:         270.0, // Downwind = 90 deg (East)
		TemperatureC:        16.0,
		RelativeHumidityPct: 70.0,
		GroundSurface:       domain.SurfaceForestFloor,
		TrailAgeHours:       2.0,
	}
	plume := ComputeGaussianScentPlume(originLat, originLng, env, 90)

	// 1. Point right at LKP origin (<25m) -> true
	if !IsWaypointInScentPlume(originLat, originLng, plume) {
		t.Errorf("expected origin waypoint to be inside scent plume")
	}

	// 2. Point ~200m East (downwind inside cone) -> true
	downwindLng := originLng + 0.0025 // ~188m East at 47.6N
	if !IsWaypointInScentPlume(originLat, downwindLng, plume) {
		t.Errorf("expected downwind East coordinate to be inside scent plume")
	}

	// 3. Point ~200m West (upwind outside cone) -> false
	upwindLng := originLng - 0.0025
	if IsWaypointInScentPlume(originLat, upwindLng, plume) {
		t.Errorf("expected upwind West coordinate to be outside scent plume")
	}

	// 4. Point ~200m North (crosswind outside cone) -> false
	crosswindLat := originLat + 0.0020
	if IsWaypointInScentPlume(crosswindLat, originLng, plume) {
		t.Errorf("expected crosswind North coordinate to be outside scent plume")
	}

	// 5. Point 3km East (beyond effective length) -> false
	farEastLng := originLng + 0.04
	if IsWaypointInScentPlume(originLat, farEastLng, plume) {
		t.Errorf("expected far downwind coordinate beyond effective length to be outside scent plume")
	}
}

func TestRecalculateMissionMetrics(t *testing.T) {
	originLat := 47.6062
	originLng := -122.3321

	mission := domain.K9ScentMission{
		MissionID:      "k9-101",
		IncidentID:     "inc-1",
		PetID:          "pet-1",
		HandlerName:    "Sarah Connor",
		CanineName:     "Rex",
		CanineCallsign: "K9-REX",
		Discipline:     domain.K9DisciplineTrailing,
		LKPOriginLat:   originLat,
		LKPOriginLng:   originLng,
		Environment: domain.K9EnvironmentConditions{
			WindSpeedKmh:        14.0,
			WindFromDeg:         270.0, // Downwind East (90 deg)
			TemperatureC:        16.0,
			RelativeHumidityPct: 72.0,
			GroundSurface:       domain.SurfaceForestFloor,
			TrailAgeHours:       2.0,
		},
	}

	// First test with 0 waypoints
	m0 := RecalculateMissionMetrics(mission)
	if m0.ViabilityScore != 100 {
		t.Errorf("m0.ViabilityScore = %d, want 100", m0.ViabilityScore)
	}
	if m0.PlumeIntersectionPct != 0 {
		t.Errorf("m0.PlumeIntersectionPct = %v, want 0", m0.PlumeIntersectionPct)
	}
	if m0.EstimatedPOD != 35.0 {
		t.Errorf("m0.EstimatedPOD = %v, want 35.0", m0.EstimatedPOD)
	}

	// Now add 3 waypoints: 2 downwind in-plume with positive alerts, 1 crosswind out-of-plume casting
	now := time.Now().UTC()
	mission.Waypoints = []domain.K9TrackWaypoint{
		{
			WaypointID: "wp-1",
			Lat:        originLat,
			Lng:        originLng + 0.0012, // East (in plume)
			Behavior:   domain.K9BehaviorHeadSnap,
			Notes:      "Head snap entering timber",
			RecordedAt: now.Add(-10 * time.Minute),
		},
		{
			WaypointID: "wp-2",
			Lat:        originLat,
			Lng:        originLng + 0.0024, // East (in plume)
			Behavior:   domain.K9BehaviorStrongPull,
			Notes:      "Strong scent pull along trail",
			RecordedAt: now.Add(-5 * time.Minute),
		},
		{
			WaypointID: "wp-3",
			Lat:        originLat + 0.0025, // North (out of plume)
			Lng:        originLng,
			Behavior:   domain.K9BehaviorCasting,
			Notes:      "Casting north of creek",
			RecordedAt: now,
		},
	}

	m1 := RecalculateMissionMetrics(mission)
	if !m1.Waypoints[0].InScentPlume || !m1.Waypoints[1].InScentPlume {
		t.Errorf("expected waypoints 0 and 1 to be marked InScentPlume=true")
	}
	if m1.Waypoints[2].InScentPlume {
		t.Errorf("expected waypoint 2 to be marked InScentPlume=false")
	}
	// 2 of 3 in plume -> 66.7%
	if m1.PlumeIntersectionPct != 66.7 {
		t.Errorf("PlumeIntersectionPct = %v, want 66.7", m1.PlumeIntersectionPct)
	}
	// pod = 100*0.40 + 66.7*0.35 + 2*8.5 + 3*2.0 = 40 + 23.345 + 17 + 6 = 86.345 -> 86.3
	if m1.EstimatedPOD != 86.3 {
		t.Errorf("EstimatedPOD = %v, want 86.3", m1.EstimatedPOD)
	}
}

func TestExportK9MissionGPX(t *testing.T) {
	_, err := ExportK9MissionGPX(domain.K9ScentMission{})
	if err == nil {
		t.Fatalf("expected error for empty MissionID")
	}

	now := time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC)
	mission := domain.K9ScentMission{
		MissionID:        "k9-gpx-01",
		IncidentID:       "inc-01",
		PetID:            "pet-01",
		HandlerName:      "Sarah & Team",
		CanineName:       "Buster <K9>",
		CanineCallsign:   "K9-BUSTER",
		Discipline:       domain.K9DisciplineTrailing,
		ScentArticleDesc: "Collar & Blanket",
		LKPOriginLat:     47.6062,
		LKPOriginLng:     -122.3321,
		ViabilityScore:   88,
		ViabilityRating:  domain.ScentViabilityHigh,
		EstimatedPOD:     82.4,
		Waypoints: []domain.K9TrackWaypoint{
			{
				WaypointID:   "wp-1",
				Lat:          47.6063,
				Lng:          -122.3310,
				Behavior:     domain.K9BehaviorHeadSnap,
				InScentPlume: true,
				Notes:        "Alert near creek & brush",
				RecordedAt:   now,
			},
		},
	}

	gpxStr, err := ExportK9MissionGPX(mission)
	if err != nil {
		t.Fatalf("ExportK9MissionGPX failed: %v", err)
	}
	if !strings.Contains(gpxStr, `<?xml version="1.0" encoding="UTF-8"?>`) {
		t.Errorf("missing XML declaration")
	}
	if !strings.Contains(gpxStr, `xmlns="http://www.topografix.com/GPX/1/1"`) {
		t.Errorf("missing GPX 1.1 namespace")
	}
	if !strings.Contains(gpxStr, "HEAD_SNAP_ALERT") {
		t.Errorf("missing waypoint behavior in GPX output")
	}

	var parsed struct {
		XMLName  xml.Name `xml:"gpx"`
		Version  string   `xml:"version,attr"`
		Creator  string   `xml:"creator,attr"`
		Metadata struct {
			Name string `xml:"name"`
			Desc string `xml:"desc"`
		} `xml:"metadata"`
		Wpt []struct {
			Lat  string `xml:"lat,attr"`
			Lon  string `xml:"lon,attr"`
			Name string `xml:"name"`
		} `xml:"wpt"`
		Trk struct {
			Name   string `xml:"name"`
			TrkSeg struct {
				TrkPt []struct {
					Lat  string `xml:"lat,attr"`
					Lon  string `xml:"lon,attr"`
					Name string `xml:"name"`
					Desc string `xml:"desc"`
					Time string `xml:"time"`
				} `xml:"trkpt"`
			} `xml:"trkseg"`
		} `xml:"trk"`
	}

	if err := xml.Unmarshal([]byte(gpxStr), &parsed); err != nil {
		t.Fatalf("GPX XML is not well-formed: %v\nGPX:\n%s", err, gpxStr)
	}
	if parsed.Version != "1.1" {
		t.Errorf("GPX version = %q, want 1.1", parsed.Version)
	}
	if len(parsed.Wpt) != 1 {
		t.Errorf("len(Wpt) = %d, want 1", len(parsed.Wpt))
	}
	if len(parsed.Trk.TrkSeg.TrkPt) != 1 {
		t.Errorf("len(TrkPt) = %d, want 1", len(parsed.Trk.TrkSeg.TrkPt))
	}
}
