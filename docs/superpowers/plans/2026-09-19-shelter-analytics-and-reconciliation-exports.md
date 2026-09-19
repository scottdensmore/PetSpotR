# Shelter Analytics & Automated Reconciliation Exports Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide real-time municipal shelter analytics, return-to-owner (RTO) KPIs, and automated CSV / GeoJSON reconciliation exports for pet recovery coalitions.

**Architecture:** A pure Go analytics aggregation engine (`pkg/analytics`) evaluates live `FoundPetRecord` and `MatchRecord` aggregates from `store.StateStore`, computing RTO rates, median turnaround velocities, and microchip scan penetration. Native streaming serializers produce RFC 4180 CSV and RFC 7946 GeoJSON export feeds with strict Zero-PII masking. Responsive frontend views (`/shelters/analytics`) display glassmorphic KPI cards, shelter filtering, and export download links without external JS libraries.

**Tech Stack:** Go 1.26.5, `encoding/csv`, HTML5 / CSS3 (dark/light theme custom properties), Vanilla ES2020+, Playwright.

**Spec:** [`docs/superpowers/specs/2026-09-19-shelter-analytics-and-reconciliation-exports-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-19-shelter-analytics-and-reconciliation-exports-design.md)

## Global Constraints
- Go toolchain: Pinned to `go1.26.5` (`export GOTOOLCHAIN=go1.26.5`).
- Zero-PII contract: Raw microchips must be masked via `microchip.MaskMicrochip(raw)` (`HomeAgain ••••3456`); personal emails and phone numbers must never appear in exports.
- CSP compliance: `script-src 'self'` with zero inline event handlers and zero external charting scripts.
- Verification: 100% clean pass across `make verify` and full Playwright test suite.

---

### Task 1: Analytics Data Engine & KPI Aggregation (`pkg/analytics`)

**Files:**
- Create: `pkg/analytics/analytics.go`
- Create: `pkg/analytics/analytics_test.go`

**Interfaces:**
- Consumes: `domain.FoundPetRecord`, `domain.MatchRecord`, `pkg/domain`.
- Produces: `ComputeAnalytics(foundPets []domain.FoundPetRecord, matches []domain.MatchRecord, filter FilterOptions) AnalyticsReport`, `FilterOptions`, `AnalyticsReport`, `ShelterKPISummary`, `MunicipalShelterStats`.

- [ ] **Step 1: Write failing tests in `pkg/analytics/analytics_test.go`**

```go
package analytics_test

import (
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/analytics"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestComputeAnalytics(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	intakeTime1 := now.Add(-48 * time.Hour)
	intakeTime2 := now.Add(-24 * time.Hour)
	intakeTime3 := now.Add(-12 * time.Hour)

	foundPets := []domain.FoundPetRecord{
		{
			PetID:             "found-1",
			ShelterID:         "shelter-sea-01",
			ShelterName:       "Seattle Animal Shelter",
			IntakeID:          "INT-101",
			CustodyStatus:     domain.CustodyShelterCare,
			Status:            domain.FoundPetStatusFound,
			FoundAt:           intakeTime1,
			MicrochipID:       "985141000123456",
			MicrochipRegistry: "HomeAgain",
		},
		{
			PetID:             "found-2",
			ShelterID:         "shelter-sea-01",
			ShelterName:       "Seattle Animal Shelter",
			IntakeID:          "INT-102",
			CustodyStatus:     domain.CustodyShelterCare,
			Status:            domain.FoundPetStatusResolved,
			FoundAt:           intakeTime2,
			MicrochipID:       "981010000999999",
			MicrochipRegistry: "AKC Reunite",
		},
		{
			PetID:         "found-3",
			ShelterID:     "shelter-bel-02",
			ShelterName:   "Bellevue Humane Society",
			IntakeID:      "BEL-501",
			CustodyStatus: domain.CustodyShelterCare,
			Status:        domain.FoundPetStatusFound,
			FoundAt:       intakeTime3,
		},
	}

	matches := []domain.MatchRecord{
		{
			MatchID:            "match-1",
			FoundPetID:         "found-1",
			MatchedPetID:       "lost-1",
			Status:             domain.MatchStatusPendingReview,
			DeterministicMatch: true,
			MatchType:          "deterministic_microchip",
			MatchedAt:          intakeTime1.Add(2 * time.Hour),
		},
		{
			MatchID:            "match-2",
			FoundPetID:         "found-2",
			MatchedPetID:       "lost-2",
			Status:             domain.MatchStatusReunited,
			DeterministicMatch: true,
			MatchType:          "deterministic_microchip",
			MatchedAt:          intakeTime2.Add(4 * time.Hour),
		},
	}

	t.Run("Computes overall KPIs correctly across all shelters", func(t *testing.T) {
		report := analytics.ComputeAnalytics(foundPets, matches, analytics.FilterOptions{})

		if report.OverallKPIs.TotalIntakes != 3 {
			t.Errorf("expected 3 total intakes, got %d", report.OverallKPIs.TotalIntakes)
		}
		if report.OverallKPIs.ActiveInShelterCare != 2 {
			t.Errorf("expected 2 active in shelter care, got %d", report.OverallKPIs.ActiveInShelterCare)
		}
		if report.OverallKPIs.ReunitedCount != 1 {
			t.Errorf("expected 1 reunited, got %d", report.OverallKPIs.ReunitedCount)
		}
		expectedRTORate := 1.0 / 3.0
		if report.OverallKPIs.ReturnToOwnerRate < expectedRTORate-0.01 || report.OverallKPIs.ReturnToOwnerRate > expectedRTORate+0.01 {
			t.Errorf("expected RTO rate ~0.33, got %f", report.OverallKPIs.ReturnToOwnerRate)
		}
		if report.OverallKPIs.MicrochippedCount != 2 {
			t.Errorf("expected 2 microchipped pets, got %d", report.OverallKPIs.MicrochippedCount)
		}
		expectedScanRate := 2.0 / 3.0
		if report.OverallKPIs.MicrochipScanRate < expectedScanRate-0.01 || report.OverallKPIs.MicrochipScanRate > expectedScanRate+0.01 {
			t.Errorf("expected microchip scan rate ~0.67, got %f", report.OverallKPIs.MicrochipScanRate)
		}
		if report.OverallKPIs.DeterministicMatchCount != 2 {
			t.Errorf("expected 2 deterministic matches, got %d", report.OverallKPIs.DeterministicMatchCount)
		}
		if report.OverallKPIs.MedianIntakeToMatchHours != 3.0 {
			t.Errorf("expected median match velocity 3.0 hours (midpoint of 2.0 and 4.0), got %f", report.OverallKPIs.MedianIntakeToMatchHours)
		}
	})

	t.Run("Filters by ShelterID correctly", func(t *testing.T) {
		report := analytics.ComputeAnalytics(foundPets, matches, analytics.FilterOptions{
			ShelterID: "shelter-bel-02",
		})

		if report.OverallKPIs.TotalIntakes != 1 {
			t.Errorf("expected 1 intake for Bellevue, got %d", report.OverallKPIs.TotalIntakes)
		}
		if report.OverallKPIs.MicrochippedCount != 0 {
			t.Errorf("expected 0 microchipped pets for Bellevue, got %d", report.OverallKPIs.MicrochippedCount)
		}
	})
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v ./pkg/analytics/...`
Expected: FAIL (package `pkg/analytics` not found).

- [ ] **Step 3: Implement `pkg/analytics/analytics.go`**

1. Define `FilterOptions`, `ShelterKPISummary`, `MunicipalShelterStats`, `ShelterInfo`, and `AnalyticsReport`.
2. Implement helper `calculateMedian(durations []float64) float64`.
3. Implement `ComputeAnalytics(foundPets []domain.FoundPetRecord, matches []domain.MatchRecord, filter FilterOptions) AnalyticsReport`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/analytics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/analytics
git commit -m "feat(analytics): implement shelter metrics aggregation engine"
```

---

### Task 2: Reconciliation Exporters: CSV & GeoJSON Serializers (`pkg/analytics`)

**Files:**
- Create: `pkg/analytics/export.go`
- Create: `pkg/analytics/export_test.go`

**Interfaces:**
- Consumes: `pkg/microchip`, `pkg/domain`, `AnalyticsReport`.
- Produces: `ExportReconciliationCSV(w io.Writer, foundPets []domain.FoundPetRecord, matches []domain.MatchRecord, filter FilterOptions) error`, `ExportReconciliationGeoJSON(w io.Writer, foundPets []domain.FoundPetRecord, matches []domain.MatchRecord, filter FilterOptions) error`.

- [ ] **Step 1: Write failing tests in `pkg/analytics/export_test.go`**

```go
package analytics_test

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/analytics"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestExportReconciliationCSV(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	foundPets := []domain.FoundPetRecord{
		{
			PetID:             "found-1",
			ShelterID:         "shelter-sea-01",
			ShelterName:       "Seattle Animal Shelter",
			IntakeID:          "INT-8819",
			CustodyStatus:     domain.CustodyShelterCare,
			Status:            domain.FoundPetStatusFound,
			FoundAt:           now.Add(-24 * time.Hour),
			Species:           "Dog",
			Breed:             "Golden Retriever",
			PrimaryColor:      "Golden",
			MicrochipID:       "985141000123456",
			MicrochipRegistry: "HomeAgain",
		},
	}

	matches := []domain.MatchRecord{
		{
			MatchID:            "match-1",
			FoundPetID:         "found-1",
			MatchedPetID:       "lost-1",
			Score:              1.0,
			Status:             domain.MatchStatusConfirmed,
			DeterministicMatch: true,
			MatchType:          "deterministic_microchip",
			MatchedAt:          now.Add(-20 * time.Hour),
		},
	}

	var buf bytes.Buffer
	err := analytics.ExportReconciliationCSV(&buf, foundPets, matches, analytics.FilterOptions{})
	if err != nil {
		t.Fatalf("ExportReconciliationCSV failed: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 CSV rows (1 header + 1 record), got %d", len(records))
	}

	header := records[0]
	if header[0] != "IntakeID" || header[9] != "MaskedMicrochip" {
		t.Errorf("unexpected CSV headers: %v", header)
	}

	row := records[1]
	if row[0] != "INT-8819" {
		t.Errorf("expected IntakeID INT-8819, got %s", row[0])
	}
	// Zero-PII assertion
	if !strings.Contains(row[9], "••••3456") || strings.Contains(row[9], "985141000123456") {
		t.Errorf("expected masked microchip, got %s", row[9])
	}
	if row[11] != "CONFIRMED" {
		t.Errorf("expected MatchStatus CONFIRMED, got %s", row[11])
	}
}

func TestExportReconciliationGeoJSON(t *testing.T) {
	t.Parallel()

	foundPets := []domain.FoundPetRecord{
		{
			PetID:         "found-geo-1",
			ShelterID:     "shelter-sea-01",
			ShelterName:   "Seattle Animal Shelter",
			IntakeID:      "INT-9901",
			CustodyStatus: domain.CustodyShelterCare,
			Coordinates: &domain.LocationPoint{
				Latitude:  47.648,
				Longitude: -122.378,
			},
			Species: "Cat",
			Breed:   "Tabby",
		},
	}

	var buf bytes.Buffer
	err := analytics.ExportReconciliationGeoJSON(&buf, foundPets, nil, analytics.FilterOptions{})
	if err != nil {
		t.Fatalf("ExportReconciliationGeoJSON failed: %v", err)
	}

	var fc map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &fc); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}

	if fc["type"] != "FeatureCollection" {
		t.Errorf("expected FeatureCollection, got %v", fc["type"])
	}

	features := fc["features"].([]interface{})
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run "TestExportReconciliation" ./pkg/analytics/...`
Expected: FAIL (functions not defined).

- [ ] **Step 3: Implement `pkg/analytics/export.go`**

1. Implement `ExportReconciliationCSV` writing RFC 4180 header and rows, masking microchips with `microchip.MaskMicrochip(chip)`.
2. Implement `ExportReconciliationGeoJSON` writing RFC 7946 FeatureCollection with point coordinates (`[longitude, latitude]`) and intake metadata properties.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./pkg/analytics/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/analytics
git commit -m "feat(analytics): add CSV and GeoJSON reconciliation exporters"
```

---

### Task 3: Web Frontend Analytics & Export Endpoints (`internal/app/webfrontend`)

**Files:**
- Create: `internal/app/webfrontend/shelter_analytics.go`
- Modify: `internal/app/webfrontend/server.go`
- Create: `internal/app/webfrontend/shelter_analytics_test.go`

**Interfaces:**
- Consumes: `pkg/analytics`, `store.FoundPetsCollection`, `store.MatchesCollection`.
- Produces: `GET /api/v1/shelters/analytics`, `GET /api/v1/shelters/analytics/export.csv`, `GET /api/v1/shelters/analytics/export.geojson`, and `GET /shelters/analytics`.

- [ ] **Step 1: Write failing tests in `internal/app/webfrontend/shelter_analytics_test.go`**

```go
package webfrontend_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestShelterAnalyticsEndpoints(t *testing.T) {
	t.Parallel()

	st := store.NewMemoryStore()
	srv := webfrontend.NewServerWithOptions(st, webfrontend.ServerOptions{
		AllowPrivilegedMutations: true,
	})

	t.Run("GET /api/v1/shelters/analytics returns JSON report", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
			t.Errorf("expected application/json content type, got %s", rec.Header().Get("Content-Type"))
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid json body: %v", err)
		}
		if resp["overallKpis"] == nil {
			t.Error("expected overallKpis key in response")
		}
	})

	t.Run("GET /api/v1/shelters/analytics/export.csv streams CSV file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics/export.csv", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") {
			t.Errorf("expected text/csv content type, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
			t.Errorf("expected attachment Content-Disposition, got %s", rec.Header().Get("Content-Disposition"))
		}
		if !strings.Contains(rec.Body.String(), "IntakeID,ShelterID") {
			t.Errorf("expected CSV header row, got %s", rec.Body.String())
		}
	})

	t.Run("GET /api/v1/shelters/analytics/export.geojson streams GeoJSON FeatureCollection", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/shelters/analytics/export.geojson", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "application/geo+json") {
			t.Errorf("expected application/geo+json, got %s", rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), `"type":"FeatureCollection"`) {
			t.Errorf("expected FeatureCollection, got %s", rec.Body.String())
		}
	})
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestShelterAnalyticsEndpoints ./internal/app/webfrontend/...`
Expected: FAIL (404 Not Found).

- [ ] **Step 3: Implement `shelter_analytics.go` and register routes in `server.go`**

1. In `internal/app/webfrontend/shelter_analytics.go`:
   - Implement `handleApiShelterAnalytics`: fetches records from `store.FoundPetsCollection` and `store.MatchesCollection`, parses query filters (`shelterId`, `range`, `startDate`, `endDate`), calls `analytics.ComputeAnalytics`, returns JSON.
   - Implement `handleApiShelterAnalyticsExportCSV`: streams `analytics.ExportReconciliationCSV`.
   - Implement `handleApiShelterAnalyticsExportGeoJSON`: streams `analytics.ExportReconciliationGeoJSON`.
   - Implement `handleShelterAnalytics`: renders `/shelters/analytics` template.
2. In `internal/app/webfrontend/server.go`:
   - Register routes on `s.mux`:
     - `s.mux.HandleFunc("/shelters/analytics", s.handleShelterAnalytics)`
     - `s.mux.HandleFunc("/api/v1/shelters/analytics", s.handleApiShelterAnalytics)`
     - `s.mux.HandleFunc("/api/v1/shelters/analytics/export.csv", s.handleApiShelterAnalyticsExportCSV)`
     - `s.mux.HandleFunc("/api/v1/shelters/analytics/export.geojson", s.handleApiShelterAnalyticsExportGeoJSON)`

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(webfrontend): add shelter analytics API and export routes"
```

---

### Task 4: Frontend Shelter Partner Analytics Dashboard UI (`webfrontend`)

**Files:**
- Create: `internal/app/webfrontend/templates/shelter-analytics.html`
- Create: `internal/app/webfrontend/static/js/shelter-analytics.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/templates/index.html`, `pets.html`, `matches.html`, `report-lost.html`, `report-found.html` (add `Shelter Analytics` to navigation)
- Test: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: DOM elements, `/api/v1/shelters/analytics`.
- Produces: Interactive dashboard UI, live filter updating, CSV and GeoJSON download actions.

- [ ] **Step 1: Write failing test in `internal/app/webfrontend/server_test.go`**

```go
func TestShelterAnalyticsPage(t *testing.T) {
	t.Parallel()
	srv := NewDemoServer()

	req := httptest.NewRequest(http.MethodGet, "/shelters/analytics", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /shelters/analytics, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "shelter-filter") {
		t.Error("expected #shelter-filter in /shelters/analytics")
	}
	if !strings.Contains(body, "btn-export-csv") {
		t.Error("expected #btn-export-csv in /shelters/analytics")
	}
	if !strings.Contains(body, "btn-export-geojson") {
		t.Error("expected #btn-export-geojson in /shelters/analytics")
	}
	if !strings.Contains(body, "kpi-rto-rate") {
		t.Error("expected #kpi-rto-rate in /shelters/analytics")
	}
}
```

- [ ] **Step 2: Run test to verify failure**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -run TestShelterAnalyticsPage ./internal/app/webfrontend/...`
Expected: FAIL.

- [ ] **Step 3: Implement template, JavaScript, CSS, and navigation**

1. Create `internal/app/webfrontend/templates/shelter-analytics.html` with:
   - Header with `#shelter-filter` and `#date-range-filter`.
   - Export buttons `#btn-export-csv` and `#btn-export-geojson`.
   - 4 Hero KPI Cards: `#kpi-rto-rate`, `#kpi-turnaround`, `#kpi-microchip-rate`, `#kpi-deterministic-ratio`.
   - Municipal Coalition Breakdown Table `#shelter-breakdown-table`.
2. Create `internal/app/webfrontend/static/js/shelter-analytics.js`:
   - Listens to filter changes, fetches `/api/v1/shelters/analytics`, and updates card values and export URLs dynamically.
3. Update `styles.css`:
   - Add styles for `.analytics-dashboard`, `.kpi-grid`, `.kpi-card`, `.kpi-metric`, `.analytics-table`.
4. Update navigation menus in templates to include `<a href="/shelters/analytics" class="nav-link">Shelter Analytics</a>`.

- [ ] **Step 4: Run tests to verify pass**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -v -race ./internal/app/webfrontend/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend
git commit -m "feat(ui): add shelter partner analytics dashboard and export actions"
```

---

### Task 5: Automated Playwright E2E User Journey Tests & Full Verification

**Files:**
- Create: `tests/playwright/e2e/shelter-analytics-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright browser, live backend webfrontend.
- Produces: Automated verification of dashboard navigation, filter interactions, and CSV/GeoJSON export feeds.

- [ ] **Step 1: Author `tests/playwright/e2e/shelter-analytics-journey.spec.ts`**

```typescript
import { test, expect } from '@playwright/test';

const WEB_FRONTEND_URL = process.env.WEB_FRONTEND_URL || process.env.BASE_URL || 'http://localhost:8082';

test.describe('User Journey: Shelter Analytics Dashboard & Reconciliation Exports', () => {
  test('should navigate to shelter analytics dashboard, inspect KPIs, and download exports', async ({ page }) => {
    // 1. Visit /shelters/analytics
    await page.goto(`${WEB_FRONTEND_URL}/shelters/analytics`);

    // 2. Assert page title and heading
    await expect(page.locator('h1')).toContainText('Shelter Partner Analytics');

    // 3. Assert KPI cards are visible
    await expect(page.locator('#kpi-rto-rate')).toBeVisible();
    await expect(page.locator('#kpi-turnaround')).toBeVisible();
    await expect(page.locator('#kpi-microchip-rate')).toBeVisible();
    await expect(page.locator('#kpi-deterministic-ratio')).toBeVisible();

    // 4. Assert shelter filter dropdown exists
    const shelterFilter = page.locator('#shelter-filter');
    await expect(shelterFilter).toBeVisible();

    // 5. Verify CSV export button link and content
    const csvExportBtn = page.locator('#btn-export-csv');
    await expect(csvExportBtn).toBeVisible();
    const csvHref = await csvExportBtn.getAttribute('href');
    expect(csvHref).toContain('/api/v1/shelters/analytics/export.csv');

    // 6. Verify GeoJSON export button link
    const geojsonExportBtn = page.locator('#btn-export-geojson');
    await expect(geojsonExportBtn).toBeVisible();
    const geojsonHref = await geojsonExportBtn.getAttribute('href');
    expect(geojsonHref).toContain('/api/v1/shelters/analytics/export.geojson');

    // 7. Verify direct CSV API response
    const csvResponse = await page.request.get(`${WEB_FRONTEND_URL}/api/v1/shelters/analytics/export.csv`);
    expect(csvResponse.status()).toBe(200);
    expect(csvResponse.headers()['content-type']).toContain('text/csv');
    const csvText = await csvResponse.text();
    expect(csvText).toContain('IntakeID,ShelterID');

    // 8. Verify direct GeoJSON API response
    const geojsonResponse = await page.request.get(`${WEB_FRONTEND_URL}/api/v1/shelters/analytics/export.geojson`);
    expect(geojsonResponse.status()).toBe(200);
    expect(geojsonResponse.headers()['content-type']).toContain('application/geo+json');
    const geojsonBody = await geojsonResponse.json();
    expect(geojsonBody.type).toBe('FeatureCollection');
  });
});
```

- [ ] **Step 2: Run Playwright Journey Test**

Run: `cd tests/playwright && npx playwright test e2e/shelter-analytics-journey.spec.ts`
Expected: PASS.

- [ ] **Step 3: Run Full Playwright Test Suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 100% PASS across all spec files.

- [ ] **Step 4: Run Full Repository Verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS (`go vet`, `golangci-lint`, OpenTofu check, `yamllint`, Go race tests).

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/shelter-analytics-journey.spec.ts
git commit -m "test(e2e): add end-to-end user journey tests for shelter analytics and reconciliation exports"
```
