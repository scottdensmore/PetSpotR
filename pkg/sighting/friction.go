package sighting

import (
	"container/heap"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// DefaultSeattleTerrainFeatures provides the baseline physical barriers and corridors
// for the Seattle metropolitan area.
var DefaultSeattleTerrainFeatures = []domain.TerrainFeature{
	{
		ID:           "barrier-i5",
		Type:         domain.BarrierTypeFreeway,
		Name:         "Interstate 5 Corridor",
		FrictionCost: 1000.0,
		Geometry: []domain.LocationPoint{
			{Latitude: 47.5800, Longitude: -122.3200},
			{Latitude: 47.6000, Longitude: -122.3260},
			{Latitude: 47.6080, Longitude: -122.3280},
			{Latitude: 47.6180, Longitude: -122.3270},
			{Latitude: 47.6350, Longitude: -122.3240},
			{Latitude: 47.6550, Longitude: -122.3230},
			{Latitude: 47.6800, Longitude: -122.3250},
		},
	},
	{
		ID:           "barrier-elliott-bay",
		Type:         domain.BarrierTypeWaterway,
		Name:         "Elliott Bay Coastal Barrier",
		FrictionCost: 1000.0,
		Geometry: []domain.LocationPoint{
			{Latitude: 47.5800, Longitude: -122.3500},
			{Latitude: 47.6000, Longitude: -122.3430},
			{Latitude: 47.6100, Longitude: -122.3520},
			{Latitude: 47.6250, Longitude: -122.3700},
			{Latitude: 47.6400, Longitude: -122.3900},
		},
	},
	{
		ID:           "barrier-lake-union",
		Type:         domain.BarrierTypeWaterway,
		Name:         "Lake Union Water Barrier",
		FrictionCost: 1000.0,
		Geometry: []domain.LocationPoint{
			{Latitude: 47.6350, Longitude: -122.3400},
			{Latitude: 47.6450, Longitude: -122.3360},
			{Latitude: 47.6440, Longitude: -122.3250},
		},
	},
	{
		ID:           "corridor-freeway-park",
		Type:         domain.BarrierTypeGreenway,
		Name:         "Freeway Park Corridor",
		FrictionCost: 0.30,
		Geometry: []domain.LocationPoint{
			{Latitude: 47.6085, Longitude: -122.3315},
			{Latitude: 47.6115, Longitude: -122.3285},
		},
	},
	{
		ID:           "corridor-volunteer-park",
		Type:         domain.BarrierTypeGreenway,
		Name:         "Volunteer Park Nature Preserve",
		FrictionCost: 0.30,
		Geometry: []domain.LocationPoint{
			{Latitude: 47.6280, Longitude: -122.3180},
			{Latitude: 47.6320, Longitude: -122.3120},
		},
	},
	{
		ID:           "corridor-sculpture-park",
		Type:         domain.BarrierTypeGreenway,
		Name:         "Olympic Sculpture Park & Greenway",
		FrictionCost: 0.30,
		Geometry: []domain.LocationPoint{
			{Latitude: 47.6160, Longitude: -122.3540},
			{Latitude: 47.6180, Longitude: -122.3500},
		},
	},
	{
		ID:           "slope-queen-anne",
		Type:         domain.BarrierTypeSteepSlope,
		Name:         "Queen Anne Ridge Bluff",
		FrictionCost: 3.5,
		Geometry: []domain.LocationPoint{
			{Latitude: 47.6250, Longitude: -122.3580},
			{Latitude: 47.6300, Longitude: -122.3620},
		},
	},
}

type speciesProfile struct {
	baseSpeedMph      float64
	maxRadiusMeters   float64
	roadAversion      float64
	coverAffinity     float64
	hasMomentum       bool
	strictContainment bool
}

func getSpeciesProfile(species string) speciesProfile {
	s := strings.ToLower(strings.TrimSpace(species))
	switch {
	case strings.Contains(s, "dog"), strings.Contains(s, "canine"):
		return speciesProfile{
			baseSpeedMph:      3.0,
			maxRadiusMeters:   8000.0,
			roadAversion:      2.0,
			coverAffinity:     0.40,
			hasMomentum:       true,
			strictContainment: false,
		}
	case strings.Contains(s, "cat"), strings.Contains(s, "feline"):
		return speciesProfile{
			baseSpeedMph:      0.5,
			maxRadiusMeters:   550.0,
			roadAversion:      10.0,
			coverAffinity:     0.20,
			hasMomentum:       false,
			strictContainment: true,
		}
	default:
		return speciesProfile{
			baseSpeedMph:      1.5,
			maxRadiusMeters:   3200.0,
			roadAversion:      2.0,
			coverAffinity:     0.50,
			hasMomentum:       false,
			strictContainment: false,
		}
	}
}

// toCartesian converts geographical coordinates (lat, lon) to Cartesian (x, y) meters
// relative to an origin point (lat0, lon0) using the Equirectangular projection.
func toCartesian(lat, lon, lat0, lon0 float64) (float64, float64) {
	lat0Rad := lat0 * math.Pi / 180.0
	x := earthRadiusMeters * (lon - lon0) * (math.Pi / 180.0) * math.Cos(lat0Rad)
	y := earthRadiusMeters * (lat - lat0) * (math.Pi / 180.0)
	return x, y
}

// toGeographic converts Cartesian meters (x, y) relative to origin (lat0, lon0) back to (lat, lon).
func toGeographic(x, y, lat0, lon0 float64) domain.LocationPoint {
	lat0Rad := lat0 * math.Pi / 180.0
	cosLat := math.Cos(lat0Rad)
	if math.Abs(cosLat) < 1e-7 {
		cosLat = 1e-7
	}

	lat := lat0 + (y/earthRadiusMeters)*(180.0/math.Pi)
	lon := lon0 + (x/(earthRadiusMeters*cosLat))*(180.0/math.Pi)

	return domain.LocationPoint{
		Latitude:  lat,
		Longitude: lon,
	}
}

// pointToSegmentDistanceMeters calculates the shortest Euclidean distance in meters from point P
// to the line segment AB.
func pointToSegmentDistanceMeters(px, py, ax, ay, bx, by float64) float64 {
	vx, vy := bx-ax, by-ay
	wx, wy := px-ax, py-ay

	c1 := wx*vx + wy*vy
	if c1 <= 0.0 {
		return math.Hypot(px-ax, py-ay)
	}

	c2 := vx*vx + vy*vy
	if c2 <= c1 {
		return math.Hypot(px-bx, py-by)
	}

	b := c1 / c2
	projX := ax + b*vx
	projY := ay + b*vy
	return math.Hypot(px-projX, py-projY)
}

// distanceToFeatureMeters computes the minimum distance in meters from Cartesian point (px, py)
// to a polyline feature.
func distanceToFeatureMeters(px, py float64, geomCartesian [][2]float64) float64 {
	if len(geomCartesian) == 0 {
		return math.MaxFloat64
	}
	if len(geomCartesian) == 1 {
		return math.Hypot(px-geomCartesian[0][0], py-geomCartesian[0][1])
	}

	minDist := math.MaxFloat64
	for i := 0; i < len(geomCartesian)-1; i++ {
		d := pointToSegmentDistanceMeters(
			px, py,
			geomCartesian[i][0], geomCartesian[i][1],
			geomCartesian[i+1][0], geomCartesian[i+1][1],
		)
		if d < minDist {
			minDist = d
		}
	}
	return minDist
}

type dijkstraItem struct {
	cost  float64
	i, j  int
	index int
}

type dijkstraPriorityQueue []*dijkstraItem

func (pq dijkstraPriorityQueue) Len() int           { return len(pq) }
func (pq dijkstraPriorityQueue) Less(i, j int) bool { return pq[i].cost < pq[j].cost }
func (pq dijkstraPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *dijkstraPriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*dijkstraItem)
	item.index = n
	*pq = append(*pq, item)
}
func (pq *dijkstraPriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

// GeneratePredictiveTrajectory produces anisotropic time-decayed isochrones, identifies
// encountered physical barriers, and detects high-probability shelter clusters.
func GeneratePredictiveTrajectory(
	lostPetID string,
	species string,
	origin *domain.LocationPoint,
	sightings []domain.PetSightingRecord,
	elapsedHours float64,
) domain.PredictiveTrajectoryResult {
	now := time.Now().UTC()
	profile := getSpeciesProfile(species)

	// Determine starting reference coordinate and last sighting point
	var lastSightingPoint domain.LocationPoint
	if len(sightings) > 0 {
		// Sort chronologically
		sortedSightings := make([]domain.PetSightingRecord, 0, len(sightings))
		for _, s := range sightings {
			if s.Coordinates != nil {
				sortedSightings = append(sortedSightings, s)
			}
		}
		sort.SliceStable(sortedSightings, func(i, j int) bool {
			return sortedSightings[i].SightedAt.Before(sortedSightings[j].SightedAt)
		})

		if len(sortedSightings) > 0 {
			lastSightingPoint = *sortedSightings[len(sortedSightings)-1].Coordinates
		} else if origin != nil {
			lastSightingPoint = *origin
		} else {
			lastSightingPoint = domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
		}
	} else if origin != nil {
		lastSightingPoint = *origin
	} else {
		lastSightingPoint = domain.LocationPoint{Latitude: 47.6062, Longitude: -122.3321}
	}

	// Compute movement heading from chronological sightings if available
	var headingDegrees float64
	var hasHeading bool
	if len(sightings) >= 2 {
		sortedSightings := make([]domain.PetSightingRecord, 0, len(sightings))
		for _, s := range sightings {
			if s.Coordinates != nil {
				sortedSightings = append(sortedSightings, s)
			}
		}
		sort.SliceStable(sortedSightings, func(i, j int) bool {
			return sortedSightings[i].SightedAt.Before(sortedSightings[j].SightedAt)
		})
		if len(sortedSightings) >= 2 {
			p1 := *sortedSightings[len(sortedSightings)-2].Coordinates
			p2 := *sortedSightings[len(sortedSightings)-1].Coordinates
			if p1.Latitude != p2.Latitude || p1.Longitude != p2.Longitude {
				headingDegrees = initialBearingDegrees(p1, p2)
				hasHeading = true
			}
		}
	} else if len(sightings) == 1 && origin != nil && sightings[0].Coordinates != nil {
		p1 := *origin
		p2 := *sightings[0].Coordinates
		if p1.Latitude != p2.Latitude || p1.Longitude != p2.Longitude {
			headingDegrees = initialBearingDegrees(p1, p2)
			hasHeading = true
		}
	}

	if elapsedHours <= 0.0 {
		elapsedHours = 1.0
	}

	// Travel budget in meters = velocity (m/s) * time (s)
	velocityMps := (profile.baseSpeedMph * metersPerMile) / 3600.0
	budgetMeters := velocityMps * (elapsedHours * 3600.0)
	if budgetMeters > profile.maxRadiusMeters {
		budgetMeters = profile.maxRadiusMeters
	}
	if budgetMeters < 150.0 {
		budgetMeters = 150.0
	}

	// Grid configuration
	const gridSize = 80
	searchRadiusMeters := budgetMeters * 1.30
	if searchRadiusMeters > profile.maxRadiusMeters*1.15 {
		searchRadiusMeters = profile.maxRadiusMeters * 1.15
	}
	if searchRadiusMeters < 400.0 {
		searchRadiusMeters = 400.0
	}

	cellResolutionMeters := (2.0 * searchRadiusMeters) / float64(gridSize)
	centerCell := gridSize / 2

	// Convert features to local Cartesian and filter active features
	type cartesianFeature struct {
		feature  domain.TerrainFeature
		geometry [][2]float64
	}
	activeFeatures := make([]cartesianFeature, 0)
	barriersEncountered := make([]domain.TerrainFeature, 0)

	for _, tf := range DefaultSeattleTerrainFeatures {
		cartGeom := make([][2]float64, len(tf.Geometry))
		hasNearPoint := false
		for k, pt := range tf.Geometry {
			cx, cy := toCartesian(pt.Latitude, pt.Longitude, lastSightingPoint.Latitude, lastSightingPoint.Longitude)
			cartGeom[k] = [2]float64{cx, cy}
			if math.Hypot(cx, cy) <= searchRadiusMeters*1.5 {
				hasNearPoint = true
			}
		}
		if hasNearPoint {
			geomCopy := make([]domain.LocationPoint, len(tf.Geometry))
			copy(geomCopy, tf.Geometry)
			tfCopy := tf
			tfCopy.Geometry = geomCopy
			activeFeatures = append(activeFeatures, cartesianFeature{
				feature:  tfCopy,
				geometry: cartGeom,
			})
			barriersEncountered = append(barriersEncountered, tfCopy)
		}
	}

	// Build friction surface grid
	frictionGrid := make([][]float64, gridSize)
	for i := 0; i < gridSize; i++ {
		frictionGrid[i] = make([]float64, gridSize)
		for j := 0; j < gridSize; j++ {
			// Cartesian coordinates in meters (North is +y, East is +x)
			x := (float64(j) - float64(centerCell)) * cellResolutionMeters
			y := (float64(centerCell) - float64(i)) * cellResolutionMeters
			r := math.Hypot(x, y)

			cost := 1.0

			// 1. Terrain features and barriers
			barrierCost := 1.0
			terrainMultiplier := 1.0
			slopeMultiplier := 1.0

			for _, af := range activeFeatures {
				dist := distanceToFeatureMeters(x, y, af.geometry)
				switch af.feature.Type {
				case domain.BarrierTypeFreeway:
					if dist <= 50.0 {
						barrierCost = math.Max(barrierCost, af.feature.FrictionCost*(profile.roadAversion/2.0))
					}
				case domain.BarrierTypeWaterway:
					if dist <= 60.0 {
						barrierCost = math.Max(barrierCost, af.feature.FrictionCost)
					}
				case domain.BarrierTypeGreenway:
					if dist <= 120.0 {
						terrainMultiplier = math.Min(terrainMultiplier, profile.coverAffinity)
					}
				case domain.BarrierTypeSteepSlope:
					if dist <= 80.0 {
						slopeMultiplier = math.Max(slopeMultiplier, af.feature.FrictionCost)
					}
				}
			}

			// Tobler slope resistance: baseline gentle urban slope with steep slope feature elevation
			grade := 0.02
			if slopeMultiplier > 1.0 {
				grade = 0.35 // steep grade ~19 degrees
			}
			wSpeed := 6.0 * math.Exp(-3.5*math.Abs(grade+0.05))
			wFlat := 6.0 * math.Exp(-3.5*0.05)
			toblerFactor := math.Max(1.0, wFlat/wSpeed)

			cost = cost * terrainMultiplier * barrierCost * toblerFactor

			// 2. Directional momentum for canines along heading
			if profile.hasMomentum && hasHeading {
				bearingCell := math.Atan2(x, y) * 180.0 / math.Pi
				if bearingCell < 0 {
					bearingCell += 360.0
				}
				diff := math.Abs(bearingCell - headingDegrees)
				if diff > 180.0 {
					diff = 360.0 - diff
				}
				cosDiff := math.Cos(diff * math.Pi / 180.0)
				if cosDiff > 0 {
					momentumDiscount := 1.0 - (0.35 * cosDiff)
					cost *= momentumDiscount
				}
			}

			// 3. Feline territorial confinement penalty outside home range
			if profile.strictContainment && r > 400.0 {
				excess := r - 400.0
				cost *= 1.0 + math.Pow(excess/40.0, 2.0)*10.0
			}

			if cost < 0.10 {
				cost = 0.10
			}
			frictionGrid[i][j] = cost
		}
	}

	// Cost-distance Dijkstra propagation
	costGrid := make([][]float64, gridSize)
	for i := 0; i < gridSize; i++ {
		costGrid[i] = make([]float64, gridSize)
		for j := 0; j < gridSize; j++ {
			costGrid[i][j] = math.MaxFloat64
		}
	}
	costGrid[centerCell][centerCell] = 0.0

	pq := make(dijkstraPriorityQueue, 0, gridSize*gridSize)
	heap.Push(&pq, &dijkstraItem{cost: 0.0, i: centerCell, j: centerCell})

	// 8 directions (cardinal + diagonal)
	type dir struct {
		di, dj int
		dist   float64
	}
	diagDist := math.Sqrt2 * cellResolutionMeters
	cardDist := cellResolutionMeters
	directions := [8]dir{
		{di: -1, dj: 0, dist: cardDist},
		{di: 1, dj: 0, dist: cardDist},
		{di: 0, dj: -1, dist: cardDist},
		{di: 0, dj: 1, dist: cardDist},
		{di: -1, dj: -1, dist: diagDist},
		{di: -1, dj: 1, dist: diagDist},
		{di: 1, dj: -1, dist: diagDist},
		{di: 1, dj: 1, dist: diagDist},
	}

	for pq.Len() > 0 {
		top := heap.Pop(&pq).(*dijkstraItem)
		if top.cost > costGrid[top.i][top.j] {
			continue
		}
		// Early stop if far beyond search envelope
		if top.cost > budgetMeters*1.30 {
			break
		}

		for _, d := range directions {
			ni, nj := top.i+d.di, top.j+d.dj
			if ni >= 0 && ni < gridSize && nj >= 0 && nj < gridSize {
				edgeCost := d.dist * ((frictionGrid[top.i][top.j] + frictionGrid[ni][nj]) / 2.0)
				newCost := top.cost + edgeCost
				if newCost < costGrid[ni][nj] {
					costGrid[ni][nj] = newCost
					heap.Push(&pq, &dijkstraItem{cost: newCost, i: ni, j: nj})
				}
			}
		}
	}

	// Extract 3 Isochrone Contours (50% Core, 75% Search Envelope, 90% Containment Boundary)
	levels := []struct {
		prob  float64
		label string
		color string
	}{
		{prob: 0.50, label: "50% Core Isochrone", color: "#ef4444"},
		{prob: 0.75, label: "75% Search Envelope", color: "#f59e0b"},
		{prob: 0.90, label: "90% Containment Boundary", color: "#0ea5e9"},
	}

	isochrones := make([]domain.IsochroneContour, len(levels))
	const numRays = 36
	angleStep := 360.0 / float64(numRays)

	for idx, lvl := range levels {
		thresholdCost := lvl.prob * budgetMeters
		ring := make([]domain.LocationPoint, 0, numRays+1)

		for k := 0; k < numRays; k++ {
			bearingDeg := float64(k) * angleStep
			bearingRad := bearingDeg * math.Pi / 180.0
			sinB := math.Sin(bearingRad)
			cosB := math.Cos(bearingRad)

			// March outward along ray from origin
			rStep := cellResolutionMeters / 2.0
			rMax := searchRadiusMeters
			rBound := rStep

			prevCost := 0.0
			prevR := 0.0

			for r := rStep; r <= rMax; r += rStep {
				rx := r * sinB
				ry := r * cosB

				// Map to grid indices
				jFloat := float64(centerCell) + (rx / cellResolutionMeters)
				iFloat := float64(centerCell) - (ry / cellResolutionMeters)

				if iFloat < 0 || iFloat >= float64(gridSize) || jFloat < 0 || jFloat >= float64(gridSize) {
					rBound = r
					break
				}

				iIdx := int(math.Round(iFloat))
				jIdx := int(math.Round(jFloat))
				if iIdx < 0 {
					iIdx = 0
				} else if iIdx >= gridSize {
					iIdx = gridSize - 1
				}
				if jIdx < 0 {
					jIdx = 0
				} else if jIdx >= gridSize {
					jIdx = gridSize - 1
				}

				costAtR := costGrid[iIdx][jIdx]
				if costAtR >= thresholdCost {
					// Linear interpolation of crossing distance
					if costAtR > prevCost && prevCost > 0.0 {
						fraction := (thresholdCost - prevCost) / (costAtR - prevCost)
						rBound = prevR + fraction*(r-prevR)
					} else {
						rBound = r
					}
					break
				}
				prevCost = costAtR
				prevR = r
				rBound = r
			}

			if rBound < cellResolutionMeters*0.5 {
				rBound = cellResolutionMeters * 0.5
			}

			ptX := rBound * sinB
			ptY := rBound * cosB
			ptGeo := toGeographic(ptX, ptY, lastSightingPoint.Latitude, lastSightingPoint.Longitude)
			ring = append(ring, ptGeo)
		}

		// Close ring
		if len(ring) > 0 {
			ring = append(ring, ring[0])
		}

		isochrones[idx] = domain.IsochroneContour{
			ProbabilityLevel:   lvl.prob,
			Label:              lvl.label,
			ColorHex:           lvl.color,
			PolygonCoordinates: [][]domain.LocationPoint{ring},
		}
	}

	// Extract Hiding Clusters inside the 75% search envelope within greenway corridors
	hidingClusters := make([]domain.HidingCluster, 0)
	envelopeCost := 0.75 * budgetMeters

	visited := make([][]bool, gridSize)
	for i := 0; i < gridSize; i++ {
		visited[i] = make([]bool, gridSize)
	}

	clusterIdx := 1
	for _, af := range activeFeatures {
		if af.feature.Type != domain.BarrierTypeGreenway {
			continue
		}

		// Gather cells close to this greenway within envelope cost
		componentX := make([]float64, 0)
		componentY := make([]float64, 0)

		for i := 0; i < gridSize; i++ {
			for j := 0; j < gridSize; j++ {
				if visited[i][j] || costGrid[i][j] > envelopeCost {
					continue
				}
				x := (float64(j) - float64(centerCell)) * cellResolutionMeters
				y := (float64(centerCell) - float64(i)) * cellResolutionMeters

				dist := distanceToFeatureMeters(x, y, af.geometry)
				if dist <= 80.0 {
					visited[i][j] = true
					componentX = append(componentX, x)
					componentY = append(componentY, y)
				}
			}
		}

		if len(componentX) >= 3 {
			var sumX, sumY float64
			for k := 0; k < len(componentX); k++ {
				sumX += componentX[k]
				sumY += componentY[k]
			}
			avgX := sumX / float64(len(componentX))
			avgY := sumY / float64(len(componentY))

			var maxR float64
			for k := 0; k < len(componentX); k++ {
				dist := math.Hypot(componentX[k]-avgX, componentY[k]-avgY)
				if dist > maxR {
					maxR = dist
				}
			}
			if maxR < 35.0 {
				maxR = 35.0
			}

			centroid := toGeographic(avgX, avgY, lastSightingPoint.Latitude, lastSightingPoint.Longitude)
			attractionScore := 0.88
			if profile.strictContainment {
				attractionScore = 0.96 // Felines strongly gravitate to quiet greenway cover
			}

			hidingClusters = append(hidingClusters, domain.HidingCluster{
				ID:              fmt.Sprintf("cluster-%d", clusterIdx),
				Name:            fmt.Sprintf("%s Refuge", af.feature.Name),
				Centroid:        centroid,
				RadiusMeters:    math.Round(maxR*10.0) / 10.0,
				AttractionScore: attractionScore,
				Description:     fmt.Sprintf("High-probability sheltered cover zone near %s with dense vegetation and low foot traffic.", af.feature.Name),
			})
			clusterIdx++
		}
	}

	return domain.PredictiveTrajectoryResult{
		LostPetID:           lostPetID,
		Species:             species,
		GeneratedAt:         now,
		ElapsedHours:        elapsedHours,
		LastSightingPoint:   lastSightingPoint,
		HeadingDegrees:      math.Round(headingDegrees*10.0) / 10.0,
		Isochrones:          isochrones,
		BarriersEncountered: barriersEncountered,
		HidingClusters:      hidingClusters,
	}
}

// GeneratePredictiveTrajectoryForSpecies is a convenience wrapper for species-driven simulations.
func GeneratePredictiveTrajectoryForSpecies(
	species string,
	origin domain.LocationPoint,
	sightings []domain.PetSightingRecord,
	elapsedHours float64,
) domain.PredictiveTrajectoryResult {
	return GeneratePredictiveTrajectory("", species, &origin, sightings, elapsedHours)
}

// pointInPolygon returns true if point pt is geometrically inside the polygon ring.
func pointInPolygon(pt domain.LocationPoint, poly []domain.LocationPoint) bool {
	if len(poly) < 3 {
		return false
	}
	inside := false
	n := len(poly)
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := poly[i].Longitude, poly[i].Latitude
		xj, yj := poly[j].Longitude, poly[j].Latitude
		intersect := ((yi > pt.Latitude) != (yj > pt.Latitude)) &&
			(pt.Longitude < (xj-xi)*(pt.Latitude-yi)/(yj-yi)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}
	return inside
}

// lineSegmentsIntersect returns true if line segment (p1, p2) intersects line segment (p3, p4).
func lineSegmentsIntersect(p1, p2, p3, p4 domain.LocationPoint) bool {
	ccw := func(a, b, c domain.LocationPoint) float64 {
		return (b.Longitude-a.Longitude)*(c.Latitude-a.Latitude) - (b.Latitude-a.Latitude)*(c.Longitude-a.Longitude)
	}

	d1 := ccw(p3, p4, p1)
	d2 := ccw(p3, p4, p2)
	d3 := ccw(p1, p2, p3)
	d4 := ccw(p1, p2, p4)

	return ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0))
}

// polygonsOverlap returns true if polygon1 and polygon2 intersect geometrically.
func polygonsOverlap(poly1, poly2 []domain.LocationPoint) bool {
	if len(poly1) < 3 || len(poly2) < 3 {
		return false
	}

	// 1. Any point of poly1 inside poly2
	for _, pt := range poly1 {
		if pointInPolygon(pt, poly2) {
			return true
		}
	}

	// 2. Any point of poly2 inside poly1
	for _, pt := range poly2 {
		if pointInPolygon(pt, poly1) {
			return true
		}
	}

	// 3. Any edge of poly1 intersects any edge of poly2
	n1 := len(poly1)
	n2 := len(poly2)
	for i := 0; i < n1-1; i++ {
		for j := 0; j < n2-1; j++ {
			if lineSegmentsIntersect(poly1[i], poly1[i+1], poly2[j], poly2[j+1]) {
				return true
			}
		}
	}

	return false
}

// ScoreSectorUrgency evaluates the intersection between a search sector and predictive isochrone contours,
// returning an urgency score (0-100) and urgency triage level (CRITICAL, HIGH, STANDARD).
func ScoreSectorUrgency(
	sector domain.SearchSector,
	isochrones []domain.IsochroneContour,
) (float64, domain.SectorUrgencyLevel) {
	sectorPoints := sector.BoundingPolygon
	if len(sectorPoints) == 0 {
		sectorPoints = sector.PolygonPoints
	}
	if len(sectorPoints) < 3 || len(isochrones) == 0 {
		return 10.0, domain.SectorUrgencyStandard
	}

	var coreIsochrone, envelopeIsochrone, boundaryIsochrone *domain.IsochroneContour
	for i := range isochrones {
		iso := &isochrones[i]
		if iso.ProbabilityLevel <= 0.55 {
			coreIsochrone = iso
		} else if iso.ProbabilityLevel <= 0.80 {
			envelopeIsochrone = iso
		} else {
			boundaryIsochrone = iso
		}
	}

	// 1. Check overlap with 50% Core Isochrone
	if coreIsochrone != nil && len(coreIsochrone.PolygonCoordinates) > 0 {
		if polygonsOverlap(sectorPoints, coreIsochrone.PolygonCoordinates[0]) {
			score := 92.0
			return score, domain.SectorUrgencyCritical
		}
	}

	// 2. Check overlap with 75% Search Envelope
	if envelopeIsochrone != nil && len(envelopeIsochrone.PolygonCoordinates) > 0 {
		if polygonsOverlap(sectorPoints, envelopeIsochrone.PolygonCoordinates[0]) {
			score := 72.0
			return score, domain.SectorUrgencyHigh
		}
	}

	// 3. Check overlap with 90% Containment Boundary
	if boundaryIsochrone != nil && len(boundaryIsochrone.PolygonCoordinates) > 0 {
		if polygonsOverlap(sectorPoints, boundaryIsochrone.PolygonCoordinates[0]) {
			score := 35.0
			return score, domain.SectorUrgencyStandard
		}
	}

	// 4. Distant sector outside all contours
	return 15.0, domain.SectorUrgencyStandard
}
