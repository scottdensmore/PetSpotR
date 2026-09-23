package recon

import (
	"errors"
	"fmt"
	"math"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

const (
	// metersPerDegreeLat is the approximate WGS84 meridional meters per degree of latitude.
	metersPerDegreeLat = 111132.95

	// earthMeanRadiusMeters is the authalic mean earth radius in meters.
	earthMeanRadiusMeters = 6371000.0

	// defaultHFOV is the default horizontal field of view in degrees.
	defaultHFOV = 84.0

	// defaultVFOV is the default vertical field of view in degrees.
	defaultVFOV = 60.0
)

// ComputeCameraFrustumFootprint calculates the dynamic camera ground frustum footprint polygon
// [TopLeft, TopRight, BottomRight, BottomLeft, TopLeft] in WGS84 coordinates (each vertex is [lng, lat]).
func ComputeCameraFrustumFootprint(wp domain.DroneWaypoint, cam domain.CameraIntrinsics) ([][]float64, error) {
	if wp.AltitudeAGL <= 0 {
		return nil, errors.New("recon: drone altitude AGL must be positive")
	}
	if wp.Latitude < -90.0 || wp.Latitude > 90.0 {
		return nil, fmt.Errorf("recon: latitude must be between -90 and 90, got %f", wp.Latitude)
	}
	if wp.Longitude < -180.0 || wp.Longitude > 180.0 {
		return nil, fmt.Errorf("recon: longitude must be between -180 and 180, got %f", wp.Longitude)
	}

	cam = normalizeIntrinsics(cam)
	if err := validateIntrinsics(cam); err != nil {
		return nil, err
	}

	// 4 corners of normalized sensor plane: TopLeft, TopRight, BottomRight, BottomLeft
	corners := [4][2]float64{
		{0.0, 0.0}, // TopLeft
		{1.0, 0.0}, // TopRight
		{1.0, 1.0}, // BottomRight
		{0.0, 1.0}, // BottomLeft
	}

	poly := make([][]float64, 5)
	for i, corner := range corners {
		lat, lng, err := RaycastPixelToGround(corner[0], corner[1], wp, cam)
		if err != nil {
			return nil, fmt.Errorf("recon: failed to project frustum corner %d: %w", i, err)
		}
		poly[i] = []float64{lng, lat}
	}
	// Close polygon ring (5th vertex equals 1st vertex)
	poly[4] = []float64{poly[0][0], poly[0][1]}

	return poly, nil
}

// RaycastPixelToGround projects a normalized image coordinate (u, v) in [0, 1] x [0, 1]
// to ground WGS84 coordinates (lat, lng), assuming flat terrain at altitude zero relative to launch.
func RaycastPixelToGround(u, v float64, wp domain.DroneWaypoint, cam domain.CameraIntrinsics) (float64, float64, error) {
	if wp.AltitudeAGL <= 0 {
		return 0, 0, errors.New("recon: drone altitude AGL must be positive")
	}
	if wp.Latitude < -90.0 || wp.Latitude > 90.0 {
		return 0, 0, fmt.Errorf("recon: latitude must be between -90 and 90, got %f", wp.Latitude)
	}
	if wp.Longitude < -180.0 || wp.Longitude > 180.0 {
		return 0, 0, fmt.Errorf("recon: longitude must be between -180 and 180, got %f", wp.Longitude)
	}

	cam = normalizeIntrinsics(cam)
	if err := validateIntrinsics(cam); err != nil {
		return 0, 0, err
	}

	hRad := (cam.HFOV * math.Pi / 180.0) / 2.0
	vRad := (cam.VFOV * math.Pi / 180.0) / 2.0

	// Normalized sensor coordinates relative to optical center
	// u in [0, 1]: 0 is Left, 1 is Right
	// v in [0, 1]: 0 is Top, 1 is Bottom
	xc := (u - 0.5) * 2.0 * math.Tan(hRad)
	yc := -(v - 0.5) * 2.0 * math.Tan(vRad) // Top (v=0) has positive yc
	zc := 1.0                               // Optical forward axis

	// Drone orientation in ENU frame:
	// Heading / Yaw: 0 deg North (+Y), 90 deg East (+X)
	totalYawDeg := wp.HeadingDeg + wp.GimbalYawDeg
	yawRad := totalYawDeg * math.Pi / 180.0

	fwdX := math.Sin(yawRad)
	fwdY := math.Cos(yawRad)
	fwdZ := 0.0

	rightX := math.Cos(yawRad)
	rightY := -math.Sin(yawRad)
	rightZ := 0.0

	upX := 0.0
	upY := 0.0
	upZ := 1.0

	// Gimbal pitch: 0 deg Horizontal, -90 deg Nadir downward
	pitchRad := wp.GimbalPitchDeg * math.Pi / 180.0
	cosPitch := math.Cos(pitchRad)
	sinPitch := math.Sin(pitchRad)

	// Optical axis unit vector in ENU coordinates
	dirX := cosPitch*fwdX + sinPitch*upX
	dirY := cosPitch*fwdY + sinPitch*upY
	dirZ := cosPitch*fwdZ + sinPitch*upZ

	// Sensor Up unit vector (perpendicular to optical axis)
	upCamX := -sinPitch*fwdX + cosPitch*upX
	upCamY := -sinPitch*fwdY + cosPitch*upY
	upCamZ := -sinPitch*fwdZ + cosPitch*upZ

	// Sensor Right unit vector
	rightCamX := rightX
	rightCamY := rightY
	rightCamZ := rightZ

	// Optional Gimbal roll (rotation around optical axis)
	if wp.GimbalRollDeg != 0 {
		rollRad := wp.GimbalRollDeg * math.Pi / 180.0
		cosRoll := math.Cos(rollRad)
		sinRoll := math.Sin(rollRad)

		rX := cosRoll*rightCamX + sinRoll*upCamX
		rY := cosRoll*rightCamY + sinRoll*upCamY
		rZ := cosRoll*rightCamZ + sinRoll*upCamZ

		uX := -sinRoll*rightCamX + cosRoll*upCamX
		uY := -sinRoll*rightCamY + cosRoll*upCamY
		uZ := -sinRoll*rightCamZ + cosRoll*upCamZ

		rightCamX, rightCamY, rightCamZ = rX, rY, rZ
		upCamX, upCamY, upCamZ = uX, uY, uZ
	}

	// Unnormalized ray vector in ENU coordinates
	vx := xc*rightCamX + yc*upCamX + zc*dirX
	vy := xc*rightCamY + yc*upCamY + zc*dirY
	vz := xc*rightCamZ + yc*upCamZ + zc*dirZ

	// Intersect ray with ground plane Z = 0
	// Ray starts at (0, 0, AltitudeAGL), intersects when AltitudeAGL + t * vz = 0 => t = -AltitudeAGL / vz
	if vz >= -1e-9 {
		return 0, 0, errors.New("recon: ray does not intersect ground plane (points at or above horizon)")
	}

	t := -wp.AltitudeAGL / vz
	deltaX := t * vx // East offset in meters
	deltaY := t * vy // North offset in meters

	// Convert local offsets to WGS84 latitude / longitude
	deltaLat := deltaY / metersPerDegreeLat
	latRad := wp.Latitude * math.Pi / 180.0
	cosLat := math.Cos(latRad)
	if math.Abs(cosLat) < 1e-6 {
		cosLat = 1e-6
	}
	deltaLng := deltaX / (metersPerDegreeLat * cosLat)

	groundLat := wp.Latitude + deltaLat
	groundLng := wp.Longitude + deltaLng

	return groundLat, groundLng, nil
}

// ComputeSweptAreaSqMeters computes the spherical polygon area in square meters
// given a closed or open polygon of coordinates in [lng, lat] format.
func ComputeSweptAreaSqMeters(poly [][]float64) float64 {
	n := len(poly)
	if n < 3 {
		return 0.0
	}

	// Strip closing vertex if repeated
	pts := poly
	if n > 1 && pts[0][0] == pts[n-1][0] && pts[0][1] == pts[n-1][1] {
		pts = pts[:n-1]
	}

	m := len(pts)
	if m < 3 {
		return 0.0
	}

	// Spherical excess area calculation (Chamberlain & Duquette algorithm)
	var sum float64
	for i := 0; i < m; i++ {
		j := (i + 1) % m
		lng1 := pts[i][0] * math.Pi / 180.0
		lat1 := pts[i][1] * math.Pi / 180.0
		lng2 := pts[j][0] * math.Pi / 180.0
		lat2 := pts[j][1] * math.Pi / 180.0

		dLng := lng2 - lng1
		for dLng > math.Pi {
			dLng -= 2 * math.Pi
		}
		for dLng < -math.Pi {
			dLng += 2 * math.Pi
		}

		sum += dLng * (math.Sin(lat1) + math.Sin(lat2)) / 2.0
	}

	area := math.Abs(sum) * earthMeanRadiusMeters * earthMeanRadiusMeters
	return area
}

// FindIntersectingSectorIDs returns the SectorIDs of all search party sectors that
// intersect with the given ground footprint polygon [lng, lat].
func FindIntersectingSectorIDs(poly [][]float64, sectors []domain.SearchPartySector) []string {
	if len(poly) < 3 || len(sectors) == 0 {
		return []string{}
	}

	polyA := cleanRing(poly)
	if len(polyA) < 3 {
		return []string{}
	}

	minLngA, maxLngA, minLatA, maxLatA := computeBounds(polyA)

	matched := make([]string, 0, len(sectors))
	seen := make(map[string]bool)

	for _, sec := range sectors {
		pts := sec.PolygonPoints
		if len(pts) < 3 {
			pts = sec.BoundingPolygon
		}
		if len(pts) < 3 {
			continue
		}

		polyB := make([][2]float64, len(pts))
		for i, p := range pts {
			polyB[i] = [2]float64{p.Longitude, p.Latitude}
		}
		polyB = cleanRingCoords(polyB)
		if len(polyB) < 3 {
			continue
		}

		minLngB, maxLngB, minLatB, maxLatB := computeBounds(polyB)

		// Bounding box rejection test
		if maxLngA < minLngB || minLngA > maxLngB || maxLatA < minLatB || minLatA > maxLatB {
			continue
		}

		// Exact polygon intersection test
		if polygonsIntersect(polyA, polyB) {
			if !seen[sec.SectorID] {
				seen[sec.SectorID] = true
				matched = append(matched, sec.SectorID)
			}
		}
	}

	return matched
}

func normalizeIntrinsics(cam domain.CameraIntrinsics) domain.CameraIntrinsics {
	if cam.HFOV == 0 {
		cam.HFOV = defaultHFOV
	}
	if cam.VFOV == 0 {
		cam.VFOV = defaultVFOV
	}
	return cam
}

func validateIntrinsics(cam domain.CameraIntrinsics) error {
	if cam.HFOV <= 0 || cam.HFOV >= 180 {
		return fmt.Errorf("recon: camera HFOV must be between 0 and 180 degrees, got %f", cam.HFOV)
	}
	if cam.VFOV <= 0 || cam.VFOV >= 180 {
		return fmt.Errorf("recon: camera VFOV must be between 0 and 180 degrees, got %f", cam.VFOV)
	}
	return nil
}

func cleanRing(raw [][]float64) [][2]float64 {
	coords := make([][2]float64, 0, len(raw))
	for _, p := range raw {
		if len(p) >= 2 {
			coords = append(coords, [2]float64{p[0], p[1]})
		}
	}
	return cleanRingCoords(coords)
}

func cleanRingCoords(coords [][2]float64) [][2]float64 {
	n := len(coords)
	if n > 1 && coords[0][0] == coords[n-1][0] && coords[0][1] == coords[n-1][1] {
		coords = coords[:n-1]
	}
	return coords
}

func computeBounds(poly [][2]float64) (minLng, maxLng, minLat, maxLat float64) {
	minLng, maxLng = poly[0][0], poly[0][0]
	minLat, maxLat = poly[0][1], poly[0][1]
	for _, p := range poly[1:] {
		if p[0] < minLng {
			minLng = p[0]
		}
		if p[0] > maxLng {
			maxLng = p[0]
		}
		if p[1] < minLat {
			minLat = p[1]
		}
		if p[1] > maxLat {
			maxLat = p[1]
		}
	}
	return minLng, maxLng, minLat, maxLat
}

func polygonsIntersect(polyA, polyB [][2]float64) bool {
	// 1. Any edge of A intersects any edge of B
	na := len(polyA)
	nb := len(polyB)

	for i := 0; i < na; i++ {
		p1 := polyA[i]
		p2 := polyA[(i+1)%na]
		for j := 0; j < nb; j++ {
			q1 := polyB[j]
			q2 := polyB[(j+1)%nb]
			if segmentsIntersect(p1, p2, q1, q2) {
				return true
			}
		}
	}

	// 2. Any vertex of A is inside B
	for _, p := range polyA {
		if pointInPoly(p[0], p[1], polyB) {
			return true
		}
	}

	// 3. Any vertex of B is inside A
	for _, p := range polyB {
		if pointInPoly(p[0], p[1], polyA) {
			return true
		}
	}

	return false
}

func segmentsIntersect(p1, p2, q1, q2 [2]float64) bool {
	o1 := orientation(p1, p2, q1)
	o2 := orientation(p1, p2, q2)
	o3 := orientation(q1, q2, p1)
	o4 := orientation(q1, q2, p2)

	// General case
	if (o1*o2 < 0) && (o3*o4 < 0) {
		return true
	}

	// Special cases: points are collinear and lie on segment
	if o1 == 0 && onSegment(p1, q1, p2) {
		return true
	}
	if o2 == 0 && onSegment(p1, q2, p2) {
		return true
	}
	if o3 == 0 && onSegment(q1, p1, q2) {
		return true
	}
	if o4 == 0 && onSegment(q1, p2, q2) {
		return true
	}

	return false
}

func orientation(p, q, r [2]float64) float64 {
	val := (q[0]-p[0])*(r[1]-p[1]) - (q[1]-p[1])*(r[0]-p[0])
	if math.Abs(val) < 1e-12 {
		return 0
	}
	if val > 0 {
		return 1
	}
	return -1
}

func onSegment(p, q, r [2]float64) bool {
	return q[0] <= math.Max(p[0], r[0])+1e-9 && q[0] >= math.Min(p[0], r[0])-1e-9 &&
		q[1] <= math.Max(p[1], r[1])+1e-9 && q[1] >= math.Min(p[1], r[1])-1e-9
}

func pointInPoly(px, py float64, poly [][2]float64) bool {
	n := len(poly)
	if n < 3 {
		return false
	}
	inside := false
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := poly[i][0], poly[i][1]
		xj, yj := poly[j][0], poly[j][1]

		intersect := ((yi > py) != (yj > py)) &&
			(px < (xj-xi)*(py-yi)/(yj-yi)+xi)
		if intersect {
			inside = !inside
		}
		j = i
	}
	return inside
}
