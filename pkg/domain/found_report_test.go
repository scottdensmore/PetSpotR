package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestFoundPetReportNormalizesAndRedactsFinderContact(t *testing.T) {
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID:               " found-101 ",
		ImageURL:            " https://storage.petspotr.io/found-101.jpg ",
		FoundAt:             time.Date(2026, time.August, 15, 13, 0, 0, 0, time.FixedZone("PDT", -7*60*60)),
		Location:            " Seattle, WA ",
		FinderEmail:         " FINDER@EXAMPLE.COM ",
		Species:             " dog ",
		Breed:               " Golden Retriever ",
		PrimaryColor:        " Golden ",
		SecondaryColor:      " Cream ",
		DistinctiveMarkings: []string{" White chest patch ", "", "White chest patch"},
		CustodyStatus:       domain.CustodyFinderHome,
	})

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.PetID != "found-101" || report.FinderEmail != "finder@example.com" ||
		report.Species != "Dog" || report.Breed != "Golden Retriever" ||
		report.PrimaryColor != "Golden" || report.SecondaryColor != "Cream" ||
		len(report.DistinctiveMarkings) != 1 || report.DistinctiveMarkings[0] != "White chest patch" ||
		report.Location != "Seattle, WA" || report.Status != domain.FoundPetStatusFound ||
		report.GeocodingStatus != domain.GeocodingPending || report.FoundAt.Location() != time.UTC {
		t.Fatalf("normalized report = %#v", report)
	}

	publicData, err := json.Marshal(report.Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicData), "finder@example.com") ||
		strings.Contains(string(publicData), "finderEmail") {
		t.Fatalf("public report exposed private contact: %s", publicData)
	}
	eventData, err := json.Marshal(report.ReportedEvent())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(eventData), "finder@example.com") ||
		strings.Contains(string(eventData), "finderEmail") {
		t.Fatalf("integration event exposed private contact: %s", eventData)
	}
}

func TestFoundPetReportRejectsInvalidCanonicalData(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.FoundPetReport)
	}{
		{name: "missing image", mutate: func(report *domain.FoundPetReport) { report.ImageURL = "" }},
		{name: "missing location", mutate: func(report *domain.FoundPetReport) { report.Location = "" }},
		{name: "invalid finder email", mutate: func(report *domain.FoundPetReport) { report.FinderEmail = "not-an-email" }},
		{name: "unsupported species", mutate: func(report *domain.FoundPetReport) { report.Species = "Ferret" }},
		{name: "unsupported custody", mutate: func(report *domain.FoundPetReport) { report.CustodyStatus = "At Large" }},
		{name: "oversized marking", mutate: func(report *domain.FoundPetReport) {
			report.DistinctiveMarkings = []string{strings.Repeat("x", 201)}
		}},
		{name: "pending coordinates", mutate: func(report *domain.FoundPetReport) {
			report.Coordinates = &domain.LocationPoint{Latitude: 47.6, Longitude: -122.3}
		}},
		{name: "verified without coordinates", mutate: func(report *domain.FoundPetReport) {
			report.GeocodingStatus = domain.GeocodingVerified
		}},
		{name: "invalid verified coordinates", mutate: func(report *domain.FoundPetReport) {
			report.GeocodingStatus = domain.GeocodingVerified
			report.Coordinates = &domain.LocationPoint{Latitude: 100, Longitude: -122.3}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
				PetID:         "found-101",
				ImageURL:      "https://storage.petspotr.io/found-101.jpg",
				FoundAt:       time.Now().UTC(),
				Location:      "Seattle, WA",
				FinderEmail:   "finder@example.com",
				Species:       "Dog",
				CustodyStatus: domain.CustodyFinderHome,
			})
			tt.mutate(&report)
			if err := report.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid report")
			}
		})
	}
}

func TestFoundPetReportPreservesLegacyOptionalFields(t *testing.T) {
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID:    "found-legacy",
		ImageURL: "https://storage.petspotr.io/found-legacy.jpg",
		Location: "Seattle, WA",
	})

	if report.GeocodingStatus != domain.GeocodingPending || report.CustodyStatus != domain.CustodyUnknown ||
		report.Status != domain.FoundPetStatusFound || report.Coordinates != nil {
		t.Fatalf("normalized legacy report = %#v", report)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestFoundPetReportedV2RemainsReadableByPayloadV1Consumer(t *testing.T) {
	report := domain.NormalizeFoundPetReport(domain.FoundPetReport{
		PetID:               "found-101",
		ImageObject:         "images/found-pets/found-101/image.jpg",
		FoundAt:             time.Date(2026, time.August, 15, 13, 0, 0, 0, time.UTC),
		Location:            "Seattle, WA",
		FinderEmail:         "finder@example.com",
		Species:             "Dog",
		Breed:               "Golden Retriever",
		DistinctiveMarkings: []string{"White chest patch"},
		CustodyStatus:       domain.CustodyLocalShelter,
	})
	payload, err := json.Marshal(report.ReportedEvent())
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       report.FoundAt,
		AggregateID:      report.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.FoundPetReportedPayloadVersion,
		Payload:          payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	var legacy domain.FoundPetEvent
	metadata, err := domain.DecodeEventPayload(data, domain.EventTypeFoundPetReported, &legacy)
	if err != nil {
		t.Fatalf("DecodeEventPayload() error = %v", err)
	}
	if metadata.PayloadVersion != domain.FoundPetReportedPayloadVersion || legacy.PetID != report.PetID ||
		legacy.ImageObject != report.ImageObject || legacy.Location != report.Location {
		t.Fatalf("legacy event = %#v; metadata = %#v", legacy, metadata)
	}
}

func TestFoundPetReportedV2ReaderAcceptsPriorPayloadShapes(t *testing.T) {
	legacy := domain.FoundPetEvent{
		PetID:    "found-legacy",
		ImageURL: "https://storage.petspotr.io/found-legacy.jpg",
		FoundAt:  time.Date(2026, time.August, 15, 13, 0, 0, 0, time.UTC),
		Location: "Seattle, WA",
	}
	legacyData, err := legacy.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	legacyEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeFoundPetReported,
		OccurredAt:       legacy.FoundAt,
		AggregateID:      legacy.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.FoundPetReportedLegacyPayloadVersion,
		Payload:          legacyData,
	})
	if err != nil {
		t.Fatal(err)
	}
	envelopeData, err := json.Marshal(legacyEnvelope)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name         string
		data         []byte
		wantMetadata bool
	}{
		{name: "raw legacy payload", data: legacyData},
		{name: "payload-v1 envelope", data: envelopeData, wantMetadata: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var current domain.FoundPetReportedV2
			metadata, err := domain.DecodeEventPayload(tt.data, domain.EventTypeFoundPetReported, &current)
			if err != nil {
				t.Fatalf("DecodeEventPayload() error = %v", err)
			}
			if (metadata != nil) != tt.wantMetadata {
				t.Fatalf("metadata = %#v, want present %t", metadata, tt.wantMetadata)
			}
			if current.PetID != legacy.PetID || current.ImageURL != legacy.ImageURL ||
				current.FoundAt != legacy.FoundAt || current.Location != legacy.Location {
				t.Fatalf("decoded current event = %#v", current)
			}
		})
	}
}
