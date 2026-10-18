# Canine (K9) Scent-Trailing Telemetry, Wind Plume Dispersion & Handler Cockpit — Implementation Plan

- **Milestone:** 12.4
- **Spec:** `docs/superpowers/specs/2026-10-17-k9-scent-trailing-wind-plume-design.md`
- **Toolchain:** `export GOTOOLCHAIN=go1.26.5`
- **Worktree:** `.worktrees/feat-k9-scent-trailing` (`feat/k9-scent-trailing`)

---

## Task Breakdown (Subagent-Driven TDD)

### Task 1: Domain Models, State Store Collection & Validation (`pkg/domain/k9_scent.go`, `pkg/store/store.go`)
- **Files:**
  - Create `pkg/domain/k9_scent.go`
  - Create `pkg/domain/k9_scent_test.go`
  - Modify `pkg/store/store.go`
- **Details:**
  - Define `K9Discipline`, `GroundSurfaceType`, `ScentViabilityRating`, `K9BehaviorAlert`, `K9EnvironmentConditions`, `ScentPlumePolygon`, `K9TrackWaypoint`, and `K9ScentMission` with validation methods (`Validate()`).
  - Add `CollectionK9ScentMissions = "k9_scent_missions"` in `pkg/store/store.go`.
  - Write comprehensive unit tests in `pkg/domain/k9_scent_test.go`.

### Task 2: Pure-Go Atmospheric Scent Plume Modeler, Viability Engine & GPX Exporter (`pkg/k9`)
- **Files:**
  - Create `pkg/k9/plume.go`
  - Create `pkg/k9/telemetry.go`
  - Create `pkg/k9/k9_test.go`
- **Details:**
  - Implement `EvaluateScentViability`, `ComputeGaussianScentPlume`, `IsWaypointInScentPlume`, `RecalculateMissionMetrics`, and `ExportK9MissionGPX`.
  - Write unit tests in `pkg/k9/k9_test.go` covering atmospheric conditions, thermal paw hazards, downwind plume azimuths/vertices, waypoint corridor containment, POD calculation, and GPX 1.1 XML serialization.

### Task 3: WebFrontend REST API, Sighting Promotion, GPX Export & SSE/Mesh Stream (`internal/app/webfrontend/k9_handlers.go`)
- **Files:**
  - Create `internal/app/webfrontend/k9_handlers.go`
  - Create `internal/app/webfrontend/k9_handlers_test.go`
  - Modify `internal/app/webfrontend/server.go`
  - Create stub `internal/app/webfrontend/templates/k9_ops.html`
- **Details:**
  - Register routes in `server.go`:
    - `GET /k9-ops` -> `s.handleK9OpsPage`
    - `GET /api/v1/k9/missions` -> `s.handleListK9Missions`
    - `POST /api/v1/k9/missions` -> `s.handleCreateK9Mission`
    - `POST /api/v1/k9/missions/{id}/waypoints` -> `s.handleAddK9Waypoint`
    - `GET /api/v1/k9/missions/{id}/gpx` -> `s.handleExportK9MissionGPX`
    - `GET /api/v1/k9/stream` -> `s.handleK9Stream`

### Task 4: K9 Scent-Trailing & Wind Plume Cockpit UI (`/k9-ops`), SVG Plume Visualizer & WCAG AAA CSS
- **Files:**
  - Update `internal/app/webfrontend/templates/k9_ops.html`
  - Create `internal/app/webfrontend/static/js/k9-ops.js`
  - Modify `internal/app/webfrontend/static/css/styles.css`
  - Create `tests/playwright/unit/k9-ops.spec.ts`
- **Details:**
  - Build `/k9-ops` featuring:
    - **Section 1: Active K9 Scent Missions & Viability Monitor (`#k9-missions-list`, `#form-create-k9-mission`)**
    - **Section 2: Interactive SVG Wind Plume & K9 Track Radar (`#k9-plume-svg`, `#plume-azimuth-badge`, `#mission-pod-badge`, `#paw-hazard-banner`)**
    - **Section 3: K9 Waypoint & Behavioral Alert Logger (`#form-log-k9-waypoint`, `#k9-waypoints-list`, `#btn-sim-in-plume-alert`, `#btn-export-k9-gpx`)**
  - Enforce WCAG AAA (`> 7:1`) contrast for viability and behavioral alert badges (`.badge-viability-high`, `.badge-viability-moderate`, `.badge-viability-degraded`, `.badge-viability-expired`, `.badge-k9-behavior`) across dark and light modes.

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification
- **Files:**
  - Create `tests/playwright/e2e/k9-ops-journey.spec.ts`
- **Details:**
  - 6-step E2E journey verifying seeded K9 missions, creating a new K9 mantrailing mission with environmental telemetry, inspecting the SVG Gaussian downwind scent cone, logging in-plume `HEAD_SNAP_ALERT` and `STRONG_SCENT_PULL` waypoints with sighting promotion, verifying `EstimatedPOD` progression and paw-pad thermal hazard alert, and exporting GPX 1.1 XML telemetry.

### Task 6: Final Whole-Branch Code Review & Verification Gate
- **Details:**
  - Run whole-branch review, verify `export GOTOOLCHAIN=go1.26.5 && make verify` and 100% of Playwright tests pass, push, open PR (#285), and squash-merge into `main`.
