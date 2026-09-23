package domain

import "time"

// FlightStatus represents the operational state of a drone flight sortie.
type FlightStatus string

const (
	FlightStatusActive    FlightStatus = "ACTIVE"
	FlightStatusCompleted FlightStatus = "COMPLETED"
)

// HotspotStatus represents operator or AI verification state of a thermal anomaly.
type HotspotStatus string

const (
	HotspotUnverified HotspotStatus = "UNVERIFIED"
	HotspotConfirmed  HotspotStatus = "CONFIRMED"
	HotspotDismissed  HotspotStatus = "DISMISSED"
)

// ThermalPalette represents the radiometric or pseudocolor palette in use.
type ThermalPalette string

const (
	PaletteWhiteHot ThermalPalette = "WHITE_HOT"
	PaletteBlackHot ThermalPalette = "BLACK_HOT"
	PaletteIronbow  ThermalPalette = "IRONBOW"
)

// NormalizedBox defines a normalized bounding rectangle [0.0 - 1.0].
type NormalizedBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// DroneWaypoint represents a single telemetry sample during flight.
type DroneWaypoint struct {
	Timestamp      time.Time `json:"timestamp"`
	Latitude       float64   `json:"latitude"`
	Longitude      float64   `json:"longitude"`
	AltitudeAGL    float64   `json:"altitudeMetersAGL"` // Above ground level
	HeadingDeg     float64   `json:"headingDeg"`        // 0-360 true north
	GimbalPitchDeg float64   `json:"gimbalPitchDeg"`    // -90 nadir, 0 horizontal
	GimbalRollDeg  float64   `json:"gimbalRollDeg"`
	GimbalYawDeg   float64   `json:"gimbalYawDeg"`
	GroundSpeedMps float64   `json:"groundSpeedMps"`
	BatteryPercent int       `json:"batteryPercent"`
}

// ThermalHotspot represents a detected biological thermal anomaly.
type ThermalHotspot struct {
	ID                 string         `json:"id"`
	MissionID          string         `json:"missionId"`
	PetID              string         `json:"petId"`
	Timestamp          time.Time      `json:"timestamp"`
	Latitude           float64        `json:"latitude"`
	Longitude          float64        `json:"longitude"`
	EstimatedTempC     float64        `json:"estimatedTempC"`
	ConfidenceScore    float64        `json:"confidenceScore"`    // 0.0 to 1.0
	Palette            ThermalPalette `json:"palette"`
	BoundingBox        NormalizedBox  `json:"boundingBox"`
	FrameTimeOffsetSec float64        `json:"frameTimeOffsetSec"` // Offset into flight video
	ThumbnailBase64    string         `json:"thumbnailBase64,omitempty"`
	Status             HotspotStatus  `json:"status"`
	Classification     string         `json:"classification,omitempty"` // e.g. "Canine Signature", "Heat Anomaly"
	LinkedSightingID   string         `json:"linkedSightingId,omitempty"`
}

// DroneMission tracks an entire aerial reconnaissance sortie.
type DroneMission struct {
	ID               string           `json:"id"`
	PetID            string           `json:"petId"`
	SearchPartyID    string           `json:"searchPartyId,omitempty"`
	PilotCallsign    string           `json:"pilotCallsign"`
	DroneModel       string           `json:"droneModel"`
	StartTime        time.Time        `json:"startTime"`
	EndTime          *time.Time       `json:"endTime,omitempty"`
	Status           FlightStatus     `json:"status"`
	Waypoints        []DroneWaypoint  `json:"waypoints"`
	FootprintPoly    [][]float64      `json:"footprintPolygon"` // Exterior boundary [lng, lat]
	Hotspots         []ThermalHotspot `json:"hotspots"`
	SweptAreaSqM     float64          `json:"sweptAreaSqMeters"`
	CoveredSectorIDs []string         `json:"coveredSectorIds"`
}

// CameraIntrinsics encapsulates camera optical properties.
type CameraIntrinsics struct {
	HFOV float64 `json:"hfov"` // Horizontal field of view in degrees (default 84.0)
	VFOV float64 `json:"vfov"` // Vertical field of view in degrees (default 60.0)
}

// SearchPartySector is an alias for SearchSector to match aerial recon domain nomenclature.
type SearchPartySector = SearchSector
