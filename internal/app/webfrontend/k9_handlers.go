package webfrontend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/k9"
	"github.com/scottdensmore/petspotr/pkg/store"
)

var (
	lastK9Nano atomic.Int64
	k9SeedMu   sync.Mutex
)

func nextK9Nano() int64 {
	now := time.Now().UTC().UnixNano()
	for {
		prev := lastK9Nano.Load()
		next := now
		if next <= prev {
			next = prev + 1
		}
		if lastK9Nano.CompareAndSwap(prev, next) {
			return next
		}
	}
}

// K9OpsPageData represents template data passed to templates/k9_ops.html.
type K9OpsPageData struct {
	Missions []domain.K9ScentMission
	Locale   string
}

type createK9MissionRequest struct {
	IncidentID          string                          `json:"incidentId"`
	PetID               string                          `json:"petId"`
	HandlerVolunteerID  string                          `json:"handlerVolunteerId"`
	HandlerName         string                          `json:"handlerName"`
	CanineName          string                          `json:"canineName"`
	CanineCallsign      string                          `json:"canineCallsign"`
	Discipline          domain.K9Discipline             `json:"discipline"`
	ScentArticleDesc    string                          `json:"scentArticleDesc"`
	LKPOriginLat        float64                         `json:"lkpOriginLat"`
	LKPOriginLng        float64                         `json:"lkpOriginLng"`
	Environment         *domain.K9EnvironmentConditions `json:"environment"`
	WindSpeedKmh        *float64                        `json:"windSpeedKmh"`
	WindFromDeg         *float64                        `json:"windFromDeg"`
	TemperatureC        *float64                        `json:"temperatureC"`
	RelativeHumidityPct *float64                        `json:"relativeHumidityPct"`
	GroundSurface       domain.GroundSurfaceType        `json:"groundSurface"`
	TrailAgeHours       *float64                        `json:"trailAgeHours"`
}

type addK9WaypointRequest struct {
	Lat               float64                `json:"lat"`
	Lng               float64                `json:"lng"`
	Behavior          domain.K9BehaviorAlert `json:"behavior"`
	Notes             string                 `json:"notes"`
	PromoteToSighting bool                   `json:"promoteToSighting"`
}

type k9MissionEnvelope struct {
	domain.K9ScentMission
	Mission  domain.K9ScentMission   `json:"mission"`
	Waypoint *domain.K9TrackWaypoint `json:"waypoint,omitempty"`
	Sighting *domain.Sighting        `json:"sighting,omitempty"`
}

func defaultK9Missions(now time.Time) []domain.K9ScentMission {
	m1 := domain.K9ScentMission{
		MissionID:          "mission-k9-101",
		IncidentID:         "INC-2026-WILDFIRE-01",
		PetID:              "pet-lost-k9-bella",
		HandlerVolunteerID: "vol-sar-101",
		HandlerName:        "Maya Lin",
		CanineName:         "K9 Atlas",
		CanineCallsign:     "K9-ATLAS-1",
		Discipline:         domain.K9DisciplineTrailing,
		ScentArticleDesc:   "Fleece collar & blanket sealed in sterile gauze jar",
		LKPOriginLat:       47.6062,
		LKPOriginLng:       -122.3321,
		Environment: domain.K9EnvironmentConditions{
			WindSpeedKmh:        12.0,
			WindFromDeg:         270.0,
			TemperatureC:        16.5,
			RelativeHumidityPct: 72.0,
			GroundSurface:       domain.SurfaceForestFloor,
			TrailAgeHours:       3.0,
		},
		Waypoints: []domain.K9TrackWaypoint{
			{
				WaypointID: "wp-k9-101-1",
				Lat:        47.6062,
				Lng:        -122.3308,
				Behavior:   domain.K9BehaviorCasting,
				Notes:      "Initial scent cone pick-up at trailhead",
				RecordedAt: now.Add(-15 * time.Minute),
			},
			{
				WaypointID: "wp-k9-101-2",
				Lat:        47.6063,
				Lng:        -122.3295,
				Behavior:   domain.K9BehaviorHeadSnap,
				Notes:      "Sharp head-snap into downwind fern gully",
				RecordedAt: now.Add(-8 * time.Minute),
			},
		},
		CreatedAt: now,
		UpdatedAt: now.Add(time.Millisecond),
	}
	m1 = k9.RecalculateMissionMetrics(m1)

	m2 := domain.K9ScentMission{
		MissionID:          "mission-k9-102",
		IncidentID:         "INC-2026-URBAN-02",
		PetID:              "pet-lost-k9-shadow",
		HandlerVolunteerID: "vol-sar-102",
		HandlerName:        "Carlos Rivera",
		CanineName:         "K9 Freya",
		CanineCallsign:     "K9-FREYA-2",
		Discipline:         domain.K9DisciplineAirScent,
		ScentArticleDesc:   "Carrier pad from vehicle",
		LKPOriginLat:       47.6150,
		LKPOriginLng:       -122.3400,
		Environment: domain.K9EnvironmentConditions{
			WindSpeedKmh:        15.0,
			WindFromDeg:         180.0,
			TemperatureC:        31.5,
			RelativeHumidityPct: 34.0,
			GroundSurface:       domain.SurfaceUrbanAsphalt,
			TrailAgeHours:       6.0,
		},
		Waypoints: []domain.K9TrackWaypoint{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	m2 = k9.RecalculateMissionMetrics(m2)

	return []domain.K9ScentMission{m1, m2}
}

func (s *Server) ensureDefaultK9Missions(ctx context.Context) {
	if s.stateStore == nil {
		return
	}
	k9SeedMu.Lock()
	defer k9SeedMu.Unlock()

	existing, err := s.stateStore.ListState(ctx, store.CollectionK9ScentMissions)
	if err == nil && len(existing) > 0 {
		return
	}

	now := time.Now().UTC()
	for _, m := range defaultK9Missions(now) {
		if data, err := json.Marshal(m); err == nil {
			_ = s.stateStore.SaveState(ctx, store.CollectionK9ScentMissions, m.MissionID, data)
		}
	}
}

func (s *Server) loadK9Missions(ctx context.Context, incidentIDFilter, petIDFilter string) []domain.K9ScentMission {
	missions := make([]domain.K9ScentMission, 0)
	if s.stateStore == nil {
		return missions
	}
	raw, err := s.stateStore.ListState(ctx, store.CollectionK9ScentMissions)
	if err != nil {
		return missions
	}
	for _, b := range raw {
		var m domain.K9ScentMission
		if err := json.Unmarshal(b, &m); err == nil {
			if incidentIDFilter != "" && !strings.EqualFold(m.IncidentID, incidentIDFilter) {
				continue
			}
			if petIDFilter != "" && !strings.EqualFold(m.PetID, petIDFilter) {
				continue
			}
			if m.Waypoints == nil {
				m.Waypoints = []domain.K9TrackWaypoint{}
			}
			if m.PlumeCone.Vertices == nil {
				m.PlumeCone.Vertices = []domain.PlumeCoordinate{}
			}
			missions = append(missions, m)
		}
	}
	sort.Slice(missions, func(i, j int) bool {
		if !missions[i].UpdatedAt.Equal(missions[j].UpdatedAt) {
			return missions[i].UpdatedAt.After(missions[j].UpdatedAt)
		}
		return missions[i].MissionID < missions[j].MissionID
	})
	return missions
}

func (s *Server) handleK9OpsPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.ensureDefaultK9Missions(r.Context())
	missions := s.loadK9Missions(r.Context(), "", "")

	locale := LocaleFromContext(r.Context())
	if strings.TrimSpace(locale) == "" {
		locale = "en"
	}

	data := K9OpsPageData{
		Missions: missions,
		Locale:   locale,
	}

	s.renderCockpitTemplate(w, r, "k9_ops.html", "K9 Ops", data)
}

func (s *Server) handleListK9Missions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	s.ensureDefaultK9Missions(r.Context())
	q := r.URL.Query()
	missions := s.loadK9Missions(
		r.Context(),
		strings.TrimSpace(q.Get("incidentId")),
		strings.TrimSpace(q.Get("petId")),
	)

	writeCockpitJSON(w, http.StatusOK, map[string]any{
		"missions": missions,
	})
}

func (s *Server) handleCreateK9Mission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req createK9MissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultK9Missions(r.Context())

	incidentID := strings.TrimSpace(req.IncidentID)
	if incidentID == "" {
		incidentID = "INC-2026-SAR-01"
	}

	discipline := domain.K9Discipline(strings.ToUpper(strings.TrimSpace(string(req.Discipline))))
	if discipline == "" {
		discipline = domain.K9DisciplineTrailing
	}

	var env domain.K9EnvironmentConditions
	if req.Environment != nil {
		env = *req.Environment
	} else {
		if req.WindSpeedKmh != nil {
			env.WindSpeedKmh = *req.WindSpeedKmh
		}
		if req.WindFromDeg != nil {
			env.WindFromDeg = *req.WindFromDeg
		}
		if req.TemperatureC != nil {
			env.TemperatureC = *req.TemperatureC
		}
		if req.RelativeHumidityPct != nil {
			env.RelativeHumidityPct = *req.RelativeHumidityPct
		}
		env.GroundSurface = req.GroundSurface
		if req.TrailAgeHours != nil {
			env.TrailAgeHours = *req.TrailAgeHours
		}
	}
	env.GroundSurface = domain.GroundSurfaceType(strings.ToUpper(strings.TrimSpace(string(env.GroundSurface))))
	if env.GroundSurface == "" {
		env.GroundSurface = domain.SurfaceForestFloor
	}

	if math.IsNaN(req.LKPOriginLat) || math.IsInf(req.LKPOriginLat, 0) ||
		math.IsNaN(req.LKPOriginLng) || math.IsInf(req.LKPOriginLng, 0) {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid LKP origin coordinates")
		return
	}

	handlerVolID := strings.TrimSpace(req.HandlerVolunteerID)
	if handlerVolID == "" {
		handlerVolID = "vol-sar-101"
	}
	canineName := strings.TrimSpace(req.CanineName)
	canineCallsign := strings.ToUpper(strings.TrimSpace(req.CanineCallsign))
	if canineCallsign == "" && canineName != "" {
		canineCallsign = strings.ToUpper(strings.ReplaceAll(canineName, " ", "-"))
	}
	scentArticle := strings.TrimSpace(req.ScentArticleDesc)
	if scentArticle == "" {
		scentArticle = "Sterile gauze scent article"
	}

	seqNano := nextK9Nano()
	now := time.Unix(0, seqNano).UTC()

	mission := domain.K9ScentMission{
		MissionID:          fmt.Sprintf("mission-k9-%d", seqNano),
		IncidentID:         incidentID,
		PetID:              strings.TrimSpace(req.PetID),
		HandlerVolunteerID: handlerVolID,
		HandlerName:        strings.TrimSpace(req.HandlerName),
		CanineName:         canineName,
		CanineCallsign:     canineCallsign,
		Discipline:         discipline,
		ScentArticleDesc:   scentArticle,
		LKPOriginLat:       req.LKPOriginLat,
		LKPOriginLng:       req.LKPOriginLng,
		Environment:        env,
		Waypoints:          []domain.K9TrackWaypoint{},
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	mission = k9.RecalculateMissionMetrics(mission)
	if err := mission.Validate(); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	missionBytes, err := json.Marshal(mission)
	if err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to encode K9 mission")
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionK9ScentMissions, mission.MissionID, missionBytes); err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to save K9 mission")
		return
	}

	s.broadcastK9Event("k9_mission_updated", map[string]any{
		"type":    "k9_mission_updated",
		"mission": mission,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(k9MissionEnvelope{
		K9ScentMission: mission,
		Mission:        mission,
	})
}

func (s *Server) handleAddK9Waypoint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	missionID := strings.TrimSpace(r.PathValue("id"))
	if missionID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req addK9WaypointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	s.ensureDefaultK9Missions(r.Context())

	req.Behavior = domain.K9BehaviorAlert(strings.ToUpper(strings.TrimSpace(string(req.Behavior))))
	if !req.Behavior.IsValid() {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid K9 behavior alert")
		return
	}
	if math.IsNaN(req.Lat) || math.IsInf(req.Lat, 0) || req.Lat < -90 || req.Lat > 90 ||
		math.IsNaN(req.Lng) || math.IsInf(req.Lng, 0) || req.Lng < -180 || req.Lng > 180 {
		writeCockpitJSONError(w, http.StatusBadRequest, "Invalid waypoint coordinates")
		return
	}

	seqNano := nextK9Nano()
	now := time.Unix(0, seqNano).UTC()
	newWaypoint := domain.K9TrackWaypoint{
		WaypointID: fmt.Sprintf("wp-k9-%d", seqNano),
		Lat:        req.Lat,
		Lng:        req.Lng,
		Behavior:   req.Behavior,
		Notes:      strings.TrimSpace(req.Notes),
		RecordedAt: now,
	}
	if err := newWaypoint.Validate(); err != nil {
		writeCockpitJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var updatedMission domain.K9ScentMission
	err := s.stateStore.UpdateState(r.Context(), store.CollectionK9ScentMissions, missionID, func(current []byte) ([]byte, error) {
		var m domain.K9ScentMission
		if err := json.Unmarshal(current, &m); err != nil {
			return nil, fmt.Errorf("corrupt K9 mission: %w", err)
		}
		m.Waypoints = append(m.Waypoints, newWaypoint)
		m.UpdatedAt = now
		m = k9.RecalculateMissionMetrics(m)
		updatedMission = m
		return json.Marshal(m)
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "K9 mission not found")
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to update K9 mission: %v", err))
		return
	}

	var promotedSighting *domain.Sighting
	if req.PromoteToSighting {
		sightingID := fmt.Sprintf("sighting-k9-%d", nextK9Nano())
		coords := &domain.LocationPoint{
			Latitude:  req.Lat,
			Longitude: req.Lng,
		}
		sighting := domain.Sighting{
			ID:                  sightingID,
			SightingID:          sightingID,
			PetID:               updatedMission.PetID,
			LostPetID:           updatedMission.PetID,
			ReportedAt:          now,
			SightedAt:           now,
			LocationDescription: fmt.Sprintf("K9 %s (%s)", updatedMission.CanineName, updatedMission.CanineCallsign),
			Coordinates:         coords,
			Notes:               fmt.Sprintf("[K9 %s %s] %s", updatedMission.CanineName, req.Behavior, strings.TrimSpace(req.Notes)),
			Status:              domain.SightingStatusActive,
		}
		if sBytes, err := json.Marshal(sighting); err == nil {
			if err := s.stateStore.SaveState(r.Context(), store.SightingsCollection, sighting.ID, sBytes); err == nil {
				promotedSighting = &sighting
				updatedMission.PromotedSightingID = sighting.ID
				if mBytes, err := json.Marshal(updatedMission); err == nil {
					_ = s.stateStore.SaveState(r.Context(), store.CollectionK9ScentMissions, updatedMission.MissionID, mBytes)
				}
			}
		}
	}

	var recordedWaypoint *domain.K9TrackWaypoint
	if n := len(updatedMission.Waypoints); n > 0 {
		wp := updatedMission.Waypoints[n-1]
		recordedWaypoint = &wp
	}

	s.broadcastK9Event("k9_mission_updated", map[string]any{
		"type":     "k9_mission_updated",
		"mission":  updatedMission,
		"waypoint": recordedWaypoint,
		"sighting": promotedSighting,
	}, now)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(k9MissionEnvelope{
		K9ScentMission: updatedMission,
		Mission:        updatedMission,
		Waypoint:       recordedWaypoint,
		Sighting:       promotedSighting,
	})
}

func (s *Server) handleExportK9MissionGPX(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.stateStore == nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "state store uninitialized")
		return
	}

	missionID := strings.TrimSpace(r.PathValue("id"))
	if missionID == "" {
		writeCockpitJSONError(w, http.StatusBadRequest, "mission id is required")
		return
	}

	s.ensureDefaultK9Missions(r.Context())

	raw, err := s.stateStore.GetState(r.Context(), store.CollectionK9ScentMissions, missionID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
			writeCockpitJSONError(w, http.StatusNotFound, "K9 mission not found")
			return
		}
		writeCockpitJSONError(w, http.StatusInternalServerError, "Failed to load K9 mission")
		return
	}

	var mission domain.K9ScentMission
	if err := json.Unmarshal(raw, &mission); err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, "Corrupt K9 mission record")
		return
	}

	gpxXML, err := k9.ExportK9MissionGPX(mission)
	if err != nil {
		writeCockpitJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to export GPX: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/gpx+xml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"k9-mission-%s.gpx\"", mission.MissionID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(gpxXML))
}

func (s *Server) handleK9Stream(w http.ResponseWriter, r *http.Request) {
	s.serveCockpitSSEStream(w, r, "k9")
}

func (s *Server) broadcastK9Event(eventType string, payload map[string]any, now time.Time) {
	s.broadcastCockpitEvent("k9", "k9-scent-telemetry-hub", eventType, payload, now)
}
