# Milestone 10.1: BLE Collar Beacon Scanning & Proximity Triangulation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Provide community search volunteers with passive and active BLE collar beacon scanning, real-time RSSI-based distance estimation, an intuitive Web Audio Geiger counter, multi-observation 2D multilateration, and one-tap sighting logging.

**Architecture:** A pure Go beacon mathematical engine (`pkg/beacon`) handles path-loss distance modeling and non-linear least-squares trilateration, paired with REST handlers and real-time SSE broadcast in `webfrontend`. A client-side Web Bluetooth scanner and Web Audio API oscillator (`pet-beacon-scanner.js`) power an interactive Radar HUD inside the search party modal with offline IndexedDB resilience.

**Tech Stack:** Go 1.26.5, Web Bluetooth API (`navigator.bluetooth`), Web Audio API (`AudioContext`), Leaflet 1.9.4, IndexedDB, Playwright, OpenTofu, Server-Sent Events (SSE).

**Spec:** [`docs/superpowers/specs/2026-09-20-ble-collar-beacon-triangulation-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-20-ble-collar-beacon-triangulation-design.md)

## Global Constraints
- Pure Go standard library for mathematical calculations in `pkg/beacon` (no CGO, no external math dependencies).
- Toolchain: `export GOTOOLCHAIN=go1.26.5`.
- Pre-push verification requirement: `make verify` must pass with 0 lint issues and 100% tests passing.
- Web Bluetooth scanner must gracefully degrade in non-BLE browsers or headless automated test environments using simulated test injection (`window.__mockBluetoothScanner`).
- Strict tab sequence and WCAG AAA compliance must be preserved on all modified templates.

---

### Task 1: Domain Models, Store Constants & Pure Go Beacon Engine (`pkg/beacon`)

**Files:**
- Create: `pkg/beacon/types.go`
- Create: `pkg/beacon/pathloss.go`
- Create: `pkg/beacon/trilateration.go`
- Create: `pkg/beacon/beacon_test.go`
- Modify: `pkg/domain/lost_report.go:101-125`
- Modify: `pkg/store/store.go:20-40`

**Interfaces:**
- Produces:
  - `pkg/domain.CollarBeaconConfig` struct
  - `pkg/domain.BeaconProtocol` enum
  - `pkg/store.BeaconPingsCollection = "beacon_pings"`
  - `pkg/beacon.BeaconPing` struct
  - `pkg/beacon.ProximityZone` enum (`immediate`, `near`, `far`, `out_of_range`)
  - `pkg/beacon.TriangulationResult` struct
  - `pkg/beacon.EstimateDistance(rssi int, txPower1m int, pathLossExponent float64) float64`
  - `pkg/beacon.DetermineProximity(distance float64, rssi int) ProximityZone`
  - `pkg/beacon.TriangulateBeacon(pings []BeaconPing) TriangulationResult`

- [ ] **Step 1: Write the failing tests in `pkg/beacon/beacon_test.go`**

```go
package beacon_test

import (
	"math"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/beacon"
	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestEstimateDistance(t *testing.T) {
	// At calibrated txPower (-59 dBm), distance at 1m should equal 1.0m
	dist1m := beacon.EstimateDistance(-59, -59, 2.5)
	if math.Abs(dist1m-1.0) > 0.05 {
		t.Errorf("expected ~1.0m, got %.2fm", dist1m)
	}

	// At -74 dBm (15 dB drop, ~2.5 exponent), distance should be approx 3.98m
	distNear := beacon.EstimateDistance(-74, -59, 2.5)
	if distNear < 3.0 || distNear > 5.0 {
		t.Errorf("expected between 3m and 5m, got %.2fm", distNear)
	}

	// Proximity classifications
	if beacon.DetermineProximity(0.8, -55) != beacon.ProximityImmediate {
		t.Errorf("expected immediate proximity for 0.8m")
	}
	if beacon.DetermineProximity(3.2, -68) != beacon.ProximityNear {
		t.Errorf("expected near proximity for 3.2m")
	}
	if beacon.DetermineProximity(14.0, -82) != beacon.ProximityFar {
		t.Errorf("expected far proximity for 14.0m")
	}
	if beacon.DetermineProximity(45.0, -95) != beacon.ProximityOutOfRange {
		t.Errorf("expected out of range for 45.0m")
	}
}

func TestTriangulateBeacon_SinglePing(t *testing.T) {
	now := time.Now().UTC()
	pings := []beacon.BeaconPing{
		{
			PingID:         "p1",
			PetID:          "pet-123",
			VolunteerAlias: "Volunteer Alpha",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321},
			RSSI:           -68,
			TxPower1m:      -59,
			DistanceMeters: 3.2,
			RecordedAt:     now,
		},
	}

	res := beacon.TriangulateBeacon(pings)
	if res.ObservationCount != 1 {
		t.Fatalf("expected 1 observation, got %d", res.ObservationCount)
	}
	if math.Abs(res.EstimatedCoordinates.Latitude-47.6062) > 1e-5 {
		t.Errorf("expected latitude 47.6062, got %f", res.EstimatedCoordinates.Latitude)
	}
	if res.AccuracyRadiusMeters != 3.2 {
		t.Errorf("expected accuracy radius 3.2, got %f", res.AccuracyRadiusMeters)
	}
}

func TestTriangulateBeacon_MultiPing_LeastSquares(t *testing.T) {
	now := time.Now().UTC()
	pings := []beacon.BeaconPing{
		{
			PingID:         "p1",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6000, Longitude: -122.3300},
			RSSI:           -65,
			TxPower1m:      -59,
			DistanceMeters: 5.0,
			RecordedAt:     now,
		},
		{
			PingID:         "p2",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.6001, Longitude: -122.3300},
			RSSI:           -62,
			TxPower1m:      -59,
			DistanceMeters: 3.0,
			RecordedAt:     now.Add(10 * time.Second),
		},
		{
			PingID:         "p3",
			PetID:          "pet-123",
			ObserverCoords: domain.LocationPoint{Latitude: 47.60005, Longitude: -122.3301},
			RSSI:           -60,
			TxPower1m:      -59,
			DistanceMeters: 2.0,
			RecordedAt:     now.Add(20 * time.Second),
		},
	}

	res := beacon.TriangulateBeacon(pings)
	if res.ObservationCount != 3 {
		t.Fatalf("expected 3 observations, got %d", res.ObservationCount)
	}
	if res.ConfidenceScore < 0.7 {
		t.Errorf("expected high confidence score for 3 pings, got %f", res.ConfidenceScore)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test ./pkg/beacon/...`
Expected: FAIL ("cannot find package" or undefined types)

- [ ] **Step 3: Implement domain types, store constant, and `pkg/beacon` package**

Extend `pkg/domain/lost_report.go` with `CollarBeaconConfig` and `BeaconProtocol`.
Add `BeaconPingsCollection = "beacon_pings"` in `pkg/store/store.go`.
Create `pkg/beacon/types.go`, `pkg/beacon/pathloss.go`, and `pkg/beacon/trilateration.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/beacon/...`
Expected: PASS

- [ ] **Step 5: Commit changes**

```bash
git add pkg/domain/lost_report.go pkg/store/store.go pkg/beacon/
git commit -m "feat(beacon): add domain models, store constants, and pure Go path loss triangulation engine"
```

---

### Task 2: Web Frontend REST Handlers & Real-Time SSE Stream Integration

**Files:**
- Create: `internal/app/webfrontend/beacon.go`
- Create: `internal/app/webfrontend/beacon_test.go`
- Modify: `internal/app/webfrontend/server.go:120-170` (register routes)
- Modify: `internal/app/webfrontend/searchparty.go` (include beacon config in search party data)
- Modify: `pkg/domain/reunion_stream.go` (add beacon ping event payload envelope)

**Interfaces:**
- Consumes:
  - `pkg/beacon.BeaconPing`, `pkg/beacon.TriangulateBeacon`
  - `store.BeaconPingsCollection`
  - `reunionHub.Broadcast`
- Produces:
  - `POST /api/v1/search-parties/{petId}/beacon-pings`
  - `GET /api/v1/search-parties/{petId}/beacon-triangulation`
  - `domain.ReunionEventBeaconPing = "beacon_ping"`

- [ ] **Step 1: Write failing tests in `internal/app/webfrontend/beacon_test.go`**

```go
package webfrontend_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/scottdensmore/petspotr/internal/app/webfrontend"
	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/store"
)

func TestBeaconPings_RESTEndpoints(t *testing.T) {
	memStore := store.NewMemoryStateStore()
	server := webfrontend.NewTestServer(t, memStore)

	// Ingest ping
	payload := map[string]interface{}{
		"volunteerAlias": "Volunteer Alpha",
		"observerCoords": map[string]float64{"latitude": 47.6062, "longitude": -122.3321},
		"rssi":           -64,
		"txPower1m":      -59,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/search-parties/pet-test-1/beacon-pings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
	}

	// Fetch triangulation
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/search-parties/pet-test-1/beacon-triangulation", nil)
	getW := httptest.NewRecorder()
	server.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", getW.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test ./internal/app/webfrontend -run TestBeaconPings`
Expected: FAIL (404 Not Found)

- [ ] **Step 3: Implement REST handlers in `internal/app/webfrontend/beacon.go` and mount routes in `server.go`**

Implement `handleBeaconPingSubmission` and `handleGetBeaconTriangulation`. In `handleBeaconPingSubmission`, save the ping, retrieve pings in the last 15 minutes, compute `TriangulateBeacon`, broadcast via `reunionHub.Broadcast`, and return HTTP 201 with the result.

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./internal/app/webfrontend -run TestBeaconPings`
Expected: PASS

- [ ] **Step 5: Commit changes**

```bash
git add internal/app/webfrontend/beacon* internal/app/webfrontend/server.go pkg/domain/reunion_stream.go
git commit -m "feat(webfrontend): add beacon ping ingestion and triangulation REST endpoints with SSE broadcast"
```

---

### Task 3: Client Web Bluetooth Scanner, Web Audio Geiger Counter & Styles

**Files:**
- Create: `internal/app/webfrontend/static/js/pet-beacon-scanner.js`
- Modify: `internal/app/webfrontend/static/css/styles.css`
- Modify: `internal/app/webfrontend/templates/searchparty_modal.html`

**Interfaces:**
- Produces:
  - `window.PetBeaconScanner` class with `startScan()`, `stopScan()`, `toggleAudio()`, `injectMockPing()`
  - Audio Geiger synthesizer using Web Audio API
  - CSS styling for `#beacon-radar-hud`, `.beacon-signal-meter`, `.beacon-pulse-indicator`, and `#btn-log-beacon-sighting`

- [ ] **Step 1: Create client scanner script `pet-beacon-scanner.js`**

Implement:
- `PetBeaconScanner` initialization with `petId`, `beaconConfig`, and callbacks for `onPing` and `onTriangulationUpdate`.
- Web Bluetooth listener (`navigator.bluetooth.requestLEScan` or `requestDevice`) with fallback to `window.__mockBluetoothScanner` test hook.
- Exponential smoothing filter ($\alpha = 0.35$).
- Web Audio `AudioContext` synthesizer:
  - Generates periodic beep pulses where pitch ($220\text{Hz} - 880\text{Hz}$) and repetition interval ($1.0\text{s} - 0.1\text{s}$) scale with proximity.
  - Audio mute/unmute toggle persisted in `localStorage`.

- [ ] **Step 2: Add glassmorphic radar styles to `styles.css`**

Add styles for:
- `.beacon-radar-panel`: dark glassmorphic container in the sector briefing.
- `.signal-strength-bar`: 4-segment visual level indicator with color thresholds (`out-of-range`: gray, `far`: amber, `near`: green, `immediate`: bright pulse).
- `.btn-beacon-action`: accessible button styling for `#btn-start-beacon-scan`, `#btn-toggle-beacon-audio`, `#btn-log-beacon-sighting`.
- Focus outlines and ARIA high-contrast WCAG AAA compliance.

- [ ] **Step 3: Update `searchparty_modal.html` with Radar HUD markup**

Add the `#beacon-scanner-container` markup inside the sector details panel:
- Signal strength readout, distance badge (`#beacon-distance-display`), audio toggle (`#btn-toggle-beacon-audio`), and one-tap log button (`#btn-log-beacon-sighting`).

- [ ] **Step 4: Verify template markup and styles via Go unit tests**

Run: `export GOTOOLCHAIN=go1.26.5 && go test ./internal/app/webfrontend/...`
Expected: PASS

- [ ] **Step 5: Commit changes**

```bash
git add internal/app/webfrontend/static/js/pet-beacon-scanner.js internal/app/webfrontend/static/css/styles.css internal/app/webfrontend/templates/searchparty_modal.html
git commit -m "feat(ui): add Web Bluetooth scanner, Web Audio Geiger counter, and Radar HUD styling"
```

---

### Task 4: Radar HUD Integration, Leaflet Beacon Pulse & Sighting Modal Auto-Fill

**Files:**
- Modify: `internal/app/webfrontend/static/js/pet-search-party.js`
- Modify: `internal/app/webfrontend/static/js/outbox-sync.js`

**Interfaces:**
- Consumes:
  - `PetBeaconScanner`
  - Leaflet Map (`window.L.circle`, `window.L.marker`)
  - IndexedDB `petspotr_beacon_outbox`
- Produces:
  - Interactive Leaflet beacon marker `.beacon-ping-marker` with confidence circle
  - Automated sighting pre-fill when clicking `#btn-log-beacon-sighting`
  - Offline buffering in IndexedDB and automatic sync upon `online` event

- [ ] **Step 1: Wire `PetBeaconScanner` into `pet-search-party.js`**

In `pet-search-party.js`:
- Check if `activeSearchParty.lostPet.collarBeaconConfig` exists.
- If present, instantiate `PetBeaconScanner` and render Radar HUD controls.
- On ping arrival:
  - Update `#beacon-rssi-meter` and `#beacon-distance-display`.
  - Draw/update `.beacon-ping-marker` on Leaflet map with pulsing halo.
  - If proximity is `near` or `immediate`, enable `#btn-log-beacon-sighting`.
- Wire `#btn-log-beacon-sighting` to open the sighting modal with pre-populated coordinates and description `"Collar beacon detected nearby (~3.2m, RSSI: -64 dBm)"`.

- [ ] **Step 2: Add offline IndexedDB buffering in `outbox-sync.js`**

Support `petspotr_beacon_outbox` store:
- Queue pings when `!navigator.onLine`.
- Flush queue via `POST /api/v1/search-parties/{petId}/beacon-pings` when `window` receives `online` event.

- [ ] **Step 3: Run full Go verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS

- [ ] **Step 4: Commit changes**

```bash
git add internal/app/webfrontend/static/js/pet-search-party.js internal/app/webfrontend/static/js/outbox-sync.js
git commit -m "feat(frontend): integrate beacon radar HUD, Leaflet map pulse overlay, and offline outbox sync"
```

---

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/beacon-triangulation-journey.spec.ts`

**Interfaces:**
- Consumes:
  - Web frontend running at `http://localhost:8082`
  - Window test hook `window.__mockBluetoothScanner`
- Produces:
  - Comprehensive Playwright test suite covering beacon activation, signal distance transitions, audio toggle, one-tap sighting creation, and offline recovery.

- [ ] **Step 1: Write `tests/playwright/e2e/beacon-triangulation-journey.spec.ts`**

Implement the 5 user journey tests:
- Test 1: Setup lost pet with `CollarBeaconConfig` and initiate search party via API.
- Test 2: Verify Radar HUD renders in sector briefing modal, start scan, and toggle audio synthesizer.
- Test 3: Inject simulated BLE pings at varying distances, verify distance updates and Leaflet beacon marker.
- Test 4: Click one-tap "Confirm & Log Sighting", verify coordinates are pre-populated in sighting modal, submit, and confirm sighting is persisted.
- Test 5: Simulate offline disconnect, buffer pings in IndexedDB, reconnect, and verify outbox auto-sync.

- [ ] **Step 2: Run targeted Playwright test**

Run: `cd tests/playwright && npx playwright test e2e/beacon-triangulation-journey.spec.ts`
Expected: PASS (all 5 journey tests green)

- [ ] **Step 3: Run full Playwright test suite**

Run: `cd tests/playwright && npx playwright test`
Expected: PASS (all 118+ tests passing)

- [ ] **Step 4: Run full repository verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS (0 lint issues, OpenTofu valid, yamllint valid, all unit & race tests passing)

- [ ] **Step 5: Commit changes**

```bash
git add tests/playwright/e2e/beacon-triangulation-journey.spec.ts
git commit -m "test(playwright): add E2E user journey tests for BLE collar beacon scanning and proximity triangulation"
```

---

### Task 6: Final Whole-Branch Code Review, PR Creation & Squash Merge

**Files:**
- Review all modified files across branch
- Update task tracking artifact

- [ ] **Step 1: Dispatch deep code review using `pro` model**
- [ ] **Step 2: Address any review findings and verify clean tests**
- [ ] **Step 3: Push branch and open Pull Request with `gh pr create`**
- [ ] **Step 4: Squash-merge PR into `main` with `gh pr merge --squash --delete-branch`**
- [ ] **Step 5: Pull latest `main` and update task progress artifact**
