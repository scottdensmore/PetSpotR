package webhook

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

// Atom and GeoRSS namespace constants.
const (
	AtomNamespace   = "http://www.w3.org/2005/Atom"
	GeoRSSNamespace = "http://www.georss.org/georss"
)

type atomAuthor struct {
	Name string `xml:"name"`
}

type atomFeed struct {
	XMLName     xml.Name    `xml:"feed"`
	Xmlns       string      `xml:"xmlns,attr"`
	XmlnsGeoRSS string      `xml:"xmlns:georss,attr"`
	Title       string      `xml:"title"`
	ID          string      `xml:"id"`
	Updated     string      `xml:"updated"`
	Author      atomAuthor  `xml:"author"`
	Link        atomLink    `xml:"link"`
	Entries     []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr,omitempty"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	Title       string   `xml:"title"`
	Link        atomLink `xml:"link"`
	ID          string   `xml:"id"`
	Updated     string   `xml:"updated"`
	Summary     string   `xml:"summary"`
	GeoRSSPoint string   `xml:"georss:point,omitempty"`
}

// GenerateLostPetsAtomFeed serializes active lost-pet reports into an RFC 4287 Atom 1.0
// compliant XML feed with W3C GeoRSS point coordinates and optional radial geofence filtering.
func GenerateLostPetsAtomFeed(pets []domain.LostPetRecord, baseURL string, fence *GeoFence) (string, error) {
	base := strings.TrimRight(baseURL, "/")

	var entries []atomEntry
	var latestTime time.Time

	for _, pet := range pets {
		if !isWithinFence(pet.Coordinates, fence) {
			continue
		}

		if pet.ReportedAt.After(latestTime) {
			latestTime = pet.ReportedAt
		}

		updatedStr := time.Now().UTC().Format(time.RFC3339)
		if !pet.ReportedAt.IsZero() {
			updatedStr = pet.ReportedAt.UTC().Format(time.RFC3339)
		}

		linkHref := base + "/pets/" + pet.PetID
		if base == "" {
			linkHref = "/pets/" + pet.PetID
		}

		summary := strings.TrimSpace(pet.Description)
		if summary == "" {
			summary = "Lost pet reported"
		}

		entry := atomEntry{
			Title: formatLostPetTitle(pet),
			Link: atomLink{
				Href: linkHref,
			},
			ID:          "urn:uuid:" + pet.PetID,
			Updated:     updatedStr,
			Summary:     summary,
			GeoRSSPoint: formatGeoRSSPoint(pet.Coordinates),
		}
		entries = append(entries, entry)
	}

	updatedFeed := time.Now().UTC().Format(time.RFC3339)
	if !latestTime.IsZero() {
		updatedFeed = latestTime.UTC().Format(time.RFC3339)
	}

	feedHref := base + "/feeds/lost-pets.atom"
	if base == "" {
		feedHref = "/feeds/lost-pets.atom"
	}

	feed := atomFeed{
		Xmlns:       AtomNamespace,
		XmlnsGeoRSS: GeoRSSNamespace,
		Title:       "PetSpotR - Lost Pets",
		ID:          "urn:petspotr:feeds:lost-pets",
		Updated:     updatedFeed,
		Author:      atomAuthor{Name: "PetSpotR"},
		Link: atomLink{
			Rel:  "self",
			Href: feedHref,
		},
		Entries: entries,
	}

	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal lost-pets atom feed XML: %w", err)
	}

	return xml.Header + string(data), nil
}

// GenerateSightingsAtomFeed serializes community sightings into an RFC 4287 Atom 1.0
// compliant XML feed with W3C GeoRSS point coordinates and optional radial geofence filtering.
func GenerateSightingsAtomFeed(sightings []domain.SightingRecord, baseURL string, fence *GeoFence) (string, error) {
	base := strings.TrimRight(baseURL, "/")

	var entries []atomEntry
	var latestTime time.Time

	for _, sighting := range sightings {
		if !isWithinFence(sighting.Coordinates, fence) {
			continue
		}

		timestamp := sighting.ReportedAt
		if timestamp.IsZero() {
			timestamp = sighting.SightedAt
		}
		if timestamp.After(latestTime) {
			latestTime = timestamp
		}

		updatedStr := time.Now().UTC().Format(time.RFC3339)
		if !timestamp.IsZero() {
			updatedStr = timestamp.UTC().Format(time.RFC3339)
		}

		linkHref := base + "/pets/" + sighting.LostPetID
		if base == "" {
			linkHref = "/pets/" + sighting.LostPetID
		}
		if sighting.LostPetID == "" {
			linkHref = base + "/sightings/" + sighting.SightingID
			if base == "" {
				linkHref = "/sightings/" + sighting.SightingID
			}
		}

		entry := atomEntry{
			Title: formatSightingTitle(sighting),
			Link: atomLink{
				Href: linkHref,
			},
			ID:          "urn:uuid:" + sighting.SightingID,
			Updated:     updatedStr,
			Summary:     formatSightingSummary(sighting),
			GeoRSSPoint: formatGeoRSSPoint(sighting.Coordinates),
		}
		entries = append(entries, entry)
	}

	updatedFeed := time.Now().UTC().Format(time.RFC3339)
	if !latestTime.IsZero() {
		updatedFeed = latestTime.UTC().Format(time.RFC3339)
	}

	feedHref := base + "/feeds/sightings.atom"
	if base == "" {
		feedHref = "/feeds/sightings.atom"
	}

	feed := atomFeed{
		Xmlns:       AtomNamespace,
		XmlnsGeoRSS: GeoRSSNamespace,
		Title:       "PetSpotR - Community Sightings",
		ID:          "urn:petspotr:feeds:sightings",
		Updated:     updatedFeed,
		Author:      atomAuthor{Name: "PetSpotR"},
		Link: atomLink{
			Rel:  "self",
			Href: feedHref,
		},
		Entries: entries,
	}

	data, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal sightings atom feed XML: %w", err)
	}

	return xml.Header + string(data), nil
}

// GenerateAtomFeed provides a convenience wrapper generating a lost-pets Atom 1.0 feed
// with GeoRSS annotations using default parameters.
func GenerateAtomFeed(pets []domain.LostPetRecord) (string, error) {
	return GenerateLostPetsAtomFeed(pets, "", nil)
}

func isWithinFence(coords *domain.LocationPoint, fence *GeoFence) bool {
	if fence == nil {
		return true
	}
	if coords == nil {
		return false
	}
	center := domain.LocationPoint{
		Latitude:  fence.CenterLat,
		Longitude: fence.CenterLng,
	}
	dist := domain.HaversineDistanceMiles(center, *coords)
	return dist <= fence.RadiusMiles
}

func formatGeoRSSPoint(point *domain.LocationPoint) string {
	if point == nil {
		return ""
	}
	return fmt.Sprintf("%s %s",
		strconv.FormatFloat(point.Latitude, 'f', -1, 64),
		strconv.FormatFloat(point.Longitude, 'f', -1, 64),
	)
}

func formatLostPetTitle(pet domain.LostPetRecord) string {
	species := strings.TrimSpace(pet.Species)
	if species == "" {
		species = "Pet"
	}
	name := strings.TrimSpace(pet.PetName)
	breed := strings.TrimSpace(pet.Breed)

	switch {
	case name != "" && breed != "":
		return fmt.Sprintf("Lost %s: %s (%s)", species, name, breed)
	case name != "" && breed == "":
		return fmt.Sprintf("Lost %s: %s", species, name)
	case name == "" && breed != "":
		return fmt.Sprintf("Lost %s (%s)", species, breed)
	default:
		return fmt.Sprintf("Lost %s", species)
	}
}

func formatSightingTitle(sighting domain.SightingRecord) string {
	loc := strings.TrimSpace(sighting.LocationDescription)
	if loc != "" {
		return fmt.Sprintf("Sighting: %s", loc)
	}
	petID := strings.TrimSpace(sighting.LostPetID)
	if petID != "" {
		return fmt.Sprintf("Sighting: Pet %s", petID)
	}
	return "Sighting"
}

func formatSightingSummary(sighting domain.SightingRecord) string {
	notes := strings.TrimSpace(sighting.Notes)
	if notes != "" {
		return notes
	}
	loc := strings.TrimSpace(sighting.LocationDescription)
	if loc != "" {
		return fmt.Sprintf("Sighting reported near %s", loc)
	}
	return "Community sighting report"
}
