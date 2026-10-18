package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestK9ScentDomainValidationAndSerialization(t *testing.T) {
	if store.CollectionK9ScentMissions != "k9_scent_missions" {
		t.Fatalf("expected store.CollectionK9ScentMissions to be k9_scent_missions, got %q", store.CollectionK9ScentMissions)
	}

	validDisciplines := []domain.K9Discipline{
		domain.K9DisciplineTrailing,
		domain.K9DisciplineAirScent,
		domain.K9DisciplineHRDCadaver,
	}
	for _, d := range validDisciplines {
		if !d.IsValid() {
			t.Errorf("expected discipline %q to be valid", d)
		}
	}
	if domain.K9Discipline("UNKNOWN").IsValid() {
		t.Error("expected invalid discipline to fail IsValid()")
	}

	validSurfaces := []domain.GroundSurfaceType{
		domain.SurfaceLushVegetation,
		domain.SurfaceForestFloor,
		domain.SurfaceDryGrass,
		domain.SurfaceUrbanAsphalt,
		domain.SurfaceSnowIce,
	}
	for _, s := range validSurfaces {
		if !s.IsValid() {
			t.Errorf("expected surface %q to be valid", s)
		}
	}
	if domain.GroundSurfaceType("MUD_SWAMP").IsValid() {
		t.Error("expected invalid surface to fail IsValid()")
	}

	validRatings := []domain.ScentViabilityRating{
		domain.ScentViabilityHigh,
		domain.ScentViabilityModerate,
		domain.ScentViabilityDegraded,
		domain.ScentViabilityExpired,
	}
	for _, r := range validRatings {
		if !r.IsValid() {
			t.Errorf("expected viability rating %q to be valid", r)
		}
	}
	if domain.ScentViabilityRating("INVALID").IsValid() {
		t.Error("expected invalid rating to fail IsValid()")
	}

	validBehaviors := []domain.K9BehaviorAlert{
		domain.K9BehaviorCasting,
		domain.K9BehaviorHeadSnap,
		domain.K9BehaviorStrongPull,
		domain.K9BehaviorScentPooling,
		domain.K9BehaviorTrailLost,
	}
	for _, b := range validBehaviors {
		if !b.IsValid() {
			t.Errorf("expected behavior %q to be valid", b)
		}
	}
	if domain.K9BehaviorAlert("BARKING").IsValid() {
		t.Error("expected invalid behavior to fail IsValid()")
	}

	validEnv := domain.K9EnvironmentConditions{
		WindSpeedKmh:        12.5,
		WindFromDeg:         270,
		TemperatureC:        18.0,
		RelativeHumidityPct: 65.0,
		GroundSurface:       domain.SurfaceLushVegetation,
		TrailAgeHours:       4.5,
	}
	if err := validEnv.Validate(); err != nil {
		t.Fatalf("expected valid environment to pass Validate(), got %v", err)
	}

	envCases := []struct {
		name   string
		mutate func(*domain.K9EnvironmentConditions)
	}{
		{"negative wind speed", func(e *domain.K9EnvironmentConditions) { e.WindSpeedKmh = -1 }},
		{"excessive wind speed", func(e *domain.K9EnvironmentConditions) { e.WindSpeedKmh = 201 }},
		{"negative wind deg", func(e *domain.K9EnvironmentConditions) { e.WindFromDeg = -1 }},
		{"wind deg > 360", func(e *domain.K9EnvironmentConditions) { e.WindFromDeg = 361 }},
		{"temp < -50", func(e *domain.K9EnvironmentConditions) { e.TemperatureC = -51 }},
		{"temp > 70", func(e *domain.K9EnvironmentConditions) { e.TemperatureC = 71 }},
		{"humidity < 0", func(e *domain.K9EnvironmentConditions) { e.RelativeHumidityPct = -1 }},
		{"humidity > 100", func(e *domain.K9EnvironmentConditions) { e.RelativeHumidityPct = 101 }},
		{"invalid ground surface", func(e *domain.K9EnvironmentConditions) { e.GroundSurface = "GRAVEL" }},
		{"negative trail age", func(e *domain.K9EnvironmentConditions) { e.TrailAgeHours = -0.5 }},
	}
	for _, tc := range envCases {
		e := validEnv
		tc.mutate(&e)
		if err := e.Validate(); err == nil {
			t.Errorf("expected error for %s, got nil", tc.name)
		}
	}

	now := time.Now().UTC().Truncate(time.Second)
	validWaypoint := domain.K9TrackWaypoint{
		WaypointID:   "wp-1",
		Lat:          47.6062,
		Lng:          -122.3321,
		Behavior:     domain.K9BehaviorHeadSnap,
		InScentPlume: true,
		Notes:        "K9 snapped head into wind",
		RecordedAt:   now,
	}
	if err := validWaypoint.Validate(); err != nil {
		t.Fatalf("expected valid waypoint to pass Validate(), got %v", err)
	}

	wpCases := []struct {
		name   string
		mutate func(*domain.K9TrackWaypoint)
	}{
		{"empty waypointId", func(w *domain.K9TrackWaypoint) { w.WaypointID = " " }},
		{"lat < -90", func(w *domain.K9TrackWaypoint) { w.Lat = -91 }},
		{"lat > 90", func(w *domain.K9TrackWaypoint) { w.Lat = 91 }},
		{"lng < -180", func(w *domain.K9TrackWaypoint) { w.Lng = -181 }},
		{"lng > 180", func(w *domain.K9TrackWaypoint) { w.Lng = 181 }},
		{"invalid behavior", func(w *domain.K9TrackWaypoint) { w.Behavior = "SLEEPING" }},
	}
	for _, tc := range wpCases {
		w := validWaypoint
		tc.mutate(&w)
		if err := w.Validate(); err == nil {
			t.Errorf("expected error for waypoint %s, got nil", tc.name)
		}
	}

	validMission := domain.K9ScentMission{
		MissionID:          "k9-mission-1",
		IncidentID:         "inc-101",
		PetID:              "pet-202",
		HandlerVolunteerID: "vol-303",
		HandlerName:        "Sarah Connor",
		CanineName:         "Rex",
		CanineCallsign:     "K9-REX-1",
		Discipline:         domain.K9DisciplineTrailing,
		ScentArticleDesc:   "Unwashed fleece blanket in sterile mason jar",
		LKPOriginLat:       47.6062,
		LKPOriginLng:       -122.3321,
		Environment:        validEnv,
		ViabilityScore:     88,
		ViabilityRating:    domain.ScentViabilityHigh,
		PawThermalHazard:   false,
		SafetyAdvisory:     "Optimal scenting conditions",
		PlumeCone: domain.ScentPlumePolygon{
			OriginLat:          47.6062,
			OriginLng:          -122.3321,
			DownwindAzimuthDeg: 90,
			EffectiveLengthM:   450,
			SpreadAngleDeg:     35,
			Vertices: []domain.PlumeCoordinate{
				{Lat: 47.6062, Lng: -122.3321},
				{Lat: 47.6075, Lng: -122.3260},
				{Lat: 47.6049, Lng: -122.3260},
				{Lat: 47.6062, Lng: -122.3321},
			},
		},
		Waypoints:            []domain.K9TrackWaypoint{validWaypoint},
		PlumeIntersectionPct: 100.0,
		EstimatedPOD:         0.78,
		PromotedSightingID:   "sighting-99",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := validMission.Validate(); err != nil {
		t.Fatalf("expected valid mission to pass Validate(), got %v", err)
	}

	missionCases := []struct {
		name   string
		mutate func(*domain.K9ScentMission)
	}{
		{"empty missionId", func(m *domain.K9ScentMission) { m.MissionID = "" }},
		{"empty petId", func(m *domain.K9ScentMission) { m.PetID = " " }},
		{"empty handlerName", func(m *domain.K9ScentMission) { m.HandlerName = "" }},
		{"empty canineName", func(m *domain.K9ScentMission) { m.CanineName = "" }},
		{"invalid discipline", func(m *domain.K9ScentMission) { m.Discipline = "BAD" }},
		{"lkpOriginLat < -90", func(m *domain.K9ScentMission) { m.LKPOriginLat = -95 }},
		{"lkpOriginLat > 90", func(m *domain.K9ScentMission) { m.LKPOriginLat = 95 }},
		{"lkpOriginLng < -180", func(m *domain.K9ScentMission) { m.LKPOriginLng = -185 }},
		{"lkpOriginLng > 180", func(m *domain.K9ScentMission) { m.LKPOriginLng = 185 }},
		{"invalid environment", func(m *domain.K9ScentMission) { m.Environment.WindSpeedKmh = -10 }},
	}
	for _, tc := range missionCases {
		m := validMission
		tc.mutate(&m)
		if err := m.Validate(); err == nil {
			t.Errorf("expected error for mission %s, got nil", tc.name)
		}
	}

	raw, err := json.Marshal(validMission)
	if err != nil {
		t.Fatalf("failed to marshal mission: %v", err)
	}
	var decoded domain.K9ScentMission
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("failed to unmarshal mission: %v", err)
	}
	if decoded.MissionID != validMission.MissionID ||
		decoded.CanineName != validMission.CanineName ||
		decoded.Discipline != validMission.Discipline ||
		len(decoded.Waypoints) != 1 ||
		decoded.Waypoints[0].Behavior != domain.K9BehaviorHeadSnap ||
		len(decoded.PlumeCone.Vertices) != 4 {
		t.Fatalf("round-trip JSON mismatch: %+v", decoded)
	}
}
