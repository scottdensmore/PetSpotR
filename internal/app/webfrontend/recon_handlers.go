package webfrontend

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/mesh"
	"github.com/scottdensmore/petspotr/pkg/recon"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
	"github.com/scottdensmore/petspotr/pkg/store"
)

// maxReconPayloadBytes limits incoming telemetry / image uploads to 32MB.
const maxReconPayloadBytes = 32 << 20

func generateMissionID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("mission-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("mission-%s", hex.EncodeToString(random[:]))
}

// handleApiReconMissions routes GET and POST requests for drone missions.
func (s *Server) handleApiReconMissions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.handleCreateReconMission(w, r)
	case http.MethodGet:
		s.handleGetReconMissions(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCreateReconMission registers a new drone reconnaissance sortie.
func (s *Server) handleCreateReconMission(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1048576)
	var req struct {
		ID            string `json:"id,omitempty"`
		PetID         string `json:"petId"`
		SearchPartyID string `json:"searchPartyId,omitempty"`
		PilotCallsign string `json:"pilotCallsign"`
		DroneModel    string `json:"droneModel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	req.PetID = strings.TrimSpace(req.PetID)
	req.PilotCallsign = strings.TrimSpace(req.PilotCallsign)
	req.DroneModel = strings.TrimSpace(req.DroneModel)
	req.SearchPartyID = strings.TrimSpace(req.SearchPartyID)

	if req.PetID == "" {
		http.Error(w, "petId is required", http.StatusBadRequest)
		return
	}
	if req.PilotCallsign == "" {
		http.Error(w, "pilotCallsign is required", http.StatusBadRequest)
		return
	}
	if req.DroneModel == "" {
		http.Error(w, "droneModel is required", http.StatusBadRequest)
		return
	}

	missionID := strings.TrimSpace(req.ID)
	if missionID == "" {
		missionID = generateMissionID()
	}

	mission := domain.DroneMission{
		ID:               missionID,
		PetID:            req.PetID,
		SearchPartyID:    req.SearchPartyID,
		PilotCallsign:    req.PilotCallsign,
		DroneModel:       req.DroneModel,
		StartTime:        time.Now().UTC(),
		Status:           domain.FlightStatusActive,
		Waypoints:        []domain.DroneWaypoint{},
		FootprintPoly:    [][]float64{},
		Hotspots:         []domain.ThermalHotspot{},
		SweptAreaSqM:     0.0,
		CoveredSectorIDs: []string{},
	}

	mBytes, err := json.Marshal(mission)
	if err != nil {
		http.Error(w, "Failed to encode mission", http.StatusInternalServerError)
		return
	}

	if err := s.stateStore.SaveState(r.Context(), store.CollectionReconMissions, mission.ID, mBytes); err != nil {
		http.Error(w, "Failed to persist mission", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(mission)
}

// handleGetReconMissions retrieves drone missions, optionally filtered by petId.
func (s *Server) handleGetReconMissions(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	petID := strings.TrimSpace(r.URL.Query().Get("petId"))

	rawMissions, err := s.stateStore.ListState(r.Context(), store.CollectionReconMissions)
	if err != nil && !errors.Is(err, store.ErrStoreNotFound) && !errors.Is(err, store.ErrNotFound) {
		http.Error(w, "Failed to list recon missions", http.StatusInternalServerError)
		return
	}

	missions := make([]domain.DroneMission, 0)
	for _, b := range rawMissions {
		var m domain.DroneMission
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		if petID != "" && m.PetID != petID {
			continue
		}
		missions = append(missions, m)
	}

	sort.Slice(missions, func(i, j int) bool {
		if !missions[i].StartTime.Equal(missions[j].StartTime) {
			return missions[i].StartTime.After(missions[j].StartTime)
		}
		return missions[i].ID > missions[j].ID
	})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(missions)
}

// handleApiReconTelemetryParse parses flight logs (.srt, .kml, .geojson) and returns waypoints, footprint, and swept sectors.
func (s *Server) handleApiReconTelemetryParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleParseReconTelemetry(w, r)
}

// handleParseReconTelemetry executes flight log ingestion.
func (s *Server) handleParseReconTelemetry(w http.ResponseWriter, r *http.Request) {
	var (
		content       []byte
		filename      string
		searchPartyID string
		petID         string
		missionID     string
	)

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxReconPayloadBytes); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse multipart form: %v", err), http.StatusBadRequest)
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			file, header, err = r.FormFile("telemetry")
		}
		if err != nil {
			http.Error(w, "Missing file or telemetry part in multipart form", http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()

		var buf bytes.Buffer
		if _, err := io.Copy(&buf, file); err != nil {
			http.Error(w, "Failed to read uploaded file", http.StatusBadRequest)
			return
		}
		content = buf.Bytes()
		filename = header.Filename
		searchPartyID = strings.TrimSpace(r.FormValue("searchPartyId"))
		petID = strings.TrimSpace(r.FormValue("petId"))
		missionID = strings.TrimSpace(r.FormValue("missionId"))
	} else {
		r.Body = http.MaxBytesReader(w, r.Body, maxReconPayloadBytes)
		var req struct {
			Content       string `json:"content"`
			Filename      string `json:"filename,omitempty"`
			SearchPartyID string `json:"searchPartyId,omitempty"`
			PetID         string `json:"petId,omitempty"`
			MissionID     string `json:"missionId,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}
		content = []byte(req.Content)
		filename = req.Filename
		searchPartyID = strings.TrimSpace(req.SearchPartyID)
		petID = strings.TrimSpace(req.PetID)
		missionID = strings.TrimSpace(req.MissionID)
	}

	if len(content) == 0 {
		http.Error(w, "Telemetry content cannot be empty", http.StatusBadRequest)
		return
	}

	// Parse waypoints from content
	var waypoints []domain.DroneWaypoint
	var parseErr error

	if filename != "" {
		waypoints, parseErr = recon.ParseFlightLog(content, filename)
	} else {
		waypoints, parseErr = recon.ParseDJISRT(bytes.NewReader(content))
		if parseErr != nil || len(waypoints) == 0 {
			waypoints, parseErr = recon.ParseFlightLog(content, "flight.srt")
		}
	}

	if parseErr != nil || len(waypoints) == 0 {
		http.Error(w, fmt.Sprintf("Failed to parse telemetry: %v", parseErr), http.StatusBadRequest)
		return
	}

	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	// Compute overall footprint polygon
	var footprint [][]float64
	if len(waypoints) == 1 {
		wp := waypoints[0]
		if wp.AltitudeAGL <= 0 {
			wp.AltitudeAGL = 30.0
		}
		if wp.GimbalPitchDeg >= 0 {
			wp.GimbalPitchDeg = -90.0
		}
		footprint, _ = recon.ComputeCameraFrustumFootprint(wp, cam)
	} else {
		var allCorners [][2]float64
		for _, wp := range waypoints {
			wpNorm := wp
			if wpNorm.AltitudeAGL <= 0 {
				wpNorm.AltitudeAGL = 30.0
			}
			if wpNorm.GimbalPitchDeg >= 0 {
				wpNorm.GimbalPitchDeg = -90.0
			}
			poly, err := recon.ComputeCameraFrustumFootprint(wpNorm, cam)
			if err == nil {
				for _, pt := range poly {
					if len(pt) >= 2 {
						allCorners = append(allCorners, [2]float64{pt[0], pt[1]})
					}
				}
			} else {
				// Fallback small box around waypoint
				allCorners = append(allCorners,
					[2]float64{wp.Longitude - 0.0002, wp.Latitude - 0.0002},
					[2]float64{wp.Longitude + 0.0002, wp.Latitude - 0.0002},
					[2]float64{wp.Longitude + 0.0002, wp.Latitude + 0.0002},
					[2]float64{wp.Longitude - 0.0002, wp.Latitude + 0.0002},
				)
			}
		}
		footprint = computeConvexHull(allCorners)
	}

	sweptArea := recon.ComputeSweptAreaSqMeters(footprint)

	// Determine intersecting search party sectors
	coveredSectorIDs := make([]string, 0)
	var targetParty searchparty.SearchParty
	var partyFound bool

	if s.stateStore != nil {
		if searchPartyID != "" {
			targetParty, partyFound, _ = s.getSearchPartyByID(r.Context(), searchPartyID)
		} else if petID != "" {
			targetParty, partyFound, _ = s.findSearchPartyByPetID(r.Context(), petID)
		}
		if partyFound && len(targetParty.Sectors) > 0 {
			domainSectors := make([]domain.SearchPartySector, 0, len(targetParty.Sectors))
			for _, sec := range targetParty.Sectors {
				domainSectors = append(domainSectors, domain.SearchPartySector{
					SectorID:      sec.SectorID,
					Name:          sec.Name,
					PolygonPoints: sec.PolygonPoints,
					Status:        string(sec.Status),
					TotalAreaSqM:  sec.TotalAreaSqM,
				})
			}
			coveredSectorIDs = recon.FindIntersectingSectorIDs(footprint, domainSectors)
		}
	}

	// If missionID specified, update the mission in state store
	if missionID != "" && s.stateStore != nil {
		if mBytes, err := s.stateStore.GetState(r.Context(), store.CollectionReconMissions, missionID); err == nil {
			var m domain.DroneMission
			if json.Unmarshal(mBytes, &m) == nil {
				m.Waypoints = waypoints
				m.FootprintPoly = footprint
				m.SweptAreaSqM = sweptArea
				m.CoveredSectorIDs = coveredSectorIDs
				if updatedBytes, err := json.Marshal(m); err == nil {
					_ = s.stateStore.SaveState(r.Context(), store.CollectionReconMissions, missionID, updatedBytes)
				}
			}
		}
	}

	resp := struct {
		Waypoints         []domain.DroneWaypoint `json:"waypoints"`
		FootprintPolygon  [][]float64            `json:"footprintPolygon"`
		SweptAreaSqMeters float64                `json:"sweptAreaSqMeters"`
		CoveredSectorIDs  []string               `json:"coveredSectorIds"`
	}{
		Waypoints:         waypoints,
		FootprintPolygon:  footprint,
		SweptAreaSqMeters: math.Round(sweptArea*100.0) / 100.0,
		CoveredSectorIDs:  coveredSectorIDs,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// handleApiReconThermalScan runs pure Go radiometric thermal analysis on aerial frames.
func (s *Server) handleApiReconThermalScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleScanReconThermal(w, r)
}

// handleScanReconThermal processes thermal frames.
func (s *Server) handleScanReconThermal(w http.ResponseWriter, r *http.Request) {
	var (
		img                image.Image
		missionID          string
		petID              string
		palette            = domain.PaletteWhiteHot
		wp                 domain.DroneWaypoint
		cam                = domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}
		frameTimeOffsetSec float64
	)

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxReconPayloadBytes); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse multipart form: %v", err), http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("image")
		if err != nil {
			file, _, err = r.FormFile("file")
		}
		if err != nil {
			http.Error(w, "image or file is required in multipart upload", http.StatusBadRequest)
			return
		}
		defer func() { _ = file.Close() }()

		var decodeErr error
		img, _, decodeErr = image.Decode(file)
		if decodeErr != nil {
			http.Error(w, fmt.Sprintf("Failed to decode image: %v", decodeErr), http.StatusBadRequest)
			return
		}

		missionID = strings.TrimSpace(r.FormValue("missionId"))
		petID = strings.TrimSpace(r.FormValue("petId"))
		if pal := strings.TrimSpace(r.FormValue("palette")); pal != "" {
			palette = domain.ThermalPalette(strings.ToUpper(pal))
		}

		if altStr := r.FormValue("altitudeMetersAGL"); altStr != "" {
			wp.AltitudeAGL, _ = strconv.ParseFloat(altStr, 64)
		} else if altStr := r.FormValue("altitudeAGL"); altStr != "" {
			wp.AltitudeAGL, _ = strconv.ParseFloat(altStr, 64)
		}

		if latStr := r.FormValue("latitude"); latStr != "" {
			wp.Latitude, _ = strconv.ParseFloat(latStr, 64)
		}
		if lngStr := r.FormValue("longitude"); lngStr != "" {
			wp.Longitude, _ = strconv.ParseFloat(lngStr, 64)
		}
		if headStr := r.FormValue("headingDeg"); headStr != "" {
			wp.HeadingDeg, _ = strconv.ParseFloat(headStr, 64)
		} else if headStr := r.FormValue("heading"); headStr != "" {
			wp.HeadingDeg, _ = strconv.ParseFloat(headStr, 64)
		}
		if pitchStr := r.FormValue("gimbalPitchDeg"); pitchStr != "" {
			wp.GimbalPitchDeg, _ = strconv.ParseFloat(pitchStr, 64)
		}
		if rollStr := r.FormValue("gimbalRollDeg"); rollStr != "" {
			wp.GimbalRollDeg, _ = strconv.ParseFloat(rollStr, 64)
		}
		if yawStr := r.FormValue("gimbalYawDeg"); yawStr != "" {
			wp.GimbalYawDeg, _ = strconv.ParseFloat(yawStr, 64)
		}
		if offStr := r.FormValue("frameTimeOffsetSec"); offStr != "" {
			frameTimeOffsetSec, _ = strconv.ParseFloat(offStr, 64)
		}
	} else {
		// application/json
		r.Body = http.MaxBytesReader(w, r.Body, maxReconPayloadBytes)
		var req struct {
			ImageBase64        string                  `json:"imageBase64"`
			MissionID          string                  `json:"missionId"`
			PetID              string                  `json:"petId"`
			Palette            domain.ThermalPalette   `json:"palette"`
			Waypoint           domain.DroneWaypoint    `json:"waypoint"`
			Camera             domain.CameraIntrinsics `json:"camera"`
			FrameTimeOffsetSec float64                 `json:"frameTimeOffsetSec"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		rawB64 := req.ImageBase64
		if idx := strings.Index(rawB64, ","); idx != -1 {
			rawB64 = rawB64[idx+1:]
		}
		imgBytes, err := base64.StdEncoding.DecodeString(rawB64)
		if err != nil {
			http.Error(w, "Invalid base64 image data", http.StatusBadRequest)
			return
		}
		var decodeErr error
		img, _, decodeErr = image.Decode(bytes.NewReader(imgBytes))
		if decodeErr != nil {
			http.Error(w, fmt.Sprintf("Failed to decode image: %v", decodeErr), http.StatusBadRequest)
			return
		}

		missionID = strings.TrimSpace(req.MissionID)
		petID = strings.TrimSpace(req.PetID)
		if req.Palette != "" {
			palette = req.Palette
		}
		wp = req.Waypoint
		if req.Camera.HFOV > 0 && req.Camera.VFOV > 0 {
			cam = req.Camera
		}
		frameTimeOffsetSec = req.FrameTimeOffsetSec
	}

	if wp.AltitudeAGL <= 0 {
		wp.AltitudeAGL = 30.0
	}
	if wp.GimbalPitchDeg >= 0 {
		wp.GimbalPitchDeg = -90.0
	}
	if wp.Timestamp.IsZero() {
		wp.Timestamp = time.Now().UTC()
	}

	hotspots, err := recon.AnalyzeThermalImage(img, palette, wp, cam)
	if err != nil {
		http.Error(w, fmt.Sprintf("Thermal analysis failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Enrich hotspots with mission & pet identifiers and persist
	for i := range hotspots {
		if missionID != "" {
			hotspots[i].MissionID = missionID
		}
		if petID != "" {
			hotspots[i].PetID = petID
		}
		hotspots[i].FrameTimeOffsetSec = frameTimeOffsetSec

		if s.stateStore != nil {
			hBytes, err := json.Marshal(hotspots[i])
			if err == nil {
				_ = s.stateStore.SaveState(r.Context(), store.CollectionReconHotspots, hotspots[i].ID, hBytes)
			}
		}
	}

	// Update mission record if missionID exists
	if missionID != "" && s.stateStore != nil && len(hotspots) > 0 {
		if mBytes, err := s.stateStore.GetState(r.Context(), store.CollectionReconMissions, missionID); err == nil {
			var m domain.DroneMission
			if json.Unmarshal(mBytes, &m) == nil {
				m.Hotspots = append(m.Hotspots, hotspots...)
				if updatedBytes, err := json.Marshal(m); err == nil {
					_ = s.stateStore.SaveState(r.Context(), store.CollectionReconMissions, missionID, updatedBytes)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(hotspots)
}

// handleApiReconHotspotStatus updates the operator/AI status of a thermal hotspot.
func (s *Server) handleApiReconHotspotStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PUT, PATCH")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleUpdateReconHotspotStatus(w, r)
}

// handleUpdateReconHotspotStatus executes status change and automated sighting promotion.
func (s *Server) handleUpdateReconHotspotStatus(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	hotspotID := strings.TrimSpace(r.PathValue("id"))
	if hotspotID == "" {
		http.Error(w, "Hotspot ID is required", http.StatusBadRequest)
		return
	}

	hotspotBytes, err := s.stateStore.GetState(r.Context(), store.CollectionReconHotspots, hotspotID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve hotspot", http.StatusInternalServerError)
		return
	}

	var hotspot domain.ThermalHotspot
	if err := json.Unmarshal(hotspotBytes, &hotspot); err != nil {
		http.Error(w, "Failed to decode hotspot data", http.StatusInternalServerError)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 65536)
	var req struct {
		Status domain.HotspotStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	switch req.Status {
	case domain.HotspotConfirmed, domain.HotspotDismissed, domain.HotspotUnverified:
		hotspot.Status = req.Status
	default:
		http.Error(w, fmt.Sprintf("Invalid hotspot status: %s", req.Status), http.StatusBadRequest)
		return
	}

	// If CONFIRMED, promote to Community Sighting, update sector, and broadcast mesh:thermal-hotspot
	if hotspot.Status == domain.HotspotConfirmed {
		if hotspot.LinkedSightingID == "" {
			sightingID := generateSightingID()
			hotspot.LinkedSightingID = sightingID

			coords := &domain.LocationPoint{
				Latitude:  hotspot.Latitude,
				Longitude: hotspot.Longitude,
			}

			sighting := domain.PetSightingRecord{
				SightingID:          sightingID,
				LostPetID:           hotspot.PetID,
				ReportedAt:          time.Now().UTC(),
				SightedAt:           hotspot.Timestamp,
				LocationDescription: fmt.Sprintf("Aerial Thermal Hotspot (%s)", hotspot.Classification),
				Coordinates:         coords,
				MovementDirection:   "",
				Notes: fmt.Sprintf("Thermal anomaly confirmed: %.1f°C estimated temp, %.0f%% confidence via aerial reconnaissance.",
					hotspot.EstimatedTempC, hotspot.ConfidenceScore*100),
				Status: domain.SightingStatusActive,
			}

			if sBytes, err := json.Marshal(sighting); err == nil {
				_ = s.stateStore.SaveState(r.Context(), store.SightingsCollection, sightingID, sBytes)
			}

			// Reunion SSE Hub notification
			if s.reunionHub != nil {
				event := domain.ReunionStreamEvent{
					EventID:   fmt.Sprintf("evt_sighting_%s", sightingID),
					Type:      domain.ReunionEventSighting,
					MatchID:   hotspot.PetID,
					Timestamp: time.Now().UTC(),
					Payload: domain.SightingEventPayload{
						Type:                "sighting",
						PetID:               hotspot.PetID,
						SightingID:          sightingID,
						SightedAt:           sighting.SightedAt,
						LocationDescription: sighting.LocationDescription,
						Coordinates:         sighting.Coordinates,
					},
				}
				s.reunionHub.Broadcast(event)
			}
		}

		// Find and update search party sector
		var party searchparty.SearchParty
		var partyFound bool
		var mission domain.DroneMission

		if hotspot.MissionID != "" {
			if mBytes, err := s.stateStore.GetState(r.Context(), store.CollectionReconMissions, hotspot.MissionID); err == nil {
				_ = json.Unmarshal(mBytes, &mission)
			}
		}

		if mission.SearchPartyID != "" {
			party, partyFound, _ = s.getSearchPartyByID(r.Context(), mission.SearchPartyID)
		}
		if !partyFound && hotspot.PetID != "" {
			party, partyFound, _ = s.findSearchPartyByPetID(r.Context(), hotspot.PetID)
		}

		if partyFound {
			sectorUpdated := false
			pt := domain.LocationPoint{Latitude: hotspot.Latitude, Longitude: hotspot.Longitude}

			for i := range party.Sectors {
				poly := party.Sectors[i].PolygonPoints
				if searchparty.PointInPolygon(pt, poly) {
					party.Sectors[i].Status = searchparty.SectorStatusSightingReported
					sectorUpdated = true
					break
				}
			}

			if !sectorUpdated && len(mission.CoveredSectorIDs) > 0 {
				for _, covID := range mission.CoveredSectorIDs {
					for i := range party.Sectors {
						if party.Sectors[i].SectorID == covID {
							party.Sectors[i].Status = searchparty.SectorStatusSightingReported
							sectorUpdated = true
							break
						}
					}
					if sectorUpdated {
						break
					}
				}
			}

			if sectorUpdated {
				if pBytes, err := json.Marshal(party); err == nil {
					_ = s.stateStore.SaveState(r.Context(), store.SearchPartiesCollection, party.PartyID, pBytes)
				}
				if s.reunionHub != nil {
					s.reunionHub.Broadcast(domain.ReunionStreamEvent{
						EventID:   fmt.Sprintf("evt_party_%d", time.Now().UnixNano()),
						Type:      domain.ReunionEventSearchPartyUpdated,
						MatchID:   party.PartyID,
						Timestamp: time.Now().UTC(),
						Payload: domain.SearchPartyEventPayload{
							Type:                  "search_party_updated",
							PartyID:               party.PartyID,
							Status:                string(searchparty.SectorStatusSightingReported),
							CoveragePercentage:    party.CoveragePercentage,
							ActiveVolunteersCount: party.ActiveVolunteersCount,
						},
					})
				}
			}

			// Broadcast mesh:thermal-hotspot to WebRTC signaling peers
			if s.signalingHub != nil {
				payloadMap := map[string]interface{}{
					"id":             hotspot.ID,
					"missionId":      hotspot.MissionID,
					"petId":          hotspot.PetID,
					"latitude":       hotspot.Latitude,
					"longitude":      hotspot.Longitude,
					"confidence":     hotspot.ConfidenceScore,
					"estimatedTempC": hotspot.EstimatedTempC,
					"palette":        hotspot.Palette,
					"thumbnail":      hotspot.ThumbnailBase64,
					"status":         hotspot.Status,
					"classification": hotspot.Classification,
				}
				payloadBytes, _ := json.Marshal(payloadMap)
				envelope := mesh.SignalingEnvelope{
					Type:          mesh.SignalingType("THERMAL_HOTSPOT"),
					SearchPartyID: party.PartyID,
					SenderNodeID:  "pilot-station",
					Payload:       payloadBytes,
					Timestamp:     time.Now().UTC(),
				}
				s.signalingHub.Broadcast(envelope)
			}
		}
	}

	// Persist updated hotspot
	updatedBytes, err := json.Marshal(hotspot)
	if err != nil {
		http.Error(w, "Failed to encode updated hotspot", http.StatusInternalServerError)
		return
	}
	if err := s.stateStore.SaveState(r.Context(), store.CollectionReconHotspots, hotspot.ID, updatedBytes); err != nil {
		http.Error(w, "Failed to persist updated hotspot", http.StatusInternalServerError)
		return
	}

	// Update mission if hotspot is linked
	if hotspot.MissionID != "" {
		if mBytes, err := s.stateStore.GetState(r.Context(), store.CollectionReconMissions, hotspot.MissionID); err == nil {
			var m domain.DroneMission
			if json.Unmarshal(mBytes, &m) == nil {
				for i := range m.Hotspots {
					if m.Hotspots[i].ID == hotspot.ID {
						m.Hotspots[i] = hotspot
						break
					}
				}
				if updatedMBytes, err := json.Marshal(m); err == nil {
					_ = s.stateStore.SaveState(r.Context(), store.CollectionReconMissions, hotspot.MissionID, updatedMBytes)
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(hotspot)
}

// handleApiReconMissionExport exports mission telemetry and hotspots as GeoJSON FeatureCollection.
func (s *Server) handleApiReconMissionExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.handleExportReconMission(w, r)
}

// handleExportReconMission builds the GeoJSON export.
func (s *Server) handleExportReconMission(w http.ResponseWriter, r *http.Request) {
	if s.stateStore == nil {
		http.Error(w, "state store uninitialized", http.StatusInternalServerError)
		return
	}

	missionID := strings.TrimSpace(r.PathValue("id"))
	if missionID == "" {
		http.NotFound(w, r)
		return
	}

	mBytes, err := s.stateStore.GetState(r.Context(), store.CollectionReconMissions, missionID)
	if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrStoreNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "Failed to retrieve mission", http.StatusInternalServerError)
		return
	}

	var mission domain.DroneMission
	if err := json.Unmarshal(mBytes, &mission); err != nil {
		http.Error(w, "Failed to decode mission", http.StatusInternalServerError)
		return
	}

	features := make([]map[string]interface{}, 0)

	// Feature 1: Flight Trajectory (LineString)
	if len(mission.Waypoints) > 0 {
		lineCoords := make([][]float64, 0, len(mission.Waypoints))
		for _, wp := range mission.Waypoints {
			lineCoords = append(lineCoords, []float64{wp.Longitude, wp.Latitude, wp.AltitudeAGL})
		}
		features = append(features, map[string]interface{}{
			"type": "Feature",
			"geometry": map[string]interface{}{
				"type":        "LineString",
				"coordinates": lineCoords,
			},
			"properties": map[string]interface{}{
				"name":           "Flight Path",
				"missionId":      mission.ID,
				"petId":          mission.PetID,
				"pilotCallsign":  mission.PilotCallsign,
				"droneModel":     mission.DroneModel,
				"status":         mission.Status,
				"waypointsCount": len(mission.Waypoints),
			},
		})
	}

	// Feature 2: Frustum Footprint (Polygon)
	if len(mission.FootprintPoly) >= 3 {
		features = append(features, map[string]interface{}{
			"type": "Feature",
			"geometry": map[string]interface{}{
				"type":        "Polygon",
				"coordinates": [][][]float64{mission.FootprintPoly},
			},
			"properties": map[string]interface{}{
				"name":              "Camera Frustum Footprint",
				"missionId":         mission.ID,
				"sweptAreaSqMeters": mission.SweptAreaSqM,
				"coveredSectorIds":  mission.CoveredSectorIDs,
			},
		})
	}

	// Feature 3...N: Thermal Hotspots (Points)
	for _, h := range mission.Hotspots {
		features = append(features, map[string]interface{}{
			"type": "Feature",
			"geometry": map[string]interface{}{
				"type":        "Point",
				"coordinates": []float64{h.Longitude, h.Latitude},
			},
			"properties": map[string]interface{}{
				"id":                 h.ID,
				"missionId":          mission.ID,
				"petId":              h.PetID,
				"estimatedTempC":     h.EstimatedTempC,
				"confidenceScore":    h.ConfidenceScore,
				"status":             h.Status,
				"classification":     h.Classification,
				"palette":            h.Palette,
				"timestamp":          h.Timestamp,
				"frameTimeOffsetSec": h.FrameTimeOffsetSec,
				"linkedSightingId":   h.LinkedSightingID,
			},
		})
	}

	featureCollection := map[string]interface{}{
		"type":     "FeatureCollection",
		"features": features,
	}

	w.Header().Set("Content-Type", "application/geo+json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"mission-%s-export.geojson\"", mission.ID))
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(featureCollection)
}

// computeConvexHull calculates the 2D convex hull of coordinates using Monotone Chain.
func computeConvexHull(pts [][2]float64) [][]float64 {
	if len(pts) == 0 {
		return [][]float64{}
	}
	if len(pts) < 3 {
		res := make([][]float64, 0, len(pts)+1)
		for _, p := range pts {
			res = append(res, []float64{p[0], p[1]})
		}
		if len(pts) > 0 {
			res = append(res, []float64{pts[0][0], pts[0][1]})
		}
		return res
	}

	// Sort points primarily by X (lng), secondarily by Y (lat)
	sort.Slice(pts, func(i, j int) bool {
		if pts[i][0] != pts[j][0] {
			return pts[i][0] < pts[j][0]
		}
		return pts[i][1] < pts[j][1]
	})

	unique := make([][2]float64, 0, len(pts))
	for i, p := range pts {
		if i == 0 || (math.Abs(p[0]-unique[len(unique)-1][0]) > 1e-9 || math.Abs(p[1]-unique[len(unique)-1][1]) > 1e-9) {
			unique = append(unique, p)
		}
	}

	if len(unique) < 3 {
		res := make([][]float64, 0, len(unique)+1)
		for _, p := range unique {
			res = append(res, []float64{p[0], p[1]})
		}
		res = append(res, []float64{unique[0][0], unique[0][1]})
		return res
	}

	cross := func(o, a, b [2]float64) float64 {
		return (a[0]-o[0])*(b[1]-o[1]) - (a[1]-o[1])*(b[0]-o[0])
	}

	// Lower hull
	lower := make([][2]float64, 0, len(unique))
	for _, p := range unique {
		for len(lower) >= 2 && cross(lower[len(lower)-2], lower[len(lower)-1], p) <= 0 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, p)
	}

	// Upper hull
	upper := make([][2]float64, 0, len(unique))
	for i := len(unique) - 1; i >= 0; i-- {
		p := unique[i]
		for len(upper) >= 2 && cross(upper[len(upper)-2], upper[len(upper)-1], p) <= 0 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, p)
	}

	// Concatenate lower and upper hull
	hull := append(lower[:len(lower)-1], upper...)

	res := make([][]float64, len(hull))
	for i, p := range hull {
		res[i] = []float64{p[0], p[1]}
	}
	return res
}
