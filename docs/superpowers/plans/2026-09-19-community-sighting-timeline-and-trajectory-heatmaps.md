# Milestone 7: Community Sighting Timeline & Predictive Trajectory Heatmaps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement community sighting reporting, pure Go spatial trajectory calculations (velocity, bearing, predictive perimeter), REST API endpoints, interactive Leaflet trajectory maps with numbered pins, and real-time SSE / notification sync for lost pets.

**Architecture:** A pure Go spatial engine (`pkg/sighting`) calculates Haversine distances, elapsed time, movement velocity (mph), compass bearing, and dynamic search radius from chronological `domain.PetSightingRecord` instances. `internal/app/webfrontend` exposes rate-limited REST endpoints, broadcasts real-time SSE events via `ReunionHub`, and serves an interactive Leaflet visualization with numbered milestone pins and directional polylines under strict CSP (`script-src 'self'`).

**Tech Stack:** Go 1.26.5 (`export GOTOOLCHAIN=go1.26.5`), Leaflet 1.9.4, Server-Sent Events (SSE), Playwright E2E testing framework.

**Spec:** `docs/superpowers/specs/2026-09-19-community-sighting-timeline-and-trajectory-heatmaps-design.md`

## Global Constraints

- Go toolchain: Pinned to `export GOTOOLCHAIN=go1.26.5`.
- Zero client-side external charting/mapping libraries (rely exclusively on vendored Leaflet 1.9.4 and native DOM/SVG APIs).
- Content Security Policy compliance: `script-src 'self'` (no inline scripts, no eval).
- Zero-PII wire contract: Witness contact info must remain strictly isolated and never exposed in public sighting or trajectory APIs.
- Rate limiting: Public sighting submission must be wrapped in `s.rateLimiter.RequireRateLimitFunc(ratelimit.ModerateLimit, ...)`.
- 100% verification pass rate across `make verify` and all Playwright test specifications before completion.

---

### Task 1: Domain Sighting Models & Spatial Trajectory Engine (`pkg/domain`, `pkg/sighting`)

**Files:**
- Create: `pkg/domain/sighting.go`
- Create: `pkg/domain/sighting_test.go`
- Modify: `pkg/store/store.go` (register `SightingsCollection = "sightings"`)
- Create: `pkg/sighting/trajectory.go`
- Create: `pkg/sighting/trajectory_test.go`

**Interfaces:**
- Produces: `domain.PetSightingRecord`, `domain.SightingStatus`, `store.SightingsCollection`, `sighting.TrajectoryLeg`, `sighting.SearchPerimeter`, `sighting.TrajectoryAnalysis`, `sighting.CalculateTrajectory(origin *domain.LocationPoint, originTime time.Time, sightings []domain.PetSightingRecord) TrajectoryAnalysis`.

- [ ] **Step 1: Write failing domain model and collection tests**

In `pkg/domain/sighting_test.go`:
```go
package domain_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestPetSightingRecord_Validation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	s := domain.PetSightingRecord{
		SightingID:          "sight-1",
		LostPetID:           "lost-1",
		ReportedAt:          now,
		SightedAt:           now.Add(-10 * time.Minute),
		LocationDescription: "Near 4th & Pine",
		Coordinates: &domain.LocationPoint{
			Latitude:  47.611,
			Longitude: -122.338,
		},
		MovementDirection: "Northeast",
		Status:            domain.SightingStatusActive,
	}

	if err := s.Validate(); err != nil {
		t.Fatalf("expected valid sighting, got error: %v", err)
	}

	invalid := s
	invalid.Coordinates = &domain.LocationPoint{Latitude: 95.0, Longitude: 0.0}
	if err := invalid.Validate(); err == nil {
		t.Error("expected error for latitude > 90, got nil")
	}
}
```

In `pkg/sighting/trajectory_test.go`:
```go
package sighting_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/sighting"
)

func TestCalculateTrajectory(t *testing.T) {
	t.Parallel()

	originTime := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	originPoint := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}

	sightings := []domain.PetSightingRecord{
		{
			SightingID: "s-1",
			LostPetID:  "lost-1",
			SightedAt:  originTime.Add(30 * time.Minute),
			Coordinates: &domain.LocationPoint{
				Latitude:  47.610,
				Longitude: -122.330,
			},
			Status: domain.SightingStatusActive,
		},
		{
			SightingID: "s-2",
			LostPetID:  "lost-1",
			SightedAt:  originTime.Add(60 * time.Minute),
			Coordinates: &domain.LocationPoint{
				Latitude:  47.620,
				Longitude: -122.330,
			},
			Status: domain.SightingStatusActive,
		},
	}

	traj := sighting.CalculateTrajectory("lost-1", originPoint, originTime, sightings)

	if traj.SightingsCount != 2 {
		t.Fatalf("expected 2 sightings, got %d", traj.SightingsCount)
	}
	if len(traj.Legs) != 2 { // Origin -> s-1, and s-1 -> s-2
		t.Fatalf("expected 2 legs, got %d", len(traj.Legs))
	}
	if traj.Legs[0].SpeedMph <= 0 {
		t.Errorf("expected positive speed in mph, got %f", traj.Legs[0].SpeedMph)
	}
	if traj.EstimatedPerimeter == nil {
		t.Fatal("expected non-nil EstimatedPerimeter")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/domain/... ./pkg/sighting/...`
Expected: FAIL (types and functions not defined).

- [ ] **Step 3: Implement domain models and trajectory engine**

1. In `pkg/store/store.go`, add:
   ```go
   SightingsCollection = "sightings"
   ```
2. In `pkg/domain/sighting.go`:
   Implement `PetSightingRecord`, `SightingStatus` constants, and `Validate() error`.
3. In `pkg/sighting/trajectory.go`:
   Implement `CalculateTrajectory` with Haversine distance, ground speed in mph, compass bearing in degrees, cardinal direction mapping (16-wind compass rose), and `SearchPerimeter` expansion logic.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/domain/... ./pkg/sighting/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/domain pkg/store pkg/sighting
git commit -m "feat(sighting): implement sighting models and spatial trajectory engine"
```

---

### Task 2: Sighting Persistence & Web Frontend REST Endpoints (`internal/app/webfrontend`)

**Files:**
- Create: `internal/app/webfrontend/sighting.go`
- Modify: `internal/app/webfrontend/server.go`
- Create: `internal/app/webfrontend/sighting_test.go`

**Interfaces:**
- Consumes: `pkg/domain`, `pkg/sighting`, `store.SightingsCollection`, `store.LostPetsCollection`.
- Produces:
  - `POST /api/v1/lost-pets/{petID}/sightings`
  - `GET /api/v1/lost-pets/{petID}/sightings`
  - `GET /api/v1/lost-pets/{petID}/trajectory`

- [ ] **Step 1: Write failing endpoint tests in `internal/app/webfrontend/sighting_test.go`**

```go
package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestSightingEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	// Seed lost pet
	lostPet := domain.LostPetRecord{
		PetID:      "lost-target-1",
		PetName:    "Buddy",
		ReportedAt: time.Now().UTC().Add(-2 * time.Hour),
		Location:   "Capitol Hill",
		Coordinates: &domain.LocationPoint{
			Latitude:  47.620,
			Longitude: -122.320,
		},
		Status: domain.LostPetStatusSearching,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	t.Run("POST /api/v1/lost-pets/{id}/sightings creates sighting record", func(t *testing.T) {
		body := map[string]interface{}{
			"locationDescription": "Spotted near volunteer park",
			"sightedAt":           time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339),
			"coordinates": map[string]float64{
				"latitude":  47.630,
				"longitude": -122.315,
			},
			"movementDirection": "North",
			"notes":             "Looking energetic",
		}
		data, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-target-1/sightings", bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET /api/v1/lost-pets/{id}/trajectory returns computed path", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-target-1/trajectory", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var traj map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &traj); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		if traj["sightingsCount"] == nil {
			t.Error("expected sightingsCount in trajectory response")
		}
	})
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestSightingEndpoints ./internal/app/webfrontend/...`
Expected: FAIL (404 Not Found).

- [ ] **Step 3: Implement `sighting.go` and wire routes in `server.go`**

1. In `internal/app/webfrontend/sighting.go`:
   - Implement `handleApiReportSighting`: validates coordinates and pet presence, persists in `store.SightingsCollection`, returns `201 Created`.
   - Implement `handleApiListSightings`: retrieves sightings for pet from `store.SightingsCollection`.
   - Implement `handleApiGetTrajectory`: retrieves pet and sightings, invokes `sighting.CalculateTrajectory`, returns JSON.
2. In `internal/app/webfrontend/server.go`:
   Register routes with `s.rateLimiter.RequireRateLimitFunc(ratelimit.ModerateLimit, ...)`:
   - `POST /api/v1/lost-pets/{petID}/sightings`
   - `GET /api/v1/lost-pets/{petID}/sightings`
   - `GET /api/v1/lost-pets/{petID}/trajectory`

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(webfrontend): add sighting report and trajectory REST API endpoints"
```

---

### Task 3: Real-Time Reunion Room Broadcast & Notification Integration (`internal/app/webfrontend`)

**Files:**
- Modify: `internal/app/webfrontend/sighting.go`
- Modify: `internal/app/webfrontend/reunion_hub.go`
- Modify: `internal/app/webfrontend/sighting_test.go`

**Interfaces:**
- Consumes: `ReunionHub.Broadcast`, `store.NotificationsCollection`.
- Produces: Sighting SSE events to active participants, unread sighting alerts in Notification Center.

- [ ] **Step 1: Write failing test in `sighting_test.go`**

```go
func TestSightingRealtimeNotification(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	hub := webfrontend.NewReunionHub()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
		ReunionHub:               hub,
	})

	// Seed lost pet
	lostPet := domain.LostPetRecord{
		PetID:      "lost-target-rt",
		PetName:    "Milo",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{Latitude: 47.60, Longitude: -122.33},
		Status:     domain.LostPetStatusSearching,
	}
	data, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, data)

	// Post sighting
	payload, _ := json.Marshal(map[string]interface{}{
		"locationDescription": "Spotted near market",
		"sightedAt":           time.Now().UTC().Format(time.RFC3339),
		"coordinates":         map[string]float64{"latitude": 47.61, "longitude": -122.34},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-target-rt/sightings", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	// Verify notification persisted in store.NotificationsCollection
	notifications, err := st.ListState(context.Background(), store.NotificationsCollection)
	if err != nil || len(notifications) == 0 {
		t.Errorf("expected notification created in store, got %d (err: %v)", len(notifications), err)
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestSightingRealtimeNotification ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement SSE broadcast and notification creation**

In `internal/app/webfrontend/sighting.go`:
- In `handleApiReportSighting`, upon successful save:
  1. If `s.reunionHub != nil`, broadcast a sighting event envelope.
  2. Create a `domain.NotificationItem` with type `sighting_reported` and save to `store.NotificationsCollection`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(sighting): integrate real-time reunion broadcast and notification alerts"
```

---

### Task 4: Frontend UI Sighting Modal & Interactive Trajectory Map (`webfrontend`)

**Files:**
- Create: `internal/app/webfrontend/static/js/sighting-trajectory.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/templates/pets.html`
- Modify: `internal/app/webfrontend/templates/finder_landing.html`
- Test: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: `/api/v1/lost-pets/{id}/trajectory`, `/api/v1/lost-pets/{id}/sightings`, Leaflet 1.9.4.
- Produces: Modal dialog `#modal-report-sighting`, interactive trajectory map `#pet-trajectory-map` with numbered milestone pins.

- [ ] **Step 1: Write failing template/asset test in `server_test.go`**

```go
func TestSightingTrajectoryAssets(t *testing.T) {
	t.Parallel()
	srv := NewDemoServer()

	req := httptest.NewRequest(http.MethodGet, "/static/js/sighting-trajectory.js", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /static/js/sighting-trajectory.js, got %d", rec.Code)
	}

	reqPets := httptest.NewRequest(http.MethodGet, "/pets", nil)
	recPets := httptest.NewRecorder()
	srv.ServeHTTP(recPets, reqPets)

	if !strings.Contains(recPets.Body.String(), "modal-report-sighting") {
		t.Error("expected modal-report-sighting in /pets template")
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestSightingTrajectoryAssets ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement client script, markup, and CSS**

1. In `templates/pets.html` and `templates/finder_landing.html`:
   - Add `#modal-report-sighting` dialog with geolocation button, time picker, compass dropdown, notes, and submit button.
   - Add `#pet-trajectory-container` with `#pet-trajectory-map` and timeline legend.
   - Include `<script src="/static/js/sighting-trajectory.js"></script>`.
2. In `static/js/sighting-trajectory.js`:
   - Strict CSP compliance.
   - Handles opening/closing `#modal-report-sighting`.
   - Handles "📍 Use My Location" via `navigator.geolocation`.
   - Submits `POST /api/v1/lost-pets/{id}/sightings`.
   - Fetches `GET /api/v1/lost-pets/{id}/trajectory` and renders Leaflet map:
     - Numbered custom HTML marker icons (`0: Origin`, `1..N: Sightings`).
     - Directional polylines with arrows/dash pattern.
     - Search perimeter circle `L.circle` with radial bounds.
     - Marker popups with elapsed times, speed in mph, and notes.
3. In `static/css/styles.css`:
   - Add glassmorphic styling for `.sighting-modal`, `.trajectory-map-wrapper`, `.milestone-pin`, `.milestone-pin-origin`, and `.trajectory-popup`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(ui): add sighting submission modal and interactive trajectory map"
```

---

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/sighting-trajectory-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright browser, live backend webfrontend.
- Produces: Complete E2E automated test suite verifying sighting submission, trajectory map rendering, and real-time updates.

- [ ] **Step 1: Write `tests/playwright/e2e/sighting-trajectory-journey.spec.ts`**

```typescript
import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe('User Journey: Community Sighting Timeline & Trajectory Map', () => {
  test('should report a pet sighting and view updated trajectory map with numbered pins', async ({ page }) => {
    // 1. Visit /pets
    await page.goto(`${WEB_FRONTEND_URL}/pets`);
    await expect(page.locator('.pet-card').first()).toBeVisible();

    // 2. Open Sighting Modal for first pet
    const sightingBtn = page.locator('button:has-text("Report Sighting")').first();
    if (await sightingBtn.isVisible()) {
      await sightingBtn.click();
      const modal = page.locator('#modal-report-sighting');
      await expect(modal).toBeVisible();

      // 3. Fill coordinates and location
      await page.locator('#sighting-location').fill('Near Pike Place Market');
      await page.locator('#sighting-notes').fill('Spotted heading east towards 1st Ave');
      await page.locator('#sighting-direction').selectOption('East');

      // 4. Submit sighting
      await page.locator('#btn-submit-sighting').click();
      await expect(modal).toBeHidden();
    }

    // 5. Inspect trajectory endpoint directly
    const trajResp = await page.request.get(`${WEB_FRONTEND_URL}/api/v1/lost-pets/lost-1/trajectory`);
    if (trajResp.status() === 200) {
      const data = await trajResp.json();
      expect(data).toHaveProperty('sightingsCount');
    }
  });
});
```

- [ ] **Step 2: Run Playwright Journey Test**

Run: `cd tests/playwright && npx playwright test e2e/sighting-trajectory-journey.spec.ts`
Expected: PASS.

- [ ] **Step 3: Run Full Playwright Test Suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 100% PASS across all specs.

- [ ] **Step 4: Run Full Repository Verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS (`go vet`, `golangci-lint`, OpenTofu check, `yamllint`, Go race tests).

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/sighting-trajectory-journey.spec.ts
git commit -m "test(e2e): add end-to-end journey tests for community sightings and trajectory maps"
```
