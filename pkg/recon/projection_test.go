package recon_test

import (
	"math"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/recon"
)

func TestComputeCameraFrustumFootprint_Nadir(t *testing.T) {
	wp := domain.DroneWaypoint{
		Timestamp:      time.Now(),
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    50.0,  // 50 meters AGL
		HeadingDeg:     0.0,   // True North
		GimbalPitchDeg: -90.0, // Nadir downward
	}
	cam := domain.CameraIntrinsics{
		HFOV: 84.0,
		VFOV: 60.0,
	}

	poly, err := recon.ComputeCameraFrustumFootprint(wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(poly) != 5 { // 4 corners + closed loop
		t.Fatalf("expected 5 vertices in closed polygon, got %d", len(poly))
	}

	// Verify centroid is near drone lat/lng
	var sumLat, sumLng float64
	for i := 0; i < 4; i++ {
		sumLng += poly[i][0]
		sumLat += poly[i][1]
	}
	avgLat := sumLat / 4.0
	avgLng := sumLng / 4.0
	if math.Abs(avgLat-wp.Latitude) > 0.0001 || math.Abs(avgLng-wp.Longitude) > 0.0001 {
		t.Errorf("expected footprint centroid near %f,%f; got %f,%f", wp.Latitude, wp.Longitude, avgLat, avgLng)
	}

	area := recon.ComputeSweptAreaSqMeters(poly)
	if area < 1000.0 || area > 10000.0 {
		t.Errorf("expected ground footprint area roughly ~4500-6000 m^2 for 50m AGL, got %f", area)
	}
}

func TestRaycastPixelToGround_CenterPixel(t *testing.T) {
	wp := domain.DroneWaypoint{
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    60.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: -90.0,
	}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	lat, lng, err := recon.RaycastPixelToGround(0.5, 0.5, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(lat-wp.Latitude) > 0.00001 || math.Abs(lng-wp.Longitude) > 0.00001 {
		t.Errorf("center pixel (0.5, 0.5) at nadir should project to drone location; got %f, %f", lat, lng)
	}
}

func TestComputeCameraFrustumFootprint_ObliqueHeading(t *testing.T) {
	wp := domain.DroneWaypoint{
		Timestamp:      time.Now(),
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    80.0,
		HeadingDeg:     90.0,  // East
		GimbalPitchDeg: -45.0, // 45 degrees downward
	}
	cam := domain.CameraIntrinsics{
		HFOV: 84.0,
		VFOV: 60.0,
	}

	poly, err := recon.ComputeCameraFrustumFootprint(wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(poly) != 5 {
		t.Fatalf("expected 5 vertices in closed polygon, got %d", len(poly))
	}

	// At heading 90 (East) and pitch -45, footprint center should be East of drone (greater longitude)
	var sumLng float64
	for i := 0; i < 4; i++ {
		sumLng += poly[i][0]
	}
	avgLng := sumLng / 4.0
	if avgLng <= wp.Longitude {
		t.Errorf("expected oblique East-facing footprint centroid to be East of drone, got avgLng %f <= %f", avgLng, wp.Longitude)
	}
}

func TestComputeCameraFrustumFootprint_Errors(t *testing.T) {
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	// Altitude <= 0
	wpInvalidAlt := domain.DroneWaypoint{
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    0.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: -90.0,
	}
	if _, err := recon.ComputeCameraFrustumFootprint(wpInvalidAlt, cam); err == nil {
		t.Errorf("expected error for altitude <= 0, got nil")
	}

	// Gimbal pitch pointing above horizon (e.g. +10 deg)
	wpLookUp := domain.DroneWaypoint{
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    50.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: 10.0,
	}
	if _, err := recon.ComputeCameraFrustumFootprint(wpLookUp, cam); err == nil {
		t.Errorf("expected error when camera points above horizon, got nil")
	}

	// Invalid FOV
	wpNormal := domain.DroneWaypoint{
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    50.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: -90.0,
	}
	invalidCam := domain.CameraIntrinsics{HFOV: -10.0, VFOV: 60.0}
	if _, err := recon.ComputeCameraFrustumFootprint(wpNormal, invalidCam); err == nil {
		t.Errorf("expected error for negative HFOV, got nil")
	}
}

func TestRaycastPixelToGround_CornersAndHorizon(t *testing.T) {
	wp := domain.DroneWaypoint{
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    50.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: -90.0,
	}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	// TopLeft: u=0, v=0 should be West and North of drone
	latTL, lngTL, err := recon.RaycastPixelToGround(0.0, 0.0, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error for TopLeft: %v", err)
	}
	if latTL <= wp.Latitude || lngTL >= wp.Longitude {
		t.Errorf("expected TopLeft to be North and West of drone, got lat=%f, lng=%f", latTL, lngTL)
	}

	// BottomRight: u=1, v=1 should be East and South of drone
	latBR, lngBR, err := recon.RaycastPixelToGround(1.0, 1.0, wp, cam)
	if err != nil {
		t.Fatalf("unexpected error for BottomRight: %v", err)
	}
	if latBR >= wp.Latitude || lngBR <= wp.Longitude {
		t.Errorf("expected BottomRight to be South and East of drone, got lat=%f, lng=%f", latBR, lngBR)
	}

	// Ray pointing at or above horizon (pitch 0, v=0 points up by VFOV/2)
	wpHoriz := domain.DroneWaypoint{
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    50.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: 0.0,
	}
	if _, _, err := recon.RaycastPixelToGround(0.5, 0.0, wpHoriz, cam); err == nil {
		t.Errorf("expected error when ray points at or above horizon, got nil")
	}
}

func TestComputeSweptAreaSqMeters(t *testing.T) {
	// Construct a 100m x 100m square at latitude 37.7749
	lat := 37.7749
	lng := -122.4194
	mPerDegLat := 111132.95
	mPerDegLng := 111132.95 * math.Cos(lat*math.Pi/180.0)

	dLat := 100.0 / mPerDegLat
	dLng := 100.0 / mPerDegLng

	squarePoly := [][]float64{
		{lng, lat},
		{lng + dLng, lat},
		{lng + dLng, lat + dLat},
		{lng, lat + dLat},
		{lng, lat}, // closed
	}

	area := recon.ComputeSweptAreaSqMeters(squarePoly)
	expectedArea := 10000.0 // 100m * 100m
	if math.Abs(area-expectedArea)/expectedArea > 0.02 {
		t.Errorf("expected swept area near %f m^2, got %f (error > 2%%)", expectedArea, area)
	}

	// Degenerate polygon
	if recon.ComputeSweptAreaSqMeters(nil) != 0.0 {
		t.Errorf("expected 0 for nil polygon")
	}
	if recon.ComputeSweptAreaSqMeters([][]float64{{lng, lat}, {lng + dLng, lat}}) != 0.0 {
		t.Errorf("expected 0 for 2-point polygon")
	}
}

func TestFindIntersectingSectorIDs(t *testing.T) {
	// Footprint square roughly 100m x 100m
	lat := 37.7749
	lng := -122.4194
	mPerDegLat := 111132.95
	mPerDegLng := 111132.95 * math.Cos(lat*math.Pi/180.0)
	dLat := 100.0 / mPerDegLat
	dLng := 100.0 / mPerDegLng

	footprint := [][]float64{
		{lng - dLng, lat - dLat},
		{lng + dLng, lat - dLat},
		{lng + dLng, lat + dLat},
		{lng - dLng, lat + dLat},
		{lng - dLng, lat - dLat},
	}

	// Sector 1: Completely inside footprint
	sector1 := domain.SearchPartySector{
		SectorID: "sector-inside",
		PolygonPoints: []domain.LocationPoint{
			{Latitude: lat - 0.2*dLat, Longitude: lng - 0.2*dLng},
			{Latitude: lat - 0.2*dLat, Longitude: lng + 0.2*dLng},
			{Latitude: lat + 0.2*dLat, Longitude: lng + 0.2*dLng},
			{Latitude: lat + 0.2*dLat, Longitude: lng - 0.2*dLng},
			{Latitude: lat - 0.2*dLat, Longitude: lng - 0.2*dLng},
		},
	}

	// Sector 2: Overlapping footprint border
	sector2 := domain.SearchPartySector{
		SectorID: "sector-overlapping",
		PolygonPoints: []domain.LocationPoint{
			{Latitude: lat + 0.5*dLat, Longitude: lng + 0.5*dLng},
			{Latitude: lat + 0.5*dLat, Longitude: lng + 2.0*dLng},
			{Latitude: lat + 2.0*dLat, Longitude: lng + 2.0*dLng},
			{Latitude: lat + 2.0*dLat, Longitude: lng + 0.5*dLng},
			{Latitude: lat + 0.5*dLat, Longitude: lng + 0.5*dLng},
		},
	}

	// Sector 3: Completely outside footprint
	sector3 := domain.SearchPartySector{
		SectorID: "sector-outside",
		PolygonPoints: []domain.LocationPoint{
			{Latitude: lat + 5.0*dLat, Longitude: lng + 5.0*dLng},
			{Latitude: lat + 5.0*dLat, Longitude: lng + 6.0*dLng},
			{Latitude: lat + 6.0*dLat, Longitude: lng + 6.0*dLng},
			{Latitude: lat + 6.0*dLat, Longitude: lng + 5.0*dLng},
			{Latitude: lat + 5.0*dLat, Longitude: lng + 5.0*dLng},
		},
	}

	// Sector 4: Defined via BoundingPolygon, overlapping
	sector4 := domain.SearchPartySector{
		SectorID: "sector-bounding-poly",
		BoundingPolygon: []domain.LocationPoint{
			{Latitude: lat, Longitude: lng},
			{Latitude: lat + 0.1*dLat, Longitude: lng},
			{Latitude: lat + 0.1*dLat, Longitude: lng + 0.1*dLng},
			{Latitude: lat, Longitude: lng + 0.1*dLng},
			{Latitude: lat, Longitude: lng},
		},
	}

	sectors := []domain.SearchPartySector{sector1, sector2, sector3, sector4}
	matched := recon.FindIntersectingSectorIDs(footprint, sectors)

	expectedMap := map[string]bool{
		"sector-inside":        true,
		"sector-overlapping":   true,
		"sector-bounding-poly": true,
	}

	if len(matched) != 3 {
		t.Fatalf("expected 3 intersecting sectors, got %d: %v", len(matched), matched)
	}

	for _, id := range matched {
		if !expectedMap[id] {
			t.Errorf("unexpected sector id %q matched", id)
		}
	}
}

func TestFindIntersectingSectorIDs_EnclosingAndDegenerate(t *testing.T) {
	lat := 37.7749
	lng := -122.4194
	mPerDegLat := 111132.95
	mPerDegLng := 111132.95 * math.Cos(lat*math.Pi/180.0)
	dLat := 10.0 / mPerDegLat
	dLng := 10.0 / mPerDegLng

	// Small footprint
	smallFootprint := [][]float64{
		{lng - dLng, lat - dLat},
		{lng + dLng, lat - dLat},
		{lng + dLng, lat + dLat},
		{lng - dLng, lat + dLat},
		{lng - dLng, lat - dLat},
	}

	// Giant sector that completely encloses the footprint
	giantSector := domain.SearchPartySector{
		SectorID: "giant-enclosing",
		PolygonPoints: []domain.LocationPoint{
			{Latitude: lat - 100*dLat, Longitude: lng - 100*dLng},
			{Latitude: lat - 100*dLat, Longitude: lng + 100*dLng},
			{Latitude: lat + 100*dLat, Longitude: lng + 100*dLng},
			{Latitude: lat + 100*dLat, Longitude: lng - 100*dLng},
			{Latitude: lat - 100*dLat, Longitude: lng - 100*dLng},
		},
	}

	// Degenerate sector with only 2 points
	degenerateSector := domain.SearchPartySector{
		SectorID: "degenerate-sector",
		PolygonPoints: []domain.LocationPoint{
			{Latitude: lat, Longitude: lng},
			{Latitude: lat + dLat, Longitude: lng + dLng},
		},
	}

	matched := recon.FindIntersectingSectorIDs(smallFootprint, []domain.SearchPartySector{giantSector, degenerateSector})
	if len(matched) != 1 || matched[0] != "giant-enclosing" {
		t.Fatalf("expected giant-enclosing to match, got %v", matched)
	}

	// Empty inputs
	if len(recon.FindIntersectingSectorIDs(nil, []domain.SearchPartySector{giantSector})) != 0 {
		t.Errorf("expected empty result for nil footprint")
	}
	if len(recon.FindIntersectingSectorIDs(smallFootprint, nil)) != 0 {
		t.Errorf("expected empty result for nil sectors")
	}
}

func TestRaycastPixelToGround_ValidationErrors(t *testing.T) {
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	// Negative altitude
	wpNegAlt := domain.DroneWaypoint{
		Latitude:    37.7749,
		Longitude:   -122.4194,
		AltitudeAGL: -10.0,
	}
	if _, _, err := recon.RaycastPixelToGround(0.5, 0.5, wpNegAlt, cam); err == nil {
		t.Errorf("expected error for negative altitude, got nil")
	}

	// Invalid latitude
	wpInvalidLat := domain.DroneWaypoint{
		Latitude:    95.0,
		Longitude:   -122.4194,
		AltitudeAGL: 50.0,
	}
	if _, _, err := recon.RaycastPixelToGround(0.5, 0.5, wpInvalidLat, cam); err == nil {
		t.Errorf("expected error for lat > 90, got nil")
	}

	// Invalid longitude
	wpInvalidLng := domain.DroneWaypoint{
		Latitude:    37.7749,
		Longitude:   -195.0,
		AltitudeAGL: 50.0,
	}
	if _, _, err := recon.RaycastPixelToGround(0.5, 0.5, wpInvalidLng, cam); err == nil {
		t.Errorf("expected error for lng < -180, got nil")
	}
}

func TestComputeCameraFrustumFootprint_WithGimbalRoll(t *testing.T) {
	wp := domain.DroneWaypoint{
		Latitude:       37.7749,
		Longitude:      -122.4194,
		AltitudeAGL:    50.0,
		HeadingDeg:     0.0,
		GimbalPitchDeg: -90.0,
		GimbalRollDeg:  15.0,
	}
	cam := domain.CameraIntrinsics{HFOV: 84.0, VFOV: 60.0}

	poly, err := recon.ComputeCameraFrustumFootprint(wp, cam)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(poly) != 5 {
		t.Fatalf("expected 5 vertices, got %d", len(poly))
	}
	area := recon.ComputeSweptAreaSqMeters(poly)
	if area < 1000.0 || area > 10000.0 {
		t.Errorf("expected area ~5000 m^2 with roll, got %f", area)
	}
}
