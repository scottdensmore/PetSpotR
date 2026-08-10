package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
)

func TestNewEventEnvelopeIsStableAndComplete(t *testing.T) {
	payload := json.RawMessage(`{"petId":"lost-101"}`)
	occurredAt := time.Date(2026, time.August, 10, 18, 30, 0, 0, time.UTC)
	input := domain.EventEnvelopeInput{
		Type:             "petspotr.lost-pet.reported",
		OccurredAt:       occurredAt,
		CorrelationID:    "corr-123",
		TraceID:          "trace-456",
		AggregateID:      "lost-101",
		AggregateVersion: 1,
		PayloadVersion:   1,
		Payload:          payload,
	}

	first, err := domain.NewEventEnvelope(input)
	if err != nil {
		t.Fatalf("NewEventEnvelope() error = %v", err)
	}
	second, err := domain.NewEventEnvelope(input)
	if err != nil {
		t.Fatalf("NewEventEnvelope() second error = %v", err)
	}

	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("stable event ID = %q, second = %q", first.ID, second.ID)
	}
	if first.EnvelopeVersion != domain.CurrentEnvelopeVersion {
		t.Fatalf("EnvelopeVersion = %d, want %d", first.EnvelopeVersion, domain.CurrentEnvelopeVersion)
	}
	if first.OccurredAt != occurredAt || first.CorrelationID != "corr-123" || first.TraceID != "trace-456" {
		t.Fatalf("envelope metadata = %#v", first)
	}
}

func TestDecodeEventPayloadAcceptsEnvelopeAndLegacyPayload(t *testing.T) {
	want := domain.LostPetEvent{
		PetID:         "lost-101",
		ReporterEmail: "owner@example.com",
		ReportedAt:    time.Date(2026, time.August, 10, 18, 30, 0, 0, time.UTC),
		Location:      "Seattle, WA",
	}
	payload, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             "petspotr.lost-pet.reported",
		OccurredAt:       want.ReportedAt,
		CorrelationID:    "corr-123",
		TraceID:          "trace-123",
		AggregateID:      want.PetID,
		AggregateVersion: 1,
		PayloadVersion:   1,
		Payload:          payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	wrapped, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}

	for name, data := range map[string][]byte{"envelope": wrapped, "legacy": payload} {
		t.Run(name, func(t *testing.T) {
			var got domain.LostPetEvent
			metadata, err := domain.DecodeEventPayload(data, "petspotr.lost-pet.reported", &got)
			if err != nil {
				t.Fatalf("DecodeEventPayload() error = %v", err)
			}
			if got.PetID != want.PetID || got.ReporterEmail != want.ReporterEmail {
				t.Fatalf("decoded payload = %#v, want %#v", got, want)
			}
			if name == "envelope" && metadata == nil {
				t.Fatal("DecodeEventPayload() metadata = nil for envelope")
			}
			if name == "legacy" && metadata != nil {
				t.Fatalf("DecodeEventPayload() metadata = %#v for legacy payload", metadata)
			}
		})
	}
}

func TestDecodeEventPayloadRejectsTamperedEnvelopeIdentity(t *testing.T) {
	payload := json.RawMessage(`{"petId":"lost-101","reporterEmail":"owner@example.com"}`)
	envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
		Type:             domain.EventTypeLostPetReported,
		OccurredAt:       time.Now().UTC(),
		AggregateID:      "lost-101",
		AggregateVersion: 1,
		PayloadVersion:   1,
		Payload:          payload,
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]domain.EventEnvelope{
		"changed ID": func() domain.EventEnvelope {
			changed := envelope
			changed.ID = "evt_tampered"
			return changed
		}(),
		"changed payload": func() domain.EventEnvelope {
			changed := envelope
			changed.Payload = json.RawMessage(`{"petId":"lost-999","reporterEmail":"attacker@example.com"}`)
			return changed
		}(),
	}

	for name, tampered := range tests {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(tampered)
			if err != nil {
				t.Fatal(err)
			}
			var event domain.LostPetEvent
			if _, err := domain.DecodeEventPayload(data, domain.EventTypeLostPetReported, &event); err == nil {
				t.Fatal("DecodeEventPayload() error = nil, want identity mismatch")
			}
		})
	}
}
