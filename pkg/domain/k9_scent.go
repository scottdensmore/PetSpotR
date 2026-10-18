package domain

import (
	"errors"
	"strings"
	"time"
)

type K9Discipline string

const (
	K9DisciplineTrailing   K9Discipline = "MANTRAILING_SCENT_SPECIFIC"
	K9DisciplineAirScent   K9Discipline = "AIR_SCENT_AREA"
	K9DisciplineHRDCadaver K9Discipline = "EVIDENCE_ARTICLE_SEARCH"
)

func (d K9Discipline) IsValid() bool {
	switch d {
	case K9DisciplineTrailing, K9DisciplineAirScent, K9DisciplineHRDCadaver:
		return true
	default:
		return false
	}
}

type GroundSurfaceType string

const (
	SurfaceLushVegetation GroundSurfaceType = "LUSH_VEGETATION"
	SurfaceForestFloor    GroundSurfaceType = "FOREST_FLOOR"
	SurfaceDryGrass       GroundSurfaceType = "DRY_GRASS"
	SurfaceUrbanAsphalt   GroundSurfaceType = "URBAN_ASPHALT"
	SurfaceSnowIce        GroundSurfaceType = "SNOW_ICE"
)

func (s GroundSurfaceType) IsValid() bool {
	switch s {
	case SurfaceLushVegetation, SurfaceForestFloor, SurfaceDryGrass, SurfaceUrbanAsphalt, SurfaceSnowIce:
		return true
	default:
		return false
	}
}

type ScentViabilityRating string

const (
	ScentViabilityHigh     ScentViabilityRating = "HIGH_VIABILITY"
	ScentViabilityModerate ScentViabilityRating = "MODERATE_VIABILITY"
	ScentViabilityDegraded ScentViabilityRating = "DEGRADED_THERMAL_HAZARD"
	ScentViabilityExpired  ScentViabilityRating = "EXPIRED_SCENT_TRAIL"
)

func (r ScentViabilityRating) IsValid() bool {
	switch r {
	case ScentViabilityHigh, ScentViabilityModerate, ScentViabilityDegraded, ScentViabilityExpired:
		return true
	default:
		return false
	}
}

type K9BehaviorAlert string

const (
	K9BehaviorCasting      K9BehaviorAlert = "CASTING_SEARCH"
	K9BehaviorHeadSnap     K9BehaviorAlert = "HEAD_SNAP_ALERT"
	K9BehaviorStrongPull   K9BehaviorAlert = "STRONG_SCENT_PULL"
	K9BehaviorScentPooling K9BehaviorAlert = "SCENT_POOLING"
	K9BehaviorTrailLost    K9BehaviorAlert = "TRAIL_LOST"
)

func (b K9BehaviorAlert) IsValid() bool {
	switch b {
	case K9BehaviorCasting, K9BehaviorHeadSnap, K9BehaviorStrongPull, K9BehaviorScentPooling, K9BehaviorTrailLost:
		return true
	default:
		return false
	}
}

type K9EnvironmentConditions struct {
	WindSpeedKmh        float64           `json:"windSpeedKmh"`
	WindFromDeg         float64           `json:"windFromDeg"`
	TemperatureC        float64           `json:"temperatureC"`
	RelativeHumidityPct float64           `json:"relativeHumidityPct"`
	GroundSurface       GroundSurfaceType `json:"groundSurface"`
	TrailAgeHours       float64           `json:"trailAgeHours"`
}

func (e K9EnvironmentConditions) Validate() error {
	if e.WindSpeedKmh < 0 || e.WindSpeedKmh > 200 {
		return errors.New("windSpeedKmh must be between 0 and 200")
	}
	if e.WindFromDeg < 0 || e.WindFromDeg > 360 {
		return errors.New("windFromDeg must be between 0 and 360")
	}
	if e.TemperatureC < -50 || e.TemperatureC > 70 {
		return errors.New("temperatureC must be between -50 and 70")
	}
	if e.RelativeHumidityPct < 0 || e.RelativeHumidityPct > 100 {
		return errors.New("relativeHumidityPct must be between 0 and 100")
	}
	if !e.GroundSurface.IsValid() {
		return errors.New("invalid groundSurface")
	}
	if e.TrailAgeHours < 0 {
		return errors.New("trailAgeHours cannot be negative")
	}
	return nil
}

type PlumeCoordinate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type ScentPlumePolygon struct {
	OriginLat          float64           `json:"originLat"`
	OriginLng          float64           `json:"originLng"`
	DownwindAzimuthDeg float64           `json:"downwindAzimuthDeg"`
	EffectiveLengthM   float64           `json:"effectiveLengthM"`
	SpreadAngleDeg     float64           `json:"spreadAngleDeg"`
	Vertices           []PlumeCoordinate `json:"vertices"`
}

type K9TrackWaypoint struct {
	WaypointID   string          `json:"waypointId"`
	Lat          float64         `json:"lat"`
	Lng          float64         `json:"lng"`
	Behavior     K9BehaviorAlert `json:"behavior"`
	InScentPlume bool            `json:"inScentPlume"`
	Notes        string          `json:"notes,omitempty"`
	RecordedAt   time.Time       `json:"recordedAt"`
}

func (w K9TrackWaypoint) Validate() error {
	if strings.TrimSpace(w.WaypointID) == "" {
		return errors.New("waypointId is required")
	}
	if w.Lat < -90 || w.Lat > 90 {
		return errors.New("lat must be between -90 and 90")
	}
	if w.Lng < -180 || w.Lng > 180 {
		return errors.New("lng must be between -180 and 180")
	}
	if !w.Behavior.IsValid() {
		return errors.New("invalid k9 behavior alert")
	}
	return nil
}

type K9ScentMission struct {
	MissionID            string                  `json:"missionId"`
	IncidentID           string                  `json:"incidentId"`
	PetID                string                  `json:"petId"`
	HandlerVolunteerID   string                  `json:"handlerVolunteerId"`
	HandlerName          string                  `json:"handlerName"`
	CanineName           string                  `json:"canineName"`
	CanineCallsign       string                  `json:"canineCallsign"`
	Discipline           K9Discipline            `json:"discipline"`
	ScentArticleDesc     string                  `json:"scentArticleDesc"`
	LKPOriginLat         float64                 `json:"lkpOriginLat"`
	LKPOriginLng         float64                 `json:"lkpOriginLng"`
	Environment          K9EnvironmentConditions `json:"environment"`
	ViabilityScore       int                     `json:"viabilityScore"`
	ViabilityRating      ScentViabilityRating    `json:"viabilityRating"`
	PawThermalHazard     bool                    `json:"pawThermalHazard"`
	SafetyAdvisory       string                  `json:"safetyAdvisory"`
	PlumeCone            ScentPlumePolygon       `json:"plumeCone"`
	Waypoints            []K9TrackWaypoint       `json:"waypoints"`
	PlumeIntersectionPct float64                 `json:"plumeIntersectionPct"`
	EstimatedPOD         float64                 `json:"estimatedPod"`
	PromotedSightingID   string                  `json:"promotedSightingId,omitempty"`
	CreatedAt            time.Time               `json:"createdAt"`
	UpdatedAt            time.Time               `json:"updatedAt"`
}

func (m K9ScentMission) Validate() error {
	if strings.TrimSpace(m.MissionID) == "" {
		return errors.New("missionId is required")
	}
	if strings.TrimSpace(m.PetID) == "" {
		return errors.New("petId is required")
	}
	if strings.TrimSpace(m.HandlerName) == "" {
		return errors.New("handlerName is required")
	}
	if strings.TrimSpace(m.CanineName) == "" {
		return errors.New("canineName is required")
	}
	if !m.Discipline.IsValid() {
		return errors.New("invalid k9 discipline")
	}
	if m.LKPOriginLat < -90 || m.LKPOriginLat > 90 {
		return errors.New("lkpOriginLat must be between -90 and 90")
	}
	if m.LKPOriginLng < -180 || m.LKPOriginLng > 180 {
		return errors.New("lkpOriginLng must be between -180 and 180")
	}
	return m.Environment.Validate()
}
