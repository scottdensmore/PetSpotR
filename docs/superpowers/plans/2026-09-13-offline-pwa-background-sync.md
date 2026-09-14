# Offline PWA & Background Sync Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a robust, offline-first Progressive Web App (PWA) experience with Service Worker asset/tile/data caching, IndexedDB-backed outbox storage for multi-photo lost/found reports, and automatic background sync upon network reconnection.

**Architecture:** Layered Service Worker (`sw.js`) intercepts network requests applying Stale-While-Revalidate for app shell, Cache-First with LRU eviction for OSM map tiles, and Network-First with cache fallback for pet listings. Client outbox manager (`outbox-sync.js`) persists offline report drafts and photo `Blob`s in IndexedDB (`petspotr_offline_db`), synchronizing them via a dual-trigger pipeline (Service Worker Background Sync API + window `online` fallback) that requests pre-signed URLs, uploads image blobs, and submits the finalized reports to backend endpoints.

**Tech Stack:** Native JavaScript (ES2022+), Service Worker API, Cache Storage API, IndexedDB API, Background Sync API, Go 1.26.5 (`webfrontend`), Playwright E2E.

**Spec:** `docs/superpowers/specs/2026-09-13-offline-pwa-background-sync-design.md`

## Global Constraints

- Pinned toolchain: `export GOTOOLCHAIN=go1.26.5`
- Zero external client-side NPM dependencies; pure vanilla JavaScript with local browser APIs.
- Strict Content Security Policy (CSP): `script-src 'self'`, `connect-src 'self' https://storage.petspotr.io ...`.
- All background tasks and async operations must be fully verified via `make verify` and Playwright tests before committing.

---

### Task 1: PWA Web Manifest & Offline Fallback Page

**Files:**
- Create: `internal/app/webfrontend/static/manifest.webmanifest`
- Create: `internal/app/webfrontend/templates/offline.html`
- Modify: `internal/app/webfrontend/server.go:130-180`
- Test: `internal/app/webfrontend/server_test.go`

**Interfaces:**
- Consumes: Go standard library `http.ServeMux`, `embed.FS`.
- Produces: `GET /manifest.webmanifest` returning `application/manifest+json`, `GET /offline.html` returning HTML offline fallback.

- [ ] **Step 1: Write the failing tests in `server_test.go`**

```go
func TestManifestAndOfflinePage(t *testing.T) {
	t.Parallel()
	srv := NewServer()

	t.Run("GET /manifest.webmanifest", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		contentType := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/manifest+json") {
			t.Errorf("Content-Type = %q, want application/manifest+json", contentType)
		}
		var manifest map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
			t.Fatalf("invalid json manifest: %v", err)
		}
		if manifest["name"] != "PetSpotR — Lost & Found Pet Recovery" {
			t.Errorf("manifest name = %v, want PetSpotR — Lost & Found Pet Recovery", manifest["name"])
		}
	})

	t.Run("GET /offline.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/offline.html", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		contentType := rec.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", contentType)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "You're Offline") {
			t.Errorf("body does not contain 'You're Offline': %s", body)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v -run TestManifestAndOfflinePage ./internal/app/webfrontend/...`
Expected: FAIL with status 404.

- [ ] **Step 3: Create `manifest.webmanifest` and `offline.html`**

Create `internal/app/webfrontend/static/manifest.webmanifest`:
```json
{
  "name": "PetSpotR — Lost & Found Pet Recovery",
  "short_name": "PetSpotR",
  "description": "AI-powered lost and found pet recovery platform.",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#0f172a",
  "theme_color": "#4f46e5",
  "icons": [
    {
      "src": "/static/favicon.svg",
      "sizes": "any",
      "type": "image/svg+xml"
    }
  ]
}
```

Create `internal/app/webfrontend/templates/offline.html`:
```html
<!DOCTYPE html>
<html lang="en" data-theme="dark">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Offline — PetSpotR</title>
  <link rel="icon" type="image/svg+xml" href="/static/favicon.svg">
  <link rel="stylesheet" href="/static/css/styles.css">
</head>
<body>
  <div class="container offline-container">
    <div class="glass-card offline-card text-center">
      <div class="offline-icon" aria-hidden="true">📡</div>
      <h1 class="page-title">You're Offline</h1>
      <p class="offline-desc">
        It looks like you've lost internet connectivity. You can still browse previously cached pet listings and draft new reports that will automatically synchronize when you reconnect.
      </p>
      <div class="offline-actions">
        <a href="/pets" class="btn btn-primary">Browse Cached Directory</a>
        <a href="/" class="btn btn-secondary">Return Home</a>
      </div>
    </div>
  </div>
</body>
</html>
```

- [ ] **Step 4: Register HTTP routes in `internal/app/webfrontend/server.go`**

Add routes in `registerRoutes`:
```go
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		data, err := embeddedFiles.ReadFile("static/manifest.webmanifest")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(data)
	})

	mux.HandleFunc("/offline.html", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}
		data, err := embeddedFiles.ReadFile("templates/offline.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(data)
	})
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -v -run TestManifestAndOfflinePage ./internal/app/webfrontend/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/app/webfrontend/static/manifest.webmanifest internal/app/webfrontend/templates/offline.html internal/app/webfrontend/server.go internal/app/webfrontend/server_test.go
git commit -m "feat(pwa): add web app manifest and offline fallback page"
```

---

### Task 2: Service Worker Caching Strategies & Background Sync Event

**Files:**
- Modify: `internal/app/webfrontend/static/sw.js`

**Interfaces:**
- Consumes: Service Worker `fetch` event, `sync` event, Cache Storage API.
- Produces: Transparent offline caching for app shell, OSM raster map tiles (bounded LRU 250), API fallback, and background sync event dispatcher.

- [ ] **Step 1: Update `sw.js` with Cache Management and Fetch Strategies**

Update `internal/app/webfrontend/static/sw.js`:
- Define cache constants:
  - `SHELL_CACHE = 'petspotr-shell-v1'`
  - `DATA_CACHE = 'petspotr-data-v1'`
  - `TILE_CACHE = 'petspotr-tiles-v1'`
  - `MAX_TILE_CACHE = 250`
- Precache core assets in `install`:
  - `/`, `/offline.html`, `/manifest.webmanifest`, `/static/css/styles.css`, `/static/favicon.svg`, `/static/js/outbox-sync.js`, `/static/js/lost-report.js`, `/static/js/found-report.js`, `/static/js/pet-directory.js`, `/static/js/match-dashboard.js`, `/static/js/push-notifier.js`, `/static/vendor/leaflet/leaflet.js`, `/static/vendor/leaflet/leaflet.css`, `/static/vendor/leaflet/images/marker-icon.png`, `/static/vendor/leaflet/images/marker-shadow.png`.
- Implement LRU tile pruning helper:
  ```javascript
  async function pruneTileCache(cache, maxEntries) {
    const keys = await cache.keys();
    if (keys.length > maxEntries) {
      for (let i = 0; i < keys.length - maxEntries; i++) {
        await cache.delete(keys[i]);
      }
    }
  }
  ```
- Implement `fetch` listener:
  1. **OSM Tiles (`*.tile.openstreetmap.org`)**: Cache-First. Return cached tile if present; otherwise fetch, put in `TILE_CACHE`, and prune to 250 entries.
  2. **Directory API (`/api/v1/lost-pets`, `/api/v1/found-pets`)**: Network-First with Cache Fallback. Fetch with 3s timeout; if successful, clone into `DATA_CACHE`. If failed/offline, return cached response with header `X-PetSpotR-Offline: true`.
  3. **Static Assets (`/static/*`, `/manifest.webmanifest`)**: Stale-While-Revalidate. Return cache immediately, revalidate in background.
  4. **HTML Navigations (`mode === 'navigate'`)**: Network-First with fallback to cached shell or `/offline.html`.
- Implement `sync` listener:
  ```javascript
  self.addEventListener('sync', (event) => {
    if (event.tag === 'petspotr-outbox-sync') {
      event.waitUntil(notifyClientsToSync());
    }
  });

  async function notifyClientsToSync() {
    const clientList = await clients.matchAll({ type: 'window', includeUncontrolled: true });
    if (clientList.length > 0) {
      for (const client of clientList) {
        client.postMessage({ type: 'PETSPOTR_TRIGGER_SYNC' });
      }
    }
  }
  ```

- [ ] **Step 2: Verify Service Worker script syntax via node check**

Run: `node -c internal/app/webfrontend/static/sw.js`
Expected: clean exit (0 syntax errors).

- [ ] **Step 3: Commit**

```bash
git add internal/app/webfrontend/static/sw.js
git commit -m "feat(pwa): implement layered service worker caching and background sync handler"
```

---

### Task 3: Client IndexedDB Outbox Engine (`outbox-sync.js`)

**Files:**
- Create: `internal/app/webfrontend/static/js/outbox-sync.js`
- Test: `tests/playwright/e2e/offline-pwa-journey.spec.ts`

**Interfaces:**
- Consumes: IndexedDB API, Fetch API, Service Worker Message API.
- Produces: `window.PetSpotROutbox` global module exposing:
  - `enqueueReport({ type, payload, photos }) => Promise<string>`
  - `getPendingReports() => Promise<Array<OutboxReportRecord>>`
  - `syncAll() => Promise<{ synced: number, failed: number }>`
  - `getQueueCount() => Promise<number>`
  - Event triggers for UI status updates.

- [ ] **Step 1: Write `outbox-sync.js`**

Implement `internal/app/webfrontend/static/js/outbox-sync.js`:
- Open/upgrade IndexedDB `petspotr_offline_db` (version 1) creating store `outbox_reports` with keyPath `id`, index `by_status`, and index `by_created`.
- Implement `enqueueReport({ type, payload, photos })`:
  - Assigns unique `id = crypto.randomUUID()`.
  - Serializes `photos` array containing raw `Blob`s and semantic tags (`primary`, `face`, `coat`, `collar`).
  - Saves record with `status: 'pending'`, `attempts: 0`, `createdAt: new Date().toISOString()`.
  - Triggers custom event `petspotr:outbox-updated` and requests Background Sync:
    ```javascript
    if ('serviceWorker' in navigator && 'SyncManager' in window) {
      navigator.serviceWorker.ready.then(reg => reg.sync.register('petspotr-outbox-sync')).catch(() => {});
    }
    ```
- Implement `syncAll()`:
  - Finds all records with `status === 'pending'`.
  - For each record:
    1. Mark `status = 'syncing'`.
    2. For each photo:
       - `POST /api/v1/uploads/presigned-url` with `{ purpose: type === 'lost' ? 'lost-pet' : 'found-pet', fileName: photo.fileName, contentType: photo.contentType }`.
       - `PUT` photo `Blob` directly to `uploadUrl`.
       - Record `objectName`.
    3. Assemble finalized report payload with `imageObject` and `images` array.
    4. `POST /api/v1/lost-pets` or `POST /api/v1/found-pets`.
    5. On HTTP 201: delete record from IndexedDB, dispatch event `petspotr:report-synced`, and show toast.
    6. On error: increment `attempts`, update `lastError`, mark `status = attempts >= 5 ? 'failed' : 'pending'`.
- Register dual sync trigger:
  - `window.addEventListener('online', () => PetSpotROutbox.syncAll())`.
  - `navigator.serviceWorker.addEventListener('message', (e) => { if (e.data?.type === 'PETSPOTR_TRIGGER_SYNC') PetSpotROutbox.syncAll(); })`.

- [ ] **Step 2: Validate JavaScript syntax**

Run: `node -c internal/app/webfrontend/static/js/outbox-sync.js`
Expected: clean exit (0 syntax errors).

- [ ] **Step 3: Commit**

```bash
git add internal/app/webfrontend/static/js/outbox-sync.js
git commit -m "feat(pwa): implement indexeddb outbox storage and orchestrated sync engine"
```

---

### Task 4: UI Indicators, Badges & Toast Notifications

**Files:**
- Modify: `internal/app/webfrontend/templates/layout.html:1-40`
- Modify: `internal/app/webfrontend/templates/pets.html:30-60`
- Modify: `internal/app/webfrontend/static/css/styles.css:1100-1250`

**Interfaces:**
- Consumes: `petspotr:outbox-updated`, `petspotr:report-synced`, `petspotr:sync-started` events from `outbox-sync.js`.
- Produces: Real-time UI indicator in navbar (`#offline-indicator`), outbox count badge, and `#toast-container`.

- [ ] **Step 1: Update `layout.html`**

1. In `<head>`:
   ```html
   <link rel="manifest" href="/manifest.webmanifest">
   <meta name="theme-color" content="#4f46e5">
   ```
2. In `.nav-actions`:
   ```html
   <div id="offline-indicator" class="offline-pill" role="status" aria-live="polite" hidden>
     <span class="offline-dot" aria-hidden="true"></span>
     <span id="offline-status-text">Offline</span>
     <span id="outbox-count-badge" class="badge badge-warning" hidden>0</span>
   </div>
   ```
3. Before `</body>`:
   ```html
   <div id="toast-container" class="toast-container" role="region" aria-live="polite" aria-label="Notifications"></div>
   <script src="/static/js/outbox-sync.js" defer></script>
   ```

- [ ] **Step 2: Add Cached Directory Banner in `pets.html`**

In `pets.html` above `#pets-grid`:
```html
<div id="offline-directory-notice" class="offline-notice-banner" role="status" hidden>
  <span class="banner-icon" aria-hidden="true">ℹ️</span>
  <span>Viewing cached pet directory. Real-time updates and new reports will refresh when reconnected.</span>
</div>
```

- [ ] **Step 3: Add CSS Styles in `styles.css`**

Add styling for:
- `.offline-pill`: Glassmorphic pill badge with border, subtle shadow, font size 0.85rem.
- `.offline-dot`: 8px circle. Amber for offline (`#f59e0b`), blue pulsing for syncing (`#38bdf8`), green for complete (`#10b981`).
- `.toast-container` and `.toast-item`: Fixed position bottom-right (`bottom: 1.5rem; right: 1.5rem`), z-index 1000, glassmorphic backdrop filter, accessible contrast.
- `.offline-notice-banner`: Top notice bar styled with amber theme accent.

- [ ] **Step 4: Commit**

```bash
git add internal/app/webfrontend/templates/layout.html internal/app/webfrontend/templates/pets.html internal/app/webfrontend/static/css/styles.css
git commit -m "feat(ui): add connectivity status pill, outbox badge, and live toast container"
```

---

### Task 5: Multi-Photo Form Offline Interception

**Files:**
- Modify: `internal/app/webfrontend/static/js/lost-report.js`
- Modify: `internal/app/webfrontend/static/js/found-report.js`

**Interfaces:**
- Consumes: Form submission events, `PetSpotROutbox.enqueueReport`.
- Produces: Seamless offline report capture that intercepts submit when `!navigator.onLine` or fetch throws network error, queueing all staged photos.

- [ ] **Step 1: Update `lost-report.js`**

In form submit handler:
- If `!navigator.onLine`:
  ```javascript
  const stagedPhotos = stagedImages.map(img => ({
    fileName: img.file.name,
    contentType: img.file.type || 'image/jpeg',
    tag: img.tag || 'primary',
    blob: img.file
  }));
  await window.PetSpotROutbox.enqueueReport({
    type: 'lost',
    payload: {
      petName: form.petName.value,
      species: form.species.value,
      breed: form.breed.value,
      primaryColor: form.primaryColor.value,
      description: form.description.value,
      reporterEmail: form.reporterEmail.value,
      phone: form.phone.value,
      location: form.location.value,
      reportedAt: new Date().toISOString()
    },
    photos: stagedPhotos
  });
  window.PetSpotROutbox.showToast('Report saved offline. It will submit automatically when you reconnect.');
  form.reset();
  return;
  ```
- Also wrap the upload fetch call in `try...catch`: if error is network failure (`TypeError`), fallback to `PetSpotROutbox.enqueueReport`.

- [ ] **Step 2: Update `found-report.js`**

Apply the same offline check and network-fallback pattern for found pet reporting:
- Captures `petId: 'found-' + crypto.randomUUID()`, `species`, `breed`, `primaryColor`, `secondaryColor`, `distinctiveMarkings`, `custodyStatus`, and all staged photo `Blob`s.
- Enqueues via `PetSpotROutbox.enqueueReport({ type: 'found', payload, photos })`.
- Clears form and displays offline confirmation toast.

- [ ] **Step 3: Validate JavaScript syntax**

Run: `node -c internal/app/webfrontend/static/js/lost-report.js && node -c internal/app/webfrontend/static/js/found-report.js`
Expected: clean exit (0 syntax errors).

- [ ] **Step 4: Commit**

```bash
git add internal/app/webfrontend/static/js/lost-report.js internal/app/webfrontend/static/js/found-report.js
git commit -m "feat(pwa): integrate multi-photo offline reporting with outbox queue"
```

---

### Task 6: End-to-End User Journey Tests & Full Repository Verification

**Files:**
- Create: `tests/playwright/e2e/offline-pwa-journey.spec.ts`

**Interfaces:**
- Consumes: Playwright browser context, offline mode emulation (`context.setOffline`).
- Produces: Automated E2E verification of offline shell, IndexedDB queueing, and background sync replay.

- [ ] **Step 1: Write `tests/playwright/e2e/offline-pwa-journey.spec.ts`**

Implement 3 complete end-to-end journey tests:
1. `should cache app shell and render directory when offline`:
   - Navigates to `/pets` while online to warm Service Worker cache.
   - Calls `await context.setOffline(true)`.
   - Reloads `/pets` and navigates to `/`; asserts response is HTTP 200, DOM contains `.glass-nav` and `#offline-indicator` is visible with text `Offline`.
2. `should stage lost pet report with photos in IndexedDB when offline`:
   - While offline, visits `/report-lost`.
   - Fills out pet name ("Shadow"), species ("Dog"), breed ("Husky"), email ("owner@example.com").
   - Attaches a test photo.
   - Clicks submit; asserts offline toast notification appears: `"Report saved offline"`.
   - Asserts `#outbox-count-badge` displays `1`.
   - Inspects IndexedDB to verify record is stored in `outbox_reports` with photo `Blob`.
3. `should automatically sync queued report upon reconnection and persist on backend`:
   - Restores network connectivity (`await context.setOffline(false)`).
   - Waits for sync event to trigger and complete.
   - Asserts `#outbox-count-badge` becomes hidden / 0.
   - Fetches `/api/v1/lost-pets` and asserts report "Shadow" is now durably stored on the backend.

- [ ] **Step 2: Run Playwright E2E journey tests**

Run: `cd tests/playwright && npx playwright test e2e/offline-pwa-journey.spec.ts`
Expected: 3 passed.

- [ ] **Step 3: Run Full Playwright Test Suite**

Run: `cd tests/playwright && npx playwright test`
Expected: 100% passing (65+ tests).

- [ ] **Step 4: Run Full Repository Verification**

Run: `make verify`
Expected: All checks, linters, OpenTofu, yamllint, and Go tests passing.

- [ ] **Step 5: Commit**

```bash
git add tests/playwright/e2e/offline-pwa-journey.spec.ts
git commit -m "test(pwa): add comprehensive offline pwa and background sync user journey tests"
```
