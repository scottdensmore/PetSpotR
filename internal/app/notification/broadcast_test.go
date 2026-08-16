package notification

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/scottdensmore/petspotr/pkg/domain"
	"github.com/scottdensmore/petspotr/pkg/pubsub"
)

func TestGeoBroadcastEngine(t *testing.T) {
	emailSender := NewMockEmailSender()
	smsSender := NewMockSMSSender()
	pushSender := NewMockWebPushSender()
	dispatcher := NewMultiChannelDispatcher(emailSender, smsSender, pushSender)

	// Community Subscribers
	subscribers := []CommunitySubscriber{
		{
			ID:          "sub-1",
			Email:       "capitolhill.resident@example.com",
			Phone:       "+12065550100",
			Coordinates: domain.LocationPoint{Latitude: 47.6150, Longitude: -122.3200}, // Capitol Hill
			RadiusMiles: 5.0,
			Channels:    []Channel{ChannelEmail, ChannelSMS},
		},
		{
			ID:          "sub-2",
			Email:       "ballard.resident@example.com",
			Phone:       "+12065550200",
			Coordinates: domain.LocationPoint{Latitude: 47.6684, Longitude: -122.3847}, // Ballard (~5.2 miles from Capitol Hill)
			RadiusMiles: 3.0,                                                           // Out of 3-mile radius
			Channels:    []Channel{ChannelEmail},
		},
	}

	engine := NewGeoBroadcastEngine(subscribers, dispatcher)

	t.Run("dispatches lost pet broadcast alert to nearby subscribers within radius", func(t *testing.T) {
		emailSender.Reset()
		smsSender.Reset()

		evt := &domain.LostPetReportedV3{
			PetID:           "lost-buddy",
			ReportedAt:      time.Now().UTC(),
			Location:        "Capitol Hill, Seattle, WA",
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     &domain.LocationPoint{Latitude: 47.615, Longitude: -122.32},
			Status:          domain.LostPetStatusLost,
		}

		results, err := engine.BroadcastLostPetAlert(context.Background(), evt, 5.0)
		if err != nil {
			t.Fatalf("BroadcastLostPetAlert failed: %v", err)
		}

		// Only sub-1 (Capitol Hill, within radius) should receive broadcast
		if len(results) != 2 { // sub-1 has Email + SMS
			t.Errorf("expected 2 channel dispatch results for sub-1, got %d", len(results))
		}

		if len(emailSender.SentMessages) != 1 {
			t.Errorf("expected 1 email broadcast sent to nearby resident, got %d", len(emailSender.SentMessages))
		}
	})

	t.Run("does not broadcast canonical reports without verified coordinates", func(t *testing.T) {
		emailSender.Reset()
		smsSender.Reset()

		evt := &domain.LostPetReportedV3{
			PetID:           "lost-pending-location",
			ReportedAt:      time.Now().UTC(),
			Location:        "Capitol Hill, Seattle, WA",
			GeocodingStatus: domain.GeocodingPending,
			Status:          domain.LostPetStatusLost,
		}

		results, err := engine.BroadcastLostPetAlert(context.Background(), evt, 5.0)
		if err != nil {
			t.Fatalf("BroadcastLostPetAlert failed: %v", err)
		}
		if len(results) != 0 || len(emailSender.SentMessages) != 0 || len(smsSender.SentMessages) != 0 {
			t.Fatalf("unverified location dispatched results=%d email=%d sms=%d", len(results), len(emailSender.SentMessages), len(smsSender.SentMessages))
		}
	})
}

func TestWorker_ProcessLostPetBroadcast(t *testing.T) {
	ps := pubsub.NewMemoryPubSub()
	worker := NewWorker(ps)

	t.Run("raw payload-v1 is accepted without inventing coordinates", func(t *testing.T) {
		evt := domain.LostPetEvent{
			PetID:         "lost-luna",
			ReporterEmail: "owner@example.com",
			ReportedAt:    time.Now().UTC(),
			Location:      "Green Lake Park, Seattle, WA",
		}
		data, _ := evt.ToJSON()

		results, err := worker.ProcessLostPetBroadcast(context.Background(), data)
		if err != nil {
			t.Fatalf("ProcessLostPetBroadcast failed: %v", err)
		}

		if len(results) != 0 {
			t.Errorf("payload-v1 broadcast results = %d, want none until geocoding", len(results))
		}
	})

	t.Run("versioned lostPet envelope remains consumable", func(t *testing.T) {
		evt := domain.LostPetEvent{
			PetID:         "lost-envelope-luna",
			ReporterEmail: "owner@example.com",
			ReportedAt:    time.Now().UTC(),
			Location:      "Green Lake Park, Seattle, WA",
		}
		payload, _ := evt.ToJSON()
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeLostPetReported,
			OccurredAt:       evt.ReportedAt,
			AggregateID:      evt.PetID,
			AggregateVersion: 1,
			PayloadVersion:   1,
			Payload:          payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(envelope)
		results, err := worker.ProcessLostPetBroadcast(context.Background(), data)
		if err != nil {
			t.Fatalf("ProcessLostPetBroadcast(envelope) error = %v", err)
		}
		if len(results) != 0 {
			t.Errorf("payload-v1 envelope results = %d, want none until geocoding", len(results))
		}
	})

	t.Run("canonical payload-v2 does not require reporter contact", func(t *testing.T) {
		evt := domain.LostPetReportedV2{
			PetID:           "lost-canonical-luna",
			PetName:         "Luna",
			Species:         "Dog",
			ReportedAt:      time.Now().UTC(),
			Location:        "Green Lake Park, Seattle, WA",
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     &domain.LocationPoint{Latitude: 47.68, Longitude: -122.329},
			Status:          domain.LostPetStatusLost,
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeLostPetReported,
			OccurredAt:       evt.ReportedAt,
			AggregateID:      evt.PetID,
			AggregateVersion: 1,
			PayloadVersion:   domain.LostPetReportedContactPayloadVersion,
			Payload:          payload,
		})
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(envelope)
		if err != nil {
			t.Fatal(err)
		}
		results, err := worker.ProcessLostPetBroadcast(context.Background(), data)
		if err != nil {
			t.Fatalf("ProcessLostPetBroadcast(payload-v2) error = %v", err)
		}
		if len(results) == 0 {
			t.Error("expected payload-v2 broadcast dispatch results")
		}
	})

	t.Run("contact-redacted payload-v3 dispatches from verified coordinates", func(t *testing.T) {
		evt := domain.LostPetReportedV3{
			PetID:           "lost-v3-luna",
			PetName:         "Luna",
			Species:         "Dog",
			ReportedAt:      time.Now().UTC(),
			Location:        "Green Lake Park, Seattle, WA",
			GeocodingStatus: domain.GeocodingVerified,
			Coordinates:     &domain.LocationPoint{Latitude: 47.68, Longitude: -122.329},
			Status:          domain.LostPetStatusLost,
		}
		payload, err := json.Marshal(evt)
		if err != nil {
			t.Fatal(err)
		}
		envelope, err := domain.NewEventEnvelope(domain.EventEnvelopeInput{
			Type:             domain.EventTypeLostPetReported,
			OccurredAt:       evt.ReportedAt,
			AggregateID:      evt.PetID,
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
		results, err := worker.ProcessLostPetBroadcast(context.Background(), data)
		if err != nil {
			t.Fatalf("ProcessLostPetBroadcast(payload-v3) error = %v", err)
		}
		if len(results) == 0 {
			t.Error("expected payload-v3 broadcast dispatch results")
		}
	})
}
