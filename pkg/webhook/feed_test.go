package webhook_test

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/webhook"
)

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	XMLNS   string      `xml:"xmlns,attr"`
	GeoRSS  string      `xml:"georss,attr"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Link    atomLink    `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	Title   string   `xml:"title"`
	Link    atomLink `xml:"link"`
	ID      string   `xml:"id"`
	Updated string   `xml:"updated"`
	Summary string   `xml:"summary"`
	Point   string   `xml:"point"`
}

func TestFeed_GenerateLostPetsAtomFeed_FormatAndNamespaces(t *testing.T) {
	t.Parallel()

	reportedAt := time.Date(2026, 9, 19, 18, 0, 0, 0, time.UTC)
	pets := []domain.LostPetRecord{
		{
			PetID:       "lost-789",
			PetName:     "Rusty",
			Species:     "Dog",
			Breed:       "Golden Retriever",
			Description: "Friendly golden retriever wearing a blue collar.",
			ReportedAt:  reportedAt,
			Coordinates: &domain.LocationPoint{Latitude: 47.613, Longitude: -122.337},
			Status:      domain.LostPetStatusLost,
		},
		{
			PetID:       "lost-456",
			PetName:     "Milo",
			Species:     "Cat",
			Breed:       "Tabby",
			Description: "Orange striped cat, very shy.",
			ReportedAt:  reportedAt.Add(-2 * time.Hour),
			Coordinates: nil, // no coordinates
			Status:      domain.LostPetStatusLost,
		},
	}

	baseURL := "https://petspotr.example.com"
	xmlStr, err := webhook.GenerateLostPetsAtomFeed(pets, baseURL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Verify XML declaration and namespaces
	if !strings.HasPrefix(xmlStr, "<?xml") {
		t.Error("expected XML declaration header")
	}
	if !strings.Contains(xmlStr, `xmlns="http://www.w3.org/2005/Atom"`) {
		t.Error("expected Atom xmlns namespace")
	}
	if !strings.Contains(xmlStr, `xmlns:georss="http://www.georss.org/georss"`) {
		t.Error("expected georss namespace")
	}

	// 2. Parse using encoding/xml to verify well-formed Atom 1.0 XML
	var parsed atomFeed
	if err := xml.Unmarshal([]byte(xmlStr), &parsed); err != nil {
		t.Fatalf("failed to parse generated Atom feed XML: %v", err)
	}

	if parsed.Title != "PetSpotR - Lost Pets" {
		t.Errorf("expected feed title 'PetSpotR - Lost Pets', got %q", parsed.Title)
	}
	if parsed.ID != "urn:petspotr:feeds:lost-pets" {
		t.Errorf("expected feed id 'urn:petspotr:feeds:lost-pets', got %q", parsed.ID)
	}
	if parsed.Link.Rel != "self" || parsed.Link.Href != "https://petspotr.example.com/feeds/lost-pets.atom" {
		t.Errorf("unexpected self link: rel=%q href=%q", parsed.Link.Rel, parsed.Link.Href)
	}
	if len(parsed.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(parsed.Entries))
	}

	// Entry 1: Rusty with coordinates
	entry1 := parsed.Entries[0]
	if entry1.ID != "urn:uuid:lost-789" {
		t.Errorf("expected entry ID 'urn:uuid:lost-789', got %q", entry1.ID)
	}
	if entry1.Title != "Lost Dog: Rusty (Golden Retriever)" {
		t.Errorf("expected title 'Lost Dog: Rusty (Golden Retriever)', got %q", entry1.Title)
	}
	if entry1.Link.Href != "https://petspotr.example.com/pets/lost-789" {
		t.Errorf("expected link href 'https://petspotr.example.com/pets/lost-789', got %q", entry1.Link.Href)
	}
	if entry1.Summary != "Friendly golden retriever wearing a blue collar." {
		t.Errorf("expected summary, got %q", entry1.Summary)
	}
	if entry1.Updated != reportedAt.Format(time.RFC3339) {
		t.Errorf("expected updated %q, got %q", reportedAt.Format(time.RFC3339), entry1.Updated)
	}
	if !strings.Contains(xmlStr, "<georss:point>47.613 -122.337</georss:point>") {
		t.Errorf("expected georss:point tag with coords in XML, got:\n%s", xmlStr)
	}

	// Entry 2: Milo without coordinates
	entry2 := parsed.Entries[1]
	if entry2.ID != "urn:uuid:lost-456" {
		t.Errorf("expected entry ID 'urn:uuid:lost-456', got %q", entry2.ID)
	}
	if entry2.Title != "Lost Cat: Milo (Tabby)" {
		t.Errorf("expected title 'Lost Cat: Milo (Tabby)', got %q", entry2.Title)
	}
	if entry2.Point != "" {
		t.Errorf("expected empty georss point for Milo, got %q", entry2.Point)
	}
}

func TestFeed_GenerateLostPetsAtomFeed_GeofenceFiltering(t *testing.T) {
	t.Parallel()

	// Capitol Hill Seattle center: 47.615, -122.320
	fence := &webhook.GeoFence{
		CenterLat:   47.615,
		CenterLng:   -122.320,
		RadiusMiles: 5.0,
	}

	pets := []domain.LostPetRecord{
		{
			PetID:       "pet-inside-1",
			PetName:     "Bella",
			Species:     "Dog",
			Breed:       "Poodle",
			Description: "Lost in Capitol Hill",
			ReportedAt:  time.Now().UTC(),
			Coordinates: &domain.LocationPoint{Latitude: 47.616, Longitude: -122.321}, // ~0.1 miles away
		},
		{
			PetID:       "pet-outside-tacoma",
			PetName:     "Rocky",
			Species:     "Dog",
			Breed:       "Boxer",
			Description: "Lost in Tacoma",
			ReportedAt:  time.Now().UTC(),
			Coordinates: &domain.LocationPoint{Latitude: 47.2529, Longitude: -122.4443}, // ~25 miles away
		},
		{
			PetID:       "pet-no-coords",
			PetName:     "Whiskers",
			Species:     "Cat",
			Breed:       "Persian",
			Description: "No coordinates known",
			ReportedAt:  time.Now().UTC(),
			Coordinates: nil,
		},
	}

	xmlStr, err := webhook.GenerateLostPetsAtomFeed(pets, "https://petspotr.io", fence)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed atomFeed
	if err := xml.Unmarshal([]byte(xmlStr), &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(parsed.Entries) != 1 {
		t.Fatalf("expected exactly 1 entry within geofence, got %d", len(parsed.Entries))
	}
	if parsed.Entries[0].ID != "urn:uuid:pet-inside-1" {
		t.Errorf("expected pet-inside-1, got %q", parsed.Entries[0].ID)
	}
}

func TestFeed_GenerateLostPetsAtomFeed_EmptyAndNilFence(t *testing.T) {
	t.Parallel()

	xmlStr, err := webhook.GenerateLostPetsAtomFeed(nil, "https://petspotr.io", nil)
	if err != nil {
		t.Fatalf("unexpected error for empty feed: %v", err)
	}

	var parsed atomFeed
	if err := xml.Unmarshal([]byte(xmlStr), &parsed); err != nil {
		t.Fatalf("failed to unmarshal empty feed: %v", err)
	}

	if parsed.Title != "PetSpotR - Lost Pets" {
		t.Errorf("unexpected title: %q", parsed.Title)
	}
	if len(parsed.Entries) != 0 {
		t.Errorf("expected 0 entries for empty list, got %d", len(parsed.Entries))
	}
	if parsed.Updated == "" {
		t.Error("expected non-empty updated timestamp")
	}
}

func TestFeed_GenerateSightingsAtomFeed_FormatAndGeofence(t *testing.T) {
	t.Parallel()

	sightedAt := time.Date(2026, 9, 19, 19, 30, 0, 0, time.UTC)
	sightings := []domain.SightingRecord{
		{
			SightingID:          "sight-001",
			LostPetID:           "lost-789",
			ReportedAt:          sightedAt,
			SightedAt:           sightedAt,
			LocationDescription: "Near Cal Anderson Park",
			Coordinates:         &domain.LocationPoint{Latitude: 47.6174, Longitude: -122.3197},
			MovementDirection:   "Heading North",
			Notes:               "Wearing a blue collar, seemed friendly.",
			Status:              domain.SightingStatusActive,
		},
		{
			SightingID:          "sight-002",
			LostPetID:           "lost-999",
			ReportedAt:          sightedAt,
			SightedAt:           sightedAt,
			LocationDescription: "Point Defiance Zoo, Tacoma",
			Coordinates:         &domain.LocationPoint{Latitude: 47.3060, Longitude: -122.5220},
			MovementDirection:   "Stationary",
			Notes:               "Seen resting near the entrance.",
			Status:              domain.SightingStatusActive,
		},
	}

	// 1. Without fence: all included
	xmlStr, err := webhook.GenerateSightingsAtomFeed(sightings, "https://petspotr.io", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(xmlStr, `xmlns="http://www.w3.org/2005/Atom"`) {
		t.Error("missing Atom namespace")
	}
	if !strings.Contains(xmlStr, `xmlns:georss="http://www.georss.org/georss"`) {
		t.Error("missing GeoRSS namespace")
	}
	if !strings.Contains(xmlStr, "<georss:point>47.6174 -122.3197</georss:point>") {
		t.Errorf("expected georss point in XML, got:\n%s", xmlStr)
	}

	var parsed atomFeed
	if err := xml.Unmarshal([]byte(xmlStr), &parsed); err != nil {
		t.Fatalf("failed to unmarshal sightings feed: %v", err)
	}

	if parsed.ID != "urn:petspotr:feeds:sightings" {
		t.Errorf("expected feed id 'urn:petspotr:feeds:sightings', got %q", parsed.ID)
	}
	if len(parsed.Entries) != 2 {
		t.Fatalf("expected 2 sightings entries, got %d", len(parsed.Entries))
	}

	// 2. With geofence centered on Capitol Hill Seattle (5 mile radius)
	fence := &webhook.GeoFence{
		CenterLat:   47.615,
		CenterLng:   -122.320,
		RadiusMiles: 5.0,
	}

	fencedXML, err := webhook.GenerateSightingsAtomFeed(sightings, "https://petspotr.io", fence)
	if err != nil {
		t.Fatalf("unexpected error with fence: %v", err)
	}

	var fencedParsed atomFeed
	if err := xml.Unmarshal([]byte(fencedXML), &fencedParsed); err != nil {
		t.Fatalf("failed to unmarshal fenced feed: %v", err)
	}

	if len(fencedParsed.Entries) != 1 {
		t.Fatalf("expected 1 entry within geofence, got %d", len(fencedParsed.Entries))
	}
	if fencedParsed.Entries[0].ID != "urn:uuid:sight-001" {
		t.Errorf("expected sight-001, got %q", fencedParsed.Entries[0].ID)
	}
}

func TestFeed_AtomGeneration_ConvenienceWrapper(t *testing.T) {
	t.Parallel()

	pets := []domain.LostPetRecord{
		{
			PetID:       "lost-001",
			PetName:     "Buddy",
			Species:     "Dog",
			Breed:       "Beagle",
			Description: "Tri-color beagle",
			ReportedAt:  time.Now().UTC(),
			Coordinates: &domain.LocationPoint{Latitude: 47.61, Longitude: -122.33},
			Status:      domain.LostPetStatusLost,
		},
	}

	feedXML, err := webhook.GenerateAtomFeed(pets)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(feedXML, "<georss:point>") {
		t.Error("expected georss point in output")
	}
}
