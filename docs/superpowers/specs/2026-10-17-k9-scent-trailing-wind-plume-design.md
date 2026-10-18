# Canine (K9) Scent-Trailing Telemetry, Wind Plume Dispersion & Handler Cockpit — Design Specification

- **Milestone:** 12.4
- **Date:** 2026-10-17
- **Status:** Approved
- **Author:** Antigravity & User

---

## 1. Problem Statement & Objectives

Certified Search-and-Rescue Canine (K9) units (`K9_HANDLER`, introduced in Milestone 12.3) are among the highest-probability recovery assets in wilderness and urban pet searches. However, effective K9 deployment requires:
1. **Physics-Aware Atmospheric Wind Plume & Scent Cone Dispersion Modeling:** Scent rafts and volatile organic compounds (VOCs) disperse downwind from a Last Known Point (LKP) or Scent Article Drop according to wind speed, wind azimuth (direction wind is blowing *from*), Pasquill-Gifford atmospheric stability, ambient temperature, relative humidity, and ground surface retention (`LUSH_VEGETATION`, `FOREST_FLOOR`, `DRY_GRASS`, `URBAN_ASPHALT`, `SNOW_ICE`).
2. **Scent Viability & Environmental Decay Assessment:** High temperatures (`> 30°C`), low humidity (`< 25%`), and hot asphalt accelerate bacterial/UV scent degradation, whereas cool, humid conditions over vegetated terrain preserve scent trails for 24–72+ hours. Handlers and Incident Commanders need deterministic viability scores (`HIGH_VIABILITY`, `MODERATE_VIABILITY`, `DEGRADED_THERMAL_HAZARD`, `EXPIRED_SCENT_TRAIL`) and canine paw-pad thermal safety alerts.
3. **K9 GPS Track & Scent-Corridor Intersection / Probability of Detection (POD) Analysis:** Recording K9 GPS waypoints alongside canine behavioral indicators (`HEAD_SNAP_ALERT`, `CASTING_SEARCH`, `STRONG_SCENT_PULL`, `SCENT_POOLING`, `TRAIL_LOST`) and computing down-wind corridor coverage and mathematical Probability of Detection (`POD`).
4. **Dedicated K9 Scent-Trailing & Wind Plume Handler Cockpit (`/k9-ops`):** An interactive WCAG AAA (`> 7:1` contrast) cockpit with a live SVG/Canvas Gaussian Wind Plume & K9 Track Visualizer, Scent Article & Environmental Weather Logger, K9 Mission Telemetry & Behavioral Alert Recorder, and GPX/JSON Mission Export.

---

## 2. Domain Models & State Store Collections

### 2.1 State Store Collections (`pkg/store/store.go`)
- `CollectionK9ScentMissions = "k9_scent_missions"`

### 2.2 Domain Models (`pkg/domain/k9_scent.go`)

```go
package domain

import "time"

type K9Discipline string

const (
	K9DisciplineTrailing     K9Discipline = "MANTRAILING_SCENT_SPECIFIC"
	K9DisciplineAirScent     K9Discipline = "AIR_SCENT_AREA"
	K9DisciplineHRDCadaver   K9Discipline = "EVIDENCE_ARTICLE_SEARCH"
)

type GroundSurfaceType string

const (
	SurfaceLushVegetation GroundSurfaceType = "LUSH_VEGETATION"
	SurfaceForestFloor    GroundSurfaceType = "FOREST_FLOOR"
	SurfaceDryGrass       GroundSurfaceType = "DRY_GRASS"
	SurfaceUrbanAsphalt   GroundSurfaceType = "URBAN_ASPHALT"
	SurfaceSnowIce        GroundSurfaceType = "SNOW_ICE"
)

type ScentViabilityRating string

const (
	ScentViabilityHigh     ScentViabilityRating = "HIGH_VIABILITY"          // >= 75 score
	ScentViabilityModerate ScentViabilityRating = "MODERATE_VIABILITY"      // 45–74 score
	ScentViabilityDegraded ScentViabilityRating = "DEGRADED_THERMAL_HAZARD" // 20–44 score or temp >= 32C
	ScentViabilityExpired  ScentViabilityRating = "EXPIRED_SCENT_TRAIL"     // < 20 score
)

type K9BehaviorAlert string

const (
	K9BehaviorCasting        K9BehaviorAlert = "CASTING_SEARCH"
	K9BehaviorHeadSnap       K9BehaviorAlert = "HEAD_SNAP_ALERT"
	K9BehaviorStrongPull     K9BehaviorAlert = "STRONG_SCENT_PULL"
	K9BehaviorScentPooling   K9BehaviorAlert = "SCENT_POOLING"
	K9BehaviorTrailLost      K9BehaviorAlert = "TRAIL_LOST"
)

type K9EnvironmentConditions struct {
	WindSpeedKmh        float64           `json:"windSpeedKmh"`
	WindFromDeg         float64           `json:"windFromDeg"` // Meteorological degrees (0-360) wind blows FROM
	TemperatureC        float64           `json:"temperatureC"`
	RelativeHumidityPct float64           `json:"relativeHumidityPct"`
	GroundSurface       GroundSurfaceType `json:"groundSurface"`
	TrailAgeHours       float64           `json:"trailAgeHours"`
}

typeGeoPoint struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type ScentPlumePolygon struct {
	OriginLat           float64    `json:"originLat"`
	OriginLng           float64    `json:"originLng"`
	DownwindAzimuthDeg  float64    `json:"downwindAzimuthDeg"` // (WindFromDeg + 180) mod 360
	EffectiveLengthM    float64    `json:"effectiveLengthM"`
	SpreadAngleDeg      float64    `json:"spreadAngleDeg"`
	Vertices            []GeoPoint `json:"vertices"` // Closed polygon vertices [origin, leftFlank..., arc..., rightFlank..., origin]
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
	EstimatedPOD         float64                 `json:"estimatedPod"` // 0.0 - 100.0%
	PromotedSightingID   string                  `json:"promotedSightingId,omitempty"`
	CreatedAt            time.Time               `json:"createdAt"`
	UpdatedAt            time.Time               `json:"updatedAt"`
}
```

---

## 3. Pure-Go Atmospheric Scent Plume & Viability Engine (`pkg/k9`)

### 3.1 Scent Viability & Canine Thermal Safety (`pkg/k9/plume.go`)
- `EvaluateScentViability(env domain.K9EnvironmentConditions) (int, domain.ScentViabilityRating, bool, string)`:
  - Base score = `100`
  - Surface modifier:
    - `LUSH_VEGETATION`: `+10`
    - `FOREST_FLOOR`: `+5`
    - `SNOW_ICE`: `0`
    - `DRY_GRASS`: `-10`
    - `URBAN_ASPHALT`: `-25`
  - Humidity modifier:
    - `>= 60%`: `+10`
    - `< 30%`: `-20`
  - Temperature modifier & Paw Pad Thermal Hazard:
    - `PawThermalHazard = true` if `env.TemperatureC >= 30.0` or (`env.GroundSurface == SurfaceUrbanAsphalt && env.TemperatureC >= 27.0`).
    - If `env.TemperatureC >= 32.0`: `-30` score; else if `env.TemperatureC >= 27.0`: `-15`; else if `env.TemperatureC >= 10.0 && env.TemperatureC <= 22.0`: `+5`.
  - Trail Age decay: `- int(env.TrailAgeHours * 1.5)`
  - Wind speed modifier: if `env.WindSpeedKmh > 35.0` (turbulent dispersal): `-20`; if `env.WindSpeedKmh >= 5.0 && env.WindSpeedKmh <= 20.0` (ideal laminar transport): `+5`.
  - Clamp `score` to `[0, 100]`.
  - Determine `ScentViabilityRating`:
    - If `PawThermalHazard` and `score >= 20`: returns `ScentViabilityDegraded` (or if `score < 45` and `score >= 20`, `ScentViabilityDegraded`).
    - Else if `score >= 75`: `ScentViabilityHigh`
    - Else if `score >= 45`: `ScentViabilityModerate`
    - Else if `score >= 20`: `ScentViabilityDegraded`
    - Else: `ScentViabilityExpired`

### 3.2 Gaussian Wind Plume Sector Polygon Generator (`pkg/k9/plume.go`)
- `ComputeGaussianScentPlume(originLat, originLng float64, env domain.K9EnvironmentConditions, viabilityScore int) domain.ScentPlumePolygon`:
  - `DownwindAzimuthDeg = math.Mod(env.WindFromDeg+180.0, 360.0)`
  - `EffectiveLengthM`: base `250.0 + env.WindSpeedKmh*18.0`, scaled by `float64(max(viabilityScore, 20))/100.0`, clamped to `[120.0, 1200.0]`.
  - `SpreadAngleDeg`: calm/light wind (`< 8 km/h`) causes wider lateral meandering (`60.0°`), moderate wind (`8–22 km/h`) produces focused cone (`36.0°`), strong wind (`> 22 km/h`) produces narrow jet (`24.0°`).
  - Generates 9 closed GeoPoint vertices `[origin, arc_0, arc_1, ..., arc_6, origin]` projecting outward from `(originLat, originLng)` along bearings `[DownwindAzimuthDeg - SpreadAngleDeg/2 ... DownwindAzimuthDeg + SpreadAngleDeg/2]` at `EffectiveLengthM`.

### 3.3 Waypoint Plume Containment, POD & GPX Exporter (`pkg/k9/telemetry.go`)
- `IsWaypointInScentPlume(lat, lng float64, plume domain.ScentPlumePolygon) bool`:
  - Computes distance (`m`) and forward bearing (`0–360°`) from `(plume.OriginLat, plume.OriginLng)` to `(lat, lng)`.
  - Returns `true` if `distanceM <= plume.EffectiveLengthM * 1.05` and angular difference from `plume.DownwindAzimuthDeg` is `<= (plume.SpreadAngleDeg / 2.0) + 3.0` (or `distanceM <= 25.0` near origin).
- `RecalculateMissionMetrics(mission domain.K9ScentMission) domain.K9ScentMission`:
  - Evaluates each waypoint's `InScentPlume`, computes `PlumeIntersectionPct`, and computes `EstimatedPOD` (`0–99.0%`) based on `ViabilityScore`, `PlumeIntersectionPct`, number of waypoints, and high-confidence canine alerts (`HEAD_SNAP_ALERT`, `STRONG_SCENT_PULL`, `SCENT_POOLING`).
- `ExportK9MissionGPX(mission domain.K9ScentMission) (string, error)`:
  - Generates a standard GPX 1.1 XML document (`<gpx version="1.1" creator="PetSpotR K9 SAR Telemetry">`) containing `<metadata>`, LKP `<wpt>`, and `<trk><trkseg><trkpt>` entries for all recorded K9 waypoints.

---

## 4. REST API & K9 Scent-Trailing Handler Cockpit (`internal/app/webfrontend/k9_handlers.go`)

1. `GET /k9-ops` — Renders the Canine (K9) Scent-Trailing Telemetry, Wind Plume Dispersion & Handler Cockpit (`templates/k9_ops.html`).
2. `GET /api/v1/k9/missions` — Lists K9 scent missions sorted by `UpdatedAt` descending (seeds 2 default active K9 missions on first lookup).
3. `POST /api/v1/k9/missions` — Creates a new K9 scent-trailing mission, computes `ViabilityScore`, `ViabilityRating`, `PawThermalHazard`, and `PlumeCone`, persists to `CollectionK9ScentMissions`, and broadcasts `k9_mission_updated` via SSE (`/api/v1/k9/stream`) and WebRTC mesh.
4. `POST /api/v1/k9/missions/{id}/waypoints` — Appends a K9 GPS waypoint + behavioral alert (`CASTING_SEARCH`, `HEAD_SNAP_ALERT`, `STRONG_SCENT_PULL`, `SCENT_POOLING`, `TRAIL_LOST`), recalculates `InScentPlume`, `PlumeIntersectionPct`, and `EstimatedPOD`, and optionally promotes a `STRONG_SCENT_PULL` / `HEAD_SNAP_ALERT` waypoint into a verified Community Sighting (`promoteToSighting: true`).
5. `GET /api/v1/k9/missions/{id}/gpx` — Exports the K9 mission track as GPX 1.1 XML (`application/gpx+xml`).
6. `GET /api/v1/k9/stream` — Public SSE stream on topic `"k9"` for real-time K9 telemetry updates.
