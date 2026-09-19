package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/scottdensmore/petspotr/pkg/microchip"
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
	// FoundPetStatusResolved identifies a terminal report resolved by its finder.
	FoundPetStatusResolved FoundPetStatus = "resolved"
	// FoundPetStatusExpired identifies a terminal report that aged out.
	FoundPetStatusExpired FoundPetStatus = "expired"
)

// IsActive reports whether the found-pet report remains open.
func (s FoundPetStatus) IsActive() bool {
	return s == FoundPetStatusFound
}

// IsTerminal reports whether no further found-pet lifecycle transition is valid.
func (s FoundPetStatus) IsTerminal() bool {
	switch s {
	case FoundPetStatusResolved, FoundPetStatusExpired:
		return true
	default:
		return false
	}
}

// CanTransitionTo reports whether next is a valid one-way lifecycle transition.
func (s FoundPetStatus) CanTransitionTo(next FoundPetStatus) bool {
	return s.IsActive() && next.IsTerminal()
}

// CustodyStatus records where the found pet is currently located.
type CustodyStatus string

const (
	CustodyUnknown       CustodyStatus = "Unknown"
	CustodyFinderHome    CustodyStatus = "Finder Home"
	CustodyLocalShelter  CustodyStatus = "Local Shelter"
	CustodyAnimalControl CustodyStatus = "Animal Control"
	CustodySightedOnly   CustodyStatus = "Sighted Only"
	CustodyShelterCare   CustodyStatus = "Shelter Care"
)

// FoundPetReport is the canonical application-boundary model for a found-pet
// report. Persisted separates its private contact into ReportContact.
type FoundPetReport struct {
	PetID               string          `json:"petId"`
	ImageURL            string          `json:"imageUrl,omitempty"`
	ImageObject         string          `json:"imageObject,omitempty"`
	Images              []PetImage      `json:"images,omitempty"`
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
	CustodyStatus       CustodyStatus   `json:"custodyStatus,omitempty"`
	Status              FoundPetStatus  `json:"status"`
	OwnedBy             *PrincipalRef   `json:"-"`
	MicrochipID         string          `json:"microchipId,omitempty"`
	MicrochipRegistry   string          `json:"microchipRegistry,omitempty"`
	ShelterID           string          `json:"shelterId,omitempty"`
	ShelterName         string          `json:"shelterName,omitempty"`
	IntakeID            string          `json:"intakeId,omitempty"`
}

// FoundPetRecord is the persisted found-pet aggregate. Private finder contact
// is stored separately and linked by FinderIdentityRef. OwnedBy identifies the
// authenticated resource owner when the producer supplied one.
type FoundPetRecord struct {
	PetID               string                  `json:"petId"`
	ImageURL            string                  `json:"imageUrl,omitempty"`
	ImageObject         string                  `json:"imageObject,omitempty"`
	Images              []PetImage              `json:"images,omitempty"`
	Embedding           []float32               `json:"embedding,omitempty"`
	FoundAt             time.Time               `json:"foundAt"`
	Location            string                  `json:"location"`
	GeocodingStatus     GeocodingStatus         `json:"geocodingStatus"`
	Coordinates         *LocationPoint          `json:"coordinates,omitempty"`
	FinderIdentityRef   string                  `json:"finderIdentityRef"`
	Species             string                  `json:"species,omitempty"`
	Breed               string                  `json:"breed,omitempty"`
	PrimaryColor        string                  `json:"primaryColor,omitempty"`
	SecondaryColor      string                  `json:"secondaryColor,omitempty"`
	DistinctiveMarkings []string                `json:"distinctiveMarkings,omitempty"`
	CustodyStatus       CustodyStatus           `json:"custodyStatus,omitempty"`
	Status              FoundPetStatus          `json:"status"`
	ImageAnalysis       *ImageTraitAnalysis     `json:"imageAnalysis,omitempty"`
	OwnedBy             *PrincipalRef           `json:"ownedBy,omitempty"`
	LifecycleAudit      *FoundPetLifecycleAudit `json:"lifecycleAudit,omitempty"`
	MicrochipID         string                  `json:"microchipId,omitempty"`
	MicrochipRegistry   string                  `json:"microchipRegistry,omitempty"`
	ShelterID           string                  `json:"shelterId,omitempty"`
	ShelterName         string                  `json:"shelterName,omitempty"`
	IntakeID            string                  `json:"intakeId,omitempty"`
}

// FoundPetReportedV2 is the additive payload-v2 integration event. Its legacy
// fields retain their original JSON names so payload-v1 readers can continue
// decoding the image and location fields they understand.
type FoundPetReportedV2 struct {
	PetID               string          `json:"petId"`
	ImageURL            string          `json:"imageUrl,omitempty"`
	ImageObject         string          `json:"imageObject,omitempty"`
	Images              []PetImage      `json:"images,omitempty"`
	Embedding           []float32       `json:"embedding,omitempty"`
	FoundAt             time.Time       `json:"foundAt"`
	Location            string          `json:"location"`
	GeocodingStatus     GeocodingStatus `json:"geocodingStatus"`
	Coordinates         *LocationPoint  `json:"coordinates,omitempty"`
	Species             string          `json:"species,omitempty"`
	Breed               string          `json:"breed,omitempty"`
	PrimaryColor        string          `json:"primaryColor,omitempty"`
	SecondaryColor      string          `json:"secondaryColor,omitempty"`
	DistinctiveMarkings []string        `json:"distinctiveMarkings,omitempty"`
	CustodyStatus       CustodyStatus   `json:"custodyStatus,omitempty"`
	Status              FoundPetStatus  `json:"status"`
	MicrochipID         string          `json:"microchipId,omitempty"`
	MicrochipRegistry   string          `json:"microchipRegistry,omitempty"`
	ShelterID           string          `json:"shelterId,omitempty"`
	ShelterName         string          `json:"shelterName,omitempty"`
	IntakeID            string          `json:"intakeId,omitempty"`
}

// DecodeFoundPetReported reads every found-pet payload shape published by the
// application and returns the normalized canonical integration event. Raw
// payloads predate envelopes and are therefore interpreted as payload v1.
func DecodeFoundPetReported(data []byte) (FoundPetReportedV2, *EventEnvelope, error) {
	var event FoundPetReportedV2
	envelope, err := DecodeEventPayload(data, EventTypeFoundPetReported, &event)
	if err != nil {
		return FoundPetReportedV2{}, nil, err
	}

	payloadVersion := FoundPetReportedLegacyPayloadVersion
	if envelope != nil {
		payloadVersion = envelope.PayloadVersion
	}
	switch payloadVersion {
	case FoundPetReportedLegacyPayloadVersion:
		legacy := FoundPetEvent{
			PetID:       event.PetID,
			ImageURL:    event.ImageURL,
			ImageObject: event.ImageObject,
			FoundAt:     event.FoundAt,
			Location:    event.Location,
		}
		if err := legacy.Validate(); err != nil {
			return FoundPetReportedV2{}, nil, fmt.Errorf("domain: invalid found-pet payload v1: %w", err)
		}
		event = normalizeFoundPetReported(FoundPetReportedV2{
			PetID:       legacy.PetID,
			ImageURL:    legacy.ImageURL,
			ImageObject: legacy.ImageObject,
			FoundAt:     legacy.FoundAt,
			Location:    legacy.Location,
		})
	case FoundPetReportedPayloadVersion:
		if err := event.Validate(); err != nil {
			return FoundPetReportedV2{}, nil, fmt.Errorf("domain: invalid found-pet payload v%d: %w", payloadVersion, err)
		}
		event = normalizeFoundPetReported(event)
	default:
		return FoundPetReportedV2{}, nil, fmt.Errorf("domain: unsupported found-pet payload version %d", payloadVersion)
	}
	if envelope != nil && strings.TrimSpace(envelope.AggregateID) != event.PetID {
		return FoundPetReportedV2{}, nil, errors.New("domain: found-pet aggregate ID does not match payload")
	}
	return event, envelope, nil
}

// Validate checks the payload-v2 integration contract independently of finder
// contact, which is private aggregate state and is not published.
func (e FoundPetReportedV2) Validate() error {
	if e.FoundAt.IsZero() {
		return errors.New("domain: foundAt is required")
	}
	if len(e.Embedding) > 0 {
		if err := ValidateEmbedding(e.Embedding); err != nil {
			return err
		}
	}
	return FoundPetReport{
		PetID:               e.PetID,
		ImageURL:            e.ImageURL,
		ImageObject:         e.ImageObject,
		Images:              e.Images,
		FoundAt:             e.FoundAt,
		Location:            e.Location,
		GeocodingStatus:     e.GeocodingStatus,
		Coordinates:         e.Coordinates,
		Species:             e.Species,
		Breed:               e.Breed,
		PrimaryColor:        e.PrimaryColor,
		SecondaryColor:      e.SecondaryColor,
		DistinctiveMarkings: e.DistinctiveMarkings,
		CustodyStatus:       e.CustodyStatus,
		Status:              e.Status,
		MicrochipID:         e.MicrochipID,
		MicrochipRegistry:   e.MicrochipRegistry,
		ShelterID:           e.ShelterID,
		ShelterName:         e.ShelterName,
		IntakeID:            e.IntakeID,
	}.Validate()
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
	MicrochipID         string          `json:"microchipId,omitempty"`
	MicrochipRegistry   string          `json:"microchipRegistry,omitempty"`
	ShelterID           string          `json:"shelterId,omitempty"`
	ShelterName         string          `json:"shelterName,omitempty"`
	IntakeID            string          `json:"intakeId,omitempty"`
}

// NormalizeFoundPetReport canonicalizes user-supplied values before
// validation, persistence, event identity derivation, and retry comparison.
func NormalizeFoundPetReport(report FoundPetReport) FoundPetReport {
	report.PetID = strings.TrimSpace(report.PetID)
	report.ImageURL = strings.TrimSpace(report.ImageURL)
	report.ImageObject = strings.TrimSpace(report.ImageObject)
	report.Images = NormalizePetImages(report.Images)
	if len(report.Images) == 0 && report.ImageObject != "" {
		report.Images = []PetImage{{Object: report.ImageObject, Tag: PetImageTagPrimary}}
	}
	if report.ImageObject == "" {
		if primary, ok := PrimaryPetImage(report.Images); ok {
			report.ImageObject = primary.Object
		}
	}
	if !report.FoundAt.IsZero() {
		report.FoundAt = report.FoundAt.UTC()
	}
	report.Location = strings.TrimSpace(report.Location)
	report.FinderEmail = strings.ToLower(strings.TrimSpace(report.FinderEmail))
	report.OwnedBy = normalizePrincipalRef(report.OwnedBy)
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
	report.MicrochipID = strings.TrimSpace(report.MicrochipID)
	report.MicrochipRegistry = strings.TrimSpace(report.MicrochipRegistry)
	report.ShelterID = strings.TrimSpace(report.ShelterID)
	report.ShelterName = strings.TrimSpace(report.ShelterName)
	report.IntakeID = strings.TrimSpace(report.IntakeID)
	if report.MicrochipID != "" {
		val := microchip.ValidateAndNormalize(report.MicrochipID)
		if val.Valid {
			report.MicrochipID = val.NormalizedID
			if report.MicrochipRegistry == "" {
				reg := microchip.IdentifyIssuingRegistry(val.NormalizedID)
				report.MicrochipRegistry = reg.RegistryName
			}
		}
	}
	return report
}

// Validate checks the canonical found-pet aggregate at the application boundary.
func (r FoundPetReport) Validate() error {
	imageObj := r.ImageObject
	if imageObj == "" {
		if primary, ok := PrimaryPetImage(r.Images); ok {
			imageObj = primary.Object
		}
	}
	legacy := FoundPetEvent{
		PetID:       r.PetID,
		ImageURL:    r.ImageURL,
		ImageObject: imageObj,
		FoundAt:     r.FoundAt,
		Location:    r.Location,
	}
	if err := legacy.Validate(); err != nil {
		return err
	}
	if r.OwnedBy != nil {
		if err := r.OwnedBy.Validate(); err != nil {
			return err
		}
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
	case CustodyUnknown, CustodyFinderHome, CustodyLocalShelter, CustodyAnimalControl, CustodySightedOnly, CustodyShelterCare:
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
		MicrochipID:         microchip.MaskMicrochip(r.MicrochipID),
		MicrochipRegistry:   r.MicrochipRegistry,
		ShelterID:           r.ShelterID,
		ShelterName:         r.ShelterName,
		IntakeID:            r.IntakeID,
	}
}

// Persisted separates private finder contact from the report aggregate.
func (r FoundPetReport) Persisted() (FoundPetRecord, ReportContact) {
	identityRef := reportIdentityRef("found", r.PetID, "finder")
	var emb []float32
	if primary, ok := PrimaryPetImage(r.Images); ok && len(primary.Embedding) > 0 {
		emb = append([]float32(nil), primary.Embedding...)
	}
	return FoundPetRecord{
			PetID:               r.PetID,
			ImageURL:            r.ImageURL,
			ImageObject:         r.ImageObject,
			Images:              clonePetImages(r.Images),
			Embedding:           emb,
			FoundAt:             r.FoundAt,
			Location:            r.Location,
			GeocodingStatus:     r.GeocodingStatus,
			Coordinates:         cloneLocationPoint(r.Coordinates),
			FinderIdentityRef:   identityRef,
			Species:             r.Species,
			Breed:               r.Breed,
			PrimaryColor:        r.PrimaryColor,
			SecondaryColor:      r.SecondaryColor,
			DistinctiveMarkings: append([]string(nil), r.DistinctiveMarkings...),
			CustodyStatus:       r.CustodyStatus,
			Status:              r.Status,
			OwnedBy:             normalizePrincipalRef(r.OwnedBy),
			MicrochipID:         r.MicrochipID,
			MicrochipRegistry:   r.MicrochipRegistry,
			ShelterID:           r.ShelterID,
			ShelterName:         r.ShelterName,
			IntakeID:            r.IntakeID,
		}, NormalizeReportContact(ReportContact{
			IdentityRef: identityRef,
			Email:       r.FinderEmail,
		})
}

// NormalizeFoundPetRecord canonicalizes persisted state, including legacy
// records that predate explicit identity references.
func NormalizeFoundPetRecord(record FoundPetRecord) FoundPetRecord {
	report := NormalizeFoundPetReport(FoundPetReport{
		PetID:               record.PetID,
		ImageURL:            record.ImageURL,
		ImageObject:         record.ImageObject,
		Images:              record.Images,
		FoundAt:             record.FoundAt,
		Location:            record.Location,
		GeocodingStatus:     record.GeocodingStatus,
		Coordinates:         record.Coordinates,
		Species:             record.Species,
		Breed:               record.Breed,
		PrimaryColor:        record.PrimaryColor,
		SecondaryColor:      record.SecondaryColor,
		DistinctiveMarkings: record.DistinctiveMarkings,
		CustodyStatus:       record.CustodyStatus,
		Status:              record.Status,
		OwnedBy:             record.OwnedBy,
		MicrochipID:         record.MicrochipID,
		MicrochipRegistry:   record.MicrochipRegistry,
		ShelterID:           record.ShelterID,
		ShelterName:         record.ShelterName,
		IntakeID:            record.IntakeID,
	})
	normalized, _ := report.Persisted()
	if identityRef := strings.TrimSpace(record.FinderIdentityRef); identityRef != "" {
		normalized.FinderIdentityRef = identityRef
	}
	if len(record.Embedding) > 0 {
		normalized.Embedding = append([]float32(nil), record.Embedding...)
	}
	normalized.ImageAnalysis = NormalizeImageTraitAnalysis(record.ImageAnalysis)
	if record.LifecycleAudit != nil {
		audit := *record.LifecycleAudit
		normalized.LifecycleAudit = &audit
	}
	return normalized
}

// Public returns the unauthenticated representation of persisted state.
func (r FoundPetRecord) Public() PublicFoundPetReport {
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
		MicrochipID:         microchip.MaskMicrochip(r.MicrochipID),
		MicrochipRegistry:   r.MicrochipRegistry,
		ShelterID:           r.ShelterID,
		ShelterName:         r.ShelterName,
		IntakeID:            r.IntakeID,
	}
}

// ReportedEvent returns the contact-redacted payload-v2 integration event.
func (r FoundPetReport) ReportedEvent() FoundPetReportedV2 {
	var emb []float32
	if primary, ok := PrimaryPetImage(r.Images); ok && len(primary.Embedding) > 0 {
		emb = append([]float32(nil), primary.Embedding...)
	}
	return FoundPetReportedV2{
		PetID:               r.PetID,
		ImageURL:            r.ImageURL,
		ImageObject:         r.ImageObject,
		Images:              clonePetImages(r.Images),
		Embedding:           emb,
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
		MicrochipID:         r.MicrochipID,
		MicrochipRegistry:   r.MicrochipRegistry,
		ShelterID:           r.ShelterID,
		ShelterName:         r.ShelterName,
		IntakeID:            r.IntakeID,
	}
}

func normalizeFoundPetReported(event FoundPetReportedV2) FoundPetReportedV2 {
	report := NormalizeFoundPetReport(FoundPetReport{
		PetID:               event.PetID,
		ImageURL:            event.ImageURL,
		ImageObject:         event.ImageObject,
		Images:              event.Images,
		FoundAt:             event.FoundAt,
		Location:            event.Location,
		GeocodingStatus:     event.GeocodingStatus,
		Coordinates:         event.Coordinates,
		Species:             event.Species,
		Breed:               event.Breed,
		PrimaryColor:        event.PrimaryColor,
		SecondaryColor:      event.SecondaryColor,
		DistinctiveMarkings: event.DistinctiveMarkings,
		CustodyStatus:       event.CustodyStatus,
		Status:              event.Status,
		MicrochipID:         event.MicrochipID,
		MicrochipRegistry:   event.MicrochipRegistry,
		ShelterID:           event.ShelterID,
		ShelterName:         event.ShelterName,
		IntakeID:            event.IntakeID,
	})
	res := report.ReportedEvent()
	if len(event.Embedding) > 0 {
		res.Embedding = append([]float32(nil), event.Embedding...)
	}
	return res
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
	case "shelter care":
		return CustodyShelterCare
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
		{name: "microchipId", value: r.MicrochipID, limit: 64},
		{name: "microchipRegistry", value: r.MicrochipRegistry, limit: 128},
		{name: "shelterId", value: r.ShelterID, limit: 128},
		{name: "shelterName", value: r.ShelterName, limit: 256},
		{name: "intakeId", value: r.IntakeID, limit: 128},
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
	if err := ValidatePetImages(r.Images); err != nil {
		return err
	}
	return nil
}
