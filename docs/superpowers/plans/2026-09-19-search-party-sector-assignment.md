# Milestone 8.1: Community Search Party Dispatch & Volunteer Geo-Fenced Sector Assignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement spatial sector decomposition, search party management, volunteer sector claiming, REST API endpoints, interactive Leaflet sector overlay maps, and real-time SSE coverage updates for lost pets.

**Architecture:** A pure Go spatial engine (`pkg/searchparty`) decomposes search perimeters into distinct geo-fenced sectors (wedges) based on center points and radii. `internal/app/webfrontend` exposes REST endpoints for party creation, sector assignment, and status updates, broadcasts real-time SSE events via `ReunionHub`, and serves an interactive Leaflet visualization with sector polygons, claim modals, and a live coverage progress bar.

**Tech Stack:** Go 1.26.5 (`export GOTOOLCHAIN=go1.26.5`), Leaflet 1.9.4, Server-Sent Events (SSE), Playwright E2E testing framework.

**Spec:** `docs/superpowers/specs/2026-09-19-search-party-sector-assignment-design.md`

## Global Constraints

- User Review Required: None (standard SDD automation).
- Go toolchain: Pinned to `export GOTOOLCHAIN=go1.26.5`.
- Zero client-side external charting/mapping libraries (rely exclusively on vendored Leaflet 1.9.4 and native DOM/SVG APIs).
- Content Security Policy compliance: `script-src 'self'` (no inline scripts, no eval).
- Zero-PII wire contract: Volunteer contact info must remain strictly isolated and never exposed in public sector APIs; only aliases/pseudonyms are permitted.
- 100% verification pass rate across `make verify` and all Playwright test specifications before completion.

---

### Task 1: Domain Models & Spatial Sector Decomposition Engine (`pkg/domain`, `pkg/searchparty`, `pkg/store`)

**Files:**
- Create: `pkg/searchparty/sector.go`
- Create: `pkg/searchparty/sector_test.go`
- Modify: `pkg/store/store.go` (register `SearchPartiesCollection = "searchParties"` and `SectorAssignmentsCollection = "sectorAssignments"`)

**Interfaces:**
- Produces: `searchparty.SectorStatus`, `searchparty.SearchSector`, `searchparty.SectorAssignment`, `searchparty.SearchParty`, `searchparty.DecomposePerimeter(center *domain.LocationPoint, radiusMeters float64, sectorCount int) []searchparty.SearchSector`.

- [ ] **Step 1: Write failing domain model and collection tests**

In `pkg/searchparty/sector_test.go`:
```go
package searchparty_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/searchparty"
)

func TestSearchParty_DecomposePerimeter(t *testing.T) {
	t.Parallel()

	center := &domain.LocationPoint{Latitude: 47.600, Longitude: -122.330}
	radius := 1000.0 // 1km
	sectorCount := 4

	sectors := searchparty.DecomposePerimeter(center, radius, sectorCount)

	if len(sectors) != sectorCount {
		t.Fatalf("expected %d sectors, got %d", sectorCount, len(sectors))
	}

	for _, s := range sectors {
		if len(s.PolygonPoints) < 3 {
			t.Errorf("sector %s has invalid polygon with %d points", s.SectorID, len(s.PolygonPoints))
		}
		if s.Status != searchparty.SectorStatusUnassigned {
			t.Errorf("expected new sector to be unassigned, got %s", s.Status)
		}
		if s.TotalAreaSqM <= 0 {
			t.Errorf("expected positive area, got %f", s.TotalAreaSqM)
		}
	}
}

func TestSearchParty_Coverage(t *testing.T) {
	t.Parallel()

	party := searchparty.SearchParty{
		PartyID: "party-1",
		Sectors: []searchparty.SearchSector{
			{SectorID: "s-1", Status: searchparty.SectorStatusCleared, TotalAreaSqM: 500},
			{SectorID: "s-2", Status: searchparty.SectorStatusUnassigned, TotalAreaSqM: 500},
		},
	}

	coverage := party.CalculateCoverage()
	if coverage != 50.0 {
		t.Errorf("expected 50%% coverage, got %f", coverage)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/searchparty/...`
Expected: FAIL (types and functions not defined).

- [ ] **Step 3: Implement domain models and decomposition engine**

1. In `pkg/store/store.go`, add:
   ```go
   SearchPartiesCollection = "searchParties"
   SectorAssignmentsCollection = "sectorAssignments"
   ```
2. In `pkg/searchparty/sector.go`:
   Implement `SearchParty`, `SearchSector`, `SectorAssignment`, `SectorStatus` constants.
   Implement `DecomposePerimeter` with spatial math to generate polygons.
   Implement `CalculateCoverage` logic on `SearchParty`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/searchparty/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/store pkg/searchparty
git commit -m "feat(searchparty): implement spatial sector decomposition engine and domain models"
```

---

### Task 2: Web Frontend Search Party REST Endpoints (`internal/app/webfrontend/searchparty.go`)

**Files:**
- Create: `internal/app/webfrontend/searchparty.go`
- Modify: `internal/app/webfrontend/server.go`
- Create: `internal/app/webfrontend/searchparty_test.go`

**Interfaces:**
- Consumes: `pkg/domain`, `pkg/searchparty`, `store.SearchPartiesCollection`, `store.SectorAssignmentsCollection`.
- Produces:
  - `POST /api/v1/lost-pets/{petID}/search-party`
  - `GET /api/v1/lost-pets/{petID}/search-party`
  - `POST /api/v1/search-parties/{partyID}/sectors/{sectorID}/claim`
  - `POST /api/v1/search-parties/{partyID}/sectors/{sectorID}/status`

- [ ] **Step 1: Write failing endpoint tests in `internal/app/webfrontend/searchparty_test.go`**

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

func TestSearchPartyEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	// Seed lost pet
	lostPet := domain.LostPetRecord{
		PetID:      "lost-target-2",
		PetName:    "Luna",
		ReportedAt: time.Now().UTC().Add(-1 * time.Hour),
		Coordinates: &domain.LocationPoint{Latitude: 47.620, Longitude: -122.320},
		Status: domain.LostPetStatusSearching,
	}
	lostBytes, _ := json.Marshal(lostPet)
	_ = st.SaveState(context.Background(), store.LostPetsCollection, lostPet.PetID, lostBytes)

	t.Run("POST /api/v1/lost-pets/{id}/search-party creates party", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/lost-pets/lost-target-2/search-party", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET /api/v1/lost-pets/{id}/search-party returns party", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/lost-pets/lost-target-2/search-party", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		var party map[string]interface{}
		json.Unmarshal(rec.Body.Bytes(), &party)
		if party["partyId"] == nil {
			t.Error("expected partyId in response")
		}
	})
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestSearchPartyEndpoints ./internal/app/webfrontend/...`
Expected: FAIL (404 Not Found).

- [ ] **Step 3: Implement `searchparty.go` and wire routes in `server.go`**

1. In `internal/app/webfrontend/searchparty.go`:
   - Implement handlers for creating party, getting party, claiming sector, and updating sector status.
   - Enforce Zero-PII by generating volunteer aliases.
2. In `internal/app/webfrontend/server.go`:
   - Register the 4 routes.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(webfrontend): add search party REST API endpoints"
```

---

### Task 3: Real-Time Reunion Room SSE Broadcast & Coverage Updates (`internal/app/webfrontend/searchparty.go`, `reunion_hub.go`)

**Files:**
- Modify: `internal/app/webfrontend/searchparty.go`
- Modify: `internal/app/webfrontend/reunion_hub.go`
- Modify: `internal/app/webfrontend/searchparty_test.go`

**Interfaces:**
- Consumes: `ReunionHub.Broadcast`.
- Produces: Search party SSE events (`search_party_updated`) reflecting coverage and status changes.

- [ ] **Step 1: Write failing test in `searchparty_test.go`**

```go
func TestSearchPartyRealtimeBroadcast(t *testing.T) {
	// Setup test similar to Task 2, but inject ReunionHub
	// Trigger sector status change
	// Verify hub receives the broadcast event.
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestSearchPartyRealtimeBroadcast ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement SSE broadcast**

In `internal/app/webfrontend/searchparty.go`:
- In `handleApiUpdateSectorStatus`, calculate new coverage percentage and broadcast `search_party_updated` event via `ReunionHub`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(searchparty): integrate real-time reunion broadcast for coverage updates"
```

---

### Task 4: Frontend UI Sector Overlay & Volunteer Claim Modal (`webfrontend`, `search-party.js`, `pets.html`, `styles.css`)

**Files:**
- Create: `internal/app/webfrontend/static/js/search-party.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/templates/pets.html`
- Modify: `internal/app/webfrontend/templates/finder_landing.html`
- Test: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: Search party REST endpoints, Leaflet 1.9.4.
- Produces: `#search-party-map` polygon overlay, quick-claim modal, clearance form, and live coverage progress bar.

- [ ] **Step 1: Write failing template/asset test in `server_test.go`**

```go
func TestSearchPartyAssets(t *testing.T) {
	// Verify /static/js/search-party.js is served
	// Verify #modal-claim-sector exists in templates
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestSearchPartyAssets ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement client script, markup, and CSS**

1. In templates: Add `#modal-claim-sector`, `#modal-clear-sector`, and `#search-party-map`.
2. In `static/js/search-party.js`:
   - Fetch search party data, render Leaflet polygons with color-coding based on status.
   - Wire up click handlers to claim/update sectors.
   - Listen to SSE `search_party_updated` to dynamically update polygon colors and coverage UI.
3. In `static/css/styles.css`:
   - Add styles for modals and map polygons.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(ui): add search party sector map and claim modals"
```

---

### Task 5: Automated Playwright E2E User Journey & Full Verification (`tests/playwright/e2e/search-party-journey.spec.ts`)

**Files:**
- Create: `tests/playwright/e2e/search-party-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright browser, live backend webfrontend.
- Produces: E2E automated test suite verifying search party initialization, sector claiming, map updates, and coverage progress.

- [ ] **Step 1: Write `tests/playwright/e2e/search-party-journey.spec.ts`**

```typescript
import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe('User Journey: Community Search Party & Sector Claiming', () => {
  test('should initialize a search party, claim a sector, and mark it cleared', async ({ page }) => {
    // 1. Visit /pets
    await page.goto(`${WEB_FRONTEND_URL}/pets`);
    // 2. Click Initialize Search Party
    // 3. Verify Map and Sectors appear
    // 4. Click a sector, claim it
    // 5. Verify sector color changes
    // 6. Mark sector as cleared with notes
    // 7. Verify coverage percentage increases
  });
});
```

- [ ] **Step 2: Run Playwright Journey Test**

Run: `cd tests/playwright && npx playwright test e2e/search-party-journey.spec.ts`
Expected: PASS.

- [ ] **Step 3: Run Full Playwright Test Suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 100% PASS across all specs.

- [ ] **Step 4: Run Full Repository Verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS (`go vet`, `golangci-lint`, OpenTofu check, `yamllint`, Go race tests).

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/search-party-journey.spec.ts
git commit -m "test(e2e): add end-to-end journey tests for search party sector assignments"
```
