# Milestone 11.1: Off-Grid Peer-to-Peer Mesh Sync & Wilderness Search Coordination Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build an off-grid peer-to-peer data synchronization engine enabling search squads and disaster relief volunteers to coordinate sectors, GPS breadcrumbs, sightings, and evacuation rosters in disconnected wilderness environments using browser-to-browser WebRTC DataChannels, dual signaling (local Wi-Fi SSE hub + optical camera QR handshake), monotonic CRDT conflict resolution, and automatic WAN cloud reconciliation.

**Architecture:** Peer devices establish direct WebRTC DataChannels in a distributed mesh. In-memory and IndexedDB state is governed by a monotonic Conflict-Free Replicated Data Type (CRDT) engine with Lamport clocks ensuring mathematical convergence across network partitions. Dual signaling allows pairing via local Wi-Fi/hotspot HTTP/SSE endpoints or air-gapped camera QR code SDP exchanges. On WAN reconnect, a background uplink worker flushes mutations to `/api/v1/mesh/uplink-sync` for cloud `StateStore` reconciliation and SSE real-time broadcast.

**Tech Stack:** Go 1.25+ (`export GOTOOLCHAIN=go1.26.5`), pure standard library `net/http` and `encoding/json`, WebRTC `RTCPeerConnection` and `RTCDataChannel`, HTML5 Canvas/MediaDevices QR scanning, pure Go `pkg/qrcode`, Leaflet 1.9.4 overlays, IndexedDB.

**Spec:** `docs/superpowers/specs/2026-09-22-p2p-mesh-sync-wilderness-coordination-design.md`

## Global Constraints
- Toolchain: `export GOTOOLCHAIN=go1.26.5`.
- Zero external CGO dependencies; pure Go standard library.
- Strict CSP compliance: Zero inline `<script>`, zero `eval()`.
- WCAG AAA contrast (>7:1) across both dark and light modes.
- Monotonic state progression: `UNCLAIMED` (0) $\to$ `CLAIMED` (1) $\to$ `SEARCHING` (2) $\to$ `CLEARED` (3). Lower ranks can never overwrite higher ranks.
- Pre-push verification gate: `export GOTOOLCHAIN=go1.26.5 && make verify` must pass cleanly.
- Web-first assertions for Playwright test suite (100% passing across all 124 existing + new journey test).

---

### Task 1: Domain Models, Store Constants & Pure Go Monotonic CRDT Engine

**Files:**
- Create: `pkg/domain/mesh.go`
- Create: `pkg/mesh/crdt.go`
- Test: `pkg/mesh/crdt_test.go`

**Interfaces:**
- Consumes: `pkg/domain/location.go` (`LocationPoint`), `pkg/domain/pet.go`
- Produces:
  - `domain.MeshNode`, `domain.MeshRole`, `domain.MeshMessageType`, `domain.MeshMessage`
  - `domain.SectorRank`, `domain.MeshSectorDelta`, `domain.MeshBreadcrumbDelta`, `domain.MeshSightingDelta`, `domain.MeshEvacuationDelta`, `domain.MeshSOSAlert`, `domain.MeshBatchDelta`, `domain.MeshUplinkSyncResponse`
  - `mesh.NewCRDTEngine(nodeID string) *CRDTEngine`
  - `(*CRDTEngine) MergeSector(delta domain.MeshSectorDelta) (bool, error)`
  - `(*CRDTEngine) MergeBreadcrumb(delta domain.MeshBreadcrumbDelta) (bool, error)`
  - `(*CRDTEngine) MergeSighting(delta domain.MeshSightingDelta) (bool, error)`
  - `(*CRDTEngine) MergeEvacuation(delta domain.MeshEvacuationDelta) (bool, error)`
  - `(*CRDTEngine) MergeSOSAlert(alert domain.MeshSOSAlert) (bool, error)`
  - `(*CRDTEngine) GenerateDigest() domain.MeshBatchDelta`
  - `(*CRDTEngine) ApplyBatch(batch domain.MeshBatchDelta) int`

- [ ] **Step 1: Write failing CRDT engine tests**

Create `pkg/mesh/crdt_test.go` with tests asserting:
1. Monotonic sector rank progression: `CLAIMED` cannot overwrite `CLEARED`, but `CLEARED` overwrites `CLAIMED`.
2. Equal rank tie-breaking via Lamport clock, then lexicographical `NodeID`.
3. Monotonic append-only breadcrumbs deduplicated by `(volunteerId, seq)`.
4. Add-Wins sightings deduplicated by `sightingId` with LWW Lamport clocks.
5. Partition simulation: 3 simulated nodes diverge and converge to identical state upon mutual `ApplyBatch`.

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/mesh`
Expected: FAIL with package not found or undefined types.

- [ ] **Step 3: Implement domain models and monotonic CRDT engine**

1. Create `pkg/domain/mesh.go` containing domain types, constants, and JSON tags as specified in Section 3 of the design specification.
2. Create `pkg/mesh/crdt.go`:
   - Enforce monotonic ranks: `SectorRankUnclaimed (0) < SectorRankClaimed (1) < SectorRankSearching (2) < SectorRankCleared (3)`.
   - Implement thread-safe mutex-protected collections for sectors, breadcrumbs, sightings, evacuation manifests, and alerts.
   - Implement Lamport clock tracking and vector digest generation.

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./pkg/mesh`
Expected: PASS (all CRDT tests pass including partition simulation under race detector).

- [ ] **Step 5: Commit**

```bash
git add pkg/domain/mesh.go pkg/mesh/crdt.go pkg/mesh/crdt_test.go
git commit -m "feat(mesh): domain models and pure Go monotonic CRDT state engine"
```

---

### Task 2: Backend Signaling Hub & Cloud Uplink Reconciliation Handlers

**Files:**
- Create: `pkg/mesh/signaling.go`
- Create: `pkg/mesh/uplink.go`
- Create: `internal/app/webfrontend/mesh_handlers.go`
- Modify: `internal/app/webfrontend/webfrontend.go` (register routes)
- Test: `internal/app/webfrontend/mesh_handlers_test.go`

**Interfaces:**
- Consumes: `pkg/domain`, `pkg/mesh/crdt.go`, `pkg/store/store.go`
- Produces:
  - Route `GET /api/v1/mesh/signal/events`: SSE stream for peer signaling envelopes (Offer, Answer, ICECandidate, PeerJoined, PeerLeft).
  - Route `POST /api/v1/mesh/signal/message`: HTTP endpoint to publish signaling frames to active peers in `searchPartyId`.
  - Route `POST /api/v1/mesh/uplink-sync`: Transactionally reconciles field `MeshBatchDelta` into cloud `StateStore`, updates sectors, breadcrumbs, sightings, and evacuation rosters, emitting SSE updates to `pkg/reunion`.
  - Route `GET /api/v1/mesh/qr-signaling`: Pure Go SVG QR code generator endpoint for compressed SDP envelopes.

- [ ] **Step 1: Write failing handler tests**

Create `internal/app/webfrontend/mesh_handlers_test.go` with tests:
1. `TestMeshSignaling_PeerJoinAndRelay`: Two clients subscribe to SSE stream, one posts offer, verifying recipient receives framed event.
2. `TestMeshUplinkSync_Success`: Post `MeshBatchDelta` with sectors and breadcrumbs; assert HTTP 200, record counts, and store update.
3. `TestMeshUplinkSync_MonotonicConflictResolution`: Post `CLAIMED` sector when server has `CLEARED`; assert server retains `CLEARED` and returns server latest deltas.
4. `TestMeshQRSignaling_SVGGeneration`: Request `/api/v1/mesh/qr-signaling?data=...`; assert HTTP 200, `image/svg+xml` content type.

- [ ] **Step 2: Run test to verify it fails**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./internal/app/webfrontend -run TestMesh`
Expected: FAIL with undefined handlers/routes.

- [ ] **Step 3: Implement signaling hub and uplink handlers**

1. In `pkg/mesh/signaling.go`:
   - Implement `SignalingHub` with thread-safe client channels mapped by `searchPartyId`.
   - Broadcast events: `PEER_JOINED`, `PEER_LEFT`, `OFFER`, `ANSWER`, `ICE_CANDIDATE`.
2. In `pkg/mesh/uplink.go`:
   - Implement `ReconcileUplinkBatch(ctx context.Context, store store.StateStore, batch domain.MeshBatchDelta) (*domain.MeshUplinkSyncResponse, error)`:
     - Compare sector rank: apply higher rank; tie-break equal ranks with Lamport clock.
     - Store breadcrumbs into search party trails.
     - Store sightings with media metadata into sighting store.
     - Dispatch real-time reunion events if active.
3. In `internal/app/webfrontend/mesh_handlers.go`:
   - Implement HTTP handlers for `/api/v1/mesh/signal/events`, `/api/v1/mesh/signal/message`, `/api/v1/mesh/uplink-sync`, and `/api/v1/mesh/qr-signaling`.
4. In `internal/app/webfrontend/webfrontend.go`:
   - Register the mesh routes with authentication middleware and CORS support.

- [ ] **Step 4: Run test to verify it passes**

Run: `export GOTOOLCHAIN=go1.26.5 && go test -race -v ./internal/app/webfrontend -run TestMesh`
Expected: PASS (all signaling, uplink, and QR tests pass under `-race`).

- [ ] **Step 5: Commit**

```bash
git add pkg/mesh/signaling.go pkg/mesh/uplink.go internal/app/webfrontend/mesh_handlers.go internal/app/webfrontend/webfrontend.go internal/app/webfrontend/mesh_handlers_test.go
git commit -m "feat(mesh): local signaling hub and cloud uplink reconciliation handlers"
```

---

### Task 3: Client WebRTC DataChannel Controller & Offline IndexedDB Engine

**Files:**
- Create: `internal/app/webfrontend/static/js/mesh-sync.js`
- Modify: `internal/app/webfrontend/templates/layout.html` (script tag inclusion before search-party.js)

**Interfaces:**
- Consumes: Browser WebRTC API (`RTCPeerConnection`, `RTCDataChannel`), IndexedDB API, `/api/v1/mesh/signal/*`, `/api/v1/mesh/uplink-sync`
- Produces:
  - Global `window.petSpotRMesh`:
    - `init(options)`
    - `joinSearchParty(partyId, volunteerInfo)`
    - `claimSector(petId, sectorId, state)`
    - `recordBreadcrumb(petId, lat, lng)`
    - `broadcastSOS(message)`
    - `generateOpticalOffer()`
    - `consumeOpticalAnswer(answerStr)`
    - `syncUplink()`
    - Custom DOM Events: `mesh:peer-joined`, `mesh:peer-left`, `mesh:sector-updated`, `mesh:breadcrumb-received`, `mesh:sos-alert`, `mesh:uplink-complete`

- [ ] **Step 1: Write client mesh controller unit/integration tests**

Create tests in `tests/playwright/unit/mesh-sync.spec.ts` (or headless node tests) verifying:
1. `MeshSyncController` initialization and IndexedDB creation (`petspotr-mesh-store`).
2. Sector monotonic ranking logic in JavaScript: `CLEARED` (3) retains precedence over incoming `CLAIMED` (1).
3. Outbox queueing when offline and automatic drain on simulated `online` event.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tests/playwright && npx playwright test tests/playwright/unit/mesh-sync.spec.ts`
Expected: FAIL with missing script/module.

- [ ] **Step 3: Implement client WebRTC mesh controller**

Create `internal/app/webfrontend/static/js/mesh-sync.js`:
1. IndexedDB wrapper for `sectors`, `breadcrumbs`, `sightings`, `outbox`.
2. WebRTC connection manager with dual signaling:
   - Mode 1: Connects to `/api/v1/mesh/signal/events` SSE and posts signaling envelopes.
   - Mode 2: Optical QR Code compressor/decompressor for SDP offers and answers.
3. DataChannel listener: decodes `GOSSIP_DIGEST`, `DELTA_PAYLOAD`, and `SOS_ALERT`.
4. Event dispatcher publishing standard DOM CustomEvents (`mesh:sector-updated`, `mesh:breadcrumb-received`, `mesh:sos-alert`).
5. Uplink auto-sync worker on `window.addEventListener('online')`.
6. Include `mesh-sync.js` in `layout.html` with proper `defer`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd tests/playwright && npx playwright test tests/playwright/unit/mesh-sync.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/static/js/mesh-sync.js internal/app/webfrontend/templates/layout.html tests/playwright/unit/mesh-sync.spec.ts
git commit -m "feat(mesh): client WebRTC DataChannel controller and IndexedDB outbox engine"
```

---

### Task 4: Wilderness Coordination HUD, Mesh Modal & Map Integration

**Files:**
- Create: `internal/app/webfrontend/templates/mesh_modal.html`
- Modify: `internal/app/webfrontend/templates/searchparty_modal.html` (include mesh indicator and modal trigger)
- Modify: `internal/app/webfrontend/static/js/search-party.js` (wire mesh events to Leaflet layers and sector UI)
- Modify: `internal/app/webfrontend/static/css/styles.css` (high-contrast HUD, peer cards, SOS banner)

**Interfaces:**
- Consumes: `window.petSpotRMesh` events, Leaflet map instance in `search-party.js`
- Produces:
  - `#mesh-status-indicator`: Interactive header pill displaying live peer count with pulsating indicator.
  - `#mesh-modal`: Dialog containing Peer Roster, Pairing Toolbar (`#btn-mesh-auto-pair`, `#btn-mesh-show-qr`, `#btn-mesh-scan-qr`), and Emergency SOS Beacon (`#btn-mesh-sos`).
  - Dynamic Leaflet updates: Sectors claimed via mesh instantly change color and badge; peer breadcrumbs render on live trail polyline.
  - `#mesh-sos-banner`: Full-width assertive emergency banner for distress signals.

- [ ] **Step 1: Write integration tests for UI elements**

Create UI component test asserting:
1. `#mesh-status-indicator` renders in Search Party modal with WCAG AAA contrast (>7.5:1).
2. Clicking indicator opens `#mesh-modal` and traps keyboard focus.
3. Simulating `mesh:sector-updated` updates sector card status badge to "CLEARED (Mesh Verified)".
4. Simulating `mesh:sos-alert` displays `#mesh-sos-banner` with coordinates.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd tests/playwright && npx playwright test tests/playwright/e2e/p2p-mesh-sync-journey.spec.ts`
Expected: FAIL with missing modal elements.

- [ ] **Step 3: Implement modal template, styles, and search-party.js event wiring**

1. Create `internal/app/webfrontend/templates/mesh_modal.html` with:
   - Header with `#mesh-modal-title`, close button, and live status pill.
   - Peer roster section `#mesh-peer-roster`.
   - Pairing toolbar with `#btn-mesh-auto-pair`, `#btn-mesh-show-qr`, `#btn-mesh-scan-qr`.
   - Viewfinder / QR container `#mesh-qr-view`.
   - Distress section with `#btn-mesh-sos` button.
2. In `searchparty_modal.html`:
   - Mount `#mesh-status-indicator` in toolbar.
   - Include `mesh_modal.html`.
3. In `styles.css`:
   - High-contrast styles for `.mesh-pill`, `.mesh-peer-card`, `.mesh-sos-banner`, and pulsating status dot.
   - Verified WCAG AAA contrast ratio > 7.5:1 in light and dark mode.
4. In `search-party.js`:
   - Listen for `mesh:sector-updated`: update sector card DOM and Leaflet polygon colors.
   - Listen for `mesh:breadcrumb-received`: append lat/lng to volunteer trail layer.
   - Listen for `mesh:sos-alert`: render distress notification banner.
   - When volunteer claims a sector, call `petSpotRMesh.claimSector(...)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd tests/playwright && npx playwright test tests/playwright/e2e/p2p-mesh-sync-journey.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/templates/mesh_modal.html internal/app/webfrontend/templates/searchparty_modal.html internal/app/webfrontend/static/js/search-party.js internal/app/webfrontend/static/css/styles.css
git commit -m "feat(mesh): wilderness coordination HUD, mesh network modal, and Leaflet map integration"
```

---

### Task 5: Automated Playwright E2E User Journey & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/p2p-mesh-sync-journey.spec.ts`

**Interfaces:**
- Consumes: Complete PetSpotR application stack running via Playwright `webServer`

- [ ] **Step 1: Implement full E2E user journey test**

Create `tests/playwright/e2e/p2p-mesh-sync-journey.spec.ts`:
1. Setup: Create lost pet report and launch Search Party modal.
2. Step 1: Verify `#mesh-status-indicator` is visible and accessible.
3. Step 2: Open `#mesh-modal`, verify peer roster, optical QR code offer display, and pairing buttons.
4. Step 3: Trigger simulated peer mesh sector claim; assert sector status badge advances to `CLEARED` and polygon style updates on Leaflet map in real time.
5. Step 4: Trigger simulated volunteer GPS breadcrumb; assert breadcrumb trail renders.
6. Step 5: Click Tactical SOS button (`#btn-mesh-sos`); assert high-priority SOS emergency banner appears with coordinates.
7. Step 6: Post cloud uplink sync to `/api/v1/mesh/uplink-sync`; assert HTTP 200 and data persistence in backend store.

- [ ] **Step 2: Run new Playwright E2E journey test**

Run: `cd tests/playwright && npx playwright test tests/playwright/e2e/p2p-mesh-sync-journey.spec.ts`
Expected: PASS (all steps pass).

- [ ] **Step 3: Run full Playwright test suite**

Run: `cd tests/playwright && npx playwright test`
Expected: PASS (all 125 tests passing).

- [ ] **Step 4: Run full repository verification**

Run: `export GOTOOLCHAIN=go1.26.5 && make verify`
Expected: PASS (0 lint issues, OpenTofu valid, yamllint clean, Go tests with `-race -cover` 100% passing).

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/p2p-mesh-sync-journey.spec.ts
git commit -m "test(playwright): add E2E user journey tests for off-grid peer-to-peer mesh sync and wilderness search coordination"
```

---

### Task 6: Final Whole-Branch Review & Polishing Wave

**Files:**
- All touched files across Milestone 11.1

- [ ] **Step 1: Perform whole-branch code review with `pro` model subagent**
- [ ] **Step 2: Address any findings (accessibility, contrast, memory leaks, error handling)**
- [ ] **Step 3: Re-verify full test suite and `make verify`**
- [ ] **Step 4: Commit any review fixes**
