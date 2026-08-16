package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// FoundPetReportedPayloadVersion is the current found-pet integration payload.
// Payload version 1 remains represented by FoundPetEvent for in-flight message
// compatibility.
const (
	FoundPetReportedLegacyPayloadVersion = 1
	FoundPetReportedPayloadVersion       = 2
)

// FoundPetStatus is the persisted lifecycle state of a found-pet report.
type FoundPetStatus string

const (
	// FoundPetStatusFound identifies an active report for a pet that was found.
	FoundPetStatusFound FoundPetStatus = "found"
)

// CustodyStatus records where the found pet is currently located.
type CustodyStatus string

const (
	CustodyUnknown       CustodyStatus = "Unknown"
	CustodyFinderHome    CustodyStatus = "Finder Home"
	CustodyLocalShelter  CustodyStatus = "Local Shelter"
	CustodyAnimalControl CustodyStatus = "Animal Control"
	CustodySightedOnly   CustodyStatus = "Sighted Only"
)

// FoundPetReport is the canonical persisted aggregate for a found-pet report.
// FinderEmail remains private state and is deliberately omitted from
// PublicFoundPetReport and FoundPetReportedV2.
type FoundPetReport struct {
	PetID               string          `json:"petId"`
	ImageURL            string          `json:"imageUrl,omitempty"`
	ImageObject         string          `json:"imageObject,omitempty"`
	FoundAt             time.Time       `json:"foundAt"`
	Location            string          `json:"location"`
	GeocodingStatus     GeocodingStatus `json:"geocodingStatus"`
	Coordinates         *LocationPoint  `json:"coordinates,omitempty"`
	FinderEmail         string          `json:"finderEmail,omitempty"`
	Species             string          `json:"species,omitempty"`
	Breed               string          `json:"breed,omitempty"`
	PrimaryColor        string          `json:"primaryColor,omitempty"`
	SecondaryColor      string          `json:"secondaryColor,omitempty"`
	DistinctiveMarkings []string        `json:"distinctiveMarkings,omitempty"`
	CustodyStatus       CustodyStatus   `json:"custodyStatus"`
	Status              FoundPetStatus  `json:"status"`
}

// FoundPetReportedV2 is the additive payload-v2 integration event. Its legacy
// fields retain their original JSON names so payload-v1 readers can continue
// decoding the image and location fields they understand.
type FoundPetReportedV2 struct {
	PetID               string          `json:"petId"`
	ImageURL            string          `json:"imageUrl,omitempty"`
	ImageObject         string          `json:"imageObject,omitempty"`
	FoundAt             time.Time       `json:"foundAt"`
	Location            string          `json:"location"`
	GeocodingStatus     GeocodingStatus `json:"geocodingStatus"`
	Coordinates         *LocationPoint  `json:"coordinates,omitempty"`
	Species             string          `json:"species,omitempty"`
	Breed               string          `json:"breed,omitempty"`
	PrimaryColor        string          `json:"primaryColor,omitempty"`
	SecondaryColor      string          `json:"secondaryColor,omitempty"`
	DistinctiveMarkings []string        `json:"distinctiveMarkings,omitempty"`
	CustodyStatus       CustodyStatus   `json:"custodyStatus"`
	Status              FoundPetStatus  `json:"status"`
}

// PublicFoundPetReport is the unauthenticated found-pet listing DTO. It cannot
// serialize finder contact because that field is absent by type.
type PublicFoundPetReport struct {
	PetID               string          `json:"petId"`
	ImageURL            string          `json:"imageUrl,omitempty"`
	ImageObject         string          `json:"imageObject,omitempty"`
	FoundAt             time.Time       `json:"foundAt"`
	Location            string          `json:"location"`
	GeocodingStatus     GeocodingStatus `json:"geocodingStatus,omitempty"`
	Coordinates         *LocationPoint  `json:"coordinates,omitempty"`
	Species             string          `json:"species,omitempty"`
	Breed               string          `json:"breed,omitempty"`
	PrimaryColor        string          `json:"primaryColor,omitempty"`
	SecondaryColor      string          `json:"secondaryColor,omitempty"`
	DistinctiveMarkings []string        `json:"distinctiveMarkings,omitempty"`
	CustodyStatus       CustodyStatus   `json:"custodyStatus,omitempty"`
	Status              FoundPetStatus  `json:"status,omitempty"`
}

// NormalizeFoundPetReport canonicalizes user-supplied values before
// validation, persistence, event identity derivation, and retry comparison.
func NormalizeFoundPetReport(report FoundPetReport) FoundPetReport {
	report.PetID = strings.TrimSpace(report.PetID)
	report.ImageURL = strings.TrimSpace(report.ImageURL)
	report.ImageObject = strings.TrimSpace(report.ImageObject)
	if !report.FoundAt.IsZero() {
		report.FoundAt = report.FoundAt.UTC()
	}
	report.Location = strings.TrimSpace(report.Location)
	report.FinderEmail = strings.ToLower(strings.TrimSpace(report.FinderEmail))
	report.Species = normalizeSpecies(report.Species)
	report.Breed = strings.TrimSpace(report.Breed)
	report.PrimaryColor = strings.TrimSpace(report.PrimaryColor)
	report.SecondaryColor = strings.TrimSpace(report.SecondaryColor)
	report.DistinctiveMarkings = normalizeMarkings(report.DistinctiveMarkings)
	report.CustodyStatus = normalizeCustodyStatus(report.CustodyStatus)
	if report.Status == "" {
		report.Status = FoundPetStatusFound
	}
	if report.GeocodingStatus == "" {
		if report.Location == "" {
			report.GeocodingStatus = GeocodingUnavailable
		} else {
			report.GeocodingStatus = GeocodingPending
		}
	}
	return report
}

// Validate checks the canonical found-pet aggregate at the application boundary.
func (r FoundPetReport) Validate() error {
	legacy := FoundPetEvent{
		PetID:       r.PetID,
		ImageURL:    r.ImageURL,
		ImageObject: r.ImageObject,
		FoundAt:     r.FoundAt,
		Location:    r.Location,
	}
	if err := legacy.Validate(); err != nil {
		return err
	}
	if r.Location == "" {
		return errors.New("foundpet: location is required")
	}
	if r.FinderEmail != "" && (!strings.Contains(r.FinderEmail, "@") || !strings.Contains(r.FinderEmail, ".")) {
		return fmt.Errorf("domain: invalid finderEmail address: %s", r.FinderEmail)
	}
	if err := validateFoundPetLengths(r); err != nil {
		return err
	}
	if r.Species != "" && r.Species != "Dog" && r.Species != "Cat" && r.Species != "Bird" && r.Species != "Other" {
		return fmt.Errorf("domain: unsupported species %q", r.Species)
	}
	switch r.CustodyStatus {
	case CustodyUnknown, CustodyFinderHome, CustodyLocalShelter, CustodyAnimalControl, CustodySightedOnly:
	default:
		return fmt.Errorf("domain: unsupported custody status %q", r.CustodyStatus)
	}
	if r.Status != FoundPetStatusFound {
		return fmt.Errorf("domain: unsupported found-pet status %q", r.Status)
	}
	switch r.GeocodingStatus {
	case GeocodingPending:
		if r.Coordinates != nil {
			return errors.New("domain: pending geocoding cannot include coordinates")
		}
	case GeocodingUnavailable:
		if r.Coordinates != nil {
			return errors.New("domain: unavailable geocoding cannot include coordinates")
		}
	case GeocodingVerified:
		if r.Coordinates == nil {
			return errors.New("domain: verified geocoding requires coordinates")
		}
		if err := r.Coordinates.Validate(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("domain: unsupported geocoding status %q", r.GeocodingStatus)
	}
	return nil
}

// Public returns the redacted listing representation of the aggregate.
func (r FoundPetReport) Public() PublicFoundPetReport {
	return PublicFoundPetReport{
		PetID:               r.PetID,
		ImageURL:            r.ImageURL,
		ImageObject:         r.ImageObject,
		FoundAt:             r.FoundAt,
		Location:            r.Location,
		GeocodingStatus:     r.GeocodingStatus,
		Coordinates:         cloneLocationPoint(r.Coordinates),
		Species:             r.Species,
		Breed:               r.Breed,
		PrimaryColor:        r.PrimaryColor,
		SecondaryColor:      r.SecondaryColor,
		DistinctiveMarkings: append([]string(nil), r.DistinctiveMarkings...),
		CustodyStatus:       r.CustodyStatus,
		Status:              r.Status,
	}
}

// ReportedEvent returns the contact-redacted payload-v2 integration event.
func (r FoundPetReport) ReportedEvent() FoundPetReportedV2 {
	return FoundPetReportedV2{
		PetID:               r.PetID,
		ImageURL:            r.ImageURL,
		ImageObject:         r.ImageObject,
		FoundAt:             r.FoundAt,
		Location:            r.Location,
		GeocodingStatus:     r.GeocodingStatus,
		Coordinates:         cloneLocationPoint(r.Coordinates),
		Species:             r.Species,
		Breed:               r.Breed,
		PrimaryColor:        r.PrimaryColor,
		SecondaryColor:      r.SecondaryColor,
		DistinctiveMarkings: append([]string(nil), r.DistinctiveMarkings...),
		CustodyStatus:       r.CustodyStatus,
		Status:              r.Status,
	}
}

func normalizeCustodyStatus(status CustodyStatus) CustodyStatus {
	switch strings.ToLower(strings.TrimSpace(string(status))) {
	case "":
		return CustodyUnknown
	case "unknown":
		return CustodyUnknown
	case "finder home":
		return CustodyFinderHome
	case "local shelter":
		return CustodyLocalShelter
	case "animal control":
		return CustodyAnimalControl
	case "sighted only":
		return CustodySightedOnly
	default:
		return CustodyStatus(strings.TrimSpace(string(status)))
	}
}

func normalizeMarkings(markings []string) []string {
	if len(markings) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(markings))
	seen := make(map[string]struct{}, len(markings))
	for _, marking := range markings {
		marking = strings.TrimSpace(marking)
		if marking == "" {
			continue
		}
		if _, exists := seen[marking]; exists {
			continue
		}
		seen[marking] = struct{}{}
		normalized = append(normalized, marking)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func validateFoundPetLengths(r FoundPetReport) error {
	fields := []struct {
		name  string
		value string
		limit int
	}{
		{name: "petId", value: r.PetID, limit: 256},
		{name: "imageUrl", value: r.ImageURL, limit: 1_000_000},
		{name: "imageObject", value: r.ImageObject, limit: 1024},
		{name: "location", value: r.Location, limit: 500},
		{name: "finderEmail", value: r.FinderEmail, limit: 320},
		{name: "species", value: r.Species, limit: 32},
		{name: "breed", value: r.Breed, limit: 200},
		{name: "primaryColor", value: r.PrimaryColor, limit: 100},
		{name: "secondaryColor", value: r.SecondaryColor, limit: 100},
	}
	for _, field := range fields {
		if utf8.RuneCountInString(field.value) > field.limit {
			return fmt.Errorf("domain: %s exceeds %d characters", field.name, field.limit)
		}
	}
	if len(r.DistinctiveMarkings) > 20 {
		return errors.New("domain: distinctiveMarkings exceeds 20 items")
	}
	for _, marking := range r.DistinctiveMarkings {
		if utf8.RuneCountInString(marking) > 200 {
			return errors.New("domain: distinctiveMarkings item exceeds 200 characters")
		}
	}
	return nil
}
