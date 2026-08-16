package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestLostPetReportedV3RedactsContactAndReadsEveryPublishedVersion(t *testing.T) {
	reportedAt := time.Date(2026, time.August, 16, 14, 0, 0, 0, time.UTC)
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID:         "lost-v3",
		PetName:       "Buddy",
		Species:       "Dog",
		ReporterEmail: "owner@example.com",
		Phone:         "(555) 019-2834",
		ReportedAt:    reportedAt,
		Location:      "Seattle, WA",
	})
	v3Payload, err := json.Marshal(report.ReportedEvent())
	if err != nil {
		t.Fatal(err)
	}
	if domain.LostPetReportedPayloadVersion != 3 {
		t.Fatalf("current lost-pet payload version = %d, want 3", domain.LostPetReportedPayloadVersion)
	}
	if strings.Contains(string(v3Payload), "owner@example.com") || strings.Contains(string(v3Payload), "reporterEmail") ||
		strings.Contains(string(v3Payload), report.Phone) || strings.Contains(string(v3Payload), "phone") {
		t.Fatalf("payload-v3 exposed private contact: %s", v3Payload)
	}

	v3Envelope := lostEnvelope(t, report.PetID, reportedAt, domain.LostPetReportedPayloadVersion, v3Payload)
	legacy := domain.LostPetEvent{
		PetID:         "lost-v1",
		ReporterEmail: "legacy@example.com",
		ReportedAt:    reportedAt,
		Location:      "Seattle, WA",
	}
	v1Payload, err := legacy.ToJSON()
	if err != nil {
		t.Fatal(err)
	}
	v1Envelope := lostEnvelope(t, legacy.PetID, reportedAt, domain.LostPetReportedLegacyPayloadVersion, v1Payload)
	v2 := domain.LostPetReportedV2{
		PetID:           "lost-v2",
		PetName:         "Luna",
		Species:         "Dog",
		ReporterEmail:   "prior-owner@example.com",
		ReportedAt:      reportedAt,
		Location:        "Portland, OR",
		GeocodingStatus: domain.GeocodingPending,
		Status:          domain.LostPetStatusLost,
	}
	v2Payload, err := json.Marshal(v2)
	if err != nil {
		t.Fatal(err)
	}
	v2Envelope := lostEnvelope(t, v2.PetID, reportedAt, domain.LostPetReportedContactPayloadVersion, v2Payload)

	tests := []struct {
		name        string
		data        []byte
		wantPetID   string
		wantVersion int
	}{
		{name: "raw payload-v1", data: v1Payload, wantPetID: legacy.PetID},
		{name: "enveloped payload-v1", data: v1Envelope, wantPetID: legacy.PetID, wantVersion: 1},
		{name: "contact-bearing payload-v2", data: v2Envelope, wantPetID: v2.PetID, wantVersion: 2},
		{name: "contact-redacted payload-v3", data: v3Envelope, wantPetID: report.PetID, wantVersion: 3},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, metadata, err := domain.DecodeLostPetReported(test.data)
			if err != nil {
				t.Fatalf("DecodeLostPetReported() error = %v", err)
			}
			if event.PetID != test.wantPetID || event.Status != domain.LostPetStatusLost {
				t.Fatalf("decoded event = %#v", event)
			}
			canonicalData, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(canonicalData), "reporterEmail") || strings.Contains(string(canonicalData), "@example.com") {
				t.Fatalf("canonical event retained private contact: %s", canonicalData)
			}
			if test.wantVersion == 0 {
				if metadata != nil {
					t.Fatalf("metadata = %#v, want nil", metadata)
				}
			} else if metadata == nil || metadata.PayloadVersion != test.wantVersion {
				t.Fatalf("metadata = %#v, want payload version %d", metadata, test.wantVersion)
			}
		})
	}
}

func TestDecodeLostPetReportedRejectsIncompletePayloadV3(t *testing.T) {
	reportedAt := time.Date(2026, time.August, 16, 14, 0, 0, 0, time.UTC)
	base := domain.LostPetReportedV3{
		PetID:           "lost-incomplete-v3",
		ReportedAt:      reportedAt,
		Location:        "Seattle, WA",
		GeocodingStatus: domain.GeocodingPending,
		Status:          domain.LostPetStatusLost,
	}
	tests := []struct {
		name   string
		mutate func(*domain.LostPetReportedV3)
	}{
		{name: "missing reportedAt", mutate: func(event *domain.LostPetReportedV3) { event.ReportedAt = time.Time{} }},
		{name: "missing geocodingStatus", mutate: func(event *domain.LostPetReportedV3) { event.GeocodingStatus = "" }},
		{name: "missing status", mutate: func(event *domain.LostPetReportedV3) { event.Status = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := base
			test.mutate(&event)
			payload, err := json.Marshal(event)
			if err != nil {
				t.Fatal(err)
			}
			data := lostEnvelope(t, event.PetID, reportedAt, domain.LostPetReportedPayloadVersion, payload)
			if _, _, err := domain.DecodeLostPetReported(data); err == nil {
				t.Fatal("DecodeLostPetReported() error = nil, want incomplete payload-v3 rejection")
			}
		})
	}
}

func lostEnvelope(t *testing.T, petID string, occurredAt time.Time, payloadVersion int, payload []byte) []byte {
	t.Helper()
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeLostPetReported,
		OccurredAt:       occurredAt,
		AggregateID:      petID,
		AggregateVersion: 1,
		PayloadVersion:   payloadVersion,
		Payload:          payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
