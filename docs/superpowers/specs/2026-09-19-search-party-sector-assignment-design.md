# Milestone 8.1: Community Search Party Dispatch & Volunteer Geo-Fenced Sector Assignment Design Specification

- **Date:** 2026-09-19
- **Status:** Approved
- **Author:** Antigravity (Pair Programming Assistant)
- **Target Release:** PetSpotR Milestone 8.1

---

## 1. Executive Summary & Problem Statement

When a pet goes missing, rallying a community search party is often the most effective way to canvas a neighborhood. However, disorganized volunteer efforts frequently result in duplicated searches, overlooked areas, and poor coordination. Without a systematic method to divide and conquer the search perimeter, volunteers lose precious time and motivation.

**Milestone 8.1: Community Search Party Dispatch & Volunteer Geo-Fenced Sector Assignment** introduces a structured dispatch engine. Building upon Milestone 7's predictive trajectory heatmaps, this system decomposes the estimated search perimeter into distinct, geo-fenced sectors (wedges or quadrants). Volunteers can digitally "claim" sectors, update their status (`active_search`, `cleared`, `sighting_reported`), and contribute to a real-time coverage map. This ensures coordinated, systematic grid searches while preserving volunteer privacy and feeding directly into the Reunion Room's live updates.

---

## 2. Architecture & Subsystem Boundaries

The system is decomposed into distinct layers for spatial computation, persistence, API transport, and real-time visualization:

```text
                  ┌─────────────────────────────────────────────────────────┐
                  │                 Browser Clients / Mobile                │
                  │ (Search Sector Map, Quick-Claim Modal, Sector Briefing) │
                  └────────────────────────────┬────────────────────────────┘
                                               │
                                       HTTP / SSE / REST
                                               │
                  ┌────────────────────────────▼────────────────────────────┐
                  │           internal/app/webfrontend                      │
                  │  - POST /api/v1/lost-pets/{petID}/search-party          │
                  │  - GET  /api/v1/lost-pets/{petID}/search-party          │
                  │  - POST /api/v1/search-parties/{partyID}/sectors/...    │
                  │  - Real-time Reunion Hub SSE                            │
                  └──────────────┬───────────────────────────┬──────────────┘
                                 │                           │
                   In-memory / Firestore       Spatial Sector Decomposition
                                 │                           │
                  ┌──────────────▼────────────┐┌─────────────▼──────────────┐
                  │ store.SearchParties       ││       pkg/searchparty      │
                  │ store.SectorAssignments   ││  (Pure Go Sector Engine)   │
                  └───────────────────────────┘└────────────────────────────┘
```

---

## 3. Domain Models & Spatial Sector Decomposition (`pkg/searchparty`)

### 3.1 Domain Data Models

#### Search Party & Sector Models
```go
package searchparty

import (
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// SectorStatus represents the current state of a geo-fenced search sector.
type SectorStatus string

const (
	SectorStatusUnassigned       SectorStatus = "unassigned"
	SectorStatusActiveSearch     SectorStatus = "active_search"
	SectorStatusCleared          SectorStatus = "cleared"
	SectorStatusSightingReported SectorStatus = "sighting_reported"
)

// SearchSector defines a specific spatial polygon assigned to volunteers.
type SearchSector struct {
	SectorID       string                 `json:"sectorId"`
	Name           string                 `json:"name"` // e.g., "Sector Alpha (NW)"
	PolygonPoints  []domain.LocationPoint `json:"polygonPoints"` // Ordered vertices
	Status         SectorStatus           `json:"status"`
	PriorityScore  float64                `json:"priorityScore"` // Based on trajectory bearing
	TotalAreaSqM   float64                `json:"totalAreaSqM"`
}

// SectorAssignment tracks a volunteer's claim on a sector.
type SectorAssignment struct {
	AssignmentID   string       `json:"assignmentId"`
	SectorID       string       `json:"sectorId"`
	VolunteerAlias string       `json:"volunteerAlias"` // Pseudonym, e.g., "Volunteer #3"
	ClaimedAt      time.Time    `json:"claimedAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
	Status         SectorStatus `json:"status"`
	ClearanceNotes string       `json:"clearanceNotes,omitempty"`
}

// SearchParty encapsulates the entire search operation for a lost pet.
type SearchParty struct {
	PartyID               string             `json:"partyId"`
	LostPetID             string             `json:"lostPetId"`
	CenterCoordinates     domain.LocationPoint `json:"centerCoordinates"`
	RadiusMeters          float64            `json:"radiusMeters"`
	CreatedAt             time.Time          `json:"createdAt"`
	Sectors               []SearchSector     `json:"sectors"`
	ActiveAssignments     []SectorAssignment `json:"activeAssignments"`
	CoveragePercentage    float64            `json:"coveragePercentage"`
	ActiveVolunteersCount int                `json:"activeVolunteersCount"`
}
```

### 3.2 Sector Decomposition & Coverage Algorithm
1. **Spatial Decomposition**: Given a center point (from `sighting.EstimatedPerimeter`), radius, and sector count (e.g., 4 or 8), the engine calculates polygon coordinates for each wedge. Vertices are generated using Great-Circle distance and bearing calculations.
2. **Priority Scoring**: Sectors intersecting the last known directional bearing of the pet are assigned higher `PriorityScore`s.
3. **Coverage Calculation**: The percentage of total perimeter area cleared is dynamically calculated: `(Sum of area for Cleared Sectors) / (Total Area) * 100`.

---

## 4. Web Frontend Endpoints & Real-Time Sync

### 4.1 HTTP REST Endpoints (`internal/app/webfrontend`)

#### 1. `POST /api/v1/lost-pets/{petID}/search-party`
- Initializes a new `SearchParty` based on the latest trajectory perimeter.
- Decomposes the perimeter into `SearchSector` polygons.
- Stored in `store.SearchPartiesCollection = "searchParties"`.

#### 2. `GET /api/v1/lost-pets/{petID}/search-party`
- Retrieves the active search party, its sectors, assigned volunteers (using aliases), and real-time coverage metrics.

#### 3. `POST /api/v1/search-parties/{partyID}/sectors/{sectorID}/claim`
- A volunteer claims a sector. Creates a `SectorAssignment` in `store.SectorAssignmentsCollection = "sectorAssignments"`.
- Generates an anonymous identifier (e.g., "Volunteer #4") to protect PII.
- Updates sector status to `active_search`.

#### 4. `POST /api/v1/search-parties/{partyID}/sectors/{sectorID}/status`
- Updates the assignment and sector status (e.g., transitioning from `active_search` to `cleared` or `sighting_reported`).
- Accepts `clearanceNotes` for context.

### 4.2 Real-Time Event Envelopes & Notifications
- **SSE Broadcast in Reunion Room**:
  Emits `event: search_party_updated` through `ReunionHub.Broadcast` when a sector status changes.
  ```json
  {
    "type": "search_party_updated",
    "partyId": "party-123",
    "sectorId": "sector-456",
    "status": "cleared",
    "coveragePercentage": 25.5,
    "activeVolunteersCount": 4
  }
  ```

---

## 5. Frontend UI & Interactive Visualization

### 5.1 Leaflet Sector Overlay (`#search-party-map`)
- **Polygon Rendering**: Uses `L.polygon` to overlay sectors onto the map.
- **Color-Coding**:
  - **Amber (Opacity 0.4)**: `unassigned`
  - **Blue/Indigo (Opacity 0.5)**: `active_search`
  - **Green (Opacity 0.4)**: `cleared`
  - **Red Border Pulse**: `sighting_reported` within the sector.
- **Interactivity**: Clicking a polygon opens the Sector Details popup.

### 5.2 Volunteer Interaction
- **Quick-Claim Modal**: Triggered from the map or a list view, allowing a user to accept a sector assignment.
- **Clearance Notes Form**: A form to mark a sector as cleared, providing details (e.g., "Checked alleyways and under cars; no sign of pet.").
- **Printable Briefing**: A generated summary view showing sector boundaries and instructions for physical printout.

---

## 6. Zero-PII Wire Contract & Privacy

1. **Volunteer Anonymity**:
   The public sector view and all API responses related to search parties expose only a pseudonym or anonymous identifier for volunteers (e.g., "Volunteer #3").
2. **Private Contact Isolation**:
   Email addresses, phone numbers, and actual user IDs of volunteers are strictly isolated and never transmitted to the frontend in `GET` requests for search parties or assignments.
3. **Access Control**:
   Only authorized system administrators or the original pet owner (in a secure view) may potentially correlate pseudonyms with actual user accounts if required for moderation.

---

## 7. Verification & Testing Strategy

1. **Unit Tests (`pkg/searchparty/sector_test.go`)**:
   - Pure Go tests verifying polygon geometry generation (vertex coordinates, closure).
   - Validation of area calculations and priority scoring based on bearings.
   - Coverage percentage algorithm correctness.
2. **Web Frontend Integration Tests (`internal/app/webfrontend/searchparty_test.go`)**:
   - Verification of HTTP routes, 404/400 handling, rate limiting, and request validation.
   - Validation of the Zero-PII contract in API responses.
   - Emitting of SSE payloads via `ReunionHub.Broadcast`.
3. **Playwright E2E User Journey (`tests/playwright/e2e/search-party-journey.spec.ts`)**:
   - End-to-end journey: initializing a search party, a volunteer viewing the map, claiming a sector (amber to blue), and updating the status to cleared (blue to green) with notes.
   - Verification of the real-time coverage progress bar updating in the UI.
