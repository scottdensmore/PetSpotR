# Milestone 5.1: Interactive Geospatial Directory & Mapping Design Spec

- **Author**: Antigravity & Scott Densmore
- **Date**: 2026-09-13
- **Status**: Proposed
- **Target Epic**: Milestone 5.1 (`docs/ROADMAP.md`)

---

## 1. Context & Objectives

The PetSpotR public directory (`/pets`) currently presents active lost and found pet reports in a responsive card grid. Backend domain models (`domain.LocationPoint`) and geocoding pipelines already capture and validate latitude/longitude coordinates (`GeocodingStatusSuccess`) on pet reports, and the query parser (`parseDirectoryQueryParams`) supports spatial radius filtering via `lat`, `lng`, and `radiusMiles` query parameters.

However, users currently lack a visual, spatial interface to locate reports geographically or visually explore proximity around their current location or neighborhood.

### 1.1 Goals
1. **Interactive Map View on `/pets`**: Provide an accessible view toggle between the standard **Grid View** and an expansive **Map View**.
2. **Strict Security & Offline Resilience**: Vendor Leaflet assets locally under `internal/app/webfrontend/static/vendor/leaflet/` to maintain strict Content Security Policy (`script-src 'self'`, `style-src 'self'`). Allow OpenStreetMap raster tiles via `img-src 'self' ... https://*.tile.openstreetmap.org`.
3. **Status-Coded Custom Pins & Popups**: Plot lost reports in amber (`#f59e0b`) and found reports in emerald (`#10b981`). Clicking pins opens an interactive popup card displaying the pet thumbnail, status badge, species/breed, location, and link to full details.
4. **Geospatial Proximity & Radius Sync**: Provide a "Use My Location" browser geolocation action and radius dropdown (5, 10, 25, 50, 100 miles) that renders a visual proximity circle on the map and synchronizes with existing `lat`, `lng`, and `radiusMiles` query parameters.
5. **Progressive Enhancement**: Ensure complete functionality when JavaScript is disabled or geolocation is denied, gracefully handling unlocated reports.

### 1.2 Non-Goals
- Proprietary map APIs requiring paid third-party API keys (e.g. Google Maps JavaScript API, Mapbox GL access tokens).
- Dynamic client-side polygon geofencing or complex shape drawing.
- Replacing the server-rendered Go template architecture with a Single Page Application framework.

---

## 2. Architecture & Security

```
+-------------------------------------------------------------------------+
| Browser Client (/pets)                                                 |
|                                                                         |
|  +-----------------------+     +-------------------------------------+  |
|  | Directory Toolbar     |     | View Switcher                       |  |
|  | - Species / Status / Q|     | [ ⊞ Grid View ]   [ 🗺 Map View ]   |  |
|  | - Lat / Lng / Radius  |     +-------------------------------------+  |
|  +-----------+-----------+                                              |
|              |                                                          |
|      (Toggles Active Container)                                         |
|              |                                                          |
|              +--------------------------+                               |
|              |                          |                               |
|              v                          v                               |
|     +------------------+       +------------------------------------+   |
|     |   #pets-grid     |       |   #pets-map-container              |   |
|     |  .pet-card items |       |   - Leaflet Map (OSM raster tiles) |   |
|     |  with data-lat/  |       |   - Lost/Found Custom SVG Pins     |   |
|     |  data-lng attrs  |       |   - Proximity Radius Circle (blue) |   |
|     +------------------+       |   - Mini-Popup Pet Cards           |   |
|                                +------------------------------------+   |
|                                                 ^                       |
|                                                 | (reads JSON)          |
|  +----------------------------------------------+                       |
|  | <script id="pets-data" type="application/json">                      |
|  |   [{"petId": "...", "lat": 37.77, "lng": -122.41, "status": ...}]    |
|  +----------------------------------------------------------------------+
+-------------------------------------------------------------------------+
```

### 2.1 Content Security Policy (CSP) Updates
In `internal/app/webfrontend/server.go`, the application applies a defense-in-depth Content Security Policy header.

To accommodate OpenStreetMap tile loading while keeping all script and stylesheet execution strictly origin-local:
```
default-src 'self';
base-uri 'self';
object-src 'none';
frame-ancestors 'none';
form-action 'self';
script-src 'self';
style-src 'self' https://fonts.googleapis.com;
font-src 'self' https://fonts.gstatic.com;
img-src 'self' data: blob: https://storage.petspotr.io https://*.tile.openstreetmap.org;
connect-src 'self';
worker-src 'self'
```
*Key constraint*: `script-src` and `style-src` do NOT allow external CDNs (such as unpkg or cdnjs). All Leaflet scripts and stylesheet dependencies must be served locally from `/static/vendor/leaflet/`.

### 2.2 Vendored Assets
Assets placed in `internal/app/webfrontend/static/vendor/leaflet/`:
- `leaflet.js` (minified release v1.9.4)
- `leaflet.css` (minified release v1.9.4)
- `images/marker-icon.png`, `images/marker-shadow.png`

These assets are automatically compiled into the Go executable via `//go:embed static/* templates/*`.

---

## 3. Data Flow & Server-Side Integration

### 3.1 Backend Model Additions in `directory.go`

1. **`PublicPetDirectoryItem` Coordinates**:
   Already defined:
   ```go
   Coordinates     *domain.LocationPoint  `json:"coordinates,omitempty"`
   GeocodingStatus domain.GeocodingStatus `json:"geocodingStatus,omitempty"`
   ```

2. **`PetsPageData` Extension**:
   ```go
   type PetsPageData struct {
       Pets        []PublicPetDirectoryItem
       TotalCount  int
       CurrentPage int
       TotalPages  int
       Species     string
       Status      string
       Query       string
       Limit       int
       Offset      int
       HasPrev     bool
       HasNext     bool
       PrevPageURL string
       NextPageURL string
       NextCursor  string
       PrevCursor  string

       // Geospatial extensions
       Lat         float64
       Lng         float64
       RadiusMiles float64
       HasGeo      bool
       MappedCount int
       PetsJSON    template.JS
   }
   ```

3. **Serialization Logic**:
   In `handleDirectoryPage`:
   - Inspect `pagedItems`: count items where `Coordinates != nil` and coordinates are valid (`Latitude != 0 || Longitude != 0`).
   - Construct a slice of lightweight marker descriptors:
     ```go
     type PetMapMarker struct {
         PetID       string  `json:"petId"`
         Status      string  `json:"status"`
         PetName     string  `json:"petName"`
         Species     string  `json:"species"`
         Breed       string  `json:"breed"`
         Location    string  `json:"location"`
         ImageURL    string  `json:"imageUrl"`
         ReportedAt  string  `json:"reportedAt"`
         Latitude    float64 `json:"lat"`
         Longitude   float64 `json:"lng"`
     }
     ```
   - Serialize to JSON and assign to `PetsPageData.PetsJSON`.
   - Forward `params.HasGeo`, `params.GeoPoint.Latitude`, `params.GeoPoint.Longitude`, and `params.RadiusMiles` to `PetsPageData`.

---

## 4. Frontend UI & Interaction Design

### 4.1 Toolbar Controls in `templates/pets.html`

1. **View Switcher**:
   Adjacent to the results count in the directory header:
   ```html
   <div class="view-switcher" role="group" aria-label="Directory view mode">
     <button type="button" class="btn-view-toggle active" id="btn-view-grid" aria-pressed="true">
       <svg ...>...</svg> <span>Grid</span>
     </button>
     <button type="button" class="btn-view-toggle" id="btn-view-map" aria-pressed="false">
       <svg ...>...</svg> <span>Map</span>
     </button>
   </div>
   ```

2. **Proximity & Radius Filter Form**:
   Inside `#pet-filter-form`:
   ```html
   <div class="directory-geo-group">
     <input type="hidden" name="lat" id="filter-lat" value="{{ if .HasGeo }}{{ .Lat }}{{ end }}">
     <input type="hidden" name="lng" id="filter-lng" value="{{ if .HasGeo }}{{ .Lng }}{{ end }}">
     <button type="button" class="btn btn-secondary btn-geo" id="btn-geolocation">
       📍 Use My Location
     </button>
     <select id="filter-radius" name="radiusMiles" class="form-control" {{ if not .HasGeo }}disabled{{ end }}>
       <option value="5"{{ if eq .RadiusMiles 5.0 }} selected{{ end }}>Within 5 miles</option>
       <option value="10"{{ if or (eq .RadiusMiles 10.0) (not .HasGeo) }} selected{{ end }}>Within 10 miles</option>
       <option value="25"{{ if eq .RadiusMiles 25.0 }} selected{{ end }}>Within 25 miles</option>
       <option value="50"{{ if eq .RadiusMiles 50.0 }} selected{{ end }}>Within 50 miles</option>
     </select>
     {{ if .HasGeo }}
       <a href="/pets?species={{ .Species }}&status={{ .Status }}&q={{ .Query }}" class="btn-link-clear-geo" id="btn-clear-geo">✕ Clear Location</a>
     {{ end }}
   </div>
   ```

3. **Results Indicator**:
   ```html
   Showing {{ len .Pets }} of {{ .TotalCount }} active reports
   {{ if gt .MappedCount 0 }}({{ .MappedCount }} mapped){{ end }}
   ```

### 4.2 Map Container & Leaflet Controller (`pet-directory.js`)

1. **Container Initialization**:
   - Container `#pets-map` is mounted inside `#pets-map-container` (hidden by default unless `?view=map` or saved in `localStorage`).
   - When switched to Map View, if the Leaflet map instance is not yet created, initialize:
     ```javascript
     const map = L.map('pets-map', { scrollWheelZoom: false });
     L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
       attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
       maxZoom: 19
     }).addTo(map);
     ```
   - In dark theme (`data-theme="dark"`), apply CSS tile filter:
     ```css
     [data-theme="dark"] .leaflet-tile-pane {
       filter: brightness(0.65) invert(1) contrast(3) hue-rotate(200deg) saturate(0.3);
     }
     ```

2. **Custom Pin Icons**:
   - Distinct SVG div-icons:
     - **Lost**: Amber teardrop pin with paw glyph (`#f59e0b`, white border).
     - **Found**: Emerald teardrop pin with checkmark glyph (`#10b981`, white border).
     - **Search Center**: Pulsing azure beacon (`#3b82f6`) when `HasGeo` is true.

3. **Marker Popups**:
   - On click, displays an accessible popup:
     - Thumbnail image (or fallback icon)
     - Status pill (`lost` or `found`)
     - Pet Name / Species
     - Reported location and date
     - Link to view report / matches

4. **Radius Circle & Map Clicks**:
   - When coordinates and radius are defined, render `L.circle([lat, lng], { radius: meters, color: '#3b82f6', fillColor: '#3b82f6', fillOpacity: 0.12 })`.
   - When in Map View, clicking on the map updates the hidden `lat` and `lng` inputs and shows a floating action button or tooltip: *"Search within X miles of here"*.

5. **Auto-fit Viewport**:
   - If markers exist, call `map.fitBounds(markersLayer.getBounds(), { padding: [50, 50] })`.
   - If no markers exist, center on default region (or user location if available).

---

## 5. Error Handling & Edge Cases

1. **Unlocated Reports**:
   Reports without geocoding or invalid coordinates are excluded from map markers but remain clearly visible in the card grid.
2. **Geolocation Denied**:
   If `navigator.geolocation` permission is denied, display a non-blocking toast/notice: *"Location access was denied. You can click on the map to set a search center."*
3. **Offline / Tile Request Failure**:
   Leaflet tiles failing to load will display the styled dark background; markers and radius circles remain fully interactive and functional.
4. **Zero Results**:
   Display the standard PetSpotR empty state card with a button to reset spatial filters.

---

## 6. Testing & Verification Plan

### 6.1 Backend Unit Tests (`internal/app/webfrontend/directory_test.go`)
1. **`TestDirectory_GeospatialDataInjection`**:
   - Seed reports with valid coordinates, invalid coordinates, and missing coordinates.
   - Assert that `handleDirectoryPage` renders valid JSON in `<script id="pets-data">` containing only mapped items.
   - Assert `.pet-card` elements have corresponding `data-lat` and `data-lng` attributes.
2. **`TestDirectory_ProximityQueryParams`**:
   - Test queries with `lat`, `lng`, and `radiusMiles`.
   - Assert that form inputs retain query values and radius calculations accurately filter results.
3. **`TestDirectory_ContentSecurityPolicy`**:
   - Request `/pets` and assert that the `Content-Security-Policy` header includes `https://*.tile.openstreetmap.org` in `img-src`.
4. **`TestDirectory_StaticVendoredAssets`**:
   - Perform HTTP GET requests to `/static/vendor/leaflet/leaflet.js` and `/static/vendor/leaflet/leaflet.css`.
   - Assert HTTP 200 and valid MIME types (`text/javascript`, `text/css`).

### 6.2 Full CI Verification
- Execute `make verify`:
  - `go vet ./...`
  - `golangci-lint run`
  - `tofu validate`
  - `yamllint .`
  - `go test -race -cover ./...`
