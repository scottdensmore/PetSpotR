# Milestone 10.2: Topography & Friction-Aware Predictive Trajectory Modeling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a pure Go topography- and friction-aware predictive trajectory modeling engine that accounts for physical barriers (freeways, rivers), attraction corridors (parks, greenways), slope gradients, and species behavioral profiles (Canine vs Feline), extracting GeoJSON anisotropic isochrone contours, high-probability hiding clusters, and dynamically prioritizing Search Party sectors.

**Architecture:** 
- `pkg/domain`: Domain types for barrier classifications, terrain features, isochrone contour polygons, hiding clusters, predictive trajectory envelopes, and sector urgency levels.
- `pkg/sighting`: Pure Go spatial grid projection (Equirectangular to Cartesian meters), barrier/corridor rasterization, Tobler slope resistance, species behavioral profiling (momentum heading, feline territory decay), priority-queue Dijkstra cost-distance propagation, polygon contour tracing, and sector polygon intersection scoring.
- `internal/app/webfrontend`: Enhanced trajectory REST endpoints (`GET /api/v1/lost-pets/{petId}/trajectory`, `GET /api/v1/lost-pets/{petId}/predictive-trajectory`) and search party sector urgency annotation (`GET /api/v1/lost-pets/{petId}/search-party`).
- `internal/app/webfrontend/static/js`: Interactive Leaflet multi-tier isochrone layers, barrier polylines, hiding cluster markers, and Search Party sector priority sorting and badges.
- `tests/playwright/e2e`: Automated end-to-end journey tests covering sighting trajectories, isochrone visualization, and sector prioritization.

**Tech Stack:** Go 1.26 (pure standard library math/geometry, no CGO), Leaflet.js, OpenStreetMap raster tiles, vanilla JavaScript, Playwright TypeScript test runner.

**Spec:** [`docs/superpowers/specs/2026-09-21-predictive-trajectory-modeling-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-21-predictive-trajectory-modeling-design.md)

## Global Constraints
- Pure Go standard library for mathematical modeling and spatial geometry (zero CGO, zero external GIS or elevation service dependencies).
- Toolchain: `export GOTOOLCHAIN=go1.26.5`.
- Full backward compatibility with existing `GET /api/v1/lost-pets/{petId}/trajectory` response schemas.
- Strict CSP compliance: Zero inline `<script>` tags, zero `eval()`.
- WCAG AAA compliance and keyboard accessibility across all new UI elements and map layers.
- Zero lint issues (`make verify`) and 100% passing Playwright test suite.

---

### Task 1: Domain Models, Store Constants & Pure Go Predictive Trajectory Engine (`pkg/domain`, `pkg/sighting`)

**Files:**
- Create: `pkg/domain/trajectory.go`
- Modify: `pkg/domain/search_party.go`
- Create: `pkg/sighting/friction.go`
- Create: `pkg/sighting/friction_test.go`

**Interfaces:**
- Consumes: `domain.LocationPoint`, `domain.PetSightingRecord`, `domain.SearchSector`
- Produces: `domain.PredictiveTrajectoryResult`, `domain.IsochroneContour`, `domain.HidingCluster`, `domain.TerrainFeature`, `sighting.GeneratePredictiveTrajectory(species string, origin domain.LocationPoint, sightings []domain.PetSightingRecord, elapsedHours float64) domain.PredictiveTrajectoryResult`, `sighting.ScoreSectorUrgency(sector domain.SearchSector, isochrones []domain.IsochroneContour) (float64, domain.SectorUrgencyLevel)`

- [ ] **Step 1: Write failing tests in `pkg/sighting/friction_test.go`**
```go
package sighting

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestPredictiveTrajectory_SpeciesAndBarriers(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	now := time.Now().UTC()

	// 1. Dog profile with directional momentum
	dogSightings := []domain.PetSightingRecord{
		{
			SightingID:  "sight-1",
			SightedAt:   now.Add(-2 * time.Hour),
			Coordinates: &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		},
		{
			SightingID:  "sight-2",
			SightedAt:   now.Add(-1 * time.Hour),
			Coordinates: &domain.LocationPoint{Latitude: 47.6100, Longitude: -122.3321}, // Heading North
		},
	}

	resultDog := GeneratePredictiveTrajectory("dog", origin, dogSightings, 1.0)
	if len(resultDog.Isochrones) != 3 {
		t.Fatalf("expected 3 isochrone levels, got %d", len(resultDog.Isochrones))
	}
	if resultDog.HeadingDegrees < 350 && resultDog.HeadingDegrees > 10 {
		t.Errorf("expected heading roughly North (~0/360 deg), got %.1f", resultDog.HeadingDegrees)
	}

	// 2. Cat profile tightly bounded
	catSightings := []domain.PetSightingRecord{
		{
			SightingID:  "sight-cat-1",
			SightedAt:   now.Add(-30 * time.Minute),
			Coordinates: &domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
		},
	}
	resultCat := GeneratePredictiveTrajectory("cat", origin, catSightings, 0.5)
	if len(resultCat.Isochrones) != 3 {
		t.Fatalf("expected 3 isochrone levels for cat, got %d", len(resultCat.Isochrones))
	}
}

func TestScoreSectorUrgency(t *testing.T) {
	origin := domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	result := GeneratePredictiveTrajectory("dog", origin, nil, 1.0)

	// Sector overlapping center point
	sectorCore := domain.SearchSector{
		SectorID: "sector-core",
		BoundingPolygon: []domain.LocationPoint{
			{Latitude: 47.6050, Longitude: -122.3330},
			{Latitude: 47.6070, Longitude: -122.3330},
			{Latitude: 47.6070, Longitude: -122.3310},
			{Latitude: 47.6050, Longitude: -122.3310},
		},
	}

	score, urgency := ScoreSectorUrgency(sectorCore, result.Isochrones)
	if urgency != domain.SectorUrgencyCritical {
		t.Errorf("expected SectorUrgencyCritical for core sector, got %s (score %.2f)", urgency, score)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**
Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/sighting/... -run TestPredictiveTrajectory`
Expected: Compilation failure due to missing types and functions.

- [ ] **Step 3: Implement domain types in `pkg/domain/trajectory.go` and update `pkg/domain/search_party.go`**
Add `BarrierType`, `TerrainFeature`, `IsochroneContour`, `HidingCluster`, `PredictiveTrajectoryResult`, `SectorUrgencyLevel` constants and fields to `SearchSector`.

- [ ] **Step 4: Implement pure Go spatial engine in `pkg/sighting/friction.go`**
Implement Cartesian equirectangular conversion, discrete grid with default physical barriers (e.g. I-5 corridor, Lake Union/Elliot Bay water barriers for the Seattle metro baseline, or synthetic barriers based on bounding coordinates), Tobler slope resistance, species movement parameters, Dijkstra cost-distance propagation, contour boundary extraction into GeoJSON polygons, hiding cluster grouping, and `ScoreSectorUrgency`.

- [ ] **Step 5: Run tests to verify they pass**
Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/sighting/...`
Expected: PASS with >= 90% coverage.

- [ ] **Step 6: Commit**
```bash
git add pkg/domain/ pkg/sighting/
git commit -m "feat(sighting): add domain models and pure Go friction-aware trajectory engine"
```

---

### Task 2: Backend REST Endpoints & Search Sector Urgency Integration (`internal/app/webfrontend`)

**Files:**
- Modify: `internal/app/webfrontend/trajectory.go`
- Modify: `internal/app/webfrontend/searchparty.go`
- Modify: `internal/app/webfrontend/server.go`
- Modify: `internal/app/webfrontend/trajectory_test.go`
- Modify: `internal/app/webfrontend/searchparty_test.go`

**Interfaces:**
- Consumes: `sighting.GeneratePredictiveTrajectory`, `sighting.ScoreSectorUrgency`, `store.LostPetStore`, `store.SearchPartyStore`
- Produces: Extended `GET /api/v1/lost-pets/{petId}/trajectory`, new `GET /api/v1/lost-pets/{petId}/predictive-trajectory`, prioritized `GET /api/v1/lost-pets/{petId}/search-party`

- [ ] **Step 1: Write failing tests in `internal/app/webfrontend/trajectory_test.go`**
Test that `GET /api/v1/lost-pets/{petId}/trajectory` includes `predictiveModel` in response payload with isochrones and hiding clusters.
Test that `GET /api/v1/lost-pets/{petId}/predictive-trajectory?species=dog&elapsedHours=2.5` returns customized scenario modeling.
Test that `GET /api/v1/lost-pets/{petId}/search-party` returns sectors annotated with `urgencyLevel` and `priorityScore`.

- [ ] **Step 2: Run tests to verify they fail**
Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./internal/app/webfrontend/... -run Trajectory`
Expected: FAIL due to missing fields / handlers.

- [ ] **Step 3: Implement handler methods and routing**
In `trajectory.go`:
- Extend `handleGetTrajectory` to fetch pet species from `lostPetStore`, compute `sighting.GeneratePredictiveTrajectory`, and assign to `analysis.PredictiveModel`.
- Implement `handleGetPredictiveTrajectoryScenario` reading `elapsedHours` and `species` query parameters.
In `searchparty.go`:
- When retrieving search party sectors, fetch pet sightings, compute isochrones, and call `sighting.ScoreSectorUrgency` for each sector, setting `sector.PriorityScore` and `sector.UrgencyLevel`.
In `server.go`:
- Register `r.Get("/api/v1/lost-pets/{id}/predictive-trajectory", s.handleGetPredictiveTrajectoryScenario)`.

- [ ] **Step 4: Run tests to verify they pass**
Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/app/webfrontend/
git commit -m "feat(webfrontend): integrate predictive trajectory endpoints and search party sector urgency"
```

---

### Task 3: Leaflet Predictive Isochrone, Barrier & Cluster Visualizer (`sighting-trajectory.js`, `styles.css`)

**Files:**
- Modify: `internal/app/webfrontend/static/js/sighting-trajectory.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/templates/pets.html` (or trajectory container modal markup)

**Interfaces:**
- Consumes: `GET /api/v1/lost-pets/{petId}/trajectory` response with `predictiveModel`
- Produces: Leaflet polygon layers for 50%, 75%, 90% isochrones, barrier polylines, hiding cluster pins, layer toggle control pills.

- [ ] **Step 1: Add modal layer toggle controls in HTML markup**
Add segmented button pills in `#pet-trajectory-container`:
`<div class="trajectory-layer-toggle" role="radiogroup" aria-label="Map Layer Selection">` with buttons for `Standard Perimeter`, `Friction Isochrones`, and `Hiding Clusters`.

- [ ] **Step 2: Implement styling in `styles.css`**
Add glassmorphic classes:
- `.trajectory-layer-toggle`, `.layer-pill-btn`, `.layer-pill-active`
- `.isochrone-legend`, `.legend-chip`
- `.barrier-polyline-highway`, `.barrier-polyline-water`
- `.hiding-cluster-pin`
- High-contrast accessible badges conforming to WCAG AAA.

- [ ] **Step 3: Implement Leaflet layers in `sighting-trajectory.js`**
- Parse `data.predictiveModel`:
  - Render isochrone contours with `L.polygon` using respective probability styling (50% red, 75% amber, 90% blue).
  - Render barriers using `L.polyline`.
  - Render hiding clusters with custom `L.divIcon` shelter pins and detailed popups.
- Wire layer toggle click events to switch active Leaflet feature groups (`standardPerimeterGroup`, `frictionIsochroneGroup`, `hidingClusterGroup`).

- [ ] **Step 4: Run make verify to confirm CSS and JS formatting**
Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS.

- [ ] **Step 5: Commit**
```bash
git add internal/app/webfrontend/static/ internal/app/webfrontend/templates/
git commit -m "feat(ui): add Leaflet predictive isochrone contours, barrier lines, and hiding cluster pins"
```

---

### Task 4: Search Party Modal Sector Prioritization & Barrier Safety Alerts (`search-party.js`, `searchparty_modal.html`)

**Files:**
- Modify: `internal/app/webfrontend/static/js/search-party.js`
- Modify: `internal/app/webfrontend/templates/searchparty_modal.html`
- Modify: `internal/app/webfrontend/static/css/styles.css`

**Interfaces:**
- Consumes: `GET /api/v1/lost-pets/{petId}/search-party` response containing sectors with `priorityScore` and `urgencyLevel`.
- Produces: Dynamic sector card badges (`🔥 Critical Search Zone`, `⚡ High Priority`), "Sort by Urgency" toggle, and terrain barrier safety banner.

- [ ] **Step 1: Update `searchparty_modal.html`**
Add `#btn-sort-sectors-urgency` in the sector list toolbar and safety advisory banner container `#sector-barrier-advisory`.

- [ ] **Step 2: Update `search-party.js`**
- Render dynamic urgency badges on sector cards based on `sector.urgencyLevel`:
  - `CRITICAL`: `<span class="badge badge-critical" data-testid="sector-urgency-critical">🔥 Critical Search Zone</span>`
  - `HIGH`: `<span class="badge badge-high" data-testid="sector-urgency-high">⚡ High Priority</span>`
- Wire `#btn-sort-sectors-urgency` to sort sector cards descending by `priorityScore`.
- On sector selection, check for associated barriers or hiding clusters and display advisory in `#sector-barrier-advisory`.

- [ ] **Step 3: Verify and Commit**
Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
```bash
git add internal/app/webfrontend/static/js/search-party.js internal/app/webfrontend/templates/searchparty_modal.html internal/app/webfrontend/static/css/styles.css
git commit -m "feat(searchparty): add dynamic sector urgency badges, priority sorting, and barrier safety alerts"
```

---

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/predictive-trajectory-journey.spec.ts`

**Interfaces:**
- End-to-end multi-step user journey verifying the entire workflow:
  1. Register lost dog with initial coordinates.
  2. Post multiple sightings with directional heading.
  3. Inspect trajectory modal, verify 3-tier isochrone rings and layer toggling.
  4. Inspect hiding cluster markers.
  5. Inspect search party modal, verify prioritized sector badges and urgency sorting.
  6. Claim prioritized sector.

- [ ] **Step 1: Author `tests/playwright/e2e/predictive-trajectory-journey.spec.ts`**
Author comprehensive Playwright test verifying all steps, element presence, ARIA attributes, and API responses.

- [ ] **Step 2: Run new Playwright journey test**
Run: `cd tests/playwright && npx playwright test tests/playwright/e2e/predictive-trajectory-journey.spec.ts`
Expected: 1/1 passed.

- [ ] **Step 3: Run full Playwright test suite**
Run: `cd tests/playwright && npx playwright test`
Expected: 123/123 passed (100%).

- [ ] **Step 4: Run full repository verification**
Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: 0 lint issues, OpenTofu valid, yamllint clean, race tests pass.

- [ ] **Step 5: Commit**
```bash
git add tests/playwright/e2e/predictive-trajectory-journey.spec.ts
git commit -m "test(playwright): add E2E user journey tests for predictive trajectory modeling and sector prioritization"
```

---

### Task 6: Final Whole-Branch Code Review, PR Creation & Squash Merge

- [ ] **Step 1: Perform whole-branch code review**
Review complete git diff against spec requirements, concurrency safety, and performance constraints. Address any findings.

- [ ] **Step 2: Push branch and create Pull Request**
Create PR targeting `main`.

- [ ] **Step 3: Squash-merge PR and sync `main`**
Merge PR, delete feature branch, remove worktree, pull `main`, and re-run verification.
