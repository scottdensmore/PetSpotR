# Milestone 10.2: Topography & Friction-Aware Predictive Trajectory Modeling — Design Specification

## 1. Executive Summary & Vision
Milestone 10.2 elevates PetSpotR's search and recovery capabilities by upgrading the simple radial search perimeter in `pkg/sighting` into an anisotropic, physics- and biology-grounded **Topography & Friction-Aware Predictive Trajectory Engine**.

In the real world, lost domestic animals do not disperse into uniform geometric circles. Their movement is constrained by physical terrain barriers (rivers, multi-lane freeways, steep topographical grades) and drawn toward natural attraction corridors (parks, trail systems, greenways, wooded pockets). Furthermore, canine and feline species exhibit radically different wandering behaviors: dogs roam along linear corridors with higher velocity and momentum, whereas cats remain in tight, high-cover territory radii and exhibit extreme aversion to crossing open thoroughfares.

By generating time-decayed cost-distance isochrones (50% Core, 75% Search Envelope, 90% Containment), identifying high-probability hiding clusters, and dynamically prioritizing active Search Party sectors, PetSpotR empowers rescue volunteers to focus ground efforts where pets are most likely to be located.

---

## 2. Mathematical & Algorithmic Foundation

### 2.1 Spatial Grid & Local Coordinate Projection
The predictive engine operates on a local discrete 2D grid centered at the pet's latest known coordinate $(\phi_0, \lambda_0)$ (or original lost location if no sightings exist).

To ensure high performance without CGO or external GIS runtime dependencies, coordinates are projected to a Cartesian grid $(x, y)$ in meters using the Equirectangular projection parameterized by the origin latitude:
$$x = R_{\text{earth}} \cdot (\lambda - \lambda_0) \cdot \frac{\pi}{180} \cdot \cos\left(\phi_0 \cdot \frac{\pi}{180}\right)$$
$$y = R_{\text{earth}} \cdot (\phi - \phi_0) \cdot \frac{\pi}{180}$$
where $R_{\text{earth}} = 6{,}371{,}000\text{ m}$.

The bounding extent scales dynamically based on elapsed time $\Delta t = t_{\text{now}} - t_{\text{last\_sighting}}$ and species maximum range:
- Grid dimension: $N \times N$ cells (default $100 \times 100$).
- Cell resolution: $\delta = \frac{2 \cdot R_{\text{search}}}{N}$ (typically 20m to 50m per cell).

### 2.2 Friction Surface Model ($C_{\text{cell}}$)
Each grid cell $(i, j)$ has an associated movement cost multiplier $C(i, j) \ge 0.1$:
$$C(i, j) = C_{\text{base}} \times M_{\text{terrain}}(i, j) \times M_{\text{barrier}}(i, j) \times M_{\text{slope}}(i, j) \times M_{\text{species}}(i, j)$$

1. **Base Surface Cost**: $C_{\text{base}} = 1.0$ (neutral walkable ground).
2. **Physical Barriers ($M_{\text{barrier}}$)**:
   - Major limited-access freeways / interstates (e.g. I-5): $M_{\text{barrier}} = 1000.0$ (impassable/extreme risk).
   - Major water bodies & rivers: $M_{\text{barrier}} = 1000.0$ (impassable).
   - Minor surface streets: $M_{\text{barrier}} = 1.5$ (mild resistance).
3. **Attraction Corridors ($M_{\text{terrain}}$)**:
   - Greenways, parks, public trails, wooded patches: $M_{\text{terrain}} = 0.30$ (low friction, high draw).
   - Residential side streets and alleys: $M_{\text{terrain}} = 0.70$.
4. **Topographic Slope Friction ($M_{\text{slope}}$)**:
   Calculated based on Tobler's hiking formulation:
   $$W(\theta) = 6 \cdot e^{-3.5 \cdot |\tan(\theta) + 0.05|}$$
   Steep grades ($> 15^\circ$) sharply increase cost ($M_{\text{slope}} \ge 3.0$).

### 2.3 Species-Specific Behavioral Profiles
Movement dynamics vary significantly by animal species:

| Characteristic | Canine (`Dog`) | Feline (`Cat`) | General / Other |
| :--- | :--- | :--- | :--- |
| **Base Velocity ($v_{\text{base}}$)** | $3.0\text{ mph}$ ($1.34\text{ m/s}$) | $0.5\text{ mph}$ ($0.22\text{ m/s}$) | $1.5\text{ mph}$ ($0.67\text{ m/s}$) |
| **Max Roaming Radius ($R_{\text{max}}$)** | $5.0\text{ miles}$ ($8000\text{ m}$) | $0.35\text{ miles}$ ($550\text{ m}$) | $2.0\text{ miles}$ ($3200\text{ m}$) |
| **Open Road Aversion** | Moderate ($2.0\times$ penalty) | Extreme ($10.0\times$ penalty) | Moderate ($2.0\times$) |
| **Cover / Hiding Affinity** | Balanced ($0.4\times$ in parks) | Critical ($0.2\times$ in dense cover) | Balanced ($0.5\times$) |
| **Directional Momentum** | Strong (persists along heading) | Low (erratic / stationary shelter) | Low |

For dogs with an established movement heading $\theta_{\text{heading}}$ from the last two sightings, cells in direction $\theta_{\text{cell}}$ receive a directional momentum discount:
$$M_{\text{momentum}} = 1.0 - 0.35 \cdot \max\left(0, \cos(\theta_{\text{cell}} - \theta_{\text{heading}})\right)$$

### 2.4 Anisotropic Cost-Distance Wavefront (Dijkstra Solver)
The accumulated minimum cost distance $T(i, j)$ from the origin cell $(i_0, j_0)$ to all other grid cells is calculated using a priority-queue Dijkstra / Fast-Marching solver:
- Edge weight between adjacent cells: $d_{\text{step}} \times \frac{C_{\text{from}} + C_{\text{to}}}{2}$, where $d_{\text{step}} = \delta$ for cardinal steps and $\sqrt{2} \cdot \delta$ for diagonal steps.
- Total allowable travel budget for elapsed time $\Delta t$:
  $$T_{\text{budget}} = v_{\text{species}} \cdot \Delta t$$
- Probability Contours extracted:
  - **50% Core Isochrone**: $T(i, j) \le 0.50 \cdot T_{\text{budget}}$
  - **75% Search Envelope**: $T(i, j) \le 0.75 \cdot T_{\text{budget}}$
  - **90% Containment Boundary**: $T(i, j) \le 1.00 \cdot T_{\text{budget}}$
- Boundary cells are traced using radial border following and smoothed into closed GeoJSON `Polygon` rings.

### 2.5 Hiding & Shelter Cluster Extraction
Local cost minima located inside low-friction attraction zones within the 75% envelope are clustered:
- Contiguous low-cost cells are grouped into distinct physical clusters.
- Centroid coordinates $(\phi_c, \lambda_c)$, radius, and attraction score $(0.0 - 1.0)$ are computed.
- Named based on intersecting terrain feature (e.g. "Green Lake Park Corridor", "North Woodland Refuge").

---

## 3. Architecture & Domain Models

### 3.1 Domain Types (`pkg/domain/trajectory.go`)
```go
package domain

import "time"

type BarrierType string

const (
	BarrierTypeWaterway   BarrierType = "waterway"
	BarrierTypeFreeway    BarrierType = "freeway"
	BarrierTypeGreenway   BarrierType = "greenway"
	BarrierTypeSteepSlope BarrierType = "steep_slope"
)

type TerrainFeature struct {
	ID           string          `json:"id"`
	Type         BarrierType     `json:"type"`
	Name         string          `json:"name"`
	FrictionCost float64         `json:"frictionCost"`
	Geometry     []LocationPoint `json:"geometry"`
}

type IsochroneContour struct {
	ProbabilityLevel   float64           `json:"probabilityLevel"` // 0.50, 0.75, 0.90
	Label              string            `json:"label"`
	ColorHex           string            `json:"colorHex"`
	PolygonCoordinates [][]LocationPoint `json:"polygonCoordinates"`
}

type HidingCluster struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Centroid        LocationPoint `json:"centroid"`
	RadiusMeters    float64       `json:"radiusMeters"`
	AttractionScore float64       `json:"attractionScore"`
	Description     string        `json:"description"`
}

type PredictiveTrajectoryResult struct {
	LostPetID           string             `json:"lostPetId"`
	Species             string             `json:"species"`
	GeneratedAt         time.Time          `json:"generatedAt"`
	ElapsedHours        float64            `json:"elapsedHours"`
	LastSightingPoint   LocationPoint      `json:"lastSightingPoint"`
	HeadingDegrees      float64            `json:"headingDegrees"`
	Isochrones          []IsochroneContour `json:"isochrones"`
	BarriersEncountered []TerrainFeature   `json:"barriersEncountered"`
	HidingClusters      []HidingCluster    `json:"hidingClusters"`
}
```

### 3.2 Search Sector Urgency Integration (`pkg/domain/search_party.go`)
```go
type SectorUrgencyLevel string

const (
	SectorUrgencyCritical SectorUrgencyLevel = "CRITICAL" // Overlaps 50% core isochrone
	SectorUrgencyHigh     SectorUrgencyLevel = "HIGH"     // Overlaps 75% search envelope
	SectorUrgencyStandard SectorUrgencyLevel = "STANDARD" // Normal search sector
)

// SearchSector fields added:
type SearchSector struct {
	// ... existing fields ...
	PriorityScore    float64            `json:"priorityScore,omitempty"`
	UrgencyLevel     SectorUrgencyLevel `json:"urgencyLevel,omitempty"`
	HidingClusterIDs []string           `json:"hidingClusterIds,omitempty"`
}
```

---

## 4. Backend REST Endpoints & Handlers

### 4.1 Endpoint Overview
1. **`GET /api/v1/lost-pets/{petId}/trajectory`** (Enhanced):
   - Returns existing `TrajectoryAnalysis` augmented with `predictiveModel *domain.PredictiveTrajectoryResult`.
   - Preserves 100% backward compatibility for existing consumers.
2. **`GET /api/v1/lost-pets/{petId}/predictive-trajectory`** (New):
   - Accepts query params `?elapsedHours=X&species=dog|cat` to allow search party coordinators to simulate hypothetical timeframes.
3. **`GET /api/v1/lost-pets/{petId}/search-party`** (Enhanced):
   - Computes sector intersection against predictive isochrones and returns prioritized sectors with urgency levels (`CRITICAL`, `HIGH`, `STANDARD`).

---

## 5. Frontend & UI Architecture

### 5.1 Trajectory Modal Visualization (`sighting-trajectory.js`)
- **Layer Switcher**: Glassmorphic segmented pill control:
  - `🔘 Standard Perimeter`: Classic radial perimeter circle.
  - `🔘 Topography & Friction Contours`: 3-tier isochrone rings + barrier lines + attraction corridors.
  - `🔘 Hiding Clusters`: High-probability refuge pins.
- **Isochrone Polygons**:
  - `50% Core`: Vivid red-amber translucent polygon (`fillOpacity: 0.25`, `color: #ef4444`).
  - `75% Active`: Amber dashed outline polygon (`fillOpacity: 0.15`, `color: #f59e0b`).
  - `90% Containment`: Sky blue dashed boundary (`fillOpacity: 0.08`, `color: #0ea5e9`).
- **Barriers & Corridors**:
  - Physical barriers (freeways/rivers): Red hatched polyline overlay.
  - Corridors (parks/greenways): Emerald green softly glowing polylines.
- **Cluster Pins**:
  - Custom `divIcon` shelter pins (`🏕️`) displaying popup with attraction score and terrain notes.

### 5.2 Search Party Sector Prioritization (`search-party.js`, `searchparty_modal.html`)
- **Urgency Badges**:
  - Sectors overlapping the 50% isochrone display `🔥 Critical Search Zone`.
  - Sectors overlapping the 75% isochrone display `⚡ High Priority`.
- **Urgency Filter/Sort**:
  - Quick-action toggle: `Sort by Priority` so volunteers claim high-probability sectors immediately.
- **Safety / Terrain Warnings**:
  - Warning banner when a claimed sector intersects an impassable highway or waterway.

---

## 6. Verification & Quality Gates

1. **Pure Go Unit & Race Tests**:
   - `pkg/sighting/friction_test.go`: Cartesian projection, barrier deflection, species-specific dispersion constraints, and sector polygon intersection math.
   - Concurrency safety verified with `-race` under `export GOTOOLCHAIN=go1.26.5`.
2. **Repository Verification**:
   - `make verify` passing with 0 lint issues, valid OpenTofu configs, and clean YAML.
3. **Playwright End-to-End Journey Test**:
   - `tests/playwright/e2e/predictive-trajectory-journey.spec.ts`:
     - Creates lost pet report and multiple chronological sightings.
     - Inspects Trajectory Modal and verifies rendered isochrones, barriers, and hiding clusters.
     - Toggles layer display mode.
     - Opens Search Party Modal, verifies dynamic `CRITICAL` sector priority badges, and claims a prioritized sector.
   - All Playwright tests must pass (123+ tests).
