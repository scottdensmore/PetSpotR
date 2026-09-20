# Milestone 9.1: Real-Time Volunteer GPS Breadcrumbs & Offline Sector Walk Recording Implementation Plan

- **Target Branch:** `feat/volunteer-gps-breadcrumbs`
- **Worktree:** `.worktrees/track-b`

---

## Proposed Tasks

### Task 1: Domain Models, Breadcrumb Calculations & Store Constants (`pkg/searchparty`, `pkg/store`)
- Extend `pkg/searchparty/sector.go` with `BreadcrumbPoint` and `VolunteerBreadcrumbTrail`.
- Implement `CalculateTrailDistance` and `PointInSectorPolygon` helper in `pkg/searchparty/trail.go` with unit tests.
- Add `store.BreadcrumbsCollection = "volunteerBreadcrumbs"` to `pkg/store/names.go`.

### Task 2: Backend REST Handlers & SSE Breadcrumb Broadcast (`internal/app/webfrontend`)
- Implement `POST /api/v1/search-parties/{petID}/sectors/{sectorID}/breadcrumbs` and `GET /api/v1/search-parties/{petID}/sectors/{sectorID}/breadcrumbs`.
- Broadcast real-time `event: breadcrumb_updated` to connected clients via `ReunionHub`.
- Write unit tests in `internal/app/webfrontend/breadcrumbs_test.go`.

### Task 3: Client GPS Recording, IndexedDB Offline Buffer & Leaflet Polyline Overlay (`webfrontend`)
- Update `static/js/search-party.js` to support GPS route recording using `navigator.geolocation.watchPosition()`.
- Add IndexedDB store `search_party_breadcrumbs` to buffer coordinates while offline and flush when online.
- Draw real-time Leaflet polylines (`L.polyline`) with color differentiation per volunteer.

### Task 4: Sector Modal UI Controls & Trail Stats Display
- Add "Record Search Path" button, distance walked display, and active trail counter to search party UI and modals.
- Add responsive styling in `styles.css` for trail badges and active recording indicator.

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification
- Create `tests/playwright/e2e/volunteer-breadcrumbs-journey.spec.ts`.
- Test recording coordinates, offline simulation/sync, and live polyline rendering.
- Run `make verify` ensuring all linters and tests pass cleanly.
