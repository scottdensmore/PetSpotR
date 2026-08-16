package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestLostPetReportNormalizesAndRedactsPublicContact(t *testing.T) {
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID:         " lost-101 ",
		PetName:       " Buddy ",
		Species:       " dog ",
		ReporterEmail: " OWNER@EXAMPLE.COM ",
		Phone:         " (555) 019-2834 ",
		ReportedAt:    time.Date(2026, time.August, 15, 12, 0, 0, 0, time.FixedZone("PDT", -7*60*60)),
		Location:      " Seattle, WA ",
	})

	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.PetID != "lost-101" || report.PetName != "Buddy" || report.Species != "Dog" ||
		report.ReporterEmail != "owner@example.com" || report.Phone != "(555) 019-2834" ||
		report.Location != "Seattle, WA" || report.Status != domain.LostPetStatusLost ||
		report.GeocodingStatus != domain.GeocodingPending || report.ReportedAt.Location() != time.UTC {
		t.Fatalf("normalized report = %#v", report)
	}

	publicData, err := json.Marshal(report.Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicData), "owner@example.com") || strings.Contains(string(publicData), "reporterEmail") ||
		strings.Contains(string(publicData), "phone") {
		t.Fatalf("public report exposed private contact: %s", publicData)
	}
}

func TestLostPetReportRejectsUnverifiedOrInvalidLocationData(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*domain.LostPetReport)
	}{
		{name: "pending without location", mutate: func(report *domain.LostPetReport) { report.Location = "" }},
		{name: "unsupported species", mutate: func(report *domain.LostPetReport) { report.Species = "Ferret" }},
		{name: "oversized description", mutate: func(report *domain.LostPetReport) {
			report.Description = strings.Repeat("x", 2001)
		}},
		{name: "pending coordinates", mutate: func(report *domain.LostPetReport) {
			report.Coordinates = &domain.LocationPoint{Latitude: 47.6, Longitude: -122.3}
		}},
		{name: "verified without coordinates", mutate: func(report *domain.LostPetReport) {
			report.GeocodingStatus = domain.GeocodingVerified
		}},
		{name: "invalid verified coordinates", mutate: func(report *domain.LostPetReport) {
			report.GeocodingStatus = domain.GeocodingVerified
			report.Coordinates = &domain.LocationPoint{Latitude: 100, Longitude: -122.3}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := domain.NormalizeLostPetReport(domain.LostPetReport{
				PetID:         "lost-101",
				Species:       "Dog",
				ReporterEmail: "owner@example.com",
				ReportedAt:    time.Now().UTC(),
				Location:      "Seattle, WA",
			})
			tt.mutate(&report)
			if err := report.Validate(); err == nil {
				t.Fatal("Validate() error = nil, want invalid report")
			}
		})
	}
}

func TestLostPetReportPreservesLegacyMissingLocationWithExplicitStatus(t *testing.T) {
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID:         "lost-legacy",
		ReporterEmail: "owner@example.com",
	})

	if report.GeocodingStatus != domain.GeocodingUnavailable || report.Coordinates != nil {
		t.Fatalf("normalized legacy location = %#v", report)
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLostPetReportedV2RemainsReadableByPayloadV1Consumer(t *testing.T) {
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID:         "lost-101",
		PetName:       "Buddy",
		Species:       "Dog",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC),
		Location:      "Seattle, WA",
	})
	payload, err := json.Marshal(report.ReportedEvent())
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeLostPetReported,
		OccurredAt:       report.ReportedAt,
		AggregateID:      report.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.LostPetReportedPayloadVersion,
		Payload:          payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	var legacy domain.LostPetEvent
	metadata, err := domain.DecodeEventPayload(data, domain.EventTypeLostPetReported, &legacy)
	if err != nil {
		t.Fatalf("DecodeEventPayload() error = %v", err)
	}
	if metadata.PayloadVersion != domain.LostPetReportedPayloadVersion || legacy.PetID != report.PetID ||
		legacy.ReporterEmail != report.ReporterEmail || legacy.Location != report.Location {
		t.Fatalf("legacy event = %#v; metadata = %#v", legacy, metadata)
	}
}

func TestLostPetReportedV2ReaderAcceptsPriorPayloadShapes(t *testing.T) {
	legacy := domain.LostPetEvent{
		PetID:         "lost-legacy",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC),
		Location:      "Seattle, WA",
	}
	legacyData, err := legacy.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	legacyEnvelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeLostPetReported,
		OccurredAt:       legacy.ReportedAt,
		AggregateID:      legacy.PetID,
		AggregateVersion: 1,
		PayloadVersion:   domain.LostPetReportedLegacyPayloadVersion,
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
			var current domain.LostPetReportedV2
			metadata, err := domain.DecodeEventPayload(tt.data, domain.EventTypeLostPetReported, &current)
			if err != nil {
				t.Fatalf("DecodeEventPayload() error = %v", err)
			}
			if (metadata != nil) != tt.wantMetadata {
				t.Fatalf("metadata = %#v, want present %t", metadata, tt.wantMetadata)
			}
			if current.PetID != legacy.PetID || current.ReporterEmail != legacy.ReporterEmail ||
				current.ReportedAt != legacy.ReportedAt || current.Location != legacy.Location {
				t.Fatalf("decoded current event = %#v", current)
			}
		})
	}
}
