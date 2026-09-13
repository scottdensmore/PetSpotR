# Milestone 5.1: Interactive Geospatial Directory & Mapping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver an accessible, secure, and interactive Leaflet geospatial map view on `/pets` with custom status-coded markers, radius proximity filtering, and zero external runtime API key dependencies.

**Architecture:** Vendor Leaflet v1.9.4 locally within embedded Go assets, update Content Security Policy for OpenStreetMap raster tiles, inject coordinates via a safe JSON data script tag in `templates/pets.html`, and provide client-side view toggling (`Grid` vs `Map`), proximity circles, and interactive popup cards.

**Tech Stack:** Go 1.26 (html/template, net/http, embed), Leaflet.js 1.9.4, OpenStreetMap raster tiles, Vanilla ES6+ JavaScript, CSS3 custom properties & glassmorphism.

**Spec:** [`docs/superpowers/specs/2026-09-13-interactive-geospatial-map-design.md`](file:///home/scottdensmore/Developer/scottdensmore/petspotr/docs/superpowers/specs/2026-09-13-interactive-geospatial-map-design.md)

## Global Constraints

- Must maintain defense-in-depth CSP (`script-src 'self'`, `style-src 'self' https://fonts.googleapis.com`); no external script/style CDNs allowed.
- Allow OpenStreetMap tile raster images via `img-src 'self' data: blob: https://storage.petspotr.io https://*.tile.openstreetmap.org;`.
- Leaflet JS and CSS must be vendored under `internal/app/webfrontend/static/vendor/leaflet/` and embedded into the Go executable via `//go:embed`.
- Zero proprietary third-party mapping API keys.
- Preserve backward-compatibility and progressive enhancement: page remains fully usable when JavaScript is disabled or geolocation is denied.
- Every commit must follow Conventional Commits formatting (`feat(...)`, `test(...)`, `docs(...)`).
- Must pass `make verify` (`go vet`, `golangci-lint`, `tofu validate`, `yamllint`, `go test -race -cover ./...`).

---

### Task 1: Vendor Leaflet 1.9.4 Assets & Update Content Security Policy

**Files:**
- Create: `internal/app/webfrontend/static/vendor/leaflet/leaflet.js`
- Create: `internal/app/webfrontend/static/vendor/leaflet/leaflet.css`
- Create: `internal/app/webfrontend/static/vendor/leaflet/images/marker-icon.png`
- Create: `internal/app/webfrontend/static/vendor/leaflet/images/marker-shadow.png`
- Modify: `internal/app/webfrontend/server.go:35-36`
- Modify: `internal/app/webfrontend/directory_test.go`

**Interfaces:**
- Consumes: `internal/app/webfrontend/server.go:contentSecurityPolicy`
- Produces: Vendored static assets served at `/static/vendor/leaflet/*` and updated CSP header permitting `https://*.tile.openstreetmap.org` in `img-src`.

- [ ] **Step 1: Write the failing tests in `directory_test.go`**

```go
func TestDirectory_ContentSecurityPolicy_PermitsOpenStreetMapTiles(t *testing.T) {
	srv := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/pets", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rec.Code)
	}

	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "https://*.tile.openstreetmap.org") {
		t.Fatalf("expected CSP to allow openstreetmap tiles in img-src, got: %s", csp)
	}
}

func TestDirectory_StaticVendoredLeafletAssets(t *testing.T) {
	srv := NewServer()

	tests := []struct {
		urlPath     string
		contentType string
	}{
		{urlPath: "/static/vendor/leaflet/leaflet.js", contentType: "text/javascript"},
		{urlPath: "/static/vendor/leaflet/leaflet.css", contentType: "text/css"},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, tc.urlPath, nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("asset %s returned status %d, expected 200", tc.urlPath, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, tc.contentType) {
			t.Errorf("asset %s returned content-type %s, expected to contain %s", tc.urlPath, ct, tc.contentType)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run "TestDirectory_ContentSecurityPolicy_PermitsOpenStreetMapTiles|TestDirectory_StaticVendoredLeafletAssets" ./internal/app/webfrontend`
Expected: FAIL with missing asset or CSP mismatch.

- [ ] **Step 3: Vendor Leaflet 1.9.4 and update `server.go`**

1. Create directory `internal/app/webfrontend/static/vendor/leaflet/images`.
2. Download minified assets:
   - `curl -sL https://unpkg.com/leaflet@1.9.4/dist/leaflet.js -o internal/app/webfrontend/static/vendor/leaflet/leaflet.js`
   - `curl -sL https://unpkg.com/leaflet@1.9.4/dist/leaflet.css -o internal/app/webfrontend/static/vendor/leaflet/leaflet.css`
   - `curl -sL https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png -o internal/app/webfrontend/static/vendor/leaflet/images/marker-icon.png`
   - `curl -sL https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png -o internal/app/webfrontend/static/vendor/leaflet/images/marker-shadow.png`
3. In `internal/app/webfrontend/server.go`, update `contentSecurityPolicy`:
```go
const contentSecurityPolicy = "default-src 'self'; base-uri 'self'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: blob: https://storage.petspotr.io https://*.tile.openstreetmap.org; connect-src 'self'; worker-src 'self'"
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -run "TestDirectory_ContentSecurityPolicy_PermitsOpenStreetMapTiles|TestDirectory_StaticVendoredLeafletAssets" ./internal/app/webfrontend`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/static/vendor/leaflet internal/app/webfrontend/server.go internal/app/webfrontend/directory_test.go
git commit -m "feat(frontend): vendor leaflet 1.9.4 assets and permit openstreetmap tiles in CSP"
```

---

### Task 2: Extend `PetsPageData` and Implement Geospatial Serialization in `directory.go`

**Files:**
- Modify: `internal/app/webfrontend/directory.go:55-75,450-480`
- Modify: `internal/app/webfrontend/directory_test.go`

**Interfaces:**
- Consumes: `PublicPetDirectoryItem.Coordinates`, `DirectoryQueryParams` (`HasGeo`, `GeoPoint`, `RadiusMiles`)
- Produces: `PetsPageData.PetsJSON template.JS`, `PetsPageData.MappedCount int`, `PetsPageData.Lat float64`, `PetsPageData.Lng float64`, `PetsPageData.RadiusMiles float64`, `PetsPageData.HasGeo bool`

- [ ] **Step 1: Write the failing tests in `directory_test.go`**

```go
func TestDirectory_GeospatialDataSerialization(t *testing.T) {
	memStore := store.NewMemoryStore()
	ctx := context.Background()

	// Seed one report with coordinates and one without
	now := time.Now().UTC()
	lat := 37.7749
	lng := -122.4194

	lostPetWithGeo := domain.LostPetReport{
		PetID:       "pet-geo-1",
		ReporterID:  "reporter-1",
		PetName:     "Milo",
		Species:     domain.SpeciesDog,
		Breed:       "Beagle",
		Status:      domain.ReportStatusActive,
		ReportedAt:  now,
		Location:    domain.LocationPoint{Latitude: lat, Longitude: lng},
		LastSeenAt:  now,
		Description: "Friendly hound near the park",
	}
	if err := memStore.SaveLostPetReport(ctx, lostPetWithGeo); err != nil {
		t.Fatalf("failed to seed lost pet report: %v", err)
	}

	lostPetWithoutGeo := domain.LostPetReport{
		PetID:       "pet-nogeo-2",
		ReporterID:  "reporter-2",
		PetName:     "Shadow",
		Species:     domain.SpeciesCat,
		Breed:       "Domestic Shorthair",
		Status:      domain.ReportStatusActive,
		ReportedAt:  now.Add(-time.Hour),
		Location:    domain.LocationPoint{Latitude: 0, Longitude: 0},
		LastSeenAt:  now.Add(-time.Hour),
		Description: "Black cat",
	}
	if err := memStore.SaveLostPetReport(ctx, lostPetWithoutGeo); err != nil {
		t.Fatalf("failed to seed lost pet report: %v", err)
	}

	srv := NewServerWithOptions(memStore, ServerOptions{})
	req := httptest.NewRequest(http.MethodGet, "/pets?lat=37.7749&lng=-122.4194&radiusMiles=15", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, `id="pets-data"`) {
		t.Errorf("expected HTML body to contain script id='pets-data'")
	}
	if !strings.Contains(body, "pet-geo-1") {
		t.Errorf("expected script id='pets-data' or body to contain pet-geo-1")
	}
	if !strings.Contains(body, `37.7749`) || !strings.Contains(body, `-122.4194`) {
		t.Errorf("expected coordinates to be serialized in page body")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestDirectory_GeospatialDataSerialization" ./internal/app/webfrontend`
Expected: FAIL due to missing `pets-data` script or unpopulated fields.

- [ ] **Step 3: Implement `PetsPageData` extensions and marker serialization in `directory.go`**

1. Define `PetMapMarker` struct in `directory.go`:
```go
// PetMapMarker represents a lightweight client marker payload for directory map pins.
type PetMapMarker struct {
	PetID      string  `json:"petId"`
	Status     string  `json:"status"`
	PetName    string  `json:"petName"`
	Species    string  `json:"species"`
	Breed      string  `json:"breed"`
	Location   string  `json:"location"`
	ImageURL   string  `json:"imageUrl"`
	ReportedAt string  `json:"reportedAt"`
	Latitude   float64 `json:"lat"`
	Longitude  float64 `json:"lng"`
}
```

2. Add fields to `PetsPageData`:
```go
	Lat         float64
	Lng         float64
	RadiusMiles float64
	HasGeo      bool
	MappedCount int
	PetsJSON    template.JS
```

3. In `handleDirectoryPage`, construct markers slice, serialize with `json.Marshal`, and populate `PetsPageData`:
```go
	var markers []PetMapMarker
	for _, item := range pagedItems {
		if item.Coordinates != nil && (item.Coordinates.Latitude != 0 || item.Coordinates.Longitude != 0) {
			markers = append(markers, PetMapMarker{
				PetID:      item.PetID,
				Status:     item.Status,
				PetName:    item.PetName,
				Species:    item.Species,
				Breed:      item.Breed,
				Location:   item.Location,
				ImageURL:   item.ImageURL,
				ReportedAt: item.ReportedAt.Format("Jan 02, 2006"),
				Latitude:   item.Coordinates.Latitude,
				Longitude:  item.Coordinates.Longitude,
			})
		}
	}

	var petsJSON template.JS = "[]"
	if jsonBytes, err := json.Marshal(markers); err == nil {
		petsJSON = template.JS(jsonBytes)
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run "TestDirectory_GeospatialDataSerialization" ./internal/app/webfrontend`
Expected: PASS (after template snippet in Task 3, or pass data checks).

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/directory.go internal/app/webfrontend/directory_test.go
git commit -m "feat(frontend): add geospatial marker serialization and proximity data to PetsPageData"
```

---

### Task 3: Template Markup for View Switcher, Proximity Filter Inputs & Map Container

**Files:**
- Modify: `internal/app/webfrontend/templates/pets.html`
- Modify: `internal/app/webfrontend/directory_test.go`

**Interfaces:**
- Consumes: `PetsPageData.PetsJSON`, `PetsPageData.MappedCount`, `PetsPageData.HasGeo`, `PetsPageData.Lat`, `PetsPageData.Lng`, `PetsPageData.RadiusMiles`
- Produces: HTML structure containing `#pets-map-container`, `#pets-map`, view switcher buttons (`#btn-view-grid`, `#btn-view-map`), proximity controls (`#filter-lat`, `#filter-lng`, `#filter-radius`, `#btn-geolocation`), and `<script id="pets-data">`.

- [ ] **Step 1: Write the failing test in `directory_test.go`**

```go
func TestDirectory_TemplateMapElementsPresent(t *testing.T) {
	srv := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/pets", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	requiredElements := []string{
		`id="btn-view-grid"`,
		`id="btn-view-map"`,
		`id="pets-map-container"`,
		`id="pets-map"`,
		`id="filter-lat"`,
		`id="filter-lng"`,
		`id="filter-radius"`,
		`id="btn-geolocation"`,
		`id="pets-data"`,
		`/static/vendor/leaflet/leaflet.css`,
		`/static/vendor/leaflet/leaflet.js`,
	}

	for _, elem := range requiredElements {
		if !strings.Contains(body, elem) {
			t.Errorf("expected pets.html to contain %s", elem)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run "TestDirectory_TemplateMapElementsPresent" ./internal/app/webfrontend`
Expected: FAIL with missing markup elements.

- [ ] **Step 3: Update `templates/pets.html`**

1. Add Leaflet stylesheet in `<head>`:
```html
  <link rel="stylesheet" href="/static/vendor/leaflet/leaflet.css">
```
2. In `#pet-filter-form`, add proximity inputs and geolocation button:
```html
        <div class="directory-form-group directory-geo-group">
          <input type="hidden" name="lat" id="filter-lat" value="{{ if .HasGeo }}{{ .Lat }}{{ end }}">
          <input type="hidden" name="lng" id="filter-lng" value="{{ if .HasGeo }}{{ .Lng }}{{ end }}">
          <button type="button" class="btn btn-secondary btn-geo" id="btn-geolocation" title="Use current device location">
            <span class="geo-icon">📍</span>
            <span id="geo-btn-label">{{ if .HasGeo }}Location Set{{ else }}Near Me{{ end }}</span>
          </button>
          <select id="filter-radius" name="radiusMiles" class="form-control geo-radius-select" {{ if not .HasGeo }}disabled{{ end }} aria-label="Search radius">
            <option value="5"{{ if eq .RadiusMiles 5.0 }} selected{{ end }}>5 miles</option>
            <option value="10"{{ if or (eq .RadiusMiles 10.0) (not .HasGeo) }} selected{{ end }}>10 miles</option>
            <option value="25"{{ if eq .RadiusMiles 25.0 }} selected{{ end }}>25 miles</option>
            <option value="50"{{ if eq .RadiusMiles 50.0 }} selected{{ end }}>50 miles</option>
            <option value="100"{{ if eq .RadiusMiles 100.0 }} selected{{ end }}>100 miles</option>
          </select>
          {{ if .HasGeo }}
            <a href="/pets?species={{ .Species }}&status={{ .Status }}&q={{ .Query }}" class="btn btn-secondary btn-clear-geo" id="btn-clear-geo" title="Clear proximity filter">✕ Clear Location</a>
          {{ end }}
        </div>
```
3. In the summary section, add the view switcher buttons and mapped count:
```html
    <div class="directory-summary-bar">
      <p id="results-count" class="results-count" role="status" aria-live="polite">
        {{ if eq .TotalCount 0 }}
          No active reports found
        {{ else }}
          Showing {{ len .Pets }} of {{ .TotalCount }} active reports
          {{ if gt .MappedCount 0 }}<span class="mapped-count-badge">({{ .MappedCount }} mapped)</span>{{ end }}
        {{ end }}
      </p>

      <div class="view-switcher" role="group" aria-label="Directory view mode">
        <button type="button" class="btn-view-toggle active" id="btn-view-grid" aria-pressed="true" title="Grid View">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="3" width="7" height="7"></rect><rect x="14" y="3" width="7" height="7"></rect><rect x="14" y="14" width="7" height="7"></rect><rect x="3" y="14" width="7" height="7"></rect></svg>
          <span>Grid</span>
        </button>
        <button type="button" class="btn-view-toggle" id="btn-view-map" aria-pressed="false" title="Map View">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="1 6 1 22 8 18 16 22 23 18 23 2 16 6 8 2 1 6"></polygon><line x1="8" y1="2" x2="8" y2="18"></line><line x1="16" y1="6" x2="16" y2="22"></line></svg>
          <span>Map</span>
        </button>
      </div>
    </div>
```
4. Add `#pets-map-container` above or next to `#pets-grid` (hidden by default):
```html
    <section class="glass-card pets-map-container hidden" id="pets-map-container" aria-label="Interactive map view of pet reports">
      <div id="pets-map" class="pets-map" role="region" aria-label="Map showing active lost and found pet markers"></div>
      <div class="map-status-overlay" id="map-status-overlay" aria-live="polite"></div>
    </section>
```
5. Enhance `.pet-card` attributes:
```html
          <article class="glass-card pet-card" data-pet-id="{{ .PetID }}" data-status="{{ .Status }}" {{ if .Coordinates }}data-lat="{{ .Coordinates.Latitude }}" data-lng="{{ .Coordinates.Longitude }}"{{ end }}>
```
6. Add scripts at the bottom:
```html
  <script id="pets-data" type="application/json">{{ .PetsJSON }}</script>
  <script src="/static/vendor/leaflet/leaflet.js"></script>
  <script src="/static/js/theme.js"></script>
  <script src="/static/js/pet-directory.js"></script>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run "TestDirectory_TemplateMapElementsPresent" ./internal/app/webfrontend`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/app/webfrontend/templates/pets.html internal/app/webfrontend/directory_test.go
git commit -m "feat(frontend): add map container, view switcher, and proximity inputs to pets template"
```

---

### Task 4: CSS Styling for View Switcher, Glassmorphic Map Container & Dark Tiles

**Files:**
- Modify: `internal/app/webfrontend/static/css/styles.css`

**Interfaces:**
- Consumes: CSS class names `.view-switcher`, `.btn-view-toggle`, `.pets-map-container`, `.pets-map`, `.map-pet-popup`, `.custom-map-pin`
- Produces: Responsive styling, glassmorphic layout, dark mode raster tile filter, and mobile media queries.

- [ ] **Step 1: Write CSS rules in `internal/app/webfrontend/static/css/styles.css`**

Add rules:
```css
/* Directory Summary Bar & View Switcher */
.directory-summary-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
  flex-wrap: wrap;
  gap: 1rem;
}

.mapped-count-badge {
  color: var(--text-secondary);
  font-size: 0.9rem;
  margin-left: 0.5rem;
}

.view-switcher {
  display: inline-flex;
  background: var(--bg-card);
  border: 1px solid var(--border-glass);
  border-radius: var(--radius-md);
  padding: 0.25rem;
  gap: 0.25rem;
}

.btn-view-toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.85rem;
  border-radius: var(--radius-sm);
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 0.9rem;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-view-toggle:hover {
  color: var(--text-primary);
}

.btn-view-toggle.active {
  background: var(--accent-primary);
  color: #ffffff;
  box-shadow: 0 2px 8px rgba(79, 70, 229, 0.4);
}

/* Proximity Controls */
.directory-geo-group {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.btn-geo {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.5rem 0.85rem;
  white-space: nowrap;
}

.geo-radius-select {
  max-width: 130px;
}

.btn-clear-geo {
  font-size: 0.85rem;
  color: var(--accent-lost);
  padding: 0.4rem 0.6rem;
}

/* Interactive Map Container */
.pets-map-container {
  position: relative;
  width: 100%;
  margin-bottom: 2rem;
  padding: 0.75rem;
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.pets-map {
  width: 100%;
  height: 560px;
  border-radius: var(--radius-md);
  background: var(--bg-surface);
  z-index: 1;
}

/* Leaflet Dark Theme Harmonization */
[data-theme="dark"] .leaflet-tile-pane {
  filter: brightness(0.65) invert(1) contrast(3) hue-rotate(200deg) saturate(0.3);
}

[data-theme="dark"] .leaflet-container {
  background: #111827;
}

/* Custom Pin Markers */
.custom-map-pin {
  display: flex;
  justify-content: center;
  align-items: center;
}

.map-pin-inner {
  width: 32px;
  height: 32px;
  border-radius: 50% 50% 50% 0;
  transform: rotate(-45deg);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 3px 10px rgba(0, 0, 0, 0.4);
  border: 2px solid #ffffff;
  transition: transform 0.2s ease;
}

.map-pin-inner:hover {
  transform: rotate(-45deg) scale(1.15);
}

.map-pin-inner span {
  transform: rotate(45deg);
  font-size: 14px;
}

.pin-lost {
  background: #f59e0b;
}

.pin-found {
  background: #10b981;
}

.pin-center {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #3b82f6;
  border: 3px solid #ffffff;
  box-shadow: 0 0 15px #3b82f6;
  animation: pulse-ring 2s infinite;
}

@keyframes pulse-ring {
  0% { transform: scale(0.9); box-shadow: 0 0 0 0 rgba(59, 130, 246, 0.7); }
  70% { transform: scale(1); box-shadow: 0 0 0 12px rgba(59, 130, 246, 0); }
  100% { transform: scale(0.9); box-shadow: 0 0 0 0 rgba(59, 130, 246, 0); }
}

/* Map Popup Card */
.map-pet-popup .leaflet-popup-content-wrapper {
  background: var(--bg-card);
  color: var(--text-primary);
  border-radius: var(--radius-md);
  border: 1px solid var(--border-glass);
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(12px);
  padding: 0;
  overflow: hidden;
}

.map-pet-popup .leaflet-popup-content {
  margin: 0;
  padding: 0.85rem;
  max-width: 240px;
}

.popup-img {
  width: 100%;
  height: 120px;
  object-fit: cover;
  border-radius: var(--radius-sm);
  margin-bottom: 0.5rem;
}

.popup-title {
  font-size: 1.05rem;
  font-weight: 700;
  margin: 0 0 0.25rem 0;
}

.popup-meta {
  font-size: 0.85rem;
  color: var(--text-secondary);
  margin: 0 0 0.5rem 0;
}

.popup-btn {
  display: block;
  width: 100%;
  text-align: center;
  padding: 0.35rem;
  font-size: 0.85rem;
}
```

- [ ] **Step 2: Run verification to ensure valid syntax**

Run: `make test`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/app/webfrontend/static/css/styles.css
git commit -m "style(frontend): add responsive glassmorphic styles for map view, pins, and popups"
```

---

### Task 5: Client-Side Interactive Map & Proximity Controller in `pet-directory.js`

**Files:**
- Modify: `internal/app/webfrontend/static/js/pet-directory.js`

**Interfaces:**
- Consumes: `#pets-data` JSON script tag, `#btn-view-grid`, `#btn-view-map`, `#filter-lat`, `#filter-lng`, `#filter-radius`, `#btn-geolocation`, Leaflet `L` global object
- Produces: Dynamic Leaflet map initialization, custom SVG pins, popup cards, visual proximity radius circles, and click-to-pin coordinate updates.

- [ ] **Step 1: Implement controller in `internal/app/webfrontend/static/js/pet-directory.js`**

Implement:
1. `initViewSwitcher()`:
   - Handle toggling between grid and map.
   - Sync view state with `localStorage.getItem('petspotr_view_mode')` and URL search param `?view=map`.
   - Lazily call `initMap()` when map container becomes visible.
2. `initMap()`:
   - Read JSON data from `#pets-data`.
   - Initialize Leaflet map instance with OSM tile layer.
   - Add status-colored custom marker pins (`L.divIcon`) for each pet item.
   - Bind accessible popup cards displaying thumbnail, status badge, title, location, and details link.
   - Render blue proximity circle if `lat` and `lng` are present.
   - Add click-to-pin listener: clicking on the map sets `filter-lat` and `filter-lng`, moves the radius circle, and displays an option to apply proximity filter.
   - Automatically fit map bounds to markers via `map.fitBounds(markersGroup.getBounds(), { padding: [50, 50] })`.
3. `initGeolocation()`:
   - Wire `#btn-geolocation` to `navigator.geolocation.getCurrentPosition()`.
   - On success: populate `filter-lat` and `filter-lng`, enable `filter-radius`, and submit the filter form or redraw the radius circle.
   - On error: display non-intrusive alert/message.
4. `initRadiusSelect()`:
   - When radius select changes, if coordinates are set, auto-submit the form or redraw the circle.

- [ ] **Step 2: Run Go and asset tests**

Run: `go test -race -cover ./internal/app/webfrontend/...`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/app/webfrontend/static/js/pet-directory.js
git commit -m "feat(frontend): implement client-side interactive map and proximity controller"
```

---

### Task 6: Full Verification & Integration Validation

**Files:**
- Run all test suites and verify complete system integrity.

- [ ] **Step 1: Run comprehensive local verification suite**

Run: `make verify`
Expected:
- `go vet ./...` exits 0.
- `golangci-lint run` exits 0.
- `tofu validate` exits 0.
- `yamllint .` exits 0.
- `go test -race -cover ./...` passes 100% across all packages.

- [ ] **Step 2: Verify git status and clean working tree**

Run: `git status`
Expected: Clean working tree on `feat/interactive-geospatial-map`.

- [ ] **Step 3: Update Task documentation if needed**
