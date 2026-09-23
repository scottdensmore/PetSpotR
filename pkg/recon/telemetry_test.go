package recon_test

import (
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/recon"
)

func TestParseDJISRT_StandardFormat(t *testing.T) {
	srtContent := `1
00:00:01,000 --> 00:00:02,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude : 37.774929] [longitude : -122.419416] [rel_alt: 45.200] [heading: 142.5] [pitch: -45.0] [roll: 0.0] [yaw: 142.5]

2
00:00:02,000 --> 00:00:03,000
[iso : 100] [shutter : 1/500] [fnum : 2.8] [latitude : 37.775010] [longitude : -122.419350] [rel_alt: 45.500] [heading: 143.0] [pitch: -45.0] [roll: 0.0] [yaw: 143.0]
`
	waypoints, err := recon.ParseDJISRT(strings.NewReader(srtContent))
	if err != nil {
		t.Fatalf("unexpected error parsing SRT: %v", err)
	}
	if len(waypoints) != 2 {
		t.Fatalf("expected 2 waypoints, got %d", len(waypoints))
	}
	wp1 := waypoints[0]
	if wp1.Latitude != 37.774929 || wp1.Longitude != -122.419416 {
		t.Errorf("expected lat/lng 37.774929/-122.419416, got %f/%f", wp1.Latitude, wp1.Longitude)
	}
	if wp1.AltitudeAGL != 45.2 {
		t.Errorf("expected altitude 45.2, got %f", wp1.AltitudeAGL)
	}
	if wp1.HeadingDeg != 142.5 || wp1.GimbalPitchDeg != -45.0 {
		t.Errorf("expected heading 142.5 and pitch -45.0, got %f / %f", wp1.HeadingDeg, wp1.GimbalPitchDeg)
	}
	if wp1.GimbalRollDeg != 0.0 || wp1.GimbalYawDeg != 142.5 {
		t.Errorf("expected roll 0.0 and yaw 142.5, got %f / %f", wp1.GimbalRollDeg, wp1.GimbalYawDeg)
	}
}

func TestParseDJISRT_AlternateFormatWithDLat(t *testing.T) {
	srtContent := `1
00:00:00,500 --> 00:00:01,000
HOME(-122.4194,37.7749) 2026.09.22 12:00:00
GPS(-122.4194,37.7749,15) [dlatitude: 37.774950] [dlongitude: -122.419400] [altitude: 50.0] [rel_alt: 40.0] [heading: 90.0]
`
	waypoints, err := recon.ParseDJISRT(strings.NewReader(srtContent))
	if err != nil {
		t.Fatalf("unexpected error parsing alternate SRT: %v", err)
	}
	if len(waypoints) != 1 {
		t.Fatalf("expected 1 waypoint, got %d", len(waypoints))
	}
	if waypoints[0].Latitude != 37.774950 || waypoints[0].Longitude != -122.419400 {
		t.Errorf("expected lat/lng 37.774950/-122.419400, got %f/%f", waypoints[0].Latitude, waypoints[0].Longitude)
	}
	if waypoints[0].AltitudeAGL != 40.0 {
		t.Errorf("unexpected waypoint altitude: %+v", waypoints[0])
	}
	if waypoints[0].HeadingDeg != 90.0 {
		t.Errorf("expected heading 90.0, got %f", waypoints[0].HeadingDeg)
	}
	expectedTime := time.Date(2026, 9, 22, 12, 0, 0, 500*int(time.Millisecond), time.UTC)
	if !waypoints[0].Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got %v", expectedTime, waypoints[0].Timestamp)
	}
}

func TestParseDJISRT_AutelFormat(t *testing.T) {
	srtContent := `1
00:00:01,000 --> 00:00:02,000
2026-09-22 14:30:15
Lat: 37.776100 Long: -122.418200 Alt: 55.5m
Gimbal: Pitch: -60.0 Roll: 1.5 Yaw: 180.0
`
	waypoints, err := recon.ParseDJISRT(strings.NewReader(srtContent))
	if err != nil {
		t.Fatalf("unexpected error parsing Autel SRT: %v", err)
	}
	if len(waypoints) != 1 {
		t.Fatalf("expected 1 waypoint, got %d", len(waypoints))
	}
	wp := waypoints[0]
	if wp.Latitude != 37.776100 || wp.Longitude != -122.418200 {
		t.Errorf("expected lat/lng 37.776100/-122.418200, got %f/%f", wp.Latitude, wp.Longitude)
	}
	if wp.AltitudeAGL != 55.5 {
		t.Errorf("expected altitude 55.5, got %f", wp.AltitudeAGL)
	}
	if wp.GimbalPitchDeg != -60.0 || wp.GimbalRollDeg != 1.5 || wp.GimbalYawDeg != 180.0 {
		t.Errorf("unexpected gimbal angles: %+v", wp)
	}
}

func TestParseFlightLog_SRTFile(t *testing.T) {
	srtContent := []byte(`1
00:00:01,000 --> 00:00:02,000
[latitude: 37.7749] [longitude: -122.4194] [rel_alt: 30.0] [heading: 45.0] [pitch: -90.0]
`)
	waypoints, err := recon.ParseFlightLog(srtContent, "flight_record.srt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waypoints) != 1 {
		t.Fatalf("expected 1 waypoint, got %d", len(waypoints))
	}
	if waypoints[0].Latitude != 37.7749 || waypoints[0].Longitude != -122.4194 {
		t.Errorf("unexpected coords: %f, %f", waypoints[0].Latitude, waypoints[0].Longitude)
	}
}

func TestParseFlightLog_KMLFile(t *testing.T) {
	kmlContent := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<kml xmlns="http://www.opengis.net/kml/2.2">
  <Document>
    <Placemark>
      <LineString>
        <coordinates>
          -122.4194,37.7749,45.0
          -122.4193,37.7750,48.0
        </coordinates>
      </LineString>
    </Placemark>
  </Document>
</kml>`)
	waypoints, err := recon.ParseFlightLog(kmlContent, "flight_path.kml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waypoints) != 2 {
		t.Fatalf("expected 2 waypoints, got %d", len(waypoints))
	}
	if waypoints[0].Latitude != 37.7749 || waypoints[0].Longitude != -122.4194 || waypoints[0].AltitudeAGL != 45.0 {
		t.Errorf("unexpected waypoint 0: %+v", waypoints[0])
	}
	if waypoints[1].Latitude != 37.7750 || waypoints[1].Longitude != -122.4193 || waypoints[1].AltitudeAGL != 48.0 {
		t.Errorf("unexpected waypoint 1: %+v", waypoints[1])
	}
}

func TestParseFlightLog_GeoJSONFile(t *testing.T) {
	geoJSONContent := []byte(`{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "geometry": {
        "type": "LineString",
        "coordinates": [
          [-122.4194, 37.7749, 50.0],
          [-122.4192, 37.7751, 52.0]
        ]
      },
      "properties": {
        "heading": 90.0,
        "pitch": -45.0
      }
    }
  ]
}`)
	waypoints, err := recon.ParseFlightLog(geoJSONContent, "flight.geojson")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waypoints) != 2 {
		t.Fatalf("expected 2 waypoints, got %d", len(waypoints))
	}
	if waypoints[0].Latitude != 37.7749 || waypoints[0].Longitude != -122.4194 || waypoints[0].AltitudeAGL != 50.0 {
		t.Errorf("unexpected waypoint 0: %+v", waypoints[0])
	}
	if waypoints[0].HeadingDeg != 90.0 || waypoints[0].GimbalPitchDeg != -45.0 {
		t.Errorf("unexpected attitude: heading=%f, pitch=%f", waypoints[0].HeadingDeg, waypoints[0].GimbalPitchDeg)
	}
}

func TestParseFlightLog_EmptyContent(t *testing.T) {
	_, err := recon.ParseFlightLog([]byte(""), "empty.srt")
	if err == nil {
		t.Fatal("expected error on empty flight log, got nil")
	}
}

func TestParseFlightLog_BareGeoJSONLineString(t *testing.T) {
	geoJSONContent := []byte(`{
  "type": "LineString",
  "coordinates": [
    [-122.4194, 37.7749, 50.0],
    [-122.4194, 37.7759, 52.0]
  ]
}`)
	waypoints, err := recon.ParseFlightLog(geoJSONContent, "bare_linestring.geojson")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(waypoints) != 2 {
		t.Fatalf("expected 2 waypoints, got %d", len(waypoints))
	}
	// From (37.7749, -122.4194) to (37.7759, -122.4194) is due north (0 degrees)
	if waypoints[0].HeadingDeg < -0.1 || waypoints[0].HeadingDeg > 0.1 {
		t.Errorf("expected heading ~0.0 deg (North), got %f", waypoints[0].HeadingDeg)
	}
}
