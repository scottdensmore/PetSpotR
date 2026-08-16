package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// LostPetReportedPayloadVersion is the current contact-redacted integration
// payload. Versions 1 and 2 remain readable for in-flight compatibility.
const (
	LostPetReportedLegacyPayloadVersion  = 1
	LostPetReportedContactPayloadVersion = 2
	LostPetReportedPayloadVersion        = 3
)

// LostPetStatus is the persisted lifecycle state of a lost-pet report.
type LostPetStatus string

const (
	// LostPetStatusLost identifies an active report for a pet that is still lost.
	LostPetStatusLost LostPetStatus = "lost"
)

// GeocodingStatus records whether a user-supplied location has verified
// coordinates. Pending locations must never receive invented coordinates.
type GeocodingStatus string

const (
	GeocodingPending     GeocodingStatus = "pending"
	GeocodingVerified    GeocodingStatus = "verified"
	GeocodingUnavailable GeocodingStatus = "unavailable"
)

// LostPetReport is the canonical application-boundary model for a lost-pet
// report. Persisted separates its private contact into ReportContact.
type LostPetReport struct {
	PetID           string          `json:"petId"`
	PetName         string          `json:"petName,omitempty"`
	Species         string          `json:"species,omitempty"`
	Breed           string          `json:"breed,omitempty"`
	PrimaryColor    string          `json:"primaryColor,omitempty"`
	Description     string          `json:"description,omitempty"`
	ReporterEmail   string          `json:"reporterEmail"`
	Phone           string          `json:"phone,omitempty"`
	ReportedAt      time.Time       `json:"reportedAt"`
	Location        string          `json:"location"`
	GeocodingStatus GeocodingStatus `json:"geocodingStatus"`
	Coordinates     *LocationPoint  `json:"coordinates,omitempty"`
	Status          LostPetStatus   `json:"status"`
}

// LostPetRecord is the persisted lost-pet aggregate. Private owner contact is
// stored separately and linked by OwnerIdentityRef.
type LostPetRecord struct {
	PetID            string          `json:"petId"`
	PetName          string          `json:"petName,omitempty"`
	Species          string          `json:"species,omitempty"`
	Breed            string          `json:"breed,omitempty"`
	PrimaryColor     string          `json:"primaryColor,omitempty"`
	Description      string          `json:"description,omitempty"`
	OwnerIdentityRef string          `json:"ownerIdentityRef"`
	ReportedAt       time.Time       `json:"reportedAt"`
	Location         string          `json:"location"`
	GeocodingStatus  GeocodingStatus `json:"geocodingStatus"`
	Coordinates      *LocationPoint  `json:"coordinates,omitempty"`
	Status           LostPetStatus   `json:"status"`
}

// LostPetReportedV2 is the additive payload-v2 integration event. Its legacy
// fields retain their original JSON names so payload-v1 readers can continue
// decoding the fields they understand. ReporterEmail remains only so prior
// payload-v2 events can be reconstructed and decoded; phone was never
// published. New producers use LostPetReportedV3.
type LostPetReportedV2 struct {
	PetID           string          `json:"petId"`
	PetName         string          `json:"petName,omitempty"`
	Species         string          `json:"species,omitempty"`
	Breed           string          `json:"breed,omitempty"`
	PrimaryColor    string          `json:"primaryColor,omitempty"`
	Description     string          `json:"description,omitempty"`
	ReporterEmail   string          `json:"reporterEmail,omitempty"`
	ReportedAt      time.Time       `json:"reportedAt"`
	Location        string          `json:"location"`
	GeocodingStatus GeocodingStatus `json:"geocodingStatus"`
	Coordinates     *LocationPoint  `json:"coordinates,omitempty"`
	Status          LostPetStatus   `json:"status"`
}

// LostPetReportedV3 is the contact-redacted lost-pet integration event. Owner
// contact is available only through the private persisted ReportContact.
type LostPetReportedV3 struct {
	PetID           string          `json:"petId"`
	PetName         string          `json:"petName,omitempty"`
	Species         string          `json:"species,omitempty"`
	Breed           string          `json:"breed,omitempty"`
	PrimaryColor    string          `json:"primaryColor,omitempty"`
	Description     string          `json:"description,omitempty"`
	ReportedAt      time.Time       `json:"reportedAt"`
	Location        string          `json:"location"`
	GeocodingStatus GeocodingStatus `json:"geocodingStatus"`
	Coordinates     *LocationPoint  `json:"coordinates,omitempty"`
	Status          LostPetStatus   `json:"status"`
}

// DecodeLostPetReported reads every lost-pet payload shape published by the
// application and returns the normalized canonical integration event. Raw
// payloads predate envelopes and are therefore interpreted as payload v1.
func DecodeLostPetReported(data []byte) (LostPetReportedV3, *EventEnvelope, error) {
	var payload json.RawMessage
	envelope, err := DecodeEventPayload(data, EventTypeLostPetReported, &payload)
	if err != nil {
		return LostPetReportedV3{}, nil, err
	}

	payloadVersion := LostPetReportedLegacyPayloadVersion
	if envelope != nil {
		payloadVersion = envelope.PayloadVersion
	}
	var event LostPetReportedV3
	switch payloadVersion {
	case LostPetReportedLegacyPayloadVersion:
		var legacy LostPetEvent
		if err := json.Unmarshal(payload, &legacy); err != nil {
			return LostPetReportedV3{}, nil, fmt.Errorf("domain: decode lost-pet payload v1: %w", err)
		}
		if err := legacy.Validate(); err != nil {
			return LostPetReportedV3{}, nil, fmt.Errorf("domain: invalid lost-pet payload v1: %w", err)
		}
		event = normalizeLostPetReportedV3(LostPetReportedV3{
			PetID:      legacy.PetID,
			ReportedAt: legacy.ReportedAt,
			Location:   legacy.Location,
		})
	case LostPetReportedContactPayloadVersion:
		var prior LostPetReportedV2
		if err := json.Unmarshal(payload, &prior); err != nil {
			return LostPetReportedV3{}, nil, fmt.Errorf("domain: decode lost-pet payload v2: %w", err)
		}
		if err := prior.Validate(); err != nil {
			return LostPetReportedV3{}, nil, fmt.Errorf("domain: invalid lost-pet payload v2: %w", err)
		}
		event = normalizeLostPetReportedV3(prior.redacted())
	case LostPetReportedPayloadVersion:
		if err := json.Unmarshal(payload, &event); err != nil {
			return LostPetReportedV3{}, nil, fmt.Errorf("domain: decode lost-pet payload v3: %w", err)
		}
		if err := event.Validate(); err != nil {
			return LostPetReportedV3{}, nil, fmt.Errorf("domain: invalid lost-pet payload v3: %w", err)
		}
		event = normalizeLostPetReportedV3(event)
	default:
		return LostPetReportedV3{}, nil, fmt.Errorf("domain: unsupported lost-pet payload version %d", payloadVersion)
	}
	if envelope != nil && strings.TrimSpace(envelope.AggregateID) != event.PetID {
		return LostPetReportedV3{}, nil, errors.New("domain: lost-pet aggregate ID does not match payload")
	}
	return event, envelope, nil
}

// Validate checks the prior contact-bearing payload-v2 integration contract.
func (e LostPetReportedV2) Validate() error {
	if strings.TrimSpace(e.PetID) == "" {
		return errors.New("domain: petId is required")
	}
	if e.ReportedAt.IsZero() {
		return errors.New("domain: reportedAt is required")
	}
	if err := validateLostPetCanonicalFields(LostPetReport{
		PetID:           e.PetID,
		PetName:         e.PetName,
		Species:         e.Species,
		Breed:           e.Breed,
		PrimaryColor:    e.PrimaryColor,
		Description:     e.Description,
		ReporterEmail:   e.ReporterEmail,
		ReportedAt:      e.ReportedAt,
		Location:        e.Location,
		GeocodingStatus: e.GeocodingStatus,
		Coordinates:     e.Coordinates,
		Status:          e.Status,
	}); err != nil {
		return err
	}
	if e.ReporterEmail != "" && (!strings.Contains(e.ReporterEmail, "@") || !strings.Contains(e.ReporterEmail, ".")) {
		return fmt.Errorf("domain: invalid reporterEmail address: %s", e.ReporterEmail)
	}
	return nil
}

// Validate checks the contact-independent payload-v3 integration contract.
func (e LostPetReportedV3) Validate() error {
	if strings.TrimSpace(e.PetID) == "" {
		return errors.New("domain: petId is required")
	}
	if e.ReportedAt.IsZero() {
		return errors.New("domain: reportedAt is required")
	}
	return validateLostPetCanonicalFields(LostPetReport{
		PetID:           e.PetID,
		PetName:         e.PetName,
		Species:         e.Species,
		Breed:           e.Breed,
		PrimaryColor:    e.PrimaryColor,
		Description:     e.Description,
		ReportedAt:      e.ReportedAt,
		Location:        e.Location,
		GeocodingStatus: e.GeocodingStatus,
		Coordinates:     e.Coordinates,
		Status:          e.Status,
	})
}

// PublicLostPetReport is the unauthenticated lost-pet listing DTO. It cannot
// serialize reporter contact details because those fields are absent by type.
type PublicLostPetReport struct {
	PetID           string          `json:"petId"`
	PetName         string          `json:"petName,omitempty"`
	Species         string          `json:"species,omitempty"`
	Breed           string          `json:"breed,omitempty"`
	PrimaryColor    string          `json:"primaryColor,omitempty"`
	Description     string          `json:"description,omitempty"`
	ReportedAt      time.Time       `json:"reportedAt"`
	Location        string          `json:"location"`
	GeocodingStatus GeocodingStatus `json:"geocodingStatus,omitempty"`
	Coordinates     *LocationPoint  `json:"coordinates,omitempty"`
	Status          LostPetStatus   `json:"status,omitempty"`
}

// NormalizeLostPetReport canonicalizes user-supplied values before validation,
// persistence, event identity derivation, and retry comparison.
func NormalizeLostPetReport(report LostPetReport) LostPetReport {
	report.PetID = strings.TrimSpace(report.PetID)
	report.PetName = strings.TrimSpace(report.PetName)
	report.Species = normalizeSpecies(report.Species)
	report.Breed = strings.TrimSpace(report.Breed)
	report.PrimaryColor = strings.TrimSpace(report.PrimaryColor)
	report.Description = strings.TrimSpace(report.Description)
	report.ReporterEmail = strings.ToLower(strings.TrimSpace(report.ReporterEmail))
	report.Phone = strings.TrimSpace(report.Phone)
	report.Location = strings.TrimSpace(report.Location)
	if !report.ReportedAt.IsZero() {
		report.ReportedAt = report.ReportedAt.UTC()
	}
	if report.Status == "" {
		report.Status = LostPetStatusLost
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

// Validate checks the canonical lost-pet aggregate at the application boundary.
func (r LostPetReport) Validate() error {
	legacy := LostPetEvent{
		PetID:         r.PetID,
		ReporterEmail: r.ReporterEmail,
		ReportedAt:    r.ReportedAt,
		Location:      r.Location,
	}
	if err := legacy.Validate(); err != nil {
		return err
	}
	return validateLostPetCanonicalFields(r)
}

func validateLostPetCanonicalFields(r LostPetReport) error {
	if err := validateLostPetLengths(r); err != nil {
		return err
	}
	if r.Species != "" && r.Species != "Dog" && r.Species != "Cat" && r.Species != "Bird" && r.Species != "Other" {
		return fmt.Errorf("domain: unsupported species %q", r.Species)
	}
	if r.Status != LostPetStatusLost {
		return fmt.Errorf("domain: unsupported lost-pet status %q", r.Status)
	}
	switch r.GeocodingStatus {
	case GeocodingPending:
		if r.Location == "" {
			return errors.New("domain: pending geocoding requires a location")
		}
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

func normalizeLostPetReportedV3(event LostPetReportedV3) LostPetReportedV3 {
	report := NormalizeLostPetReport(LostPetReport{
		PetID:           event.PetID,
		PetName:         event.PetName,
		Species:         event.Species,
		Breed:           event.Breed,
		PrimaryColor:    event.PrimaryColor,
		Description:     event.Description,
		ReportedAt:      event.ReportedAt,
		Location:        event.Location,
		GeocodingStatus: event.GeocodingStatus,
		Coordinates:     event.Coordinates,
		Status:          event.Status,
	})
	return report.ReportedEvent()
}

func (e LostPetReportedV2) redacted() LostPetReportedV3 {
	return LostPetReportedV3{
		PetID:           e.PetID,
		PetName:         e.PetName,
		Species:         e.Species,
		Breed:           e.Breed,
		PrimaryColor:    e.PrimaryColor,
		Description:     e.Description,
		ReportedAt:      e.ReportedAt,
		Location:        e.Location,
		GeocodingStatus: e.GeocodingStatus,
		Coordinates:     cloneLocationPoint(e.Coordinates),
		Status:          e.Status,
	}
}

// Public returns the redacted listing representation of the aggregate.
func (r LostPetReport) Public() PublicLostPetReport {
	return PublicLostPetReport{
		PetID:           r.PetID,
		PetName:         r.PetName,
		Species:         r.Species,
		Breed:           r.Breed,
		PrimaryColor:    r.PrimaryColor,
		Description:     r.Description,
		ReportedAt:      r.ReportedAt,
		Location:        r.Location,
		GeocodingStatus: r.GeocodingStatus,
		Coordinates:     cloneLocationPoint(r.Coordinates),
		Status:          r.Status,
	}
}

// Persisted separates private owner contact from the report aggregate.
func (r LostPetReport) Persisted() (LostPetRecord, ReportContact) {
	identityRef := reportIdentityRef("lost", r.PetID, "owner")
	return LostPetRecord{
			PetID:            r.PetID,
			PetName:          r.PetName,
			Species:          r.Species,
			Breed:            r.Breed,
			PrimaryColor:     r.PrimaryColor,
			Description:      r.Description,
			OwnerIdentityRef: identityRef,
			ReportedAt:       r.ReportedAt,
			Location:         r.Location,
			GeocodingStatus:  r.GeocodingStatus,
			Coordinates:      cloneLocationPoint(r.Coordinates),
			Status:           r.Status,
		}, NormalizeReportContact(ReportContact{
			IdentityRef: identityRef,
			Email:       r.ReporterEmail,
			Phone:       r.Phone,
		})
}

// NormalizeLostPetRecord canonicalizes persisted state, including legacy
// records that predate explicit identity references.
func NormalizeLostPetRecord(record LostPetRecord) LostPetRecord {
	report := NormalizeLostPetReport(LostPetReport{
		PetID:           record.PetID,
		PetName:         record.PetName,
		Species:         record.Species,
		Breed:           record.Breed,
		PrimaryColor:    record.PrimaryColor,
		Description:     record.Description,
		ReportedAt:      record.ReportedAt,
		Location:        record.Location,
		GeocodingStatus: record.GeocodingStatus,
		Coordinates:     record.Coordinates,
		Status:          record.Status,
	})
	normalized, _ := report.Persisted()
	if identityRef := strings.TrimSpace(record.OwnerIdentityRef); identityRef != "" {
		normalized.OwnerIdentityRef = identityRef
	}
	return normalized
}

// Public returns the unauthenticated representation of persisted state.
func (r LostPetRecord) Public() PublicLostPetReport {
	return PublicLostPetReport{
		PetID:           r.PetID,
		PetName:         r.PetName,
		Species:         r.Species,
		Breed:           r.Breed,
		PrimaryColor:    r.PrimaryColor,
		Description:     r.Description,
		ReportedAt:      r.ReportedAt,
		Location:        r.Location,
		GeocodingStatus: r.GeocodingStatus,
		Coordinates:     cloneLocationPoint(r.Coordinates),
		Status:          r.Status,
	}
}

// ReportedEvent returns the contact-redacted current integration event.
func (r LostPetReport) ReportedEvent() LostPetReportedV3 {
	return LostPetReportedV3{
		PetID:           r.PetID,
		PetName:         r.PetName,
		Species:         r.Species,
		Breed:           r.Breed,
		PrimaryColor:    r.PrimaryColor,
		Description:     r.Description,
		ReportedAt:      r.ReportedAt,
		Location:        r.Location,
		GeocodingStatus: r.GeocodingStatus,
		Coordinates:     cloneLocationPoint(r.Coordinates),
		Status:          r.Status,
	}
}

// ReportedEventV2 reconstructs the prior contact-bearing event for stable
// retry compatibility. New producers must publish ReportedEvent instead.
func (r LostPetReport) ReportedEventV2() LostPetReportedV2 {
	return LostPetReportedV2{
		PetID:           r.PetID,
		PetName:         r.PetName,
		Species:         r.Species,
		Breed:           r.Breed,
		PrimaryColor:    r.PrimaryColor,
		Description:     r.Description,
		ReporterEmail:   r.ReporterEmail,
		ReportedAt:      r.ReportedAt,
		Location:        r.Location,
		GeocodingStatus: r.GeocodingStatus,
		Coordinates:     cloneLocationPoint(r.Coordinates),
		Status:          r.Status,
	}
}

func normalizeSpecies(species string) string {
	switch strings.ToLower(strings.TrimSpace(species)) {
	case "dog":
		return "Dog"
	case "cat":
		return "Cat"
	case "bird":
		return "Bird"
	case "other":
		return "Other"
	default:
		return strings.TrimSpace(species)
	}
}

func validateLostPetLengths(r LostPetReport) error {
	fields := []struct {
		name  string
		value string
		limit int
	}{
		{name: "petId", value: r.PetID, limit: 256},
		{name: "petName", value: r.PetName, limit: 200},
		{name: "species", value: r.Species, limit: 32},
		{name: "breed", value: r.Breed, limit: 200},
		{name: "primaryColor", value: r.PrimaryColor, limit: 100},
		{name: "description", value: r.Description, limit: 2000},
		{name: "reporterEmail", value: r.ReporterEmail, limit: 320},
		{name: "phone", value: r.Phone, limit: 64},
		{name: "location", value: r.Location, limit: 500},
	}
	for _, field := range fields {
		if utf8.RuneCountInString(field.value) > field.limit {
			return fmt.Errorf("domain: %s exceeds %d characters", field.name, field.limit)
		}
	}
	return nil
}

func cloneLocationPoint(point *LocationPoint) *LocationPoint {
	if point == nil {
		return nil
	}
	cloned := *point
	return &cloned
}
