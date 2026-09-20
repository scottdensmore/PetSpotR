# Milestone 7: Community Sighting Timeline & Predictive Trajectory Heatmaps Design Specification

- **Date:** 2026-09-19
- **Status:** Approved
- **Author:** Antigravity (Pair Programming Assistant)
- **Target Release:** PetSpotR Milestone 7

---

## 1. Executive Summary & Problem Statement

When a pet goes missing, search squads, neighborhood volunteers, and passersby often spot the animal roaming through streets, parks, or alleyways. Currently, PetSpotR only supports filing a formal Lost Pet or Found Pet report. Filing a full Found Pet report requires custody details, contact information, and intake attributes that casual witnesses cannot or will not provide if they only caught a brief glimpse of the fleeing pet.

**Milestone 7: Community Sighting Timeline & Predictive Trajectory Heatmaps** introduces lightweight, real-time community sighting reporting. Witnesses can submit a sighting in seconds with one tap ("📍 Use My Location"), an optional quick photo, and observed direction of travel. PetSpotR aggregates these chronological sightings into a **Spatial Trajectory Engine**, calculating movement velocity (mph), directional vectors (compass bearing), and dynamic search boundary perimeters. The results are visualized directly on interactive Leaflet maps with numbered chronological pins, movement vectors, and real-time alerts in the Reunion Room and Notification Center.

---

## 2. Architecture & Subsystem Boundaries

The system is decomposed into distinct, cohesive layers:

```
                  ┌─────────────────────────────────────────────────────────┐
                  │                 Browser Clients / Mobile                │
                  │  (Quick Sighting Modal, Trajectory Map, Reunion Room)   │
                  └────────────────────────────┬────────────────────────────┘
                                               │
                                       HTTP / SSE / REST
                                               │
                  ┌────────────────────────────▼────────────────────────────┐
                  │           internal/app/webfrontend                      │
                  │  - POST /api/v1/lost-pets/{id}/sightings (Rate-limited) │
                  │  - GET  /api/v1/lost-pets/{id}/sightings                │
                  │  - GET  /api/v1/lost-pets/{id}/trajectory               │
                  │  - Real-time Reunion Hub SSE & Web Push Notification    │
                  └──────────────┬───────────────────────────┬──────────────┘
                                 │                           │
                   In-memory / Firestore       Trajectory Calculation
                                 │                           │
                  ┌──────────────▼────────────┐┌─────────────▼──────────────┐
                  │ store.SightingsCollection ││       pkg/sighting         │
                  │ (StateStore persistence)  ││ (Pure Go Trajectory Engine)│
                  └───────────────────────────┘└─────────────────────────────┘
```

---

## 3. Domain Models & Trajectory Calculation Engine

### 3.1 Domain Data Models (`pkg/domain`, `pkg/sighting`)

#### Sighting Record (`domain.PetSightingRecord`)
```go
package domain

import "time"

// SightingStatus represents the moderation or lifecycle state of a sighting.
type SightingStatus string

const (
	SightingStatusActive    SightingStatus = "active"
	SightingStatusFlagged   SightingStatus = "flagged"
	SightingStatusDismissed SightingStatus = "dismissed"
)

// PetSightingRecord captures a community witness report of a lost pet.
type PetSightingRecord struct {
	SightingID          string         `json:"sightingId"`
	LostPetID           string         `json:"lostPetId"`
	ReportedAt          time.Time      `json:"reportedAt"`
	SightedAt           time.Time      `json:"sightedAt"`
	LocationDescription string         `json:"locationDescription"`
	Coordinates         *LocationPoint `json:"coordinates"`
	MovementDirection   string         `json:"movementDirection,omitempty"` // e.g. "North", "Stationary"
	ImageURL            string         `json:"imageUrl,omitempty"`
	ImageObject         string         `json:"imageObject,omitempty"`
	Notes               string         `json:"notes,omitempty"`
	Status              SightingStatus `json:"status"`
}
```

#### Trajectory Analysis (`pkg/sighting`)
```go
package sighting

import (
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// TrajectoryLeg represents the movement vector between two consecutive sightings.
type TrajectoryLeg struct {
	FromSightingID   string  `json:"fromSightingId"`
	ToSightingID     string  `json:"toSightingId"`
	DistanceMeters   float64 `json:"distanceMeters"`
	DistanceMiles    float64 `json:"distanceMiles"`
	ElapsedDuration  float64 `json:"elapsedSeconds"`
	SpeedMph         float64 `json:"speedMph"`
	BearingDegrees   float64 `json:"bearingDegrees"`   // 0 - 360 degrees
	CardinalHeading  string  `json:"cardinalHeading"`  // e.g. "NE", "SSE"
}

// SearchPerimeter defines the predictive radial boundary centered on the latest sighting.
type SearchPerimeter struct {
	CenterCoordinates domain.LocationPoint `json:"centerCoordinates"`
	RadiusMeters      float64              `json:"radiusMeters"`
	RadiusMiles       float64              `json:"radiusMiles"`
	ConfidenceLevel   string               `json:"confidenceLevel"` // "High", "Expanding", "Wide"
}

// TrajectoryAnalysis encapsulates the chronological path and motion metrics of a lost pet.
type TrajectoryAnalysis struct {
	LostPetID        string               `json:"lostPetId"`
	GeneratedAt      time.Time            `json:"generatedAt"`
	OriginLocation   *domain.LocationPoint `json:"originLocation,omitempty"` // Initial last seen spot from LostPetRecord
	SightingsCount   int                  `json:"sightingsCount"`
	OrderedSightings []domain.PetSightingRecord `json:"orderedSightings"`
	Legs             []TrajectoryLeg      `json:"legs"`
	TotalDistanceMiles float64            `json:"totalDistanceMiles"`
	EstimatedPerimeter *SearchPerimeter   `json:"estimatedPerimeter,omitempty"`
}
```

### 3.2 Mathematical Trajectory Formulas
1. **Haversine Distance**:
   Calculates great-circle surface distance between two latitude/longitude points:
   $$d = 2R \arcsin \left(\sqrt{\sin^2\left(\frac{\Delta \phi}{2}\right) + \cos(\phi_1)\cos(\phi_2)\sin^2\left(\frac{\Delta \lambda}{2}\right)}\right)$$
   Where $R \approx 6,371,000 \text{ m}$ (mean Earth radius).
2. **Elapsed Time & Ground Speed**:
   $$\Delta t = t_2 - t_1$$
   $$v = \frac{d}{\Delta t} \times \text{conversion factor to mph}$$
   If $\Delta t \le 0$, speed is defined as $0.0$.
3. **Initial Compass Bearing**:
   $$\theta = \text{atan2}\left(\sin(\Delta \lambda)\cos(\phi_2), \cos(\phi_1)\sin(\phi_2) - \sin(\phi_1)\cos(\phi_2)\cos(\Delta \lambda)\right)$$
   Normalized to $[0^\circ, 360^\circ)$ and mapped to 16 cardinal points (N, NNE, NE, ENE, E, etc.).
4. **Predictive Search Perimeter Calculation**:
   - Centered on the most recent chronological sighting point.
   - Initial radius: $0.5 \text{ miles}$ for sightings within the last 2 hours.
   - Expanding radius: $+0.25 \text{ miles}$ per additional 2 hours elapsed, capped at $3.0 \text{ miles}$.

---

## 4. Web Frontend Endpoints & Real-Time Sync

### 4.1 HTTP REST Endpoints (`internal/app/webfrontend`)

#### 1. `POST /api/v1/lost-pets/{petID}/sightings`
- **Authentication / Rate Limit**: Protected by `ratelimit.ModerateLimit` (`s.rateLimiter`).
- **Validation**:
  - `petID` must exist in `store.LostPetsCollection`.
  - `coordinates` must be valid (`latitude` in $[-90, 90]$, `longitude` in $[-180, 180]$).
  - `sightedAt` must be non-zero and not in the future.
- **Payload**:
  ```json
  {
    "sightedAt": "2026-09-19T18:15:00Z",
    "locationDescription": "4th and Olive Way near the transit station",
    "coordinates": {
      "latitude": 47.613,
      "longitude": -122.337
    },
    "movementDirection": "Northeast",
    "notes": "Wearing red harness, seemed spooked by sirens",
    "imageObject": "sightings/sight-123.jpg",
    "reporterContact": {
      "name": "Jane Witness",
      "phone": "206-555-0199",
      "email": "witness@example.com"
    }
  }
  ```
- **Response**: `201 Created` with created `PetSightingRecord` (sensitive contact info stripped).

#### 2. `GET /api/v1/lost-pets/{petID}/sightings`
- Returns array of active `PetSightingRecord` objects sorted chronologically by `sightedAt`.

#### 3. `GET /api/v1/lost-pets/{petID}/trajectory`
- Returns computed `TrajectoryAnalysis` JSON payload.

### 4.2 Real-Time Event Envelopes & Notifications
- **SSE Broadcast in Reunion Room**:
  Emits `event: sighting` through `s.reunionHub`:
  ```json
  {
    "type": "sighting",
    "petId": "lost-789",
    "sightingId": "sight-101",
    "sightedAt": "2026-09-19T18:15:00Z",
    "locationDescription": "4th & Olive Way",
    "coordinates": {"latitude": 47.613, "longitude": -122.337},
    "movementDirection": "Northeast"
  }
  ```
- **Web Push Notifications**:
  Subscribers whose geographic alert zones overlap the sighting coordinates receive an instant notification:
  - Title: `Pet Sighting: Rusty spotted near Olive Way!`
  - Body: `A community member spotted Rusty 0.3 miles from your alert zone heading Northeast.`
  - URL: `/pets?focus=lost-789#trajectory`

---

## 5. Frontend UI & Interactive Visualization

### 5.1 Quick Sighting Modal (`#modal-report-sighting`)
- **Trigger**: "👁️ Report Sighting" button on:
  - Pet cards in `/pets` directory
  - Printable recovery posters `/pets/{id}/poster`
  - Quick-action landing view `/p/{id}`
- **Form Controls**:
  - `#sighting-geolocation-btn`: "📍 Use My Location" (`navigator.geolocation`).
  - `#sighting-time-mode`: "Just now" vs "Earlier today".
  - `#sighting-direction`: Compass dropdown (N, NE, E, SE, S, SW, W, NW, Stationary).
  - `#sighting-photo-input`: Dropzone / camera snapshot.
  - `#sighting-notes`: Description of demeanor, direction, and landmarks.
  - Private finder contact inputs for bilateral owner follow-up.

### 5.2 Leaflet Trajectory Map (`#pet-trajectory-map`)
- **Numbered Milestone Markers**:
  - `Pin 0`: Initial Lost Point (Red circular marker with paw icon).
  - `Pin 1..N`: Chronological sightings (Indigo circular markers with bold sequence numbers `1`, `2`, `3`).
- **Directional Trajectory Polyline**:
  - Polylines connecting `Pin 0` -> `Pin 1` -> ... -> `Pin N` with directional dashed stroke or arrow indicators.
- **Dynamic Search Perimeter Overlay**:
  - `L.circle` centered on the latest pin with radius computed by the trajectory engine.
- **Milestone Popups**:
  - Sighting timestamp, photo thumbnail, elapsed duration, speed between legs, and witness notes.

---

## 6. Zero-PII Wire Contract & Security

1. **Private Contact Isolation**:
   Witness contact information is separated and stored in private contact collections or encrypted metadata. It is never exposed in public `GET /api/v1/lost-pets/{petID}/sightings` or `GET /api/v1/lost-pets/{petID}/trajectory` endpoints.
2. **Rate Limiting**:
   `POST /api/v1/lost-pets/{petID}/sightings` is strictly bounded by `s.rateLimiter` (`ratelimit.ModerateLimit`) to mitigate spam and DoS attempts.
3. **Content Security Policy (CSP)**:
   Strict adherence to `script-src 'self'`. All map and modal logic resides in native static JavaScript files (`static/js/sighting-trajectory.js`), with zero inline scripts or unverified third-party scripts.

---

## 7. Verification & Testing Strategy

1. **Unit Tests (`pkg/sighting`)**:
   - Mathematical accuracy of Haversine distance, elapsed durations, ground speeds, and compass bearings.
   - Coordinate validation and out-of-bounds guards.
   - Single-point, zero-point, stationary, and multi-point trajectory scenarios.
2. **Web Frontend Integration Tests (`internal/app/webfrontend/sighting_test.go`)**:
   - HTTP routes verification, rate limiting, request validation, 400 Bad Request, and 404 Not Found.
   - Real-time SSE event dispatching via `ReunionHub`.
3. **Playwright E2E User Journey (`tests/playwright/e2e/sighting-trajectory-journey.spec.ts`)**:
   - End-to-end journey: opening modal, reporting sighting with coordinates and photo, inspecting interactive trajectory map with numbered pins, and verifying real-time feed update in the Reunion Room.
4. **Full Verification**:
   - `export GOTOOLCHAIN=go1.26.5 && make verify` clean across all local checks.
