package domain_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestLostPetReportedV4CarriesPrivateImageAndReadsPriorVersions(t *testing.T) {
	reportedAt := time.Date(2026, time.August, 17, 16, 0, 0, 0, time.UTC)
	report := domain.NormalizeLostPetReport(domain.LostPetReport{
		PetID:         "lost-v4",
		PetName:       "Buddy",
		Species:       "Dog",
		ReporterEmail: "owner@example.com",
		Phone:         "(555) 019-2834",
		ImageObject:   "images/lost-pets/lost-v4/image.jpg",
		ReportedAt:    reportedAt,
		Location:      "Seattle, WA",
	})
	v4Payload, err := json.Marshal(report.ReportedEvent())
	if err != nil {
		t.Fatal(err)
	}
	if domain.LostPetReportedRedactedPayloadVersion != 3 || domain.LostPetReportedPayloadVersion != 4 {
		t.Fatalf("lost-pet payload versions = redacted %d/current %d, want 3/4",
			domain.LostPetReportedRedactedPayloadVersion, domain.LostPetReportedPayloadVersion)
	}
	if !strings.Contains(string(v4Payload), report.ImageObject) {
		t.Fatalf("payload-v4 omitted private image object: %s", v4Payload)
	}
	if strings.Contains(string(v4Payload), "owner@example.com") || strings.Contains(string(v4Payload), "reporterEmail") ||
		strings.Contains(string(v4Payload), report.Phone) || strings.Contains(string(v4Payload), "phone") {
		t.Fatalf("payload-v4 exposed private contact: %s", v4Payload)
	}
	publicPayload, err := json.Marshal(report.Public())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicPayload), "imageObject") || strings.Contains(string(publicPayload), report.ImageObject) {
		t.Fatalf("public lost-pet DTO exposed private image object: %s", publicPayload)
	}

	v3 := domain.LostPetReportedV3{
		PetID:           "lost-v3",
		PetName:         "Luna",
		Species:         "Dog",
		ReportedAt:      reportedAt,
		Location:        "Portland, OR",
		GeocodingStatus: domain.GeocodingPending,
		Status:          domain.LostPetStatusLost,
	}
	v3Payload, err := json.Marshal(v3)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		data            []byte
		wantPetID       string
		wantImageObject string
		wantVersion     int
	}{
		{
			name:        "contact-redacted payload-v3",
			data:        lostEnvelope(t, v3.PetID, reportedAt, domain.LostPetReportedRedactedPayloadVersion, v3Payload),
			wantPetID:   v3.PetID,
			wantVersion: 3,
		},
		{
			name:            "private-image payload-v4",
			data:            lostEnvelope(t, report.PetID, reportedAt, domain.LostPetReportedPayloadVersion, v4Payload),
			wantPetID:       report.PetID,
			wantImageObject: report.ImageObject,
			wantVersion:     4,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event, envelope, err := domain.DecodeLostPetReported(test.data)
			if err != nil {
				t.Fatalf("DecodeLostPetReported() error = %v", err)
			}
			if event.PetID != test.wantPetID || event.ImageObject != test.wantImageObject {
				t.Fatalf("decoded event = %#v", event)
			}
			if envelope == nil || envelope.PayloadVersion != test.wantVersion {
				t.Fatalf("decoded envelope = %#v, want version %d", envelope, test.wantVersion)
			}
		})
	}
}

func TestDecodeLostPetReportedRejectsIncompletePayloadV4(t *testing.T) {
	reportedAt := time.Date(2026, time.August, 17, 16, 0, 0, 0, time.UTC)
	base := domain.LostPetReportedV4{
		PetID:           "lost-incomplete-v4",
		ImageObject:     "images/lost-pets/lost-incomplete-v4/image.jpg",
		ReportedAt:      reportedAt,
		Location:        "Seattle, WA",
		GeocodingStatus: domain.GeocodingPending,
		Status:          domain.LostPetStatusLost,
	}
	tests := []struct {
		name   string
		mutate func(*domain.LostPetReportedV4)
	}{
		{name: "missing reportedAt", mutate: func(event *domain.LostPetReportedV4) { event.ReportedAt = time.Time{} }},
		{name: "missing geocodingStatus", mutate: func(event *domain.LostPetReportedV4) { event.GeocodingStatus = "" }},
		{name: "missing status", mutate: func(event *domain.LostPetReportedV4) { event.Status = "" }},
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
				t.Fatal("DecodeLostPetReported() error = nil, want incomplete payload-v4 rejection")
			}
		})
	}
}
