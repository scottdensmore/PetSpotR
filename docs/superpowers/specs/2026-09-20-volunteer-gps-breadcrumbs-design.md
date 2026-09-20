# Milestone 9.1: Real-Time Volunteer GPS Breadcrumbs & Offline Sector Walk Recording Design Specification

- **Date:** 2026-09-20
- **Status:** Approved
- **Author:** Antigravity (Pair Programming Assistant)
- **Target Release:** PetSpotR Milestone 9.1

---

## 1. Executive Summary & Problem Statement

In Milestone 8.1, community search parties were organized into geo-fenced sectors. However, coordinators and pet owners cannot currently see where volunteers have actually physically walked within their claimed sectors. In dense neighborhoods, greenbelts, or parks, areas can be erroneously assumed searched when a volunteer only walked the main perimeter. Additionally, volunteers frequently lose mobile connectivity while searching wooded ravines or trails.

**Milestone 9.1: Real-Time Volunteer GPS Breadcrumbs & Offline Sector Walk Recording** addresses this:
1. **Volunteer Route Recording:** Volunteers actively searching a sector can toggle "Record Search Trail". High-accuracy GPS breadcrumbs (`lat`, `lng`, `timestamp`, `accuracyMeters`) are recorded at periodic intervals.
2. **Offline Buffer & Auto-Sync:** GPS breadcrumb batches are persisted to browser IndexedDB if network connectivity drops, syncing automatically when connectivity resumes.
3. **Live Polyline Overlay & Spatial Coverage:** Walked routes are rendered as animated polylines over the sector polygon on Leaflet maps, updating coordinators and pet owners via real-time SSE broadcasts.

---

## 2. Architecture & Subsystem Boundaries

```text
               ┌───────────────────────────────────────────────────────────┐
               │                  Volunteer Mobile Client                  │
               │  - HTML5 Geolocation watchPosition()                      │
               │  - IndexedDB Breadcrumb Outbox Buffer                     │
               │  - Leaflet Map Polyline Trail Renderer                    │
               └─────────────────────────────┬─────────────────────────────┘
                                             │
                                     HTTP / REST / SSE
                                             │
               ┌─────────────────────────────▼─────────────────────────────┐
               │               internal/app/webfrontend                    │
               │  - POST /api/v1/search-parties/{petID}/sectors/{id}/breadcrumbs │
               │  - GET  /api/v1/search-parties/{petID}/sectors/{id}/breadcrumbs │
               │  - Broadcast SSE event: breadcrumb_updated                │
               └──────────────┬─────────────────────────────┬──────────────┘
                              │                             │
               ┌──────────────▼────────────┐  ┌─────────────▼──────────────┐
               │      pkg/searchparty      │  │        pkg/store           │
               │  - Breadcrumb Domain Model│  │  - store.Breadcrumbs       │
               │  - Spatial Trail Distance │  │    Collection              │
               │  - Sector Boundary Bounds │  │                            │
               └───────────────────────────┘  └────────────────────────────┘
```

---

## 3. Data Models & API Contracts

### 3.1 Domain Models (`pkg/searchparty`)
```go
type BreadcrumbPoint struct {
    Latitude       float64   `json:"latitude"`
    Longitude      float64   `json:"longitude"`
    Timestamp      time.Time `json:"timestamp"`
    AccuracyMeters float64   `json:"accuracyMeters"`
}

type VolunteerBreadcrumbTrail struct {
    TrailID            string            `json:"trailId"`
    SearchPartyID      string            `json:"searchPartyId"`
    SectorID           string            `json:"sectorId"`
    VolunteerAlias     string            `json:"volunteerAlias"`
    Points             []BreadcrumbPoint `json:"points"`
    TotalDistanceM     float64           `json:"totalDistanceM"`
    DurationSeconds    int               `json:"durationSeconds"`
    CreatedAt          time.Time         `json:"createdAt"`
    UpdatedAt          time.Time         `json:"updatedAt"`
}
```

### 3.2 REST API Endpoints
- `POST /api/v1/search-parties/{petID}/sectors/{sectorID}/breadcrumbs`:
  - Request Body: `{"points": [{"latitude": 47.61, "longitude": -122.33, "timestamp": "...", "accuracyMeters": 5.2}], "volunteerAlias": "Trail Scout #42"}`
  - Validates points, updates cumulative distance and path bounds, persists to store, and emits SSE event `breadcrumb_updated`.
  - Response: `201 Created` with updated `VolunteerBreadcrumbTrail`.
- `GET /api/v1/search-parties/{petID}/sectors/{sectorID}/breadcrumbs`:
  - Returns `200 OK` with array of active trails for the sector.

---

## 4. UI & Offline Interactions
- "Record Trail" toggle in `#sector-detail-modal`.
- Leaflet polyline overlay with pulsing pin at volunteer's latest coordinate.
- IndexedDB offline queueing with auto-flush on online event.
