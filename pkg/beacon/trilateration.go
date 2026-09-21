package beacon

import (
	"math"
	"sort"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	// EarthMeanRadiusMeters is the approximate mean spherical radius of Earth in meters.
	EarthMeanRadiusMeters = 6371000.0

	// DegToRad converts degrees to radians.
	DegToRad = math.Pi / 180.0
)

// TriangulateBeacon estimates the coordinates, accuracy radius, and confidence score of a BLE beacon
// given a history of observations.
//
// Behavior:
//   - If pings is empty, returns an empty TriangulationResult with ObservationCount 0.
//   - Considers up to the 10 most recent observations, sorted descending by RecordedAt.
//   - 1 observation: returns observer coords, accuracy radius equal to observed distance, and 0.5 confidence.
//   - 2 observations: returns inverse-variance weighted midpoint (w_i = 1/d_i^2), accuracy radius min(d1, d2)*0.85,
//     and 0.75 confidence.
//   - 3+ observations: solves non-linear least squares in a local tangent plane initialized with the
//     distance-weighted centroid, falling back smoothly to centroid if collinear or degenerate,
//     with confidence min(0.95, 0.6 + 0.1*min(k, 4)).
func TriangulateBeacon(pings []BeaconPing) TriangulationResult {
	if len(pings) == 0 {
		return TriangulationResult{
			ObservationCount: 0,
		}
	}

	// Copy and sort by RecordedAt descending (most recent first)
	p := make([]BeaconPing, len(pings))
	copy(p, pings)
	sort.Slice(p, func(i, j int) bool {
		return p[i].RecordedAt.After(p[j].RecordedAt)
	})

	if len(p) > 10 {
		p = p[:10]
	}

	k := len(p)
	latest := p[0]

	// 1 Observation
	if k == 1 {
		dist := latest.DistanceMeters
		if dist < MinDistanceMeters {
			dist = MinDistanceMeters
		}
		return TriangulationResult{
			EstimatedCoordinates: latest.ObserverCoords,
			AccuracyRadiusMeters: dist,
			ObservationCount:     1,
			LastObservedAt:       latest.RecordedAt,
			ConfidenceScore:      0.5,
			Proximity:            DetermineProximity(dist, latest.RSSI),
		}
	}

	// 2 Observations: inverse-variance distance-weighted midpoint
	if k == 2 {
		d0 := math.Max(p[0].DistanceMeters, MinDistanceMeters)
		d1 := math.Max(p[1].DistanceMeters, MinDistanceMeters)

		w0 := 1.0 / (d0 * d0)
		w1 := 1.0 / (d1 * d1)
		wSum := w0 + w1

		estLat := (w0*p[0].ObserverCoords.Latitude + w1*p[1].ObserverCoords.Latitude) / wSum
		estLon := (w0*p[0].ObserverCoords.Longitude + w1*p[1].ObserverCoords.Longitude) / wSum

		accuracy := math.Min(d0, d1) * 0.85
		if accuracy < MinDistanceMeters {
			accuracy = MinDistanceMeters
		}

		return TriangulationResult{
			EstimatedCoordinates: domain.LocationPoint{
				Latitude:  estLat,
				Longitude: estLon,
			},
			AccuracyRadiusMeters: accuracy,
			ObservationCount:     2,
			LastObservedAt:       latest.RecordedAt,
			ConfidenceScore:      0.75,
			Proximity:            DetermineProximity(accuracy, latest.RSSI),
		}
	}

	// 3+ Observations: distance-weighted centroid + damped Gauss-Newton non-linear least squares
	distances := make([]float64, k)
	weights := make([]float64, k)
	var wSum float64
	var weightedLat, weightedLon float64
	minDist := math.MaxFloat64

	for i := 0; i < k; i++ {
		d := math.Max(p[i].DistanceMeters, MinDistanceMeters)
		distances[i] = d
		if d < minDist {
			minDist = d
		}
		w := 1.0 / (d * d)
		weights[i] = w
		wSum += w
		weightedLat += w * p[i].ObserverCoords.Latitude
		weightedLon += w * p[i].ObserverCoords.Longitude
	}

	centroidLat := weightedLat / wSum
	centroidLon := weightedLon / wSum

	// Project observations to local tangent Cartesian coordinates in meters relative to centroid
	metersPerLat := EarthMeanRadiusMeters * DegToRad
	metersPerLon := EarthMeanRadiusMeters * DegToRad * math.Cos(centroidLat*DegToRad)
	if math.Abs(metersPerLon) < 1e-6 {
		metersPerLon = 1e-6
	}

	obsX := make([]float64, k)
	obsY := make([]float64, k)
	var sumX, sumY float64

	for i := 0; i < k; i++ {
		x := (p[i].ObserverCoords.Longitude - centroidLon) * metersPerLon
		y := (p[i].ObserverCoords.Latitude - centroidLat) * metersPerLat
		obsX[i] = x
		obsY[i] = y
		sumX += x
		sumY += y
	}

	meanX := sumX / float64(k)
	meanY := sumY / float64(k)
	var spreadSq float64
	for i := 0; i < k; i++ {
		dx := obsX[i] - meanX
		dy := obsY[i] - meanY
		spreadSq += dx*dx + dy*dy
	}
	spread := math.Sqrt(spreadSq / float64(k))

	estX, estY := 0.0, 0.0

	// Calculate initial objective at centroid (0,0)
	var initObj float64
	for i := 0; i < k; i++ {
		r := math.Hypot(obsX[i], obsY[i])
		res := r - distances[i]
		initObj += weights[i] * res * res
	}

	// Optimize if observer points have geometric spread (not coincident)
	if spread >= 0.5 {
		curX, curY := 0.0, 0.0
		lambda := 0.01

		for iter := 0; iter < 30; iter++ {
			var a00, a01, a11 float64
			var b0, b1 float64

			for i := 0; i < k; i++ {
				dx := curX - obsX[i]
				dy := curY - obsY[i]
				r := math.Hypot(dx, dy)
				if r < 1e-6 {
					r = 1e-6
				}
				jx := dx / r
				jy := dy / r
				res := r - distances[i]

				w := weights[i]
				a00 += w * jx * jx
				a01 += w * jx * jy
				a11 += w * jy * jy
				b0 += -w * jx * res
				b1 += -w * jy * res
			}

			// Levenberg damping
			dampedA00 := a00 + lambda
			dampedA11 := a11 + lambda
			det := dampedA00*dampedA11 - a01*a01

			if det <= 1e-9 {
				break
			}

			stepX := (dampedA11*b0 - a01*b1) / det
			stepY := (-a01*b0 + dampedA00*b1) / det

			stepNorm := math.Hypot(stepX, stepY)
			if stepNorm > 20.0 {
				stepX *= 20.0 / stepNorm
				stepY *= 20.0 / stepNorm
			}

			curX += stepX
			curY += stepY

			if stepNorm < 1e-4 {
				break
			}
		}

		// Verify objective improvement and reasonableness
		var finalObj float64
		for i := 0; i < k; i++ {
			r := math.Hypot(curX-obsX[i], curY-obsY[i])
			res := r - distances[i]
			finalObj += weights[i] * res * res
		}

		distFromCentroid := math.Hypot(curX, curY)
		if !math.IsNaN(curX) && !math.IsNaN(curY) && !math.IsInf(curX, 0) && !math.IsInf(curY, 0) &&
			distFromCentroid <= 150.0 && finalObj <= initObj {
			estX = curX
			estY = curY
		}
	}

	estLat := centroidLat + (estY / metersPerLat)
	estLon := centroidLon + (estX / metersPerLon)

	// Compute RMSE for accuracy radius
	var sumResSq float64
	for i := 0; i < k; i++ {
		r := math.Hypot(estX-obsX[i], estY-obsY[i])
		res := r - distances[i]
		sumResSq += res * res
	}
	rmse := math.Sqrt(sumResSq / float64(k))

	accuracy := minDist * 0.75
	if rmse > 0 && rmse < accuracy {
		accuracy = math.Max(0.1, rmse)
	}
	if accuracy < MinDistanceMeters {
		accuracy = MinDistanceMeters
	}

	kCap := k
	if kCap > 4 {
		kCap = 4
	}
	confidence := math.Min(0.95, 0.6+0.1*float64(kCap))

	return TriangulationResult{
		EstimatedCoordinates: domain.LocationPoint{
			Latitude:  estLat,
			Longitude: estLon,
		},
		AccuracyRadiusMeters: accuracy,
		ObservationCount:     k,
		LastObservedAt:       latest.RecordedAt,
		ConfidenceScore:      confidence,
		Proximity:            DetermineProximity(accuracy, latest.RSSI),
	}
}
