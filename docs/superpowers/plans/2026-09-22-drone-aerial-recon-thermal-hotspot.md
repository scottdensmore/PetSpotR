# Milestone 11.2: Drone / UAV Aerial Reconnaissance & Thermal Hotspot Sighting Feeds Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement PetSpotR's Drone / UAV Aerial Reconnaissance and Thermal Hotspot Sighting subsystem, enabling video/SRT telemetry synchronization, photogrammetric ground plane projection, pure Go radiometric thermal anomaly detection, multimodal verification, accessible operator HUD cockpit, and off-grid P2P mesh alert propagation.

**Architecture:** A pure standard library Go package `pkg/recon` processes DJI/Autel SRT subtitle logs and aerial stills, projecting camera view frustums onto terrain and detecting animal-scale thermal signatures across White-Hot, Black-Hot, and Ironbow palettes. REST handlers in `internal/app/webfrontend` expose telemetry parsing, thermal scanning, and sighting promotion, broadcasting `mesh:thermal-hotspot` events over WebRTC DataChannels (Milestone 11.1). A responsive, WCAG AAA-compliant client cockpit (`drone-recon.js` + `drone_modal.html`) provides synchronized telemetry HUD gauges, reticle overlay, and Leaflet map tracking.

**Tech Stack:** Go 1.25.8 / Go 1.26.5 toolchain (`image`, `image/color`, `image/jpeg`, `image/png`), Vanilla ES2022 JavaScript, Leaflet 1.9.4, IndexedDB, WebRTC DataChannel, Ollama Gemma 4 (`gemma4:e2b`), Playwright E2E.

**Spec:** [`docs/superpowers/specs/2026-09-22-drone-aerial-recon-thermal-hotspot-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-22-drone-aerial-recon-thermal-hotspot-design.md)

## Global Constraints

- Pinned toolchain: `export GOTOOLCHAIN=go1.26.5`.
- Zero external CGO dependencies: Pure Go standard library for image decoding and spatial math.
- Strict CSP compliance: Zero inline `<script>`, zero `eval()`, zero inline event handlers.
- Strict WCAG AAA compliance: All text, icons, and indicators $\ge 7:1$ contrast in light and dark mode.
- Non-blocking, graceful fallback: Deterministic radiometric confidence score when Ollama is offline.
- Pre-push verification gate: Clean pass on `make verify` (vet, lint, OpenTofu, yamllint, tests `-race -cover`) and all Playwright tests.

---

### Task 1: Domain Models, Store Collections & Pure Go DJI/Autel Telemetry Parser

**Files:**
- Create: `pkg/domain/recon.go`
- Modify: `pkg/store/store.go`
- Create: `pkg/recon/telemetry.go`
- Create: `pkg/recon/telemetry_test.go`

**Interfaces:**
- Produces:
  - `domain.FlightStatus`, `domain.HotspotStatus`, `domain.ThermalPalette`, `domain.NormalizedBox`
  - `domain.DroneWaypoint`, `domain.ThermalHotspot`, `domain.DroneMission`, `domain.CameraIntrinsics`
  - `store.CollectionReconMissions = "recon_missions"`, `store.CollectionReconHotspots = "recon_hotspots"`
  - `recon.ParseDJISRT(r io.Reader) ([]domain.DroneWaypoint, error)`
  - `recon.ParseFlightLog(content []byte, filename string) ([]domain.DroneWaypoint, error)`

- [ ] **Step 1: Write the failing test for telemetry parsing**

In `pkg/recon/telemetry_test.go`:
```go
package recon_test

import (
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/recon"
)

func TestParseDJISRT_StandardFormat(t *testing.T) {
	srtContent := `1
00:00:01,000 --> 00:00:02,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude : 37.774929] [longitude : -122.419416] [rel_alt: 45.200] [heading: 142.5] [pitch: -45.0] [roll: 0.0] [yaw: 142.5]

2
00:00:02,000 --> 00:00:03,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude : 37.775010] [longitude : -122.419350] [rel_alt: 45.500] [heading: 143.0] [pitch: -45.0] [roll: 0.0] [yaw: 143.0]
`
	waypoints, err := recon.ParseDJISRT(strings.NewReader(srtContent))
	if err != nil {
		t.Fatalf("unexpected error parsing SRT: %v", err)
	}
	if len(waypoints) != 2 {
		t.Fatalf("expected 2 waypoints, got %d", len(waypoints))
	}
	wp1 := waypoints[0]
	if wp1.Latitude != 37.774929 || wp1.Longitude != -122.419416 {
		t.Errorf("expected lat/lng 37.774929/-122.419416, got %f/%f", wp1.Latitude, wp1.Longitude)
	}
	if wp1.AltitudeAGL != 45.2 {
		t.Errorf("expected altitude 45.2, got %f", wp1.AltitudeAGL)
	}
	if wp1.HeadingDeg != 142.5 || wp1.GimbalPitchDeg != -45.0 {
		t.Errorf("expected heading 142.5 and pitch -45.0, got %f / %f", wp1.HeadingDeg, wp1.GimbalPitchDeg)
	}
}

func TestParseDJISRT_AlternateFormatWithDLat(t *testing.T) {
	srtContent := `1
00:00:00,500 --> 00:00:01,000
HOME(-122.4194,37.7749) 2026.09.22 12:00:00
GPS(-122.4194,37.7749,15) [dlatitude: 37.774950] [dlongitude: -122.419400] [altitude: 50.0] [rel_alt: 40.0] [heading: 90.0]
`
	waypoints, err := recon.ParseDJISRT(strings.NewReader(srtContent))
	if err != nil {
		t.Fatalf("unexpected error parsing alternate SRT: %v", err)
	}
	if len(waypoints) != 1 {
		t.Fatalf("expected 1 waypoint, got %d", len(waypoints))
	}
	if waypoints[0].Latitude != 37.774950 || waypoints[0].AltitudeAGL != 40.0 {
		t.Errorf("unexpected waypoint values: %+v", waypoints[0])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/recon/... -run TestParseDJISRT`
Expected: Compilation failure (`undefined: recon.ParseDJISRT`).

- [ ] **Step 3: Implement domain models and telemetry parser**

1. Create `pkg/domain/recon.go`:
   Implement `FlightStatus`, `HotspotStatus`, `ThermalPalette`, `NormalizedBox`, `DroneWaypoint`, `ThermalHotspot`, `DroneMission`, `CameraIntrinsics`.
2. Add store collection constants in `pkg/store/store.go`:
   `CollectionReconMissions = "recon_missions"`
   `CollectionReconHotspots = "recon_hotspots"`
3. Implement `pkg/recon/telemetry.go`:
   Implement `ParseDJISRT(r io.Reader) ([]domain.DroneWaypoint, error)` and `ParseFlightLog(content []byte, filename string) ([]domain.DroneWaypoint, error)` with regex extraction for latitude, longitude, altitude, heading, and gimbal pitch/roll/yaw.

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/recon/... -run TestParseDJISRT`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add pkg/domain/recon.go pkg/store/store.go pkg/recon/telemetry.go pkg/recon/telemetry_test.go
git commit -m "feat(recon): add domain models, store constants, and DJI SRT telemetry parser"
```

---

### Task 2: Photogrammetric Ground Projection & Raycasting Engine

**Files:**
- Create: `pkg/recon/projection.go`
- Create: `pkg/recon/projection_test.go`

**Interfaces:**
- Consumes: `domain.DroneWaypoint`, `domain.CameraIntrinsics`, `domain.NormalizedBox` from Task 1.
- Produces:
  - `recon.ComputeCameraFrustumFootprint(wp domain.DroneWaypoint, cam domain.CameraIntrinsics) ([][]float64, error)`
  - `recon.RaycastPixelToGround(u, v float64, wp domain.DroneWaypoint, cam domain.CameraIntrinsics) (lat, lng float64, err error)`
  - `recon.ComputeSweptAreaSqMeters(poly [][]float64) float64`
  - `recon.FindIntersectingSectorIDs(poly [][]float64, sectors []domain.SearchPartySector) []string`

- [ ] **Step 1: Write the failing test for ground projection**

In `pkg/recon/projection_test.go`:
```go
package recon_test

import (
	"math"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/recon"
)

func TestComputeCameraFrustumFootprint_Nadir(t *testing.T) {
	wp := domain.DroneWaypoint{
		Timestamp:      time.Now(),
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    50.0,  // 50 meters AGL
		HeadingDeg:     0.0,   // True North
		GimbalPitchDeg: -90.0, // Nadir downward
	}
	cam := domain.CameraIntrinsics{
		HFOV: 84.0,
		VFOV: 60.0,
	}

	poly, err := recon.ComputeCameraFrustumFootprint(wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(poly) != 5 { // 4 corners + closed loop
		t.Fatalf("expected 5 vertices in closed polygon, got %d", len(poly))
	}

	// Verify centroid is near drone lat/lng
	var sumLat, sumLng float64
	for i := 0; i < 4; i++ {
		sumLng += poly[i][0]
		sumLat += poly[i][1]
	}
	avgLat := sumLat / 4.0
	avgLng := sumLng / 4.0
	if math.Abs(avgLat-wp.Latitude) > 0.0001 || math.Abs(avgLng-wp.Longitude) > 0.0001 {
		t.Errorf("expected footprint centroid near %f,%f; got %f,%f", wp.Latitude, wp.Longitude, avgLat, avgLng)
	}

	area := recon.ComputeSweptAreaSqMeters(poly)
	if area < 1000.0 || area > 10000.0 {
		t.Errorf("expected ground footprint area roughly ~4500-6000 m^2 for 50m AGL, got %f", area)
	}
}

func TestRaycastPixelToGround_CenterPixel(t *testing.T) {
	wp := domain.DroneWaypoint{
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    60.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: -90.0,
	}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	lat, lng, err := recon.RaycastPixelToGround(0.5, 0.5, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(lat-wp.Latitude) > 0.00001 || math.Abs(lng-wp.Longitude) > 0.00001 {
		t.Errorf("center pixel (0.5, 0.5) at nadir should project to drone location; got %f, %f", lat, lng)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/recon/... -run TestComputeCameraFrustumFootprint`
Expected: Compilation failure (`undefined: recon.ComputeCameraFrustumFootprint`).

- [ ] **Step 3: Implement photogrammetric projection math**

In `pkg/recon/projection.go`:
- Implement `ComputeCameraFrustumFootprint`: calculates camera trapezoid ground intersections from altitude AGL, gimbal pitch, and heading, rotating and translating to WGS84 coordinates.
- Implement `RaycastPixelToGround`: translates $(u, v) \in [0, 1]$ to angular offsets from optical center, rotates by drone attitude, intersects ground plane, and returns WGS84 $(\text{lat}, \text{lng})$.
- Implement `ComputeSweptAreaSqMeters`: spherical polygon area formula in square meters.
- Implement `FindIntersectingSectorIDs`: polygon bounding-box / ray-casting intersection against search party sector polygons.

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/recon/... -run "TestComputeCameraFrustumFootprint|TestRaycastPixelToGround"`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add pkg/recon/projection.go pkg/recon/projection_test.go
git commit -m "feat(recon): implement photogrammetric ground projection and raycasting engine"
```

---

### Task 3: Pure Go Radiometric Thermal Analyzer & Multimodal Verifier

**Files:**
- Create: `pkg/recon/thermal.go`
- Create: `pkg/recon/verifier.go`
- Create: `pkg/recon/thermal_test.go`
- Create: `pkg/recon/verifier_test.go`

**Interfaces:**
- Consumes: `domain.DroneWaypoint`, `domain.ThermalPalette`, `domain.ThermalHotspot`, `domain.CameraIntrinsics`
- Produces:
  - `recon.AnalyzeThermalImage(img image.Image, palette domain.ThermalPalette, wp domain.DroneWaypoint, cam domain.CameraIntrinsics) ([]domain.ThermalHotspot, error)`
  - `recon.VerifyHotspotWithOllama(ctx context.Context, ollamaClient *ollama.Client, hotspot domain.ThermalHotspot, fullImage image.Image) (domain.ThermalHotspot, error)`

- [ ] **Step 1: Write the failing test for radiometric thermal analysis**

In `pkg/recon/thermal_test.go`:
```go
package recon_test

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/recon"
)

func TestAnalyzeThermalImage_WhiteHot(t *testing.T) {
	// Create 100x100 dark grayscale background (cold terrain)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
		}
	}
	// Inject a 6x6 bright hot cluster at (50, 50) simulating a pet body
	for y := 47; y <= 53; y++ {
		for x := 47; x <= 53; x++ {
			img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}

	wp := domain.DroneWaypoint{
		Timestamp:      time.Now(),
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    30.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: -90.0,
	}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	hotspots, err := recon.AnalyzeThermalImage(img, domain.PaletteWhiteHot, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hotspots) != 1 {
		t.Fatalf("expected 1 detected hotspot, got %d", len(hotspots))
	}
	h := hotspots[0]
	if h.ConfidenceScore < 0.70 {
		t.Errorf("expected high confidence score, got %f", h.ConfidenceScore)
	}
	if h.BoundingBox.X < 0.40 || h.BoundingBox.X > 0.55 {
		t.Errorf("expected bounding box X centered around 0.5, got %f", h.BoundingBox.X)
	}
}

func TestAnalyzeThermalImage_SuppressUniformSolarRoad(t *testing.T) {
	// Create a large 80x20 hot horizontal bar (e.g. solar-heated road)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			if y >= 40 && y <= 60 && x >= 10 && x <= 90 {
				img.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
			} else {
				img.Set(x, y, color.RGBA{R: 30, G: 30, B: 30, A: 255})
			}
		}
	}

	wp := domain.DroneWaypoint{AltitudeAGL: 30.0, GimbalPitchDeg: -90.0}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	hotspots, err := recon.AnalyzeThermalImage(img, domain.PaletteWhiteHot, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Massive continuous road should be rejected by GSD maximum ground dimension filter
	if len(hotspots) != 0 {
		t.Fatalf("expected road to be filtered out as false-positive, got %d hotspots", len(hotspots))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/recon/... -run TestAnalyzeThermalImage`
Expected: Compilation failure (`undefined: recon.AnalyzeThermalImage`).

- [ ] **Step 3: Implement thermal image analyzer & verifier**

1. In `pkg/recon/thermal.go`:
   - Implement luminance/chrominance conversion for `WHITE_HOT`, `BLACK_HOT`, and `IRONBOW`.
   - Calculate adaptive mean and standard deviation threshold.
   - Connected-component 8-way flood fill to label candidate blobs.
   - Ground Sample Distance (GSD) size filtering (rejecting $< 0.2\text{m}$ and $> 2.0\text{m}$ objects).
   - Local ring contrast evaluation.
   - Crop thumbnail, encode base64 JPEG, and raycast centroid to real-world ground coordinates via `RaycastPixelToGround`.
2. In `pkg/recon/verifier.go`:
   - Implement `VerifyHotspotWithOllama`: optional multimodal evaluation via `gemma4:e2b` with a 1.5s timeout. If unavailable, falls back gracefully to radiometric scoring.

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/recon/... -run TestAnalyzeThermalImage`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add pkg/recon/thermal.go pkg/recon/verifier.go pkg/recon/thermal_test.go pkg/recon/verifier_test.go
git commit -m "feat(recon): implement pure Go radiometric thermal analyzer and multimodal verifier"
```

---

### Task 4: WebFrontend REST Endpoints, State Handlers & P2P Mesh Integration

**Files:**
- Create: `internal/app/webfrontend/recon_handlers.go`
- Modify: `internal/app/webfrontend/server.go`
- Create: `internal/app/webfrontend/recon_handlers_test.go`

**Interfaces:**
- Consumes: `pkg/domain/recon.go`, `pkg/recon`, `pkg/store`, `pkg/mesh`
- Produces:
  - `POST /api/v1/recon/missions`
  - `GET /api/v1/recon/missions`
  - `POST /api/v1/recon/telemetry/parse`
  - `POST /api/v1/recon/thermal/scan`
  - `PUT /api/v1/recon/hotspots/{id}/status`
  - `GET /api/v1/recon/missions/{id}/export`

- [ ] **Step 1: Write the failing test for recon endpoints**

In `internal/app/webfrontend/recon_handlers_test.go`:
```go
package webfrontend_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestReconMissionsEndpoint(t *testing.T) {
	server := webfrontend.NewTestServer(t)

	missionReq := map[string]interface{}{
		"petId":         "test-pet-101",
		"pilotCallsign": "SkyWatcher-1",
		"droneModel":    "DJI Matrice 30T",
	}
	body, _ := json.Marshal(missionReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/recon/missions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusOK {
		t.Fatalf("expected 200/201, got %d (body: %s)", w.Code, w.Body.String())
	}

	var mission domain.DroneMission
	if err := json.Unmarshal(w.Body.Bytes(), &mission); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if mission.PilotCallsign != "SkyWatcher-1" || mission.PetID != "test-pet-101" {
		t.Errorf("unexpected mission response: %+v", mission)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/... -run TestReconMissionsEndpoint`
Expected: 404 Not Found.

- [ ] **Step 3: Implement webfrontend REST handlers**

1. Create `internal/app/webfrontend/recon_handlers.go`:
   - Implement `handleReconMissions` (POST & GET).
   - Implement `handleReconTelemetryParse` (multipart/json parser for SRT/KML/GeoJSON).
   - Implement `handleReconThermalScan` (multipart image upload + telemetry analysis).
   - Implement `handleReconHotspotStatus` (confirmation triggers `pkg/sighting` registration and broadcasts `mesh:thermal-hotspot` over WebRTC DataChannel / SSE).
   - Implement `handleReconMissionExport` (GeoJSON FeatureCollection export).
2. Wire routes into `internal/app/webfrontend/server.go`:
   ```go
   s.router.HandleFunc("POST /api/v1/recon/missions", s.handleCreateReconMission)
   s.router.HandleFunc("GET /api/v1/recon/missions", s.handleGetReconMissions)
   s.router.HandleFunc("POST /api/v1/recon/telemetry/parse", s.handleParseReconTelemetry)
   s.router.HandleFunc("POST /api/v1/recon/thermal/scan", s.handleScanReconThermal)
   s.router.HandleFunc("PUT /api/v1/recon/hotspots/{id}/status", s.handleUpdateReconHotspotStatus)
   s.router.HandleFunc("GET /api/v1/recon/missions/{id}/export", s.handleExportReconMission)
   ```

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./internal/app/webfrontend/... -run TestRecon`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add internal/app/webfrontend/recon_handlers.go internal/app/webfrontend/server.go internal/app/webfrontend/recon_handlers_test.go
git commit -m "feat(webfrontend): add aerial reconnaissance REST endpoints and mesh broadcast"
```

---

### Task 5: Frontend Aerial Reconnaissance Cockpit UI & Leaflet Map Synchronization

**Files:**
- Create: `internal/app/webfrontend/templates/drone_modal.html`
- Create: `internal/app/webfrontend/static/js/drone-recon.js`
- Modify: `internal/app/webfrontend/templates/pets.html`
- Modify: `internal/app/webfrontend/templates/finder_landing.html`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/server.go` (ensure template parsing includes `drone_modal.html`)

**Interfaces:**
- Produces:
  - Accessible `#recon-modal` with dual tabs (`#tab-recon-cockpit`, `#tab-recon-batch`).
  - `#recon-reticle-canvas` drawing bounding boxes over thermal anomalies.
  - `#hud-attitude`, `#hud-heading`, `#hud-altitude`, `#hud-speed`, `#hud-battery`.
  - Leaflet map layers: cyan flight path polyline, dynamic camera frustum polygon, pulsating flame pins.
  - Event listener for `mesh:thermal-hotspot`.

- [ ] **Step 1: Write template integration test**

In `internal/app/webfrontend/server_test.go` (or `recon_assets_test.go`):
```go
func TestDroneReconAssets(t *testing.T) {
	server := NewTestServer(t)

	// Verify JS asset is served with correct headers
	req := httptest.NewRequest(http.MethodGet, "/static/js/drone-recon.js", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for drone-recon.js, got %d", w.Code)
	}

	// Verify modal template renders in /pets page
	req = httptest.NewRequest(http.MethodGet, "/pets", nil)
	w = httptest.NewRecorder()
	server.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "recon-modal") {
		t.Errorf("expected pets page to include recon-modal")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/... -run TestDroneReconAssets`
Expected: FAIL.

- [ ] **Step 3: Implement HTML template, controller JS, and CSS styles**

1. Create `internal/app/webfrontend/templates/drone_modal.html`:
   - `{{ define "drone_modal.html" }}`
   - Modal container `#recon-modal` with ARIA semantics, focus trap, and close button.
   - Dual-mode tabs: `#tab-recon-cockpit` and `#tab-recon-batch`.
   - Video container with `#recon-video` and `#recon-reticle-canvas`.
   - Telemetry HUD readouts: attitude artificial horizon, compass heading, altitude AGL, speed, battery.
   - Timeline slider `#recon-timeline-slider` and play/pause controls.
   - File dropzone `#recon-dropzone` for `.srt` and aerial images.
   - Hotspot drawer `#recon-hotspot-drawer` with confirmation buttons.
2. Include template in `pets.html` and `finder_landing.html`:
   - Add tactical launch button `#btn-open-drone-recon`.
   - Include `{{ template "drone_modal.html" . }}`.
3. Update `server.go` template parsing to load `templates/drone_modal.html`.
4. Create `internal/app/webfrontend/static/js/drone-recon.js`:
   - IIFE, strict mode, zero inline handlers, strict CSP compliance.
   - Video scrubber synchronization with SRT waypoint array.
   - Updates Leaflet map: draws cyan polyline, updates drone position marker, draws rotating camera frustum polygon.
   - Renders bounding boxes on canvas.
   - Handles `#btn-confirm-hotspot` and `#btn-dismiss-hotspot`.
   - Listens for `mesh:thermal-hotspot` to plot real-time aerial findings on map.
5. Add styles in `internal/app/webfrontend/static/css/styles.css`:
   - Glassmorphic cockpit styling with WCAG AAA contrast ratio $>7:1$.
   - Pulsating radar flame animation (`.marker-hotspot-pulse`).
   - Compass and artificial horizon gauges.

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./internal/app/webfrontend/... -run TestDroneReconAssets`
Expected: PASS.

- [ ] **Step 5: Commit changes**

```bash
git add internal/app/webfrontend/templates/drone_modal.html internal/app/webfrontend/static/js/drone-recon.js internal/app/webfrontend/static/css/styles.css internal/app/webfrontend/templates/pets.html internal/app/webfrontend/templates/finder_landing.html internal/app/webfrontend/server.go internal/app/webfrontend/recon_assets_test.go
git commit -m "feat(frontend): add drone reconnaissance cockpit, telemetry HUD, and Leaflet frustum visualizer"
```

---

### Task 6: Automated Playwright E2E User Journey & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/drone-aerial-recon-journey.spec.ts`

**Interfaces:**
- Consumes: All endpoints and UI components from Tasks 1–5.
- Produces: Automated end-to-end browser test verifying telemetry ingestion, HUD gauges, thermal anomaly detection, video-map synchronization, hotspot confirmation, and mesh propagation.

- [ ] **Step 1: Implement the Playwright E2E journey test**

In `tests/playwright/e2e/drone-aerial-recon-journey.spec.ts`:
- Create test verifying:
  1. Open `/pets` and click `#btn-open-drone-recon` to display `#recon-modal`.
  2. Ingest sample DJI SRT telemetry with 3 waypoints. Assert HUD displays Altitude AGL (e.g. `45.2 m`), Heading (e.g. `142.5°`), and Speed.
  3. Verify Leaflet map contains the `.leaflet-overlay-pane` with cyan flight trajectory polyline and camera frustum polygon.
  4. Ingest thermal image with hot pet anomaly. Assert reticle canvas draws bounding box and Leaflet map renders pulsating hotspot pin (`.marker-hotspot-pulse`).
  5. Scrub timeline slider `#recon-timeline-slider` forward; verify drone icon translates and camera footprint updates.
  6. Click `#btn-confirm-hotspot`; assert status badge updates to `CONFIRMED` and community sighting is registered.
  7. Verify `mesh:thermal-hotspot` DOM event is dispatched.

- [ ] **Step 2: Run single Playwright test**

Run: `cd tests/playwright && npx playwright test tests/playwright/e2e/drone-aerial-recon-journey.spec.ts`
Expected: PASS.

- [ ] **Step 3: Run full Playwright test suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 137/137 tests passing (100%).

- [ ] **Step 4: Run full repository verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: Clean pass with 0 lint errors, valid OpenTofu, clean yamllint, and 100% Go tests passing under `-race -cover`.

- [ ] **Step 5: Commit changes**

```bash
git add tests/playwright/e2e/drone-aerial-recon-journey.spec.ts
git commit -m "test(playwright): add E2E user journey tests for drone aerial reconnaissance and thermal hotspot detection"
```

---

### Task 7: Final Whole-Branch Code Review

**Files:**
- All touched files across Milestone 11.2

- [ ] **Step 1: Perform comprehensive review**
  - Run full whole-branch code review with `pro` model subagent.
  - Verify pure Go zero-CGO standard library compliance.
  - Verify WCAG AAA contrast ratio ($\ge 7:1$) and strict CSP compliance (0 inline scripts/eval).
  - Verify memory leak protection on video streams and canvas contexts.
- [ ] **Step 2: Apply any reviewer findings**
- [ ] **Step 3: Commit review fixes and verify**
  - `export GOTOOLCHAIN=go1.26.5 && make verify`
  - `cd tests/playwright && npx playwright test`
