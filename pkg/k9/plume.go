package k9

import (
	"math"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const earthRadiusM = 6371000.0

// EvaluateScentViability computes the scent retention viability score (0-100), rating,
// paw pad thermal hazard indicator, and handler safety/tactical advisory for K9 operations.
func EvaluateScentViability(env domain.K9EnvironmentConditions) (int, domain.ScentViabilityRating, bool, string) {
	score := 80

	switch env.GroundSurface {
	case domain.SurfaceLushVegetation:
		score += 12
	case domain.SurfaceForestFloor:
		score += 8
	case domain.SurfaceSnowIce:
		score += 2
	case domain.SurfaceDryGrass:
		score -= 10
	case domain.SurfaceUrbanAsphalt:
		score -= 22
	}

	if env.RelativeHumidityPct >= 60.0 {
		score += 10
	} else if env.RelativeHumidityPct < 30.0 {
		score -= 18
	}

	pawHazard := env.TemperatureC >= 30.0 || (env.GroundSurface == domain.SurfaceUrbanAsphalt && env.TemperatureC >= 27.0)
	if env.TemperatureC >= 32.0 {
		score -= 30
	} else if env.TemperatureC >= 27.0 {
		score -= 15
	} else if env.TemperatureC >= 10.0 && env.TemperatureC <= 22.0 {
		score += 6
	}

	score -= int(math.Round(env.TrailAgeHours * 1.5))

	if env.WindSpeedKmh > 35.0 {
		score -= 20
	} else if env.WindSpeedKmh >= 5.0 && env.WindSpeedKmh <= 20.0 {
		score += 5
	}

	if score < 0 {
		score = 0
	} else if score > 100 {
		score = 100
	}

	var rating domain.ScentViabilityRating
	var advisory string

	if score < 20 {
		rating = domain.ScentViabilityExpired
		advisory = "Scent trail severely degraded (<20 viability) — deploy camera traps or aerial IR recon"
	} else if pawHazard {
		rating = domain.ScentViabilityDegraded
		advisory = "THERMAL PAW HAZARD: Hot surface / ambient temperature — equip K9 protective booties and limit work cycle to 20m"
	} else if score >= 75 {
		rating = domain.ScentViabilityHigh
		advisory = "Optimal scent retention & laminar transport — high mantrailing & air-scent corridor fidelity"
	} else if score >= 45 {
		rating = domain.ScentViabilityModerate
		advisory = "Moderate scent retention — focus K9 casting along shaded vegetation & moisture traps"
	} else {
		rating = domain.ScentViabilityDegraded
		advisory = "Degraded scent conditions — shorten K9 lead and verify secondary scent articles"
	}

	return score, rating, pawHazard, advisory
}

// ComputeGaussianScentPlume computes the downwind scent dispersion cone polygon from the LKP origin.
func ComputeGaussianScentPlume(originLat, originLng float64, env domain.K9EnvironmentConditions, viabilityScore int) domain.ScentPlumePolygon {
	downwindAzimuth := math.Mod(env.WindFromDeg+180.0, 360.0)
	if downwindAzimuth < 0 {
		downwindAzimuth += 360.0
	}

	effectiveViability := math.Max(float64(viabilityScore), 20.0)
	lengthM := (250.0 + env.WindSpeedKmh*18.0) * (effectiveViability / 100.0)
	if lengthM < 120.0 {
		lengthM = 120.0
	} else if lengthM > 1200.0 {
		lengthM = 1200.0
	}

	spreadAngle := 36.0
	if env.WindSpeedKmh < 8.0 {
		spreadAngle = 60.0
	} else if env.WindSpeedKmh > 22.0 {
		spreadAngle = 24.0
	}

	vertices := make([]domain.PlumeCoordinate, 0, 9)
	vertices = append(vertices, domain.PlumeCoordinate{Lat: originLat, Lng: originLng})

	const arcSteps = 6
	startBearing := downwindAzimuth - spreadAngle/2.0
	stepDeg := spreadAngle / float64(arcSteps)

	for i := 0; i <= arcSteps; i++ {
		bearingDeg := startBearing + float64(i)*stepDeg
		lat, lng := projectDestination(originLat, originLng, bearingDeg, lengthM)
		vertices = append(vertices, domain.PlumeCoordinate{
			Lat: math.Round(lat*1e6) / 1e6,
			Lng: math.Round(lng*1e6) / 1e6,
		})
	}

	vertices = append(vertices, domain.PlumeCoordinate{Lat: originLat, Lng: originLng})

	return domain.ScentPlumePolygon{
		OriginLat:          originLat,
		OriginLng:          originLng,
		DownwindAzimuthDeg: math.Round(downwindAzimuth*10) / 10,
		EffectiveLengthM:   math.Round(lengthM*10) / 10,
		SpreadAngleDeg:     spreadAngle,
		Vertices:           vertices,
	}
}

func projectDestination(latDeg, lngDeg, bearingDeg, distanceM float64) (float64, float64) {
	lat1 := latDeg * math.Pi / 180.0
	lng1 := lngDeg * math.Pi / 180.0
	brng := bearingDeg * math.Pi / 180.0
	angDist := distanceM / earthRadiusM

	sinLat2 := math.Sin(lat1)*math.Cos(angDist) + math.Cos(lat1)*math.Sin(angDist)*math.Cos(brng)
	lat2 := math.Asin(math.Max(-1.0, math.Min(1.0, sinLat2)))

	y := math.Sin(brng) * math.Sin(angDist) * math.Cos(lat1)
	x := math.Cos(angDist) - math.Sin(lat1)*math.Sin(lat2)
	lng2 := lng1 + math.Atan2(y, x)

	return lat2 * 180.0 / math.Pi, lng2 * 180.0 / math.Pi
}
